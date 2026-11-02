package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

type openCodeSiblingFixture struct {
	engine                                 *Engine
	request                                Request
	runner                                 *capturingRunner
	evidence                               opencodehost.VersionEvidence
	probes, handoffs                       int
	root, source, openCodeRoot, executable string
}

func writeSiblingPackage(t *testing.T, root, name string, revision int) {
	t.Helper()
	mustWriteV2(t, filepath.Join(root, "plugin.json"), []byte(fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":%q,"version":"%d.0.0"}`, name, revision)))
	mustWriteV2(t, filepath.Join(root, "skills", "guide", "SKILL.md"), []byte(fmt.Sprintf("---\nname: guide\ndescription: Sibling host regression\n---\nRevision %d.\n", revision)))
}

// Real installer, owned state and projections; native listing/probe responses
// use the existing controlled fixture seams, not live-client qualification.
func newOpenCodeSiblingFixture(t *testing.T, related bool) *openCodeSiblingFixture {
	t.Helper()
	skipWindowsLauncherExecuteBit(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &openCodeSiblingFixture{root: root, source: filepath.Join(root, "package"), openCodeRoot: filepath.Join(root, "opencode"), executable: buildProbe(t),
		evidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture:original"}}
	claudeRoot := filepath.Join(root, "config", "claude")
	f.runner = &capturingRunner{inner: listingRunner{configRoot: claudeRoot}}
	registry, err := clients.NewRegistry(claude.New(), opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	f.engine, err = New(Config{StateRoot: filepath.Join(root, "state"), Registry: registry, TrustedLocalPackages: true, HelperExecutable: f.executable, Runner: f.runner,
		OpenCodeProbe: func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			f.probes++
			if target.Executable != f.executable {
				t.Fatal("probed an uncaptured sibling executable", target.Executable)
			}
			return clientdetect.ProbeEvidence{VersionEvidence: f.evidence}, nil
		}, OnCommittedBinding: func(context.Context, BindingFacts) error { f.handoffs++; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	writeSiblingPackage(t, f.source, "sibling-host", 1)
	const installationID = "00000000-0000-4000-8000-0000000000f1"
	f.request = Request{Operation: OpInstall, InstallationID: installationID, OperationID: "claude-install", ClientID: "claude", ClientConfigRoot: claudeRoot, ClientExecutable: f.executable, PackageRoot: f.source}
	apply := func(req Request) {
		t.Helper()
		prepared, err := f.engine.Prepare(testCtx(t), req)
		if err != nil {
			t.Fatal(err)
		}
		defer closeOpenCodeTestHandle(t, prepared)
		result, err := f.engine.Apply(testCtx(t), prepared, Decision{Confirmed: true})
		if err != nil || result.Outcome != OutcomeCompleted {
			t.Fatalf("install: %+v %v", result, err)
		}
	}
	apply(f.request)
	openCode := f.request
	openCode.ClientID, openCode.ClientConfigRoot, openCode.OperationID = "opencode", f.openCodeRoot, "opencode-install"
	if !related {
		openCode.InstallationID = "00000000-0000-4000-8000-0000000000f2"
		openCode.PackageRoot = filepath.Join(root, "unrelated-package")
		writeSiblingPackage(t, openCode.PackageRoot, "unrelated-host", 1)
	}
	apply(openCode)
	if related {
		state, err := f.engine.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		installation, _ := findInstall(state, installationID)
		binding, _, ok := findBinding(installation, domain.ClientOpenCode)
		if !ok {
			t.Fatal("missing installed sibling")
		}
		f.request.KnownTargets = []TargetFacts{{ClientID: "opencode", BindingID: binding.ClientBindingID, ConfigRoot: f.openCodeRoot, Executable: f.executable}}
	}
	writeSiblingPackage(t, f.source, "sibling-host", 2)
	f.request.Operation, f.request.OperationID = OpUpdate, "claude-update"
	f.probes, f.handoffs, f.runner.calls = 0, 0, nil
	return f
}

func TestUpdatePreparesExactOpenCodeSiblingHost(t *testing.T) {
	f := newOpenCodeSiblingFixture(t, true)
	prepared, err := f.engine.Prepare(testCtx(t), f.request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, prepared)
	if f.probes != 1 || len(prepared.openCodeSiblings) != 1 || prepared.openCodeHost != nil {
		t.Fatal("selected Claude update did not retain exactly its OpenCode sibling", f.probes)
	}
	sibling := prepared.openCodeSiblings[0]
	host, ok := sibling.OpenCodeHost.(*OpenCodePreparedHost)
	if !ok || host.Evidence() != f.evidence || sibling.ExecutablePath != f.executable || sibling.ConfigRoot != f.openCodeRoot {
		t.Fatal("sibling authority differs from captured facts", sibling)
	}
	result, err := f.engine.Apply(testCtx(t), prepared, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted || f.probes != 2 || f.handoffs != 1 {
		t.Fatalf("compatibility update: %+v probes=%d handoffs=%d err=%v", result, f.probes, f.handoffs, err)
	}
	if got, err := os.ReadFile(filepath.Join(f.openCodeRoot, "skills", "guide", "SKILL.md")); err != nil || string(got) != "---\nname: guide\ndescription: Sibling host regression\n---\nRevision 1.\n" {
		t.Fatal("compatibility check mutated the unselected sibling", string(got), err)
	}
}

func TestUpdateUnrelatedOpenCodeInstallationDoesNotProbe(t *testing.T) {
	f := newOpenCodeSiblingFixture(t, false)
	f.evidence.ProbeStatus = "unavailable"
	prepared, err := f.engine.Prepare(testCtx(t), f.request)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, prepared)
	result, err := f.engine.Apply(testCtx(t), prepared, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted || f.probes != 0 || len(prepared.openCodeSiblings) != 0 {
		t.Fatalf("unrelated host became an update requirement: %+v probes=%d err=%v", result, f.probes, err)
	}
}

func TestUpdateOpenCodeSiblingDriftRefusesBeforeEffects(t *testing.T) {
	for _, change := range []string{"version", "executable_identity", "root"} {
		t.Run(change, func(t *testing.T) {
			if change == "root" && runtime.GOOS == "windows" {
				t.Skip("directory symlink privilege is qualified on Unix")
			}
			f := newOpenCodeSiblingFixture(t, true)
			prepared, err := f.engine.Prepare(testCtx(t), f.request)
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, prepared)
			switch change {
			case "version":
				f.evidence.Version = "2.0.21"
			case "executable_identity":
				f.evidence.ExecutableIdentity = "fixture:replaced"
			case "root":
				if err := os.Rename(f.openCodeRoot, f.openCodeRoot+"-retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), f.openCodeRoot); err != nil {
					t.Fatal(err)
				}
			}
			before := v2EffectSnapshot(t, f.engine)
			f.runner.calls = nil
			probes := f.probes
			result, err := f.engine.Apply(testCtx(t), prepared, Decision{Confirmed: true})
			if !errors.Is(err, ErrPlanChanged) || result.Outcome != OutcomeConflict || result.Mutated || f.handoffs != 0 || len(f.runner.calls) != 0 || before != v2EffectSnapshot(t, f.engine) {
				t.Fatalf("sibling drift performed effects: %+v calls=%v err=%v", result, f.runner.calls, err)
			}
			if change == "root" && f.probes != probes {
				t.Fatal("changed root reached native revalidation")
			}
		})
	}
}

func TestUpdateOpenCodeSiblingRejectsStaleBindingBeforeProbe(t *testing.T) {
	f := newOpenCodeSiblingFixture(t, true)
	f.request.KnownTargets[0].BindingID = "stale-binding"
	before := v2EffectSnapshot(t, f.engine)
	prepared, err := f.engine.Prepare(testCtx(t), f.request)
	if prepared != nil {
		defer closeOpenCodeTestHandle(t, prepared)
	}
	if !errors.Is(err, ErrTargetFactsUnavailable) || prepared != nil || f.probes != 0 || before != v2EffectSnapshot(t, f.engine) {
		t.Fatal("stale sibling authority reached probing or effects", err, f.probes)
	}
}
