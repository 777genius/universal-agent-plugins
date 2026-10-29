//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

const copilotPostAddRegistryMode = "copilot-post-add-registry-first-capture"
const copilotPostAddListMetadata = "native-list-profile-metadata.json"

var copilotPostAddRegistryCommands = []string{"git-A", "git-B", "version", "add-user", "native-list"}

func (f *copilotProjectAttempt) postAddRegistryCommand(label, cwd string, argv []string, timeout time.Duration) error {
	sequence, ok := f.evidence["commands"].([]string)
	if !ok || len(sequence) >= 5 || !slices.Equal(sequence, copilotPostAddRegistryCommands[:len(sequence)]) {
		return fmt.Errorf("Copilot post-add registry command prefix refused")
	}
	if len(sequence) < 4 {
		return f.userRegistrationCommand(label, cwd, argv, timeout)
	}
	if label != "native-list" || cwd != f.outside || timeout != 30*time.Second || !slices.Equal(argv, []string{f.client, "plugin", "list"}) {
		return fmt.Errorf("Copilot post-add registry fixed list command refused")
	}
	return nil
}

func (f *copilotProjectAttempt) postAddRegistryEvidence() {
	f.userRegistrationEvidence()
	f.evidence["mode"] = copilotPostAddRegistryMode
}

func copilotPostAddPublic(body []byte) bool {
	v, err := projectNativeRuntimeJSON(body)
	row, ok := v.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "schema_version command result data", "") != nil || row["schema_version"] != json.Number("1") || row["command"] != "add" || row["result"] != "success" {
		return false
	}
	_, ok = row["data"].(map[string]any)
	return ok && nativeValidateInstallerJSON(body) == nil
}

// Each phase returns ordinarily only after its own metadata-only defer settles.
// A fatal original command/config/pin guard cannot reach the next phase.
func (f *copilotProjectAttempt) postAddUserPhase(t *testing.T) {
	t.Helper()
	f.evidence["stage"] = "before-user"
	root := filepath.Join(f.base.Home, ".copilot")
	before, err := copilotUserMetadataCapture(f.ctx, root)
	if err != nil {
		f.keep(t, "user-profile-metadata.json", copilotUserMetadataRefusal())
		t.Fatal("Copilot post-add original pre-user metadata refused")
	}
	defer func() { f.keep(t, "user-profile-metadata.json", copilotUserMetadataAfter(f.ctx, root, before)) }()
	f.evidence["stage"] = "public-user"
	r := f.command(t, "add-user", f.outside, f.env(), []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"}, 30*time.Second)
	f.evidence["stage"] = "public-result"
	if !copilotPostAddPublic(r.Stdout) {
		t.Fatal("Copilot post-add public successful versioned add envelope refused")
	}
}

func (f *copilotProjectAttempt) postAddListPhase(t *testing.T) {
	t.Helper()
	f.evidence["stage"] = "before-list"
	root := filepath.Join(f.base.Home, ".copilot")
	before, err := copilotUserMetadataCapture(f.ctx, root)
	if err != nil {
		f.keep(t, copilotPostAddListMetadata, copilotMetadataRefusalAtStage("native-list"))
		t.Fatal("Copilot post-add original pre-list metadata refused")
	}
	defer func() { f.keep(t, copilotPostAddListMetadata, copilotEmptyRegistryMetadataAfter(f.ctx, root, before)) }()
	f.evidence["stage"] = "native-list"
	f.command(t, "native-list", f.outside, f.env(), []string{f.client, "plugin", "list"}, 30*time.Second)
}

func (f *copilotProjectAttempt) observePostAddRegistry(t *testing.T) {
	t.Helper()
	f.postAddUserPhase(t)
	if status, ok := copilotPostAddMetadata(f.files["user-profile-metadata.json"], "add-user"); !ok || status != "observed" {
		f.evidence["stage"] = "after-user-metadata"
		t.Fatal("Copilot post-add user metadata refused; no native list started")
	}
	f.postAddListPhase(t)
	if status, ok := copilotPostAddMetadata(f.files[copilotPostAddListMetadata], "native-list"); !ok || status != "observed" {
		f.evidence["stage"] = "after-list-metadata"
		t.Fatal("Copilot post-add native-list metadata refused")
	}
	f.evidence["stage"] = "review-barrier"
	t.Fatal("genuine post-add native registry listing captured; exact raw review required; all qualification remains false")
}

func copilotPostAddMetadata(body []byte, stage string) (string, bool) {
	if len(body) > copilotUserMetadataLimit {
		return "", false
	}
	value, err := projectNativeRuntimeJSON(body)
	row, ok := value.(map[string]any)
	if err != nil || !ok || row["stage"] != stage {
		return "", false
	}
	if row["status"] == "refused" {
		return "refused", projectNativeRuntimeKeys(row, "stage status", "") == nil
	}
	var projection copilotUserMetadataProjection
	if json.Unmarshal(body, &projection) != nil || !copilotPostAddMetadataCounts(projection) {
		return "", false
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		return "", false
	}
	roundtrip, err := projectNativeRuntimeJSON(encoded)
	if err != nil || !reflect.DeepEqual(value, roundtrip) {
		return "", false
	}
	previous := ""
	for _, entry := range projection.Entries {
		if !copilotPostAddMetadataEntry(entry, previous) {
			return "", false
		}
		previous = entry.PathSHA256
	}
	return "observed", true
}

func copilotPostAddMetadataCounts(p copilotUserMetadataProjection) bool {
	return p.Status == "observed" && p.BeforeEntries >= 1 && p.BeforeEntries <= 1024 && p.AfterEntries >= 1 && p.AfterEntries <= 1024 && p.ChangedEntries >= 0 && p.ChangedEntries <= p.BeforeEntries+p.AfterEntries && p.OmittedCount >= 0 && p.ChangedEntries == len(p.Entries)+p.OmittedCount && len(p.Entries) <= 16 && (p.ChangedEntries == 0 || len(p.Entries) > 0) && p.Entries != nil
}

func copilotPostAddMetadataHash(hash, previous string) bool {
	if len(hash) != 64 || hash <= previous {
		return false
	}
	for _, ch := range hash {
		if ch < '0' || ch > '9' && ch < 'a' || ch > 'f' {
			return false
		}
	}
	return true
}

func copilotPostAddMetadataKind(kind string, permissions uint32, present bool) bool {
	return (kind == "absent" || kind == "file" || kind == "directory") && present == (kind != "absent") && permissions <= 07777 && (present || permissions == 0)
}

func copilotPostAddMetadataEntry(e copilotUserMetadataChange, previous string) bool {
	if !copilotPostAddMetadataHash(e.PathSHA256, previous) || !e.BeforePresent && !e.AfterPresent || !copilotPostAddMetadataKind(e.BeforeKind, e.BeforePermissions, e.BeforePresent) || !copilotPostAddMetadataKind(e.AfterKind, e.AfterPermissions, e.AfterPresent) {
		return false
	}
	if e.BeforePresent && e.AfterPresent {
		return e.OwnerEqual && e.GroupEqual && e.ModeEqual == (e.BeforeKind == e.AfterKind && e.BeforePermissions == e.AfterPermissions)
	}
	return !e.OwnerEqual && !e.GroupEqual && !e.IdentityEqual && !e.ModeEqual
}

func copilotPostAddStage(stage string, n int) (metadata int, public bool, ok bool) {
	switch stage {
	case "prepare":
		return 0, false, n <= 3
	case "before-user":
		return 1, false, n == 3
	case "public-user":
		return 1, false, n == 3 || n == 4
	case "public-result":
		return 1, false, n == 4
	case "after-user-metadata":
		return 1, true, n == 4
	case "before-list":
		return 2, true, n == 4
	case "native-list":
		return 2, true, n == 4 || n == 5
	case "after-list-metadata", "review-barrier":
		return 2, true, n == 5
	default:
		return 0, false, false
	}
}

type copilotPostAddCommandReceipt struct {
	Argv         []string `json:"argv"`
	CWD          string   `json:"cwd"`
	Exit         int      `json:"exit"`
	Elapsed      int      `json:"elapsed_ms"`
	Error        bool     `json:"error"`
	ContextError bool     `json:"context_error"`
	Cleanup      string   `json:"cleanup"`
}

func copilotPostAddReceipt(body []byte) (copilotPostAddCommandReceipt, bool) {
	var command copilotPostAddCommandReceipt
	value, err := projectNativeRuntimeJSON(body)
	row, typed := value.(map[string]any)
	if err != nil || !typed || projectNativeRuntimeKeys(row, "argv cwd exit elapsed_ms error context_error cleanup", "") != nil || json.Unmarshal(body, &command) != nil {
		return command, false
	}
	if command.Exit < -1 || command.Exit > 255 || command.Elapsed < 0 || command.Elapsed > 35000 || command.Cleanup != "ordinary full-descendant supervision; forced cleanup is a failure" {
		return command, false
	}
	encoded, err := json.Marshal(command)
	if err != nil {
		return command, false
	}
	roundtrip, err := projectNativeRuntimeJSON(encoded)
	return command, err == nil && reflect.DeepEqual(row, roundtrip)
}

func (f *copilotProjectAttempt) postAddCapturedCommand(sequence []string, index int, stage string) bool {
	label := sequence[index]
	command, ok := copilotPostAddReceipt(f.files[label+".command.json"])
	if !ok {
		return false
	}
	copyAttempt := *f
	copyAttempt.evidence = map[string]any{"commands": append([]string{}, sequence[:index]...)}
	if copyAttempt.postAddRegistryCommand(label, command.CWD, command.Argv, 30*time.Second) != nil {
		return false
	}
	current := index == len(sequence)-1 && (stage == "prepare" || stage == "public-user" && label == "add-user" || stage == "native-list" && label == "native-list")
	if current {
		return true
	}
	if command.Exit != 0 || command.Error || command.ContextError {
		return false
	}
	if label == "git-A" || label == "git-B" {
		return len(f.files[label+".stdout.log"]) == 0 && len(f.files[label+".stderr.log"]) == 0
	}
	return true
}

func copilotPostAddMetadataStages(stage string, statuses []string) bool {
	switch stage {
	case "before-user", "after-user-metadata":
		return statuses[0] == "refused"
	case "before-list", "after-list-metadata":
		return statuses[0] == "observed" && statuses[1] == "refused"
	case "native-list":
		return statuses[0] == "observed"
	case "review-barrier":
		return statuses[0] == "observed" && statuses[1] == "observed"
	default:
		return true
	}
}

func (f *copilotProjectAttempt) postAddRegistryMetadataFiles(count int, allowed map[string]bool) ([]string, bool) {
	statuses := []string{}
	for index, name := range []string{"user-profile-metadata.json", copilotPostAddListMetadata} {
		if index >= count {
			break
		}
		allowed[name] = true
		status, valid := copilotPostAddMetadata(f.files[name], []string{"add-user", "native-list"}[index])
		if !valid {
			return nil, false
		}
		statuses = append(statuses, status)
	}
	return statuses, true
}

func (f *copilotProjectAttempt) postAddRegistryOutputs(stage string, public bool) bool {
	if stage != "prepare" && (!bytes.Equal(f.files["version.stdout.log"], []byte("GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n")) || len(f.files["version.stderr.log"]) != 0) {
		return false
	}
	if public && !copilotPostAddPublic(f.files["add-user.stdout.log"]) {
		return false
	}
	_, changed := f.evidence["original_config_change"]
	return !changed || stage == "prepare" || stage == "public-user" || stage == "native-list"
}

func copilotPostAddClosedBodies(files map[string][]byte, allowed map[string]bool) bool {
	total := 0
	for name, body := range files {
		total += len(body)
		if !allowed[name] || len(body) > copilotProjectBodyLimit {
			return false
		}
	}
	return total <= 16<<20
}

func (f *copilotProjectAttempt) postAddRegistryFiles(sequence []string, stage string) bool {
	n := len(sequence)
	if n > 5 || !slices.Equal(sequence, copilotPostAddRegistryCommands[:n]) {
		return false
	}
	metadata, public, ok := copilotPostAddStage(stage, n)
	if !ok || len(f.files) != n*3+metadata || len(f.files) > 17 {
		return false
	}
	allowed := map[string]bool{}
	for index, label := range sequence {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			allowed[label+suffix] = true
		}
		if !f.postAddCapturedCommand(sequence, index, stage) {
			return false
		}
	}
	statuses, valid := f.postAddRegistryMetadataFiles(metadata, allowed)
	return valid && copilotPostAddMetadataStages(stage, statuses) && f.postAddRegistryOutputs(stage, public) && copilotPostAddClosedBodies(f.files, allowed)
}

func (f *copilotProjectAttempt) finalizePostAddRegistry(t *testing.T) {
	t.Helper()
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["vendor_authentication_qualified"], f.evidence["real_model_qualified"] = false, false
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["probe_status"] = "absent"
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		f.evidence["probe_status"] = "refused"
		t.Error("unexpected probe state in post-add registry observation")
	}
	sequence, sequenceOK := f.evidence["commands"].([]string)
	stage, stageOK := f.evidence["stage"].(string)
	if !sequenceOK || !stageOK || !f.postAddRegistryFiles(sequence, stage) {
		t.Error("Copilot post-add registry closed prefix/member contract refused")
	}
	if !t.Failed() {
		t.Error("Copilot post-add registry observation cannot pass or qualify")
	}
}

func TestAgentpluginsCopilotNativePostAddRegistryFirstObservation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_POST_ADD_REGISTRY_FIRST_CAPTURE") != "1" {
		t.Skip("opt-in genuine failed-only post-add registry observation")
	}
	f := newCopilotProjectAttempt(t)
	f.postAddRegistryEvidence()
	defer f.finish(t)
	f.preparePackage(t)
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("unexpected probe state before post-add registry observation")
	}
	f.observePostAddRegistry(t)
}
