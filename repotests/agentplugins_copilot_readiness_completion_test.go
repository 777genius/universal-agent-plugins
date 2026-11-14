//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func copilotReadinessCapture(files map[string][]byte) bool {
	names := make([]string, 0, 20)
	names = append(names, "probe-events.jsonl", "mcp.json.installed", "uapcopilota-SKILL.md.installed", "provider-request-1.json", "provider-request-2.json")
	for _, label := range []string{"git-A", "git-B", "version", "add-A", "native-attempt"} {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			names = append(names, label+suffix)
		}
	}
	if len(files) != len(names) {
		return false
	}
	for _, name := range names {
		if _, has := files[name]; !has {
			return false
		}
	}
	return true
}

func TestCopilotReadinessLateFailureAndCaptureCannotQualify(t *testing.T) {
	for _, flags := range [][5]bool{{true, false, true, true, true}, {false, false, true, true, true}, {true, true, true, true, true}, {true, false, false, true, true}, {true, false, true, false, true}, {true, false, true, true, false}} {
		got := copilotReadinessComplete(flags[0], flags[1], flags[2], flags[3], flags[4], time.Second)
		if got != (flags == [5]bool{true, false, true, true, true}) {
			t.Fatal("late failure/completion contradiction")
		}
	}
	for _, elapsed := range []time.Duration{-1, 180 * time.Second, 181 * time.Second} {
		if copilotReadinessComplete(true, false, true, true, true, elapsed) {
			t.Fatal("original completion budget widened")
		}
	}
	f, _, _ := copilotReadinessActual(t)
	delete(f.files, "runtime-attempt.json")
	if !copilotReadinessCapture(f.files) {
		t.Fatal("actual twenty pre-summary members refused")
	}
	delete(f.files, "probe-events.jsonl")
	if copilotReadinessCapture(f.files) {
		t.Fatal("missing capture accepted")
	}
	f.files["foreign"] = nil
	if copilotReadinessCapture(f.files) {
		t.Fatal("same-count foreign member accepted")
	}
}
func TestCopilotReadinessLateOriginalAuthorityMutationRefuses(t *testing.T) {
	for _, protected := range []bool{false, true} {
		f := &copilotProjectAttempt{immutable: map[string]string{}, protected: map[string]string{}}
		path := filepath.Join(t.TempDir(), "owned")
		projectNativeRuntimeWrite(t, path, []byte("held"), 0600)
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		if protected {
			f.protected[path] = digest
		} else {
			f.immutable[path] = digest
		}
		if err := f.readinessAuthority(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("late mutation"), 0600); err != nil {
			t.Fatal(err)
		}
		if f.readinessAuthority() == nil {
			t.Fatal("late original authority drift accepted")
		}
	}
}
func (f *copilotProjectAttempt) readinessAuthority() error {
	for path, expected := range f.immutable {
		got, err := projectNativeRuntimeDigest(path)
		if err != nil || got != expected {
			return fmt.Errorf("Copilot readiness late immutable authority failure")
		}
	}
	for path, expected := range f.protected {
		got, err := f.protectedDigest(path, expected)
		if err != nil || got != expected {
			return fmt.Errorf("Copilot readiness late original authority failure")
		}
	}
	return nil
}
func copilotReadinessComplete(observed, failed, workAlive, providerClosed, captured bool, elapsed time.Duration) bool {
	return observed && !failed && workAlive && providerClosed && captured && elapsed >= 0 && elapsed < 180*time.Second
}
func (f *copilotProjectAttempt) finalizeReadiness(t *testing.T) {
	t.Helper()
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["readiness_observed"] = f.readinessObserved
	f.evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	f.evidence["vendor_authentication_qualified"], f.evidence["real_model_qualified"] = false, false
	if err := f.readinessAuthority(); err != nil {
		t.Error(err)
	}
	workAlive := f.ctx != nil && f.ctx.Err() == nil
	if f.readinessObserved && !workAlive {
		t.Error("Copilot readiness original work deadline exhausted")
	}
	closed, _ := f.evidence["provider_closed"].(bool)
	complete := copilotReadinessComplete(f.readinessObserved, t.Failed(), workAlive, closed, copilotReadinessCapture(f.files), time.Since(f.started))
	if f.readinessObserved && !complete {
		t.Error("Copilot readiness final ordinary closure/capture gate failed")
	}
	f.evidence["runtime_attempt_complete"] = complete
}
