//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func windComponentPublicJSON(raw []byte) error {
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		return fmt.Errorf("component strict public JSON refused")
	}
	return nativeValidateInstallerJSON(raw)
}
func (w *windComponentFixture) proofFrames(r *windComponentRecipe) (map[string]map[string]windProfileRecord, error) {
	result := map[string]map[string]windProfileRecord{}
	for _, path := range []string{r.StateRoot, r.Profile, r.Project} {
		frame, err := windOwnedTree(w.f.ctx, path)
		if err != nil {
			return nil, fmt.Errorf("component proof frame refused")
		}
		result[path] = frame
	}
	return result, nil
}
func (w *windComponentFixture) proofStable(r *windComponentRecipe, frames map[string]map[string]windProfileRecord, raw []byte) error {
	state := frames[r.StateRoot]["state-v2.json"]
	if state.Kind != "file" || state.Size != int64(len(raw)) || state.SHA256 != windsurfConfigDigest(raw) {
		return fmt.Errorf("component raw state/frame correlation refused")
	}
	if err := windFrozenTrees(w.f.ctx, frames); err != nil {
		return fmt.Errorf("component complete proof frames changed")
	}
	r.proven = frames
	return nil
}
func (w *windComponentFixture) freezeProof(t *testing.T, r *windComponentRecipe) {
	t.Helper()
	if len(r.proven) != 3 || r.proven[r.StateRoot]["state-v2.json"].Kind != "file" || r.proven[r.Profile]["mcp_config.json"].Kind != "file" || r.proven[r.Project]["."].Kind != "directory" {
		t.Fatal("component complete proven frame required before freeze")
	}
	for path, frame := range r.proven {
		cloned := make(map[string]windProfileRecord, len(frame))
		for key, record := range frame {
			cloned[key] = record
		}
		w.trees[path] = cloned
	}
	w.f.protected[r.Config] = r.proven[r.Profile]["mcp_config.json"].SHA256
	w.f.protected[filepath.Join(r.StateRoot, "state-v2.json")] = r.proven[r.StateRoot]["state-v2.json"].SHA256
}
func TestWindsurfComponentPublicJSONRefusesDuplicateTrailingNonfiniteAndNestedHostiles(t *testing.T) {
	valid := []byte(`{"schema_version":1,"command":"add","result":"success","data":{}}`)
	if windComponentPublicJSON(valid) != nil {
		t.Fatal("valid current installer envelope refused")
	}
	for _, raw := range [][]byte{
		[]byte(`{"schema_version":1,"command":"add","result":"failure","result":"success","data":{}}`),
		[]byte(`{"schema_version":1,"command":"add","result":"success","data":{"nested":{"key":1,"key":2}}}`),
		append(bytes.Clone(valid), []byte(` {"extra":true}`)...),
		[]byte(`{"schema_version":1,"command":"add","result":"success","data":{"number":1e999}}`),
		[]byte(`{"schema_version":1,"command":"add","result":"success","data":{"number":NaN}}`),
	} {
		if windComponentPublicJSON(raw) == nil {
			t.Fatal("non-strict public envelope accepted")
		}
	}
}
func TestWindsurfComponentFreezeUsesOnlyProvenFramesAndRefusesLaterMutation(t *testing.T) {
	w := windComponentInert(t)
	r := w.recipes["user"]
	raw := windComponentStageActual(t, w, "user")
	if _, err := w.validateProof(r, raw); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.StateRoot, "plugin-data", "caller-owned", "keep.txt")
	before := r.proven[r.StateRoot]["plugin-data/caller-owned/keep.txt"]
	if err := os.WriteFile(path, []byte("changed after complete proof"), 0600); err != nil {
		t.Fatal(err)
	}
	w.freezeProof(t, r)
	if w.trees[r.StateRoot]["plugin-data/caller-owned/keep.txt"] != before || windFrozenTrees(w.f.ctx, w.trees) == nil {
		t.Fatal("unchecked later bytes adopted as a baseline")
	}
	expected := w.trees[r.StateRoot]["plugin-data/caller-owned/keep.txt"]
	r.proven[r.StateRoot]["plugin-data/caller-owned/keep.txt"] = windProfileRecord{}
	if !reflect.DeepEqual(expected, w.trees[r.StateRoot]["plugin-data/caller-owned/keep.txt"]) {
		t.Fatal("retained proof frame aliases frozen authority")
	}
}

func TestWindsurfComponentFrozenRefusalIdentifiesActualFileBeforeParentMetadata(t *testing.T) {
	for _, operation := range []string{"changed", "added", "missing", "metadata"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			name := "private-name\nDO_NOT_DISCLOSE"
			path := filepath.Join(root, name)
			projectNativeRuntimeWrite(t, path, []byte("PRIVATE ORIGINAL CONTENT"), 0600)
			before, err := windOwnedTree(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "changed":
				err = os.WriteFile(path, []byte("PRIVATE CHANGED CONTENT"), 0600)
			case "added":
				name += "-added"
				err = os.WriteFile(filepath.Join(root, name), []byte("PRIVATE ADDED CONTENT"), 0600)
			case "missing":
				err = os.Remove(path)
			case "metadata":
				err = os.Chmod(path, 0400)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = windFrozenTrees(context.Background(), map[string]map[string]windProfileRecord{root: before})
			if err == nil {
				t.Fatal("frozen mutation accepted")
			}
			message := err.Error()
			wantOperation := operation
			if operation == "metadata" {
				wantOperation = "changed"
			}
			for _, wanted := range []string{"root_sha256=" + windsurfConfigDigest([]byte(root)), "key_sha256=" + windsurfConfigDigest([]byte(name)), "operation=" + wantOperation, "remaining_in_root="} {
				if !strings.Contains(message, wanted) {
					t.Fatal("frozen refusal lost actual file boundary", message)
				}
			}
			if len(message) > 2<<10 || strings.Contains(message, root) || strings.Contains(message, "PRIVATE") || strings.Contains(message, "DO_NOT_DISCLOSE") || strings.ContainsRune(message, '\n') {
				t.Fatal("frozen refusal leaked unknown path/content or exceeded bound")
			}
			if operation == "metadata" && (!strings.Contains(message, "mode=false") || !strings.Contains(message, "sha256=true")) {
				t.Fatal("frozen metadata/content distinction lost")
			}
		})
	}
}

func TestWindsurfComponentFrozenRefusalOrderingFallbackAndOpaqueInventory(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "a"), filepath.Join(root, "z")
	projectNativeRuntimeWrite(t, filepath.Join(first, "held"), []byte("old"), 0600)
	projectNativeRuntimeWrite(t, filepath.Join(second, "held"), []byte("old"), 0600)
	trees := map[string]map[string]windProfileRecord{}
	for _, path := range []string{second, first} {
		tree, err := windOwnedTree(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		trees[path] = tree
		if err := os.WriteFile(filepath.Join(path, "held"), []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	err := windFrozenTrees(context.Background(), trees)
	if err == nil || !strings.Contains(err.Error(), "root_sha256="+windsurfConfigDigest([]byte(first))) {
		t.Fatal("frozen root ordering changed")
	}
	before := map[string]windProfileRecord{".": {Kind: "directory", Mode: 040755}}
	after := map[string]windProfileRecord{".": {Kind: "directory", Mode: 040700}}
	if message := windFrozenTreeRefusal(strings.Repeat("private", 4096), before, after).Error(); !strings.Contains(message, "key_sha256="+windsurfConfigDigest([]byte("."))) || len(message) > 2<<10 || strings.Contains(message, "private") {
		t.Fatal("directory-only fallback/path hashing failed")
	}
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	err = windFrozenTrees(context.Background(), trees)
	if err == nil || !strings.Contains(err.Error(), "inventory=refused") || strings.Contains(err.Error(), first) {
		t.Fatal("frozen inventory failure became content-bearing or accepted")
	}
}
