package statev2

import (
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These types freeze the schema 3 ChatGPT state. Lifecycle provenance fields
// introduced by schema 4 must be rejected under a schema 3 header.
type legacyStateFileV3 struct {
	SchemaVersion int                    `json:"schema_version"`
	Installations []legacyInstallationV3 `json:"installations"`
}

type legacyInstallationV3 struct {
	InstallationID string                           `json:"installation_id"`
	DeclaredName   string                           `json:"declared_name"`
	Source         legacySourceBindingV2            `json:"source"`
	Package        legacyPackageBindingV3           `json:"package"`
	Clients        map[string]legacyClientBindingV3 `json:"clients"`
	NeedsRebind    bool                             `json:"needs_rebind,omitempty"`
	CreatedAt      string                           `json:"created_at"`
	UpdatedAt      string                           `json:"updated_at"`
}

type legacyPackageBindingV3 struct {
	LoaderKind     string                    `json:"loader_kind"`
	FormatID       string                    `json:"format_id"`
	SchemaURI      string                    `json:"schema_uri"`
	DeclaredName   string                    `json:"declared_name"`
	Version        string                    `json:"version,omitempty"`
	ManifestDigest string                    `json:"manifest_digest"`
	Inventory      domain.ComponentInventory `json:"inventory"`
}

type legacyClientBindingV3 struct {
	ClientBindingID  string                         `json:"client_binding_id"`
	ClientID         string                         `json:"client_id"`
	Scope            string                         `json:"scope"`
	TargetLocator    string                         `json:"target_locator"`
	PhysicalArtifact string                         `json:"physical_artifact_id"`
	Materialization  domain.MaterializationState    `json:"materialization"`
	Activation       domain.ActivationState         `json:"activation"`
	Authentication   domain.AuthenticationState     `json:"authentication"`
	Policy           domain.PolicyState             `json:"policy"`
	Verification     domain.VerificationState       `json:"verification"`
	PackageRevision  *legacyClientPackageRevisionV3 `json:"package_revision,omitempty"`
	NativeObjects    []legacyNativeObjectV2         `json:"native_objects,omitempty"`
	Receipts         []legacyMutationReceiptV2      `json:"receipts,omitempty"`
	UpdatedAt        string                         `json:"updated_at"`
}

type legacyClientPackageRevisionV3 struct {
	Version          string                  `json:"version,omitempty"`
	ResolvedRevision string                  `json:"resolved_revision,omitempty"`
	TreeDigest       string                  `json:"tree_digest"`
	ManifestDigest   string                  `json:"manifest_digest"`
	CatalogEvidence  *domain.CatalogEvidence `json:"catalog_evidence,omitempty"`
}

func decodeLegacyStateV3(body []byte) (domain.StateFileV2, error) {
	var legacy legacyStateFileV3
	if err := decodeStrictJSON(body, &legacy); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("decode legacy state v3: %w", err)
	}
	legacy.SchemaVersion = domain.StateSchemaVersion
	converted, err := json.Marshal(legacy)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(converted, &state); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("convert legacy state v3: %w", err)
	}
	return state, nil
}
