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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	receiptPhaseStateCommitted = "state_committed"
	receiptPhaseCommitted      = "committed"
)

var fullLowercaseSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Store struct {
	// ProjectRoot is operational context; machine state copied across projects is rejected.
	ProjectRoot string `json:"-"`
	Path        string
}

func (store Store) SchemaVersion() (int, error) {
	if err := store.validateProjectPaths(); err != nil {
		return 0, err
	}
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
		return 0, fmt.Errorf("%w: decode state header: %w", ErrStateCorrupt, err)
	}
	return header.SchemaVersion, nil
}

func (store Store) RequireMutationReady() error {
	version, err := store.SchemaVersion()
	if err != nil {
		return err
	}
	if version >= domain.LegacyStateSchemaVersion && version < domain.StateSchemaVersion {
		return &MigrationRequiredError{SourceSchema: version, TargetSchema: domain.StateSchemaVersion}
	}
	if version != domain.StateSchemaVersion {
		return &UnsupportedStateSchemaError{SourceSchema: version, TargetSchema: domain.StateSchemaVersion}
	}
	return nil
}

func managedBindingDigest(binding domain.ClientBinding) string {
	if binding.PackageRevision != nil && binding.PackageRevision.TreeDigest != "" {
		return binding.PackageRevision.TreeDigest
	}
	for _, object := range binding.NativeObjects {
		if object.ManagedDigest != "" {
			return object.ManagedDigest
		}
	}
	return ""
}

func (store Store) Load() (domain.StateFileV2, error) {
	if err := store.validateProjectPaths(); err != nil {
		return domain.StateFileV2{}, err
	}
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
		return domain.StateFileV2{}, fmt.Errorf("%w: decode state v2 header: %w", ErrStateCorrupt, err)
	}
	var state domain.StateFileV2
	switch header.SchemaVersion {
	case domain.LegacyStateSchemaVersion:
		state, err = decodeLegacyStateV2(body)
		normalizeCurrentState(&state)
	case domain.PreviousStateSchemaVersion:
		state, err = decodeLegacyStateV3(body)
		normalizeCurrentState(&state)
	case domain.PreviousStateSchemaV4:
		err = decodeStrictJSON(body, &state)
		if len(state.EnvironmentReceipts) != 0 {
			return domain.StateFileV2{}, fmt.Errorf("%w: schema 4 cannot contain environment ownership", ErrStateCorrupt)
		}
		state.SchemaVersion = domain.StateSchemaVersion
		normalizeCurrentState(&state)
	case domain.StateSchemaVersion:
		err = decodeStrictJSON(body, &state)
	default:
		return domain.StateFileV2{}, &UnsupportedStateSchemaError{SourceSchema: header.SchemaVersion, TargetSchema: domain.StateSchemaVersion}
	}
	if err != nil {
		return domain.StateFileV2{}, fmt.Errorf("%w: %w", ErrStateCorrupt, err)
	}
	if err := Validate(state); err != nil {
		return domain.StateFileV2{}, fmt.Errorf("%w: validate state: %w", ErrStateCorrupt, err)
	}
	if err := store.validateProject(state); err != nil {
		return domain.StateFileV2{}, err
	}
	return state, nil
}

func (store Store) Save(state domain.StateFileV2) error {
	if err := store.validateProjectPaths(); err != nil {
		return err
	}
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
	if err := store.validateProject(state); err != nil {
		return err
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
	if err := rejectShadowedObservations(body); err != nil {
		return err
	}
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
