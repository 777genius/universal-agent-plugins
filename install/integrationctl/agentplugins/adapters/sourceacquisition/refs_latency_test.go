package sourceacquisition

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type refLatencyRunner struct {
	remote  string
	peeled  string
	fetches int
}

func (runner *refLatencyRunner) Run(_ context.Context, command Command) ([]byte, error) {
	if len(command.Args) > 0 && command.Args[0] == "check-ref-format" {
		return nil, nil
	}
	if len(command.Args) > 0 && command.Args[0] == "ls-remote" {
		return []byte(runner.remote), nil
	}
	if len(command.Args) > 2 && command.Args[2] == "fetch" {
		runner.fetches++
	}
	if runner.peeled != "" {
		if len(command.Args) > 2 && command.Args[2] == "rev-parse" {
			return []byte(runner.peeled + "\n"), nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected Git operation while resolving a branch: %v", command.Args)
}

func TestAnnotatedTagStillFetchesAndPeelsCommit(t *testing.T) {
	object, commit := strings.Repeat("3", 40), strings.Repeat("4", 40)
	runner := &refLatencyRunner{remote: object + "\trefs/tags/v1\n", peeled: commit}
	got, err := (Acquirer{TempRoot: t.TempDir(), Runner: runner}).ResolveGitHubSelector(t.Context(), "example/fixture", "refs/tags/v1")
	if err != nil || got.Commit != commit || got.ResolvedRef != "refs/tags/v1" || runner.fetches != 1 {
		t.Fatalf("annotated tag was not peeled: %+v %v fetches=%d", got, err, runner.fetches)
	}
}

func TestBranchResolutionAvoidsRedundantFetch(t *testing.T) {
	sha := strings.Repeat("1", 40)
	for _, ref := range []string{"main", "refs/heads/main", "default", strings.Repeat("2", 40)} {
		t.Run(ref, func(t *testing.T) {
			branch := strings.TrimPrefix(ref, "refs/heads/")
			runner := &refLatencyRunner{remote: sha + "\trefs/heads/" + branch + "\n"}
			acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
			got, err := acquirer.ResolveGitHubRef(t.Context(), "example/fixture", ref)
			if err != nil || got.Commit != sha || got.ResolvedRef != "refs/heads/"+branch || got.RequestedRef != ref {
				t.Fatalf("exact branch resolution changed: %+v %v", got, err)
			}
			if runner.fetches != 0 {
				t.Fatal("branch ref resolution fetched an unnecessary repository")
			}
		})
	}
}

func TestBranchURLResolutionAvoidsRedundantFetch(t *testing.T) {
	sha := strings.Repeat("1", 40)
	runner := &refLatencyRunner{remote: sha + "\trefs/heads/feature/design\n"}
	got, path, err := (Acquirer{TempRoot: t.TempDir(), Runner: runner}).ResolveGitHubURLPath(t.Context(), "example/fixture", "feature/design/skills/frontend", "")
	if err != nil || path != "skills/frontend" || got.Commit != sha || got.ResolvedRef != "refs/heads/feature/design" || runner.fetches != 0 {
		t.Fatalf("branch URL resolution: %+v %q %v fetches=%d", got, path, err, runner.fetches)
	}
}

func TestBranchFastPathPreservesAmbiguityAndRemoteRefChecks(t *testing.T) {
	sha := strings.Repeat("1", 40)
	for _, remote := range []string{
		sha + "\trefs/heads/main\n" + strings.Repeat("2", 40) + "\trefs/tags/main\n",
		sha + "\trefs/heads/unexpected\n",
		"invalid-object\trefs/heads/main\n",
	} {
		runner := &refLatencyRunner{remote: remote}
		if _, err := (Acquirer{TempRoot: t.TempDir(), Runner: runner}).ResolveGitHubSelector(t.Context(), "example/fixture", "main"); err == nil {
			t.Fatalf("invalid or ambiguous remote ref accepted: %q", remote)
		}
		if runner.fetches != 0 {
			t.Fatal("refusal performed a fetch")
		}
	}
}
