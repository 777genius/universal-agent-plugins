package sourceacquisition

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type rawSkillTransportRunner struct {
	commit string
	tree   string
	calls  []Command
}

func (runner *rawSkillTransportRunner) Run(_ context.Context, command Command) ([]byte, error) {
	runner.calls = append(runner.calls, command)
	args := command.Args
	if len(args) > 2 && args[0] == "-C" {
		args = args[2:]
	}
	if len(args) > 0 {
		switch args[0] {
		case "rev-parse":
			return []byte(runner.commit + "\n"), nil
		case "ls-tree":
			return []byte(runner.tree), nil
		}
	}
	return nil, nil
}

func TestNativeSkillTreeFetchesCompleteAuthenticatedCommit(t *testing.T) {
	runner := &rawSkillTransportRunner{commit: strings.Repeat("1", 40)}
	acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
	tree, err := acquirer.OpenGitTree(t.Context(), sourcespec.Git{URL: "ssh://git@git.example/team/tools.git"}, runner.commit)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tree.Close() }()
	fetches := 0
	for _, call := range runner.calls {
		if len(call.Args) > 0 && call.Args[0] == "fetch" {
			fetches++
			if !call.NativeCredentials || strings.Contains(strings.Join(call.Args, " "), "--filter") {
				t.Fatal("immutable Skill tree lost authentication or retained promised objects")
			}
		}
	}
	if fetches != 1 {
		t.Fatalf("immutable fetch count=%d", fetches)
	}
	runner.commit = strings.Repeat("2", 40)
	if _, err := acquirer.OpenGitTree(t.Context(), sourcespec.Git{URL: "https://git.example/team/tools.git"}, strings.Repeat("1", 40)); err == nil {
		t.Fatal("immutable tree accepted another commit")
	}
}

func TestNativeRawSkillRejectsRootAndSelectedSubmodules(t *testing.T) {
	for _, selected := range []string{"", "skills/pdf"} {
		commit := strings.Repeat("1", 40)
		prefix := ""
		if selected != "" {
			prefix = selected + "/"
		}
		runner := &rawSkillTransportRunner{commit: commit, tree: "100644 blob " + commit + "\t" + prefix + "SKILL.md\x00160000 commit " + commit + "\t" + prefix + "resources\x00"}
		acquirer := Acquirer{TempRoot: t.TempDir(), Runner: runner}
		if _, err := acquirer.AcquireGitSkill(t.Context(), sourcespec.Git{URL: "ssh://git@git.example/team/tools.git", Path: selected}, commit, ""); err == nil {
			t.Fatalf("raw Skill silently omitted submodule from %q", selected)
		}
		for _, call := range runner.calls {
			if strings.Contains(strings.Join(call.Args, " "), "checkout") {
				t.Fatal("invalid raw Skill proceeded to checkout")
			}
		}
	}
}
