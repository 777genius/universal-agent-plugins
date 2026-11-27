package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type installerPolicyFunc func(context.Context, domain.InstallPolicyRequest) (string, error)

func (f installerPolicyFunc) Check(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	return f(ctx, request)
}

func installerPolicyDenial() error {
	return &domain.InstallPolicyError{Violations: []domain.InstallPolicyViolation{{Code: "components.denied", Subject: "skills", Message: "skills are restricted"}}}
}

func TestInstallationPolicyRejectsSDKBeforeOpenCodeHostPreparation(t *testing.T) {
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "package")
	deathWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"policy-host","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "policy"))
	deathWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.com/mcp"}}}`)
	probes := 0
	denied := installerPolicyDenial()
	engine := openCodeEngine(t, Config{
		StateRoot: filepath.Join(root, "state"), HelperExecutable: executable,
		InstallationPolicy: installerPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
			if request.Envelope == nil || len(request.Envelope.Skills) != 1 || !request.Envelope.MCP.Present || request.Targets[0] != domain.ClientOpenCode {
				t.Fatalf("SDK policy lacks loaded package facts: %+v", request)
			}
			return "", denied
		}),
		OpenCodeProbe: func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			probes++
			return clientdetect.ProbeEvidence{}, errors.New("probe must not run")
		},
	})
	req := Request{Operation: OpInstall, OnlinePreview: true, PackageRoot: pkg, ClientID: "opencode", ClientConfigRoot: filepath.Join(root, "client"), ClientExecutable: executable}
	handle, err := engine.Prepare(t.Context(), req)
	if !errors.Is(err, denied) || handle != nil || probes != 0 {
		t.Fatalf("denied SDK package prepared a host: handle=%v probes=%d err=%v", handle, probes, err)
	}
	for _, path := range []string{engine.cfg.StateFile, engine.cfg.ManagedRoot, req.ClientConfigRoot} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("denied SDK preparation wrote %s: %v", path, err)
		}
	}
}

func TestInstallationPolicyChangesInvalidatePreparedSDKOperations(t *testing.T) {
	for _, change := range []string{"rules", "denial", "unreadable"} {
		t.Run(change, func(t *testing.T) {
			f := newProjectFilesFixture(t, "cline", "default")
			fingerprint, gateErr := "reviewed", error(nil)
			f.engine = f.engine.WithInstallationPolicy(installerPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return fingerprint, gateErr }))
			handle, err := f.engine.Prepare(t.Context(), f.req)
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenCodeTestHandle(t, handle)
			var expected error
			switch change {
			case "rules":
				fingerprint, expected = "changed-but-still-allows", domain.ErrInstallPolicyChanged
			case "denial":
				gateErr = installerPolicyDenial()
				expected = gateErr
			case "unreadable":
				gateErr = errors.New("required policy cannot be read")
				expected = gateErr
			}
			result, err := f.engine.Apply(t.Context(), handle, Decision{Confirmed: true})
			if !errors.Is(err, expected) || result.Outcome != OutcomeConflict || result.Mutated || f.state(t) != "" {
				t.Fatalf("stale SDK review applied: result=%+v err=%v", result, err)
			}
			for _, path := range []string{f.skill, f.config, f.engine.cfg.ManagedRoot} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("stale SDK review wrote %s: %v", path, err)
				}
			}
		})
	}
}

func TestInstallationPolicySDKRechecksAfterProgressCallback(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "default")
	fingerprint, handoffs := "reviewed", 0
	f.cfg.InstallationPolicy = installerPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return fingerprint, nil })
	f.cfg.Progress = func(event ProgressEvent) {
		if event.Phase == ProgressStage {
			fingerprint = "changed-after-preflight"
		}
	}
	f.cfg.OnCommittedBinding = func(context.Context, BindingFacts) error { handoffs++; return nil }
	f.reopen(t)
	result, err := f.execute(t, OpInstall)
	if !errors.Is(err, domain.ErrInstallPolicyChanged) || result.Mutated || handoffs != 0 || f.state(t) != "" {
		t.Fatalf("late SDK policy change crossed stage: result=%+v err=%v handoffs=%d", result, err, handoffs)
	}
	if _, err := os.Stat(f.skill); !os.IsNotExist(err) {
		t.Fatalf("late policy change installed a skill: %v", err)
	}
}

func TestInstallationPolicySDKPinsAreCopiedFromRequestedFacts(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "default")
	commonPin, targetPin := true, false
	f.req.GitPinned = &commonPin
	f.req.Targets = []ClientTarget{{ClientID: f.req.ClientID, ClientConfigRoot: f.req.ClientConfigRoot, GitPinned: &targetPin}}
	checks := 0
	f.engine = f.engine.WithInstallationPolicy(installerPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
		checks++
		if request.GitPinned == nil || *request.GitPinned || request.ArchivePinned != nil {
			t.Fatalf("requested pin facts rewritten or inferred: %+v", request)
		}
		return "reviewed", nil
	}))
	handle, err := f.engine.Prepare(t.Context(), f.req)
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenCodeTestHandle(t, handle)
	commonPin, targetPin = false, true
	result, err := f.engine.Apply(t.Context(), handle, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted || checks < 2 {
		t.Fatalf("copied pin facts were not retained: result=%+v err=%v checks=%d", result, err, checks)
	}
}

func TestInstallationPolicySDKChecksEveryGroupEnvelope(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "default")
	f.req.Targets = []ClientTarget{
		{ClientID: "cline", ClientConfigRoot: f.user},
		{ClientID: "codex", ClientConfigRoot: filepath.Join(f.base, "user", ".codex")},
	}
	var seen []domain.ClientID
	denied := installerPolicyDenial()
	f.engine = f.engine.WithInstallationPolicy(installerPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
		seen = append(seen, request.Targets[0])
		if request.Envelope == nil {
			t.Fatal("group checked without an envelope")
		}
		if request.Targets[0] == domain.ClientCodex {
			return "", denied
		}
		return "reviewed", nil
	}))
	handle, err := f.engine.Prepare(t.Context(), f.req)
	if !errors.Is(err, denied) || handle != nil || !reflect.DeepEqual(seen, []domain.ClientID{domain.ClientCline, domain.ClientCodex}) || f.state(t) != "" {
		t.Fatalf("group policy admission incomplete: handle=%v err=%v seen=%v", handle, err, seen)
	}
}

func TestInstallationPolicySDKCopyKeepsRemovalAndInspectionAvailable(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "default")
	checks := 0
	invalid := errors.New("configured policy is invalid")
	restricted := f.engine.WithInstallationPolicy(installerPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) {
		checks++
		return "", invalid
	}))
	if _, err := restricted.Prepare(t.Context(), f.req); !errors.Is(err, invalid) {
		t.Fatalf("restricted engine admitted package: %v", err)
	}
	// The derived policy session cannot mutate the original engine's config.
	f.apply(t, OpInstall)
	f.engine = restricted
	before := f.state(t)
	for _, op := range []Operation{OpUpdate, OpRepair, OpRefreshProjection} {
		if result, err := f.execute(t, op); !errors.Is(err, invalid) || result.Mutated || f.state(t) != before {
			t.Fatalf("retained %s bypassed policy: result=%+v err=%v", op, result, err)
		}
	}
	checksBeforeRemoval := checks
	if _, err := f.engine.Inspect(t.Context()); err != nil {
		t.Fatal(err)
	}
	f.apply(t, OpRemove)
	if checks != checksBeforeRemoval {
		t.Fatalf("inspection/removal consulted invalid installation policy: before=%d after=%d", checksBeforeRemoval, checks)
	}
}
