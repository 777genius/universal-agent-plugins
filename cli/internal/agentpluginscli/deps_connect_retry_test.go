package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

// stallingRunner fails the first fetch of every (remote, revision) with a
// connect-class error, as a stalled connection would, and runs the rest.
type stallingRunner struct {
	delegate sourceacquisition.Runner
	mu       sync.Mutex
	remotes  map[string]string // repository directory -> remote URL
	stalled  map[string]bool
	fetches  int
}

func (runner *stallingRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	args := command.Args
	if len(args) >= 6 && args[0] == "-C" && args[2] == "remote" && args[3] == "add" {
		runner.mu.Lock()
		runner.remotes[args[1]] = args[5]
		runner.mu.Unlock()
	}
	if len(args) >= 3 && args[0] == "-C" && args[2] == "fetch" {
		runner.mu.Lock()
		runner.fetches++
		key := runner.remotes[args[1]] + "@" + args[len(args)-1]
		stall := !runner.stalled[key]
		runner.stalled[key] = true
		runner.mu.Unlock()
		if stall {
			return nil, fmt.Errorf("injected stalled connection: %w", sourceacquisition.ErrGitConnect)
		}
	}
	return runner.delegate.Run(ctx, command)
}

func (runner *stallingRunner) reset() {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.remotes, runner.stalled, runner.fetches = map[string]string{}, map[string]bool{}, 0
}

// A stalled connection on every source costs one retry per source, and the
// serial and prefetched resolutions retry the same reads to the same lock.
func TestConnectRetryRescuesEverySourceSeriallyAndPrefetched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("per-repository fixture links need symlink privileges on Windows")
	}
	app, requirements, source := mixedRootsJourney(t)
	fixture := source.Acquirer.Acquirer.URLForRepo("")
	links := t.TempDir()
	source.Acquirer.Acquirer.URLForRepo = func(repository string) string {
		link := filepath.Join(links, strings.ReplaceAll(repository, "/", "-"))
		if _, err := os.Lstat(link); err != nil {
			if err := os.Symlink(fixture, link); err != nil && !os.IsExist(err) {
				t.Errorf("link %s: %v", repository, err)
			}
		}
		return link
	}
	for _, repository := range []string{"acme/pa", "acme/pb", "other/pc", "acme/skills", "other/skills"} {
		source.Acquirer.Acquirer.URLForRepo(repository)
	}
	runner := &stallingRunner{delegate: source.Acquirer.Acquirer.Runner}
	source.Acquirer.Acquirer.Runner = runner

	trace := filepath.Join(t.TempDir(), "prefetch.jsonl")
	t.Setenv(prefetchTraceEnv, trace)
	results := map[bool]string{}
	fetches := map[bool]int{}
	stalled := map[bool]int{}
	for _, serial := range []bool{true, false} {
		runner.reset()
		lock, err := resolveMixedRoots(t, t.Context(), app, requirements, nil, serial)
		if err != nil {
			t.Fatalf("serial=%v: one stalled connection per source was not rescued: %v", serial, err)
		}
		results[serial], fetches[serial], stalled[serial] = string(lock), runner.fetches, len(runner.stalled)
	}
	if results[true] != results[false] {
		t.Fatal("serial and prefetched resolutions produced different locks")
	}
	// 6 roots over 5 distinct (remote, revision) pairs: each pair stalls once
	// and is retried once; the second Skill of a shared pair fetches cleanly.
	for _, serial := range []bool{true, false} {
		if stalled[serial] != 5 || fetches[serial] != 6+5 {
			t.Fatalf("serial=%v: stalled pairs %d, fetches %d; want 5 and 11", serial, stalled[serial], fetches[serial])
		}
	}
	// The prefetch trace records the retry of each stalled root.
	body, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(body), `"retried":1`); got != 5 {
		t.Fatalf("trace shows %d retried roots, want 5:\n%s", got, body)
	}
}
