package commandext

import (
	"bytes"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// SkillFile is the main file of a rendered skill.
const SkillFile = "SKILL.md"

// windowsReserved are characters no Windows file name may hold. fs.ValidPath
// accepts them, so portableFileName rejects them itself.
const windowsReserved = `\:*?"<>|`

// Render turns a command into the client-neutral skill: name and description
// in the header, the command body unchanged. Client decorators may add fields
// and files to the result; nothing else is per client.
func Render(command domain.Command, pluginName string) domain.RenderedSkill {
	return domain.RenderedSkill{
		Frontmatter: []domain.FrontmatterField{
			{Key: "name", Value: SkillName(pluginName, command.Name)},
			{Key: "description", Value: command.Description},
		},
		Body: append([]byte(nil), command.Body...),
	}
}

// Marshal returns the files of a rendered skill, keyed by slash path below the
// skill directory. The header keeps field order; strings are double-quoted so
// a value such as "yes" stays a string, and bools are plain YAML booleans.
func Marshal(skill domain.RenderedSkill) (map[string][]byte, error) {
	header := &yaml.Node{Kind: yaml.MappingNode}
	seen := map[string]bool{}
	for _, field := range skill.Frontmatter {
		if field.Key == "" || seen[field.Key] {
			return nil, fmt.Errorf("rendered skill frontmatter field %q is empty or repeated", field.Key)
		}
		seen[field.Key] = true
		value, err := scalar(field)
		if err != nil {
			return nil, err
		}
		header.Content = append(header.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: field.Key}, value)
	}
	var document bytes.Buffer
	document.WriteString("---\n")
	encoder := yaml.NewEncoder(&document)
	encoder.SetIndent(2)
	if err := encoder.Encode(header); err != nil {
		return nil, fmt.Errorf("encode rendered skill frontmatter: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("encode rendered skill frontmatter: %w", err)
	}
	document.WriteString("---\n")
	document.Write(skill.Body)
	files := map[string][]byte{SkillFile: document.Bytes()}
	for name, body := range skill.Files {
		if !portableFileName(name) || strings.EqualFold(name, SkillFile) {
			return nil, fmt.Errorf("rendered skill file %q is not a plain relative path", name)
		}
		files[name] = append([]byte(nil), body...)
	}
	return files, nil
}

// portableFileName reports whether a rendered file path can be written on
// every client platform: slash-separated segments without a control
// character, a character Windows forbids, a trailing dot or space, or a
// reserved Windows device name.
func portableFileName(name string) bool {
	if !fs.ValidPath(name) || name == "." {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if strings.ContainsAny(segment, windowsReserved) || strings.ContainsFunc(segment, func(r rune) bool { return r < 0x20 }) ||
			strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") || domain.ReservedPhysicalStem(segment) {
			return false
		}
	}
	return true
}

func scalar(field domain.FrontmatterField) (*yaml.Node, error) {
	switch value := field.Value.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}, nil
	default:
		return nil, fmt.Errorf("rendered skill frontmatter field %q has unsupported type %T", field.Key, field.Value)
	}
}
