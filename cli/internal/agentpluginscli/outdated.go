package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type outdatedReleaseIdentity struct {
	DistributionID  string `json:"distribution_id"`
	ReleaseSequence uint64 `json:"release_sequence"`
	PackageVersion  string `json:"package_version,omitempty"`
	Revision        string `json:"revision,omitempty"`
	TreeDigest      string `json:"tree_digest,omitempty"`
	ManifestDigest  string `json:"manifest_digest,omitempty"`
}

type outdatedInstallation struct {
	Alias          string                   `json:"alias,omitempty"`
	Source         string                   `json:"source,omitempty"`
	Requested      *environment.Selector    `json:"requested,omitempty"`
	RequiredBy     []environment.RequiredBy `json:"required_by,omitempty"`
	InstallationID string                   `json:"installation_id"`
	Name           string                   `json:"name"`
	Status         string                   `json:"status"`
	Reason         string                   `json:"reason"`
	Targets        []domain.ClientID        `json:"targets,omitempty"`
	Installed      *outdatedReleaseIdentity `json:"installed,omitempty"`
	Available      *outdatedReleaseIdentity `json:"available,omitempty"`
	Warnings       []publicSafetyWarning    `json:"warnings,omitempty"`
}

type outdatedResult struct {
	Pinned                int                    `json:"pinned"`
	CheckUnavailable      int                    `json:"check_unavailable"`
	Saved                 *savedUpdateCheck      `json:"saved,omitempty"`
	ReadOnly              bool                   `json:"read_only"`
	Snapshot              uint64                 `json:"snapshot_sequence,omitempty"`
	SnapshotHash          string                 `json:"snapshot_digest,omitempty"`
	DiscoverySnapshot     uint64                 `json:"discovery_snapshot_sequence,omitempty"`
	DiscoverySnapshotHash string                 `json:"discovery_snapshot_digest,omitempty"`
	Outdated              int                    `json:"outdated"`
	Current               int                    `json:"current"`
	Blocked               int                    `json:"blocked"`
	Unknown               int                    `json:"unknown"`
	Unmanaged             int                    `json:"unmanaged"`
	Installations         []outdatedInstallation `json:"installations"`
}

func newOutdatedCommand(app App, opts *options) *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "outdated [name-or-installation-id]",
		Short: "Check saved sources and tracked release identities without changing installations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			if all && len(args) > 0 {
				return fmt.Errorf("choose either one installation or --all")
			}
			selector := ""
			if len(args) == 1 {
				selector = args[0]
			}
			return runOutdated(cmd.Context(), cmd, app, opts, selector)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "check saved dependencies and tracked installations")
	return command
}

type outdatedSession struct {
	savedOwned     map[string]bool
	ctx            context.Context
	app            App
	opts           *options
	state          domain.StateFileV2
	installations  []domain.Installation
	result         outdatedResult
	needsDirectory bool
	bundle         directoryv1.VerifiedBundle
	bundleOK       bool
	discovery      discoveryv1.VerifiedBundle
	discoveryErr   error
	detected       map[domain.ClientID]domain.DetectedClient
	detectionErr   error
}

func runOutdated(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string) error {
	session := &outdatedSession{ctx: ctx, app: app, opts: opts}
	if err := session.loadInstallations(selector); err != nil {
		return err
	}
	if err := session.loadCatalogs(); err != nil {
		return err
	}
	session.inspectAll()
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "outdated", session.result)
	}
	return renderOutdated(cmd.OutOrStdout(), session.result)
}

func (session *outdatedSession) loadInstallations(selector string) error {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return err
	}
	session.state = state
	session.result = outdatedResult{ReadOnly: true, Installations: []outdatedInstallation{}}
	selectedSaved, err := session.loadSaved(selector)
	if err != nil {
		return err
	}
	installations := []domain.Installation{}
	for _, item := range state.Installations {
		if !session.savedOwned[item.InstallationID] {
			installations = append(installations, item)
		}
	}
	if selectedSaved {
		installations = nil
	} else if strings.TrimSpace(selector) != "" {
		installation, err := selectInstallation(state, selector)
		if err != nil {
			return err
		}
		installations = []domain.Installation{installation}
	}
	sort.Slice(installations, func(i, j int) bool {
		left, right := strings.ToLower(installations[i].DeclaredName), strings.ToLower(installations[j].DeclaredName)
		if left != right {
			return left < right
		}
		return installations[i].InstallationID < installations[j].InstallationID
	})
	session.state = state
	session.installations = installations
	return nil
}

func (session *outdatedSession) loadCatalogs() error {
	if err := session.loadDirectory(); err != nil {
		return err
	}
	session.loadDiscovery()
	session.loadDetection()
	return nil
}

func (session *outdatedSession) loadDirectory() error {
	for _, installation := range session.installations {
		if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil {
			session.needsDirectory = true
			break
		}
	}
	if !session.needsDirectory {
		return nil
	}
	bundle, bundleOK, err := directoryBundleForRead(session.ctx, session.app, session.state, true)
	if err != nil {
		return fmt.Errorf("load signed Directory: %w", err)
	}
	session.bundle, session.bundleOK = bundle, bundleOK
	if bundleOK {
		session.result.Snapshot, session.result.SnapshotHash = bundle.Snapshot.Sequence, bundle.Digest
	}
	return nil
}

func (session *outdatedSession) loadDiscovery() {
	needsDiscovery := false
	for _, installation := range session.installations {
		if strings.HasPrefix(strings.TrimSpace(installation.Source.RequestedSource), "discovery:") {
			needsDiscovery = true
			break
		}
	}
	if !needsDiscovery {
		return
	}
	if session.app.DiscoveryClient == nil {
		session.discoveryErr = errors.New("signed Discovery Index dependencies are unavailable")
		return
	}
	discovery, discoveryErr := session.app.DiscoveryClient.Load(session.ctx, 0)
	session.discovery, session.discoveryErr = discovery, discoveryErr
	if discoveryErr == nil {
		session.result.DiscoverySnapshot, session.result.DiscoverySnapshotHash = discovery.Snapshot.Sequence, discovery.Digest
	}
}

func (session *outdatedSession) loadDetection() {
	if !session.needsDirectory || !session.bundleOK {
		return
	}
	clients, err := detectClientsForLifecycleResolution(session.ctx, session.app.Detector, false)
	if err != nil {
		session.detectionErr = fmt.Errorf("detect AI clients: %w", err)
		return
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients)+1)
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	session.detected = detected
}

func (session *outdatedSession) inspectAll() {
	for _, installation := range session.installations {
		var item outdatedInstallation
		if strings.HasPrefix(strings.TrimSpace(installation.Source.RequestedSource), "discovery:") {
			item = inspectDiscoveryOutdated(session.discovery, session.discoveryErr, installation, session.opts.scope)
		} else {
			item = inspectOutdatedInstallation(session.bundle, session.bundleOK, session.detected, session.detectionErr, session.app.Version, installation, session.opts.scope)
		}
		session.result.Installations = append(session.result.Installations, item)
		session.countStatus(item.Status)
	}
}

func (session *outdatedSession) countStatus(status string) {
	switch status {
	case "pinned":
		session.result.Pinned++
	case "check_unavailable":
		session.result.CheckUnavailable++
	case "outdated":
		session.result.Outdated++
	case "current":
		session.result.Current++
	case "blocked":
		session.result.Blocked++
	case "unknown":
		session.result.Unknown++
	default:
		session.result.Unmanaged++
	}
}
