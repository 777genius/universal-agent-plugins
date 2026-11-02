package agentpluginscli

import (
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A recorded project installation proves delivery, not an automatically loaded
// skill, trusted workspace, model turn, MCP handshake, or provider authorization.
// This presentation-only view never changes the authoritative state or probes a
// client, and therefore cannot promote a prepared binding to an active one.
type projectReadinessRecord struct {
	InstallationID  string                      `json:"installation_id"`
	BindingID       string                      `json:"binding_id"`
	ClientID        string                      `json:"client_id"`
	Status          string                      `json:"status"`
	Materialization domain.MaterializationState `json:"materialization"`
	Activation      domain.ActivationState      `json:"activation"`
	Authentication  domain.AuthenticationState  `json:"authentication"`
	Policy          domain.PolicyState          `json:"policy"`
	Verification    domain.VerificationState    `json:"verification"`
	NativeEvidence  string                      `json:"native_evidence"`
	NextActions     []string                    `json:"next_actions"`
}

// projectReadinessRecords deliberately contains no native paths, TOML or JSON
// content, secrets, profile names, ambient environment, or remote account facts.
func projectReadinessRecords(state domain.StateFileV2, selected *domain.Installation) []projectReadinessRecord {
	installations := state.Installations
	if selected != nil {
		installations = []domain.Installation{*selected}
	}
	records := make([]projectReadinessRecord, 0)
	for _, installation := range installations {
		for _, binding := range installation.Clients {
			if binding.Scope != string(domain.ScopeProject) || binding.Materialization == domain.MaterializationAbsent {
				continue
			}
			status := projectReadinessStatus(installation, binding)
			evidence := "native_execution_not_observed_by_doctor"
			if binding.Verification == domain.VerificationRuntime {
				evidence = "recorded_runtime_verification_not_rechecked"
			}
			records = append(records, projectReadinessRecord{
				InstallationID: installation.InstallationID,
				BindingID:       binding.ClientBindingID,
				ClientID:        binding.ClientID,
				Status:          status,
				Materialization: binding.Materialization,
				Activation:      binding.Activation,
				Authentication:  binding.Authentication,
				Policy:          binding.Policy,
				Verification:    binding.Verification,
				NativeEvidence:  evidence,
				NextActions:     projectReadinessActions(binding.ClientID, status),
			})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.InstallationID != b.InstallationID {
			return a.InstallationID < b.InstallationID
		}
		if a.ClientID != b.ClientID {
			return a.ClientID < b.ClientID
		}
		return a.BindingID < b.BindingID
	})
	return records
}

func projectReadinessStatus(installation domain.Installation, binding domain.ClientBinding) string {
	switch {
	case installation.NeedsRebind:
		return "source_rebind_required"
	case binding.Materialization == domain.MaterializationDegraded || binding.Verification == domain.VerificationFailed || binding.Activation == domain.ActivationFailed || binding.Authentication == domain.AuthenticationFailed:
		return "failed"
	case binding.Policy == domain.PolicyBlocked:
		return "policy_blocked"
	case binding.Policy == domain.PolicyApprovalRequired:
		return "policy_approval_required"
	case binding.Authentication == domain.AuthenticationPending:
		return "authentication_pending"
	case binding.Materialization != domain.MaterializationMaterialized:
		return "materialization_pending"
	case binding.Activation == domain.ActivationPrepared:
		return "prepared_native_unverified"
	case binding.Activation == domain.ActivationManual:
		return "manual_activation_required"
	case binding.Activation == domain.ActivationActive && binding.Verification == domain.VerificationRuntime:
		return "runtime_verified_in_record_only"
	case binding.Activation == domain.ActivationActive:
		return "native_unverified"
	default:
		return "not_qualified"
	}
}

// These are vendor-client observation instructions, never claims that the
// source-only doctor has executed a vendor binary. Wording deliberately avoids
// requiring a new UAP add/update for a correct Prepared installation.
func projectReadinessActions(clientID, status string) []string {
	switch status {
	case "source_rebind_required":
		return []string{"Review the original immutable source identity; do not adopt a different project or source automatically"}
	case "failed":
		return []string{"Inspect the recorded failure and repair only the original project-owned objects after explicit review"}
	case "policy_blocked":
		return []string{"Review the selected project's managed/native policy with the owner; do not override it automatically"}
	case "policy_approval_required":
		return []string{"Review and approve the native client's request in its own trusted UI; UAP must not synthesize consent"}
	case "authentication_pending":
		return []string{"Complete the native provider's authentication flow; project file installation does not authenticate an account"}
	case "materialization_pending":
		return []string{"Finish or recover the existing installation transaction before checking native activation"}
	}
	var explanation string
	if status == "runtime_verified_in_record_only" {
		explanation = "A previous runtime observation is recorded; rerun a fresh native-client session for current proof"
	} else {
		explanation = "The project files are not proof of effective native discovery, model tool use, workspace trust or authentication"
	}
	actions := append([]string{explanation}, projectNativeClientActions(clientID)...)
	if status == "manual_activation_required" {
		actions = append(actions, "Perform the documented manual native activation; do not treat this as automatic installation")
	}
	return actions
}

// Keep the per-client instructions apart from status policy so adding a client
// cannot accidentally promote its project files to verified runtime execution.
func projectNativeClientActions(clientID string) []string {
	switch clientID {
	case "codex":
		return []string{"Start Codex in the selected project, review workspace trust and effective native settings",
			"Check .agents/skills and the project .codex/config.toml; inspect codex mcp list --json in a fresh session"}
	case "claude":
		return []string{"Start Claude Code in the selected project and review the native trust/plugin state",
			"Inspect /plugin and /mcp, then use /reload-plugins or restart after accepting project trust"}
	case "opencode":
		return []string{"Preserve the existing OpenCode v1/v2 installation; never switch generations implicitly",
			"Start the selected OpenCode generation in this project and inspect effective config, skills and MCP with native debug/mcp commands"}
	case "gemini":
		return []string{"Start Gemini CLI in the selected project and review workspace trust and .gemini/settings.json",
			"Inspect native skill discovery and MCP status in the current Gemini CLI session"}
	case "copilot":
		return []string{"Start GitHub Copilot CLI in the selected project and review native trust and MCP configuration",
			"Confirm project .github/skills and project MCP loading in a fresh Copilot CLI session"}
	case "cursor":
		return []string{"Open the selected project in Cursor and review workspace trust, project Skills and MCP settings",
			"Verify tool visibility and MCP server status inside Cursor; file preparation alone is not activation"}
	case "pi":
		return []string{"Start Pi in the selected project and review its native extension loading and applicable trust policy",
			"Verify extension discovery in a fresh Pi session; UAP's Pi target does not project portable Skills or MCP servers"}
	case "kimi":
		return []string{"Start Kimi Code from the selected project Git root and inspect selected Skill directories",
			"Check effective project .kimi-code/mcp.json loading and trust in a fresh session"}
	case "windsurf":
		return []string{"Review the installed Devin Local/Windsurf client generation, project Skill selection and MCP import before use"}
	case "kiro":
		return []string{"Inspect Kiro's project workspace, native trust, Skills and MCP selection in its own application"}
	case "cline":
		return []string{"Review Cline project Skills and the explicitly selected native MCP settings; no default IDE forwarding is claimed"}
	case "vscode":
		return []string{"Open the exact VS Code project folder and review workspace trust, project Skills and .vscode/mcp.json"}
	default:
		return []string{"Open the supported client in the selected project and check its own trust, Skills and MCP discovery"}
	}
}

func projectPreparedFinding(installation domain.Installation, binding domain.ClientBinding) doctorFinding {
	guidance := projectReadinessActions(binding.ClientID, "prepared_native_unverified")
	return scopedFinding("unknown", "project_native_unverified", installation, binding.ClientID,
		"project components are prepared, but effective native discovery and model/tool execution have not been observed",
		guidance[len(guidance)-1])
}
