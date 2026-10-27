//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"reflect"
	"time"
)

func geminiCycleStream(raw []byte, q *geminiCyclePhase, home string, start, end time.Time) (string, error) {
	if len(raw) == 0 || len(raw) > 1<<20 || raw[len(raw)-1] != '\n' {
		return "", fmt.Errorf("Gemini cycle whole native JSONL")
	}
	stage := 0
	session, assistant := "", ""
	last := time.Time{}
	for _, line := range bytes.Split(raw[:len(raw)-1], []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		row, ok := value.(map[string]any)
		if err != nil || !ok {
			return "", fmt.Errorf("Gemini cycle native event object")
		}
		stamp, ok := row["timestamp"].(string)
		when, e := time.Parse(time.RFC3339Nano, stamp)
		if !ok || e != nil || when.Before(last) || when.Before(start) || when.After(end) {
			return "", fmt.Errorf("Gemini cycle native event clock")
		}
		last = when
		switch row["type"] {
		case "init":
			if stage != 0 || projectNativeRuntimeKeys(row, "type timestamp session_id model", "") != nil || !projectNativeRuntimeText(row["session_id"], 256) || row["session_id"] == "" || row["model"] != geminiProjectModel {
				return "", fmt.Errorf("Gemini cycle native init")
			}
			session = row["session_id"].(string)
			stage = 1
		case "message":
			if projectNativeRuntimeKeys(row, "type timestamp role content", "delta") != nil || !projectNativeRuntimeText(row["content"], 1<<20) {
				return "", fmt.Errorf("Gemini cycle native message fields")
			}
			if v, exists := row["delta"]; exists {
				if _, ok := v.(bool); !ok {
					return "", fmt.Errorf("Gemini cycle native delta")
				}
			}
			switch row["role"] {
			case "user":
				if stage != 1 || row["content"] != q.prompt {
					return "", fmt.Errorf("Gemini cycle native exact prompt")
				}
				stage = 2
			case "assistant":
				if stage != 4 && stage != 5 && (stage != 2 || q.operation != "none") {
					return "", fmt.Errorf("Gemini cycle native assistant order")
				}
				assistant += row["content"].(string)
				if len(assistant) > len(q.sentinel) {
					return "", fmt.Errorf("Gemini cycle native assistant bound")
				}
				stage = 5
			default:
				return "", fmt.Errorf("Gemini cycle native role")
			}
		case "tool_use":
			if stage != 2 || q.operation == "none" || projectNativeRuntimeKeys(row, "type timestamp tool_name tool_id parameters", "") != nil || row["tool_name"] != q.tool || row["tool_id"] != q.id || !reflect.DeepEqual(row["parameters"], q.arguments()) {
				return "", fmt.Errorf("Gemini cycle exact native call identity/action")
			}
			stage = 3
		case "tool_result":
			if stage != 3 || projectNativeRuntimeKeys(row, "type timestamp tool_id status output", "") != nil || row["tool_id"] != q.id || row["status"] != "success" || !projectNativeRuntimeText(row["output"], 1<<20) {
				return "", fmt.Errorf("Gemini cycle native result identity")
			}
			facts, e := geminiCycleFacts(row["output"].(string), q.r, home)
			if e != nil || !reflect.DeepEqual(facts, q.facts) {
				return "", fmt.Errorf("Gemini cycle native/provider fact join")
			}
			stage = 4
		case "result":
			if stage != 5 || assistant != q.sentinel || projectNativeRuntimeKeys(row, "type timestamp status", "stats") != nil || row["status"] != "success" {
				return "", fmt.Errorf("Gemini cycle native terminal")
			}
			if stats, exists := row["stats"]; exists {
				if err := geminiProjectStats(stats); err != nil {
					return "", err
				}
			}
			stage = 6
		default:
			return "", fmt.Errorf("Gemini cycle unknown native event")
		}
	}
	if stage != 6 {
		return "", fmt.Errorf("Gemini cycle incomplete native session")
	}
	return session, nil
}
