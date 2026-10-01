package installer

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// OpenCodePreparedHost is immutable, private operational evidence. Accessors
// return copies. The domain carries only its validation port, keeping it free
// of adapter and parent-module imports. No fields enter public JSON.
type OpenCodePreparedHost struct {
	executable, root string
	environment      []string
	evidence         opencodehost.VersionEvidence
	profile          opencodehost.Profile
	selections       []opencodehost.Selection
}

func newOpenCodePreparedHost(executable, root string, environment []string, evidence opencodehost.VersionEvidence, profile opencodehost.Profile, selections []opencodehost.Selection) *OpenCodePreparedHost {
	h := &OpenCodePreparedHost{executable: executable, root: root, environment: slices.Clone(environment), evidence: evidence, profile: profile.Clone()}
	for _, selection := range selections {
		h.selections = append(h.selections, selection.Clone())
	}
	return h
}
func (h *OpenCodePreparedHost) Profile() opencodehost.Profile { return h.profile.Clone() }

// ConfigDialect exposes only the immutable prepared config selection.
func (h *OpenCodePreparedHost) ConfigDialect() string                  { return string(h.profile.ConfigDialect) }
func (h *OpenCodePreparedHost) Evidence() opencodehost.VersionEvidence { return h.evidence }
func (h *OpenCodePreparedHost) Root() string                           { return h.root }
func (h *OpenCodePreparedHost) Target() (string, []string) {
	return h.executable, slices.Clone(h.environment)
}
func (h *OpenCodePreparedHost) Selections() []opencodehost.Selection {
	out := make([]opencodehost.Selection, len(h.selections))
	for i, selection := range h.selections {
		out[i] = selection.Clone()
	}
	return out
}

// selectOpenCodeNative selects skills and MCP independently of observers. The
// selected config candidate binds the native projection to the prepared profile.
func selectOpenCodeNative(profile opencodehost.Profile, skills bool, transports []string) ([]opencodehost.Selection, error) {
	var out []opencodehost.Selection
	if skills {
		selection, err := opencodehost.Select(profile, []opencodehost.ArtifactRequirement{{ID: "global-skills", Adapter: opencodehost.SkillDirectory, Required: []opencodehost.Capability{opencodehost.GlobalSkillDirectory}}})
		if err != nil {
			return nil, err
		}
		out = append(out, selection)
	}
	var required []opencodehost.Capability
	for _, transport := range transports {
		var capability opencodehost.Capability
		switch transport {
		case "stdio":
			capability = opencodehost.MCPStdio
		case "streamable-http":
			capability = opencodehost.MCPStreamableHTTP
		default:
			return nil, errOpenCodeTransportUnsupported
		}
		if !slices.Contains(required, capability) {
			required = append(required, capability)
		}
	}
	if len(required) > 0 {
		slices.Sort(required)
		selection, err := opencodehost.Select(profile, []opencodehost.ArtifactRequirement{
			{ID: "mcp-config-v1", Adapter: opencodehost.ConfigV1, Required: required},
			{ID: "mcp-config-v2", Adapter: opencodehost.ConfigV2, Required: required},
		})
		if err != nil {
			return nil, err
		}
		out = append(out, selection)
	}
	return out, nil
}

func (h *OpenCodePreparedHost) ValidateNative(skills bool, transports []string) error {
	_, err := selectOpenCodeNative(h.profile, skills, transports)
	return err
}
