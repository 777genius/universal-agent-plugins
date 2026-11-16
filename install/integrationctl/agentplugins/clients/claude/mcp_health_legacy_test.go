package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Captured from the SHA-pinned Claude 2.1.263 CLI against an inert /bin/false
// stdio child in an isolated profile. The command exited naturally with code 0.
const claudeLegacyFailedHealth = "plugin:uap-health-failure-proof:failed:\n" +
	"  Scope: Dynamic config (from command line)\n" +
	"  Status: ✘ Failed to connect\n" +
	"  Issue: -32000: MCP error -32000: Connection closed\n"

// These complete responses were captured from the pinned native binaries with
// TERM=linux. Claude's official fallback formatter also serves Windows without
// a Unicode-capable terminal. The control ran on Linux, not Windows.
const claudeFallbackConnectedHealth = "plugin:uap-health-fallback-proof:connected:\n" +
	"  Scope: Dynamic config (from command line)\n" +
	"  Status: √ Connected\n"

const claudeFallbackFailedHealth = "plugin:uap-health-fallback-proof:failed:\n" +
	"  Scope: Dynamic config (from command line)\n" +
	"  Status: × Failed to connect\n" +
	"  Issue: -32000: MCP error -32000: Connection closed\n"

const claude325FallbackDetails = "  Type: stdio\n  Command: stdio\n  Args:\n" +
	"  Environment:\n    CLAUDE_PLUGIN_ROOT=[REDACTED]\n    CLAUDE_PLUGIN_DATA=[REDACTED]\n"

func TestClaudeMCPHealthCapturedFallbackSymbols(t *testing.T) {
	for _, test := range []struct {
		name, selector, body, transport string
		state                           claudeMCPHealth
	}{
		{"263-connected", "connected", claudeFallbackConnectedHealth, "", claudeMCPHealthConnected},
		{"263-failed", "failed", claudeFallbackFailedHealth, "", claudeMCPHealthUnready},
		{"325-connected", "connected", claudeFallbackConnectedHealth + claude325FallbackDetails, "stdio", claudeMCPHealthConnected},
		{"325-failed", "failed", strings.Replace(claudeFallbackFailedHealth, "-32000: MCP error -32000: Connection closed", "CONNECTION_CLOSED: Connection closed", 1) + claude325FallbackDetails, "stdio", claudeMCPHealthUnready},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, ending := range []string{"\n", "\r\n"} {
				body := strings.ReplaceAll(test.body, "\n", ending)
				got := observeClaudeMCPHealth([]byte(body), "plugin:uap-health-fallback-proof:"+test.selector)
				if got.state != test.state || got.transport != test.transport {
					t.Fatalf("captured fallback status or transport changed: %+v", got)
				}
			}
		})
	}
}

func TestClaudeMCPHealthFallbackPreservesExactStatusBoundary(t *testing.T) {
	for _, status := range []string{"× Connected", "√ Failed to connect", "Connected", "✔ connected", "√ Connected extra", "√ Connected\n  Status: × Failed to connect"} {
		body := strings.Replace(claudeFallbackConnectedHealth, "√ Connected", status, 1)
		if got := observeClaudeMCPHealth([]byte(body), "plugin:uap-health-fallback-proof:connected"); got != (claudeMCPObservation{}) {
			t.Fatalf("unrecognized status %q accepted: %+v", status, got)
		}
	}
	// The same official formatter substitutes M.cross in its rejection text.
	body := strings.Replace(claudeFallbackConnectedHealth, "√ Connected", "× Rejected (see disabledMcpjsonServers in settings)", 1)
	if got := observeClaudeMCPHealth([]byte(body), "plugin:uap-health-fallback-proof:connected"); got.state != claudeMCPHealthUnready || got.transport != "" {
		t.Fatalf("explicit rejection changed state or invented transport: %+v", got)
	}
}

func TestClaudeLegacyMCPHealthCapturedFailure(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		body := strings.ReplaceAll(claudeLegacyFailedHealth, "\n", ending)
		got := observeClaudeMCPHealth([]byte(body), "plugin:uap-health-failure-proof:failed")
		if got.state != claudeMCPHealthUnready || got.transport != "" {
			t.Fatalf("captured unready response changed state or invented transport: %+v", got)
		}
	}
}

func TestClaudeLegacyMCPHealthIssueRejectsMalformedOrReadyShapes(t *testing.T) {
	const issue = "  Issue: -32000: MCP error -32000: Connection closed\n"
	for name, body := range map[string]string{
		"foreign-selector":         strings.Replace(claudeLegacyFailedHealth, "uap-health-failure-proof", "foreign", 1),
		"foreign-scope":            strings.Replace(claudeLegacyFailedHealth, "Dynamic config (from command line)", "User config", 1),
		"unknown-status":           strings.Replace(claudeLegacyFailedHealth, "✘ Failed to connect", "not observed", 1),
		"connected-issue":          strings.Replace(claudeLegacyFailedHealth, "✘ Failed to connect", "✔ Connected", 1),
		"fallback-connected-issue": strings.Replace(claudeLegacyFailedHealth, "✘ Failed to connect", "√ Connected", 1),
		"duplicate-issue":          claudeLegacyFailedHealth + issue,
		"unknown-line":             strings.Replace(claudeLegacyFailedHealth, "  Issue:", "  Other:", 1),
		"empty-issue":              strings.Replace(claudeLegacyFailedHealth, issue, "  Issue: \n", 1),
		"blank-issue":              strings.Replace(claudeLegacyFailedHealth, issue, "  Issue: \t \n", 1),
		"oversized-issue":          strings.Replace(claudeLegacyFailedHealth, issue, "  Issue: "+strings.Repeat("x", 4097)+"\n", 1),
		"missing-newline":          strings.TrimSuffix(claudeLegacyFailedHealth, "\n"),
		"extra-newline":            claudeLegacyFailedHealth + "\n",
		"second-status":            claudeLegacyFailedHealth + "  Status: ✔ Connected\n",
		"bare-carriage":            strings.Replace(claudeLegacyFailedHealth, "Connection closed", "Connection\rclosed", 1),
		"escape":                   claudeLegacyFailedHealth + "\x1b[0m",
		"nul":                      strings.Replace(claudeLegacyFailedHealth, "Connection closed", "Connection\x00closed", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if got := observeClaudeMCPHealth([]byte(body), "plugin:uap-health-failure-proof:failed"); got != (claudeMCPObservation{}) {
				t.Fatalf("malformed legacy response accepted: %+v", got)
			}
		})
	}
}

func TestClaudeFallbackConnectedHealthRequiresVerifiedSelectedLocalTransport(t *testing.T) {
	for _, test := range []struct {
		name, body string
		local      bool
	}{
		{"local", `{"local":{"command":"fixture"}}`, true},
		{"foreign", `{"other":{"command":"fixture"}}`, false},
		{"remote", `{"local":{"type":"http","url":"https://example.invalid/mcp"}}`, false},
		{"no-digest", `{"local":{"command":"fixture"}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, _ := claudeLegacyMCPRequest(t, domain.ScopeProject, test.body)
			if test.name == "no-digest" {
				request.Delivery.ArtifactDigest = ""
			}
			runner := &claudeMCPTestRunner{t: t, status: "√ Connected", omitType: true}
			initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled,
				Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked, NativeEffect: domain.NativeEffectUnchanged}
			outcome, err := finishClaudeMCPActivation(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, initial)
			want := domain.AuthenticationNotChecked
			if test.local {
				want = domain.AuthenticationNotRequired
			}
			if err != nil || outcome.Activation != initial.Activation || outcome.Authentication != want || outcome.NativeEffect != initial.NativeEffect || runner.calls != 2 {
				t.Fatalf("fallback status bypassed selected transport proof: %+v %v", outcome, err)
			}
			if _, err := os.Lstat(filepath.Join(request.Plan.TargetAnchor, "settings.local.json")); !os.IsNotExist(err) {
				t.Fatalf("read-only health changed approval: %v", err)
			}
		})
	}
}

func TestClaudeLegacyMCPFailureRemainsPreparedWithoutAuthenticationProof(t *testing.T) {
	request, _ := claudeLegacyMCPRequest(t, domain.ScopeProject, `{"local":{"command":"fixture"}}`)
	runner := &claudeMCPTestRunner{t: t, status: "✘ Failed to connect", omitType: true,
		issue: "-32000: MCP error -32000: Connection closed"}
	initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled,
		Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked, NativeEffect: domain.NativeEffectUnchanged}
	outcome, err := finishClaudeMCPActivation(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, initial)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Authentication != domain.AuthenticationNotChecked ||
		outcome.Verification != initial.Verification || outcome.Policy != initial.Policy || outcome.NativeEffect != initial.NativeEffect ||
		runner.calls != 2 || len(outcome.UserActions) == 0 {
		t.Fatalf("unready legacy health manufactured readiness or lost guidance: %+v %v", outcome, err)
	}
	if _, err := os.Lstat(filepath.Join(request.Plan.TargetAnchor, "settings.local.json")); !os.IsNotExist(err) {
		t.Fatalf("read-only unready health changed approval: %v", err)
	}
}
