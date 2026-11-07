package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func acquireSavedAdd(ctx context.Context, app App, opts *options, source string, targets []domain.ClientID, matches []string, requirement environment.Requirement, requirements environment.Requirements) (environment.Requirement, environment.LockedPackage, error) {
	var pin environment.LockedPackage
	loaded, err := loadSavedAddPackage(ctx, app, source, targets, matches, requirement)
	if err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	defer func() { _ = loaded.cleanup() }()
	alias := opts.alias
	if alias == "" {
		alias = strings.ReplaceAll(strings.ToLower(loaded.envelope.Manifest.Name), ".", "-")
		if alias == "" || alias[0] < 'a' || alias[0] > 'z' {
			alias = "plugin-" + alias
		}
		if len(alias) > 64 {
			alias = alias[:64]
		}
	}
	if loaded.directorySelection != nil {
		selection := loaded.directorySelection
		requirement = environment.Requirement{Registry: &environment.RegistrySource{Distribution: selection.DistributionID, Selector: environment.Selector{Kind: environment.SelectorRelease, Value: fmt.Sprint(selection.ReleaseSequence)}}}
		pin.Source.Registry = &environment.LockedRegistrySource{Distribution: selection.DistributionID, DistributionID: selection.DistributionID, ProductID: selection.ProductID, ReleaseSequence: selection.ReleaseSequence, Requested: requirement.Registry.Selector, Repository: selection.Source.Repository, Path: selection.Source.Path, Commit: selection.Source.Revision}
	} else {
		pin.Source.Git = &environment.LockedGitSource{Repository: requirement.Git.Repository, Path: requirement.Git.Path, Commit: requirement.Git.Selector.Value, Requested: requirement.Git.Selector}
	}
	requirement.Alias = alias
	if err := requirement.Validate(); err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	if old, ok := requirements.Dependencies[alias]; ok && mustSourceKey(old) != mustSourceKey(requirement) {
		return environment.Requirement{}, environment.LockedPackage{}, errors.New("alias already belongs to another source; use --alias")
	}
	pin.Alias = alias
	pin.SourceKey = mustSourceKey(requirement)
	pin.DigestAlgorithm = domain.TreeDigestAlgorithm
	pin.TreeDigest = loaded.envelope.TreeDigest
	pin.ManifestDigest = loaded.envelope.ManifestDigest
	pin.Version = loaded.envelope.Manifest.Version
	pin.ExecutableFiles = append([]string{}, loaded.envelope.ExecutableFiles...)
	if err := (dependencyResolver{app: app, dryRun: opts.dryRun}).finishLoaded(ctx, loaded, pin); err != nil {
		return environment.Requirement{}, environment.LockedPackage{}, err
	}
	return requirement, pin, nil

}

func rejectKnownOwnedAdd(app App, source string) error {
	if app.StateStore == nil {
		return nil
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	key := savedDirectSourceKey(source)
	if matches := exactGitPattern.FindStringSubmatch(source); matches != nil {
		key = "github:" + strings.ToLower(matches[1]) + ":" + matches[3]
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID != app.environmentID() {
			continue
		}
		if key != "" && receipt.SourceKey == key || isDirectorySelector(source) && receipt.SourceKey == "directory:"+strings.ToLower(source) {
			return fmt.Errorf("source belongs to saved dependency %s; use agentplugins add without --no-save or agentplugins update %s", receipt.Alias, receipt.Alias)
		}
	}
	return nil
}

func savedAddTargets(cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID) ([]domain.ClientID, error) {
	var err error
	if len(targets) == 0 && app.Terminal && opts.format == "human" {
		clients, e := app.Detector.Detect(cmd.Context())
		if e != nil {
			return nil, e
		}
		var eligible []domain.DetectedClient
		var skipped []targetSkip
		for _, client := range clients {
			switch {
			case client.DetectionError != nil:
				skipped = append(skipped, targetSkip{Client: client.ClientID, Reason: "client configuration could not be inspected safely; correct it and retry"})
			case client.Status != domain.DetectionDetected:
				skipped = append(skipped, targetSkip{Client: client.ClientID, Reason: "client is not detected; install or configure it before selecting it"})
			case !supportsDependencyTarget(app, string(client.ClientID)):
				skipped = append(skipped, targetSkip{Client: client.ClientID, Reason: "use --no-save"})
			default:
				eligible = append(eligible, client)
			}
		}
		if len(eligible) > 0 {
			var incompatible []targetSkip
			eligible, incompatible, err = compatibleSavedAddTargets(cmd.Context(), app, source, eligible)
			if err != nil {
				return nil, err
			}
			skipped = append(skipped, incompatible...)
		}
		if len(eligible) > 1 {
			if app, err = withPrompter(cmd, app, opts); err != nil {
				return nil, err
			}
		}
		targets, _, err = promptTargetChoices(cmd, app, eligible, skipped, clients)
		if err != nil {
			return nil, err
		}
	}
	return validateSavedAddTargets(app, opts, source, targets)
}

func validateSavedAddTargets(app App, opts *options, source string, targets []domain.ClientID) ([]domain.ClientID, error) {
	if len(targets) == 0 {
		return nil, errors.New("automated installation requires --target")
	}
	for _, target := range targets {
		if domain.ClientTraitsFor(target).RequiresPersonalMappingForPrepare && len(targets) > 1 {
			return nil, errors.New("ChatGPT Web is prepared manually; run agentplugins add <source> --target chatgptweb separately, or use --no-save for the mixed imperative workflow")
		}
		if !supportsDependencyTarget(app, string(target)) {
			return nil, fmt.Errorf("target %s cannot be saved; retry agentplugins add %s --target %s --no-save", publicCLITargetName(target), source, strings.Join(clientIDStrings(targets), ","))
		}
	}
	if opts.scope != "user" && opts.scope != "project" {
		return nil, errors.New("saved dependencies support user scope only")
	}

	return targets, nil
}
