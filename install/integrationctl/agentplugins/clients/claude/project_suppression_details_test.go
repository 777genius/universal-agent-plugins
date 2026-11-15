package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func currentClaudeSuppression() map[string]any {
	// Exact seven-field shape observed read-only from Claude Code 2.1.325 under
	// the adapter's bounded environment. No real profile or trust state is used.
	return map[string]any{
		"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project",
		"enabled": false, "installPath": "", "notes": []string{claudeUntrustedProjectNote},
		"noteDetails": []any{map[string]any{"type": "project-scope-suppressed-untrusted"}},
	}
}

func TestCurrentClaudeSuppressionMetadataRemainsSetupRequired(t *testing.T) {
	fixture := projectListFixture(t)
	body, err := json.Marshal([]any{currentClaudeSuppression()})
	if err != nil {
		t.Fatal(err)
	}
	got := observeClaudeProjectList(body, "demo", fixture.active, fixture.root, fixture.user)
	if got.state != projectListSuppressed || !got.untrusted || !strings.Contains(got.action(), "trust") {
		t.Fatalf("current native suppression became %+v", got)
	}
	if finding := claudeProjectRegistryFinding(body, domain.DetectedClient{ConfigRoot: fixture.user}, domain.DeliveryPlan{
		DeclaredName: "demo", ActivePath: fixture.active, TargetContext: domain.TargetContext{ProjectRoot: fixture.root},
	}, nil); finding != clients.RegistryClear {
		t.Fatalf("suppression claimed native discovery: %v", finding)
	}
}

func TestCurrentClaudePluralSuppressionMatchesExactNativeCountGrammar(t *testing.T) {
	fixture := projectListFixture(t)
	for _, count := range []string{"0", "1", "2", "3", "10", "100", "01", "02", "-2", "+2", "4294967296", "2 ", "two"} {
		t.Run(count, func(t *testing.T) {
			row := currentClaudeSuppression()
			row["notes"] = []string{count + claudeUntrustedProjectsNoteSuffix}
			body, err := json.Marshal([]any{row})
			if err != nil {
				t.Fatal(err)
			}
			got := observeClaudeProjectList(body, "demo", fixture.active, fixture.root, fixture.user)
			n, err := strconv.ParseUint(count, 10, 32)
			valid := err == nil && n > 1 && strconv.FormatUint(n, 10) == count
			if valid {
				if got.state != projectListSuppressed || !got.untrusted {
					t.Fatalf("plural native suppression lost: %+v", got)
				}
			} else if got.state != projectListUnknown {
				t.Fatalf("invalid count manufactured trust: %+v", got)
			}
		})
	}
}

func TestCapturedClaudeTwoDirectorySuppressionRemainsSetupRequired(t *testing.T) {
	fixture := projectListFixture(t)
	body, err := os.ReadFile("testdata/claude-2.1.325-project-suppressed-two.json")
	if err != nil {
		t.Fatal(err)
	}
	got := observeClaudeProjectList(body, "demo", fixture.active, fixture.root, fixture.user)
	if got.state != projectListSuppressed || !got.untrusted || !strings.Contains(got.action(), "trust") {
		t.Fatalf("actual two-directory native listing became %+v", got)
	}
}

func TestCurrentClaudeSuppressionRejectsMalformedAndContradictoryMetadata(t *testing.T) {
	for _, change := range []string{"null", "string", "empty", "many", "wrong-item", "unknown-type", "extra-key", "missing-type", "wrong-type", "contradictory-note", "mixed-notes", "extra-field", "selected", "foreign", "duplicate-key"} {
		t.Run(change, func(t *testing.T) {
			fixture := projectListFixture(t)
			row := currentClaudeSuppression()
			switch change {
			case "null":
				row["noteDetails"] = nil
			case "string":
				row["noteDetails"] = "project-scope-suppressed-untrusted"
			case "empty":
				row["noteDetails"] = []any{}
			case "many":
				row["noteDetails"] = []any{map[string]any{"type": "project-scope-suppressed-untrusted"}, map[string]any{"type": "project-scope-suppressed-untrusted"}}
			case "wrong-item":
				row["noteDetails"] = []any{"project-scope-suppressed-untrusted"}
			case "unknown-type":
				row["noteDetails"] = []any{map[string]any{"type": "project-scope-enabled"}}
			case "extra-key":
				row["noteDetails"] = []any{map[string]any{"type": "project-scope-suppressed-untrusted", "enabled": true}}
			case "missing-type":
				row["noteDetails"] = []any{map[string]any{}}
			case "wrong-type":
				row["noteDetails"] = []any{map[string]any{"type": true}}
			case "contradictory-note":
				row["notes"] = []string{"Project plugins were enabled."}
			case "mixed-notes":
				row["notes"] = []string{claudeUntrustedProjectNote, "Project plugins were enabled."}
			case "extra-field":
				row["unexpected"] = true
			}
			rows := []any{row}
			if change == "selected" || change == "foreign" {
				if change == "foreign" {
					fixture.selected["installPath"] = fixture.active + "-foreign"
				}
				rows = append(rows, fixture.selected)
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if change == "duplicate-key" {
				body = []byte(strings.Replace(string(body), `"type":"project-scope-suppressed-untrusted"`, `"type":"project-scope-suppressed-untrusted","type":"project-scope-enabled"`, 1))
			}
			got := observeClaudeProjectList(body, "demo", fixture.active, fixture.root, fixture.user)
			want := projectListUnknown
			if change == "foreign" {
				want = projectListForeign
			}
			if got.state != want {
				t.Fatalf("unsupported metadata became %+v, want %v", got, want)
			}
		})
	}
}

func TestCurrentClaudeSuppressionActivationRequiresIndependentDeliveryProof(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".claude")
	active := filepath.Join(project, ".claude", "skills", "demo-managed")
	for _, path := range []string{active, user} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	body, err := json.Marshal([]any{currentClaudeSuppression()})
	if err != nil {
		t.Fatal(err)
	}
	runner := timeoutProbeRunner{run: func(context.Context) (legacyports.CommandResult, error) {
		return legacyports.CommandResult{Stdout: body, NoChildStarted: true}, nil
	}}
	request := domain.ActivationRequest{
		DeclaredName: "demo", BackendExecutable: filepath.Join(base, "claude"), Client: domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: user},
		Plan:     domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: user}, NativeRegistryRoot: filepath.Join(project, ".claude"), TargetAnchor: filepath.Join(project, ".claude"), TargetRoot: filepath.Join(project, ".claude", "skills"), ActivePath: active},
		Delivery: domain.StagedDelivery{ActivePath: active, ArtifactDigest: "sha256:" + strings.Repeat("1", 64)},
	}
	outcome, err := activateClaudeProject(t.Context(), clients.Env{Runner: runner}, request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || len(outcome.UserActions) == 0 {
		t.Fatalf("suppression was not preserved as setup-required: %+v %v", outcome, err)
	}
	body, err = os.ReadFile("testdata/claude-2.1.325-project-suppressed-two.json")
	if err != nil {
		t.Fatal(err)
	}
	outcome, err = activateClaudeProject(t.Context(), clients.Env{Runner: runner}, request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || len(outcome.UserActions) == 0 {
		t.Fatalf("actual plural suppression was not preserved as setup-required: %+v %v", outcome, err)
	}
	request.Delivery.ArtifactDigest = ""
	if outcome, err = activateClaudeProject(t.Context(), clients.Env{Runner: runner}, request); err == nil || outcome.Verification == domain.VerificationInstalled {
		t.Fatalf("sentinel manufactured filesystem ownership: %+v %v", outcome, err)
	}
}
