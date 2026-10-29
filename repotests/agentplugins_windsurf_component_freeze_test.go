//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
