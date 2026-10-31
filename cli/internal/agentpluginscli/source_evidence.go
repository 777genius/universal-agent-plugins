package agentpluginscli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/catalog"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func stableCatalogPackageMode(mode domain.PackageMode) (string, bool) {
	switch mode {
	case domain.PackageNative:
		return "native", true
	case domain.PackageProjection:
		return "projected", true
	case domain.PackagePrepared:
		return "prepared", true
	default:
		return "", false
	}
}

func validateDirectoryCompatibilityPolicy(policy domain.DirectoryReleasePolicy) error {
	for _, target := range policy.Targets {
		capabilities, ok := clientplanner.Capabilities(target.Client)
		if !ok {
			return fmt.Errorf("signed Directory target %q is unsupported", target.Client)
		}
		_, ok = stableCatalogPackageMode(capabilities.PackageMode)
		if !ok {
			return fmt.Errorf("signed Directory target %q has unsupported package mode %q", target.Client, capabilities.PackageMode)
		}
		expectedDelivery, ok := domain.ExpectedDirectoryDelivery(target.Client)
		if !ok || target.Delivery != expectedDelivery {
			return fmt.Errorf("signed Directory delivery %q is incompatible with target %q; expected %q", target.Delivery, target.Client, expectedDelivery)
		}
	}
	return nil
}

type directoryEvidenceEnvironment struct {
	InstallerVersion   string
	OS                 string
	Architecture       string
	ClientVersions     map[domain.ClientID]string
	DependencyIdentity map[domain.ClientID]string
}

func directoryEnvironment(clients map[domain.ClientID]domain.DetectedClient) directoryEvidenceEnvironment {
	versions := make(map[domain.ClientID]string, len(clients))
	for id, client := range clients {
		if client.ClientID == id && client.Version != "" {
			versions[id] = client.Version
		}
	}
	return directoryEvidenceEnvironment{OS: runtime.GOOS, Architecture: runtime.GOARCH, ClientVersions: versions, DependencyIdentity: map[domain.ClientID]string{}}
}

// deterministicDependencyIdentity returns a value only when the immutable
// package declares exactly one external stdio command. It inspects manifest
// metadata only; it does not resolve or execute the dependency.
func deterministicDependencyIdentity(envelope domain.PackageEnvelope, targets []domain.ClientID) map[domain.ClientID]string {
	commands := map[string]struct{}{}
	for _, server := range envelope.MCP.Servers {
		if requirement := server.StdioRequirement; requirement != nil && requirement.Kind == domain.ExecutableBare && strings.TrimSpace(requirement.Command) != "" {
			commands[requirement.Command] = struct{}{}
		}
	}
	if len(commands) != 1 {
		return map[domain.ClientID]string{}
	}
	var identity string
	for command := range commands {
		identity = command
	}
	result := make(map[domain.ClientID]string, len(targets))
	for _, target := range targets {
		result[target] = identity
	}
	return result
}

func applyDirectoryCompatibility(loaded *loadedPackage, bundle directoryv1.VerifiedBundle, selection domain.DirectorySelection, policy domain.DirectoryReleasePolicy, environment directoryEvidenceEnvironment) error {
	if err := validateDirectoryCompatibilityPolicy(policy); err != nil {
		return err
	}
	compatibility := make(map[string]domain.CatalogCompatibility, len(policy.Targets))
	current := currentDirectoryEvidence(bundle.Snapshot.Sequence, bundle.Snapshot.Evidence, policy.CurrentEvidence)
	for _, target := range policy.Targets {
		capabilities, _ := clientplanner.Capabilities(target.Client)
		packageMode, _ := stableCatalogPackageMode(capabilities.PackageMode)
		applicable := make([]domain.DirectoryEvidence, 0, len(current))
		for _, evidence := range current {
			if evidence.HasTrustedEligibilityProvenanceAtSequence(bundle.Snapshot.Sequence) && directoryEvidenceApplies(evidence, selection, target.Client, environment) {
				applicable = append(applicable, evidence)
			}
		}
		entry := domain.CatalogCompatibility{Package: packageMode, Verification: directoryVerification(applicable), Authentication: target.Authentication, Evidence: applicable, EvidenceOutcomes: directoryEvidenceOutcomes(applicable)}
		if target.AppBinding != nil {
			server, ok := loaded.envelope.MCP.Servers[target.AppBinding.MCPServer]
			if !ok || !loaded.envelope.MCP.Enabled {
				return fmt.Errorf("signed Directory ChatGPT app binding references missing MCP server %q", target.AppBinding.MCPServer)
			}
			mcpURL, ok := server.Decoded["url"].(string)
			if !ok {
				return fmt.Errorf("signed Directory ChatGPT app binding MCP server %q has no remote URL", target.AppBinding.MCPServer)
			}
			binding := domain.CatalogAppBinding{AppKey: target.AppBinding.AppKey, ID: target.AppBinding.ID, MCPServer: target.AppBinding.MCPServer, MCPURL: mcpURL}
			if err := catalog.ValidateAppBindingReference(binding); err != nil {
				return fmt.Errorf("signed Directory ChatGPT app binding is invalid: %w", err)
			}
			entry.AppBinding = &binding
		}
		compatibility[string(target.Client)] = entry
	}
	loaded.hints.Compatibility = cloneCatalogCompatibility(compatibility)
	loaded.envelope.CatalogEvidence = &domain.CatalogEvidence{
		SchemaVersion: 1, CatalogVersion: "directory-snapshot-1", Digest: bundle.Digest,
		Revision: bundle.Snapshot.SourceCommit, MinimumCLIVersion: policy.MinimumInstallerVersion,
		AgentPluginsSchema: loaded.envelope.SchemaURI, Compatibility: cloneCatalogCompatibility(compatibility),
		CurrentEvidence: append([]domain.DirectoryEvidence(nil), current...),
	}
	return nil
}

func currentDirectoryEvidence(sequence uint64, history []domain.DirectoryEvidence, ids []string) []domain.DirectoryEvidence {
	byID := make(map[string]domain.DirectoryEvidence, len(history))
	for _, evidence := range history {
		byID[evidence.ID] = evidence
	}
	current := make([]domain.DirectoryEvidence, 0, len(ids))
	for _, id := range ids {
		if evidence, ok := byID[id]; ok && evidence.HasTrustedProvenanceAtSequence(sequence) {
			current = append(current, evidence)
		}
	}
	return current
}

func directoryEvidenceApplies(evidence domain.DirectoryEvidence, selection domain.DirectorySelection, client domain.ClientID, environment directoryEvidenceEnvironment) bool {
	if evidence.DistributionID != selection.DistributionID || evidence.ReleaseSequence != selection.ReleaseSequence || evidence.PackageTreeDigest != selection.TreeDigest {
		return false
	}
	if evidence.Level == "schema" {
		return evidence.Client == ""
	}
	if evidence.Client != client || !directoryEvidenceDimensionMatches(evidence.InstallerVersion, environment.InstallerVersion) ||
		!directoryEvidenceDimensionMatches(evidence.OS, environment.OS) || !directoryEvidenceDimensionMatches(evidence.Architecture, environment.Architecture) {
		return false
	}
	if evidence.ClientVersion != "" && environment.ClientVersions[client] != evidence.ClientVersion {
		return false
	}
	if evidence.DependencyIdentity != "" && environment.DependencyIdentity[client] != evidence.DependencyIdentity {
		return false
	}
	return true
}

func directoryEvidenceDimensionMatches(recorded, actual string) bool {
	return recorded == "" || recorded == actual
}

func directoryVerification(evidence []domain.DirectoryEvidence) string {
	schemaPassed := false
	for _, record := range evidence {
		if record.Outcome != "passed" {
			continue
		}
		if record.Level == "schema" {
			schemaPassed = true
			continue
		}
		if record.Level == "runtime" {
			return "tested"
		}
	}
	if schemaPassed {
		return "schema_only"
	}
	return "not_tested"
}

func directoryEvidenceOutcomes(evidence []domain.DirectoryEvidence) map[string]string {
	outcomes := map[string]string{"schema": "not_tested", "materialization": "not_tested", "discovery": "not_tested", "runtime": "not_tested", "oauth": "not_tested"}
	for _, record := range evidence {
		outcomes[record.Level] = record.Outcome
	}
	return outcomes
}

func directoryRecords(snapshot domain.DirectorySnapshot, selection domain.DirectorySelection) (*domain.DirectoryProduct, *domain.DirectoryDistribution, *domain.DirectoryRelease, *domain.DirectoryReleasePolicy) {
	var product *domain.DirectoryProduct
	for index := range snapshot.Products {
		if snapshot.Products[index].ID == selection.ProductID {
			product = &snapshot.Products[index]
			break
		}
	}
	var distribution *domain.DirectoryDistribution
	for index := range snapshot.Distributions {
		if snapshot.Distributions[index].ID == selection.DistributionID {
			distribution = &snapshot.Distributions[index]
			break
		}
	}
	if distribution == nil {
		return product, nil, nil, nil
	}
	var release *domain.DirectoryRelease
	var policy *domain.DirectoryReleasePolicy
	for index := range distribution.Releases {
		if distribution.Releases[index].Sequence == selection.ReleaseSequence {
			release = &distribution.Releases[index]
		}
	}
	for index := range distribution.ReleasePolicies {
		if distribution.ReleasePolicies[index].ReleaseSequence == selection.ReleaseSequence {
			policy = &distribution.ReleasePolicies[index]
		}
	}
	return product, distribution, release, policy
}

func snapshotRevokes(snapshot domain.DirectorySnapshot, selection domain.DirectorySelection) bool {
	for _, revocation := range snapshot.Revocations {
		if revocation.DistributionID == selection.DistributionID && revocation.ReleaseSequence == selection.ReleaseSequence {
			return true
		}
	}
	return false
}

func installedDirectoryFloor(state domain.StateFileV2) uint64 {
	var floor uint64
	for _, installation := range state.Installations {
		if installation.Directory != nil && installation.Directory.SnapshotSchema == directoryv1.SnapshotSchemaVersion && installation.Directory.SnapshotSequence > floor {
			floor = installation.Directory.SnapshotSequence
		}
	}
	return floor
}

func cloneDirectoryOrigin(value *domain.DirectoryOrigin) *domain.DirectoryOrigin {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneDirectorySelection(value *domain.DirectorySelection) *domain.DirectorySelection {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Diagnostics = append([]domain.DirectoryDiagnostic(nil), value.Diagnostics...)
	return &cloned
}

func cloneCatalogCompatibility(source map[string]domain.CatalogCompatibility) map[string]domain.CatalogCompatibility {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]domain.CatalogCompatibility, len(source))
	for client, compatibility := range source {
		compatibility.Evidence = append([]domain.DirectoryEvidence(nil), compatibility.Evidence...)
		if compatibility.EvidenceOutcomes != nil {
			compatibility.EvidenceOutcomes = make(map[string]string, len(compatibility.EvidenceOutcomes))
			for level, outcome := range source[client].EvidenceOutcomes {
				compatibility.EvidenceOutcomes[level] = outcome
			}
		}
		if compatibility.AppBinding != nil {
			binding := *compatibility.AppBinding
			compatibility.AppBinding = &binding
		}
		result[client] = compatibility
	}
	return result
}
