package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func localSkillJourney(t *testing.T) (savedJourney, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "review")
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: review\ndescription: Review local code\n---\nRead scripts/check.sh.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "check.sh"), []byte("#!/bin/sh\nexit 97\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return newSavedJourney(t, t.TempDir()), root
}

func localResource(t *testing.T, app App, pin environment.LockedPackage) string {
	t.Helper()
	loaded, err := materializeLockedPackage(context.Background(), app, pin, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	body, err := os.ReadFile(filepath.Join(loaded.envelope.SnapshotRoot, "skills", "review", "scripts", "check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertLocalInstalledResource(t *testing.T, j savedJourney, expected string) {
	t.Helper()
	state := j.state(t)
	for _, target := range []string{"codex", "claude"} {
		binding := savedJourneyBinding(t, state, target)
		root := binding.TargetLocator
		if target == "claude" {
			root = filepath.Join(root, ".agentplugins-runtime")
		}
		p := filepath.Join(root, "skills", "review", "scripts", "check.sh")
		if body, err := os.ReadFile(p); err != nil || string(body) != expected {
			t.Fatalf("%s installed resource bytes differ: %q %v", target, body, err)
		}
	}
}

func TestLocalSkillCLISavedSnapshotLifecycle(t *testing.T) {
	j, root := localSkillJourney(t)
	args := []string{"skills", "add", root, "--target", "codex,claude", "--yes", "--format", "json"}
	j.run(t, append(append([]string(nil), args...), "--dry-run")...)
	mp, lp := j.cli.app.dependencyPaths()
	for _, p := range []string{mp, lp, j.cli.app.EnvironmentStoreRoot} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("dry run wrote %s: %v", p, err)
		}
	}
	j.run(t, args...)
	reqs, lock := j.intent(t)
	pin := lock.Packages[0]
	if reqs.Dependencies["review"].Local == nil || pin.Source.Local == nil || pin.Source.Local.Normalizer != environment.SkillNormalizerV1 || pin.Source.Local.RawTreeDigest == pin.TreeDigest {
		t.Fatalf("missing saved local provenance: %+v %+v", reqs, pin)
	}
	if runtime.GOOS != "windows" && !slices.Contains(pin.ExecutableFiles, "skills/review/scripts/check.sh") {
		t.Fatal("executable resource inventory lost", pin.ExecutableFiles)
	}
	before := j.state(t)
	if len(before.EnvironmentReceipts) != 2 {
		t.Fatal("local add bypassed environment receipts", before.EnvironmentReceipts)
	}
	oldBytes := localResource(t, j.cli.app, pin)
	assertLocalInstalledResource(t, j, oldBytes)
	newBytes := "#!/bin/sh\nexit 98\n"
	if err := os.WriteFile(filepath.Join(root, "scripts", "check.sh"), []byte(newBytes), 0700); err != nil {
		t.Fatal(err)
	}
	j.run(t, args...)
	_, repeated := j.intent(t)
	if !environment.SamePin(pin, repeated.Packages[0]) || localResource(t, j.cli.app, pin) != oldBytes {
		t.Fatal("re-add adopted mutable local bytes")
	}
	assertLocalInstalledResource(t, j, oldBytes)
	j.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	j.run(t, "update", "review", "--format", "json")
	_, updated := j.intent(t)
	next := updated.Packages[0]
	after := j.state(t)
	if pin.SourceKey != next.SourceKey || pin.TreeDigest == next.TreeDigest || localResource(t, j.cli.app, next) != newBytes || before.Installations[0].InstallationID != after.Installations[0].InstallationID || before.Installations[0].Source.SourceBindingID != after.Installations[0].Source.SourceBindingID {
		t.Fatal("explicit local update changed ownership or failed to snapshot source")
	}
	assertLocalInstalledResource(t, j, newBytes)
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	// The verified artifact owns replay and each binding's recorded repair.
	savedRawSkillDropOwnedFixture(t, j, savedJourneyBinding(t, after, "codex").TargetLocator)
	j.run(t, "repair", "review", "--target", "codex", "--format", "json")
	assertLocalInstalledResource(t, j, newBytes)
	j.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	exchange := filepath.Join(t.TempDir(), "exchange")
	j.run(t, "deps", "export", exchange, "--yes", "--format", "json")
	j.run(t, "deps", "import", exchange, "--install", "--yes", "--format", "json")
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeLockedPackage(context.Background(), j.cli.app, next, false, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("cold missing local source did not refuse: %v", err)
	}
	j.run(t, "remove", "review", "--target", "codex,claude", "--format", "json")
	reqs, lock = j.intent(t)
	if len(reqs.Dependencies) != 0 || len(lock.Packages) != 0 {
		t.Fatal("remove left saved local declaration", reqs, lock)
	}
}

func TestLocalSkillColdReplayVerifiesRawAndNormalizedBytes(t *testing.T) {
	j, root := localSkillJourney(t)
	j.run(t, "skills", "add", root, "--target", "codex", "--yes", "--format", "json")
	_, lock := j.intent(t)
	pin := lock.Packages[0]
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	if localResource(t, j.cli.app, pin) == "" {
		t.Fatal("exact local source did not restore cold artifact")
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "check.sh"), []byte("new local bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := materializeLockedPackage(context.Background(), j.cli.app, pin, false, nil); err == nil || !strings.Contains(err.Error(), "changed since uap.lock") {
		t.Fatalf("changed cold source did not refuse: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := j.cli.app.acquireLockedLocalSkill(ctx, pin); !errors.Is(err, context.Canceled) {
		t.Fatalf("local acquisition hid cancellation: %v", err)
	}
}

func TestLocalSkillWarmStoreReplaysForeignOSCoordinates(t *testing.T) {
	j, root := localSkillJourney(t)
	ctx := context.Background()
	foreign := "C:/not-on-this-host/review"
	if runtime.GOOS == "windows" {
		foreign = "/not-on-this-host/review"
	}
	if _, err := nativeLocalSkillPath(foreign); err == nil {
		t.Fatal("foreign source coordinates reopened as a native path")
	}
	raw, err := j.cli.app.SourceAcquirer.AcquireLocal(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	key := sourcespec.LocalSkillKey(foreign, "review")
	snapshot, err := skillsource.Normalize(ctx, raw, "review", key)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	pin := environment.LockedPackage{Alias: "review", SourceKey: key, DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: environment.LockedSource{Local: &environment.LockedLocalSource{Path: foreign, Skill: "review", RawTreeDigest: raw.TreeDigest, Normalizer: environment.SkillNormalizerV1}}}
	snapshot.Source = localSkillSnapshotSource(pin)
	envelope, err := j.cli.app.loadStandardEnvelope(ctx, domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source})
	if err != nil {
		t.Fatal(err)
	}
	pin.ManifestDigest = envelope.ManifestDigest
	if _, err := j.cli.app.dependencyStore().Put(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if localResource(t, j.cli.app, pin) == "" {
		t.Fatal("warm replay consulted incompatible original source path")
	}
}

func TestLocalSkillSelectionRefusalsDoNotWrite(t *testing.T) {
	for _, flags := range [][]string{{"--ref", "main"}, {"--path", "review"}, {"--skill", "other"}, {"--no-save"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			j, root := localSkillJourney(t)
			args := append([]string{"skills", "add", root, "--target", "codex", "--yes", "--format", "json"}, flags...)
			if _, _, err := j.cli.execute(false, args...); err == nil {
				t.Fatal("accepted ambiguous local selection")
			}
			mp, _ := j.cli.app.dependencyPaths()
			if _, err := os.Stat(mp); !os.IsNotExist(err) || len(j.state(t).Installations) != 0 {
				t.Fatal("refused selection wrote intent or profile")
			}
		})
	}
}

func TestLocalSkillRelativeDirectoryUsesCanonicalSnapshotIdentity(t *testing.T) {
	j, root := localSkillJourney(t)
	t.Chdir(filepath.Dir(root))
	j.run(t, "skills", "add", "./review", "--target", "codex", "--yes", "--format", "json")
	reqs, _ := j.intent(t)
	if reqs.Dependencies["review"].Local.Path != filepath.ToSlash(root) {
		t.Fatal("relative source was saved against a future working directory")
	}
}

func TestLocalSkillPathHazardsRefuseBeforePublication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation needs an explicit host privilege")
	}
	for _, hazard := range []string{"root-link", "resource-escape", "document-link"} {
		t.Run(hazard, func(t *testing.T) {
			j, root := localSkillJourney(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.WriteFile(outside, []byte("private outside bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			switch hazard {
			case "root-link":
				link := filepath.Join(t.TempDir(), "review")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			case "resource-escape":
				if err := os.Symlink(outside, filepath.Join(root, "resource")); err != nil {
					t.Fatal(err)
				}
			case "document-link":
				if err := os.Rename(filepath.Join(root, "SKILL.md"), filepath.Join(root, "document.md")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("document.md", filepath.Join(root, "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := j.cli.execute(false, "skills", "add", root, "--target", "codex", "--yes", "--format", "json"); err == nil {
				t.Fatal("accepted unsafe local Skill source", hazard)
			}
			mp, _ := j.cli.app.dependencyPaths()
			if _, err := os.Stat(mp); !os.IsNotExist(err) || len(j.state(t).Installations) != 0 {
				t.Fatal("unsafe source published an installation")
			}
		})
	}
}
