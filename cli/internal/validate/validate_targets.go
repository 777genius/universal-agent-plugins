package validate

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func validatePortableContractCoverage(target string, profile platformmeta.PlatformProfile, graph pluginmanifest.PackageGraph) []Failure {
	return validatePortableSkillsCoverage(target, profile, graph)
}
