package agentpluginscli

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type publicClient struct {
	InstallIntent             domain.InstallIntent           `json:"install_intent,omitempty"`
	BindingID                 string                         `json:"-"`
	ClientID                  string                         `json:"client_id"`
	Scope                     string                         `json:"scope"`
	Materialization           domain.MaterializationState    `json:"materialization"`
	Activation                domain.ActivationState         `json:"activation"`
	Authentication            domain.AuthenticationState     `json:"authentication"`
	Policy                    domain.PolicyState             `json:"policy"`
	Verification              domain.VerificationState       `json:"verification"`
	PackageRevision           *publicClientRevision          `json:"package_revision,omitempty"`
	AffectedSurfaces          []string                       `json:"affected_surfaces,omitempty"`
	ReceiptReconciled         *bool                          `json:"receipt_reconciled,omitempty"`
	NativeDiscoveryReconciled *bool                          `json:"native_discovery_reconciled,omitempty"`
	NativeIdentityState       domain.NativeIdentityState     `json:"native_identity_state,omitempty"`
	ClientVersion             string                         `json:"client_version,omitempty"`
	NativeDiscoveryEvidence   *publicNativeDiscoveryEvidence `json:"native_discovery_evidence,omitempty"`
}

type publicNativeDiscoveryEvidence struct {
	Basis              string                   `json:"basis"`
	VersionOperation   publicVersionOperation   `json:"version_operation"`
	DiscoveryOperation publicDiscoveryOperation `json:"discovery_operation"`
}

type publicVersionOperation struct {
	Argv                  []string `json:"argv"`
	ObservedClientVersion string   `json:"observed_client_version,omitempty"`
}

type publicDiscoveryOperation struct {
	Argv       []string `json:"argv"`
	Discovered bool     `json:"discovered"`
	ProductID  string   `json:"product_id"`
}

type publicClientRevision struct {
	Version          string           `json:"version,omitempty"`
	ResolvedRevision string           `json:"resolved_revision,omitempty"`
	DistributionID   string           `json:"distribution_id,omitempty"`
	ReleaseSequence  uint64           `json:"release_sequence,omitempty"`
	TreeDigest       string           `json:"tree_digest"`
	ManifestDigest   string           `json:"manifest_digest"`
	Evidence         []publicEvidence `json:"evidence,omitempty"`
}

type publicDetectedClient struct {
	ClientID    domain.ClientID        `json:"client_id"`
	DisplayName string                 `json:"display_name"`
	Status      domain.DetectionStatus `json:"status"`
	Version     string                 `json:"version,omitempty"`
	Surfaces    []domain.ClientSurface `json:"surfaces,omitempty"`
}

type publicInstallation struct {
	InstallationID    string                    `json:"installation_id"`
	Name              string                    `json:"name"`
	Version           string                    `json:"version,omitempty"`
	Source            string                    `json:"source"`
	NeedsRebind       bool                      `json:"needs_rebind,omitempty"`
	Clients           []publicClient            `json:"clients"`
	Directory         *publicInstalledDirectory `json:"directory,omitempty"`
	Warnings          []publicSafetyWarning     `json:"warnings,omitempty"`
	MixedVersion      bool                      `json:"mixed_version"`
	ConvergenceAction string                    `json:"convergence_action,omitempty"`
}

func publicInstallations(state domain.StateFileV2, includeAbsent bool) []publicInstallation {
	values := make([]publicInstallation, 0, len(state.Installations))
	for _, installation := range state.Installations {
		value := publicInstallationView(installation, includeAbsent)
		if len(value.Clients) == 0 && !includeAbsent {
			continue
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Name == values[j].Name {
			return values[i].InstallationID < values[j].InstallationID
		}
		return values[i].Name < values[j].Name
	})
	return values
}

func publicInstallationView(installation domain.Installation, includeAbsent bool) publicInstallation {
	value := publicInstallation{
		InstallationID: installation.InstallationID,
		Name:           installation.DeclaredName, Version: installation.Package.Version,
		Source: publicSource(installation.Source), NeedsRebind: installation.NeedsRebind,
	}
	if installation.Directory != nil {
		origin := installation.Directory
		value.Directory = &publicInstalledDirectory{ProductID: origin.ProductID,
			RecordedDistribution: origin.DistributionID, CurrentDistribution: origin.DistributionID,
			RecordedRevision: installation.Source.ResolvedRevision, CurrentRevision: installation.Source.ResolvedRevision,
			RecordedReleaseSequence: origin.DesiredReleaseSequence, CurrentReleaseSequence: origin.DesiredReleaseSequence,
			RecordedSnapshotSequence: origin.SnapshotSequence}
	}
	for _, client := range installation.Clients {
		if client.Materialization == domain.MaterializationAbsent && !includeAbsent {
			continue
		}
		affectedSurfaces := append([]string(nil), client.AffectedSurfaces...)
		sort.Strings(affectedSurfaces)
		value.Clients = append(value.Clients, publicClient{
			InstallIntent: client.InstallIntent,
			BindingID:     client.ClientBindingID,
			ClientID:      client.ClientID, Scope: client.Scope, Materialization: client.Materialization,
			Activation: client.Activation, Authentication: client.Authentication,
			Policy: client.Policy, Verification: client.Verification,
			PackageRevision:  publicPackageRevision(client.PackageRevision),
			AffectedSurfaces: affectedSurfaces,
		})
	}
	sort.Slice(value.Clients, func(i, j int) bool { return value.Clients[i].ClientID < value.Clients[j].ClientID })
	value.MixedVersion, value.ConvergenceAction = convergenceState(installation)
	return value
}

func publicPackageRevision(revision *domain.ClientPackageRevision) *publicClientRevision {
	if revision == nil {
		return nil
	}
	return &publicClientRevision{Version: revision.Version, ResolvedRevision: publicImmutableRevision(revision.ResolvedRevision),
		DistributionID: revision.DistributionID, ReleaseSequence: revision.ReleaseSequence,
		TreeDigest: revision.TreeDigest, ManifestDigest: revision.ManifestDigest}
}

func publicImmutableRevision(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 40 {
		return ""
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return ""
		}
	}
	return value
}

func publicSource(source domain.SourceBinding) string {
	if source.Repository != "" {
		return publicRepositoryPath(source.Repository, source.PackageSubpath)
	}
	canonical := strings.TrimSpace(source.CanonicalSource)
	if source.SourceKind != "" {
		canonical = source.SourceURL
		if source.SourceKind == "git" && !strings.Contains(canonical, "://") {
			return canonical
		}
	}
	if canonical == "" || filepath.IsAbs(canonical) {
		return "local"
	}
	parsed, err := url.Parse(canonical)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "local"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// publicRepositoryPath is presentation-only. The direct-source selector uses
// owner/repo@revision//subpath, but exposing that parser delimiter as a doubled
// slash makes a normal GitHub path look malformed in plans, JSON, and doctor.
func publicRepositoryPath(repository, packageSubpath string) string {
	repository = strings.Trim(repository, "/")
	packageSubpath = strings.Trim(packageSubpath, "/")
	if packageSubpath == "" {
		return repository
	}
	return repository + "/" + packageSubpath
}

func selectInstallation(state domain.StateFileV2, selector string) (domain.Installation, error) {
	selector = strings.TrimSpace(selector)
	var matches []domain.Installation
	for _, installation := range state.Installations {
		if installation.InstallationID == selector {
			return installation, nil
		}
		if installationMatchesSelector(installation, selector) {
			matches = append(matches, installation)
		}
	}
	if len(matches) == 0 {
		return domain.Installation{}, fmt.Errorf("installation %q was not found", selector)
	}
	if len(matches) > 1 {
		return domain.Installation{}, fmt.Errorf("installation name %q is ambiguous; use installation_id", selector)
	}
	return matches[0], nil
}

func renderInstallationList(writer io.Writer, installations []publicInstallation) error {
	if len(installations) == 0 {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Muted, "No Agent Plugins installations are tracked."))
		return nil
	}
	for _, installation := range installations {
		if err := renderInstallation(writer, installation); err != nil {
			return err
		}
	}
	return nil
}

func renderInstallation(writer io.Writer, installation publicInstallation) error {
	_, _ = fmt.Fprintf(writer, "%s %s (%s)\n", installation.Name, installation.Version, installation.InstallationID)
	_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Source"), installation.Source)
	for _, client := range installation.Clients {
		_, _ = fmt.Fprintf(writer, "  %s: materialization=%s activation=%s auth=%s verification=%s\n",
			terminaltheme.For(writer).Text(terminaltheme.Label, prompt.SafeText(string(client.ClientID))), client.Materialization, client.Activation, client.Authentication, client.Verification)
		if client.ReceiptReconciled != nil && client.NativeDiscoveryReconciled != nil {
			_, _ = fmt.Fprintf(writer, "    native_identity=%s receipt_reconciled=%t native_discovery_reconciled=%t client_version=%s\n",
				client.NativeIdentityState, *client.ReceiptReconciled, *client.NativeDiscoveryReconciled, client.ClientVersion)
		}
	}
	if installation.Directory != nil {
		value := installation.Directory
		_, _ = fmt.Fprintf(writer, "  %s: recorded=%s@%s release=%d current=%s@%s release=%d\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Directory"),
			value.RecordedDistribution, value.RecordedRevision, value.RecordedReleaseSequence,
			value.CurrentDistribution, value.CurrentRevision, value.CurrentReleaseSequence)
	}
	if installation.MixedVersion {
		_, _ = fmt.Fprintf(writer, "  Mixed version: true\n  Convergence: %s\n", installation.ConvergenceAction)
	}
	for _, warning := range installation.Warnings {
		_, _ = fmt.Fprintf(writer, "  %s [%s]: %s\n", terminaltheme.For(writer).Text(terminaltheme.Warning, "WARNING"), warning.Code, warning.Message)
		if warning.Action != "" {
			_, _ = fmt.Fprintf(writer, "    %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Action"), warning.Action)
		}
	}
	return nil
}
