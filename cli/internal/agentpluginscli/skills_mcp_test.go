package agentpluginscli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/mcpskills"
)

func TestMCPReadOnlyCLIHTTPJourney(t *testing.T) {
	document := "---\nname: review\ndescription: Check changes\n---\n# Review\n"
	digest := sha256.Sum256([]byte(document))
	const uri = "skill://review/SKILL.md"
	calls := []string{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Method string `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			w.WriteHeader(400)
			return
		}
		calls = append(calls, input.Method)
		skill := map[string]any{"uri": uri, "frontmatter": map[string]any{"name": "review", "description": "Check changes"}, "resources": []any{map[string]any{"uri": uri, "digest": "sha256:" + hex.EncodeToString(digest[:]), "size": len(document)}}}
		result := map[string]any{"resultType": "complete", "ttlMs": 0, "cacheScope": "private"}
		switch input.Method {
		case "server/discover":
			result["supportedVersions"] = []string{"2026-07-28"}
			result["capabilities"] = map[string]any{"resources": map[string]any{}, "extensions": map[string]any{"io.modelcontextprotocol/skills": map[string]any{}}}
		case "skills/list":
			result["skills"] = []any{skill}
		case "skills/get":
			result["skill"] = skill
		case "resources/read":
			result["contents"] = []any{map[string]any{"uri": input.Params.URI, "text": document}}
		default:
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	})
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	urlServer, _ := url.Parse(server.URL)
	client := server.Client()
	base := client.Transport
	client.Transport = cliRoundTripper(func(request *http.Request) (*http.Response, error) {
		copy := request.Clone(request.Context())
		copy.URL.Host = urlServer.Host
		copy.Host = urlServer.Host
		return base.RoundTrip(copy)
	})
	factory := func(endpoint string) (*mcpskills.Client, error) { return mcpskills.NewWithHTTPClient(endpoint, client) }
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "--endpoint", "https://example.com/mcp"}, `"read_only":true`},
		{[]string{"inspect", "--endpoint", "https://example.com/mcp", "--uri", uri}, `"integrity":"verified"`},
	} {
		opts := &options{format: "json", scope: "user"}
		command := newSkillsMCPCommand(opts, factory)
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(test.args)
		if err := command.ExecuteContext(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), test.want) || strings.Contains(output.String(), "# Review") {
			t.Fatalf("incorrect sanitized JSON for %v: %s", test.args, output.String())
		}
	}
	if got := strings.Join(calls, ","); got != "server/discover,skills/list,server/discover,skills/get,resources/read" {
		t.Fatalf("unexpected RPC sequence: %s", got)
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("PoC mutated HOME: %v, %v", entries, err)
	}
}

type cliRoundTripper func(*http.Request) (*http.Response, error)

func (f cliRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMCPReadOnlyRejectsMutationOptions(t *testing.T) {
	for _, opts := range []*options{
		{format: "json", scope: "project"},
		{format: "json", scope: "user", target: "codex"},
		{format: "json", scope: "user", dryRun: true},
		{format: "json", scope: "user", acceptSecurityRisk: true},
	} {
		called := false
		command := newSkillsMCPCommand(opts, func(string) (*mcpskills.Client, error) { called = true; return nil, nil })
		command.SetArgs([]string{"list", "--endpoint", "https://example.com/mcp"})
		if err := command.Execute(); err == nil || called {
			t.Fatalf("mutating option accepted: %+v", opts)
		}
	}
}
