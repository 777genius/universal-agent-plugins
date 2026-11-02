package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func groupTargetFailureFromActivation(ctx context.Context, err error, outcome domain.ActivationOutcome) *GroupTargetFailure {
	stage := "activation"
	switch {
	case ctx.Err() != nil, errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		stage, _ = groupActivationStopReason(ctx, err)
	case outcome.Authentication == domain.AuthenticationFailed &&
		outcome.Activation != domain.ActivationFailed &&
		outcome.Verification != domain.VerificationFailed:
		stage = "authentication"
	case outcome.Verification == domain.VerificationFailed &&
		outcome.Activation != domain.ActivationFailed:
		// Providers often set VerificationFailed together with ActivationFailed.
		// Prefer activation unless verification is the only failed dimension.
		stage = "verification"
	}
	message := "activation failed"
	if err != nil {
		message = err.Error()
	}
	return &GroupTargetFailure{Stage: stage, Message: message}
}

func countNotAttempted(targets []AddResult) int {
	count := 0
	for _, target := range targets {
		if target.GroupPhase == GroupTargetExternalNotAttempted {
			count++
		}
	}
	return count
}

func preservedGroupAuthentication(planned, current domain.AuthenticationState) domain.AuthenticationState {
	if current == domain.AuthenticationComplete || planned == "" || planned == domain.AuthenticationNotChecked {
		return current
	}
	return planned
}

func switchPluginDataDecision(installation domain.Installation) domain.PluginDataDecision {
	decision := domain.PluginDataDecision{
		Disposition: domain.PluginDataNone, Ownership: domain.PluginDataOwnershipNone,
		Compatibility: domain.PluginDataCompatibilityNotApplicable,
	}
	if len(installation.DataReceipts) == 0 {
		return decision
	}
	decision.Disposition = domain.PluginDataRetained
	decision.Present = true
	decision.ReceiptCount = len(installation.DataReceipts)
	decision.Ownership = domain.PluginDataOwnershipOwned
	decision.Compatibility = domain.PluginDataCompatibilityNotProven
	decision.Warning = domain.PluginDataCompatibilityWarning
	for _, receipt := range installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned || receipt.OwnershipDigest == "" {
			decision.Ownership = domain.PluginDataOwnershipIndeterminate
			break
		}
	}
	return decision
}

func assignGroupReceipts(targets []AddResult, planned []plannedGroupTarget, receipts []domain.MutationReceipt) {
	byBinding := make(map[string]domain.MutationReceipt, len(receipts))
	for _, receipt := range receipts {
		byBinding[receipt.ClientBindingID] = receipt
	}
	for _, target := range planned {
		receipt, ok := byBinding[target.clientBindingID]
		if !ok {
			continue
		}
		for _, resultIndex := range target.resultIndexes {
			targets[resultIndex].Receipt = receipt
		}
	}
}

func containsSurface(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniqueSortedSurfaces(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func groupPackageUnchanged(binding domain.ClientBinding, input AddInput) bool {
	revision := binding.PackageRevision
	if !packageRevisionMatches(revision, input.Envelope) || revision.ResolvedRevision != input.Envelope.Source.ResolvedRevision {
		return false
	}
	if normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory {
		return input.DirectoryResolution != nil && revision.DistributionID == input.DirectoryResolution.DistributionID && revision.ReleaseSequence == input.DirectoryResolution.DesiredReleaseSequence
	}
	return true
}
