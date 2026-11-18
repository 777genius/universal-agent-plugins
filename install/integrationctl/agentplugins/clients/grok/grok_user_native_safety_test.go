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
		if entries, err := parseList(body); err == nil {
			t.Fatalf("native registry admitted folded aliases: %+v", entries)
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
			base := &fakeGrok{}
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
			if uncertain && outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("uncertain install effect lost: %+v", outcome)
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
			if uncertain && outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("uncertain removal effect lost: %+v", outcome)
			}
		})
	}
}
