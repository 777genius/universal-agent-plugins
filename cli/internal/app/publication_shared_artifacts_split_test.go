package app

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestInspectionManagedPathsForTargetReturnsCopy(t *testing.T) {
	t.Parallel()

	paths, err := inspectionManagedPathsForTarget(pluginmanifest.Inspection{
		Targets: []pluginmanifest.InspectTarget{{
			Target:           "codex",
			ManagedArtifacts: []string{"a", "b"},
		}},
	}, "codex")
	if err != nil {
		t.Fatalf("inspectionManagedPathsForTarget: %v", err)
	}
	paths[0] = "changed"

	again, err := inspectionManagedPathsForTarget(pluginmanifest.Inspection{
		Targets: []pluginmanifest.InspectTarget{{
			Target:           "codex",
			ManagedArtifacts: []string{"a", "b"},
		}},
	}, "codex")
	if err != nil {
		t.Fatalf("inspectionManagedPathsForTarget: %v", err)
	}
	if !reflect.DeepEqual(again, []string{"a", "b"}) {
		t.Fatalf("paths = %#v", again)
	}
}

func TestSortedSlashPathsTrimsAndSorts(t *testing.T) {
	t.Parallel()

	got := sortedSlashPaths([]string{" b", "", "c", " a "})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v want %#v", got, want)
	}
}
