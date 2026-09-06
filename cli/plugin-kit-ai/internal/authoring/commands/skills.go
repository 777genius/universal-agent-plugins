package commands

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/spf13/cobra"
)

// Each factory owns the group, children, flags and report capture. Only these
// two implemented standard-package jobs are mounted; there are no v1 wrappers.
func (a App) skillsCommand(capture func(report.Report)) (*cobra.Command, error) {
	group := &cobra.Command{Use: "skills", Short: "Create and validate standard package Skills", Args: cobra.NoArgs}
	group.RunE = func(c *cobra.Command, _ []string) error {
		if _, err := authoringcli.AdaptFlags(c, authoringcli.Support{Format: true, NoColor: true}); err != nil {
			return err
		}
		return c.Help()
	}
	for _, verb := range []string{"init", "validate"} {
		use, summary, args := "validate [package-path]", "Validate immediate Skills and package readiness using the embedded profile; no execution", cobra.MaximumNArgs(1)
		if verb == "init" {
			use, summary, args = "init <name> [package-path]", "Atomically add one Skill to an absent skills/<name> destination", cobra.RangeArgs(1, 2)
		}
		cmd, err := authoringcli.NewCommand(authoringcli.Spec[skills.Request, report.Report]{
			Use: use, Short: summary,
			Long: summary + "\n\nThe package path is the exact standard root containing plugin.json; omission uses only the current directory. No ancestor search. Description is required explicitly for init; no prompts, inferred authors or identity normalization. Validate reports package-wide readiness and isolated immediate Skills; scripts, references, assets and allowed-tools are not execution or authorization evidence.",
			Args: args, Support: authoringcli.Support{Format: true, NoColor: true},
			Configure: func(c *cobra.Command) {
				c.Flags().Bool("include-root", false, "disclose the explicitly selected package root")
				if verb == "init" {
					c.Flags().String("description", "", "required explicit Skill description (1–1024 characters)")
				}
				c.Example = "  plugin-kit-ai skills init docs-helper ./package --description 'Use for documentation requests'\n  agentplugins author skills validate ./package\n  plugin-kit-ai skills validate"
			},
			Decode: func(c *cobra.Command, _ authoringcli.Options, args []string) (skills.Request, error) {
				req := skills.Request{}
				req.IncludeRoot, _ = c.Flags().GetBool("include-root")
				if verb == "init" {
					req.Name, args = args[0], args[1:]
					req.Description, _ = c.Flags().GetString("description")
				}
				root := ""
				if len(args) > 0 {
					root = args[0]
				}
				var err error
				req.Root, err = skills.ExactRoot(root)
				return req, err
			},
			Runner: authoringcli.RunnerFunc[skills.Request, report.Report](func(ctx context.Context, req skills.Request) (report.Report, error) {
				s := skills.Service{Projects: a.Projects, Revision: a.Revision}
				var r report.Report
				var err error
				if verb == "init" {
					r, err = s.Init(ctx, req)
				} else {
					r, err = s.Validate(ctx, req)
				}
				if err != nil && r.Error == nil && !errors.Is(err, skills.ErrChecks) {
					phase := "read"
					if verb == "init" {
						phase = "apply"
					}
					code, action := failure(err, phase)
					r.AddError(code, action)
				}
				if err != nil && r.Committed {
					if r.Error == nil {
						r.Error = &report.Error{Code: "skill_committed_checks_failed", Action: "The Skill was committed; resulting package checks failed or are incomplete. Inspect the package before retrying."}
					} else {
						r.Error.Action = "The Skill was committed. " + r.Error.Action
					}
				}
				return r, err
			}),
			Render: func(_ authoringcli.Streams, _ authoringcli.Options, r report.Report, _ error) error {
				capture(r)
				return nil
			},
		})
		if err != nil {
			return nil, err
		}
		group.AddCommand(cmd)
	}
	return group, nil
}
