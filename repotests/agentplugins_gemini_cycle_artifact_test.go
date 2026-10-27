//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func (c *geminiProjectCycle) finish(t *testing.T) {
	t.Helper()
	candidate := c.observed
	c.observed = false
	defer c.f.cancel()
	defer c.archive(t)
	closed := false
	if c.owner != nil {
		closed = c.owner.close(t)
	}
	if !closed {
		t.Error("Gemini cycle provider did not ordinarily close all finite stages")
	}
	if time.Since(c.f.started) > 180*time.Second {
		t.Error("Gemini cycle original total budget")
	}
	c.check(t)
	valid := candidate && closed && !c.openEvents && c.stage == "complete-cycle-capture-review" && time.Since(c.f.started) <= 180*time.Second && reflect.DeepEqual(c.public, geminiCyclePublic) && reflect.DeepEqual(c.listings, geminiCycleListings) && reflect.DeepEqual(c.natives, geminiCycleNative)
	for _, body := range c.f.files {
		if geminiCyclePrivateCredential(body) {
			t.Error("Gemini cycle final semantic privacy/resource refusal")
			valid = false
		}
	}
	c.observed = valid
}
func (c *geminiProjectCycle) archive(t *testing.T) {
	t.Helper()
	f := c.f
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["vendor_authentication_qualified"] = false
	f.evidence["real_model_qualified"] = false
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["cycle_observed_complete"] = c.observed
	f.evidence["cycle_public_stages"] = append([]string{}, c.public...)
	f.evidence["cycle_listing_stages"] = append([]string{}, c.listings...)
	f.evidence["cycle_native_stages"] = append([]string{}, c.natives...)
	f.evidence["attempt_stage"] = c.stage
	f.evidence["test_failed"] = t.Failed()
	f.evidence["elapsed_ms"] = time.Since(f.started).Milliseconds()
	raw, err := json.MarshalIndent(f.evidence, "", "  ")
	if err != nil {
		t.Error("Gemini cycle summary serialization")
		return
	}
	raw = append(raw, '\n')
	if len(raw) > 1<<20 || geminiCyclePrivateCredential(raw) {
		t.Error("Gemini cycle closed summary privacy/bound")
		return
	}
	f.files["runtime-attempt.json"] = raw
	total := 0
	names := make([]string, 0, len(f.files))
	for name, body := range f.files {
		if filepath.Base(name) != name || len(body) > 2<<20 || geminiCyclePrivateCredential(body) {
			t.Error("Gemini cycle member privacy/bound")
			return
		}
		total += len(body)
		names = append(names, name)
	}
	if len(names) > 57 || total > 16<<20 || geminiProjectSecretFree(f.files, []string{f.secret}) != nil {
		t.Error("Gemini cycle complete artifact closure/bound")
		return
	}
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "research.zip")); err != nil {
		t.Error("Gemini cycle artifact parent")
		return
	}
	file, err := os.OpenFile(filepath.Join(f.output, "research.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Error("Gemini cycle exclusive artifact")
		return
	}
	archive := zip.NewWriter(file)
	sort.Strings(names)
	for _, name := range names {
		entry, e := archive.Create(name)
		if e != nil {
			t.Error("Gemini cycle archive member")
			break
		}
		if _, e = entry.Write(f.files[name]); e != nil {
			t.Error("Gemini cycle archive body")
			break
		}
	}
	if err := archive.Close(); err != nil {
		t.Error("Gemini cycle archive close")
	}
	if err := file.Close(); err != nil {
		t.Error("Gemini cycle file close")
	}
}
