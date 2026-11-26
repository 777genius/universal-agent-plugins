package environment

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestGenericGitSkillSourceAndLockIdentity(t *testing.T) {
	for _, endpoint := range []string{"https://git.example/team/tools.git", "ssh://git@git.example/team/tools.git", "git@git.example:team/tools.git"} {
		req := Requirement{Alias: "pdf", Git: &GitSource{URL: endpoint, Path: "skills/pdf", Kind: SkillKind, Skill: "pdf", Selector: Selector{Kind: SelectorDefault}}}
		key, err := req.SourceKey()
		if err != nil || key != sourcespec.GitSkillKey(endpoint, "pdf", "skills/pdf") {
			t.Fatal(key, err)
		}
		plain := req
		git := *req.Git
		git.Kind, git.Skill = "", ""
		plain.Git = &git
		if plainKey, _ := plain.SourceKey(); plainKey == key {
			t.Fatal("plugin and Skill source identity collapsed")
		}
		pin := LockedPackage{Alias: "pdf", SourceKey: key, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Source: LockedSource{Git: &LockedGitSource{URL: endpoint, Kind: SkillKind, Skill: "pdf", Path: "skills/pdf", Requested: req.Git.Selector, Commit: strings.Repeat("1", 40), ResolvedSkillPath: "moved/pdf", RawTreeDigest: "sha256:" + strings.Repeat("c", 64), Normalizer: SkillNormalizerV1}}}
		if err := validatePin(pin); err != nil {
			t.Fatal(err)
		}
		pin.Source.Git.URL = "https://other.example/team/tools.git"
		if err := validatePin(pin); err == nil {
			t.Fatal("endpoint substitution accepted")
		}
	}
}
