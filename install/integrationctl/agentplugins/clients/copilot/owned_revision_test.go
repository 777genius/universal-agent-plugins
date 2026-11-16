package copilot

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Regression: a changed-version update plans the new version while Copilot
// still lists the installed one; identity preflight must not turn that into an
// indeterminate ownership finding.
func TestCopilotUserUpdateAcceptsInstalledOwnedRevision(t *testing.T) {
	request := copilotUserLiveRequest(t)
	installed := string(copilotUserLiveDocument(request))
	request.Plan.DeclaredVersion = "1.3.0"
	foreign := "  • foreign-plugin@foreign-mkt (v2.0.0) (enabled)\n      from " + filepath.Join(filepath.Dir(request.Plan.ActivePath), "foreign") + "\n"
	owned := func() *domain.ClientBinding {
		return &domain.ClientBinding{ClientID: "copilot", Scope: "user", PhysicalArtifact: request.Plan.PhysicalArtifactID,
			TargetLocator: request.Plan.ActivePath, PackageRevision: &domain.ClientPackageRevision{Version: "1.2.3"}}
	}
	for _, test := range []struct {
		name    string
		body    string
		managed func() *domain.ClientBinding
		want    clients.RegistryFinding
	}{
		{"installed revision", installed, owned, clients.RegistryExpected},
		{"installed revision on 1.0.112 with foreign plugin", strings.Replace(installed, copilot103BuiltinSection, copilot112BuiltinSection, 1) + foreign, owned, clients.RegistryExpected},
		{"planned revision", strings.Replace(installed, "(v1.2.3)", "(v1.3.0)", 1), owned, clients.RegistryExpected},
		{"other revision", strings.Replace(installed, "(v1.2.3)", "(v9.9.9)", 1), owned, clients.RegistryIndeterminate},
		{"disabled", strings.Replace(installed, "(enabled)", "(disabled)", 1), owned, clients.RegistryIndeterminate},
		{"foreign path", strings.Replace(installed, "from "+request.Plan.ActivePath, "from "+request.Plan.ActivePath+"-foreign", 1), owned, clients.RegistryIndeterminate},
		{"unowned", installed, func() *domain.ClientBinding { return nil }, clients.RegistryIndeterminate},
		{"binding without revision", installed, func() *domain.ClientBinding {
			binding := owned()
			binding.PackageRevision = nil
			return binding
		}, clients.RegistryIndeterminate},
		{"binding for another artifact", installed, func() *domain.ClientBinding {
			binding := owned()
			binding.PhysicalArtifact = "other-owned"
			return binding
		}, clients.RegistryIndeterminate},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &copilotUserLiveRunner{t: t, steps: []copilotUserLiveStep{copilotUserLiveListStep(request, []byte(test.body), true)}}
			got, err := shared.InspectCopilotRegistryWithListingRecognition(context.Background(), clients.Env{Runner: runner}, request.Plan, test.managed(), recognizeCopilotUserRegistry)
			if err != nil || got != test.want || runner.calls != 1 {
				t.Fatalf("finding=%v want=%v err=%v calls=%d", got, test.want, err, runner.calls)
			}
		})
	}
}

// Regression: with COPILOT_HOME unset after a selected-profile install, update
// registered an untracked second copy in the default profile.
func TestCopilotUserBindingRefusesDifferentSelectedProfile(t *testing.T) {
	request := copilotUserLiveRequest(t)
	recorded, other := filepath.Join(t.TempDir(), "selected"), filepath.Join(t.TempDir(), "default")
	managed := &domain.ClientBinding{ClientID: "copilot", Scope: "user", PhysicalArtifact: request.Plan.PhysicalArtifactID,
		TargetLocator: request.Plan.ActivePath, NativeProfileRoot: recorded}
	runner := &copilotUserLiveRunner{t: t}
	finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: runner}, domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: other}, request.Plan, managed)
	if err == nil || finding != clients.RegistryIndeterminate || runner.calls != 0 {
		t.Fatalf("different profile admitted: finding=%v err=%v calls=%d", finding, err, runner.calls)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeUser}
	for _, test := range []struct {
		name     string
		input    clients.PlanInput
		admitted bool
	}{
		{"fresh install", clients.PlanInput{Client: domain.DetectedClient{ConfigRoot: other}}, true},
		{"legacy binding", clients.PlanInput{Client: domain.DetectedClient{ConfigRoot: other}, PreviousBindingRecorded: true}, true},
		{"recorded profile", clients.PlanInput{Client: domain.DetectedClient{ConfigRoot: recorded}, PreviousBindingRecorded: true, PreviousNativeProfileRoot: recorded}, true},
		{"other profile", clients.PlanInput{Client: domain.DetectedClient{ConfigRoot: other}, PreviousBindingRecorded: true, PreviousNativeProfileRoot: recorded}, false},
	} {
		if err := New().CheckPlanPrecondition(test.input, &plan); (err == nil) != test.admitted {
			t.Fatalf("%s: err=%v", test.name, err)
		}
	}
}
