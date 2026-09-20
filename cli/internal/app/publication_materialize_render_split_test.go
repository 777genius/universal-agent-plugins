package app

import "testing"

func TestBuildPublicationMaterializeDetailsPreservesCatalogFields(t *testing.T) {
	t.Parallel()

	details := buildPublicationMaterializeDetails(publicationMaterializePlan{
		packageRootAction:  "create",
		packageFiles:       make([]artifactResult, 3),
		catalogArtifactAct: "merge",
		catalogArtifact:    artifactResult{RelPath: ".agents/plugins/marketplace.json"},
	})
	if details["package_file_count"] != "3" || details["catalog_artifact_action"] != "merge" {
		t.Fatalf("details = %#v", details)
	}
}
