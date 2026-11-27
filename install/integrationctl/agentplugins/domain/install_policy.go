package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// PolicyAllowList distinguishes no restriction from an explicitly empty set.
type PolicyAllowList struct {
	Present bool     `json:"present"`
	Values  []string `json:"values"`
}

type InstallPolicySources struct {
	AllowHosts        PolicyAllowList `json:"allow_hosts"`
	DenyHosts         []string        `json:"deny_hosts,omitempty"`
	AllowedKinds      PolicyAllowList `json:"allowed_kinds"`
	AllowRepositories PolicyAllowList `json:"allow_repositories"`
	DenyRepositories  []string        `json:"deny_repositories,omitempty"`
}

type InstallPolicyIntegrity struct {
	RequirePinnedGit     bool `json:"require_pinned_git"`
	RequireArchiveSHA256 bool `json:"require_archive_sha256"`
}

type InstallPolicyMCP struct {
	AllowedTransports PolicyAllowList `json:"allowed_transports"`
	AllowHosts        PolicyAllowList `json:"allow_hosts"`
	DenyHosts         []string        `json:"deny_hosts,omitempty"`
}

type InstallPolicyInstallation struct {
	AllowedTargets PolicyAllowList `json:"allowed_targets"`
	AllowedScopes  PolicyAllowList `json:"allowed_scopes"`
}

type InstallPolicyComponents struct {
	Denied []string `json:"denied,omitempty"`
}

// InstallPolicy is an inert, versioned set of installation restrictions. A
// policy can grant neither native trust nor permission to mutate foreign files.
type InstallPolicy struct {
	SchemaVersion int                       `json:"schema_version"`
	Sources       InstallPolicySources      `json:"sources"`
	Integrity     InstallPolicyIntegrity    `json:"integrity"`
	MCP           InstallPolicyMCP          `json:"mcp"`
	Install       InstallPolicyInstallation `json:"install"`
	Components    InstallPolicyComponents   `json:"components"`
}

// InstallPolicyRequest carries facts already owned by acquisition and planning.
// A nil Envelope is a partial source preflight. Pin facts describe requested
// intent, never a commit/checksum computed from downloaded bytes.
type InstallPolicyRequest struct {
	Source        SourceIdentity
	OriginMode    OriginMode
	Envelope      *PackageEnvelope
	Scope         InstallScope
	Targets       []ClientID
	GitPinned     *bool
	ArchivePinned *bool
}

// InstallPolicySource is the adapter-normalized authority of a source. The
// evaluator does not interpret caller URLs or perform filesystem/network I/O.
type InstallPolicySource struct {
	Kind       string
	Host       string
	Repository string
}

type InstallPolicyViolation struct {
	Code       string `json:"code"`
	Layer      string `json:"layer,omitempty"`
	PolicyPath string `json:"policy_path,omitempty"`
	Subject    string `json:"subject"`
	Message    string `json:"message"`
}

type InstallPolicyError struct {
	Violations []InstallPolicyViolation `json:"violations"`
}

func (e *InstallPolicyError) Error() string {
	if e == nil || len(e.Violations) == 0 {
		return "installation denied by policy"
	}
	v := e.Violations[0]
	return fmt.Sprintf("installation denied by policy: %s (%s; %s; policy %s); review the policy or ask its owner", PolicySafeText(v.Message), PolicySafeText(v.Subject), PolicySafeText(v.Code), PolicySafeText(v.PolicyPath))
}

var ErrInstallPolicyChanged = errors.New("installation policy changed since review; prepare and review the operation again")

// PolicySafeText bounds diagnostics and removes terminal/control formatting.
func PolicySafeText(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if out.Len() >= 240 {
			out.WriteString("...")
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}

func ValidPolicyHost(host string) bool {
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

// RestrictionCount counts constrained axes, not entries. Legacy schema 1 also
// restricts all sources to recognized remote authorities even without a rule.
func (p InstallPolicy) RestrictionCount() int {
	count := 0
	for _, active := range []bool{
		p.SchemaVersion == 1, p.Sources.AllowHosts.Present, len(p.Sources.DenyHosts) > 0,
		p.Sources.AllowedKinds.Present, p.Sources.AllowRepositories.Present, len(p.Sources.DenyRepositories) > 0,
		p.Integrity.RequirePinnedGit, p.Integrity.RequireArchiveSHA256,
		p.MCP.AllowedTransports.Present, p.MCP.AllowHosts.Present, len(p.MCP.DenyHosts) > 0,
		p.Install.AllowedTargets.Present, p.Install.AllowedScopes.Present, len(p.Components.Denied) > 0,
	} {
		if active {
			count++
		}
	}
	return count
}

func (p InstallPolicy) ConstrainsSource() bool {
	return p.SchemaVersion == 1 || p.Sources.AllowHosts.Present || len(p.Sources.DenyHosts) > 0 ||
		p.Sources.AllowedKinds.Present || p.Sources.AllowRepositories.Present || len(p.Sources.DenyRepositories) > 0 ||
		p.Integrity.RequirePinnedGit || p.Integrity.RequireArchiveSHA256
}
