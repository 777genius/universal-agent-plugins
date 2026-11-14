//go:build linux

package process

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const localGitCleanupIterations = 512

// This bounded observation runs only on the explicit diagnostic workflow.
// Passing the sample does not explain a previously observed cleanup failure.
func TestOSRunnerLocalGitFetchCleanupObservation(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_GIT_CLEANUP_DIAGNOSTIC") != "1" {
		t.Skip("opt-in bounded local Git cleanup observation")
	}
	if err := (OS{}).OrdinaryPreflight(); err != nil {
		t.Fatal("local Git cleanup observation requires the real ordinary containment prerequisite")
	}
	f := newLocalGitCleanupFixture(t)
	source, revision := f.source(t)
	for iteration := range localGitCleanupIterations {
		f.fetch(t, source, revision, iteration)
	}
	t.Logf("local Git cleanup observation completed iterations=%d", localGitCleanupIterations)
}

type localGitCleanupFixture struct {
	ctx                 context.Context
	root, git, template string
	environment         []string
	options             []string
}

func newLocalGitCleanupFixture(t *testing.T) localGitCleanupFixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(git) {
		t.Fatal("local Git cleanup observation requires a located absolute Git executable")
	}
	root := t.TempDir()
	home, temporary := filepath.Join(root, "home"), filepath.Join(root, "tmp")
	template, hooks := filepath.Join(root, "template"), filepath.Join(root, "hooks")
	for _, directory := range []string{home, filepath.Join(home, ".config"), temporary, template, hooks} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal("create private local Git fixture directory")
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	t.Cleanup(cancel)
	return localGitCleanupFixture{
		ctx: ctx, root: root, git: git, template: template,
		environment: []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + home, "USERPROFILE=" + home,
			"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "TMPDIR=" + temporary, "TMP=" + temporary, "TEMP=" + temporary,
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0",
			"GIT_ALLOW_PROTOCOL=file", "GIT_LFS_SKIP_SMUDGE=1", "GCM_INTERACTIVE=Never", "SSH_ASKPASS_REQUIRE=never",
			"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "LC_ALL=C", "LANG=C",
		},
		options: []string{
			"-c", "core.hooksPath=" + hooks, "-c", "core.fsmonitor=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0",
			"-c", "fetch.recurseSubmodules=false", "-c", "submodule.recurse=false", "-c", "protocol.allow=never",
			"-c", "protocol.file.allow=always", "-c", "credential.helper=", "-c", "commit.gpgsign=false",
		},
	}
}

func (f localGitCleanupFixture) source(t *testing.T) (string, string) {
	t.Helper()
	root := filepath.Join(f.root, "source")
	f.run(t, "source_init", -1, "init", "--quiet", "--initial-branch=main", "--template="+f.template, root)
	for index := range 8 {
		body := strings.Repeat(fmt.Sprintf("fixed local Git cleanup payload %d\n", index), 128)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("payload-%02d.txt", index)), []byte(body), 0600); err != nil {
			t.Fatal("write fixed local Git cleanup payload")
		}
	}
	f.run(t, "source_add", -1, "-C", root, "add", ".")
	f.run(t, "source_commit", -1, "-C", root, "commit", "--quiet", "-m", "fixed local cleanup fixture")
	revision := strings.TrimSpace(string(f.run(t, "source_revision", -1, "-C", root, "rev-parse", "--verify", "HEAD^{commit}")))
	decoded, err := hex.DecodeString(revision)
	if err != nil || len(decoded) != 20 || revision != strings.ToLower(revision) {
		t.Fatal("local Git fixture did not produce an immutable SHA-1 commit")
	}
	return root, revision
}

func (f localGitCleanupFixture) fetch(t *testing.T, source, revision string, iteration int) {
	t.Helper()
	destination := filepath.Join(f.root, fmt.Sprintf("fetch-%03d", iteration))
	f.run(t, "destination_init", iteration, "init", "--quiet", "--template="+f.template, destination)
	f.run(t, "remote", iteration, "-C", destination, "remote", "add", "origin", source)
	for _, setting := range [][]string{{"core.autocrlf", "false"}, {"core.filemode", "true"}, {"core.symlinks", "true"}, {"fetch.recurseSubmodules", "false"}} {
		f.run(t, "settings", iteration, "-C", destination, "config", setting[0], setting[1])
	}
	// The positively classified terminal fixture uses direct AcquireGit:
	// native=true and this exact fetch graph, without the anonymous filter flag.
	f.run(t, "fetch", iteration, "-C", destination, "fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "origin", revision)
	got := f.run(t, "verify", iteration, "-C", destination, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if strings.TrimSpace(string(got)) != revision {
		t.Fatalf("local Git fetched revision mismatch iteration=%d", iteration)
	}
	if err := os.RemoveAll(destination); err != nil {
		t.Fatalf("remove completed private Git destination iteration=%d", iteration)
	}
}

func (f localGitCleanupFixture) run(t *testing.T, phase string, iteration int, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.ctx, 15*time.Second)
	defer cancel()
	argv := append([]string{f.git}, f.options...)
	argv = append(argv, arguments...)
	result, err := (OS{}).RunWithTreeExitGrace(ctx, ports.Command{
		Argv: argv, Env: f.environment, Dir: f.root, StdoutLimitBytes: 4096,
	}, 5*time.Second)
	if err != nil {
		class := OrdinarySupervisorFailureClass(err)
		const cleanup = "ordinary_supervisor_descendant_cleanup"
		if class == cleanup || strings.HasPrefix(class, cleanup+"_") {
			// This is only the real process error from the fixed local fixture,
			// with no inherited Git/auth/network inputs. Quote before applying the
			// output cap so escaping cannot expand the emitted cause past 4 KiB.
			cause := fmt.Sprintf("%q", err.Error())
			if len(cause) > 4096 {
				cause = cause[:4096]
			}
			t.Fatalf("local Git cleanup phase=%s iteration=%d class=%s exit_status=unavailable cause=%s", phase, iteration, class, cause)
		}
		t.Fatalf("local Git cleanup phase=%s iteration=%d class=%s exit_status=unavailable", phase, iteration, class)
	}
	if result.ExitCode != 0 {
		t.Fatalf("local Git cleanup phase=%s iteration=%d class=git_exit exit_status=%d", phase, iteration, result.ExitCode)
	}
	return result.Stdout
}
