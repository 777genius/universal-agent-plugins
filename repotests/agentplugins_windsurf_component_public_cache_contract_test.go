//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func windAssessmentOutput(t *testing.T, assessment domain.SecurityAssessment) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"schema_version": 1, "command": "add", "result": "success", "data": map[string]any{"batch": true, "succeeded": 1, "failed": 0, "security": assessment}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func windAssessmentActual(t *testing.T, w *windComponentFixture, scope string) ([]byte, windPublicCache, map[string]map[string]windProfileRecord) {
	t.Helper()
	r := w.recipes[scope]
	raw := windComponentStageActual(t, w, scope)
	global := w.recipes["user"].StateRoot
	start := time.Now()
	// Keep the actual file writes inside the strict interval despite local
	// sub-millisecond filesystem timestamp granularity. Production has no delay.
	time.Sleep(20 * time.Millisecond)
	snapshot, err := (sourceacquisition.Acquirer{TempRoot: global}).AcquireLocal(w.f.ctx, r.Source)
	if err != nil {
		t.Fatal(err)
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	scanner := &windInertCacheScanner{}
	assessment, err := (securityscan.Evaluator{Scanner: scanner, Requirement: securityscan.DefaultRequirement(), Cache: securityscan.FileCache{Root: filepath.Join(global, windAssessmentDirectory)}}).Evaluate(w.f.ctx, domain.SecurityEvaluationInput{SnapshotRoot: r.Source, TreeDigest: r.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"]), PluginName: windComponentName})
	end := time.Now()
	if err != nil || scanner.calls != 1 {
		t.Fatal("actual cold evaluator", err)
	}
	cache, err := windPublicAssessment(windAssessmentOutput(t, assessment), r, start, end)
	if err != nil {
		t.Fatal(err)
	}
	frames, err := w.proofFrames(r)
	if err != nil {
		t.Fatal(err)
	}
	return raw, cache, frames
}

func TestWindsurfPublicAssessmentUsesActualCacheAndExactGlobalFrames(t *testing.T) {
	w := windComponentInert(t)
	for _, scope := range []string{"user", "a", "b"} {
		raw, cache, frames := windAssessmentActual(t, w, scope)
		r := w.recipes[scope]
		before := maps.Clone(w.trees[w.recipes["user"].StateRoot])
		if _, err := w.validateProof(r, raw); err == nil {
			t.Fatal("low-level proof admitted unproved cache writes")
		}
		if err := w.publicStateFrames(r, frames, &cache); err != nil {
			t.Fatal("exact actual source/cache frame refused", scope, err)
		}
		if _, err := w.validateProofWithAssessment(r, raw, &cache); err != nil {
			t.Fatal("complete real state/native/cache proof refused", scope, err)
		}
		wantRoots := 4
		if scope == "user" {
			wantRoots = 3
		}
		if len(r.proven) != wantRoots || !reflect.DeepEqual(before, w.trees[w.recipes["user"].StateRoot]) {
			t.Fatal("accepted proof roots/baseline renewal differ")
		}
		w.freezeProof(t, r)
		w.check(t)
	}
}

func TestWindsurfPublicAssessmentRefusesSchemaSubjectAndNoncoldEvidence(t *testing.T) {
	w := windComponentInert(t)
	_, cache, _ := windAssessmentActual(t, w, "user")
	r := w.recipes["user"]
	for _, kind := range []string{"unknown", "subject", "manifest", "scanner", "policy", "report", "cache", "signed", "blocking", "count", "command", "batch", "missing", "trailing", "duplicate", "window"} {
		t.Run(kind, func(t *testing.T) {
			value, err := projectNativeRuntimeJSON(windAssessmentOutput(t, cache.assessment))
			if err != nil {
				t.Fatal(err)
			}
			outer := value.(map[string]any)
			data := outer["data"].(map[string]any)
			assessment := data["security"].(map[string]any)
			start, end := cache.start, cache.end
			switch kind {
			case "unknown":
				assessment["extra"] = true
			case "subject":
				assessment["subject"].(map[string]any)["tree_digest"] = "sha256:" + windsurfConfigDigest([]byte("foreign"))
			case "manifest":
				assessment["subject"].(map[string]any)["manifest_digest"] = "sha256:" + windsurfConfigDigest([]byte("foreign"))
			case "scanner":
				assessment["scanner"].(map[string]any)["version"] = "foreign"
			case "policy":
				assessment["policy"].(map[string]any)["version"] = 1
			case "report":
				assessment["report_digest"] = "invalid"
			case "cache":
				assessment["evidence_source"] = "cache"
			case "signed":
				assessment["evidence_source"] = "signed_index"
			case "blocking":
				assessment["counts"] = map[string]int{"blocking": 1, "warnings": 0, "total": 1}
				assessment["outcome"] = "blocking_findings"
			case "count":
				assessment["counts"].(map[string]any)["total"] = 1
			case "command":
				outer["command"] = "update"
			case "batch":
				data["batch"] = false
			case "missing":
				delete(data, "security")
			case "window":
				start, end = end, start
			}
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "trailing" {
				body = append(body, []byte(` {}`)...)
			}
			if kind == "duplicate" {
				body = []byte(`{"schema_version":1,"schema_version":1,"command":"add","result":"success","data":{}}`)
			}
			if _, err := windPublicAssessment(body, r, start, end); err == nil {
				t.Fatal("hostile public security admitted", kind)
			}
		})
	}
}

func TestWindsurfPublicAssessmentRefusesAuthorityMembershipAndLateMutation(t *testing.T) {
	w := windComponentInert(t)
	raw, first, _ := windAssessmentActual(t, w, "user")
	if _, err := w.validateProofWithAssessment(w.recipes["user"], raw, &first); err != nil {
		t.Fatal(err)
	}
	w.freezeProof(t, w.recipes["user"])
	raw, cache, frames := windAssessmentActual(t, w, "a")
	r := w.recipes["a"]
	global := w.recipes["user"].StateRoot
	leaf, err := w.assessmentFrame(r, frames[global], cache)
	if err != nil {
		t.Fatal(err)
	}
	prior := ""
	for key := range w.trees[global] {
		if filepath.Dir(key) == windAssessmentDirectory {
			prior = key
		}
	}
	if prior == "" {
		t.Fatal("real prior assessment required")
	}
	for _, kind := range []string{"mode", "uid", "gid", "device", "link", "inode", "size", "hash", "type", "mtime", "ctime", "bad-nsec", "parent-mode", "parent-inode", "parent-owner", "parent-links", "parent-size", "parent-time", "missing", "unknown", "temporary", "prior", "security", "user-state", "root-inode", "root-size", "root-time", "root-links", "cache-hit"} {
		t.Run(kind, func(t *testing.T) {
			copyFrames := maps.Clone(frames)
			after := maps.Clone(frames[global])
			copyFrames[global] = after
			file, parent, root := after[leaf], after[windAssessmentDirectory], after["."]
			before := w.trees[global]
			switch kind {
			case "mode":
				file.Mode = 0100644
			case "uid":
				file.UID++
			case "gid":
				file.GID++
			case "device":
				file.Device++
			case "link":
				file.Links++
			case "inode":
				file.Inode = 0
			case "size":
				file.Size++
			case "hash":
				file.SHA256 = windsurfConfigDigest([]byte("foreign"))
			case "type":
				file.Kind = "directory"
			case "mtime":
				file.Modified[0]--
			case "ctime":
				file.Changed[0]++
			case "bad-nsec":
				file.Modified[1] = 1e9
			case "parent-mode":
				parent.Mode = 040755
			case "parent-inode":
				parent.Inode++
			case "parent-owner":
				parent.GID++
			case "parent-links":
				parent.Links++
			case "parent-size":
				parent.Size = 1<<20 + 1
			case "parent-time":
				parent.Changed[0]++
			case "root-inode":
				root.Inode++
			case "root-size":
				root.Size++
			case "root-time":
				root.Modified[0]--
			case "root-links":
				root.Links++
			case "prior":
				old := after[prior]
				old.Changed[0]++
				after[prior] = old
			case "security":
				old := after["security"]
				old.Modified[0]++
				after["security"] = old
			case "user-state":
				old := after["state-v2.json"]
				old.SHA256 = "foreign"
				after["state-v2.json"] = old
			case "unknown":
				after[windAssessmentDirectory+"/foreign.json"] = file
			case "temporary":
				after[windAssessmentDirectory+"/.assessment-foreign.tmp"] = file
			case "cache-hit":
				renewed := maps.Clone(before)
				renewed[leaf] = file
				w.trees[global] = renewed
			}
			after[leaf], after[windAssessmentDirectory], after["."] = file, parent, root
			if kind == "missing" {
				delete(after, leaf)
			}
			if err := w.publicStateFrames(r, copyFrames, &cache); err == nil {
				t.Fatal("hostile cache/global frame admitted", kind)
			}
			w.trees[global] = before
		})
	}
	if _, err := w.validateProofWithAssessment(r, raw, &cache); err != nil {
		t.Fatal(err)
	}
	accepted := r.proven[global][leaf]
	path := filepath.Join(global, leaf)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".replacement", body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".replacement", path); err != nil {
		t.Fatal(err)
	}
	w.freezeProof(t, r)
	if w.trees[global][leaf] != accepted || windFrozenTrees(w.f.ctx, w.trees) == nil {
		t.Fatal("late same-byte cache replacement adopted")
	}
}
