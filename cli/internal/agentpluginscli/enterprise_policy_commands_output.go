package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type enterprisePolicyRule struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type enterprisePolicyLayer struct {
	Layer            string                 `json:"layer"`
	Path             string                 `json:"path"`
	Required         bool                   `json:"required"`
	Status           string                 `json:"status"`
	SchemaVersion    int                    `json:"schema_version,omitempty"`
	RestrictionCount int                    `json:"restriction_count"`
	Rules            []enterprisePolicyRule `json:"rules,omitempty"`
}

type enterprisePolicyReport struct {
	Outcome          string                          `json:"outcome"`
	Coverage         string                          `json:"coverage,omitempty"`
	Scope            string                          `json:"scope,omitempty"`
	ProjectRoot      string                          `json:"project_root,omitempty"`
	Path             string                          `json:"path,omitempty"`
	Subject          string                          `json:"subject,omitempty"`
	Fingerprint      string                          `json:"fingerprint,omitempty"`
	RestrictionCount int                             `json:"restriction_count"`
	Layers           []enterprisePolicyLayer         `json:"layers,omitempty"`
	Rules            []enterprisePolicyRule          `json:"rules,omitempty"`
	Violations       []domain.InstallPolicyViolation `json:"violations,omitempty"`
	Checks           []dependencyCheckResult         `json:"checks,omitempty"`
	Code             string                          `json:"code,omitempty"`
	Message          string                          `json:"message"`
}

func policySnapshotReport(app App, files enterprisepolicy.Files, snapshot enterprisepolicy.Snapshot) enterprisePolicyReport {
	report := enterprisePolicyReport{Outcome: "active", Coverage: "configuration", Scope: string(app.scope()),
		ProjectRoot: app.ProjectRoot, Fingerprint: snapshot.Fingerprint, RestrictionCount: snapshot.RestrictionCount,
		Message: "Every active layer applies. An allow in one file cannot override a restriction in another."}
	if snapshot.RestrictionCount == 0 {
		report.Outcome, report.Message = "unrestricted", "No installation restriction is active."
	}
	for index, path := range files.Paths {
		layer := enterprisePolicyLayer{Layer: path.Layer, Path: path.Path, Required: path.Required, Status: "not_checked"}
		if index < len(snapshot.Layers) {
			loaded := snapshot.Layers[index]
			layer.Layer, layer.Path, layer.Status = loaded.Layer, loaded.Path, "absent"
			if loaded.Present {
				layer.Status, layer.SchemaVersion = "loaded", loaded.Policy.SchemaVersion
				layer.RestrictionCount, layer.Rules = loaded.Policy.RestrictionCount(), enterprisePolicyRules(loaded.Policy)
			}
		}
		report.Layers = append(report.Layers, layer)
	}
	return report
}

func policyFailureReport(report enterprisePolicyReport, err error) enterprisePolicyReport {
	report.Outcome, report.Code = "error", "POLICY_CHECK_FAILED"
	report.Message = "The policy check could not be completed. Inspect the selected source and policy files; no installation was authorized."
	var denied *domain.InstallPolicyError
	var config *enterprisepolicy.ConfigError
	var usage CLIUsageError
	switch {
	case errors.As(err, &denied):
		report.Outcome, report.Code = "denied", "POLICY_DENIED"
		report.Violations = safeEnterprisePolicyViolations(denied.Violations)
		report.Message = "Installation is denied. Remove the blocked dependency or ask the policy owner to update the named rule."
	case errors.As(err, &config):
		failure, _ := enterprisePolicyAgentFailure(err)
		report.Code, report.Message = failure.Code, enterprisePolicyFailureText(failure)
		for index := range report.Layers {
			if report.Layers[index].Path == config.Path {
				report.Layers[index].Status = "invalid"
			}
		}
	case errors.As(err, &usage):
		report.Code, report.Message = usage.Code, usage.Message
	case errors.Is(err, domain.ErrInstallPolicyChanged):
		report.Outcome, report.Code = "incomplete", "POLICY_CHANGED"
		report.Message = "Policy changed during inspection. Run the check again against the current files."
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		report.Code, report.Message = "POLICY_CHECK_CANCELED", "The policy check was canceled before a complete result was available."
	}
	return report
}

func finishPolicyCommand(cmd *cobra.Command, opts *options, action string, report enterprisePolicyReport) error {
	commandErr := policyCommandError(report)
	var renderErr error
	if opts.format == "json" {
		result := outputResultSuccess
		if commandErr != nil {
			result = outputResultFailure
		}
		renderErr = writeJSONResult(cmd.OutOrStdout(), "policy "+action, result, report)
	} else {
		renderErr = renderPolicyReport(cmd.OutOrStdout(), action, report)
	}
	return errors.Join(commandErr, renderErr)
}

func renderPolicyReport(writer io.Writer, action string, report enterprisePolicyReport) error {
	if _, err := fmt.Fprintf(writer, "Installation policy %s: %s\n%s\n", action, report.Outcome, prompt.SafeText(report.Message)); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{{"File", report.Path}, {"Project", report.ProjectRoot}, {"Package", report.Subject}, {"Coverage", report.Coverage}} {
		if field.value != "" {
			if _, err := fmt.Fprintf(writer, "%s: %s\n", field.name, prompt.SafeText(field.value)); err != nil {
				return err
			}
		}
	}
	for _, layer := range report.Layers {
		if _, err := fmt.Fprintf(writer, "  %s: %s (%s, %d restrictions)\n", prompt.SafeText(layer.Layer), prompt.SafeText(layer.Path), layer.Status, layer.RestrictionCount); err != nil {
			return err
		}
		if err := renderPolicyRules(writer, layer.Rules); err != nil {
			return err
		}
	}
	if err := renderPolicyRules(writer, report.Rules); err != nil {
		return err
	}
	return renderPolicyFindings(writer, report)
}

func renderPolicyFindings(writer io.Writer, report enterprisePolicyReport) error {
	for _, violation := range report.Violations {
		if _, err := fmt.Fprintf(writer, "  %s: %s\n    Rule: %s; policy: %s (%s)\n", prompt.SafeText(violation.Subject), prompt.SafeText(violation.Message),
			prompt.SafeText(violation.Code), prompt.SafeText(violation.PolicyPath), prompt.SafeText(violation.Layer)); err != nil {
			return err
		}
	}
	for _, check := range report.Checks {
		if check.Outcome != dependencyCheckValid {
			if err := renderDependencyCheckResult(writer, check); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderPolicyRules(writer io.Writer, rules []enterprisePolicyRule) error {
	for _, rule := range rules {
		if _, err := fmt.Fprintf(writer, "    %s: %v\n", rule.Name, rule.Value); err != nil {
			return err
		}
	}
	return nil
}

func enterprisePolicyRules(policy domain.InstallPolicy) []enterprisePolicyRule {
	var rules []enterprisePolicyRule
	if policy.SchemaVersion == 1 {
		rules = append(rules, enterprisePolicyRule{Name: "schema_1.remote_sources_only", Value: true})
	}
	for _, item := range []struct {
		name string
		list domain.PolicyAllowList
	}{
		{"sources.allowed_kinds", policy.Sources.AllowedKinds}, {"sources.allow_hosts", policy.Sources.AllowHosts},
		{"sources.allow_repositories", policy.Sources.AllowRepositories}, {"install.allowed_targets", policy.Install.AllowedTargets},
		{"install.allowed_scopes", policy.Install.AllowedScopes}, {"mcp.allowed_transports", policy.MCP.AllowedTransports},
		{"mcp.allow_hosts", policy.MCP.AllowHosts},
	} {
		if item.list.Present {
			rules = append(rules, enterprisePolicyRule{Name: item.name, Value: append([]string{}, item.list.Values...)})
		}
	}
	for _, item := range []struct {
		name   string
		values []string
	}{
		{"sources.deny_hosts", policy.Sources.DenyHosts}, {"sources.deny_repositories", policy.Sources.DenyRepositories},
		{"components.denied", policy.Components.Denied}, {"mcp.deny_hosts", policy.MCP.DenyHosts},
	} {
		if len(item.values) != 0 {
			rules = append(rules, enterprisePolicyRule{Name: item.name, Value: append([]string(nil), item.values...)})
		}
	}
	if policy.Integrity.RequirePinnedGit {
		rules = append(rules, enterprisePolicyRule{Name: "integrity.require_pinned_git", Value: true})
	}
	if policy.Integrity.RequireArchiveSHA256 {
		rules = append(rules, enterprisePolicyRule{Name: "integrity.require_archive_sha256", Value: true})
	}
	return rules
}
