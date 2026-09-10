package planner

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const KiroPrepareAction = "After preparation, open or restart Kiro, review the MCP servers and complete authentication in Kiro on first connection when prompted, then verify their tools in Kiro. Runtime connections have not been verified."

// ApplyInstallIntent retains all package and Directory compatibility decisions.
func ApplyInstallIntent(plan *domain.DeliveryPlan, intent domain.InstallIntent) error {
	if err := intent.Validate(plan.ClientID); err != nil {
		return err
	}
	if intent == domain.InstallIntentPrepare && plan.Scope != domain.ScopeUser {
		return fmt.Errorf("preparation supports user scope only")
	}
	plan.InstallIntent = intent
	if intent != domain.InstallIntentPrepare || plan.Status == domain.PlanUnsupported {
		return nil
	}
	if plan.ClientID == domain.ClientChatGPT {
		if !plan.PersonalChatGPTPreparation {
			return fmt.Errorf("ChatGPT preparation requires a validated Context7 personal mapping")
		}
		plan.Status = domain.PlanReady
		plan.Activation = domain.ActivationPrepared
		plan.Authentication = domain.AuthenticationPending
		plan.Verification = domain.VerificationPackageValid
		plan.UserActions = []string{domain.ChatGPTRegistrationAction}
		return nil
	}
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" || !hasOnlyKiroNativeComponents(plan.Components) {
		return fmt.Errorf("Kiro preparation requires a native config root and supported skills or MCP servers")
	}
	plan.Status = domain.PlanReady
	plan.Activation = domain.ActivationPrepared
	plan.Verification = domain.VerificationPackageValid
	plan.UserActions = []string{KiroPrepareAction}
	return nil
}
