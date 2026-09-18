package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	kiroACPTimeout    = 40 * time.Second
	kiroACPMaxBytes   = 1024 * 1024
	kiroACPMaxLine    = 256 * 1024
	kiroACPMaxRecords = 256
	kiroACPSettlement = 150 * time.Millisecond
)

var (
	errKiroACPContractUnknown = errors.New("Kiro structured ACP verification is unavailable or unrecognized")
	errKiroACPEarlyExit       = errors.New("Kiro ACP process exited before verification completed")
	errKiroACPPartialExit     = fmt.Errorf("%w: Kiro ACP process exited with a partial trailing record", shared.ErrRecognizedNegativeEvidence)
)

type duplexCommandRunner = ports.DuplexCommandRunner

type duplexCapabilityRunner = ports.DuplexCapabilityRunner

type kiroACPServerState struct {
	connecting       bool
	connected        bool
	sessionID        string
	connectingRecord string
	connectedRecord  string
}

// verifyKiroACP performs only ACP initialization and session creation. The
// empty mcpServers list is intentional: Kiro must discover the native config
// installed by the preceding import rather than receive package definitions
// through the verification channel.
func verifyKiroACP(ctx context.Context, runner duplexCommandRunner, executable, cwd string, servers []string) error {
	return verifyKiroACPWithTimeout(ctx, runner, executable, cwd, servers, kiroACPTimeout)
}

func verifyKiroACPWithTimeout(ctx context.Context, runner duplexCommandRunner, executable, cwd string, servers []string, startupTimeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runner == nil {
		return fmt.Errorf("%w: interactive process runner is unavailable", errKiroACPContractUnknown)
	}
	absoluteCWD, err := filepath.Abs(cwd)
	if err != nil || !filepath.IsAbs(absoluteCWD) {
		return fmt.Errorf("%w: resolve prepared verification directory", errKiroACPContractUnknown)
	}
	expected := make(map[string]*kiroACPServerState, len(servers))
	for _, rawName := range servers {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return fmt.Errorf("%w: planned MCP server name is empty", errKiroACPContractUnknown)
		}
		if _, duplicate := expected[name]; duplicate {
			return fmt.Errorf("%w: duplicate planned MCP server identity %q", errKiroACPContractUnknown, name)
		}
		expected[name] = &kiroACPServerState{}
	}
	if len(expected) == 0 {
		return fmt.Errorf("%w: no planned MCP servers", errKiroACPContractUnknown)
	}

	if startupTimeout <= 0 {
		return fmt.Errorf("%w: ACP startup timeout must be positive", errKiroACPContractUnknown)
	}
	bounded, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()
	command := legacyports.Command{
		Argv: []string{executable, "acp", "--agent-engine", "v3", "--auth-method", "cli"},
		Dir:  absoluteCWD,
	}
	var exchangeEntered atomic.Bool
	err = runner.RunDuplexWithPlannedShutdown(bounded, command, func(stdin io.Writer, stdout io.Reader) error {
		exchangeEntered.Store(true)
		exchangeErr := exchangeKiroACP(stdin, stdout, absoluteCWD, expected)
		if exchangeErr == nil {
			return nil
		}
		// Once the client has entered the ACP exchange, malformed records,
		// partial output, protocol violations, and stream/process interruption
		// are authoritative verification failures rather than fallback signals.
		return errors.Join(shared.ErrRecognizedNegativeEvidence, exchangeErr)
	})
	if err == nil {
		if ctxErr := bounded.Err(); ctxErr != nil {
			return errors.Join(shared.ErrRecognizedNegativeEvidence, ctxErr)
		}
		return nil
	}
	if exchangeEntered.Load() {
		if !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
			err = errors.Join(shared.ErrRecognizedNegativeEvidence, err)
		}
		// The phase-specific deadline belongs to the ACP exchange even when pipe
		// closure is observed first. Preserve that cause instead of degrading a
		// no-data timeout into an indistinguishable early EOF.
		if boundedErr := bounded.Err(); boundedErr != nil && !errors.Is(err, boundedErr) {
			err = errors.Join(err, boundedErr)
		}
	}
	if errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(err, ctxErr) {
			return errors.Join(err, ctxErr)
		}
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("%w: %v", errKiroACPContractUnknown, err)
}

func exchangeKiroACP(stdin io.Writer, stdout io.Reader, cwd string, expected map[string]*kiroACPServerState) error {
	_, ok := stdin.(io.Closer)
	if !ok {
		return fmt.Errorf("ACP stdin does not support close")
	}
	initialize := map[string]any{
		"jsonrpc": "2.0", "id": 0, "method": "initialize",
		"params": map[string]any{
			"protocolVersion":    1,
			"clientCapabilities": map[string]any{},
			"clientInfo":         map[string]any{"name": "agentplugins", "version": "1"},
		},
	}
	if err := writeACPRecord(stdin, initialize); err != nil {
		return fmt.Errorf("write initialize request: %w", err)
	}

	reader := bufio.NewReaderSize(stdout, kiroACPMaxLine+1)
	initialized := false
	sessionCreated := false
	sessionID := ""
	var negativeErr error
	settling := false
	totalBytes := 0
	for record := 0; ; record++ {
		line, err := readBoundedACPLine(reader, &totalBytes)
		if err != nil {
			if negativeErr != nil {
				return negativeErr
			}
			if settling && errors.Is(err, os.ErrDeadlineExceeded) {
				queued, eof, probeErr := queuedACPRealPipeEvidence(stdout)
				if probeErr != nil {
					return fmt.Errorf("inspect ACP pipe at settlement boundary: %w", probeErr)
				}
				if eof {
					return fmt.Errorf("%w: ACP output reached EOF at the post-success settlement boundary", shared.ErrRecognizedNegativeEvidence)
				}
				if !queued {
					return nil
				}
				deadlineWriter := stdout.(interface{ SetReadDeadline(time.Time) error })
				if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
					return fmt.Errorf("rearm ACP queued-evidence drain deadline: %w", err)
				}
				continue
			}
			if settling && errors.Is(err, errKiroACPEarlyExit) {
				return fmt.Errorf("%w: ACP output reached EOF during post-success settlement", shared.ErrRecognizedNegativeEvidence)
			}
			if sessionCreated && !allKiroServersConnected(expected, sessionID) && errors.Is(err, errKiroACPEarlyExit) {
				return fmt.Errorf("%w: not every planned MCP server reported connected", shared.ErrRecognizedNegativeEvidence)
			}
			return err
		}
		if record >= kiroACPMaxRecords {
			return fmt.Errorf("ACP output exceeded its record bound")
		}
		message, err := decodeACPMessage(line)
		if err != nil {
			if negativeErr != nil {
				return negativeErr
			}
			return err
		}
		if !initialized {
			if err := validateInitializeResponse(message); err != nil {
				return err
			}
			initialized = true
			sessionRequest := map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "session/new",
				"params": map[string]any{"cwd": cwd, "mcpServers": []any{}},
			}
			if err := writeACPRecord(stdin, sessionRequest); err != nil {
				return fmt.Errorf("write session/new request: %w", err)
			}
			continue
		}

		if _, hasID := message.document["id"]; hasID {
			if sessionCreated || !acpIDEquals(message.document["id"], 1) {
				if negativeErr != nil {
					return negativeErr
				}
				return fmt.Errorf("ACP response has an unexpected or duplicate id")
			}
			result, ok := message.document["result"].(map[string]any)
			_, hasError := message.document["error"]
			if !ok || hasError {
				if negativeErr != nil {
					return negativeErr
				}
				return fmt.Errorf("session/new did not return a valid result")
			}
			var okSession bool
			sessionID, okSession = result["sessionId"].(string)
			if !okSession || strings.TrimSpace(sessionID) == "" {
				if negativeErr != nil {
					return negativeErr
				}
				return fmt.Errorf("session/new response is missing its session identity")
			}
			sessionCreated = true
			for _, state := range expected {
				if state.sessionID != "" && state.sessionID != sessionID {
					return fmt.Errorf("%w: MCP status belongs to a different ACP session", shared.ErrRecognizedNegativeEvidence)
				}
			}
		} else {
			method, ok := message.document["method"].(string)
			if !ok || strings.TrimSpace(method) == "" {
				if negativeErr != nil {
					return negativeErr
				}
				return fmt.Errorf("ACP notification is missing its method")
			}
			if method == "_kiro/mcp/status" {
				if err := consumeKiroMCPStatus(message.document, expected); err != nil {
					if errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
						if negativeErr == nil {
							negativeErr = err
						}
					} else {
						if negativeErr != nil {
							return negativeErr
						}
						return err
					}
				}
			}
		}

		if sessionCreated && negativeErr != nil {
			return negativeErr
		}
		if sessionCreated && allKiroServersConnected(expected, sessionID) {
			// Kiro ACP is intentionally long-lived. Keep stdin open while draining
			// the bounded settlement window: closing it would manufacture EOF and
			// can let the leader exit while a KAS descendant survives. A nil return
			// after settlement causally requests the runner's planned tree shutdown.
			if !settling {
				deadlineWriter, ok := stdout.(interface{ SetReadDeadline(time.Time) error })
				if !ok {
					// Finite readers used by callers/tests still provide EOF as the
					// completion boundary. Real process pipes always support deadlines.
					continue
				}
				if err := deadlineWriter.SetReadDeadline(time.Now().Add(kiroACPSettlement)); err != nil {
					return fmt.Errorf("arm ACP post-success settlement deadline: %w", err)
				}
				settling = true
			}
		}
	}
}

type acpMessage struct {
	document map[string]any
}

func consumeKiroMCPStatus(document map[string]any, expected map[string]*kiroACPServerState) error {
	params, ok := document["params"].(map[string]any)
	if !ok {
		return fmt.Errorf("Kiro MCP status params are malformed")
	}
	rawServers, hasServers := params["servers"]
	_, hasServerName := params["serverName"]
	if hasServers && hasServerName {
		return fmt.Errorf("Kiro MCP status mixes array and legacy server shapes")
	}
	if _, hasAlias := params["name"]; hasAlias {
		return fmt.Errorf("Kiro MCP status contains an unknown or conflicting server identity")
	}
	sessionID, ok := params["sessionId"].(string)
	if !ok || strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("Kiro MCP status is missing its session identity")
	}
	if hasServers {
		servers, ok := rawServers.([]any)
		if !ok || len(servers) == 0 {
			return fmt.Errorf("Kiro MCP status servers are malformed or empty")
		}
		seen := make(map[string]struct{}, len(servers))
		for _, rawServer := range servers {
			server, ok := rawServer.(map[string]any)
			if !ok {
				return fmt.Errorf("Kiro MCP status contains a malformed server record")
			}
			if _, conflicting := server["serverName"]; conflicting {
				return fmt.Errorf("Kiro MCP status array record contains a conflicting identity field")
			}
			name, ok := server["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return fmt.Errorf("Kiro MCP status server has no unambiguous identity")
			}
			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("Kiro MCP status contains duplicate server identity %q", name)
			}
			seen[name] = struct{}{}
			if err := consumeKiroMCPServer(name, sessionID, server, expected); err != nil {
				return err
			}
		}
		// The array form is a complete native-registry snapshot, not a delta.
		// Once a planned server has appeared, its omission from a later full
		// snapshot revokes the sticky state accumulated from older snapshots.
		// Failing immediately also prevents a previously complete snapshot from
		// surviving the settlement window after Kiro removes a server.
		for name, state := range expected {
			if _, present := seen[name]; !present && (state.connecting || state.connected) {
				return fmt.Errorf("%w: full Kiro MCP status snapshot omitted planned server %s", shared.ErrRecognizedNegativeEvidence, name)
			}
		}
		return nil
	}
	if !hasServerName {
		return fmt.Errorf("Kiro MCP status has no recognized server shape")
	}
	name, ok := params["serverName"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("Kiro MCP status has no unambiguous server identity")
	}
	return consumeKiroMCPServer(name, sessionID, params, expected)
}

func consumeKiroMCPServer(name, sessionID string, server map[string]any, expected map[string]*kiroACPServerState) error {
	if authType, present := server["authType"]; present {
		value, valid := authType.(string)
		if !valid || strings.TrimSpace(value) == "" {
			return fmt.Errorf("Kiro MCP server %s has malformed auth type", name)
		}
	}
	if disabled, present := server["disabled"]; present {
		if _, valid := disabled.(bool); !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed disabled state", name)
		}
	}
	if status, valid := server["status"].(string); !valid || strings.TrimSpace(status) == "" {
		return fmt.Errorf("Kiro MCP server %s has malformed status", name)
	}
	if rawTools, present := server["tools"]; present && rawTools != nil {
		tools, valid := rawTools.([]any)
		if !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed tools", name)
		}
		seenToolNames := make(map[string]struct{}, len(tools))
		for _, rawTool := range tools {
			tool, valid := rawTool.(map[string]any)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s has a malformed tool record", name)
			}
			toolName, valid := tool["name"].(string)
			if !valid || strings.TrimSpace(toolName) == "" {
				return fmt.Errorf("Kiro MCP server %s has an unnamed tool", name)
			}
			if _, duplicate := seenToolNames[toolName]; duplicate {
				return fmt.Errorf("Kiro MCP server %s has duplicate tool identity %s", name, toolName)
			}
			seenToolNames[toolName] = struct{}{}
			if disabled, valid := tool["disabled"].(bool); !valid {
				return fmt.Errorf("Kiro MCP server %s has a tool with malformed disabled state", name)
			} else if disabled {
				// Disabled tools are authoritative only for planned servers. An
				// unrelated native-registry entry must not veto this package.
				if _, planned := expected[name]; planned {
					return fmt.Errorf("%w: Kiro MCP server %s has a disabled tool", shared.ErrRecognizedNegativeEvidence, name)
				}
			}
		}
	}
	state, planned := expected[name]
	if !planned {
		// Kiro loads the complete native registry. Other well-formed server
		// notifications are unrelated to the package being verified.
		return nil
	}
	if state.sessionID != "" && state.sessionID != sessionID {
		return fmt.Errorf("%w: Kiro MCP server %s has conflicting session identities", shared.ErrRecognizedNegativeEvidence, name)
	}
	state.sessionID = sessionID
	if disabled, present := server["disabled"]; present {
		value, valid := disabled.(bool)
		if !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed disabled state", name)
		}
		if value {
			return fmt.Errorf("%w: Kiro MCP server %s is disabled", shared.ErrRecognizedNegativeEvidence, name)
		}
	}
	status, ok := server["status"].(string)
	if !ok {
		return fmt.Errorf("Kiro MCP server %s has malformed status", name)
	}
	switch status {
	case "connecting":
		fingerprint, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("encode Kiro MCP connecting state: %w", err)
		}
		// Once connected has been observed, every connecting record is a
		// regression. In particular, do not accept an exact duplicate of the
		// earlier connecting record when it was queued before settlement.
		if state.connected {
			return fmt.Errorf("%w: Kiro MCP server %s has regressive connecting status after connected", shared.ErrRecognizedNegativeEvidence, name)
		}
		if state.connecting && state.connectingRecord == string(fingerprint) {
			return nil
		}
		if state.connecting {
			return fmt.Errorf("%w: Kiro MCP server %s has duplicate or regressive status", shared.ErrRecognizedNegativeEvidence, name)
		}
		state.connecting = true
		state.connectingRecord = string(fingerprint)
		return nil
	case "connected":
		fingerprint, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("encode Kiro MCP connected state: %w", err)
		}
		if state.connected {
			// The servers-array notification is a full registry snapshot. While
			// another server advances, Kiro repeats already-connected entries in
			// later snapshots. An identical repeat is idempotent evidence; a
			// changed connected record remains contradictory and fails closed.
			if state.connectedRecord == string(fingerprint) {
				return nil
			}
			return fmt.Errorf("%w: Kiro MCP server %s has duplicate connected identities", shared.ErrRecognizedNegativeEvidence, name)
		}
		tools, present := server["tools"].([]any)
		if !present || len(tools) == 0 {
			return fmt.Errorf("%w: Kiro MCP server %s has no usable tools", shared.ErrRecognizedNegativeEvidence, name)
		}
		seenTools := make(map[string]struct{}, len(tools))
		for _, rawTool := range tools {
			tool, valid := rawTool.(map[string]any)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s has a malformed tool record", name)
			}
			toolName, valid := tool["name"].(string)
			if !valid || strings.TrimSpace(toolName) == "" {
				return fmt.Errorf("Kiro MCP server %s has an unnamed tool", name)
			}
			if _, duplicate := seenTools[toolName]; duplicate {
				return fmt.Errorf("Kiro MCP server %s has duplicate tool identity %s", name, toolName)
			}
			seenTools[toolName] = struct{}{}
			disabled, valid := tool["disabled"].(bool)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s tool %s has malformed disabled state", name, toolName)
			}
			if disabled {
				return fmt.Errorf("%w: Kiro MCP server %s tool %s is disabled", shared.ErrRecognizedNegativeEvidence, name, toolName)
			}
		}
		state.connected = true
		state.connectedRecord = string(fingerprint)
		return nil
	case "pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure", "unhealthy":
		return fmt.Errorf("%w: Kiro MCP server %s reported %s", shared.ErrRecognizedNegativeEvidence, name, status)
	default:
		return fmt.Errorf("Kiro MCP server %s reported an unknown status %q", name, status)
	}
}

func allKiroServersConnected(expected map[string]*kiroACPServerState, sessionID string) bool {
	for _, state := range expected {
		if !state.connected || state.sessionID != sessionID {
			return false
		}
	}
	return true
}

func writeACPRecord(writer io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	written, err := writer.Write(body)
	if err == nil && written != len(body) {
		return io.ErrShortWrite
	}
	return err
}

func readBoundedACPLine(reader *bufio.Reader, total *int) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	*total += len(line)
	if errors.Is(err, bufio.ErrBufferFull) || len(line) > kiroACPMaxLine || *total > kiroACPMaxBytes {
		return nil, fmt.Errorf("ACP output exceeded its byte bound")
	}
	if err != nil {
		// ReadSlice may return both bytes and a terminal/deadline error. Those
		// Bytes accompanying EOF or another terminal read error are queued
		// protocol evidence. Preserve them as a partial-record failure.
		if len(line) > 0 {
			return nil, fmt.Errorf("%w: ACP output ended with a non-delimited %d-byte record", errKiroACPPartialExit, len(line))
		}
		if errors.Is(err, io.EOF) {
			return nil, errKiroACPEarlyExit
		}
		return nil, fmt.Errorf("read ACP output: %w", err)
	}
	if len(line) == 1 {
		return nil, fmt.Errorf("ACP output contained an empty record")
	}
	return line[:len(line)-1], nil
}

func decodeACPMessage(line []byte) (acpMessage, error) {
	// encoding/json accepts invalid UTF-8 by replacing it with U+FFFD. ACP
	// evidence must remain byte-exact: replacement could turn a malformed
	// status, identity, object key, or extension value into trusted evidence.
	if !utf8.Valid(line) {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: record is not valid UTF-8")
	}
	if err := validateJSONSurrogateEscapes(line); err != nil {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(line)))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return acpMessage{}, fmt.Errorf("malformed ACP JSON: %w", err)
	}
	if _, err := decoder.Token(); err == nil || !errors.Is(err, io.EOF) {
		return acpMessage{}, fmt.Errorf("ACP record contains multiple JSON values")
	}
	document, ok := value.(map[string]any)
	if !ok || document["jsonrpc"] != "2.0" {
		return acpMessage{}, fmt.Errorf("ACP record has an invalid JSON-RPC envelope")
	}
	_, hasMethod := document["method"]
	_, hasID := document["id"]
	_, hasResult := document["result"]
	_, hasParams := document["params"]
	errorValue, hasError := document["error"]
	if hasMethod {
		if hasID || hasResult || hasError {
			return acpMessage{}, fmt.Errorf("ACP record mixes JSON-RPC request and response members")
		}
	} else if !hasID || hasParams || hasResult == hasError || hasError && errorValue == nil {
		return acpMessage{}, fmt.Errorf("ACP response does not have an exclusive result or error member")
	}
	return acpMessage{document: document}, nil
}

func validateJSONSurrogateEscapes(document []byte) error {
	inString := false
	for index := 0; index < len(document); index++ {
		switch document[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(document) {
				continue
			}
			index++
			if document[index] != 'u' || index+4 >= len(document) {
				continue
			}
			value, ok := parseJSONHexQuad(document[index+1 : index+5])
			if !ok {
				continue // encoding/json reports the malformed escape itself
			}
			index += 4
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+6 >= len(document) || document[index+1] != '\\' || document[index+2] != 'u' {
					return fmt.Errorf("unpaired high UTF-16 surrogate escape")
				}
				low, valid := parseJSONHexQuad(document[index+3 : index+7])
				if !valid || low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("unpaired high UTF-16 surrogate escape")
				}
				index += 6
			case value >= 0xdc00 && value <= 0xdfff:
				return fmt.Errorf("unpaired low UTF-16 surrogate escape")
			}
		}
	}
	return nil
}

func parseJSONHexQuad(value []byte) (uint16, bool) {
	if len(value) != 4 {
		return 0, false
	}
	var result uint16
	for _, digit := range value {
		result <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			result |= uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			result |= uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			result |= uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return result, true
}

func validateInitializeResponse(message acpMessage) error {
	if !acpIDEquals(message.document["id"], 0) {
		return fmt.Errorf("initialize response has the wrong id")
	}
	result, ok := message.document["result"].(map[string]any)
	_, hasError := message.document["error"]
	if !ok || hasError || !acpIDEquals(result["protocolVersion"], 1) {
		return fmt.Errorf("initialize response did not select ACP protocol v1")
	}
	return nil
}

func acpIDEquals(value any, expected int) bool {
	number, ok := value.(json.Number)
	return ok && number.String() == fmt.Sprintf("%d", expected)
}
