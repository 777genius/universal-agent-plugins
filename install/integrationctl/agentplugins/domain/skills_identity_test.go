package domain

import "testing"

func TestSkillBindingSurvivesRevisionAndRelocation(t *testing.T) {
	source := SourceIdentity{SourceBindingHint: "uap-dependency-skill", StableSourceKey: "github-skill:acme/tools:pdf:skills/pdf", Repository: "acme/tools", PackageSubpath: "skills/pdf", CanonicalSource: "github:acme/tools@old//skills/pdf"}
	before := ComputeSourceBindingID(source)
	source.PackageSubpath = "moved/pdf"
	source.ResolvedRevision = "new"
	source.CanonicalSource = "github:acme/tools@new//moved/pdf"
	if got := ComputeSourceBindingID(source); got != before {
		t.Fatal("relocation changed binding", got, before)
	}
	source.StableSourceKey = "github-skill:acme/tools:other:skills/pdf"
	if ComputeSourceBindingID(source) == before {
		t.Fatal("distinct Skill aliases shared identity")
	}
}
