package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func openAIOAuthApplies(clientID domain.ClientID, envelope domain.PackageEnvelope, hints domain.CompatibilityHints) bool {
	if !domain.ClientTraitsFor(clientID).HonorsOpenAIMCPAuthHints {
		return false
	}
	if envelope.CatalogEvidence != nil {
		if _, present := envelope.CatalogEvidence.Compatibility[string(clientID)]; present {
			return false
		}
	}
	if _, present := hints.Compatibility[string(clientID)]; present {
		return false
	}
	for serverName := range envelope.MCP.Servers {
		if strings.TrimSpace(hints.OpenAIMCPAuth[serverName].OAuthResource) != "" {
			return true
		}
	}
	return false
}

// bindStagedDeliveryToPhysicalOwner keeps a shared backend's immutable
// ownership receipt bound to the client identity that originally created the
// physical binding. The requested logical surface remains on the delivery and
// plan so activation and user-facing results still describe what the caller
// selected.
func bindStagedDeliveryToPhysicalOwner(delivery domain.StagedDelivery, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.StagedDelivery, error) {
	if !sharesPhysicalBackend(plan.ClientID) {
		return delivery, nil
	}
	owner := plan.ClientID
	if managed != nil {
		owner = domain.ClientID(managed.ClientID)
		if !sameNativeBackend(owner, plan.ClientID) || managed.PhysicalArtifact != plan.PhysicalArtifactID {
			return delivery, fmt.Errorf("shared physical binding identity does not match the staged delivery")
		}
	}
	expected := "package:" + string(plan.ClientID) + ":" + plan.PhysicalArtifactID
	canonical := "package:" + string(owner) + ":" + plan.PhysicalArtifactID
	found := false
	for index := range delivery.NativeObjects {
		object := &delivery.NativeObjects[index]
		if object.Kind != "managed_package_directory" {
			continue
		}
		if found || object.ObjectID != expected {
			return delivery, fmt.Errorf("shared staged delivery has an invalid managed package ownership identity")
		}
		object.ObjectID = canonical
		found = true
	}
	if !found {
		return delivery, fmt.Errorf("shared staged delivery is missing its managed package ownership identity")
	}
	return delivery, nil
}

func initialLifecycle(plan domain.DeliveryPlan) (domain.ActivationState, domain.VerificationState) {
	return plan.Activation, plan.Verification
}

func nextSequence(client domain.ClientBinding) int {
	maximum := 0
	for _, receipt := range client.Receipts {
		if receipt.Sequence > maximum {
			maximum = receipt.Sequence
		}
	}
	return maximum + 1
}

func managedDigest(client domain.ClientBinding) string {
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" && object.ManagedDigest != "" {
			return object.ManagedDigest
		}
	}
	return ""
}

func newOperationID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate operation id: %w", err)
	}
	return "op-" + hex.EncodeToString(random[:]), nil
}

func packageNeedsPluginData(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) bool {
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type == "stdio" {
			return true
		}
	}
	return false
}

func (service Service) preflightActivation(input AddInput, plan domain.DeliveryPlan) error {
	preflighter, ok := service.Activator.(ports.ActivationPreflighter)
	if !ok {
		return nil
	}
	return preflighter.PreflightActivation(domain.ActivationRequest{
		Client: input.Client, Plan: plan, BackendExecutable: input.BackendExecutable,
		VerifyOnly: input.DryRun,
	})
}
