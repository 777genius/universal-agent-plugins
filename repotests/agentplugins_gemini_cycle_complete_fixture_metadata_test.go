//go:build linux

package pluginkitairepo_test

import (
	"path/filepath"
	"testing"
	"time"
)

func (f *geminiCompleteContractFixture) acquisition() map[string]any {
	// Only known pinned identities are used in this synthetic metadata inventory;
	// it is never described as an acquired or executed client distribution.
	members := []map[string]any{
		{"name": "gemini.js", "kind": "file", "bytes": 5092, "sha256": "41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59"},
		{"name": "builtin/antigravity-support/SKILL.md", "kind": "file", "bytes": 3364, "sha256": "2b866bce962f642be8c0dd74ffc2548fa29f2100bc945518b2f738b7d7d6fd38"},
		{"name": "builtin/skill-creator/SKILL.md", "kind": "file", "bytes": 20225, "sha256": "aab93935325978cb5a54b4907ae1ca11d7b4a903925ed0cc1739345392280db9"},
	}
	runtime := make([]map[string]any, 0, len(members)+1)
	for _, member := range members {
		runtime = append(runtime, map[string]any{"path": filepath.Join(filepath.Dir(f.paths["client"]), member["name"].(string)), "sha256": member["sha256"]})
	}
	nodeSHA := "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c"
	runtime = append(runtime, map[string]any{"path": f.paths["node"], "sha256": nodeSHA})
	return map[string]any{"client": "gemini", "acquisition": map[string]any{"sha256": "bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73", "pin_verified": true, "members": members}, "node": map[string]any{"sha256": "fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6", "pin_verified": true, "members": []map[string]any{{"name": "node-v24.21.0-linux-x64/bin/node", "kind": "file", "bytes": 126595440, "sha256": nodeSHA}}}, "runtime_files": runtime}
}

func (f *geminiCompleteContractFixture) metadata() {
	result := map[string]any{
		"client": "gemini", "provider_kind": "scripted_loopback", "activation": "prepared", "vendor_authentication_qualified": false, "real_model_qualified": false,
		"capture_mode": "gemini-complete-cycle", "cycle_capture_review_required": false, "cycle_observed_complete": true, "runtime_attempt_complete": true, "test_failed": false, "attempt_stage": "complete-cycle", "elapsed_ms": 31000,
		"qualification":            map[string]bool{"project-lifecycle": true, "native-restart-removal": true, "mcp-initialize": true, "mcp-tools-list": true, "mcp-tools-call": true},
		"cycle_public_stages":      []string{"git-a", "git-b", "git-user", "version", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"},
		"cycle_listing_stages":     []string{"baseline-a", "baseline-b", "baseline-user", "installed-user", "installed-a", "installed-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"},
		"cycle_native_stages":      []string{"baseline-a", "installed-user", "installed-a", "installed-b", "restarted-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"},
		"cycle_provider_completed": []string{"baseline-a", "installed-user", "installed-a", "installed-b", "restarted-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"},
		"cycle_layout":             f.paths, "source_identity": f.contractSource(), "acquisition": f.acquisition(), "gemini_detection_link": map[string]string{"path": f.paths["root"] + "/detection-bin/gemini", "target": f.paths["client"]},
		"client_source": map[string]string{"commit": "6006c14a4decfe4b891c6263ad6ad5992e404b5b", "genai_commit": "38cac5bbf4941ec5fa760238bd423c0ecc2c6f04", "genai_converters_blob": "509660270e9cfe2173850ba0a8e9e4ddbeb3b3af", "genai_types_blob": "24ed38f1aaace9165a0dfb4bcbfe794ca94a75de", "mcp_tool_blob": "997f1957eed193f61bf6090b607d3ad402da7742", "output_types_blob": "1d4159a1f8e545ec2da2b8df413cb6778b3f79dc", "tools_blob": "ab2870984aba77e77fc5bacb25009cd47698582e"},
		"scanner":       map[string]string{"archive_sha256": "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "binary_sha256": "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba", "platform": "linux-amd64", "route": "prepopulated supported scanner cache; ordinary CLI security assessment", "source_tag": "v0.1.3", "version": "0.1.3"},
		"cycle_completion": map[string]any{"schema_version": 1, "started_at": f.date(f.started), "work_deadline_at": f.date(f.started.Add(150 * time.Second)), "finished_at": f.date(f.started.Add(30 * time.Second)), "work_context_error": false, "original_checks_error": false, "open_authorities": 0, "open_events": false,
			"provider": map[string]any{"shutdown_complete": true, "forced_close": false, "serve_returned": true, "error": false, "busy": 0, "current_stage": "", "requests": 16, "completed_stages": []string{"baseline-a", "installed-user", "installed-a", "installed-b", "restarted-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"}}},
	}
	f.files["runtime-attempt.json"] = geminiCompleteContractEncode(f.t, result)
}

func geminiCompleteSyntheticFiles(t *testing.T) (map[string][]byte, map[string]any) {
	t.Helper()
	f := geminiCompleteContractFixtureNew(t)
	f.publicRecords()
	f.observations()
	f.encodeRows()
	f.metadata()
	return f.files, geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, f.contractSource()))
}

func TestGeminiCycleCompleteIndependentSyntheticFullProtocol(t *testing.T) {
	files, source := geminiCompleteSyntheticFiles(t)
	if len(files) != 57 {
		t.Fatal("independent synthetic complete member count")
	}
	if err := geminiCycleCompleteFiles(files, source); err != nil {
		t.Fatal("independent synthetic complete protocol refused", err)
	}
	for name := range files {
		copyFiles := map[string][]byte{}
		for candidate, body := range files {
			if candidate != name {
				copyFiles[candidate] = body
			}
		}
		if err := geminiCycleCompleteFiles(copyFiles, source); err == nil {
			t.Fatal("missing semantic success member admitted", name)
		}
	}
}
