package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

func TestPhase8SurfaceNormalizeAndClaudeImport(t *testing.T) {
	app := publicApp(t)
	app.JSONMaintenance = true

	caps, code, raw := publicRun(t, app, []string{"capabilities", "--format=json"}, true)
	if code != 0 || !slices.Contains(caps.Data.Surface, "author.normalize") || !slices.Contains(caps.Data.Surface, "author.import.native") {
		t.Fatalf("Phase 8 surface: %d %s", code, raw)
	}

	root := t.TempDir()
	plugin := `{"version":"0.1.0","extensions":{"com.example":{"z":1e+03,"a":[-0.00,9007199254740993123456789]}},"name":"phase8-plugin","description":"Phase 8 fixture.","$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"}`
	write(t, root, "plugin.json", plugin)
	before, _ := os.ReadFile(filepath.Join(root, "plugin.json"))
	planned, code, body := publicRun(t, app, []string{"normalize", root, "--document=plugin.json", "--format=json"}, true)
	if code != 0 || planned.Data.JSONDocument == nil || !planned.Data.JSONDocument.Changed || planned.Data.Committed || planned.Data.Mode != "read" || !bytes.Equal(before, mustRead(t, filepath.Join(root, "plugin.json"))) {
		t.Fatalf("normalize plan: %d %s", code, body)
	}
	written, code, body := publicRun(t, app, []string{"normalize", root, "--document=plugin.json", "--write", "--format=json"}, true)
	if code != 0 || !written.Data.Committed || written.Data.Mode != "local_mutation" || !reflect.DeepEqual(written.Data.Paths, []string{"plugin.json"}) {
		t.Fatalf("normalize write: %d %s", code, body)
	}
	normalized := mustRead(t, filepath.Join(root, "plugin.json"))
	if !bytes.Contains(normalized, []byte("9007199254740993123456789")) || !bytes.Contains(normalized, []byte("1e+03")) || !bytes.Contains(normalized, []byte("-0.00")) || normalized[len(normalized)-1] != '\n' {
		t.Fatalf("number/value drift: %s", normalized)
	}
	old := mustStat(t, filepath.Join(root, "plugin.json")).ModTime()
	noop, code, body := publicRun(t, app, []string{"normalize", root, "--document=mcp.json", "--write", "--format=json"}, true)
	if code == 0 || noop.Data.Error == nil || noop.Data.Error.Code != "document_missing" || noop.Data.Committed {
		t.Fatalf("missing selected document: %d %s", code, body)
	}
	noop, code, body = publicRun(t, app, []string{"normalize", root, "--document=plugin.json", "--write", "--format=json"}, true)
	if code != 0 || noop.Data.JSONDocument == nil || noop.Data.JSONDocument.Changed || noop.Data.Committed || !mustStat(t, filepath.Join(root, "plugin.json")).ModTime().Equal(old) {
		t.Fatalf("normalize noop: %d %s", code, body)
	}

	secret := "literal-super-secret-token"
	source := filepath.Join(t.TempDir(), "claude.json")
	sourceBody := []byte(`{"unsupported":"` + secret + `","mcpServers":{"safe":{"command":"node","args":["relative.js"]},"secret":{"command":"node","env":{"TOKEN":"` + secret + `"}},"absolute":{"command":"node","args":["/private/tool"]}}}`)
	if err := os.WriteFile(source, sourceBody, 0600); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	outputs := []string{filepath.Join(parent, "one"), filepath.Join(parent, "two")}
	var digests []string
	for _, output := range outputs {
		args := []string{"import", "native", source, "--from=claude", "--output=" + output, "--name=imported-plugin", "--description=Imported package.", "--format=json"}
		plan, exit, raw := publicRun(t, app, args, true)
		if exit != 0 || plan.Data.NativeImport == nil || plan.Data.NativeImport.SafeServers != 1 || len(plan.Data.NativeImport.SkippedServers) != 2 || len(plan.Data.NativeImport.UnsupportedTopLevel) != 1 || plan.Data.Committed || bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("import plan: %d %s", exit, raw)
		}
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatalf("plan created output: %v", err)
		}
		created, exit, raw := publicRun(t, app, append(args[:len(args)-1], "--write", "--format=json"), true)
		if exit != 0 || !created.Data.Committed || created.Data.Readiness.Status != report.Pass || created.Data.Identity.TreeDigest == "" || bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("import write: %d %s", exit, raw)
		}
		digests = append(digests, created.Data.Identity.TreeDigest)
		if bytes.Contains(treeBytes(t, output), []byte(secret)) {
			t.Fatal("secret copied")
		}
	}
	if digests[0] != digests[1] || !bytes.Equal(sourceBody, mustRead(t, source)) {
		t.Fatalf("nondeterministic/source changed: %v", digests)
	}
	beforeTree := treeBytes(t, outputs[0])
	_, exit, raw := publicRun(t, app, []string{"import", "native", source, "--from=claude", "--output=" + outputs[0], "--name=imported-plugin", "--description=Imported package.", "--write", "--format=json"}, true)
	if exit == 0 || !bytes.Equal(beforeTree, treeBytes(t, outputs[0])) || bytes.Contains(raw, []byte(secret)) {
		t.Fatalf("existing output changed: %d %s", exit, raw)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	i, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func treeBytes(t *testing.T, root string) []byte {
	t.Helper()
	var out bytes.Buffer
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out.WriteString(filepath.ToSlash(rel))
		out.WriteByte(0)
		out.Write(body)
		out.WriteByte(0)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
