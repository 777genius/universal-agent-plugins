package kiro

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func DeactivateNative(ctx context.Context, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

func applyKiroNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (resultErr error) {
	prepared, err := prepareKiroNativeApply(configRoot, activePath, previous, desired)
	if err != nil {
		return err
	}
	txn, err := newKiroNativeTxn(prepared)
	if err != nil {
		return err
	}
	if txn.transactionRoot != "" {
		defer func() { _ = os.RemoveAll(txn.transactionRoot) }()
	}
	defer func() {
		if resultErr != nil {
			if rollbackErr := txn.rollback(); rollbackErr != nil {
				resultErr = fmt.Errorf("%w; Kiro native rollback failed: %w", resultErr, rollbackErr)
			}
		}
	}()
	if err := txn.stageSkills(); err != nil {
		return err
	}
	if err := txn.prepareMCP(); err != nil {
		return err
	}
	if err := txn.applySkillsAndMCP(); err != nil {
		return err
	}
	return VerifyNativeObjects(prepared.configRoot, prepared.desired, false)
}

type kiroNativeApply struct {
	configRoot   string
	activePath   string
	previous     []domain.NativeObjectOwnership
	desired      []domain.NativeObjectOwnership
	previousByID map[string]domain.NativeObjectOwnership
	desiredByID  map[string]domain.NativeObjectOwnership
}

func prepareKiroNativeApply(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (*kiroNativeApply, error) {
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" {
		return nil, fmt.Errorf("the Kiro config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return nil, err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return nil, fmt.Errorf("the Kiro native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireKiroObjectAbsent(configRoot, object); err != nil {
			return nil, err
		}
	}
	return &kiroNativeApply{
		configRoot: configRoot, activePath: activePath, previous: previous, desired: desired,
		previousByID: previousByID, desiredByID: desiredByID,
	}, nil
}
