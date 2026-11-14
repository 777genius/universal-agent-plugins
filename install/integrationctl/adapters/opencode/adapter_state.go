package opencode

import (
	"context"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func sortedManagedKeys(values map[string]any) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		if key == "$schema" {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedMapKeys(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (a Adapter) removeStaleFiles(ctx context.Context, previous, keep []string) error {
	keepSet := mapFromSlice(keep, func(value string) string { return value })
	for _, path := range previous {
		if keepSet[path] {
			continue
		}
		if err := a.fs().Remove(ctx, path); err != nil {
			return domain.NewError(domain.ErrMutationApply, "remove stale OpenCode projected asset", err)
		}
		a.removeEmptyParents(path, a.assetsRootForPath(path))
	}
	return nil
}
