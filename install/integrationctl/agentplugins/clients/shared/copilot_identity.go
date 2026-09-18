package shared

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// InspectCopilotRegistry reads Copilot/VS Code's plugin list. The two clients
// share a backend, so this helper is the one place that knows the listing
// contract.
func InspectCopilotRegistry(ctx context.Context, env clients.Env, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if strings.TrimSpace(plan.NativeRegistryExecutable) != "" {
		return inspectCopilotCLI(ctx, env, plan, managed)
	}
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return clients.RegistryClear, nil
	} else if err != nil {
		return clients.RegistryIndeterminate, err
	}
	// Copilot and VS Code share a backend, but its on-disk registry contract
	// is not implemented by these adapters. An existing config root without
	// the CLI therefore cannot prove absence.
	return clients.RegistryIndeterminate, nil
}

func inspectCopilotCLI(ctx context.Context, env clients.Env, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if env.Runner == nil {
		return clients.RegistryIndeterminate, nil
	}
	result, err := RunNativeRegistry(ctx, env.Runner, legacyports.Command{Argv: []string{plan.NativeRegistryExecutable, "plugin", "list"}})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return clients.RegistryIndeterminate, ctxErr
		}
		return clients.RegistryIndeterminate, err
	}
	if result.ExitCode != 0 {
		return clients.RegistryIndeterminate, fmt.Errorf("the Copilot plugin registry command failed with exit code %d", result.ExitCode)
	}
	expectedPath := plan.ActivePath
	if managed != nil && strings.TrimSpace(managed.TargetLocator) != "" {
		expectedPath = managed.TargetLocator
	}
	return CopilotRegistryFindingAt(result.Stdout, plan.DeclaredName, ManagedMarketplaceName(plan.PhysicalArtifactID), MarketplaceVersion(plan.DeclaredVersion), expectedPath, managed != nil), nil
}

// CopilotRegistryFinding classifies a Copilot/VS Code `plugin list` document
// without a path constraint.
func CopilotRegistryFinding(stdout []byte, name, expectedMarketplace, expectedVersion string, owned bool) clients.RegistryFinding {
	return CopilotRegistryFindingAt(stdout, name, expectedMarketplace, expectedVersion, "", owned)
}

// CopilotRegistryFindingAt classifies a Copilot/VS Code `plugin list` document
// against the managed package identity, version, and optional install path.
func CopilotRegistryFindingAt(stdout []byte, name, expectedMarketplace, expectedVersion, expectedPath string, owned bool) clients.RegistryFinding {
	expected := name + "@" + expectedMarketplace
	if status, recognized := CopilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		return copilotLiveFinding(status, owned)
	}
	return copilotInstalledListingFinding(stdout, name, expectedMarketplace, expectedVersion, owned)
}

func copilotLiveFinding(status CopilotStatus, owned bool) clients.RegistryFinding {
	switch status {
	case CopilotStatusInstalled:
		if !owned {
			return clients.RegistryCollision
		}
		return clients.RegistryExpected
	case CopilotStatusAbsent:
		return clients.RegistryClear
	default:
		return clients.RegistryIndeterminate
	}
}

func copilotInstalledListingFinding(stdout []byte, name, expectedMarketplace, expectedVersion string, owned bool) clients.RegistryFinding {
	normalized := strings.ReplaceAll(string(stdout), "\r\n", "\n")
	document := strings.TrimSuffix(normalized, "\n")
	if document == "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin." {
		return clients.RegistryClear
	}
	scan := copilotListingScan{finding: clients.RegistryClear, seen: map[string]bool{}}
	for _, line := range strings.Split(document, "\n") {
		if finding, stop := scan.consume(line, name, expectedMarketplace, expectedVersion, owned); stop {
			return finding
		}
	}
	if !scan.recognizedHeader || (!scan.recognizedEntry && !scan.recognizedEmpty) {
		return clients.RegistryIndeterminate
	}
	return scan.finding
}

type copilotListingScan struct {
	inInstalled      bool
	recognizedHeader bool
	recognizedEmpty  bool
	recognizedEntry  bool
	finding          clients.RegistryFinding
	seen             map[string]bool
}

func (scan *copilotListingScan) consume(line, name, expectedMarketplace, expectedVersion string, owned bool) (clients.RegistryFinding, bool) {
	if strings.TrimSpace(line) == "Installed plugins:" {
		if scan.inInstalled || scan.recognizedHeader {
			return clients.RegistryIndeterminate, true
		}
		scan.inInstalled, scan.recognizedHeader = true, true
		return 0, false
	}
	if !scan.inInstalled {
		return clients.RegistryIndeterminate, true
	}
	if line != "" && line[0] != ' ' && line[0] != '\t' {
		return clients.RegistryIndeterminate, true
	}
	return scan.consumeBody(line, name, expectedMarketplace, expectedVersion, owned)
}

func (scan *copilotListingScan) consumeBody(line, name, expectedMarketplace, expectedVersion string, owned bool) (clients.RegistryFinding, bool) {
	match := CopilotInstalledEntry.FindStringSubmatch(line)
	if len(match) == 3 {
		return scan.consumeEntry(match, name, expectedMarketplace, expectedVersion, owned)
	}
	trimmed := strings.TrimSpace(line)
	if (trimmed == "No plugins installed." || trimmed == "No plugins installed") && !scan.recognizedEntry && !scan.recognizedEmpty {
		scan.recognizedEmpty = true
		return 0, false
	}
	return clients.RegistryIndeterminate, true
}

func (scan *copilotListingScan) consumeEntry(match []string, name, expectedMarketplace, expectedVersion string, owned bool) (clients.RegistryFinding, bool) {
	if scan.recognizedEmpty {
		return clients.RegistryIndeterminate, true
	}
	scan.recognizedEntry = true
	identity := match[1]
	if scan.seen[identity] {
		return clients.RegistryIndeterminate, true
	}
	scan.seen[identity] = true
	parts := strings.Split(identity, "@")
	if len(parts) != 2 {
		return clients.RegistryIndeterminate, true
	}
	if parts[0] != name || parts[1] != expectedMarketplace {
		return 0, false
	}
	if match[2] != expectedVersion {
		return clients.RegistryIndeterminate, true
	}
	if !owned {
		return clients.RegistryCollision, true
	}
	scan.finding = clients.RegistryExpected
	return 0, false
}
