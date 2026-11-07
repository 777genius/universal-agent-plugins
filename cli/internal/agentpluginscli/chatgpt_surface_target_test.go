package agentpluginscli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The public CLI flag is normalized once. Canonical state/Directory identifiers
// must still parse with chatgpt meaning the remote Web client.
func TestPublicChatGPTNamesStaySeparateFromCanonicalIDs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ input, want string }{
		{"codex", "codex"},
		{"chatgpt", "codex"},
		{"chatgptweb", "chatgpt"},
		{"codex,chatgptweb", "codex,chatgpt"},
		{"chatgpt,chatgptweb", "codex,chatgpt"},
		{"codex,chatgpt", "codex,codex"},
	} {
		var canonical, raw string
		flag := targetFlagValue{target: &canonical, input: &raw}
		if err := flag.Set(tc.input); err != nil || canonical != tc.want || raw != tc.input {
			t.Errorf("target %q => %q / %q, %v; want %q", tc.input, canonical, raw, err, tc.want)
		}
	}
	var normalized, input string
	flag := targetFlagValue{target: &normalized, input: &input}
	if err := flag.Set("codex,chatgpt"); err != nil {
		t.Fatal(err)
	}
	if _, err := parseTargetOption(normalized); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("two local names must not duplicate one native plugin: %v", err)
	}
	if err := flag.Set("chatgpt"); err != nil {
		t.Fatal(err)
	}
	if err := validateCommonOptions(&options{target: normalized, targetInput: input, scope: "project", format: "human"}); err == nil || !strings.Contains(err.Error(), "only --scope user") {
		t.Fatalf("ChatGPT Desktop project request was not refused: %v", err)
	}
	if err := flag.Set("chatgptweb"); err != nil {
		t.Fatal(err)
	}
	if got, err := parseTargetOption(normalized); err != nil || !reflect.DeepEqual(got, []domain.ClientID{domain.ClientChatGPT}) {
		t.Fatalf("remote Web mapped away from canonical ChatGPT: %v %v", got, err)
	}
}

func TestChatGPTSurfaceTargetParsingAndScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want []domain.ClientID
	}{
		{"chatgpt-desktop", []domain.ClientID{domain.ClientCodex}},
		{"chatgpt-web", []domain.ClientID{domain.ClientChatGPT}},
		{"CHATGPT-DESKTOP,CHATGPT-WEB", []domain.ClientID{domain.ClientCodex, domain.ClientChatGPT}},
		{"codex,chatgpt", []domain.ClientID{domain.ClientCodex, domain.ClientChatGPT}},
	} {
		got, err := parseTargetOption(tc.raw)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("parseTargetOption(%q) = %v, %v; want %v", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{"codex,chatgpt-desktop", "chatgpt,chatgpt-web"} {
		_, err := parseTargetOption(raw)
		if err == nil || !strings.Contains(err.Error(), "duplicate client") {
			t.Errorf("expected a duplicate backend refusal for %q; got %v", raw, err)
		}
	}
	for _, scope := range []string{"project", "user"} {
		for _, raw := range []string{" chatgpt ", "CHATGPT", "codex,chatgpt", " chatgpt-desktop ", "codex,CHATGPT-DESKTOP"} {
			var canonical, input string
			flag := targetFlagValue{target: &canonical, input: &input}
			if err := flag.Set(raw); err != nil {
				t.Fatal(err)
			}
			err := validateCommonOptions(&options{format: "human", scope: scope, target: canonical, targetInput: input})
			if scope == "project" && err == nil {
				t.Errorf("ChatGPT Desktop must refuse project scope before mutation, %q", raw)
			}
			if scope == "user" && raw == "chatgpt" && err != nil {
				t.Errorf("ChatGPT Desktop user scope must remain available: %v", err)
			}
		}
	}
}

func checkDesktopOnlyCodexBinding(t *testing.T, fixture cliFixture, receipts int) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("expected one canonical installation and binding, got %+v", state.Installations)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID != string(domain.ClientCodex) || binding.Materialization != domain.MaterializationMaterialized || len(binding.Receipts) != receipts {
			t.Fatalf("Desktop alias must retain exactly the Codex binding and %d receipts: %+v", receipts, state.Installations[0].Clients)
		}
	}
}

func TestChatGPTDesktopAliasUsesCodexLifecycleWithoutSecondBinding(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)})
	plugin := writeCLIPlugin(t)

	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt", "--format", "json"); err != nil {
		t.Fatalf("Desktop alias add: %v", err)
	}
	checkDesktopOnlyCodexBinding(t, fixture, 1)

	manifest := filepath.Join(plugin, "plugin.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest,
		[]byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "update", "demo", "--target", "chatgpt", "--format", "json"); err != nil {
		t.Fatalf("Desktop alias update: %v", err)
	}
	checkDesktopOnlyCodexBinding(t, fixture, 2)

	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "chatgpt", "--external-uninstalled", "--format", "json"); err != nil {
		t.Fatalf("Desktop alias remove: %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 ||
		len(state.Installations[0].Clients) != 0 ||
		!state.Installations[0].DataRetained {
		t.Fatalf("Desktop alias removal must retain data but no native client binding: %+v", state.Installations)
	}
}
