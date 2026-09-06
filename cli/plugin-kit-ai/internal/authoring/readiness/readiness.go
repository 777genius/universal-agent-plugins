// Package readiness evaluates retained project evidence without host discovery,
// filesystem access, process execution, or installer lifecycle dependencies.
package readiness

import (
	"errors"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

var ErrTargets = errors.New("select distinct explicit registry clients")

// Targets rejects ambiguous selections before any project read. Neither aliases
// nor 'all' imply ambient detection. Limits bound even malicious CLI input.
func Targets(value string) ([]domain.ClientID, error) {
	if value == "" || len(value) > 4096 {
		return nil, ErrTargets
	}
	parts := strings.Split(value, ",")
	if len(parts) > len(domain.ClientDefinitions()) {
		return nil, ErrTargets
	}
	seen := map[domain.ClientID]bool{}
	ids := make([]domain.ClientID, 0, len(parts))
	for _, part := range parts {
		id := domain.ClientID(part)
		if seen[id] || !domain.IsSupportedClient(id) {
			return nil, ErrTargets
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// Compatibility does not turn unusable/unknown-schema input into an empty,
// successful package. Valid siblings remain available on partial input.
func Compatibility(p project.Result, ids []domain.ClientID) ([]planner.ClientCompatibility, error) {
	if p.Facts.Package == nil {
		return nil, nil
	}
	return planner.Compatibility(*p.Facts.Package, ids)
}

type Schema struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}
type Capabilities struct {
	Schemas  []Schema                      `json:"schemas"`
	Profiles []conformance.ProfileIdentity `json:"profiles"`
	Clients  []domain.ClientCapabilities   `json:"clients"`
	Commands []string                      `json:"commands"`
	Evidence []string                      `json:"evidence_limits"`
}

// Engine uses embedded schema checksums, shared profiles and the live registry.
// Commands must come from the actual registration table, never a roadmap list.
func Engine(commands []string) (*Capabilities, error) {
	registry, err := specregistry.New()
	if err != nil {
		return nil, err
	}
	c := &Capabilities{Profiles: conformance.ProfileIdentities(), Commands: append([]string{}, commands...),
		Evidence: []string{"static_only", "no_path_lookup", "no_executable_version_probe", "no_runtime_or_oauth_evidence", "native_files_metadata_only"}}
	for _, id := range []string{domain.PluginSchemaV1, domain.MCPSchemaV1} {
		digest, ok := registry.Digest(id)
		if !ok || !registry.Supports(id) {
			return nil, errors.New("embedded schema unavailable")
		}
		c.Schemas = append(c.Schemas, Schema{id, digest})
	}
	for _, def := range domain.ClientDefinitions() {
		c.Clients = append(c.Clients, def.Capabilities)
	}
	sort.Strings(c.Commands)
	sort.Slice(c.Clients, func(i, j int) bool { return c.Clients[i].ClientID < c.Clients[j].ClientID })
	return c, nil
}
