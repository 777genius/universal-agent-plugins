package environment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRemoteSourceLockIdentityAndOldReaderRefusal(t *testing.T) {
	for _, requirement := range []Requirement{
		{Alias: "demo", Git: &GitSource{URL: "ssh://git@example.test:2222/Team/Repo", Path: "plugins/demo", Selector: Selector{Kind: SelectorRef, Value: "main"}}},
		{Alias: "demo", Archive: &ArchiveSource{URL: "https://example.test/releases/package", Path: "plugins/demo"}},
	} {
		reqs := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"demo": requirement}, Targets: map[string][]string{"codex": {"demo"}}}
		resolver := remoteGraphResolver{}
		lock, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), reqs, Lock{}, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := lock.Validate(reqs); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(lock.Packages[0].Source)
		if err != nil {
			t.Fatal(err)
		}
		var old struct {
			Git *struct {
				Repository, Path string
				Requested        Selector
				Commit           string
			} `json:"git,omitempty"`
			Registry *LockedRegistrySource `json:"registry,omitempty"`
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&old) == nil {
			t.Fatal("old reader silently accepted new source semantics")
		}
		if !reflect.DeepEqual(requirement, lock.Packages[0].Requirement()) {
			t.Fatalf("intent changed after lock round trip: %+v", lock)
		}
		changed := lock.Packages[0]
		if changed.Source.Git != nil {
			copy := *changed.Source.Git
			copy.URL = "ssh://git@other.test/Team/Repo"
			changed.Source.Git = &copy
		} else {
			copy := *changed.Source.Archive
			copy.SHA256 = "bad"
			changed.Source.Archive = &copy
		}
		lock.Packages[0] = changed
		if lock.Validate(reqs) == nil {
			t.Fatal("rewritten pin accepted")
		}
	}
}

type remoteGraphResolver struct{}

func (remoteGraphResolver) Resolve(_ context.Context, req Requirement, targets []string, old *LockedPackage, update bool) (LockedPackage, error) {
	if old != nil && !update {
		return *old, nil
	}
	key, err := req.SourceKey()
	if err != nil {
		return LockedPackage{}, err
	}
	pkg := LockedPackage{Alias: req.Alias, SourceKey: key, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: targets}
	if g := req.Git; g != nil {
		pkg.Source.Git = &LockedGitSource{Repository: g.Repository, URL: g.URL, Path: g.Path, Requested: g.Selector, Commit: strings.Repeat("1", 40)}
	}
	if a := req.Archive; a != nil {
		pkg.Source.Archive = &LockedArchiveSource{URL: a.URL, Path: a.Path, RequestedSHA256: a.SHA256, SHA256: strings.Repeat("2", 64)}
		if a.SHA256 != "" {
			pkg.Source.Archive.SHA256 = a.SHA256
		}
	}
	return pkg, nil
}
func (remoteGraphResolver) Candidates(context.Context, Requirement, []string) ([]Candidate, error) {
	return nil, errors.New("remote sources must not enumerate Directory versions")
}
func (remoteGraphResolver) AcquireCandidate(context.Context, Requirement, Candidate, []string) (ResolvedPackage, error) {
	return ResolvedPackage{}, errors.New("unexpected Directory acquisition")
}
func (remoteGraphResolver) DescribePinned(_ context.Context, pkg LockedPackage) (ResolvedPackage, error) {
	return ResolvedPackage{Package: pkg}, nil
}
func (remoteGraphResolver) Eligible(context.Context, LockedPackage, []string) (bool, error) {
	return true, nil
}

func TestArchiveMovingUpdateAndSyncRevision(t *testing.T) {
	req := Requirement{Alias: "demo", Archive: &ArchiveSource{URL: "https://example.test/package"}}
	reqs := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"demo": req}, Targets: map[string][]string{"codex": {"demo"}}}
	coordinator := Coordinator{Resolver: remoteGraphResolver{}}
	lock, err := coordinator.Resolve(t.Context(), reqs, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := coordinator.Resolve(t.Context(), reqs, lock, "demo")
	if err != nil {
		t.Fatal(err)
	}
	pkg := updated.Packages[0]
	plan, err := coordinator.PrepareSync(reqs, updated, []BindingObservation{{Alias: "demo", ClientID: "codex", Managed: true, SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: "sha256:" + pkg.Source.Archive.SHA256}})
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionUnchanged {
		t.Fatalf("archive replay changed: %+v %v", plan, err)
	}
	req.Archive.SHA256 = pkg.Source.Archive.SHA256
	reqs.Dependencies["demo"] = req
	if _, err := coordinator.Resolve(t.Context(), reqs, Lock{}, "demo"); err == nil {
		t.Fatal("explicit checksum updated without intent change")
	}
}
