package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// The existing dirswap manager owns physical publication, crash receipts and
// rollback. This small record preserves user-visible undo data after commit; it
// is never installation ownership and is written before any active mutation.
type localSkillReceipt struct {
	Version      int    `json:"version"`
	OperationID  string `json:"operationId"`
	RootID       string `json:"rootId"`
	RootPath     string `json:"rootPath"`
	SkillName    string `json:"skillName"`
	BeforeDigest string `json:"beforeDigest"`
	AfterDigest  string `json:"afterDigest"`
	BeforeExists bool   `json:"beforeExists"`
	BackupName   string `json:"backupName"`
	Phase        string `json:"phase"`
}

func (b *ManagementBackend) localSkillManager() dirswap.Manager {
	return dirswap.Manager{JournalDir: filepath.Join(b.config.StateRoot, "local-skills", "journals")}
}
func (b *ManagementBackend) localSkillReceiptPath(id string) string {
	return filepath.Join(b.config.StateRoot, "local-skills", "receipts", id+".json")
}
func (b *ManagementBackend) saveLocalSkillReceipt(receipt localSkillReceipt) error {
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if !localSkillSHA(receipt.OperationID) {
		return errors.New("invalid local receipt identity")
	}
	if err = pathcontract.RequireContainedChild(b.config.StateRoot, b.localSkillReceiptPath(receipt.OperationID)); err != nil {
		return err
	}
	parent := filepath.Dir(b.localSkillReceiptPath(receipt.OperationID))
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	if err = os.Chmod(parent, 0700); err != nil {
		return err
	}
	return atomicfile.Write(b.localSkillReceiptPath(receipt.OperationID), append(body, '\n'), 0600)
}
func (b *ManagementBackend) readLocalSkillReceipt(id string, root management.LocalSkillRoot, name string) (*localSkillReceipt, error) {
	receipt, err := b.loadLocalSkillReceipt(id)
	if err != nil || receipt == nil {
		return receipt, err
	}
	if receipt.RootID != root.Id || receipt.RootPath != root.Path || receipt.SkillName != name {
		return nil, localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The retained backup belongs to a different captured Skill")
	}
	return receipt, nil
}
func (b *ManagementBackend) loadLocalSkillReceipt(id string) (*localSkillReceipt, error) {
	if !localSkillSHA(id) {
		return nil, localSkillRefusal("INVALID_REQUEST", "Restore requires a retained local operation identity")
	}
	filename := b.localSkillReceiptPath(id)
	if err := pathcontract.RequireContainedChild(b.config.StateRoot, filename); err != nil {
		return nil, err
	}
	// Refuse nonregular files before opening; a named pipe could block the open.
	info, err := os.Lstat(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	file, err := os.OpenInRoot(b.config.StateRoot, filepath.Join("local-skills", "receipts", id+".json"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	body, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return nil, err
	}
	if len(body) > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	var receipt localSkillReceipt
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("invalid local Skill recovery record")
	}
	if !validLocalSkillReceipt(receipt, id) {
		return nil, localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The retained backup belongs to a different captured Skill")
	}
	return &receipt, nil
}
func validLocalSkillReceipt(receipt localSkillReceipt, id string) bool {
	return receipt.Version == 1 && receipt.OperationID == id && receipt.RootID != "" && filepath.IsAbs(receipt.RootPath) && filepath.Clean(receipt.RootPath) == receipt.RootPath && localSkillName(receipt.SkillName) && receipt.BackupName == id && localSkillSHA(receipt.BeforeDigest) && localSkillSHA(receipt.AfterDigest) && (receipt.Phase == "prepared" || receipt.Phase == "committed")
}
func (p *managementLocalSkill) validateRestoreSnapshot() error {
	manager := p.backend.localSkillManager()
	journal, err := manager.Load(p.input.RestoreOperationId)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && (journal.OwnedBase != p.root.Path || journal.ActivePath != filepath.Join(p.root.Path, p.input.SkillName)) {
		return localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The local receipt belongs to a different captured Skill")
	}
	if p.restored == nil {
		if err == nil {
			return localSkillRefusal("RECOVERY_REQUIRED", "Preserve the local receipt; its review metadata is unavailable")
		}
		return nil // By construction no active mutation can precede both records.
	}
	if p.before.digest() != p.restored.BeforeDigest && p.before.digest() != p.restored.AfterDigest && p.before.Exists {
		return localSkillRefusal("LOCAL_EDIT_CONFLICT", "The Skill changed after the saved operation; preserve its files and review those edits before restoration")
	}
	if p.restored.BeforeExists {
		backup, readErr := localSkillObserve(p.backend.localSkillBackupRoot(), p.restored.BackupName, false)
		if readErr != nil || backup.digest() != p.restored.BeforeDigest {
			return localSkillRefusal("BACKUP_CHANGED", "The retained backup is missing or changed; no local files were restored")
		}
	}
	return nil
}
