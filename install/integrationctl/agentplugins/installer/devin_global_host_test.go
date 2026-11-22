package installer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type devinGlobalHostFixture struct {
	root, profile, legacy string
	engine                *installer.Engine
	request               installer.Request
	store                 statev2.Store
	runner                *devinGlobalHostRunner
}

type devinGlobalHostRunner struct{ calls int }

func (runner *devinGlobalHostRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected native command in Devin file lifecycle")
}

func newDevinGlobalHostFixture(t *testing.T, project bool) devinGlobalHostFixture {
	t.Helper()
	root := devinSDKTemp(t)
	f := devinGlobalHostFixture{root: root, profile: filepath.Join(root, "original", "devin"), legacy: filepath.Join(root, ".codeium", "windsurf"), runner: &devinGlobalHostRunner{}}
	for _, path := range []string{f.profile, f.legacy} {
		devinSDKWrite(t, filepath.Join(path, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.invalid","disabled":true}},"keep":true}`)
	}
	pkg := filepath.Join(root, "package")
	devinSDKRemotePackage(t, pkg)
	registry, err := clients.NewRegistry(windsurf.New(), cline.New())
	if err != nil {
		t.Fatal(err)
	}
	f.request = installer.Request{Scope: domain.ScopeUser, Operation: installer.OpInstall, OperationID: "global-host-install",
		InstallationID: "00000000-0000-4000-8000-000000002338", PackageRoot: pkg, ClientID: "windsurf",
		ClientConfigRoot: f.legacy, ClientNativeConfigRoot: f.profile, ClientExecutable: filepath.Join(root, "never-start", "devin"), ClientVersion: "3000.12.57"}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := installer.Config{StateRoot: filepath.Join(root, "state"), Registry: registry, Runner: f.runner, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper}
	if project {
		cfg.ProjectRoot = filepath.Join(root, "project")
		devinSDKWrite(t, filepath.Join(cfg.ProjectRoot, "keep.txt"), "original project")
		cfg.StateRoot = filepath.Join(cfg.ProjectRoot, ".uap")
		f.request.Scope, f.request.ProjectRoot = domain.ScopeProject, cfg.ProjectRoot
		f.request.ClientVersion = "3000.11.205"
	}
	f.store = statev2.Store{Path: filepath.Join(cfg.StateRoot, "state-v2.json"), ProjectRoot: cfg.ProjectRoot}
	f.engine, err = installer.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFacadeDevinGlobalOriginalRootAdmission(t *testing.T) {
	for _, name := range []string{"missing", "relative", "unclean", "legacy-alias", "missing-version", "unsupported-version", "non-opted", "group"} {
		t.Run(name, func(t *testing.T) {
			f := newDevinGlobalHostFixture(t, false)
			before := devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json"))
			legacy := devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json"))
			switch name {
			case "missing":
				f.request.ClientNativeConfigRoot = ""
			case "relative":
				f.request.ClientNativeConfigRoot = "relative/devin"
			case "unclean":
				f.request.ClientNativeConfigRoot += string(filepath.Separator) + "."
			case "legacy-alias":
				f.request.ClientNativeConfigRoot = f.legacy
			case "missing-version":
				f.request.ClientVersion = ""
			case "unsupported-version":
				f.request.ClientVersion = "3000.12.58"
			case "non-opted":
				f.request.ClientID = "cline"
			case "group":
				f.request.Targets = []installer.ClientTarget{{ClientID: "windsurf", ClientConfigRoot: f.legacy}}
			}
			prepared, err := f.engine.Prepare(t.Context(), f.request)
			if prepared != nil {
				if closeErr := prepared.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			}
			if err == nil {
				t.Fatal("unqualified native User authority was accepted")
			}
			if name == "non-opted" && (!errors.Is(err, installer.ErrInvalidRequest) || !strings.Contains(err.Error(), "ClientNativeConfigRoot requires an explicit absolute clean User root and an opted-in adapter")) {
				t.Fatal("non-opted native User authority did not fail at admission", err)
			}
			state, stateErr := f.store.Load()
			if stateErr != nil || len(state.Installations) != 0 || f.runner.calls != 0 ||
				before != devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")) || legacy != devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json")) {
				t.Fatal("refusal changed original profile or installation state", stateErr)
			}
			if _, statErr := os.Lstat(filepath.Join(f.profile, "skills", "docs")); !os.IsNotExist(statErr) {
				t.Fatal("refusal installed native Skill", statErr)
			}
		})
	}
}

func TestFacadeDevinProjectOriginalUserRootIsReadOnlyContext(t *testing.T) {
	for _, authority := range []string{"observed", "missing"} {
		t.Run(authority, func(t *testing.T) {
			f := newDevinGlobalHostFixture(t, true)
			if authority == "missing" {
				f.request.ClientNativeConfigRoot = ""
			}
			global := devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json"))
			legacy := devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json"))
			prepared, err := f.engine.Prepare(t.Context(), f.request)
			if err != nil {
				t.Fatal("Project must retain its separate write layout", err)
			}
			defer func() {
				if err := prepared.Close(); err != nil {
					t.Error(err)
				}
			}()
			if prepared.Plan().ConfigRoot != f.legacy {
				t.Fatal("original User root replaced Cascade authority")
			}
			// A caller mutation after review cannot replace the frozen context.
			f.request.ClientNativeConfigRoot = filepath.Join(f.root, "replacement", "devin")
			result, err := f.engine.Apply(t.Context(), prepared, installer.Decision{Confirmed: true})
			if err != nil || result.Client.Activation != string(domain.ActivationPrepared) || result.Client.Verification != string(domain.VerificationInstalled) || f.runner.calls != 0 {
				t.Fatal("mixed Project file delivery claimed native runtime", result, err)
			}
			state, err := f.store.Load()
			if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
				t.Fatal("Project binding missing", state, err)
			}
			for _, binding := range state.Installations[0].Clients {
				if binding.NativeProfileRoot != filepath.Join(f.request.ProjectRoot, ".devin") || binding.Scope != "project" {
					t.Fatal("Project delivery selected a User write target", binding)
				}
			}
			if global != devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")) || legacy != devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json")) {
				t.Fatal("Project changed original Global or Cascade configuration")
			}
			if _, err := os.Stat(filepath.Join(f.profile, "skills", "docs")); !os.IsNotExist(err) {
				t.Fatal("Project installed a User Skill", err)
			}
		})
	}
}

func TestFacadeProjectOriginalUserRootAdmissionRefusals(t *testing.T) {
	for _, mutation := range []string{"relative", "unclean", "non-opted", "group"} {
		t.Run(mutation, func(t *testing.T) {
			f := newDevinGlobalHostFixture(t, true)
			switch mutation {
			case "relative":
				f.request.ClientNativeConfigRoot = "relative/devin"
			case "unclean":
				f.request.ClientNativeConfigRoot += string(filepath.Separator) + "."
			case "non-opted":
				f.request.ClientID = "cline"
			case "group":
				f.request.Targets = []installer.ClientTarget{{ClientID: "windsurf", ClientConfigRoot: f.legacy}}
			}
			global, legacy := devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")), devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json"))
			prepared, err := f.engine.Prepare(t.Context(), f.request)
			if prepared != nil {
				_ = prepared.Close()
			}
			if !errors.Is(err, installer.ErrInvalidRequest) || f.runner.calls != 0 {
				t.Fatal("unqualified explicit Project context accepted", err, f.runner.calls)
			}
			state, loadErr := f.store.Load()
			if loadErr != nil || len(state.Installations) != 0 || global != devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")) || legacy != devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json")) {
				t.Fatal("Project admission refusal changed authority", loadErr)
			}
		})
	}
}

func TestFacadeDevinGlobalOriginalRootIsFrozenAndPrivate(t *testing.T) {
	f := newDevinGlobalHostFixture(t, false)
	request := f.request
	body, err := json.Marshal(request)
	if err != nil || strings.Contains(string(body), "ClientNativeConfigRoot") || strings.Contains(string(body), filepath.ToSlash(f.profile)) || strings.Contains(string(body), strings.ReplaceAll(f.profile, `\`, `\\`)) {
		t.Fatal("native User root leaked into Request JSON", err)
	}
	prepared, err := f.engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := prepared.Close(); err != nil {
			t.Error(err)
		}
	})
	if prepared.Plan().ConfigRoot != f.legacy {
		t.Fatal("native root replaced independent legacy host authority")
	}
	request.ClientNativeConfigRoot = filepath.Join(f.root, "replacement", "devin")
	devinSDKWrite(t, filepath.Join(request.ClientNativeConfigRoot, "mcp_config.json"), `{"mcpServers":{},"replacement":"keep"}`)
	beforeReplacement := devinSDKRead(t, filepath.Join(request.ClientNativeConfigRoot, "mcp_config.json"))
	result, err := f.engine.Apply(t.Context(), prepared, installer.Decision{Confirmed: true})
	if err != nil || result.Client.Activation != string(domain.ActivationPrepared) || result.Client.Verification != string(domain.VerificationInstalled) || f.runner.calls != 0 {
		t.Fatal("prepared Global install failed or asserted native runtime", result, err)
	}
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatal("Global installed state missing", err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.NativeProfileRoot != f.profile {
			t.Fatal("Apply accepted caller root replacement")
		}
		if err := windsurf.New().ValidateBindingProfile(f.profile, binding); err != nil {
			t.Fatal(err)
		}
	}
	if beforeReplacement != devinSDKRead(t, filepath.Join(request.ClientNativeConfigRoot, "mcp_config.json")) {
		t.Fatal("Apply changed replacement profile")
	}
	devinGlobalRecordedRootRefusal(t, f, state)
}

// A current Devin user without Windsurf has no legacy Cascade root. The
// selected native Global root alone must be enough for User install and removal.
func TestFacadeDevinGlobalWithoutLegacyCascadeRoot(t *testing.T) {
	f := newDevinGlobalHostFixture(t, false)
	f.request.ClientConfigRoot = ""
	native := filepath.Join(f.profile, "mcp_config.json")
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpRemove} {
		f.request.Operation, f.request.OperationID = op, "global-no-cascade-"+string(op)
		prepared, err := f.engine.Prepare(t.Context(), f.request)
		if err != nil {
			t.Fatal(op, err)
		}
		_, err = f.engine.Apply(t.Context(), prepared, installer.Decision{Confirmed: true})
		if closeErr := prepared.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		if err != nil || f.runner.calls != 0 {
			t.Fatal(op, err, f.runner.calls)
		}
		_, skillErr := os.Lstat(filepath.Join(f.profile, "skills", "docs", "SKILL.md"))
		if installed := op == installer.OpInstall; installed != (skillErr == nil) || installed != strings.Contains(devinSDKRead(t, native), `"docs"`) {
			t.Fatal(op, "unexpected native Global state", skillErr, devinSDKRead(t, native))
		}
	}
	if body := devinSDKRead(t, native); !strings.Contains(body, `"foreign"`) {
		t.Fatal("foreign Global entry was not preserved", body)
	}
}

func devinGlobalRecordedRootRefusal(t *testing.T, f devinGlobalHostFixture, state domain.StateFileV2) {
	t.Helper()
	for key, binding := range state.Installations[0].Clients {
		binding.NativeProfileRoot = filepath.Join(f.root, "recorded-other", "devin")
		state.Installations[0].Clients[key] = binding
	}
	if err := f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, native, legacy := devinSDKRead(t, f.store.Path), devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")), devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json"))
	f.request.Operation, f.request.OperationID = installer.OpRepair, "global-root-mismatch"
	prepared, err := f.engine.Prepare(t.Context(), f.request)
	if prepared != nil {
		if closeErr := prepared.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}
	if err == nil || !strings.Contains(err.Error(), "recorded Windsurf user root differs") {
		t.Fatal("recorded root mismatch did not refuse", err)
	}
	if before != devinSDKRead(t, f.store.Path) || native != devinSDKRead(t, filepath.Join(f.profile, "mcp_config.json")) || legacy != devinSDKRead(t, filepath.Join(f.legacy, "mcp_config.json")) || f.runner.calls != 0 {
		t.Fatal("recorded root refusal changed original authority")
	}
}
