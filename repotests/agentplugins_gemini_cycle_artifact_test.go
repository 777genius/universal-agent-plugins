//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func (c *geminiProjectCycle) finish(t *testing.T, heldSource any) {
	t.Helper()
	candidate := c.observed
	c.observed = false
	geminiCycleClearCompletion(c.f.evidence)
	defer c.f.cancel()
	defer c.archive(t, heldSource)
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
	if t.Failed() {
		return
	}
	if !valid {
		t.Error("Gemini complete cycle final live closure refused")
		return
	}
	if err := c.admitComplete(t, heldSource); err != nil {
		geminiCycleClearCompletion(c.f.evidence)
		t.Error(err)
	}
}
func (c *geminiProjectCycle) archive(t *testing.T, heldSource any) {
	t.Helper()
	f := c.f
	defer func() {
		if t.Failed() {
			geminiCycleClearCompletion(f.evidence)
		}
	}()
	if f.ctx.Err() != nil {
		c.observed = false
		t.Error("Gemini original work expired at archive entry")
	}
	if time.Since(f.started) > 180*time.Second {
		c.observed = false
		t.Error("Gemini original total budget exceeded at archive entry")
	}
	if t.Failed() || !c.observed {
		geminiCycleClearCompletion(f.evidence)
		if c.stage == "complete-cycle" {
			c.stage = "complete-cycle-capture-review"
		}
	}
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
	if f.evidence["runtime_attempt_complete"] == true {
		decoded, err := geminiCompleteJSON(raw)
		started, _, closureErr := geminiCompleteClosure(decoded)
		if err != nil || closureErr != nil || !started.Equal(f.started) || geminiCycleCompleteFiles(f.files, heldSource) != nil {
			geminiCycleClearCompletion(f.evidence)
			t.Error("Gemini final serialized complete evidence refused")
			return
		}
	}
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
	if err := geminiCycleWriteArchive(f.output, f.files, func() error {
		if f.evidence["runtime_attempt_complete"] == true && (t.Failed() || f.ctx.Err() != nil || time.Since(f.started) > 180*time.Second) {
			return geminiCompleteRefusal()
		}
		return nil
	}); err != nil {
		t.Error("Gemini checked archive write/close/expose failed", err)
	}
}
