package agentpluginscli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

const maxLocalDependencyPolicyBytes int64 = 64 << 10

// LocalDependencyPolicy is an explicit, standalone install-time admission gate.
// It is not an organization policy discovery mechanism or a runtime sandbox.
// A present empty allow-list denies all. An omitted list imposes no restriction.
// This distinction prevents "allow_hosts: []" from silently disabling policy.
type policyAllowList struct {
	Values  []string
	Present bool
}

func (list *policyAllowList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("policy allow-list must be a YAML sequence, not null or scalar")
	}
	var values []string
	if err := node.Decode(&values); err != nil {
		return err
	}
	list.Values, list.Present = values, true
	return nil
}

type localDependencyPolicy struct {
	SchemaVersion int `yaml:"schema_version"`
	Sources       struct {
		AllowHosts policyAllowList `yaml:"allow_hosts"`
		DenyHosts  []string        `yaml:"deny_hosts"`
	} `yaml:"sources"`
	Integrity struct {
		RequirePinnedGit     bool `yaml:"require_pinned_git"`
		RequireArchiveSHA256 bool `yaml:"require_archive_sha256"`
	} `yaml:"integrity"`
	MCP struct {
		AllowedTransports policyAllowList `yaml:"allowed_transports"`
	} `yaml:"mcp"`
}

func readLocalDependencyPolicy(path string) (localDependencyPolicy, error) {
	var policy localDependencyPolicy
	if strings.TrimSpace(path) != path || path == "" {
		return policy, errors.New("local dependency policy must have a nonempty unambiguous path")
	}
	body, err := readDependencyDocument(path, maxLocalDependencyPolicyBytes)
	if err != nil {
		return policy, fmt.Errorf("read local dependency policy: %w", err)
	}
	// yaml.v3 skips custom decoders for explicit null values. Reject them
	// before KnownFields decoding so null cannot silently disable policy.
	var syntax yaml.Node
	if err := yaml.Unmarshal(body, &syntax); err != nil {
		return policy, fmt.Errorf("parse local dependency policy: %w", err)
	}
	if err := rejectNullPolicyNode(&syntax); err != nil {
		return policy, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&policy); err != nil {
		return policy, fmt.Errorf("decode local dependency policy: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return policy, errors.New("local dependency policy must have exactly one YAML document")
		}
		return policy, fmt.Errorf("decode trailing local dependency policy data: %w", err)
	}
	if policy.SchemaVersion != 1 {
		return policy, fmt.Errorf("unsupported local dependency policy schema_version %d", policy.SchemaVersion)
	}
	for _, hosts := range [][]string{policy.Sources.AllowHosts.Values, policy.Sources.DenyHosts} {
		for _, host := range hosts {
			if !validPolicyHost(host) {
				return policy, fmt.Errorf("invalid local dependency policy hostname %q", host)
			}
		}
	}
	for _, transport := range policy.MCP.AllowedTransports.Values {
		switch transport {
		case "stdio", "streamable-http", "sse":
		default:
			return policy, fmt.Errorf("unsupported local dependency policy MCP transport %q", transport)
		}
	}
	return policy, nil
}

// Explicit null is never an opt-out: omit optional keys instead.
func rejectNullPolicyNode(node *yaml.Node) error {
	if node.Tag == "!!null" {
		return errors.New("local dependency policy must not contain explicit null values")
	}
	for _, child := range node.Content {
		if err := rejectNullPolicyNode(child); err != nil {
			return err
		}
	}
	return nil
}

func validPolicyHost(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || strings.TrimSpace(host) != host {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// Exact source authority comes from an already admitted lock. Fail closed if a
// new source type cannot be classified: never silently exempt it from policy.
func lockedSourceHost(pin environment.LockedPackage) (string, error) {
	switch {
	case pin.Source.Git != nil:
		// A concrete URL takes precedence over a GitHub-style shorthand.
		// Treating a remote URL as github.com merely because Repository is
		// populated could silently bypass a host allow/deny policy.
		if pin.Source.Git.URL != "" {
			return hostFromPolicyURL(pin.Source.Git.URL)
		}
		if pin.Source.Git.Repository != "" {
			return "github.com", nil
		}
		return "", errors.New("git dependency source has no origin authority")
	case pin.Source.Registry != nil:
		// Registry coordinates bind a signed Directory publication to a
		// GitHub owner/repository + exact commit.
		return "github.com", nil
	case pin.Source.Archive != nil:
		return hostFromPolicyURL(pin.Source.Archive.URL)
	case pin.Source.Web != nil:
		return hostFromPolicyURL(pin.Source.Web.URL)
	default:
		return "", errors.New("unrecognized dependency source kind")
	}
}

func hostFromPolicyURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "ssh") {
		return "", errors.New("dependency source is not a recognized HTTPS/SSH URL")
	}
	host := strings.ToLower(u.Hostname())
	if !validPolicyHost(host) {
		return "", errors.New("dependency source has an invalid hostname")
	}
	return host, nil
}

func containsPolicyHost(hosts []string, host string) bool {
	for _, candidate := range hosts {
		if candidate == host {
			return true
		}
	}
	return false
}

func (policy localDependencyPolicy) evaluate(lock environment.Lock) error {
	for _, pin := range lock.Packages {
		host, err := lockedSourceHost(pin)
		if err != nil {
			return fmt.Errorf("local dependency policy: source for %q is not admissible: %w", pin.Alias, err)
		}
		if containsPolicyHost(policy.Sources.DenyHosts, host) {
			return fmt.Errorf("local dependency policy: source for %q uses denied host %q", pin.Alias, host)
		}
		if policy.Sources.AllowHosts.Present && !containsPolicyHost(policy.Sources.AllowHosts.Values, host) {
			return fmt.Errorf("local dependency policy: source for %q uses unapproved host %q", pin.Alias, host)
		}
		if policy.Integrity.RequirePinnedGit && pin.Source.Git != nil && pin.Source.Git.Requested.Kind != environment.SelectorCommit {
			return fmt.Errorf("local dependency policy: Git source for %q must request an exact commit", pin.Alias)
		}
		if policy.Integrity.RequireArchiveSHA256 && pin.Source.Archive != nil && pin.Source.Archive.RequestedSHA256 == "" {
			return fmt.Errorf("local dependency policy: archive source for %q must request an exact SHA256", pin.Alias)
		}
	}
	return nil
}

// evaluateLocalDependencyPolicy returns a fingerprint of the exact
// *evaluated semantics* so a policy changed between review and final apply
// cannot silently authorize a different dependency operation. Read only.
func evaluateLocalDependencyPolicy(ctx context.Context, app App, path string, lock environment.Lock) (string, error) {
	if path == "" {
		return "", nil // Unchanged default: explicit opt-in only.
	}
	policy, err := readLocalDependencyPolicy(path)
	if err != nil {
		return "", err
	}
	if err := policy.evaluate(lock); err != nil {
		return "", err
	}
	if !policy.MCP.AllowedTransports.Present {
		return fingerprintLocalDependencyPolicy(policy)
	}
	for _, pin := range lock.Packages {
		_, err := inspectPinnedDependency(ctx, app, pin, func(envelope domain.PackageEnvelope) error {
			for name, server := range envelope.MCP.Servers {
				allowed := false
				for _, transport := range policy.MCP.AllowedTransports.Values {
					allowed = allowed || transport == server.Type
				}
				if !allowed {
					return fmt.Errorf("local dependency policy: package %q MCP %q uses disallowed transport %q", pin.Alias, name, server.Type)
				}
			}
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("local dependency policy: verify package %q MCP transport: %w", pin.Alias, err)
		}
	}
	return fingerprintLocalDependencyPolicy(policy)
}

func fingerprintLocalDependencyPolicy(policy localDependencyPolicy) (string, error) {
	// All fields are explicit and serializable; a JSON digest prevents changing
	// the rules after confirmation while ignoring inconsequential YAML trivia.
	body, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("encode local dependency policy: %w", err)
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:]), nil
}

func checkLocalDependencyPolicy(ctx context.Context, app App, path string, lock environment.Lock) error {
	_, err := evaluateLocalDependencyPolicy(ctx, app, path, lock)
	return err
}
