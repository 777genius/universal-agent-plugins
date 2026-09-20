package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// updateGoldenEnv mirrors the agentplugins core helper. The CLI lives in a
// different module and cannot import that package's internal/ tree.
const updateGoldenEnv = "UPDATE_GOLDEN"

// volatileIdentifiers are freshly generated per run. Their shape is part of the
// contract, their value is not.
var volatileIdentifiers = []struct {
	pattern *regexp.Regexp
	token   string
}{
	{regexp.MustCompile(`op-[0-9a-f]{32}`), "op-<operation>"},
	{regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), "<uuid>"},
	{regexp.MustCompile(`demo-[0-9a-f]{12}`), "demo-<artifact>"},
}

// TestAddDryRunGolden freezes the public --format json contract of a dry run
// across the client shapes the refactor touches: a single native client, a
// grouped install, and the ChatGPT preparation path.
func TestAddDryRunGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		clients []domain.ClientID
		target  string
		plugin  func(*testing.T) string
	}{
		{name: "cursor_manifest_only", clients: []domain.ClientID{domain.ClientCursor}, target: "cursor", plugin: writeCLIPlugin},
		{name: "codex_and_cursor_remote_mcp", clients: []domain.ClientID{domain.ClientCodex, domain.ClientCursor}, target: "codex,cursor", plugin: writeCLIPluginWithMCP},
		{name: "chatgpt_official_app", clients: []domain.ClientID{domain.ClientChatGPT}, target: "chatgpt", plugin: writeCLIOfficialAppPlugin},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			detected := make([]domain.DetectedClient, 0, len(test.clients))
			for _, client := range test.clients {
				detected = append(detected, fixtureClient(t, client))
			}
			fixture := newCLIFixture(t, detected)
			source := test.plugin(t)
			stdout, _, err := fixture.execute(false, "add", source, "--target", test.target, "--dry-run", "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			replacements := []string{source, "<source>", fixture.root, "<root>"}
			for _, client := range detected {
				replacements = append(replacements, filepath.Dir(client.ConfigRoot), "<home-"+string(client.ClientID)+">")
			}
			assertCLIGolden(t, test.name, stdout, replacements...)
		})
	}
}

func writeCLIPluginWithMCP(t *testing.T) string {
	t.Helper()
	root := writeCLIPlugin(t)
	writeCLIMCP(t, root)
	return root
}

// assertCLIGolden compares indented CLI output against testdata/golden/<name>.json
// after replacing volatile roots. replacements are from/to pairs applied in
// order, so longer paths must be listed first.
func assertCLIGolden(t *testing.T, name, output string, replacements ...string) {
	t.Helper()
	if len(replacements)%2 != 0 {
		t.Fatal("replacements must be from/to pairs")
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(output), "", "  "); err != nil {
		t.Fatalf("%v; output was %s", err, output)
	}
	rendered := indented.String()
	for index := 0; index < len(replacements); index += 2 {
		rendered = strings.ReplaceAll(rendered, replacements[index], replacements[index+1])
	}
	for _, volatile := range volatileIdentifiers {
		rendered = volatile.pattern.ReplaceAllString(rendered, volatile.token)
	}
	if filepath.Separator != '/' {
		rendered = strings.ReplaceAll(rendered, `\\`, "/")
	}
	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	path := filepath.Join("testdata", "golden", name+".json")
	if os.Getenv(updateGoldenEnv) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; rerun with %s=1 to record the baseline", err, updateGoldenEnv)
	}
	if string(expected) != rendered {
		t.Fatalf("%s changed behavior.\n--- recorded ---\n%s\n--- current ---\n%s", path, expected, rendered)
	}
}
