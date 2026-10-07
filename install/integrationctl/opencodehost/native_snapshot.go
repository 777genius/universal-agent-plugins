package opencodehost

import (
	"slices"

	"errors"
)

// NativePrepared is immutable, private operational evidence. Accessors
// return copies. The domain carries only its validation port, keeping it free
// of adapter and parent-module imports. No fields enter public JSON.
var ErrNativeTransportUnsupported = errors.New("host_transport_unsupported")

type NativePrepared struct {
	executable, root string
	environment      []string
	evidence         VersionEvidence
	profile          Profile
	selections       []Selection
}

func NewNativePrepared(executable, root string, environment []string, evidence VersionEvidence, profile Profile, selections []Selection) *NativePrepared {
	h := &NativePrepared{executable: executable, root: root, environment: slices.Clone(environment), evidence: evidence, profile: profile.Clone()}
	for _, selection := range selections {
		h.selections = append(h.selections, selection.Clone())
	}
	return h
}
func (h *NativePrepared) Profile() Profile { return h.profile.Clone() }

// ConfigDialect exposes only the immutable prepared config selection.
func (h *NativePrepared) ConfigDialect() string     { return string(h.profile.ConfigDialect) }
func (h *NativePrepared) Evidence() VersionEvidence { return h.evidence }
func (h *NativePrepared) Root() string              { return h.root }
func (h *NativePrepared) Target() (string, []string) {
	return h.executable, slices.Clone(h.environment)
}
func (h *NativePrepared) Selections() []Selection {
	out := make([]Selection, len(h.selections))
	for i, selection := range h.selections {
		out[i] = selection.Clone()
	}
	return out
}

// SelectNative selects skills and MCP independently of observers. The
// selected config candidate binds the native projection to the prepared profile.
func SelectNative(profile Profile, skills bool, transports []string) ([]Selection, error) {
	var out []Selection
	if skills {
		selection, err := Select(profile, []ArtifactRequirement{{ID: "global-skills", Adapter: SkillDirectory, Required: []Capability{GlobalSkillDirectory}}})
		if err != nil {
			return nil, err
		}
		out = append(out, selection)
	}
	var required []Capability
	for _, transport := range transports {
		var capability Capability
		switch transport {
		case "stdio":
			capability = MCPStdio
		case "streamable-http":
			capability = MCPStreamableHTTP
		default:
			return nil, ErrNativeTransportUnsupported
		}
		if !slices.Contains(required, capability) {
			required = append(required, capability)
		}
	}
	if len(required) > 0 {
		slices.Sort(required)
		selection, err := Select(profile, []ArtifactRequirement{
			{ID: "mcp-config-v1", Adapter: ConfigV1, Required: required},
			{ID: "mcp-config-v2", Adapter: ConfigV2, Required: required},
		})
		if err != nil {
			return nil, err
		}
		out = append(out, selection)
	}
	return out, nil
}

func (h *NativePrepared) ValidateNative(skills bool, transports []string) error {
	if h.profile.Qualification != "tested_exact" {
		return ErrUnverifiedCapability
	}
	_, err := SelectNative(h.profile, skills, transports)
	return err
}
