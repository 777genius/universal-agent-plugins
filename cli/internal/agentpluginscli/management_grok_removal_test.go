package agentpluginscli

import (
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real registry, package projection, owned receipts and HTTP acceptance are
// exercised with the repository's existing controlled Grok command boundary.
func TestManagementGrokMissingRuntimeRequiresExplicitRemovalReview(t *testing.T) {
	for _, action := range []management.ChangeAction{management.Remove, management.Recover} {
		t.Run(string(action), func(t *testing.T) {
			repo := dependencyRepo(t)
			sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
			fixture, _, runner := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientGrok, executable: true, activation: "active"})
			backend := managementJourneyBackend(t, savedJourney{cli: fixture})
			managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "github:acme/demo@" + sha, TargetIds: []string{"profile-grok"}}))
			if runner.grokPath == "" {
				t.Fatal("fixture did not register the real projected package")
			}
			missing := managementGrokWithoutRuntime(t, &fixture, backend)
			request := management.ChangeRequest{Action: action, Aliases: []string{"demo"}, TargetIds: []string{"profile-grok"}}
			var prior management.Operation
			if action == management.Recover {
				if _, _, err := fixture.execute(false, "remove", "demo", "--target", "grok", "--format", "json"); err == nil {
					t.Fatal("legacy Grok cleanup invented missing-runtime confirmation")
				}
				prior = managementHistoricalRemoval(t, missing)
				request = management.ChangeRequest{Action: action, OperationId: prior.OperationId}
			}
			manifest, lock := fixture.app.dependencyPaths()
			beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
			before, err := fixture.app.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			mutations := runner.mutations
			server := managementRemovalServer(t, missing)
			var blocked management.Plan
			managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &blocked)
			assertManagementGrokRemovalReview(t, blocked, false)
			var failure management.Error
			managementRemovalHTTP(t, server, "POST", "/api/v1/plans/"+blocked.PlanId+"/apply", management.ApplyRequest{RequestId: "blocked-grok-missing-runtime"}, 409, &failure)
			after, err := fixture.app.StateStore.Load()
			if err != nil || failure.Code != "PLAN_STALE" || beforeManifest != dependencyRead(t, manifest) || beforeLock != dependencyRead(t, lock) || !reflect.DeepEqual(before, after) || mutations != runner.mutations {
				t.Fatal("blocked Grok removal changed intent, receipts or native state", failure, err)
			}
			// Model the explicit owner action outside UAP before supplying its
			// attestation. No native CLI is rediscovered or silently invoked.
			runner.grokPath = ""
			request.ExternalUninstalled = true
			var review management.Plan
			managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &review)
			assertManagementGrokRemovalReview(t, review, true)
			var accepted management.Operation
			managementRemovalHTTP(t, server, "POST", "/api/v1/plans/"+review.PlanId+"/apply", management.ApplyRequest{RequestId: "attested-grok-missing-runtime"}, 202, &accepted)
			completed := managementAwaitRemoval(t, server, accepted.OperationId)
			state, err := fixture.app.StateStore.Load()
			if err != nil || completed.State != management.Succeeded || len(state.EnvironmentReceipts) != 0 || mutations != runner.mutations {
				t.Fatal("explicit Grok cleanup did not converge without a native executable", completed, err)
			}
			if action == management.Recover {
				var historical management.Operation
				managementRemovalHTTP(t, server, "GET", "/api/v1/operations/"+prior.OperationId, nil, 200, &historical)
				if historical.ResolvedBy != completed.OperationId || historical.State != management.Partial {
					t.Fatal("fresh Grok recovery did not preserve and resolve the old observation", historical)
				}
			}
		})
	}
}

func managementGrokWithoutRuntime(t *testing.T, fixture *cliFixture, backend *ManagementBackend) *ManagementBackend {
	t.Helper()
	detected, err := fixture.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for i := range detected {
		if err = os.Remove(detected[i].ExecutablePath); err != nil {
			t.Fatal(err)
		}
		detected[i].ExecutablePath = ""
	}
	fixture.app.Detector = staticDetector{clients: detected}
	config := backend.config
	config.Targets = append([]management.StartupTarget(nil), config.Targets...)
	for i := range config.Targets {
		config.Targets[i].Executable = ""
	}
	missing, err := NewManagementBackend(fixture.app, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(missing.Close)
	return missing
}

func assertManagementGrokRemovalReview(t *testing.T, plan management.Plan, attested bool) {
	t.Helper()
	if plan.Confirmable != attested {
		t.Fatal("Grok review has incorrect confirmability", plan)
	}
	diagnostics, code := plan.Conflicts, "EXTERNAL_UNINSTALL_REQUIRED"
	if attested {
		diagnostics, code = plan.Warnings, "EXTERNAL_UNINSTALL_ATTESTED"
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Alias == "demo" && diagnostic.TargetId == "profile-grok" {
			return
		}
	}
	t.Fatal("Grok review lost its exact required confirmation binding", plan)
}
