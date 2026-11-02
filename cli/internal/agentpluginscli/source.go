package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
