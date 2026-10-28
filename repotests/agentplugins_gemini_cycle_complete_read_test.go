//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type geminiCompleteCapture struct {
	Files             map[string][]byte
	Paths             map[string]string
	Rows              map[string][]geminiCompleteRow
	Proof             geminiCompleteProof
	Started, Finished time.Time
}

func geminiCompleteLayout(value any) (map[string]string, error) {
	row, err := geminiCompleteObject(value, "root home a b outside installer client node git", "")
	paths, seen := map[string]string{}, map[string]bool{}
	if err != nil {
		return nil, err
	}
	for key, value := range row {
		path, ok := value.(string)
		if !ok || !geminiCompleteText(path, 4096, true) || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") || seen[path] {
			return nil, geminiCompleteRefusal()
		}
		paths[key], seen[path] = path, true
	}
	for key, suffix := range map[string]string{"home": "home", "a": "project", "b": "project-B", "outside": "outside"} {
		if paths[key] != paths["root"]+"/"+suffix {
			return nil, geminiCompleteRefusal()
		}
	}
	return paths, nil
}

func geminiCompleteClosure(result map[string]any) (time.Time, time.Time, error) {
	value, err := geminiCompleteObject(result["cycle_completion"], "schema_version started_at work_deadline_at finished_at work_context_error original_checks_error open_authorities open_events provider", "")
	encoded, encodeErr := json.Marshal(value)
	started, startErr := geminiCompleteTime(value["started_at"])
	deadline, deadlineErr := geminiCompleteTime(value["work_deadline_at"])
	finished, finishErr := geminiCompleteTime(value["finished_at"])
	if err != nil || encodeErr != nil || len(encoded) > 8192 || startErr != nil || deadlineErr != nil || finishErr != nil || !geminiCompleteIntegerIs(value["schema_version"], 1) || value["work_context_error"] != false || value["original_checks_error"] != false || !geminiCompleteIntegerIs(value["open_authorities"], 0) || value["open_events"] != false || !deadline.Equal(started.Add(150*time.Second)) || finished.Before(started) || finished.After(deadline) || !geminiCompleteInteger(result["elapsed_ms"], finished.Sub(started).Milliseconds(), 180000) {
		return started, finished, geminiCompleteRefusal()
	}
	provider, err := geminiCompleteObject(value["provider"], "shutdown_complete forced_close serve_returned error busy current_stage requests completed_stages", "")
	if err != nil || provider["shutdown_complete"] != true || provider["forced_close"] != false || provider["serve_returned"] != true || provider["error"] != false || !geminiCompleteIntegerIs(provider["busy"], 0) || provider["current_stage"] != "" || !geminiCompleteIntegerIs(provider["requests"], 16) || !geminiCompleteStrings(provider["completed_stages"], geminiCycleNative) {
		return started, finished, geminiCompleteRefusal()
	}
	return started, finished, nil
}

func geminiCompleteSummary(result map[string]any, source any) error {
	if projectNativeRuntimeKeys(result, "acquisition activation attempt_stage capture_mode client client_source cycle_capture_review_required cycle_layout cycle_listing_stages cycle_native_stages cycle_observed_complete cycle_provider_completed cycle_public_stages elapsed_ms gemini_detection_link provider_kind qualification real_model_qualified runtime_attempt_complete scanner source_identity test_failed vendor_authentication_qualified cycle_completion", "") != nil || result["client"] != "gemini" || result["provider_kind"] != "scripted_loopback" || result["capture_mode"] != geminiCycleCompleteMode || result["cycle_capture_review_required"] != false || result["cycle_observed_complete"] != true || result["runtime_attempt_complete"] != true || result["test_failed"] != false || result["activation"] != "prepared" || result["vendor_authentication_qualified"] != false || result["real_model_qualified"] != false || !reflect.DeepEqual(result["source_identity"], source) {
		return geminiCompleteRefusal()
	}
	identity, err := geminiCompleteObject(source, "installer_base_commit installer_tree installer_source_state installer_patch_sha256", "")
	commit, cok := identity["installer_base_commit"].(string)
	tree, tok := identity["installer_tree"].(string)
	if err != nil || !cok || !tok || !regexpGitIdentity(commit) || !regexpGitIdentity(tree) || identity["installer_source_state"] != "committed" || !reflect.DeepEqual(identity["installer_patch_sha256"], []any{}) {
		return geminiCompleteRefusal()
	}
	flags, err := geminiCompleteObject(result["qualification"], "project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call", "")
	if err != nil {
		return err
	}
	for _, value := range flags {
		if value != true {
			return geminiCompleteRefusal()
		}
	}
	for name, want := range map[string][]string{"cycle_public_stages": geminiCyclePublic, "cycle_listing_stages": geminiCycleListings, "cycle_native_stages": geminiCycleNative, "cycle_provider_completed": geminiCycleNative} {
		if !geminiCompleteStrings(result[name], want) {
			return geminiCompleteRefusal()
		}
	}
	if result["attempt_stage"] != "complete-cycle" {
		return geminiCompleteRefusal()
	}
	return nil
}

func regexpGitIdentity(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' && char < 'a' || char > 'f' {
			return false
		}
	}
	return true
}

func geminiCycleCompleteFiles(files map[string][]byte, source any) error {
	result, err := geminiCompleteJSON(files["runtime-attempt.json"])
	if err != nil || geminiCompleteSummary(result, source) != nil {
		return geminiCompleteRefusal()
	}
	c := geminiCompleteCapture{Files: files, Rows: map[string][]geminiCompleteRow{}}
	c.Paths, err = geminiCompleteLayout(result["cycle_layout"])
	if err != nil {
		return err
	}
	c.Started, c.Finished, err = geminiCompleteClosure(result)
	if err != nil || geminiCompletePins(result, c.Paths) != nil || geminiCompleteMembers(files) != nil {
		return geminiCompleteRefusal()
	}
	c.Proof, err = geminiCompleteProofs(files["cycle-proof.jsonl"], c.Paths)
	if err != nil {
		return err
	}
	if err := c.stateClocks(); err != nil {
		return err
	}
	if err := c.public(); err != nil {
		return err
	}
	if err := c.observations(); err != nil {
		return err
	}
	return c.protocol()
}

func (c *geminiCompleteCapture) stateClocks() error {
	for stage, body := range c.Proof.Raw {
		state, err := geminiCycleState(body)
		if err != nil {
			return err
		}
		i := state.Installations[0]
		created, createErr := geminiCompleteTime(i.CreatedAt)
		updated, updateErr := geminiCompleteTime(i.UpdatedAt)
		if createErr != nil || updateErr != nil || created.Before(c.Started) || updated.Before(created) || updated.After(c.Finished) {
			return geminiCompleteRefusal()
		}
		r := c.Proof.Views[stage][geminiCycleScope(stage)]
		bindingTime, err := geminiCompleteTime(r.Binding.UpdatedAt)
		if err != nil || bindingTime.Before(created) || bindingTime.After(c.Finished) {
			return geminiCompleteRefusal()
		}
		for _, receipt := range i.DataReceipts {
			for _, stamp := range []string{receipt.CreatedAt, receipt.UpdatedAt} {
				if stamp == "" {
					continue
				}
				when, err := geminiCompleteTime(stamp)
				if err != nil || when.Before(c.Started) || when.After(c.Finished) {
					return geminiCompleteRefusal()
				}
			}
		}
	}
	return nil
}

func geminiCompleteMembers(files map[string][]byte) error {
	allowed := map[string]bool{"runtime-attempt.json": true}
	for _, label := range geminiCyclePublic {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			allowed[label+suffix] = true
		}
	}
	for _, name := range geminiCycleAggregates {
		allowed[name] = true
	}
	total := 0
	if len(files) != 57 {
		return geminiCompleteRefusal()
	}
	for name, body := range files {
		if !allowed[name] || len(body) > 1<<20 || geminiCyclePrivateCredential(body) {
			return geminiCompleteRefusal()
		}
		total += len(body)
	}
	if total > 16<<20 {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompletePins(result map[string]any, paths map[string]string) error {
	var identity geminiProjectAttemptIdentity
	if err := geminiCompleteTyped(result["acquisition"], &identity); err != nil || !geminiCompleteEqual(result["acquisition"], identity) || identity.Client != "gemini" || !identity.Acquisition.Pin || identity.Acquisition.SHA != "bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73" || identity.Node == nil || !identity.Node.Pin || identity.Node.SHA != "fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6" || len(identity.Node.Members) != 1 {
		return geminiCompleteRefusal()
	}
	node := identity.Node.Members[0]
	if node.Name != "node-v24.21.0-linux-x64/bin/node" || node.Kind != "file" || node.Bytes != 126595440 || node.SHA != "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c" {
		return geminiCompleteRefusal()
	}
	expected, seen := map[string]string{}, map[string]bool{}
	if len(identity.Acquisition.Members) < 1 || len(identity.Acquisition.Members) > 512 {
		return geminiCompleteRefusal()
	}
	for _, member := range identity.Acquisition.Members {
		if !geminiCompleteText(member.Name, 4096, true) || member.Name == "." || filepath.IsAbs(member.Name) || filepath.Clean(member.Name) != member.Name || strings.Contains(member.Name, "\\") || seen[member.Name] || member.Bytes < 0 || member.Bytes > 1<<30 {
			return geminiCompleteRefusal()
		}
		for _, component := range strings.Split(member.Name, "/") {
			if component == ".." {
				return geminiCompleteRefusal()
			}
		}
		seen[member.Name] = true
		if member.Kind == "directory" && member.Bytes == 0 && member.SHA == "" {
			continue
		}
		if member.Kind != "file" || !geminiCycleDigest("sha256:"+member.SHA) {
			return geminiCompleteRefusal()
		}
		expected[filepath.Join(filepath.Dir(paths["client"]), member.Name)] = member.SHA
	}
	if expected[paths["client"]] != "41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59" {
		return geminiCompleteRefusal()
	}
	for name, hash := range geminiCycleBuiltinSHA {
		if expected[filepath.Join(filepath.Dir(paths["client"]), "builtin", name, "SKILL.md")] != hash {
			return geminiCompleteRefusal()
		}
	}
	if _, duplicate := expected[paths["node"]]; duplicate {
		return geminiCompleteRefusal()
	}
	expected[paths["node"]] = node.SHA
	if len(identity.RuntimeFiles) != len(expected) {
		return geminiCompleteRefusal()
	}
	for _, file := range identity.RuntimeFiles {
		if expected[file.Path] == "" || expected[file.Path] != file.SHA {
			return geminiCompleteRefusal()
		}
		delete(expected, file.Path)
	}
	return geminiCompleteSourcePins(result, paths)
}

func geminiCompleteSourcePins(result map[string]any, paths map[string]string) error {
	want := map[string]any{"commit": "6006c14a4decfe4b891c6263ad6ad5992e404b5b", "genai_commit": "38cac5bbf4941ec5fa760238bd423c0ecc2c6f04", "genai_converters_blob": "509660270e9cfe2173850ba0a8e9e4ddbeb3b3af", "genai_types_blob": "24ed38f1aaace9165a0dfb4bcbfe794ca94a75de", "mcp_tool_blob": "997f1957eed193f61bf6090b607d3ad402da7742", "output_types_blob": "1d4159a1f8e545ec2da2b8df413cb6778b3f79dc", "tools_blob": "ab2870984aba77e77fc5bacb25009cd47698582e"}
	scanner := map[string]any{"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"}
	if !reflect.DeepEqual(result["client_source"], want) || !reflect.DeepEqual(result["scanner"], scanner) || !reflect.DeepEqual(result["gemini_detection_link"], map[string]any{"path": paths["root"] + "/detection-bin/gemini", "target": paths["client"]}) {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteReceipt(value map[string]any, argv []string, cwd string, limit int64, exit int) error {
	if projectNativeRuntimeKeys(value, "argv cwd exit error context_error elapsed_ms cleanup", "") != nil || !geminiCompleteStrings(value["argv"], argv) || value["cwd"] != cwd || !geminiCompleteIntegerIs(value["exit"], int64(exit)) || value["error"] != false || value["context_error"] != false || value["cleanup"] != "ordinary full-descendant supervision; forced cleanup is a failure" || !geminiCompleteInteger(value["elapsed_ms"], 0, limit) || len(argv) < 1 || len(argv) > 32 || !geminiCompleteText(cwd, 4096, true) {
		return geminiCompleteRefusal()
	}
	for _, arg := range argv {
		if !geminiCompleteText(arg, 4096, true) {
			return geminiCompleteRefusal()
		}
	}
	return nil
}
