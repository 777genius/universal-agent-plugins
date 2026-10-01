// Package opencodehost is the closed, pure OpenCode qualification contract.
// It performs no discovery, process execution or filesystem mutation.
package opencodehost

import (
	"maps"
	"regexp"
	"strings"
)

type Family string
type Dialect string
type Capability string
type Support string
type AdapterID string

const (
	Unknown               Family     = "unknown"
	LegacyV1              Family     = "legacy_v1"
	ModernV1              Family     = "modern_v1"
	V2                    Family     = "v2"
	Unsupported           Family     = "unsupported"
	DialectUnknown        Dialect    = "unknown"
	DialectV1             Dialect    = "opencode_v1"
	DialectV2             Dialect    = "opencode_v2"
	Supported             Support    = "supported"
	Unverified            Support    = "unverified"
	SupportUnsupported    Support    = "unsupported"
	LocalPluginDual       Capability = "local_plugin_dual"
	GlobalSkillDirectory  Capability = "global_skill_directory"
	MCPStdio              Capability = "mcp_stdio"
	MCPStreamableHTTP     Capability = "mcp_streamable_http"
	ObserverCompletion    Capability = "observer_completion"
	ObserverQuestion      Capability = "observer_question"
	ObserverPermission    Capability = "observer_permission"
	ObserverTerminalError Capability = "observer_terminal_error"
	DualPlacement         AdapterID  = "dual_placement"
	SkillDirectory        AdapterID  = "skill_directory"
	ConfigV1              AdapterID  = "config_v1"
	ConfigV2              AdapterID  = "config_v2"
	ObserverV1            AdapterID  = "observer_v1"
	ObserverV2            AdapterID  = "observer_v2"
)

var capabilities = [...]Capability{LocalPluginDual, GlobalSkillDirectory, MCPStdio, MCPStreamableHTTP, ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}

type VersionEvidence struct {
	Version            string
	Source             string
	ProbeStatus        string
	ExecutableIdentity string `json:"-"`
}

type Profile struct {
	Schema        int
	Family        Family
	Version       string
	ConfigDialect Dialect
	PluginEntry   string
	Qualification string
	EvidenceID    string
	Capabilities  map[Capability]Support
	Reason        string
}

// Clone returns a separately owned snapshot, including the capability map.
func (p Profile) Clone() Profile { p.Capabilities = maps.Clone(p.Capabilities); return p }

var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// ValidVersion accepts complete SemVer, including its numeric prerelease rule.
func ValidVersion(v string) bool {
	if !semver.MatchString(v) {
		return false
	}
	core := strings.SplitN(v, "+", 2)[0]
	if _, pre, ok := strings.Cut(core, "-"); ok {
		for _, id := range strings.Split(pre, ".") {
			if len(id) > 1 && id[0] == '0' && strings.Trim(id, "0123456789") == "" {
				return false
			}
		}
	}
	return true
}

// Resolve grants only the per-capability facts frozen in lane-a-freeze.md.
// Explicit declarations do not bind a runtime and cannot grant eligibility.
func Resolve(e VersionEvidence) Profile {
	p := Profile{Schema: 1, Family: Unknown, ConfigDialect: DialectUnknown, PluginEntry: "unknown", Qualification: "unverified", Reason: "host_unverified", Capabilities: map[Capability]Support{}}
	for _, c := range capabilities {
		p.Capabilities[c] = Unverified
	}
	if e.ProbeStatus != "ok" {
		switch e.ProbeStatus {
		case "absent", "failed", "timed_out", "malformed", "output_limit", "not_requested":
			p.Reason = "probe_" + e.ProbeStatus
		}
		return p
	}
	if !ValidVersion(e.Version) {
		p.Reason = "probe_malformed"
		return p
	}
	if e.Source != "executable_version" && e.Source != "host_runtime" {
		p.Reason = "evidence_not_authoritative"
		return p
	}
	p.Version = e.Version
	core := strings.SplitN(strings.SplitN(e.Version, "+", 2)[0], "-", 2)[0]
	parts := strings.Split(core, ".")
	switch parts[0] {
	case "1":
		p.Family = LegacyV1
		p.ConfigDialect = DialectV1
		p.PluginEntry = "legacy_function"
		if numberAtLeast(parts[1], "18") && (parts[1] != "18" || numberAtLeast(parts[2], "29")) {
			p.Family = ModernV1
			p.PluginEntry = "dual"
		}
	case "2":
		p.Family = V2
		p.ConfigDialect = DialectV2
		p.PluginEntry = "dual"
	default:
		p.Family = Unsupported
		p.PluginEntry = "unavailable"
		p.Reason = "host_unsupported"
		for _, c := range capabilities {
			p.Capabilities[c] = SupportUnsupported
		}
		return p
	}
	if p.Family == LegacyV1 {
		p.Reason = "legacy_host"
		p.Capabilities[LocalPluginDual] = SupportUnsupported
		return p
	}
	// No prerelease, build variant, new minor or major inherits semantic support.
	switch e.Version {
	case "1.18.33", "1.18.34", "2.0.21":
		p.Qualification = "tested_exact"
		p.EvidenceID = "p0-lane-a-20261001-generic-" + e.Version
		p.Reason = "generic_qualified"
		for _, c := range []Capability{LocalPluginDual, GlobalSkillDirectory, MCPStdio, MCPStreamableHTTP} {
			p.Capabilities[c] = Supported
		}
	default:
		p.Reason = "version_unqualified"
	}
	return p
}

func numberAtLeast(a, b string) bool { return len(a) > len(b) || len(a) == len(b) && a >= b }
