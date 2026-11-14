package main

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/capabilities"
	"github.com/spf13/cobra"
)

func TestVisibleCommandManifestSkipsHiddenChildren(t *testing.T) {
	t.Parallel()
	root := &cobra.Command{Use: "root", Short: "root short"}
	visible := &cobra.Command{Use: "alpha", Short: "alpha short", Long: "  alpha long  ", Aliases: []string{"a"}}
	hidden := &cobra.Command{Use: "beta", Hidden: true}
	root.AddCommand(visible, hidden)

	manifest := visibleCommandManifest(root)
	if len(manifest) != 2 {
		t.Fatalf("manifest entries = %d", len(manifest))
	}
	if manifest[0].CommandPath != "root" || manifest[1].CommandPath != "root alpha" {
		t.Fatalf("manifest command paths = %#v", manifest)
	}
	if manifest[1].Slug != "root-alpha" {
		t.Fatalf("slug = %q", manifest[1].Slug)
	}
	if manifest[1].FileName != "root_alpha.md" {
		t.Fatalf("file name = %q", manifest[1].FileName)
	}
	if manifest[1].Long != "alpha long" {
		t.Fatalf("long = %q", manifest[1].Long)
	}
	if !reflect.DeepEqual(manifest[1].Aliases, []string{"a"}) {
		t.Fatalf("aliases = %#v", manifest[1].Aliases)
	}
}

func TestUniqueCapabilitiesDedupesAndSorts(t *testing.T) {
	t.Parallel()
	entries := []capabilities.Entry{
		{Capabilities: []string{"beta", "alpha"}},
		{Capabilities: []string{"alpha", "gamma"}},
	}
	if got := uniqueCapabilities(entries); !reflect.DeepEqual(got, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("uniqueCapabilities = %#v", got)
	}
}
