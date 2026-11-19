package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// clientFailingCLIStager fails staging for selected clients only.
type clientFailingCLIStager struct {
	ports.PackageStager
	fail map[domain.ClientID]error
}

func (stager clientFailingCLIStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if err := stager.fail[plan.ClientID]; err != nil {
		return domain.StagedDelivery{}, err
	}
	return stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
}

func permissionDenied(client domain.ClientID) error {
	return fmt.Errorf("create client target root: %w", &fs.PathError{Op: "mkdir", Path: "/project/." + string(client) + "/skills", Err: syscall.EACCES})
}

// unavailable is a cause with a plain explanation; its path must stay out of
// the report.
func unavailable(client domain.ClientID) error {
	return fmt.Errorf("probe /project/.%s: %w", client, installer.ErrTargetFactsUnavailable)
}

var isolationClients = []domain.ClientID{domain.ClientCodex, domain.ClientCursor, domain.ClientKiro}

func newIsolationFixture(t *testing.T, fail map[domain.ClientID]error) cliFixture {
	t.Helper()
	clients := make([]domain.DetectedClient, len(isolationClients))
	for index, client := range isolationClients {
		clients[index] = fixtureClient(t, client)
	}
	fixture := newCLIFixture(t, clients)
	if len(fail) > 0 {
		fixture.app.Lifecycle.Stager = clientFailingCLIStager{PackageStager: fixture.app.Lifecycle.Stager, fail: fail}
	}
	return fixture
}

// installedProjection is what an installation means for a person, without
// ids, paths and timestamps that differ between runs.
func installedProjection(t *testing.T, fixture cliFixture) []string {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, installation := range state.Installations {
		out = append(out, fmt.Sprintf("installation %s %s %s", installation.DeclaredName, installation.OriginMode, installation.Source.TreeDigest))
		for _, client := range installation.Clients {
			kinds := make([]string, 0, len(client.NativeObjects))
			for _, object := range client.NativeObjects {
				kinds = append(kinds, object.Kind+":"+object.ProtectionClass)
			}
			receipts := make([]string, 0, len(client.Receipts))
			for _, receipt := range client.Receipts {
				receipts = append(receipts, string(receipt.MutationType)+":"+string(receipt.Phase))
			}
			out = append(out, fmt.Sprintf("client %s %s %s %s %s %s %v %v %v", client.ClientID, client.Scope, client.Materialization,
				client.Activation, client.Authentication, client.Verification, client.PackageRevision != nil && client.PackageRevision.TreeDigest == installation.Source.TreeDigest, kinds, receipts))
		}
	}
	sort.Strings(out)
	return out
}

func targetsExcept(skip domain.ClientID) string {
	var out []string
	for _, client := range isolationClients {
		if client != skip {
			out = append(out, string(client))
		}
	}
	return strings.Join(out, ",")
}

// One agent that cannot be prepared must not stop the others. The others end
// exactly as if only they had been selected, the failed agent is reported with
// a plain cause and a retry step, and that retry installs it.
func TestGroupAddInstallsPeersWhenOneAgentCannotBePrepared(t *testing.T) {
	t.Parallel()
	all := strings.Join([]string{string(domain.ClientCodex), string(domain.ClientCursor), string(domain.ClientKiro)}, ",")
	for _, failing := range isolationClients {
		t.Run(string(failing), func(t *testing.T) {
			t.Parallel()
			plugin := writeCLIPlugin(t)
			fixture := newIsolationFixture(t, map[domain.ClientID]error{failing: unavailable(failing)})
			stdout, _, err := fixture.execute(false, "add", plugin, "--target", all)
			if err == nil || err.Error() != "1 of 3 client installations failed; see results above" {
				t.Fatalf("partial error = %v\n%s", err, stdout)
			}
			label := reviewClientName(failing)
			if !strings.Contains(stdout, "installation finished with issues") ||
				!strings.Contains(stdout, "Retry: run the same command with --target "+publicCLITargetName(failing)) ||
				strings.Contains(stdout, "/project/.") {
				t.Fatalf("partial report for %s:\n%s", label, stdout)
			}

			alone := newIsolationFixture(t, nil)
			if _, _, err := alone.execute(false, "add", plugin, "--target", targetsExcept(failing)); err != nil {
				t.Fatalf("subset install = %v", err)
			}
			got, want := installedProjection(t, fixture), installedProjection(t, alone)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("peers differ from installing only them:\n got %v\nwant %v", got, want)
			}

			delete(fixture.app.Lifecycle.Stager.(clientFailingCLIStager).fail, failing)
			if _, _, err := fixture.execute(false, "add", plugin, "--target", publicCLITargetName(failing)); err != nil {
				t.Fatalf("retry of %s = %v", failing, err)
			}
			everything := newIsolationFixture(t, nil)
			if _, _, err := everything.execute(false, "add", plugin, "--target", all); err != nil {
				t.Fatalf("full install = %v", err)
			}
			if got, want := installedProjection(t, fixture), installedProjection(t, everything); !reflect.DeepEqual(got, want) {
				t.Fatalf("retry did not complete the selection:\n got %v\nwant %v", got, want)
			}
		})
	}
}

// A cause without a plain explanation is shown precisely in the row and in
// the command's error, as the whole-group refusal showed it before.
func TestGroupAddNamesUnexplainedCausesPrecisely(t *testing.T) {
	t.Parallel()
	cause := errors.New("stage cursor: the fixture broke in an unusual way")
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCursor: cause})
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	want := "1 of 3 client installations failed; see results above; cursor: " + cause.Error()
	if err == nil || err.Error() != want || !strings.Contains(stdout, "  "+cause.Error()+"\n") {
		t.Fatalf("error = %v\nwant %s\n%s", err, want, stdout)
	}
}

// When every agent fails on its own, nothing changed and the result is the one
// the whole selection always reported: the first agent's cause.
func TestGroupAddEveryAgentFailingKeepsTheWholeSelectionResult(t *testing.T) {
	t.Parallel()
	// Unclassified causes keep this test about isolation, not about wording.
	fail := map[domain.ClientID]error{}
	for _, client := range isolationClients {
		fail[client] = fmt.Errorf("stage %s: broken fixture", client)
	}
	fixture := newIsolationFixture(t, fail)
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	want := fmt.Sprintf("group preparation failed before client changes (selected targets: [codex cursor kiro]): %v; no selected client was changed", fail[domain.ClientCodex])
	if err == nil || err.Error() != want {
		t.Fatalf("all-fail error = %v\nwant %s", err, want)
	}
	if stdout != "No selected client was changed: package preparation failed. See the error below, then retry the same command.\n" {
		t.Fatalf("all-fail output = %q", stdout)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("all-fail changed state: %v", got)
	}
}

// Causes that concern the whole operation still stop every agent, even when
// they surface while one agent is being prepared.
func TestGroupAddGroupWideCausesStillStopEveryAgent(t *testing.T) {
	t.Parallel()
	for name, cause := range map[string]error{
		"recovery":      installer.ErrRecoveryRequired,
		"plan changed":  installer.ErrPlanChanged,
		"source change": environment.ErrSourceChange,
		"security":      installer.ErrAssessmentRejected,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCursor: fmt.Errorf("stage cursor: %w", cause)})
			_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
			if err == nil || !errors.Is(err, cause) || !strings.Contains(err.Error(), "no selected client was changed") {
				t.Fatalf("group-wide cause = %v", err)
			}
			if got := installedProjection(t, fixture); len(got) != 0 {
				t.Fatalf("group-wide cause changed state: %v", got)
			}
		})
	}
}

func TestFailureStaysWithTargetClassifiesEveryCause(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err   error
		local bool
	}{
		"permission":         {permissionDenied(domain.ClientClaude), true},
		"foreign":            {fmt.Errorf("x: %w", nativeconfig.ErrCollision), true},
		"target unavailable": {fmt.Errorf("x: %w", installer.ErrTargetFactsUnavailable), true},
		"unsupported":        {fmt.Errorf("x: %w", installer.ErrIncomplete), true},
		"unclassified":       {errors.New("something odd"), true},
		"cleanup":            {fmt.Errorf("x: %w", &domain.UnclosedClientProbe{}), false},
		"recovery":           {fmt.Errorf("x: %w", installer.ErrRecoveryRequired), false},
		"security":           {fmt.Errorf("x: %w", installer.ErrAssessmentRejected), false},
		"managed changed":    {fmt.Errorf("x: %w", installer.ErrManagedSkillChanged), false},
		"update required":    {fmt.Errorf("x: %w", installer.ErrUpdateRequired), false},
		"plan changed":       {fmt.Errorf("x: %w", installer.ErrPlanChanged), false},
		"source changed":     {fmt.Errorf("x: %w", environment.ErrSourceChange), false},
		"timeout":            {fmt.Errorf("x: %w", context.DeadlineExceeded), false},
		"canceled":           {fmt.Errorf("x: %w", context.Canceled), false},
	}
	for name, tc := range cases {
		if got := failureStaysWithTarget(tc.err); got != tc.local {
			t.Errorf("%s: failureStaysWithTarget = %v, want %v", name, got, tc.local)
		}
	}
}

func TestTargetLocalGroupFailureRequiresAnUnchangedAttributedGroup(t *testing.T) {
	t.Parallel()
	attributed := &usecase.GroupTargetError{Clients: []domain.ClientID{domain.ClientKiro}, Err: permissionDenied(domain.ClientKiro)}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := map[string]struct {
		ctx    context.Context
		result usecase.GroupResult
		err    error
		want   bool
	}{
		"preparation":      {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePreparationFailed}, attributed, true},
		"preflight":        {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePlanned}, fmt.Errorf("wrapped: %w", attributed), true},
		"mutated":          {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePreparationFailed, Mutated: true}, attributed, false},
		"after commit":     {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhaseManagedCommitted}, attributed, false},
		"not attributed":   {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePreparationFailed}, permissionDenied(domain.ClientKiro), false},
		"canceled context": {canceled, usecase.GroupResult{Phase: usecase.GroupPhasePreparationFailed}, attributed, false},
		"group-wide cause": {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePlanned}, &usecase.GroupTargetError{Clients: []domain.ClientID{domain.ClientKiro}, Err: installer.ErrRecoveryRequired}, false},
		"success":          {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhaseCompleted}, nil, false},
	}
	for name, tc := range cases {
		got := targetLocalGroupFailure(tc.ctx, tc.result, tc.err)
		if got[domain.ClientKiro] != tc.want || len(got) > 1 {
			t.Errorf("%s: failed = %v, want kiro=%v", name, got, tc.want)
		}
	}
}

// JSON keeps its existing fields and statuses; the agent left out is an
// ordinary failed row in selection order.
func TestGroupAddPartialJSONUsesExistingContract(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	decode := func(t *testing.T, stdout string) map[string]any {
		t.Helper()
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, stdout)
		}
		return envelope
	}
	okOut, _, err := newIsolationFixture(t, nil).execute(false, "add", plugin, "--target", "codex,cursor,kiro", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCursor: unavailable(domain.ClientCursor)}).
		execute(false, "add", plugin, "--target", "codex,cursor,kiro", "--format", "json")
	if err == nil {
		t.Fatal("partial JSON run succeeded")
	}
	ok, partial := decode(t, okOut), decode(t, stdout)
	if partial["result"] != "failure" {
		t.Fatalf("result = %v", partial["result"])
	}
	okData, data := ok["data"].(map[string]any), partial["data"].(map[string]any)
	for key := range data {
		if _, known := okData[key]; !known {
			t.Fatalf("new top-level field %q", key)
		}
	}
	if data["status"] != string(usecase.GroupPhaseExternalPartialFailure) || data["failed"] != float64(1) || data["succeeded"] != float64(2) {
		t.Fatalf("partial summary = %v %v %v", data["status"], data["failed"], data["succeeded"])
	}
	allowed := map[string]bool{"target": true, "status": true, "next_action": true, "error": true, "retry_command": true, "output": true}
	var order, okOrder []string
	for _, raw := range okData["targets"].([]any) {
		okOrder = append(okOrder, raw.(map[string]any)["target"].(string))
	}
	for _, raw := range data["targets"].([]any) {
		row := raw.(map[string]any)
		for key := range row {
			if !allowed[key] {
				t.Fatalf("new target field %q", key)
			}
		}
		order = append(order, row["target"].(string))
		if row["target"] != "cursor" {
			continue
		}
		failure, _ := row["error"].(map[string]any)
		if row["status"] != string(usecase.GroupTargetExternalFailed) || failure["stage"] != "preparation" || strings.Contains(fmt.Sprint(failure["message"]), "/project/") {
			t.Fatalf("failed row = %v", row)
		}
		if outcome := data["target_outcomes"].(map[string]any)["cursor"].(map[string]any)["outcome"]; outcome != "failed" {
			t.Fatalf("cursor proof outcome = %v", outcome)
		}
	}
	if !reflect.DeepEqual(order, okOrder) {
		t.Fatalf("row order = %v, want %v", order, okOrder)
	}
}

// When the agents still selected are then refused for a reason that concerns
// them all, nothing changed: the whole selection is reported with that reason.
func TestGroupAddLaterGroupWideRefusalReportsTheWholeSelection(t *testing.T) {
	t.Parallel()
	recovery := fmt.Errorf("stage cursor: %w", installer.ErrRecoveryRequired)
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCodex: unavailable(domain.ClientCodex), domain.ClientCursor: recovery})
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	want := fmt.Sprintf("group preparation failed before client changes (selected targets: [codex cursor kiro]): %v; no selected client was changed", recovery)
	if err == nil || err.Error() != want || !errors.Is(err, installer.ErrRecoveryRequired) {
		t.Fatalf("later refusal = %v\nwant %s", err, want)
	}
	if !strings.HasPrefix(stdout, "No selected client was changed") {
		t.Fatalf("later refusal output = %q", stdout)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("later refusal changed state: %v", got)
	}
}

// Unconfirmed process cleanup stops every retry route, including the retry
// line of an agent that was left out earlier.
func TestExcludedAgentGetsNoRetryWhenCleanupIsUnconfirmed(t *testing.T) {
	failure := describeSyncFailure(unavailable(domain.ClientCursor))
	excluded := &usecase.GroupTargetFailure{Stage: "preparation", Message: failure.Summary + " " + failure.Next}
	activation := &usecase.GroupTargetFailure{Stage: "activation", Message: "activation failed"}
	combined := addMultiResult{Plugin: "demo", Status: string(usecase.GroupPhaseExternalPartialFailure), Targets: []addTargetResult{
		{Target: "codex", Status: string(usecase.GroupTargetExternalFailed), Error: activation,
			Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed, Failure: activation}}},
		{Target: "cursor", Status: string(usecase.GroupTargetExternalFailed), Error: excluded, failure: failure,
			Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed, Failure: excluded}}},
	}}
	cmd, out, _ := retryTestCommand(t)
	app := App{InstallLogDir: filepath.Join(t.TempDir(), "logs")}
	opts := &options{format: "human"}
	probe := &domain.UnclosedClientProbe{Operation: "native operation", Err: context.DeadlineExceeded}
	_ = app.recordGroupOutcome(cmd, opts, &combined, probe)
	if err := renderAddMultiResult(cmd, opts, combined, domain.PackageEnvelope{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Retry") || !strings.Contains(out.String(), "processes have stopped") {
		t.Fatalf("cleanup offered a retry: %s", out.String())
	}
}

// A cause shown as is must not carry terminal control sequences.
func TestExcludedDryRunCauseIsTerminalSafe(t *testing.T) {
	failure := describeSyncFailure(errors.New("bad \x1b]0;title\a \x1b[2J cause"))
	row := addTargetResult{Target: "cursor", Status: string(usecase.GroupTargetExternalFailed), failure: failure,
		Error: &usecase.GroupTargetFailure{Stage: "preparation", Message: failure.Detail}}
	var out bytes.Buffer
	if err := renderAddMultiDryRun(&out, addMultiResult{Plugin: "demo", DryRun: true, Targets: []addTargetResult{row}}); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), 0x1b) || !strings.Contains(out.String(), "cursor: cannot be installed") {
		t.Fatalf("unsafe dry-run output: %q", out.String())
	}
}

// VS Code is frozen and shares its delivery with Copilot, so a selection with
// it keeps the whole-selection behaviour.
func TestGroupIsolationIsOffWhenVSCodeIsSelected(t *testing.T) {
	t.Parallel()
	with := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCopilot}, {ClientID: domain.ClientVSCode}}, nil, "op")
	without := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCopilot}, {ClientID: domain.ClientCursor}}, nil, "op")
	if with.enabled() || !without.enabled() {
		t.Fatalf("enabled = %v/%v", with.enabled(), without.enabled())
	}
}

// flaggingStager fails one client and records that it did.
type flaggingStager struct {
	ports.PackageStager
	client domain.ClientID
	err    error
	failed *bool
}

func (stager flaggingStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if plan.ClientID == stager.client {
		*stager.failed = true
		return domain.StagedDelivery{}, stager.err
	}
	return stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
}

// lockAfterFailure refuses the mutation lock once the stager failed.
type lockAfterFailure struct {
	ports.MutationLock
	failed *bool
	err    error
}

func (lock lockAfterFailure) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	if *lock.failed {
		return nil, lock.err
	}
	return lock.MutationLock.Acquire(ctx)
}

// A later attempt refused before it planned anything still reports the whole
// selection, with that attempt's cause.
func TestGroupAddLaterRefusalBeforePlanningReportsTheWholeSelection(t *testing.T) {
	t.Parallel()
	failed := false
	busy := errors.New("another installation holds the lock")
	fixture := newIsolationFixture(t, nil)
	fixture.app.Lifecycle.Stager = flaggingStager{PackageStager: fixture.app.Lifecycle.Stager, client: domain.ClientCodex, err: unavailable(domain.ClientCodex), failed: &failed}
	fixture.app.Lifecycle.Lock = lockAfterFailure{MutationLock: fixture.app.Lifecycle.Lock, failed: &failed, err: busy}
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	want := fmt.Sprintf("group preparation failed before client changes (selected targets: [codex cursor kiro]): %v; no selected client was changed", busy)
	if err == nil || err.Error() != want {
		t.Fatalf("later refusal = %v\nwant %s", err, want)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("later refusal changed state: %v", got)
	}
}
