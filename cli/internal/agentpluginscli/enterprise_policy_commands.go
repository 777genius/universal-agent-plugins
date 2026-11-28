package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
)

func newPolicyCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{
		Use: "policy", Short: "Configure and inspect installation restrictions",
		Long: "Inspect installation policy for sources, clients, scopes and package components.\nActive layers all apply; a local allow cannot override an organization restriction.\nThese rules govern UAP installation, not what an agent executes afterward.",
	}
	command.AddCommand(newPolicyInitCommand(opts), newPolicyValidateCommand(opts),
		newPolicyStatusCommand(app, opts), newPolicyCheckCommand(app, opts))
	return command
}

func newPolicyInitCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "init [path]", Short: "Create a commented policy template without overwriting a file",
		Long: "Create a schema-2 template. Restrictive examples are commented out.\nThe default is uap-policy.yaml in the selected project or current directory.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			path, err := policyInitPath(opts, args)
			report := enterprisePolicyReport{Outcome: "created", Path: path,
				Message: "Template created with no active restrictions. Uncomment the rules you need, then run policy validate and policy status."}
			if err == nil && opts.dryRun {
				err = previewPolicyTemplate(cmd.Context(), path)
				report.Outcome, report.Message = "planned", "A schema-2 template with no active restrictions would be created at the named path. No file was written."
			} else if err == nil {
				err = createPolicyTemplate(cmd.Context(), path)
			}
			if err != nil {
				report = policyFailureReport(report, err)
			}
			return finishPolicyCommand(cmd, opts, "init", report)
		},
	}
}

func policyInitPath(opts *options, args []string) (string, error) {
	path := "uap-policy.yaml"
	if len(args) != 0 {
		path = args[0]
	} else if opts.scope == "project" && opts.projectRoot != "" {
		path = filepath.Join(opts.projectRoot, path)
	}
	if path == "" {
		return "", CLIUsageError{Code: "POLICY_PATH_REQUIRED", Message: "Choose a nonempty destination path."}
	}
	return filepath.Abs(path)
}

func createPolicyTemplate(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return CLIUsageError{Code: "POLICY_FILE_EXISTS", Message: "The destination already exists. Choose another path or edit the existing policy."}
		}
		return CLIUsageError{Code: "POLICY_WRITE_FAILED", Message: "Cannot create the policy template. Check the parent directory and its permissions."}
	}
	_, writeErr := file.WriteString(enterprisePolicyTemplate)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return CLIUsageError{Code: "POLICY_WRITE_FAILED", Message: "The policy template could not be completely written. Inspect the destination before using it."}
	}
	return nil
}

func previewPolicyTemplate(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return CLIUsageError{Code: "POLICY_FILE_EXISTS", Message: "The destination already exists. Choose another path or edit the existing policy."}
		}
		return CLIUsageError{Code: "POLICY_WRITE_FAILED", Message: "Cannot inspect the policy template destination. Check its permissions."}
	}
	if parent, err := os.Stat(filepath.Dir(path)); err != nil || !parent.IsDir() {
		return CLIUsageError{Code: "POLICY_WRITE_FAILED", Message: "The policy template requires an existing parent directory."}
	}
	return nil
}

func newPolicyValidateCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "validate <path>", Short: "Validate one policy file without loading active policy or package sources",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			report := enterprisePolicyReport{Outcome: "valid", Path: args[0], Message: "Policy syntax and rule values are valid. This does not approve a package."}
			if err := cmd.Context().Err(); err != nil {
				return finishPolicyCommand(cmd, opts, "validate", policyFailureReport(report, err))
			}
			policy, err := enterprisepolicy.ReadFile(args[0])
			if err == nil {
				report.RestrictionCount = policy.RestrictionCount()
				report.Rules = enterprisePolicyRules(policy)
			} else {
				report = policyFailureReport(report, err)
			}
			return finishPolicyCommand(cmd, opts, "validate", report)
		},
	}
}

func newPolicyStatusCommand(app App, opts *options) *cobra.Command {
	var required bool
	command := &cobra.Command{
		Use: "status", Short: "Show active rules and every policy location checked",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected, err := app.selectScopeForOptions(opts)
			if err != nil {
				return err
			}
			files := enterpriseCommandFiles(selected, "")
			snapshot, err := files.Load(cmd.Context())
			report := policySnapshotReport(selected, files, snapshot)
			if err != nil {
				report = policyFailureReport(report, err)
			} else if required && snapshot.RestrictionCount == 0 {
				report.Outcome, report.Code = "unrestricted", "POLICY_REQUIRED"
				report.Message = "No active restriction is configured. Configure a policy before using --require-policy in CI."
			}
			return finishPolicyCommand(cmd, opts, "status", report)
		},
	}
	command.Flags().BoolVar(&required, "require-policy", false, "fail unless a valid policy has at least one active restriction")
	return command
}

func enterpriseCommandFiles(app App, candidate string) enterprisepolicy.Files {
	var files enterprisepolicy.Files
	if app.PolicyFiles != nil {
		files.Paths = append(files.Paths, app.PolicyFiles.Paths...)
	}
	if candidate != "" {
		files.Paths = append(files.Paths, enterprisepolicy.Path{Layer: "candidate", Path: candidate, Required: true})
	}
	return files
}

const enterprisePolicyTemplate = `# UAP installation policy. All configured layers apply together.
# Omitted allow-lists are unrestricted; an explicit [] allows nothing.
# Start with policy validate <path>, then policy check --policy <path>.
schema_version: 2

# sources:
#   allowed_kinds: [directory, git]
#   allow_hosts: [github.com]
#   deny_hosts: [unapproved.example.com]
#   allow_repositories: ["github.com/acme/*"]
#   deny_repositories: ["github.com/acme/retired-plugin"]

# integrity:
#   require_pinned_git: true
#   require_archive_sha256: true

# install:
#   allowed_targets: [codex, claude]
#   allowed_scopes: [project]

# components:
#   denied: [mcp, apps, extensions]

# mcp:
#   allowed_transports: [streamable-http]
#   allow_hosts: [mcp.example.com]
#   deny_hosts: [unapproved.example.com]
`

func policyCommandError(report enterprisePolicyReport) error {
	if report.Code != "" || report.Outcome == "denied" || report.Outcome == "incomplete" || report.Outcome == "error" {
		return fmt.Errorf("policy %s: %s", report.Outcome, report.Message)
	}
	return nil
}
