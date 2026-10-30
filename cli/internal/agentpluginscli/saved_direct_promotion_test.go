package agentpluginscli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDirectSavedAddPromotesRetainedIntentWithoutAcquisition(t *testing.T) {
	commit := strings.Repeat("1", 40)
	checksum := strings.Repeat("a", 64)
	archiveURL, gitURL := "https://packages.test/c.zip", "https://git.test/repo.git"
	archive := func(sha string) environment.Requirement {
		return environment.Requirement{Alias: "c", Archive: &environment.ArchiveSource{URL: archiveURL, SHA256: sha}}
	}
	git := func(kind environment.SelectorKind, value string) environment.Requirement {
		return environment.Requirement{Alias: "c", Git: &environment.GitSource{URL: gitURL, Selector: environment.Selector{Kind: kind, Value: value}}}
	}
	for _, tc := range []struct {
		name, input string
		intent      environment.Requirement
	}{
		{"archive moving URL", archiveURL, archive("")},
		{"archive exact checksum", archiveURL + "#sha256=" + checksum, archive(checksum)},
		{"Git default", gitURL, git(environment.SelectorDefault, "")},
		{"Git named ref", gitURL + "#main", git(environment.SelectorRef, "main")},
		{"Git exact commit", gitURL + "#" + commit, git(environment.SelectorCommit, commit)},
		{"Git pinned commit retains requested ref", gitURL + "#" + commit, git(environment.SelectorRef, "main")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, declared := range []bool{false, true} {
				t.Run(map[bool]string{false: "transitive", true: "already direct"}[declared], func(t *testing.T) {
					snapshot, expectedPin := directPromotionSnapshot(t, tc.intent, declared)
					before, _ := json.Marshal(snapshot)
					// A nil SourceAcquirer makes any accidental resolution/download fail.
					got, pin, err := resolveDirectSavedAdd(t.Context(), App{}, &options{alias: "c"}, tc.input, snapshot)
					if err != nil || !reflect.DeepEqual(got, tc.intent) || !reflect.DeepEqual(pin, expectedPin) {
						t.Fatalf("retained source changed portable intent or immutable pin: got=%+v pin=%+v err=%v", got, pin, err)
					}
					after, _ := json.Marshal(snapshot)
					if string(before) != string(after) {
						t.Fatal("promotion resolution mutated the existing graph or parent edges")
					}
				})
			}
		})
	}
}

func TestDirectSavedPromotionRejectsSelectorOrAliasChanges(t *testing.T) {
	commit := strings.Repeat("1", 40)
	checksum := strings.Repeat("a", 64)
	archiveURL, gitURL := "https://packages.test/c.zip", "https://git.test/repo.git"
	for _, tc := range []struct {
		name, input, alias, want string
		intent                   environment.Requirement
	}{
		{"archive different checksum", archiveURL + "#sha256=" + strings.Repeat("b", 64), "c", "saved selector must be changed", environment.Requirement{Alias: "c", Archive: &environment.ArchiveSource{URL: archiveURL, SHA256: checksum}}},
		{"archive does not add checksum to moving intent", archiveURL + "#sha256=" + checksum, "c", "saved selector must be changed", environment.Requirement{Alias: "c", Archive: &environment.ArchiveSource{URL: archiveURL}}},
		{"archive does not drop requested checksum", archiveURL, "c", "saved selector must be changed", environment.Requirement{Alias: "c", Archive: &environment.ArchiveSource{URL: archiveURL, SHA256: checksum}}},
		{"Git different ref", gitURL + "#other", "c", "saved selector must be changed", environment.Requirement{Alias: "c", Git: &environment.GitSource{URL: gitURL, Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}},
		{"Git named ref does not replace default", gitURL + "#main", "c", "saved selector must be changed", environment.Requirement{Alias: "c", Git: &environment.GitSource{URL: gitURL, Selector: environment.Selector{Kind: environment.SelectorDefault}}}},
		{"Git different immutable commit", gitURL + "#" + strings.Repeat("2", 40), "c", "saved selector must be changed", environment.Requirement{Alias: "c", Git: &environment.GitSource{URL: gitURL, Selector: environment.Selector{Kind: environment.SelectorCommit, Value: commit}}}},
		{"archive alias belongs to retained source", archiveURL, "other", "source_change: source already saved under alias c", environment.Requirement{Alias: "c", Archive: &environment.ArchiveSource{URL: archiveURL}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, declared := range []bool{false, true} {
				t.Run(map[bool]string{false: "transitive", true: "already direct"}[declared], func(t *testing.T) {
					snapshot, _ := directPromotionSnapshot(t, tc.intent, declared)
					before, _ := json.Marshal(snapshot)
					_, _, err := resolveDirectSavedAdd(t.Context(), App{}, &options{alias: tc.alias}, tc.input, snapshot)
					if err == nil || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("retained selector/alias change was accepted or acquired: %v", err)
					}
					after, _ := json.Marshal(snapshot)
					if string(before) != string(after) {
						t.Fatal("refused promotion mutated the existing graph")
					}
				})
			}
		})
	}
}

func directPromotionSnapshot(t *testing.T, intent environment.Requirement, declared bool) (environmentSnapshot, environment.LockedPackage) {
	t.Helper()
	requirements := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{"gemini": {"a", "b"}}}
	locked := environment.Lock{LockVersion: environment.LockVersion}
	pin := environment.LockedPackage{Alias: "c", Version: "1.0.0", SourceKey: mustSourceKey(intent), Targets: []string{"gemini"}, DigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest: "sha256:" + strings.Repeat("c", 64), ManifestDigest: "sha256:" + strings.Repeat("d", 64),
		RequiredBy: []environment.RequiredBy{{Alias: "a", Selector: intent.RequestedSelector()}, {Alias: "b", Selector: intent.RequestedSelector()}}}
	if intent.Archive != nil {
		pin.Source.Archive = &environment.LockedArchiveSource{URL: intent.Archive.URL, RequestedSHA256: intent.Archive.SHA256, SHA256: strings.Repeat("a", 64)}
	} else {
		pin.Source.Git = &environment.LockedGitSource{URL: intent.Git.URL, Requested: intent.Git.Selector, Commit: strings.Repeat("1", 40)}
	}
	for _, alias := range []string{"a", "b"} {
		root := environment.Requirement{Alias: alias, Archive: &environment.ArchiveSource{URL: "https://packages.test/" + alias + ".zip"}}
		requirements.Dependencies[alias] = root
		locked.Packages = append(locked.Packages, environment.LockedPackage{Alias: alias, Version: "1.0.0", SourceKey: mustSourceKey(root), Targets: []string{"gemini"}, DigestAlgorithm: domain.TreeDigestAlgorithm,
			TreeDigest: "sha256:" + strings.Repeat("e", 64), ManifestDigest: "sha256:" + strings.Repeat("f", 64),
			Source: environment.LockedSource{Archive: &environment.LockedArchiveSource{URL: root.Archive.URL, SHA256: strings.Repeat("b", 64)}}})
	}
	if declared {
		requirements.Dependencies["c"] = intent
		requirements.Targets["gemini"] = append(requirements.Targets["gemini"], "c")
	}
	locked.Packages = append(locked.Packages, pin)
	var err error
	locked.ManifestDigest, err = environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	if err := locked.Validate(requirements); err != nil {
		t.Fatalf("invalid promotion test graph: %v", err)
	}
	return environmentSnapshot{Requirements: requirements, Lock: locked}, pin
}
