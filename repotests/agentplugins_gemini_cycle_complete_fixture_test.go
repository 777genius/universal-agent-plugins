//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Independently authored synthetic protocol records are combined with the
// existing actual in-process Loader/Planner/Stager/Kernel/Adapter fixture. No
// command, probe, native client, scanner or public installer is executed.
type geminiCompleteContractFixture struct {
	t            *testing.T
	base         *geminiProjectCycle
	paths        map[string]string
	files        map[string][]byte
	rows         map[string][]map[string]any
	views        map[string]map[string]*geminiCycleRecipe
	recipes      map[string]*geminiCycleRecipe
	states       map[string]domain.StateFileV2
	started      time.Time
	aliasedNonce bool
}

func geminiCompleteContractEncode(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func (f *geminiCompleteContractFixture) stream(body []byte) map[string]any {
	return map[string]any{"bytes": len(body), "sha256": geminiCycleSHA(body), "body": append([]byte{}, body...)}
}

func (f *geminiCompleteContractFixture) add(member, stage string, value any) {
	f.rows[member] = append(f.rows[member], map[string]any{"stage": stage, "value": value})
}

func (f *geminiCompleteContractFixture) snapshot(stage string) {
	view := map[string]*geminiCycleRecipe{}
	for scope, recipe := range f.recipes {
		body := geminiCompleteContractEncode(f.t, recipe)
		var copyRecipe geminiCycleRecipe
		if err := json.Unmarshal(body, &copyRecipe); err != nil {
			f.t.Fatal(err)
		}
		view[scope] = &copyRecipe
	}
	f.views[stage] = view
}

func (f *geminiCompleteContractFixture) source(stage string) {
	for _, line := range bytes.Split(bytes.TrimSpace(f.base.aggregates["cycle-proof.jsonl"]), []byte{'\n'}) {
		row := geminiCycleClone(f.t, line)
		if row["stage"] == stage {
			f.add("cycle-proof.jsonl", stage, row["value"])
			return
		}
	}
	f.t.Fatal("independent authored source row missing", stage)
}

func (f *geminiCompleteContractFixture) proof(stage string) {
	if strings.HasPrefix(stage, "source-") {
		if stage == "source-a-A-two" {
			r := f.base.recipes["a"]
			r.Revision = "A-two"
			f.base.author(f.t, r, true)
			f.recipes["a"].Revision, f.recipes["a"].Digest, f.recipes["a"].Authored = r.Revision, r.Digest, r.Authored
		}
		f.source(stage)
		f.snapshot(stage)
		return
	}
	scope := "a"
	if strings.HasSuffix(stage, "-user") {
		scope = "user"
	}
	if strings.HasSuffix(stage, "-b") {
		scope = "b"
	}
	r := f.recipes[scope]
	i, b := r.Installation, r.Binding
	mutating := strings.HasPrefix(stage, "add-") || stage == "intact-repair-a" || stage == "update-a" || stage == "missing-repair-a"
	if strings.HasPrefix(stage, "add-") {
		i, b = f.base.recipes[scope].Installation, f.base.recipes[scope].Binding
		b.Receipts = nil
	}
	if mutating {
		f.swap(r, &i, &b, stage)
	}
	i.Clients = map[string]domain.ClientBinding{b.ClientBindingID: b}
	state := domain.StateFileV2{SchemaVersion: 5, Installations: []domain.Installation{i}}
	if strings.HasPrefix(stage, "remove-") || stage == "repeat-remove-a" {
		i.Clients, i.DataRetained = map[string]domain.ClientBinding{}, true
		state.Installations[0] = i
		state.TransactionReceipts = []domain.MutationReceipt{f.removal(b, "remove-"+scope)}
		i.OperationGroupID = state.TransactionReceipts[0].OperationGroupID
		state.Installations[0] = i
		r.Removed = true
	}
	r.Installation, r.Binding = i, b
	f.states[stage] = state
	f.add("cycle-proof.jsonl", stage, map[string]any{"scope": scope, "revision": r.Revision, "removed": r.Removed, "source_digest": r.Digest, "state": f.stream(geminiCompleteContractEncode(f.t, state)), "binding": b})
	f.snapshot(stage)
}

func (f *geminiCompleteContractFixture) swap(r *geminiCycleRecipe, i *domain.Installation, b *domain.ClientBinding, stage string) {
	i.Source.TreeDigest = r.Digest
	b.PackageRevision = &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: r.Digest, ManifestDigest: i.Package.ManifestDigest}
	payload := "sha256:" + geminiCycleSHA([]byte("independently authored inert payload "+r.Scope+r.Revision))
	group := "op-" + geminiCycleSHA([]byte("inert operation " + stage))[:32]
	i.OperationGroupID = group
	if i.CreatedAt == "" {
		i.CreatedAt = f.date(f.started)
	}
	i.UpdatedAt = f.date(f.started.Add(time.Duration(len(f.states)+1) * time.Millisecond))
	b.UpdatedAt = i.UpdatedAt
	operation := group + "-001"
	before := ""
	if len(b.Receipts) > 0 {
		before = b.Receipts[len(b.Receipts)-1].AfterDigest
	}
	receipt := domain.MutationReceipt{OperationID: operation, OperationGroupID: group, Sequence: len(b.Receipts) + 1, MutationType: "directory_swap", ClientBindingID: b.ClientBindingID, ActivePath: r.Active, StagingPath: filepath.Dir(r.Active) + "/.agentplugins-staging-" + geminiCycleSHA([]byte(operation))[:16], BackupPath: filepath.Dir(r.Active) + "/.agentplugins-backup-" + geminiCycleSHA([]byte(operation))[:16], BeforeDigest: before, AfterDigest: payload, Phase: "committed"}
	b.Receipts = append(append([]domain.MutationReceipt{}, b.Receipts...), receipt)
	entry := f.base.expectedEntry(r)
	entryBody := geminiCompleteContractEncode(f.t, entry)
	skill := r.Authored["skills/"+r.Name+"/SKILL.md"]
	b.NativeObjects = []domain.NativeObjectOwnership{
		{ObjectID: "package:gemini:" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: "uap-gemini-cycle", Path: r.Active, ManagedDigest: payload, ProtectionClass: "managed"},
		{ObjectID: "gemini-mcp:" + r.Name, Kind: "gemini_global_mcp_server", LogicalName: r.Name, Path: r.Config, ManagedDigest: "sha256:" + geminiCycleSHA(append([]byte("agentplugins-native-mcp-v1\x00gemini-mcpServers\x00"+r.Name+"\x00"), entryBody...)), ProtectionClass: "managed"},
		{ObjectID: "gemini-skill:" + r.Name, Kind: "gemini_global_skill_directory", LogicalName: r.Name, Path: filepath.Dir(r.SkillPath), SourceRelative: "skills/" + r.Name, ManagedDigest: "sha256:" + geminiCycleSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0)), ProtectionClass: "managed"},
	}
}

func (f *geminiCompleteContractFixture) removal(b domain.ClientBinding, stage string) domain.MutationReceipt {
	group := "op-" + geminiCycleSHA([]byte("inert operation " + stage))[:32]
	operation := group + "-001"
	return domain.MutationReceipt{OperationID: operation, OperationGroupID: group, Sequence: len(b.Receipts) + 1, MutationType: "directory_remove", ClientBindingID: b.ClientBindingID, ActivePath: b.TargetLocator, BackupPath: filepath.Dir(b.TargetLocator) + "/.agentplugins-backup-" + geminiCycleSHA([]byte(operation))[:16], BeforeDigest: b.Receipts[len(b.Receipts)-1].AfterDigest, Phase: "committed"}
}

func geminiCompleteContractFixtureNew(t *testing.T) *geminiCompleteContractFixture {
	return geminiCompleteContractFixtureNonce(t, false)
}

func geminiCompleteContractFixtureNonce(t *testing.T, alias bool) *geminiCompleteContractFixture {
	t.Helper()
	base := geminiCycleInert(t)
	if alias {
		original := bytes.Split(bytes.TrimSuffix(base.aggregates["cycle-proof.jsonl"], []byte{'\n'}), []byte{'\n'})
		base.recipes["user"].Nonce = base.recipes["a"].Nonce
		base.author(t, base.recipes["user"], true)
		all := bytes.Split(bytes.TrimSuffix(base.aggregates["cycle-proof.jsonl"], []byte{'\n'}), []byte{'\n'})
		body := append(append([]byte{}, all[len(all)-1]...), '\n')
		for _, row := range original[1:] {
			body = append(append(body, row...), '\n')
		}
		base.aggregates["cycle-proof.jsonl"], base.f.files["cycle-proof.jsonl"] = body, bytes.Clone(body)
	}
	for _, scope := range []string{"user", "a", "b"} {
		geminiCycleStageActual(t, base, scope)
	}
	f := &geminiCompleteContractFixture{t: t, base: base, paths: base.f.evidence["cycle_layout"].(map[string]string), files: map[string][]byte{}, rows: map[string][]map[string]any{}, views: map[string]map[string]*geminiCycleRecipe{}, recipes: map[string]*geminiCycleRecipe{}, states: map[string]domain.StateFileV2{}, started: base.f.started.UTC()}
	f.aliasedNonce = alias
	for scope, recipe := range base.recipes {
		body := geminiCompleteContractEncode(t, recipe)
		var copyRecipe geminiCycleRecipe
		if err := json.Unmarshal(body, &copyRecipe); err != nil {
			t.Fatal(err)
		}
		f.recipes[scope] = &copyRecipe
		f.recipes[scope].Binding, f.recipes[scope].Installation = domain.ClientBinding{}, domain.Installation{}
	}
	for _, stage := range []string{"source-user-U-one", "source-a-A-one", "source-b-B-one", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-A-two", "update-a", "before-missing-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"} {
		f.proof(stage)
	}
	// Baseline uses the actual authored recipes but deliberately has no installed
	// binding in this independently authored observation timeline.
	f.views[""] = f.views["source-b-B-one"]
	return f
}

func (f *geminiCompleteContractFixture) encodeRows() {
	for _, member := range []string{"cycle-native-commands.jsonl", "cycle-native-events.jsonl", "cycle-discovery.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl", "cycle-proof.jsonl"} {
		var body []byte
		for _, row := range f.rows[member] {
			body = append(append(body, geminiCompleteContractEncode(f.t, row)...), '\n')
		}
		f.files[member] = body
	}
}

func geminiCompleteContractDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok {
		t.Fatal("independent inert contract object required", err)
	}
	return row
}

func (f *geminiCompleteContractFixture) date(stamp time.Time) string {
	return stamp.UTC().Format(time.RFC3339Nano)
}

func (f *geminiCompleteContractFixture) contractSource() map[string]any {
	return map[string]any{"installer_base_commit": strings.Repeat("a", 40), "installer_tree": strings.Repeat("b", 40), "installer_source_state": "committed", "installer_patch_sha256": []string{}}
}
