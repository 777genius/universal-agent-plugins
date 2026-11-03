package installer

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) detectedClient(req Request) (domain.DetectedClient, error) {
	id := domain.ClientID(req.ClientID)
	if !e.SupportsClient(req.ClientID) {
		return domain.DetectedClient{}, fmt.Errorf("%w: client %q is not registered for this installer", ErrUnsupported, req.ClientID)
	}
	if (req.ClientConfigRoot == "" && e.scope() != domain.ScopeProject) || (req.ClientConfigRoot != "" && !validRoot(req.ClientConfigRoot)) {
		return domain.DetectedClient{}, fmt.Errorf("%w: ClientConfigRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if (req.ClientExecutable != "" && !validRoot(req.ClientExecutable)) || (req.Operation == OpInstall && requiresExecutable(e.cfg.Registry, id, e.scope()) && req.ClientExecutable == "") {
		return domain.DetectedClient{}, fmt.Errorf("%w: ClientExecutable must be an explicit absolute clean path", ErrInvalidRequest)
	}
	root := req.ClientConfigRoot
	if resolver, ok := clients.As[clients.ProfileResolver](e.cfg.Registry, id); ok {
		var err error
		root, err = resolver.ResolveProfileRoot(root)
		if err != nil {
			return domain.DetectedClient{}, fmt.Errorf("%w: invalid client profile: %w", ErrInvalidRequest, err)
		}
	}
	return domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: root, ExecutablePath: req.ClientExecutable, Version: req.ClientVersion}, nil
}

func missingRequired(envelope domain.PackageEnvelope, required []string) []string {
	var missing []string
	for _, item := range required {
		switch item {
		case "mcp":
			if len(envelope.MCP.Servers) == 0 {
				missing = append(missing, item)
			}
		case "skills":
			if len(envelope.Skills) == 0 {
				missing = append(missing, item)
			}
		}
	}
	return missing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func findInstall(state domain.StateFileV2, id string) (domain.Installation, bool) {
	for _, item := range state.Installations {
		if item.InstallationID == id {
			return item, true
		}
	}
	return domain.Installation{}, false
}

func findBinding(installation domain.Installation, client domain.ClientID) (domain.ClientBinding, domain.DataReceipt, bool) {
	for _, binding := range installation.Clients {
		if binding.ClientID != string(client) {
			continue
		}
		return binding, installation.DataReceipts[binding.DataReceiptID], true
	}
	return domain.ClientBinding{}, domain.DataReceipt{}, false
}
