//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func geminiCompleteContractPython(t *testing.T, code string, input []byte) []byte {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("independent Python success reader is required", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	scripts := filepath.Join(RepoRoot(t), "scripts")
	command := exec.CommandContext(ctx, python, "-c", code)
	command.Dir = scripts
	command.Env = []string{"PATH=/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE=1", "PYTHONPATH=" + scripts}
	command.Stdin = bytes.NewReader(input)
	body, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("independent inert Python fixture/reader failed: %v: %s", err, body)
	}
	return body
}

func TestGeminiCycleCompleteIndependentReadersAcceptEachOthersSyntheticEvidence(t *testing.T) {
	files, source := geminiCompleteSyntheticFiles(t)
	input := geminiCompleteContractEncode(t, map[string]any{"files": files, "source": map[string]any{"source_commit": source["installer_base_commit"], "source_tree": source["installer_tree"]}})
	const code = `import base64,json,sys
import project_native_runtime as runtime
from test_project_native_gemini_complete import fixture
value=json.load(sys.stdin)
bodies={name:base64.b64decode(body,validate=True) for name,body in value["files"].items()}
result=runtime.attempt_members(bodies,value["source"],"gemini")
assert result["runtime_attempt_complete"] is True and result["test_failed"] is False
assert all(v is True for v in result["qualification"].values())
assert result["vendor_authentication_qualified"] is False and result["real_model_qualified"] is False
other,source=fixture()
print(json.dumps({"files":{name:base64.b64encode(body).decode("ascii") for name,body in other.items()},
"identity":{"installer_base_commit":source["source_commit"],"installer_tree":source["source_tree"],"installer_source_state":"committed","installer_patch_sha256":[]}}))`
	body := geminiCompleteContractPython(t, code, input)
	var other struct {
		Files    map[string][]byte `json:"files"`
		Identity map[string]any    `json:"identity"`
	}
	if err := json.Unmarshal(body, &other); err != nil {
		t.Fatal("independent Python synthetic evidence envelope", err)
	}
	if len(other.Files) != 57 || geminiCycleCompleteFiles(other.Files, other.Identity) != nil {
		t.Fatal("Go reader refused independent Python full synthetic evidence")
	}
	t.Log("Both independent full 57-member synthetic fixtures admitted by the other language; no native execution or qualification")
}
