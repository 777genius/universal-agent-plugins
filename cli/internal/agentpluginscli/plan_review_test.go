package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestInstallReviewCardsFitAndDeduplicateSharedFacts(t *testing.T) {
	envelope, results := installReviewFixture()
	for _, width := range []int{40, 50, 80, 100} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			var output bytes.Buffer
			if err := renderInstallReviewAtWidth(&output, envelope, results, width); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			for number, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Fatalf("line %d width = %d, limit = %d\n%s", number+1, got, width, line)
				}
			}
			if got := strings.Count(text, "playwright 0.0.80"); got != 1 {
				t.Fatalf("plugin identity count = %d\n%s", got, text)
			}
			for _, target := range []string{"cursor", "claude", "gemini", "opencode"} {
				if got := strings.Count(text, "Target: "+target); got != 1 {
					t.Fatalf("target %s count = %d\n%s", target, got, text)
				}
			}
			if strings.Contains(text, catalogNotTestedWarning) || strings.Contains(text, catalogRuntimeNotTestedWarning) {
				t.Fatalf("raw shared warning leaked\n%s", text)
			}
			if got := strings.Count(text, "Catalog and runtime testing evidence"); got != 1 {
				t.Fatalf("verification note count = %d\n%s", got, text)
			}
			if got := strings.Count(strings.Join(strings.Fields(text), " "), "Verify the plugin once in each selected client"); got != 1 {
				t.Fatalf("shared action count = %d\n%s", got, text)
			}
			if got := strings.Count(text, "✓ AUTO"); got != 3 {
				t.Fatalf("automatic installation badge count = %d, want 3\n%s", got, text)
			}
			if got := strings.Count(text, "! MANUAL STEP"); got != 1 {
				t.Fatalf("manual installation badge count = %d, want 1\n%s", got, text)
			}
			if got := strings.Count(text, "! REVIEW"); got != 0 {
				t.Fatalf("review badge count = %d, want 0\n%s", got, text)
			}
			if strings.Contains(text, "✓ READY") || strings.Contains(text, "! SETUP") {
				t.Fatalf("ambiguous legacy statuses leaked\n%s", text)
			}
		})
	}
}

func TestInstallReviewUsesDefaultForegroundForEssentialText(t *testing.T) {
	envelope, results := installReviewFixture()
	var output bytes.Buffer
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	writer := terminaltheme.Wrap(&output, &policy, &format)
	if err := renderInstallReviewAtWidth(writer, envelope, results[:1], 120); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "\x1b[1mplaywright 0.0.80") || !strings.Contains(text, "\x1b[1mCursor") {
		t.Fatalf("essential names are not emphasized with default foreground: %q", text)
	}
	for _, forbidden := range []string{"\x1b[37m", "\x1b[97m", "38;2;"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("background-specific foreground leaked: %q", forbidden)
		}
	}
}

func TestInstallReviewKeepsExplicitPreparationManual(t *testing.T) {
	status, _ := reviewPlanStatus(domain.DeliveryPlan{
		Status: domain.PlanReady, Activation: domain.ActivationPrepared,
		InstallIntent:  domain.InstallIntentPrepare,
		Authentication: domain.AuthenticationNotRequired,
		Verification:   domain.VerificationPackageValid,
	}, newInstallReviewRenderer(80, false).styles)
	if status != "! MANUAL STEP" {
		t.Fatalf("explicit preparation badge = %q", status)
	}
}

func TestInstallReviewPreservesThemeThroughPlanWriter(t *testing.T) {
	envelope, results := installReviewFixture()
	var output bytes.Buffer
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	writer := &planWriter{writer: terminaltheme.Wrap(&output, &policy, &format)}
	if err := renderInstallReviewAtWidth(writer, envelope, results[:1], 120); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("semantic theme was lost through planWriter: %q", output.String())
	}
}

func TestInstallReviewGitHubLinkAndPlainFallback(t *testing.T) {
	envelope, results := installReviewFixture()
	wantURL := "https://github.com/777genius/universal-agent-plugins-registry/tree/128b01ff2b4f7242ad061ad823d5e4018af6b2e4/plugins/playwright"

	var colored bytes.Buffer
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	if err := renderInstallReviewAtWidth(terminaltheme.Wrap(&colored, &policy, &format), envelope, results[:1], 220); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(colored.String(), ansi.SetHyperlink(wantURL)) || !strings.Contains(colored.String(), "github.com/777genius/universal-agent-plugins-registry/plugins/playwright ↗") {
		t.Fatalf("OSC-8 source link missing: %q", colored.String())
	}
	if strings.Contains(colored.String(), "universal-agent-plugins-registry//plugins") {
		t.Fatalf("canonical separator leaked into display: %q", colored.String())
	}

	var plain bytes.Buffer
	if err := renderInstallReviewAtWidth(&plain, envelope, results[:1], 220); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain.String(), wantURL) || strings.Contains(plain.String(), "\x1b") {
		t.Fatalf("plain source is not a copyable URL: %q", plain.String())
	}
}

func TestPublicSourcesUseGitHubPathSeparators(t *testing.T) {
	want := "777genius/universal-agent-plugins-registry/plugins/playwright"
	identity := domain.SourceIdentity{
		Repository:     "777genius/universal-agent-plugins-registry/",
		PackageSubpath: "/plugins/playwright/",
	}
	if got := publicPackageSource(identity); got != want {
		t.Fatalf("public package source = %q, want %q", got, want)
	}
	binding := domain.SourceBinding{Repository: identity.Repository, PackageSubpath: identity.PackageSubpath}
	if got := publicSource(binding); got != want {
		t.Fatalf("stored public source = %q, want %q", got, want)
	}
}

func TestInstallReviewKeepsTargetDiagnosticsWithoutDuplicateWarning(t *testing.T) {
	envelope, results := installReviewFixture()
	results = results[3:]
	results[0].Plan.Warnings = append(results[0].Plan.Warnings, openCodeNamespaceNotice)
	var output bytes.Buffer
	if err := renderInstallReviewAtWidth(&output, envelope, results, 180); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), openCodeNamespaceNotice); got != 1 {
		t.Fatalf("OpenCode diagnostic count = %d\n%s", got, output.String())
	}
	if !strings.Contains(output.String(), "Project settings, later config changes and live tool catalogs were not checked") {
		t.Fatalf("target-specific diagnostic missing\n%s", output.String())
	}
}

type richReviewPrompter struct {
	t *testing.T
}

func (p richReviewPrompter) RichReview() bool { return true }
func (p richReviewPrompter) SelectTargets(context.Context, prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	return prompt.TargetSelectionResult{}, errors.New("unexpected selection")
}
func (p richReviewPrompter) Confirm(context.Context, prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	p.t.Fatal("consent requested after review output failed")
	return prompt.ConfirmationResult{}, nil
}

func TestInstallReviewShortWriteAbortsBeforeConsent(t *testing.T) {
	envelope, results := installReviewFixture()
	cmd := NewRoot(App{})
	app := App{reviewOutput: shortPromptWriter{}, Prompter: richReviewPrompter{t: t}}
	accepted, err := confirmInstall(context.Background(), cmd, app, loadedPackage{envelope: envelope}, results[:1])
	if accepted || !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(accepted, err)
	}
}

func installReviewFixture() (domain.PackageEnvelope, []usecase.AddResult) {
	source := domain.SourceIdentity{
		Repository:       "777genius/universal-agent-plugins-registry",
		PackageSubpath:   "plugins/playwright",
		ResolvedRevision: "128b01ff2b4f7242ad061ad823d5e4018af6b2e4",
	}
	envelope := domain.PackageEnvelope{
		Manifest:   domain.PluginManifest{Name: "playwright", Version: "0.0.80"},
		Source:     source,
		TreeDigest: "sha256:8f0c8ac610974293c4f91f1ecb782fbe9d740f79f84cafe40fdfed1db234d103",
	}
	component := func(support domain.SupportLevel) []domain.ComponentDecision {
		return []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "playwright", Support: support}}
	}
	plan := func(client domain.ClientID, mode domain.PackageMode, status domain.PlanStatus, activation domain.ActivationState, support domain.SupportLevel, next string) usecase.AddResult {
		return usecase.AddResult{Plan: domain.DeliveryPlan{
			ClientID: client, Scope: domain.ScopeUser, PackageMode: mode, Status: status, Activation: activation,
			Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationPackageValid,
			Components: component(support), Warnings: []string{catalogNotTestedWarning, catalogRuntimeNotTestedWarning},
			UserActions: []string{verifySelectedClientAction, next},
		}}
	}
	openCode := plan(domain.ClientOpenCode, domain.PackagePrepared, domain.PlanReady, domain.ActivationPrepared, domain.SupportPrepared, "restart OpenCode")
	openCode.Plan.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityInfo, Code: openCodeNamespaceNotice, Message: "Initial global config server-name preflight found no potential overlap. Project settings, later config changes and live tool catalogs were not checked; verify tools in OpenCode before relying on them."}}
	return envelope, []usecase.AddResult{
		plan(domain.ClientCursor, domain.PackageNative, domain.PlanManualActivationRequired, domain.ActivationManual, domain.SupportNative, "reload Cursor and verify the plugin appears"),
		plan(domain.ClientClaude, domain.PackageProjection, domain.PlanReady, domain.ActivationActive, domain.SupportProjected, "start a new Claude Code session or run /reload-plugins"),
		plan(domain.ClientGemini, domain.PackageNative, domain.PlanReady, domain.ActivationPrepared, domain.SupportNative, "reload or restart Gemini CLI"),
		openCode,
	}
}
