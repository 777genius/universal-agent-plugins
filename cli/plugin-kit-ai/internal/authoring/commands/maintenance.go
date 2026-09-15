package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/jsonmaint"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/nativeimport"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/spf13/cobra"
)

type normalizeRequest struct {
	root, document  string
	write, disclose bool
}

func (a App) normalizeCommand(capture func(report.Report)) (*cobra.Command, error) {
	return authoringcli.NewCommand(authoringcli.Spec[normalizeRequest, report.Report]{
		Use: "normalize [package-path]", Short: summary("normalize"), Args: cobra.MaximumNArgs(1),
		Support: authoringcli.Support{Format: true, NoColor: true},
		Configure: func(c *cobra.Command) {
			c.Flags().String("document", "", "selected standard document: plugin.json or mcp.json")
			c.Flags().Bool("write", false, "atomically replace the selected document")
			c.Flags().Bool("include-root", false, "disclose the explicitly selected root in this report")
		},
		Decode: func(c *cobra.Command, _ authoringcli.Options, args []string) (normalizeRequest, error) {
			document, _ := c.Flags().GetString("document")
			if document != "plugin.json" && document != "mcp.json" {
				return normalizeRequest{}, &inputError{"document_invalid", "Select exactly --document plugin.json or --document mcp.json."}
			}
			root := ""
			if len(args) == 1 {
				root = filepath.FromSlash(args[0])
			}
			exact, err := skills.ExactRoot(root)
			if err != nil {
				return normalizeRequest{}, err
			}
			write, _ := c.Flags().GetBool("write")
			disclose, _ := c.Flags().GetBool("include-root")
			return normalizeRequest{exact, document, write, disclose}, nil
		},
		Runner: authoringcli.RunnerFunc[normalizeRequest, report.Report](a.runNormalize),
		Render: func(_ authoringcli.Streams, _ authoringcli.Options, r report.Report, _ error) error {
			capture(r)
			return nil
		},
	})
}

func (a App) runNormalize(ctx context.Context, req normalizeRequest) (report.Report, error) {
	p, err := a.Projects.Read(ctx, req.root)
	r := report.Build("normalize", a.Revision, p, false)
	if req.disclose {
		r.Root = req.root
	}
	if err != nil {
		return operationFailed(r, err, "read")
	}
	if !r.Successful() {
		return operationFailed(r, errors.New("standard package validation failed"), "normalize")
	}
	document := p.Input.Plugin
	if req.document == "mcp.json" {
		document = p.Input.MCP
	}
	plan, err := jsonmaint.Build(req.document, document.Bytes)
	if err != nil {
		return operationFailed(r, err, "normalize")
	}
	r.JSONDocument = &report.JSONDocument{Path: plan.Document, BeforeSHA256: plan.BeforeSHA256, AfterSHA256: plan.AfterSHA256, Changed: plan.Changed()}
	if !req.write || !plan.Changed() {
		return r, nil
	}
	committed, err := jsonmaint.Apply(ctx, plan, jsonmaint.ApplyOptions{Root: req.root, Validate: func(ctx context.Context) error {
		validated, readErr := a.Projects.Read(ctx, req.root)
		if readErr == nil && !report.Build("normalize", a.Revision, validated, false).Successful() {
			readErr = errors.New("standard package validation failed")
		}
		return readErr
	}})
	r.Committed = committed
	if committed {
		r.Paths = []string{plan.Document}
	}
	if err != nil {
		return operationFailed(r, err, "normalize")
	}
	return r, nil
}

type nativeRequest struct {
	source, from, output, name, description string
	write                                   bool
}

func (a App) importCommand(capture func(report.Report)) (*cobra.Command, error) {
	group := &cobra.Command{Use: "import", Short: summary("import"), Args: cobra.NoArgs}
	native, err := authoringcli.NewCommand(authoringcli.Spec[nativeRequest, report.Report]{
		Use: "native <source-file>", Short: "Plan or write one explicit Claude MCP import", Args: cobra.ExactArgs(1),
		Support: authoringcli.Support{Format: true, NoColor: true},
		Configure: func(c *cobra.Command) {
			c.Flags().String("from", "", "native client: claude")
			c.Flags().String("output", "", "absolute absent package destination")
			c.Flags().String("name", "", "explicit standard plugin name")
			c.Flags().String("description", "", "explicit package description")
			c.Flags().Bool("write", false, "publish the validated package to the absent output")
		},
		Decode: func(c *cobra.Command, _ authoringcli.Options, args []string) (nativeRequest, error) {
			get := func(name string) string { value, _ := c.Flags().GetString(name); return value }
			write, _ := c.Flags().GetBool("write")
			req := nativeRequest{source: args[0], from: get("from"), output: get("output"), name: get("name"), description: get("description"), write: write}
			if req.from != "claude" || req.output == "" || !filepath.IsAbs(req.output) || req.name == "" || req.description == "" {
				return nativeRequest{}, &inputError{"import_arguments_invalid", "Use native <source-file> --from claude --output <absolute-absent-path> --name <plugin-name> --description <text> [--write]."}
			}
			return req, nil
		},
		Runner: authoringcli.RunnerFunc[nativeRequest, report.Report](a.runNativeImport),
		Render: func(_ authoringcli.Streams, _ authoringcli.Options, r report.Report, _ error) error {
			capture(r)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	group.AddCommand(native)
	return group, nil
}

func (a App) runNativeImport(ctx context.Context, req nativeRequest) (report.Report, error) {
	r := report.New("import native", a.Revision)
	plan, err := nativeimport.Build(ctx, req.source, req.from, req.name, req.description)
	if err != nil {
		return operationFailed(r, err, "import")
	}
	if err = nativeimport.ValidateOutput(plan, req.output); err != nil {
		return operationFailed(r, err, "import")
	}
	issues := func(values []nativeimport.Issue) []report.ImportIssue {
		out := make([]report.ImportIssue, len(values))
		for i, issue := range values {
			out[i] = report.ImportIssue{Code: issue.Code, ItemID: issue.ItemID}
		}
		return out
	}
	r.NativeImport = &report.NativeImport{Client: "claude", SourceSHA256: plan.SourceSHA256, SafeServers: plan.SafeServers, SkippedServers: issues(plan.SkippedServers), UnsupportedTopLevel: issues(plan.UnsupportedTopLevel)}
	var validatedReport report.Report
	if !req.write {
		return r, nil
	}
	if plan.SafeServers == 0 {
		return operationFailed(r, &nativeimport.Error{Code: "no_safe_servers"}, "import")
	}
	result, err := nativeimport.Apply(ctx, plan, req.output, func(ctx context.Context, stage string, dir *os.Root) error {
		p, readErr := a.Projects.ReadGeneratedStaging(ctx, stage, dir)
		validated := report.Build("import native", a.Revision, p, false)
		validatedReport = validated
		if readErr == nil && !validated.Successful() {
			readErr = errors.New("imported package validation failed")
		}
		return readErr
	})
	if result.Committed {
		details := r.NativeImport
		r = validatedReport
		r.NativeImport = details
	}
	r.Committed = result.Committed
	if result.Committed {
		for _, file := range plan.Package.Files() {
			r.Paths = append(r.Paths, file.Path)
		}
	}
	if err != nil {
		return operationFailed(r, err, "import")
	}
	return r, nil
}

func operationFailed(r report.Report, err error, phase string) (report.Report, error) {
	code, action := failure(err, phase)
	var jsonErr *jsonmaint.Error
	var importErr *nativeimport.Error
	switch {
	case errors.As(err, &jsonErr):
		code, action = jsonErr.Code, "Check the selected standard JSON document and retry from an unchanged valid package."
	case errors.As(err, &importErr):
		code, action = importErr.Code, "Check the explicit Claude source, safe stdio entries, identity, and absent output path."
	}
	r.AddError(code, action)
	return r, err
}
