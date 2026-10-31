//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Cross the actual current Store/Kernel/adapter writer into the independent
// Python admission; handwritten envelopes cannot substitute for this seam.
func TestWindsurfComponentActualWriterProofsPassPythonAdmission(t *testing.T) {
	interpreter, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python3 required for the inert cross-language admission contract")
	}
	w := windComponentInert(t)
	proofs := make([]map[string]any, 0, 3)
	for _, scope := range []string{"user", "a", "b"} {
		raw := windComponentStageActual(t, w, scope)
		proof, err := w.validateProof(w.recipes[scope], raw)
		if err != nil {
			t.Fatal(err)
		}
		binding := proof["binding"]
		encoded, err := json.Marshal(binding)
		if err != nil {
			t.Fatal(err)
		}
		value, err := projectNativeRuntimeJSON(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if _, present := value.(map[string]any)["install_intent"]; present {
			t.Fatal("actual fresh automatic writer emitted an install-intent field")
		}
		proofs = append(proofs, proof)
	}
	input, err := json.Marshal(map[string]any{"layout": w.f.evidence["component_layout"], "proofs": proofs})
	if err != nil || len(input) > 1<<20 {
		t.Fatal("bounded actual writer proof input")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Clean(filepath.Join(cwd, "..", "scripts"))
	script := `import sys,json
sys.path.insert(0,sys.argv[1])
import project_native_runtime as runtime
import project_native_windsurf_components as component
value=runtime.strict_json(sys.stdin.buffer.read((1<<20)+1))
identities=[component._proof(row,scope,value["layout"],runtime.strict_json) for row,scope in zip(value["proofs"],("user","a","b"))]
assert len(value["proofs"])==3 and all(len({row[i] for row in identities})==3 for i in range(4))
print("actual-current-writer-proofs:3")
`
	ctx, cancel := context.WithTimeout(w.f.ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, interpreter, "-I", "-S", "-B", "-c", script, scripts)
	command.Dir = scripts
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "HOME=" + w.f.base.Home}
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil || ctx.Err() != nil || string(output) != "actual-current-writer-proofs:3\n" {
		t.Fatalf("inert actual writer to Python admission failed: %v %s", err, output)
	}
}
