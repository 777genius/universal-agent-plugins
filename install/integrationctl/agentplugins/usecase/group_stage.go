package usecase

import (
	"context"
	"fmt"
)

func (session *groupSession) cleanupStaged() {
	for _, target := range session.planned {
		if target.delivery.StagingPath != "" {
			_ = session.service.Stager.Discard(context.Background(), target.delivery)
		}
		if target.dataCreated {
			_ = session.service.PluginData.PurgeData(context.Background(), target.dataReceipt)
		}
	}
}

func (session *groupSession) stageGroupDeliveries() error {
	for targetIndex := range session.planned {
		target := &session.planned[targetIndex]
		if target.noChange {
			continue
		}
		if err := session.stageOneGroupDelivery(targetIndex, target); err != nil {
			session.cleanupStaged()
			return err
		}
	}
	return nil
}

func (session *groupSession) stageOneGroupDelivery(targetIndex int, target *plannedGroupTarget) error {
	operationID := fmt.Sprintf("%s-%03d", session.groupID, targetIndex+1)
	if packageNeedsPluginData(target.input.Envelope, target.plan) {
		if session.service.PluginData == nil {
			return fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
		}
		receipt, created, err := session.service.PluginData.EnsureData(session.ctx, session.installationID, target.plan.PhysicalArtifactID, string(target.input.Scope))
		if err != nil {
			return err
		}
		target.dataReceipt, target.dataCreated = receipt, created
	}
	delivery, err := session.service.stagePackage(session.ctx, target.input.Envelope, target.plan, operationID, target.input.Hints, target.dataReceipt.Locator)
	if err != nil {
		return err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, target.plan, target.managed)
	if err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return err
	}
	target.delivery = delivery
	return nil
}

func (session *groupSession) reobserveGroupIdentity() error {
	for _, target := range session.planned {
		if target.recovering {
			if err := session.reobserveRecoveringTarget(target); err != nil {
				return err
			}
			continue
		}
		if err := session.service.observeGroupNativeIdentity(session.ctx, target.input.Client, target.plan, target.managed, session.input.Repair); err != nil {
			return fmt.Errorf("native identity changed before group commit: %w", err)
		}
	}
	return nil
}

func (session *groupSession) reobserveRecoveringTarget(target plannedGroupTarget) error {
	// The recovering target's directory is still absent at this point; its
	// native registry is expected to keep failing until the group's
	// directories are actually restored. Re-confirm, from the filesystem
	// alone, that nothing has since occupied the target: a genuine native
	// verification happens once, after restoration, in PostApplyVerify.
	if !session.service.observeGroupRecoveryEligibility(session.ctx, target.input.Client, target.plan, target.managed) {
		return fmt.Errorf("native identity changed before group commit: recorded absent target %s is no longer eligible for recovery", target.input.Client.ClientID)
	}
	// The staged reconstruction must reproduce the exact prior receipt
	// digest, not merely a new, self-consistent build: repair only ever
	// authorizes restoring what was already recorded as owned.
	if expected := managedDigest(*target.managed); expected == "" || target.delivery.ArtifactDigest != expected {
		return fmt.Errorf("staged reconstruction for %s does not match the recorded package digest", target.input.Client.ClientID)
	}
	return nil
}
