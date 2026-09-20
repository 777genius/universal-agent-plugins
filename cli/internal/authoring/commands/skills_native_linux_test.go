package commands_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

// Harness subprocesses are native CLI entrypoints in disposable fixtures only.
// With UAP_SKILLS_STRACE, every invocation is observed for child execution,
// socket/DNS activity, PATH probing and private HOME/profile access.
func TestSkillsNativeEntrypoints(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	module := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	bins := t.TempDir()
	binary := []string{filepath.Join(bins, "plugin-kit-ai"), filepath.Join(bins, "agentplugins")}
	revision := "0e74767e09169f58145d56f5d3e10b4038171e3a+skills-worktree"
	traceTool, evidence := os.Getenv("UAP_SKILLS_STRACE"), os.Getenv("UAP_SKILLS_EVIDENCE")
	guard := os.Getenv("UAP_SKILLS_GUARD")
	if guard != "" && !filepath.IsAbs(guard) {
		t.Fatal("guard must be an explicit absolute tool")
	}
	if traceTool != "" && !filepath.IsAbs(traceTool) {
		t.Fatal("strace must be an explicit absolute tool")
	}
	for i, name := range []string{"plugin-kit-ai", "agentplugins"} {
		prefix := "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
		build := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-p", "2", "-ldflags", "-X "+prefix+".Enabled=vertical-slice-v1 -X "+prefix+".Revision="+revision, "-o", binary[i], "./cmd/"+name)
		build.Dir = module
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("native build: %v %s", err, out)
		}
		body, err := os.ReadFile(binary[i])
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("binary=%s sha256=%x revision=%s toolchain=%s platform=%s/%s", name, sha256.Sum256(body), revision, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
	homes, scratch, roots := []string{t.TempDir(), t.TempDir()}, []string{t.TempDir(), t.TempDir()}, []string{t.TempDir(), t.TempDir()}
	traps := t.TempDir()
	effect := filepath.Join(traps, "effect")
	for _, name := range []string{"node", "npm", "npx", "python", "python3", "go", "git", "curl", "wget", "claude", "codex", "agentplugins", "plugin-kit-ai"} {
		write(t, traps, name, "#!/bin/sh\nprintf invoked > '"+effect+"'\nexit 79\n")
		if err := os.Chmod(filepath.Join(traps, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for i, root := range roots {
		write(t, root, "plugin.json", plugin(""))
		write(t, root, "mcp.json", mcp(`{"offline":{"type":"stdio","command":"missing-fixture-runtime"}}`))
		for _, path := range []string{".codex/auth.json", ".claude/settings.json", ".config/agentplugins/state.json", ".npmrc", ".gitconfig"} {
			write(t, homes[i], path, marker)
		}
	}
	homeBefore := []map[string]string{tree(t, homes[0]), tree(t, homes[1])}
	invocation := 0
	run := func(i int, cwd string, args ...string) (report.Report, int, []byte) {
		t.Helper()
		invocation++
		if i == 1 {
			args = append([]string{"author"}, args...)
		}
		args = append(args, "--format=json")
		cmd := exec.Command(binary[i], args...)
		if guard != "" {
			access := "read"
			offset := 0
			if i == 1 {
				offset = 1
			}
			if len(args) > offset+1 && args[offset] == "skills" && args[offset+1] == "init" {
				access = "write"
			}
			cmd = exec.Command(guard, append([]string{binary[i], scratch[i], cwd, access, "--"}, args...)...)
		}
		trace := filepath.Join(t.TempDir(), "trace")
		if traceTool != "" {
			cmd = exec.Command(traceTool, append([]string{"-f", "-qq", "-s", "4096", "-e", "trace=%process,%network,%file", "-o", trace, binary[i]}, args...)...)
		}
		cmd.Dir = cwd
		cmd.Env = []string{"HOME=" + homes[i], "XDG_CONFIG_HOME=" + homes[i], "XDG_CACHE_HOME=" + homes[i], "PATH=" + traps, "TMPDIR=" + scratch[i], "GOMAXPROCS=2", "GIT_CONFIG_NOSYSTEM=1", "AGENTPLUGINS_DIRECTORY_ORIGIN=" + marker}
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			e, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = e.ExitCode()
		}
		if stderr.Len() != 0 {
			t.Fatalf("unexpected stderr: %s", stderr.Bytes())
		}
		r := decodeReport(t, out.Bytes())
		if r.Revision != revision {
			t.Fatal("revision mismatch")
		}
		if traceTool != "" {
			body, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			execs := 0
			for _, line := range strings.Split(string(body), "\n") {
				if strings.Contains(line, "execve(") {
					execs++
					continue
				}
				for _, call := range []string{"execveat(", "fork(", "vfork(", "socket(", "socketpair(", "connect(", "bind(", "listen(", "accept(", "sendto(", "sendmsg(", "recvfrom(", "recvmsg("} {
					if strings.Contains(line, call) {
						t.Fatalf("static effect: %s", line)
					}
				}
				if (strings.Contains(line, "clone(") || strings.Contains(line, "clone3(")) && !strings.Contains(line, "CLONE_THREAD") {
					t.Fatalf("child process: %s", line)
				}
				if strings.Contains(line, homes[i]) || strings.Contains(line, traps) {
					t.Fatalf("profile or PATH lookup: %s", line)
				}
			}
			if execs != 1 {
				t.Fatalf("expected only entrypoint exec, got %d", execs)
			}
			if evidence != "" {
				if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("skills-native-%03d.trace", invocation)), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !reflect.DeepEqual(homeBefore[i], tree(t, homes[i])) || len(tree(t, scratch[i])) != 0 {
			t.Fatal("private HOME changed or scratch leaked")
		}
		if _, err := os.Stat(effect); !os.IsNotExist(err) {
			t.Fatal("PATH process trap fired")
		}
		if evidence != "" {
			if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("skills-native-%03d.json", invocation)), out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("case=%03d binary=%d command=%s exit=%d sha256=%x syscall_observation=%t guard=%t", invocation, i, r.Command, code, sha256.Sum256(out.Bytes()), traceTool != "", guard != "")
		return r, code, out.Bytes()
	}
	var generated []byte
	for i := range roots {
		r, code, b := run(i, roots[i], "skills", "init", "docs-helper", "--description", marker)
		if code != 0 || !r.Committed || r.Runtime.Status != report.NotEvaluated {
			t.Fatalf("native init: %d %+v", code, r)
		}
		if i == 0 {
			generated = b
		} else if !bytes.Equal(generated, b) {
			t.Fatalf("native mutation parity:\n%s\n%s", generated, b)
		}
	}
	for _, args := range [][]string{{"skills", "validate"}, {"validate", roots[0]}, {"test", roots[0]}, {"skills", "init", "docs-helper", "--description", marker}, {"skills", "init", "new"}, {"skills", "install", marker}, {"skills", "validate", "--profile", marker}, {"skills", "init", "new", "--description", marker, "--scope", "user"}} {
		_, ac, ab := run(0, roots[0], args...)
		_, bc, bb := run(1, roots[0], args...)
		if ac != bc || !bytes.Equal(ab, bb) {
			t.Fatalf("native parity %v: %d %d\n%s\n%s", args, ac, bc, ab, bb)
		}
		success := len(args) == 2 && (args[0] == "validate" || args[0] == "test" || args[1] == "validate")
		if (ac == 0) != success {
			t.Fatalf("exit for %v: %d", args, ac)
		}
	}
	// Retain an independently usable sibling in a malformed component package.
	write(t, roots[0], "skills/bad/SKILL.md", "---\nname: bad\ndescription: 7\n---\n")
	before := tree(t, roots[0])
	r, ac, ab := run(0, roots[0], "skills", "validate")
	_, bc, bb := run(1, roots[0], "skills", "validate")
	if ac != 1 || bc != 1 || !bytes.Equal(ab, bb) {
		t.Fatal("bad sibling parity")
	}
	pass, fail := 0, 0
	for _, c := range r.Components {
		if c.Type == "skill" {
			if c.Status == report.Pass {
				pass++
			} else if c.Status == report.Fail {
				fail++
			}
		}
	}
	if r.HostSafety.Status != report.Pass || pass != 1 || fail != 1 || !reflect.DeepEqual(before, tree(t, roots[0])) {
		t.Fatal("failure isolation or source preservation")
	}
	// Omission binds cwd, even if a valid package exists immediately above it.
	child := filepath.Join(roots[1], "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		r, code, _ := run(i, child, "skills", "validate")
		if code == 0 || r.Loadability.Status == report.Pass {
			t.Fatal("native ancestor discovery")
		}
	}
}
