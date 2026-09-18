package usecase

import (
	"fmt"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func upsertPreparedInstallation(
	state domain.StateFileV2,
	installationIndex int,
	existing bool,
	input AddInput,
	plan domain.DeliveryPlan,
	installationID, clientBindingID, sourceBindingID string,
	now time.Time,
) (domain.StateFileV2, int) {
	timestamp := now.Format(time.RFC3339Nano)
	installation := newPreparedInstallation(input, installationID, sourceBindingID, timestamp)
	if existing {
		installation = overlayExistingInstallation(state.Installations[installationIndex], input, sourceBindingID, timestamp)
	}
	if input.Envelope.LocalChatGPTMapping != nil {
		mapping := *input.Envelope.LocalChatGPTMapping
		installation.LocalChatGPTMapping = &mapping
	}
	installation.Clients[clientBindingID] = stagedClientBinding(installation.Clients[clientBindingID], input, plan, clientBindingID, timestamp)
	if existing {
		state.Installations[installationIndex] = installation
		return state, installationIndex
	}
	state.Installations = append(state.Installations, installation)
	return state, len(state.Installations) - 1
}

func newPreparedInstallation(input AddInput, installationID, sourceBindingID, timestamp string) domain.Installation {
	return domain.Installation{
		InstallationID: installationID,
		DeclaredName:   input.Envelope.Manifest.Name,
		Source: domain.SourceBinding{
			SourceBindingID:  sourceBindingID,
			RequestedSource:  input.Envelope.Source.RequestedSource,
			CanonicalSource:  input.Envelope.Source.CanonicalSource,
			Repository:       input.Envelope.Source.Repository,
			PackageSubpath:   input.Envelope.Source.PackageSubpath,
			ResolvedRevision: input.Envelope.Source.ResolvedRevision,
			TreeDigest:       input.Envelope.TreeDigest,
		},
		Package: domain.PackageBinding{
			LoaderKind: input.Envelope.LoaderKind, FormatID: input.Envelope.FormatID,
			SchemaURI: input.Envelope.SchemaURI, DeclaredName: input.Envelope.Manifest.Name,
			Version: input.Envelope.Manifest.Version, ManifestDigest: input.Envelope.ManifestDigest,
			Inventory: input.Envelope.Inventory,
		},
		OriginMode: normalizedOriginMode(input.OriginMode),
		Directory:  cloneDirectoryOrigin(input.DirectoryResolution),
		Clients:    map[string]domain.ClientBinding{},
		CreatedAt:  timestamp,
		UpdatedAt:  timestamp,
	}
}

func overlayExistingInstallation(installation domain.Installation, input AddInput, sourceBindingID, timestamp string) domain.Installation {
	installation.DeclaredName = input.Envelope.Manifest.Name
	installation.Source.ResolvedRevision = input.Envelope.Source.ResolvedRevision
	installation.Source.TreeDigest = input.Envelope.TreeDigest
	if installation.OriginMode == domain.OriginModeDirectory {
		installation.Source = domain.SourceBinding{SourceBindingID: sourceBindingID, RequestedSource: input.Envelope.Source.RequestedSource,
			CanonicalSource: input.Envelope.Source.CanonicalSource, Repository: input.Envelope.Source.Repository, PackageSubpath: input.Envelope.Source.PackageSubpath,
			ResolvedRevision: input.Envelope.Source.ResolvedRevision, TreeDigest: input.Envelope.TreeDigest}
	}
	installation.Package = domain.PackageBinding{
		LoaderKind: input.Envelope.LoaderKind, FormatID: input.Envelope.FormatID,
		SchemaURI: input.Envelope.SchemaURI, DeclaredName: input.Envelope.Manifest.Name,
		Version: input.Envelope.Manifest.Version, ManifestDigest: input.Envelope.ManifestDigest,
		Inventory: input.Envelope.Inventory,
	}
	installation.UpdatedAt = timestamp
	if installation.OriginMode == domain.OriginModeDirectory && input.DirectoryResolution != nil {
		installation.Directory = cloneDirectoryOrigin(input.DirectoryResolution)
	}
	if installation.Clients == nil {
		installation.Clients = map[string]domain.ClientBinding{}
	}
	return installation
}

func stagedClientBinding(previous domain.ClientBinding, input AddInput, plan domain.DeliveryPlan, clientBindingID, timestamp string) domain.ClientBinding {
	bindingClientID := string(input.Client.ClientID)
	if previous.ClientID != "" {
		bindingClientID = previous.ClientID
	}
	return domain.ClientBinding{
		InstallIntent:   input.InstallIntent,
		ClientBindingID: clientBindingID, ClientID: bindingClientID, Scope: string(input.Scope),
		TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID,
		Materialization: domain.MaterializationStaged, Activation: domain.ActivationPrepared,
		Authentication: plan.Authentication, Policy: domain.PolicyAllowed,
		Verification: domain.VerificationPackageValid, UpdatedAt: timestamp,
		PackageRevision:  packageRevisionForInput(input),
		Receipts:         append([]domain.MutationReceipt(nil), previous.Receipts...),
		NativeObjects:    append([]domain.NativeObjectOwnership(nil), previous.NativeObjects...),
		AffectedSurfaces: preparedAffectedSurfaces(previous, input.Client.ClientID),
	}
}

func preparedAffectedSurfaces(previous domain.ClientBinding, requested domain.ClientID) []string {
	values := append([]string(nil), previous.AffectedSurfaces...)
	if previous.ClientID != "" {
		values = append(values, previous.ClientID)
	}
	values = append(values, string(requested))
	for _, sibling := range domain.BackendSiblings(requested) {
		values = append(values, string(sibling))
	}
	return uniqueSortedSurfaces(values)
}
func findSourceInstallation(state domain.StateFileV2, sourceBindingID string) (int, bool) {
	for index, installation := range state.Installations {
		if installation.Source.SourceBindingID == sourceBindingID {
			return index, true
		}
	}
	return -1, false
}

func findStickyInstallation(state domain.StateFileV2, input AddInput, sourceBindingID string) (int, bool, bool, error) {
	if selector := strings.TrimSpace(input.InstallationID); selector != "" {
		for index, installation := range state.Installations {
			if installation.InstallationID == selector {
				return index, true, true, nil
			}
		}
		// An explicit, unused installation ID denotes a new installation. Do not
		// silently reinterpret it as a request to mutate a same-name binding;
		// backend collision checks below will fail closed with the useful native
		// identity diagnostic.
		return -1, false, false, nil
	}
	if input.DirectoryResolution != nil && input.DirectoryResolution.ProductID != "" {
		for index, installation := range state.Installations {
			if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && installation.Directory.ProductID == input.DirectoryResolution.ProductID {
				return index, true, true, nil
			}
		}
	}
	nameMatch := -1
	for index, installation := range state.Installations {
		if installation.Source.SourceBindingID == sourceBindingID {
			return index, true, false, nil
		}
		if installation.DeclaredName == input.Envelope.Manifest.Name {
			if nameMatch >= 0 {
				return -1, false, false, fmt.Errorf("installed manifest identity %q is ambiguous; use installation_id", input.Envelope.Manifest.Name)
			}
			nameMatch = index
		}
	}
	if nameMatch >= 0 {
		return nameMatch, true, true, nil
	}
	return -1, false, false, nil
}
func materializedClient(installation domain.Installation, clientBindingID string) bool {
	client, ok := installation.Clients[clientBindingID]
	return ok && client.Materialization != domain.MaterializationAbsent
}

func rejectNativeNameCollision(state domain.StateFileV2, installationID, declaredName string, clientID domain.ClientID) error {
	for _, installation := range state.Installations {
		if installation.InstallationID == installationID || installation.DeclaredName != declaredName {
			continue
		}
		for _, client := range installation.Clients {
			boundClientID := domain.ClientID(client.ClientID)
			if sameNativeBackend(boundClientID, clientID) && client.Materialization != domain.MaterializationAbsent {
				return fmt.Errorf("native client name collision for %q on %s; remove or rebind the existing source first", declaredName, clientID)
			}
		}
	}
	return nil
}

func sameNativeBackend(first, second domain.ClientID) bool {
	return domain.SameClientBackend(first, second)
}
