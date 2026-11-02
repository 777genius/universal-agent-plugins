package conformance

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"gopkg.in/yaml.v3"
)

func (d Decoder) DecodeSkill(ctx context.Context, input SkillInput) (Facts, error) {
	f := d.newFacts()
	l := d.Limits.bounded()
	if !validRelative(input.Directory) || strings.Contains(input.Directory, "/") {
		f.add("skill_directory_invalid", "host/input", "skills", "", "", HostSafety, domain.BoundarySkill)
		f.finish(l)
		return f, nil
	}
	expected := "skills/" + input.Directory + "/SKILL.md"
	ready, state, err := d.observe(ctx, &f, input.Document, expected, l.SkillBytes, domain.BoundarySkill, true)
	f.Coverage.Skills = state
	if err != nil {
		return f, err
	}
	if !ready {
		f.finish(l)
		return f, nil
	}
	skill, err := parseSkill(input.Directory, input.Document.body, func(body []byte) (map[string]any, error) { return boundedFrontmatter(ctx, body, l) }, true)
	if ctx.Err() != nil {
		return f, ctx.Err()
	}
	if err != nil {
		if ctx.Err() != nil {
			return f, ctx.Err()
		}
		code := "skill_yaml_invalid"
		layer := Normative
		rule := "skills/frontmatter"
		var budget *parseFailure
		var failure *skillFailure
		if errors.As(err, &budget) {
			code = budget.Code
			layer = HostSafety
			rule = "host/yaml"
		} else if errors.As(err, &failure) {
			code = failure.Code
		}
		f.add(code, rule, "skills", "", input.Directory, layer, domain.BoundarySkill)
		if layer == Normative {
			f.Coverage.Skills = Fail
		} else {
			f.Coverage.Skills = NotEvaluated
		}
	} else {
		f.Package = &domain.PackageEnvelope{Skills: map[string]domain.Skill{input.Directory: skill}}
	}
	f.finish(l)
	return f, nil
}

// Frontmatter is a Markdown YAML header decoded under the bounded author
// grammar. It is syntax only: no trust, no package identity.
type Frontmatter struct {
	// Keys are the root keys in document order, as written. Merge and
	// non-string keys stay in the list so a closed grammar can reject them.
	Keys []string
	// Values holds only the requested root fields; other values stay opaque.
	Values map[string]any
	// Body is everything after the closing delimiter, with CRLF folded to LF.
	// It is set as soon as the delimiters are found, even when the header
	// then fails to decode, so a caller can still check the content.
	Body []byte
}

// DecodeFrontmatter applies the author Skill grammar to any Markdown document
// with a YAML header: one document, a mapping root, no duplicate keys, and the
// byte, depth, node and member budgets of limits. Nonpositive limits select
// the defaults. Failures are the same typed errors DecodeSkill reports.
func DecodeFrontmatter(ctx context.Context, body []byte, limits Limits, fields map[string]struct{}) (Frontmatter, error) {
	return decodeFrontmatter(ctx, body, limits.bounded(), fields)
}

func boundedFrontmatter(ctx context.Context, body []byte, l Limits) (map[string]any, error) {
	front, err := decodeFrontmatter(ctx, body, l, skillFields)
	return front.Values, err
}

func decodeFrontmatter(ctx context.Context, body []byte, l Limits, fields map[string]struct{}) (Frontmatter, error) {
	// Share the exact LF/CRLF and closing-delimiter grammar with the installer.
	// A leading UTF-8 encoding signature is not Skill content. Installer grammar
	// remains unchanged; original bytes are retained in the canonical Skill.
	data, rest, err := splitFrontmatter(bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}))
	if err != nil {
		return Frontmatter{}, err
	}
	front := Frontmatter{Body: rest}
	if len(data) > l.FrontmatterBytes {
		return front, &parseFailure{Code: "frontmatter_bytes_limit"}
	}
	if err = ctx.Err(); err != nil {
		return front, err
	}
	if err = preflightYAML(ctx, data, l.Depth); err != nil {
		return front, err
	}
	// Decode nodes without alias expansion. At most 64 KiB reaches YAML's parser;
	// node/depth/member budgets are checked before map construction or expansion.
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err = decoder.Decode(&document); err != nil {
		return front, skillError("skill_yaml_invalid", "invalid YAML frontmatter")
	}
	var extra yaml.Node
	if err = decoder.Decode(&extra); err != io.EOF {
		return front, skillError("skill_yaml_documents", "frontmatter must contain one YAML document")
	}
	if err = checkYAMLBudget(ctx, &document, l); err != nil {
		return front, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return front, skillError("skill_frontmatter_type", "frontmatter must be an object")
	}
	root := document.Content[0]
	var values map[string]any
	if err = filterFields(root, fields).Decode(&values); err != nil {
		return front, &parseFailure{Code: "yaml_materialization_unavailable"}
	}
	front.Values = values
	front.Keys = make([]string, 0, len(root.Content)/2)
	for i := 0; i < len(root.Content); i += 2 {
		front.Keys = append(front.Keys, root.Content[i].Value)
	}
	return front, nil
}

func checkYAMLBudget(ctx context.Context, document *yaml.Node, l Limits) error {
	nodes, members := 0, 0
	active := map[*yaml.Node]bool{}
	var walk func(*yaml.Node, int) error
	walk = func(n *yaml.Node, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		nodes++
		if nodes > l.Tokens {
			return &parseFailure{Code: "yaml_node_limit"}
		}
		if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
			depth++
		}
		if depth > l.Depth {
			return &parseFailure{Code: "yaml_depth_limit"}
		}
		if active[n] {
			return &parseFailure{Code: "yaml_alias_cycle"}
		}
		active[n] = true
		defer delete(active, n)
		if n.Kind == yaml.AliasNode {
			if n.Alias == nil {
				return &parseFailure{Code: "yaml_alias_invalid"}
			}
			return walk(n.Alias, depth)
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				members++
				if members > l.Members {
					return &parseFailure{Code: "yaml_member_limit"}
				}
				key := n.Content[i]

				if key.Kind == yaml.ScalarNode {
					identity := key.Tag + "\x00" + key.Value
					if seen[identity] {
						return &parseFailure{Code: "yaml_duplicate_key"}
					}
					seen[identity] = true
				}
			}
		}
		for _, child := range n.Content {
			if err := walk(child, depth); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(document, 0)
}

// Unknown YAML values are opaque. Filter only at the root field boundary, also
// through bounded YAML merge sources; never coerce known scalars into strings.
func filterFields(n *yaml.Node, fields map[string]struct{}) *yaml.Node {
	copy := *n
	switch n.Kind {
	case yaml.AliasNode:
		copy.Alias = filterFields(n.Alias, fields)
	case yaml.SequenceNode:
		copy.Content = nil
		for _, v := range n.Content {
			copy.Content = append(copy.Content, filterFields(v, fields))
		}
	case yaml.MappingNode:
		copy.Content = nil
		for i := 0; i < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Tag == "!!merge" {
				copy.Content = append(copy.Content, key, filterFields(value, fields))
				continue
			}
			if _, known := fields[key.Value]; known && key.Kind == yaml.ScalarNode && key.Tag == "!!str" {
				copy.Content = append(copy.Content, key, value)
			}
		}
	}
	return &copy
}
