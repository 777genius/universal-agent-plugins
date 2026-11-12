package mcpskills

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

const (
	protocolVersion  = "2026-07-28"
	extensionID      = "io.modelcontextprotocol/skills"
	maxResponseBytes = 2 << 20
)

var (
	ErrProtocol = errors.New("unsupported or invalid MCP response")
	ErrRemote   = errors.New("MCP server returned an error")
)

type cacheFields struct {
	ResultType string `json:"resultType"`
	TTLMS      *int64 `json:"ttlMs"`
	CacheScope string `json:"cacheScope"`
}

func (h cacheFields) valid() bool {
	return h.ResultType == "complete" && h.TTLMS != nil && *h.TTLMS >= 0 &&
		(h.CacheScope == "private" || h.CacheScope == "public")
}

// rpcCall intentionally supports only four read-only methods, without a generic
// MCP runtime or implicit server-to-client interactions.
func (c *Client) rpcCall(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	switch method {
	case "server/discover", "skills/list", "skills/get", "resources/read":
	default:
		return nil, ErrProtocol
	}
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = map[string]any{
		"io.modelcontextprotocol/protocolVersion":    protocolVersion,
		"io.modelcontextprotocol/clientInfo":         map[string]string{"name": "agentplugins-mcp-skills-poc", "version": "0.1"},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return nil, ErrProtocol
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ErrProtocol
	}
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("MCP-Protocol-Version", protocolVersion)
	request.Header.Set("Mcp-Method", method)
	if method == "resources/read" {
		uri, ok := params["uri"].(string)
		if !ok || validateURI(uri) != nil {
			return nil, ErrProtocol
		}
		request.Header.Set("Mcp-Name", encodeHeaderValue(uri))
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		return nil, ErrRemote
	}
	defer func() { _ = response.Body.Close() }()
	return decodeHTTPResponse(ctx, response)
}

// decodeHTTPResponse enforces status, encoding and body caps before accepting any result.
func decodeHTTPResponse(ctx context.Context, response *http.Response) (json.RawMessage, error) {
	if response.StatusCode != http.StatusOK {
		return nil, ErrRemote
	}
	if response.ContentLength > maxResponseBytes {
		return nil, ErrProtocol
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, ErrProtocol
	}
	media, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil {
		return nil, ErrProtocol
	}
	if media == "text/event-stream" {
		// An SSE response need not close the stream after the final JSON-RPC
		// reply. Return as soon as that event is complete, within read bounds.
		result, err := decodeEventStream(response.Body, true)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return result, err
	}
	if media != "application/json" {
		return nil, ErrProtocol
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrRemote
	}
	if len(payload) > maxResponseBytes {
		return nil, ErrProtocol
	}
	return decodeResponse(payload)
}

func encodeHeaderValue(value string) string {
	safe := value != "" && strings.TrimSpace(value) == value &&
		!strings.HasPrefix(value, "=?base64?")
	for _, ch := range []byte(value) {
		if ch < ' ' || ch > '~' {
			safe = false
			break
		}
	}
	if safe && utf8.ValidString(value) {
		return value
	}
	return "=?base64?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
}

func decodeResponse(payload []byte) (json.RawMessage, error) {
	if conformance.RejectDuplicateJSONKeys(payload) != nil {
		return nil, ErrProtocol
	}
	var reply struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   json.RawMessage `json:"error"`
		Method  string          `json:"method"`
	}
	if err := json.Unmarshal(payload, &reply); err != nil || reply.Version != "2.0" ||
		string(reply.ID) != "1" || reply.Method != "" {
		return nil, ErrProtocol
	}
	if len(reply.Error) != 0 && string(reply.Error) != "null" {
		return nil, ErrRemote
	}
	if len(reply.Result) == 0 || string(reply.Result) == "null" {
		return nil, ErrProtocol
	}
	return reply.Result, nil
}

func decodeEvents(payload []byte) (json.RawMessage, error) {
	// Offline decoding checks the *whole* payload, including duplicate replies.
	return decodeEventStream(bytes.NewReader(payload), false)
}

func decodeEventStream(reader io.Reader, stopAfterFinal bool) (json.RawMessage, error) {
	// The reader limit covers all SSE lines, not only individual data fields.
	limited := &io.LimitedReader{R: reader, N: maxResponseBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), maxResponseBytes)
	var data []string
	var final json.RawMessage

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := acceptSSEData(data, &final); err != nil {
				return nil, err
			}
			data = nil
			if stopAfterFinal && final != nil {
				if limited.N == 0 {
					return nil, ErrProtocol
				}
				return final, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		// Ignore id, retry and event fields; they are not MCP JSON-RPC messages.
	}
	if scanner.Err() != nil || limited.N == 0 {
		return nil, ErrProtocol
	}
	if err := acceptSSEData(data, &final); err != nil {
		return nil, err
	}
	if final == nil {
		return nil, ErrProtocol
	}
	return final, nil
}

// acceptSSEData allows progress notifications before the one matching response.
func acceptSSEData(data []string, final *json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	raw := []byte(strings.Join(data, "\n"))
	if conformance.RejectDuplicateJSONKeys(raw) != nil {
		return ErrProtocol
	}
	var message struct {
		Version string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
	}
	if json.Unmarshal(raw, &message) != nil || message.Version != "2.0" {
		return ErrProtocol
	}
	if len(message.ID) == 0 && strings.HasPrefix(message.Method, "notifications/") && *final == nil {
		return nil
	}
	if *final != nil || string(message.ID) != "1" || message.Method != "" {
		return ErrProtocol
	}
	result, err := decodeResponse(raw)
	if err != nil {
		return err
	}
	*final = result
	return nil
}
