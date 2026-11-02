package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (session *repairSession) repairPackage() (AddResult, error) {
	verification, ok := repairMismatchKind(session.verifyErr)
	if !ok {
		return session.result, fmt.Errorf("managed package integrity could not be determined; refusing repair: %w", session.verifyErr)
	}
	if verification.Kind == ports.VerificationDigestMismatch && domain.ClientTraitsFor(session.input.Client.ClientID).ProtectsModifiedPackage {
		return session.result, fmt.Errorf("managed native extension was edited; preserve the changes before repair: %w", session.verifyErr)
	}
	beforeDigest, err := repairBeforeDigest(verification)
	if err != nil {
		return session.result, err
	}
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	delivery, err := session.stageRepairDelivery(false)
	if err != nil {
		return session.result, err
	}
	defer func() { session.service.discardSettledDelivery(session.input.OperationID, delivery) }()
	if err := session.service.verifyRepairPrecondition(session.ctx, session.target.ActivePath, session.expectedDigest, verification.Kind, beforeDigest); err != nil {
		return session.result, err
	}
	if err := session.service.checkMCPNamespace(session.ctx, session.input.Client, &session.plan, &session.client); err != nil {
		return session.result, fmt.Errorf("MCP namespace changed before repair commit: %w", err)
	}
	return session.commitRepairPackage(delivery, beforeDigest)
}

func repairBeforeDigest(verification *ports.VerificationError) (string, error) {
	if verification.Kind != ports.VerificationDigestMismatch {
		return "", nil
	}
	if strings.TrimSpace(verification.ActualDigest) == "" {
		return "", fmt.Errorf("managed package verifier reported a digest mismatch without the actual digest; refusing repair")
	}
	return verification.ActualDigest, nil
}

func (session *repairSession) stageRepairDelivery(allowProjectionChange bool) (domain.StagedDelivery, error) {
	operationID := strings.TrimSpace(session.input.OperationID)
	if operationID == "" {
		id, err := newOperationID()
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		operationID = id
	}
	dataPath, err := session.repairDataPath()
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	delivery, err := session.service.stagePackage(session.ctx, session.input.Envelope, cloneLocalObservationPlan(session.plan), operationID, session.input.Hints, dataPath)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, session.plan, &session.client)
	if err == nil {
		delivery.NativeObjects, err = retainRecordedLocalSelector(session.plan.SelectedDelivery, session.client, delivery.NativeObjects)
	}
	if err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, err
	}
	if delivery.ActivePath != session.target.ActivePath {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, fmt.Errorf("stager returned an unexpected repair target")
	}
	if !allowProjectionChange && delivery.ArtifactDigest != session.expectedDigest {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, fmt.Errorf("resolved repair projection digest differs from the originally managed package")
	}
	if dataPath != "" {
		if err := session.service.PluginData.PrepareRuntime(session.ctx, session.input.Envelope, session.plan, dataPath); err != nil {
			_ = session.service.Stager.Discard(context.Background(), delivery)
			return domain.StagedDelivery{}, fmt.Errorf("prepare locked MCP runtime before repair activation: %w", err)
		}
	}
	if err := sealStagedSelection(&session.plan, delivery); err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, err
	}
	session.result.Plan = session.plan
	session.input.OperationID = operationID
	return delivery, nil
}

func (session *repairSession) repairDataPath() (string, error) {
	if !packageNeedsPluginData(session.input.Envelope, session.plan) {
		return "", nil
	}
	if session.service.PluginData == nil {
		return "", fmt.Errorf("PLUGIN_DATA manager is required for stdio repair")
	}
	receipt, ok := session.installation.DataReceipts[session.client.DataReceiptID]
	if !ok || receipt.DataReceiptID == "" {
		return "", fmt.Errorf("exact repair is missing its owned PLUGIN_DATA receipt; run reviewed state recovery")
	}
	if err := session.service.PluginData.ValidateData(session.ctx, receipt); err != nil {
		return "", fmt.Errorf("validate repair PLUGIN_DATA ownership: %w", err)
	}
	return receipt.Locator, nil
}
