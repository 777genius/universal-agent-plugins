package directoryv1

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validatePolicy(p domain.DirectoryReleasePolicy, reader snapshotReader) error {
	if p.Status != domain.ReleaseActive && p.Status != domain.ReleaseSuperseded && p.Status != domain.ReleaseRevoked {
		return fmt.Errorf("invalid status")
	}
	if _, err := parseSemver(p.MinimumInstallerVersion); err != nil {
		return fmt.Errorf("invalid minimum installer version")
	}
	if len(p.Targets) < 1 || p.CurrentEvidence == nil {
		return fmt.Errorf("targets/evidence arrays required")
	}
	clients := map[domain.ClientID]bool{}
	for _, target := range p.Targets {
		if clients[target.Client] {
			return fmt.Errorf("invalid target")
		}
		if err := validateTarget(target, reader); err != nil {
			return err
		}
		clients[target.Client] = true
	}
	seen := map[string]bool{}
	for _, id := range p.CurrentEvidence {
		if !evidenceIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid current evidence ID")
		}
		seen[id] = true
	}
	return nil
}

func validateTarget(target domain.DirectoryTarget, reader snapshotReader) error {
	if target.Scope != "" || target.MinimumInstallerVersion != "" || target.CurrentEvidence != nil || target.CanonicalClosure != nil {
		return fmt.Errorf("v1 target cannot carry scoped grant fields")
	}
	if !reader.validClient(target.Client) || len(target.Scopes) != 1 || target.Scopes[0] != domain.ScopeUser || !validTargetDelivery(target, reader) {
		return fmt.Errorf("invalid target")
	}
	if target.Authentication != domain.AuthenticationRequirementNotRequired && target.Authentication != domain.AuthenticationRequirementRequired && target.Authentication != domain.AuthenticationRequirementUnknown {
		return fmt.Errorf("invalid target authentication")
	}
	return validateTargetAppBinding(target)
}

func validTargetDelivery(target domain.DirectoryTarget, reader snapshotReader) bool {
	expected, supported := domain.ExpectedDirectoryDelivery(target.Client)
	if domain.IsSupportedClient(target.Client) {
		return supported && target.Delivery == expected
	}
	// Future client identities may only use the existing schema-1 delivery
	// vocabulary. A known adapter must still authorize the exact delivery.
	return reader == modernReader && oneOf(target.Delivery, "managed", "prepared", "manual_activation")
}

func validateTargetAppBinding(target domain.DirectoryTarget) error {
	definition, ok := domain.ClientDefinitionFor(target.Client)
	if ok && definition.Capabilities.AppSupport != domain.SupportUnsupported {
		if target.AppBinding == nil || target.AppBinding.AppKey == "" || target.AppBinding.ID == "" || target.AppBinding.MCPServer == "" {
			return fmt.Errorf("chatgpt app binding required")
		}
		if err := domain.ValidateAppBindingIdentity(target.AppBinding.AppKey, target.AppBinding.ID, target.AppBinding.MCPServer); err != nil {
			return fmt.Errorf("invalid chatgpt app binding: %w", err)
		}
	} else if target.AppBinding != nil {
		return fmt.Errorf("app binding only allowed for chatgpt")
	}
	return nil
}
