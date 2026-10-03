package opencodehost

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Regression: loose token extraction, failed probes, declarations, and newer
// hosts must never manufacture a qualified V1 codec or observer capability.
func TestResolveClosedQualification(t *testing.T) {
	for _, status := range []string{"absent", "failed", "timed_out", "malformed", "output_limit", "not_requested", "invented"} {
		p := Resolve(VersionEvidence{Version: "1.18.33", Source: "executable_version", ProbeStatus: status})
		if p.Family != Unknown || p.ConfigDialect != DialectUnknown || p.Capabilities[MCPStdio] != Unverified {
			t.Fatalf("%s: %+v", status, p)
		}
	}
	for _, version := range []string{"1.18", "v1.18.33", "01.18.33", "1.18.33-01", "opencode v1.18.33", "1.18.33\n2.0.21", "1.18.33+"} {
		if ValidVersion(version) || Resolve(VersionEvidence{version, "executable_version", "ok", ""}).Family != Unknown {
			t.Fatalf("accepted %q", version)
		}
	}
	for _, source := range []string{"explicit_version", "", "ambient_path"} {
		if p := Resolve(VersionEvidence{"1.18.33", source, "ok", ""}); p.Family != Unknown {
			t.Fatalf("authoritative declaration %s: %+v", source, p)
		}
	}
	cases := []struct {
		version   string
		family    Family
		dialect   Dialect
		qualified bool
	}{
		{"1.18.28", LegacyV1, DialectV1, false}, {"1.18.29", ModernV1, DialectV1, false},
		{"1.18.33", ModernV1, DialectV1, true}, {"1.18.34", ModernV1, DialectV1, true},
		{"2.0.21", V2, DialectV2, true}, {"1.19.0", ModernV1, DialectV1, false},
		{"2.1.0", V2, DialectV2, false}, {"2.0.21-rc.1", V2, DialectV2, false},
		{"2.0.21+other", V2, DialectV2, false}, {"3.0.0", Unsupported, DialectUnknown, false},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			p := Resolve(VersionEvidence{tc.version, "executable_version", "ok", "private-target"})
			if p.Family != tc.family || p.ConfigDialect != tc.dialect || len(p.Capabilities) != 8 {
				t.Fatalf("profile: %+v", p)
			}
			if (p.Capabilities[MCPStdio] == Supported) != tc.qualified || (p.Qualification == "tested_exact") != tc.qualified {
				t.Fatalf("qualification: %+v", p)
			}
			for _, c := range []Capability{GlobalSkillDirectory, MCPStreamableHTTP, LocalPluginDual} {
				if (p.Capabilities[c] == Supported) != tc.qualified {
					t.Fatalf("independent generic qualification %s: %+v", c, p)
				}
			}
			for _, c := range []Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError} {
				if p.Capabilities[c] == Supported {
					t.Fatalf("unproved %s", c)
				}
			}
			body, err := json.Marshal(p)
			if err != nil || strings.Contains(string(body), "private-target") {
				t.Fatalf("public profile leaks: %s %v", body, err)
			}
		})
	}
}

// Regression: sorting candidates cannot hide an invalid lower-priority artifact;
// generic skills and MCP cannot accidentally depend on observer qualification.
func TestSelectValidatesAllAndSortsSupportedArtifacts(t *testing.T) {
	p := Resolve(VersionEvidence{"1.18.33", "executable_version", "ok", ""})
	candidates := []ArtifactRequirement{
		{"z", ConfigV1, []Capability{MCPStdio, MCPStreamableHTTP}},
		{"a", SkillDirectory, []Capability{GlobalSkillDirectory}},
	}
	original := append([]ArtifactRequirement(nil), candidates...)
	for _, order := range [][]ArtifactRequirement{candidates, {candidates[1], candidates[0]}} {
		s, err := Select(p, order)
		if err != nil || s.ArtifactID != "a" || s.Adapter != SkillDirectory {
			t.Fatalf("selection: %+v %v", s, err)
		}
	}
	if !reflect.DeepEqual(candidates, original) {
		t.Fatal("caller order changed")
	}
	invalid := []ArtifactRequirement{
		{"", DualPlacement, nil}, {"bad", AdapterID("unknown"), nil},
		{"bad", DualPlacement, []Capability{LocalPluginDual}}, {"bad", SkillDirectory, nil},
		{"bad", SkillDirectory, []Capability{MCPStdio}}, {"bad", ConfigV1, nil},
		{"bad", ConfigV2, []Capability{GlobalSkillDirectory}}, {"bad", ConfigV1, []Capability{MCPStdio, MCPStdio}},
		{"bad", ConfigV1, []Capability{Capability("unknown")}}, {"bad", ObserverV1, []Capability{LocalPluginDual}},
		{"bad", ObserverV2, []Capability{ObserverCompletion}}, {"a", DualPlacement, nil},
	}
	for _, r := range invalid {
		s, err := Select(p, append(append([]ArtifactRequirement(nil), candidates...), r))
		if !errors.Is(err, ErrInvalidRequirement) || s.ArtifactID != "" || s.Adapter != "" {
			t.Fatalf("invalid %+v: %+v %v", r, s, err)
		}
	}
}

// Regression: a pure V2 candidate must not select V1; deferred placement remains
// usable without a runtime, while observer failure cannot return a usable codec.
func TestSelectEligibilityAndFailureDiagnostics(t *testing.T) {
	v2 := Resolve(VersionEvidence{"2.0.21", "executable_version", "ok", ""})
	unknown := Resolve(VersionEvidence{ProbeStatus: "absent"})
	legacy := Resolve(VersionEvidence{"1.18.28", "executable_version", "ok", ""})
	unqualified := Resolve(VersionEvidence{"2.1.0", "executable_version", "ok", ""})
	tests := []struct {
		name    string
		p       Profile
		req     []ArtifactRequirement
		adapter AdapterID
		err     error
	}{
		{"v2", v2, []ArtifactRequirement{{"v1", ConfigV1, []Capability{MCPStdio}}, {"v2", ConfigV2, []Capability{MCPStdio}}}, ConfigV2, nil},
		{"no-fallback", v2, []ArtifactRequirement{{"v1", ConfigV1, []Capability{MCPStdio}}}, "", ErrNoAdapter},
		{"unknown", unknown, []ArtifactRequirement{{"v1", ConfigV1, []Capability{MCPStdio}}}, "", ErrNoAdapter},
		{"deferred", unknown, []ArtifactRequirement{{"dual", DualPlacement, nil}}, DualPlacement, nil},
		{"observer", v2, []ArtifactRequirement{{"observer", ObserverV2, []Capability{LocalPluginDual, ObserverTerminalError, ObserverCompletion}}}, "", ErrUnverifiedCapability},
		{"new-minor", unqualified, []ArtifactRequirement{{"v2", ConfigV2, []Capability{MCPStdio}}}, "", ErrUnverifiedCapability},
		{"legacy-observer", legacy, []ArtifactRequirement{{"observer", ObserverV1, []Capability{LocalPluginDual, ObserverCompletion}}}, "", ErrNoAdapter},
		{"unqualified-skills", unqualified, []ArtifactRequirement{{"skills", SkillDirectory, []Capability{GlobalSkillDirectory}}}, "", ErrNoAdapter},
		{"empty", v2, nil, "", ErrNoAdapter},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Select(tc.p, tc.req)
			if !errors.Is(err, tc.err) || s.Adapter != tc.adapter {
				t.Fatalf("%+v %v", s, err)
			}
			if err != nil && s.ArtifactID != "" {
				t.Fatal("failure returned usable artifact")
			}
		})
	}
	v2.Capabilities[MCPStdio] = Unverified
	v2.Capabilities[MCPStreamableHTTP] = SupportUnsupported
	s, err := Select(v2, []ArtifactRequirement{{"mixed", ConfigV2, []Capability{MCPStdio, MCPStreamableHTTP}}})
	if !errors.Is(err, ErrUnsupportedCapability) || !reflect.DeepEqual(s.Missing, []Capability{MCPStreamableHTTP}) || !reflect.DeepEqual(s.Unverified, []Capability{MCPStdio}) {
		t.Fatalf("diagnostics: %+v %v", s, err)
	}
	clone := s.Clone()
	clone.Missing[0] = MCPStdio
	profileCopy := v2.Clone()
	profileCopy.Capabilities[MCPStdio] = Supported
	if s.Missing[0] != MCPStreamableHTTP || v2.Capabilities[MCPStdio] != Unverified {
		t.Fatal("snapshot alias")
	}
}
