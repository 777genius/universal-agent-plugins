//go:build linux

package pluginkitairepo_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var qoderCycleOperation = regexp.MustCompile(`^op-[0-9a-f]{32}$`)

func qoderCycleRawState(raw []byte) error {
	value, err := projectNativeRuntimeJSON(raw)
	packet, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(packet, "schema_version installations", "transaction_receipts") != nil {
		return fmt.Errorf("Qoder closed state fields differ")
	}
	rows, ok := packet["installations"].([]any)
	if !ok || len(rows) != 1 {
		return fmt.Errorf("Qoder closed installation count")
	}
	i, ok := rows[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(i, "installation_id declared_name source package clients origin_mode operation_group_id created_at updated_at", "data_receipts data_retained") != nil {
		return fmt.Errorf("Qoder closed installation authority fields differ")
	}
	for field, keys := range map[string]string{"source": "source_binding_id requested_source canonical_source resolved_revision tree_digest", "package": "loader_kind format_id schema_uri declared_name version manifest_digest inventory"} {
		object, ok := i[field].(map[string]any)
		if !ok || projectNativeRuntimeKeys(object, keys, "") != nil {
			return fmt.Errorf("Qoder exact source/package field closure differs")
		}
	}
	pkg := i["package"].(map[string]any)
	inventory, ok := pkg["inventory"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(inventory, "mcp_present mcp_enabled skills", "") != nil {
		return fmt.Errorf("Qoder skills-only inventory field closure differs")
	}
	bindings, ok := i["clients"].(map[string]any)
	if !ok {
		return fmt.Errorf("Qoder exact clients object required")
	}
	for _, value := range bindings {
		b, ok := value.(map[string]any)
		if !ok || projectNativeRuntimeKeys(b, "client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision affected_surfaces native_profile_root native_objects receipts updated_at", "") != nil {
			return fmt.Errorf("Qoder closed binding authority fields differ")
		}
		if err := qoderCycleRawReceipts(b["receipts"]); err != nil {
			return err
		}
	}
	if rows, exists := packet["transaction_receipts"]; exists {
		return qoderCycleRawReceipts(rows)
	}
	return nil
}

func qoderCycleRawReceipts(value any) error {
	rows, ok := value.([]any)
	if !ok {
		return fmt.Errorf("Qoder receipt array required")
	}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "operation_id operation_group_id sequence mutation_type client_binding_id active_path backup_path phase", "staging_path before_digest after_digest") != nil {
			return fmt.Errorf("Qoder closed receipt authority fields differ")
		}
	}
	return nil
}

func qoderCycleTimes(created, updated string) bool {
	a, first := time.Parse(time.RFC3339Nano, created)
	b, second := time.Parse(time.RFC3339Nano, updated)
	return first == nil && second == nil && !b.Before(a)
}

func qoderCycleExpectedDigest(ctx context.Context, key, revision, nonce string) (string, error) {
	files := qoderCycleAuthored(key, revision, nonce)
	files[".agentplugins-qoder-native/.qoder-plugin/plugin.json"] = qoderCycleNativeManifest()
	files[".agentplugins-qoder-native/skills/"+qoderCycleName+"/SKILL.md"] = files["skills/"+qoderCycleName+"/SKILL.md"]
	dirs := map[string]bool{}
	for path := range files {
		for parent := filepath.Dir(path); parent != "."; parent = filepath.Dir(parent) {
			dirs[parent] = true
		}
	}
	entries := make([]packagedigest.CapturedEntry, 0, len(files)+len(dirs))
	for path, body := range files {
		entries = append(entries, packagedigest.CapturedEntry{Path: path, Kind: "file", Content: body})
	}
	for path := range dirs {
		entries = append(entries, packagedigest.CapturedEntry{Path: path, Kind: "directory"})
	}
	return projectNativeRuntimeInstalledDigest(ctx, entries)
}

func qoderCycleReceipt(row domain.MutationReceipt, b domain.ClientBinding, sequence int, kind, before, after string) error {
	if !qoderCycleOperation.MatchString(row.OperationGroupID) || row.OperationID != row.OperationGroupID+"-001" {
		return fmt.Errorf("Qoder exact receipt operation/group differs")
	}
	suffix := qoderDeltaHash(row.OperationID)[:16]
	want := domain.MutationReceipt{OperationID: row.OperationID, OperationGroupID: row.OperationGroupID, Sequence: sequence, MutationType: kind, ClientBindingID: b.ClientBindingID, ActivePath: b.TargetLocator, BackupPath: filepath.Join(b.NativeProfileRoot, ".agentplugins-backup-"+suffix), BeforeDigest: before, AfterDigest: after, Phase: "committed"}
	if kind == "directory_swap" {
		want.StagingPath = filepath.Join(b.NativeProfileRoot, ".agentplugins-staging-"+suffix)
	}
	if !reflect.DeepEqual(row, want) {
		return fmt.Errorf("Qoder exact receipt authority/path/sequence/digest differs")
	}
	return nil
}

func (c *qoderSkillsCycle) receipts(r *qoderCycleRecipe, i domain.Installation, prior domain.ClientBinding, rows []domain.MutationReceipt, stage string) error {
	count := 1
	if r.Key == "a" {
		switch stage {
		case "intact-repair-a", "before-source-revision-a":
			count = 2
		case "update-a", "before-missing-a":
			count = 3
		case "missing-repair-a", "remove-a", "repeat-remove-a":
			count = 4
		}
	}
	b := r.Binding
	if len(b.Receipts) != count || len(prior.Receipts) > count || len(prior.Receipts) > 0 && !reflect.DeepEqual(prior.Receipts, b.Receipts[:len(prior.Receipts)]) {
		return fmt.Errorf("Qoder exact stage/prior receipt history differs")
	}
	one, err := qoderCycleExpectedDigest(c.f.ctx, r.Key, r.Key+"-one", r.Nonce)
	if err != nil {
		return err
	}
	two := one
	if count > 2 {
		two, err = qoderCycleExpectedDigest(c.f.ctx, r.Key, "a-two", r.Nonce)
		if err != nil {
			return err
		}
	}
	before, after := []string{"", one, one, ""}, []string{one, one, two, two}
	seen := map[string]bool{}
	for index, row := range b.Receipts {
		if seen[row.OperationID] || qoderCycleReceipt(row, b, index+1, "directory_swap", before[index], after[index]) != nil {
			return fmt.Errorf("Qoder exact ordered swap history differs")
		}
		seen[row.OperationID] = true
	}
	if !r.Removed {
		if len(rows) != 0 || i.OperationGroupID != b.Receipts[count-1].OperationGroupID {
			return fmt.Errorf("Qoder active group/state transaction differs")
		}
		return nil
	}
	if len(rows) != 1 || rows[0].OperationGroupID != i.OperationGroupID || seen[rows[0].OperationID] {
		return fmt.Errorf("Qoder exact sole removal transaction differs")
	}
	digest, err := qoderCycleExpectedDigest(c.f.ctx, r.Key, r.Revision, r.Nonce)
	if err != nil {
		return err
	}
	return qoderCycleReceipt(rows[0], b, count+1, "directory_remove", digest, "")
}
