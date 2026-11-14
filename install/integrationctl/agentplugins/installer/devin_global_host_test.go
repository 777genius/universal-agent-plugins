package installer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/antigravity"
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
	registry, err := clients.NewRegistry(windsurf.New(), antigravity.New())
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
	for _, name := range []string{"missing", "relative", "unclean", "legacy-alias", "missing-version", "unsupported-version", "project", "non-opted", "group"} {
		t.Run(name, func(t *testing.T) {
			f := newDevinGlobalHostFixture(t, name == "project")
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
				f.request.ClientID = "antigravity"
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
