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
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return err
	}
	previousMap, err := windsurfObjectMap(configRoot, previous)
	if err != nil {
		return err
	}
	desiredMap, err := windsurfObjectMap(configRoot, desired)
	if err != nil {
		return err
	}
	desiredServers := map[string]nativeconfig.Server{}
	if len(desiredMap) > 0 {
		desiredServers, err = readProjectedWindsurfServers(activePath)
		if err != nil {
			return err
		}
	}
	if len(desiredServers) != len(desiredMap) {
		return fmt.Errorf("prepared Windsurf MCP projection does not match desired ownership")
	}

	namesCapacity, capacityErr := shared.CheckedCombinedCapacity(len(previousMap), len(desiredMap))
	if capacityErr != nil {
		return fmt.Errorf("prepare managed Windsurf MCP server set: %w", capacityErr)
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

	mutations := make([]windsurfMutation, 0, len(names))
	for _, name := range names {
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
			continue
		}
		if mutation.previous != nil {
			present, owned, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, mutation.previous)
			if inspectErr != nil {
				return fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", name, inspectErr)
			}
			if !present {
				if mutation.action == nativeconfig.ActionRemove {
					continue
				}
				mutation.action = nativeconfig.ActionAdd
				mutation.previous = nil
			} else if !owned {
				return fmt.Errorf("preflight existing Windsurf MCP entry %q: %w", name, nativeconfig.ErrNotOwned)
			}
		} else {
			present, _, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, nil)
			if inspectErr != nil {
				return fmt.Errorf("preflight Windsurf MCP entry %q: %w", name, inspectErr)
			}
			if present {
				return fmt.Errorf("preflight Windsurf MCP entry %q: %w", name, nativeconfig.ErrCollision)
			}
		}
		if mutation.desired != nil {
			preview, previewErr := desiredWindsurfReceipt(configPath, name, mutation.server)
			if previewErr != nil {
				return fmt.Errorf("preview Windsurf MCP entry %q: %w", name, previewErr)
			}
			if preview != *mutation.desired {
				return fmt.Errorf("Windsurf MCP entry %q desired ownership digest drifted", name)
			}
		}
		mutations = append(mutations, mutation)
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
			return fmt.Errorf("Windsurf MCP entry %q ownership digest changed during apply", mutations[index].name)
		}
	}
	return VerifyNativeObjects(configRoot, activePath, desired, false)
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
