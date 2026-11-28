package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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
	for _, shape := range []string{"single", "group"} {
		for _, change := range []string{"rules", "denial", "unreadable", "embedded-host"} {
			t.Run(shape+"/"+change, func(t *testing.T) {
				f := newProjectFilesFixture(t, "cline", "default")
				paths := []string{f.skill, f.config}
				if shape == "group" {
					codexUser := filepath.Join(f.base, "user", ".codex")
					seedCodexProjectTrust(t, codexUser, f.project)
					f.req.Targets = []ClientTarget{{ClientID: "cline", ClientConfigRoot: f.user}, {ClientID: "codex", ClientConfigRoot: codexUser}}
					paths = append(paths, filepath.Join(f.project, ".agents", "skills", "docs", "SKILL.md"), filepath.Join(f.project, ".codex", "config.toml"))
				}
				gate, mutate, expected := sdkCallbackPolicy(t, change)
				f.cfg.InstallationPolicy = gate
				handoffs, callbacks := 0, 0
				f.cfg.Progress = func(event ProgressEvent) {
					if event.Phase == ProgressStage {
						callbacks++
						mutate()
					}
				}
				f.cfg.OnCommittedBinding = func(context.Context, BindingFacts) error { handoffs++; return nil }
				f.reopen(t)
				result, err := f.execute(t, OpInstall)
				reason := "policy_denied"
				if change == "rules" {
					reason = "policy_changed"
				}
				if !errors.Is(err, expected) || result.Outcome != OutcomeConflict || result.Reason != reason || result.Mutated || handoffs != 0 || callbacks != 1 || f.state(t) != "" {
					t.Fatalf("late SDK policy failure lost its cause or crossed stage: result=%+v err=%v handoffs=%d callbacks=%d", result, err, handoffs, callbacks)
				}
				if change == "unreadable" {
					var config *enterprisepolicy.ConfigError
					if !errors.As(err, &config) || config.Code != "required_file_unavailable" {
						t.Fatalf("late file-policy failure lost its typed adapter cause: %v", err)
					}
				}
				for _, path := range paths {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("late policy failure installed client content at %s: %v", path, err)
					}
				}
			})
		}
	}
}

func sdkCallbackPolicy(t *testing.T, change string) (ports.InstallationPolicy, func(), error) {
	t.Helper()
	if change == "unreadable" {
		path := filepath.Join(t.TempDir(), "policy.yaml")
		deathWrite(t, path, "schema_version: 2\n")
		gate := enterprisepolicy.Files{Paths: []enterprisepolicy.Path{{Layer: "deployment", Path: path, Required: true}}}
		return gate, func() {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}, os.ErrNotExist
	}
	fingerprint, gateErr := "reviewed", error(nil)
	gate := installerPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return fingerprint, gateErr })
	if change == "rules" {
		return gate, func() { fingerprint = "changed-after-preflight" }, domain.ErrInstallPolicyChanged
	}
	cause := errors.New("embedded host policy authority unavailable")
	if change == "denial" {
		cause = installerPolicyDenial()
	}
	return gate, func() { gateErr = cause }, cause
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

func TestInstallationPolicySDKDoesNotDowngradeAcquiredGroupProvenance(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "default")
	f.req.AcquiredSnapshot = &domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirectory}
	f.req.Targets = []ClientTarget{
		{ClientID: "cline", ClientConfigRoot: f.user},
		{ClientID: "codex", ClientConfigRoot: filepath.Join(f.base, "user", ".codex")},
	}
	f.engine = f.engine.WithInstallationPolicy(installerPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) {
		t.Fatal("unsupported acquired group was reinterpreted as a direct local package")
		return "", nil
	}))
	handle, err := f.engine.Prepare(t.Context(), f.req)
	if !errors.Is(err, ErrInvalidRequest) || handle != nil || f.state(t) != "" {
		t.Fatalf("group acquisition provenance was silently ignored: handle=%v err=%v", handle, err)
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
