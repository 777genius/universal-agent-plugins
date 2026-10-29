//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func grokCycleReceiptCount(stage, scope string) int {
	if scope == "b" {
		return 1
	}
	switch stage {
	case "intact-repair-a", "before-source-revision-a":
		return 2
	case "update-a", "before-missing-a":
		return 3
	case "missing-repair-a", "remove-a", "repeat-remove-a":
		return 4
	default:
		return 1
	}
}

func grokCycleExpectedDigest(ctx context.Context, name, revision, nonce string) (string, error) {
	r := &grokCycleRecipe{Name: name, Authored: grokCycleAuthored(name, revision, nonce)}
	files := grokCycleProjected(r)
	entries := make([]packagedigest.CapturedEntry, 0, 3+len(files))
	entries = append(entries, packagedigest.CapturedEntry{Path: ".grok-plugin", Kind: "directory"}, packagedigest.CapturedEntry{Path: "skills", Kind: "directory"}, packagedigest.CapturedEntry{Path: "skills/" + name, Kind: "directory"})
	for path, body := range files {
		entries = append(entries, packagedigest.CapturedEntry{Path: path, Kind: "file", Content: body})
	}
	return projectNativeRuntimeInstalledDigest(ctx, entries)
}

func grokCycleReceipt(row domain.MutationReceipt, b domain.ClientBinding, sequence int, kind, before, after string) error {
	if !regexp.MustCompile(`^op-[0-9a-f]{32}$`).MatchString(row.OperationGroupID) || row.OperationID != row.OperationGroupID+"-001" {
		return fmt.Errorf("Grok grouped receipt operation/group differs")
	}
	suffix := clineAttemptSHA([]byte(row.OperationID))[:16]
	want := domain.MutationReceipt{OperationID: row.OperationID, OperationGroupID: row.OperationGroupID, Sequence: sequence, MutationType: kind, ClientBindingID: b.ClientBindingID, ActivePath: b.TargetLocator, BackupPath: filepath.Join(filepath.Dir(b.TargetLocator), ".agentplugins-backup-"+suffix), BeforeDigest: before, AfterDigest: after, Phase: "committed"}
	if kind == "directory_swap" {
		want.StagingPath = filepath.Join(b.NativeProfileRoot, ".agentplugins-staging-"+suffix)
	}
	if !reflect.DeepEqual(row, want) {
		return fmt.Errorf("Grok exact receipt type/path/sequence/digest/authority differs")
	}
	return nil
}

func (c *grokProjectCycle) swapReceipts(r *grokCycleRecipe, b domain.ClientBinding, stage string) error {
	if len(b.Receipts) != grokCycleReceiptCount(stage, r.Scope) {
		return fmt.Errorf("Grok grouped receipt stage count differs")
	}
	one, err := grokCycleExpectedDigest(c.f.ctx, r.Name, r.Scope+"-one", c.f.base.Nonce)
	if err != nil {
		return err
	}
	two := one
	if r.Scope == "a" && len(b.Receipts) > 2 {
		two, err = grokCycleExpectedDigest(c.f.ctx, r.Name, "a-two", c.f.base.Nonce)
		if err != nil {
			return err
		}
	}
	before, after := []string{"", one, one, ""}, []string{one, one, two, two}
	seen := map[string]bool{}
	for index, receipt := range b.Receipts {
		if seen[receipt.OperationID] || grokCycleReceipt(receipt, b, index+1, "directory_swap", before[index], after[index]) != nil {
			return fmt.Errorf("Grok grouped receipt history differs")
		}
		seen[receipt.OperationID] = true
	}
	if len(r.Binding.Receipts) > len(b.Receipts) || len(r.Binding.Receipts) > 0 && !reflect.DeepEqual(r.Binding.Receipts, b.Receipts[:len(r.Binding.Receipts)]) {
		return fmt.Errorf("Grok prior receipt history changed")
	}
	return nil
}

func (c *grokProjectCycle) transactionReceipts(r *grokCycleRecipe, i domain.Installation, rows []domain.MutationReceipt) error {
	if !r.Removed {
		if len(rows) != 0 {
			return fmt.Errorf("Grok active package has unexpected state-level receipt")
		}
		return nil
	}
	if len(rows) != 1 || rows[0].OperationGroupID != i.OperationGroupID {
		return fmt.Errorf("Grok retained removal has no exact state-level receipt")
	}
	digest, err := grokCycleExpectedDigest(c.f.ctx, r.Name, r.Revision, c.f.base.Nonce)
	if err != nil {
		return err
	}
	return grokCycleReceipt(rows[0], r.Binding, len(r.Binding.Receipts)+1, "directory_remove", digest, "")
}
