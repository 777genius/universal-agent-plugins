package agentpluginscli

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectReadinessFixture(status domain.ActivationState) domain.ClientBinding {
	return domain.ClientBinding{
		ClientID: "codex", ClientBindingID: "binding-a", Scope: string(domain.ScopeProject),
		Materialization: domain.MaterializationMaterialized, Activation: status,
		Authentication: domain.AuthenticationNotChecked, Verification: domain.VerificationInstalled,
		Policy: domain.PolicyAllowed,
	}
}

func TestProjectReadinessDoesNotClaimPreparedNativeExecution(t *testing.T) {
	t.Parallel()
	binding := projectReadinessFixture(domain.ActivationPrepared)
	recorded := domain.Installation{InstallationID: "project-1", DeclaredName: "fixture",
		Clients: map[string]domain.ClientBinding{"codex": binding}}
	records := projectReadinessRecords(domain.StateFileV2{Installations: []domain.Installation{recorded}}, nil)
	if len(records) != 1 {
		t.Fatalf("readiness records = %#v", records)
	}
	item := records[0]
	if item.Status != "prepared_native_unverified" ||
		item.NativeEvidence != "native_execution_not_observed_by_doctor" ||
		item.Verification != domain.VerificationInstalled || len(item.NextActions) < 2 {
		t.Fatalf("prepared project was promoted or lost native guidance: %+v", item)
	}
	evidence, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(evidence), "native_verified") || strings.Contains(string(evidence), "authenticated\"") {
		t.Fatalf("source-only doctor claimed vendor model execution: %s", evidence)
	}
	finding := projectPreparedFinding(recorded, binding)
	if finding.Status != "unknown" || finding.Code != "project_native_unverified" ||
		!strings.Contains(finding.Message, "not been observed") || strings.Contains(finding.RecoveryAction, "agentplugins add") {
		t.Fatalf("normal project delivery became a broken installation: %+v", finding)
	}
}

func TestProjectReadinessStatusPrecedence(t *testing.T) {
	t.Parallel()
	base := projectReadinessFixture(domain.ActivationPrepared)
	cases := []struct {
		name   string
		change func(*domain.Installation, *domain.ClientBinding)
		want   string
	}{
		{"prepared", func(*domain.Installation, *domain.ClientBinding) {}, "prepared_native_unverified"},
		{"needs rebind", func(i *domain.Installation, _ *domain.ClientBinding) { i.NeedsRebind = true }, "source_rebind_required"},
		{"failed verification", func(_ *domain.Installation, b *domain.ClientBinding) { b.Verification = domain.VerificationFailed }, "failed"},
		{"failed activation", func(_ *domain.Installation, b *domain.ClientBinding) { b.Activation = domain.ActivationFailed }, "failed"},
		{"failed authentication", func(_ *domain.Installation, b *domain.ClientBinding) { b.Authentication = domain.AuthenticationFailed }, "failed"},
		{"degraded materialization", func(_ *domain.Installation, b *domain.ClientBinding) {
			b.Materialization = domain.MaterializationDegraded
		}, "failed"},
		{"policy blocked", func(_ *domain.Installation, b *domain.ClientBinding) { b.Policy = domain.PolicyBlocked }, "policy_blocked"},
		{"policy approval", func(_ *domain.Installation, b *domain.ClientBinding) { b.Policy = domain.PolicyApprovalRequired }, "policy_approval_required"},
		{"auth pending", func(_ *domain.Installation, b *domain.ClientBinding) { b.Authentication = domain.AuthenticationPending }, "authentication_pending"},
		{"staged", func(_ *domain.Installation, b *domain.ClientBinding) {
			b.Materialization = domain.MaterializationStaged
		}, "materialization_pending"},
		{"manual", func(_ *domain.Installation, b *domain.ClientBinding) { b.Activation = domain.ActivationManual }, "manual_activation_required"},
		{"active but no runtime observation", func(_ *domain.Installation, b *domain.ClientBinding) { b.Activation = domain.ActivationActive }, "native_unverified"},
		{"previous runtime", func(_ *domain.Installation, b *domain.ClientBinding) {
			b.Activation = domain.ActivationActive
			b.Verification = domain.VerificationRuntime
		}, "runtime_verified_in_record_only"},
		{"not qualified", func(_ *domain.Installation, b *domain.ClientBinding) { b.Activation = domain.ActivationNotRequired }, "not_qualified"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := base
			i := domain.Installation{InstallationID: "install"}
			c.change(&i, &b)
			if actual := projectReadinessStatus(i, b); actual != c.want {
				t.Fatalf("status=%q, want=%q", actual, c.want)
			}
		})
	}
}

func TestProjectReadinessSortedAndScopedAcrossPopularClients(t *testing.T) {
	t.Parallel()
	state := domain.StateFileV2{Installations: []domain.Installation{
		{InstallationID: "b", Clients: map[string]domain.ClientBinding{
			"hidden-user": {ClientID: "claude", Scope: string(domain.ScopeUser), Materialization: domain.MaterializationMaterialized},
			"removed":     {ClientID: "opencode", Scope: string(domain.ScopeProject), Materialization: domain.MaterializationAbsent},
			"z":           {ClientID: "pi", ClientBindingID: "z", Scope: string(domain.ScopeProject), Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared},
			"a":           {ClientID: "codex", ClientBindingID: "a", Scope: string(domain.ScopeProject), Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared},
		}},
		{InstallationID: "a", Clients: map[string]domain.ClientBinding{
			"c": {ClientID: "opencode", ClientBindingID: "c", Scope: string(domain.ScopeProject), Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared},
			"b": {ClientID: "claude", ClientBindingID: "b", Scope: string(domain.ScopeProject), Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared},
		}},
	}}
	expected := []string{"a/claude/b", "a/opencode/c", "b/codex/a", "b/pi/z"}
	for range 30 {
		records := projectReadinessRecords(state, nil)
		keys := make([]string, 0, len(records))
		for _, entry := range records {
			keys = append(keys, entry.InstallationID+"/"+entry.ClientID+"/"+entry.BindingID)
		}
		if !reflect.DeepEqual(keys, expected) {
			t.Fatalf("nondeterministic or cross-scope readiness: %#v", keys)
		}
	}
	selected := projectReadinessRecords(state, &state.Installations[0])
	if len(selected) != 2 || selected[0].ClientID != "codex" || selected[1].ClientID != "pi" {
		t.Fatalf("selected project report leaked other installation: %+v", selected)
	}
}

func TestProjectNativeGuidanceForTwelveAgents(t *testing.T) {
	t.Parallel()
	for client, expected := range map[string]string{
		"codex":    ".codex/config.toml",
		"claude":   "/reload-plugins",
		"opencode": "v1/v2",
		"gemini":   ".gemini/settings.json",
		"copilot":  ".github/skills",
		"cursor":   "Cursor",
		"pi":       "trust",
		"kimi":     ".kimi-code/mcp.json",
		"windsurf": "Devin",
		"kiro":     "Kiro",
		"cline":    "Cline",
		"vscode":   ".vscode/mcp.json",
	} {
		t.Run(client, func(t *testing.T) {
			actions := projectReadinessActions(client, "prepared_native_unverified")
			joined := strings.Join(actions, " | ")
			if !strings.Contains(joined, expected) || !strings.Contains(joined, "not proof") {
				t.Fatalf("client %s guidance missing truth/target: %s", client, joined)
			}
			for _, action := range actions {
				if strings.Contains(action, "agentplugins add ") || strings.Contains(action, "/Users/") || strings.Contains(action, "C:\\Users\\") {
					t.Fatalf("native advice can leak paths or invent source mutation: %q", action)
				}
			}
		})
	}
	pi := strings.Join(projectReadinessActions("pi", "prepared_native_unverified"), " ")
	if !strings.Contains(pi, "does not project portable Skills or MCP servers") {
		t.Fatalf("Pi guidance claimed unsupported portable component installation: %q", pi)
	}
	for status, want := range map[string]string{
		"source_rebind_required":   "source identity",
		"failed":                   "explicit review",
		"policy_blocked":           "policy",
		"policy_approval_required": "consent",
		"authentication_pending":   "authentication",
		"materialization_pending":  "recover",
	} {
		a := projectReadinessActions("codex", status)
		if len(a) != 1 || !strings.Contains(a[0], want) {
			t.Fatalf("incorrect recovery for %s: %#v", status, a)
		}
	}
}

func TestDoctorReadOnlyPublicProjectMatrixEmptyWithoutInstallations(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	out, _, err := fixture.execute(false, "doctor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Data struct {
			ReadOnly         bool                     `json:"read_only"`
			ProjectReadiness []projectReadinessRecord `json:"project_readiness"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.ReadOnly || response.Data.ProjectReadiness == nil || len(response.Data.ProjectReadiness) != 0 {
		t.Fatalf("doctor omitted deterministic empty readiness matrix: %#v", response.Data)
	}
	if _, err := os.Lstat(fixture.store.Path); !os.IsNotExist(err) {
		t.Fatalf("read-only doctor wrote state: %v", err)
	}
}

func TestProjectScopeFlagMakesProjectRootExplicit(t *testing.T) {
	t.Parallel()
	project := domain.ClientBinding{Scope: string(domain.ScopeProject)}
	user := domain.ClientBinding{Scope: string(domain.ScopeUser)}
	if got := doctorScopeFlag(project); got != " --scope project --project-root \"<PROJECT_ROOT>\"" {
		t.Fatalf("project recovery may accidentally select another scope: %q", got)
	}
	if got := doctorScopeFlag(user); got != "" {
		t.Fatalf("user recovery acquired project root: %q", got)
	}
}

func TestProjectReadinessOnlyDependsOnRecordedState(t *testing.T) {
	t.Parallel()
	b := projectReadinessFixture(domain.ActivationPrepared)
	b.NativeProfileRoot = "/private/should-not-appear"
	b.TargetLocator = "/secret/user/target"
	inst := domain.Installation{InstallationID: "scope", Clients: map[string]domain.ClientBinding{"codex": b}}
	recorded := projectReadinessRecords(domain.StateFileV2{Installations: []domain.Installation{inst}}, nil)
	raw, err := json.Marshal(recorded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "should-not-appear") || strings.Contains(string(raw), "secret/user") || strings.Contains(string(raw), "localhost:") {
		t.Fatalf("status report disclosed native profile or provider data: %s", raw)
	}
}

func TestDoctorHealthySentinelNeverContradictsNativeDrift(t *testing.T) {
	t.Parallel()
	healthy := doctorFinding{Status: "healthy", Code: "no_degradation_detected"}
	drift := doctorFinding{Status: "degraded", Code: "codex_project_mcp_member_missing"}
	unknown := doctorFinding{Status: "unknown", Code: "project_native_unverified"}
	if got := dropDoctorHealthySentinel([]doctorFinding{healthy}); len(got) != 1 || got[0] != healthy {
		t.Fatalf("healthy-only report was changed: %+v", got)
	}
	for _, scenario := range [][]doctorFinding{{healthy, drift}, {drift, healthy}, {healthy, unknown, drift}} {
		got := dropDoctorHealthySentinel(append([]doctorFinding(nil), scenario...))
		if len(got) != len(scenario)-1 {
			t.Fatalf("stale healthy sentinel survived: %+v", got)
		}
		for _, entry := range got {
			if entry.Code == "no_degradation_detected" {
				t.Fatalf("doctor gave contradictory healthy and degraded findings: %+v", got)
			}
		}
	}
}
