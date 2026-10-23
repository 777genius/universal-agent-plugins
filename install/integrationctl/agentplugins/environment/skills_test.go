package environment

import (
	"strings"
	"testing"
)

func TestSkillSourceIntentAndRelocatedPin(t *testing.T) {
	req := Requirement{Alias: "pdf", Git: &GitSource{Kind: SkillKind, Skill: "pdf", Repository: "Acme/Tools", Path: "skills/pdf", Selector: Selector{Kind: SelectorRef, Value: "main"}}}
	key, err := req.SourceKey()
	if err != nil {
		t.Fatal(err)
	}
	if key != "github-skill:acme/tools:pdf:skills/pdf" {
		t.Fatal(key)
	}
	plain := req
	sourceCopy := *req.Git
	sourceCopy.Kind, sourceCopy.Skill = "", ""
	plain.Git = &sourceCopy
	old, _ := plain.SourceKey()
	if old != "github:acme/tools:skills/pdf" {
		t.Fatal(old)
	}
	pin := LockedPackage{Alias: "pdf", SourceKey: key, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: []string{"codex"}, Source: LockedSource{Git: &LockedGitSource{Kind: SkillKind, Skill: "pdf", Repository: "acme/tools", Path: "skills/pdf", Requested: req.Git.Selector, Commit: strings.Repeat("1", 40), ResolvedSkillPath: "moved/pdf", RawTreeDigest: "sha256:" + strings.Repeat("c", 64), Normalizer: SkillNormalizerV1}}}
	requirements := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"pdf": req}, Targets: map[string][]string{"codex": {"pdf"}}}
	digest, err := ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	lock := Lock{LockVersion: 1, ManifestDigest: digest, Packages: []LockedPackage{pin}}
	if err = lock.Validate(requirements); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*LockedGitSource){func(g *LockedGitSource) { g.Normalizer = "future" }, func(g *LockedGitSource) { g.Kind = "" }, func(g *LockedGitSource) { g.Skill = "other" }, func(g *LockedGitSource) { g.RawTreeDigest = "" }, func(g *LockedGitSource) { g.ResolvedSkillPath = "../outside" }} {
		g := *pin.Source.Git
		mutate(&g)
		bad := pin
		bad.Source.Git = &g
		if err := validatePin(bad); err == nil {
			t.Fatalf("accepted invalid Skill lock: %+v", g)
		}
	}
}
