package grok

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestGrokUserRejectsCaseFoldedRegistryAliases(t *testing.T) {
	for _, names := range [][2]string{{"demo", "DEMO"}, {"K", "K"}} {
		body := []byte("[{\"name\":\"" + names[0] + "\",\"status\":\"installed\"},{\"name\":\"" + names[1] + "\",\"status\":\"installed\"}]")
		if entries, err := parseList(body); err == nil || errors.Is(err, errUnknownList) {
			t.Fatalf("native alias did not return known negative evidence: %+v %v", entries, err)
		}
	}

	request := grokRequest(t)
	runner := &fakeGrok{entries: []pluginEntry{{Name: "DEMO", Status: "installed",
		Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
	env := clients.Env{Runner: runner}
	for _, verify := range []bool{false, true} {
		request.VerifyOnly = verify
		runner.commands = nil
		outcome, err := New().Activate(context.Background(), env, request)
		if err == nil || outcome.Activation == domain.ActivationActive ||
			len(runner.commands) != 1 || runner.commands[0][2] != "list" {
			t.Fatalf("native alias installed or verified: %+v %v %+v", outcome, err, runner.commands)
		}
	}
	runner.commands = nil
	finding, err := New().InspectNativeRegistry(context.Background(), env, request.Client, request.Plan, nil)
	if err != nil || finding != clients.RegistryCollision || len(runner.commands) != 1 {
		t.Fatalf("native alias passed collision inspection: %v %v %+v", finding, err, runner.commands)
	}
	runner.commands = nil
	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
		BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath, Confirmed: true}
	if _, err := New().Deactivate(context.Background(), env, remove); err == nil || len(runner.commands) != 1 {
		t.Fatalf("native alias caused uninstall: %v %+v", err, runner.commands)
	}
	if err := verifyAbsent(context.Background(), env, request.BackendExecutable, request.DeclaredName); err == nil {
		t.Fatal("casefold alias was mistaken for absence")
	}
}

type grokFaultRunner struct {
	other *fakeGrok
	verb  string
	err   error
}

func (f *grokFaultRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) > 2 && command.Argv[2] == f.verb {
		f.other.commands = append(f.other.commands, append([]string(nil), command.Argv...))
		return legacyports.CommandResult{}, f.err
	}
	return f.other.Run(ctx, command)
}

func TestGrokUserFailedNativeMutationDoesNotIssueSecondCommand(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		name := "closed"
		if uncertain {
			name = "unclosed"
		}
		t.Run("install-"+name, func(t *testing.T) {
			request := grokRequest(t)
			base := &fakeGrok{entries: []pluginEntry{}}
			env := clients.Env{Runner: base}
			if uncertain {
				env.Runner = &grokFaultRunner{other: base, verb: "install", err: &domain.UnclosedClientProbe{
					Operation: "grok install", Err: errors.New("still running")}}
			} else {
				base.failInstall = true
			}
			outcome, err := New().Activate(context.Background(), env, request)
			if err == nil || outcome.Verification == domain.VerificationInstalled || len(base.commands) != 2 ||
				base.commands[0][2] != "list" || base.commands[1][2] != "install" {
				t.Fatalf("failed install re-probed native state: %+v %v %+v", outcome, err, base.commands)
			}
			if outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("failed native install may have partial side effects: %+v", outcome)
			}
		})
		t.Run("uninstall-"+name, func(t *testing.T) {
			request := grokRequest(t)
			base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
				Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
			env := clients.Env{Runner: base}
			if uncertain {
				env.Runner = &grokFaultRunner{other: base, verb: "uninstall", err: &domain.UnclosedClientProbe{
					Operation: "grok uninstall", Err: errors.New("still running")}}
			} else {
				base.failUninstall = true
			}
			remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
				BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath, Confirmed: true}
			outcome, err := New().Deactivate(context.Background(), env, remove)
			if err == nil || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
				len(base.commands) != 2 || base.commands[0][2] != "list" || base.commands[1][2] != "uninstall" {
				t.Fatalf("failed removal re-probed or authorized cleanup: %+v %v %+v", outcome, err, base.commands)
			}
			if outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("failed native unregister may have partial side effects: %+v", outcome)
			}
		})
	}
}

func TestGrokUserCannotAttestDuplicatedNativeRegistration(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	runner := &fakeGrok{entries: []pluginEntry{
		{Name: "demo", Status: "installed", Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion},
		{Name: "DEMO", Status: "installed", Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion},
	}}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
	if err == nil || outcome.ActivationAttested || outcome.Activation == domain.ActivationActive ||
		outcome.Verification == domain.VerificationInstalled || !outcome.AuthoritativeObservation ||
		len(runner.commands) != 1 || runner.commands[0][2] != "list" {
		t.Fatalf("duplicate native identities became attested activation: %+v %v %+v", outcome, err, runner.commands)
	}
}

type grokUnclosedRemovalReadback struct {
	base         *fakeGrok
	afterRemoval bool
}

func (f *grokUnclosedRemovalReadback) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) >= 3 && command.Argv[2] == "list" && f.afterRemoval {
		f.base.commands = append(f.base.commands, append([]string(nil), command.Argv...))
		return legacyports.CommandResult{}, &domain.UnclosedClientProbe{
			Operation: "grok post-removal list", Err: errors.New("cannot confirm native process closure")}
	}
	result, err := f.base.Run(ctx, command)
	if len(command.Argv) >= 3 && command.Argv[2] == "uninstall" && err == nil && result.ExitCode == 0 {
		f.afterRemoval = true
	}
	return result, err
}

func TestGrokUserUnclosedRemovalReadbackNeverAllowsArtifactDeletion(t *testing.T) {
	request := grokRequest(t)
	base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
		Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
	runner := &grokUnclosedRemovalReadback{base: base}
	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
		BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath, Confirmed: true}
	outcome, err := New().Deactivate(context.Background(), clients.Env{Runner: runner}, remove)
	if err == nil || !domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed || len(base.commands) != 3 ||
		base.commands[0][2] != "list" || base.commands[1][2] != "uninstall" || base.commands[2][2] != "list" {
		t.Fatalf("unclosed post-uninstall query enabled deletion: %+v %v %+v", outcome, err, base.commands)
	}
}

type grokMalformedReplacementReadback struct {
	base         *fakeGrok
	afterRemoval bool
}

func (f *grokMalformedReplacementReadback) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) >= 3 && command.Argv[2] == "list" && f.afterRemoval {
		f.base.commands = append(f.base.commands, append([]string(nil), command.Argv...))
		return legacyports.CommandResult{Stdout: []byte("[] {}")}, nil
	}
	result, err := f.base.Run(ctx, command)
	if len(command.Argv) >= 3 && command.Argv[2] == "uninstall" && err == nil && result.ExitCode == 0 {
		f.afterRemoval = true
	}
	return result, err
}

// A stale explicit attestation must not turn a removed native registration
// into a successful replacement when its post-uninstall listing is unknown.
func TestGrokUserReplacementUnknownReadbackCannotBeAttested(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	request.Replacing = true
	base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
		Source: request.Delivery.ActivePath, Version: "1.0.0"}}}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: &grokMalformedReplacementReadback{base: base}}, request)
	if err == nil || outcome.Activation != domain.ActivationFailed || outcome.Verification != domain.VerificationFailed ||
		outcome.NativeEffect != domain.NativeEffectUncertain || outcome.ActivationAttested || len(base.commands) != 3 ||
		base.commands[0][2] != "list" || base.commands[1][2] != "uninstall" || base.commands[2][2] != "list" {
		t.Fatalf("unreadable replacement removal became attested or triggered install: %+v %v %+v", outcome, err, base.commands)
	}
}

// A failed listing or a foreign same-name plugin must never authorize
// deletion of the owned artifact. Only a fresh native absence or a verified
// completed unregister releases the filesystem cleanup boundary.
func TestGrokUserRemovalRequiresPositiveNativeReadback(t *testing.T) {
	request := grokRequest(t)
	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
		BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath, Confirmed: true}
	t.Run("unreadable-list", func(t *testing.T) {
		base := &fakeGrok{}
		cause := errors.New("native listing unavailable")
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: &grokFaultRunner{
			other: base, verb: "list", err: cause}}, remove)
		if !errors.Is(err, cause) || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
			len(base.commands) != 1 || base.commands[0][2] != "list" {
			t.Fatalf("unknown native registry allowed cleanup: %+v %v %+v", outcome, err, base.commands)
		}
	})
	t.Run("foreign-registration", func(t *testing.T) {
		base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
			Source: request.Delivery.ActivePath + "-foreign", Version: request.Plan.DeclaredVersion}}}
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: base}, remove)
		if err == nil || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
			len(base.commands) != 1 || base.commands[0][2] != "list" {
			t.Fatalf("foreign native registration allowed cleanup: %+v %v %+v", outcome, err, base.commands)
		}
	})
	for _, tc := range []struct {
		name    string
		entries []pluginEntry
		calls   int
	}{
		{name: "confirmed-absent", calls: 1},
		{name: "verified-owned-uninstall", calls: 3, entries: []pluginEntry{{Name: request.DeclaredName,
			Status: "installed", Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &fakeGrok{entries: tc.entries}
			outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: base}, remove)
			if err != nil || !outcome.ExternalRemovalComplete || !outcome.ArtifactRemovalAllowed || len(base.commands) != tc.calls {
				t.Fatalf("verified native removal was not authorized: %+v %v %+v", outcome, err, base.commands)
			}
		})
	}
}

// --external-uninstalled is an attestation for an unavailable client, not a
// bypass of a runnable Grok CLI's live registration. Never remove managed
// bytes while an existing registration could still refer to those bytes.
func TestGrokUserExternalUninstalledCannotBypassAvailableNativeRegistry(t *testing.T) {
	request := grokRequest(t)
	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
		BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath,
		Confirmed: true, ExternalUninstalled: true}

	t.Run("unconfirmed-preview-does-not-probe-native-cli", func(t *testing.T) {
		runner := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
			Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
		preview := remove
		preview.Confirmed = false
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, preview)
		if err != nil || len(runner.commands) != 0 || !outcome.ArtifactRemovalAllowed {
			t.Fatalf("external preview invoked native Grok or failed: %+v %v %+v", outcome, err, runner.commands)
		}
	})
	t.Run("still-registered", func(t *testing.T) {
		runner := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
			Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, remove)
		if err == nil || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
			len(runner.commands) != 1 || runner.commands[0][2] != "list" {
			t.Fatalf("external attestation bypassed native registration: %+v %v %+v", outcome, err, runner.commands)
		}
	})
	t.Run("native-list-unreadable", func(t *testing.T) {
		base := &fakeGrok{}
		cause := errors.New("native listing unreadable")
		runner := &grokFaultRunner{other: base, verb: "list", err: cause}
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, remove)
		if !errors.Is(err, cause) || outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
			len(base.commands) != 1 || base.commands[0][2] != "list" {
			t.Fatalf("external attestation hid unreadable native registration: %+v %v %+v", outcome, err, base.commands)
		}
	})
	t.Run("native-list-unclosed", func(t *testing.T) {
		base := &fakeGrok{}
		runner := &grokFaultRunner{other: base, verb: "list", err: &domain.UnclosedClientProbe{
			Operation: "grok plugin list", Err: errors.New("unknown process closure")}}
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, remove)
		if !domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain ||
			outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
			len(base.commands) != 1 || base.commands[0][2] != "list" {
			t.Fatalf("external attestation hid unclosed native registration: %+v %v %+v", outcome, err, base.commands)
		}
	})
	t.Run("native-confirmed-absent", func(t *testing.T) {
		runner := &fakeGrok{}
		outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, remove)
		if err != nil || !outcome.ExternalRemovalComplete || !outcome.ArtifactRemovalAllowed ||
			len(runner.commands) != 1 || runner.commands[0][2] != "list" {
			t.Fatalf("verified external removal was rejected: %+v %v %+v", outcome, err, runner.commands)
		}
	})
}

type grokMalformedInstallReadback struct {
	base         *fakeGrok
	afterInstall bool
}

func (f *grokMalformedInstallReadback) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) >= 3 && command.Argv[2] == "list" && f.afterInstall {
		f.base.commands = append(f.base.commands, append([]string(nil), command.Argv...))
		return legacyports.CommandResult{Stdout: []byte("[] {}")}, nil
	}
	result, err := f.base.Run(ctx, command)
	if len(command.Argv) >= 3 && command.Argv[2] == "install" && err == nil && result.ExitCode == 0 {
		f.afterInstall = true
	}
	return result, err
}

// An old operator confirmation is not proof of the new install's visibility.
// A malformed post-install list must fail closed without a second mutation.
func TestGrokUserPostInstallUnknownReadbackCannotReuseManualAttestation(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	base := &fakeGrok{entries: []pluginEntry{}}
	runner := &grokMalformedInstallReadback{base: base}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: runner}, request)
	if !errors.Is(err, errUnknownList) || outcome.Activation != domain.ActivationFailed ||
		outcome.Verification != domain.VerificationFailed || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.ActivationAttested || outcome.AuthoritativeObservation || len(base.commands) != 3 ||
		base.commands[0][2] != "list" || base.commands[1][2] != "install" || base.commands[2][2] != "list" {
		t.Fatalf("unreadable post-install registration became attested Ready: %+v %v %+v", outcome, err, base.commands)
	}
}

func TestGrokUserVerifyOnlyMayStillUseExplicitManualAttestation(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	request.VerifyOnly = true
	base := &fakeGrok{}
	runner := &grokMalformedInstallReadback{base: base, afterInstall: true}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: runner}, request)
	if err != nil || outcome.Activation != domain.ActivationActive || !outcome.ActivationAttested ||
		outcome.NativeEffect == domain.NativeEffectUncertain || len(base.commands) != 1 ||
		base.commands[0][2] != "list" {
		t.Fatalf("read-only manual attestation contract changed: %+v %v %+v", outcome, err, base.commands)
	}
}

type grokMutationReadbackFailure struct {
	base             *fakeGrok
	mutationComplete bool
}

func (r *grokMutationReadbackFailure) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) > 2 && command.Argv[2] == "list" && r.mutationComplete {
		r.base.commands = append(r.base.commands, append([]string(nil), command.Argv...))
		return legacyports.CommandResult{}, errors.New("native Grok list failed after completed mutation")
	}
	result, err := r.base.Run(ctx, command)
	if len(command.Argv) > 2 && (command.Argv[2] == "install" || command.Argv[2] == "uninstall") &&
		err == nil && result.ExitCode == 0 {
		r.mutationComplete = true
	}
	return result, err
}

func TestGrokUserReplacementFailedReadbackKeepsUncertainEffect(t *testing.T) {
	request := grokRequest(t)
	request.Replacing = true
	request.ActivationComplete = true
	base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
		Source: request.Delivery.ActivePath, Version: "1.0.0"}}}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: &grokMutationReadbackFailure{base: base}}, request)
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.Activation != domain.ActivationFailed || outcome.ActivationAttested ||
		len(base.commands) != 3 || base.commands[0][2] != "list" ||
		base.commands[1][2] != "uninstall" || base.commands[2][2] != "list" {
		t.Fatalf("replacement continued without trustworthy post-uninstall state: %+v %v %+v", outcome, err, base.commands)
	}
}

func TestGrokUserRemovalFailedReadbackKeepsUncertainEffect(t *testing.T) {
	request := grokRequest(t)
	base := &fakeGrok{entries: []pluginEntry{{Name: request.DeclaredName, Status: "installed",
		Source: request.Delivery.ActivePath, Version: request.Plan.DeclaredVersion}}}
	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: request.DeclaredName,
		BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath, Confirmed: true}
	outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: &grokMutationReadbackFailure{base: base}}, remove)
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.ExternalRemovalComplete || outcome.ArtifactRemovalAllowed ||
		len(base.commands) != 3 || base.commands[0][2] != "list" ||
		base.commands[1][2] != "uninstall" || base.commands[2][2] != "list" {
		t.Fatalf("unverified native removal allowed artifact deletion: %+v %v %+v", outcome, err, base.commands)
	}
}

type grokUnexpectedInstalledVersion struct {
	base *fakeGrok
}

func (r *grokUnexpectedInstalledVersion) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := r.base.Run(ctx, command)
	if len(command.Argv) > 2 && command.Argv[2] == "install" && err == nil && result.ExitCode == 0 {
		r.base.entries[0].Version = "unexpected-native-version"
	}
	return result, err
}

func TestGrokUserPostInstallNegativeReadbackKeepsUncertainEffect(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	base := &fakeGrok{entries: []pluginEntry{}}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: &grokUnexpectedInstalledVersion{base: base}}, request)
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.Activation != domain.ActivationFailed || outcome.ActivationAttested ||
		len(base.commands) != 3 || base.commands[0][2] != "list" ||
		base.commands[1][2] != "install" || base.commands[2][2] != "list" {
		t.Fatalf("negative readback after install lost native uncertainty: %+v %v %+v", outcome, err, base.commands)
	}
}

func TestGrokUserFailedNativeMutationCannotUseUnknownListAttestation(t *testing.T) {
	request := grokRequest(t)
	request.ActivationComplete = true
	base := &fakeGrok{entries: []pluginEntry{}}
	outcome, err := New().Activate(t.Context(), clients.Env{Runner: &grokFaultRunner{
		other: base, verb: "install", err: errUnknownList,
	}}, request)
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain ||
		outcome.Activation == domain.ActivationActive || outcome.ActivationAttested ||
		outcome.Verification == domain.VerificationInstalled || len(base.commands) != 2 ||
		base.commands[0][2] != "list" || base.commands[1][2] != "install" {
		t.Fatalf("failed native mutation reused manual list attestation: %+v %v %+v", outcome, err, base.commands)
	}
}
