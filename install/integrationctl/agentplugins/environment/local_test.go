package environment

import (
	"strings"
	"testing"
)

func localSkillPin(t *testing.T) (Requirement, LockedPackage) {
	t.Helper()
	req := Requirement{Alias: "review", Local: &LocalSource{Path: "/skills/review", Skill: "review"}}
	key, err := req.SourceKey()
	if err != nil {
		t.Fatal(err)
	}
	pin := LockedPackage{Alias: req.Alias, SourceKey: key, Targets: []string{"codex"}, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Source: LockedSource{Local: &LockedLocalSource{Path: req.Local.Path, Skill: req.Local.Skill, RawTreeDigest: "sha256:" + strings.Repeat("c", 64), Normalizer: SkillNormalizerV1}}}
	return req, pin
}

func TestLocalSkillPinsRejectMixedSourcesAndTransitiveAuthority(t *testing.T) {
	req, pin := localSkillPin(t)
	if err := validatePin(pin); err != nil {
		t.Fatal(err)
	}
	if !MovingRequirement(req) || pin.Revision() != pin.Source.Local.RawTreeDigest || !sameSourceRequirement(req, pin.Requirement()) {
		t.Fatal("local Skill did not retain source/update/revision intent")
	}
	for _, arm := range []LockedSource{
		{Git: &LockedGitSource{Repository: "acme/tools"}},
		{Archive: &LockedArchiveSource{URL: "https://example.com/tools.zip"}},
		{Web: &LockedWebSource{}}, {Registry: &LockedRegistrySource{}},
	} {
		arm.Local = pin.Source.Local
		bad := pin
		bad.Source = arm
		if validatePin(bad) == nil {
			t.Fatalf("accepted mixed source union: %+v", arm)
		}
	}
	for _, mutate := range []func(*LockedPackage){
		func(p *LockedPackage) { p.Source.Local.Normalizer = "future" },
		func(p *LockedPackage) { p.Source.Local.RawTreeDigest = "" },
		func(p *LockedPackage) { p.Source.Local.Path = "./review" },
		func(p *LockedPackage) { p.Source.Local.Skill = "other" },
		func(p *LockedPackage) {
			p.RequiredBy = []RequiredBy{{Alias: "parent", Selector: Selector{Kind: SelectorDefault}}}
		},
	} {
		bad := pin
		source := *pin.Source.Local
		bad.Source.Local = &source
		mutate(&bad)
		if validatePin(bad) == nil {
			t.Fatalf("accepted malformed local pin: %+v", bad)
		}
	}
}
