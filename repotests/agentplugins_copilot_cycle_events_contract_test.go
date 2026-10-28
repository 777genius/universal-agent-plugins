//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
)

// This is an inert projection test of known event families, not a native
// observation or qualification of the predicted zero-tool serialization.
func copilotCycleNoCallInert(t *testing.T) (*copilotProjectAttempt, *copilotProjectProvider, []byte) {
	t.Helper()
	f, p, raw := copilotReadinessActual(t)
	old, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]map[string]any, 0, len(copilotCycleNoCallIndices))
	for _, i := range copilotCycleNoCallIndices {
		rows = append(rows, old.rows[i])
	}
	for _, row := range rows {
		if data, ok := row["data"].(map[string]any); ok {
			if _, has := data["turnId"]; has {
				data["turnId"] = "0"
			}
		}
	}
	rows[7]["parentId"], rows[8]["parentId"] = rows[5]["id"], rows[5]["id"]
	value, err := projectNativeRuntimeJSON(p.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	request := value.(map[string]any)
	delete(request, "tools")
	body, _ := json.Marshal(request)
	p.requests = [][]byte{body}
	return f, p, copilotReadinessEncode(t, rows)
}

func TestCopilotCycleNoCallGraphRequiresWholeTextOnlyClosure(t *testing.T) {
	f, p, raw := copilotCycleNoCallInert(t)
	s, err := copilotCycleNoCallDecode(raw, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal("known-family inert no-call projection refused", err)
	}
	if err = s.noCallMessages(copilotReadinessPrompt, p, f.nativeStarted, f.nativeEnded); err != nil {
		t.Fatal(err)
	}
	if err = s.noCallHistory(p); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing", "extra", "tool-event", "unknown-field", "parent", "terminal-session", "result-failed", "time", "ephemeral"} {
		f, _, raw = copilotCycleNoCallInert(t)
		value, err := copilotCycleNoCallDecode(raw, f.nativeStarted, f.nativeEnded)
		if err != nil {
			t.Fatal(err)
		}
		rows := value.rows
		switch kind {
		case "missing":
			rows = rows[:14]
		case "extra":
			rows = append(rows, rows[13])
		case "tool-event":
			rows[7]["type"] = "tool.execution_start"
		case "unknown-field":
			rows[4]["foreign"] = true
		case "parent":
			rows[7]["parentId"] = rows[0]["id"]
		case "terminal-session":
			rows[14]["sessionId"] = rows[4]["id"]
		case "result-failed":
			rows[14]["exitCode"] = json.Number("1")
		case "time":
			rows[8]["timestamp"] = "2020-01-01T00:00:00Z"
		case "ephemeral":
			delete(rows[7], "ephemeral")
		}
		if _, err = copilotCycleNoCallDecode(copilotReadinessEncode(t, rows), f.nativeStarted, f.nativeEnded); err == nil {
			t.Fatal("invalid no-call graph accepted", kind)
		}
	}
}

func TestCopilotCycleSkillCatalogCannotDropForeignOrCrossScope(t *testing.T) {
	c := copilotCycleInert(t)
	copilotCycleStageInert(t, c, "b")
	f, _, raw := copilotReadinessActual(t)
	old, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal(err)
	}
	descriptions := map[string]any{}
	for _, value := range old.data[3]["skills"].([]any) {
		row := value.(map[string]any)
		descriptions[row["name"].(string)] = row["description"]
	}
	for _, kind := range []string{"complete", "omit-foreign", "unknown", "duplicate", "wrong-source", "foreign-invoked", "wrong-description"} {
		catalog := []any{}
		for name, row := range c.skills(c.f.other) {
			if row["source"] == "builtin" {
				row["description"] = descriptions[name]
			}
			if kind == "omit-foreign" && name == "uap-foreign-b" {
				continue
			}
			if name == "uap-foreign-user" {
				switch kind {
				case "wrong-source":
					row["source"] = "project"
				case "foreign-invoked":
					row["invoked"] = true
				case "wrong-description":
					row["description"] = "changed"
				}
			}
			catalog = append(catalog, row)
		}
		if kind == "unknown" {
			catalog = append(catalog, map[string]any{"name": "foreign"})
		}
		if kind == "duplicate" {
			catalog[len(catalog)-1] = catalog[0]
		}
		err = c.skillCatalog(catalog, c.f.other)
		if (err == nil) != (kind == "complete") {
			t.Fatal("whole skill authority widened", kind, err)
		}
	}
}

func TestCopilotCycleCachedStartupOnlyAdmitsSourceDefinedPartialOrder(t *testing.T) {
	for _, kind := range []string{"cached", "interleaved", "summary-before-ready", "duplicate-state", "failed-state"} {
		f, _, raw := copilotReadinessActual(t)
		s, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
		if err != nil {
			t.Fatal(err)
		}
		order := []int{0, 2, 1, 3, 4, 5, 6}
		if kind == "interleaved" {
			order = []int{3, 0, 4, 2, 1, 5, 6}
		}
		if kind == "summary-before-ready" {
			order = []int{0, 2, 5, 1, 3, 4, 6}
		}
		rows := append([]map[string]any{}, s.rows...)
		stamps := make([]any, 7)
		for i := range stamps {
			stamps[i] = s.rows[i]["timestamp"]
		}
		for i, original := range order {
			rows[i] = s.rows[original]
			rows[i]["timestamp"] = stamps[i]
		}
		if kind == "duplicate-state" {
			rows[2]["data"] = map[string]any{"serverName": "uapcopilota", "status": "pending"}
		}
		if kind == "failed-state" {
			rows[2]["data"].(map[string]any)["status"] = "failed"
		}
		body := copilotReadinessEncode(t, rows)
		_, err = copilotReadinessDecodeOrder(body, f.nativeStarted, f.nativeEnded, true)
		if (err == nil) != (kind == "cached" || kind == "interleaved") {
			t.Fatal("startup partial order widened or lost supported cache path", kind, err)
		}
		if _, err = copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded); err == nil {
			t.Fatal("legacy exact observed grammar changed", kind)
		}
	}
}
