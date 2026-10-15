// Package manifestedit edits the supported outer syntax of uap.toml. It does
// not validate dependency semantics: the caller must use the strict manifest
// reader before editing and validate the result against its expected intent
// before publishing it. Formatting may normalize whitespace.
package manifestedit

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/creachadair/tomledit/scanner"
)

// ErrUnsupportedLayout means the manifest needs a manual edit before saving.
var ErrUnsupportedLayout = errors.New("unsupported TOML layout; edit uap.toml manually")

// Dependency contains the complete replacement fields of one dependency table.
// Empty fields are omitted. Release is an exact sequence represented as a TOML
// string; semantic acceptance remains the strict manifest reader's job.
type Dependency struct {
	Git, Path, Ref, Commit, Registry, Distribution, Release string
}

type DependencyEdit struct {
	Alias string
	// Value nil removes the dependency table.
	Value *Dependency
}

type TargetEdit struct {
	Name    string
	Aliases []string
	// Remove removes the assignment; an empty Aliases slice otherwise writes [].
	Remove bool
}

type Changes struct {
	Dependencies []DependencyEdit
	Targets      []TargetEdit
}

// Apply returns edited bytes without writing files. It accepts explicit
// [dependencies.alias] tables and a [targets] table, including quoted keys.
// Inline and dotted assignments for these tables are rejected before output.
// Existing key/table order and comments outside replaced values survive edits.
// LF or CRLF is preserved for uniformly terminated input; mixed endings are
// rejected rather than silently normalized. The caller owns semantic validation.
func Apply(body []byte, changes Changes) ([]byte, error) {
	crlf := bytes.Contains(body, []byte("\r\n"))
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	if crlf && bytes.Count(body, []byte("\n")) != bytes.Count(body, []byte("\r\n")) {
		return nil, fmt.Errorf("%w: mixed line endings", ErrUnsupportedLayout)
	}
	doc, err := tomledit.Parse(bytes.NewReader(normalized))
	if err != nil {
		return nil, fmt.Errorf("parse manifest syntax: %w", err)
	}
	if err := supported(doc); err != nil {
		return nil, err
	}
	for _, edit := range changes.Dependencies {
		if err := editDependency(doc, edit); err != nil {
			return nil, err
		}
	}
	for _, edit := range changes.Targets {
		if err := editTarget(doc, edit); err != nil {
			return nil, err
		}
	}
	var out bytes.Buffer
	if err := tomledit.Format(&out, doc); err != nil {
		return nil, fmt.Errorf("format manifest syntax: %w", err)
	}
	result := out.Bytes()
	if crlf {
		result = bytes.ReplaceAll(result, []byte("\n"), []byte("\r\n"))
	}
	return result, nil
}

func editDependency(doc *tomledit.Document, edit DependencyEdit) error {
	if edit.Alias == "" {
		return errors.New("dependency alias is empty")
	}
	name := parser.Key{"dependencies", edit.Alias}
	section := sectionFor(doc, name)
	if edit.Value == nil {
		if section != nil {
			removeSection(doc, section)
		}
		return nil
	}
	if section == nil {
		section = appendSection(doc, name)
	}
	fields := []struct{ name, value string }{
		{"git", edit.Value.Git}, {"path", edit.Value.Path}, {"ref", edit.Value.Ref},
		{"commit", edit.Value.Commit}, {"registry", edit.Value.Registry},
		{"distribution", edit.Value.Distribution}, {"release", edit.Value.Release},
	}
	for _, field := range fields {
		if field.value == "" {
			removeKey(section, field.name)
			continue
		}
		value, err := stringValue(field.value)
		if err != nil {
			return err
		}
		setKey(section, field.name, value)
	}
	return nil
}

func editTarget(doc *tomledit.Document, edit TargetEdit) error {
	if edit.Name == "" {
		return errors.New("target name is empty")
	}
	section := sectionFor(doc, parser.Key{"targets"})
	if edit.Remove {
		if section != nil {
			removeKey(section, edit.Name)
		}
		return nil
	}
	if section == nil {
		section = appendSection(doc, parser.Key{"targets"})
	}
	array := make(parser.Array, 0, len(edit.Aliases))
	for _, alias := range edit.Aliases {
		value, err := stringValue(alias)
		if err != nil {
			return err
		}
		array = append(array, value)
	}
	setKey(section, edit.Name, parser.Value{X: array})
	return nil
}

func supported(doc *tomledit.Document) error {
	sections := append([]*tomledit.Section{doc.Global}, doc.Sections...)
	for _, section := range sections {
		if err := supportedSection(section); err != nil {
			return err
		}
	}
	return nil
}

func managedKey(name parser.Key) bool {
	return len(name) > 0 && (name[0] == "dependencies" || name[0] == "targets")
}

func supportedSection(section *tomledit.Section) error {
	if section == nil {
		return nil
	}
	name := section.TableName()
	managed := managedKey(name)
	if managed && (section.IsArray || len(name) > 2 || (name[0] == "targets" && len(name) != 1)) {
		return fmt.Errorf("%w: table %s", ErrUnsupportedLayout, name)
	}
	for _, item := range section.Items {
		kv, ok := item.(*parser.KeyValue)
		if !ok {
			continue
		}
		if len(name) == 0 && managedKey(kv.Name) {
			return fmt.Errorf("%w: assignment %s", ErrUnsupportedLayout, kv.Name)
		}
		if managed {
			if err := supportedAssignment(name, kv); err != nil {
				return err
			}
		}
	}
	return nil
}

func supportedAssignment(name parser.Key, kv *parser.KeyValue) error {
	if len(kv.Name) != 1 || (name[0] == "dependencies" && len(name) != 2) {
		return fmt.Errorf("%w: assignment in %s", ErrUnsupportedLayout, name)
	}
	if name[0] == "dependencies" {
		if _, ok := kv.Value.X.(parser.Token); !ok {
			return fmt.Errorf("%w: dependency value in %s", ErrUnsupportedLayout, name)
		}
	} else if _, ok := kv.Value.X.(parser.Array); !ok {
		return fmt.Errorf("%w: target value in %s", ErrUnsupportedLayout, name)
	}
	return nil
}

func sectionFor(doc *tomledit.Document, name parser.Key) *tomledit.Section {
	for _, section := range doc.Sections {
		if section.TableName().Equals(name) {
			return section
		}
	}
	return nil
}

func appendSection(doc *tomledit.Document, name parser.Key) *tomledit.Section {
	section := &tomledit.Section{Heading: &parser.Heading{Name: name}}
	doc.Sections = append(doc.Sections, section)
	return section
}

func stringValue(value string) (parser.Value, error) {
	parsed, err := parser.ParseValue(`"` + string(scanner.Escape(value)) + `"`)
	if err != nil {
		return parser.Value{}, fmt.Errorf("encode TOML string: %w", err)
	}
	return parsed, nil
}

func setKey(section *tomledit.Section, name string, value parser.Value) {
	for _, item := range section.Items {
		if kv, ok := item.(*parser.KeyValue); ok && kv.Name.Equals(parser.Key{name}) {
			value.Trailer = kv.Value.Trailer
			kv.Value = value
			return
		}
	}
	section.Items = append(section.Items, &parser.KeyValue{Name: parser.Key{name}, Value: value})
}

func keyComments(kv *parser.KeyValue) parser.Comments {
	comments := append(parser.Comments(nil), kv.Block...)
	if kv.Value.Trailer != "" {
		comments = append(comments, kv.Value.Trailer)
	}
	return comments
}

func removeKey(section *tomledit.Section, name string) {
	for i, item := range section.Items {
		if kv, ok := item.(*parser.KeyValue); ok && kv.Name.Equals(parser.Key{name}) {
			replacement := []parser.Item{}
			if comments := keyComments(kv); len(comments) > 0 {
				replacement = append(replacement, comments)
			}
			section.Items = append(append(section.Items[:i:i], replacement...), section.Items[i+1:]...)
			return
		}
	}
}

func removeSection(doc *tomledit.Document, section *tomledit.Section) {
	for i, existing := range doc.Sections {
		if existing != section {
			continue
		}
		comments := append(parser.Comments(nil), section.Block...)
		if section.Trailer != "" {
			comments = append(comments, section.Trailer)
		}
		for _, item := range section.Items {
			switch item := item.(type) {
			case parser.Comments:
				comments = append(comments, item...)
			case *parser.KeyValue:
				comments = append(comments, keyComments(item)...)
			}
		}
		if len(comments) > 0 {
			var previous *tomledit.Section
			if i > 0 {
				previous = doc.Sections[i-1]
			} else {
				if doc.Global == nil {
					doc.Global = &tomledit.Section{}
				}
				previous = doc.Global
			}
			previous.Items = append(previous.Items, comments)
		}
		doc.Sections = append(doc.Sections[:i], doc.Sections[i+1:]...)
		return
	}
}
