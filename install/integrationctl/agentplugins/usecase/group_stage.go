package usecase

import (
	"context"
	"fmt"
	"sync"
)

func (service Service) stagePlannedGroupTargets(ctx context.Context, planned []plannedGroupTarget, groupID, installationID string) error {
	for index := range planned {
		if err := service.ensurePlannedPluginData(ctx, &planned[index], installationID); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wait sync.WaitGroup
	var mu sync.Mutex
	var first error
	for index := range planned {
		if planned[index].noChange {
			continue
		}
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			err := service.stageOnePlannedTarget(ctx, &planned[index], fmt.Sprintf("%s-%03d", groupID, index+1))
			if err == nil {
				return
			}
			mu.Lock()
			if first == nil {
				first = err
			}
			mu.Unlock()
			cancel()
		}(index)
	}
	wait.Wait()
	return first
}

func (service Service) ensurePlannedPluginData(ctx context.Context, target *plannedGroupTarget, installationID string) error {
	if target.noChange || !packageNeedsPluginData(target.input.Envelope, target.plan) {
		return nil
	}
	if service.PluginData == nil {
		return fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
	}
	receipt, created, err := service.PluginData.EnsureData(ctx, installationID, target.plan.PhysicalArtifactID, string(target.input.Scope))
	if err != nil {
		return err
	}
	target.dataReceipt, target.dataCreated = receipt, created
	return nil
}

func (service Service) stageOnePlannedTarget(ctx context.Context, target *plannedGroupTarget, operationID string) error {
	delivery, err := service.stagePackage(ctx, target.input.Envelope, target.plan, operationID, target.input.Hints, target.dataReceipt.Locator)
	if err != nil {
		return err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, target.plan, target.managed)
	if err != nil {
		_ = service.Stager.Discard(context.Background(), delivery)
		return err
	}
	target.delivery = delivery
	return nil
}
