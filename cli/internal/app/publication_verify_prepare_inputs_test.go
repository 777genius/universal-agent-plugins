package app

import "testing"

func TestBuildPublicationVerifyPlanPreservesCatalogPath(t *testing.T) {
	t.Parallel()
	plan, err := buildPublicationVerifyPlan(publicationContext{}, publicationVerifyInputs{
		catalogRel: ".claude-plugin/marketplace.json",
	})
	if err != nil {
		t.Fatalf("buildPublicationVerifyPlan: %v", err)
	}
	if plan.catalogRel != ".claude-plugin/marketplace.json" {
		t.Fatalf("catalogRel = %q", plan.catalogRel)
	}
}
