package main

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

func TestShouldVerifyPublicationLocalRootSkipsInactiveStatusesAndEmptyDest(t *testing.T) {
	t.Parallel()

	if shouldVerifyPublicationLocalRoot("", "ready") {
		t.Fatal("expected empty dest to skip verification")
	}
	if shouldVerifyPublicationLocalRoot("/tmp/market", "inactive") {
		t.Fatal("expected inactive diagnosis to skip verification")
	}
	if shouldVerifyPublicationLocalRoot("/tmp/market", "needs_channels") {
		t.Fatal("expected needs_channels diagnosis to skip verification")
	}
	if !shouldVerifyPublicationLocalRoot("/tmp/market", "ready") {
		t.Fatal("expected ready diagnosis with dest to verify")
	}
}

func TestMergePublicationDiagnosisLocalRootNextStepsAppendsOnlyWhenNotReady(t *testing.T) {
	t.Parallel()

	got := mergePublicationDiagnosisLocalRootNextSteps([]string{"base"}, &app.PluginPublicationVerifyRootResult{
		Ready:     false,
		NextSteps: []string{"extra"},
	})
	if !reflect.DeepEqual(got, []string{"base", "extra"}) {
		t.Fatalf("next steps = %+v", got)
	}
}

func TestNormalizedPublicationLocalRootInitializesNilSlices(t *testing.T) {
	t.Parallel()

	got := normalizedPublicationLocalRoot(&app.PluginPublicationVerifyRootResult{})
	if got == nil || got.Issues == nil || got.NextSteps == nil {
		t.Fatalf("normalized local root = %+v", got)
	}
}
