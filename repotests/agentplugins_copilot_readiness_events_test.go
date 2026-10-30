//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const copilotReadinessMode = "copilot-installed-tool-readiness"
const copilotReadinessPrompt = "Call only the installed inspect_runtime tool once for the fixture challenge."

var copilotReadinessUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// The finite sequence and closed fields are measured in genuine source9b8d,
// run40553325878/native12760476935. Parent UUIDs are not session identities.
var copilotReadinessGrammar = [26][2]string{
	{"session.mcp_server_status_changed", "serverName status"},
	{"session.mcp_server_status_changed", "serverName status configSource"},
	{"session.mcp_server_tools_loaded", "serverName toolCount"},
	{"session.skills_loaded", "skills"},
	{"session.info", "infoType message"},
	{"session.mcp_servers_loaded", "servers"},
	{"session.tools_updated", "model"},
	{"user.message", "content transformedContent messageId supportedNativeDocumentMimeTypes delivery interactionId turnId parentAgentTaskId"},
	{"assistant.turn_start", "turnId interactionId"},
	{"model.call_start", "turnId model"},
	{"assistant.tool_call_delta", "toolCallId toolName toolType inputDelta"},
	{"model.call_finished", "turnId dispatchDurationMs outcome editClassifierVersion interactionId containsBuiltInFileEditRequest"},
	{"assistant.message", "messageId originatingMessageId model content toolRequests interactionId turnId rte apiCallId"},
	{"tool.execution_start", "toolCallId toolName arguments turnId model toolTitle mcpServerName mcpConfigServerName mcpTransport mcpConfigSource mcpToolName"},
	{"model.call_final_result", "model isByok result"},
	{"tool.execution_complete", "toolCallId model interactionId turnId rte success result toolTelemetry"},
	{"assistant.turn_end", "turnId"},
	{"assistant.turn_start", "turnId interactionId"},
	{"model.call_start", "turnId model"},
	{"assistant.message_start", "messageId"},
	{"assistant.message_delta", "messageId deltaContent"},
	{"model.call_finished", "turnId dispatchDurationMs outcome editClassifierVersion interactionId containsBuiltInFileEditRequest"},
	{"assistant.message", "messageId originatingMessageId model content toolRequests interactionId turnId rte apiCallId"},
	{"model.call_final_result", "model isByok result"},
	{"assistant.turn_end", "turnId"},
	{"assistant.idle", ""},
}

type copilotReadinessStream struct {
	rows    []map[string]any
	data    []map[string]any
	session string
}

func copilotReadinessObject(value any, keys string) (map[string]any, error) {
	row, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, keys, "") != nil {
		return nil, fmt.Errorf("Copilot readiness closed object")
	}
	return row, nil
}
func copilotReadinessNatural(value any, limit int64) bool {
	n, ok := value.(json.Number)
	if !ok {
		return false
	}
	i, err := n.Int64()
	return err == nil && i >= 0 && i <= limit
}
func copilotReadinessID(value any) bool {
	s, ok := value.(string)
	return ok && copilotReadinessUUID.MatchString(s)
}
func copilotReadinessDecode(body []byte, start, end time.Time) (*copilotReadinessStream, error) {
	return copilotReadinessDecodeOrder(body, start, end, false)
}

func copilotReadinessDecodeOrder(body []byte, start, end time.Time, interleaved bool) (*copilotReadinessStream, error) {
	if len(body) > copilotProjectBodyLimit || !bytes.HasSuffix(body, []byte{'\n'}) || start.IsZero() || end.Before(start) || end.Sub(start) > 60*time.Second {
		return nil, fmt.Errorf("Copilot readiness stream/command interval")
	}
	lines := bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'})
	if (!interleaved && len(lines) != 27) || (interleaved && len(lines) < 27) {
		return nil, fmt.Errorf("Copilot readiness finite event count")
	}
	header := len(lines) - 20
	s := &copilotReadinessStream{}
	seen := map[any]bool{}
	last := start
	var startup [7]int
	for i, line := range lines {
		v, err := projectNativeRuntimeJSON(line)
		grammarIndex := i
		if interleaved {
			grammarIndex = i - header + 7
		}
		if interleaved && i < header {
			var kindErr error
			grammarIndex, kindErr = copilotCycleStartupIndex(v)
			if kindErr != nil {
				return nil, kindErr
			}
		} else if interleaved && i == header && len(s.rows) != 7 {
			return nil, fmt.Errorf("Copilot cycle incomplete startup observations")
		}
		keys := "type data id timestamp parentId"
		ephemeral := grammarIndex < 7 || grammarIndex == 9 || grammarIndex == 10 || grammarIndex == 11 || grammarIndex == 14 || (grammarIndex >= 18 && grammarIndex <= 21) || grammarIndex == 23 || grammarIndex == 25
		if ephemeral {
			keys += " ephemeral"
		}
		if grammarIndex == 26 {
			keys = "type timestamp sessionId exitCode usage"
		}
		row, shapeErr := copilotReadinessObject(v, keys)
		if err != nil || shapeErr != nil {
			return nil, fmt.Errorf("Copilot readiness event shape")
		}
		stamp, ok := row["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if !ok || err != nil || when.Before(last) || when.After(end) {
			return nil, fmt.Errorf("Copilot readiness event time")
		}
		last = when
		if grammarIndex == 26 {
			if row["type"] != "result" || !copilotReadinessID(row["sessionId"]) || seen[row["sessionId"]] || row["exitCode"] != json.Number("0") {
				return nil, fmt.Errorf("Copilot readiness terminal session/result")
			}
			s.session = row["sessionId"].(string)
			s.rows = append(s.rows, row)
			continue
		}
		if row["type"] != copilotReadinessGrammar[grammarIndex][0] || !copilotReadinessID(row["id"]) || seen[row["id"]] || !copilotReadinessID(row["parentId"]) || ephemeral && row["ephemeral"] != true {
			return nil, fmt.Errorf("Copilot readiness event identity/order")
		}
		seen[row["id"]] = true
		data, err := copilotReadinessObject(row["data"], copilotReadinessGrammar[grammarIndex][1])
		if err != nil {
			return nil, err
		}
		if interleaved && i < header {
			keep, err := s.retainCycleStartup(row, data, grammarIndex, &startup)
			if err != nil {
				return nil, err
			}
			if !keep {
				continue
			}
		}
		s.rows = append(s.rows, row)
		s.data = append(s.data, data)
	}
	if interleaved {
		if err := s.cycleStartup(startup); err != nil {
			return nil, err
		}
	}
	if err := s.parents(seen); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *copilotReadinessStream) parents(ids map[any]bool) error {
	external := map[any]bool{}
	for _, i := range []int{0, 8, 11, 21} {
		parent := s.rows[i]["parentId"]
		if ids[parent] || external[parent] || parent == s.session {
			return fmt.Errorf("Copilot readiness external parent identity")
		}
		external[parent] = true
	}
	for i := 1; i < 8; i++ {
		if s.rows[i]["parentId"] != s.rows[0]["parentId"] {
			return fmt.Errorf("Copilot readiness startup parent")
		}
	}
	for _, edge := range [][2]int{{9, 8}, {10, 8}, {13, 12}, {14, 13}, {15, 13}, {16, 15}, {17, 16}, {18, 17}, {19, 17}, {20, 17}, {23, 22}, {24, 22}, {25, 24}} {
		if s.rows[edge[0]]["parentId"] != s.rows[edge[1]]["id"] {
			return fmt.Errorf("Copilot readiness exported parent edge")
		}
	}
	if s.rows[12]["parentId"] != s.rows[11]["parentId"] || s.rows[22]["parentId"] != s.rows[21]["parentId"] {
		return fmt.Errorf("Copilot readiness shared external parent")
	}
	return nil
}
