package vscode

import (
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Validate independently recorded selector ownership before reconstructing any
// receipt. A digest checks integrity; neither construction nor a bool owns bytes.
func localRecordedReceipt(selected domain.SelectedDelivery, objects []domain.NativeObjectOwnership, observation *domain.LocalEntryObservation) (*vscodeprofile.Receipt, error) {
	facts, err := recordedLocalFacts(selected)
	if err != nil {
		return nil, err
	}
	owned, err := ownedLocalObjects(facts, objects)
	if err != nil {
		return nil, err
	}
	if observation == nil {
		return nil, nil
	}
	if !owned {
		return nil, fmt.Errorf("local observation requires independently recorded selector ownership")
	}
	binding := domain.ClientBinding{SelectedDelivery: selected, NativeObjects: objects, LocalEntryObservation: observation, TargetLocator: facts.Registration.Selector, NativeProfileRoot: facts.ProfileRoot}
	if err := binding.ValidateLocalEntryObservation(); err != nil {
		return nil, err
	}
	basis, err := recordedLocalFacts(observation.Facts().RevisionBasis)
	if err != nil {
		return nil, err
	}
	o := observation.Facts()
	return &vscodeprofile.Receipt{Version: "1", Selector: "chat.pluginLocations", Identity: profileIdentity(basis), Enabled: o.Enabled, Digest: o.ReceiptDigest}, nil
}

// Only actual parser/Plan results enter the domain carrier. The supplied basis
// is the revision just verified, never a constructor's convenient replacement.
func localObservation(selected domain.SelectedDelivery, receipt *vscodeprofile.Receipt) (*domain.LocalEntryObservation, error) {
	facts, ok := selected.LocalFacts()
	if !ok || facts.ProjectionDigest == "" || receipt == nil || receipt.Version != "1" || receipt.Selector != "chat.pluginLocations" || receipt.Identity != profileIdentity(facts) {
		return nil, fmt.Errorf("local actual receipt differs from sealed revision")
	}
	return domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: selected, Enabled: receipt.Enabled, ReceiptDigest: receipt.Digest})
}

func localSameBasis(selected domain.SelectedDelivery, observation *domain.LocalEntryObservation) bool {
	return observation != nil && reflect.DeepEqual(selected, observation.Facts().RevisionBasis)
}

func registrationResult(result vscodeprofile.Result) RegistrationState {
	if result.Disabled {
		return RegistrationDisabled
	}
	return RegistrationActive
}
