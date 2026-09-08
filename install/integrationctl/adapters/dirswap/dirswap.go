package dirswap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

const receiptSchemaVersion = 3

const (
	OperationSwap   = "swap"
	OperationRemove = "remove"

	PhaseIntent            = "intent"
	PhaseBackupPending     = "backup_pending"
	PhaseOldBackedUp       = "old_backed_up"
	PhaseActivationPending = "activation_pending"
	PhaseActivated         = "activated"
	PhaseCommitPending     = "commit_pending"
	PhaseCommitted         = "committed"
	PhaseRollbackPending   = "rollback_pending"
	PhaseRolledBack        = "rolled_back"

	FaultBackupRenamed     = "backup_renamed"
	FaultActivationApplied = "activation_applied"
	FaultBackupRemoved     = "backup_removed"
	FaultRollbackRemoved   = "rollback_active_removed"
)

type Receipt struct {
	SchemaVersion     int    `json:"schema_version"`
	Operation         string `json:"operation"`
	OperationID       string `json:"operation_id"`
	ClientBindingID   string `json:"client_binding_id"`
	Sequence          int    `json:"sequence"`
	OwnedBase         string `json:"owned_base"`
	ActivePath        string `json:"active_path"`
	StagingPath       string `json:"staging_path,omitempty"`
	BackupPath        string `json:"backup_path"`
	HadActive         bool   `json:"had_active"`
	Phase             string `json:"phase"`
	PublishedIdentity string `json:"published_identity,omitempty"`
	PublishedDigest   string `json:"published_digest,omitempty"`
}

type Input struct {
	OperationID     string
	ClientBindingID string
	Sequence        int
	OwnedBase       string
	ActivePath      string
	StagingPath     string
	Remove          bool
	// RequireAbsent rejects the whole operation, before any journal write or
	// filesystem mutation, if ActivePath already exists. It exists for a caller
	// reconstructing a target it has independently confirmed absent: an earlier
	// absence check can go stale before this call runs, and normal Apply
	// semantics would otherwise treat newly appeared content as an existing
	// directory to back up and later discard on Commit. Publication also uses
	// an exclusive rename so a newly appeared target is never overwritten.
	RequireAbsent bool
}

type Manager struct {
	JournalDir string
	// Fault is a test seam around durable phases and otherwise unreachable
	// crash windows. Production callers leave it nil.
	Fault func(phase string) error
}

func (manager Manager) Apply(ctx context.Context, input Input) (Receipt, error) {
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}
	receipt, err := manager.newReceipt(input)
	if err != nil {
		return Receipt{}, err
	}
	if _, err := os.Lstat(manager.journalPath(receipt.OperationID)); err == nil {
		return Receipt{}, fmt.Errorf("directory swap operation %q already exists", receipt.OperationID)
	} else if !os.IsNotExist(err) {
		return Receipt{}, fmt.Errorf("inspect directory swap journal: %w", err)
	}
	if err := manager.save(receipt); err != nil {
		return Receipt{}, err
	}
	receipt.Phase = PhaseBackupPending
	if err := manager.save(receipt); err != nil {
		return receipt, err
	}
	if err := manager.inject(PhaseBackupPending); err != nil {
		return receipt, err
	}
	if receipt.HadActive {
		if err := os.Rename(receipt.ActivePath, receipt.BackupPath); err != nil {
			return receipt, fmt.Errorf("move active directory to backup: %w", err)
		}
		if err := atomicfile.SyncDirectory(receipt.OwnedBase); err != nil {
			return receipt, fmt.Errorf("sync active parent after backup rename: %w", err)
		}
	}
	if err := manager.inject(FaultBackupRenamed); err != nil {
		return receipt, err
	}
	receipt.Phase = PhaseOldBackedUp
	if err := manager.save(receipt); err != nil {
		return receipt, err
	}
	if err := manager.inject(PhaseOldBackedUp); err != nil {
		return receipt, err
	}
	receipt.Phase = PhaseActivationPending
	if err := manager.save(receipt); err != nil {
		return receipt, err
	}
	if err := manager.inject(PhaseActivationPending); err != nil {
		return receipt, err
	}
	if receipt.Operation == OperationSwap {
		rename := os.Rename
		if !receipt.HadActive {
			rename = renameDirectoryExclusive
		}
		if err := rename(receipt.StagingPath, receipt.ActivePath); err != nil {
			return receipt, fmt.Errorf("activate staged directory: %w", err)
		}
	}
	if err := syncReceiptParents(receipt, receipt.Operation == OperationSwap); err != nil {
		return receipt, fmt.Errorf("sync directory swap parents after activation rename: %w", err)
	}
	if err := manager.inject(FaultActivationApplied); err != nil {
		return receipt, err
	}
	receipt.Phase = PhaseActivated
	if err := manager.save(receipt); err != nil {
		return receipt, err
	}
	if err := manager.inject(PhaseActivated); err != nil {
		return receipt, err
	}
	return receipt, nil
}

func (manager Manager) Commit(ctx context.Context, receipt Receipt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := manager.validateReceipt(receipt); err != nil {
		return err
	}
	if receipt.Phase != PhaseActivated && receipt.Phase != PhaseCommitPending && receipt.Phase != PhaseCommitted {
		return fmt.Errorf("cannot commit directory swap in phase %q", receipt.Phase)
	}
	if receipt.Phase == PhaseActivated {
		receipt.Phase = PhaseCommitPending
		if err := manager.save(receipt); err != nil {
			return err
		}
		if err := manager.inject(PhaseCommitPending); err != nil {
			return err
		}
	}
	if receipt.HadActive {
		if err := removeOwnedDirectory(receipt.OwnedBase, receipt.BackupPath); err != nil {
			return fmt.Errorf("remove committed directory backup: %w", err)
		}
		if err := atomicfile.SyncDirectory(receipt.OwnedBase); err != nil {
			return fmt.Errorf("sync active parent after backup cleanup: %w", err)
		}
	}
	if err := manager.inject(FaultBackupRemoved); err != nil {
		return err
	}
	receipt.Phase = PhaseCommitted
	if err := manager.save(receipt); err != nil {
		return err
	}
	return manager.removeJournal(receipt.OperationID)
}

func (manager Manager) Rollback(ctx context.Context, receipt Receipt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := manager.validateReceipt(receipt); err != nil {
		return err
	}
	switch receipt.Phase {
	case PhaseIntent, PhaseBackupPending, PhaseOldBackedUp, PhaseActivationPending, PhaseActivated:
		receipt.Phase = PhaseRollbackPending
		if err := manager.save(receipt); err != nil {
			return err
		}
		if err := manager.inject(PhaseRollbackPending); err != nil {
			return err
		}
	case PhaseRollbackPending:
		// Resume the idempotent filesystem reconciliation below.
	default:
		return fmt.Errorf("cannot roll back directory swap in phase %q", receipt.Phase)
	}
	if err := manager.restoreOld(receipt); err != nil {
		return err
	}
	if err := syncReceiptParents(receipt, receipt.Operation == OperationSwap); err != nil {
		return fmt.Errorf("sync directory swap parents after rollback: %w", err)
	}
	receipt.Phase = PhaseRolledBack
	if err := manager.save(receipt); err != nil {
		return err
	}
	return manager.removeJournal(receipt.OperationID)
}

func (manager Manager) Recover(ctx context.Context, operationID string, stateCommitted bool) error {
	receipt, err := manager.Load(operationID)
	if err != nil {
		return err
	}
	if receipt.Phase == PhaseRolledBack {
		if stateCommitted {
			return fmt.Errorf("state committed but directory swap is rolled back")
		}
		return manager.removeJournal(receipt.OperationID)
	}
	if (receipt.Phase == PhaseCommitPending || receipt.Phase == PhaseCommitted) && !stateCommitted {
		return fmt.Errorf("directory swap committed without a durable state receipt")
	}
	if stateCommitted {
		if receipt.Phase != PhaseActivated && receipt.Phase != PhaseCommitPending && receipt.Phase != PhaseCommitted {
			return fmt.Errorf("state references directory swap in unrecoverable phase %q", receipt.Phase)
		}
		return manager.Commit(ctx, receipt)
	}
	return manager.Rollback(ctx, receipt)
}

func (manager Manager) Load(operationID string) (Receipt, error) {
	if err := pathpolicy.ValidateLeafID(operationID); err != nil {
		return Receipt{}, fmt.Errorf("unsafe directory swap operation id: %w", err)
	}
	body, err := os.ReadFile(manager.journalPath(operationID))
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("decode directory swap receipt: %w", err)
	}
	if err := manager.validateReceipt(receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (manager Manager) ListOpen() ([]Receipt, error) {
	if strings.TrimSpace(manager.JournalDir) == "" {
		return nil, fmt.Errorf("directory swap journal dir is required")
	}
	entries, err := os.ReadDir(manager.JournalDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var receipts []Receipt
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		operationID := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		receipt, err := manager.Load(operationID)
		if err != nil {
			return nil, fmt.Errorf("read directory swap journal %s: %w", entry.Name(), err)
		}
		receipts = append(receipts, receipt)
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].OperationID < receipts[j].OperationID })
	return receipts, nil
}

func (manager Manager) newReceipt(input Input) (Receipt, error) {
	if strings.TrimSpace(manager.JournalDir) == "" {
		return Receipt{}, fmt.Errorf("directory swap journal dir is required")
	}
	if err := pathpolicy.ValidateLeafID(input.OperationID); err != nil {
		return Receipt{}, fmt.Errorf("unsafe directory swap operation id: %w", err)
	}
	if err := pathpolicy.ValidateLeafID(input.ClientBindingID); err != nil {
		return Receipt{}, fmt.Errorf("unsafe directory swap client binding id: %w", err)
	}
	if input.Sequence < 1 {
		return Receipt{}, fmt.Errorf("directory swap sequence must be positive")
	}
	ownedBase, err := filepath.Abs(input.OwnedBase)
	if err != nil {
		return Receipt{}, err
	}
	activePath, err := filepath.Abs(input.ActivePath)
	if err != nil {
		return Receipt{}, err
	}
	ownedBase, activePath = filepath.Clean(ownedBase), filepath.Clean(activePath)
	operation := OperationSwap
	stagingPath := ""
	if input.Remove {
		operation = OperationRemove
	} else {
		stagingPath, err = filepath.Abs(input.StagingPath)
		if err != nil {
			return Receipt{}, err
		}
		stagingPath = filepath.Clean(stagingPath)
		if activePath == stagingPath {
			return Receipt{}, fmt.Errorf("active and staging paths must be distinct")
		}
	}
	if filepath.Dir(activePath) != ownedBase {
		return Receipt{}, fmt.Errorf("active path must be a direct child of owned base")
	}
	if err := pathpolicy.RequireContainedChild(ownedBase, activePath); err != nil {
		return Receipt{}, fmt.Errorf("unsafe active directory: %w", err)
	}
	if operation == OperationSwap {
		if err := validateStagingPath(ownedBase, stagingPath); err != nil {
			return Receipt{}, fmt.Errorf("unsafe staging directory: %w", err)
		}
		stagingInfo, statErr := os.Lstat(stagingPath)
		if statErr != nil {
			return Receipt{}, fmt.Errorf("inspect staging path: %w", statErr)
		}
		if !stagingInfo.IsDir() || stagingInfo.Mode()&os.ModeSymlink != 0 {
			return Receipt{}, fmt.Errorf("staging path must be a real directory")
		}
	}
	hadActive := false
	if activeInfo, activeErr := os.Lstat(activePath); activeErr == nil {
		if !activeInfo.IsDir() || activeInfo.Mode()&os.ModeSymlink != 0 {
			return Receipt{}, fmt.Errorf("active path must be a real directory")
		}
		hadActive = true
	} else if !os.IsNotExist(activeErr) {
		return Receipt{}, activeErr
	}
	if input.RequireAbsent && hadActive {
		return Receipt{}, fmt.Errorf("active path unexpectedly exists; concurrent modification detected")
	}
	sum := sha256.Sum256([]byte(input.OperationID))
	backupPath := filepath.Join(ownedBase, ".agentplugins-backup-"+hex.EncodeToString(sum[:8]))
	if err := pathpolicy.RequireContainedChild(ownedBase, backupPath); err != nil {
		return Receipt{}, fmt.Errorf("unsafe backup directory: %w", err)
	}
	if _, err := os.Lstat(backupPath); err == nil {
		return Receipt{}, fmt.Errorf("directory swap backup already exists")
	} else if !os.IsNotExist(err) {
		return Receipt{}, err
	}
	publishedIdentity, publishedDigest := "", ""
	if !hadActive && operation == OperationSwap {
		publishedIdentity, publishedDigest, err = publicationProof(stagingPath)
		if err != nil {
			return Receipt{}, fmt.Errorf("capture staged ownership: %w", err)
		}
	}
	return Receipt{
		PublishedIdentity: publishedIdentity, PublishedDigest: publishedDigest,
		SchemaVersion:   receiptSchemaVersion,
		Operation:       operation,
		OperationID:     input.OperationID,
		ClientBindingID: input.ClientBindingID,
		Sequence:        input.Sequence,
		OwnedBase:       ownedBase,
		ActivePath:      activePath,
		StagingPath:     stagingPath,
		BackupPath:      backupPath,
		HadActive:       hadActive,
		Phase:           PhaseIntent,
	}, nil
}

func (manager Manager) validateReceipt(receipt Receipt) error {
	if receipt.SchemaVersion != receiptSchemaVersion {
		return fmt.Errorf("unsupported directory swap receipt schema_version %d", receipt.SchemaVersion)
	}
	if err := pathpolicy.ValidateLeafID(receipt.OperationID); err != nil {
		return fmt.Errorf("unsafe directory swap receipt operation id: %w", err)
	}
	if err := pathpolicy.ValidateLeafID(receipt.ClientBindingID); err != nil {
		return fmt.Errorf("unsafe directory swap receipt client binding id: %w", err)
	}
	if receipt.Sequence < 1 {
		return fmt.Errorf("directory swap receipt sequence must be positive")
	}
	if receipt.Operation != OperationSwap && receipt.Operation != OperationRemove {
		return fmt.Errorf("unsupported directory operation %q", receipt.Operation)
	}
	paths := map[string]string{
		"active": receipt.ActivePath, "backup": receipt.BackupPath,
	}
	if receipt.Operation == OperationSwap {
		if strings.TrimSpace(receipt.StagingPath) == "" {
			return fmt.Errorf("swap receipt requires staging path")
		}
		paths["staging"] = receipt.StagingPath
	} else if receipt.StagingPath != "" {
		return fmt.Errorf("remove receipt cannot contain staging path")
	}
	for label, path := range paths {
		if label == "staging" {
			if err := validateStagingPath(receipt.OwnedBase, path); err != nil {
				return fmt.Errorf("unsafe staging path: %w", err)
			}
			continue
		}
		if filepath.Dir(filepath.Clean(path)) != filepath.Clean(receipt.OwnedBase) {
			return fmt.Errorf("%s path is not a direct child of owned base", label)
		}
		if err := pathpolicy.RequireContainedChild(receipt.OwnedBase, path); err != nil {
			return fmt.Errorf("unsafe %s path: %w", label, err)
		}
	}
	return nil
}

func validateStagingPath(ownedBase, stagingPath string) error {
	ownedBase, stagingPath = filepath.Clean(ownedBase), filepath.Clean(stagingPath)
	stagingParent := filepath.Dir(stagingPath)
	if stagingParent == ownedBase {
		return pathpolicy.RequireContainedChild(ownedBase, stagingPath)
	}
	if !strings.HasPrefix(filepath.Base(stagingPath), ".agentplugins-staging-") {
		return fmt.Errorf("cross-parent staging path does not have the reserved leaf prefix")
	}
	ownedParent := filepath.Dir(ownedBase)
	if stagingParent != ownedParent {
		return fmt.Errorf("staging path is not a child of owned base or its exact parent")
	}
	return pathpolicy.RequireContainedChild(ownedParent, stagingPath)
}

func syncReceiptParents(receipt Receipt, includeStaging bool) error {
	parents := []string{filepath.Clean(receipt.OwnedBase)}
	if includeStaging {
		parents = append(parents, filepath.Dir(receipt.StagingPath))
	}
	seen := map[string]bool{}
	for _, parent := range parents {
		parent = filepath.Clean(parent)
		if seen[parent] {
			continue
		}
		seen[parent] = true
		if err := atomicfile.SyncDirectory(parent); err != nil {
			return err
		}
	}
	return nil
}

func (manager Manager) save(receipt Receipt) error {
	if err := os.MkdirAll(manager.JournalDir, 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicfile.Write(manager.journalPath(receipt.OperationID), body, 0o600)
}

func (manager Manager) removeJournal(operationID string) error {
	path := manager.journalPath(operationID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return atomicfile.SyncDirectory(manager.JournalDir)
}

func (manager Manager) journalPath(operationID string) string {
	return filepath.Join(manager.JournalDir, operationID+".json")
}

func (manager Manager) inject(phase string) error {
	if manager.Fault == nil {
		return nil
	}
	return manager.Fault(phase)
}

func (manager Manager) restoreOld(receipt Receipt) error {
	activeExists, err := realDirectoryExists(receipt.ActivePath)
	if err != nil {
		return fmt.Errorf("inspect active directory during rollback: %w", err)
	}
	backupExists, err := realDirectoryExists(receipt.BackupPath)
	if err != nil {
		return fmt.Errorf("inspect backup directory during rollback: %w", err)
	}
	if !receipt.HadActive {
		return manager.rollbackAbsent(receipt, activeExists, backupExists)
	}
	if !backupExists {
		if activeExists {
			// Either no rename happened or a previous rollback already restored it.
			return nil
		}
		return fmt.Errorf("old active directory and its backup are both missing")
	}
	if activeExists {
		if err := removeOwnedDirectory(receipt.OwnedBase, receipt.ActivePath); err != nil {
			return fmt.Errorf("remove activated directory during rollback: %w", err)
		}
		if err := manager.inject(FaultRollbackRemoved); err != nil {
			return err
		}
	}
	if err := os.Rename(receipt.BackupPath, receipt.ActivePath); err != nil {
		return fmt.Errorf("restore directory backup: %w", err)
	}
	return nil
}

func realDirectoryExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("path %q is not a real directory", path)
	}
	return true, nil
}

func removeOwnedDirectory(base, path string) error {
	if err := pathpolicy.RequireContainedChild(base, path); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
