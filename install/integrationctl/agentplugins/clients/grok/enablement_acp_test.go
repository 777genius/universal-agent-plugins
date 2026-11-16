package grok

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func enablementACPFrames(t *testing.T, enabled bool) string {
	t.Helper()
	root := "/project/.grok/plugins/physical"
	row := map[string]any{"id": "project/" + enablementDigest([]byte(root))[7:15] + "/plugin", "name": "plugin", "root": root,
		"scope": "project", "version": "1.0.0", "enabled": enabled, "trusted": false}
	listing, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"result": map[string]any{"plugins": []any{row}}}})
	if err != nil {
		t.Fatal(err)
	}
	return `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"_meta":{"agentVersion":"1.0.70","currentWorkingDirectory":"/project"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"_x.ai/mcp/servers_updated","params":{"mcpServers":[]}}` + "\n" + string(listing) + "\n"
}

func TestEnablementACPRequestsOnlySessionlessMetadata(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var requests bytes.Buffer
		state, err := exchangeEnablementInventory(&requests, bufio.NewReaderSize(strings.NewReader(enablementACPFrames(t, enabled)), grokACPFrameLimit+1), "random-nonexistent-id", "/project")
		if err != nil || !state.complete || len(state.plugins) != 1 || state.plugins[0].enabled != enabled {
			t.Fatalf("complete native inventory: %+v %v", state, err)
		}
		decoder := json.NewDecoder(&requests)
		var first, second map[string]any
		if err := decoder.Decode(&first); err != nil {
			t.Fatal(err)
		}
		if err := decoder.Decode(&second); err != nil {
			t.Fatal(err)
		}
		if first["method"] != "initialize" || second["method"] != "_x.ai/plugins/list" {
			t.Fatalf("unexpected ACP action: %v %v", first, second)
		}
		var unexpected any
		if err := decoder.Decode(&unexpected); err != io.EOF {
			t.Fatalf("extra ACP request: %v %v", unexpected, err)
		}
		if _, err := selectEnablementPlugin(state.plugins, "/project/.grok/plugins/physical", "plugin", "1.0.0"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnablementACPRejectsIncompleteUnknownAndAmbiguousResponses(t *testing.T) {
	valid := enablementACPFrames(t, false)
	lines := strings.SplitAfter(valid, "\n")
	cases := map[string]string{
		"wrong-version":              strings.ReplaceAll(valid, "1.0.70", "1.0.71"),
		"wrong-project":              strings.Replace(valid, `"currentWorkingDirectory":"/project"`, `"currentWorkingDirectory":"/foreign"`, 1),
		"duplicate-key":              strings.Replace(valid, `"id":1`, `"id":1,"id":1`, 1),
		"foreign-notification":       strings.ReplaceAll(valid, "_x.ai/mcp/servers_updated", "session/update"),
		"nonempty-mcp":               strings.Replace(valid, `"mcpServers":[]`, `"mcpServers":[{}]`, 1),
		"duplicate-notification":     lines[0] + lines[1] + lines[1] + lines[2],
		"response-before-initialize": lines[2] + lines[0],
		"partial-response":           strings.TrimSuffix(valid, "\n"),
		"duplicate-initialize":       lines[0] + lines[0] + lines[2],
		"wrong-full-id":              strings.Replace(valid, `"id":"project/`, `"id":"user/`, 1),
		"missing-response":           lines[0],
		"oversized-frame":            strings.Repeat("x", grokACPFrameLimit+1) + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := exchangeEnablementInventory(&bytes.Buffer{}, bufio.NewReaderSize(strings.NewReader(body), grokACPFrameLimit+1), "fresh-id", "/project"); err == nil {
				t.Fatal("unrecognized or incomplete native stream accepted")
			}
		})
	}
}

func TestEnablementACPRequiresExactUnambiguousProjectIdentity(t *testing.T) {
	var requests bytes.Buffer
	state, err := exchangeEnablementInventory(&requests, bufio.NewReaderSize(strings.NewReader(enablementACPFrames(t, false)), grokACPFrameLimit+1), "fresh-id", "/project")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*grokACPPlugin){
		func(row *grokACPPlugin) { row.root += "-foreign" },
		func(row *grokACPPlugin) { row.name = "Plugin" },
		func(row *grokACPPlugin) { row.version = "2.0.0" },
		func(row *grokACPPlugin) { row.scope = "user" },
		func(row *grokACPPlugin) { row.conflict = true },
	} {
		row := state.plugins[0]
		mutate(&row)
		if _, err := selectEnablementPlugin([]grokACPPlugin{row}, "/project/.grok/plugins/physical", "plugin", "1.0.0"); err == nil {
			t.Fatalf("foreign plugin accepted: %+v", row)
		}
	}
	row := state.plugins[0]
	object := map[string]any{"id": row.id, "root": row.root, "name": row.name, "scope": row.scope, "version": row.version, "enabled": false, "trusted": false}
	if _, err := parseEnablementPlugins([]any{object, object}); err == nil {
		t.Fatal("duplicate native identity accepted")
	}
}
