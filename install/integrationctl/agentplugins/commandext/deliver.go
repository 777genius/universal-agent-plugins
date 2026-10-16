package commandext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Decorate adds a client's explicit-invocation mechanism to a rendered skill.
// It may add header fields and files and must leave everything else alone.
type Decorate func(domain.RenderedSkill) (domain.RenderedSkill, error)

// Files renders a command into the files of its skill directory. A command
// that is not explicit-only is never decorated. An explicit-only command
// needs decorate, and the result is checked rather than trusted: the name,
// description and body stay as rendered, and SKILL.md carries
// domain.ExplicitInvocationField as the YAML boolean true. That check is the
// same for every client, so no decorator can deliver the command without it.
func Files(ctx context.Context, command domain.Command, pluginName string, decorate Decorate) (map[string][]byte, error) {
	skill := Render(command, pluginName)
	if command.ExplicitOnly {
		if decorate == nil {
			return nil, fmt.Errorf("command %q is explicit-only and has no client decorator", command.Name)
		}
		decorated, err := decorate(cloneSkill(skill))
		if err != nil {
			return nil, fmt.Errorf("decorate command %q: %w", command.Name, err)
		}
		if !keepsContent(skill, decorated) {
			return nil, fmt.Errorf("decorator changed the name, description or body of command %q", command.Name)
		}
		skill = decorated
	}
	files, err := Marshal(skill)
	if err != nil {
		return nil, err
	}
	if command.ExplicitOnly {
		if err := requireExplicitFlag(ctx, files[SkillFile]); err != nil {
			return nil, fmt.Errorf("command %q: %w", command.Name, err)
		}
	}
	return files, nil
}

// PlannedCommand returns the command behind a planned component and checks
// that the plan names exactly the skill this package's command becomes.
func PlannedCommand(envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.Command, error) {
	command, ok := envelope.Commands[component.SourceName]
	if !ok || component.Source != domain.ComponentSourceCommand || component.Kind != domain.ComponentSkill || component.Name != SkillName(envelope.Manifest.Name, command.Name) {
		return domain.Command{}, fmt.Errorf("planned command skill %q does not match the package", component.Name)
	}
	return command, nil
}

// WithCommandSkills adds the planned command skills to envelope.Skills as they
// appear in a staged or installed tree, so client projectors handle them like
// authored skills. It never touches the filesystem.
func WithCommandSkills(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (domain.PackageEnvelope, error) {
	skills := maps.Clone(envelope.Skills)
	for _, component := range plan.Components {
		if component.Source != domain.ComponentSourceCommand {
			continue
		}
		command, err := PlannedCommand(envelope, component)
		if err != nil {
			return envelope, err
		}
		if _, exists := skills[component.Name]; exists {
			return envelope, fmt.Errorf("command skill %q collides with a package skill", component.Name)
		}
		if skills == nil {
			skills = map[string]domain.Skill{}
		}
		skills[component.Name] = domain.Skill{Name: component.Name, Description: command.Description, RelativePath: "skills/" + component.Name + "/" + SkillFile}
	}
	envelope.Skills = skills
	return envelope, nil
}

// ProjectedSkillNames lists, sorted, every skill directory a command of this
// package can own in an installed tree under any rules: the projection of each
// valid command, and of each rejected entry whose name would still make a
// portable skill name. A name the package's own skills/ uses is never one.
func ProjectedSkillNames(envelope domain.PackageEnvelope) []string {
	taken := foldedNames(envelope.SkillsDirEntries)
	for name := range envelope.Skills {
		taken[strings.ToLower(name)] = true
	}
	keys := append(slices.Collect(maps.Keys(envelope.Commands)), envelope.InvalidCommands...)
	var names []string
	for _, key := range keys {
		projected := SkillName(envelope.Manifest.Name, key)
		if !taken[strings.ToLower(projected)] && portableSkillName(projected) {
			names = append(names, projected)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func cloneSkill(skill domain.RenderedSkill) domain.RenderedSkill {
	skill.Frontmatter = append([]domain.FrontmatterField(nil), skill.Frontmatter...)
	skill.Body = append([]byte(nil), skill.Body...)
	skill.Files = maps.Clone(skill.Files)
	return skill
}

// keepsContent requires the decorated header to start with the rendered one
// and the body to be unchanged.
func keepsContent(rendered, decorated domain.RenderedSkill) bool {
	return len(decorated.Frontmatter) >= len(rendered.Frontmatter) &&
		reflect.DeepEqual(decorated.Frontmatter[:len(rendered.Frontmatter)], rendered.Frontmatter) &&
		bytes.Equal(decorated.Body, rendered.Body)
}

// requireExplicitFlag reads the written header back, so a value that only
// looked like a bool before encoding cannot pass.
func requireExplicitFlag(ctx context.Context, document []byte) error {
	front, err := conformance.DecodeFrontmatter(ctx, document, conformance.DefaultLimits(), map[string]struct{}{domain.ExplicitInvocationField: {}})
	if err != nil {
		return fmt.Errorf("read rendered skill header: %w", err)
	}
	if flag, ok := front.Values[domain.ExplicitInvocationField].(bool); !ok || !flag {
		return errors.New("the rendered skill does not carry " + domain.ExplicitInvocationField + ": true")
	}
	return nil
}
