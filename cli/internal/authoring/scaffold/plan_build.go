package scaffold

import (
	"fmt"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// BuildPlan admits one explicit template, then renders a deterministic, private package tree.
func BuildPlan(o Options) (Plan, error) {
	if err := validatePlanInput(o); err != nil {
		return Plan{}, err
	}
	registry, err := specregistry.New()
	if err != nil {
		return Plan{}, err
	}
	manifest, err := planManifest(o, registry)
	if err != nil {
		return Plan{}, err
	}
	lane, err := planLane(o)
	if err != nil {
		return Plan{}, err
	}
	if err := validatePlanTransport(o, lane); err != nil {
		return Plan{}, err
	}
	hasSkill := o.Template == Skill || o.Template == Hybrid
	if err := validatePlanSkill(&o, hasSkill); err != nil {
		return Plan{}, err
	}
	license, err := licenseText(o)
	if err != nil {
		return Plan{}, err
	}
	if license != "" {
		manifest["license"] = o.License
	}
	return renderPlan(o, manifest, registry, lane, hasSkill, license)
}

func validatePlanInput(o Options) error {
	if len(o.Template) > 32 || len(o.MCPChoice) > 32 || len(o.Runtime) > 32 || len(o.License) > 32 {
		return fmt.Errorf("template, MCP choice, runtime, and license values must be bounded identifiers")
	}
	if len(o.Name) > 64 || !utf8.ValidString(o.Name) {
		return identityError("plugin name", o.Name)
	}
	return nil
}

// planManifest enforces the first schema and portable identity checks before template selection.
func planManifest(o Options, registry *specregistry.Registry) (map[string]any, error) {
	manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": o.Name, "version": "0.1.0"}
	if registry.Validate(domain.PluginSchemaV1, manifest) != nil {
		return nil, identityError("plugin name", o.Name)
	}
	if err := portableLeaf(o.Name); err != nil {
		return nil, fmt.Errorf("%w (%s)", identityError("host-safe plugin name", o.Name), err.Error())
	}
	if !textValue(o.Description, 1024) {
		return nil, fmt.Errorf("description must contain 1–1024 characters of nonempty text")
	}
	manifest["description"] = o.Description
	if o.AuthorName != "" {
		if !textValue(o.AuthorName, 256) {
			return nil, fmt.Errorf("invalid explicit author name")
		}
		manifest["author"] = map[string]any{"name": o.AuthorName}
	}
	return manifest, nil
}

func planLane(o Options) (Template, error) {
	lane := o.Template
	switch o.Template {
	case Skill, MCPRemote, MCPStdio:
		if o.MCPChoice != "" {
			return "", fmt.Errorf("MCP choice is only accepted for hybrid")
		}
	case Hybrid:
		if o.MCPChoice != MCPRemote && o.MCPChoice != MCPStdio {
			return "", fmt.Errorf("hybrid requires an explicit mcp-remote or mcp-stdio choice")
		}
		lane = o.MCPChoice
	default:
		return "", fmt.Errorf("unsupported template %q", o.Template)
	}
	return lane, nil
}

func validatePlanTransport(o Options, lane Template) error {
	if lane == MCPStdio {
		if o.Runtime != "node" {
			return fmt.Errorf("stdio requires runtime node (Node >=22); runtime %q is not implemented", o.Runtime)
		}
		if o.RemoteURL != "" {
			return fmt.Errorf("remote URL is not accepted for stdio")
		}
	} else if o.Runtime != "" {
		return fmt.Errorf("runtime is only accepted for stdio; only node is implemented")
	}
	if lane == MCPRemote {
		if err := validateURL(o.RemoteURL); err != nil {
			return err
		}
	} else if o.RemoteURL != "" {
		return fmt.Errorf("remote URL is only accepted for remote MCP")
	}
	return nil
}

func validatePlanSkill(o *Options, hasSkill bool) error {
	if hasSkill {
		if o.SkillName == "" {
			o.SkillName = o.Name
		}
		if len(o.SkillName) > 64 || !skillName.MatchString(o.SkillName) {
			return identityError("skill name", o.SkillName)
		}
		if err := portableLeaf(o.SkillName); err != nil {
			return fmt.Errorf("%w (%s)", identityError("host-safe skill name", o.SkillName), err.Error())
		}
	} else if o.SkillName != "" {
		return fmt.Errorf("skill name requires a skill component")
	}
	return nil
}
