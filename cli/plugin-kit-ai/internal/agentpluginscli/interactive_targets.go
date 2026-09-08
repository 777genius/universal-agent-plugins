package agentpluginscli

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/spf13/cobra"
)

// promptCompatibleDetectedTargets keeps the no-flag path convenient without
// turning ambient client detection into an unsafe all-or-nothing guess. It
// first narrows the installed clients to one complete target set that the
// selected package can actually serve, then asks the user to confirm it.
func promptCompatibleDetectedTargets(ctx context.Context, cmd *cobra.Command, app App, source string) (string, []domain.DetectedClient, *loadedPackage, error) {
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return "", nil, nil, fmt.Errorf("detect AI clients: %w", err)
	}
	detected := detectedSupportedClients(clients)
	if len(detected) == 0 {
		selection, all, err := promptTargetChoices(cmd, detected, nil, clients)
		return selection, all, nil, err
	}

	compatible, preloaded, err := app.compatibleDetectedTargets(ctx, source, detected)
	if err != nil {
		return "", nil, nil, err
	}
	skipped := subtractDetectedClients(detected, compatible)
	selection, all, err := promptTargetChoices(cmd, compatible, skipped, clients)
	if err != nil {
		if preloaded != nil && preloaded.cleanup != nil {
			_ = preloaded.cleanup()
		}
		return "", nil, nil, err
	}
	return selection, all, preloaded, nil
}

func detectedSupportedClients(clients []domain.DetectedClient) []domain.DetectedClient {
	result := make([]domain.DetectedClient, 0, len(clients))
	for _, client := range clients {
		if client.Status == domain.DetectionDetected && supportedTarget(client.ClientID) {
			result = append(result, client)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return targetOrder(result[i].ClientID) < targetOrder(result[j].ClientID) })
	return result
}

func subtractDetectedClients(all, selected []domain.DetectedClient) []domain.DetectedClient {
	kept := make(map[domain.ClientID]struct{}, len(selected))
	for _, client := range selected {
		kept[client.ClientID] = struct{}{}
	}
	result := make([]domain.DetectedClient, 0, len(all)-len(selected))
	for _, client := range all {
		if _, ok := kept[client.ClientID]; !ok {
			result = append(result, client)
		}
	}
	return result
}

func (app App) compatibleDetectedTargets(ctx context.Context, source string, detected []domain.DetectedClient) ([]domain.DetectedClient, *loadedPackage, error) {
	switch {
	case strings.HasPrefix(source, "discovery:"):
		compatible, err := app.compatibleDiscoveryTargets(ctx, source, detected)
		return compatible, nil, err
	case isDirectorySelector(source):
		compatible, err := app.compatibleDirectoryTargets(ctx, source, detected)
		if err != nil {
			return nil, nil, err
		}
		loaded, err := app.loadPackageFor(
			ctx,
			source,
			withDetectedClients(app.addResolutionRequest(source, detectedClientIDs(compatible)), detectedClientMap(detected)),
		)
		if err != nil {
			return nil, nil, err
		}
		compatible = app.compatibleLoadedTargets(ctx, loaded, compatible)
		if len(compatible) == 0 {
			if loaded.cleanup != nil {
				_ = loaded.cleanup()
			}
			return nil, nil, fmt.Errorf("the package cannot be installed automatically in any detected supported client; use --target to inspect a specific client")
		}
		return compatible, &loaded, nil
	default:
		loaded, err := app.loadPackageFor(ctx, source, app.addResolutionRequest(source, nil))
		if err != nil {
			return nil, nil, err
		}
		compatible := app.compatibleLoadedTargets(ctx, loaded, detected)
		if len(compatible) == 0 {
			if loaded.cleanup != nil {
				_ = loaded.cleanup()
			}
			return nil, nil, fmt.Errorf("the package cannot be installed in any detected supported client; use --target to inspect a specific client")
		}
		return compatible, &loaded, nil
	}
}

func (app App) compatibleDiscoveryTargets(ctx context.Context, selector string, detected []domain.DetectedClient) ([]domain.DetectedClient, error) {
	if app.DiscoveryClient == nil {
		return nil, fmt.Errorf("signed Discovery Index dependencies are unavailable; use owner/repository@FULL_SHA[//path]")
	}
	bundle, err := app.DiscoveryClient.Load(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("load signed Discovery Index: %w", err)
	}
	record, err := resolveDiscoveryRecord(bundle, selector)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(record.CompatibleClients))
	for _, client := range record.CompatibleClients {
		allowed[client] = struct{}{}
	}
	compatible := make([]domain.DetectedClient, 0, len(detected))
	for _, client := range detected {
		if _, ok := allowed[string(client.ClientID)]; ok {
			compatible = append(compatible, client)
		}
	}
	if len(compatible) == 0 {
		return nil, fmt.Errorf("discovery package %q cannot be installed in any detected supported client; use --target to inspect a specific client", selector)
	}
	return compatible, nil
}

func (app App) compatibleDirectoryTargets(ctx context.Context, selector string, detected []domain.DetectedClient) ([]domain.DetectedClient, error) {
	if app.DirectoryClient == nil || app.StateStore == nil {
		return nil, fmt.Errorf("signed Directory dependencies are unavailable; use a direct local or exact full-SHA source")
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	if err != nil {
		return nil, fmt.Errorf("load signed Directory: %w", err)
	}
	request, err := retainDirectoryRelease(bundle.Snapshot, state, selector, app.addResolutionRequest(selector, nil))
	if err != nil {
		return nil, err
	}
	resolveSelector := selector
	if request.Selector != "" {
		resolveSelector = request.Selector
	}
	clients := detectedClientMap(detected)
	environment := directoryEnvironment(clients)
	environment.InstallerVersion = app.Version
	operation := request.Operation
	if operation == "" {
		operation = domain.DirectoryInstall
	}

	// At most ten clients are supported, so enumerating subsets is bounded to
	// 1,023 pure resolver calls. This preserves one signed distribution/release
	// for the complete set instead of combining incompatible per-client picks.
	var singleTargetErr error
	for size := len(detected); size >= 1; size-- {
		var match []domain.DetectedClient
		forEachDetectedSubset(detected, size, func(candidate []domain.DetectedClient) bool {
			targets := make([]domain.ClientID, len(candidate))
			for index, client := range candidate {
				targets[index] = client.ClientID
			}
			_, resolveErr := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{
				Selector: resolveSelector, Targets: targets, Scope: domain.ScopeUser,
				InstallerVersion: app.Version, ClientVersions: environment.ClientVersions,
				OS: runtime.GOOS, Architecture: runtime.GOARCH, DependencyIdentity: environment.DependencyIdentity,
				SchemaVersion: "1.0.0", Operation: operation, Recorded: request.Recorded,
			})
			if size == 1 && singleTargetErr == nil && resolveErr != nil {
				singleTargetErr = resolveErr
			}
			if resolveErr == nil {
				match = append([]domain.DetectedClient(nil), candidate...)
				return false
			}
			return true
		})
		if len(match) > 0 {
			return match, nil
		}
	}
	if singleTargetErr != nil {
		return nil, singleTargetErr
	}
	return nil, fmt.Errorf("no signed Directory release for %q supports any detected client", selector)
}

func forEachDetectedSubset(values []domain.DetectedClient, size int, visit func([]domain.DetectedClient) bool) {
	if size < 1 || size > len(values) {
		return
	}
	indices := make([]int, size)
	var walk func(int, int) bool
	walk = func(depth, start int) bool {
		if depth == size {
			candidate := make([]domain.DetectedClient, size)
			for index, valueIndex := range indices {
				candidate[index] = values[valueIndex]
			}
			return visit(candidate)
		}
		for index := start; index <= len(values)-(size-depth); index++ {
			indices[depth] = index
			if !walk(depth+1, index+1) {
				return false
			}
		}
		return true
	}
	walk(0, 0)
}

func (app App) compatibleLoadedTargets(ctx context.Context, loaded loadedPackage, detected []domain.DetectedClient) []domain.DetectedClient {
	clientMap := detectedClientMap(detected)
	planner := clientplanner.Planner{ManagedRoot: app.ManagedRoot, Detected: clientMap}
	physicalID := domain.ComputePhysicalArtifactID(loaded.envelope.Manifest.Name, "00000000-0000-4000-8000-000000000000")
	compatible := make([]domain.DetectedClient, 0, len(detected))
	for _, client := range detected {
		candidate := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&candidate, client.ClientID); err != nil {
			continue
		}
		plan, err := planner.Plan(ctx, candidate.envelope, client, domain.ScopeUser, physicalID)
		if err != nil || plan.Status == domain.PlanUnsupported {
			continue
		}
		if preflighter, ok := app.Lifecycle.Activator.(interface {
			PreflightActivation(domain.ActivationRequest) error
		}); ok {
			err = preflighter.PreflightActivation(domain.ActivationRequest{
				Client: client, Plan: plan, BackendExecutable: backendExecutable(client, clientMap), VerifyOnly: true,
			})
			if err != nil {
				continue
			}
		}
		compatible = append(compatible, client)
	}
	return compatible
}

func detectedClientIDs(clients []domain.DetectedClient) []domain.ClientID {
	result := make([]domain.ClientID, len(clients))
	for index, client := range clients {
		result[index] = client.ClientID
	}
	return result
}

func detectedClientMap(clients []domain.DetectedClient) map[domain.ClientID]domain.DetectedClient {
	result := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		result[client.ClientID] = client
	}
	return result
}

func resolveDiscoveryRecord(bundle discoveryv1.VerifiedBundle, selector string) (discoveryv1.Record, error) {
	var matches []discoveryv1.Record
	for _, record := range bundle.Search.Records {
		if record.Slug == selector {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q was not found", selector)
	}
	if len(matches) != 1 {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q is ambiguous", selector)
	}
	if matches[0].Availability != "available" {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q is unavailable and cannot be newly acquired", selector)
	}
	return matches[0], nil
}
