//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func copilotReadinessActual(t *testing.T) (*copilotProjectAttempt, *copilotProjectProvider, []byte) {
	t.Helper()
	directory, err := filepath.Abs("testdata/copilot-readiness-40553325878")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := projectNativeRuntimeRead(filepath.Join(directory, "manifest.json"), 64<<10)
	h := sha256.Sum256(manifest)
	if err != nil || hex.EncodeToString(h[:]) != "807b961c1f4280081cc4a9c2d2c28c4997147b1df0985678cf0ace91e938703a" {
		t.Fatal("actual Copilot capture manifest differs", err)
	}
	var record struct{ Files map[string]string }
	if err := json.Unmarshal(manifest, &record); err != nil || len(record.Files) != 21 {
		t.Fatal("actual Copilot closed fixture manifest", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 22 {
		t.Fatal("actual Copilot fixture member closure", err)
	}
	files := map[string][]byte{}
	for name, expected := range record.Files {
		body, err := projectNativeRuntimeRead(filepath.Join(directory, name), copilotProjectBodyLimit)
		h := sha256.Sum256(body)
		if err != nil || hex.EncodeToString(h[:]) != expected {
			t.Fatal("actual Copilot fixture bytes differ", name, err)
		}
		files[name] = body
	}
	const root = "/home/runner/work/_temp/project-runtime-copilot/tmp/TestAgentpluginsCopilotNativeProjectRuntimeAttempt1397621521/001"
	const nonce = "fb6cd26aaf0bd16f7f5e5dfb7be2efd7"
	const project = root + "/project"
	f := &copilotProjectAttempt{base: &nativeFixture{Root: root, Project: project, Home: root + "/home", Events: root + "/events.jsonl", Nonce: nonce},
		binding: domain.ClientBinding{TargetLocator: project + "/.uap/managed/clients/copilot/native-proof-ac613ed30810", NativeObjects: []domain.NativeObjectOwnership{
			{ObjectID: "copilot_project_mcp_server:uapcopilota", Kind: "copilot_project_mcp_server", LogicalName: "uapcopilota", Path: project + "/.github/mcp.json", ManagedDigest: "sha256:cf35360150119fe765a7b8e6bf24babb883f08ff0336dae5dcf7a9ac0597e833", ProtectionClass: "managed"},
			{ObjectID: "copilot_project_skill_directory:uapcopilota", Kind: "copilot_project_skill_directory", LogicalName: "uapcopilota", Path: project + "/.github/skills/uapcopilota", ManagedDigest: "sha256:fbd12fe6fa822edd96282152f8192a61624c1020f99f06800b6722950d72df0d", ProtectionClass: "managed"}}},
		data: project + "/.uap/plugin-data/native-proof-ac613ed30810", files: files}
	// This explicit inert replay interval encloses exported wire events. The old
	// capture did not export actual command instants and remains failed.
	f.nativeStarted, err = time.Parse(time.RFC3339Nano, "2026-10-27T18:31:21.7Z")
	if err != nil {
		t.Fatal(err)
	}
	f.nativeEnded = f.nativeStarted.Add(200 * time.Millisecond)
	p := newCopilotProjectProvider("uapcopilota-inspect_runtime", nonce, nil)
	p.requests = [][]byte{files["provider-request-1.json"], files["provider-request-2.json"]}
	p.returned = true
	return f, p, files["native-attempt.stdout.log"]
}
func copilotReadinessEncode(t *testing.T, rows []map[string]any) []byte {
	t.Helper()
	var result []byte
	for _, row := range rows {
		body, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, append(body, '\n')...)
	}
	return result
}
func TestCopilotReadinessActualCaptureJoinsAllDistinctAuthorities(t *testing.T) {
	f, p, body := copilotReadinessActual(t)
	if err := f.installedReadiness(); err != nil {
		t.Fatal(err)
	}
	session, err := f.readinessBytes(body, f.files["probe-events.jsonl"], p)
	if err != nil || session != "86dc5cc4-2f2d-47c3-8831-52eb0b5e7ad6" || f.facts.PID != 5913 || f.facts.Session != "5913-1793125881785659195" {
		t.Fatal("actual measured Copilot readiness replay refused", err)
	}
}
func TestCopilotReadinessRejectsMalformedOrIncompleteWireBytes(t *testing.T) {
	f, p, body := copilotReadinessActual(t)
	for _, bad := range [][]byte{nil, body[:len(body)-1], append(bytes.Clone(body), body...), append(bytes.Clone(body), '\n'), {0xff}, bytes.Replace(body, []byte(`"ephemeral":true`), []byte(`"ephemeral":NaN`), 1), append([]byte(`{"type":"private",`), body[1:]...)} {
		if _, err := f.readinessBytes(bad, f.files["probe-events.jsonl"], p); err == nil {
			t.Fatal("malformed native stream accepted")
		}
	}
	for _, interval := range [][2]time.Time{{{}, f.nativeEnded}, {f.nativeEnded, f.nativeStarted}, {f.nativeStarted, f.nativeStarted.Add(61 * time.Second)}} {
		if _, err := copilotReadinessDecode(body, interval[0], interval[1]); err == nil {
			t.Fatal("absent/reversed/widened native interval accepted")
		}
	}
}
func TestCopilotReadinessRejectsEveryMissingAndUnknownEventField(t *testing.T) {
	f, p, body := copilotReadinessActual(t)
	s, err := copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range s.rows {
		keys := make([]string, 0, len(row))
		for key := range row {
			keys = append(keys, key)
		}
		for _, key := range keys {
			value := row[key]
			delete(row, key)
			if _, err := f.readinessBytes(copilotReadinessEncode(t, s.rows), f.files["probe-events.jsonl"], p); err == nil {
				t.Fatal("missing event field admitted", key)
			}
			row[key] = value
		}
		row["unknown"] = "uap-copilot-private-canary"
		if _, err := f.readinessBytes(copilotReadinessEncode(t, s.rows), f.files["probe-events.jsonl"], p); err == nil {
			t.Fatal("unknown event field admitted")
		}
		delete(row, "unknown")
	}
}
func TestCopilotReadinessRefusesForeignGraphCallsCatalogsHistoryAndFacts(t *testing.T) {
	for _, fault := range []string{"order", "duplicate-id", "session-from-parent", "turn-type", "interaction", "parent-edge", "event-time", "unknown-data", "skill-source", "skill-path", "builtin-metadata", "server-source", "call-id", "call-args", "tool-extra", "result-copy", "telemetry-type", "telemetry-bytes", "code-change", "sentinel", "history-prompt", "history-parameters", "probe-root", "probe-time", "probe-duplicate", "installed-skill", "installed-entry", "ownership"} {
		t.Run(fault, func(t *testing.T) {
			f, p, body := copilotReadinessActual(t)
			s, err := copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded)
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "order":
				s.rows[9], s.rows[10] = s.rows[10], s.rows[9]
			case "duplicate-id":
				s.rows[10]["id"] = s.rows[9]["id"]
			case "session-from-parent":
				s.rows[26]["sessionId"] = s.rows[0]["parentId"]
			case "turn-type":
				s.data[8]["turnId"] = json.Number("0")
			case "interaction":
				s.data[17]["interactionId"] = s.data[7]["messageId"]
			case "parent-edge":
				s.rows[15]["parentId"] = s.rows[12]["id"]
			case "event-time":
				s.rows[25]["timestamp"] = f.nativeEnded.Add(time.Second).Format(time.RFC3339Nano)
			case "unknown-data":
				s.data[25]["private"] = "uap-copilot-private-canary"
			case "skill-source", "skill-path", "builtin-metadata":
				index, key, value := 0, "source", "builtin"
				switch fault {
				case "skill-path":
					key, value = "path", f.base.Home+"/SKILL.md"
				case "builtin-metadata":
					index, key, value = 1, "description", "foreign metadata"
				}
				s.data[3]["skills"].([]any)[index].(map[string]any)[key] = value
			case "server-source":
				s.data[5]["servers"].([]any)[5].(map[string]any)["source"] = "user"
			case "call-id":
				s.data[13]["toolCallId"] = "foreign"
			case "call-args":
				s.data[10]["inputDelta"] = `{"nonce":"foreign","operation":"write","marker":"foreign"}`
			case "tool-extra":
				s.data[22]["toolRequests"] = s.data[12]["toolRequests"]
			case "result-copy":
				s.data[15]["result"].(map[string]any)["detailedContent"] = "{}"
			case "telemetry-type", "telemetry-bytes":
				var value any = true
				if fault == "telemetry-bytes" {
					value = json.Number("1")
				}
				s.data[15]["toolTelemetry"].(map[string]any)["metrics"].(map[string]any)["mcp_result_content_bytes"] = value
			case "code-change":
				s.rows[26]["usage"].(map[string]any)["codeChanges"].(map[string]any)["linesAdded"] = json.Number("1")
			case "sentinel":
				s.data[22]["content"] = "foreign"
			case "history-prompt", "history-parameters":
				v, err := projectNativeRuntimeJSON(p.requests[1])
				if err != nil {
					t.Fatal(err)
				}
				row := v.(map[string]any)
				if fault == "history-prompt" {
					row["messages"].([]any)[1].(map[string]any)["content"] = copilotReadinessPrompt
				} else {
					row["temperature"] = json.Number("1")
				}
				p.requests[1], err = json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
			case "probe-root", "probe-time":
				lines := bytes.Split(bytes.TrimSuffix(f.files["probe-events.jsonl"], []byte{'\n'}), []byte{'\n'})
				v, err := projectNativeRuntimeJSON(lines[2])
				if err != nil {
					t.Fatal(err)
				}
				row := v.(map[string]any)
				if fault == "probe-root" {
					row["facts"].(map[string]any)["plugin_root"] = f.base.Home
				} else {
					row["timestamp"] = f.nativeEnded.Add(time.Second).Format(time.RFC3339Nano)
				}
				lines[2], err = json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
				f.files["probe-events.jsonl"] = append(bytes.Join(lines, []byte{'\n'}), '\n')
			case "probe-duplicate":
				f.files["probe-events.jsonl"] = append(f.files["probe-events.jsonl"], f.files["probe-events.jsonl"]...)
			case "installed-skill", "installed-entry", "ownership":
				switch fault {
				case "ownership":
					f.binding.NativeObjects[0].ManagedDigest = "sha256:foreign"
				case "installed-skill":
					f.files["uapcopilota-SKILL.md.installed"] = []byte("foreign")
				default:
					f.files["mcp.json.installed"] = []byte(`{"mcpServers":{}}`)
				}
				if f.installedReadiness() == nil {
					t.Fatal("foreign installed authority admitted")
				}
				return
			}
			if _, err := f.readinessBytes(copilotReadinessEncode(t, s.rows), f.files["probe-events.jsonl"], p); err == nil {
				t.Fatal("foreign readiness observation admitted")
			}
		})
	}
}
