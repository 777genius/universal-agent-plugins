package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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
	profileRoot := previous.NativeProfileRoot
	if token := plan.ProfileAuthority(); token != nil {
		profileRoot = token.Facts().CanonicalRoot
	}
	if profileRoot == "" && plan.SelectedDelivery.EffectiveTraits(input.Client.ClientID).BindsNativeProfileRoot {
		profileRoot = input.Client.ConfigRoot
		if facts, ok := plan.SelectedDelivery.LocalFacts(); ok {
			profileRoot = facts.ProfileRoot
		}
	}
	return domain.ClientBinding{
		ProfileAuthority: plan.ProfileAuthority(), ProfileNamespace: plan.ProfileNamespace(),
		SelectedDelivery:      plan.SelectedDelivery,
		LocalEntryObservation: previous.LocalEntryObservation.Clone(),
		PendingNativeIntent:   previous.PendingNativeIntent.Clone(),
		InstallIntent:         input.InstallIntent,
		ClientBindingID:       clientBindingID, ClientID: bindingClientID, Scope: string(input.Scope),
		TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID,
		Materialization: domain.MaterializationStaged, Activation: domain.ActivationPrepared,
		Authentication: plan.Authentication, Policy: domain.PolicyAllowed,
		Verification: domain.VerificationPackageValid, UpdatedAt: timestamp,
		PackageRevision:         packageRevisionForInput(input),
		Receipts:                append([]domain.MutationReceipt(nil), previous.Receipts...),
		NativeObjects:           append([]domain.NativeObjectOwnership(nil), previous.NativeObjects...),
		NativeProfileRoot:       profileRoot,
		NativeActivationAttempt: previous.NativeActivationAttempt,
		AffectedSurfaces:        preparedAffectedSurfaces(previous, input.Client.ClientID, plan.SelectedDelivery),
	}
}

func preparedAffectedSurfaces(previous domain.ClientBinding, requested domain.ClientID, selected domain.SelectedDelivery) []string {
	values := append([]string(nil), previous.AffectedSurfaces...)
	if previous.ClientID != "" {
		values = append(values, previous.ClientID)
	}
	values = append(values, string(requested))
	if selected.SharesBackend(requested) {
		for _, sibling := range domain.BackendSiblings(requested) {
			values = append(values, string(sibling))
		}
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

func (service Service) authorityPort() ports.PhysicalProfileAuthority {
	if service.PhysicalAuthority != nil {
		return service.PhysicalAuthority
	}
	port, _ := service.Planner.(ports.PhysicalProfileAuthority)
	return port
}
func (service Service) checkProfiles(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if service.profileCheck != nil {
		return service.profileCheck()
	}
	for _, c := range service.PhysicalProfiles {
		if err := service.checkProfile(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (service Service) checkProfile(ctx context.Context, c domain.DetectedClient) error {
	token := domain.ProfileAuthority{}
	if c.ProfileAuthority != nil {
		token = *c.ProfileAuthority
		if token.IsZero() {
			return fmt.Errorf("opted profile token is missing")
		}
		if c.ProfileNamespace == "" || c.ProfileNamespace != service.Kernel.Namespace {
			return fmt.Errorf("physical profile namespace differs")
		}
	}
	port := service.authorityPort()
	if port == nil {
		if !token.IsZero() {
			return fmt.Errorf("physical profile verifier is required")
		}
		return nil
	}
	return port.RevalidateProfileAuthority(ctx, c.ClientID, token)
}
func (service Service) freezeProfiles(ctx context.Context, installationID string, selected []domain.DetectedClient, capture bool) (Service, []domain.DetectedClient, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return service, nil, err
	}
	frozen := append([]domain.DetectedClient(nil), selected...)
	opted := false
	shared, err := service.sharedProfiles(ctx, state, installationID)
	if err != nil {
		return service, nil, err
	}
	for _, c := range shared {
		opted = opted || c.ProfileAuthority != nil
	}
	for i, c := range frozen {
		current, err := service.freezeProfile(ctx, state, installationID, c, capture)
		if err != nil {
			return service, nil, err
		}
		frozen[i] = current
		opted = opted || current.ProfileAuthority != nil
	}
	if !opted {
		return service, frozen, nil
	}
	return service.guardFrozenProfiles(ctx, frozen, shared), frozen, nil
}

func (service Service) dataProfileOwners(dataID string) []domain.PhysicalProfileOwner {
	state, err := service.StateStore.Load()
	if err != nil {
		return nil
	}
	var owners []domain.PhysicalProfileOwner
	for _, i := range state.Installations {
		for key, b := range i.Clients {
			if b.DataReceiptID == dataID && b.ProfileAuthority != nil {
				owners = append(owners, domain.PhysicalProfileOwner{Namespace: b.ProfileNamespace, InstallationID: i.InstallationID, ClientID: b.ClientID, ClientBindingID: key, Authority: domain.CloneProfileAuthority(b.ProfileAuthority)})
			}
		}
	}
	return owners
}

func (service Service) sharedProfiles(ctx context.Context, state domain.StateFileV2, installationID string) ([]domain.DetectedClient, error) {
	var shared []domain.DetectedClient
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, b := range installation.Clients {
			if b.PhysicalArtifact != domain.ComputePhysicalArtifactID(installation.DeclaredName, installationID) {
				continue
			}
			if key != b.ClientBindingID {
				return nil, fmt.Errorf("shared data binding key differs")
			}
			c := domain.DetectedClient{ClientID: domain.ClientID(b.ClientID), ConfigRoot: b.NativeProfileRoot, ProfileAuthority: domain.CloneProfileAuthority(b.ProfileAuthority), ProfileNamespace: b.ProfileNamespace}
			if err := service.checkProfile(ctx, c); err != nil {
				return nil, err
			}
			shared = append(shared, c)
		}
	}
	return shared, nil
}
func recordedProfile(state domain.StateFileV2, installationID string, c domain.DetectedClient) (*domain.ClientBinding, error) {
	var recorded *domain.ClientBinding
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, b := range installation.Clients {
			if b.ClientID != string(c.ClientID) || b.Scope != string(domain.ScopeUser) {
				continue
			}
			if key != b.ClientBindingID || recorded != nil {
				return nil, fmt.Errorf("physical owner binding is ambiguous or malformed")
			}
			bindingCopy := b
			recorded = &bindingCopy
		}
	}
	return recorded, nil
}
func (service Service) freezeProfile(ctx context.Context, state domain.StateFileV2, installationID string, c domain.DetectedClient, capture bool) (domain.DetectedClient, error) {
	recorded, err := recordedProfile(state, installationID, c)
	if err != nil {
		return c, err
	}
	if recorded != nil {
		if c.ProfileAuthority != nil && !domain.SameProfileAuthority(c.ProfileAuthority, recorded.ProfileAuthority) {
			return c, fmt.Errorf("caller profile differs from recorded owner")
		}
		c.ProfileAuthority = domain.CloneProfileAuthority(recorded.ProfileAuthority)
		c.ProfileNamespace = recorded.ProfileNamespace
	} else if c.ProfileAuthority == nil && capture && service.authorityPort() != nil {
		token, err := service.authorityPort().CaptureProfileAuthority(ctx, c)
		if err != nil {
			return c, err
		}
		if !token.IsZero() {
			c.ProfileAuthority = &token
			c.ProfileNamespace = service.Kernel.Namespace
		}
	}
	if c.ProfileAuthority != nil {
		root := c.ProfileAuthority.Facts().CanonicalRoot
		spelling, err := filepath.EvalSymlinks(c.ConfigRoot)
		if err != nil || spelling != root {
			return c, fmt.Errorf("selected profile differs from recorded canonical root")
		}
		c.ConfigRoot = root
	}
	if err := service.checkProfile(ctx, c); err != nil {
		return c, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	return c, nil
}
