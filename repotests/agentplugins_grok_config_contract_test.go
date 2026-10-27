//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func grokConfigFixture(t *testing.T) *projectNativeCLIAttempt {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "grok"}, ctx: ctx, cancel: cancel, started: time.Now(),
		base: &nativeFixture{Root: root, Home: filepath.Join(root, "original-home")}, protected: map[string]string{},
		files: map[string][]byte{"native-attempt.stdout.log": {}, "native-attempt.stderr.log": {}, "native-attempt.command.json": []byte(`{"inert":true}`)},
		used:  map[string]bool{"native-attempt": true}, evidence: map[string]any{}}
	t.Cleanup(func() {
		if f.grok != nil {
			f.grok.stop()
		}
		cancel()
	})
	f.configureGrokProvider(t)
	return f
}

// This deliberately changes only the file authored in this inert test fixture.
// The diagnostic itself never writes, repairs or adopts the changed file.
func grokConfigRewrite(t *testing.T, path string, body []byte) {
	t.Helper()
	if _, err := projectNativeRuntimeRead(path, 64<<10); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func grokConfigObservedFile(t *testing.T, f *projectNativeCLIAttempt, body []byte) grokConfigObservation {
	t.Helper()
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	grokConfigRewrite(t, path, body)
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	want := f.protected[path]
	f.observeGrokConfigMutation(path, want, nativeSHA(t, path))
	observation, ok := f.evidence["grok_config_mutation"].(grokConfigObservation)
	if !ok {
		t.Fatal("eligible held-file observation missing")
	}
	raw, err := json.Marshal(observation)
	if err != nil || len(raw) > 2048 || bytes.Contains(raw, []byte("PRIVATE-")) || bytes.Contains(raw, []byte("uap-grok-private-")) {
		t.Fatal("finite diagnostic bound or private-value projection differs")
	}
	after, statErr := os.Lstat(path)
	actual, readErr := os.ReadFile(path)
	if statErr != nil || readErr != nil || !os.SameFile(before, after) || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) ||
		!bytes.Equal(body, actual) || f.protected[path] != want || !bytes.Equal(f.grok.authoredConfig, f.grok.configuration()) {
		t.Fatal("observation changed original file identity, recipe or protected digest")
	}
	return observation
}

func grokConfigRequireKnown(t *testing.T, comparison *grokConfigComparison, unequal string) {
	t.Helper()
	if comparison == nil || len(comparison.Known) != 8 {
		t.Fatal("eight authored scalar facts missing")
	}
	for path, fact := range comparison.Known {
		if fact.Equal != (path != unequal) {
			t.Fatal("authored scalar equality differs", path)
		}
	}
}

func TestGrokConfigDiagnosticReportsOnlyIndependentlyEncodedAddedTypes(t *testing.T) {
	for _, test := range []struct{ toml, kind, canonical string }{
		{`"PRIVATE-BODY"`, "string", `"PRIVATE-BODY"`}, {"false", "boolean", "false"}, {"1", "integer", "1"},
		{"1.25", "float", "1.25"}, {`["PRIVATE-BODY", 1]`, "array", `["PRIVATE-BODY",1]`},
		{"{ nested = true }", "table", `{"nested":true}`},
	} {
		t.Run(test.kind, func(t *testing.T) {
			f := grokConfigFixture(t)
			body := append([]byte("PRIVATE-KEY = "+test.toml+"\n"), f.grok.authoredConfig...)
			observation := grokConfigObservedFile(t, f, body)
			comparison := observation.Comparison
			grokConfigRequireKnown(t, comparison, "")
			if observation.Status != "captured" || comparison.SemanticEqual || comparison.BeforeFields != 2 || comparison.AfterFields != 3 || comparison.ChangedCount != 1 || comparison.OmittedCount != 0 || len(comparison.Differences) != 1 {
				t.Fatal("one added top-level member facts differ")
			}
			want := grokConfigDifference{KeySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("PRIVATE-KEY"))), BeforeType: "absent", AfterType: test.kind,
				AfterSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(test.canonical)))}
			if comparison.Differences[0] != want {
				t.Fatal("literal semantic byte hash/type projection differs")
			}
		})
	}
}

func TestGrokConfigDiagnosticPreservesKnownSafetyAndIntegerFloatDistinctions(t *testing.T) {
	for _, test := range []struct{ before, after, path, kind string }{
		{"supports_backend_search = false", "supports_backend_search = true", "model.uap-grok-fixture.supports_backend_search", "boolean"},
		{"supports_reasoning_effort = false", "supports_reasoning_effort = true", "model.uap-grok-fixture.supports_reasoning_effort", "boolean"},
		{"max_retries = 0", "max_retries = 0.0", "model.uap-grok-fixture.max_retries", "float"},
		{"max_retries = 0\n", "", "model.uap-grok-fixture.max_retries", "absent"},
	} {
		f := grokConfigFixture(t)
		body := bytes.Replace(f.grok.authoredConfig, []byte(test.before), []byte(test.after), 1)
		observation := grokConfigObservedFile(t, f, body)
		grokConfigRequireKnown(t, observation.Comparison, test.path)
		if observation.Status != "captured" || observation.Comparison.SemanticEqual || observation.Comparison.ChangedCount != 1 || observation.Comparison.Known[test.path].Type != test.kind {
			t.Fatal("changed safety value or exact scalar type was adopted")
		}
	}
}

func TestGrokConfigDiagnosticFormatAndRemovalArePresenceAware(t *testing.T) {
	for _, mode := range []string{"format", "removed", "type-changed"} {
		f := grokConfigFixture(t)
		body := append([]byte("# PRIVATE-COMMENT\n"), f.grok.authoredConfig...)
		if mode != "format" {
			body = bytes.Replace(f.grok.authoredConfig, []byte("[models]\ndefault = \"uap-grok-fixture\"\n"), nil, 1)
			if mode == "type-changed" {
				body = append([]byte("models = \"PRIVATE-BODY\"\n"), body...)
			}
		}
		observation := grokConfigObservedFile(t, f, body)
		comparison := observation.Comparison
		if mode == "format" {
			grokConfigRequireKnown(t, comparison, "")
			if !comparison.SemanticEqual || comparison.ChangedCount != 0 || len(comparison.Differences) != 0 {
				t.Fatal("format-only rewrite fabricated a semantic change")
			}
			continue
		}
		grokConfigRequireKnown(t, comparison, "models.default")
		if comparison.SemanticEqual || comparison.ChangedCount != 1 || len(comparison.Differences) != 1 || comparison.Known["models.default"].Present {
			t.Fatal("removed scalar was confused with an equal existing value")
		}
		want := grokConfigDifference{KeySHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("models"))), BeforeType: "table", AfterType: "absent",
			BeforeSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(`{"default":"uap-grok-fixture"}`)))}
		if mode == "type-changed" {
			want.AfterType, want.AfterSHA256 = "string", fmt.Sprintf("%x", sha256.Sum256([]byte(`"PRIVATE-BODY"`)))
		}
		if comparison.Differences[0] != want {
			t.Fatal("presence-aware removed/changed semantic hash differs")
		}
	}
}

func TestGrokConfigDiagnosticSelectionAndBoundAreDeterministic(t *testing.T) {
	var prior *grokConfigComparison
	keys := []string{"PRIVATE-0", "PRIVATE-1", "PRIVATE-2", "PRIVATE-3", "PRIVATE-4", "PRIVATE-5", "PRIVATE-6"}
	want := make([]string, 0, len(keys))
	for _, key := range keys {
		want = append(want, fmt.Sprintf("%x", sha256.Sum256([]byte(key))))
	}
	sort.Strings(want)
	for _, reverse := range []bool{false, true} {
		f := grokConfigFixture(t)
		prefix := make([]string, 0, len(keys))
		for index := range keys {
			if reverse {
				index = len(keys) - index - 1
			}
			prefix = append(prefix, keys[index]+" = \"PRIVATE-BODY\"")
		}
		observation := grokConfigObservedFile(t, f, append([]byte(strings.Join(prefix, "\n")+"\n"), f.grok.authoredConfig...))
		comparison := observation.Comparison
		if observation.Status != "entry-limit" || comparison.ChangedCount != 7 || comparison.OmittedCount != 4 || len(comparison.Differences) != 3 {
			t.Fatal("three-detail bound or exact omitted count differs")
		}
		for index, difference := range comparison.Differences {
			if difference.KeySHA256 != want[index] || difference.BeforeType != "absent" || difference.AfterType != "string" || difference.AfterSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(`"PRIVATE-BODY"`))) {
				t.Fatal("selected deterministic semantic member differs")
			}
		}
		if prior != nil && !reflect.DeepEqual(prior, comparison) {
			t.Fatal("authored key order changed the bounded comparison")
		}
		prior = comparison
	}
}
