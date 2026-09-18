package cline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func mutateClineMCPWithKernelAndCapacity(configRoot, activePath string, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel, idsCapacity int) error {
	if !hasClineMCP(previous) && !hasClineMCP(desired) {
		return nil
	}
	path := ClineMCPSettingsPath(configRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	projection := ClineProjection{Servers: map[string]nativeconfig.Server{}}
	if activePath != "" {
		var err error
		projection, err = ReadClineProjection(activePath)
		if err != nil {
			return err
		}
	}
	ids := clineMCPObjectIDs(previous, desired, idsCapacity)
	requests := make([]nativeconfig.Request, 0, len(ids))
	for _, id := range ids {
		request, include, err := clineMCPRequest(path, projection, previous, desired, kernel, id)
		if err != nil {
			return err
		}
		if include {
			requests = append(requests, request)
		}
	}
	_, err := kernel.ApplyBatch(requests)
	if err != nil {
		return err
	}
	return nil
}

func clineMCPObjectIDs(previous, desired map[string]domain.NativeObjectOwnership, idsCapacity int) []string {
	ids := make([]string, 0, idsCapacity)
	seen := map[string]bool{}
	for id, object := range previous {
		if object.Kind == ClineMCPObjectKind {
			ids, seen[id] = append(ids, id), true
		}
	}
	for id, object := range desired {
		if object.Kind == ClineMCPObjectKind && !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func clineMCPRequest(path string, projection ClineProjection, previous, desired map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel, id string) (nativeconfig.Request, bool, error) {
	prior, hadPrior := previous[id]
	next, hasNext := desired[id]
	req := nativeconfig.Request{Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCline}
	present, err := inspectClineMCP(kernel, req, prior, next, hadPrior, hasNext)
	if err != nil {
		return nativeconfig.Request{}, false, err
	}
	if err := assignClineMCPAction(&req, projection, prior, next, hadPrior, hasNext, present); err != nil {
		return nativeconfig.Request{}, false, err
	}
	if hadPrior && !hasNext && !present {
		return nativeconfig.Request{}, false, nil
	}
	if hasNext {
		receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCline, next.LogicalName, req.Server, req.Placeholders)
		if err != nil {
			return nativeconfig.Request{}, false, err
		}
		if receipt.Digest != next.ManagedDigest {
			return nativeconfig.Request{}, false, fmt.Errorf("the Cline MCP desired receipt mismatch for %q", next.LogicalName)
		}
		req.Desired = &receipt
	}
	return req, true, nil
}

func assignClineMCPAction(req *nativeconfig.Request, projection ClineProjection, prior, next domain.NativeObjectOwnership, hadPrior, hasNext, present bool) error {
	switch {
	case hadPrior && hasNext:
		if !present {
			if !sameClineMCPObject(prior, next) {
				return fmt.Errorf("managed Cline MCP server %q is absent during update: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
			}
			req.Action, req.Name = nativeconfig.ActionAdd, next.LogicalName
		} else {
			receipt := clineReceipt(prior)
			req.Action, req.Name, req.Owned = nativeconfig.ActionUpdate, next.LogicalName, &receipt
		}
		req.Server = projection.Servers[next.LogicalName]
	case hadPrior:
		if !present {
			return nil
		}
		receipt := clineReceipt(prior)
		req.Action, req.Name, req.Owned = nativeconfig.ActionRemove, prior.LogicalName, &receipt
	case hasNext:
		req.Action, req.Name, req.Server = nativeconfig.ActionAdd, next.LogicalName, projection.Servers[next.LogicalName]
	}
	return nil
}

func inspectClineMCP(kernel nativeconfig.Kernel, req nativeconfig.Request, prior, next domain.NativeObjectOwnership, hadPrior, hasNext bool) (bool, error) {
	if hadPrior {
		receipt := clineReceipt(prior)
		present, exactlyOwned, err := kernel.Inspect(req.Paths, req.Codec, prior.LogicalName, &receipt)
		if err != nil {
			return false, fmt.Errorf("inspect managed Cline MCP server %q: %w", prior.LogicalName, err)
		}
		if present && !exactlyOwned {
			return false, fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", prior.LogicalName, nativeconfig.ErrNotOwned)
		}
		return present, nil
	}
	if !hasNext {
		return false, nil
	}
	present, _, err := kernel.Inspect(req.Paths, req.Codec, next.LogicalName, nil)
	if err != nil {
		return false, fmt.Errorf("inspect Cline MCP server %q before add: %w", next.LogicalName, err)
	}
	if present {
		return false, fmt.Errorf("the Cline MCP server %q already exists: %w", next.LogicalName, nativeconfig.ErrCollision)
	}
	return present, nil
}
