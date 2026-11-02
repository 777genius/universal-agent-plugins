package shared

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectComponentMutation swaps exact skill directories around one kernel
// batch. The adapter owns scope selection and prepares all native server values.
type ProjectComponentMutation struct {
	Layout            ProjectComponents
	ActivePath        string
	Previous, Desired []domain.NativeObjectOwnership
	Servers           map[string]nativeconfig.Server
	Kernel            nativeconfig.Kernel
}

func (mutation ProjectComponentMutation) prepare(ctx context.Context) ([]nativeconfig.Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := mutation.Layout.ValidateObjects(mutation.Desired); err != nil {
		return nil, err
	}
	if err := mutation.Layout.Verify(mutation.Previous, true, mutation.Kernel); err != nil {
		return nil, err
	}
	if len(mutation.Desired) > 0 {
		if err := mutation.Layout.packagePath(mutation.ActivePath); err != nil {
			return nil, err
		}
	}
	previous := ObjectMap(mutation.Previous)
	desiredMCP := 0
	for _, next := range mutation.Desired {
		if prior, ok := previous[next.ObjectID]; ok {
			if prior.Path != next.Path || prior.Kind != next.Kind || prior.LogicalName != next.LogicalName {
				return nil, fmt.Errorf("project component identity changed")
			}
		} else if err := mutation.requireAbsent(next); err != nil {
			return nil, err
		}
		if next.Kind == mutation.Layout.MCPKind {
			desiredMCP++
		}
	}
	if len(mutation.Servers) != desiredMCP {
		return nil, fmt.Errorf("prepared MCP server selection differs from desired ownership")
	}
	return mutation.requests(previous, ObjectMap(mutation.Desired))
}

func (mutation ProjectComponentMutation) requireAbsent(object domain.NativeObjectOwnership) error {
	if object.Kind == mutation.Layout.SkillKind {
		_, err := os.Lstat(object.Path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("project skill %q exists without ownership", object.LogicalName)
	}
	present, _, err := mutation.Kernel.Inspect(mutation.Layout.ConfigPaths, mutation.Layout.Codec, object.LogicalName, nil)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("project MCP server %q exists without ownership", object.LogicalName)
	}
	return nil
}

func (mutation ProjectComponentMutation) requests(previous, desired map[string]domain.NativeObjectOwnership) ([]nativeconfig.Request, error) {
	ids := map[string]bool{}
	for id := range previous {
		ids[id] = true
	}
	for id := range desired {
		ids[id] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Strings(ordered)
	requests := []nativeconfig.Request{}
	for _, id := range ordered {
		prior, hadPrior := previous[id]
		next, hasNext := desired[id]
		if prior.Kind != mutation.Layout.MCPKind && next.Kind != mutation.Layout.MCPKind {
			continue
		}
		request, include, err := mutation.request(prior, next, hadPrior, hasNext)
		if err != nil {
			return nil, err
		}
		if include {
			requests = append(requests, request)
		}
	}
	return requests, nil
}

func (mutation ProjectComponentMutation) request(prior, next domain.NativeObjectOwnership, hadPrior, hasNext bool) (nativeconfig.Request, bool, error) {
	name := prior.LogicalName
	if hasNext {
		name = next.LogicalName
	}
	var owned *nativeconfig.Receipt
	if hadPrior {
		owned = mutation.Layout.receipt(prior)
	}
	present, valid, err := mutation.Kernel.Inspect(mutation.Layout.ConfigPaths, mutation.Layout.Codec, name, owned)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	request := nativeconfig.Request{Paths: mutation.Layout.ConfigPaths, Codec: mutation.Layout.Codec, Name: name, Owned: owned}
	if !hasNext {
		request.Action = nativeconfig.ActionRemove
		return request, present, nil
	}
	server, ok := mutation.Servers[name]
	if !ok {
		return request, false, fmt.Errorf("missing prepared MCP server %q", name)
	}
	expected, err := mutation.Layout.MCPObject(name, server, next.Path)
	if err != nil {
		return request, false, err
	}
	if expected.ManagedDigest != next.ManagedDigest {
		return request, false, fmt.Errorf("prepared MCP server %q differs from staged receipt", name)
	}
	request.Server, request.Desired = server, mutation.Layout.receipt(next)
	request.Action = nativeconfig.ActionAdd
	if present {
		if !valid {
			return request, false, fmt.Errorf("project MCP server %q is no longer owned", name)
		}
		request.Action = nativeconfig.ActionUpdate
	}
	return request, true, nil
}
