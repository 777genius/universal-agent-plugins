package domain

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// EvaluateInstallPolicy returns all relevant denials for one independent layer.
// Applying layers conjunctively avoids trying to intersect wildcard strings.
func EvaluateInstallPolicy(p InstallPolicy, req InstallPolicyRequest, source InstallPolicySource) []InstallPolicyViolation {
	var result []InstallPolicyViolation
	add := func(code, subject, message string) {
		result = append(result, InstallPolicyViolation{Code: code, Subject: PolicySafeText(subject), Message: message})
	}
	evaluatePolicySource(p, req, source, add)
	evaluatePolicyInstallation(p, req, add)
	if req.Envelope != nil {
		evaluatePolicyComponents(p, *req.Envelope, add)
		evaluatePolicyMCP(p.MCP, *req.Envelope, add)
	}
	return result
}

type policyDenial func(code, subject, message string)

func evaluatePolicySource(p InstallPolicy, req InstallPolicyRequest, source InstallPolicySource, add policyDenial) {
	if !p.ConstrainsSource() {
		return
	}
	if source.Kind == "" || (p.SchemaVersion == 1 && source.Host == "") {
		add("sources.identity", "source", "source has no admissible remote authority")
		return
	}
	evaluatePolicySourceLists(p.Sources, source, add)
	if p.Integrity.RequirePinnedGit && source.Kind == "git" && (req.GitPinned == nil || !*req.GitPinned) {
		add("integrity.require_pinned_git", "source", "Git source must request an exact commit; a resolved commit is insufficient")
	}
	if p.Integrity.RequireArchiveSHA256 && (source.Kind == "archive" || source.Archive) && (req.ArchivePinned == nil || !*req.ArchivePinned) {
		add("integrity.require_archive_sha256", "source", "archive source must request an exact SHA256; a downloaded checksum is insufficient")
	}
}

func evaluatePolicySourceLists(p InstallPolicySources, s InstallPolicySource, add policyDenial) {
	if p.AllowedKinds.Present && !slices.Contains(p.AllowedKinds.Values, s.Kind) {
		add("sources.allowed_kinds", s.Kind, "source kind is not allowed")
	}
	if s.Host != "" && slices.Contains(p.DenyHosts, s.Host) {
		add("sources.deny_hosts", s.Host, "source uses a denied host")
	}
	if p.AllowHosts.Present && (s.Host == "" || !slices.Contains(p.AllowHosts.Values, s.Host)) {
		add("sources.allow_hosts", "source", "source does not use an approved host")
	}
	if policyRepositoryMatches(p.DenyRepositories, s.Repository) {
		add("sources.deny_repositories", s.Repository, "source repository is denied")
	}
	if p.AllowRepositories.Present && !policyRepositoryMatches(p.AllowRepositories.Values, s.Repository) {
		add("sources.allow_repositories", "source", "source has no approved repository identity")
	}
}

func policyRepositoryMatches(patterns []string, repository string) bool {
	if repository == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern == repository || (strings.HasSuffix(pattern, "/*") && strings.HasPrefix(repository, strings.TrimSuffix(pattern, "*"))) {
			return true
		}
	}
	return false
}

func evaluatePolicyInstallation(p InstallPolicy, req InstallPolicyRequest, add policyDenial) {
	if p.Install.AllowedScopes.Present && (req.Scope != "" || req.Envelope != nil) && !slices.Contains(p.Install.AllowedScopes.Values, string(req.Scope)) {
		add("install.allowed_scopes", "installation", "installation scope is missing or not allowed")
	}
	if !p.Install.AllowedTargets.Present {
		return
	}
	if len(req.Targets) == 0 && req.Envelope != nil {
		add("install.allowed_targets", "installation", "selected target facts are required")
	}
	for _, target := range req.Targets {
		if !slices.Contains(p.Install.AllowedTargets.Values, string(target)) {
			add("install.allowed_targets", string(target), "installation target is not allowed")
		}
	}
}

func evaluatePolicyComponents(p InstallPolicy, e PackageEnvelope, add policyDenial) {
	components := map[string]bool{
		"mcp":        e.MCP.Present || e.Inventory.MCPPresent || len(e.MCP.Servers) > 0 || len(e.MCP.InvalidServer) > 0,
		"skills":     len(e.Skills) > 0 || len(e.Inventory.Skills) > 0 || len(e.Inventory.InvalidSkills) > 0 || e.Inventory.InvalidSkillsRoot,
		"commands":   len(e.Commands) > 0 || len(e.InvalidCommands) > 0,
		"apps":       e.App.Present || e.App.Declared || e.Inventory.AppPresent || len(e.App.Bindings) > 0,
		"extensions": len(e.Manifest.Extensions) > 0 || len(e.Inventory.Extensions) > 0 || policyRawExtensionsPresent(e.Manifest.RawExtensions),
	}
	for _, kind := range p.Components.Denied {
		if components[kind] {
			add("components.denied", kind, "package contains a denied component; choose a package without it")
		}
	}
}

func policyRawExtensionsPresent(raw []byte) bool {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return true
	}
	switch compact.String() {
	case "null", "{}", "[]":
		return false
	default:
		return true
	}
}
