package providers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Production loader/planner/rendering/staging and provider lifecycle, with an
// inert exact-list runner. This is not a native Claude execution observation.
func TestProjectClaudeComposedProjectionListingAndScopeIsolation(t *testing.T) {
	f := newClaudeProjectFixture(t)
	user, other := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
	actor := f.activator()
	for _, verify := range []bool{false, true, false} {
		f.request.VerifyOnly = verify
		outcome, err := actor.Activate(t.Context(), f.request)
		if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested {
			t.Fatalf("exact project listing=%+v,%v", outcome, err)
		}
	}
	active := f.request.Plan.ActivePath
	body, err := os.ReadFile(filepath.Join(active, claude.ClaudeRuntimeDirectory, "skills", "demo-echo", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "UAP-CLAUDE-PROJECT-MARKER") {
		t.Fatalf("rendered command=%q,%v", body, err)
	}
	mcp, err := os.ReadFile(filepath.Join(active, ".mcp.json"))
	if err != nil || !strings.Contains(string(mcp), "https://managed.test/v1") {
		t.Fatalf("project plugin MCP=%q,%v", mcp, err)
	}
	finding, err := claude.New().InspectNativeRegistry(t.Context(), clients.Env{Runner: f.runner}, f.request.Client, f.request.Plan, &domain.ClientBinding{})
	if err != nil || finding != clients.RegistryExpected {
		t.Fatalf("project registry=%v,%v", finding, err)
	}
	for range 2 {
		outcome, err := actor.Deactivate(t.Context(), f.removal())
		if err != nil || !outcome.ExternalRemovalComplete {
			t.Fatalf("exact project removal admission=%+v,%v", outcome, err)
		}
	}
	if !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
		t.Fatal("project listing/projection changed other scopes")
	}
}

func TestProjectClaudeWrongDisabledOrUnknownNativeListingRefuses(t *testing.T) {
	for _, condition := range []string{"user-scope", "wrong-project", "malformed"} {
		t.Run(condition, func(t *testing.T) {
			f := newClaudeProjectFixture(t)
			switch condition {
			case "user-scope":
				f.runner.scope = "user"
			case "wrong-project":
				f.runner.wrongPath = true
			case "disabled":
				f.runner.enabled = false
			case "malformed":
				f.runner.malformed = true
			}
			before := projectGeminiInventory(t, f.request.Plan.TargetContext.ProjectRoot)
			if _, err := f.activator().Activate(t.Context(), f.request); err == nil {
				t.Fatal("nonexact listing established activation")
			}
			if _, err := f.activator().Deactivate(t.Context(), f.removal()); err == nil {
				t.Fatal("nonexact listing admitted active removal")
			}
			f.request.ActivationComplete = true
			for _, prepared := range []bool{false, true} {
				if prepared {
					f.request.Plan.Components[0].Support = domain.SupportPrepared
				}
				if _, err := f.activator().Activate(t.Context(), f.request); err == nil || !strings.Contains(err.Error(), "activation attestation") {
					t.Fatalf("attestation bypassed project listing: %v", err)
				}
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.request.Plan.TargetContext.ProjectRoot)) {
				t.Fatal("read-only listing refusal changed native files")
			}
		})
	}
}

func TestProjectClaudeChangedContextRefusesBeforeNativeListing(t *testing.T) {
	for _, change := range []string{"scope", "original-user", "registry", "copied-project"} {
		t.Run(change, func(t *testing.T) {
			f := newClaudeProjectFixture(t)
			switch change {
			case "scope":
				f.request.Plan.TargetContext.Scope = domain.ScopeUser
			case "original-user":
				f.request.Plan.TargetContext.UserConfigRoot = f.other
			case "registry":
				f.request.Plan.NativeRegistryRoot = f.user
			case "copied-project":
				f.request.Plan.TargetContext.ProjectRoot = f.other
				f.request.Plan.NativeRegistryRoot = filepath.Join(f.other, ".claude")
			}
			if _, err := f.activator().Activate(t.Context(), f.request); err == nil {
				t.Fatal("changed project context admitted")
			}
			if f.runner.calls != 0 {
				t.Fatal("changed authority reached native runner")
			}
		})
	}
}

func TestProjectClaudeBlockedDiscoveryIsPreparedAndInactiveRemovalIsOwned(t *testing.T) {
	for _, condition := range []string{"disabled", "suppressed", "suppressed-user"} {
		t.Run(condition, func(t *testing.T) {
			f := newClaudeProjectFixture(t)
			if condition == "disabled" {
				f.runner.enabled = false
			} else {
				f.runner.suppressed = true
			}
			f.runner.otherSource = condition == "suppressed-user"
			beforeUser, beforeB := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
			for _, verify := range []bool{false, true} {
				f.request.VerifyOnly = verify
				out, err := f.activator().Activate(t.Context(), f.request)
				if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationInstalled || out.ActivationAttested || len(out.UserActions) == 0 {
					t.Fatalf("blocked observation=%+v,%v", out, err)
				}
			}
			request := f.removal()
			if _, err := f.activator().Deactivate(t.Context(), request); err == nil {
				t.Fatal("blocked native evidence silently removed formerly active binding")
			}
			request.CurrentActivation = domain.ActivationPrepared
			out, err := f.activator().Deactivate(t.Context(), request)
			if err != nil || !out.ExternalRemovalComplete || !out.ArtifactRemovalAllowed {
				t.Fatalf("inactive removal admission=%+v,%v", out, err)
			}
			if !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, f.other)) {
				t.Fatal("blocked lifecycle changed other scope")
			}
		})
	}
}

func TestProjectClaudeSuppressionDoesNotSupplyOwnershipOrDiscovery(t *testing.T) {
	f := newClaudeProjectFixture(t)
	f.runner.suppressed = true
	observer := providers.NativeIdentityObserver{Registry: f.registry, Runner: f.runner, Stager: f.stager}
	binding := domain.ClientBinding{NativeObjects: f.request.Delivery.NativeObjects}
	observe := func(managed *domain.ClientBinding) domain.NativeIdentityObservation {
		t.Helper()
		out, err := observer.ObserveNativeIdentity(t.Context(), f.request.Client, f.request.Plan, managed)
		if err != nil {
			t.Fatal(err)
		}
		if out.NativeDiscoveryReconciled {
			t.Fatal("suppression reconciled native discovery")
		}
		return out
	}
	if out := observe(&binding); out.State != domain.NativeIdentityManaged || !out.ReceiptReconciled {
		t.Fatalf("owned suppressed=%+v", out)
	}
	if out := observe(nil); out.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("unowned exact destination was adopted: %+v", out)
	}
	if err := os.RemoveAll(f.request.Plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	if out := observe(nil); out.State != domain.NativeIdentityAbsent {
		t.Fatalf("clear initial destination=%+v", out)
	}
	projectGeminiWrite(t, filepath.Join(f.request.Plan.TargetRoot, "foreign-same-name", ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	if out := observe(nil); out.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("suppression hid foreign project artifact: %+v", out)
	}
}
