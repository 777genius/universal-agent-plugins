package enterprisepolicy

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (f Files) Check(ctx context.Context, req domain.InstallPolicyRequest) (string, error) {
	snapshot, err := f.Load(ctx)
	if err != nil {
		return snapshot.Fingerprint, err
	}
	return snapshot.Check(req)
}

// Check evaluates all layers independently; every layer must admit the input.
// It is read-only and reusable for a whole closure under one consistent snapshot.
func (s Snapshot) Check(req domain.InstallPolicyRequest) (string, error) {
	var denied []domain.InstallPolicyViolation
	for _, layer := range s.Layers {
		if !layer.Present || layer.Policy.RestrictionCount() == 0 {
			continue
		}
		violations := checkLayer(layer.Policy, req)
		for _, violation := range violations {
			violation.Layer, violation.PolicyPath = domain.PolicySafeText(layer.Layer), domain.PolicySafeText(layer.Path)
			denied = append(denied, violation)
		}
	}
	if len(denied) > 0 {
		return s.Fingerprint, &domain.InstallPolicyError{Violations: denied}
	}
	return s.Fingerprint, nil
}

func checkLayer(policy domain.InstallPolicy, req domain.InstallPolicyRequest) []domain.InstallPolicyViolation {
	_, normalized, err := policyRequestSource(req)
	if errors.Is(err, errSourceMismatch) {
		return sourceViolation("requested origin disagrees with the actual package source")
	}
	if err != nil && policy.ConstrainsSource() {
		return sourceViolation("source identity cannot be evaluated under this policy")
	}
	if err == nil && repositoryConstrained(policy) && (normalized.facts.Kind == "git" || normalized.facts.Kind == "directory") && !normalized.repositoryKnown {
		return sourceViolation("Git repository authority is ambiguous or uses a nondefault port")
	}
	return domain.EvaluateInstallPolicy(policy, req, normalized.facts)
}

func repositoryConstrained(policy domain.InstallPolicy) bool {
	return policy.Sources.AllowRepositories.Present || len(policy.Sources.DenyRepositories) > 0
}

func sourceViolation(message string) []domain.InstallPolicyViolation {
	return []domain.InstallPolicyViolation{{Code: "sources.identity", Subject: "source", Message: message}}
}
