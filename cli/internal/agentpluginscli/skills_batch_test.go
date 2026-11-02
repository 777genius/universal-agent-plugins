package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func skillJourney(t *testing.T) (savedJourney, string, string) {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, "skills", "pdf")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: pdf\ndescription: Process PDF documents\n---\nRead references/note.txt.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "references"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "references", "note.txt"), []byte("original resource"), 0644); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "init", "-b", "main")
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "fixture")
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	j := newSavedJourney(t, repo)
	j.cli.app.SourceAcquirer = skillsource.Acquirer{Acquirer: j.source.Acquirer}
	return j, repo, sha
}

func TestSkillsBatchPrepareApplyMoveAndColdReplay(t *testing.T) {
	j, repo, sha := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}, TargetIDs: []string{"codex"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	mp, lp := j.cli.app.dependencyPaths()
	if _, err := os.Stat(mp); !os.IsNotExist(err) {
		t.Fatal("prepare wrote manifest", err)
	}
	result, err := plan.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" {
		t.Fatal(result)
	}
	beforeState := j.state(t)
	before := beforeState.Installations[0]
	oldRepairs, err := buildDependencyRepairs(ctx, j.cli.app, beforeState, before, []domain.ClientID{domain.ClientCodex})
	if err != nil {
		t.Fatal(err)
	}
	if oldRepairs[0].Package.Source.Git.RawTreeDigest == "" {
		t.Fatal("per-client Skill provenance was not persisted")
	}
	oldPhysical := savedJourneyBinding(t, beforeState, "codex").PhysicalArtifact
	req, lock := j.intent(t)
	pin := lock.Packages[0]
	if pin.Source.Git.Kind != "skill" || pin.Source.Git.RawTreeDigest == pin.TreeDigest {
		t.Fatal("raw/artifact identity missing", pin)
	}
	if req.Dependencies["pdf"].Git.Selector.Kind != environment.SelectorRef {
		t.Fatal(req)
	}
	// Move the exact same Skill, then update only its explicit moving selector.
	if err := os.MkdirAll(filepath.Join(repo, "moved"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(repo, "skills", "pdf"), filepath.Join(repo, "moved", "pdf")); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", "-A")
	savedJourneyGit(t, repo, "commit", "-m", "move")
	j.run(t, "update", "pdf", "--format", "json")
	afterState := j.state(t)
	after := afterState.Installations[0]
	if savedJourneyBinding(t, afterState, "codex").PhysicalArtifact != oldPhysical {
		t.Fatal("relocation changed physical artifact")
	}
	if before.InstallationID != after.InstallationID || before.Source.SourceBindingID != after.Source.SourceBindingID || before.DeclaredName != after.DeclaredName {
		t.Fatal("relocation changed installer identity")
	}
	req, lock = j.intent(t)
	if lock.Packages[0].Source.Git.ResolvedSkillPath != "moved/pdf" || req.Dependencies["pdf"].Git.Path != "skills/pdf" {
		t.Fatal(lock)
	}
	// A missing store must reconstruct the selected moved directory at the lock SHA,
	// even after the tracking branch has advanced again.
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	loaded, err := materializeLockedPackage(ctx, j.cli.app, lock.Packages[0], false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	body, err := os.ReadFile(filepath.Join(loaded.envelope.SnapshotRoot, "skills", "pdf", "references", "note.txt"))
	if err != nil || string(body) != "original resource" {
		t.Fatal(string(body), err)
	}
	oldLoaded, err := materializeLockedPackage(ctx, j.cli.app, *oldRepairs[0].Package, false, nil)
	if err != nil {
		t.Fatalf("old per-client cold repair: %v", err)
	}
	defer func() { _ = oldLoaded.cleanup() }()
	if oldLoaded.envelope.Source.PackageSubpath != "skills/pdf" || oldLoaded.envelope.Source.ResolvedRevision != sha {
		t.Fatal("repair consulted current desired source")
	}
	if _, err = os.Stat(lp); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsBatchRejectsStaleAndUnresolvedBeforeEffects(t *testing.T) {
	j, _, sha := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf"}}, TargetIDs: []string{"codex"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	mp, _ := j.cli.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(mp), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, []byte("external change"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Apply(ctx); !errors.Is(err, ErrSkillsPlanStale) {
		t.Fatal("expected stale refusal", err)
	}
	if len(j.state(t).Installations) != 0 {
		t.Fatal("stale plan changed profiles")
	}
	if err := os.Remove(mp); err != nil {
		t.Fatal(err)
	}
	request.Selections = append(request.Selections, SkillsSelection{Repository: "acme/tools", Commit: sha, Path: "missing", Name: "missing", CandidateID: "missing-candidate"})
	if _, err = j.cli.app.PrepareSkills(ctx, request); err == nil {
		t.Fatal("accepted unresolved batch")
	}
	var selectionError *SkillsSelectionError
	if !errors.As(err, &selectionError) || selectionError.CandidateID != "missing-candidate" || selectionError.Unwrap() == nil {
		t.Fatal("failed source lost candidate context", err)
	}
	if _, err = os.Stat(mp); !os.IsNotExist(err) {
		t.Fatal("failed batch wrote manifest", err)
	}
}

// A partial rollout has two accepted client revisions. Repair must restore the
// selected client's own bytes even after lock advancement and complete store loss.
type skillFailSecondPrepare struct {
	inner *dependencyInstaller
	calls int
}

func (f *skillFailSecondPrepare) Prepare(ctx context.Context, action environment.PreparedBinding) (*installer.PreparedOperation, error) {
	f.calls++
	if f.calls == 2 {
		return nil, errors.New("sandbox injected second-target failure")
	}
	return f.inner.Prepare(ctx, action)
}
func (f *skillFailSecondPrepare) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	return f.inner.Apply(ctx, prepared)
}

func TestSkillsPartialRevisionColdRepairAfterLockAdvance(t *testing.T) {
	j, repo, oldSHA := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: oldSHA, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}, TargetIDs: []string{"codex", "claude"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Apply(ctx); err != nil {
		plan.Close()
		t.Fatal(err)
	}
	plan.Close()
	before := j.state(t)
	original := before.Installations[0]
	if err = os.MkdirAll(filepath.Join(repo, "moved"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(filepath.Join(repo, "skills", "pdf"), filepath.Join(repo, "moved", "pdf")); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(repo, "moved", "pdf", "references", "note.txt"), []byte("new resource"), 0644); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", "-A")
	savedJourneyGit(t, repo, "commit", "-m", "move and change selected resource")
	newSHA := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	// Lock advancement alone does not touch profiles. Apply the normal complete
	// prepared batch with a failure injected only at its second SDK target.
	j.run(t, "deps", "update", "pdf", "--format", "json")
	review, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer review.Close()
	actions := review.request.prepared
	if len(actions) != 2 {
		t.Fatalf("expected both alias targets, got %+v", actions)
	}
	newClient, oldClient := actions[0].ClientID, actions[1].ClientID
	oldBinding := savedJourneyBinding(t, before, oldClient)
	injected := &skillFailSecondPrepare{inner: &dependencyInstaller{app: j.cli.app, opts: &options{}, detected: review.request.detected, beforeApply: review.request.checkInputs}}
	results, err := (environment.Coordinator{Mutator: injected}).ApplySync(ctx, environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, ManifestDigest: review.request.lock.ManifestDigest, Bindings: actions}, true)
	if err == nil || len(results) != 1 || injected.calls != 2 {
		t.Fatalf("expected exactly one accepted target before injected failure: results=%+v error=%v", results, err)
	}
	advanced := j.state(t)
	updated := savedJourneyBinding(t, advanced, newClient)
	older := savedJourneyBinding(t, advanced, oldClient)
	_, lock := j.intent(t)
	if updated.PackageRevision.ResolvedRevision != newSHA || older.PackageRevision.ResolvedRevision != oldSHA || lock.Packages[0].Source.Git.Commit != newSHA {
		t.Fatal("fixture did not establish separate accepted target pins")
	}
	if advanced.Installations[0].InstallationID != original.InstallationID || advanced.Installations[0].Source.SourceBindingID != original.Source.SourceBindingID || advanced.Installations[0].DeclaredName != original.DeclaredName || older.PhysicalArtifact != oldBinding.PhysicalArtifact {
		t.Fatal("partial relocation changed stable installation identity")
	}
	if updated.PackageRevision.TreeDigest == older.PackageRevision.TreeDigest {
		t.Fatal("fixture resource change did not change artifact digest")
	}
	_, lockPath := j.cli.app.dependencyPaths()
	lockedBytes, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	drift := filepath.Join(older.TargetLocator, "user-drift.txt")
	if err = os.WriteFile(drift, []byte("explicitly repair this drift"), 0644); err != nil {
		t.Fatal(err)
	}
	j.run(t, "repair", original.InstallationID, "--target", oldClient, "--format", "json")
	repaired := j.state(t)
	repairedOlder := savedJourneyBinding(t, repaired, oldClient)
	repairedUpdated := savedJourneyBinding(t, repaired, newClient)
	if repairedOlder.PackageRevision.ResolvedRevision != oldSHA || repairedOlder.PackageRevision.TreeDigest != oldBinding.PackageRevision.TreeDigest || repairedOlder.PackageRevision.Skill.ResolvedPath != "skills/pdf" {
		t.Fatal("repair promoted the older client to current lock")
	}
	if repairedUpdated.PackageRevision.ResolvedRevision != newSHA || repaired.Installations[0].Source.ResolvedRevision != newSHA {
		t.Fatal("repair rewound the desired or sibling revision")
	}
	if repairedOlder.PhysicalArtifact != oldBinding.PhysicalArtifact || repairedOlder.ClientBindingID != oldBinding.ClientBindingID {
		t.Fatal("repair replaced accepted target identity")
	}
	if _, err = os.Stat(drift); !os.IsNotExist(err) {
		t.Fatal("explicit repair did not remove managed drift", err)
	}
	body, err := os.ReadFile(filepath.Join(repairedOlder.TargetLocator, "skills", "pdf", "references", "note.txt"))
	if err != nil || string(body) != "original resource" {
		t.Fatalf("cold repair restored wrong bytes: %q %v", body, err)
	}
	afterLock, err := os.ReadFile(lockPath)
	if err != nil || string(afterLock) != string(lockedBytes) {
		t.Fatal("repair changed desired lock", err)
	}
}
