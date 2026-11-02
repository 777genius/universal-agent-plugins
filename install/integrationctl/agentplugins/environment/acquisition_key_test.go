package environment

import (
	"strings"
	"testing"
)

func TestAcquisitionKeyCopiesOnlyEmptyOperationalAlias(t *testing.T) {
	for _, req := range []Requirement{
		{Archive: &ArchiveSource{URL: "https://packages.test/archive.zip"}},
		{Git: &GitSource{URL: "https://git.test/team/repo.git", Selector: Selector{Kind: SelectorDefault}}},
	} {
		if req.Validate() == nil {
			t.Fatal("public validation admitted empty alias")
		}
		if _, err := req.SourceKey(); err == nil {
			t.Fatal("public SourceKey admitted empty alias")
		}
		explicit := req
		explicit.Alias = "candidate"
		want, err := explicit.SourceKey()
		if err != nil {
			t.Fatal(err)
		}
		got, err := acquisitionSourceKey(req)
		if err != nil || got != want || req.Alias != "" {
			t.Fatalf("operational key changed tuple/request: %s %s %+v %v", got, want, req, err)
		}
		if got, err := acquisitionSourceKey(explicit); err != nil || got != want || explicit.Alias != "candidate" {
			t.Fatal("explicit candidate alias changed", err)
		}
		req.Alias = "bad alias"
		if _, err := acquisitionSourceKey(req); err == nil {
			t.Fatal("nonempty invalid alias was normalized")
		}
	}
	invalid := Requirement{Archive: &ArchiveSource{URL: "http://unsafe.test/archive.zip", SHA256: strings.Repeat("a", 64)}}
	if _, err := acquisitionSourceKey(invalid); err == nil {
		t.Fatal("private key bypassed endpoint validation")
	}
}
