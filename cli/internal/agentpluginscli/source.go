package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/catalog"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

var (
	shortNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)
	exactGitPattern  = regexp.MustCompile(`^(?:github:)?([A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*)@([0-9a-f]{40})(?://(.+))?$`)
)

const maxAutodiscoveryPackageCandidates = 16

type loadedPackage struct {
	envelope              domain.PackageEnvelope
	security              *domain.SecurityAssessment
	securityAuthorized    bool
	hints                 domain.CompatibilityHints
	origin                domain.OriginMode
	directory             *domain.DirectoryOrigin
	distributionSuspended bool
	releaseRevoked        bool
	localChatGPTMapping   *domain.ChatGPTLocalMapping
	chatGPTPreparation    bool
	directorySelection    *domain.DirectorySelection
	cleanup               func() error
}

type packageResolutionRequest struct {
	StandardOnly           bool
	Targets                []domain.ClientID
	Operation              domain.DirectoryOperation
	Recorded               *domain.RecordedDirectoryRelease
	Clients                map[domain.ClientID]domain.DetectedClient
	RetainRecorded         bool
	Selector               string
	RequestedSelector      string
	DirectBinding          *domain.SourceBinding
	DesiredReleaseSequence uint64
}

func (app App) loadPackage(ctx context.Context, raw string) (loadedPackage, error) {
	return app.loadPackageFor(ctx, raw, packageResolutionRequest{Operation: domain.DirectoryInstall})
}

func (app App) loadPackageFor(ctx context.Context, raw string, request packageResolutionRequest) (loadedPackage, error) {
	requested := strings.TrimSpace(raw)
	if requested == "" {
		return loadedPackage{}, fmt.Errorf("plugin name or source is required")
	}
	if localArchiveInput(requested) {
		spec, err := sourcespec.Parse(requested)
		if err != nil {
			return loadedPackage{}, err
		}
		return app.acquireDirectInput(ctx, spec)
	}
	if explicitLocalPath(requested) {
		return app.acquireLocal(ctx, requested)
	}
	if match := exactGitPattern.FindStringSubmatch(requested); match != nil {
		return app.acquireGitHub(ctx, requested, match[1], match[2], match[3], "")
	}
	if strings.HasPrefix(requested, "discovery:") {
		return app.acquireDiscovery(ctx, requested, request)
	}
	if !isDirectorySelector(requested) {
		spec, err := sourcespec.Parse(requested)
		if err != nil {
			return loadedPackage{}, err
		}
		return app.acquireDirectInput(ctx, spec)
	}
	return app.acquireDirectory(ctx, requested, request)
}

func (app App) acquireAutodiscoveredGitHub(ctx context.Context, requested, repository, revision string) (loadedPackage, error) {
	paths, err := app.SourceAcquirer.DiscoverGitHubPackages(ctx, repository, revision)
	if err != nil {
		return loadedPackage{}, err
	}
	// Each candidate requires an independently sealed package snapshot before
	// the loader can decide whether it is valid. Bound that work so a pathless
	// source cannot turn one user action into an unbounded number of fetches.
	if len(paths) > maxAutodiscoveryPackageCandidates {
		return loadedPackage{}, fmt.Errorf("Found %d package candidates. Choose one explicitly with //path", len(paths))
	}
	// Root plugin.json has precedence. An empty candidate set preserves support
	// for repository-root native packages such as .codex-plugin/plugin.json.
	if len(paths) == 0 || (len(paths) == 1 && paths[0] == "") {
		return app.acquireGitHubPath(ctx, requested, repository, revision, "", "")
	}

	type candidate struct {
		path   string
		loaded loadedPackage
	}
	valid := make([]candidate, 0, len(paths))
	var firstFailure error
	for _, packagePath := range paths {
		explicitSource := repository + "@" + revision + "//" + packagePath
		loaded, loadErr := app.acquireGitHubPath(ctx, explicitSource, repository, revision, packagePath, "")
		if loadErr != nil {
			var invalidPackage *domain.LoadError
			if !errors.As(loadErr, &invalidPackage) {
				for _, item := range valid {
					_ = item.loaded.cleanup()
				}
				return loadedPackage{}, loadErr
			}
			if firstFailure == nil {
				firstFailure = loadErr
			}
			continue
		}
		// Preserve what the user typed for update/replay while canonical identity,
		// state, and digests remain bound to the selected package root.
		loaded.envelope.Source.RequestedSource = requested
		valid = append(valid, candidate{path: packagePath, loaded: loaded})
	}
	if len(valid) == 1 {
		return valid[0].loaded, nil
	}
	for _, item := range valid {
		_ = item.loaded.cleanup()
	}
	if len(valid) == 0 {
		listed := make([]string, 0, len(paths))
		for _, packagePath := range paths {
			listed = append(listed, "//"+packagePath)
		}
		if firstFailure != nil {
			return loadedPackage{}, fmt.Errorf("no valid Agent Plugins package found among %s: %w", strings.Join(listed, ", "), firstFailure)
		}
		return loadedPackage{}, fmt.Errorf("no valid Agent Plugins package found among %s", strings.Join(listed, ", "))
	}
	listed := make([]string, 0, len(valid))
	for _, item := range valid {
		listed = append(listed, "//"+item.path)
	}
	return loadedPackage{}, fmt.Errorf("Found packages: %s. Choose one explicitly", strings.Join(listed, ", "))
}

func (app App) acquireDirectory(ctx context.Context, selector string, request packageResolutionRequest) (loadedPackage, error) {
	if app.DirectoryClient == nil || app.SourceAcquirer == nil {
		return loadedPackage{}, fmt.Errorf("signed Directory dependencies are unavailable; use a direct local or exact full-SHA source")
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return loadedPackage{}, err
	}
	bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	if err != nil {
		return loadedPackage{}, fmt.Errorf("load signed Directory: %w", err)
	}
	request, err = retainDirectoryRelease(bundle.Snapshot, state, selector, request)
	if err != nil {
		return loadedPackage{}, err
	}
	operation := request.Operation
	if operation == "" {
		operation = domain.DirectoryInstall
	}
	environment := directoryEnvironment(request.Clients)
	environment.InstallerVersion = app.Version
	resolveSelector := selector
	if request.Selector != "" {
		resolveSelector = request.Selector
	}
	productID, _ := directorySelectorProductID(bundle.Snapshot, resolveSelector)
	affectedTargets := expandAffectedSurfaceTargets(request.Targets, app.scope())
	resolveRequest := domain.DirectoryResolveRequest{
		Selector: resolveSelector, Targets: affectedTargets, Scope: app.scope(),
		InstallerVersion: app.Version, ClientVersions: environment.ClientVersions, OS: environment.OS, Architecture: environment.Architecture,
		DependencyIdentity: environment.DependencyIdentity,
		SchemaVersion:      "1.0.0", Operation: operation, Recorded: request.Recorded,
		DesiredReleaseSequence: request.DesiredReleaseSequence,
	}
	preparingChatGPT := false
	var retainedMapping *domain.ChatGPTLocalMapping
	installation, _ := locallyMatchedInstallation(state, resolveSelector)
	intents := lifecycleInstallIntents(installation, string(app.scope()), nil)
	for _, target := range affectedTargets {
		if requiresPersonalMapping(target) && (productID == "context7" || app.chatGPTPreparation || intents[target] == domain.InstallIntentPrepare) {
			preparingChatGPT = true
		}
	}
	if preparingChatGPT {
		if id, purpose, ok := directoryPreparationClient(affectedTargets); ok {
			resolveRequest.Purpose = purpose
			resolveRequest.Targets = []domain.ClientID{id}
		}
		retainedMapping = installation.LocalChatGPTMapping
	}
	selection, err := domain.ResolveDirectory(bundle.Snapshot, resolveRequest)
	if err != nil {
		return loadedPackage{}, err
	}
	product, distribution, release, policy := directoryRecords(bundle.Snapshot, selection)
	if product == nil || distribution == nil || release == nil || policy == nil {
		return loadedPackage{}, fmt.Errorf("signed Directory selection cannot be reproduced from its snapshot")
	}
	if err := validateDirectoryCompatibilityPolicy(*policy); err != nil {
		return loadedPackage{}, err
	}
	snapshot, err := app.SourceAcquirer.AcquireGitHubVerified(ctx, selection.Source.Repository, selection.Source.Revision, selection.Source.Path, selection.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	snapshot.Source.RequestedSource = selector
	if request.RequestedSelector != "" {
		snapshot.Source.RequestedSource = request.RequestedSelector
	}
	snapshot.Source.SourceBindingHint = "directory-v1"
	origin := &domain.DirectoryOrigin{
		ProductID: selection.ProductID, DistributionID: selection.DistributionID, DistributionKind: selection.DistributionKind,
		DesiredReleaseSequence: selection.ReleaseSequence, SnapshotSchema: directoryv1.SnapshotSchemaVersion,
		SnapshotSequence: selection.SnapshotSequence, SnapshotDigest: bundle.Digest,
	}
	load := app.loadSnapshot
	if request.StandardOnly {
		load = app.loadStandardSnapshot
	}
	loaded, err := load(ctx, snapshot, domain.OriginModeDirectory, origin, &selection)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.ManifestDigest != selection.ManifestDigest {
		_ = loaded.cleanup()
		return loadedPackage{}, fmt.Errorf("acquired package manifest digest %s does not match signed Directory digest %s", loaded.envelope.ManifestDigest, selection.ManifestDigest)
	}
	if loaded.envelope.Manifest.Name != product.ManifestName || loaded.envelope.Manifest.Name != release.ManifestName {
		_ = loaded.cleanup()
		return loadedPackage{}, fmt.Errorf("signed Directory identity does not match acquired package manifest")
	}
	environment.DependencyIdentity = deterministicDependencyIdentity(loaded.envelope, affectedTargets)
	exactRequest := resolveRequest
	exactRequest.DesiredReleaseSequence = 0
	exactRequest.Selector = selection.DistributionID
	exactRequest.Operation = domain.DirectoryNewTarget
	exactRequest.Recorded = &domain.RecordedDirectoryRelease{
		ProductID: selection.ProductID, DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence,
		Repository: selection.Source.Repository, ResolvedRevision: selection.Source.Revision, Path: selection.Source.Path,
		TreeDigestAlgorithm: selection.TreeDigestAlgorithm, TreeDigest: selection.TreeDigest, ManifestDigest: selection.ManifestDigest,
	}
	exactRequest.DependencyIdentity = environment.DependencyIdentity
	checked, err := domain.ResolveDirectory(bundle.Snapshot, exactRequest)
	if err != nil || checked.DistributionID != selection.DistributionID || checked.ReleaseSequence != selection.ReleaseSequence {
		_ = loaded.cleanup()
		if err != nil {
			return loadedPackage{}, fmt.Errorf("acquired exact Directory release failed compatibility recheck: %w", err)
		}
		return loadedPackage{}, fmt.Errorf("acquired exact Directory release changed during compatibility recheck")
	}
	// The preparation resolver intentionally accepts exactly ChatGPT. Compose
	// mixed requests by checking peers against its exact immutable selection with
	// normal signed eligibility; never acquire another target's substitute package.
	if preparingChatGPT {
		var peers []domain.ClientID
		for _, target := range affectedTargets {
			if !requiresPersonalMapping(target) {
				peers = append(peers, target)
			}
		}
		if len(peers) > 0 {
			peerRequest := exactRequest
			peerRequest.Purpose = ""
			peerRequest.Targets = peers
			peer, peerErr := domain.ResolveDirectory(bundle.Snapshot, peerRequest)
			if peerErr != nil || peer.DistributionID != selection.DistributionID || peer.ReleaseSequence != selection.ReleaseSequence || peer.TreeDigest != selection.TreeDigest {
				_ = loaded.cleanup()
				return loadedPackage{}, fmt.Errorf("mixed preparation peers must qualify the same immutable Context7 release: %v", peerErr)
			}
		}
		if err := domain.ValidateContext7PreparationPackage(loaded.envelope); err != nil {
			_ = loaded.cleanup()
			return loadedPackage{}, err
		}
		if loaded.envelope.App.Present || loaded.envelope.App.Declared {
			_ = loaded.cleanup()
			return loadedPackage{}, fmt.Errorf("refuse personal registration for a publisher-mapped package")
		}
		loaded.chatGPTPreparation = true
		loaded.localChatGPTMapping = retainedMapping
		if retainedMapping != nil && retainedMapping.IsLegacyContext7Registration() {
			// The 0.1.56 receipt came from guidance that cannot produce a working
			// ChatGPT registration. Require a new explicit app ID to migrate it.
			loaded.localChatGPTMapping = nil
		}
		if app.chatGPTAppID != "" {
			mapping := domain.ChatGPTLocalMapping{ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7", Server: "context7", URL: domain.Context7ChatGPTURL, AppID: app.chatGPTAppID}
			if retainedMapping != nil && *retainedMapping != mapping && !retainedMapping.IsLegacyContext7Registration() {
				_ = loaded.cleanup()
				return loadedPackage{}, fmt.Errorf("explicit ChatGPT ID conflicts with retained personal registration receipt")
			}
			loaded.localChatGPTMapping = &mapping
		}
		if loaded.localChatGPTMapping != nil {
			if err := loaded.localChatGPTMapping.ValidatePackage(loaded.envelope); err != nil {
				_ = loaded.cleanup()
				return loadedPackage{}, err
			}
		}
	}
	loaded.distributionSuspended = distribution.Status == domain.DistributionSuspended
	loaded.releaseRevoked = policy.Status == domain.ReleaseRevoked || snapshotRevokes(bundle.Snapshot, selection)
	if err := applyDirectoryCompatibility(&loaded, bundle, selection, *policy, environment); err != nil {
		_ = loaded.cleanup()
		return loadedPackage{}, err
	}
	return loaded, nil
}

func prepareLoadedPackageForClient(loaded *loadedPackage, clientID domain.ClientID) error {
	if loaded == nil || !requiresPersonalMapping(clientID) {
		return nil
	}
	if loaded.chatGPTPreparation {
		if loaded.localChatGPTMapping == nil {
			return fmt.Errorf("action_required: %s", domain.ChatGPTRegistrationAction)
		}
		mapping := *loaded.localChatGPTMapping
		if err := mapping.ValidatePackage(loaded.envelope); err != nil {
			return err
		}
		if loaded.envelope.App.Present || loaded.envelope.App.Declared {
			return fmt.Errorf("refuse to replace publisher app mapping with personal registration")
		}
		raw, err := json.Marshal(map[string]any{"apps": map[string]any{mapping.Server: map[string]string{"id": mapping.AppID}}})
		if err != nil {
			return err
		}
		entryRaw, _ := json.Marshal(map[string]string{"id": mapping.AppID})
		loaded.envelope.LocalChatGPTMapping = &mapping
		loaded.envelope.App = domain.AppComponent{Present: true, Declared: true, Enabled: true, Raw: raw, Bindings: map[string]domain.AppBinding{mapping.Server: {Alias: mapping.Server, ID: mapping.AppID, Raw: entryRaw}}}
		loaded.envelope.Inventory.AppPresent = true
		loaded.envelope.Inventory.AppBindings = []string{mapping.Server}
		return nil
	}
	compatibility, ok := loaded.hints.Compatibility[string(clientID)]
	if !ok || compatibility.AppBinding == nil {
		return nil
	}
	binding := *compatibility.AppBinding
	validateBinding := catalog.ValidateAppBinding
	if loaded.origin == domain.OriginModeDirectory {
		validateBinding = catalog.ValidateAppBindingReference
	}
	if err := validateBinding(binding); err != nil {
		return fmt.Errorf("Directory ChatGPT app binding is invalid: %w", err)
	}
	server, ok := loaded.envelope.MCP.Servers[binding.MCPServer]
	if !ok || !loaded.envelope.MCP.Enabled {
		return fmt.Errorf("Directory ChatGPT app binding references missing MCP server %q", binding.MCPServer)
	}
	serverURL, ok := server.Decoded["url"].(string)
	if !ok || serverURL != binding.MCPURL {
		return fmt.Errorf("Directory ChatGPT app binding URL does not match MCP server %q", binding.MCPServer)
	}
	if loaded.envelope.App.Present {
		existing, matches := loaded.envelope.App.Bindings[binding.AppKey]
		if !loaded.envelope.App.Enabled || !matches || len(loaded.envelope.App.Bindings) != 1 || existing.ID != binding.ID {
			return fmt.Errorf("package .app.json does not exactly match the Directory ChatGPT app binding")
		}
		return nil
	}
	entry := struct {
		ID string `json:"id"`
	}{ID: binding.ID}
	document := struct {
		Apps map[string]any `json:"apps"`
	}{Apps: map[string]any{binding.AppKey: entry}}
	raw, err := json.Marshal(document)
	if err != nil {
		return err
	}
	entryRaw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	loaded.envelope.App = domain.AppComponent{Present: true, Declared: true, Enabled: true, Raw: raw, Bindings: map[string]domain.AppBinding{binding.AppKey: {Alias: binding.AppKey, ID: binding.ID, Raw: entryRaw}}}
	loaded.envelope.Inventory.AppPresent = true
	loaded.envelope.Inventory.AppBindings = []string{binding.AppKey}
	return nil
}

func restoreCatalogEvidence(loaded *loadedPackage, binding domain.ClientBinding) {
	if loaded == nil || binding.PackageRevision == nil {
		return
	}
	// Directory acquisition has already composed compatibility evidence for the
	// currently detected installer, client, platform, and dependency tuple. The
	// recorded copy identifies what was applicable when this binding was last
	// written and must not replace that freshly filtered evidence during repair.
	if loaded.origin == domain.OriginModeDirectory && loaded.envelope.CatalogEvidence != nil {
		return
	}
	if binding.PackageRevision.CatalogEvidence == nil {
		loaded.envelope.CatalogEvidence = nil
		loaded.hints.Compatibility = nil
		return
	}
	evidence := *binding.PackageRevision.CatalogEvidence
	evidence.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
	loaded.envelope.CatalogEvidence = &evidence
	loaded.hints.Compatibility = cloneCatalogCompatibility(evidence.Compatibility)
}

func isDirectorySelector(value string) bool {
	if isShortName(value) {
		return true
	}
	parts := strings.Split(value, "/")
	return len(parts) == 2 && isShortName(parts[0]) && isShortName(parts[1])
}

func isShortName(value string) bool {
	return shortNamePattern.MatchString(value) && !strings.ContainsAny(value, `/\\`)
}

func explicitLocalPath(value string) bool {
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) {
		return true
	}
	// Classify syntax independently of the host OS, including Unix roots and
	// only genuinely absolute Windows spellings, so copied sources are stable;
	// drive-relative forms such as C:plugin intentionally remain selectors.
	if len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if !strings.HasPrefix(value, `\\`) {
		return false
	}
	unc := strings.TrimPrefix(value, `\\`)
	server, share, ok := strings.Cut(unc, `\`)
	return ok && server != "" && strings.Trim(share, `\`) != ""
}
