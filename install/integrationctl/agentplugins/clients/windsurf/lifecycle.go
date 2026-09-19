package windsurf

import (
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type windsurfMutation struct {
	action   nativeconfig.Action
	name     string
	server   nativeconfig.Server
	previous *nativeconfig.Receipt
	desired  *nativeconfig.Receipt
}

type windsurfNativeState struct {
	configPath     string
	previousMap    map[string]domain.NativeObjectOwnership
	desiredMap     map[string]domain.NativeObjectOwnership
	desiredServers map[string]nativeconfig.Server
}

func applyWindsurfNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	state, err := loadWindsurfNativeState(configRoot, activePath, previous, desired)
	if err != nil {
		return err
	}
	mutations, err := planWindsurfMutations(kernel, state)
	if err != nil {
		return err
	}
	if len(mutations) == 0 {
		return VerifyWindsurfNativeObjects(configRoot, activePath, desired, false, kernel)
	}
	requests := make([]nativeconfig.Request, len(mutations))
	for index := range mutations {
		requests[index] = windsurfNativeRequest(state.configPath, mutations[index])
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
	return VerifyWindsurfNativeObjects(configRoot, activePath, desired, false, kernel)
}

func loadWindsurfNativeState(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (windsurfNativeState, error) {
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return windsurfNativeState{}, err
	}
	previousMap, err := windsurfObjectMap(configRoot, previous)
	if err != nil {
		return windsurfNativeState{}, err
	}
	desiredMap, err := windsurfObjectMap(configRoot, desired)
	if err != nil {
		return windsurfNativeState{}, err
	}
	desiredServers := map[string]nativeconfig.Server{}
	if len(desiredMap) > 0 {
		desiredServers, err = readProjectedWindsurfServers(activePath)
		if err != nil {
			return windsurfNativeState{}, err
		}
	}
	if len(desiredServers) != len(desiredMap) {
		return windsurfNativeState{}, fmt.Errorf("prepared Windsurf MCP projection does not match desired ownership")
	}
	return windsurfNativeState{
		configPath: configPath, previousMap: previousMap, desiredMap: desiredMap, desiredServers: desiredServers,
	}, nil
}

func planWindsurfMutations(kernel nativeconfig.Kernel, state windsurfNativeState) ([]windsurfMutation, error) {
	names, err := windsurfMutationNames(state.previousMap, state.desiredMap)
	if err != nil {
		return nil, err
	}
	mutations := make([]windsurfMutation, 0, len(names))
	for _, name := range names {
		mutation, include, err := planWindsurfMutation(kernel, state, name)
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

func planWindsurfMutation(kernel nativeconfig.Kernel, state windsurfNativeState, name string) (windsurfMutation, bool, error) {
	oldObject, hadOld := state.previousMap[name]
	newObject, hasNew := state.desiredMap[name]
	mutation := windsurfMutation{name: name}
	switch {
	case hadOld && hasNew:
		oldReceipt := windsurfReceipt(oldObject)
		newReceipt := windsurfReceipt(newObject)
		mutation.action, mutation.server, mutation.previous, mutation.desired = nativeconfig.ActionUpdate, state.desiredServers[name], &oldReceipt, &newReceipt
	case !hadOld && hasNew:
		newReceipt := windsurfReceipt(newObject)
		mutation.action, mutation.server, mutation.desired = nativeconfig.ActionAdd, state.desiredServers[name], &newReceipt
	case hadOld && !hasNew:
		oldReceipt := windsurfReceipt(oldObject)
		mutation.action, mutation.previous = nativeconfig.ActionRemove, &oldReceipt
	default:
		return windsurfMutation{}, false, nil
	}
	include, err := preflightWindsurfMutation(kernel, state.configPath, &mutation)
	if err != nil || !include {
		return windsurfMutation{}, false, err
	}
	if err := previewWindsurfMutation(state.configPath, &mutation); err != nil {
		return windsurfMutation{}, false, err
	}
	return mutation, true, nil
}

func preflightWindsurfMutation(kernel nativeconfig.Kernel, configPath string, mutation *windsurfMutation) (bool, error) {
	if mutation.previous != nil {
		return preflightOwnedWindsurfMutation(kernel, configPath, mutation)
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

func preflightOwnedWindsurfMutation(kernel nativeconfig.Kernel, configPath string, mutation *windsurfMutation) (bool, error) {
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
		return true, nil
	}
	if !owned {
		return false, fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", mutation.name, nativeconfig.ErrNotOwned)
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
