package app

import "slices"

func resolvePublicationManagedPaths(ctx publicationContext) ([]string, error) {
	managedPaths, err := inspectionManagedPathsForTarget(ctx.inspection, ctx.target)
	if err != nil {
		return nil, err
	}
	managedPaths = append(managedPaths, ctx.graph.Portable.Paths("skills")...)
	return slices.Compact(sortedSlashPaths(managedPaths)), nil
}
