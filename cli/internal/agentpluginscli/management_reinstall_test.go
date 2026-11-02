package agentpluginscli

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Real Management/source/installer transactions with controlled native/scanner
// seams: removing the last target must leave its retained identity reusable.
func TestManagementSkillReinstallPreservesRetainedInstallation(t *testing.T) {
	for _, target := range []string{"codex", "claude"} {
		t.Run(target, func(t *testing.T) {
			backend, j, source, changeContent := managementMutableWebsite(t)
			prepare := func(target string) management.Prepared {
				t.Helper()
				catalog, err := backend.SkillsCatalog(t.Context(), management.SourceRequest{Source: source})
				if err != nil || len(catalog.Candidates) != 1 {
					t.Fatal(catalog, err)
				}
				return managementPrepare(t, backend, management.ChangeRequest{Action: management.InstallSkills,
					CatalogId: catalog.CatalogId, CandidateIds: []string{catalog.Candidates[0].Id}, TargetIds: []string{"profile-" + target}})
			}
			managementApply(t, prepare("codex"))
			changeContent("---\nname: pdf\ndescription: Read PDFs\n---\nUpdated reviewed content.\n")
			managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Update, Aliases: []string{"pdf"}}))
			managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Remove,
				Aliases: []string{"pdf"}, TargetIds: []string{"profile-codex"}, ExternalUninstalled: true}))
			removed := j.state(t)
			if len(removed.Installations) != 1 || len(removed.EnvironmentReceipts) != 0 ||
				len(removed.Installations[0].Clients) != 0 || !removed.Installations[0].DataRetained || len(removed.Installations[0].DataReceipts) == 0 {
				t.Fatal("last-binding removal did not retain only installation data", removed)
			}
			retained := removed.Installations[0]
			dataFiles := map[string]string{}
			for id, receipt := range retained.DataReceipts {
				path := filepath.Join(receipt.Locator, "retained-user-data.txt")
				dependencyWrite(t, path, "keep this data")
				dataFiles[id] = path
			}
			before := dependencyRead(t, j.cli.store.Path)
			prepared := prepare(target)
			actions := prepared.(*managementPrepared).request.prepared
			if len(actions) != 1 || actions[0].InstallationID != retained.InstallationID || !unboundDependencyInstall(actions[0]) {
				t.Fatal("review did not reuse the empty installation without borrowing live authority", actions)
			}
			if dependencyRead(t, j.cli.store.Path) != before {
				t.Fatal("preparation changed retained state")
			}
			managementApply(t, prepared)
			after := j.state(t)
			if len(after.Installations) != 1 || after.Installations[0].InstallationID != retained.InstallationID ||
				after.Installations[0].Source.SourceBindingID != retained.Source.SourceBindingID ||
				len(after.EnvironmentReceipts) != 1 || after.EnvironmentReceipts[0].ClientID != target {
				t.Fatal("reinstall duplicated source identity or lost the new owner receipt", after)
			}
			binding := savedJourneyBinding(t, after, target)
			if binding.TargetLocator != prepared.Review().Actions[0].DestinationPath {
				t.Fatal("apply changed the reviewed physical destination")
			}
			for id, path := range dataFiles {
				receipt, ok := after.Installations[0].DataReceipts[id]
				if !ok || receipt.Locator != retained.DataReceipts[id].Locator || receipt.OwnershipDigest != retained.DataReceipts[id].OwnershipDigest ||
					dependencyRead(t, path) != "keep this data" {
					t.Fatal("reinstall lost retained PLUGIN_DATA ownership or bytes", id, receipt)
				}
			}
		})
	}
}

func TestSkillReservationRejectsRetainedIdentityChanges(t *testing.T) {
	for _, test := range []struct {
		name  string
		prior []dependencyAction
		ids   []string
	}{
		{name: "ambiguous retained source", ids: []string{"retained-a", "retained-b"}},
		{name: "retained identity replaced reviewed reservation", prior: []dependencyAction{{Alias: "pdf", InstallationID: "reviewed-id"}}, ids: []string{"new-retained-id"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := installer.Inspection{}
			for _, id := range test.ids {
				view.Installations = append(view.Installations, installer.InspectedInstallation{InstallationID: id, SourceKey: "https-skill:https://publisher.test:pdf"})
			}
			actions := []dependencyAction{{Alias: "pdf", Action: environment.ActionInstall,
				Package: &environment.LockedPackage{SourceKey: "https-skill:https://publisher.test:pdf"}}}
			if err := reserveSkillBindingIDs(view, actions, test.prior); !errors.Is(err, installer.ErrPlanChanged) || actions[0].InstallationID != "" {
				t.Fatal("changed or ambiguous retained identity was reserved", actions, err)
			}
		})
	}
}

func TestSkillReservationDoesNotAdoptForeignBindingsOrOtherSources(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		binding []installer.InspectedBinding
		pkg     *environment.LockedPackage
	}{
		{name: "other source", source: "https-skill:https://other.test:pdf", pkg: &environment.LockedPackage{SourceKey: "https-skill:https://publisher.test:pdf"}},
		{name: "foreign live binding", source: "https-skill:https://publisher.test:pdf", binding: []installer.InspectedBinding{{ClientID: "claude", BindingID: "foreign"}}, pkg: &environment.LockedPackage{SourceKey: "https-skill:https://publisher.test:pdf"}},
		{name: "missing source", pkg: &environment.LockedPackage{}},
		{name: "missing package"},
	} {
		t.Run(test.name, func(t *testing.T) {
			view := installer.Inspection{Installations: []installer.InspectedInstallation{{InstallationID: "unrelated", SourceKey: test.source, Bindings: test.binding}}}
			authority := domain.EnvironmentAuthority{EnvironmentID: environment.EnvironmentUserDefault, Alias: "pdf", ClientID: "codex", ExpectedAction: "install"}
			before := authority
			actions := []dependencyAction{{Alias: "pdf", Action: environment.ActionInstall, Package: test.pkg, Authority: &authority}}
			if err := reserveSkillBindingIDs(view, actions, nil); err != nil {
				t.Fatal(err)
			}
			if actions[0].InstallationID == "" || actions[0].InstallationID == "unrelated" || !reflect.DeepEqual(*actions[0].Authority, before) || !unboundDependencyInstall(actions[0]) {
				t.Fatal("unrelated state granted installation authority", actions)
			}
		})
	}
}
