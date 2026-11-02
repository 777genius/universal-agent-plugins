package environment

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The pin contract must reject source mixing and any loss of acquisition facts
// before a locked Skill can be restored or assigned to an environment.
func TestWebsiteSkillPinRoundTripAndValidation(t *testing.T) {
	req := Requirement{Alias: "pdf", Web: &WebSource{URL: "https://publisher.example/prefix", Skill: "pdf"}}
	key, err := req.SourceKey()
	if err != nil || key != "https-skill:https://publisher.example/prefix:pdf" {
		t.Fatalf("%s: %v", key, err)
	}
	pin := LockedPackage{Alias: "pdf", SourceKey: key, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: []string{"codex"}, Source: LockedSource{Web: &LockedWebSource{URL: req.Web.URL, Skill: "pdf", IndexURL: req.Web.URL + "/.well-known/skills/index.json", IndexSHA256: strings.Repeat("c", 64), Files: []domain.WebSkillFile{{Path: "SKILL.md", URL: req.Web.URL + "/.well-known/skills/pdf/SKILL.md", SHA256: strings.Repeat("d", 64)}}, RawTreeDigest: "sha256:" + strings.Repeat("e", 64), Normalizer: SkillNormalizerV1}}}
	reqs := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"pdf": req}, Targets: map[string][]string{"codex": {"pdf"}}}
	digest, err := ManifestDigest(reqs)
	if err != nil {
		t.Fatal(err)
	}
	lock := Lock{LockVersion: LockVersion, ManifestDigest: digest, Packages: []LockedPackage{pin}}
	body, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Lock
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(reqs); err != nil {
		t.Fatal(err)
	}
	coordinator := Coordinator{Resolver: websitePinResolver{pin: pin}}
	resolved, err := coordinator.Resolve(t.Context(), reqs, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Resolve(t.Context(), reqs, resolved, "pdf"); err != nil {
		t.Fatal(err)
	}
	plan, err := coordinator.PrepareSync(reqs, resolved, []BindingObservation{{Alias: "pdf", ClientID: "codex", SourceKey: key, PackageDigest: pin.TreeDigest, Revision: pin.Source.Web.RawTreeDigest, Managed: true}})
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionUnchanged {
		t.Fatalf("website sync changed stable pin: %+v, %v", plan, err)
	}
	if decoded.Packages[0].Revision() != pin.Source.Web.RawTreeDigest || decoded.Packages[0].Requirement().RequestedSelector() != (Selector{Kind: SelectorDefault}) {
		t.Fatal("lost website source identity")
	}
	reconstructed := decoded.Packages[0].Requirement()
	reconstructed.Web.Skill = "other"
	if decoded.Packages[0].Source.Web.Skill != "pdf" {
		t.Fatal("requirement aliases locked source")
	}
	for _, mutate := range []func(*LockedPackage){
		func(p *LockedPackage) { p.Source.Web = nil },
		func(p *LockedPackage) {
			p.Source.Git = &LockedGitSource{Repository: "acme/tools", Requested: Selector{Kind: SelectorDefault}}
		},
		func(p *LockedPackage) { p.Source.Web.Skill = "other" },
		func(p *LockedPackage) { p.Source.Web.IndexSHA256 = "" },
		func(p *LockedPackage) { p.Source.Web.RawTreeDigest = "" },
		func(p *LockedPackage) { p.Source.Web.Normalizer = "future" },
		func(p *LockedPackage) { p.Source.Web.Files = nil },
		func(p *LockedPackage) { p.Source.Web.Files[0].URL = "https://elsewhere.example/SKILL.md" },
		func(p *LockedPackage) { p.Source.Web.Files[0].Path = "../SKILL.md" },
	} {
		var bad LockedPackage
		body, _ := json.Marshal(pin)
		if err := json.Unmarshal(body, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(&bad)
		if err := validatePin(bad); err == nil {
			t.Fatalf("accepted malformed website lock: %+v", bad)
		}
	}
}

// Embed the existing graph fixture so accidental Directory enumeration fails.
type websitePinResolver struct {
	remoteGraphResolver
	pin LockedPackage
}

func (resolver websitePinResolver) Resolve(_ context.Context, req Requirement, targets []string, _ *LockedPackage, _ bool) (LockedPackage, error) {
	pin := resolver.pin
	pin.Alias = req.Alias
	pin.Targets = append([]string(nil), targets...)
	return pin, nil
}
