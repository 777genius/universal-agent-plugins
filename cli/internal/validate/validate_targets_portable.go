package validate

import (
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func validatePortableSkillsCoverage(target string, profile platformmeta.PlatformProfile, graph pluginmanifest.PackageGraph) []Failure {
	var failures []Failure
	if len(graph.Portable.Paths("skills")) == 0 {
		return failures
	}
	if !slices.Contains(profile.Contract.PortableComponentKinds, "skills") {
		return failures
	}
	if !portableSkillsManaged(profile) {
		failures = append(failures, Failure{
			Kind:    FailureGeneratedContractInvalid,
			Path:    unsupportedPortablePath(graph.Portable, "skills"),
			Target:  target,
			Message: fmt.Sprintf("target %s declares portable skills support but does not declare managed skill artifacts", target),
		})
	}
	return failures
}

func portableSkillsManaged(profile platformmeta.PlatformProfile) bool {
	for _, spec := range profile.ManagedArtifacts {
		if spec.Kind == platformmeta.ManagedArtifactPortableSkills {
			return true
		}
	}
	return false
}
