//go:build uappm_e2e

package agentpluginscli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pmPostPullGit runs a genuine local Git operation in a disposable repository.
// Portable source rows are the only tracked files; state/receipts/native files
// belong to distinct fixture roots and are never copied through Git.
func pmPostPullGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{
		"-C", root,
		"-c", "core.autocrlf=false",
		"-c", "commit.gpgsign=false",
		"-c", "tag.gpgsign=false",
		"-c", "core.hooksPath=" + filepath.Join(t.TempDir(), "disabled-hooks"),
		"-c", "user.name=777genius",
		"-c", "user.email=fixture@example.invalid",
	}, args...)...)
	// A test must not inherit routing, tracing, identity or other Git
	// configuration from a developer's current worktree. The fixture uses
	// only disposable local repositories and does not need remote credentials.
	// Git also derives its implicit excludes/attributes files from HOME and
	// XDG_CONFIG_HOME. Without a private home, a developer-wide *.ignore rule
	// can hide leaked untracked files and make the oracle falsely pass.
	isolatedHome := t.TempDir()
	env := make([]string, 0, len(os.Environ())+6)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		switch upper := strings.ToUpper(key); {
		case strings.HasPrefix(upper, "GIT_"), upper == "HOME",
			upper == "USERPROFILE", upper == "XDG_CONFIG_HOME":
			continue
		}
		env = append(env, value)
	}
	command.Env = append(env,
		"HOME="+isolatedHome, "USERPROFILE="+isolatedHome,
		"XDG_CONFIG_HOME="+isolatedHome,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_GLOBAL="+filepath.Join(isolatedHome, "empty-global-gitconfig"))
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("real Git %q failed: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func pmPostPullCommit(t *testing.T, root, message string) {
	t.Helper()
	pmPostPullGit(t, root, "add", "--", "uap.toml", "uap.lock")
	pmPostPullGit(t, root, "commit", "-q", "-m", message)
	if got := pmPostPullGit(t, root, "status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Fatalf("author has unexpected files after commit: %q", got)
	}
}

func pmPostPullNoUntracked(t *testing.T, root string) {
	t.Helper()
	if got := pmPostPullGit(t, root, "status", "--porcelain", "--untracked-files=all"); got != "" {
		t.Fatalf("portable checkout changed after locked install: %q", got)
	}
}

func pmPostPullAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("orphaned managed/native file remains at %q: %v", path, err)
	}
}


func pmPostPullGitHostIgnoreNegativeControl(t *testing.T) {
	t.Helper()
	// An external HOME with a global ignore for *.ghost must not make the
	// checked-out test repo appear clean when it contains a stray file.
	t.Run("host-global-ignore-cannot-hide-untracked", func(t *testing.T) {
		host := t.TempDir()
		config := filepath.Join(host, ".config")
		if err := os.MkdirAll(filepath.Join(config, "git"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, "git", "ignore"), []byte("*.ghost\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", host)
		t.Setenv("USERPROFILE", host)
		t.Setenv("XDG_CONFIG_HOME", config)
		root := filepath.Join(t.TempDir(), "disposable-git")
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		pmPostPullGit(t, root, "init", "-q", "-b", "main")
		if err := os.WriteFile(filepath.Join(root, "orphan.ghost"), []byte("an unexpected file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if status := pmPostPullGit(t, root, "status", "--porcelain", "--untracked-files=all"); !strings.Contains(status, "orphan.ghost") {
			t.Fatalf("host-global excludes hid an orphaned untracked file: %q", status)
		}
	})
}

// C9: APM #3188 reports agent-visible orphan Skills after git pull drops a
// dependency. This scenario strengthens the previously proven one-root case:
// two parents share child C, and only the *last* removal may retire C. The
// teammate never has the prior portable agreement (Git overwrote it); only
// local durable owner receipts can determine what must be reconciled.
func pmEvolutionPostPullSharedOwnerAndDrift(t *testing.T) {
	pmPostPullGitHostIgnoreNegativeControl(t)
	graph := pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": nil}
	origin := newPMArchiveOrigin(t, graph)
	root := t.TempDir()
	authorRoot := filepath.Join(root, "author")
	teammateRoot := filepath.Join(root, "teammate")
	if err := os.MkdirAll(authorRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	author := newPMArchiveEnvironment(t, origin, false)
	author.app.DependenciesRoot = authorRoot
	author.manifest(t, origin, []string{"a", "b"}, map[string][]string{"gemini": {"a", "b"}})
	author.run(t, "install", "--yes", "--format", "json")
	author.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	author.assertInstalled(t, pmArchiveAll(graph, "gemini"))

	pmPostPullGit(t, authorRoot, "init", "-q", "-b", "main")
	pmPostPullCommit(t, authorRoot, "both parents require c")
	pmPostPullGit(t, root, "clone", "-q", authorRoot, teammateRoot)

	teammate := newPMArchiveEnvironment(t, origin, false)
	teammate.app.DependenciesRoot = teammateRoot
	teammate.run(t, "install", "--locked", "--yes", "--format", "json")
	teammate.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	initial := teammate.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	pmPostPullNoUntracked(t, teammateRoot)

	nativeRoot := teammate.clients["gemini"].ConfigRoot
	foreign := filepath.Join(nativeRoot, "skills", "foreign-neighbor", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, foreign, "---\nname: foreign-neighbor\ndescription: independent user Skill\n---\nDo not modify.\n")
	userData := pmEvolutionSeedData(t, teammate)
	foreignBefore := freshRestoreFiles(t, foreign, userData)
	nativeC := filepath.Join(nativeRoot, "skills", "c", "SKILL.md")
	managedC := filepath.Join(initial["c/gemini"].TargetPath, "marker.txt")
	nativeCBefore, err := os.Stat(nativeC)
	if err != nil {
		t.Fatal(err)
	}
	managedCBefore, err := os.Stat(managedC)
	if err != nil {
		t.Fatal(err)
	}
	survivorC := freshRestoreFiles(t, nativeC, managedC)

	// Drop B while A still requires C. A real Git pull overwrites the
	// teammate's previously committed lock, but not its installation receipts.
	author.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	author.run(t, "deps", "lock", "--format", "json")
	author.run(t, "install", "--locked", "--yes", "--format", "json")
	firstExpected := pmArchiveTargets{"a": {"gemini"}, "c": {"gemini"}}
	author.assertGraph(t, origin, graph, firstExpected)
	author.assertInstalled(t, firstExpected)
	pmPostPullCommit(t, authorRoot, "drop b; a retains c")
	pmPostPullGit(t, teammateRoot, "pull", "-q", "--ff-only")
	pmPostPullNoUntracked(t, teammateRoot)

	mp, lp := teammate.app.dependencyPaths()
	pulled := freshRestoreFiles(t, mp, lp)
	observed := pmEvolutionFiles(t, teammate)
	facts := readDependencyStatusTestFacts(t, teammate.run(t, "deps", "status", "--format", "json"))
	if !facts["not_synced"] {
		t.Fatalf("post-pull stale B receipt was invisibly reported as synced: %v", facts)
	}
	pmArchiveAssertEqual(t, "read-only post-pull status", observed, pmEvolutionFiles(t, teammate))
	downloads := origin.counts()
	teammate.run(t, "install", "--locked", "--yes", "--format", "json")
	teammate.assertGraph(t, origin, graph, firstExpected)
	retained := teammate.assertInstalled(t, firstExpected)
	if retained["c/gemini"].BindingID != initial["c/gemini"].BindingID {
		t.Fatal("dropping B recreated the shared C binding")
	}
	if after, err := os.Stat(nativeC); err != nil || !os.SameFile(nativeCBefore, after) {
		t.Fatalf("dropping B replaced the native C file: %v", err)
	}
	if after, err := os.Stat(managedC); err != nil || !os.SameFile(managedCBefore, after) {
		t.Fatalf("dropping B replaced the managed C payload: %v", err)
	}
	pmArchiveAssertEqual(t, "shared C native and package bytes+mtime", survivorC, freshRestoreFiles(t, nativeC, managedC))
	pmArchiveAssertEqual(t, "unrelated user Skill and data", foreignBefore, freshRestoreFiles(t, foreign, userData))
	pmArchiveAssertEqual(t, "pulled portable agreement", pulled, freshRestoreFiles(t, mp, lp))
	pmArchiveAssertEqual(t, "remove-only replay acquired no new source", downloads, origin.counts())
	pmPostPullAbsent(t, initial["b/gemini"].TargetPath)
	pmPostPullAbsent(t, filepath.Join(nativeRoot, "skills", "b", "SKILL.md"))
	pmPostPullNoUntracked(t, teammateRoot)

	// Drop the final parent. A changed managed C native Skill must not be
	// silently deleted: a failed replay cannot report a successful cleanup.
	author.manifest(t, origin, nil, map[string][]string{"gemini": {}})
	author.run(t, "deps", "lock", "--format", "json")
	author.run(t, "install", "--locked", "--yes", "--format", "json")
	author.assertGraph(t, origin, graph, pmArchiveTargets{})
	author.assertInstalled(t, pmArchiveTargets{})
	pmPostPullCommit(t, authorRoot, "drop final a and orphan c")
	pmPostPullGit(t, teammateRoot, "pull", "-q", "--ff-only")
	pmPostPullNoUntracked(t, teammateRoot)
	finalPortable := freshRestoreFiles(t, mp, lp)
	dependencyWrite(t, nativeC, "user edited managed Skill; preserve on refusal\n")
	// The engine can commit earlier, independently safe removals before a later
	// native refusal; the public result must then report a partial operation.
	// Never equate a nonzero exit with a transaction-wide rollback.
	pendingC := freshRestoreFiles(t, nativeC, managedC)
	downloads = origin.counts()
	_, err = teammate.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("last-consumer removal silently deleted an edited native Skill")
	}
	pmArchiveAssertEqual(t, "failed cleanup kept edited native C and package bytes", pendingC, freshRestoreFiles(t, nativeC, managedC))
	pmArchiveAssertEqual(t, "failed cleanup preserved portable lock", finalPortable, freshRestoreFiles(t, mp, lp))
	pmArchiveAssertEqual(t, "failed cleanup did not fetch a source", downloads, origin.counts())
	pmArchiveAssertEqual(t, "failed cleanup preserved user files", foreignBefore, freshRestoreFiles(t, foreign, userData))
	if got := dependencyRead(t, nativeC); got != "user edited managed Skill; preserve on refusal\n" {
		t.Fatalf("last-consumer refusal overwrote edited Skill: %q", got)
	}
	// Read committed native/owner facts independently of CLI's failed exit.
	// A previously completed removal of A is acceptable only if *both* native
	// and managed A are absent, and its owner receipt is gone. Otherwise A must
	// remain fully intact. A mismatched/unknown partial cannot be credited.
	partial, inspectErr := teammate.app.Installer.Inspect(t.Context())
	if inspectErr != nil || partial.Recovery.Required {
		t.Fatalf("last-consumer refusal left uninspectable or uncertain receipts: %+v %v", partial.Recovery, inspectErr)
	}
	live := map[string]bool{}
	for _, installation := range partial.Installations {
		for _, binding := range installation.Bindings {
			if binding.Owner == nil || binding.ClientID != "gemini" ||
				binding.Owner.Alias != "a" && binding.Owner.Alias != "c" {
				t.Fatalf("foreign/unknown live binding after refusal: %+v", binding)
			}
			if live[binding.Owner.Alias] {
				t.Fatalf("duplicate live binding for %q", binding.Owner.Alias)
			}
			live[binding.Owner.Alias] = true
			if binding.Owner.Alias == "c" && (binding.BindingID != initial["c/gemini"].BindingID ||
				binding.TargetPath != initial["c/gemini"].TargetPath) {
				t.Fatalf("refusal changed shared C ownership: %+v", binding)
			}
		}
	}
	if !live["c"] {
		t.Fatal("edited shared C lost its durable owner receipt")
	}
	for _, receipt := range partial.EnvironmentReceipts {
		if receipt.Alias == "c" && receipt.ClientID == "gemini" {
			continue
		}
		if receipt.Alias == "a" && receipt.ClientID == "gemini" && live["a"] {
			continue
		}
		t.Fatalf("orphan or contradictory live environment receipt: %+v", receipt)
	}
	for _, path := range []string{initial["a/gemini"].TargetPath, filepath.Join(nativeRoot, "skills", "a", "SKILL.md")} {
		if live["a"] {
			if _, statErr := os.Lstat(path); statErr != nil {
				t.Fatalf("owned A retained in state but missing on disk %q: %v", path, statErr)
			}
		} else {
			pmPostPullAbsent(t, path)
		}
	}
	facts = readDependencyStatusTestFacts(t, teammate.run(t, "deps", "status", "--format", "json"))
	if !facts["not_synced"] && !facts["partial"] {
		t.Fatalf("refused last-consumer cleanup was falsely reported synced: %v", facts)
	}
	t.Logf("native drift caused a truthful %s removal with shared C and unrelated user bytes preserved", map[bool]string{true: "partial", false: "preflight"}[!live["a"]])

	// The user explicitly restores the known original owned bytes. One
	// locked retry retires A and C; a second invocation must be a no-op.
	dependencyWrite(t, nativeC, pmArchiveSkill("c"))
	teammate.run(t, "install", "--locked", "--yes", "--format", "json")
	teammate.assertGraph(t, origin, graph, pmArchiveTargets{})
	teammate.assertInstalled(t, pmArchiveTargets{})
	for _, target := range []string{
		initial["a/gemini"].TargetPath, initial["c/gemini"].TargetPath,
		filepath.Join(nativeRoot, "skills", "a", "SKILL.md"), nativeC,
	} {
		pmPostPullAbsent(t, target)
	}
	pmArchiveAssertEqual(t, "final last-owner cleanup preserved foreign content", foreignBefore, freshRestoreFiles(t, foreign, userData))
	pmArchiveAssertEqual(t, "last-owner cleanup did not rewrite portable lock", finalPortable, freshRestoreFiles(t, mp, lp))
	pmPostPullNoUntracked(t, teammateRoot)
	done := pmEvolutionFiles(t, teammate)
	downloads = origin.counts()
	teammate.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "last-owner no-op bytes and metadata", done, pmEvolutionFiles(t, teammate))
	pmArchiveAssertEqual(t, "last-owner no-op source requests", downloads, origin.counts())
}
