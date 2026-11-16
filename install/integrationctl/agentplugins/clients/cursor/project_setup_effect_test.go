package cursor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type cursorApprovalFaultFiles struct {
	path   string
	effect domain.NativeEffectState
	fault  error
	writes int
}

func (*cursorApprovalFaultFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}

func (files *cursorApprovalFaultFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	if path != files.path {
		return errors.New("unexpected mutation outside the approval store")
	}
	files.writes++
	if files.writes == 2 && files.effect == domain.NativeEffectCommitted {
		return files.fault
	}
	if files.writes == 2 && files.effect == domain.NativeEffectUncertain {
		body = []byte(`["foreign-concurrent-approval"]`)
	}
	if err := os.WriteFile(path, body, mode); err != nil {
		return err
	}
	if files.writes == 1 || files.effect == domain.NativeEffectUncertain {
		return files.fault
	}
	return nil
}

func (*cursorApprovalFaultFiles) RemoveNoFollow(string) error {
	return errors.New("existing approval store must not be removed")
}

type cursorSetupRunnerFunc func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (runner cursorSetupRunnerFunc) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return runner(ctx, command)
}

func TestCursorCompletionRetainsFailedApprovalRollbackEffect(t *testing.T) {
	for _, effect := range []domain.NativeEffectState{domain.NativeEffectUnchanged, domain.NativeEffectCommitted, domain.NativeEffectUncertain} {
		t.Run(string(effect), func(t *testing.T) {
			request, kernel := cursorApprovalFixture(t)
			path := cursorMCPPolicyPath(request, "mcp-approvals.json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			original := []byte(`["foreign-approval"]`)
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			key, err := cursorOwnedApprovalKey(clients.Env{NativeConfig: kernel}, request, "audit")
			if err != nil {
				t.Fatal(err)
			}
			fault := errors.New("approval write reported failure after becoming visible")
			files := &cursorApprovalFaultFiles{path: path, effect: effect, fault: fault}
			calls := []string{}
			runner := cursorSetupRunnerFunc(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				args := strings.Join(command.Argv[1:], " ")
				calls = append(calls, args)
				switch args {
				case "mcp list":
					return legacyports.CommandResult{Stdout: []byte("audit: not loaded (needs approval)\n"), NoChildStarted: true}, nil
				case "--version":
					return legacyports.CommandResult{Stdout: []byte("2026.09.18-9a7762b\n"), NoChildStarted: true}, nil
				default:
					return legacyports.CommandResult{NoChildStarted: true}, fmt.Errorf("unexpected native command: %s", args)
				}
			})
			executable := filepath.Join(t.TempDir(), "cursor-agent")
			env := clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: runner,
				LookupExecutable: func(string) (string, error) { return executable, nil }}
			t.Setenv("CURSOR_CONFIG_DIR", request.Client.ConfigRoot)
			t.Setenv("CURSOR_DATA_DIR", request.Client.ConfigRoot)
			initial := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
				NativeEffect: domain.NativeEffectCommitted, NativeObjects: request.Delivery.NativeObjects}
			outcome, err := completeCursorProjectMCP(t.Context(), env, request, initial)
			wantEffect := domain.NativeEffectCommitted // The preceding native-config install already committed.
			if effect == domain.NativeEffectUncertain {
				wantEffect = domain.NativeEffectUncertain
				var observed *shared.NativeEffectError
				if !errors.Is(err, fault) || !errors.As(err, &observed) || observed.Effect != effect {
					t.Fatalf("uncertain approval was downgraded to guidance: %+v %v", outcome, err)
				}
			} else if err != nil || len(outcome.UserActions) == 0 {
				t.Fatalf("known approval outcome lost existing prepared guidance: %+v %v", outcome, err)
			}
			if outcome.NativeEffect != wantEffect || outcome.Activation != domain.ActivationPrepared || !reflect.DeepEqual(outcome.NativeObjects, initial.NativeObjects) || files.writes != 2 || !reflect.DeepEqual(calls, []string{"mcp list", "--version"}) {
				t.Fatalf("approval failure lost effect or continued setup: %+v %v; writes=%d calls=%v", outcome, err, files.writes, calls)
			}
			want := original
			switch effect {
			case domain.NativeEffectCommitted:
				want, err = cursorAppendApproval(original, key)
				if err != nil {
					t.Fatal(err)
				}
			case domain.NativeEffectUncertain:
				want = []byte(`["foreign-concurrent-approval"]`)
			}
			body, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(body, want) {
				t.Fatalf("rollback evidence or foreign approval changed: %q %v", body, err)
			}
		})
	}
}

func TestCursorReviewDisclosesOnlySelectedProjectApproval(t *testing.T) {
	for _, test := range []struct {
		name                  string
		project, mcp, prepare bool
	}{
		{"project MCP", true, true, false},
		{"project Skill", true, false, false},
		{"user MCP", false, true, false},
		{"preparation", true, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, _ := cursorApprovalFixture(t)
			plan := request.Plan
			if !test.mcp {
				plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative}}
			}
			plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "withheld-server", Support: domain.SupportUnsupported})
			if !test.project {
				plan.Scope, plan.TargetContext.Scope = domain.ScopeUser, domain.ScopeUser
			}
			in := clients.PlanInput{Client: request.Client, TargetContext: plan.TargetContext}
			if test.prepare {
				in.Intent = domain.InstallIntentPrepare
			}
			if err := New().RefinePlan(t.Context(), in, &plan); err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(plan.Warnings, "\n")
			want := test.project && test.mcp && !test.prepare
			if strings.Contains(joined, "Installation may approve") != want {
				t.Fatalf("wrong approval disclosure: %q", joined)
			}
			if want && (!strings.Contains(joined, "servers audit") || !strings.Contains(joined, fmt.Sprintf("project %q", plan.TargetContext.ProjectRoot)) || !strings.Contains(joined, "approvals remain after removal") || !strings.Contains(joined, "Existing disablement is preserved") || strings.Contains(joined, "withheld-server")) {
				t.Fatalf("approval warning lacks exact scope or policy: %q", joined)
			}
			before := append([]string(nil), plan.Warnings...)
			if err := New().RefinePlan(t.Context(), in, &plan); err != nil || !reflect.DeepEqual(plan.Warnings, before) {
				t.Fatalf("repeated planning changed disclosure: %q %v", plan.Warnings, err)
			}
		})
	}
}
