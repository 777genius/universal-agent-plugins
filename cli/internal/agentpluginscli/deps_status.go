package agentpluginscli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyStatusFact struct {
	Scope            domain.InstallScope       `json:"-"`
	Alias            string                    `json:"alias,omitempty"`
	ClientID         string                    `json:"client_id,omitempty"`
	InstallationID   string                    `json:"installation_id,omitempty"`
	BindingID        string                    `json:"binding_id,omitempty"`
	NextActions      []dependencyNextActionDTO `json:"next_actions,omitempty"`
	Owner            string                    `json:"owner,omitempty"`
	Revision         string                    `json:"revision,omitempty"`
	PackageDigest    string                    `json:"package_digest,omitempty"`
	ProjectionDigest string                    `json:"projection_digest,omitempty"`
	Facts            []string                  `json:"facts"`
}

type dependencyExcludedTarget struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type dependencyStatus struct {
	ExcludedTargets  []dependencyExcludedTarget `json:"excluded_targets"`
	Scope            string                     `json:"scope"`
	ManifestPath     string                     `json:"manifest_path"`
	LockPath         string                     `json:"lock_path"`
	SupportedTargets []string                   `json:"supported_targets"`
	ManifestDigest   string                     `json:"manifest_digest,omitempty"`
	LockDigest       string                     `json:"lock_digest,omitempty"`
	PendingRecovery  bool                       `json:"pending_recovery"`
	ValidLock       bool                       `json:"-"`
	Facts            []dependencyStatusFact     `json:"facts"`
}

func runDependencyStatus(cmd *cobra.Command, app App, opts *options) error {
	manifestPath, lockPath := app.dependencyPaths()
	status := dependencyStatus{Scope: string(app.scope()), ManifestPath: manifestPath, LockPath: lockPath, SupportedTargets: supportedDependencyTargets(app), Facts: []dependencyStatusFact{}}
	status.ExcludedTargets = []dependencyExcludedTarget{}
	for _, definition := range domain.ClientDefinitions() {
		reason := installer.ClientSupportExclusionReason(app.ClientRegistry, string(definition.ID))
		if app.scope() == domain.ScopeProject {
			reason = ""
			if !supportsDependencyTarget(app, string(definition.ID)) {
				reason = "project_scope_unsupported"
			}
		}
		if reason != "" && reason != "not_registered" {
			status.ExcludedTargets = append(status.ExcludedTargets, dependencyExcludedTarget{ID: string(definition.ID), Reason: reason})
		}
	}

	requirements, err := readDependencyManifest(manifestPath)
	missingManifest := os.IsNotExist(err)
	if err != nil && !missingManifest {
		return err
	}
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	var lock environment.Lock
	validLock := false
	if missingManifest {
		status.Facts = append(status.Facts, dependencyStatusFact{Facts: []string{"manifest_missing"}})
	} else {
		status.ManifestDigest, _ = environment.ManifestDigest(requirements)
		var lockErr error
		lock, lockErr = readDependencyLock(lockPath, requirements)
		validLock = lockErr == nil
		status.ValidLock = validLock
		if lockErr != nil {
			status.Facts = append(status.Facts, dependencyStatusFact{Facts: []string{"not_synced"}})
		} else {
			if err := canonicalizeRequirementsFromLock(&requirements, lock); err != nil {
				return err
			}
			status.ManifestDigest, _ = environment.ManifestDigest(requirements)
			status.LockDigest = lock.ManifestDigest
		}
	}
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	status.PendingRecovery = state.Recovery.Required

	status.Facts = append(status.Facts, inspectDependencyFacts(cmd.Context(), app, state, lock, validLock)...)
	if status.PendingRecovery {
		status.Facts = append(status.Facts, dependencyStatusFact{Facts: []string{"pending_recovery"}})
	}
	return renderDependencyStatus(cmd, opts, status)
}

func canonicalizeRequirementsFromLock(requirements *environment.Requirements, lock environment.Lock) error {
	for key, requirement := range requirements.Dependencies {
		if requirement.Registry == nil || strings.Contains(requirement.Registry.Distribution, "/") {
			continue
		}
		var candidates []environment.LockedPackage
		for _, pkg := range lock.Packages {
			source := pkg.Source.Registry
			if source == nil {
				continue
			}
			if strings.EqualFold(requirement.Registry.Distribution, source.ProductID) || strings.EqualFold(requirement.Registry.Distribution, path.Base(source.DistributionID)) {
				candidates = append(candidates, pkg)
			}
		}
		// Preserve an existing unambiguous alias association; otherwise require a
		// unique source match. Full lock validation follows this offline lookup.
		var selected *environment.LockedPackage
		for i := range candidates {
			if candidates[i].Alias == key {
				selected = &candidates[i]
				break
			}
		}
		if selected == nil && len(candidates) == 1 {
			selected = &candidates[0]
		}
		if len(candidates) == 0 {
			continue
		}
		if selected == nil {
			return fmt.Errorf("dependency %q cannot be mapped unambiguously to uap.lock registry facts; use its canonical distribution", key)
		}
		source := *requirement.Registry
		source.Distribution = selected.Source.Registry.DistributionID
		requirement.Registry = &source
		requirements.Dependencies[key] = requirement
	}
	return nil
}

func appendBindingStatusFacts(fact *dependencyStatusFact, binding installer.InspectedBinding, pkg environment.LockedPackage, environmentIDs ...string) {
	mergeDependencyGuidanceScope(fact, binding.Scope)
	_, lifecycle := environment.LifecycleFacts(binding)
	fact.Facts = append(fact.Facts, lifecycle...)
	fact.NextActions = dependencyNextActions(*fact)
	fact.Revision, fact.ProjectionDigest = binding.Revision, binding.ProjectionDigest
	if binding.TreeDigest != pkg.TreeDigest || binding.Revision != lockedCommit(pkg) {
		fact.Facts = append(fact.Facts, "not_synced")
	}
	if binding.Owner == nil || binding.Owner.EnvironmentID != selectedEnvironmentID(environmentIDs) {
		fact.Facts = append(fact.Facts, "foreign")
	}
	if binding.ProjectionStatus != "" {
		fact.Facts = append(fact.Facts, binding.ProjectionStatus)
	}
}

func findInspectedEnvironmentBinding(view installer.Inspection, receipt domain.EnvironmentReceipt) (installer.InspectedBinding, bool) {
	for _, installation := range view.Installations {
		if installation.InstallationID == receipt.InstallationID {
			for _, binding := range installation.Bindings {
				if binding.BindingID == receipt.ClientBindingID {
					return binding, true
				}
			}
		}
	}
	return installer.InspectedBinding{}, false
}

func lockedCommit(pkg environment.LockedPackage) string {
	return pkg.Revision()
}

func joinUnique(values []string) string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return "current"
	}
	result := ""
	for index, value := range out {
		if index > 0 {
			result += ","
		}
		result += value
	}
	return result
}

func runDependencyRecover(cmd *cobra.Command, app App, opts *options, yes bool) error {
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	observed, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	if !observed.Recovery.Required {
		if opts.format == "json" {
			return renderJSON(cmd.OutOrStdout(), map[string]any{"outcome": "unchanged", "recovery": observed.Recovery})
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No dependency recovery is pending.")
		return err
	}
	if opts.dryRun {
		return renderJSON(cmd.OutOrStdout(), map[string]any{"dry_run": true, "recovery": observed.Recovery})
	}

	if opts.format == "json" && !yes {
		return errors.New("dependency recover with --format json requires --yes")
	}
	if !yes {
		reader := bufio.NewReader(cmd.InOrStdin())
		yes, err = promptYesNo(cmd.Context(), reader, cmd.OutOrStdout(), cmd.ErrOrStderr(), "Recover the complete owned dependency scope? [y/N]")
		if err != nil {
			return err
		}
	}
	if !yes {
		if opts.format == "json" {
			return renderJSON(cmd.OutOrStdout(), map[string]any{"outcome": "canceled", "recovery": observed.Recovery})
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return err
	}
	release, lockErr := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
	if lockErr != nil {
		return lockErr
	}
	defer func() { _ = release() }()
	result, recoverErr := app.Installer.RecoverEnvironment(cmd.Context(), observed, app.environmentID())
	if opts.format == "json" {
		if renderErr := renderJSON(cmd.OutOrStdout(), map[string]any{"result": result, "recovery": observed.Recovery}); renderErr != nil && recoverErr == nil {
			return renderErr
		}
	} else if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Recovery outcome: %s\nRun deps sync separately after recovery.\n", result.Outcome); writeErr != nil && recoverErr == nil {
		return writeErr
	}
	return recoverErr
}

func renderDependencyStatus(cmd *cobra.Command, opts *options, status dependencyStatus) error {
	sort.SliceStable(status.Facts, func(i, j int) bool {
		if status.Facts[i].Alias != status.Facts[j].Alias {
			return status.Facts[i].Alias < status.Facts[j].Alias
		}
		return status.Facts[i].ClientID < status.Facts[j].ClientID
	})
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), status)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Dependency environment (%s)\nManifest: %s\nLock: %s\nSupported targets: %s\n", status.Scope, status.ManifestPath, status.LockPath, strings.Join(status.SupportedTargets, ", ")); err != nil {
		return err
	}
	for _, fact := range status.Facts {
		name := fact.Alias
		if fact.ClientID != "" {
			name += "/" + fact.ClientID
		}
		if name == "" {
			name = "environment"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", name, joinUnique(fact.Facts)); err != nil {
			return err
		}
		for _, next := range fact.NextActions {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  Next:", next.Kind, next.Reason); err != nil {
				return err
			}
		}
	}
	return renderDependencyStatusGuidance(cmd.OutOrStdout(), status)
}

func renderDependencyStatusGuidance(writer io.Writer, status dependencyStatus) error {
	missing := false
	for _, fact := range status.Facts {
		for _, code := range fact.Facts {
			missing = missing || code == "manifest_missing"
		}
	}
	var explanation string
	switch {
	case status.PendingRecovery:
		explanation = "Recovery is pending. Inspect it before any other changes: agentplugins deps recover --dry-run"
	case missing:
		explanation = "No saved uap.toml was found. This does not mean other plugins are absent. To save a supported Registry/Git package, start with agentplugins add <source> --target <agent>"
	case !status.ValidLock:
		explanation = "No valid uap.lock was found. Review uap.toml before explicitly running agentplugins deps lock; locked installation is not available yet."
	default:
		explanation = "Inspect exact saved versions and dependency relationships: agentplugins deps tree"
	}
	if _, err := fmt.Fprintln(writer, "\n"+explanation); err != nil {
		return err
	}
	if status.Scope == string(domain.ScopeProject) {
		_, err := fmt.Fprintln(writer, "For this project, use --scope project and the same --project-root directory with the suggested command.")
		return err
	}
	_, err := fmt.Fprintln(writer, "For this user environment, use --scope user with the suggested command.")
	return err
}

func inspectDependencyFacts(ctx context.Context, app App, state installer.Inspection, lock environment.Lock, validLock bool) []dependencyStatusFact {
	receipts := map[string]domain.EnvironmentReceipt{}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == app.environmentID() {
			receipts[receipt.Alias+"\x00"+receipt.ClientID] = receipt
		}
	}
	var facts []dependencyStatusFact
	if validLock {
		for _, pkg := range lock.Packages {
			for _, target := range pkg.Targets {
				key := pkg.Alias + "\x00" + target
				receipt, owned := receipts[key]
				facts = append(facts, desiredDependencyFact(ctx, app, state, pkg, target, receipt, owned))
				delete(receipts, key)
			}
		}
	}
	for _, receipt := range receipts {
		fact := dependencyStatusFact{Scope: app.scope(), Alias: receipt.Alias, ClientID: receipt.ClientID, PackageDigest: receipt.PackageDigest, Facts: []string{"not_synced"}}
		appendStoreStatus(ctx, app, &fact, domain.TreeDigestAlgorithm)
		appendReceiptStatus(&fact, state, receipt)
		facts = append(facts, fact)
	}
	return facts
}

func desiredDependencyFact(ctx context.Context, app App, state installer.Inspection, pkg environment.LockedPackage, target string, receipt domain.EnvironmentReceipt, owned bool) dependencyStatusFact {
	fact := dependencyStatusFact{Scope: app.scope(), Alias: pkg.Alias, ClientID: target, PackageDigest: pkg.TreeDigest}
	appendStoreStatus(ctx, app, &fact, pkg.DigestAlgorithm)
	if !owned {
		fact.Facts = append(fact.Facts, "not_synced")
		if hasDependencyBinding(state, pkg.SourceKey, target) {
			fact.Facts = append(fact.Facts, "foreign")
		}
		return fact
	}
	if receipt.SourceKey != pkg.SourceKey || receipt.PackageDigest != pkg.TreeDigest {
		fact.Facts = append(fact.Facts, "not_synced")
	}
	fact.InstallationID, fact.BindingID, fact.Owner = receipt.InstallationID, receipt.ClientBindingID, receipt.EnvironmentID
	if binding, found := findInspectedEnvironmentBinding(state, receipt); found {
		appendBindingStatusFacts(&fact, binding, pkg, app.environmentID())
	} else {
		fact.Facts = append(fact.Facts, "partial")
	}
	return fact
}

func hasDependencyBinding(state installer.Inspection, sourceKey, target string) bool {
	for _, installation := range state.Installations {
		if installation.SourceKey != sourceKey {
			continue
		}
		for _, binding := range installation.Bindings {
			if binding.ClientID == target {
				return true
			}
		}
	}
	return false
}

func appendStoreStatus(ctx context.Context, app App, fact *dependencyStatusFact, algorithm string) {
	if _, err := app.dependencyStore().Get(ctx, algorithm, fact.PackageDigest); err != nil {
		if errors.Is(err, environmentstore.ErrMismatch) {
			fact.Facts = append(fact.Facts, "store_mismatch")
		} else {
			fact.Facts = append(fact.Facts, "store_missing")
		}
	}
}

func appendReceiptStatus(fact *dependencyStatusFact, state installer.Inspection, receipt domain.EnvironmentReceipt) {
	fact.InstallationID, fact.BindingID, fact.Owner = receipt.InstallationID, receipt.ClientBindingID, receipt.EnvironmentID
	binding, found := findInspectedEnvironmentBinding(state, receipt)
	if !found {
		fact.Facts = append(fact.Facts, "partial")
		return
	}
	mergeDependencyGuidanceScope(fact, binding.Scope)
	fact.Revision, fact.ProjectionDigest = binding.Revision, binding.ProjectionDigest
	if binding.ProjectionStatus != "" {
		fact.Facts = append(fact.Facts, binding.ProjectionStatus)
	}
	_, lifecycle := environment.LifecycleFacts(binding)
	fact.Facts = append(fact.Facts, lifecycle...)
	fact.NextActions = dependencyNextActions(*fact)
}

func dependencyNextActions(fact dependencyStatusFact, sdk ...installer.NextAction) []dependencyNextActionDTO {
	out := make([]dependencyNextActionDTO, 0, len(sdk)+1)
	for _, next := range sdk {
		out = append(out, dependencyNextActionDTO{Kind: next.Kind, Reason: next.Reason})
	}
	for _, item := range fact.Facts {
		if item == "activation_manual" || item == "activation_prepared" {
			if fact.Scope == domain.ScopeProject {
				out = append(out, dependencyNextActionDTO{Kind: "review_project_activation", Reason: "review the selected project's client instructions and native loading, then retry supported verification in the same selected project; prepared files do not establish native activation"})
			} else {
				out = append(out, dependencyNextActionDTO{Kind: "attest_activation", Reason: fmt.Sprintf("agentplugins add <same-source> --target %s --activation-complete", fact.ClientID)})
			}
		}
		if item == "auth_pending" || item == "auth_failed" {
			return append(out, dependencyNextActionDTO{Kind: "attest_authentication", Reason: fmt.Sprintf("agentplugins add <same-source> --target %s --auth-complete", fact.ClientID)})
		}
	}
	return out
}

// Never downgrade a project context to user guidance because inspected metadata
// disagrees. This chooses conservative text only; it confers no authority.
func mergeDependencyGuidanceScope(fact *dependencyStatusFact, bindingScope string) {
	if fact.Scope == domain.ScopeProject || bindingScope == string(domain.ScopeProject) {
		fact.Scope = domain.ScopeProject
		return
	}
	if fact.Scope == "" {
		fact.Scope = domain.InstallScope(bindingScope)
	}
}
