package domain

import (
	"fmt"
	"slices"
)

// ResolveDirectory selects an exact signed publication for its complete target
// set. Its scope is both the publication scope and the affected backend scope.
func ResolveDirectory(snapshot DirectorySnapshot, request DirectoryResolveRequest) (DirectorySelection, error) {
	return resolveDirectory(snapshot, request, request.Scope)
}

// ResolveDirectoryV1ProjectSource selects a signed v1 User publication for
// explicit local Project placement. Publication grants and evidence remain
// User-scoped; affected targets follow the independently qualified Project
// backends. The result does not claim a signed Project grant or runtime proof.
func ResolveDirectoryV1ProjectSource(snapshot DirectorySnapshot, request DirectoryResolveRequest) (DirectorySelection, error) {
	if snapshot.SnapshotSchemaVersion != 1 || request.Scope != ScopeProject {
		return DirectorySelection{}, fmt.Errorf("%w: v1 Project source selection requires explicit Project scope", ErrDirectoryIneligible)
	}
	for _, client := range request.Targets {
		definition, ok := ClientDefinitionFor(client)
		if !ok || !definition.ProjectPolicy.Valid() || !slices.Contains(definition.Capabilities.Scopes, ScopeProject) {
			return DirectorySelection{}, fmt.Errorf("%w: %s has no qualified project installation policy", ErrDirectoryIneligible, client)
		}
	}
	request.Scope = ScopeUser
	return resolveDirectory(snapshot, request, ScopeProject)
}
