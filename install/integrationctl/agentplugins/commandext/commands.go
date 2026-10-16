// Package commandext reads, checks and renders portable commands: Markdown
// files in the package's io.github.777genius.agentplugins/commands directory
// that the installer delivers as skills. It does no I/O; the loader and the
// authoring reader hand it what they saw, so both apply exactly one format.
package commandext

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RelativeDirectory is the commands directory relative to the package root.
const RelativeDirectory = domain.ExtensionNamespace + "/" + domain.CommandsDir

const (
	fieldDescription  = "description"
	fieldExplicitOnly = "disable-model-invocation"
	maxDescription    = 1024
	maxSkillName      = 64
)

var (
	commandFileName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	skillName       = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	commandFields   = map[string]struct{}{fieldDescription: {}, fieldExplicitOnly: {}}
)

// RootState is what the reader found at RelativeDirectory.
type RootState int

const (
	RootAbsent RootState = iota
	RootDirectory
	// RootInvalid is a symlink or anything else that is not a real directory,
	// at the commands directory or at the namespace above it.
	RootInvalid
)

// EntryKind is the type of a directory entry, observed without following links.
type EntryKind int

const (
	EntryFile EntryKind = iota
	EntryDirectory
	EntrySymlink
	EntryOther
)

// Entry is one immediate child of the commands directory.
type Entry struct {
	Name string
	Kind EntryKind
	// Body holds a regular file's bytes. A reader stops after one byte more
	// than conformance.DefaultLimits().SkillBytes; a longer body is rejected.
	Body []byte
	// Unreadable marks a regular file whose bytes could not be read.
	Unreadable bool
}

// Listing is the commands directory as a reader observed it. Truncated means
// the reader stopped after more than conformance.DefaultLimits().Members
// entries.
type Listing struct {
	Root      RootState
	Entries   []Entry
	Truncated bool
}

// WithCommands checks every entry of listing against the command format and
// against the package it belongs to, and returns envelope with Commands,
// InvalidCommands and the command diagnostics filled in. It reads
// Manifest.Name and SkillsDirEntries from envelope, so those must be set.
// The only error is the context's.
func WithCommands(ctx context.Context, envelope domain.PackageEnvelope, listing Listing) (domain.PackageEnvelope, error) {
	limits := conformance.DefaultLimits()
	report := &report{limit: limits.Diagnostics}
	commands := map[string]domain.Command{}
	invalid := map[string]bool{}
	switch {
	case listing.Root == RootInvalid:
		report.add(domain.Diagnostic{Code: "commands_root_invalid", Path: RelativeDirectory, Message: "commands must be a real directory inside a real namespace directory"})
	case listing.Truncated || len(listing.Entries) > limits.Members:
		report.add(domain.Diagnostic{Code: "commands_root_invalid", Path: RelativeDirectory, Message: fmt.Sprintf("commands has more than %d entries", limits.Members)})
	case listing.Root == RootDirectory:
		entries := append([]Entry(nil), listing.Entries...)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		taken := foldedNames(envelope.SkillsDirEntries)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name, ".") {
				continue
			}
			command, codes, err := check(ctx, entry, limits)
			if err != nil {
				return envelope, err
			}
			if command.Name != "" {
				codes = append(codes, packageCodes(envelope.Manifest.Name, command.Name, taken)...)
			}
			if len(codes) == 0 {
				commands[command.Name] = command
				continue
			}
			key := strings.TrimSuffix(entry.Name, ".md")
			invalid[key] = true
			for _, code := range codes {
				report.add(domain.Diagnostic{Code: code.code, Path: RelativeDirectory + "/" + entry.Name, Item: key, Message: code.message})
			}
		}
	}
	envelope.Commands, envelope.InvalidCommands = nil, nil
	if len(commands) > 0 {
		envelope.Commands = commands
	}
	for key := range invalid {
		envelope.InvalidCommands = append(envelope.InvalidCommands, key)
	}
	sort.Strings(envelope.InvalidCommands)
	envelope.Diagnostics = append(append([]domain.Diagnostic(nil), envelope.Diagnostics...), report.diagnostics...)
	return envelope, nil
}

// SkillName is the skill a command becomes: the plugin name with dots turned
// into dashes, a dash, and the command name.
func SkillName(pluginName, command string) string {
	return strings.ReplaceAll(pluginName, ".", "-") + "-" + command
}

type finding struct{ code, message string }

type report struct {
	limit       int
	diagnostics []domain.Diagnostic
}

// add caps the output at the diagnostics budget. Rejected commands are still
// listed in InvalidCommands, so nothing becomes deliverable by being cut.
func (r *report) add(diagnostic domain.Diagnostic) {
	if len(r.diagnostics) >= r.limit {
		return
	}
	diagnostic.Severity = domain.SeverityError
	diagnostic.Boundary = domain.BoundaryCommand
	r.diagnostics = append(r.diagnostics, diagnostic)
}

// check applies the per-file rules in their documented order. Only an
// invalid name returns a command without a name. A frontmatter failure does
// not stop the body checks, so the author sees every problem at once.
func check(ctx context.Context, entry Entry, limits conformance.Limits) (domain.Command, []finding, error) {
	name := strings.TrimSuffix(entry.Name, ".md")
	if entry.Kind != EntryFile || !commandFileName.MatchString(entry.Name) || domain.ReservedPhysicalStem(name) {
		return domain.Command{}, []finding{{"command_name_invalid", "a command must be a regular lowercase kebab-case .md file with a portable name"}}, nil
	}
	command := domain.Command{Name: name, RelativePath: RelativeDirectory + "/" + entry.Name}
	if entry.Unreadable {
		return command, []finding{{"command_read_failed", "the command file could not be read"}}, nil
	}
	if len(entry.Body) > limits.SkillBytes {
		return command, []finding{{"command_frontmatter_invalid", fmt.Sprintf("the command file is larger than %d bytes", limits.SkillBytes)}}, nil
	}
	front, err := conformance.DecodeFrontmatter(ctx, entry.Body, limits, commandFields)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return domain.Command{}, nil, ctxErr
	}
	var findings []finding
	body := front.Body
	if err != nil {
		findings = append(findings, finding{"command_frontmatter_invalid", "the command must start with one YAML mapping between --- lines, with unique keys and within the size and nesting limits"})
		if body == nil {
			body = bytes.ReplaceAll(bytes.TrimPrefix(entry.Body, []byte{0xef, 0xbb, 0xbf}), []byte("\r\n"), []byte("\n"))
		}
	} else {
		findings = append(findings, checkFrontmatter(front, &command)...)
	}
	findings = append(findings, checkBody(body)...)
	command.Body = body
	return command, findings, nil
}

func checkFrontmatter(front conformance.Frontmatter, command *domain.Command) []finding {
	var findings []finding
	for _, key := range front.Keys {
		if _, ok := commandFields[key]; !ok {
			findings = append(findings, finding{"command_unknown_field", fmt.Sprintf("frontmatter field %q is not supported; a command allows only %s and %s", key, fieldDescription, fieldExplicitOnly)})
			break
		}
	}
	description, ok := front.Values[fieldDescription].(string)
	switch {
	case !ok:
		findings = append(findings, finding{"command_description_invalid", "description is required and must be a string"})
	case strings.TrimSpace(description) == "" || utf8.RuneCountInString(description) > maxDescription || strings.ContainsAny(description, "\r\n"):
		findings = append(findings, finding{"command_description_invalid", fmt.Sprintf("description must be one non-blank line of at most %d characters", maxDescription)})
	default:
		command.Description = description
	}
	explicit, ok := front.Values[fieldExplicitOnly].(bool)
	if !ok {
		findings = append(findings, finding{"command_invocation_flag_invalid", fieldExplicitOnly + " is required and must be the YAML boolean true or false"})
	}
	command.ExplicitOnly = explicit
	return findings
}

// packageCodes checks the projected skill name of a command that is valid on
// its own: it must be a portable skill name and must not collide with any
// entry under skills/, compared case-insensitively.
func packageCodes(pluginName, command string, taken map[string]bool) []finding {
	projected := SkillName(pluginName, command)
	switch {
	case strings.Contains(projected, "--") || !skillName.MatchString(projected):
		return []finding{{"command_projected_name_invalid", fmt.Sprintf("the skill name %q made from the plugin name is not a portable skill name", projected)}}
	case len(projected) > maxSkillName:
		return []finding{{"command_projected_name_too_long", fmt.Sprintf("the skill name %q is longer than %d characters", projected, maxSkillName)}}
	case taken[strings.ToLower(projected)]:
		return []finding{{"command_skill_name_collision", fmt.Sprintf("skills/%s already exists in the package", projected)}}
	}
	return nil
}

func foldedNames(names []string) map[string]bool {
	folded := make(map[string]bool, len(names))
	for _, name := range names {
		folded[strings.ToLower(name)] = true
	}
	return folded
}
