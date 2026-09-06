//go:build linux || (windows && amd64)

package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

type pathFixBinaries struct {
	paths    [2]string
	revision string
}

func buildPathFixBinaries(t *testing.T) pathFixBinaries {
	t.Helper()
	bin := os.Getenv("AUTHORING_NATIVE_BIN_DIR")
	supplied := bin != ""
	revision := "bad16ef524f612a18534bad0ea7d633e76bca50a+native-path-fix"
	if supplied {
		revision = os.Getenv("EXPECTED_HEAD")
		if len(revision) != 40 {
			t.Fatal("supplied binaries require EXPECTED_HEAD")
		}
	} else {
		bin = t.TempDir()
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	_, file, _, _ := runtime.Caller(0)
	module := filepath.Join(filepath.Dir(file), "../../..")
	b := pathFixBinaries{revision: revision}
	for i, name := range []string{"plugin-kit-ai", "agentplugins"} {
		b.paths[i] = filepath.Join(bin, name+suffix)
		if !supplied {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			prefix := "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
			cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"+suffix), "build", "-p", "2", "-ldflags", "-X "+prefix+".Enabled=vertical-slice-v1 -X "+prefix+".Revision="+revision, "-o", b.paths[i], "./cmd/"+name)
			cmd.Dir = module
			out, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("build actual %s: %v\n%s", name, err, out)
			}
		}
		body, err := os.ReadFile(b.paths[i])
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("path fix binary=%s revision=%s sha256=%x os=%s arch=%s", name, revision, sha256.Sum256(body), runtime.GOOS, runtime.GOARCH)
	}
	return b
}

func (b pathFixBinaries) run(t *testing.T, i int, cwd, scratch string, args ...string) (report.Report, int, []byte) {
	t.Helper()
	if i == 1 {
		args = append([]string{"author"}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.paths[i], append(args, "--format=json")...)
	cmd.Dir = cwd
	cmd.Env = append(nativeEnvironment(t.TempDir(), scratch, t.TempDir()), "GOMAXPROCS=2")
	var out, errout bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errout
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	if ctx.Err() != nil || errout.Len() != 0 {
		t.Fatalf("process timeout or raw stderr: %v %s", ctx.Err(), errout.Bytes())
	}
	r := decodeReport(t, out.Bytes())
	if r.Revision != b.revision {
		t.Fatalf("unexpected engine revision: %s", out.Bytes())
	}
	return r, code, out.Bytes()
}

func (b pathFixBinaries) journey(t *testing.T, parent, scratch string) {
	t.Helper()
	for i := range b.paths {
		cwd := filepath.Join(parent, fmt.Sprintf("entry-%d", i))
		if err := os.Mkdir(cwd, 0700); err != nil {
			t.Fatal(err)
		}
		name := "demo"
		r, code, out := b.run(t, i, cwd, scratch, "init", name, "--template=skill", "--name=demo", "--description=Disposable fixture.")
		if code != 0 || !r.Committed {
			t.Fatalf("relative init: %d %s", code, out)
		}
		root := filepath.Join(cwd, name)
		before := tree(t, root)
		for _, command := range []string{"validate", "inspect", "test"} {
			_, code, absolute := b.run(t, i, cwd, scratch, command, root)
			if code != 0 {
				t.Fatalf("absolute control: %d %s", code, absolute)
			}
			paths := []string{name, "./" + name, name + "/../" + name}
			if runtime.GOOS == "windows" {
				paths = append(paths, `.\`+name, name+`\..\`+name)
			}
			for _, path := range paths {
				r, code, out := b.run(t, i, cwd, scratch, command, path)
				if code != 0 || !bytes.Equal(out, absolute) || r.Identity.ReadProfile != "packageview-local-"+runtime.GOOS+"-v1" {
					t.Fatalf("relative %s %s differs: %d %s / %s", command, path, code, out, absolute)
				}
			}
			_, code, dot := b.run(t, i, root, scratch, command, ".")
			if code != 0 || !bytes.Equal(dot, absolute) {
				t.Fatalf("dot %s differs: %d %s", command, code, dot)
			}
		}
		if !reflect.DeepEqual(before, tree(t, root)) {
			t.Fatal("relative reads changed source bytes/modes")
		}
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatal("scratch leaked", err)
	}
}

func TestNativePathFixBothBinaries(t *testing.T) {
	b := buildPathFixBinaries(t)
	t.Run("ordinary-relative-roots", func(t *testing.T) {
		b.journey(t, t.TempDir(), t.TempDir())
	})
	t.Run("traversal-order", func(t *testing.T) {
		parent, scratch := t.TempDir(), t.TempDir()
		write(t, parent, "demo/plugin.json", plugin(""))
		write(t, parent, "regular", "inert")
		for _, path := range []string{"missing/../demo", "regular/../demo"} {
			for _, command := range []string{"validate", "inspect", "test"} {
				for i := range b.paths {
					r, code, out := b.run(t, i, parent, scratch, command, path)
					if code == 0 || r.Error == nil || r.Identity.ScopeDigest != "" {
						t.Fatalf("erased traversal evidence: %s %s: %d %s", command, path, code, out)
					}
				}
			}
		}
	})
	t.Run("reparse-before-dot-dot", func(t *testing.T) {
		parent, scratch := t.TempDir(), t.TempDir()
		write(t, parent, "demo/plugin.json", plugin(""))
		outside := t.TempDir()
		write(t, outside, "sub/keep", "inert")
		write(t, outside, "demo/plugin.json", plugin(""))
		write(t, outside, "demo/physical-target", "distinct identity")
		pathFixAlias(t, filepath.Join(parent, "alias"), filepath.Join(outside, "sub"))
		for _, command := range []string{"validate", "inspect", "test"} {
			for i := range b.paths {
				r, code, out := b.run(t, i, parent, scratch, command, "alias/../demo")
				if runtime.GOOS == "linux" {
					// Linux root selection already follows intermediate symlinks in
					// kernel traversal order. Keep that contract, including /.. .
					_, physicalCode, physical := b.run(t, i, parent, scratch, command, filepath.Join(outside, "demo"))
					if code != 0 || physicalCode != 0 || !bytes.Equal(out, physical) {
						t.Fatalf("Linux physical traversal changed: %d %s / %s", code, out, physical)
					}
					continue
				}
				if code == 0 || r.Error == nil || r.Error.Code != "root_unreadable" || r.Identity.ScopeDigest != "" {
					t.Fatalf("reparse/.. bypassed root gate: %d %s", code, out)
				}
			}
		}
		entries, err := os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatal("rejected root wrote scratch", err)
		}
	})
	pathFixWindowsContracts(t, b)
}

func pathFixNoScratch(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "packageview-") {
			t.Fatal("separation failure created scratch")
		}
	}
}
