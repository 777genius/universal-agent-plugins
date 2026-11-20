package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
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
				receipts = append(receipts, receipt.MutationType+":"+receipt.Phase)
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

// An unclassified cause stops every agent, exactly as before.
func TestGroupAddUnclassifiedCauseStopsEveryAgent(t *testing.T) {
	t.Parallel()
	cause := errors.New("stage cursor: the fixture broke in an unusual way")
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCursor: cause})
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	want := fmt.Sprintf("group preparation failed before client changes (selected targets: [codex cursor kiro]): %v; no selected client was changed", cause)
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v\nwant %s", err, want)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("unclassified cause changed state: %v", got)
	}
}

// When every agent fails on its own, nothing changed and the result is the one
// the whole selection always reported: the first agent's cause.
func TestGroupAddEveryAgentFailingKeepsTheWholeSelectionResult(t *testing.T) {
	t.Parallel()
	// Each agent fails for its own target-local cause, so each is left out in
	// turn until none remains.
	fail := map[domain.ClientID]error{}
	for _, client := range isolationClients {
		fail[client] = unavailable(client)
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

// Only causes that concern one agent leave the others to install. Everything
// else, unclassified causes and mixtures with a group-wide cause included,
// stops every agent.
func TestFailureStaysWithTargetClassifiesEveryCause(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		err   error
		local bool
	}{
		"foreign":                         {fmt.Errorf("x: %w", nativeconfig.ErrCollision), true},
		"foreign identity":                {&domain.TypedError{Text: "unmanaged", Kind: domain.ErrForeignNativeIdentity}, true},
		"target unavailable":              {fmt.Errorf("x: %w", installer.ErrTargetFactsUnavailable), true},
		"unsupported":                     {fmt.Errorf("x: %w", installer.ErrIncomplete), true},
		"unsupported target":              {&domain.TypedError{Text: "unsupported", Kind: domain.ErrTargetUnsupported}, true},
		"unclassified":                    {errors.New("something odd"), false},
		"wrapped unclassified":            {fmt.Errorf("stage: %w", errors.New("something odd")), false},
		"edited copy":                     {&domain.TypedError{Text: "managed package was changed", Kind: domain.ErrManagedPackageChanged, Cause: unavailable(domain.ClientCodex)}, false},
		"verification":                    {fmt.Errorf("x: %w", &ports.VerificationError{Kind: ports.VerificationDigestMismatch, Err: installer.ErrTargetFactsUnavailable}), false},
		"disk full":                       {fmt.Errorf("write: %w", &fs.PathError{Op: "write", Path: "/p", Err: syscall.ENOSPC}), false},
		"network":                         {fmt.Errorf("fetch: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("no route")}), false},
		"join collision and plan change":  {errors.Join(nativeconfig.ErrCollision, installer.ErrPlanChanged), false},
		"join unavailable and update":     {errors.Join(installer.ErrTargetFactsUnavailable, installer.ErrUpdateRequired), false},
		"two-%w incomplete and source":    {fmt.Errorf("%w; %w", installer.ErrIncomplete, environment.ErrSourceChange), false},
		"join collision and cleanup":      {errors.Join(nativeconfig.ErrCollision, &domain.UnclosedClientProbe{Err: installer.ErrTargetFactsUnavailable}), false},
		"join collision and unclassified": {errors.Join(nativeconfig.ErrCollision, errors.New("something odd")), false},
		"join two target-local causes":    {errors.Join(nativeconfig.ErrCollision, installer.ErrTargetFactsUnavailable), true},
		"attributed target-local":         {&usecase.GroupTargetError{Clients: []domain.ClientID{domain.ClientKiro}, Err: unavailable(domain.ClientKiro)}, true},
		"cleanup":                         {fmt.Errorf("x: %w", &domain.UnclosedClientProbe{}), false},
		"recovery":                        {fmt.Errorf("x: %w", installer.ErrRecoveryRequired), false},
		"security":                        {fmt.Errorf("x: %w", installer.ErrAssessmentRejected), false},
		"managed changed":                 {fmt.Errorf("x: %w", installer.ErrManagedSkillChanged), false},
		"update required":                 {fmt.Errorf("x: %w", installer.ErrUpdateRequired), false},
		"plan changed":                    {fmt.Errorf("x: %w", installer.ErrPlanChanged), false},
		"source changed":                  {fmt.Errorf("x: %w", environment.ErrSourceChange), false},
		"timeout":                         {fmt.Errorf("x: %w", context.DeadlineExceeded), false},
		"canceled":                        {fmt.Errorf("x: %w", context.Canceled), false},
	}
	for name, tc := range cases {
		if got := failureStaysWithTarget(tc.err); got != tc.local {
			t.Errorf("%s: failureStaysWithTarget = %v, want %v", name, got, tc.local)
		}
	}
	for _, kind := range []failureKind{"DISK_FULL", "NETWORK_UNAVAILABLE", failureUnknown, failureSkipped, failureRecovery, failureProcessCleanup} {
		if targetLocalKinds[kind] {
			t.Errorf("%s must stop every agent", kind)
		}
	}
}

func TestTargetLocalGroupFailureRequiresAnUnchangedAttributedGroup(t *testing.T) {
	t.Parallel()
	attributed := &usecase.GroupTargetError{Clients: []domain.ClientID{domain.ClientKiro}, Err: unavailable(domain.ClientKiro)}
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
		"not attributed":   {context.Background(), usecase.GroupResult{Phase: usecase.GroupPhasePreparationFailed}, unavailable(domain.ClientKiro), false},
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
	targets := data["targets"].([]any)
	order, okOrder := make([]string, 0, len(targets)), make([]string, 0, len(targets))
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
// it keeps the whole-selection behavior.
func TestGroupIsolationIsOffWhenVSCodeIsSelected(t *testing.T) {
	t.Parallel()
	with := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCopilot}, {ClientID: domain.ClientVSCode}}, nil, "op")
	without := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCopilot}, {ClientID: domain.ClientCursor}}, nil, "op")
	if with.enabled() || !without.enabled() {
		t.Fatalf("enabled = %v/%v", with.enabled(), without.enabled())
	}
}

// Native backend ownership, not a generic CLI ClientID switch, decides whether
// a prepared sibling may split off after a failed group. Preserve the existing
// VS Code whole-selection rule in both User and Project without disabling the
// native Copilot owner or independent clients.
func TestGroupIsolationUsesNativeBackendOwnerAcrossScopes(t *testing.T) {
	t.Parallel()
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			owner := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCopilot}}, []usecase.AddInput{{Scope: scope}}, "op")
			sibling := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientVSCode}}, []usecase.AddInput{{Scope: scope}}, "op")
			independent := newGroupIsolation([]domain.DetectedClient{{ClientID: domain.ClientCursor}}, []usecase.AddInput{{Scope: scope}}, "op")
			if !owner.enabled() || sibling.enabled() || !independent.enabled() {
				t.Fatalf("scope %s: native owner=%v, prepared sibling=%v, independent=%v", scope, owner.enabled(), sibling.enabled(), independent.enabled())
			}
		})
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
	// Nothing was planned by the refused attempt, so this is a preflight refusal.
	want := fmt.Sprintf("group apply preflight failed; no target was changed (selected targets: [codex cursor kiro]): %v", busy)
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("later refusal = %v\nwant %s", err, want)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("later refusal changed state: %v", got)
	}
}

// recordingStager remembers every operation id it staged and can fail one
// client's staging or every discard.
type recordingStager struct {
	ports.PackageStager
	fail       map[domain.ClientID]error
	discardErr error
	operations *[]string
}

func (stager recordingStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	*stager.operations = append(*stager.operations, string(plan.ClientID)+"@"+operationID)
	if err := stager.fail[plan.ClientID]; err != nil {
		return domain.StagedDelivery{}, err
	}
	return stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
}

func (stager recordingStager) Discard(ctx context.Context, delivery domain.StagedDelivery) error {
	if stager.discardErr != nil {
		return stager.discardErr
	}
	return stager.PackageStager.Discard(ctx, delivery)
}

func stagingLeftovers(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && strings.Contains(entry.Name(), ".agentplugins-staging-") {
			found = append(found, path)
		}
		return nil
	})
	return found
}

// The retry is a new operation: a new operation id, and nothing staged by the
// failed attempt is left behind or reused.
func TestGroupAddRetryIsANewCleanOperation(t *testing.T) {
	t.Parallel()
	var operations []string
	fixture := newIsolationFixture(t, nil)
	fixture.app.Lifecycle.Stager = recordingStager{PackageStager: fixture.app.Lifecycle.Stager,
		fail: map[domain.ClientID]error{domain.ClientKiro: unavailable(domain.ClientKiro)}, operations: &operations}
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro"); err == nil {
		t.Fatal("partial run succeeded")
	}
	groups := map[string]bool{}
	for _, operation := range operations {
		id := operation[strings.Index(operation, "@")+1:]
		groups[id[:strings.LastIndex(id, "-")]] = true
	}
	if len(operations) != 5 || len(groups) != 2 {
		t.Fatalf("attempts must use distinct operation ids: %v", operations)
	}
	if leftovers := stagingLeftovers(t, fixture.root); len(leftovers) != 0 {
		t.Fatalf("the failed attempt left staged packages: %v", leftovers)
	}
}

// When the failed attempt cannot remove what it staged, its state is not known
// to be clean, so every agent stops.
func TestGroupAddCleanupFailureStopsEveryAgent(t *testing.T) {
	t.Parallel()
	var operations []string
	discard := errors.New("discard refused")
	fixture := newIsolationFixture(t, nil)
	fixture.app.Lifecycle.Stager = recordingStager{PackageStager: fixture.app.Lifecycle.Stager,
		fail: map[domain.ClientID]error{domain.ClientKiro: unavailable(domain.ClientKiro)}, discardErr: discard, operations: &operations}
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	if err == nil || !errors.Is(err, discard) || !strings.Contains(err.Error(), "no selected client was changed") {
		t.Fatalf("cleanup failure = %v", err)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("cleanup failure changed state: %v", got)
	}
}

// An edited installed copy stops the whole selection, exactly as before.
func TestGroupAddEditedInstalledCopyStopsEveryAgent(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	fixture := newIsolationFixture(t, nil)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "codex"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("seed: %+v %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		if err := os.WriteFile(filepath.Join(binding.TargetLocator, "plugin.json"), []byte(`{"edited":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err = fixture.execute(false, "add", plugin, "--target", "codex,cursor")
	if err == nil || !strings.Contains(err.Error(), "group preflight failed; no target was changed") {
		t.Fatalf("edited copy = %v", err)
	}
	after, _ := fixture.store.Load()
	if len(after.Installations) != 1 || len(after.Installations[0].Clients) != 1 {
		t.Fatalf("edited copy let a peer install: %+v", after)
	}
}

// The person confirms only what will be applied: the plan lists the agents
// that remain, and the notice names the agent left out and why.
func TestGroupAddConfirmationShowsOnlyTheAppliedSubset(t *testing.T) {
	t.Parallel()
	fixture := newIsolationFixture(t, nil)
	fixture.app.Lifecycle.NativeObserver = selectiveNativeObserver{foreign: domain.ClientKiro}
	stdout, _, err := fixture.executeInput(true, "\ny\n", "add", writeCLIPlugin(t))
	if err == nil {
		t.Fatalf("partial run succeeded:\n%s", stdout)
	}
	review := stdout
	if cut := strings.Index(stdout, "installation finished with issues"); cut > 0 {
		review = stdout[:cut]
	}
	if !strings.Contains(review, "kiro will not be installed: Something UAP does not own already exists at the install location.") {
		t.Fatalf("notice missing before confirmation:\n%s", stdout)
	}
	if strings.Contains(review, "Target: kiro") || !strings.Contains(review, "Target: codex") || !strings.Contains(review, "Target: cursor") {
		t.Fatalf("confirmed plan differs from the applied subset:\n%s", review)
	}
}

// A group refused later for a group-wide cause keeps the earlier agent's own
// failure instead of reporting it as not attempted.
func TestGroupAddLeftOutRowSurvivesLaterPreparationFailure(t *testing.T) {
	t.Parallel()
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCursor: fmt.Errorf("stage cursor: %w", installer.ErrRecoveryRequired)})
	fixture.app.Lifecycle.NativeObserver = selectiveNativeObserver{foreign: domain.ClientKiro}
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro", "--format", "json")
	if err == nil || !errors.Is(err, installer.ErrRecoveryRequired) {
		t.Fatalf("later failure = %v", err)
	}
	var envelope struct {
		Data struct {
			Status  string `json:"status"`
			Targets []struct {
				Target string                      `json:"target"`
				Status string                      `json:"status"`
				Error  *usecase.GroupTargetFailure `json:"error"`
				Output struct {
					Result struct {
						InstallationID string `json:"installation_id"`
					} `json:"result"`
				} `json:"output"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if envelope.Data.Status != "preparation_failed" || len(envelope.Data.Targets) != 3 {
		t.Fatalf("summary = %+v", envelope.Data)
	}
	for _, row := range envelope.Data.Targets {
		switch row.Target {
		case "kiro":
			if row.Status != string(usecase.GroupTargetExternalFailed) || row.Error == nil || row.Error.Stage != "preparation" || row.Output.Result.InstallationID != "" {
				t.Fatalf("left-out row = %+v", row)
			}
		default:
			if row.Status != string(usecase.GroupTargetExternalNotAttempted) {
				t.Fatalf("remaining row = %+v", row)
			}
		}
	}
}

// Three selected and one remaining: the remaining agent is installed as a
// one-target group, the same as selecting only it.
func TestGroupAddSingleRemainingAgentEqualsSelectingOnlyIt(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientCodex: unavailable(domain.ClientCodex), domain.ClientKiro: unavailable(domain.ClientKiro)})
	_, _, err := fixture.execute(false, "add", plugin, "--target", "codex,cursor,kiro")
	if err == nil || err.Error() != "2 of 3 client installations failed; see results above" {
		t.Fatalf("partial = %v", err)
	}
	alone := newIsolationFixture(t, nil)
	if _, _, err := alone.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if got, want := installedProjection(t, fixture), installedProjection(t, alone); !reflect.DeepEqual(got, want) {
		t.Fatalf("single remaining agent differs:\n got %v\nwant %v", got, want)
	}
}

// The retry adds the agent to the installation of its peers, so it can only
// install the same package bytes; a changed package is refused.
func TestGroupAddRetryIsPinnedToThePeersPackage(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientKiro: unavailable(domain.ClientKiro)})
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "codex,cursor,kiro"); err == nil {
		t.Fatal("partial run succeeded")
	}
	writeCLIPluginVersion(t, plugin, "1.0.1")
	delete(fixture.app.Lifecycle.Stager.(clientFailingCLIStager).fail, domain.ClientKiro)
	_, _, err := fixture.execute(false, "add", plugin, "--target", "kiro")
	if err == nil || !strings.Contains(err.Error(), "recorded desired package bytes") {
		t.Fatalf("retry with a changed package = %v", err)
	}
	if got := installedProjection(t, fixture); len(got) != 3 {
		t.Fatalf("changed-package retry changed state: %v", got)
	}
}

// VS Code is frozen: a selection with it keeps the whole-selection behavior.
func TestGroupAddWithVSCodeKeepsWholeSelectionBehaviour(t *testing.T) {
	t.Parallel()
	vscode := fixtureClient(t, domain.ClientVSCode)
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), vscode})
	fixture.app.Lifecycle.Stager = clientFailingCLIStager{PackageStager: fixture.app.Lifecycle.Stager, fail: map[domain.ClientID]error{domain.ClientCursor: unavailable(domain.ClientCursor)}}
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor,vscode")
	if err == nil || !strings.Contains(err.Error(), "no selected client was changed") {
		t.Fatalf("vscode selection = %v", err)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("vscode selection changed state: %v", got)
	}
}

// machineProblems are causes the plain-cause classifier names for one agent.
var machineProblems = map[string]struct {
	err  error
	kind failureKind
}{
	"permission":    {permissionDenied(domain.ClientKiro), failurePermission},
	"read-only":     {fmt.Errorf("create client target root: %w", &fs.PathError{Op: "mkdir", Path: "/project/.kiro", Err: syscall.EROFS}), failureReadOnly},
	"agent missing": {fmt.Errorf("probe kiro: %w", &exec.Error{Name: "kiro-cli", Err: exec.ErrNotFound}), failureAgentMissing},
}

// A machine problem of one agent stays with it: the peers install.
func TestMachineProblemsStayWithTheAgent(t *testing.T) {
	t.Parallel()
	for name, tc := range machineProblems {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if kind := failureKindOf(tc.err); kind != tc.kind || !failureStaysWithTarget(tc.err) {
				t.Fatalf("%s: kind %s, stays %v", name, kind, failureStaysWithTarget(tc.err))
			}
			fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientKiro: tc.err})
			_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
			if err == nil || err.Error() != "1 of 3 client installations failed; see results above" {
				t.Fatalf("classified cause = %v", err)
			}
			if got := installedProjection(t, fixture); len(got) != 3 {
				t.Fatalf("peers were not installed: %v", got)
			}
		})
	}
}

// The group usecase's sentinels are used only for the group decision. Every
// other path classifies these errors exactly as before: a generic, retryable
// failure.
func TestGroupSentinelsDoNotChangeTheSharedClassification(t *testing.T) {
	t.Parallel()
	verification := &ports.VerificationError{Kind: ports.VerificationDigestMismatch, Err: errors.New("artifact digest mismatch")}
	for name, err := range map[string]error{
		"edited copy":      &domain.TypedError{Text: "managed package was changed or is missing; refusing silent add and retaining state: artifact digest mismatch", Kind: domain.ErrManagedPackageChanged, Cause: verification},
		"identity absent":  &domain.TypedError{Text: "managed native identity is unexpectedly absent", Kind: domain.ErrManagedPackageChanged},
		"foreign identity": &domain.TypedError{Text: "native identity is unmanaged; remove it through its owning client or choose a distinct identity", Kind: domain.ErrForeignNativeIdentity},
		"unsupported":      &domain.TypedError{Text: "target codex is unsupported; group preflight caused no mutation", Kind: domain.ErrTargetUnsupported},
	} {
		summary, next, retryable := applyFailureWording(failureUnknown)
		want := installFailure{Kind: failureUnknown, Summary: summary, Next: next, Retryable: retryable, Detail: err.Error()}
		if got := describeSyncFailure(err); *got != want || failureKindOf(err) != failureUnknown || !got.Retryable {
			t.Errorf("%s: shared classification changed: %+v", name, *got)
		}
	}
}

// A single-target VS Code add refused for an edited installed copy returns the
// group usecase's typed error; the shared classification of that real error is
// unchanged: generic and retryable.
func TestSingleTargetVSCodeEditedCopyKeepsItsClassification(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientVSCode)})
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "vscode"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("seed: %+v %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		if err := os.WriteFile(filepath.Join(binding.TargetLocator, "plugin.json"), []byte(`{"edited":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err = fixture.execute(false, "add", plugin, "--target", "vscode")
	if err == nil || !errors.Is(err, domain.ErrManagedPackageChanged) {
		t.Fatalf("edited copy = %v", err)
	}
	if failure := describeSyncFailure(err); failure.Kind != failureUnknown || !failure.Retryable {
		t.Fatalf("shared classification changed: %+v", failure)
	}
}

// failingPurge refuses to purge newly created plugin data.
type failingPurge struct {
	usecase.PluginDataManager
	err error
}

func (data failingPurge) PurgeData(context.Context, domain.DataReceipt) error { return data.err }

// When the failed attempt cannot purge the plugin data it created, its state
// is not known to be clean, so every agent stops.
func TestGroupAddPurgeFailureStopsEveryAgent(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"sh"}}}`
	if err := os.WriteFile(filepath.Join(plugin, "mcp.json"), []byte(mcp), 0o644); err != nil {
		t.Fatal(err)
	}
	purge := errors.New("purge refused")
	fixture := newIsolationFixture(t, map[domain.ClientID]error{domain.ClientKiro: unavailable(domain.ClientKiro)})
	fixture.app.Lifecycle.PluginData = failingPurge{PluginDataManager: fixture.app.Lifecycle.PluginData, err: purge}
	_, _, err := fixture.execute(false, "add", plugin, "--target", "codex,cursor,kiro")
	if err == nil || !errors.Is(err, purge) || !strings.Contains(err.Error(), "no selected client was changed") {
		t.Fatalf("purge failure = %v", err)
	}
	if got := installedProjection(t, fixture); len(got) != 0 {
		t.Fatalf("purge failure changed state: %v", got)
	}
}
