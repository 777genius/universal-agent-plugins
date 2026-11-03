package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// The index belongs to one verification operation. Each parent's slice retains
// every recorded edge: duplicates and distinct selectors must remain evidence
// of an unexpected edge, rather than overwrite one another in a source map.
type dependencyDeclarationIndex struct {
	bySource map[string]environment.LockedPackage
	byParent map[string][]dependencyRecordedEdge
}

type dependencyRecordedEdge struct {
	sourceKey string
	selector  environment.Selector
}

func indexDependencyDeclarations(ctx context.Context, lock environment.Lock) (dependencyDeclarationIndex, error) {
	index := dependencyDeclarationIndex{
		bySource: make(map[string]environment.LockedPackage, len(lock.Packages)),
		byParent: make(map[string][]dependencyRecordedEdge),
	}
	for _, pin := range lock.Packages {
		if err := ctx.Err(); err != nil {
			return dependencyDeclarationIndex{}, err
		}
		index.bySource[pin.SourceKey] = pin
		for _, edge := range pin.RequiredBy {
			if err := ctx.Err(); err != nil {
				return dependencyDeclarationIndex{}, err
			}
			index.byParent[edge.Alias] = append(index.byParent[edge.Alias], dependencyRecordedEdge{sourceKey: pin.SourceKey, selector: edge.Selector})
		}
	}
	return index, ctx.Err()
}

func (index dependencyDeclarationIndex) verifyDeclaredEdges(ctx context.Context, parent string, declarations map[string]environment.Requirement) error {
	expected := make(map[string]environment.Selector, len(declarations))
	for _, requirement := range declarations {
		if err := ctx.Err(); err != nil {
			return err
		}
		key, err := requirement.SourceKey()
		if err != nil {
			return err
		}
		expected[key] = requirement.RequestedSelector()
	}
	for _, edge := range index.byParent[parent] {
		if err := ctx.Err(); err != nil {
			return err
		}
		selector, exists := expected[edge.sourceKey]
		if !exists || selector != edge.selector {
			return fmt.Errorf("%s has an unexpected dependency edge to %s; run agentplugins deps lock", parent, index.bySource[edge.sourceKey].Alias)
		}
		delete(expected, edge.sourceKey)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(expected) != 0 {
		return fmt.Errorf("%s pinned manifest declares missing dependency edges; run agentplugins deps lock", parent)
	}
	return nil
}
