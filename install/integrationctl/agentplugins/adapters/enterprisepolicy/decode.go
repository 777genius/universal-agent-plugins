// Package enterprisepolicy adapts local policy files to the installation gate.
// It owns decoding and origin normalization, never native-client effects.
package enterprisepolicy

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const MaxBytes int64 = 64 << 10

// ConfigError deliberately excludes raw YAML, source URLs and decoder details.
type ConfigError struct {
	Layer string `json:"layer,omitempty"`
	Path  string `json:"path,omitempty"`
	Code  string `json:"code"`
}

func (e *ConfigError) Error() string {
	return "installation policy configuration " + domain.PolicySafeText(e.Code) + " at " + domain.PolicySafeText(e.Path) + "; validate and correct the policy file"
}

func configError(code string) error { return &ConfigError{Code: code} }

// Decode accepts the legacy schema-1 rules and the layered schema-2 rules.
// Node validation precedes decoding so YAML cannot coerce numeric strings,
// nulls, aliases or duplicate keys into weaker rules.
func Decode(body []byte) (domain.InstallPolicy, error) {
	var policy domain.InstallPolicy
	if int64(len(body)) > MaxBytes {
		return policy, configError("file_too_large")
	}
	root, err := decodeDocument(body)
	if err != nil {
		return policy, err
	}
	version := mappingValue(root, "schema_version")
	if version == nil || version.Kind != yaml.ScalarNode || version.Tag != "!!int" {
		return policy, configError("schema_version_required")
	}
	policy.SchemaVersion, err = strconv.Atoi(version.Value)
	if err != nil || (policy.SchemaVersion != 1 && policy.SchemaVersion != 2) {
		return policy, configError("unsupported_schema_version")
	}
	if err := decodeRules(root, &policy); err != nil {
		return domain.InstallPolicy{}, err
	}
	if err := validateRules(&policy); err != nil {
		return domain.InstallPolicy{}, err
	}
	return policy, nil
}

func decodeDocument(body []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	var document, trailing yaml.Node
	if err := decoder.Decode(&document); err != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, configError("invalid_yaml_document")
	}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, configError("multiple_yaml_documents")
	}
	if err := validateSyntax(&document, 0); err != nil {
		return nil, err
	}
	return document.Content[0], nil
}

func validateSyntax(node *yaml.Node, depth int) error {
	if depth > 8 || node.Kind == yaml.AliasNode || node.Anchor != "" || node.Tag == "!!null" || node.Tag == "!!merge" {
		return configError("null_alias_merge_or_nested_value")
	}
	if (node.Kind == yaml.MappingNode && node.Tag != "!!map") || (node.Kind == yaml.SequenceNode && node.Tag != "!!seq") {
		return configError("unsupported_yaml_tag")
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return configError("duplicate_or_invalid_key")
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateSyntax(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func decodeRules(root *yaml.Node, p *domain.InstallPolicy) error {
	for i := 0; i < len(root.Content); i += 2 {
		section, node := root.Content[i].Value, root.Content[i+1]
		if section == "schema_version" {
			continue
		}
		if !slices.Contains([]string{"sources", "integrity", "mcp", "install", "components"}, section) || node.Kind != yaml.MappingNode {
			return configError("unknown_or_invalid_section")
		}
		if p.SchemaVersion == 1 && (section == "install" || section == "components") {
			return configError("rule_requires_schema_2")
		}
		for j := 0; j < len(node.Content); j += 2 {
			if err := decodeRule(section+"."+node.Content[j].Value, node.Content[j+1], p); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeRule(key string, node *yaml.Node, p *domain.InstallPolicy) error {
	legacy := []string{"sources.allow_hosts", "sources.deny_hosts", "integrity.require_pinned_git", "integrity.require_archive_sha256", "mcp.allowed_transports"}
	if p.SchemaVersion == 1 && !slices.Contains(legacy, key) {
		return configError("rule_requires_schema_2")
	}
	allows := map[string]*domain.PolicyAllowList{
		"sources.allow_hosts": &p.Sources.AllowHosts, "sources.allowed_kinds": &p.Sources.AllowedKinds,
		"sources.allow_repositories": &p.Sources.AllowRepositories, "mcp.allowed_transports": &p.MCP.AllowedTransports,
		"mcp.allow_hosts": &p.MCP.AllowHosts, "install.allowed_targets": &p.Install.AllowedTargets, "install.allowed_scopes": &p.Install.AllowedScopes,
	}
	if list, ok := allows[key]; ok {
		values, err := stringSequence(node)
		list.Present, list.Values = true, values
		return err
	}
	lists := map[string]*[]string{"sources.deny_hosts": &p.Sources.DenyHosts, "sources.deny_repositories": &p.Sources.DenyRepositories, "mcp.deny_hosts": &p.MCP.DenyHosts, "components.denied": &p.Components.Denied}
	if list, ok := lists[key]; ok {
		values, err := stringSequence(node)
		*list = values
		return err
	}
	flags := map[string]*bool{"integrity.require_pinned_git": &p.Integrity.RequirePinnedGit, "integrity.require_archive_sha256": &p.Integrity.RequireArchiveSHA256}
	if flag, ok := flags[key]; ok {
		value := strings.ToLower(node.Value)
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" || (value != "true" && value != "false") {
			return configError("invalid_boolean")
		}
		*flag = value == "true"
		return nil
	}
	return configError("unknown_rule")
}

func stringSequence(node *yaml.Node) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, configError("expected_string_list")
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return nil, configError("expected_string_value")
		}
		values = append(values, item.Value)
	}
	slices.Sort(values)
	return slices.Compact(values), nil
}
