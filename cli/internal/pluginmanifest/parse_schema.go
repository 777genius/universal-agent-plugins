package pluginmanifest

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type schemaSpec struct {
	Kind   yaml.Kind
	Scalar scalarKind
	Fields map[string]schemaSpec
	Seq    *schemaSpec
}

type scalarKind int

const (
	scalarAny scalarKind = iota
	scalarString
)

func manifestSchema() schemaSpec {
	return schemaSpec{Kind: yaml.MappingNode, Fields: map[string]schemaSpec{
		"api_version": {Kind: yaml.ScalarNode, Scalar: scalarString},
		"format":      {Kind: yaml.ScalarNode, Scalar: scalarString},
		"name":        {Kind: yaml.ScalarNode, Scalar: scalarString},
		"version":     {Kind: yaml.ScalarNode, Scalar: scalarString},
		"description": {Kind: yaml.ScalarNode, Scalar: scalarString},
		"author": {Kind: yaml.MappingNode, Fields: map[string]schemaSpec{
			"name":  {Kind: yaml.ScalarNode, Scalar: scalarString},
			"email": {Kind: yaml.ScalarNode, Scalar: scalarString},
			"url":   {Kind: yaml.ScalarNode, Scalar: scalarString},
		}},
		"homepage":   {Kind: yaml.ScalarNode, Scalar: scalarString},
		"repository": {Kind: yaml.ScalarNode, Scalar: scalarString},
		"license":    {Kind: yaml.ScalarNode, Scalar: scalarString},
		"keywords":   {Kind: yaml.SequenceNode, Seq: &schemaSpec{Kind: yaml.ScalarNode, Scalar: scalarString}},
		"targets":    {Kind: yaml.SequenceNode, Seq: &schemaSpec{Kind: yaml.ScalarNode, Scalar: scalarString}},
	}}
}

func launcherSchema() schemaSpec {
	return schemaSpec{Kind: yaml.MappingNode, Fields: map[string]schemaSpec{
		"runtime":    {Kind: yaml.ScalarNode, Scalar: scalarString},
		"entrypoint": {Kind: yaml.ScalarNode, Scalar: scalarString},
	}}
}

func walkNode(node *yaml.Node, path string, spec schemaSpec, seen map[string]struct{}, warnings *[]Warning) {
	if node == nil {
		return
	}
	if len(spec.Fields) > 0 && node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			key := strings.TrimSpace(keyNode.Value)
			keyPath := joinPath(path, key)
			child, ok := spec.Fields[key]
			if !ok {
				appendWarning(seen, warnings, Warning{
					Kind:    WarningUnknownField,
					Path:    keyPath,
					Message: "unknown plugin.yaml field: " + keyPath,
				})
				continue
			}
			walkNode(valNode, keyPath, child, seen, warnings)
		}
		return
	}
	if spec.Seq != nil && node.Kind == yaml.SequenceNode {
		for idx, item := range node.Content {
			walkNode(item, fmt.Sprintf("%s[%d]", path, idx), *spec.Seq, seen, warnings)
		}
	}
}

func validateSchema(body []byte, label string, spec schemaSpec, allowUnknown bool) error {
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&doc); err != nil {
		return fmt.Errorf("parse %s: %w", label, err)
	}
	if len(doc.Content) == 0 {
		return nil
	}
	return validateSchemaNode(doc.Content[0], label, spec, allowUnknown)
}

func validateSchemaNode(node *yaml.Node, path string, spec schemaSpec, allowUnknown bool) error {
	if node == nil {
		return nil
	}
	if spec.Kind != 0 && node.Kind != spec.Kind {
		return fmt.Errorf("invalid %s: expected %s", path, describeSchemaKind(spec.Kind))
	}
	switch spec.Kind {
	case yaml.MappingNode:
		seen := map[string]struct{}{}
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			if keyNode.Kind != yaml.ScalarNode || !isStringSchemaScalar(keyNode) {
				return fmt.Errorf("invalid %s: mapping keys must be strings", path)
			}
			key := strings.TrimSpace(keyNode.Value)
			keyPath := joinPath(path, key)
			if _, ok := seen[key]; ok {
				return fmt.Errorf("invalid %s: duplicate field %q", path, key)
			}
			seen[key] = struct{}{}
			child, ok := spec.Fields[key]
			if !ok {
				if allowUnknown {
					continue
				}
				return fmt.Errorf("invalid %s: unknown field %q", path, key)
			}
			if err := validateSchemaNode(valNode, keyPath, child, allowUnknown); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			if err := validateSchemaNode(item, fmt.Sprintf("%s[%d]", path, idx), *spec.Seq, allowUnknown); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if spec.Scalar == scalarString && !isStringSchemaScalar(node) {
			return fmt.Errorf("invalid %s: expected string", path)
		}
	}
	return nil
}

func isStringSchemaScalar(node *yaml.Node) bool {
	switch node.Tag {
	case "", "!!str", "tag:yaml.org,2002:str", "!!null", "tag:yaml.org,2002:null":
		return true
	default:
		return false
	}
}

func describeSchemaKind(kind yaml.Kind) string {
	switch kind {
	case yaml.MappingNode:
		return "a YAML mapping"
	case yaml.SequenceNode:
		return "a YAML sequence"
	case yaml.ScalarNode:
		return "a YAML scalar"
	default:
		return "a valid YAML value"
	}
}

func collectWarnings(body []byte) ([]Warning, error) {
	var doc yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil
	}
	var warnings []Warning
	seen := map[string]struct{}{}
	walkNode(doc.Content[0], "", manifestSchema(), seen, &warnings)
	return warnings, nil
}
