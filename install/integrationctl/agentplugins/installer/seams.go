package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type seamStager struct {
	profileCheck func(context.Context) error
	providers.Stager
	serverName  string
	projectArgs func(BindingFacts) ([]string, error)
	facts       BindingFacts
}

func (s seamStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	_, cursorSelected := plan.SelectedDelivery.CursorFacts()
	if s.serverName != "" && s.projectArgs != nil && (cursorSelected && len(domain.SelectedMCPNames(plan)) > 0 || !selectedNativeOnly(plan.SelectedDelivery)) {
		return domain.StagedDelivery{}, fmt.Errorf("host projection requires owned plugin data")
	}
	return s.Stager.Stage(ctx, envelope, plan, operationID, hints)
}

func (s seamStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, data string) (domain.StagedDelivery, error) {
	callback := s.projectArgs
	if _, selected := plan.SelectedDelivery.CursorFacts(); selected && s.serverName != "" && callback != nil {
		names := domain.SelectedMCPNames(plan)
		if len(names) == 0 {
			callback = nil // Retain the selected Stop-only path.
		} else if !slices.Contains(names, s.serverName) {
			return domain.StagedDelivery{}, fmt.Errorf("cursor projection requires selected declared MCP server %s", s.serverName)
		}
	}
	if s.profileCheck != nil {
		if err := s.profileCheck(ctx); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	facts := s.facts
	facts.TargetContext = plan.TargetContext
	facts.ProfileAuthority = plan.ProfileAuthority()
	facts.TargetPath = plan.ActivePath
	facts.DataRoot = data
	facts.ClientID = string(plan.ClientID)
	facts.Scope = string(plan.Scope)
	facts.SelectedDelivery = plan.SelectedDelivery
	if envelope.TreeDigest != "" {
		facts.TreeDigest = envelope.TreeDigest
	}
	if facts.InstallationID != "" {
		facts.BindingID = domain.ComputeClientBindingID(facts.InstallationID, facts.ClientID, facts.Scope, plan.ActivePath)
	}
	projected, err := projectArgs(envelope, s.serverName, callback, facts)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	if s.profileCheck != nil {
		if err := s.profileCheck(ctx); err != nil {
			return domain.StagedDelivery{}, err
		}
	}
	return s.Stager.StageWithPluginData(ctx, projected, plan, operationID, hints, data)
}

func projectArgs(envelope domain.PackageEnvelope, serverName string, args func(BindingFacts) ([]string, error), facts BindingFacts) (domain.PackageEnvelope, error) {
	if serverName == "" || args == nil {
		return envelope, nil
	}
	if err := facts.SelectedDelivery.Validate(); err != nil {
		return domain.PackageEnvelope{}, err
	}
	if _, selected := facts.SelectedDelivery.CursorFacts(); selected {
		if err := facts.SelectedDelivery.ValidateClient(domain.ClientID(facts.ClientID)); err != nil {
			return domain.PackageEnvelope{}, err
		}
		if !filepath.IsAbs(facts.DataRoot) || filepath.Clean(facts.DataRoot) != facts.DataRoot || facts.DataRoot == string(filepath.Separator) {
			return domain.PackageEnvelope{}, fmt.Errorf("cursor MCP projection requires owned plugin data")
		}
	} else if selectedNativeOnly(facts.SelectedDelivery) {
		return envelope, nil
	}
	raw, err := json.Marshal(envelope.MCP.Servers)
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	var servers map[string]domain.MCPServer
	if err := json.Unmarshal(raw, &servers); err != nil {
		return domain.PackageEnvelope{}, err
	}
	server, ok := servers[serverName]
	if !ok {
		return domain.PackageEnvelope{}, fmt.Errorf("package is missing declared MCP server %s", serverName)
	}
	if _, selected := facts.SelectedDelivery.CursorFacts(); selected && (server.Type != "stdio" || server.Decoded == nil) {
		return domain.PackageEnvelope{}, fmt.Errorf("cursor args projection requires declared stdio MCP server %s", serverName)
	}
	if server.Decoded == nil {
		server.Decoded = map[string]any{}
	}
	facts.ProfileAuthority = domain.CloneProfileAuthority(facts.ProfileAuthority)
	replacement, err := args(facts)
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	copied := make([]any, len(replacement))
	for i, item := range replacement {
		copied[i] = item
	}
	server.Decoded["args"] = copied
	server.Raw, err = json.Marshal(server.Decoded)
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	servers[serverName] = server
	envelope.MCP.Servers = servers
	return envelope, nil
}
