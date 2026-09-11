package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func TestSharedGroupHumanReviewIdentifiesLogicalTargets(t *testing.T) {
	for _, order := range [][]domain.ClientID{{domain.ClientCopilot, domain.ClientVSCode}, {domain.ClientVSCode, domain.ClientCopilot}} {
		t.Run(string(order[0]), func(t *testing.T) {
			first, second := fixtureClient(t, order[0]), fixtureClient(t, order[1])
			if first.ClientID == domain.ClientCopilot {
				first.ExecutablePath = "/test/bin/copilot"
			} else {
				second.ExecutablePath = "/test/bin/copilot"
			}
			f := newCLIFixture(t, []domain.DetectedClient{first, second})
			out, _, err := f.executeInput(true, "\nn\n", "add", writeCLIPlugin(t))
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{"copilot", "vscode"} {
				if strings.Count(out, "Target: "+target+"\n") != 1 {
					t.Fatalf("selected target missing or repeated: %s", out)
				}
			}
			if !strings.Contains(out, "Uses shared physical binding owned by ") ||
				strings.Count(out, "Authentication: not_checked") != 2 || !strings.Contains(out, "Installation not applied") {
				t.Fatal(out)
			}
			state, err := f.store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("declined review mutated state: %+v, %v", state, err)
			}
		})
	}
}

func TestGroupCollisionErrorIncludesSelectedTargetContext(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientKiro)})
	f.app.Lifecycle.NativeObserver = selectiveNativeObserver{foreign: domain.ClientCursor}
	_, _, err := f.execute(false, "add", writeCLIPlugin(t), "--target", "cursor,kiro")
	if err == nil || !strings.Contains(err.Error(), "selected targets: [cursor kiro]") ||
		!strings.Contains(err.Error(), "unmanaged") || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("collision error: %v", err)
	}
	state, loadErr := f.store.Load()
	if loadErr != nil || len(state.Installations) != 0 {
		t.Fatalf("collision mutated state: %+v, %v", state, loadErr)
	}
}

func TestBatchActivationApplySummaryAndExitCodes(t *testing.T) {
	t.Parallel()
	t.Run("human summary keeps order and hides technical phases", func(t *testing.T) {
		result := addMultiResult{
			Plugin: "demo", Version: "1.0.0", Status: string(usecase.GroupPhaseExternalPartialFailure),
			Succeeded: 3, Failed: 1, ActionRequired: 2,
			Targets: []addTargetResult{
				{Target: "codex", displayName: "OpenAI Codex", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{
					GroupPhase: usecase.GroupTargetExternalCompleted,
					Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationInstalled},
				}}},
				{Target: "kiro", displayName: "Kiro", Status: string(usecase.GroupTargetExternalCompleted), NextAction: "Restart Kiro and approve the connection.", Output: addResultData{Result: usecase.AddResult{
					GroupPhase: usecase.GroupTargetExternalCompleted,
					Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationPending, Verification: domain.VerificationInstalled},
				}}},
				{Target: "chatgpt", displayName: "ChatGPT", Status: "action_required", NextAction: "Finish ChatGPT setup."},
				{Target: "claude", displayName: "Claude Code", Status: string(usecase.GroupTargetExternalFailed), RetryCommand: "npx universal-agent-plugins add demo --target claude",
					Error:  &usecase.GroupTargetFailure{Stage: "activation", Message: "Could not update ~/.claude/config.json: permission denied."},
					Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed, Failure: &usecase.GroupTargetFailure{Stage: "activation", Message: "Could not update ~/.claude/config.json: permission denied."}}}},
			},
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		if !strings.Contains(body, "demo installation finished with issues") || !strings.Contains(body, "Client") || !strings.Contains(body, "Result") {
			t.Fatalf("summary header missing: %s", body)
		}
		for _, want := range []string{"OpenAI Codex", "Installed", "Kiro", "Sign-in required", "ChatGPT", "Setup required", "Claude Code", "Failed"} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in %s", want, body)
			}
		}
		if strings.Contains(body, "external_completed") || strings.Contains(body, "external_failed") {
			t.Fatalf("technical phases leaked: %s", body)
		}
		attention := strings.Index(body, "Needs attention")
		if attention < 0 {
			t.Fatalf("needs attention missing: %s", body)
		}
		detail := body[attention:]
		if !strings.Contains(detail, "Restart Kiro") || !strings.Contains(detail, "Finish ChatGPT setup.") ||
			!strings.Contains(detail, "permission denied") || !strings.Contains(detail, "Retry:") ||
			!strings.Contains(detail, "npx universal-agent-plugins add demo --target claude") {
			t.Fatalf("attention details = %s", detail)
		}
		if strings.Contains(detail, "OpenAI Codex") {
			t.Fatalf("installed target listed under needs attention: %s", detail)
		}
		codexPos := strings.Index(body, "OpenAI Codex")
		kiroPos := strings.Index(body, "Kiro")
		chatgptPos := strings.Index(body, "ChatGPT")
		claudePos := strings.Index(body, "Claude Code")
		if !(codexPos < kiroPos && kiroPos < chatgptPos && chatgptPos < claudePos) {
			t.Fatalf("client order broken: %s", body)
		}
	})
	t.Run("long failure keeps two column summary", func(t *testing.T) {
		long := strings.Repeat("permission denied while writing managed config path ", 8)
		result := addMultiResult{
			Plugin: "demo", Status: string(usecase.GroupPhaseExternalPartialFailure), Succeeded: 1, Failed: 1,
			Targets: []addTargetResult{
				{Target: "codex", displayName: "Codex", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{
					GroupPhase: usecase.GroupTargetExternalCompleted,
					Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationInstalled},
				}}},
				{Target: "cursor", displayName: "Cursor", Status: string(usecase.GroupTargetExternalFailed), RetryCommand: "npx universal-agent-plugins add demo --target cursor",
					Error:  &usecase.GroupTargetFailure{Stage: "activation", Message: long},
					Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}},
			},
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(out.String(), "\n")
		var summaryLines []string
		for _, line := range lines {
			if strings.HasPrefix(line, "Codex") || strings.HasPrefix(line, "Cursor") || strings.HasPrefix(line, "Client") {
				summaryLines = append(summaryLines, line)
			}
		}
		if len(summaryLines) < 3 {
			t.Fatalf("summary lines = %#v body=%s", summaryLines, out.String())
		}
		for _, line := range summaryLines {
			if strings.Contains(line, "permission denied") {
				t.Fatalf("long error leaked into summary columns: %q", line)
			}
		}
	})
	t.Run("quoted retry arguments", func(t *testing.T) {
		got := batchRetryCommand("demo-plugin", domain.ClientClaude)
		if got != "npx universal-agent-plugins add demo-plugin --target claude" {
			t.Fatalf("retry = %q", got)
		}
		if got := batchRetryCommand("my plugin", domain.ClientClaude); got != "" {
			t.Fatalf("unsafe spaced source must omit retry: %q", got)
		}
		if got := batchRetryCommand("demo", domain.ClientID("weird target")); got != "" {
			t.Fatalf("unsafe target must omit retry: %q", got)
		}
	})
	t.Run("action required only keeps exit zero", func(t *testing.T) {
		fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
		stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor", "--format", "json")
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Data addMultiResult `json:"data"`
		}
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatal(err)
		}
		if output.Data.Failed != 0 || output.Data.Succeeded != 2 {
			t.Fatalf("success counts = %+v", output.Data)
		}
	})
	t.Run("real failure exit one and continues activation", func(t *testing.T) {
		fixture := newCLIFixture(t, []domain.DetectedClient{
			fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientKiro),
		})
		activator := &failSecondCLIGroupActivator{}
		fixture.app.Lifecycle.Activator = activator
		stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
		if err == nil || !strings.Contains(err.Error(), "1 of 3 client installations failed; see results above") {
			t.Fatalf("err = %v", err)
		}
		if activator.calls != 3 {
			t.Fatalf("Activate calls = %d, want 3", activator.calls)
		}
		if !strings.Contains(stdout, "Needs attention") || !strings.Contains(stdout, "Failed") || !strings.Contains(stdout, "Installed") {
			t.Fatalf("human summary = %s", stdout)
		}
		if strings.Contains(stdout, "injected grouped activation failure") && strings.Count(stdout, "injected grouped activation failure") > 1 {
			t.Fatalf("raw failure duplicated after summary: %s", stdout)
		}
	})
	t.Run("dry-run output unchanged shape", func(t *testing.T) {
		fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
		stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor", "--dry-run")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout, "Plugin:") || !strings.Contains(stdout, "Targets:") || !strings.Contains(stdout, "No changes made (dry run).") {
			t.Fatalf("dry-run changed: %s", stdout)
		}
		if strings.Contains(stdout, "Needs attention") || strings.Contains(stdout, "Client  Result") {
			t.Fatalf("apply summary leaked into dry-run: %s", stdout)
		}
	})
	t.Run("auth pending and not-checked stay action-required", func(t *testing.T) {
		pending := addTargetResult{Target: "kiro", displayName: "Kiro", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{
			GroupPhase: usecase.GroupTargetExternalCompleted,
			Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationPending, Verification: domain.VerificationInstalled},
		}}}
		unchecked := addTargetResult{Target: "codex", displayName: "Codex", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{
			GroupPhase: usecase.GroupTargetExternalCompleted,
			Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotChecked, Verification: domain.VerificationInstalled},
		}}}
		if classifyBatchPresentation(pending) != batchPresentationSignInRequired {
			t.Fatalf("pending = %s", classifyBatchPresentation(pending))
		}
		if classifyBatchPresentation(unchecked) != batchPresentationSetupRequired {
			t.Fatalf("not-checked = %s", classifyBatchPresentation(unchecked))
		}
		var counts addMultiResult
		countBatchTarget(&counts, pending)
		countBatchTarget(&counts, unchecked)
		if counts.Failed != 0 || counts.Succeeded != 2 || counts.ActionRequired != 2 {
			t.Fatalf("counts = %+v", counts)
		}
	})
	t.Run("unknown commit and not-attempted retries", func(t *testing.T) {
		source := "demo-plugin"
		unknown := addTargetResult{Target: "cursor", displayName: "Cursor", Status: string(usecase.GroupTargetManagedUnknown), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetManagedUnknown}}}
		skipped := addTargetResult{Target: "kiro", displayName: "Kiro", Status: string(usecase.GroupTargetExternalNotAttempted),
			Error: &usecase.GroupTargetFailure{Stage: "persist", Message: "processing stopped because the operation was canceled"},
			Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalNotAttempted}}}
		if retry := batchRetryCommandFor(unknown, source); retry != "" {
			t.Fatalf("unknown commit must not get retry: %q", retry)
		}
		if retry := batchRetryCommandFor(skipped, source); retry != "npx universal-agent-plugins add demo-plugin --target kiro" {
			t.Fatalf("not-attempted retry = %q", retry)
		}
		result := addMultiResult{Plugin: "demo", Failed: 2, Targets: []addTargetResult{unknown, skipped}}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		if !strings.Contains(body, "Needs attention") || !strings.Contains(body, "Managed commit state is unknown") || !strings.Contains(body, "Not completed") {
			t.Fatalf("body = %s", body)
		}
		if strings.Contains(body, "Retry:\n    npx universal-agent-plugins add demo-plugin --target cursor") {
			t.Fatalf("unsafe unknown retry present: %s", body)
		}
	})
	t.Run("presentation source omits retry", func(t *testing.T) {
		if got := batchRetrySource(domain.PackageEnvelope{}, "direct local source"); got != "" {
			t.Fatalf("presentation source leaked: %q", got)
		}
		if got := batchRetryCommandFor(addTargetResult{Target: "cursor", Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}}, ""); got != "" {
			t.Fatalf("empty source retry = %q", got)
		}
		localPath := filepath.Join(t.TempDir(), "plugin")
		if got := batchRetrySource(domain.PackageEnvelope{Source: domain.SourceIdentity{RequestedSource: localPath}}, "direct local source"); got != "" {
			t.Fatalf("local absolute path leaked into retry source: %q", got)
		}
	})
	t.Run("json failure status without failed count", func(t *testing.T) {
		result := addMultiResult{Batch: true, Status: "apply_failed", Plugin: "demo", Version: "1.0.0", Source: "direct local source"}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "json"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"result":"failure"`) || strings.Contains(out.String(), `"result":"success"`) {
			t.Fatalf("json = %s", out.String())
		}
	})
	t.Run("rolled back appears under needs attention", func(t *testing.T) {
		result := addMultiResult{Plugin: "demo", Failed: 2, Targets: []addTargetResult{
			{
				Target: "cursor", displayName: "Cursor", Status: string(usecase.GroupTargetManagedRolledBack),
				Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetManagedRolledBack}},
			},
			{
				Target: "kiro", displayName: "Kiro", Status: string(usecase.GroupTargetManagedRolledBack),
				Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetManagedRolledBack}},
			},
		}}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		if !strings.Contains(body, "Rolled back") || !strings.Contains(body, "Needs attention") || !strings.Contains(body, "rolled back") {
			t.Fatalf("body = %s", body)
		}
	})
	t.Run("aggregate error excludes deferred chatgpt", func(t *testing.T) {
		result := addMultiResult{
			Succeeded: 1, Failed: 1, ActionRequired: 1,
			Targets: []addTargetResult{
				{Target: "codex", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalCompleted}}},
				{Target: "cursor", Status: string(usecase.GroupTargetExternalFailed), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}},
				{Target: "chatgpt", Status: "action_required", NextAction: "Finish ChatGPT setup."},
			},
		}
		err := batchActivationAggregateError(result)
		if err == nil || err.Error() != "1 of 2 client installations failed; see results above" {
			t.Fatalf("aggregate = %v", err)
		}
	})
	t.Run("chatgpt-only json is success with action_required", func(t *testing.T) {
		if batchStatusIndicatesFailure("action_required") {
			t.Fatal("action_required must not be treated as failure status")
		}
		result := map[string]any{"status": "action_required", "target": "chatgpt", "mutated": false}
		var out bytes.Buffer
		if err := writeJSONResult(&out, "add", outputResultSuccess, result); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"result":"success"`) || !strings.Contains(out.String(), `"status":"action_required"`) {
			t.Fatalf("json = %s", out.String())
		}
	})
	t.Run("deferred chatgpt remains visible when peers fail", func(t *testing.T) {
		result := addMultiResult{
			Plugin: "demo", Status: string(usecase.GroupPhaseManagedActivationFailed), Failed: 1, ActionRequired: 1,
			Targets: []addTargetResult{
				{Target: "kiro", displayName: "Kiro", Status: string(usecase.GroupTargetExternalFailed),
					Error: &usecase.GroupTargetFailure{Stage: "activation", Message: "kiro failed"},
					Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}},
				{Target: "chatgpt", displayName: "ChatGPT", Status: "action_required", NextAction: "Finish ChatGPT setup."},
			},
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		if !strings.Contains(body, "ChatGPT") || !strings.Contains(body, "Setup required") || !strings.Contains(body, "Finish ChatGPT setup.") {
			t.Fatalf("missing deferred chatgpt: %s", body)
		}
	})
	t.Run("multiline failure stays under attention only", func(t *testing.T) {
		result := addMultiResult{
			Plugin: "demo", Succeeded: 1, Failed: 1,
			Targets: []addTargetResult{
				{Target: "codex", displayName: "Codex", Status: string(usecase.GroupTargetExternalCompleted), Output: addResultData{Result: usecase.AddResult{
					GroupPhase: usecase.GroupTargetExternalCompleted,
					Activation: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationInstalled},
				}}},
				{Target: "cursor", displayName: "Cursor", Status: string(usecase.GroupTargetExternalFailed),
					Error: &usecase.GroupTargetFailure{Stage: "activation", Message: "first line\nsecond line with agentplugins add resume --foo"},
					Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}},
			},
		}
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		if err := renderAddMultiResult(cmd, &options{format: "human"}, result, domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}}); err != nil {
			t.Fatal(err)
		}
		body := out.String()
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Cursor") && strings.Contains(line, "first line") {
				t.Fatalf("multiline leaked into summary row: %q", line)
			}
		}
		if !strings.Contains(body, "first line") || !strings.Contains(body, "second line") {
			t.Fatalf("attention lost multiline detail: %s", body)
		}
	})
}
