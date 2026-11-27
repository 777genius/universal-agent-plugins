package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func lockedPolicyRequest(pin environment.LockedPackage, scope domain.InstallScope) domain.InstallPolicyRequest {
	origin, _ := lockedPackageOrigin(pin)
	req := domain.InstallPolicyRequest{Source: identityFromLocked(pin), OriginMode: origin, Scope: scope}
	for _, target := range pin.Targets {
		req.Targets = append(req.Targets, domain.ClientID(target))
	}
	if source := pin.Source.Git; source != nil {
		req.Source.SourceKind, req.Source.SourceURL = "git", source.URL
		pinned := source.Requested.Kind == environment.SelectorCommit
		req.GitPinned = &pinned
	}
	if source := pin.Source.Archive; source != nil {
		pinned := source.RequestedSHA256 != ""
		req.ArchivePinned = &pinned
	}
	return req
}

func checkEnterprisePolicyClosure(ctx context.Context, app App, lock environment.Lock) (string, error) {
	if app.InstallationPolicy == nil {
		return "", nil
	}
	unrestricted, err := enterprisepolicy.IsUnrestricted(ctx, app.InstallationPolicy)
	if err != nil {
		return "", err
	}
	if unrestricted {
		return app.checkEnterprisePolicy(ctx, domain.InstallPolicyRequest{})
	}
	var fingerprint string
	for _, pin := range lock.Packages {
		req := lockedPolicyRequest(pin, app.scope())
		observed, err := app.checkEnterprisePolicy(ctx, req)
		if err == nil {
			_, err = inspectEnterprisePolicyPin(ctx, app, pin, func(envelope domain.PackageEnvelope) error {
				req.Envelope = &envelope
				var checkErr error
				observed, checkErr = app.checkEnterprisePolicy(ctx, req)
				return checkErr
			})
		}
		if err != nil {
			return "", fmt.Errorf("installation policy for %s: %w", domain.PolicySafeText(pin.Alias), err)
		}
		if fingerprint != "" && fingerprint != observed {
			return "", domain.ErrInstallPolicyChanged
		}
		fingerprint = observed
	}
	return fingerprint, nil
}

// Use the integrity check's loader on existing stored bytes, or the command's
// already acquired snapshot. This helper never fetches a source or scans it.
func inspectEnterprisePolicyPin(ctx context.Context, app App, pin environment.LockedPackage, visit func(domain.PackageEnvelope) error) (environment.ResolvedPackage, error) {
	resolved, err := inspectPinnedDependency(ctx, app, pin, visit)
	if !errors.Is(err, environmentstore.ErrMissing) || app.dependencySnapshots == nil {
		return resolved, err
	}
	key, keyErr := lockedDependencySnapshotKey(pin)
	if keyErr != nil {
		return environment.ResolvedPackage{}, keyErr
	}
	acquired, ok := app.dependencySnapshots.snapshots[key]
	if !ok {
		return resolved, err
	}
	snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(ctx, acquired.Root, identityFromLocked(pin), acquired.ExecutableFiles)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	if snapshot.TreeDigest != pin.TreeDigest || snapshot.DigestAlgorithm != pin.DigestAlgorithm {
		_ = packagedigest.Remove(snapshot)
		return environment.ResolvedPackage{}, errors.New("policy package snapshot no longer matches uap.lock")
	}
	return inspectDependencySnapshot(ctx, app, pin, snapshot, visit)
}

func checkEnterpriseActionPolicies(ctx context.Context, app App, actions []dependencyAction, removalIntent, repairOnly bool) error {
	if app.InstallationPolicy == nil || policyRemovalOnly(actions, removalIntent) {
		return nil
	}
	if err := checkEnterpriseRepairPolicies(ctx, app, actions, repairOnly); err != nil {
		return err
	}
	packages := map[string]environment.LockedPackage{}
	for _, action := range actions {
		if action.Package == nil || action.Action == environment.ActionRemove {
			continue
		}
		pin, exists := packages[action.Alias]
		if !exists {
			pin = *action.Package
			pin.Targets = nil
		}
		pin.Targets = append(pin.Targets, action.ClientID)
		packages[action.Alias] = pin
	}
	aliases := make([]string, 0, len(packages))
	for alias := range packages {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	var lock environment.Lock
	for _, alias := range aliases {
		lock.Packages = append(lock.Packages, packages[alias])
	}
	_, err := checkEnterprisePolicyClosure(ctx, app, lock)
	return err
}

func policyRemovalOnly(actions []dependencyAction, removalIntent bool) bool {
	removing := removalIntent
	for _, action := range actions {
		switch action.Action {
		case environment.ActionRemove:
			removing = true
		case environment.ActionUnchanged:
		default:
			return false
		}
	}
	return removing
}

func checkEnterprisePreparedPolicy(ctx context.Context, app App, request *dependencySyncRequest) error {
	if app.InstallationPolicy == nil || policyRemovalOnly(request.prepared, request.removalOnly) {
		return nil
	}
	if err := checkEnterpriseRepairPolicies(ctx, app, request.prepared, request.repairOnly); err != nil {
		return err
	}
	if request.repairOnly {
		return nil
	}
	aliases := request.lock.ClosureAliases(request.aliases)
	lock := environmentapp.SelectedLock(request.lock, aliases)
	_, err := checkEnterprisePolicyClosure(ctx, app, lock)
	return err
}

// Declaration integrity covers the saved lock. Installation admission covers
// only the selected closure; reducing intent does not authorize new installs.
func verifySelectedPolicyDeclarations(ctx context.Context, app App, lock environment.Lock, aliases []string, removalOnly bool) error {
	if !removalOnly {
		selected := environmentapp.SelectedLock(lock, lock.ClosureAliases(aliases))
		for _, pin := range selected.Packages {
			if _, err := app.checkEnterprisePolicy(ctx, lockedPolicyRequest(pin, app.scope())); err != nil {
				return err
			}
		}
	}
	return verifyDependencyDeclarations(ctx, app, lock)
}

func (request dependencySyncRequest) recheckDependencyPolicy(ctx context.Context, app App, path string) error {
	if policyRemovalOnly(request.prepared, request.removalOnly) {
		return nil
	}
	if path == "" {
		return checkEnterprisePreparedPolicy(ctx, app, &request)
	}
	// Explicit policy retains its existing complete-lock scope and reviewed
	// fingerprint. Automatic policy otherwise checks only the selected closure.
	observed, err := evaluateLocalDependencyPolicy(ctx, app, path, request.lock)
	if err != nil {
		return err
	}
	if observed != request.policyFingerprint {
		return fmt.Errorf("local dependency policy changed after confirmation: %w", installer.ErrPlanChanged)
	}
	return nil
}

func enterprisePolicyApplyGuard(ctx context.Context, app App, request *dependencySyncRequest, original func() error) func() error {
	return func() error {
		if original != nil {
			if err := original(); err != nil {
				return err
			}
		}
		return checkEnterprisePreparedPolicy(ctx, app, request)
	}
}
