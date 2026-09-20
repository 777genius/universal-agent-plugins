package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
		Short: "Check immutable release identities without changing installations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
	command.Flags().BoolVar(&all, "all", false, "check every tracked installation")
	return command
}

type outdatedSession struct {
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
	installations := append([]domain.Installation(nil), state.Installations...)
	if strings.TrimSpace(selector) != "" {
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
	session.result = outdatedResult{ReadOnly: true, Installations: make([]outdatedInstallation, 0, len(installations))}
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

func inspectDiscoveryOutdated(bundle discoveryv1.VerifiedBundle, bundleErr error, installation domain.Installation, scope string) outdatedInstallation {
	item := discoveryOutdatedBase(installation, scope)
	if installation.NeedsRebind {
		item.Status, item.Reason = "blocked", "installation requires explicit rebind before update"
		return item
	}
	if bundleErr != nil {
		item.Status, item.Reason = "unknown", "no authenticated Discovery snapshot is available: "+bundleErr.Error()
		return item
	}
	record, ok := matchDiscoveryOutdatedRecord(bundle, strings.TrimSpace(installation.Source.RequestedSource))
	if !ok {
		item.Status, item.Reason = "blocked", "recorded Discovery selector is absent or ambiguous"
		return item
	}
	return compareDiscoveryRelease(record, installation, item)
}

func discoveryOutdatedBase(installation domain.Installation, scope string) outdatedInstallation {
	return outdatedInstallation{
		InstallationID: installation.InstallationID, Name: installation.DeclaredName, Targets: installationTargets(installation, scope),
		Installed: &outdatedReleaseIdentity{
			DistributionID: strings.TrimSpace(installation.Source.RequestedSource), PackageVersion: installation.Package.Version,
			Revision: installation.Source.ResolvedRevision, TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
		},
	}
}

func matchDiscoveryOutdatedRecord(bundle discoveryv1.VerifiedBundle, selector string) (discoveryv1.Record, bool) {
	var matches []discoveryv1.Record
	for _, record := range bundle.Search.Records {
		if record.Slug == selector {
			matches = append(matches, record)
		}
	}
	if len(matches) != 1 {
		return discoveryv1.Record{}, false
	}
	return matches[0], true
}

func compareDiscoveryRelease(record discoveryv1.Record, installation domain.Installation, item outdatedInstallation) outdatedInstallation {
	item.Available = &outdatedReleaseIdentity{DistributionID: record.Slug, Revision: record.Revision, TreeDigest: record.TreeDigest, ManifestDigest: record.ManifestDigest}
	if record.Version != nil {
		item.Available.PackageVersion = *record.Version
	}
	if record.Availability != "available" {
		item.Status, item.Reason = "blocked", "Discovery source is unavailable"
		return item
	}
	if record.Repository != installation.Source.Repository || record.PackagePath != installation.Source.PackageSubpath {
		item.Status, item.Reason = "blocked", "Discovery selector changed repository or package path; explicit switch required"
		return item
	}
	if record.Revision == installation.Source.ResolvedRevision && record.TreeDigest == installation.Source.TreeDigest && record.ManifestDigest == installation.Package.ManifestDigest {
		item.Status, item.Reason = "current", "the signed Discovery record matches the installed immutable package"
		item.Available = nil
		return item
	}
	item.Status, item.Reason = "outdated", "the same Discovery source has a newer immutable package identity"
	return item
}

func inspectOutdatedInstallation(bundle directoryv1.VerifiedBundle, bundleOK bool, detected map[domain.ClientID]domain.DetectedClient, detectionErr error, installerVersion string, installation domain.Installation, scope string) outdatedInstallation {
	item := outdatedInstallation{InstallationID: installation.InstallationID, Name: installation.DeclaredName}
	if status, reason, stop := outdatedIdentityGate(installation); stop {
		item.Status, item.Reason = status, reason
		return item
	}
	return inspectDirectoryOutdated(bundle, bundleOK, detected, detectionErr, installerVersion, installation, scope, item)
}

func outdatedIdentityGate(installation domain.Installation) (status, reason string, stop bool) {
	if installation.NeedsRebind {
		return "blocked", "installation requires explicit rebind before update", true
	}
	if installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return "unmanaged", "legacy plugin.yaml installation has no Agent Plugins release identity", true
	}
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return "unmanaged", "direct source has no signed update channel; use an explicit source switch", true
	}
	return "", "", false
}

func inspectDirectoryOutdated(
	bundle directoryv1.VerifiedBundle,
	bundleOK bool,
	detected map[domain.ClientID]domain.DetectedClient,
	detectionErr error,
	installerVersion string,
	installation domain.Installation,
	scope string,
	item outdatedInstallation,
) outdatedInstallation {
	origin := installation.Directory
	item.Targets = installationTargets(installation, scope)
	item.Installed = &outdatedReleaseIdentity{
		DistributionID: origin.DistributionID, ReleaseSequence: origin.DesiredReleaseSequence,
		PackageVersion: installation.Package.Version, Revision: installation.Source.ResolvedRevision,
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
	}
	if !bundleOK {
		item.Status, item.Reason = "unknown", "no authenticated Directory snapshot is available"
		return item
	}
	item.Warnings = installedDirectoryWarnings(bundle.Snapshot, installation)
	if status, reason, cloneInstalled, stop := directoryOutdatedPreflight(item, detectionErr, installation, scope); stop {
		item.Status, item.Reason = status, reason
		if cloneInstalled {
			item.Available = cloneOutdatedIdentity(item.Installed)
		}
		return item
	}
	return resolveDirectoryOutdated(bundle, detected, installerVersion, installation, origin, item)
}

func directoryOutdatedPreflight(item outdatedInstallation, detectionErr error, installation domain.Installation, scope string) (status, reason string, cloneInstalled, stop bool) {
	if detectionErr != nil {
		return "unknown", detectionErr.Error(), false, true
	}
	if len(item.Targets) == 0 {
		return "blocked", "installation has no materialized target in the selected scope", false, true
	}
	if selectedTargetsNeedDesiredRelease(installation, item.Targets, scope) {
		return "outdated", "one or more installed clients have not converged to the recorded desired release", true, true
	}
	return "", "", false, false
}

func resolveDirectoryOutdated(
	bundle directoryv1.VerifiedBundle,
	detected map[domain.ClientID]domain.DetectedClient,
	installerVersion string,
	installation domain.Installation,
	origin *domain.DirectoryOrigin,
	item outdatedInstallation,
) outdatedInstallation {
	_, clientMap, err := preflightSelectedTargets(context.Background(), App{Detector: staticDetectedClientSource(detected)}, item.Targets, detectedClientValues(detected), false)
	if err != nil {
		item.Status, item.Reason = "blocked", err.Error()
		return item
	}
	environment := directoryEnvironment(clientMap)
	environment.InstallerVersion = installerVersion
	selection, err := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{
		Selector: origin.ProductID, Targets: expandAffectedSurfaceTargets(item.Targets), Scope: domain.ScopeUser,
		InstallerVersion: installerVersion, ClientVersions: environment.ClientVersions, OS: environment.OS,
		Architecture: environment.Architecture, DependencyIdentity: environment.DependencyIdentity,
		SchemaVersion: "1.0.0", Operation: domain.DirectoryUpdate,
		Recorded: recordedDirectoryRelease(installation, origin.DesiredReleaseSequence, installation.Source.ResolvedRevision),
	})
	if err == nil {
		item.Status, item.Reason = "outdated", "a later eligible immutable release is available"
		item.Available = &outdatedReleaseIdentity{
			DistributionID: selection.DistributionID, ReleaseSequence: selection.ReleaseSequence,
			PackageVersion: selection.PackageVersion, Revision: selection.Source.Revision,
			TreeDigest: selection.TreeDigest, ManifestDigest: selection.ManifestDigest,
		}
		return item
	}
	return directoryOutdatedResolveError(err, item)
}

func directoryOutdatedResolveError(err error, item outdatedInstallation) outdatedInstallation {
	if errors.Is(err, domain.ErrDirectoryIneligible) && errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		if len(item.Warnings) > 0 {
			item.Status, item.Reason = "blocked", "installed release is unsafe and no later eligible release is available"
		} else {
			item.Status, item.Reason = "current", "no later eligible immutable release exists"
		}
		return item
	}
	item.Status, item.Reason = "blocked", err.Error()
	return item
}

// staticDetectedClientSource is used only to reuse the exact client-surface
// preflight rules for a previously detected, read-only snapshot.
type staticDetectedClientSource map[domain.ClientID]domain.DetectedClient

func (source staticDetectedClientSource) Detect(context.Context) ([]domain.DetectedClient, error) {
	return detectedClientValues(source), nil
}

func cloneOutdatedIdentity(identity *outdatedReleaseIdentity) *outdatedReleaseIdentity {
	if identity == nil {
		return nil
	}
	clone := *identity
	return &clone
}

func renderOutdated(writer io.Writer, result outdatedResult) error {
	if len(result.Installations) == 0 {
		_, err := fmt.Fprintln(writer, "No tracked Agent Plugins installations.")
		return err
	}
	for _, installation := range result.Installations {
		if _, err := fmt.Fprintf(writer, "%s: %s - %s\n", installation.Name, installation.Status, installation.Reason); err != nil {
			return err
		}
		if err := renderOutdatedAvailable(writer, installation); err != nil {
			return err
		}
	}
	return nil
}

func renderOutdatedAvailable(writer io.Writer, installation outdatedInstallation) error {
	if installation.Available == nil {
		return nil
	}
	label := fmt.Sprintf("%s release %d", installation.Available.DistributionID, installation.Available.ReleaseSequence)
	if installation.Available.ReleaseSequence == 0 {
		label = installation.Available.DistributionID
	}
	_, err := fmt.Fprintf(writer, "  Available: %s (%s)\n", label, installation.Available.Revision)
	return err
}
