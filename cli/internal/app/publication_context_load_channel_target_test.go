package app

import "testing"

func TestRequirePublicationTargetModelRejectsMissingPackage(t *testing.T) {
	t.Parallel()
	_, err := requirePublicationTargetModel(publicationContext{target: "claude"})
	if err == nil {
		t.Fatal("expected publication target error")
	}
}
