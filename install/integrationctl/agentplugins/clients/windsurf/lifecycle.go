package windsurf

import (
	"context"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyWindsurfNative(env, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyWindsurfNative(env, request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

type windsurfMutation struct {
	action   nativeconfig.Action
	name     string
	server   nativeconfig.Server
	previous *nativeconfig.Receipt
	desired  *nativeconfig.Receipt
}

func applyWindsurfNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyWindsurfNative(clients.Env{NativeConfig: nativeconfig.New()}, configRoot, activePath, previous, desired)
}

func applyWindsurfNative(env clients.Env, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	kernel := env.NativeConfig
	configPath, previousMap, desiredMap, desiredServers, err := loadWindsurfNativeState(configRoot, activePath, previous, desired)
	if err != nil {
		return err
	}
	mutations, err := planWindsurfMutations(kernel, configPath, previousMap, desiredMap, desiredServers)
	if err != nil {
		return err
	}
	if len(mutations) == 0 {
		return VerifyNativeObjects(configRoot, activePath, desired, false)
	}
	requests := make([]nativeconfig.Request, len(mutations))
	for index := range mutations {
		requests[index] = windsurfNativeRequest(configPath, mutations[index])
	}
	results, err := kernel.ApplyBatch(requests)
	if err != nil {
		return fmt.Errorf("apply Windsurf MCP configuration: %w", err)
	}
	for index := range mutations {
		if mutations[index].desired != nil && results[index] != *mutations[index].desired {
			return fmt.Errorf("the Windsurf MCP entry %q ownership digest changed during apply", mutations[index].name)
		}
	}
	return VerifyNativeObjects(configRoot, activePath, desired, false)
}

func loadWindsurfNativeState(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (string, map[string]domain.NativeObjectOwnership, map[string]domain.NativeObjectOwnership, map[string]nativeconfig.Server, error) {
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}
	previousMap, err := windsurfObjectMap(configRoot, previous)
	if err != nil {
		return "", nil, nil, nil, err
	}
	desiredMap, err := windsurfObjectMap(configRoot, desired)
	if err != nil {
		return "", nil, nil, nil, err
	}
	desiredServers := map[string]nativeconfig.Server{}
	if len(desiredMap) > 0 {
		desiredServers, err = readProjectedWindsurfServers(activePath)
		if err != nil {
			return "", nil, nil, nil, err
		}
	}
	if len(desiredServers) != len(desiredMap) {
		return "", nil, nil, nil, fmt.Errorf("prepared Windsurf MCP projection does not match desired ownership")
	}
	return configPath, previousMap, desiredMap, desiredServers, nil
}

func planWindsurfMutations(kernel nativeconfig.Kernel, configPath string, previousMap, desiredMap map[string]domain.NativeObjectOwnership, desiredServers map[string]nativeconfig.Server) ([]windsurfMutation, error) {
	names, err := windsurfMutationNames(previousMap, desiredMap)
	if err != nil {
		return nil, err
	}
	mutations := make([]windsurfMutation, 0, len(names))
	for _, name := range names {
		mutation, include, err := planWindsurfMutation(kernel, configPath, name, previousMap, desiredMap, desiredServers)
		if err != nil {
			return nil, err
		}
		if include {
			mutations = append(mutations, mutation)
		}
	}
	return mutations, nil
}

func windsurfMutationNames(previousMap, desiredMap map[string]domain.NativeObjectOwnership) ([]string, error) {
	namesCapacity, capacityErr := shared.CheckedCombinedCapacity(len(previousMap), len(desiredMap))
	if capacityErr != nil {
		return nil, fmt.Errorf("prepare managed Windsurf MCP server set: %w", capacityErr)
	}
	names := make([]string, 0, namesCapacity)
	seen := map[string]bool{}
	for name := range previousMap {
		seen[name] = true
		names = append(names, name)
	}
	for name := range desiredMap {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func planWindsurfMutation(kernel nativeconfig.Kernel, configPath, name string, previousMap, desiredMap map[string]domain.NativeObjectOwnership, desiredServers map[string]nativeconfig.Server) (windsurfMutation, bool, error) {
	oldObject, hadOld := previousMap[name]
	newObject, hasNew := desiredMap[name]
	mutation := windsurfMutation{name: name}
	switch {
	case hadOld && hasNew:
		oldReceipt := windsurfReceipt(oldObject)
		newReceipt := windsurfReceipt(newObject)
		mutation.action, mutation.server, mutation.previous, mutation.desired = nativeconfig.ActionUpdate, desiredServers[name], &oldReceipt, &newReceipt
	case !hadOld && hasNew:
		newReceipt := windsurfReceipt(newObject)
		mutation.action, mutation.server, mutation.desired = nativeconfig.ActionAdd, desiredServers[name], &newReceipt
	case hadOld && !hasNew:
		oldReceipt := windsurfReceipt(oldObject)
		mutation.action, mutation.previous = nativeconfig.ActionRemove, &oldReceipt
	default:
		return windsurfMutation{}, false, nil
	}
	include, err := preflightWindsurfMutation(kernel, configPath, &mutation)
	if err != nil || !include {
		return windsurfMutation{}, false, err
	}
	if err := previewWindsurfMutation(configPath, &mutation); err != nil {
		return windsurfMutation{}, false, err
	}
	return mutation, true, nil
}

func preflightWindsurfMutation(kernel nativeconfig.Kernel, configPath string, mutation *windsurfMutation) (bool, error) {
	if mutation.previous != nil {
		present, owned, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, mutation.name, mutation.previous)
		if inspectErr != nil {
			return false, fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", mutation.name, inspectErr)
		}
		if !present {
			if mutation.action == nativeconfig.ActionRemove {
				return false, nil
			}
			mutation.action = nativeconfig.ActionAdd
			mutation.previous = nil
		} else if !owned {
			return false, fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", mutation.name, nativeconfig.ErrNotOwned)
		}
		return true, nil
	}
	present, _, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, mutation.name, nil)
	if inspectErr != nil {
		return false, fmt.Errorf("preflight Windsurf MCP entry %q: %w", mutation.name, inspectErr)
	}
	if present {
		return false, fmt.Errorf("preflight Windsurf MCP entry %q: %w", mutation.name, nativeconfig.ErrCollision)
	}
	return true, nil
}

func previewWindsurfMutation(configPath string, mutation *windsurfMutation) error {
	if mutation.desired == nil {
		return nil
	}
	preview, previewErr := desiredWindsurfReceipt(configPath, mutation.name, mutation.server)
	if previewErr != nil {
		return fmt.Errorf("preview Windsurf MCP entry %q: %w", mutation.name, previewErr)
	}
	if preview != *mutation.desired {
		return fmt.Errorf("the Windsurf MCP entry %q desired ownership digest drifted", mutation.name)
	}
	return nil
}

func windsurfNativeRequest(configPath string, mutation windsurfMutation) nativeconfig.Request {
	return nativeconfig.Request{
		Paths:  nativeconfig.Paths{JSON: configPath},
		Codec:  nativeconfig.CodecWindsurf,
		Action: mutation.action,
		Name:   mutation.name,
		Server: mutation.server,
		Owned:  mutation.previous,
	}
}
