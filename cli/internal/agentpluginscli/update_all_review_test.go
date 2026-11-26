package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/installlog"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestUpdateAllFailureReportRetainsEarlierImperativeBindingSuccess(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	for _, name := range []string{"alpha", "beta", "gamma"} {
		plugin := writeNamedCLIPlugin(t, name, "1.0.0")
		if output, stderr, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
			t.Fatalf("seed %s: %v\n%s\n%s", name, err, output, stderr)
		}
		writeNamedCLIPluginVersion(t, plugin, name, "2.0.0")
	}
	fixture.app.InstallLogDir = filepath.Join(t.TempDir(), "logs")
	cmd, _, _ := retryTestCommand(t)
	opts := &options{scope: "user", format: "human", yes: true}
	session := updateAllSession{ctx: t.Context(), cmd: cmd, app: fixture.app, opts: opts}
	if empty, err := session.load(); err != nil || empty {
		t.Fatalf("imperative session did not load: empty=%t err=%v", empty, err)
	}
	defer session.cleanupPrepared()
	if err := session.preflightAll(); err != nil {
		t.Fatal(err)
	}
	if len(session.preparedItems) != 3 {
		t.Fatalf("expected three real prepared updates: %+v", session.result)
	}
	cause := fmt.Errorf("beta stage failure: %w", syscall.ENOSPC)
	prepared := session.preparedItems[1].prepared
	stager := &diagnosticStageFailure{PackageStager: prepared.service.Stager, cause: cause}
	prepared.service.Stager = stager
	if err := session.applyAll(); !errors.Is(err, cause) || stager.calls != 1 {
		t.Fatalf("second update did not fail at its real stage boundary: calls=%d err=%v", stager.calls, err)
	}
	if session.result.Updated != 1 || session.result.Installations[0].Status != "updated" || session.result.Installations[2].Status != "not_attempted" {
		t.Fatalf("fixture lost its mixed actual execution facts: %+v", session.result)
	}
	items := map[string]agentItem{}
	for _, item := range opts.installResult.Items {
		items[item.Alias] = item
	}
	if len(items) != 3 || items["alpha"].Status != "completed" || items["alpha"].failure != nil || items["beta"].failure == nil || items["gamma"].Status != "not_attempted" {
		t.Fatalf("per-agent report disagrees with the actual update summary: %+v", items)
	}
	report := opts.installResult.Agents[0]
	if !reflect.DeepEqual(report.Succeeded, []string{"alpha"}) {
		t.Fatalf("earlier applied binding disappeared from agent results: %+v", report)
	}
	body, err := os.ReadFile(opts.installResult.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	logged := map[string]installlog.Entry{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var record struct {
			Type string `json:"type"`
			installlog.Entry
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.Type == "binding" {
			logged[record.Alias] = record.Entry
		}
	}
	if len(logged) != 3 || logged["alpha"].Status != "completed" || logged["alpha"].Code != "" || logged["beta"].Code == "" || logged["gamma"].Status != "not_attempted" {
		t.Fatalf("technical log dropped or relabeled an actual successful update: %+v", logged)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range state.Installations {
		want := "1.0.0"
		if installation.DeclaredName == "alpha" {
			want = "2.0.0"
		}
		if installation.Package.Version != want {
			t.Fatalf("unattempted or failed update changed native state: %s=%s", installation.DeclaredName, installation.Package.Version)
		}
	}
}

func TestImperativeMatchedFailureRetainsRecoveryClassificationAndDetails(t *testing.T) {
	detail := "private failed-target detail"
	cause := fmt.Errorf("private aggregate detail: %w", installer.ErrRecoveryRequired)
	prepared := &preparedUpdateMany{selected: []domain.ClientID{domain.ClientClaude, domain.ClientCodex}, installationID: "installation", result: updateMultiResult{Plugin: "demo"}}
	applied := &updateMultiResult{Targets: []updateTargetResult{
		{Target: "codex", Selected: true, Status: string(usecase.GroupTargetExternalFailed), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed, Failure: &usecase.GroupTargetFailure{Stage: "activation", Message: detail}}}},
		{Target: "claude", Selected: true, Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalCompleted}}},
	}}
	before, _ := json.Marshal(applied)
	items := imperativeUpdateAgentItems(prepared, applied, cause)
	if len(items) != 2 || items[0].failure != nil || items[0].Status != "completed" {
		t.Fatalf("aggregate recovery changed the healthy target: %+v", items)
	}
	failure := items[1].failure
	if failure == nil || failure.Kind != failureRecovery || failure.Retryable || !strings.Contains(failure.Next, "agentplugins doctor") || !strings.Contains(failure.Detail, detail) || !strings.Contains(failure.Detail, cause.Error()) {
		t.Fatalf("matched failed target lost recovery policy or original diagnostics: %+v", failure)
	}
	items = append(items, failedItem("unrelated", "cursor", os.ErrPermission))
	if len(retryableAgents(reportByAgent(items))) != 0 || retryTargetHint(reportByAgent(items)) != "" {
		t.Fatal("wrapped recovery allowed an ordinary namespace retry")
	}
	after, _ := json.Marshal(applied)
	if string(before) != string(after) {
		t.Fatal("recovery classification changed the original native/JSON target result")
	}
	cleanup := errors.Join(cause, &domain.UnclosedClientProbe{Operation: "inventory", Err: errors.New("cleanup unconfirmed")})
	if got := imperativeUpdateAgentItems(prepared, applied, cleanup)[1].failure; got.Kind != failureProcessCleanup || got.Retryable {
		t.Fatalf("recovery classification overrode unconfirmed process cleanup: %+v", got)
	}
}
