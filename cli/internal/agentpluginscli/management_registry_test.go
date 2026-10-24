package agentpluginscli

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestManagementRegistryReservedIdentityIsNotInstalledAuthority(t *testing.T) {
	clients := []domain.ClientID{domain.ClientCodex, domain.ClientClaude}
	directory := newRolloutDirectoryFixture(t, clients, clients)
	repo, _ := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	j.cli.app.DirectoryClient = directory.directory
	j.cli.app.SourceAcquirer = savedRegistrySource{savedJourneySource: j.source, directory: directory.acquirer}
	backend := managementJourneyBackend(t, j)
	prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "rollout-demo", Alias: "docs", TargetIds: []string{"profile-codex", "profile-claude"}})
	retained, ok := prepared.(*managementPrepared)
	if !ok || len(retained.request.prepared) != 2 {
		t.Fatal("registry review lost its retained target set")
	}
	first := retained.request.prepared[0]
	if first.InstallationID == "" || !unboundDependencyInstall(first) {
		t.Fatal("new review did not retain an unbound reserved identity", first)
	}
	managementApply(t, prepared)
	state := j.state(t)
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 2 || state.Installations[0].InstallationID != first.InstallationID {
		t.Fatal("registry application did not share the reserved installation", state)
	}
	requirements, lock := j.intent(t)
	if requirements.Dependencies["docs"].Registry.Distribution != "owner/rollout" || len(lock.Packages) != 1 || lock.Packages[0].Source.Registry.ReleaseSequence != 1 {
		t.Fatal("signed registry identity was not retained", requirements, lock)
	}
	adapter := dependencyInstaller{app: backend.app}
	recorded, err := adapter.recordedDependencyRelease(first)
	if err != nil || recorded == nil || recorded.ReleaseSequence != 1 {
		t.Fatal("committed reserved identity did not use its actual recorded release", recorded, err)
	}
	changed := *first.Package
	changed.Source = first.Package.Source
	registry := *changed.Source.Registry
	registry.ReleaseSequence = 2
	changed.Source.Registry = &registry
	first.Package = &changed
	recorded, err = adapter.recordedDependencyRelease(first)
	if err != nil || recorded != nil {
		t.Fatal("a different release borrowed installed eligibility", recorded, err)
	}
}

func TestManagementRegistryMissingBoundAuthorityStaysStale(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	adapter := dependencyInstaller{app: j.cli.app}
	original := dependencyAction{Action: environment.ActionInstall, InstallationID: "reserved-future-installation", Package: &environment.LockedPackage{Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{DistributionID: "owner/rollout"}}}, Authority: &domain.EnvironmentAuthority{EnvironmentID: environment.EnvironmentUserDefault, Alias: "docs", ClientID: "codex", SourceKey: "directory:owner/rollout", ExpectedAction: "install"}}
	if recorded, err := adapter.recordedDependencyRelease(original); err != nil || recorded != nil {
		t.Fatal("unbound first installation was treated as existing", recorded, err)
	}
	cases := []struct {
		name   string
		mutate func(*dependencyAction)
	}{
		{"bound installation", func(a *dependencyAction) { a.Authority.InstallationID = a.InstallationID }},
		{"bound receipt", func(a *dependencyAction) { a.Authority.ReceiptID = "existing-receipt" }},
		{"bound client", func(a *dependencyAction) { a.Authority.ClientBindingID = "existing-binding" }},
		{"observed binding", func(a *dependencyAction) { a.BindingID = "existing-binding" }},
		{"missing authority", func(a *dependencyAction) { a.Authority = nil }},
		{"repair", func(a *dependencyAction) { a.Action = "repair"; a.Authority.ExpectedAction = "repair" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			action := original
			authority := *original.Authority
			action.Authority = &authority
			test.mutate(&action)
			if _, err := adapter.recordedDependencyRelease(action); !errors.Is(err, installer.ErrPlanChanged) {
				t.Fatal("missing existing authority was admitted as fresh", err)
			}
		})
	}
}
