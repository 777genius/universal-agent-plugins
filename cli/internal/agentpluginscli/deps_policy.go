package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Existing local-policy callers keep their shape while the rule language,
// validation and evaluator have a single owner shared with enterprise policy.
type policyAllowList = domain.PolicyAllowList

type localDependencyPolicy struct{ domain.InstallPolicy }

func readLocalDependencyPolicy(path string) (localDependencyPolicy, error) {
	policy, err := enterprisepolicy.ReadFile(path)
	return localDependencyPolicy{InstallPolicy: policy}, err
}

func validPolicyHost(host string) bool { return domain.ValidPolicyHost(host) }

// Compatibility helpers retain the legacy host-inspection contract used by its
// regression tests. Production evaluation is owned by enterprisepolicy.
func lockedSourceHost(pin environment.LockedPackage) (string, error) {
	switch {
	case pin.Source.Local != nil:
		return "", errors.New("local Skill source cannot be admitted by a remote host policy")
	case pin.Source.Git != nil:
		if pin.Source.Git.URL != "" {
			return hostFromPolicyURL(pin.Source.Git.URL)
		}
		if pin.Source.Git.Repository != "" {
			return "github.com", nil
		}
	case pin.Source.Registry != nil:
		return "github.com", nil
	case pin.Source.Archive != nil:
		return hostFromPolicyURL(pin.Source.Archive.URL)
	case pin.Source.Web != nil:
		return hostFromPolicyURL(pin.Source.Web.URL)
	}
	return "", errors.New("unrecognized dependency source authority")
}

func hostFromPolicyURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "ssh") {
		return "", errors.New("dependency source is not a recognized HTTPS/SSH URL")
	}
	host := strings.ToLower(u.Hostname())
	if !domain.ValidPolicyHost(host) {
		return "", errors.New("dependency source has an invalid hostname")
	}
	return host, nil
}

func (policy localDependencyPolicy) evaluate(lock environment.Lock) error {
	snapshot := enterprisepolicy.Snapshot{Layers: []enterprisepolicy.Layer{{Layer: "explicit", Present: true, Policy: policy.InstallPolicy}}}
	for _, pin := range lock.Packages {
		if _, err := snapshot.Check(lockedPolicyRequest(pin, "")); err != nil {
			return fmt.Errorf("local dependency policy for %q: %w", domain.PolicySafeText(pin.Alias), err)
		}
	}
	return nil
}

// The explicit candidate is additive to automatic layers. One loaded snapshot
// evaluates the complete closure; the same fingerprint is checked after review.
func evaluateLocalDependencyPolicy(ctx context.Context, app App, path string, lock environment.Lock) (string, error) {
	if path == "" {
		_, err := checkEnterprisePolicyClosure(ctx, app, lock)
		return "", err
	}
	automatic, err := checkEnterprisePolicyClosure(ctx, app, lock)
	if err != nil {
		return "", err
	}
	files := enterprisepolicy.Files{Paths: []enterprisepolicy.Path{{Layer: "explicit", Path: path, Required: true}}}
	snapshot, err := files.Load(ctx)
	if err != nil {
		return "", err
	}
	policy := snapshot.Layers[0].Policy
	for _, pin := range lock.Packages {
		if err := evaluateExplicitPolicyPin(ctx, app, snapshot, policy, pin); err != nil {
			return "", fmt.Errorf("local dependency policy for %q: %w", domain.PolicySafeText(pin.Alias), err)
		}
	}
	if automatic != "" {
		return automatic + "/" + snapshot.Fingerprint, nil
	}
	return snapshot.Fingerprint, nil
}

func evaluateExplicitPolicyPin(ctx context.Context, app App, snapshot enterprisepolicy.Snapshot, policy domain.InstallPolicy, pin environment.LockedPackage) error {
	req := lockedPolicyRequest(pin, app.scope())
	if _, err := snapshot.Check(req); err != nil {
		return err
	}
	if !localPolicyInspectsEnvelope(policy) {
		// Target/scope rules need a complete request, but no content acquisition
		// is necessary when the policy does not constrain package components.
		req.Envelope = &domain.PackageEnvelope{Source: req.Source}
		_, err := snapshot.Check(req)
		return err
	}
	_, err := inspectEnterprisePolicyPin(ctx, app, pin, func(envelope domain.PackageEnvelope) error {
		req.Envelope = &envelope
		_, checkErr := snapshot.Check(req)
		return checkErr
	})
	return err
}

func localPolicyInspectsEnvelope(policy domain.InstallPolicy) bool {
	return len(policy.Components.Denied) > 0 || policy.MCP.AllowedTransports.Present ||
		policy.MCP.AllowHosts.Present || len(policy.MCP.DenyHosts) > 0
}

func checkLocalDependencyPolicy(ctx context.Context, app App, path string, lock environment.Lock) error {
	_, err := evaluateLocalDependencyPolicy(ctx, app, path, lock)
	return err
}
