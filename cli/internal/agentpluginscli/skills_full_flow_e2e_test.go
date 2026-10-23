package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Every source, state root, native profile and executable boundary belongs to
// this disposable fixture; no installed agent or real user profile is started.
func skillsFullFlowFixture(t *testing.T) (savedJourney, string, []byte) {
	t.Helper()
	repo := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitConfig := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(gitConfig, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gitConfig)
	document := []byte("---\nname: review\ndescription: Review this code and its resources\nmetadata:\n  version: fixture-v1\n---\n# Review\n\n![remote](https://example.invalid/skill-image.png)\n<script>window.__skillExecuted=true</script>\n")
	for _, folder := range []string{"engineering/review", "docs/review"} {
		dir := filepath.Join(repo, filepath.FromSlash(folder))
		if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), document, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scripts", "check.sh"), []byte("#!/bin/sh\nprintf 'resource only\\n'\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	savedJourneyGit(t, repo, "init", "-b", "main")
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: disposable raw Skills end-to-end fixture")
	journey := newSavedJourney(t, repo)
	journey.cli.app.SourceAcquirer = skillsource.Acquirer{Acquirer: journey.source.Acquirer}
	return journey, repo, document
}

func TestSkillsFullCLIEndToEnd(t *testing.T) {
	journey, repo, raw := skillsFullFlowFixture(t)
	journey.run(t, "skills", "add", "acme/skills", "--path", "engineering/review", "--target", "codex", "--no-ui", "--yes", "--format", "json")
	requirements, locked := journey.intent(t)
	if len(requirements.Dependencies) != 1 || len(locked.Packages) != 1 {
		t.Fatalf("saved raw Skill: %+v %+v", requirements, locked)
	}
	pin := locked.Packages[0]
	if pin.Source.Git == nil || pin.Source.Git.Kind != environment.SkillKind || pin.Source.Git.Normalizer != "agent-skill-package/v1" || pin.Source.Git.RawTreeDigest == "" {
		t.Fatalf("missing Skill provenance: %+v", pin)
	}
	snapshot, err := journey.cli.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(snapshot.Root, "skills", "review", "SKILL.md"))
	if err != nil || string(got) != string(raw) {
		t.Fatalf("raw Skill bytes changed: %q %v", got, err)
	}
	script, err := os.Stat(filepath.Join(snapshot.Root, "skills", "review", "scripts", "check.sh"))
	if err != nil || script.Mode()&0100 == 0 {
		t.Fatalf("resource executable mode missing: %v %v", script, err)
	}
	state := journey.state(t)
	if len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("expected managed receipt: %+v", state.EnvironmentReceipts)
	}
	journey.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	after := journey.state(t)
	if after.EnvironmentReceipts[0].InstallationID != state.EnvironmentReceipts[0].InstallationID {
		t.Fatal("same pin recreated installation")
	}
	// Replay into genuinely fresh state/store/profiles after upstream advances.
	// Only the saved intent and lock cross this boundary, never installed bytes.
	if err := os.WriteFile(filepath.Join(repo, "engineering", "review", "SKILL.md"), []byte(strings.ReplaceAll(string(raw), "fixture-v1", "fixture-v2")), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: advance upstream after immutable Skill pin")
	fresh := newSavedJourney(t, repo)
	fresh.cli.app.SourceAcquirer = skillsource.Acquirer{Acquirer: fresh.source.Acquirer}
	mp, lp := journey.cli.app.dependencyPaths()
	fmp, flp := fresh.cli.app.dependencyPaths()
	for source, destination := range map[string]string{mp: fmp, lp: flp} {
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	fresh.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	_, restoredLock := fresh.intent(t)
	if restoredLock.Packages[0].TreeDigest != pin.TreeDigest || restoredLock.Packages[0].Source.Git.Commit != pin.Source.Git.Commit {
		t.Fatal("fresh restore advanced the immutable pin")
	}
	restored, err := fresh.cli.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	restoredBody, err := os.ReadFile(filepath.Join(restored.Root, "skills", "review", "SKILL.md"))
	if err != nil || string(restoredBody) != string(raw) {
		t.Fatalf("cold CLI restore did not reproduce pinned raw bytes: %v", err)
	}
	// Repair and removal must retain the same saved lifecycle and ownership rules
	// as standard packages, including when a managed projection has drifted.
	binding := savedJourneyBinding(t, fresh.state(t), "codex")
	drift := filepath.Join(binding.TargetLocator, "fixture-drift.txt")
	if err := os.WriteFile(drift, []byte("preserve through explicit repair"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fresh.cli.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("locked sync silently overwrote user drift")
	}
	fresh.run(t, "repair", fresh.state(t).EnvironmentReceipts[0].InstallationID, "--target", "codex", "--format", "json")
	fresh.run(t, "remove", pin.Alias, "--external-uninstalled", "--format", "json")
	remaining, _ := fresh.intent(t)
	if len(remaining.Dependencies) != 0 || len(fresh.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("raw Skill removal left saved requirements or ownership receipts")
	}
}

// Playwright starts this test as an ordinary Go test subprocess. It exercises
// the production App.StartSkillsPicker and embedded assets with real Git/store/
// filesystem transactions. Only external native-client commands use the existing
// disposable fixture runner. This function is never compiled into the CLI.
func TestSkillsBrowserFixture(t *testing.T) {
	ready := os.Getenv("UAP_SKILLS_BROWSER_READY")
	if ready == "" {
		t.Skip("started explicitly by the live bridge browser E2E")
	}
	if !filepath.IsAbs(ready) {
		t.Fatal("fixture readiness path must be absolute")
	}
	if !skillspicker.Bundled() {
		t.Fatal("run npm run bundle:go before live bridge E2E")
	}
	journey, _, _ := skillsFullFlowFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := journey.cli.app.StartSkillsPicker(ctx, SkillsPickerRequest{Input: "acme/skills"})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	payload, err := json.Marshal(map[string]string{"url": server.URL()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ready, payload, 0600); err != nil {
		t.Fatal(err)
	}
	timeout := time.NewTimer(4 * time.Minute)
	defer timeout.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatal("browser fixture did not finish")
		case <-server.Done():
			t.Fatal("picker session closed before browser verified installation")
		case <-tick.C:
			body, readErr := os.ReadFile(ready + ".done")
			if os.IsNotExist(readErr) {
				continue
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if strings.TrimSpace(string(body)) != "verify" {
				t.Fatal("browser aborted before verifying")
			}
			state := journey.state(t)
			if len(state.Installations) != 2 || len(state.EnvironmentReceipts) != 4 {
				t.Fatalf("two Skills across two targets not committed: installations=%d receipts=%d", len(state.Installations), len(state.EnvironmentReceipts))
			}
			_, locked := journey.intent(t)
			if len(locked.Packages) != 2 {
				t.Fatalf("expected two locked Skills, got %d", len(locked.Packages))
			}
			for _, pin := range locked.Packages {
				if pin.Source.Git == nil || pin.Source.Git.Kind != environment.SkillKind {
					t.Fatal("browser installed an ordinary package instead of raw Skills")
				}
			}
			journey.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
			return
		}
	}
}
