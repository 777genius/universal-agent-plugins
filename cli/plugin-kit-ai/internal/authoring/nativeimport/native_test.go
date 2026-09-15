package nativeimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func nativeCode(t *testing.T, err error) string {
	t.Helper()
	var target *Error
	if !errors.As(err, &target) {
		t.Fatalf("expected typed error, got %v", err)
	}
	return target.Code
}

func writeSource(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeBuildDeterministicSkipsAndRedacts(t *testing.T) {
	secret := "literal-super-secret-token"
	source := writeSource(t, `{"other":"`+secret+`","mcpServers":{"safe":{"command":"node","args":["relative.js","--quiet"]},"env":{"command":"node","env":{"TOKEN":"`+secret+`"}},"url":{"command":"node","url":"https://example.test/?token=`+secret+`"},"unix":{"command":"node","args":["/private/tool"]},"windows":{"command":"node","args":["C:\\\\private\\\\tool"]},"mixed":{"command":"node","type":"stdio"}}}`)
	first, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package.")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package.")
	if err != nil {
		t.Fatal(err)
	}
	if first.SafeServers != 1 || len(first.SkippedServers) != 5 || len(first.UnsupportedTopLevel) != 1 || first.SourceSHA256 != second.SourceSHA256 || !reflect.DeepEqual(first.Package.Files(), second.Package.Files()) {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	public, _ := json.Marshal(first)
	if bytes.Contains(public, []byte(secret)) || bytes.Contains(public, []byte("TOKEN")) || bytes.Contains(public, []byte("env")) {
		t.Fatalf("secret disclosure: %s", public)
	}
	files := first.Package.Files()
	var mcp []byte
	for _, file := range files {
		if file.Path == "mcp.json" {
			mcp = file.Bytes
		}
	}
	if bytes.Contains(mcp, []byte(secret)) || !bytes.Contains(mcp, []byte(`"safe"`)) || bytes.Contains(mcp, []byte(`"env"`)) {
		t.Fatalf("unsafe output: %s", mcp)
	}
}

func TestClaudeRejectsMalformedDuplicateSymlinkAndChangedSource(t *testing.T) {
	for _, body := range []string{`// jsonc\n{"mcpServers":{}}`, `{"mcpServers":{"x":{"command":"node","args":{"bad":true}}},"mcpServers":{}}`, `[]`} {
		source := writeSource(t, body)
		if _, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package."); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	t.Run("symlink", func(t *testing.T) {
		target := writeSource(t, `{"mcpServers":{}}`)
		link := filepath.Join(t.TempDir(), "link.json")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := Build(context.Background(), link, "claude", "imported-plugin", "Imported package."); nativeCode(t, err) != "source_unavailable" {
			t.Fatal(err)
		}
	})
	t.Run("changed", func(t *testing.T) {
		source := writeSource(t, `{"mcpServers":{"safe":{"command":"node"}}}`)
		plan, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package.")
		if err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(t.TempDir(), "out")
		if err := os.WriteFile(source, []byte(`{"mcpServers":{"other":{"command":"node"}}}`), 0600); err != nil {
			t.Fatal(err)
		}
		result, err := Apply(context.Background(), plan, output, func(context.Context, string, *os.Root) error { return nil })
		if result.Committed || nativeCode(t, err) != "source_changed" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatalf("output created: %v", err)
		}
	})
}

func TestOutputAndNoSafeWrite(t *testing.T) {
	source := writeSource(t, `{"mcpServers":{"unsafe":{"command":"/absolute/tool"}}}`)
	plan, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package.")
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	output := filepath.Join(parent, "out")
	if err := ValidateOutput(plan, output); err != nil {
		t.Fatal(err)
	}
	if result, err := Apply(context.Background(), plan, output, func(context.Context, string, *os.Root) error { return nil }); result.Committed || nativeCode(t, err) != "no_safe_servers" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	if code := nativeCode(t, ValidateOutput(plan, output)); code != "output_exists" {
		t.Fatal(code)
	}
	if code := nativeCode(t, ValidateOutput(plan, source)); code != "source_output_overlap" {
		t.Fatal(code)
	}
}

func TestSafeWritePublishesExactTreeOnce(t *testing.T) {
	source := writeSource(t, `{"mcpServers":{"safe":{"command":"node","args":[]}}}`)
	plan, err := Build(context.Background(), source, "claude", "imported-plugin", "Imported package.")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "out")
	validated := false
	result, err := Apply(context.Background(), plan, output, func(_ context.Context, stage string, _ *os.Root) error {
		validated = true
		return filepath.WalkDir(stage, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type()&os.ModeSymlink != 0 {
				return errors.New("symlink")
			}
			return nil
		})
	})
	if err != nil || !result.Committed || !validated {
		t.Fatalf("result=%+v validated=%t err=%v", result, validated, err)
	}
	for _, file := range plan.Package.Files() {
		got, readErr := os.ReadFile(filepath.Join(output, file.Path))
		if readErr != nil || !bytes.Equal(got, file.Bytes) {
			t.Fatalf("%s: %v", file.Path, readErr)
		}
	}
	before, _ := os.ReadFile(source)
	result, err = Apply(context.Background(), plan, output, func(context.Context, string, *os.Root) error { return nil })
	after, _ := os.ReadFile(source)
	if err == nil || result.Committed || !bytes.Equal(before, after) || !strings.Contains(err.Error(), "destination already exists") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
