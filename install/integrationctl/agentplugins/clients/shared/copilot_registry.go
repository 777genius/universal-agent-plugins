package shared

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func CopilotRegistryFinding(stdout []byte, name, expectedMarketplace, expectedVersion string, owned bool) clients.RegistryFinding {
	return CopilotRegistryFindingAt(stdout, name, expectedMarketplace, expectedVersion, "", owned)
}

func CopilotRegistryFindingAt(stdout []byte, name, expectedMarketplace, expectedVersion, expectedPath string, owned bool) clients.RegistryFinding {
	expected := name + "@" + expectedMarketplace
	if status, recognized := CopilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
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
	normalized := strings.ReplaceAll(string(stdout), "\r\n", "\n")
	document := strings.TrimSuffix(normalized, "\n")
	if document == "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin." {
		return clients.RegistryClear
	}
	inInstalled := false
	recognizedHeader := false
	recognizedEmpty := false
	recognizedEntry := false
	finding := clients.RegistryClear
	seen := map[string]bool{}
	for _, line := range strings.Split(document, "\n") {
		if strings.TrimSpace(line) == "Installed plugins:" {
			if inInstalled || recognizedHeader {
				return clients.RegistryIndeterminate
			}
			inInstalled, recognizedHeader = true, true
			continue
		}
		if !inInstalled {
			return clients.RegistryIndeterminate
		}
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			return clients.RegistryIndeterminate
		}
		match := copilotInstalledEntry.FindStringSubmatch(line)
		if len(match) == 3 {
			if recognizedEmpty {
				return clients.RegistryIndeterminate
			}
			recognizedEntry = true
			identity := match[1]
			if seen[identity] {
				return clients.RegistryIndeterminate
			}
			seen[identity] = true
			parts := strings.Split(identity, "@")
			if len(parts) != 2 {
				return clients.RegistryIndeterminate
			}
			if parts[0] == name && parts[1] == expectedMarketplace {
				if match[2] != expectedVersion {
					return clients.RegistryIndeterminate
				}
				if !owned {
					return clients.RegistryCollision
				}
				finding = clients.RegistryExpected
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if (trimmed == "No plugins installed." || trimmed == "No plugins installed") && !recognizedEntry && !recognizedEmpty {
			recognizedEmpty = true
			continue
		}
		return clients.RegistryIndeterminate
	}
	if !recognizedHeader || (!recognizedEntry && !recognizedEmpty) {
		return clients.RegistryIndeterminate
	}
	return finding
}
