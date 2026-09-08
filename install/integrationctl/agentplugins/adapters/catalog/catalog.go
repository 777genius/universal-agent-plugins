package catalog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"golang.org/x/mod/semver"
)

const (
	SchemaVersionV1 = 1
	SchemaVersionV2 = 2
	minimumV2CLI    = "v0.1.6"
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	commitPattern     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	namePattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)
)

var requiredCompatibility = legacyRequiredCompatibility()

func legacyRequiredCompatibility() map[string]string {
	result := map[string]string{}
	for _, definition := range domain.ClientDefinitions() {
		if definition.LegacyCatalogRequired {
			result[string(definition.ID)] = definition.CatalogPackage
		}
	}
	return result
}

type Loader struct {
	CurrentCLIVersion string
}

type Loaded struct {
	Catalog domain.CatalogV1
	Digest  string
	byName  map[string]domain.CatalogPlugin
	cli     string
}

func (loader Loader) Load(body []byte, expectedDigest string) (Loaded, error) {
	digest := sha256Digest(body)
	if expected := strings.TrimSpace(expectedDigest); expected != "" && digest != expected {
		return Loaded{}, fmt.Errorf("catalog checksum mismatch")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var value domain.CatalogV1
	if err := decoder.Decode(&value); err != nil {
		return Loaded{}, fmt.Errorf("decode catalog: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Loaded{}, fmt.Errorf("catalog contains trailing JSON values")
	}
	if err := validateCatalog(value); err != nil {
		return Loaded{}, err
	}
	cliVersion := normalizeVersion(loader.CurrentCLIVersion)
	if value.SchemaVersion == SchemaVersionV2 && cliVersion != "" && semver.Compare(cliVersion, minimumV2CLI) < 0 {
		return Loaded{}, fmt.Errorf("catalog v2 requires agentplugins 0.1.6 or newer")
	}
	byName := make(map[string]domain.CatalogPlugin, len(value.Plugins))
	for _, plugin := range value.Plugins {
		byName[plugin.Name] = plugin
	}
	return Loaded{Catalog: value, Digest: digest, byName: byName, cli: cliVersion}, nil
}

func (loaded Loaded) Resolve(name string) (domain.CatalogResolution, error) {
	name = strings.TrimSpace(name)
	entry, ok := loaded.byName[name]
	if !ok {
		return domain.CatalogResolution{}, fmt.Errorf("plugin %q is not present in catalog %s", name, loaded.Catalog.CatalogVersion)
	}
	if loaded.cli != "" && semver.Compare(loaded.cli, normalizeVersion(entry.MinimumCLIVersion)) < 0 {
		return domain.CatalogResolution{}, fmt.Errorf("plugin %q requires agentplugins %s or newer", name, entry.MinimumCLIVersion)
	}
	return domain.CatalogResolution{
		Entry:           entry,
		SourceReference: fmt.Sprintf("%s@%s//%s", loaded.Catalog.Repository, loaded.Catalog.Revision, entry.SourcePath),
		CatalogDigest:   loaded.Digest,
		Hints: domain.CompatibilityHints{
			Compatibility: cloneCompatibility(entry.Compatibility),
			OpenAIMCPAuth: cloneAuthHints(entry.OpenAIMCPAuth),
		},
		Evidence: domain.CatalogEvidence{
			SchemaVersion:      loaded.Catalog.SchemaVersion,
			CatalogVersion:     loaded.Catalog.CatalogVersion,
			Repository:         loaded.Catalog.Repository,
			Revision:           loaded.Catalog.Revision,
			Digest:             loaded.Digest,
			MinimumCLIVersion:  entry.MinimumCLIVersion,
			AgentPluginsSchema: entry.AgentPluginsSchema,
			Compatibility:      cloneCompatibility(entry.Compatibility),
		},
	}, nil
}

func validateCatalog(value domain.CatalogV1) error {
	supportedSchema := (value.SchemaVersion == SchemaVersionV1 && value.Schema == domain.CatalogSchemaV1) ||
		(value.SchemaVersion == SchemaVersionV2 && value.Schema == domain.CatalogSchemaV2)
	if !supportedSchema {
		return fmt.Errorf("unsupported catalog schema")
	}
	if !semver.IsValid(normalizeVersion(value.CatalogVersion)) {
		return fmt.Errorf("catalog_version must be semantic versioning")
	}
	if !repositoryPattern.MatchString(value.Repository) || !commitPattern.MatchString(value.Revision) {
		return fmt.Errorf("catalog repository and revision must be exact GitHub coordinates")
	}
	if _, err := time.Parse(time.RFC3339, value.PublishedAt); err != nil {
		return fmt.Errorf("catalog published_at must be RFC3339: %w", err)
	}
	if len(value.Plugins) == 0 {
		return fmt.Errorf("catalog must contain at least one plugin")
	}
	seen := map[string]domain.CatalogPlugin{}
	for index, plugin := range value.Plugins {
		if err := validatePlugin(plugin, value.SchemaVersion); err != nil {
			return fmt.Errorf("plugins[%d]: %w", index, err)
		}
		if previous, exists := seen[plugin.Name]; exists {
			if previous.Version == plugin.Version && previous.TreeDigest != plugin.TreeDigest {
				return fmt.Errorf("supply-chain conflict for %s@%s", plugin.Name, plugin.Version)
			}
			return fmt.Errorf("duplicate catalog plugin %q", plugin.Name)
		}
		seen[plugin.Name] = plugin
	}
	return nil
}

func validatePlugin(plugin domain.CatalogPlugin, schemaVersion int) error {
	if !namePattern.MatchString(plugin.Name) || strings.Contains(plugin.Name, "..") || strings.HasSuffix(plugin.Name, ".") {
		return fmt.Errorf("invalid plugin name %q", plugin.Name)
	}
	if !semver.IsValid(normalizeVersion(plugin.Version)) || !semver.IsValid(normalizeVersion(plugin.MinimumCLIVersion)) {
		return fmt.Errorf("plugin %q has invalid version metadata", plugin.Name)
	}
	if schemaVersion == SchemaVersionV2 && semver.Compare(normalizeVersion(plugin.MinimumCLIVersion), minimumV2CLI) < 0 {
		return fmt.Errorf("plugin %q catalog v2 minimum_cli_version must be 0.1.6 or newer", plugin.Name)
	}
	if plugin.AgentPluginsSchema != domain.PluginSchemaV1 {
		return fmt.Errorf("plugin %q uses unsupported Agent Plugins schema", plugin.Name)
	}
	if err := validateSourcePath(plugin.SourcePath); err != nil {
		return fmt.Errorf("plugin %q source path: %w", plugin.Name, err)
	}
	if !digestPattern.MatchString(plugin.TreeDigest) || !digestPattern.MatchString(plugin.ManifestDigest) {
		return fmt.Errorf("plugin %q requires exact SHA-256 digests", plugin.Name)
	}
	components := map[string]struct{}{}
	for _, component := range plugin.Components {
		if component != "skills" && component != "mcp" && component != "extensions" {
			return fmt.Errorf("plugin %q has unknown component %q", plugin.Name, component)
		}
		if _, duplicate := components[component]; duplicate {
			return fmt.Errorf("plugin %q repeats component %q", plugin.Name, component)
		}
		components[component] = struct{}{}
	}
	allowChatGPT := schemaVersion == SchemaVersionV2
	if (!allowChatGPT && len(plugin.Compatibility) != len(requiredCompatibility)) ||
		(allowChatGPT && (len(plugin.Compatibility) < len(requiredCompatibility) || len(plugin.Compatibility) > len(requiredCompatibility)+1)) {
		return fmt.Errorf("plugin %q compatibility has the wrong client set for catalog schema v%d", plugin.Name, schemaVersion)
	}
	for client := range plugin.Compatibility {
		if _, required := requiredCompatibility[client]; !required && (!allowChatGPT || client != string(domain.ClientChatGPT)) {
			return fmt.Errorf("plugin %q compatibility contains unsupported client %q", plugin.Name, client)
		}
	}
	var authentication domain.AuthenticationRequirement
	for client, compatibility := range plugin.Compatibility {
		definition, _ := domain.ClientDefinitionFor(domain.ClientID(client))
		if compatibility.Package != definition.CatalogPackage ||
			!validVerificationCompatibility(compatibility.Verification) || !validAuthCompatibility(compatibility.Authentication) {
			return fmt.Errorf("plugin %q has invalid compatibility for %q", plugin.Name, client)
		}
		if definition.ID != domain.ClientChatGPT && compatibility.AppBinding != nil {
			return fmt.Errorf("plugin %q app_binding is allowed only for chatgpt", plugin.Name)
		}
		if authentication == "" {
			authentication = compatibility.Authentication
		} else if compatibility.Authentication != authentication {
			return fmt.Errorf("plugin %q must use one consistent authentication requirement for every client", plugin.Name)
		}
	}
	for client := range requiredCompatibility {
		if _, ok := plugin.Compatibility[client]; !ok {
			return fmt.Errorf("plugin %q compatibility is missing %q", plugin.Name, client)
		}
	}
	if compatibility, ok := plugin.Compatibility[string(domain.ClientChatGPT)]; ok {
		_, hasMCP := components["mcp"]
		if hasMCP && compatibility.AppBinding == nil {
			return fmt.Errorf("plugin %q ChatGPT MCP compatibility requires app_binding", plugin.Name)
		}
		if compatibility.AppBinding != nil {
			if err := ValidateAppBinding(*compatibility.AppBinding); err != nil {
				return fmt.Errorf("plugin %q chatgpt app_binding: %w", plugin.Name, err)
			}
		}
	}
	for server, hint := range plugin.OpenAIMCPAuth {
		if strings.TrimSpace(server) == "" || (hint.OAuthResource == "" && hint.BearerTokenEnvVar == "") {
			return fmt.Errorf("plugin %q has invalid OpenAI auth hint", plugin.Name)
		}
	}
	return nil
}

func ValidateAppBinding(binding domain.CatalogAppBinding) error {
	if err := ValidateAppBindingReference(binding); err != nil {
		return err
	}
	if err := validateSourcePath(binding.RuntimeEvidence); err != nil {
		return fmt.Errorf("runtime_evidence: %w", err)
	}
	if !commitPattern.MatchString(binding.RuntimeEvidenceRevision) {
		return fmt.Errorf("runtime_evidence_revision must be an exact lowercase Git commit")
	}
	return nil
}

// ValidateAppBindingReference validates the common registered-app identity
// and URL contract without requiring legacy Catalog v2 runtime evidence.
func ValidateAppBindingReference(binding domain.CatalogAppBinding) error {
	if err := domain.ValidateAppBindingIdentity(binding.AppKey, binding.ID, binding.MCPServer); err != nil {
		return err
	}
	parsed, err := url.Parse(binding.MCPURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Hostname() == "" || strings.HasSuffix(parsed.Host, ":") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.String() != binding.MCPURL {
		return fmt.Errorf("mcp_url must be a normalized absolute HTTPS URL without userinfo, query, or fragment")
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("mcp_url port must be numeric and between 1 and 65535")
		}
	}
	return nil
}

func validateSourcePath(value string) error {
	if value == "" || !utf8.ValidString(value) || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") {
		return fmt.Errorf("path must be a portable relative path")
	}
	parts := strings.Split(value, "/")
	if len(parts) > 64 {
		return fmt.Errorf("path exceeds maximum depth")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") {
			return fmt.Errorf("path contains unsafe segment")
		}
	}
	if path.Clean(value) != value {
		return fmt.Errorf("path is not normalized")
	}
	return nil
}

func validVerificationCompatibility(value string) bool {
	return oneOf(value, "tested", "schema_only", "not_tested")
}

func validAuthCompatibility(value domain.AuthenticationRequirement) bool {
	return value == domain.AuthenticationRequirementNotRequired ||
		value == domain.AuthenticationRequirementRequired ||
		value == domain.AuthenticationRequirementUnknown
}

func cloneCompatibility(source map[string]domain.CatalogCompatibility) map[string]domain.CatalogCompatibility {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]domain.CatalogCompatibility, len(source))
	for key, value := range source {
		if value.AppBinding != nil {
			binding := *value.AppBinding
			value.AppBinding = &binding
		}
		result[key] = value
	}
	return result
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "development") {
		return ""
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return value
}

func sha256Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneAuthHints(source map[string]domain.OpenAIMCPAuthHint) map[string]domain.OpenAIMCPAuthHint {
	if len(source) == 0 {
		return nil
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]domain.OpenAIMCPAuthHint, len(source))
	for _, key := range keys {
		result[key] = source[key]
	}
	return result
}
