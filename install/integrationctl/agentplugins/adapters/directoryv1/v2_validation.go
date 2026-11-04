package directoryv1

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func requireV2SnapshotFields(body []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	if err := requiredMap(root, "snapshot_schema_version", "sequence", "publication_id",
		"source_commit", "generated_at", "expires_at", "products", "distributions",
		"evidence", "revocations"); err != nil {
		return err
	}
	if err := requireV2DistributionFields(root["distributions"]); err != nil {
		return err
	}
	return requireV2EvidenceFields(root["evidence"])
}

func requireV2DistributionFields(raw json.RawMessage) error {
	var distributions []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &distributions); err != nil {
		return err
	}
	for _, distribution := range distributions {
		if err := requiredMap(distribution, "schema_version", "id", "product_id",
			"kind", "status", "packager", "releases", "release_policies"); err != nil {
			return err
		}
		if err := requireV2ReleasePackageVersion(distribution["releases"]); err != nil {
			return err
		}
		if err := requireV2PolicyFields(distribution["release_policies"]); err != nil {
			return err
		}
	}
	return nil
}

func requireV2PolicyFields(raw json.RawMessage) error {
	var policies []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &policies); err != nil {
		return err
	}
	for _, policy := range policies {
		if err := requiredMap(policy, "release_sequence", "status", "minimum_installer_version",
			"targets", "current_evidence"); err != nil {
			return err
		}
		var targets []map[string]json.RawMessage
		if err := json.Unmarshal(policy["targets"], &targets); err != nil {
			return err
		}
		for _, target := range targets {
			if err := requireV2GrantFields(target); err != nil {
				return err
			}
		}
	}
	return nil
}

func requireV2GrantFields(target map[string]json.RawMessage) error {
	if _, old := target["scopes"]; old {
		return fmt.Errorf("v2 grant cannot contain legacy scopes")
	}
	if err := requiredMap(target, "client", "scope", "delivery", "authentication",
		"minimum_installer_version", "current_evidence"); err != nil {
		return err
	}
	raw, ok := target["canonical_closure"]
	if !ok {
		return nil
	}
	var closure map[string]json.RawMessage
	if err := json.Unmarshal(raw, &closure); err != nil {
		return err
	}
	if err := requiredMap(closure, "eligible"); err != nil {
		return err
	}
	var eligible bool
	if err := json.Unmarshal(closure["eligible"], &eligible); err != nil {
		return err
	}
	if eligible {
		return requiredMap(closure, "closure_digest", "minimum_installer_version")
	}
	return nil
}

func requireV2EvidenceFields(raw json.RawMessage) error {
	var evidence []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return err
	}
	for _, item := range evidence {
		if err := requiredMap(item, "schema_version", "id", "distribution_id",
			"release_sequence", "package_tree_digest", "level", "outcome", "artifact", "trust"); err != nil {
			return err
		}
	}
	return nil
}

// requireV2ReleasePackageVersion preserves the immutable release wire contract.
// A zero-value Go string otherwise hides a missing JSON field.
func requireV2ReleasePackageVersion(raw json.RawMessage) error {
	var releases []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &releases); err != nil {
		return err
	}
	for _, release := range releases {
		if err := requiredMap(release, "package_version"); err != nil {
			return err
		}
	}
	return nil
}

func validateV2Snapshot(snapshot domain.DirectorySnapshot) error {
	if snapshot.SnapshotSchemaVersion != 2 || snapshot.Sequence < 1 ||
		!publicationPattern.MatchString(snapshot.PublicationID) || !shaPattern.MatchString(snapshot.SourceCommit) {
		return strictError("invalid v2 snapshot identity")
	}
	start, err := parseTimestamp(snapshot.GeneratedAt)
	if err != nil {
		return strictError("invalid v2 generated_at")
	}
	expiry, err := parseTimestamp(snapshot.ExpiresAt)
	if err != nil || !expiry.After(start) || expiry.Sub(start) > 30*24*time.Hour {
		return strictError("invalid v2 snapshot expiry")
	}
	if snapshot.Products == nil || snapshot.Distributions == nil ||
		snapshot.Evidence == nil || snapshot.Revocations == nil {
		return strictError("v2 snapshot arrays cannot be null")
	}
	products, err := validateV2Products(snapshot.Products)
	if err != nil {
		return err
	}
	releases, policies, err := validateV2Distributions(snapshot, products, start)
	if err != nil {
		return err
	}
	if err := validateV2Evidence(snapshot, releases, policies, start); err != nil {
		return err
	}
	for _, revocation := range snapshot.Revocations {
		if !distributionPattern.MatchString(revocation.DistributionID) ||
			releases[releaseKey(revocation.DistributionID, revocation.ReleaseSequence)].Sequence == 0 {
			return strictError("v2 revocation references missing release")
		}
	}
	return nil
}

func validateV2Products(values []domain.DirectoryProduct) (map[string]domain.DirectoryProduct, error) {
	products := make(map[string]domain.DirectoryProduct, len(values))
	for _, product := range values {
		if product.SchemaVersion != 1 || !simpleIDPattern.MatchString(product.ID) ||
			!simpleIDPattern.MatchString(product.ManifestName) || product.DisplayName == "" ||
			product.Description == "" || products[product.ID].ID != "" {
			return nil, strictError("invalid or duplicate v2 product")
		}
		if len(product.Distributions) == 0 ||
			!oneOf(product.MinimumCapabilities.Skills, "required", "optional") ||
			!oneOf(product.MinimumCapabilities.MCP, "required", "optional") ||
			product.MinimumCapabilities.Skills == "optional" && product.MinimumCapabilities.MCP == "optional" {
			return nil, strictError("invalid v2 minimum product capabilities")
		}
		if err := uniqueSimpleIDs(product.Aliases, product.ID+" aliases"); err != nil {
			return nil, err
		}
		if err := uniqueDistributionIDs(product.Distributions, product.ID+" distributions"); err != nil {
			return nil, err
		}
		if !contains(product.Distributions, product.DefaultDistribution) {
			return nil, strictError("v2 default distribution is missing")
		}
		products[product.ID] = product
	}
	return products, nil
}

func validateV2Distributions(snapshot domain.DirectorySnapshot, products map[string]domain.DirectoryProduct, start time.Time) (
	map[string]domain.DirectoryRelease, map[string]domain.DirectoryReleasePolicy, error,
) {
	releases := map[string]domain.DirectoryRelease{}
	policies := map[string]domain.DirectoryReleasePolicy{}
	seen := map[string]bool{}
	for _, distribution := range snapshot.Distributions {
		product, found := products[distribution.ProductID]
		if distribution.SchemaVersion != 2 || !found || !contains(product.Distributions, distribution.ID) ||
			!distributionPattern.MatchString(distribution.ID) || seen[distribution.ID] ||
			(distribution.Status != domain.DistributionActive && distribution.Status != domain.DistributionSuspended) {
			return nil, nil, strictError("invalid v2 distribution")
		}
		seen[distribution.ID] = true
		if len(distribution.Releases) == 0 || len(distribution.Releases) != len(distribution.ReleasePolicies) {
			return nil, nil, strictError("v2 release policies mismatch")
		}
		if err := validateV2DistributionReleases(distribution, product, start, releases); err != nil {
			return nil, nil, err
		}
		if err := validateV2DistributionPolicies(distribution, releases, policies); err != nil {
			return nil, nil, err
		}
	}
	for _, product := range products {
		for _, id := range product.Distributions {
			if !seen[id] {
				return nil, nil, strictError("v2 product references missing distribution")
			}
		}
	}
	return releases, policies, nil
}

func validateV2DistributionReleases(distribution domain.DirectoryDistribution, product domain.DirectoryProduct,
	start time.Time, releases map[string]domain.DirectoryRelease,
) error {
	for _, release := range distribution.Releases {
		key := releaseKey(distribution.ID, release.Sequence)
		date, err := parseTimestamp(release.PublishedAt)
		if err != nil || date.After(start) || release.Sequence < 1 || releases[key].Sequence != 0 ||
			release.ManifestName != product.ManifestName || release.AgentPluginsSchema != supportedPluginSchema ||
			release.TreeDigestAlgorithm != domain.TreeDigestAlgorithm ||
			!digestPattern.MatchString(release.TreeDigest) || !digestPattern.MatchString(release.ManifestDigest) {
			return strictError("invalid v2 immutable release")
		}
		if err := validateSource(release.PackageSource); err != nil {
			return strictError("invalid v2 immutable source: %v", err)
		}
		if len(release.Components) == 0 {
			return strictError("v2 components required")
		}
		releases[key] = release
	}
	return nil
}

func validateV2DistributionPolicies(distribution domain.DirectoryDistribution,
	releases map[string]domain.DirectoryRelease, policies map[string]domain.DirectoryReleasePolicy,
) error {
	for _, policy := range distribution.ReleasePolicies {
		key := releaseKey(distribution.ID, policy.ReleaseSequence)
		if releases[key].Sequence == 0 || policies[key].ReleaseSequence != 0 {
			return strictError("invalid or duplicate v2 release policy")
		}
		if err := validateV2Policy(policy); err != nil {
			return err
		}
		policies[key] = policy
	}
	return nil
}

func validateV2Policy(policy domain.DirectoryReleasePolicy) error {
	if !oneOf(string(policy.Status), string(domain.ReleaseActive), string(domain.ReleaseSuperseded), string(domain.ReleaseRevoked)) {
		return strictError("invalid v2 release status")
	}
	if _, err := parseSemver(policy.MinimumInstallerVersion); err != nil ||
		policy.Targets == nil || len(policy.Targets) == 0 || policy.CurrentEvidence == nil {
		return strictError("invalid v2 policy floor or target arrays")
	}
	seen := map[string]bool{}
	for _, grant := range policy.Targets {
		key := string(grant.Client) + "\x00" + string(grant.Scope)
		if seen[key] {
			return strictError("invalid or duplicate v2 scoped grant")
		}
		seen[key] = true
		if err := validateV2Grant(grant, policy.MinimumInstallerVersion); err != nil {
			return err
		}
	}
	return validateV2EvidenceIDs(policy.CurrentEvidence)
}

func validateV2Grant(grant domain.DirectoryTarget, baseVersion string) error {
	baseFloor, _ := parseSemver(baseVersion)
	floor, err := parseSemver(grant.MinimumInstallerVersion)
	if err != nil || slices.Compare(floor[:], baseFloor[:]) < 0 || len(grant.Scopes) > 0 ||
		!oneOf(string(grant.Scope), "user", "project") {
		return strictError("invalid or duplicate v2 scoped grant")
	}
	delivery, ok := domain.ExpectedDirectoryDelivery(grant.Client)
	definition, defined := domain.ClientDefinitionFor(grant.Client)
	if !ok || !defined || delivery != grant.Delivery ||
		!slices.Contains(definition.Capabilities.Scopes, grant.Scope) || grant.CurrentEvidence == nil {
		return strictError("invalid v2 grant delivery or evidence")
	}
	if !oneOf(string(grant.Authentication), string(domain.AuthenticationRequirementNotRequired),
		string(domain.AuthenticationRequirementRequired), string(domain.AuthenticationRequirementUnknown)) {
		return strictError("invalid v2 target authentication")
	}
	if err := validateV2GrantBinding(grant); err != nil {
		return err
	}
	if err := validateV2EvidenceIDs(grant.CurrentEvidence); err != nil {
		return err
	}
	if err := validateV2GrantClosure(grant, floor[:]); err != nil {
		return err
	}
	return nil
}

func validateV2GrantBinding(grant domain.DirectoryTarget) error {
	definition, _ := domain.ClientDefinitionFor(grant.Client)
	if definition.Capabilities.AppSupport != domain.SupportUnsupported {
		if grant.AppBinding == nil || grant.AppBinding.AppKey == "" ||
			grant.AppBinding.ID == "" || grant.AppBinding.MCPServer == "" {
			return strictError("v2 app-capable client requires app binding")
		}
		if err := domain.ValidateAppBindingIdentity(grant.AppBinding.AppKey,
			grant.AppBinding.ID, grant.AppBinding.MCPServer); err != nil {
			return strictError("invalid v2 app binding: %v", err)
		}
	} else if grant.AppBinding != nil {
		return strictError("v2 app binding is not allowed for this client")
	}
	return nil
}

func validateV2GrantClosure(grant domain.DirectoryTarget, floor []uint64) error {
	if grant.CanonicalClosure == nil || !grant.CanonicalClosure.Eligible {
		return nil
	}
	closure := grant.CanonicalClosure
	if !digestPattern.MatchString(closure.ClosureDigest) {
		return strictError("invalid v2 closure digest")
	}
	minimum, err := parseSemver(closure.MinimumInstallerVersion)
	if err != nil || slices.Compare(minimum[:], floor) < 0 {
		return strictError("invalid v2 closure minimum")
	}
	return nil
}

// A signed policy may share one schema receipt across compatible targets,
// but it cannot repeat the same pointer within one applicability list.
func validateV2EvidenceIDs(ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !evidenceIDPattern.MatchString(id) || seen[id] {
			return strictError("invalid or duplicate v2 evidence pointer")
		}
		seen[id] = true
	}
	return nil
}

func validateV2Evidence(snapshot domain.DirectorySnapshot,
	releases map[string]domain.DirectoryRelease, policies map[string]domain.DirectoryReleasePolicy, generated time.Time,
) error {
	byID := map[string]domain.DirectoryEvidence{}
	for _, item := range snapshot.Evidence {
		if err := validateV2EvidenceRecord(item, releases, generated, byID); err != nil {
			return err
		}
		byID[item.ID] = item
	}
	return validateV2EvidenceSelections(policies, byID)
}

func validateV2EvidenceRecord(evidence domain.DirectoryEvidence,
	releases map[string]domain.DirectoryRelease, generated time.Time, byID map[string]domain.DirectoryEvidence,
) error {
	release := releases[releaseKey(evidence.DistributionID, evidence.ReleaseSequence)]
	if evidence.SchemaVersion != 1 || !evidenceIDPattern.MatchString(evidence.ID) ||
		byID[evidence.ID].ID != "" || release.Sequence == 0 ||
		release.TreeDigest != evidence.PackageTreeDigest || !evidence.HasTrustedProvenance() {
		return strictError("invalid v2 evidence identity or trust")
	}
	if !oneOf(evidence.Level, "schema", "materialization", "discovery", "runtime", "oauth") ||
		!oneOf(evidence.Outcome, "passed", "failed", "inconclusive", "not_tested", "not_applicable") {
		return strictError("invalid v2 evidence level or outcome")
	}
	if evidence.Level == "schema" {
		if evidence.Client != "" || evidence.Scope != "" {
			return strictError("v2 schema evidence cannot name client or scope")
		}
	} else if !validClient(evidence.Client) ||
		!oneOf(string(evidence.Scope), "", "user", "project") {
		return strictError("invalid v2 evidence client scope")
	}
	if !digestPattern.MatchString(evidence.Artifact.Digest) ||
		!repositoryPattern.MatchString(evidence.Artifact.Repository) ||
		!shaPattern.MatchString(evidence.Artifact.Revision) {
		return strictError("invalid v2 evidence artifact")
	}
	if evidence.ObservedAt != "" {
		date, err := parseTimestamp(evidence.ObservedAt)
		if err != nil || date.After(generated) {
			return strictError("invalid v2 evidence observation time")
		}
	}
	return nil
}

func validateV2EvidenceSelections(policies map[string]domain.DirectoryReleasePolicy,
	byID map[string]domain.DirectoryEvidence,
) error {
	selected := map[string]bool{}
	for key, policy := range policies {
		ids := append([]string(nil), policy.CurrentEvidence...)
		for _, grant := range policy.Targets {
			ids = append(ids, grant.CurrentEvidence...)
		}
		for _, id := range ids {
			evidence, ok := byID[id]
			if !ok || releaseKey(evidence.DistributionID, evidence.ReleaseSequence) != key {
				return strictError("v2 evidence pointer refers to another release")
			}
			selected[id] = true
		}
	}
	if len(selected) != len(byID) {
		return strictError("v2 evidence is not selected by a release")
	}
	return nil
}

