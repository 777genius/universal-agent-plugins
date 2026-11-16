package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
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

func TestCodexNativeMCPVersionGateSendsNoUnsupportedRequest(t *testing.T) {
	for _, tc := range []struct {
		agent string
		allow bool
	}{
		{"agentplugins/0.153.4", false},
		{"agentplugins/0.161.9", false},
		{"agentplugins/0.162.0", true},
		{"agentplugins/0.162.1 (Linux; x86_64)", true},
		{"agentplugins/0.163.0", true},
		{"agentplugins/1.0.0", true},
		{"agentplugins/0.162.0-alpha.1", false},
		{"agentplugins/0.162.0+build", false},
		{"agentplugins/00.162.0", false},
		{"agentplugins/+0.162.0", false},
		{"agentplugins/0.162", false},
		{"agentplugins/0.162.0.1", false},
		{"codex-cli 0.162.0", false},
		{"", false},
	} {
		t.Run(tc.agent, func(t *testing.T) {
			plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative}}}
			response := `{"id":4,"result":{"data":[{"name":"local","pluginId":null,"authStatus":"unsupported","serverInfo":{"name":"fixture","version":"1"},"tools":{}}],"nextCursor":null}}` + "\n"
			var requests bytes.Buffer
			err := verifyCodexNativeMCP(bufio.NewScanner(strings.NewReader(response)), json.NewEncoder(&requests), plan, "", tc.agent)
			if !tc.allow {
				if err == nil || !strings.Contains(err.Error(), "0.162.0") || requests.Len() != 0 {
					t.Fatalf("unqualified native version received a request: %q, %s, %v", tc.agent, &requests, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var request struct {
				Method string
				Params struct{ ServerName string }
			}
			if err := json.Unmarshal(bytes.TrimSpace(requests.Bytes()), &request); err != nil ||
				request.Method != "mcpServerStatus/list" || request.Params.ServerName != "local" {
				t.Fatalf("qualified observation was not selected-server-only: %s, %v", &requests, err)
			}
		})
	}
}

func TestCodexNativeMCPRequestsEachSelectedServerExactlyOnce(t *testing.T) {
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "first", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "second", Support: domain.SupportNative},
	}}
	var responses bytes.Buffer
	for index, name := range []string{"first", "second"} {
		if err := json.NewEncoder(&responses).Encode(map[string]any{"id": index + 4, "result": map[string]any{
			"data": []any{map[string]any{"name": name, "authStatus": "unsupported", "serverInfo": map[string]any{"name": "fixture", "version": "1"}, "tools": map[string]any{}}},
		}}); err != nil {
			t.Fatal(err)
		}
	}
	var requests bytes.Buffer
	if err := verifyCodexNativeMCP(bufio.NewScanner(&responses), json.NewEncoder(&requests), plan, "", "agentplugins/0.162.0"); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&requests)
	for _, name := range []string{"first", "second"} {
		var request struct {
			Method string
			Params struct{ ServerName string }
		}
		if err := decoder.Decode(&request); err != nil || request.Method != "mcpServerStatus/list" || request.Params.ServerName != name {
			t.Fatalf("selected MCP request differs: %+v, %v", request, err)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("observation requested unselected native work: %v, %v", extra, err)
	}
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative}}
	requests.Reset()
	if err := verifyCodexNativeMCP(bufio.NewScanner(strings.NewReader("")), json.NewEncoder(&requests), plan, "", "agentplugins/0.153.4"); err != nil || requests.Len() != 0 {
		t.Fatalf("skills-only discovery acquired an MCP version prerequisite: %s, %v", &requests, err)
	}
}
