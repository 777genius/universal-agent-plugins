package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func materializeLockedPackage(ctx context.Context, app App, locked environment.LockedPackage, publish bool, recorded *domain.RecordedDirectoryRelease, operation ...domain.DirectoryOperation) (loadedPackage, error) {
	if source := locked.Source.Git; source != nil && source.Kind == environment.SkillKind && source.Normalizer != environment.SkillNormalizerV1 {
		return loadedPackage{}, errors.New("unsupported locked skill normalizer")
	}
	store := app.dependencyStore()
	if pkg, err := store.Get(ctx, locked.DigestAlgorithm, locked.TreeDigest); err == nil {
		return loadStoredDependency(ctx, app, pkg, locked, recorded, operation...)
	} else if !errors.Is(err, environmentstore.ErrMissing) {
		return loadedPackage{}, err
	}
	snapshot, err := app.acquireLockedSnapshot(ctx, locked)
	if err != nil {
		return loadedPackage{}, err
	}
	snapshot.Source = identityFromLocked(locked)
	origin, directory := domain.OriginModeDirect, (*domain.DirectoryOrigin)(nil)
	if locked.Source.Registry != nil {
		origin = domain.OriginModeDirectory
		source := locked.Source.Registry
		directory = &domain.DirectoryOrigin{ProductID: source.ProductID, DistributionID: source.DistributionID,
			DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: source.ReleaseSequence}
	}
	loaded, err := app.loadStandardSnapshot(ctx, snapshot, origin, directory, nil)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.TreeDigest != locked.TreeDigest || loaded.envelope.ManifestDigest != locked.ManifestDigest {
		_ = loaded.cleanup()
		return loadedPackage{}, errors.New("immutable fetch does not match uap.lock")
	}
	checked, err := verifyLockedDirectory(ctx, app, loaded, locked, recorded, operation...)
	if err != nil {
		_ = loaded.cleanup()
		return loadedPackage{}, err
	}
	loaded = checked
	if publish {
		sealed := domain.PackageSnapshot{Root: loaded.envelope.SnapshotRoot, TreeDigest: loaded.envelope.TreeDigest,
			DigestAlgorithm: locked.DigestAlgorithm, ExecutableFiles: append([]string{}, locked.ExecutableFiles...), Source: loaded.envelope.Source}
		_, err = store.Put(ctx, sealed)
	} else {
		return loaded, nil
	}
	cleanupErr := loaded.cleanup()
	if err != nil {
		return loadedPackage{}, err
	}
	if cleanupErr != nil {
		return loadedPackage{}, cleanupErr
	}
	pkg, err := store.Get(ctx, locked.DigestAlgorithm, locked.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	return loadStoredDependency(ctx, app, pkg, locked, recorded, operation...)
}

func dependencySourceHint(locked environment.LockedPackage) string {
	if locked.Source.Web != nil {
		return "uap-dependency-web-skill"
	}
	if locked.Source.Git != nil && locked.Source.Git.Kind == environment.SkillKind {
		return "uap-dependency-skill"
	}
	if locked.Source.Registry != nil {
		return "uap-dependency-registry"
	}
	return "uap-dependency-git"
}

func loadStoredDependency(ctx context.Context, app App, pkg environmentstore.Package, locked environment.LockedPackage, recorded *domain.RecordedDirectoryRelease, operation ...domain.DirectoryOperation) (loadedPackage, error) {
	pkg.Metadata.Source = identityFromLocked(locked)
	input := domain.LoadInput{SnapshotRoot: pkg.Root, TreeDigest: pkg.Metadata.TreeDigest,
		ExecutableFiles: append([]string{}, pkg.Metadata.ExecutableFiles...), Source: pkg.Metadata.Source}
	envelope, err := app.loadStandardEnvelope(ctx, input)
	if err != nil {
		return loadedPackage{}, err
	}
	if _, err := packageDependencyFacts(envelope); err != nil {
		return loadedPackage{}, err
	}
	if envelope.LoaderKind != domain.LoaderKindAgentPlugins || envelope.TreeDigest != locked.TreeDigest ||
		envelope.ManifestDigest != locked.ManifestDigest {
		return loadedPackage{}, errors.New("stored dependency failed strict standard-loader identity checks")
	}
	var assessment *domain.SecurityAssessment
	if app.SecurityEvaluator != nil {
		result, securityErr := app.SecurityEvaluator.Evaluate(ctx, domain.SecurityEvaluationInput{
			SnapshotRoot: pkg.Root, TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest,
			Commands: envelope.Commands, PluginName: envelope.Manifest.Name,
		})
		if securityErr != nil {
			return loadedPackage{}, fmt.Errorf("security assessment failed before dependency sync: %w", securityErr)
		}
		assessment = &result
	}
	origin := domain.OriginModeDirect
	var directory *domain.DirectoryOrigin
	if locked.Source.Registry != nil {
		origin = domain.OriginModeDirectory
		source := locked.Source.Registry
		directory = &domain.DirectoryOrigin{ProductID: source.ProductID, DistributionID: source.DistributionID,
			DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: source.ReleaseSequence}
	}
	return verifyLockedDirectory(ctx, app, loadedPackage{envelope: envelope, security: assessment, origin: origin, directory: directory}, locked, recorded, operation...)
}

func preflightDependencyActions(cmd *cobra.Command, app App, opts *options, actions []dependencyAction, detected map[domain.ClientID]domain.DetectedClient, consent ...map[string]bool) ([]dependencyAction, error) {
	adapter := &dependencyInstaller{app: app, opts: opts, detected: detected, cmd: cmd}
	if len(consent) > 0 {
		adapter.consent = consent[0]
	}
	return preflightDependencyAdapter(cmd.Context(), adapter, actions)
}

func preflightDependencyAdapter(ctx context.Context, adapter *dependencyInstaller, actions []dependencyAction) ([]dependencyAction, error) {
	opts := adapter.opts
	claims := map[string]string{}
	updates := map[string]*installer.PreparedOperation{}
	defer func() {
		for _, update := range updates {
			_ = update.Close()
		}
	}()
	for _, action := range actions {
		if action.Action == environment.ActionUnchanged {
			continue
		}
		adapter.previewUpdate = nil
		if action.Action == environment.ActionInstall {
			adapter.previewUpdate = updates[action.Alias]
		}
		prepared, err := adapter.Prepare(ctx, action)
		if err != nil {
			return nil, fmt.Errorf("preflight %s/%s: %w", action.Alias, action.ClientID, err)
		}
		plan := prepared.Plan()
		if adapter.previewPaths != nil {
			adapter.previewPaths[action.Alias+"\x00"+action.ClientID] = plan.TargetPath
		}
		if opts.dependencyComponents == nil {
			opts.dependencyComponents = map[string][]domain.ComponentDecision{}
		}
		components := make([]domain.ComponentDecision, 0, len(plan.Delivery.Components))
		for _, component := range plan.Delivery.Components {
			components = append(components, domain.ComponentDecision{Kind: domain.ComponentKind(component.Kind), Name: component.Name, Support: domain.SupportLevel(component.Support), Reason: component.Reason, Source: component.Source, SourceName: component.SourceName})
		}
		opts.dependencyComponents[action.Alias+"\x00"+action.ClientID] = components

		if action.Action == environment.ActionUpdate && updates[action.Alias] == nil {
			updates[action.Alias] = prepared
		} else {
			_ = prepared.Close()
		}
		key := action.ClientID + "\x00" + plan.TargetPath
		if previous := claims[key]; previous != "" && previous != action.Alias {
			return nil, errors.New("conflicting dependency namespace claims")
		}
		claims[key] = action.Alias
	}
	return actions, nil
}

type dependencyInstaller struct {
	previewWarnings *[]string
	previewPaths    map[string]string
	app             App
	opts            *options
	detected        map[domain.ClientID]domain.DetectedClient
	cmd             *cobra.Command
	beforeApply     func() error
	consent         map[string]bool
	previewUpdate   *installer.PreparedOperation
	operationIDs    map[string]string
	onResult        func(environment.PreparedBinding, installer.Result)
	currentAction   environment.PreparedBinding
}

func (adapter *dependencyInstaller) Prepare(ctx context.Context, action environment.PreparedBinding) (*installer.PreparedOperation, error) {
	if adapter.beforeApply != nil {
		if err := adapter.beforeApply(); err != nil {
			return nil, err
		}
	}
	if action.Authority != nil {
		cloned := *action.Authority
		action.Authority = &cloned
	}
	client, ok := adapter.detected[domain.ClientID(action.ClientID)]
	if !ok || (action.Action != environment.ActionRemove && (client.Status != domain.DetectionDetected || client.DetectionError != nil)) {
		return nil, fmt.Errorf("target %s is not detected; nothing was changed", action.ClientID)
	}
	view, err := adapter.app.Installer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	if view.Recovery.Required {
		return nil, installer.ErrRecoveryRequired
	}
	// A preceding target may have created the shared installation. Resolve
	// identity only from this alias's live owner receipt, never a package name.
	if err := resolveDependencyBindingIdentity(view, &action); err != nil {
		return nil, err
	}
	operationID := adapter.operationIDs[action.Alias+"\x00"+action.ClientID]
	if operationID == "" {
		var err error
		operationID, err = domain.NewInstallationID()
		if err != nil {
			return nil, err
		}
	}
	adapter.currentAction = action
	request := installer.Request{OperationID: operationID, Operation: installer.Operation(action.Action), ClientID: action.ClientID, ClientConfigRoot: client.ConfigRoot,
		ClientExecutable: backendExecutable(client, adapter.detected), InstallationID: action.InstallationID,
		Selector: action.InstallationID, EnvironmentAuthority: action.Authority, ExternalUninstalled: action.Action == environment.ActionRemove && adapter.opts.externalUninstalled, ActivationComplete: action.ActivationComplete, AuthComplete: action.AuthComplete}
	if err := adapter.populateDependencyTargets(view, action.InstallationID, &request); err != nil {
		return nil, err
	}
	if action.Action == environment.ActionRemove {
		return adapter.app.Installer.Prepare(ctx, request)
	}
	recorded, err := adapter.recordedDependencyRelease(action)
	if err != nil {
		return nil, err
	}
	return adapter.prepareDependencyPackage(ctx, action, request, recorded)
}

func (adapter *dependencyInstaller) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	if adapter.beforeApply != nil {
		if err := adapter.beforeApply(); err != nil {
			return installer.Result{}, err
		}
	}
	result, err := adapter.app.Installer.Apply(ctx, prepared, installer.Decision{Confirmed: true})
	if adapter.onResult != nil {
		adapter.onResult(adapter.currentAction, result)
	}
	return result, err
}

// Recheck the exact signed publication for lock restoration. Superseded releases
// may be restored, while installed-state exceptions still require live proof.
func verifyLockedDirectory(ctx context.Context, app App, loaded loadedPackage, locked environment.LockedPackage, recorded *domain.RecordedDirectoryRelease, operation ...domain.DirectoryOperation) (loadedPackage, error) {
	source := locked.Source.Registry
	if source == nil {
		return loaded, nil
	}
	if app.DirectoryClient == nil {
		return loadedPackage{}, errors.New("signed Directory is unavailable")
	}
	bundle, err := app.DirectoryClient.Load(ctx, 0)
	if err != nil {
		return loadedPackage{}, err
	}
	var targets []domain.ClientID
	for _, target := range locked.Targets {
		targets = append(targets, domain.ClientID(target))
	}
	evidenceEnv := directoryEnvironment(nil)
	evidenceEnv.InstallerVersion = app.Version
	evidenceEnv.DependencyIdentity = deterministicDependencyIdentity(loaded.envelope, targets)
	request := domain.DirectoryResolveRequest{Selector: source.DistributionID, DesiredReleaseSequence: source.ReleaseSequence,
		Operation: domain.DirectoryInstall, Scope: domain.ScopeUser, Targets: targets, SchemaVersion: "1.0.0", InstallerVersion: app.Version,
		OS: evidenceEnv.OS, Architecture: evidenceEnv.Architecture, DependencyIdentity: evidenceEnv.DependencyIdentity}
	if recorded == nil {
		request.Purpose = domain.DirectoryResolveLockedRestore
	}
	if recorded != nil {
		request.DesiredReleaseSequence = 0
		request.Recorded = recorded
		request.Operation = domain.DirectoryRepair
		if len(operation) > 0 {
			request.Operation = operation[0]
		}
	}
	selection, err := domain.ResolveDirectory(bundle.Snapshot, request)
	if err != nil {
		return loadedPackage{}, err
	}
	if selection.ProductID != source.ProductID || selection.DistributionID != source.DistributionID || selection.ReleaseSequence != source.ReleaseSequence || selection.Source.Repository != source.Repository || selection.Source.Path != source.Path || selection.Source.Revision != source.Commit || selection.TreeDigest != locked.TreeDigest || selection.ManifestDigest != locked.ManifestDigest {
		return loadedPackage{}, errors.New("signed Directory publication differs from exact lock")
	}
	_, _, _, policy := directoryRecords(bundle.Snapshot, selection)
	if policy == nil {
		return loadedPackage{}, errors.New("directory policy missing")
	}
	if err := validateDirectoryCompatibilityPolicy(*policy); err != nil {
		return loadedPackage{}, err
	}
	loaded.directory = &domain.DirectoryOrigin{ProductID: selection.ProductID, DistributionID: selection.DistributionID, DistributionKind: selection.DistributionKind, DesiredReleaseSequence: selection.ReleaseSequence, SnapshotSchema: directoryv1.SnapshotSchemaVersion, SnapshotSequence: selection.SnapshotSequence, SnapshotDigest: bundle.Digest}
	loaded.directorySelection = &selection
	if err := applyDirectoryCompatibility(&loaded, bundle, selection, *policy, evidenceEnv); err != nil {
		return loadedPackage{}, err
	}
	return loaded, nil
}

// repairOwnedDependencies retains the ordinary repair command and confirmation
// policy, but uses the recorded per-binding pin through the public SDK.
func repairOwnedDependencies(cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) (bool, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return false, err
	}
	installation, found := locallyMatchedInstallation(state, selector)
	if !found {
		return false, nil
	}
	owned := false
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.InstallationID == installation.InstallationID {
			owned = true
		}
	}
	if !owned {
		return false, nil
	}
	if opts.scope != "" && opts.scope != "user" {
		return true, errors.New("environment repair supports user scope only")
	}
	if app.Installer == nil {
		return true, errors.New("typed SDK installer is unavailable")
	}
	preview, cleanup, err := dependencyPreviewApp(app, opts)
	if err != nil {
		return true, err
	}
	defer cleanup()
	app = preview
	clients, err := app.Detector.Detect(cmd.Context())
	if err != nil {
		return true, err
	}
	detected := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	actions, err := buildDependencyRepairs(cmd.Context(), app, state, installation, targets)
	if err != nil {
		return true, err
	}
	return true, applyDependencyRepairs(cmd, app, opts, actions, detected)
}

func resolveDependencyBindingIdentity(view installer.Inspection, action *environment.PreparedBinding) error {
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.Alias == action.Alias {
			if action.InstallationID == "" {
				action.InstallationID = receipt.InstallationID
				action.Authority.InstallationID = receipt.InstallationID
			}
			if receipt.ClientID == action.ClientID && action.Action == environment.ActionInstall {
				return installer.ErrPlanChanged
			}
		}
	}
	return nil
}

func (adapter *dependencyInstaller) populateDependencyTargets(view installer.Inspection, installationID string, request *installer.Request) error {
	for _, installation := range view.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, binding := range installation.Bindings {
			sibling, ok := adapter.detected[domain.ClientID(binding.ClientID)]
			if !ok {
				return fmt.Errorf("installed sibling %s is not detected", binding.ClientID)
			}
			request.KnownTargets = append(request.KnownTargets, installer.TargetFacts{ClientID: binding.ClientID, BindingID: binding.BindingID, ConfigRoot: sibling.ConfigRoot, Executable: backendExecutable(sibling, adapter.detected)})
		}
	}
	return nil
}

func (adapter *dependencyInstaller) recordedDependencyRelease(action environment.PreparedBinding) (*domain.RecordedDirectoryRelease, error) {
	var recorded *domain.RecordedDirectoryRelease
	if action.Package.Source.Registry != nil && (action.Action == "repair" || action.Action == environment.ActionInstall && action.InstallationID != "") {
		state, err := adapter.app.StateStore.Load()
		if err != nil {
			return nil, err
		}
		installation, found := locallyMatchedInstallation(state, action.InstallationID)
		if !found || installation.Directory == nil {
			return nil, installer.ErrPlanChanged
		}
		if action.Action == environment.ActionInstall {
			// A live owned installation, rather than the copied lock, is the authority
			// for the signed NewTarget exception. A changing pin still needs fresh
			// install eligibility during preflight until the preceding update commits.
			if installation.Source.TreeDigest != action.Package.TreeDigest || installation.Directory.DesiredReleaseSequence != action.Package.Source.Registry.ReleaseSequence {
				return nil, nil
			}
			return recordedDirectoryRelease(installation, action.Package.Source.Registry.ReleaseSequence, action.Package.Source.Registry.Commit), nil
		}
		binding, found := installation.Clients[action.BindingID]
		if !found || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != action.Package.TreeDigest {
			return nil, installer.ErrPlanChanged
		}
		revision := binding.PackageRevision
		recorded = recordedDirectoryRelease(installation, revision.ReleaseSequence, revision.ResolvedRevision)
	}
	return recorded, nil
}

func (adapter *dependencyInstaller) prepareDependencyPackage(ctx context.Context, action environment.PreparedBinding, request installer.Request, recorded *domain.RecordedDirectoryRelease) (*installer.PreparedOperation, error) {
	operation := domain.DirectoryRepair
	if action.Action == environment.ActionInstall {
		operation = domain.DirectoryNewTarget
	}
	loaded, err := materializeLockedPackage(ctx, adapter.app, *action.Package, false, recorded, operation)
	if err != nil {
		return nil, err
	}
	if loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	if loaded.security == nil {
		return nil, errors.New("scanner unavailable: dependency sync requires a digest-bound assessment")
	}
	if adapter.consent[loaded.envelope.TreeDigest] {
		loaded.securityAuthorized = true
	}
	if err := adapter.authorize(ctx, &loaded); err != nil {
		return nil, err
	}
	if adapter.consent != nil {
		adapter.consent[loaded.envelope.TreeDigest] = loaded.securityAuthorized
	}
	if action.Action == "repair" && action.Package.ExecutableFiles == nil {
		action.Package.ExecutableFiles = append([]string{}, loaded.envelope.ExecutableFiles...)
	}
	request.PackageRoot = loaded.envelope.SnapshotRoot
	request.ExecutableFiles = append([]string{}, action.Package.ExecutableFiles...)
	request.AcquiredSnapshot = &domain.AcquiredSnapshot{Source: loaded.envelope.Source, TreeDigest: action.Package.TreeDigest,
		DigestAlgorithm: action.Package.DigestAlgorithm, ExecutableFiles: append([]string{}, action.Package.ExecutableFiles...),
		OriginMode: loaded.origin, Directory: loaded.directory, Compatibility: loaded.hints}
	request.Assessment = &installer.Assessment{TreeDigest: action.Package.TreeDigest, Outcome: installer.AssessmentAllow, Reason: "authorized by CLI security policy for exact pinned bytes"}
	request.RequiredComponents = requiredFacadeComponents(loaded.envelope)
	if adapter.previewUpdate != nil {
		return adapter.app.Installer.PreviewInstallAfterUpdate(ctx, request, adapter.previewUpdate)
	}
	return adapter.app.Installer.Prepare(ctx, request)
}
