package agentpluginscli

import (
	"errors"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementManifestOnlyFailureHasFreshRecovery(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	prepared := managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha))
	manifest, lock := j.cli.app.dependencyPaths()
	publish := publishEnvironmentFile
	t.Cleanup(func() { publishEnvironmentFile = publish })
	publishEnvironmentFile = func(path string, body []byte) error {
		if path == lock {
			return errors.New("injected publication interruption")
		}
		return publish(path, body)
	}
	out, err := prepared.Apply(t.Context(), nil)
	if err == nil || out.State != management.Partial || !out.Result.Mutated || out.RecoverySettled {
		t.Fatal("manifest-first failure was not partial", out, err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal("manifest effect not observed", err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("failed lock publication succeeded", err)
	}
	if j.runner.calls != 0 {
		t.Fatal("native effect preceded intent publication")
	}
	proof := backend.proveManagementRecovery(t.Context(), backend.app, management.Outcome{State: management.Succeeded})
	if proof.RecoverySettled || proof.State != management.Partial {
		t.Fatal("no-journal divergence was cleared", proof)
	}
	publishEnvironmentFile = publish
	recovery := managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover})
	if len(recovery.Review().Actions) == 0 {
		t.Fatal("fresh recovery omitted the required native install")
	}
	completed := managementApply(t, recovery)
	if !completed.RecoverySettled {
		t.Fatal("coherent fresh recovery did not settle", completed)
	}
	requirements, locked := j.intent(t)
	if len(requirements.Dependencies) != 1 || len(locked.Packages) != 1 || len(j.state(t).EnvironmentReceipts) != 1 {
		t.Fatal("fresh recovery did not reconcile complete intent")
	}
}
