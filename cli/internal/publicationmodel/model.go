package publicationmodel

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

type Core struct {
	APIVersion  string `json:"api_version"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type Package struct {
	Target           string            `json:"target"`
	PackageFamily    string            `json:"package_family"`
	ChannelFamilies  []string          `json:"channel_families"`
	TargetClass      string            `json:"target_class"`
	InstallModel     string            `json:"install_model,omitempty"`
	AuthoredInputs   []string          `json:"authored_inputs"`
	AuthoredDocs     map[string]string `json:"authored_docs,omitempty"`
	ManagedArtifacts []string          `json:"managed_artifacts"`
}

type Model struct {
	Core     Core      `json:"core"`
	Packages []Package `json:"packages"`
	Channels []Channel `json:"channels"`
}

type Channel struct {
	Family         string            `json:"family"`
	Path           string            `json:"path"`
	PackageTargets []string          `json:"package_targets"`
	Details        map[string]string `json:"details,omitempty"`
}

func Build(graph pluginmodel.PackageGraph, publication publishschema.State, selected []string) Model {
	out := Model{
		Core: Core{
			APIVersion:  strings.TrimSpace(graph.Manifest.APIVersion),
			Name:        strings.TrimSpace(graph.Manifest.Name),
			Version:     strings.TrimSpace(graph.Manifest.Version),
			Description: strings.TrimSpace(graph.Manifest.Description),
		},
		Packages: []Package{},
		Channels: []Channel{},
	}
	for _, target := range selected {
		pkg, ok := buildPackage(graph, publication, target)
		if !ok {
			continue
		}
		out.Packages = append(out.Packages, pkg)
	}
	slices.SortFunc(out.Packages, func(a, b Package) int {
		return strings.Compare(a.Target, b.Target)
	})
	out.Channels = buildChannels(publication, out.Packages)
	return out
}
