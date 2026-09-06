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

// Only the harness launches binaries. The optional explicit strace lane observes
// every syscall in their process trees, rejecting any child exec/network syscall
// and any access to the newly-created private HOME/profile paths.
func TestNativeBinaryReadiness(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	module := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	revision := "8d4ca3eedb3afbdd401a87677b97597035f84146+readiness-worktree"
	bins := t.TempDir()
	evidence := os.Getenv("UAP_READINESS_EVIDENCE")
	traceTool := os.Getenv("UAP_READINESS_STRACE")
	guard := os.Getenv("UAP_READINESS_GUARD")
	if traceTool != "" && !filepath.IsAbs(traceTool) {
		t.Fatal("trace tool must be explicit absolute path")
	}
	binary := []string{filepath.Join(bins, "plugin-kit-ai"), filepath.Join(bins, "agentplugins")}
	for i, name := range []string{"plugin-kit-ai", "agentplugins"} {
		prefix := "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
		cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "build", "-p", "2", "-ldflags", "-X "+prefix+".Enabled=vertical-slice-v1 -X "+prefix+".Revision="+revision, "-o", binary[i], "./cmd/"+name)
		cmd.Dir = module
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		b, err := os.ReadFile(binary[i])
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("binary=%s sha256=%x revision=%s toolchain=%s platform=%s/%s", name, sha256.Sum256(b), revision, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
	homes := []string{t.TempDir(), t.TempDir()}
	scratch := []string{t.TempDir(), t.TempDir()}
	traps := t.TempDir()
	effect := filepath.Join(traps, "effect")
	for _, name := range []string{"node", "npm", "npx", "python", "python3", "go", "git", "curl", "wget", "claude", "codex", "agentplugins", "plugin-kit-ai"} {
		write(t, traps, name, "#!/bin/sh\nprintf invoked > '"+effect+"'\nexit 79\n")
		if err := os.Chmod(filepath.Join(traps, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// Marker profiles are never needed for explicit clients, even if present.
	for _, home := range homes {
		for _, name := range []string{".npmrc", ".gitconfig", ".codex/auth.json", ".config/agentplugins/state.json", ".claude/settings.json"} {
			write(t, home, name, marker)
		}
	}
	homeBefore := []map[string]string{tree(t, homes[0]), tree(t, homes[1])}
	invocation := 0
	run := func(i int, args ...string) (report.Report, int, []byte) {
		t.Helper()
		invocation++
		selected, access := "-", "read"
		invocationGuard := guard
		if args[0] == "init" {
			invocationGuard = ""
		} // fixture creation is the separately verified baseline mutation
		if len(args) > 1 && args[0] != "capabilities" {
			selected = args[1]
		}
		leadingFlag := strings.HasPrefix(args[0], "--")
		if leadingFlag {
			selected = args[2]
		}
		if args[0] == "init" {
			selected, access = filepath.Dir(selected), "write"
		}
		if i == 1 {
			if leadingFlag {
				args = append([]string{args[0], "author"}, args[1:]...)
			} else {
				args = append([]string{"author"}, args...)
			}
		}
		args = append(args, "--format=json")
		cmd := exec.Command(binary[i], args...)
		if invocationGuard != "" {
			cmd = exec.Command(invocationGuard, append([]string{binary[i], scratch[i], selected, access, "--"}, args...)...)
		}
		trace := filepath.Join(t.TempDir(), "trace")
		if traceTool != "" {
			traced := []string{"-f", "-qq", "-s", "4096", "-e", "trace=%process,%network,%file", "-o", trace, binary[i]}
			cmd = exec.Command(traceTool, append(traced, args...)...)
		}
		cmd.Dir = bins
		cmd.Env = []string{"HOME=" + homes[i], "XDG_CONFIG_HOME=" + homes[i], "XDG_CACHE_HOME=" + homes[i], "PATH=" + traps, "TMPDIR=" + scratch[i], "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "AGENTPLUGINS_DIRECTORY_ORIGIN=" + marker, "AGENTPLUGINS_SECURITY_ORIGIN=" + marker}
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				code = e.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("stderr: %s", stderr.Bytes())
		}
		if out.Len() == 0 {
			t.Fatalf("no report, process exit=%d error=%v", code, err)
		}
		r := decodeReport(t, out.Bytes())
		if r.Revision != revision {
			t.Fatal("revision drift")
		}
		if traceTool != "" {
			b, err := os.ReadFile(trace)
			if err != nil {
				t.Fatal(err)
			}
			execs := 0
			for _, line := range strings.Split(string(b), "\n") {
				if strings.Contains(line, "execve(") {
					execs++
					continue
				}
				if strings.Contains(line, "execveat(") || strings.Contains(line, "fork(") || strings.Contains(line, "vfork(") {
					t.Fatalf("process effect: %s", line)
				}
				if strings.Contains(line, "clone(") && !strings.Contains(line, "CLONE_THREAD") || strings.Contains(line, "clone3(") && !strings.Contains(line, "CLONE_THREAD") {
					t.Fatalf("non-thread clone: %s", line)
				}
				for _, call := range []string{"socket(", "socketpair(", "connect(", "bind(", "listen(", "accept(", "sendto(", "sendmsg(", "recvfrom(", "recvmsg("} {
					if strings.Contains(line, call) {
						t.Fatalf("network effect: %s", line)
					}
				}
				if strings.Contains(line, homes[i]) {
					t.Fatalf("profile access: %s", line)
				}
			}
			if execs != 1 {
				t.Fatalf("expected sole native exec, got %d", execs)
			}
			if evidence != "" {
				if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("native-%03d.trace", invocation)), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !reflect.DeepEqual(homeBefore[i], tree(t, homes[i])) || len(tree(t, scratch[i])) != 0 {
			t.Fatal("home mutation or scratch residue")
		}
		if _, err := os.Stat(effect); !os.IsNotExist(err) {
			t.Fatal("process trap fired")
		}
		t.Logf("case=%03d binary=%d command=%s exit=%d output_sha256=%x trace=%t guard=%t", invocation, i, r.Command, code, sha256.Sum256(out.Bytes()), traceTool != "", invocationGuard != "")
		if evidence != "" {
			if err := os.WriteFile(filepath.Join(evidence, fmt.Sprintf("native-%03d.json", invocation)), out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		return r, code, out.Bytes()
	}
	parity := func(args []string, want int) report.Report {
		t.Helper()
		a, ac, ab := run(0, args...)
		_, bc, bb := run(1, args...)
		if ac != want || bc != want || !bytes.Equal(ab, bb) {
			t.Fatalf("parity %v: %d %d want %d\n%s\n%s", args, ac, bc, want, ab, bb)
		}
		return a
	}
	c := parity([]string{"capabilities"}, 0)
	if c.Capabilities == nil || len(c.Capabilities.Commands) != 8 || c.Readiness.Status != report.NotEvaluated {
		t.Fatal("capabilities evidence", c)
	}
	for _, lane := range []struct {
		name   string
		extra  []string
		doctor int
	}{
		{"skill", nil, 0}, {"mcp-remote", []string{"--url=https://invalid.example/mcp"}, 1},
		{"mcp-stdio", []string{"--runtime=node"}, 1}, {"hybrid", []string{"--mcp=mcp-remote", "--url=https://invalid.example/mcp"}, 1},
		{"hybrid", []string{"--mcp=mcp-stdio", "--runtime=node"}, 1},
	} {
		root := filepath.Join(t.TempDir(), "fixture")
		args := append([]string{"init", root, "--name=demo", "--description=Disposable fixture", "--template=" + lane.name}, lane.extra...)
		if _, code, _ := run(0, args...); code != 0 {
			t.Fatal("fixture generation")
		}
		before := tree(t, root)
		r := parity([]string{"compat", root, "--target=cursor,codex,claude"}, 0)
		if len(r.Clients) != 3 || r.Compatibility.Status != report.Pass || r.Runtime.Status != report.NotEvaluated {
			t.Fatal("compat invented runtime", r)
		}
		parity([]string{"inspect", root, "--target=cursor,codex,claude"}, 0)
		r = parity([]string{"doctor", root}, lane.doctor)
		if r.Runtime.Status != report.NotEvaluated || lane.doctor == 1 && r.Toolchain.Status == report.Pass {
			t.Fatal("doctor fake proof", r)
		}
		if !reflect.DeepEqual(before, tree(t, root)) {
			t.Fatal("source changed")
		}
	}
	root := t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	parity([]string{"--target=codex", "inspect", root}, 0)
	if _, code, _ := run(1, "--scope=user", "doctor", root); code != 2 {
		t.Fatal("installer flag before author accepted")
	}
	write(t, root, "mcp.json", mcp(`{"local":{"type":"stdio","command":"./bin/fixture"}}`))
	write(t, root, "bin/fixture", "#!/bin/sh\nprintf invoked > '"+effect+"'\n")
	r := parity([]string{"doctor", root}, 1)
	if r.Toolchain.Status != report.Fail || r.Conformance.Status != report.Pass {
		t.Fatal("mode conflated with conformance", r)
	}
	if err := os.Chmod(filepath.Join(root, "bin/fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	r = parity([]string{"doctor", root}, 1)
	if r.Toolchain.Status != report.NotEvaluated {
		t.Fatal("mode proves runtime", r)
	}
	if err := os.Remove(filepath.Join(root, "bin/fixture")); err != nil {
		t.Fatal(err)
	}
	parity([]string{"doctor", root}, 1)
	write(t, root, "plugin.json", plugin(`,"extensions":{"dev.example":{"secret":"`+marker+`"}}`))
	write(t, root, "mcp.json", mcp(`{"good":{"type":"streamable-http","url":"https://invalid.example/mcp","headers":{"Authorization":"`+marker+`"}},"bad":{"type":"invalid","command":"`+marker+`"}}`))
	parity([]string{"compat", root, "--target=cursor,chatgpt,windsurf"}, 1)
	parity([]string{"doctor", root}, 1)
	write(t, root, "plugin.json", `{"$schema":"https://invalid.example/future","name":"demo"}`)
	r = parity([]string{"compat", root, "--target=codex"}, 1)
	if len(r.Clients) != 0 || r.Compatibility.Status != report.NotEvaluated {
		t.Fatal("unknown schema guessed")
	}
	r = parity([]string{"doctor", root}, 1)
	if len(r.DoctorChecks) != 0 {
		t.Fatal("future doctor guessed")
	}
	for _, args := range [][]string{
		{"compat", root}, {"compat", root, "--target=codex,codex"}, {"compat", root, "--target=" + marker},
		{"compat", root, "--target=codex", "--dry-run=false"}, {"inspect", root, "--target="},
		{"doctor", root, "--target=codex"}, {"doctor", root, "--runtime"}, {"capabilities", "unexpected"},
	} {
		parity(args, 2)
	}
	for _, flag := range []string{"--scope=user", "--accept-security-risk=false", "--security-details=false"} {
		if r, code, _ := run(1, "compat", root, "--target=codex", flag); code != 2 || r.Error.Code != "arguments_invalid" {
			t.Fatal("installer-only flag accepted")
		}
	}
	t.Logf("verified invocations=%d process_trap_hits=0 home_changes=0 scratch_residue=0 syscall_tracing=%t confinement=%t", invocation, traceTool != "", guard != "")
}

// Calibrate the optional kernel guard against effects, rather than assuming a
// clean command result proves that the external harness actually confines it.
func TestReadinessGuardTraps(t *testing.T) {
	guard := os.Getenv("UAP_READINESS_GUARD")
	if guard == "" {
		t.Log("kernel guard calibration not requested")
		return
	}
	root, home, scratch := t.TempDir(), t.TempDir(), t.TempDir()
	source := filepath.Join(root, "probe.c")
	probe := filepath.Join(root, "probe")
	body := `#include <sys/socket.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
int main(int argc,char **argv) {
 if(argc!=3) return 99;
 if(argv[1][0]=='n') return socket(AF_INET,SOCK_STREAM,0)<0?98:97;
 if(argv[1][0]=='e') { execl("/bin/true","true",(char*)0); return 96; }
 if(argv[1][0]=='f') { fork(); return 95; }
 int fd=open(argv[2],argv[1][0]=='w'?O_WRONLY:O_RDONLY);
 return fd<0 && errno==EACCES?0:94;
}`
	if err := os.WriteFile(source, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cc := os.Getenv("UAP_READINESS_CC")
	if cc == "" || !filepath.IsAbs(cc) {
		t.Fatal("guard calibration requires explicit UAP_READINESS_CC")
	}
	if out, err := exec.Command(cc, "-Wall", "-Wextra", "-Werror", "-o", probe, source).CombinedOutput(); err != nil {
		t.Fatalf("compile guard probe: %v %s", err, out)
	}
	write(t, home, "credentials", marker)
	write(t, root, "source", marker)
	for _, mode := range []string{"network", "exec", "fork", "profile", "write"} {
		path := filepath.Join(home, "credentials")
		if mode == "write" {
			path = filepath.Join(root, "source")
		}
		cmd := exec.Command(guard, probe, scratch, root, "read", "--", mode, path)
		cmd.Env = []string{"HOME=" + home, "PATH=" + root}
		out, err := cmd.CombinedOutput()
		if mode == "profile" || mode == "write" {
			if err != nil {
				t.Fatalf("guard did not deny %s file access: %v %s", mode, err, out)
			}
		} else {
			e, ok := err.(*exec.ExitError)
			if !ok || !strings.Contains(e.String(), "bad system call") {
				t.Fatalf("guard did not trap %s: %v %s", mode, err, out)
			}
		}
		t.Logf("calibration=%s enforced=true", mode)
	}
}
