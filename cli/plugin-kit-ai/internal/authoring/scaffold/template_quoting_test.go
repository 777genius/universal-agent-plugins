package scaffold

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTemplateQuotingRejectsInvalidIdentities(t *testing.T) {
	for _, lane := range lanes {
		t.Run(lane, func(t *testing.T) {
			for _, name := range []string{"a'b", `a"b`, `a\b`, "a\nb", "a\n", "a\r", "a\u2028", "a\u2029", "a`b", "a${b}", "a');throw new Error('injected');//"} {
				o := fixtureOptions(lane)
				o.Name = name
				p, err := BuildPlan(o)
				var identity *IdentityError
				if !errors.As(err, &identity) || len(p.Files()) != 0 {
					t.Errorf("name %q: expected identity rejection and empty plan; got %v, %d files", name, err, len(p.Files()))
				}
			}
		})
	}
}

func TestTemplateQuotingStringLiterals(t *testing.T) {
	// Exercise serialization directly: these strings need to remain data even if
	// identity validation changes. This does not permit them through BuildPlan.
	for _, name := range []string{"audit-plugin", "plugin.with.dots", "a'b", `a"b`, `a\b`, "a\r\n\tb", "a\x00b", "a\u2028\u2029b", "日本語<&>", "a` ${process.exit()} b", "a');throw new Error('injected');//"} {
		t.Run(name, func(t *testing.T) {
			source := string(nodeServerSource(name))
			for _, field := range []struct{ prefix, suffix, want string }{
				{"new McpServer({ name: ", ", version:", name},
				{"content: [{ type: 'text', text: ", " }],", "Hello from " + name + "!"},
			} {
				_, tail, ok := strings.Cut(source, field.prefix)
				if !ok {
					t.Fatalf("missing field %q", field.prefix)
				}
				literal, _, ok := strings.Cut(tail, field.suffix)
				if !ok {
					t.Fatalf("missing field suffix %q", field.suffix)
				}
				var got string
				if err := json.Unmarshal([]byte(literal), &got); err != nil || got != field.want {
					t.Errorf("literal %q: decoded %q, want %q (error %v)", literal, got, field.want, err)
				}
			}
			checkTemplateJavaScriptSyntax(t, []byte(source))
		})
	}
}

func TestTemplateQuotingGeneratedStdio(t *testing.T) {
	for _, lane := range []string{"mcp-stdio", "hybrid-stdio"} {
		t.Run(lane, func(t *testing.T) {
			for _, name := range []string{"audit-plugin", "plugin.with.dots", "a", strings.Repeat("a", 64)} {
				o := fixtureOptions(lane)
				o.Name = name
				if lane == "hybrid-stdio" {
					o.SkillName = "audit-skill"
				}
				p, err := BuildPlan(o)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, f := range p.Files() {
					if f.Path == "src/server.mjs" {
						found = true
						if string(f.Bytes) != string(nodeServerSource(name)) {
							t.Fatal("plan did not use encoded source")
						}
						checkTemplateJavaScriptSyntax(t, f.Bytes)
					}
				}
				if !found {
					t.Fatal("missing server source")
				}
			}
		})
	}
}

func checkTemplateJavaScriptSyntax(t *testing.T, source []byte) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Log("node unavailable; skipping optional syntax check")
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "server.mjs")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	timeout := 10 * time.Second
	if runtime.GOOS == "windows" {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Parse only a newly generated temporary fixture. Do not import or execute
	// the module, load the SDK, install dependencies, or inherit Node preload flags.
	cmd := exec.CommandContext(ctx, node, "--check", path)
	cmd.Dir = dir
	cmd.Env = []string{"HOME=" + dir, "USERPROFILE=" + dir, "APPDATA=" + dir, "LOCALAPPDATA=" + dir, "TMPDIR=" + dir, "TMP=" + dir, "TEMP=" + dir, "PATH=" + os.Getenv("PATH")}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(dir)
		cmd.Env = append(cmd.Env, "SystemRoot="+os.Getenv("SystemRoot"), "HOMEDRIVE="+volume, "HOMEPATH="+strings.TrimPrefix(dir, volume))
	}
	started := time.Now()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node --check: %v; context=%v elapsed=%s process=%v\n%s", err, ctx.Err(), time.Since(started), cmd.ProcessState, out)
	}
}
