//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const windInertCacheReport = `{"schema_version":1,"tool":{"name":"lintai","version":"0.1.3"},"policy":{"id":"agent-plugin-install","version":2,"presets":["recommended","preview","threat-review","supply-chain","advisory"]},"stats":{"scanned_files":3,"skipped_files":0},"findings":[],"diagnostics":[],"runtime_errors":[]}`

type windInertCacheScanner struct{ calls int }

func (s *windInertCacheScanner) Scan(context.Context, string) ([]byte, error) {
	s.calls++
	return []byte(windInertCacheReport), nil
}

func TestWindsurfPublicStateGuardRefusesDefaultAssessmentCache(t *testing.T) {
	w := windComponentInert(t)
	r := w.recipes["user"]
	marker := filepath.Join(r.StateRoot, "security", "lintai", "caller-owned.txt")
	projectNativeRuntimeWrite(t, marker, []byte("inert original scanner-cache marker; never executed\n"), 0600)
	w.freeze(t, r.StateRoot)
	windComponentStageActual(t, w, "user")
	if err := w.scopeClosure(r); err != nil {
		t.Fatal("exact installed setup did not pass the original closure", err)
	}
	original := maps.Clone(w.trees[r.StateRoot])
	snapshot, err := (sourceacquisition.Acquirer{TempRoot: r.StateRoot}).AcquireLocal(w.f.ctx, r.Source)
	if err != nil {
		t.Fatal(err)
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	afterSource, err := windOwnedTree(w.f.ctx, r.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	unchanged := maps.Clone(afterSource)
	unchanged["."] = original["."]
	oldRoot, nowRoot := original["."], afterSource["."]
	if !reflect.DeepEqual(original, unchanged) || !windRuntimeAuthority(oldRoot, nowRoot) || oldRoot.Size != nowRoot.Size || oldRoot.SHA256 != nowRoot.SHA256 || oldRoot.Modified == nowRoot.Modified && oldRoot.Changed == nowRoot.Changed {
		t.Fatal("default source snapshot changed more than root timestamps")
	}
	if err := w.scopeClosure(r); err != nil || !reflect.DeepEqual(original, w.trees[r.StateRoot]) {
		t.Fatal("source scratch effect changed the original User closure/baseline", err)
	}
	t.Log("actual default source snapshot: only the shared state root timestamps changed; all membership and other records fixed")
	before := afterSource
	subject := domain.SecuritySubject{TreeDigest: r.Digest, ManifestDigest: "sha256:" + windsurfConfigDigest(r.Authored["plugin.json"])}
	requirement := securityscan.DefaultRequirement()
	key := securityscan.CacheKey(subject, requirement)
	cache := securityscan.FileCache{Root: filepath.Join(r.StateRoot, "security", "assessments")}
	scanner := &windInertCacheScanner{}
	assessment, err := (securityscan.Evaluator{Scanner: scanner, Cache: cache, Requirement: requirement}).Evaluate(w.f.ctx, domain.SecurityEvaluationInput{SnapshotRoot: r.Source, TreeDigest: subject.TreeDigest, ManifestDigest: subject.ManifestDigest, PluginName: windComponentName})
	if err != nil || scanner.calls != 1 || assessment.Validate(requirement, subject) != nil || assessment.Evidence != domain.SecurityEvidenceLocalScan {
		t.Fatal("real evaluator/file cache failed exact inert scan", err)
	}
	assessment.Evidence = ""
	expected, err := json.Marshal(assessment)
	if err != nil {
		t.Fatal(err)
	}
	after, err := windOwnedTree(w.f.ctx, r.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	file := "security/assessments/" + key + ".json"
	if after[file].Kind != "file" || after[file].Mode != 0100600 || after[file].SHA256 != windsurfConfigDigest(expected) || after["security/assessments"].Kind != "directory" || after["security/assessments"].Mode != 040700 {
		t.Fatal("exact default security cache file/directory differs")
	}
	for name, record := range before {
		if name != windAssessmentDirectory && after[name] != record {
			t.Fatal("default cache changed an unrelated original state record", name)
		}
	}
	if len(after) != len(before)+1 {
		t.Fatal("default cache added unknown state membership")
	}
	for name := range after {
		if _, original := before[name]; !original && name != file {
			t.Fatal("default cache added an unrelated state member")
		}
	}
	old, now := before[windAssessmentDirectory], after[windAssessmentDirectory]
	if !windRuntimeAuthority(old, now) || old.SHA256 != now.SHA256 || reflect.DeepEqual(old, now) {
		t.Fatal("default cache parent effect differs")
	}
	if raw, err := os.ReadFile(filepath.Join(r.StateRoot, file)); err != nil || string(raw) != string(expected) {
		t.Fatal("real cache serialization differs", err)
	}
	if err := w.scopeClosure(r); err == nil || err.Error() != "component public mutation escaped selected closure" {
		t.Fatal("original state closure did not refuse the actual cache effect", err)
	}
	t.Log("actual source-defined cache effect with preauthored empty directory: only that directory metadata changed and one exact assessment file added; old closure refuses")
}
