package environment

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// APM's historical source-path and ref failures are adversarial inputs, not
// alternative rules for UAP. The saved TOML/lock boundary must use exactly
// the same offline grammar as direct source installation.
func TestSavedGitRefMatchesDirectSourceAdmission(t *testing.T) {
	refs := []string{
		"main", "release/v1.2.3", "refs/tags/v1", "feature/fast.fix",
		"", "branch@{1}", "previous..next", "main//other",
		"refs/tags/locked.lock", "trailing.", "/absolute", "-option",
		"branch/\x00invalid", "branch/\x1fcontrol", strings.Repeat("x", 1025),
	}
	for _, ref := range refs {
		t.Run(strings.ReplaceAll(ref, "/", "_"), func(t *testing.T) {
			directErr := sourcespec.ValidateRef(ref)
			saved := Requirement{Alias: "demo", Git: &GitSource{
				Repository: "acme/demo", Selector: Selector{Kind: SelectorRef, Value: ref},
			}}
			savedErr := saved.Validate()
			if (directErr == nil) != (savedErr == nil) {
				t.Fatalf("saved and direct Git reference admission disagree for %q: direct=%v saved=%v", ref, directErr, savedErr)
			}
		})
	}
}

func TestSavedPackagePathMatchesDirectSourceAdmission(t *testing.T) {
	paths := []string{
		"", "packages/demo", "skills/.config", "docs/中文",
		"../outside", "nested/../outside", "CON", "a/com1.txt",
		"skills/NUL", "trailing.", "trailing ", "directory\\nested",
		"un-normalized//file", "a/e\u0301", strings.Repeat("a", 1025),
	}
	for _, path := range paths {
		t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			_, directErr := sourcespec.NormalizeSubpath(path)
			saved := Requirement{Alias: "demo", Git: &GitSource{
				Repository: "acme/demo", Path: path,
				Selector: Selector{Kind: SelectorRef, Value: "main"},
			}}
			savedErr := saved.Validate()
			if (directErr == nil) != (savedErr == nil) {
				t.Fatalf("saved and direct path admission disagree for %q: direct=%v saved=%v", path, directErr, savedErr)
			}
		})
	}
}

// A nonportable package subdirectory must be rejected before it can acquire
// lockfile authority, even if the Git SHA itself is immutable.
func TestManifestRejectsWindowsReservedAndAmbiguousGitPath(t *testing.T) {
	for _, bad := range []string{"CON", "plugins/AUX.json", "plugins/example.", "plugins/nested/../safe"} {
		req := Requirements{SchemaVersion: ManifestSchemaVersion,
			Dependencies: map[string]Requirement{
				"demo": {Alias: "demo", Git: &GitSource{
					Repository: "acme/demo", Path: bad,
					Selector: Selector{Kind: SelectorCommit, Value: strings.Repeat("a", 40)},
				}},
			},
			Targets: map[string][]string{"codex": {"demo"}},
		}
		if err := req.Validate(); err == nil {
			t.Fatalf("manifest accepted nonportable Git package path %q", bad)
		}
	}
}
