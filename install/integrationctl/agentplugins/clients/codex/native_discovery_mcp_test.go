package codex

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCodexNativeMCPRequiresInitializedExactUnauthenticatedServer(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-name", "wrong-plugin", "not-started", "oauth", "unknown-auth", "missing-info", "tool-error", "invalid-tools", "duplicate", "next-page"} {
		t.Run(scenario, func(t *testing.T) {
			row := map[string]any{"name": "local", "pluginId": nil, "authStatus": "unsupported", "runtimeStatus": nil, "toolsError": nil, "serverInfo": map[string]any{"name": "fixture", "version": "1"}, "tools": map[string]any{}}
			result := map[string]any{"data": []any{row}, "nextCursor": nil}
			switch scenario {
			case "wrong-name":
				row["name"] = "other"
			case "wrong-plugin":
				row["pluginId"] = "foreign@market"
			case "not-started":
				row["runtimeStatus"] = "notStarted"
			case "oauth":
				row["authStatus"] = "notLoggedIn"
			case "unknown-auth":
				row["authStatus"] = "unknown"
			case "missing-info":
				row["serverInfo"] = nil
			case "tool-error":
				row["toolsError"] = "discovery failed"
			case "invalid-tools":
				row["tools"] = nil
			case "duplicate":
				result["data"] = []any{row, row}
			case "next-page":
				result["nextCursor"] = "more"
			}
			err := verifyCodexMCPStatus(result, "local", "")
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("scenario=%s err=%v", scenario, err)
			}
		})
	}
}

func TestCodexNativeMCPAuthenticationPreservesExplicitDecisions(t *testing.T) {
	for _, auth := range []domain.AuthenticationState{"", domain.AuthenticationNotChecked, domain.AuthenticationPending, domain.AuthenticationFailed, domain.AuthenticationComplete, domain.AuthenticationNotRequired} {
		plan := domain.DeliveryPlan{Authentication: auth, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative}}}
		want := auth
		if auth == "" || auth == domain.AuthenticationNotChecked {
			want = domain.AuthenticationNotRequired
		}
		if got := codexNativeAuthentication(plan); got != want {
			t.Fatalf("auth=%s got=%s want=%s", auth, got, want)
		}
	}
}
