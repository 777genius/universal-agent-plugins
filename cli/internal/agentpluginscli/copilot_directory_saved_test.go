package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This exercises saved dependencies with a preverified Directory fixture and
// native command fixtures; it is not signature or live Copilot qualification.
func TestSavedCopilotOnlyDirectoryLifecycle(t *testing.T) {
	rollout := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCopilot}, nil)
	rollout.retainV1Only()
	repo := dependencyRepo(t)
	spec := dependencyClientSpec{id: domain.ClientCopilot, executable: true, activation: "active"}
	fixture, _, _ := dependencySpecHarness(t, repo, spec)
	fixture.app.DirectoryClient = rollout.directory
	fixture.app.SourceAcquirer = rollout.acquirer
	dependencyRun(t, fixture, "add", "rollout-demo", "--target", "copilot", "--format", "json")
	status := dependencyRun(t, fixture, "deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatalf("saved Copilot status did not converge: %s", status)
	}
	dependencyRun(t, fixture, "install", "--locked", "--yes", "--format", "json")
	dependencyRun(t, fixture, "update", "demo", "--format", "json")
	dependencyRun(t, fixture, "remove", "demo", "--target", "copilot", "--format", "json")
}

func TestDirectoryBindingTargetsRetainUnrelatedAndVSCodeOwnedSurfaces(t *testing.T) {
	binding := domain.ClientBinding{ClientID: string(domain.ClientCopilot), Scope: string(domain.ScopeUser),
		AffectedSurfaces: []string{"copilot", "vscode", "cursor"}}
	if got := directoryBindingTargets(binding); !reflect.DeepEqual(got, []domain.ClientID{domain.ClientCopilot, domain.ClientCursor}) {
		t.Fatalf("Copilot binding lost an independently affected client: %v", got)
	}
	binding.ClientID = string(domain.ClientVSCode)
	if got := directoryBindingTargets(binding); !reflect.DeepEqual(got, []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode, domain.ClientCursor}) {
		t.Fatalf("VS Code-owned binding lost a shared or independent client: %v", got)
	}
}
