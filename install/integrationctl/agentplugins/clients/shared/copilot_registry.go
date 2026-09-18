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
		return copilotStatusFinding(status, owned)
	}
	return scanCopilotInstalledListing(stdout, name, expectedMarketplace, expectedVersion, owned)
}

func copilotStatusFinding(status CopilotStatus, owned bool) clients.RegistryFinding {
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

func scanCopilotInstalledListing(stdout []byte, name, expectedMarketplace, expectedVersion string, owned bool) clients.RegistryFinding {
	normalized := strings.ReplaceAll(string(stdout), "\r\n", "\n")
	document := strings.TrimSuffix(normalized, "\n")
	if document == "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin." {
		return clients.RegistryClear
	}
	state := copilotListingScan{}
	for _, line := range strings.Split(document, "\n") {
		finding, done := state.consume(line, name, expectedMarketplace, expectedVersion, owned)
		if done {
			return finding
		}
	}
	if !state.recognizedHeader || (!state.recognizedEntry && !state.recognizedEmpty) {
		return clients.RegistryIndeterminate
	}
	return state.finding
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
	if scan.seen == nil {
		scan.seen = map[string]bool{}
		scan.finding = clients.RegistryClear
	}
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
	match := copilotInstalledEntry.FindStringSubmatch(line)
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
	if parts[0] == name && parts[1] == expectedMarketplace {
		if match[2] != expectedVersion {
			return clients.RegistryIndeterminate, true
		}
		if !owned {
			return clients.RegistryCollision, true
		}
		scan.finding = clients.RegistryExpected
	}
	return 0, false
}
