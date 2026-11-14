package main

import (
	"path/filepath"
	"testing"
)

func TestNormalizePublicationRequestedTargetDefaultsToAll(t *testing.T) {
	t.Parallel()
	if got := normalizePublicationRequestedTarget("   "); got != "all" {
		t.Fatalf("target = %q", got)
	}
}

func TestShouldDiagnoseGeneratedPublicationArtifactsDetectsCanonicalManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWritePublicationTestFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\n")
	if !shouldDiagnoseGeneratedPublicationArtifacts(root) {
		t.Fatal("expected canonical manifest to enable generated artifact diagnosis")
	}
}

func TestComparePublicationIssueOrdersByCodeThenTargetThenChannelThenPath(t *testing.T) {
	t.Parallel()

	left := publicationIssue{Code: "alpha", Target: "cursor", ChannelFamily: "one", Path: "a"}
	right := publicationIssue{Code: "beta", Target: "claude", ChannelFamily: "zero", Path: "b"}
	if got := comparePublicationIssue(left, right); got >= 0 {
		t.Fatalf("compare = %d", got)
	}
}
