package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ManagedMCPNames reads delivery selection only from a digest-verified artifact.
// A second verification detects artifact drift during the read. It does not
// promise protection from same-principal ABA races.
func (stager Stager) ManagedMCPNames(ctx context.Context, client domain.ClientID, root, digest string) ([]string, error) {
	return stager.managedMCPNames(ctx, client, "", root, digest)
}

// ManagedMCPNamesForScope reads the explicitly scoped selection without changing
// the existing unscoped API. Refused scoped policy never retries another layout.
func (stager Stager) ManagedMCPNamesForScope(ctx context.Context, client domain.ClientID, scope domain.InstallScope, root, digest string) ([]string, error) {
	if scope != domain.ScopeUser && scope != domain.ScopeProject {
		return nil, fmt.Errorf("managed MCP selection requires an explicit user or project scope")
	}
	return stager.managedMCPNames(ctx, client, scope, root, digest)
}

func (stager Stager) managedMCPNames(ctx context.Context, client domain.ClientID, scope domain.InstallScope, root, digest string) ([]string, error) {
	if err := stager.requireDeps(); err != nil {
		return nil, err
	}
	if err := stager.Verify(ctx, root, digest); err != nil {
		return nil, err
	}
	layout, err := stager.managedSelectionLayout(client, scope)
	if err != nil {
		return nil, err
	}
	names, err := readSelectionNames(root, layout)
	if err != nil {
		return nil, err
	}
	if err := stager.Verify(ctx, root, digest); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func (stager Stager) managedSelectionLayout(client domain.ClientID, scope domain.InstallScope) (clients.SelectionLayout, error) {
	if scope != "" {
		if reader, ok := clients.As[clients.ScopedSelectionReader](stager.Registry, client); ok {
			return reader.ManagedMCPSelectionForScope(scope)
		}
	}
	if reader, ok := clients.As[clients.SelectionReader](stager.Registry, client); ok {
		return reader.ManagedMCPSelection(), nil
	}
	return clients.DefaultSelectionLayout, nil
}

func readSelectionNames(root string, layout clients.SelectionLayout) ([]string, error) {
	candidate := filepath.Join(root, layout.File)
	info, statErr := os.Lstat(candidate)
	if statErr != nil {
		if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("managed MCP selection is not a regular file")
	} else if body, err := os.ReadFile(candidate); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		return selectionServerNames(body, layout)
	}
	return []string{}, nil
}

func selectionServerNames(body []byte, layout clients.SelectionLayout) ([]string, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, fmt.Errorf("managed MCP selection is invalid: %w", err)
	}
	if document == nil {
		return nil, fmt.Errorf("managed MCP selection is invalid: expected an object")
	}
	servers := document
	if layout.Nested {
		raw, ok := document["mcpServers"]
		if !ok {
			if !layout.ServersMemberOptional {
				return nil, fmt.Errorf("managed MCP selection lacks mcpServers")
			}
			return []string{}, nil
		}
		servers = nil
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, fmt.Errorf("managed MCP server selection is invalid: %w", err)
		}
		if servers == nil {
			return nil, fmt.Errorf("managed MCP server selection is invalid: expected an object")
		}
	}
	names := []string{}
	for name := range servers {
		names = append(names, name)
	}
	return names, nil
}
