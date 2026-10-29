//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func geminiCycleAssessmentFixture(t *testing.T) (string, *geminiCycleRecipe, geminiCycleAssessment) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "state")
	projectNativeRuntimeWrite(t, filepath.Join(root, "caller", "keep"), []byte("caller authority"), 0600)
	projectNativeRuntimeWrite(t, filepath.Join(root, "security", "lintai", "held"), []byte("inert scanner-file authority; never executed"), 0700)
	r := &geminiCycleRecipe{Scope: "user", Revision: "U-one", Digest: "sha256:" + strings.Repeat("a", 64), Authored: map[string][]byte{"plugin.json": []byte(`{"name":"inert-cache","version":"1.0.0"}`)}}
	expected, err := geminiCycleAssessmentFor(r)
	if err != nil {
		t.Fatal(err)
	}
	return root, r, expected
}
func geminiCycleAssessmentStore(t *testing.T, root string, expected geminiCycleAssessment) securityscan.FileCache {
	t.Helper()
	var assessment domain.SecurityAssessment
	if err := json.Unmarshal(expected.Body, &assessment); err != nil {
		t.Fatal(err)
	}
	cache := securityscan.FileCache{Root: filepath.Join(root, "security", "assessments")}
	if err := cache.Store(expected.Key, assessment); err != nil {
		t.Fatal(err)
	}
	return cache
}
func TestGeminiCycleAssessmentIndependentKnownFormatAndActualStoreParity(t *testing.T) {
	root, _, expected := geminiCycleAssessmentFixture(t)
	if expected.Key != "d69633cb07bad7baf6f4ca5f9d176af67365d639f9a3c1195459c69c66776a24" || len(expected.Body) != 578 || geminiCycleSHA(expected.Body) != "0aed5dd490a38b5ff361179b3d67b5080573568e3b5819122c43da44d81c5a2d" {
		t.Fatal("independently reconstructed key/whole JSON golden differs")
	}
	before := geminiCycleTempTree(t, root)
	cache := geminiCycleAssessmentStore(t, root, expected)
	loaded, ok := cache.Load(expected.Key)
	if !ok || loaded.Evidence != "" || securityscan.CacheKey(loaded.Subject, securityscan.DefaultRequirement()) != expected.Key {
		t.Fatal("actual cache load/key framing differs")
	}
	actual, err := os.ReadFile(filepath.Join(cache.Root, expected.Key+".json"))
	if err != nil || !bytes.Equal(actual, expected.Body) {
		t.Fatal("actual Store JSON differs from independent golden", err)
	}
	after := geminiCycleTempTree(t, root)
	frame, err := geminiCycleAssessmentClosure(context.Background(), before, after, nil, &expected)
	if err != nil || frame.globalUnchanged(true) != nil || len(frame.Ledger) != 1 {
		t.Fatal("exact cache-only global transition refused", err)
	}
	// A real cache read is a hit, not a same-bytes atomic replacement.
	if _, ok := cache.Load(expected.Key); !ok {
		t.Fatal("actual cache hit absent")
	}
	now := geminiCycleTempTree(t, root)
	retained, err := geminiCycleAssessmentClosure(context.Background(), after, now, frame.Ledger, &expected)
	if err != nil || retained.globalUnchanged(true) != nil {
		t.Fatal("retained cache identity refused", err)
	}
	if _, err := geminiCycleAssessmentClosure(context.Background(), now, now, retained.Ledger, nil); err != nil {
		t.Fatal("no-source stage changed a cache entry", err)
	}
}
func TestGeminiCycleAssessmentFourRetainedSourcesAndNoCallerAdoption(t *testing.T) {
	root, recipe, _ := geminiCycleAssessmentFixture(t)
	before := geminiCycleTempTree(t, root)
	ledger := map[string]geminiCycleAssessment{}
	for index, slot := range []struct{ scope, revision string }{{"user", "U-one"}, {"a", "A-one"}, {"b", "B-one"}, {"a", "A-two"}} {
		r := *recipe
		r.Scope, r.Revision, r.Digest = slot.scope, slot.revision, "sha256:"+strings.Repeat(string(rune('a'+index)), 64)
		expected, err := geminiCycleAssessmentFor(&r)
		if err != nil {
			t.Fatal(err)
		}
		geminiCycleAssessmentStore(t, root, expected)
		after := geminiCycleTempTree(t, root)
		frame, err := geminiCycleAssessmentClosure(context.Background(), before, after, ledger, &expected)
		if err != nil || frame.globalUnchanged(true) != nil || len(frame.Ledger) != index+1 {
			t.Fatal("finite retained cache ledger refused", err)
		}
		before, ledger = after, frame.Ledger
	}
	if err := os.WriteFile(filepath.Join(root, "caller", "keep"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := geminiCycleTempTree(t, root)
	frame, err := geminiCycleAssessmentClosure(context.Background(), before, changed, ledger, nil)
	if err != nil || frame.globalUnchanged(false) == nil {
		t.Fatal("cache frame hid caller mutation", err)
	}
	if len(ledger) != 4 || reflect.DeepEqual(before, changed) {
		t.Fatal("rejected global frame adopted caller state")
	}
}
func TestGeminiCycleAssessmentProjectProofRetainsActualUserAuthority(t *testing.T) {
	c := geminiCycleInert(t)
	geminiCycleStageActual(t, c, "user")
	geminiCycleStageActual(t, c, "a")
	user := c.recipes["user"]
	state, err := os.ReadFile(filepath.Join(user.StateRoot, "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := geminiCycleAssessmentFor(c.recipes["a"])
	if err != nil {
		t.Fatal(err)
	}
	geminiCycleAssessmentStore(t, user.StateRoot, expected)
	c.proofWithAssessment(t, c.recipes["a"], "repeat-add-a", &expected)
	if len(c.assessments) != 1 {
		t.Fatal("selected project did not retain exact global cache proof")
	}
	after, err := os.ReadFile(filepath.Join(user.StateRoot, "state-v2.json"))
	if err != nil || !bytes.Equal(state, after) {
		t.Fatal("project cache proof changed user receipt", err)
	}
	c.check(t)
}
