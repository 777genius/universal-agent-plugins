package environment

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestProjectObservationCannotAdoptAnotherEnvironmentReceipt(t *testing.T) {
	receipt := domain.EnvironmentReceipt{ReceiptID: "project-receipt", EnvironmentID: EnvironmentProjectDefault,
		Alias: "tools", ClientID: "codex", InstallationID: "installed", ClientBindingID: "binding", SourceKey: "github:acme/tools:", PackageDigest: "digest"}
	view := installer.Inspection{EnvironmentReceipts: []domain.EnvironmentReceipt{receipt}, Installations: []installer.InspectedInstallation{{
		InstallationID: receipt.InstallationID, SourceKey: receipt.SourceKey,
		Bindings: []installer.InspectedBinding{{BindingID: receipt.ClientBindingID, ClientID: receipt.ClientID,
			TreeDigest: receipt.PackageDigest, Materialization: "materialized", LifecycleConverged: true}},
	}}}
	user, err := Observe(view)
	if err != nil || len(user) != 1 || user[0].Managed {
		t.Fatalf("user observation adopted project ownership: %+v, %v", user, err)
	}
	project, err := ObserveForEnvironment(view, EnvironmentProjectDefault)
	if err != nil || len(project) != 1 || !project[0].Managed || project[0].Receipt.EnvironmentID != EnvironmentProjectDefault {
		t.Fatalf("project ownership lost: %+v, %v", project, err)
	}
	requirements := Requirements{SchemaVersion: ManifestSchemaVersion, Dependencies: map[string]Requirement{}, Targets: map[string][]string{}}
	lock := Lock{LockVersion: LockVersion, ManifestDigest: mustDigest(t, requirements)}
	plan, err := (Coordinator{EnvironmentID: EnvironmentProjectDefault}).PrepareSync(requirements, lock, project)
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionRemove ||
		plan.Bindings[0].Authority.EnvironmentID != EnvironmentProjectDefault {
		t.Fatalf("removal borrowed user authority: %+v, %v", plan, err)
	}
}
