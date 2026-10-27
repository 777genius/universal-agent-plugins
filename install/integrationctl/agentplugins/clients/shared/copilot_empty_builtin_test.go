package shared

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Independently authored parser input, never an executed or acquired client.
const copilotInertDisabledBuiltin = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\nBuilt-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"

type copilotEmptyRegistryRunner struct {
	t      *testing.T
	result legacyports.CommandResult
	err    error
	cancel context.CancelFunc
	calls  int
}

func (r *copilotEmptyRegistryRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Fatal("native registry must preserve its tree-aware runner")
	return legacyports.CommandResult{}, nil
}

func (r *copilotEmptyRegistryRunner) RunWithTreeExitGrace(_ context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	r.t.Helper()
	r.calls++
	want := legacyports.Command{Argv: []string{"/inert/never-executed/copilot", "plugin", "list"}}
	if !reflect.DeepEqual(command, want) || grace != time.Second {
		r.t.Fatal("native registry command or original tree exit grace changed")
	}
	if r.cancel != nil {
		r.cancel()
	}
	return r.result, r.err
}

func copilotInertUserPlan() domain.DeliveryPlan {
	return domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeUser,
		DeclaredName: "uap-copilot-unrelated", DeclaredVersion: "1.0.0",
		PhysicalArtifactID:       "uap-copilot-unrelated-0123456789ab",
		NativeRegistryExecutable: "/inert/never-executed/copilot"}
}

func TestCopilotEmptyDisabledBuiltinExactUserRegistry(t *testing.T) {
	if len(copilotInertDisabledBuiltin) != 138 || fmt.Sprintf("%x", sha256.Sum256([]byte(copilotInertDisabledBuiltin))) != "f09cb232e9a7f7bd379daed96099ea1fdf8f6644dcc390ce43c7854874d7e879" {
		t.Fatal("independent whole observed tuple differs")
	}
	for _, owned := range []bool{false, true} {
		plan := copilotInertUserPlan()
		runner := &copilotEmptyRegistryRunner{t: t, result: legacyports.CommandResult{Stdout: []byte(copilotInertDisabledBuiltin)}}
		var managed *domain.ClientBinding
		if owned {
			managed = &domain.ClientBinding{TargetLocator: "/inert/managed/previous"}
		}
		finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, plan, managed)
		if err != nil || finding != clients.RegistryClear || runner.calls != 1 {
			t.Fatalf("owned=%v finding=%v calls=%d err=%v", owned, finding, runner.calls, err)
		}
	}
	if got := CopilotRegistryFindingAt([]byte(copilotInertDisabledBuiltin), "unrelated", "marketplace", "1.0.0", "/inert/managed", false); got != clients.RegistryIndeterminate {
		t.Fatal("generic shared classifier changed")
	}
}

func TestCopilotEmptyDisabledBuiltinRejectsAlteredWholeDocument(t *testing.T) {
	cases := []string{
		strings.ReplaceAll(copilotInertDisabledBuiltin, "[disabled]", "[enabled]"),
		strings.ReplaceAll(copilotInertDisabledBuiltin, " [disabled]", ""),
		strings.ReplaceAll(copilotInertDisabledBuiltin, "browser-use", "other-builtin"),
		strings.ReplaceAll(copilotInertDisabledBuiltin, "v0.0.4", "v0.0.5"),
		copilotInertDisabledBuiltin + "  • other (v0.0.4) [disabled]\n",
		copilotInertDisabledBuiltin + "  • browser-use (v0.0.4) [disabled]\n",
		copilotInertDisabledBuiltin + "Built-in plugins:\n",
		"unexpected\n" + copilotInertDisabledBuiltin,
		copilotInertDisabledBuiltin + "unexpected\n",
		copilotInertDisabledBuiltin + "\n",
		strings.TrimSuffix(copilotInertDisabledBuiltin, "\n"),
		strings.ReplaceAll(copilotInertDisabledBuiltin, "\n", "\r\n"),
		"\x1b[32m" + copilotInertDisabledBuiltin,
		copilotInertDisabledBuiltin + "\x00",
		strings.ReplaceAll(copilotInertDisabledBuiltin, "No plugins installed.", "No plugins installed"),
		strings.ReplaceAll(copilotInertDisabledBuiltin, "install <source>", "install <other>"),
		copilotInertDisabledBuiltin + "Installed plugins:\n  • other@marketplace (v1.0.0)\n",
		strings.TrimSuffix(copilotInertDisabledBuiltin, "  • browser-use (v0.0.4) [disabled]\n"),
	}
	for index, text := range cases {
		runner := &copilotEmptyRegistryRunner{t: t, result: legacyports.CommandResult{Stdout: []byte(text)}}
		finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, copilotInertUserPlan(), nil)
		if err != nil || finding != clients.RegistryIndeterminate || runner.calls != 1 {
			t.Fatalf("case=%d finding=%v calls=%d err=%v", index, finding, runner.calls, err)
		}
	}
}

func TestCopilotEmptyDisabledBuiltinRequiresHeldClientScopeAndName(t *testing.T) {
	mutations := []func(*domain.DeliveryPlan){
		func(p *domain.DeliveryPlan) { p.ClientID = domain.ClientVSCode },
		func(p *domain.DeliveryPlan) { p.ClientID = domain.ClientCline },
		func(p *domain.DeliveryPlan) { p.ClientID = "" },
		func(p *domain.DeliveryPlan) { p.Scope = domain.ScopeProject },
		func(p *domain.DeliveryPlan) { p.Scope = domain.ScopeLocal },
		func(p *domain.DeliveryPlan) { p.Scope = "" },
		func(p *domain.DeliveryPlan) { p.TargetContext.Scope = domain.ScopeProject },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "browser-use" },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "BROWSER-USE" },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "Browser-Use" },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "" },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "   " },
		func(p *domain.DeliveryPlan) { p.DeclaredName = " unrelated" },
		func(p *domain.DeliveryPlan) { p.DeclaredName = "unrelated\n" },
	}
	for index, mutate := range mutations {
		plan := copilotInertUserPlan()
		mutate(&plan)
		runner := &copilotEmptyRegistryRunner{t: t, result: legacyports.CommandResult{Stdout: []byte(copilotInertDisabledBuiltin)}}
		finding, err := InspectCopilotRegistry(context.Background(), clients.Env{Runner: runner}, plan, nil)
		if err != nil || finding != clients.RegistryIndeterminate || runner.calls != 1 {
			t.Fatalf("case=%d finding=%v calls=%d err=%v", index, finding, runner.calls, err)
		}
	}
}

func TestCopilotEmptyDisabledBuiltinPreservesRunnerAndContextRefusal(t *testing.T) {
	problem := errors.New("inert registry refusal")
	for _, failure := range []string{"runner", "exit", "cancel-before", "cancel-after"} {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		runner := &copilotEmptyRegistryRunner{t: t, result: legacyports.CommandResult{Stdout: []byte(copilotInertDisabledBuiltin)}}
		wantCalls := 1
		switch failure {
		case "runner":
			runner.err = problem
		case "exit":
			runner.result.ExitCode = 1
		case "cancel-before":
			cancel()
			wantCalls = 0
		case "cancel-after":
			runner.cancel = cancel
		}
		finding, err := InspectCopilotRegistry(ctx, clients.Env{Runner: runner}, copilotInertUserPlan(), nil)
		if err == nil || finding != clients.RegistryIndeterminate || runner.calls != wantCalls {
			t.Fatalf("case=%s finding=%v calls=%d err=%v", failure, finding, runner.calls, err)
		}
		if failure == "runner" && !errors.Is(err, problem) || strings.HasPrefix(failure, "cancel-") && !errors.Is(err, context.Canceled) {
			t.Fatal("original registry error propagation changed")
		}
	}
}
