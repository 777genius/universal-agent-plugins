package installer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Disabling optional native discovery must not turn a proven no-effect refusal
// into an attempt with no recoverable intent. Preserve actual user bytes and
// ownership, then allow the same removal once the injected edit is resolved.
func TestUserGeminiRemovalForeignFilesRefuseWithoutOrphanedAttempt(t *testing.T) {
	for _, afterPrepare := range []bool{false, true} {
		name := "before-prepare"
		if afterPrepare {
			name = "after-prepare"
		}
		t.Run(name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			pkg, profile := filepath.Join(base, "package"), filepath.Join(base, "profile space é")
			deathWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"gemini-remove","version":"1.0.0"}`)
			deathWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "original removal payload"))
			deathWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://original.test/mcp"}}}`)
			settings := filepath.Join(profile, "settings.json")
			deathWrite(t, settings, `{"mcpServers":{"foreign":{"httpUrl":"https://foreign.test/keep"}},"sentinel":"keep preference"}`)
			registry, err := clients.NewRegistry(gemini.New())
			if err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			engine, err := New(Config{StateRoot: filepath.Join(base, "state"), Registry: registry, Runner: &projectFilesNoRunner{}, TrustedLocalPackages: true, HelperExecutable: helper})
			if err != nil {
				t.Fatal(err)
			}
			req := Request{Operation: OpInstall, OperationID: "gemini-remove-install", PackageRoot: pkg, ClientID: "gemini", Scope: domain.ScopeUser, ClientConfigRoot: profile, InstallationID: "00000000-0000-4000-8000-000000000083"}
			apply := func(request Request) (Result, error) {
				t.Helper()
				handle, err := engine.Prepare(t.Context(), request)
				if err != nil {
					return Result{}, err
				}
				defer func() {
					if err := handle.Close(); err != nil {
						t.Error(err)
					}
				}()
				return engine.Apply(t.Context(), handle, Decision{Confirmed: true})
			}
			installed, err := apply(req)
			if err != nil || installed.Outcome != OutcomeCompleted {
				t.Fatalf("install: %+v %v", installed, err)
			}
			view, err := engine.Inspect(t.Context())
			if err != nil || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
				t.Fatalf("installed inspection: %+v %v", view, err)
			}
			projection := view.Installations[0].Bindings[0].TargetPath
			original := deathRead(t, settings)
			foreign := strings.ReplaceAll(original, "https://original.test/mcp", "https://user-edited.test/preserve")
			if foreign == original {
				t.Fatal("expected native server was not installed")
			}
			req.Operation, req.OperationID, req.InstallationID = OpRemove, "gemini-remove-edited", installed.InstallationID
			var handle *PreparedOperation
			if afterPrepare {
				handle, err = engine.Prepare(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := handle.Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			deathWrite(t, settings, foreign)
			before, stateBefore := deathTree(t, projection), deathRead(t, engine.cfg.StateFile)
			var result Result
			if afterPrepare {
				result, err = engine.Apply(t.Context(), handle, Decision{Confirmed: true})
			} else {
				result, err = apply(req)
			}
			if err == nil {
				t.Fatalf("edited native ownership accepted: %+v", result)
			}
			if deathRead(t, settings) != foreign || !reflect.DeepEqual(before, deathTree(t, projection)) {
				t.Fatal("refused removal overwrote user-native or managed payload bytes")
			}
			view, inspectErr := engine.Inspect(t.Context())
			if inspectErr != nil || view.Recovery.Required {
				t.Fatalf("known foreign native file created unrecoverable attempt: %+v %v", view.Recovery, inspectErr)
			}
			if deathRead(t, engine.cfg.StateFile) != stateBefore {
				t.Fatal("foreign native refusal changed durable ownership state")
			}
			deathWrite(t, settings, original)
			req.OperationID = "gemini-remove-resolved"
			removed, err := apply(req)
			if err != nil || removed.Outcome != OutcomeCompleted {
				t.Fatalf("same removal after resolving user edit: %+v %v", removed, err)
			}
			if _, err := os.Lstat(projection); !os.IsNotExist(err) {
				t.Fatalf("resolved removal retained projection: %v", err)
			}
			if got := deathRead(t, settings); strings.Contains(got, "https://original.test/mcp") || !strings.Contains(got, "https://foreign.test/keep") || !strings.Contains(got, "keep preference") {
				t.Fatalf("resolved removal damaged native siblings: %s", got)
			}
		})
	}
}
