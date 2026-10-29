package opencode

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSkillContentConflictWithFailedRollbackRemainsUncertain(t *testing.T) {
	root := t.TempDir()
	backup, target := filepath.Join(root, "old-demo"), filepath.Join(root, "occupied-demo")
	for _, directory := range []string{backup, target} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	contents := []byte("Preserve this original Skill backup.\n")
	if err := os.WriteFile(filepath.Join(backup, "SKILL.md"), contents, 0600); err != nil {
		t.Fatal(err)
	}
	txn := &openCodeSkillTxn{root: root, backups: map[string]openCodeBackup{"demo": {backup: backup, target: target}}, rename: shared.RenameDirectoryExclusive, removeAll: os.RemoveAll}
	_, err := txn.fail(domain.ErrManagedSkillChanged)
	var effect *shared.NativeEffectError
	if err == nil || errors.As(err, &effect) {
		t.Fatal("failed restore of an occupied native directory claimed an unchanged effect", err)
	}
	body, readErr := os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if readErr != nil || string(body) != string(contents) {
		t.Fatal("failed rollback discarded the retained original", readErr)
	}
}
