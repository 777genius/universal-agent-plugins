package agentpluginscli

import (
	"bufio"
	"errors"
	"fmt"
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
	Alias, ClientID, InstallationID, BindingID string   `json:",omitempty"`
	Owner                                      string   `json:"owner,omitempty"`
	Revision                                   string   `json:"revision,omitempty"`
	PackageDigest                              string   `json:"package_digest,omitempty"`
	ProjectionDigest                           string   `json:"projection_digest,omitempty"`
	Facts                                      []string `json:"facts"`
}

type dependencyStatus struct {
	ManifestDigest  string                 `json:"manifest_digest,omitempty"`
	LockDigest      string                 `json:"lock_digest,omitempty"`
	PendingRecovery bool                   `json:"pending_recovery"`
	Facts           []dependencyStatusFact `json:"facts"`
}

func runDependencyStatus(cmd *cobra.Command, app App, opts *options) error {
	if app.Installer == nil {
		return errors.New("typed SDK installer is unavailable")
	}
	manifestPath, lockPath := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifestPath)
	if err != nil {
		return err
	}
	status := dependencyStatus{}
	status.ManifestDigest, _ = environment.ManifestDigest(requirements)
	lock, lockErr := readDependencyLock(lockPath, requirements)
	if lockErr != nil {
		status.Facts = append(status.Facts, dependencyStatusFact{Facts: []string{"not_synced"}})
	} else {
		canonicalizeRequirementsFromLock(&requirements, lock)
		status.ManifestDigest, _ = environment.ManifestDigest(requirements)
		status.LockDigest = lock.ManifestDigest
	}
	state, err := app.Installer.Inspect(cmd.Context())
	if err != nil {
		return err
	}
	status.PendingRecovery = state.Recovery.Required

	receipts := map[string]domain.EnvironmentReceipt{}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == environment.EnvironmentUserDefault {
			receipts[receipt.Alias+"\x00"+receipt.ClientID] = receipt
		}
	}
	if lockErr == nil {
		for _, pkg := range lock.Packages {
			for _, target := range pkg.Targets {
				fact := dependencyStatusFact{Alias: pkg.Alias, ClientID: target, PackageDigest: pkg.TreeDigest}
				if _, err := app.dependencyStore().Get(cmd.Context(), pkg.DigestAlgorithm, pkg.TreeDigest); err != nil {
					if errors.Is(err, environmentstore.ErrMismatch) {
						fact.Facts = append(fact.Facts, "store_mismatch")
					} else {
						fact.Facts = append(fact.Facts, "store_missing")
					}
				}
				receipt, owned := receipts[pkg.Alias+"\x00"+target]
				if !owned {
					fact.Facts = append(fact.Facts, "not_synced")
					for _, installation := range state.Installations {
						if installation.SourceKey == pkg.SourceKey {
							for _, binding := range installation.Bindings {
								if binding.ClientID == target {
									fact.Facts = append(fact.Facts, "foreign")
								}
							}
						}
					}
					status.Facts = append(status.Facts, fact)
					continue
				}
				fact.InstallationID, fact.BindingID, fact.Owner = receipt.InstallationID, receipt.ClientBindingID, receipt.EnvironmentID
				if receipt.SourceKey != pkg.SourceKey || receipt.PackageDigest != pkg.TreeDigest {
					fact.Facts = append(fact.Facts, "not_synced")
				}
				binding, found := findInspectedEnvironmentBinding(state, receipt)
				if !found {
					fact.Facts = append(fact.Facts, "partial")
				} else {
					appendBindingStatusFacts(&fact, binding, pkg)
				}

				status.Facts = append(status.Facts, fact)
			}
		}
	}
	for key, receipt := range receipts {
		found := false
		if lockErr == nil {
			for _, pkg := range lock.Packages {
				for _, target := range pkg.Targets {
					if key == pkg.Alias+"\x00"+target {
						found = true
					}
				}
			}
		}
		if !found {
			status.Facts = append(status.Facts, dependencyStatusFact{Alias: receipt.Alias, ClientID: receipt.ClientID,
				InstallationID: receipt.InstallationID, BindingID: receipt.ClientBindingID, Owner: receipt.EnvironmentID,
				PackageDigest: receipt.PackageDigest, Facts: []string{"not_synced"}})
		}
	}
	if status.PendingRecovery {
		status.Facts = append(status.Facts, dependencyStatusFact{Facts: []string{"pending_recovery"}})
	}
	sort.SliceStable(status.Facts, func(i, j int) bool {
		if status.Facts[i].Alias != status.Facts[j].Alias {
			return status.Facts[i].Alias < status.Facts[j].Alias
		}
		return status.Facts[i].ClientID < status.Facts[j].ClientID
	})
	if opts.format == "json" {
		return renderJSON(cmd.OutOrStdout(), status)
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
	}
	return nil
}

func canonicalizeRequirementsFromLock(requirements *environment.Requirements, lock environment.Lock) {
	for alias, requirement := range requirements.Dependencies {
		if requirement.Registry == nil || strings.Contains(requirement.Registry.Distribution, "/") {
			continue
		}
		for _, pkg := range lock.Packages {
			if pkg.Alias == alias && pkg.Source.Registry != nil && (requirement.Registry.Distribution == pkg.Source.Registry.ProductID || requirement.Registry.Distribution == path.Base(pkg.Source.Registry.DistributionID)) {
				requirement.Registry.Distribution = pkg.Source.Registry.DistributionID
				requirements.Dependencies[alias] = requirement
				break
			}
		}
	}
}

func appendBindingStatusFacts(fact *dependencyStatusFact, binding installer.InspectedBinding, pkg environment.LockedPackage) {
	fact.Revision, fact.ProjectionDigest = binding.Revision, binding.ProjectionDigest
	if binding.TreeDigest != pkg.TreeDigest || binding.Revision != lockedCommit(pkg) {
		fact.Facts = append(fact.Facts, "not_synced")
	}
	if binding.Owner == nil || binding.Owner.EnvironmentID != environment.EnvironmentUserDefault {
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
	if pkg.Source.Git != nil {
		return pkg.Source.Git.Commit
	}
	if pkg.Source.Registry != nil {
		return pkg.Source.Registry.Commit
	}
	return ""
}

func findEnvironmentBinding(state domain.StateFileV2, receipt domain.EnvironmentReceipt) (domain.Installation, domain.ClientBinding, bool) {
	for _, installation := range state.Installations {
		if installation.InstallationID != receipt.InstallationID {
			continue
		}
		if binding, ok := installation.Clients[receipt.ClientBindingID]; ok {
			return installation, binding, true
		}
	}
	return domain.Installation{}, domain.ClientBinding{}, false
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
			return renderJSON(cmd.OutOrStdout(), map[string]any{"outcome": "cancelled", "recovery": observed.Recovery})
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return err
	}
	release, lockErr := processlock.Lock{Path: app.EnvironmentLockPath}.Acquire(cmd.Context())
	if lockErr != nil {
		return lockErr
	}
	defer func() { _ = release() }()
	result, recoverErr := app.Installer.RecoverEnvironment(cmd.Context(), observed, environment.EnvironmentUserDefault)
	if opts.format == "json" {
		if renderErr := renderJSON(cmd.OutOrStdout(), map[string]any{"result": result, "recovery": observed.Recovery}); renderErr != nil && recoverErr == nil {
			return renderErr
		}
	} else if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "Recovery outcome: %s\nRun deps sync separately after recovery.\n", result.Outcome); writeErr != nil && recoverErr == nil {
		return writeErr
	}
	return recoverErr
}
