package publicationmodel

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func buildChannels(publication publishschema.State, packages []Package) []Channel {
	out := []Channel{}
	if publication.Codex != nil {
		targets := packageTargetsForFamily(packages, "codex-marketplace")
		if len(targets) > 0 {
			out = append(out, Channel{
				Family:         "codex-marketplace",
				Path:           publication.Codex.Path,
				PackageTargets: targets,
				Details: map[string]string{
					"marketplace_name":      publication.Codex.MarketplaceName,
					"source_root":           publication.Codex.SourceRoot,
					"category":              publication.Codex.Category,
					"installation_policy":   publication.Codex.InstallationPolicy,
					"authentication_policy": publication.Codex.AuthenticationPolicy,
				},
			})
		}
	}
	if publication.Claude != nil {
		targets := packageTargetsForFamily(packages, "claude-marketplace")
		if len(targets) > 0 {
			out = append(out, Channel{
				Family:         "claude-marketplace",
				Path:           publication.Claude.Path,
				PackageTargets: targets,
				Details: map[string]string{
					"marketplace_name": publication.Claude.MarketplaceName,
					"owner_name":       publication.Claude.OwnerName,
					"source_root":      publication.Claude.SourceRoot,
				},
			})
		}
	}
	if publication.Gemini != nil {
		targets := packageTargetsForFamily(packages, "gemini-gallery")
		if len(targets) > 0 {
			out = append(out, Channel{
				Family:         "gemini-gallery",
				Path:           publication.Gemini.Path,
				PackageTargets: targets,
				Details: map[string]string{
					"distribution":          publication.Gemini.Distribution,
					"repository_visibility": publication.Gemini.RepositoryVisibility,
					"github_topic":          publication.Gemini.GitHubTopic,
					"manifest_root":         publication.Gemini.ManifestRoot,
				},
			})
		}
	}
	slices.SortFunc(out, func(a, b Channel) int {
		return strings.Compare(a.Family, b.Family)
	})
	return out
}

func packageTargetsForFamily(packages []Package, family string) []string {
	var out []string
	for _, pkg := range packages {
		if slices.Contains(pkg.ChannelFamilies, family) {
			out = append(out, pkg.Target)
		}
	}
	slices.Sort(out)
	return out
}
