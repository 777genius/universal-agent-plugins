package conformance

import (
	_ "embed"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type ProfileIdentity struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Digest   string `json:"digest"`
}

// ProfileIdentities pins reviewed prose, schema truth and finite parsing policy.
// Rule mappings and interpretive decisions accompany these identities in profiles/README.md.
func ProfileIdentities() []ProfileIdentity {
	return []ProfileIdentity{
		{"agent-plugins/1.0.0", "ff8ab5e392cc87bd88d87c060815a87490e51003", "sha256:97a658b7dca3ce1b4c2266b95da300fa51d9dc4ade59d73168e5f9104272da18"},
		{"agent-skills/2026-09-06", "69ef37e9424c0a7ea9dd2293b559e43ec8176379", "sha256:b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220"},
		{domain.PluginSchemaV1, "1.0.0", "sha256:0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883"},
		{domain.MCPSchemaV1, "1.0.0", "sha256:6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb"},
		{"author-document-bounds/v1", "1", sha256Digest(profileRules)},
	}
}
func (d Decoder) registryReady(uri string) bool {
	if d.Registry == nil || !d.Registry.Supports(uri) {
		return false
	}
	registry, ok := d.Registry.(interface{ Digest(string) (string, bool) })
	if !ok {
		return false
	}
	digest, ok := registry.Digest(uri)
	if !ok {
		return false
	}
	for _, p := range ProfileIdentities() {
		if p.ID == uri {
			return p.Digest == digest
		}
	}
	return false
}
func schemaViolation(err error) bool {
	var validation *jsonschema.ValidationError
	return errors.As(err, &validation)
}

//go:embed profiles/README.md
var profileRules []byte
