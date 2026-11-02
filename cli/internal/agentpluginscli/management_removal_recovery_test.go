package agentpluginscli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementHistoricalUnattestedRemovalRequiresFreshRecovery(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha)))
	if _, _, err := j.cli.execute(false, "remove", "demo", "--format", "json"); err == nil {
		t.Fatal("legacy removal unexpectedly completed without attestation")
	}
	requirements, _ := j.intent(t)
	if len(requirements.Dependencies) != 0 || len(j.state(t).EnvironmentReceipts) != 1 {
		t.Fatal("fixture did not leave published removal intent and a live native binding")
	}
	prior := managementHistoricalRemoval(t, backend)
	server := managementRemovalServer(t, backend)
	request := management.ChangeRequest{Action: management.Sync, ExternalUninstalled: true}
	var syncPlan management.Plan
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &syncPlan)
	var failure management.Error
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans/"+syncPlan.PlanId+"/apply", management.ApplyRequest{RequestId: "cannot-bypass-pending-0001"}, 409, &failure)
	if failure.Code != "RECOVERY_REQUIRED" {
		t.Fatal("attestation bypassed the durable pending gate", failure)
	}
	request = management.ChangeRequest{Action: management.Recover, OperationId: prior.OperationId}
	var blocked management.Plan
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &blocked)
	assertManagementRemovalConflict(t, blocked)
	request.ExternalUninstalled = true
	var recovery management.Plan
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &recovery)
	assertManagementRemovalAttested(t, recovery)
	var accepted management.Operation
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans/"+recovery.PlanId+"/apply", management.ApplyRequest{RequestId: "explicit-native-recovery-0001"}, 202, &accepted)
	completed := managementAwaitRemoval(t, server, accepted.OperationId)
	if completed.State != management.Succeeded || completed.Result == nil || len(j.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("explicit fresh recovery did not converge", completed)
	}
	var previous management.Operation
	managementRemovalHTTP(t, server, "GET", "/api/v1/operations/"+prior.OperationId, nil, 200, &previous)
	if previous.State != management.Partial || previous.ResolvedBy != completed.OperationId || !reflect.DeepEqual(previous.Result, prior.Result) {
		t.Fatal("recovery rewrote historical truth or failed to link it", previous)
	}
	var view management.Environment
	managementRemovalHTTP(t, server, "GET", "/api/v1/environment", nil, 200, &view)
	if len(view.PendingOperations) != 0 {
		t.Fatal("coherent explicit recovery remained fenced", view.PendingOperations)
	}
}

// This fixture is a prior-version acceptance record, not executable authority.
// Native divergence above is produced by the unchanged CLI and real installer.
func managementHistoricalRemoval(t *testing.T, backend *ManagementBackend) management.Operation {
	t.Helper()
	now := time.Now().UTC()
	operation := management.Operation{
		EnvironmentId: backend.EnvironmentID(), ContextId: "historical-captured-context", Action: management.Remove,
		OperationId: "historical-native-removal", PlanId: "historical-removal-plan", RequestId: "historical-native-request-0001",
		State: management.Partial, Phase: "applying", CreatedAt: now, UpdatedAt: now,
		Result: &management.OperationResult{Outcome: "incomplete", Mutated: true, Items: []management.OperationItem{}, NextActions: []management.NextAction{}, Message: "Native removal requires explicit external confirmation"},
	}
	hash := func(value string) string {
		digest := sha256.Sum256([]byte(value))
		return hex.EncodeToString(digest[:])
	}
	body, err := json.Marshal(map[string]any{
		"version": 1, "environmentId": backend.EnvironmentID(), "intentRoot": backend.config.DependenciesRoot,
		"contextId": operation.ContextId, "fingerprint": hash(operation.PlanId), "installerIds": []string{}, "operation": operation,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(backend.config.StateRoot, "management", hash(backend.EnvironmentID()))
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, hash(operation.RequestId)+".json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	return operation
}

func managementRemovalServer(t *testing.T, backend *ManagementBackend) *management.Server {
	t.Helper()
	server, err := management.Start(t.Context(), management.Options{Backend: backend, ContextID: backend.ContextID(), EnvironmentID: backend.EnvironmentID(), IntentRoot: backend.config.DependenciesRoot, StateRoot: backend.config.StateRoot, EngineVersion: backend.app.Version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return server
}

func managementRemovalHTTP(t *testing.T, server *management.Server, method, path string, input any, want int, output any) {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := server.Bootstrap()
	request, err := http.NewRequestWithContext(t.Context(), method, bootstrap.Url+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+bootstrap.Token)
	request.Header.Set("Origin", bootstrap.Origin)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 15 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if err = json.NewDecoder(response.Body).Decode(output); err != nil {
		t.Fatal(response.StatusCode, err)
	}
	if response.StatusCode != want {
		t.Fatalf("HTTP %s %s: status %d, wanted %d, body %+v", method, path, response.StatusCode, want, output)
	}
}

func managementAwaitRemoval(t *testing.T, server *management.Server, id string) management.Operation {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var operation management.Operation
		managementRemovalHTTP(t, server, "GET", "/api/v1/operations/"+id, nil, 200, &operation)
		if operation.State != management.Queued && operation.State != management.Running {
			return operation
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation did not settle")
	return management.Operation{}
}
