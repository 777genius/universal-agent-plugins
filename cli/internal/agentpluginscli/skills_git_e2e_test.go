package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Git network requests use real TLS, git-http-backend and the user's native
// credential-helper interface. Native agent responses remain inert fixtures.
func TestStandaloneGitSkillPrivateHTTPSLifecycle(t *testing.T) {
	transport := newDirectTransportFixture(t, "private-git", true)
	j := newSavedJourney(t, transport.work)
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir()}
	endpoint := transport.server.URL + "/repo.git"
	args := []string{"skills", "add", endpoint, "--path", "skills/demo", "--target", "codex,claude", "--no-ui", "--yes", "--format", "json"}
	j.run(t, args...)
	requirements, locked := j.intent(t)
	pin := locked.Packages[0]
	if pin.Source.Git.URL != endpoint || pin.Source.Git.Repository != "" || pin.SourceKey != sourcespec.GitSkillKey(endpoint, "demo", "skills/demo") || len(pin.Targets) != 2 {
		t.Fatalf("generic Skill identity lost: %+v", pin)
	}
	before := j.state(t).Installations[0]
	j.run(t, args...)
	_, repeated := j.intent(t)
	if !reflect.DeepEqual(locked, repeated) {
		t.Fatal("re-add changed the saved pin")
	}
	dependencyWrite(t, filepath.Join(transport.work, "skills", "demo", "SKILL.md"), "---\nname: demo\ndescription: Updated fixture\n---\nChanged content.\n")
	dependencyGit(t, transport.work, "add", ".")
	dependencyGit(t, transport.work, "commit", "-m", "advance private Skill")
	dependencyGit(t, transport.work, "push", transport.remote, "HEAD:refs/heads/main")
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	loaded, err := materializeLockedPackage(t.Context(), j.cli.app, pin, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(loaded.envelope.SnapshotRoot, "skills", "demo", "SKILL.md"))
	_ = loaded.cleanup()
	if err != nil || strings.Contains(string(body), "Changed content") {
		t.Fatalf("cold replay followed moved branch: %q %v", body, err)
	}
	j.run(t, "update", "demo", "--format", "json")
	updatedRequirements, updated := j.intent(t)
	after := j.state(t).Installations[0]
	if updated.Packages[0].Source.Git.Commit == pin.Source.Git.Commit || before.InstallationID != after.InstallationID || before.Source.SourceBindingID != after.Source.SourceBindingID || !reflect.DeepEqual(requirements, updatedRequirements) {
		t.Fatal("update changed source ownership or failed to advance the immutable pin")
	}
	transport.mu.Lock()
	authenticated, publisherReads := transport.gitAuth, transport.downloads
	transport.mu.Unlock()
	if authenticated == 0 || publisherReads != 0 {
		t.Fatalf("wrong HTTPS transport: authenticated=%d publisher=%d", authenticated, publisherReads)
	}
}

func TestStandaloneGitSkillDeniedAccessLeavesEnvironmentUntouched(t *testing.T) {
	transport := newDirectTransportFixture(t, "private-git", true)
	j := newSavedJourney(t, transport.work)
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir()}
	dependencyWrite(t, filepath.Join(os.Getenv("HOME"), ".gitconfig"), "[credential]\n helper =\n")
	before := savedJourneyTree(t, j.cli.root)
	var observedGitExit bool
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), Runner: &dependencyGitDiagnosticRunner{delegate: sourceacquisition.OSRunner{}, report: func(_, class, _ string) {
		observedGitExit = observedGitExit || class == "git_exit"
	}}}
	_, _, err := j.cli.execute(false, "skills", "add", transport.server.URL+"/repo.git", "--skill", "demo", "--target", "codex", "--yes", "--format", "json")
	if err == nil || strings.Contains(err.Error(), "disposable-fixture-token") {
		t.Fatalf("private unauthenticated source accepted or leaked credentials: %v", err)
	}
	if !observedGitExit || errors.Is(err, context.Canceled) {
		t.Fatalf("authentication refusal was not exercised by native Git: %v", err)
	}
	if differences := savedJourneyDifferences(before, savedJourneyTree(t, j.cli.root)); differences != "" {
		t.Fatalf("refused source mutated environment: %s", differences)
	}
}

func TestGenericGitSkillColdDigestRefusal(t *testing.T) {
	transport := newDirectTransportFixture(t, "git", false)
	acquirer := sourceacquisition.Acquirer{TempRoot: t.TempDir()}
	git := sourcespec.Git{URL: transport.server.URL + "/repo.git", Path: "skills/demo"}
	resolved, err := acquirer.ResolveGitSource(context.Background(), git)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := acquirer.AcquireGitSkill(t.Context(), git, resolved.Git.Commit, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = packagedigest.Remove(raw)
	if _, err = acquirer.AcquireGitSkill(t.Context(), git, resolved.Git.Commit, "sha256:"+strings.Repeat("0", 64)); err == nil {
		t.Fatal("raw digest mismatch was accepted")
	}
	if !isWebsiteSkillInput("https://publisher.example/skills") || isWebsiteSkillInput(git.URL) {
		t.Fatal("Git clone and publisher URL routes collapsed")
	}
}
