package agentpluginscli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Real TLS and the full command/environment/standard loader/transaction path;
// only the native client runner and security scanner use disposable fixtures.
func TestWebsiteSkillsSavedLifecycleAndColdRefusal(t *testing.T) {
	var mu sync.Mutex
	version := 1
	indexAvailable := true
	resourceAvailable := true
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("publisher received credentials")
		}
		switch r.URL.Path {
		case "/scoped/.well-known/agent-skills/index.json":
			if !indexAvailable {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"pdf","description":"Read PDFs","files":["SKILL.md","references/note.txt"]}]}`)
		case "/scoped/.well-known/agent-skills/pdf/SKILL.md":
			_, _ = fmt.Fprint(w, "---\nname: pdf\ndescription: Read PDF documents\n---\nRead references/note.txt.\n")
		case "/scoped/.well-known/agent-skills/pdf/references/note.txt":
			if !resourceAvailable {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintf(w, "resource version %d", version)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	makeJourney := func() savedJourney {
		j := newSavedJourney(t, t.TempDir())
		a := sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
		j.cli.app.SourceAcquirer = a
		j.cli.app.SourceAcquirerForDryRun = nil
		return j
	}
	j := makeJourney()
	j.run(t, "skills", "add", server.URL+"/scoped", "--skill", "pdf", "--target", "codex", "--no-ui", "--yes", "--format", "json")
	req, lock := j.intent(t)
	pin := lock.Packages[0]
	if req.Dependencies["pdf"].Web == nil || pin.Source.Web == nil || len(pin.Source.Web.Files) != 2 || pin.Source.Web.RawTreeDigest == pin.TreeDigest {
		t.Fatal("website source pin missing", lock)
	}
	before := j.state(t)
	addedBinding := savedJourneyBinding(t, before, "codex")
	if got := dependencyRead(t, filepath.Join(addedBinding.TargetLocator, "skills", "pdf", "references", "note.txt")); got != "resource version 1" {
		t.Fatal("installed resource lost", got)
	}
	repairs, err := buildDependencyRepairs(context.Background(), j.cli.app, before, before.Installations[0], []domain.ClientID{domain.ClientCodex})
	if err != nil || repairs[0].Package.Source.Web == nil {
		t.Fatal("repair provenance missing", err)
	}
	j.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	exchange := filepath.Join(t.TempDir(), "agreement")
	j.run(t, "deps", "export", exchange, "--yes", "--format", "json")
	// Cold import uses recorded file coordinates even when discovery is gone.
	mu.Lock()
	indexAvailable = false
	mu.Unlock()
	cold := makeJourney()
	cold.run(t, "deps", "import", exchange, "--install", "--yes", "--format", "json")
	_, restored := cold.intent(t)
	if restored.Packages[0].TreeDigest != pin.TreeDigest {
		t.Fatal("cold import changed pin")
	}
	coldState := cold.state(t)
	binding := savedJourneyBinding(t, coldState, "codex")
	resource := filepath.Join(binding.TargetLocator, "skills", "pdf", "references", "note.txt")
	if got := dependencyRead(t, resource); got != "resource version 1" {
		t.Fatal("cold projection lost supporting resource", got)
	}
	dependencyWrite(t, filepath.Join(binding.TargetLocator, "drift.txt"), "local drift")
	if err := os.RemoveAll(cold.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	cold.run(t, "repair", coldState.Installations[0].InstallationID, "--target", "codex", "--format", "json")
	if got := dependencyRead(t, resource); got != "resource version 1" {
		t.Fatal("cold repair changed pinned resource", got)
	}
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "drift.txt")); !os.IsNotExist(err) {
		t.Fatal("repair did not replace drift", err)
	}
	// Explicit update follows the publisher, while the old per-client repair and
	// exported agreement continue to require their original content hashes.
	mu.Lock()
	indexAvailable = true
	version = 2
	mu.Unlock()
	// With the local store gone, locked replay must fetch the recorded files,
	// refuse the changed publisher bytes and leave the lock, state and install intact.
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	installedResource := filepath.Join(addedBinding.TargetLocator, "skills", "pdf", "references", "note.txt")
	_, replayLockPath := j.cli.app.dependencyPaths()
	replayState, replayLock := dependencyRead(t, j.cli.store.Path), dependencyRead(t, replayLockPath)
	if _, _, err := j.cli.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Fatalf("locked replay accepted changed publisher bytes: %v", err)
	}
	if dependencyRead(t, j.cli.store.Path) != replayState || dependencyRead(t, replayLockPath) != replayLock || dependencyRead(t, installedResource) != "resource version 1" {
		t.Fatal("refused locked replay changed state, lock or installed resource")
	}
	j.run(t, "update", "pdf", "--format", "json")
	_, updated := j.intent(t)
	if updated.Packages[0].TreeDigest == pin.TreeDigest || updated.Packages[0].SourceKey != pin.SourceKey {
		t.Fatal("update failed to preserve identity and advance content")
	}
	if err = os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	savedState := dependencyRead(t, j.cli.store.Path)
	_, savedLockPath := j.cli.app.dependencyPaths()
	savedLock := dependencyRead(t, savedLockPath)
	_, err = materializeLockedPackage(context.Background(), j.cli.app, *repairs[0].Package, false, nil)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "digest") && !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Fatalf("old repair silently upgraded or wrong refusal: %v", err)
	}
	if dependencyRead(t, j.cli.store.Path) != savedState || dependencyRead(t, savedLockPath) != savedLock {
		t.Fatal("historical refusal changed current state or lock")
	}
	refused := makeJourney()
	mp, lp := refused.cli.app.dependencyPaths()
	_, _, err = refused.cli.execute(false, "deps", "import", exchange, "--install", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("changed publisher bytes accepted for historical cold import")
	}
	for _, p := range []string{mp, lp} {
		if _, e := os.Stat(p); !os.IsNotExist(e) {
			t.Fatalf("failed import published %s: %v", p, e)
		}
	}
	if len(refused.state(t).Installations) != 0 {
		t.Fatal("failed import changed profiles")
	}
	// A publisher that deleted the pinned resource must fail the same way, with no
	// published profile or lock.
	mu.Lock()
	resourceAvailable = false
	mu.Unlock()
	missing := makeJourney()
	missingRequirementsPath, missingLockPath := missing.cli.app.dependencyPaths()
	if _, _, err = missing.cli.execute(false, "deps", "import", exchange, "--install", "--yes", "--format", "json"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("deleted historical bytes accepted for cold import: %v", err)
	}
	for _, p := range []string{missingRequirementsPath, missingLockPath} {
		if _, e := os.Stat(p); !os.IsNotExist(e) {
			t.Fatalf("failed import published %s: %v", p, e)
		}
	}
	if len(missing.state(t).Installations) != 0 {
		t.Fatal("failed import changed profiles")
	}
}

// A staged artifact must not satisfy another pin whose file provenance differs,
// even when the raw and normalized tree digests have been copied unchanged.
func TestWebsiteSnapshotCacheIncludesFilePins(t *testing.T) {
	pin := environment.LockedPackage{SourceKey: "https-skill:https://publisher.example:pdf", DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), Source: environment.LockedSource{Web: &environment.LockedWebSource{URL: "https://publisher.example", Skill: "pdf", IndexSHA256: strings.Repeat("b", 64), RawTreeDigest: "sha256:" + strings.Repeat("c", 64), Normalizer: environment.SkillNormalizerV1, Files: []domain.WebSkillFile{{Path: "SKILL.md", URL: "https://publisher.example/.well-known/skills/pdf/SKILL.md", SHA256: strings.Repeat("d", 64)}}}}}
	original, err := lockedDependencySnapshotKey(pin)
	if err != nil {
		t.Fatal(err)
	}
	pin.Source.Web.Files[0].SHA256 = strings.Repeat("e", 64)
	altered, err := lockedDependencySnapshotKey(pin)
	if err != nil {
		t.Fatal(err)
	}
	if original == altered {
		t.Fatal("altered file pin can reuse a previously staged artifact")
	}
}

func TestWebsiteDispatchPreservesGitHubHostCase(t *testing.T) {
	for _, input := range []string{"https://GitHub.com/acme/skills", "https://RAW.GITHUBUSERCONTENT.COM/acme/skills/main/SKILL.md", "https://github.com/acme/skills"} {
		if isWebsiteSkillInput(input) {
			t.Fatalf("GitHub input dispatched as website: %s", input)
		}
	}
	if !isWebsiteSkillInput("https://publisher.example/skills") {
		t.Fatal("HTTPS publisher not dispatched")
	}
}
