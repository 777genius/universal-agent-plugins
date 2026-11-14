package app

import "testing"

func TestMissingPublicationVerifyCatalogArtifactIssueUsesCatalogPath(t *testing.T) {
	t.Parallel()
	issues := missingPublicationVerifyCatalogArtifactIssue(".claude-plugin/marketplace.json")
	if len(issues) != 1 {
		t.Fatalf("issues = %#v", issues)
	}
	if issues[0].Path != ".claude-plugin/marketplace.json" {
		t.Fatalf("issue path = %q", issues[0].Path)
	}
	if issues[0].Code != "missing_materialized_catalog_artifact" {
		t.Fatalf("issue code = %q", issues[0].Code)
	}
}
