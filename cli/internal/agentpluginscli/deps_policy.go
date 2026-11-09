package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

const maxLocalDependencyPolicyBytes int64 = 64 << 10

// LocalDependencyPolicy is an explicit, standalone install-time admission gate.
// It is not an organization policy discovery mechanism or a runtime sandbox.
type localDependencyPolicy struct {
	SchemaVersion int `yaml:"schema_version"`
	Sources struct {
		AllowHosts []string `yaml:"allow_hosts"`
		DenyHosts []string `yaml:"deny_hosts"`
	} `yaml:"sources"`
	Integrity struct {
		RequirePinnedGit bool `yaml:"require_pinned_git"`
		RequireArchiveSHA256 bool `yaml:"require_archive_sha256"`
	} `yaml:"integrity"`
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
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&policy); err != nil {
		return policy, fmt.Errorf("decode local dependency policy: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return policy, errors.New("local dependency policy must have exactly one YAML document")
		}
		return policy, fmt.Errorf("decode trailing local dependency policy data: %w", err)
	}
	if policy.SchemaVersion != 1 {
		return policy, fmt.Errorf("unsupported local dependency policy schema_version %d", policy.SchemaVersion)
	}
	for _, hosts := range [][]string{policy.Sources.AllowHosts, policy.Sources.DenyHosts} {
		for _, host := range hosts {
			if !validPolicyHost(host) {
				return policy, fmt.Errorf("invalid local dependency policy hostname %q", host)
			}
		}
	}
	return policy, nil
}

func validPolicyHost(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || strings.TrimSpace(host) != host {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
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
		if pin.Source.Git.Repository != "" {
			return "github.com", nil
		}
		return hostFromPolicyURL(pin.Source.Git.URL)
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
		if len(policy.Sources.AllowHosts) != 0 && !containsPolicyHost(policy.Sources.AllowHosts, host) {
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

func checkLocalDependencyPolicy(path string, lock environment.Lock) error {
	if path == "" {
		return nil // Unchanged default: explicit opt-in only.
	}
	policy, err := readLocalDependencyPolicy(path)
	if err != nil {
		return err
	}
	return policy.evaluate(lock)
}
