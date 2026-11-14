package contracttest

import (
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// OpenCodeV1Host supplies explicit, pure qualified authority for direct adapter
// and usecase fixtures that bypass Engine.Prepare. It is not host probe evidence.
// Each Profile call returns a separately owned qualification-ledger snapshot.
type OpenCodeV1Host struct{}

func (OpenCodeV1Host) Profile() opencodehost.Profile {
	return opencodehost.Resolve(opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok"})
}

func (h OpenCodeV1Host) ConfigDialect() string { return string(h.Profile().ConfigDialect) }

func (h OpenCodeV1Host) ValidateNative(skills bool, transports []string) error {
	profile := h.Profile()
	if skills {
		if _, err := opencodehost.Select(profile, []opencodehost.ArtifactRequirement{{ID: "fixture-skills", Adapter: opencodehost.SkillDirectory, Required: []opencodehost.Capability{opencodehost.GlobalSkillDirectory}}}); err != nil {
			return err
		}
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
			return fmt.Errorf("unsupported OpenCode fixture transport %q", transport)
		}
		if !slices.Contains(required, capability) {
			required = append(required, capability)
		}
	}
	if len(required) > 0 {
		_, err := opencodehost.Select(profile, []opencodehost.ArtifactRequirement{{ID: "fixture-config", Adapter: opencodehost.ConfigV1, Required: required}})
		return err
	}
	return nil
}
