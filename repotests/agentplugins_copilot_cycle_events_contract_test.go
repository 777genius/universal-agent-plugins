//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"
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

// Model the literal observed repaired-A header (tools,pending,tools,connected,
// skills,info,servers,updated) over the held inert tail, without qualifying it.
func copilotCycleRefreshRows(t *testing.T, repeats int) (*copilotProjectAttempt, []map[string]any) {
	t.Helper()
	f, _, raw := copilotReadinessActual(t)
	s, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal(err)
	}
	order := []int{2, 0}
	for i := 1; i < repeats; i++ {
		order = append(order, 2)
	}
	order = append(order, 1, 3, 4, 5, 6)
	rows := []map[string]any{}
	for i, original := range order {
		body, err := json.Marshal(s.rows[original])
		if err != nil {
			t.Fatal(err)
		}
		value, err := projectNativeRuntimeJSON(body)
		if err != nil {
			t.Fatal(err)
		}
		row := value.(map[string]any)
		row["id"] = fmt.Sprintf("00000001-0000-4000-8000-%012d", i)
		row["timestamp"] = s.rows[0]["timestamp"]
		rows = append(rows, row)
	}
	return f, append(rows, s.rows[7:]...)
}

func TestCopilotCycleRepeatedToolsLoadedPreservesStrictLegacyAndRawIdentities(t *testing.T) {
	for _, repeats := range []int{1, 2, 8} {
		f, rows := copilotCycleRefreshRows(t, repeats)
		body := copilotReadinessEncode(t, rows)
		s, err := copilotReadinessDecodeOrder(body, f.nativeStarted, f.nativeEnded, true)
		if err != nil || len(s.rows) != 27 || len(s.data) != 26 || s.data[2]["toolCount"] != json.Number("1") {
			t.Fatal("same owned tools refresh lost its strict semantic graph", repeats, err)
		}
		if len(rows) != 26+repeats || !bytes.Equal(body, copilotReadinessEncode(t, rows)) {
			t.Fatal("raw refresh rows were rewritten")
		}
		if _, err := copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded); err == nil {
			t.Fatal("cycle refresh widened historical strict grammar")
		}
	}
}

func TestCopilotCycleToolsRefreshRejectsEveryChangedAuthority(t *testing.T) {
	for _, fault := range []string{"server", "count", "field", "ephemeral", "duplicate-id", "hidden-parent", "terminal-id", "late", "status", "missing", "unknown", "time", "oversize"} {
		t.Run(fault, func(t *testing.T) {
			f, rows := copilotCycleRefreshRows(t, 2)
			switch fault {
			case "server":
				rows[2]["data"].(map[string]any)["serverName"] = "foreign"
			case "count":
				rows[2]["data"].(map[string]any)["toolCount"] = json.Number("2")
			case "field":
				rows[2]["extra"] = true
			case "ephemeral":
				rows[2]["ephemeral"] = false
			case "duplicate-id":
				rows[2]["id"] = rows[0]["id"]
			case "hidden-parent":
				rows[9]["parentId"] = rows[2]["id"]
			case "terminal-id":
				rows[len(rows)-1]["sessionId"] = rows[2]["id"]
			case "late":
				rows[2], rows[7] = rows[7], rows[2]
			case "status":
				rows[2]["type"], rows[2]["data"] = rows[1]["type"], rows[1]["data"]
			case "missing":
				rows = append(rows[:4], rows[5:]...)
			case "unknown":
				rows[2]["type"] = "unknown.startup"
			case "time":
				rows[2]["timestamp"] = f.nativeEnded.Add(time.Second).Format(time.RFC3339Nano)
			}
			body := copilotReadinessEncode(t, rows)
			if fault == "oversize" {
				body = append(body, bytes.Repeat([]byte{' '}, copilotProjectBodyLimit)...)
			}
			if _, err := copilotReadinessDecodeOrder(body, f.nativeStarted, f.nativeEnded, true); err == nil {
				t.Fatal("changed startup refresh authority accepted")
			}
		})
	}
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

func TestCopilotCycleUserPluginCommandNameIsExactlyNamespaced(t *testing.T) {
	c := copilotCycleInert(t)
	for _, key := range []string{"user", "a"} {
		copilotCycleStageInert(t, c, key)
	}
	user, project := c.recipes["user"], c.recipes["a"]
	// Literal catalog identities are independent of the expectation constructor.
	// Only the User plugin commandName has the measured plugin namespace.
	catalog := []any{
		map[string]any{"name": "uap-foreign-user", "commandName": "uap-foreign-user", "description": "Original user skill must remain unchanged", "source": "personal-copilot", "userInvocable": true, "enabled": true, "path": filepath.Join(c.f.base.Home, ".copilot", "skills", "uap-foreign-user", "SKILL.md")},
		map[string]any{"name": "uapcopilotuser", "commandName": "native-proof:uapcopilotuser", "description": "Disposable Copilot user user-one " + user.Nonce, "source": "plugin", "userInvocable": true, "enabled": true, "path": user.SkillPath},
		map[string]any{"name": "uapcopilota", "commandName": "uapcopilota", "description": "Disposable Copilot a a-one " + project.Nonce, "source": "project", "userInvocable": true, "enabled": true, "path": project.SkillPath},
	}
	f, _, raw := copilotReadinessActual(t)
	measured, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range measured.data[3]["skills"].([]any) {
		row := value.(map[string]any)
		if row["source"] == "builtin" {
			row["path"] = filepath.Join(c.f.base.Root, "xdg-cache", "copilot", "pkg", "linux-x64", "1.0.103", "builtin", row["name"].(string), "SKILL.md")
			catalog = append(catalog, row)
		}
	}
	if err := c.skillCatalog(catalog, c.f.base.Project); err != nil {
		t.Fatal("measured User spelling or unchanged other catalog identities refused", err)
	}
	for _, fault := range []struct {
		index int
		field string
		value string
	}{
		{1, "commandName", "uapcopilotuser"},
		{1, "commandName", "other:uapcopilotuser"},
		{1, "commandName", "native-proof:native-proof:uapcopilotuser"},
		{1, "name", "native-proof:uapcopilotuser"},
		{0, "commandName", "native-proof:uap-foreign-user"},
		{2, "commandName", "native-proof:uapcopilota"},
		{3, "commandName", "native-proof:" + catalog[3].(map[string]any)["name"].(string)},
	} {
		bad := append([]any{}, catalog...)
		entry := map[string]any{}
		for key, value := range catalog[fault.index].(map[string]any) {
			entry[key] = value
		}
		entry[fault.field], bad[fault.index] = fault.value, entry
		if c.skillCatalog(bad, c.f.base.Project) == nil {
			t.Fatal("User namespace correction widened a different exact identity", fault)
		}
	}
}

func TestCopilotCycleCachedStartupOnlyAdmitsSourceDefinedPartialOrder(t *testing.T) {
	for _, kind := range []string{"cached", "interleaved", "tools-before-pending", "connected-before-pending", "summary-before-ready", "duplicate-state", "failed-state"} {
		f, _, raw := copilotReadinessActual(t)
		s, err := copilotReadinessDecode(raw, f.nativeStarted, f.nativeEnded)
		if err != nil {
			t.Fatal(err)
		}
		order := []int{0, 2, 1, 3, 4, 5, 6}
		if kind == "interleaved" {
			order = []int{3, 0, 4, 2, 1, 5, 6}
		}
		if kind == "tools-before-pending" {
			order = []int{2, 0, 1, 3, 4, 5, 6}
		}
		if kind == "connected-before-pending" {
			order = []int{2, 1, 0, 3, 4, 5, 6}
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
		if (err == nil) != (kind == "cached" || kind == "interleaved" || kind == "tools-before-pending") {
			t.Fatal("startup partial order widened or lost supported cache path", kind, err)
		}
		if _, err = copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded); err == nil {
			t.Fatal("legacy exact observed grammar changed", kind)
		}
	}
}
