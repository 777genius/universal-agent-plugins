package cursor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This grants only reviewed, exact Cursor Agent tool permissions. It is not
// workspace trust, does not call native tools, and never edits Editor policy.
func completeCursorProjectToolTrust(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if !cursorHasToolApproval(request.Plan.Components) || request.VerifyOnly ||
		!request.ProjectTrustConsent || request.Plan.InstallIntent != domain.InstallIntentAutomatic ||
		outcome.Policy != domain.PolicyAllowed || outcome.Verification != domain.VerificationInstalled ||
		outcome.NativeEffect == domain.NativeEffectUncertain {
		return outcome, nil
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	project := request.Plan.TargetContext.ProjectRoot
	if !cursorExactNativeProject(project) {
		return outcome, fmt.Errorf("Cursor CLI tool permissions require the exact selected Git project root")
	}
	path := filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")
	if err := pathpolicy.RequireContainedChild(project, path); err != nil {
		return outcome, err
	}
	approved, err := cursorSealedToolApprovals(ctx, env, request)
	if err != nil {
		return outcome, err
	}
	names, _ := cursorSelectedMCP(request.Plan.Components)
	verify := func() error { return cursorValidateToolTrustSource(ctx, env, request, names) }
	if err := verify(); err != nil {
		return outcome, err
	}
	changed, err := shared.UpdateProjectTrustFile(env.NativeConfig, path, func(body []byte) ([]byte, error) {
		if err := verify(); err != nil {
			return nil, err
		}
		return cursorMergeToolPermissions(body, approved)
	})
	if changed {
		outcome.NativeEffect = domain.NativeEffectCommitted
	}
	var effect *shared.NativeEffectError
	if errors.As(err, &effect) {
		// An unchanged tool-permission rollback cannot erase an earlier
		// committed project MCP mutation. Uncertain always dominates.
		switch effect.Effect {
		case domain.NativeEffectUncertain:
			outcome.NativeEffect = domain.NativeEffectUncertain
		case domain.NativeEffectCommitted:
			if outcome.NativeEffect != domain.NativeEffectUncertain {
				outcome.NativeEffect = domain.NativeEffectCommitted
			}
		}
	}
	if err != nil {
		return outcome, err
	}
	if err := verify(); err != nil {
		return outcome, err
	}
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Cursor Agent exact project MCP tool permissions are installed; Editor workspace trust and signed-in tool execution remain independently unverified")
	return outcome, nil
}

func cursorSealedToolApprovals(ctx context.Context, env clients.Env, request domain.ActivationRequest) (cursorToolApprovals, error) {
	if env.Snapshots == nil || request.Delivery.ArtifactDigest == "" {
		return nil, fmt.Errorf("Cursor tool trust requires a verified immutable package digest")
	}
	sealed, release, err := env.Snapshots.CaptureDirectory(ctx, request.Delivery.ActivePath)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("Cursor approval snapshot has no cleanup ownership")
	}
	digest, digestErr := env.Snapshots.Digest(ctx, sealed)
	approved, approvalErr := readCursorToolApprovalsForActivation(request, env.NativeConfig, sealed)
	closeErr := release()
	if digestErr != nil || digest != request.Delivery.ArtifactDigest || approvalErr != nil || closeErr != nil {
		if digest != request.Delivery.ArtifactDigest {
			digestErr = errors.Join(digestErr, fmt.Errorf("Cursor package digest differs from reviewed source"))
		}
		return nil, errors.Join(digestErr, approvalErr, closeErr)
	}
	return approved, nil
}

func cursorValidateToolTrustSource(ctx context.Context, env clients.Env, request domain.ActivationRequest, names []string) error {
	digest, err := env.Snapshots.Digest(ctx, request.Delivery.ActivePath)
	if err != nil {
		return err
	}
	if digest != request.Delivery.ArtifactDigest {
		return fmt.Errorf("Cursor approval package changed after review")
	}
	if err := cursorComponents(request.Plan.NativeRegistryRoot).Verify(cursorNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig); err != nil {
		return err
	}
	return cursorMCPPolicyAvailable(env, request, names)
}

func cursorMergeToolPermissions(body []byte, approved cursorToolApprovals) ([]byte, error) {
	document, policy, allow, deny, ask, err := cursorReadToolPolicies(body)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, rule := range cursorRequestedToolRules(approved) {
		if cursorConflictsWithDeniedTool(rule, deny, ask) {
			return nil, fmt.Errorf("preserved Cursor CLI deny/ask rule conflicts with selected MCP tool")
		}
		if !slices.ContainsFunc(allow, func(existing string) bool { return strings.EqualFold(existing, rule) }) {
			allow = append(allow, rule)
			changed = true
		}
	}
	if len(allow) > 4096 {
		return nil, fmt.Errorf("Cursor CLI allow permission count would exceed reader limit")
	}
	if !changed {
		return body, nil
	}
	policy["allow"] = allow
	document["permissions"] = policy
	next, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(next)+1 > 1<<20 {
		return nil, fmt.Errorf("Cursor CLI permission policy would exceed reader limit")
	}
	return append(next, '\n'), nil
}

func cursorReadToolPolicies(body []byte) (map[string]any, map[string]any, []string, []string, []string, error) {
	if len(body) > 1<<20 {
		return nil, nil, nil, nil, nil, fmt.Errorf("Cursor CLI permission policy exceeds the supported size")
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("preserved malformed Cursor CLI permission policy: %w", err)
	}
	policy := map[string]any{}
	if existing, found := document["permissions"]; found {
		var ok bool
		policy, ok = existing.(map[string]any)
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("preserved malformed Cursor CLI permissions")
		}
	}
	allow, allowErr := cursorExistingToolRules(policy, "allow")
	deny, denyErr := cursorExistingToolRules(policy, "deny")
	ask, askErr := cursorExistingToolRules(policy, "ask")
	if err := errors.Join(allowErr, denyErr, askErr); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return document, policy, allow, deny, ask, nil
}

func cursorRequestedToolRules(approved cursorToolApprovals) []string {
	var rules []string
	for server, tools := range approved {
		for _, tool := range tools {
			rules = append(rules, "Mcp("+server+":"+tool+")")
		}
	}
	slices.Sort(rules)
	return rules
}

func cursorConflictsWithDeniedTool(rule string, deny, ask []string) bool {
	for _, policyRule := range append(slices.Clone(deny), ask...) {
		if strings.EqualFold(policyRule, rule) || policyRule == "*" || strings.EqualFold(policyRule, "Mcp") ||
			strings.HasPrefix(strings.ToLower(policyRule), "mcp(") && strings.ContainsAny(policyRule, "*?[]") {
			return true
		}
	}
	return false
}

func cursorExistingToolRules(policy map[string]any, key string) ([]string, error) {
	value, found := policy[key]
	if !found {
		return nil, nil
	}
	entries, ok := value.([]any)
	if !ok || len(entries) > 4096 {
		return nil, fmt.Errorf("preserved Cursor CLI %s permissions are malformed", key)
	}
	rules := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("preserved Cursor CLI %s permission is not a string", key)
		}
		rules = append(rules, name)
	}
	return rules, nil
}
