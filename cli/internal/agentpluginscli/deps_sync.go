package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/spf13/cobra"
)

func materializeLockedPackage(ctx context.Context, app App, locked environment.LockedPackage, publish bool, recorded *domain.RecordedDirectoryRelease) (loadedPackage, error) {
	store := app.dependencyStore()
	if pkg, err := store.Get(ctx, locked.DigestAlgorithm, locked.TreeDigest); err == nil {
		return loadStoredDependency(ctx, app, pkg, locked, recorded)
	}
	repository, commit, packagePath := "", "", ""
	if locked.Source.Git != nil {
		repository, commit, packagePath = locked.Source.Git.Repository, locked.Source.Git.Commit, locked.Source.Git.Path
	} else if locked.Source.Registry != nil {
		repository, commit, packagePath = locked.Source.Registry.Repository, locked.Source.Registry.Commit, locked.Source.Registry.Path
	} else {
		return loadedPackage{}, errors.New("locked package has no source")
	}
	snapshot, err := app.SourceAcquirer.AcquireGitHubVerified(ctx, repository, commit, packagePath, locked.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	identity := githubIdentity(repository, commit, packagePath)
	snapshot.Source = domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: repository,
		PackageSubpath: packagePath, ResolvedRevision: commit, SourceBindingHint: dependencySourceHint(locked)}
	origin, directory := domain.OriginModeDirect, (*domain.DirectoryOrigin)(nil)
	if locked.Source.Registry != nil {
		origin = domain.OriginModeDirectory
		source := locked.Source.Registry
		directory = &domain.DirectoryOrigin{ProductID: source.ProductID, DistributionID: source.DistributionID,
			DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: source.ReleaseSequence}
	}
	loaded, err := app.loadSnapshot(ctx, snapshot, origin, directory, nil)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.TreeDigest != locked.TreeDigest || loaded.envelope.ManifestDigest != locked.ManifestDigest {
		_ = loaded.cleanup()
		return loadedPackage{}, errors.New("immutable fetch does not match uap.lock")
	}
	if publish {
		sealed := domain.PackageSnapshot{Root: loaded.envelope.SnapshotRoot, TreeDigest: loaded.envelope.TreeDigest,
			DigestAlgorithm: locked.DigestAlgorithm, ExecutableFiles: append([]string{}, locked.ExecutableFiles...), Source: loaded.envelope.Source}
		_, err = store.Put(ctx, sealed)
	} else {
		checked, checkErr := verifyLockedDirectory(ctx, app, loaded, locked, recorded)
		if checkErr != nil && loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
		return checked, checkErr
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
	return loadStoredDependency(ctx, app, pkg, locked, recorded)
}

func dependencySourceHint(locked environment.LockedPackage) string {
	if locked.Source.Registry != nil {
		return "uap-dependency-registry"
	}
	return "uap-dependency-git"
}

func loadStoredDependency(ctx context.Context, app App, pkg environmentstore.Package, locked environment.LockedPackage, recorded *domain.RecordedDirectoryRelease) (loadedPackage, error) {
	if app.PackageLoader == nil {
		return loadedPackage{}, errors.New("standard package loader is unavailable")
	}
	if info, err := os.Lstat(filepath.Join(pkg.Root, "plugin.json")); err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("plugin.json is not a regular file")
		}
		return loadedPackage{}, fmt.Errorf("dependency %s is not a standard plugin.json package: %w", locked.Alias, err)
	}
	repository, commit, subpath := "", "", ""
	if locked.Source.Git != nil {
		repository, commit, subpath = locked.Source.Git.Repository, locked.Source.Git.Commit, locked.Source.Git.Path
	} else {
		repository, commit, subpath = locked.Source.Registry.Repository, locked.Source.Registry.Commit, locked.Source.Registry.Path
	}
	identity := githubIdentity(repository, commit, subpath)
	pkg.Metadata.Source = domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: repository, PackageSubpath: subpath, ResolvedRevision: commit, SourceBindingHint: dependencySourceHint(locked)}
	input := domain.LoadInput{SnapshotRoot: pkg.Root, TreeDigest: pkg.Metadata.TreeDigest,
		ExecutableFiles: append([]string{}, pkg.Metadata.ExecutableFiles...), Source: pkg.Metadata.Source}
	envelope, err := app.PackageLoader.Load(ctx, input)
	if err != nil {
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
	return verifyLockedDirectory(ctx, app, loadedPackage{envelope: envelope, security: assessment, origin: origin, directory: directory}, locked, recorded)
}

func preflightDependencyActions(cmd *cobra.Command, app App, opts *options, actions []dependencyAction, detected map[domain.ClientID]domain.DetectedClient) ([]dependencyAction, error) {
	adapter := &dependencyInstaller{app: app, opts: opts, detected: detected, cmd: cmd}
	claims := map[string]string{}
	for _, action := range actions {
		if action.Action == environment.ActionUnchanged {
			continue
		}
		prepared, err := adapter.Prepare(cmd.Context(), action)
		if err != nil {
			return nil, fmt.Errorf("preflight %s/%s: %w", action.Alias, action.ClientID, err)
		}
		plan := prepared.Plan()
		_ = prepared.Close()
		key := action.ClientID + "\x00" + plan.TargetPath
		if previous := claims[key]; previous != "" && previous != action.Alias {
			return nil, errors.New("conflicting dependency namespace claims")
		}
		claims[key] = action.Alias
	}
	return actions, nil
}

type dependencyInstaller struct {
	app         App
	opts        *options
	detected    map[domain.ClientID]domain.DetectedClient
	cmd         *cobra.Command
	beforeApply func() error
}

func (adapter *dependencyInstaller) Prepare(ctx context.Context, action environment.PreparedBinding) (*installer.PreparedOperation, error) {
	if adapter.beforeApply != nil {
		if err := adapter.beforeApply(); err != nil {
			return nil, err
		}
	}
	if action.Authority != nil {
		copy := *action.Authority
		action.Authority = &copy
	}
	client, ok := adapter.detected[domain.ClientID(action.ClientID)]
	if !ok {
		return nil, fmt.Errorf("target %s is not detected", action.ClientID)
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
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault && receipt.Alias == action.Alias {
			if action.InstallationID == "" {
				action.InstallationID = receipt.InstallationID
				action.Authority.InstallationID = receipt.InstallationID
			}
			if receipt.ClientID == action.ClientID && action.Action == environment.ActionInstall {
				return nil, installer.ErrPlanChanged
			}
		}
	}
	operationID, err := domain.NewInstallationID()
	if err != nil {
		return nil, err
	}
	request := installer.Request{OperationID: operationID, Operation: installer.Operation(action.Action), ClientID: action.ClientID, ClientConfigRoot: client.ConfigRoot,
		ClientExecutable: backendExecutable(client, adapter.detected), InstallationID: action.InstallationID,
		Selector: action.InstallationID, EnvironmentAuthority: action.Authority, ExternalUninstalled: adapter.opts.externalUninstalled}
	for _, installation := range view.Installations {
		if installation.InstallationID != action.InstallationID {
			continue
		}
		for _, binding := range installation.Bindings {
			sibling, ok := adapter.detected[domain.ClientID(binding.ClientID)]
			if !ok {
				return nil, fmt.Errorf("installed sibling %s is not detected", binding.ClientID)
			}
			request.KnownTargets = append(request.KnownTargets, installer.TargetFacts{ClientID: binding.ClientID, BindingID: binding.BindingID, ConfigRoot: sibling.ConfigRoot, Executable: backendExecutable(sibling, adapter.detected)})
		}
	}
	if action.Action == environment.ActionRemove {
		return adapter.app.Installer.Prepare(ctx, request)
	}
	var recorded *domain.RecordedDirectoryRelease
	if action.Action == "repair" && action.Package.Source.Registry != nil {
		state, err := adapter.app.StateStore.Load()
		if err != nil {
			return nil, err
		}
		installation, found := locallyMatchedInstallation(state, action.InstallationID)
		if !found || installation.Directory == nil {
			return nil, installer.ErrPlanChanged
		}
		binding, found := installation.Clients[action.BindingID]
		if !found || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != action.Package.TreeDigest {
			return nil, installer.ErrPlanChanged
		}
		revision := binding.PackageRevision
		recorded = recordedDirectoryRelease(installation, revision.ReleaseSequence, revision.ResolvedRevision)
	}
	loaded, err := materializeLockedPackage(ctx, adapter.app, *action.Package, false, recorded)
	if err != nil {
		return nil, err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	if loaded.security == nil {
		return nil, errors.New("scanner unavailable: dependency sync requires a digest-bound assessment")
	}
	if err := authorizeSecurityAssessment(adapter.cmd, adapter.app, adapter.opts, &loaded); err != nil {
		return nil, err
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
	return adapter.app.Installer.Prepare(ctx, request)
}

func (adapter *dependencyInstaller) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	if adapter.beforeApply != nil {
		if err := adapter.beforeApply(); err != nil {
			return installer.Result{}, err
		}
	}
	return adapter.app.Installer.Apply(ctx, prepared, installer.Decision{Confirmed: true})
}

// The lock records coordinates, never signed publication eligibility. Recheck
// the exact requested release without granting recorded-install exceptions.
func verifyLockedDirectory(ctx context.Context, app App, loaded loadedPackage, locked environment.LockedPackage, recorded *domain.RecordedDirectoryRelease) (loadedPackage, error) {
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
	if recorded != nil {
		request.DesiredReleaseSequence = 0
		request.Recorded = recorded
		request.Operation = domain.DirectoryRepair
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
		return loadedPackage{}, errors.New("Directory policy missing")
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
	preview, cleanup, err := dependencyPreviewApp(cmd.Context(), app, opts)
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
	var actions []dependencyAction
	for _, target := range targets {
		authority, err := environmentAuthorityForBinding(state, installation.InstallationID, target, "repair")
		if err != nil {
			return true, err
		}
		if authority == nil {
			return true, errors.New("mixed owned and unmanaged repair requires separate selections")
		}
		binding, err := selectedRepairBinding(installation, target, domain.ScopeUser)
		if err != nil {
			return true, err
		}
		rev := binding.PackageRevision
		if rev == nil {
			return true, errors.New("recorded package revision is missing")
		}
		pkg := environment.LockedPackage{Alias: authority.Alias, SourceKey: authority.SourceKey, DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: rev.TreeDigest, ManifestDigest: rev.ManifestDigest, Targets: []string{string(target)}}
		pkg.Source.Git = &environment.LockedGitSource{Repository: installation.Source.Repository, Path: installation.Source.PackageSubpath, Commit: rev.ResolvedRevision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: rev.ResolvedRevision}}
		if installation.Directory != nil {
			pkg.Source.Git = nil
			pkg.Source.Registry = &environment.LockedRegistrySource{Distribution: installation.Directory.DistributionID, DistributionID: installation.Directory.DistributionID, ProductID: installation.Directory.ProductID, ReleaseSequence: rev.ReleaseSequence, Repository: installation.Source.Repository, Path: installation.Source.PackageSubpath, Commit: rev.ResolvedRevision}
		}
		if stored, e := app.dependencyStore().Get(cmd.Context(), pkg.DigestAlgorithm, pkg.TreeDigest); e == nil {
			pkg.ExecutableFiles = append([]string{}, stored.Metadata.ExecutableFiles...)
		}
		actions = append(actions, dependencyAction{Alias: authority.Alias, ClientID: string(target), InstallationID: installation.InstallationID, BindingID: binding.ClientBindingID, Action: "repair", Package: &pkg, Authority: authority})
	}
	if _, err := preflightDependencyActions(cmd, app, opts, actions, detected); err != nil {
		return true, err
	}
	if opts.dryRun {
		return true, renderDependencySync(cmd, opts, actions, true)
	}
	if err := requireNonInteractiveMutation(app, opts, "repair"); err != nil {
		return true, err
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		var err error
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Repair the managed package and its recorded verification state? [y/N]")
		if err != nil {
			return true, err
		}
	}
	if !confirmed {
		return true, renderDependencySync(cmd, opts, actions, true)
	}
	adapter := &dependencyInstaller{app: app, opts: opts, cmd: cmd, detected: detected}
	results, err := (environment.Coordinator{Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, Bindings: actions}, true)
	if opts.format == "json" {
		if renderErr := renderJSON(cmd.OutOrStdout(), results); err == nil {
			err = renderErr
		}
	}
	return true, err
}
