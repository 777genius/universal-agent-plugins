package enterprisepolicy

import (
	"context"
	"errors"
	"net/url"
	"strings"

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
	source, normalized, err := policyRequestSource(req)
	if policy.SchemaVersion == 1 && !legacySourceShape(source) {
		return sourceViolation("legacy policy requires a recognized HTTPS/SSH remote source")
	}
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

// Schema 1 accepted HTTPS/SSH URLs and GitHub repository coordinates, not SCP
// syntax. Keep that installed contract even though schema 2 recognizes SCP.
func legacySourceShape(source domain.SourceIdentity) bool {
	raw := source.SourceURL
	if raw == "" && source.WebSkill != nil {
		raw = source.WebSkill.URL
	}
	if raw == "" {
		return source.Repository != ""
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "ssh") && domain.ValidPolicyHost(strings.ToLower(u.Hostname()))
}

func repositoryConstrained(policy domain.InstallPolicy) bool {
	return policy.Sources.AllowRepositories.Present || len(policy.Sources.DenyRepositories) > 0
}

func sourceViolation(message string) []domain.InstallPolicyViolation {
	return []domain.InstallPolicyViolation{{Code: "sources.identity", Subject: "source", Message: message}}
}
