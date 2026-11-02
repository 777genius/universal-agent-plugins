//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGrokConfigDiagnosticRefusesSyntaxTypesPrivateValuesAndBoundsWithoutPartialFacts(t *testing.T) {
	for _, test := range []struct{ prefix, status string }{
		{"x = 1\nx = 2\n", "toml-syntax"}, {"PRIVATE-MALFORMED = [\n", "toml-syntax"}, {"\xff\n", "toml-syntax"},
		{"PRIVATE-KEY = 1979-05-27T07:32:00Z\n", "toml-value"}, {"PRIVATE-KEY = 07:32:00\n", "toml-value"},
		{"PRIVATE-KEY = nan\n", "toml-value"}, {"PRIVATE-KEY = inf\n", "toml-value"},
		{strings.Repeat("a.", 34) + "key = 1\n", "toml-value"}, {strings.Repeat("k", 4097) + " = 1\n", "toml-value"},
		{"PRIVATE-KEY = [" + strings.Repeat("0,", 16384) + "0]\n", "toml-value"},
		{"PRIVATE-KEY = \"" + strings.Repeat("<", 12000) + "\"\n", "semantic-bound"},
		{"PRIVATE-KEY = \"\\u0075ap-grok-private-inert\"\n", "private-content"},
		{"\"\\u0075ap-grok-private-inert\" = true\n", "private-content"},
		{"PRIVATE-KEY = '{\"content\":\"\\u0075ap-grok-private-inert\"}'\n", "private-content"},
	} {
		f := grokConfigFixture(t)
		observation := grokConfigObservedFile(t, f, append([]byte(test.prefix), f.grok.authoredConfig...))
		if observation.Status != test.status || observation.Comparison != nil {
			t.Fatalf("unsafe TOML emitted partial metadata: got %s, want %s", observation.Status, test.status)
		}
	}
}

func TestGrokConfigDiagnosticReadsOnceWithinOriginalWorkTimeAfterEarlyCancellation(t *testing.T) {
	f := grokConfigFixture(t)
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	body := append([]byte("# PRIVATE-COMMENT\n"), f.grok.authoredConfig...)
	grokConfigRewrite(t, path, body)
	want, got := f.protected[path], nativeSHA(t, path)
	originalContext, originalStarted := f.ctx, f.started
	f.cancel()
	reads := 0
	read := func(name string, limit int64) ([]byte, error) {
		reads++
		if name != path || limit != 64<<10 {
			t.Fatal("diagnostic changed the fixed held-file boundary")
		}
		return grokConfigHeldRead(name, limit)
	}
	observation := f.grokReadConfigMutation(path, want, got, read)
	if observation.Status != "captured" || observation.Comparison == nil || !observation.Comparison.SemanticEqual || reads != 1 || f.ctx != originalContext || f.started != originalStarted || f.ctx.Err() == nil {
		t.Fatal("early cancellation changed the original context/budget or skipped bounded observation")
	}
	f.started = time.Now().Add(-151 * time.Second)
	reads = 0
	observation = f.grokReadConfigMutation(path, want, got, read)
	if observation.Status != "work-deadline" || reads != 0 || observation.Bytes != nil || observation.Comparison != nil {
		t.Fatal("expired original work boundary performed a read or emitted partial facts")
	}
	f.observeGrokConfigMutation(path, want, got)
	first := f.evidence["grok_config_mutation"]
	f.started = originalStarted
	f.observeGrokConfigMutation(path, want, got)
	if !reflect.DeepEqual(first, f.evidence["grok_config_mutation"]) {
		t.Fatal("a recorded deadline refusal was retried with fresh time")
	}
}

func TestGrokConfigDiagnosticDoesNotProjectChangedReadOrRecipeAndRetainsNoErrors(t *testing.T) {
	for _, mode := range []string{"read-error", "digest", "recipe", "expired-after-read"} {
		f := grokConfigFixture(t)
		path := filepath.Join(f.base.Home, ".grok", "config.toml")
		body := append([]byte("# changed\n"), f.grok.authoredConfig...)
		grokConfigRewrite(t, path, body)
		want, got := f.protected[path], nativeSHA(t, path)
		if mode == "recipe" {
			f.grok.authoredConfig = append([]byte("# changed expectation\n"), f.grok.authoredConfig...)
		}
		read := func(string, int64) ([]byte, error) {
			switch mode {
			case "read-error":
				return nil, fmt.Errorf("PRIVATE-ERROR /PRIVATE-PATH")
			case "digest":
				return []byte("different = true\n"), nil
			case "expired-after-read":
				f.started = time.Now().Add(-151 * time.Second)
			}
			return bytes.Clone(body), nil
		}
		observation := f.grokReadConfigMutation(path, want, got, read)
		wantStatus := map[string]string{"read-error": "read-refused", "digest": "observed-digest-mismatch", "recipe": "recipe-mismatch", "expired-after-read": "work-deadline"}[mode]
		raw, err := json.Marshal(observation)
		if err != nil || bytes.Contains(raw, []byte("PRIVATE-")) || observation.Status != wantStatus || observation.Comparison != nil || f.protected[path] != want {
			t.Fatal("failed held read/recipe produced private or partial facts")
		}
	}
}

func TestGrokConfigDiagnosticRequiresExactClientPathAndRecordedNativeTriple(t *testing.T) {
	for _, mode := range []string{"wrong-client", "wrong-path", "not-used", "stdout", "stderr", "command", "same", "unprotected", "private-hash"} {
		f := grokConfigFixture(t)
		path := filepath.Join(f.base.Home, ".grok", "config.toml")
		want, got := f.protected[path], strings.Repeat("a", 64)
		switch mode {
		case "wrong-client":
			f.spec.client = "qoder"
		case "wrong-path":
			path = filepath.Join(f.base.Home, "PRIVATE-PATH")
		case "not-used":
			delete(f.used, "native-attempt")
		case "stdout", "stderr":
			delete(f.files, "native-attempt."+mode+".log")
		case "command":
			delete(f.files, "native-attempt.command.json")
		case "same":
			got = want
		case "unprotected":
			delete(f.protected, path)
		case "private-hash":
			got = "PRIVATE-HASH"
		}
		f.observeGrokConfigMutation(path, want, got)
		if _, exists := f.evidence["grok_config_mutation"]; exists {
			t.Fatal("unrelated client/path/attempt entered diagnostic")
		}
	}
}

func TestGrokConfigDiagnosticRefusesAliasesLinksAndOversizedActualFiles(t *testing.T) {
	for _, mode := range []string{"symlink", "hardlink", "parent-alias", "oversized"} {
		f := grokConfigFixture(t)
		path := filepath.Join(f.base.Home, ".grok", "config.toml")
		body := append([]byte("# changed\n"), f.grok.authoredConfig...)
		if mode == "oversized" {
			body = append([]byte("#"+strings.Repeat("x", 64<<10)+"\n"), body...)
		}
		grokConfigRewrite(t, path, body)
		got := nativeSHA(t, path)
		switch mode {
		case "symlink", "hardlink":
			target := filepath.Join(f.base.Home, "PRIVATE-TARGET")
			if err := os.Rename(path, target); err != nil {
				t.Fatal(err)
			}
			link := os.Symlink
			if mode == "hardlink" {
				link = os.Link
			}
			if err := link(target, path); err != nil {
				t.Fatal(err)
			}
		case "parent-alias":
			directory, target := filepath.Dir(path), filepath.Join(f.base.Home, "PRIVATE-DIRECTORY")
			if err := os.Rename(directory, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, directory); err != nil {
				t.Fatal(err)
			}
		}
		observation := f.grokReadConfigMutation(path, f.protected[path], got, grokConfigHeldRead)
		if observation.Status != "read-refused" || observation.Comparison != nil {
			t.Fatal("unsafe actual file was interpreted or given semantic hashes")
		}
	}
}

func TestGrokConfigHeldReadRefusesFullMetadataAndParentChangesDuringRead(t *testing.T) {
	for _, mode := range []string{"mode-restored", "link-restored", "parent-replaced"} {
		f := grokConfigFixture(t)
		path := filepath.Join(f.base.Home, ".grok", "config.toml")
		read := func(name string, limit int64) ([]byte, error) {
			body, err := projectNativeRuntimeRead(name, limit)
			if err != nil {
				return nil, err
			}
			switch mode {
			case "mode-restored":
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			case "link-restored":
				link := filepath.Join(f.base.Home, "PRIVATE-LINK")
				if err := os.Link(path, link); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			case "parent-replaced":
				directory := filepath.Dir(path)
				previous := filepath.Join(f.base.Home, "PRIVATE-PREVIOUS")
				if err := os.Rename(directory, previous); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(previous, "config.toml"), path); err != nil {
					t.Fatal(err)
				}
			}
			return body, nil
		}
		if body, err := grokConfigStableRead(path, 64<<10, read); err == nil || body != nil {
			t.Fatal("restored mode/link count or replaced parent escaped full metadata refusal")
		}
	}
}

func TestGrokConfigDiagnosticCannotMakeFormatOnlyRewritePassProtectedGuard(t *testing.T) {
	grokConfigRequireProtectedFatal(t, "format")
}

func TestGrokConfigMigrationFlagChangesCannotPassProtectedGuard(t *testing.T) {
	for _, mode := range []string{"removed", "false"} {
		t.Run(mode, func(t *testing.T) { grokConfigRequireProtectedFatal(t, mode) })
	}
}

func grokConfigRequireProtectedFatal(t *testing.T, mode string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result := captureProjectNative(context.Background(), executable, t.TempDir(), []string{"UAP_GROK_CONFIG_FATAL_CHILD=" + mode},
		[]string{"-test.run=^TestGrokConfigProtectedFatalInertChild$"}, 4096, 5*time.Second)
	if result.Exit != 1 || result.Deadline || result.StdoutLimit || result.StderrLimit || len(result.Stderr) != 0 ||
		!bytes.Contains(result.Stdout, []byte("CONFIG_FATAL_DIAGNOSTIC_VERIFIED")) || !bytes.Contains(result.Stdout, []byte("native CLI immutable/protected identity changed")) {
		t.Fatalf("original fatal guard was not proved: exit=%d, error=%s", result.Exit, result.ErrorKind)
	}
}

func TestGrokConfigProtectedFatalInertChild(t *testing.T) {
	mode := os.Getenv("UAP_GROK_CONFIG_FATAL_CHILD")
	if mode == "" {
		t.Skip("inert protected-guard child only")
	}
	f := grokConfigFixture(t)
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	body := f.grok.authoredConfig
	switch mode {
	case "format":
		body = append([]byte("# PRIVATE-COMMENT\n"), body...)
	case "removed":
		body = bytes.Replace(body, []byte("\n[marketplace]\ndefault_skills_installs_purged = true\n"), nil, 1)
	case "false":
		body = bytes.Replace(body, []byte("default_skills_installs_purged = true"), []byte("default_skills_installs_purged = false"), 1)
	default:
		t.Fatal("unknown inert guard mutation")
	}
	grokConfigRewrite(t, path, body)
	defer func() {
		observation, ok := f.evidence["grok_config_mutation"].(grokConfigObservation)
		if !ok || observation.Status != "captured" || observation.Comparison == nil ||
			observation.Comparison.SemanticEqual != (mode == "format") || f.protected[path] != grokConfigSHA(f.grok.authoredConfig) {
			t.Error("CONFIG_DIAGNOSTIC_OR_AUTHORITY_DIFFERS")
			return
		}
		if _, err := fmt.Fprint(os.Stdout, "CONFIG_FATAL_DIAGNOSTIC_VERIFIED\n"); err != nil {
			t.Error("inert verification output failed")
		}
	}()
	f.check(t)
	t.Fatal("changed config unexpectedly passed")
}
