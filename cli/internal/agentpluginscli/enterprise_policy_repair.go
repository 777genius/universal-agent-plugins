package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

// Recorded revisions remain repair authority even when desired intent advanced
// or disappeared. Only a validated agreement with the same immutable source can
// prove its original selector; missing evidence never becomes an exact request.
func repairPolicyLock(app App) environment.Lock {
	manifest, path := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if err != nil {
		return environment.Lock{}
	}
	locked, err := readDependencyLock(path, requirements)
	if err != nil {
		return environment.Lock{}
	}
	return locked
}

func repairRequestedIntent(pin environment.LockedPackage, locked environment.Lock) environment.LockedPackage {
	if pin.Source.Git != nil {
		source := *pin.Source.Git
		source.Requested = environment.Selector{Kind: environment.SelectorDefault}
		pin.Source.Git = &source
	}
	if pin.Source.Archive != nil {
		source := *pin.Source.Archive
		source.RequestedSHA256 = ""
		pin.Source.Archive = &source
	}
	for _, original := range locked.Packages {
		if original.Alias != pin.Alias || original.SourceKey != pin.SourceKey || original.TreeDigest != pin.TreeDigest || original.ManifestDigest != pin.ManifestDigest || original.Revision() != pin.Revision() {
			continue
		}
		if pin.Source.Git != nil && original.Source.Git != nil {
			pin.Source.Git.Requested = original.Source.Git.Requested
		}
		if pin.Source.Archive != nil && original.Source.Archive != nil {
			pin.Source.Archive.RequestedSHA256 = original.Source.Archive.RequestedSHA256
		}
		pin.ExecutableFiles = canonicalDependencyExecutables(original.ExecutableFiles)
		break
	}
	return pin
}

func checkEnterpriseRepairPolicies(ctx context.Context, app App, actions []dependencyAction) error {
	if app.InstallationPolicy == nil {
		return nil
	}
	var repairs []dependencyAction
	for _, action := range actions {
		if dependencyRepairOperation(action.Action) && action.Package != nil {
			repairs = append(repairs, action)
		}
	}
	if len(repairs) == 0 {
		return nil
	}
	unrestricted, err := enterprisepolicy.IsUnrestricted(ctx, app.InstallationPolicy)
	if err != nil {
		return err
	}
	if unrestricted {
		_, err = app.checkEnterprisePolicy(ctx, domain.InstallPolicyRequest{})
		return err
	}
	locked := repairPolicyLock(app)
	for _, action := range repairs {
		if err := checkEnterpriseRepairClosure(ctx, app, locked, action); err != nil {
			return fmt.Errorf("installation policy for repair %s: %w", domain.PolicySafeText(action.Alias), err)
		}
	}
	return nil
}

func checkEnterpriseRepairClosure(ctx context.Context, app App, locked environment.Lock, action dependencyAction) error {
	pin := repairRequestedIntent(*action.Package, locked)
	closure := recordedRepairClosure(locked, pin, action.ClientID)
	// Check the complete known source set before any missing snapshot is read.
	for _, pkg := range closure.Packages {
		if _, err := app.checkEnterprisePolicy(ctx, lockedPolicyRequest(pkg, app.scope())); err != nil {
			return err
		}
	}
	if pin.ExecutableFiles == nil {
		var err error
		pin, err = hydrateRecordedDependencyInventory(ctx, app, pin)
		if err != nil {
			return err
		}
		closure = recordedRepairClosure(locked, pin, action.ClientID)
	}
	*action.Package = pin
	index, err := indexDependencyDeclarations(ctx, closure)
	if err != nil {
		return err
	}
	for _, pkg := range closure.Packages {
		resolved, err := inspectRepairPolicyPackage(ctx, app, pkg)
		if err != nil {
			return err
		}
		if err := index.verifyDeclaredEdges(ctx, pkg.Alias, resolved.Dependencies); err != nil {
			return err
		}
	}
	return nil
}

func recordedRepairClosure(locked environment.Lock, pin environment.LockedPackage, target string) environment.Lock {
	closure := environmentapp.SelectedLock(locked, locked.ClosureAliases([]string{pin.Alias}))
	replaced := false
	for i, pkg := range closure.Packages {
		if pkg.Alias == pin.Alias {
			pin.RequiredBy = append([]environment.RequiredBy(nil), pkg.RequiredBy...)
			closure.Packages[i] = pin
			replaced = true
		}
		closure.Packages[i].Targets = []string{target}
	}
	if !replaced {
		pin.Targets = []string{target}
		closure.Packages = append(closure.Packages, pin)
	}
	return closure
}

func inspectRepairPolicyPackage(ctx context.Context, app App, pin environment.LockedPackage) (environment.ResolvedPackage, error) {
	visit := func(envelope domain.PackageEnvelope) error {
		request := lockedPolicyRequest(pin, app.scope())
		request.Envelope = &envelope
		_, err := app.checkEnterprisePolicy(ctx, request)
		return err
	}
	resolved, err := inspectEnterprisePolicyPin(ctx, app, pin, visit)
	if !errors.Is(err, environmentstore.ErrMissing) {
		return resolved, err
	}
	if _, err := readReplayDependency(ctx, app, pin); err != nil {
		return environment.ResolvedPackage{}, err
	}
	return inspectEnterprisePolicyPin(ctx, app, pin, visit)
}
