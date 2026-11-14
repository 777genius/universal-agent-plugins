//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func geminiCycleInert(t *testing.T) *geminiProjectCycle {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	base := &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "project"), Probe: filepath.Join(root, "probe"), Events: filepath.Join(root, "events.jsonl"), Nonce: "inert-independent-gemini-cycle"}
	f := &geminiProjectAttempt{base: base, ctx: ctx, cancel: cancel, started: time.Now(), files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, evidence: map[string]any{}, secret: "inert-scripted-secret", installer: filepath.Join(root, "agentplugins"), client: filepath.Join(root, "gemini.js"), node: filepath.Join(root, "node"), git: filepath.Join(root, "git"), other: filepath.Join(root, "project-B"), outside: filepath.Join(root, "outside")}
	for _, path := range []string{base.Home, base.Project, f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{base.Probe, f.installer} {
		projectNativeRuntimeWrite(t, path, []byte("inert owned fixture; never executed\n"), 0700)
		f.immutable[path] = nativeSHA(t, path)
	}
	c := newGeminiProjectCycle(f)
	c.prepare(t)
	return c
}
func TestGeminiCycleActualStoreStagerAdapterMatchesAllThreeIndependentAuthorities(t *testing.T) {
	c := geminiCycleInert(t)
	for _, scope := range []string{"user", "a", "b"} {
		geminiCycleStageActual(t, c, scope)
		c.check(t)
	}
	if c.recipes["user"].Binding.Activation != domain.ActivationActive || c.recipes["a"].Binding.Activation != domain.ActivationPrepared {
		t.Fatal("Gemini receipt statuses conflated")
	}
	for _, r := range c.recipes {
		if err := geminiCycleAbsent(filepath.Join(r.Data, "native-marker.txt")); err != nil {
			t.Fatal("inert stage fabricated native marker")
		}
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python3 required for inert actual writer admission seam")
	}
	proofs := c.aggregates["cycle-proof.jsonl"]
	input, _ := json.Marshal(map[string]any{"proofs": string(proofs), "layout": c.f.evidence["cycle_layout"]})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Join(filepath.Dir(wd), "scripts")
	code := `import sys,json
sys.path.insert(0,sys.argv[1])
from project_native_gemini_cycle_proof import proofs
from project_native_gemini_cycle_bytes import rows
value=json.loads(sys.stdin.buffer.read((1<<20)+1))
proofs(rows(value['proofs'].encode()),value['layout'])
print('actual-gemini-user-a-b-proofs:3')`
	ctx, cancel := context.WithTimeout(c.f.ctx, 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, python, "-I", "-S", "-B", "-c", code, scripts)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8", "HOME=" + c.f.base.Home}
	command.Stdin = bytes.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil || ctx.Err() != nil || string(output) != "actual-gemini-user-a-b-proofs:3\n" {
		t.Fatalf("actual writer/Python source/state proof mismatch: %v %s", err, output)
	}
}
func TestGeminiCycleStrictCurrentStateAndIndependentNativeObjectsRefuseHostiles(t *testing.T) {
	c := geminiCycleInert(t)
	raw := geminiCycleStageActual(t, c, "a")
	r := c.recipes["a"]
	state, err := geminiCycleState(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"schema", "unknown", "duplicate", "trailing", "source", "package", "object-id", "object-path", "object-digest", "extra-object", "extra-data", "data-path", "data-body", "config-key", "config-foreign"} {
		t.Run(kind, func(t *testing.T) {
			candidate := bytes.Clone(raw)
			switch kind {
			case "schema":
				changed := state
				changed.SchemaVersion = 4
				candidate, _ = json.Marshal(changed)
			case "unknown":
				candidate = append([]byte(`{"unknown":true,`), candidate[1:]...)
			case "duplicate":
				candidate = append([]byte(`{"schema_version":5,`), candidate[1:]...)
			case "trailing":
				candidate = append(candidate, []byte(` {}`)...)
			case "source", "package":
				i := state.Installations[0]
				if kind == "source" {
					i.Source.CanonicalSource = c.recipes["b"].Source
				} else {
					i.Package.Inventory.Skills = []string{"foreign"}
				}
				if geminiCycleSource(i, r) == nil {
					t.Fatal("foreign independent recipe accepted")
				}
				return
			case "object-id", "object-path", "object-digest", "extra-object":
				b := r.Binding
				b.NativeObjects = append([]domain.NativeObjectOwnership{}, b.NativeObjects...)
				switch kind {
				case "object-id":
					b.NativeObjects[0].ObjectID = "foreign"
				case "object-path":
					b.NativeObjects[0].Path = c.recipes["b"].Profile
				case "object-digest":
					b.NativeObjects[0].ManagedDigest = "sha256:" + strings.Repeat("0", 64)
				case "extra-object":
					b.NativeObjects = append(b.NativeObjects, b.NativeObjects[0])
				}
				payload, e := c.payloadProof(r)
				if e != nil {
					t.Fatal(e)
				}
				if geminiCycleObjects(r, b, payload, c.expectedEntry(r)) == nil {
					t.Fatal("foreign native ownership accepted")
				}
				return
			case "extra-data", "data-path":
				i := state.Installations[0]
				i.DataReceipts = map[string]domain.DataReceipt{}
				for k, v := range r.Installation.DataReceipts {
					i.DataReceipts[k] = v
				}
				if kind == "extra-data" {
					i.DataReceipts["foreign"] = domain.DataReceipt{}
				} else {
					d := i.DataReceipts[r.Binding.DataReceiptID]
					d.Locator = c.recipes["b"].StateRoot
					i.DataReceipts[d.DataReceiptID] = d
				}
				if _, e := c.dataProof(r, i, r.Binding.DataReceiptID, r.Binding.PhysicalArtifact); e == nil {
					t.Fatal("foreign data authority accepted")
				}
				return
			case "data-body":
				path := filepath.Join(r.Data, "foreign")
				projectNativeRuntimeWrite(t, path, []byte("private"), 0600)
				defer func() {
					if err := os.Remove(path); err != nil {
						t.Error(err)
					}
				}()
				if _, e := c.dataProof(r, r.Installation, r.Binding.DataReceiptID, r.Binding.PhysicalArtifact); e == nil {
					t.Fatal("unowned data child accepted")
				}
				return
			case "config-key", "config-foreign":
				body, e := os.ReadFile(r.Config)
				if e != nil {
					t.Fatal(e)
				}
				defer func() {
					if err := os.WriteFile(r.Config, body, 0600); err != nil {
						t.Error(err)
					}
				}()
				var value map[string]any
				_ = json.Unmarshal(body, &value)
				if kind == "config-key" {
					value["unexpected"] = true
				} else {
					value["mcpServers"].(map[string]any)["foreign-a"] = map[string]any{"httpUrl": "http://127.0.0.1:9/other"}
				}
				changed, _ := json.Marshal(value)
				if e := os.WriteFile(r.Config, changed, 0600); e != nil {
					t.Fatal(e)
				}
				if _, e := c.configProof(r, true); e == nil {
					t.Fatal("unproved full settings object accepted")
				}
				return
			}
			if _, e := geminiCycleState(candidate); e == nil {
				t.Fatal("hostile current state accepted")
			}
		})
	}
	if !reflect.DeepEqual(r.Installation.DataReceipts, state.Installations[0].DataReceipts) {
		t.Fatal("hostile proof aliased actual state")
	}
}
