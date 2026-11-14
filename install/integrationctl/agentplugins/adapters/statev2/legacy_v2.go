package statev2

import (
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These types intentionally freeze the exact state schema written by
// agentplugins 0.1.4 and 0.1.5. Do not add v3 fields here.
type legacyStateFileV2 struct {
	SchemaVersion int                    `json:"schema_version"`
	Installations []legacyInstallationV2 `json:"installations"`
}

type legacyInstallationV2 struct {
	InstallationID string                           `json:"installation_id"`
	DeclaredName   string                           `json:"declared_name"`
	Source         legacySourceBindingV2            `json:"source"`
	Package        legacyPackageBindingV2           `json:"package"`
	Clients        map[string]legacyClientBindingV2 `json:"clients"`
	NeedsRebind    bool                             `json:"needs_rebind,omitempty"`
	CreatedAt      string                           `json:"created_at"`
	UpdatedAt      string                           `json:"updated_at"`
}

type legacySourceBindingV2 struct {
	SourceBindingID  string `json:"source_binding_id"`
	RequestedSource  string `json:"requested_source"`
	CanonicalSource  string `json:"canonical_source"`
	Repository       string `json:"repository,omitempty"`
	PackageSubpath   string `json:"package_subpath,omitempty"`
	ResolvedRevision string `json:"resolved_revision"`
	TreeDigest       string `json:"tree_digest"`
	Publisher        string `json:"publisher,omitempty"`
}

type legacyPackageBindingV2 struct {
	LoaderKind     string                     `json:"loader_kind"`
	FormatID       string                     `json:"format_id"`
	SchemaURI      string                     `json:"schema_uri"`
	DeclaredName   string                     `json:"declared_name"`
	Version        string                     `json:"version,omitempty"`
	ManifestDigest string                     `json:"manifest_digest"`
	Inventory      legacyComponentInventoryV2 `json:"inventory"`
}

type legacyComponentInventoryV2 struct {
	MCPPresent        bool     `json:"mcp_present"`
	MCPEnabled        bool     `json:"mcp_enabled"`
	MCPServers        []string `json:"mcp_servers,omitempty"`
	InvalidMCPServer  []string `json:"invalid_mcp_servers,omitempty"`
	Skills            []string `json:"skills,omitempty"`
	InvalidSkills     []string `json:"invalid_skills,omitempty"`
	InvalidSkillsRoot bool     `json:"invalid_skills_root,omitempty"`
	Extensions        []string `json:"extensions,omitempty"`
}

type legacyClientBindingV2 struct {
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
	PackageRevision  *legacyClientPackageRevisionV2 `json:"package_revision,omitempty"`
	NativeObjects    []legacyNativeObjectV2         `json:"native_objects,omitempty"`
	Receipts         []legacyMutationReceiptV2      `json:"receipts,omitempty"`
	UpdatedAt        string                         `json:"updated_at"`
}

type legacyClientPackageRevisionV2 struct {
	Version          string `json:"version,omitempty"`
	ResolvedRevision string `json:"resolved_revision,omitempty"`
	TreeDigest       string `json:"tree_digest"`
	ManifestDigest   string `json:"manifest_digest"`
}

type legacyNativeObjectV2 struct {
	ObjectID        string `json:"object_id"`
	Kind            string `json:"kind"`
	LogicalName     string `json:"logical_name,omitempty"`
	Path            string `json:"path,omitempty"`
	BeforeDigest    string `json:"before_digest,omitempty"`
	ManagedDigest   string `json:"managed_digest,omitempty"`
	ProtectionClass string `json:"protection_class"`
	UserModified    bool   `json:"user_modified,omitempty"`
}

type legacyMutationReceiptV2 struct {
	OperationID     string `json:"operation_id"`
	Sequence        int    `json:"sequence"`
	MutationType    string `json:"mutation_type"`
	ClientBindingID string `json:"client_binding_id"`
	ActivePath      string `json:"active_path,omitempty"`
	StagingPath     string `json:"staging_path,omitempty"`
	BackupPath      string `json:"backup_path,omitempty"`
	BeforeDigest    string `json:"before_digest,omitempty"`
	AfterDigest     string `json:"after_digest,omitempty"`
	Phase           string `json:"phase"`
}

func decodeLegacyStateV2(body []byte) (domain.StateFileV2, error) {
	var legacy legacyStateFileV2
	if err := decodeStrictJSON(body, &legacy); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("decode legacy state v2: %w", err)
	}
	legacy.SchemaVersion = domain.StateSchemaVersion
	converted, err := json.Marshal(legacy)
	if err != nil {
		return domain.StateFileV2{}, fmt.Errorf("encode legacy state v2 migration: %w", err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(converted, &state); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("convert legacy state v2: %w", err)
	}
	return state, nil
}
