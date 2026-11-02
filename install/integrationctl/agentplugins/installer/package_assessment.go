package installer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) assessSnapshot(ctx context.Context, snapshot domain.PackageSnapshot, supplied *Assessment) error {
	decisions := make([]Assessment, 0, 2)
	if e.cfg.Assess != nil {
		got, err := e.cfg.Assess(ctx, snapshot.Root, snapshot.TreeDigest)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrAssessmentRejected, err)
		}
		decisions = append(decisions, got)
	}
	if supplied != nil {
		decisions = append(decisions, *supplied)
	}
	if len(decisions) == 0 {
		if e.cfg.TrustedLocalPackages {
			return nil
		}
		return fmt.Errorf("%w: no evaluator, digest-bound host decision, or explicit trusted-local policy", ErrAssessmentRejected)
	}
	for _, got := range decisions {
		if got.TreeDigest != snapshot.TreeDigest {
			return fmt.Errorf("%w: assessment digest %s snapshot %s", ErrAssessmentRejected, got.TreeDigest, snapshot.TreeDigest)
		}
		if got.Outcome != AssessmentAllow {
			reason := got.Reason
			if reason == "" {
				reason = string(got.Outcome)
			}
			if reason == "" {
				reason = "unavailable"
			}
			return fmt.Errorf("%w: %s", ErrAssessmentRejected, reason)
		}
	}
	return nil
}

func (e *Engine) refuseRecordedDigestRewrite(installationID, desired string) error {
	if installationID == "" || desired == "" {
		return nil
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, installationID)
	if !ok {
		return nil
	}
	recorded := installation.Source.TreeDigest
	if recorded != "" && recorded != desired {
		return fmt.Errorf("%w: recorded digest %s desired %s", ErrUpdateRequired, recorded, desired)
	}
	return nil
}

func (e *Engine) refuseRepairRevisionRewrite(installationID, clientID, desired string) error {
	if installationID == "" || clientID == "" || desired == "" {
		return nil
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, installationID)
	if !ok {
		return nil
	}
	binding, _, ok := findBinding(installation, domain.ClientID(clientID))
	if !ok || binding.PackageRevision == nil || binding.PackageRevision.TreeDigest == "" {
		return nil
	}
	if binding.PackageRevision.TreeDigest != desired {
		return fmt.Errorf("%w: recorded digest %s desired %s", ErrUpdateRequired, binding.PackageRevision.TreeDigest, desired)
	}
	return nil
}

// reuseMatchingSourceIdentity keeps the recorded source binding when the new
// snapshot is the same logical source. Local CanonicalSource is a capture
// path, not revision identity: a same-bytes Add from a new directory must
// not become a source switch, and an authorized Update may change TreeDigest
// without rewriting SourceBindingID.
func (e *Engine) reuseMatchingSourceIdentity(installationID string, snapshot *domain.PackageSnapshot, allowDigestRewrite bool) {
	if installationID == "" || snapshot == nil || snapshot.TreeDigest == "" {
		return
	}
	state, err := e.store.Load()
	if err != nil {
		return
	}
	installation, ok := findInstall(state, installationID)
	if !ok || installation.Source.TreeDigest == "" {
		return
	}
	if !allowDigestRewrite && installation.Source.TreeDigest != snapshot.TreeDigest {
		return
	}
	if installation.Source.CanonicalSource != "" {
		snapshot.Source.CanonicalSource = installation.Source.CanonicalSource
	}
	snapshot.Source.SourceKind = installation.Source.SourceKind
	snapshot.Source.SourceURL = installation.Source.SourceURL
	snapshot.Source.ArchiveSHA256 = installation.Source.ArchiveSHA256
	if installation.Source.RequestedSource != "" {
		snapshot.Source.RequestedSource = installation.Source.RequestedSource
	}
	if installation.Source.Repository != "" {
		snapshot.Source.Repository = installation.Source.Repository
	}
	if installation.Source.PackageSubpath != "" {
		snapshot.Source.PackageSubpath = installation.Source.PackageSubpath
	}
	if installation.Source.ResolvedRevision != "" {
		snapshot.Source.ResolvedRevision = installation.Source.ResolvedRevision
	}
}

func wrapLifecycleError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if errors.Is(err, usecase.ErrProjectionRefreshRequired) ||
		strings.Contains(msg, "run update separately") ||
		strings.Contains(msg, "at a different revision; use update") ||
		strings.Contains(msg, "differs from the installed revision; use update") ||
		strings.Contains(msg, "use switch to change source") {
		return fmt.Errorf("%w: %w", ErrUpdateRequired, err)
	}
	if strings.Contains(msg, "is not bound to an existing installation") ||
		strings.Contains(msg, "resolved source is not bound to an installation") ||
		strings.Contains(msg, "plugin is not materialized") {
		return fmt.Errorf("%w: %w", ErrNotInstalled, err)
	}
	return err
}
