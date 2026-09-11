package statev2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var fullLowercaseSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Store struct {
	Path string
}

func (store Store) SchemaVersion() (int, error) {
	body, err := os.ReadFile(store.Path)
	if os.IsNotExist(err) {
		return domain.StateSchemaVersion, nil
	}
	if err != nil {
		return 0, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return 0, fmt.Errorf("decode state header: %w", err)
	}
	return header.SchemaVersion, nil
}

func (store Store) RequireMutationReady() error {
	version, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	if version == domain.LegacyStateSchemaVersion || version == domain.PreviousStateSchemaVersion {
		return fmt.Errorf("state schema %d requires explicit migration; run agentplugins migrate-state --dry-run, then agentplugins migrate-state", version)
	}
	if version != domain.StateSchemaVersion {
		return fmt.Errorf("unsupported state schema_version %d", version)
	}
	return nil
}

func (store Store) Load() (domain.StateFileV2, error) {
	if strings.TrimSpace(store.Path) == "" {
		return domain.StateFileV2{}, fmt.Errorf("state v2 path is required")
	}
	body, err := os.ReadFile(store.Path)
	if os.IsNotExist(err) {
		return domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}, nil
	}
	if err != nil {
		return domain.StateFileV2{}, fmt.Errorf("read state v2: %w", err)
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("decode state v2 header: %w", err)
	}
	var state domain.StateFileV2
	switch header.SchemaVersion {
	case domain.LegacyStateSchemaVersion:
		state, err = decodeLegacyStateV2(body)
		normalizeCurrentState(&state)
	case domain.PreviousStateSchemaVersion:
		state, err = decodeLegacyStateV3(body)
		normalizeCurrentState(&state)
	case domain.StateSchemaVersion:
		err = decodeStrictJSON(body, &state)
	default:
		return domain.StateFileV2{}, fmt.Errorf("unsupported state schema_version %d; this build reads %d, %d, and %d and writes only %d", header.SchemaVersion, domain.LegacyStateSchemaVersion, domain.PreviousStateSchemaVersion, domain.StateSchemaVersion, domain.StateSchemaVersion)
	}
	if err != nil {
		return domain.StateFileV2{}, err
	}
	if err := Validate(state); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("validate state: %w", err)
	}
	return state, nil
}

func (store Store) Save(state domain.StateFileV2) error {
	if strings.TrimSpace(store.Path) == "" {
		return fmt.Errorf("state v2 path is required")
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = domain.StateSchemaVersion
	}
	normalizeCurrentState(&state)
	if err := Validate(state); err != nil {
		return fmt.Errorf("refuse invalid state: %w", err)
	}
	state.Installations = append([]domain.Installation(nil), state.Installations...)
	sort.Slice(state.Installations, func(i, j int) bool {
		return state.Installations[i].InstallationID < state.Installations[j].InstallationID
	})
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	parent := filepath.Dir(store.Path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create state v2 directory: %w", err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return fmt.Errorf("protect state v2 directory: %w", err)
	}
	return atomicfile.Write(store.Path, body, 0o600)
}

// normalizeCurrentState keeps programmatic callers compiled against the v2
// model from accidentally writing an origin-ambiguous v3 document. Historical
// callers can only describe direct provenance; Directory provenance is never
// invented here and must be supplied explicitly by resolution or migration.
func normalizeCurrentState(state *domain.StateFileV2) {
	for index := range state.Installations {
		installation := &state.Installations[index]
		if installation.OriginMode == "" {
			installation.OriginMode = domain.OriginModeDirect
		}
		if installation.Clients == nil {
			installation.Clients = map[string]domain.ClientBinding{}
		}
	}
}

func decodeStrictJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("state contains trailing JSON values")
	}
	return nil
}

func Validate(state domain.StateFileV2) error {
	if state.SchemaVersion != domain.StateSchemaVersion {
		return fmt.Errorf("schema_version must be %d", domain.StateSchemaVersion)
	}
	installationIDs := map[string]struct{}{}
	sourceBindingIDs := map[string]struct{}{}
	clientBindingIDs := map[string]struct{}{}
	operationIDs := map[string]struct{}{}
	for index, installation := range state.Installations {
		if mapping := installation.LocalChatGPTMapping; mapping != nil {
			if err := mapping.Validate(); err != nil && !mapping.IsLegacyContext7Registration() {
				return err
			}
			if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil || installation.Directory.ProductID != mapping.ProductID || installation.Directory.DistributionID != mapping.Repository || installation.Source.Repository != mapping.Repository || installation.Source.PackageSubpath != mapping.PackagePath {
				return fmt.Errorf("personal ChatGPT receipt source does not match installation")
			}
		}
		prefix := fmt.Sprintf("installations[%d]", index)
		if err := pathpolicy.ValidateLeafID(installation.InstallationID); err != nil {
			return fmt.Errorf("%s has invalid installation_id: %w", prefix, err)
		}
		if _, duplicate := installationIDs[installation.InstallationID]; duplicate {
			return fmt.Errorf("duplicate installation_id %q", installation.InstallationID)
		}
		installationIDs[installation.InstallationID] = struct{}{}
		if strings.TrimSpace(installation.DeclaredName) == "" || installation.DeclaredName != installation.Package.DeclaredName {
			return fmt.Errorf("%s declared_name does not match package binding", prefix)
		}
		if installation.OperationGroupID != "" {
			if err := pathpolicy.ValidateLeafID(installation.OperationGroupID); err != nil {
				return fmt.Errorf("%s has invalid operation group id: %w", prefix, err)
			}
		}
		switch installation.OriginMode {
		case domain.OriginModeDirect:
			if installation.Directory != nil {
				return fmt.Errorf("%s direct origin must not contain Directory provenance", prefix)
			}
		case domain.OriginModeDirectory:
			if installation.Directory == nil {
				return fmt.Errorf("%s directory origin is missing Directory provenance", prefix)
			}
			directory := installation.Directory
			if strings.TrimSpace(directory.ProductID) == "" || strings.TrimSpace(directory.DistributionID) == "" || directory.DesiredReleaseSequence < 1 {
				return fmt.Errorf("%s directory release identity is incomplete", prefix)
			}
			if !fullLowercaseSHA.MatchString(installation.Source.ResolvedRevision) {
				return fmt.Errorf("%s Directory source resolved_revision must be a full lowercase 40-character SHA", prefix)
			}
			switch directory.DistributionKind {
			case domain.DistributionUpstream, domain.DistributionCommunityBridge, domain.DistributionCommunity:
			default:
				return fmt.Errorf("%s directory distribution kind is invalid", prefix)
			}
			if (directory.SnapshotSequence > 0 || directory.SnapshotSchema > 0 || directory.SnapshotDigest != "") &&
				(directory.SnapshotSequence < 1 || directory.SnapshotSchema < 1 || strings.TrimSpace(directory.SnapshotDigest) == "") {
				return fmt.Errorf("%s Directory snapshot provenance is incomplete", prefix)
			}
		default:
			return fmt.Errorf("%s origin_mode must be directory or direct", prefix)
		}
		preferences := map[string]bool{}
		for _, preference := range installation.InstallPreferences {
			if err := preference.InstallIntent.Validate(preference.ClientID); err != nil {
				return err
			}
			key := string(preference.ClientID) + ":" + string(preference.Scope)
			if preference.InstallIntent != domain.InstallIntentPrepare || preference.Scope != domain.ScopeUser || preferences[key] {
				return fmt.Errorf("invalid or duplicate install preference %q", key)
			}
			preferences[key] = true
		}
		if installation.DataRetained && len(installation.Clients) != 0 {
			return fmt.Errorf("%s data_retained installation must have no client bindings", prefix)
		}
		if installation.DataRetained && len(installation.DataReceipts) == 0 {
			return fmt.Errorf("%s data_retained installation has no data receipts", prefix)
		}
		for receiptKey, receipt := range installation.DataReceipts {
			if receiptKey == "" || receiptKey != receipt.DataReceiptID {
				return fmt.Errorf("%s data receipt map key does not match data_receipt_id", prefix)
			}
			if err := pathpolicy.ValidateLeafID(receipt.DataReceiptID); err != nil {
				return fmt.Errorf("%s has invalid data receipt id: %w", prefix, err)
			}
			if strings.TrimSpace(receipt.PhysicalBackend) == "" || strings.TrimSpace(receipt.Scope) == "" || strings.TrimSpace(receipt.Locator) == "" || strings.TrimSpace(receipt.OwnershipDigest) == "" {
				return fmt.Errorf("%s data receipt %q is incomplete", prefix, receipt.DataReceiptID)
			}
			switch receipt.State {
			case domain.DataReceiptOwned, domain.DataReceiptUnknown, domain.DataReceiptStale:
			default:
				return fmt.Errorf("%s data receipt %q has invalid state", prefix, receipt.DataReceiptID)
			}
		}
		if strings.TrimSpace(installation.Source.SourceBindingID) == "" {
			return fmt.Errorf("%s source_binding_id is required", prefix)
		}
		if _, duplicate := sourceBindingIDs[installation.Source.SourceBindingID]; duplicate {
			return fmt.Errorf("duplicate source_binding_id %q", installation.Source.SourceBindingID)
		}
		sourceBindingIDs[installation.Source.SourceBindingID] = struct{}{}
		if installation.Package.LoaderKind == domain.LoaderKindAgentPlugins {
			if installation.Package.FormatID == "" || installation.Source.TreeDigest == "" || installation.Package.ManifestDigest == "" {
				return fmt.Errorf("%s standard package binding is incomplete", prefix)
			}
			if installation.Package.FormatID == domain.FormatIDAgentPluginsV1 && installation.Package.SchemaURI == "" {
				return fmt.Errorf("%s portable standard package binding has no schema URI", prefix)
			}
		}
		for mapKey, client := range installation.Clients {
			if mapKey == "" || mapKey != client.ClientBindingID {
				return fmt.Errorf("%s client map key does not match client_binding_id", prefix)
			}
			if err := pathpolicy.ValidateLeafID(client.ClientBindingID); err != nil {
				return fmt.Errorf("%s has invalid client_binding_id: %w", prefix, err)
			}
			if _, duplicate := clientBindingIDs[client.ClientBindingID]; duplicate {
				return fmt.Errorf("duplicate client_binding_id %q", client.ClientBindingID)
			}
			clientBindingIDs[client.ClientBindingID] = struct{}{}
			if client.InstallIntent == domain.InstallIntentPrepare && client.Scope != string(domain.ScopeUser) {
				return fmt.Errorf("prepare binding requires user scope")
			}
			if err := client.InstallIntent.Validate(domain.ClientID(client.ClientID)); err != nil {
				return err
			}
			if strings.TrimSpace(client.ClientID) == "" || strings.TrimSpace(client.TargetLocator) == "" {
				return fmt.Errorf("%s client binding %q is incomplete", prefix, client.ClientBindingID)
			}
			if err := pathpolicy.ValidateLeafID(client.PhysicalArtifact); err != nil {
				return fmt.Errorf("%s client binding %q has invalid physical artifact id: %w", prefix, client.ClientBindingID, err)
			}
			if !validMaterializationState(client.Materialization) ||
				!validActivationState(client.Activation) ||
				!validAuthenticationState(client.Authentication) ||
				!validPolicyState(client.Policy) ||
				!validVerificationState(client.Verification) {
				return fmt.Errorf("%s client binding %q contains an unknown lifecycle state", prefix, client.ClientBindingID)
			}
			if client.PackageRevision != nil && (strings.TrimSpace(client.PackageRevision.TreeDigest) == "" || strings.TrimSpace(client.PackageRevision.ManifestDigest) == "") {
				return fmt.Errorf("%s client binding %q has an incomplete package revision", prefix, client.ClientBindingID)
			}
			if installation.OriginMode == domain.OriginModeDirectory {
				if client.PackageRevision == nil {
					return fmt.Errorf("%s client binding %q has no applied Directory revision", prefix, client.ClientBindingID)
				}
				if client.PackageRevision.DistributionID != installation.Directory.DistributionID || client.PackageRevision.ReleaseSequence < 1 || client.PackageRevision.ReleaseSequence > installation.Directory.DesiredReleaseSequence {
					return fmt.Errorf("%s client binding %q has invalid applied Directory revision", prefix, client.ClientBindingID)
				}
				if !fullLowercaseSHA.MatchString(client.PackageRevision.ResolvedRevision) {
					return fmt.Errorf("%s client binding %q Directory resolved_revision must be a full lowercase 40-character SHA", prefix, client.ClientBindingID)
				}
			}
			if client.DataReceiptID != "" {
				if _, ok := installation.DataReceipts[client.DataReceiptID]; !ok {
					return fmt.Errorf("%s client binding %q references unknown data receipt", prefix, client.ClientBindingID)
				}
			}
			objectIDs := map[string]struct{}{}
			for _, object := range client.NativeObjects {
				if strings.TrimSpace(object.ObjectID) == "" || strings.TrimSpace(object.Kind) == "" {
					return fmt.Errorf("%s client binding %q has incomplete native ownership", prefix, client.ClientBindingID)
				}
				if _, duplicate := objectIDs[object.ObjectID]; duplicate {
					return fmt.Errorf("%s client binding %q has duplicate object_id %q", prefix, client.ClientBindingID, object.ObjectID)
				}
				objectIDs[object.ObjectID] = struct{}{}
			}
			for receiptIndex, receipt := range client.Receipts {
				if err := pathpolicy.ValidateLeafID(receipt.OperationID); err != nil {
					return fmt.Errorf("%s client binding %q receipt[%d] has invalid operation id: %w", prefix, client.ClientBindingID, receiptIndex, err)
				}
				if receipt.ClientBindingID != client.ClientBindingID || receipt.Sequence < 1 || receipt.Phase == "" {
					return fmt.Errorf("%s client binding %q receipt[%d] is incomplete", prefix, client.ClientBindingID, receiptIndex)
				}
				if receipt.OperationGroupID != "" {
					if err := pathpolicy.ValidateLeafID(receipt.OperationGroupID); err != nil {
						return fmt.Errorf("%s client binding %q receipt[%d] has invalid operation group id: %w", prefix, client.ClientBindingID, receiptIndex, err)
					}
				}
				if _, duplicate := operationIDs[receipt.OperationID]; duplicate {
					return fmt.Errorf("duplicate receipt operation_id %q", receipt.OperationID)
				}
				operationIDs[receipt.OperationID] = struct{}{}
			}
		}
	}
	return nil
}

func validMaterializationState(value domain.MaterializationState) bool {
	switch value {
	case domain.MaterializationAbsent,
		domain.MaterializationStaged,
		domain.MaterializationMaterialized,
		domain.MaterializationDegraded:
		return true
	default:
		return false
	}
}

func validActivationState(value domain.ActivationState) bool {
	switch value {
	case domain.ActivationNotRequired,
		domain.ActivationPrepared,
		domain.ActivationManual,
		domain.ActivationActive,
		domain.ActivationFailed:
		return true
	default:
		return false
	}
}

func validAuthenticationState(value domain.AuthenticationState) bool {
	switch value {
	case domain.AuthenticationNotRequired,
		domain.AuthenticationNotChecked,
		domain.AuthenticationPending,
		domain.AuthenticationComplete,
		domain.AuthenticationFailed:
		return true
	default:
		return false
	}
}

func validPolicyState(value domain.PolicyState) bool {
	switch value {
	case domain.PolicyAllowed,
		domain.PolicyBlocked,
		domain.PolicyApprovalRequired:
		return true
	default:
		return false
	}
}

func validVerificationState(value domain.VerificationState) bool {
	switch value {
	case domain.VerificationNotRun,
		domain.VerificationPackageValid,
		domain.VerificationInstalled,
		domain.VerificationRuntime,
		domain.VerificationFailed:
		return true
	default:
		return false
	}
}
