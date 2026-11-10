package scaffold

import (
	"errors"
	"strings"
	"testing"
)

// Check the original first-error precedence when several fields are invalid.
// BuildPlan must reject before publishing a partial in-memory file tree.
func TestBuildPlanCombinedInvalidInputPrecedence(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*Options)
		want     string
		identity bool
	}{
		{"bounded input before identity", func(o *Options) {
			o.Runtime = strings.Repeat("x", 33)
			o.Name = "bad name"
		}, "bounded identifiers", false},
		{"identity before description", func(o *Options) {
			o.Name = "bad name"
			o.Description = ""
		}, "invalid plugin name", true},
		{"description before template", func(o *Options) {
			o.Description = " "
			o.Template = Hybrid
			o.MCPChoice = Skill
		}, "description must contain", false},
		{"template before runtime", func(o *Options) {
			o.Template = Hybrid
			o.MCPChoice = Skill
			o.Runtime = "python"
		}, "hybrid requires an explicit", false},
		{"transport before skill", func(o *Options) {
			o.Template = Hybrid
			o.MCPChoice = MCPRemote
			o.RemoteURL = "ftp://example.test"
			o.SkillName = "bad.name"
		}, "remote MCP requires", false},
		{"skill before license", func(o *Options) {
			o.SkillName = "bad.name"
			o.License = "Unknown"
		}, "invalid skill name", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := fixtureOptions("skill")
			tc.mutate(&o)
			p, err := BuildPlan(o)
			if err == nil || len(p.Files()) != 0 {
				t.Fatalf("invalid input produced a plan: %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("first error = %q, want %q", err, tc.want)
			}
			var identity *IdentityError
			if errors.As(err, &identity) != tc.identity {
				t.Fatalf("identity error kind changed: %v", err)
			}
			again, repeatErr := BuildPlan(o)
			if repeatErr == nil || repeatErr.Error() != err.Error() || len(again.Files()) != 0 {
				t.Fatalf("invalid planning is not deterministic: %v", repeatErr)
			}
		})
	}
}
