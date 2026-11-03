package agentpluginscli

import (
	"context"
	"errors"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyCheckOutcome string

const (
	dependencyCheckValid       dependencyCheckOutcome = "valid"
	dependencyCheckInvalid     dependencyCheckOutcome = "invalid"
	dependencyCheckUnavailable dependencyCheckOutcome = "unavailable"
	dependencyCheckError       dependencyCheckOutcome = "error"
	dependencyCheckCanceled    dependencyCheckOutcome = "canceled"
)

type dependencyCheckResult struct {
	Stage   string                 `json:"stage"`
	Outcome dependencyCheckOutcome `json:"outcome"`
	Code    string                 `json:"code,omitempty"`
	Alias   string                 `json:"alias,omitempty"`
	Path    string                 `json:"path,omitempty"`
	Message string                 `json:"message,omitempty"`
}

// Reuse the agreement transfer's stable, bounded file snapshots. Checking an
// agreement never invokes transfer publication or source acquisition.
type dependencyCheckedAgreement struct {
	manifest     dependencyTransferFile
	lockFile     dependencyTransferFile
	requirements environment.Requirements
	lock         environment.Lock
}

func inspectDependencyAgreement(ctx context.Context, app App) (dependencyCheckedAgreement, []dependencyCheckResult) {
	manifestPath, lockPath := app.dependencyPaths()
	agreement := dependencyCheckedAgreement{}
	var err error
	agreement.manifest, err = readDependencyTransferFile(manifestPath)
	if issue := dependencyDocumentReadIssue(ctx, "manifest", manifestPath, agreement.manifest, err); issue != nil {
		return agreement, []dependencyCheckResult{*issue}
	}
	agreement.requirements, err = readDependencyManifestBytes(manifestPath, agreement.manifest.body)
	if err != nil {
		return agreement, []dependencyCheckResult{dependencyInvalidDocument("manifest", manifestPath)}
	}
	checks := []dependencyCheckResult{{Stage: "manifest", Outcome: dependencyCheckValid, Path: manifestPath}}
	if err := validateDependencyTargetScopes(app, agreement.requirements); err != nil {
		return agreement, append(checks, dependencyCheckResult{Stage: "scope", Outcome: dependencyCheckError, Code: "scope_targets_unsupported", Path: manifestPath, Message: "The selected scope cannot inspect these declared targets; keep the agreement and choose a supported scope."})
	}
	var issue *dependencyCheckResult
	agreement.lock, agreement.lockFile, issue = inspectDependencyLockFile(ctx, lockPath, agreement.requirements)
	if issue != nil {
		return agreement, append(checks, *issue)
	}
	if err := canonicalizeRequirementsFromLock(&agreement.requirements, agreement.lock); err != nil {
		return agreement, append(checks, dependencyInvalidDocument("lock", lockPath))
	}
	return agreement, append(checks, dependencyCheckResult{Stage: "lock", Outcome: dependencyCheckValid, Path: lockPath})
}

// Status and strict check classify the same lock admission result. Status keeps
// its descriptive exit behavior; check additionally verifies cached facts.
func inspectDependencyLockFile(ctx context.Context, path string, requirements environment.Requirements) (environment.Lock, dependencyTransferFile, *dependencyCheckResult) {
	file, err := readDependencyTransferFile(path)
	if issue := dependencyDocumentReadIssue(ctx, "lock", path, file, err); issue != nil {
		return environment.Lock{}, file, issue
	}
	lock, err := decodeDependencyLockBytes(file.body, requirements, false)
	if err == nil {
		return lock, file, nil
	}
	issue := dependencyInvalidDocument("lock", path)
	if errors.Is(err, environment.ErrResolutionLimit) {
		issue.Outcome, issue.Code = dependencyCheckError, "graph_limit"
		issue.Message = "The recorded graph exceeds the supported inspection limits. Preserve the agreement and reduce its scope before retrying."
	} else if errors.Is(err, environment.ErrManifestDigestMismatch) {
		issue.Code = "lock_stale"
		issue.Message = "The lock describes different dependency intent. Review the manifest change before explicitly running deps lock."
	}
	return environment.Lock{}, file, &issue
}

func dependencyDocumentReadIssue(ctx context.Context, stage, path string, file dependencyTransferFile, err error) *dependencyCheckResult {
	if ctx.Err() != nil {
		issue := dependencyCanceledResult(stage)
		issue.Path = path
		return &issue
	}
	if err == nil && file.info != nil {
		return nil
	}
	issue := dependencyCheckResult{Stage: stage, Path: path}
	switch {
	case os.IsNotExist(err) || err == nil:
		issue.Outcome, issue.Code = dependencyCheckUnavailable, stage+"_missing"
		issue.Message = "The agreement file is absent. Restore the intended file; absence is not an empty dependency environment."
	case errors.Is(err, atomicfile.ErrFileChanged), errors.Is(err, installer.ErrPlanChanged):
		issue.Outcome, issue.Code = dependencyCheckError, "agreement_changed"
		issue.Message = "The agreement changed while it was being read. Retry after its writer has finished; no files were rewritten."
	case errors.Is(err, atomicfile.ErrUnsafeFile), errors.Is(err, atomicfile.ErrFileTooLarge):
		issue = dependencyInvalidDocument(stage, path)
	default:
		var pathError *os.PathError
		if errors.As(err, &pathError) {
			issue.Outcome, issue.Code = dependencyCheckError, stage+"_unreadable"
			issue.Message = "The agreement could not be read. Check file access without deleting or regenerating it."
		} else {
			issue = dependencyInvalidDocument(stage, path)
		}
	}
	return &issue
}

func dependencyInvalidDocument(stage, path string) dependencyCheckResult {
	return dependencyCheckResult{Stage: stage, Path: path, Outcome: dependencyCheckInvalid, Code: stage + "_invalid", Message: "The agreement failed bounded schema or identity validation. Inspect this file for invalid fields, conflicting pins or merge markers; resolve or restore it without automatic regeneration."}
}

func dependencyCanceledResult(stage string) dependencyCheckResult {
	return dependencyCheckResult{Stage: stage, Outcome: dependencyCheckCanceled, Code: "check_canceled", Message: "Inspection was canceled; local integrity was not established."}
}

func (agreement dependencyCheckedAgreement) recheck(ctx context.Context) *dependencyCheckResult {
	for _, item := range []struct {
		stage string
		file  dependencyTransferFile
	}{{"manifest", agreement.manifest}, {"lock", agreement.lockFile}} {
		current, err := readDependencyTransferFile(item.file.path)
		if issue := dependencyDocumentReadIssue(ctx, item.stage, item.file.path, current, err); issue != nil {
			return issue
		}
		if !sameDependencyTransferFile(item.file, current) || !item.file.info.ModTime().Equal(current.info.ModTime()) {
			return &dependencyCheckResult{Stage: item.stage, Path: item.file.path, Outcome: dependencyCheckError, Code: "agreement_changed", Message: "The agreement changed during inspection. Retry against a stable agreement; no files were rewritten."}
		}
	}
	return nil
}
