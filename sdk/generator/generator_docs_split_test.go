package generator

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func TestAuthoringDocPath_CanonicalizesLegacyRootsToPlugin(t *testing.T) {
	t.Parallel()

	if got := authoringDocPath("src/targets/gemini/package.yaml"); got != "plugin/targets/gemini/package.yaml" {
		t.Fatalf("legacy path canonicalized = %q", got)
	}
	if got := authoringDocPath("targets/gemini/package.yaml"); got != "plugin/targets/gemini/package.yaml" {
		t.Fatalf("relative path = %q", got)
	}
}

func TestJoinManagedArtifacts_UsesMirrorFallbackDocName(t *testing.T) {
	t.Parallel()

	profile := platformmeta.PlatformProfile{
		NativeDocs: []platformmeta.NativeDocSpec{
			{Kind: "hooks", Path: "plugin/targets/claude/hooks/hooks.json"},
		},
		ManagedArtifacts: []platformmeta.ManagedArtifactSpec{
			{Kind: platformmeta.ManagedArtifactMirror, ComponentKind: "hooks"},
		},
	}
	if got := joinManagedArtifacts(profile); got != "hooks.json (when hooks are authored)" {
		t.Fatalf("joinManagedArtifacts() = %q", got)
	}
}
