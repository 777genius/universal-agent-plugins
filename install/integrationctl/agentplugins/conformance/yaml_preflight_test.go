package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"gopkg.in/yaml.v3"
)

func TestYAMLPreflightPlainScalarQuote(t *testing.T) {
	for _, quote := range []string{"'", "\""} {
		for _, depth := range []int{4, 100, 11000} {
			t.Run(fmt.Sprintf("%s/%d", quote, depth), func(t *testing.T) {
				limit := DefaultLimits().Depth
				if depth == 4 {
					limit = 3
				}
				extra := "future: ordinary-marker " + quote + "\ndeep: " + strings.Repeat("[", depth) + "x" + strings.Repeat("]", depth) + "\n"
				input := skillInput("good", extra)
				front, err := frontmatterBytes(input.Document.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				if depth < 11000 {
					var node yaml.Node
					if err := yaml.Unmarshal(front, &node); err != nil {
						t.Fatalf("invalid fixture: %v", err)
					}
				}
				var budget *parseFailure
				if err := preflightYAML(context.Background(), front, limit); !errors.As(err, &budget) || budget.Code != "yaml_depth_limit" {
					t.Errorf("preallocation guard: %v", err)
				}
				d := decoder(t)
				d.Limits.Depth = limit
				f, err := d.DecodeSkill(context.Background(), input)
				if err != nil || f.Coverage.Skills != NotEvaluated || !hasFinding(f, "yaml_depth_limit", HostSafety, domain.BoundarySkill) {
					t.Errorf("public boundary: coverage=%s findings=%+v error=%v", f.Coverage.Skills, f.Findings, err)
				}
				raw, err := json.Marshal(f)
				if err != nil || strings.Contains(string(raw), "ordinary-marker") {
					t.Fatal("public marker leak", err)
				}
			})
		}
	}
}

// Each shallow fixture must remain legal and pass; appending depth must be
// caught by the lexical guard, independently of the later node walk.
func TestYAMLPreflightScalarContexts(t *testing.T) {
	brackets := strings.Repeat("[", 80)
	for _, tc := range []struct{ name, extra string }{
		{"plain-single", "future: ordinary-marker '\n"},
		{"plain-double", "future: ordinary-marker \"\n"},
		{"plain-punctuation", "future: ordinary-marker - ? , | > ' \" " + brackets + "\n"},
		{"plain-url", "future: https://example.test/ordinary-marker#text\"\n"},
		{"plain-multiline", "future: ordinary-marker\n  \" " + brackets + "\n  continued '\n"},
		{"plain-newline-start", "future:\n  ordinary-marker\n  \" " + brackets + "\n"},
		{"plain-comment", "future: ordinary-marker ' # \" [\n# comment [\n"},
		{"plain-blank-line", "future: ordinary-marker\n\n  \" " + brackets + "\n"},
		{"mapping-boundary", "future: ordinary-marker '\nnext: 'opaque " + brackets + "'\n"},
		{"sequence-mapping", "future:\n  - first: ordinary-marker '\n    second: \"opaque " + brackets + "\"\n"},
		{"flow-map", "future: {first: ordinary-marker ', second: \"opaque " + brackets + "\"}\n"},
		{"flow-sequence", "future: [ordinary-marker \", 'opaque " + brackets + "']\n"},
		{"flow-multiline", "future: [ordinary-marker\n  \", 'opaque " + brackets + "']\n"},
		{"single-escape", "future: 'ordinary-marker '' " + brackets + "'\n"},
		{"double-escape", "future: \"ordinary-marker \\\" " + brackets + "\"\n"},
		{"quoted-multiline", "future: \"ordinary-marker\n# " + brackets + "\"\n"},
		{"quoted-key", "\"ordinary-marker\": \"" + brackets + "\"\n"},
		{"json-flow", "future: {\"ordinary-marker\":[\"" + brackets + "\"]}\n"},
		{"json-string", "future: {\"ordinary-marker\":\", ' " + brackets + "\"}\n"},
		{"tag-anchor-quote", "future: !!str &marker \"" + brackets + "\"\n"},
		{"verbatim-tag", "future: !<tag:yaml.org,2002:str> '" + brackets + "'\n"},
		{"literal", "future: |\n  \" " + brackets + "\n"},
		{"folded", "future: >-\n  ' " + brackets + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, deep := range []bool{false, true} {
				extra := tc.extra
				if deep {
					extra += "deep: " + strings.Repeat("[", 8) + "x" + strings.Repeat("]", 8) + "\n"
				}
				input := skillInput("good", extra)
				front, err := frontmatterBytes(input.Document.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				var node yaml.Node
				if err := yaml.Unmarshal(front, &node); err != nil {
					t.Fatalf("invalid fixture: %v", err)
				}
				err = preflightYAML(context.Background(), front, 6)
				var budget *parseFailure
				if deep {
					if !errors.As(err, &budget) || budget.Code != "yaml_depth_limit" {
						t.Errorf("deep preflight: %v", err)
					}
				} else if err != nil {
					t.Errorf("opaque preflight: %v", err)
				}
				d := decoder(t)
				d.Limits.Depth = 6
				f, err := d.DecodeSkill(context.Background(), input)
				want := Pass
				if deep {
					want = NotEvaluated
				}
				if err != nil || f.Coverage.Skills != want {
					t.Errorf("deep=%v coverage=%s findings=%+v error=%v", deep, f.Coverage.Skills, f.Findings, err)
				}
			}
		})
	}
}

func TestYAMLPreflightFlowBoundaries(t *testing.T) {
	for _, extra := range []string{
		"future: [ordinary-marker ', %s]\n",
		"future: {first: ordinary-marker \", deep: %s}\n",
		"future: [ordinary-marker\n  ', {deep: %s}]\n",
		"future:\n  - first: ordinary-marker '\n    deep: %s\n",
		"future: ordinary-marker '\ndeep: &marker %s\n",
		"future: ordinary-marker '\ndeep: !!seq %s\n",
	} {
		input := skillInput("good", fmt.Sprintf(extra, strings.Repeat("[", 8)+"x"+strings.Repeat("]", 8)))
		front, err := frontmatterBytes(input.Document.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(front, &node); err != nil {
			t.Fatalf("invalid fixture: %v", err)
		}
		var budget *parseFailure
		if err := preflightYAML(context.Background(), front, 6); !errors.As(err, &budget) || budget.Code != "yaml_depth_limit" {
			t.Errorf("preflight: %v", err)
		}
	}
}

func TestYAMLPreflightCancellation(t *testing.T) {
	input := skillInput("good", "future: ordinary-marker "+strings.Repeat("x", 20000)+"\n")
	front, err := frontmatterBytes(input.Document.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := preflightYAML(&countingContext{after: 4}, front, 64); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	d := decoder(t)
	f, err := d.DecodeSkill(&countingContext{after: 8}, input)
	if !errors.Is(err, context.Canceled) || len(f.Findings) != 0 {
		t.Fatalf("findings=%+v error=%v", f.Findings, err)
	}
	raw, marshalErr := json.Marshal(f)
	if marshalErr != nil || strings.Contains(string(raw), "ordinary-marker") {
		t.Fatal("public marker leak", marshalErr)
	}
}

func TestYAMLPreflightLowLimits(t *testing.T) {
	for _, depth := range []int{1, 2, 3} {
		extra := "future: ordinary-marker ' \"\n"
		if depth > 1 {
			extra += "deep: " + strings.Repeat("[", depth-1) + "x" + strings.Repeat("]", depth-1) + "\n"
		}
		input := skillInput("good", extra)
		front, err := frontmatterBytes(input.Document.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		if err := preflightYAML(context.Background(), front, depth); err != nil {
			t.Fatalf("exact depth %d: %v", depth, err)
		}
		d := decoder(t)
		d.Limits.Depth = depth
		f, err := d.DecodeSkill(context.Background(), input)
		if err != nil || f.Coverage.Skills != Pass {
			t.Fatalf("exact depth %d: %s %+v %v", depth, f.Coverage.Skills, f.Findings, err)
		}
		if depth > 1 {
			var budget *parseFailure
			if err := preflightYAML(context.Background(), front, depth-1); !errors.As(err, &budget) || budget.Code != "yaml_depth_limit" {
				t.Fatalf("over depth %d: %v", depth-1, err)
			}
		}
	}
}

func TestYAMLPreflightCompactMappingLimit(t *testing.T) {
	input := skillInput("good", "future:\n  - first: ordinary-marker '\n    \"deep\": [x]\n")
	front, err := frontmatterBytes(input.Document.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(front, &node); err != nil {
		t.Fatal(err)
	}
	if err := preflightYAML(context.Background(), front, 4); err != nil {
		t.Fatal(err)
	}
	var budget *parseFailure
	if err := preflightYAML(context.Background(), front, 3); !errors.As(err, &budget) || budget.Code != "yaml_depth_limit" {
		t.Fatalf("compact mapping preallocation: %v", err)
	}
}
