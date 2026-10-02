package hostprep

import (
	"errors"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

var ErrTransportUnsupported = errors.New("host_transport_unsupported")

// Snapshot is immutable, private operational evidence. Accessors
// return copies. The domain carries only its validation port, keeping it free
// of adapter and parent-module imports. No fields enter public JSON.
type Snapshot struct {
	executable, root string
	environment      []string
	evidence         opencodehost.VersionEvidence
	profile          opencodehost.Profile
	selections       []opencodehost.Selection
}

func newSnapshot(executable, root string, environment []string, evidence opencodehost.VersionEvidence, profile opencodehost.Profile, selections []opencodehost.Selection) *Snapshot {
	h := &Snapshot{executable: executable, root: root, environment: slices.Clone(environment), evidence: evidence, profile: profile.Clone()}
	for _, selection := range selections {
		h.selections = append(h.selections, selection.Clone())
	}
	return h
}
func (h *Snapshot) Profile() opencodehost.Profile { return h.profile.Clone() }

// ConfigDialect exposes only the immutable prepared config selection.
func (h *Snapshot) ConfigDialect() string                  { return string(h.profile.ConfigDialect) }
func (h *Snapshot) Evidence() opencodehost.VersionEvidence { return h.evidence }
func (h *Snapshot) Root() string                           { return h.root }
func (h *Snapshot) Target() (string, []string) {
	return h.executable, slices.Clone(h.environment)
}
func (h *Snapshot) Selections() []opencodehost.Selection {
	out := make([]opencodehost.Selection, len(h.selections))
	for i, selection := range h.selections {
		out[i] = selection.Clone()
	}
	return out
}

// SelectNative selects skills and MCP independently of observers. The
// selected config candidate binds the native projection to the prepared profile.
func SelectNative(profile opencodehost.Profile, skills bool, transports []string) ([]opencodehost.Selection, error) {
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
			return nil, ErrTransportUnsupported
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

func (h *Snapshot) ValidateNative(skills bool, transports []string) error {
	_, err := SelectNative(h.profile, skills, transports)
	return err
}
