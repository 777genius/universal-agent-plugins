package usecase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// activeNativeDelivery contains the complete desired projection, derived from
// the selected components and a verified installed package. Its native objects
// are never ownership evidence until an adapter confirms the external effect.
func (service Service) activeNativeDelivery(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, client domain.ClientBinding) (domain.StagedDelivery, bool, error) {
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client)}
	if delivery.ArtifactDigest == "" {
		return delivery, false, fmt.Errorf("managed package receipt is missing")
	}
	if !nativeLifecycleClient(input.Client.ClientID, plan.SelectedDelivery) {
		delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)
		return delivery, true, nil
	}
	if !packageRevisionMatches(client.PackageRevision, input.Envelope) {
		return delivery, false, fmt.Errorf("active package revision differs from the requested native projection")
	}
	projector, ok := service.Stager.(ports.ActiveNativeProjector)
	if !ok {
		return delivery, false, fmt.Errorf("stager cannot derive the installed native projection read-only")
	}
	dataPath := ""
	if packageNeedsPluginData(input.Envelope, plan) {
		receipt, found := installation.DataReceipts[client.DataReceiptID]
		if !found || receipt.DataReceiptID == "" || service.PluginData == nil {
			return delivery, false, fmt.Errorf("native projection has no confirmed PLUGIN_DATA receipt")
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return delivery, false, fmt.Errorf("validate native projection PLUGIN_DATA receipt: %w", err)
		}
		dataPath = receipt.Locator
	}
	projected, err := projector.ProjectActiveNative(ctx, input.Envelope, plan, delivery.ArtifactDigest, dataPath)
	if err != nil {
		return delivery, false, err
	}
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			delivery.NativeObjects = append(delivery.NativeObjects, object)
		}
	}
	if len(delivery.NativeObjects) != 1 {
		return delivery, false, fmt.Errorf("active package has no unique managed directory receipt")
	}
	seen := map[string]bool{delivery.NativeObjects[0].ObjectID: true}
	for _, object := range projected {
		if object.ObjectID == "" || object.Kind == "" || seen[object.ObjectID] || object.Kind == "managed_package_directory" {
			return delivery, false, fmt.Errorf("native projection has incomplete or duplicate ownership identity")
		}
		seen[object.ObjectID] = true
		delivery.NativeObjects = append(delivery.NativeObjects, object)
	}
	return delivery, confirmedNativeProjection(client.NativeObjects, delivery.NativeObjects), nil
}

func confirmedNativeProjection(confirmed, desired []domain.NativeObjectOwnership) bool {
	if len(confirmed) != len(desired) {
		return false
	}
	byID := make(map[string]domain.NativeObjectOwnership, len(confirmed))
	for _, object := range confirmed {
		if object.ObjectID == "" || object.Kind == "" {
			return false
		}
		if _, duplicate := byID[object.ObjectID]; duplicate {
			return false
		}
		byID[object.ObjectID] = object
	}
	for _, object := range desired {
		if !reflect.DeepEqual(byID[object.ObjectID], object) {
			return false
		}
	}
	return true
}
