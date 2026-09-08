package directoryv1

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const supportedPluginSchema = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

func validatePointer(v Pointer) error {
	if v.PointerSchemaVersion != 1 || v.SnapshotSchemaVersion != 1 || v.Sequence < 1 {
		return strictError("unsupported pointer schema or sequence")
	}
	stem := fmt.Sprintf("%020d", v.Sequence)
	if err := exactArtifactPath(v.SnapshotPath, "snapshots/"+stem+".json"); err != nil {
		return err
	}
	if err := exactArtifactPath(v.EnvelopePath, "snapshots/"+stem+".envelope.json"); err != nil {
		return err
	}
	c := v.FetchContract
	if !c.HTTPSRequired || !c.SameOriginRedirectsOnly || c.ForwardCredentialsOnRedirect {
		return strictError("unsafe fetch contract")
	}
	if c.MaxRedirects < 0 || c.MaxRedirects > 2 || c.RetryAttempts < 1 || c.RetryAttempts > 3 {
		return strictError("fetch retry/redirect bounds")
	}
	if c.LatestMaxBytes < 1 || c.LatestMaxBytes > MaxLatestBytes || c.SnapshotMaxBytes < 1 || c.SnapshotMaxBytes > MaxSnapshotBytes || c.EnvelopeMaxBytes < 1 || c.EnvelopeMaxBytes > MaxEnvelopeBytes {
		return strictError("fetch size bounds")
	}
	return nil
}

func exactArtifactPath(actual, expected string) error {
	if actual != expected || strings.ContainsAny(actual, `\:`) || strings.HasPrefix(actual, "/") || path.Clean(actual) != actual {
		return strictError("unsafe artifact path %q", actual)
	}
	for _, part := range strings.Split(actual, "/") {
		if part == "" || part == "." || part == ".." {
			return strictError("unsafe artifact path %q", actual)
		}
	}
	return nil
}

func validateEnvelope(v Envelope) error {
	if v.EnvelopeSchemaVersion != 1 || v.SnapshotSchemaVersion != 1 || v.Sequence < 1 {
		return strictError("invalid envelope schema or sequence")
	}
	if !keyIDPattern.MatchString(v.KeyID) {
		return strictError("malformed key ID")
	}
	if v.Algorithm != "Ed25519" || v.SignatureDomain != SignatureDomain {
		return strictError("unsupported signature contract")
	}
	if !digestPattern.MatchString(v.SnapshotDigest) {
		return strictError("malformed snapshot digest")
	}
	decoded, err := base64Strict(v.Signature)
	if err != nil || len(decoded) != 64 {
		return strictError("malformed signature")
	}
	return nil
}

func base64Strict(value string) ([]byte, error) {
	if len(value) != 88 || !strings.HasSuffix(value, "==") {
		return nil, fmt.Errorf("invalid encoded length")
	}
	return base64.StdEncoding.Strict().DecodeString(value)
}

func validateSnapshot(v domain.DirectorySnapshot) error {
	if v.SnapshotSchemaVersion != 1 || v.Sequence < 1 {
		return strictError("invalid snapshot schema or sequence")
	}
	if !publicationPattern.MatchString(v.PublicationID) || !shaPattern.MatchString(v.SourceCommit) {
		return strictError("malformed publication identity")
	}
	generated, err := parseTimestamp(v.GeneratedAt)
	if err != nil {
		return strictError("generated_at: %v", err)
	}
	expires, err := parseTimestamp(v.ExpiresAt)
	if err != nil {
		return strictError("expires_at: %v", err)
	}
	if !expires.After(generated) || expires.Sub(generated) > 30*24*time.Hour {
		return strictError("invalid snapshot lifetime")
	}
	if v.Products == nil || v.Distributions == nil || v.Evidence == nil || v.Revocations == nil {
		return strictError("snapshot arrays cannot be null")
	}

	products := map[string]domain.DirectoryProduct{}
	selectorOwners := map[string]string{}
	for i, p := range v.Products {
		where := fmt.Sprintf("products[%d]", i)
		if p.SchemaVersion != 1 || !simpleIDPattern.MatchString(p.ID) || !simpleIDPattern.MatchString(p.ManifestName) {
			return strictError("%s malformed identity", where)
		}
		if _, ok := products[p.ID]; ok {
			return strictError("duplicate product %q", p.ID)
		}
		products[p.ID] = p
		if utf8.RuneCountInString(p.DisplayName) < 1 || utf8.RuneCountInString(p.DisplayName) > 100 || utf8.RuneCountInString(p.Description) < 1 || utf8.RuneCountInString(p.Description) > 500 {
			return strictError("%s invalid display text", where)
		}
		if p.Aliases == nil || len(p.Aliases) < 1 || p.ReservedAliases == nil || len(p.ReservedAliases) < 1 || p.Categories == nil || len(p.Categories) < 1 || len(p.Categories) > 20 || p.Distributions == nil || len(p.Distributions) < 1 {
			return strictError("%s invalid required arrays", where)
		}
		if err := uniqueSimpleIDs(p.Aliases, where+" aliases"); err != nil {
			return err
		}
		if err := uniqueSimpleIDs(p.ReservedAliases, where+" reserved aliases"); err != nil {
			return err
		}
		if err := uniqueSimpleIDs(p.Categories, where+" categories"); err != nil {
			return err
		}
		if err := uniqueDistributionIDs(p.Distributions, where+" distributions"); err != nil {
			return err
		}
		// Product IDs, manifest identities, active aliases, and reserved aliases
		// all occupy the short-selector namespace. Repetition within one product
		// is intentional (the canonical alias is commonly also reserved), but a
		// different product may not take over any of them.
		selectors := append([]string{p.ID, p.ManifestName}, p.Aliases...)
		selectors = append(selectors, p.ReservedAliases...)
		for _, selector := range selectors {
			if owner, exists := selectorOwners[selector]; exists && owner != p.ID {
				return strictError("selector %q is owned by both %q and %q", selector, owner, p.ID)
			}
			selectorOwners[selector] = p.ID
		}
		if !contains(p.Distributions, p.DefaultDistribution) {
			return strictError("%s default distribution missing", where)
		}
		if !oneOf(p.MinimumCapabilities.Skills, "required", "optional") || !oneOf(p.MinimumCapabilities.MCP, "required", "optional") || (p.MinimumCapabilities.Skills == "optional" && p.MinimumCapabilities.MCP == "optional") {
			return strictError("%s invalid minimum capabilities", where)
		}
		if p.Icon != nil && (!iconPathPattern.MatchString(p.Icon.Path) || !digestPattern.MatchString(p.Icon.Digest)) {
			return strictError("%s invalid icon", where)
		}
	}

	distributions := map[string]domain.DirectoryDistribution{}
	releases := map[string]domain.DirectoryRelease{}
	policies := map[string]domain.DirectoryReleasePolicy{}
	for i, d := range v.Distributions {
		where := fmt.Sprintf("distributions[%d]", i)
		if d.SchemaVersion != 1 || !distributionPattern.MatchString(d.ID) || !simpleIDPattern.MatchString(d.ProductID) || !simpleIDPattern.MatchString(d.Packager) {
			return strictError("%s malformed identity", where)
		}
		if _, ok := distributions[d.ID]; ok {
			return strictError("duplicate distribution %q", d.ID)
		}
		distributions[d.ID] = d
		p, ok := products[d.ProductID]
		if !ok || !contains(p.Distributions, d.ID) {
			return strictError("%s product relationship mismatch", where)
		}
		if d.Kind != domain.DistributionUpstream && d.Kind != domain.DistributionCommunityBridge && d.Kind != domain.DistributionCommunity {
			return strictError("%s invalid kind", where)
		}
		if d.Status != domain.DistributionActive && d.Status != domain.DistributionSuspended {
			return strictError("%s invalid status", where)
		}
		if d.Releases == nil || len(d.Releases) < 1 || d.ReleasePolicies == nil || len(d.ReleasePolicies) < 1 {
			return strictError("%s releases and policies required", where)
		}
		seqs := map[uint64]bool{}
		for j, r := range d.Releases {
			identity := releaseKey(d.ID, r.Sequence)
			if r.Sequence < 1 || seqs[r.Sequence] || releases[identity].Sequence != 0 {
				return strictError("%s duplicate release sequence", where)
			}
			seqs[r.Sequence] = true
			// Directory releases use the exact framing implemented by the Go
			// packagedigest adapter. Publishers recompute release digests from the
			// source bytes; hashes produced under another framing cannot be accepted
			// by merely relabelling them.
			if !simpleIDPattern.MatchString(r.ManifestName) || r.AgentPluginsSchema != supportedPluginSchema || r.TreeDigestAlgorithm != domain.TreeDigestAlgorithm || !digestPattern.MatchString(r.TreeDigest) || !digestPattern.MatchString(r.ManifestDigest) {
				return strictError("%s releases[%d] malformed release", where, j)
			}
			if r.ManifestName != p.ManifestName {
				return strictError("%s releases[%d] manifest identity mismatch", where, j)
			}
			if err := validateSource(r.PackageSource); err != nil {
				return strictError("%s releases[%d]: %v", where, j, err)
			}
			if r.BuildProvenance != nil && (!repositoryPattern.MatchString(r.BuildProvenance.UpstreamRepository) || !shaPattern.MatchString(r.BuildProvenance.UpstreamRevision)) {
				return strictError("%s invalid build provenance", identity)
			}
			if r.Components == nil || len(r.Components) < 1 {
				return strictError("%s components required", identity)
			}
			seen := map[string]bool{}
			for _, c := range r.Components {
				if !oneOf(c, "extensions", "mcp", "skills") || seen[c] {
					return strictError("%s invalid component %q", identity, c)
				}
				seen[c] = true
			}
			published, e := parseTimestamp(r.PublishedAt)
			if e != nil || published.After(generated) {
				return strictError("%s invalid published_at", identity)
			}
			releases[identity] = r
		}
		for j, p := range d.ReleasePolicies {
			identity := releaseKey(d.ID, p.ReleaseSequence)
			if p.ReleaseSequence < 1 || policies[identity].ReleaseSequence != 0 {
				return strictError("%s duplicate policy", where)
			}
			if !seqs[p.ReleaseSequence] {
				return strictError("%s policy without release", identity)
			}
			if err := validatePolicy(p); err != nil {
				return strictError("%s release_policies[%d]: %v", where, j, err)
			}
			policies[identity] = p
		}
		if len(seqs) != len(d.ReleasePolicies) {
			return strictError("%s release policies do not match releases", where)
		}
	}
	for _, p := range v.Products {
		for _, id := range p.Distributions {
			d, ok := distributions[id]
			if !ok || d.ProductID != p.ID {
				return strictError("product %q references missing distribution %q", p.ID, id)
			}
		}
	}

	evidence := map[string]domain.DirectoryEvidence{}
	legacyEvidence := v.Sequence < domain.DirectoryEvidenceTrustCutoverSequence
	for i, e := range v.Evidence {
		where := fmt.Sprintf("evidence[%d]", i)
		if e.SchemaVersion != 1 || !evidenceIDPattern.MatchString(e.ID) || !distributionPattern.MatchString(e.DistributionID) || e.ReleaseSequence < 1 || !digestPattern.MatchString(e.PackageTreeDigest) {
			return strictError("%s malformed identity", where)
		}
		if _, ok := evidence[e.ID]; ok {
			return strictError("duplicate evidence %q", e.ID)
		}
		r, ok := releases[releaseKey(e.DistributionID, e.ReleaseSequence)]
		if !ok || r.TreeDigest != e.PackageTreeDigest {
			return strictError("%s release/digest mismatch", where)
		}
		distribution := distributions[e.DistributionID]
		if legacyEvidence {
			if e.Trust != nil || e.ProductID != distribution.ProductID || e.ManifestDigest != r.ManifestDigest ||
				e.SourceRepository != r.PackageSource.Repository || e.SourceRevision != r.PackageSource.Revision || e.SourcePath != r.PackageSource.Path {
				return strictError("%s legacy evidence source identity mismatch", where)
			}
			if e.AdapterVersion != "" && strings.TrimSpace(e.AdapterVersion) == "" {
				return strictError("%s invalid adapter version", where)
			}
		} else if e.ProductID != "" || e.ManifestDigest != "" || e.SourceRepository != "" || e.SourceRevision != "" || e.SourcePath != "" || e.AdapterVersion != "" {
			return strictError("%s legacy evidence fields after cutover", where)
		}
		if !oneOf(e.Level, "schema", "materialization", "discovery", "runtime", "oauth") || !oneOf(e.Outcome, "passed", "failed", "inconclusive", "not_tested", "not_applicable") {
			return strictError("%s invalid result", where)
		}
		if e.Level == "schema" {
			if e.Client != "" {
				return strictError("%s schema evidence cannot name client", where)
			}
		} else if !validClient(e.Client) || e.ClientVersion == "" || e.InstallerVersion == "" || e.OS == "" || e.Architecture == "" || e.ObservedAt == "" {
			return strictError("%s missing applicability dimensions", where)
		}
		if e.ObservedAt != "" {
			observed, x := parseTimestamp(e.ObservedAt)
			if x != nil || observed.After(generated) {
				return strictError("%s invalid observed_at", where)
			}
		}
		if !repositoryPattern.MatchString(e.Artifact.Repository) || !shaPattern.MatchString(e.Artifact.Revision) || e.Artifact.Path == "" || !digestPattern.MatchString(e.Artifact.Digest) {
			return strictError("%s invalid artifact", where)
		}
		if e.Trust != nil {
			if e.Trust.Kind == "github_actions" {
				if !workflowPattern.MatchString(e.Trust.Workflow) || !sourceRefPattern.MatchString(e.Trust.SourceRef) || !shaPattern.MatchString(e.Trust.SourceDigest) || !e.HasTrustedProvenance() {
					return strictError("%s invalid trust", where)
				}
				for _, artifact := range []*domain.DirectoryEvidenceArtifact{e.Trust.BundleManifest, e.Trust.LaunchArtifact, e.Trust.ObserverArtifact, e.Trust.EvidenceIndex} {
					if artifact != nil && (!repositoryPattern.MatchString(artifact.Repository) || !shaPattern.MatchString(artifact.Revision) || artifact.Path == "" || !digestPattern.MatchString(artifact.Digest)) {
						return strictError("%s invalid trust artifact", where)
					}
				}
			} else if e.Trust.Kind != "reviewed_external" || e.Trust.Workflow != "" || e.Trust.SourceRef != "" || e.Trust.SourceDigest != "" ||
				e.Trust.BundleManifest != nil || e.Trust.LaunchArtifact != nil || e.Trust.ObserverArtifact != nil || e.Trust.EvidenceIndex != nil {
				return strictError("%s invalid trust", where)
			}
		}
		if !e.HasTrustedProvenanceAtSequence(v.Sequence) {
			return strictError("%s lacks recognized trusted provenance", where)
		}
		evidence[e.ID] = e
	}
	selected := map[string]bool{}
	tuples := map[string]bool{}
	for identity, p := range policies {
		for _, id := range p.CurrentEvidence {
			if selected[id] {
				return strictError("evidence %q selected more than once", id)
			}
			e, ok := evidence[id]
			if !ok || releaseKey(e.DistributionID, e.ReleaseSequence) != identity {
				return strictError("policy references inapplicable evidence %q", id)
			}
			tuple := strings.Join([]string{e.DistributionID, fmt.Sprint(e.ReleaseSequence), e.PackageTreeDigest, e.Level, string(e.Client), e.ClientVersion, e.InstallerVersion, e.DependencyIdentity, e.OS, e.Architecture}, "\x00")
			if tuples[tuple] {
				return strictError("multiple current evidence records for applicability tuple")
			}
			tuples[tuple] = true
			selected[id] = true
		}
	}
	if len(selected) != len(evidence) {
		return strictError("snapshot evidence must exactly match current policy pointers")
	}
	revoked := map[string]bool{}
	for _, r := range v.Revocations {
		identity := releaseKey(r.DistributionID, r.ReleaseSequence)
		if !distributionPattern.MatchString(r.DistributionID) || r.ReleaseSequence < 1 || releases[identity].Sequence == 0 || revoked[identity] {
			return strictError("invalid or duplicate revocation %s", identity)
		}
		revoked[identity] = true
	}
	return nil
}

func validatePolicy(p domain.DirectoryReleasePolicy) error {
	if p.Status != domain.ReleaseActive && p.Status != domain.ReleaseSuperseded && p.Status != domain.ReleaseRevoked {
		return fmt.Errorf("invalid status")
	}
	if _, err := parseSemver(p.MinimumInstallerVersion); err != nil {
		return fmt.Errorf("invalid minimum installer version")
	}
	if p.Targets == nil || len(p.Targets) < 1 || p.CurrentEvidence == nil {
		return fmt.Errorf("targets/evidence arrays required")
	}
	clients := map[domain.ClientID]bool{}
	for _, t := range p.Targets {
		expectedDelivery, supportedDelivery := domain.ExpectedDirectoryDelivery(t.Client)
		if !validClient(t.Client) || clients[t.Client] || len(t.Scopes) != 1 || t.Scopes[0] != domain.ScopeUser || !supportedDelivery || t.Delivery != expectedDelivery {
			return fmt.Errorf("invalid target")
		}
		clients[t.Client] = true
		if t.Authentication != domain.AuthenticationRequirementNotRequired && t.Authentication != domain.AuthenticationRequirementRequired && t.Authentication != domain.AuthenticationRequirementUnknown {
			return fmt.Errorf("invalid target authentication")
		}
		if t.Client == domain.ClientChatGPT {
			if t.AppBinding == nil || t.AppBinding.AppKey == "" || t.AppBinding.ID == "" || t.AppBinding.MCPServer == "" {
				return fmt.Errorf("chatgpt app binding required")
			}
			if err := domain.ValidateAppBindingIdentity(t.AppBinding.AppKey, t.AppBinding.ID, t.AppBinding.MCPServer); err != nil {
				return fmt.Errorf("invalid chatgpt app binding: %w", err)
			}
		} else if t.AppBinding != nil {
			return fmt.Errorf("app binding only allowed for chatgpt")
		}
	}
	seen := map[string]bool{}
	for _, id := range p.CurrentEvidence {
		if !evidenceIDPattern.MatchString(id) || seen[id] {
			return fmt.Errorf("invalid current evidence ID")
		}
		seen[id] = true
	}
	return nil
}

func validateSource(s domain.DirectorySource) error {
	if !repositoryPattern.MatchString(s.Repository) || !shaPattern.MatchString(s.Revision) {
		return fmt.Errorf("malformed immutable source")
	}
	return validateSourcePath(s.Path)
}

func validateSourcePath(sourcePath string) error {
	if sourcePath == "" || strings.HasPrefix(sourcePath, "/") || strings.Contains(sourcePath, "\\") {
		return fmt.Errorf("unsafe source path")
	}
	for _, part := range strings.Split(sourcePath, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("unsafe source path")
		}
		for _, r := range part {
			if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r)) {
				return fmt.Errorf("unsafe source path")
			}
		}
	}
	return nil
}
func parseTimestamp(v string) (time.Time, error) {
	if !timestampPattern.MatchString(v) {
		return time.Time{}, fmt.Errorf("must be second-precision UTC RFC3339")
	}
	return time.Parse("2006-01-02T15:04:05Z", v)
}
func parseSemver(v string) ([3]uint64, error) {
	var out [3]uint64
	if !semverPattern.MatchString(v) {
		return out, fmt.Errorf("not x.y.z")
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("not x.y.z")
	}
	for i, s := range parts {
		if s == "" || (len(s) > 1 && s[0] == '0') {
			return out, fmt.Errorf("invalid component")
		}
		n, e := fmt.Sscan(s, &out[i])
		if e != nil || n != 1 {
			return out, fmt.Errorf("invalid component")
		}
	}
	return out, nil
}
func validClient(v domain.ClientID) bool {
	return domain.IsSupportedClient(v)
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
func releaseKey(distribution string, sequence uint64) string {
	return fmt.Sprintf("%s#%d", distribution, sequence)
}
func uniqueSimpleIDs(values []string, where string) error {
	seen := map[string]bool{}
	for _, v := range values {
		if !simpleIDPattern.MatchString(v) || seen[v] {
			return strictError("%s contains malformed or duplicate %q", where, v)
		}
		seen[v] = true
	}
	return nil
}
func uniqueDistributionIDs(values []string, where string) error {
	seen := map[string]bool{}
	for _, v := range values {
		if !distributionPattern.MatchString(v) || seen[v] {
			return strictError("%s contains malformed or duplicate %q", where, v)
		}
		seen[v] = true
	}
	return nil
}
func strictError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrStrictJSON, fmt.Sprintf(format, args...))
}

func requireRootFields(body []byte, fields ...string) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	return requiredMap(root, fields...)
}
func requireSnapshotFields(body []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	if err := requiredMap(root, "snapshot_schema_version", "sequence", "publication_id", "source_commit", "generated_at", "expires_at", "products", "distributions", "evidence", "revocations"); err != nil {
		return err
	}
	var sequence uint64
	if err := json.Unmarshal(root["sequence"], &sequence); err != nil {
		return err
	}
	legacyEvidence := sequence < domain.DirectoryEvidenceTrustCutoverSequence
	var products []map[string]json.RawMessage
	if err := json.Unmarshal(root["products"], &products); err != nil {
		return err
	}
	for _, p := range products {
		if err := requiredMap(p, "schema_version", "id", "display_name", "description", "manifest_name", "aliases", "reserved_aliases", "categories", "minimum_capabilities", "default_distribution", "distributions"); err != nil {
			return err
		}
		var minimum map[string]json.RawMessage
		if err := json.Unmarshal(p["minimum_capabilities"], &minimum); err != nil {
			return err
		}
		if err := requiredMap(minimum, "skills", "mcp"); err != nil {
			return err
		}
		if raw, ok := p["icon"]; ok {
			var icon map[string]json.RawMessage
			if err := json.Unmarshal(raw, &icon); err != nil {
				return err
			}
			if err := requiredMap(icon, "path", "digest"); err != nil {
				return err
			}
		}
	}
	var distributions []map[string]json.RawMessage
	if err := json.Unmarshal(root["distributions"], &distributions); err != nil {
		return err
	}
	for _, d := range distributions {
		if err := requiredMap(d, "schema_version", "id", "product_id", "kind", "status", "packager", "releases", "release_policies"); err != nil {
			return err
		}
		var releases []map[string]json.RawMessage
		if err := json.Unmarshal(d["releases"], &releases); err != nil {
			return err
		}
		for _, r := range releases {
			if err := requiredMap(r, "sequence", "package_version", "manifest_name", "agent_plugins_schema", "package_source", "tree_digest_algorithm", "tree_digest", "manifest_digest", "components", "published_at"); err != nil {
				return err
			}
			var source map[string]json.RawMessage
			if err := json.Unmarshal(r["package_source"], &source); err != nil {
				return err
			}
			if err := requiredMap(source, "repository", "revision", "path"); err != nil {
				return err
			}
			if raw, ok := r["build_provenance"]; ok {
				var provenance map[string]json.RawMessage
				if err := json.Unmarshal(raw, &provenance); err != nil {
					return err
				}
				if err := requiredMap(provenance, "upstream_repository", "upstream_revision"); err != nil {
					return err
				}
			}
		}
		var policies []map[string]json.RawMessage
		if err := json.Unmarshal(d["release_policies"], &policies); err != nil {
			return err
		}
		for _, p := range policies {
			if err := requiredMap(p, "release_sequence", "status", "minimum_installer_version", "targets", "current_evidence"); err != nil {
				return err
			}
			var targets []map[string]json.RawMessage
			if err := json.Unmarshal(p["targets"], &targets); err != nil {
				return err
			}
			for _, target := range targets {
				if err := requiredMap(target, "client", "scopes", "delivery", "authentication"); err != nil {
					return err
				}
				if raw, ok := target["app_binding"]; ok {
					var binding map[string]json.RawMessage
					if err := json.Unmarshal(raw, &binding); err != nil {
						return err
					}
					if err := requiredMap(binding, "app_key", "id", "mcp_server"); err != nil {
						return err
					}
				}
			}
		}
	}
	var evidence []map[string]json.RawMessage
	if err := json.Unmarshal(root["evidence"], &evidence); err != nil {
		return err
	}
	for _, e := range evidence {
		required := []string{"schema_version", "id", "distribution_id", "release_sequence", "package_tree_digest", "level", "outcome", "artifact"}
		if legacyEvidence {
			if _, present := e["trust"]; present {
				return fmt.Errorf("legacy evidence cannot contain trust")
			}
			required = append(required, "product_id", "manifest_digest", "source_repository", "source_revision", "source_path")
		} else {
			for _, field := range []string{"product_id", "manifest_digest", "source_repository", "source_revision", "source_path", "adapter_version"} {
				if _, present := e[field]; present {
					return fmt.Errorf("wire evidence cannot contain legacy field %q", field)
				}
			}
			required = append(required, "trust")
		}
		if err := requiredMap(e, required...); err != nil {
			return err
		}
		var artifact map[string]json.RawMessage
		if err := json.Unmarshal(e["artifact"], &artifact); err != nil {
			return err
		}
		if err := requiredMap(artifact, "repository", "revision", "path", "digest"); err != nil {
			return err
		}
		if raw, ok := e["trust"]; ok {
			var trust map[string]json.RawMessage
			if err := json.Unmarshal(raw, &trust); err != nil {
				return err
			}
			if err := requiredMap(trust, "kind"); err != nil {
				return err
			}
			for _, field := range []string{"bundle_manifest", "launch_artifact", "observer_artifact", "evidence_index"} {
				if artifactRaw, present := trust[field]; present {
					var trustArtifact map[string]json.RawMessage
					if err := json.Unmarshal(artifactRaw, &trustArtifact); err != nil {
						return err
					}
					if err := requiredMap(trustArtifact, "repository", "revision", "path", "digest"); err != nil {
						return err
					}
				}
			}
		}
	}
	var revocations []map[string]json.RawMessage
	if err := json.Unmarshal(root["revocations"], &revocations); err != nil {
		return err
	}
	for _, r := range revocations {
		if err := requiredMap(r, "distribution_id", "release_sequence"); err != nil {
			return err
		}
	}
	return nil
}
func requiredMap(value map[string]json.RawMessage, fields ...string) error {
	if value == nil {
		return fmt.Errorf("expected object")
	}
	for _, f := range fields {
		raw, ok := value[f]
		if !ok {
			return fmt.Errorf("missing required field %q", f)
		}
		if string(raw) == "null" {
			return fmt.Errorf("required field %q cannot be null", f)
		}
	}
	return nil
}
func isJSONNull(value json.RawMessage) bool { return strings.TrimSpace(string(value)) == "null" }
