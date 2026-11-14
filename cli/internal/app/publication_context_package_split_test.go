package app

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestResolvePublicationManagedPathsIncludesPortableSkillsAndSorts(t *testing.T) {
	t.Parallel()

	graph := pluginmanifest.PackageGraph{}
	graph.Portable.Add("skills", "skills/b.md", "skills/a.md")

	got, err := resolvePublicationManagedPaths(publicationContext{
		target: "codex",
		graph:  graph,
		inspection: pluginmanifest.Inspection{
			Targets: []pluginmanifest.InspectTarget{{
				Target:           "codex",
				ManagedArtifacts: []string{"plugin/z.md", "plugin/a.md", "plugin/a.md"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("resolvePublicationManagedPaths: %v", err)
	}
	want := []string{"plugin/a.md", "plugin/z.md", "skills/a.md", "skills/b.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("managedPaths = %#v want %#v", got, want)
	}
}
