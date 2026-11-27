package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func newPolicyCheckCommand(app App, opts *options) *cobra.Command {
	var candidate string
	var required bool
	command := &cobra.Command{
		Use: "check [source]", Short: "Check a declared dependency closure offline or inspect one package",
		Long: "Without a source, verify the selected manifest, lock, cached dependency graph and policy offline.\nWith a source, inspect that package for explicit --target clients; declared dependencies make\nthis package-only check incomplete. Sources may be fetched into temporary snapshots.\nChecks never install, prepare clients, grant trust or save authorization for a later command.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, err := app.selectScopeForOptions(opts)
			if err != nil {
				return err
			}
			if len(args) == 0 && opts.target != "" {
				return CLIUsageError{Code: "INVALID_TARGET", Message: "policy check without a source uses every declared target; omit --target"}
			}
			if cmd.Flags().Changed("policy") && candidate == "" {
				return CLIUsageError{Code: "POLICY_PATH_REQUIRED", Message: "--policy requires a nonempty path"}
			}
			files := enterpriseCommandFiles(selected, candidate)
			snapshot, err := files.Load(cmd.Context())
			report := policySnapshotReport(selected, files, snapshot)
			if err == nil && required && snapshot.RestrictionCount == 0 {
				err = CLIUsageError{Code: "POLICY_REQUIRED", Message: "Configure at least one active restriction before using --require-policy in CI."}
			}
			if err != nil {
				return finishPolicyCommand(cmd, opts, "check", policyFailureReport(report, err))
			}
			selected.InstallationPolicy = enterpriseCommandGate(selected, candidate, snapshot)
			selected.PolicyFiles = &files
			report = executePolicyCheck(cmd.Context(), selected, opts, args, report)
			if current, err := files.Load(cmd.Context()); err != nil {
				report = policyFailureReport(report, err)
			} else if current.Fingerprint != snapshot.Fingerprint {
				report.Outcome, report.Code = "incomplete", "POLICY_CHANGED"
				report.Message = "Policy changed during inspection. Run the check again against the current files."
			}
			return finishPolicyCommand(cmd, opts, "check", report)
		},
	}
	command.Flags().StringVar(&candidate, "policy", "", "additional candidate policy; active restrictions still apply")
	command.Flags().BoolVar(&required, "require-policy", false, "fail unless a valid policy has at least one active restriction")
	return command
}

func executePolicyCheck(ctx context.Context, app App, opts *options, args []string, report enterprisePolicyReport) enterprisePolicyReport {
	if len(args) == 0 {
		return checkDeclaredEnterprisePolicy(ctx, app, report)
	}
	report.Coverage = "package_only"
	targets, err := parseTargetOption(opts.target)
	if err == nil && len(targets) == 0 {
		err = CLIUsageError{Code: "TARGET_REQUIRED", Message: "policy check <source> requires explicit --target clients"}
	}
	if err != nil {
		return policyFailureReport(report, err)
	}
	preview, cleanup, err := dependencyPreviewApp(app, &options{dryRun: true})
	if err != nil {
		return policyFailureReport(report, err)
	}
	defer cleanup()
	subject, incomplete, err := inspectEnterprisePolicyPackage(ctx, preview, args[0], targets)
	report.Subject = subject
	if err != nil {
		return policyFailureReport(report, err)
	}
	report.Outcome, report.Code = "allowed", ""
	report.Message = "This package satisfies installation policy for the selected targets and scope. A later installation checks policy again."
	if incomplete {
		report.Outcome, report.Code = "incomplete", "DEPENDENCIES_NOT_CHECKED"
		report.Message = "The package declares dependencies. This package-only inspection has not approved them; use policy check on the verified declared closure."
	}
	return report
}

func inspectEnterprisePolicyPackage(ctx context.Context, app App, source string, targets []domain.ClientID) (subject string, incomplete bool, err error) {
	loaded, err := app.loadPackageFor(ctx, source, packageResolutionRequest{Targets: targets, Operation: domain.DirectoryInstall})
	if err != nil {
		return "", false, err
	}
	if loaded.cleanup != nil {
		defer func() { err = errors.Join(err, loaded.cleanup()) }()
	}
	subject = domain.PolicySafeText(loaded.envelope.Manifest.Name)
	if app.InstallationPolicy != nil {
		if _, err = app.InstallationPolicy.Check(ctx, enterprisePolicyRequest(loaded, app.scope(), targets)); err != nil {
			return subject, false, err
		}
	}
	requirements, err := packageDependencyFacts(loaded.envelope)
	return subject, len(requirements) != 0, err
}

func checkDeclaredEnterprisePolicy(ctx context.Context, app App, report enterprisePolicyReport) enterprisePolicyReport {
	report.Coverage = "declared_closure"
	if err := app.checkProjectNamespacePaths(); err != nil {
		return policyFailureReport(report, err)
	}
	agreement, checks := inspectDependencyAgreement(ctx, app)
	for _, check := range checks {
		if check.Outcome != dependencyCheckValid {
			return incompletePolicyClosure(report, checks)
		}
	}
	integrity := inspectLocalDependencies(ctx, app)
	if integrity.Outcome != dependencyCheckValid {
		return incompletePolicyClosure(report, integrity.Checks)
	}
	report.Checks = integrity.Checks
	if _, err := checkEnterprisePolicyClosure(ctx, app, agreement.lock); err != nil {
		return policyFailureReport(report, err)
	}
	if changed := agreement.recheck(ctx); changed != nil {
		return incompletePolicyClosure(report, append(report.Checks, *changed))
	}
	report.Outcome, report.Code = "allowed", ""
	report.Message = "The complete declared closure satisfies installation policy for its recorded targets and scope. No package source was fetched."
	return report
}

func incompletePolicyClosure(report enterprisePolicyReport, checks []dependencyCheckResult) enterprisePolicyReport {
	report.Outcome, report.Code = "incomplete", "CLOSURE_NOT_VERIFIED"
	report.Checks = checks
	report.Message = "The declared closure could not be verified offline. Resolve the reported agreement or cache issue before relying on a policy result."
	return report
}

type enterpriseCheckGate struct {
	active    ports.InstallationPolicy
	candidate enterprisepolicy.Files
	baseline  enterprisepolicy.Snapshot
}

func enterpriseCommandGate(app App, candidate string, snapshot enterprisepolicy.Snapshot) ports.InstallationPolicy {
	active := app.InstallationPolicy
	if active == nil && app.PolicyFiles != nil {
		active = app.PolicyFiles
	}
	gate := enterpriseCheckGate{active: active, baseline: snapshot}
	if candidate != "" {
		gate.candidate.Paths = []enterprisepolicy.Path{{Layer: "candidate", Path: candidate, Required: true}}
	}
	return enterprisepolicy.NewSession(gate)
}

func (gate enterpriseCheckGate) Check(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	// Keep the inspected restrictions authoritative even if a file is briefly
	// weakened and restored before the final file-fingerprint observation.
	if _, err := gate.baseline.Check(request); err != nil {
		return "", err
	}
	var activeFingerprint string
	var activeErr error
	if gate.active != nil {
		activeFingerprint, activeErr = gate.active.Check(ctx, request)
	}
	candidateFingerprint, candidateErr := gate.candidate.Check(ctx, request)
	if activeErr != nil || candidateErr != nil {
		return "", combineEnterprisePolicyErrors(activeErr, candidateErr)
	}
	digest := sha256.Sum256([]byte(activeFingerprint + "\x00" + candidateFingerprint))
	return hex.EncodeToString(digest[:]), nil
}

func combineEnterprisePolicyErrors(active, candidate error) error {
	var activeDenied, candidateDenied *domain.InstallPolicyError
	if errors.As(active, &activeDenied) && errors.As(candidate, &candidateDenied) {
		violations := append([]domain.InstallPolicyViolation(nil), activeDenied.Violations...)
		return &domain.InstallPolicyError{Violations: append(violations, candidateDenied.Violations...)}
	}
	return errors.Join(active, candidate)
}
