package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func inspectionManagedPathsForTarget(inspection pluginmanifest.Inspection, target string) ([]string, error) {
	for _, item := range inspection.Targets {
		if item.Target == target {
			return append([]string(nil), item.ManagedArtifacts...), nil
		}
	}
	return nil, fmt.Errorf("inspect output does not include target %s", target)
}
