package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

const enablementRecordPrefix = "grok-enable-"
const enablementStagePrefix = ".grok-enable-stage-"
const enablementRecordLimit = 32 << 10
const enabledMemberKind = "grok_enabled_plugin"

// The private record describes one operation in the existing operation root.
// No native configuration body, previous state body or credentials are stored.
type enablementRecord struct {
	Version             int                           `json:"version"`
	Action              string                        `json:"action"`
	OperationID         string                        `json:"operation_id"`
	InstallationID      string                        `json:"installation_id"`
	BindingID           string                        `json:"binding_id"`
	ProjectRoot         string                        `json:"project_root"`
	UserRoot            string                        `json:"user_root"`
	UserParentAuthority directoryidentity.Authority   `json:"user_parent_authority"`
	ArtifactPath        string                        `json:"artifact_path"`
	PackageDigest       string                        `json:"package_digest"`
	BeforeStateDigest   string                        `json:"before_state_digest"`
	AttemptStateDigest  string                        `json:"attempt_state_digest"`
	DesiredStateDigest  string                        `json:"desired_state_digest"`
	PluginID            string                        `json:"plugin_id"`
	PluginName          string                        `json:"plugin_name"`
	Edit                enablementEdit                `json:"edit"`
	Hash                string                        `json:"hash"`
	PreviousObject      *domain.NativeObjectOwnership `json:"previous_object,omitempty"`
	decisionDigest      string
	footer              []byte
	headerBytes         int
	rawDigest           string
}

// EnablementRecovery is composed beside the existing transaction recovery.
// Its caller holds the existing mutation lock for the selected namespace.
type EnablementRecovery struct {
	ProjectRoot, UserRoot, OperationsDir string
	State                                ports.NativeTransitionState
	Kernel                               nativeconfig.Kernel
	journalPoint                         func(string) error
	PackageVerifier                      interface {
		Verify(context.Context, string, string) error
	}
}

func (record enablementRecord) object() domain.NativeObjectOwnership {
	if record.PreviousObject != nil {
		return *record.PreviousObject
	}
	return domain.NativeObjectOwnership{ObjectID: "grok-enabled:" + enablementDigest([]byte(record.UserRoot))[7:] + ":" + record.PluginID, Kind: enabledMemberKind,
		LogicalName: record.PluginName, Path: record.ArtifactPath,
		BeforeDigest: record.Edit.BeforeDigest, ManagedDigest: record.Edit.AfterDigest,
		ProtectionClass: "native_config_member"}
}

func enablementStateDigest(state domain.StateFileV2) string {
	digest, _ := transaction.ComparableStateDigest(state)
	return digest
}

func enablementBinding(state domain.StateFileV2, installationID, bindingID string) (int, domain.Installation, domain.ClientBinding, error) {
	for index, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		binding, found := installation.Clients[bindingID]
		if found && binding.ClientBindingID == bindingID && binding.ClientID == "grok" && binding.Scope == "project" {
			return index, installation, binding, nil
		}
	}
	return -1, domain.Installation{}, domain.ClientBinding{}, fmt.Errorf("grok enablement binding is absent or changed")
}

func (recovery EnablementRecovery) validateRecord(record enablementRecord) error {
	if record.Version != 2 || recovery.ProjectRoot == "" || recovery.UserRoot == "" ||
		record.ProjectRoot != recovery.ProjectRoot || record.UserRoot != recovery.UserRoot ||
		!enablementHash(record.PackageDigest) || !enablementHash(record.BeforeStateDigest) ||
		!enablementHash(record.AttemptStateDigest) || record.DesiredStateDigest != "" && !enablementHash(record.DesiredStateDigest) {
		return fmt.Errorf("grok enablement operation namespace or authority differs")
	}
	if err := record.validateParent(); err != nil {
		return err
	}
	for _, id := range []string{record.OperationID, record.InstallationID, record.BindingID, record.PluginName} {
		if err := pathpolicy.ValidateLeafID(id); err != nil {
			return err
		}
	}
	if err := recovery.validateRecordPaths(record); err != nil {
		return err
	}
	if err := record.validateAction(); err != nil {
		return err
	}
	return record.Edit.validate(record.PluginID)
}

func (recovery EnablementRecovery) validateRecordPaths(record enablementRecord) error {
	root := filepath.Join(record.ProjectRoot, ".grok", "plugins")
	if filepath.Dir(record.ArtifactPath) != root || !filepath.IsAbs(root) || filepath.Clean(root) != root ||
		record.PluginID != "project/"+enablementDigest([]byte(record.ArtifactPath))[7:15]+"/"+record.PluginName {
		return fmt.Errorf("grok enablement native ID or package source differs")
	}
	for _, root := range []string{record.ProjectRoot, record.UserRoot, recovery.OperationsDir} {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return fmt.Errorf("grok enablement root is not absolute and clean")
		}
	}
	return nil
}

func (recovery EnablementRecovery) recordRoot(operationID string) (string, error) {
	if err := pathpolicy.ValidateLeafID(operationID); err != nil {
		return "", err
	}
	path := filepath.Join(recovery.OperationsDir, enablementRecordPrefix+operationID)
	return path, pathpolicy.RequireContainedChild(recovery.OperationsDir, path)
}

func (recovery EnablementRecovery) writeRecord(record enablementRecord) error {
	if err := recovery.validateRecord(record); err != nil {
		return err
	}
	record.Hash = ""
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	record.Hash = enablementDigest(body)
	body, err = json.Marshal(record)
	if err != nil || len(body) > enablementRecordLimit-enablementDecisionLength {
		return fmt.Errorf("grok enablement record exceeds its bound: %w", err)
	}
	return recovery.publishRecord(record, body)
}

func (recovery EnablementRecovery) readRecord(operationID string) (enablementRecord, error) {
	body, err := recovery.readRecordBytes(operationID)
	if err != nil {
		return enablementRecord{}, err
	}
	return recovery.decodeRecord(operationID, body)
}

func (recovery EnablementRecovery) decodeRecord(operationID string, body []byte) (enablementRecord, error) {
	var record enablementRecord
	header, footer, _ := bytes.Cut(body, []byte("\n"))
	if len(header) != len(body) {
		footer = append([]byte("\n"), footer...)
	}
	decoder := json.NewDecoder(bytes.NewReader(header))
	if _, err := shared.DecodeUniqueJSONValue(decoder); err != nil {
		return record, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return record, fmt.Errorf("grok enablement record contains trailing data")
	}
	decoder = json.NewDecoder(bytes.NewReader(header))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, err
	}
	got := record.Hash
	record.Hash = ""
	unsigned, err := json.Marshal(record)
	if err != nil {
		return record, err
	}
	if record.OperationID != operationID || got != enablementDigest(unsigned) {
		return record, fmt.Errorf("grok enablement record integrity differs")
	}
	record.Hash = got
	record.rawDigest, record.footer, record.headerBytes = enablementDigest(body), footer, len(header)
	if err := recovery.validateRecord(record); err != nil {
		return record, err
	}
	err = record.decodeDecision()
	return record, err
}

func (recovery EnablementRecovery) Recover(ctx context.Context) error {
	if recovery.State == nil || recovery.PackageVerifier == nil {
		return fmt.Errorf("grok enablement recovery services are unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	pending, err := recovery.Pending()
	if err != nil {
		return err
	}
	if len(pending) != 0 {
		return errEnablementUnqualified
	}
	return nil
}
