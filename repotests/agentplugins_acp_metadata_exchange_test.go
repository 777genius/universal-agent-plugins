//go:build linux || darwin

package pluginkitairepo_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

const acpMetadataLimit = (1 << 20) - (32 << 10)
const acpMetadataMarker = "uap-acp-metadata-marker"

type acpMetadataExchange struct {
	Frames          []map[string]any `json:"frames"`
	Sent            []map[string]any `json:"sent"`
	Observation     string           `json:"observation"`
	StoppedMethod   string           `json:"stopped_method,omitempty"`
	SessionID       string           `json:"session_id,omitempty"`
	StdoutEOF       bool             `json:"stdout_eof"`
	MarkerListed    bool             `json:"marker_listed"`
	seen            map[string]bool
	closed          bool
	pendingSessions []string
}

func (p *acpMetadataExchange) send(w io.Writer, id int, method string, params any) error {
	frame := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	p.Sent = append(p.Sent, frame)
	return json.NewEncoder(w).Encode(frame)
}

func (p *acpMetadataExchange) stop(w io.Writer, observation string) error {
	p.Observation = observation
	if p.closed {
		return nil
	}
	p.closed = true
	closer, ok := w.(io.Closer)
	if !ok {
		return fmt.Errorf("finite ACP stdin must be closeable")
	}
	return closer.Close()
}

// Only two requests are permitted. Closing stdin also lets late metadata drain
// to EOF without requiring a prompt, permission reply or planned termination.
func (p *acpMetadataExchange) exchange(w io.Writer, r io.Reader, cwd string) error {
	p.seen = map[string]bool{}
	p.Observation = "initializing"
	if err := p.send(w, 1, "initialize", map[string]any{
		"protocolVersion": 1, "clientCapabilities": map[string]any{
			"fs": map[string]bool{"readTextFile": false, "writeTextFile": false}, "terminal": false},
		"clientInfo": map[string]string{"name": "uap-acp-metadata-observer", "version": "1"},
	}); err != nil {
		return err
	}
	limited := &io.LimitedReader{R: r, N: acpMetadataLimit + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), acpMetadataLimit+1)
	for scanner.Scan() {
		if limited.N <= 0 || len(p.Frames) >= 64 || !utf8.Valid(scanner.Bytes()) {
			return fmt.Errorf("ACP transcript bound")
		}
		frame, err := shared.DecodeStrictJSONObject(scanner.Bytes())
		if err != nil || frame["jsonrpc"] != "2.0" {
			return fmt.Errorf("ACP frame shape")
		}
		if err := p.frame(w, frame, cwd); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil || limited.N <= 0 {
		return errors.Join(err, fmt.Errorf("ACP transcript incomplete or oversized"))
	}
	p.StdoutEOF = true
	if !p.closed {
		return fmt.Errorf("ACP EOF before terminal response")
	}
	for _, session := range p.pendingSessions {
		if session != p.SessionID {
			return fmt.Errorf("ACP update session differs")
		}
	}
	return nil
}

func (p *acpMetadataExchange) frame(w io.Writer, frame map[string]any, cwd string) error {
	if method, exists := frame["method"]; exists {
		name, ok := method.(string)
		if !ok || name == "" {
			return fmt.Errorf("ACP method shape")
		}
		_, request := frame["id"]
		if request || strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") || strings.Contains(strings.ToLower(name), "permission") {
			// Never publish auth/token/permission request parameters or answer it.
			p.Frames = append(p.Frames, map[string]any{"jsonrpc": "2.0", "method": name, "request": true})
			p.StoppedMethod = name
			state := "client-request-observed"
			if strings.Contains(strings.ToLower(name), "auth") || strings.Contains(strings.ToLower(name), "token") {
				state = "auth-request-observed"
			}
			return p.stop(w, state)
		}
		p.Frames = append(p.Frames, frame)
		return p.notification(w, frame)
	}
	return p.response(w, frame, cwd)
}

func (p *acpMetadataExchange) response(w io.Writer, frame map[string]any, cwd string) error {
	id, ok := frame["id"].(json.Number)
	sent := id == "1" && len(p.Sent) >= 1 || id == "2" && len(p.Sent) >= 2
	if !ok || !sent || p.seen[string(id)] {
		return fmt.Errorf("ACP unexpected or duplicate response ID")
	}
	p.seen[string(id)] = true
	result, resultOK := frame["result"].(map[string]any)
	failure, failed := frame["error"]
	if failed {
		failureObject, objectOK := failure.(map[string]any)
		_, codeOK := failureObject["code"].(json.Number)
		_, messageOK := failureObject["message"].(string)
		if _, exists := frame["result"]; exists || !objectOK || !codeOK || !messageOK {
			return fmt.Errorf("ACP ambiguous error response")
		}
		p.Frames = append(p.Frames, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": failureObject["code"]}})
		return p.stop(w, "rpc-error-observed")
	}
	p.Frames = append(p.Frames, frame)
	if !resultOK || p.closed {
		return fmt.Errorf("ACP response shape or progression after stop")
	}
	if id == "1" {
		capabilityValue, present := result["agentCapabilities"]
		_, capabilities := capabilityValue.(map[string]any)
		if result["protocolVersion"] != json.Number("1") || present && !capabilities {
			return fmt.Errorf("ACP protocol version differs")
		}
		return p.send(w, 2, "session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}})
	}
	if !p.seen["1"] {
		return fmt.Errorf("ACP session before initialize")
	}
	session, ok := result["sessionId"].(string)
	if !ok || session == "" || len(session) > 4096 {
		return fmt.Errorf("ACP session ID shape")
	}
	p.SessionID = session
	return p.stop(w, "session-created-catalog-unobserved")
}

func (p *acpMetadataExchange) notification(w io.Writer, frame map[string]any) error {
	if frame["method"] != "session/update" {
		return nil // Unknown metadata never grants authority or causes a request.
	}
	if !p.seen["1"] {
		return fmt.Errorf("ACP update before initialize")
	}
	params, ok := frame["params"].(map[string]any)
	if !ok {
		return fmt.Errorf("ACP update params")
	}
	session, ok := params["sessionId"].(string)
	update, updateOK := params["update"].(map[string]any)
	if !ok || session == "" || !updateOK {
		return fmt.Errorf("ACP update session/shape")
	}
	p.pendingSessions = append(p.pendingSessions, session)
	kind, _ := update["sessionUpdate"].(string)
	if kind != "available_commands_update" {
		if kind == "tool_call" || kind == "tool_call_update" || kind == "agent_message_chunk" || kind == "agent_thought_chunk" {
			return p.stop(w, "unexpected-execution-update")
		}
		return nil
	}
	commands, ok := update["availableCommands"].([]any)
	if !ok {
		return fmt.Errorf("ACP command catalog shape")
	}
	p.MarkerListed = false // Catalog updates replace, rather than append to, current metadata.
	markerListed := false
	for _, value := range commands {
		row, ok := value.(map[string]any)
		name, nameOK := row["name"].(string)
		_, descriptionOK := row["description"].(string)
		if !ok || !nameOK || !descriptionOK {
			return fmt.Errorf("ACP catalog row")
		}
		markerListed = markerListed || name == acpMetadataMarker
	}
	p.MarkerListed = markerListed
	return nil
}
