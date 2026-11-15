package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/antigravity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Management recovery checks complete owned materialization, not IDE activation.
// SDK inputs do not carry the detector's canonical HOME provenance: even a known
// version and an existing config must retain the original file-only route.
func TestAntigravityPreparedBindingCoherenceRemainsFileOnly(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			profile, pkg := filepath.Join(root, "home", ".gemini", "config"), filepath.Join(root, "package")
			deathWrite(t, filepath.Join(profile, "foreign.txt"), "foreign control")
			deathWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"review-agy","version":"1.0.0"}`)
			deathWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "review"))
			executable := filepath.Join(root, "agy")
			if err = os.WriteFile(executable, []byte("inert; never executed"), 0700); err != nil {
				t.Fatal(err)
			}
			registry, err := clients.NewRegistry(antigravity.New())
			if err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			runner := &projectFilesNoRunner{}
			cfg := Config{StateRoot: filepath.Join(root, "state"), Registry: registry, TrustedLocalPackages: true,
				HelperExecutable: helper, EnableNativeObserver: true, Runner: runner}
			req := Request{Scope: scope, Operation: OpInstall, OperationID: "review-install", PackageRoot: pkg,
				ClientID: "antigravity", ClientConfigRoot: profile, ClientExecutable: executable, ClientVersion: "1.5.3",
				InstallationID: "00000000-0000-4000-8000-000000009402", RequiredComponents: []string{"skills"}}
			if scope == domain.ScopeProject {
				cfg.ProjectRoot = filepath.Join(root, "project")
				deathWrite(t, filepath.Join(cfg.ProjectRoot, "control.txt"), "project control")
				cfg.StateRoot = filepath.Join(cfg.ProjectRoot, ".uap")
				req.ProjectRoot = cfg.ProjectRoot
			}
			engine, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			install, err := engine.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal("install prepare", err)
			}
			result, err := engine.Apply(t.Context(), install, Decision{Confirmed: true})
			if closeErr := install.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || result.Outcome != OutcomeCompleted {
				t.Fatal("install", result, err)
			}
			config, err := json.Marshal(map[string]any{"plugins": map[string]any{filepath.Base(result.Binding.TargetPath): map[string]any{"enabled": true}}})
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(profile, "config.json"), config, 0600); err != nil {
				t.Fatal(err)
			}
			req.Operation, req.OperationID = OpRepair, "review-recovery-inspection"
			prepared, err := engine.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal("recovery prepare", err)
			}
			defer func() {
				if err := prepared.Close(); err != nil {
					t.Error(err)
				}
			}()
			before, err := engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range before.Installations[0].Clients {
				if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
					t.Fatal("fixture must retain Prepared materialization", binding.Activation, binding.Verification)
				}
			}
			projection, projectionErr := engine.VerifyPreparedProjection(t.Context(), prepared)
			coherent, observationErr := engine.VerifyPreparedBinding(t.Context(), prepared)
			after, err := engine.store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("inspection changed ownership", err)
			}
			t.Logf("scope=%s projection=%t projectionErr=%v coherent=%t observationErr=%v nativeCalls=%d nativeRootPresent=%t", scope, projection, projectionErr, coherent, observationErr, runner.calls, prepared.client.NativeConfigRoot != "")
			if !projection || projectionErr != nil {
				t.Fatal("materialization must first verify", projectionErr)
			}
			if !coherent || observationErr != nil {
				t.Fatal("intact Prepared ownership cannot finish management recovery", coherent, observationErr)
			}
			if runner.calls != 0 || prepared.client.NativeConfigRoot != "" {
				t.Fatal("SDK file-only ownership inferred native HOME authority", runner.calls)
			}
		})
	}
}
