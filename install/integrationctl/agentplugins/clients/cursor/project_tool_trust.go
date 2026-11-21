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
		outcome.Policy != domain.PolicyAllowed || outcome.Verification != domain.VerificationInstalled {
		return outcome, nil
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	if env.Snapshots == nil || request.Delivery.ArtifactDigest == "" {
		return outcome, fmt.Errorf("Cursor tool trust requires a verified immutable package digest")
	}
	// Read the publisher's reviewed list only from a sealed copy of the
	// receipt-bound package, never from a mutable active-path document.
	sealed, release, err := env.Snapshots.CaptureDirectory(ctx, request.Delivery.ActivePath)
	if err != nil || release == nil {
		return outcome, fmt.Errorf("Cursor tool trust package snapshot unavailable: %w", err)
	}
	digest, digestErr := env.Snapshots.Digest(ctx, sealed)
	approved, approvalErr := readCursorToolApprovalsForActivation(request, env.NativeConfig, sealed)
	closeErr := release()
	if digestErr != nil || digest != request.Delivery.ArtifactDigest || approvalErr != nil || closeErr != nil {
		return outcome, fmt.Errorf("Cursor reviewed tool approval package differs: %w", errors.Join(digestErr, approvalErr, closeErr, func() error {
			if digest != request.Delivery.ArtifactDigest {
				return fmt.Errorf("package digest mismatch")
			}
			return nil
		}()))
	}
	project := request.Plan.TargetContext.ProjectRoot
	if !cursorExactNativeProject(project) {
		return outcome, fmt.Errorf("Cursor CLI tool permissions require the exact selected Git project root")
	}
	path := filepath.Join(request.Plan.NativeRegistryRoot, "cli.json")
	if err := pathpolicy.RequireContainedChild(project, path); err != nil {
		return outcome, err
	}
	names, _ := cursorSelectedMCP(request.Plan.Components)
	verify := func() error {
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
		if effect.Effect == domain.NativeEffectUncertain {
			outcome.NativeEffect = domain.NativeEffectUncertain
		} else if effect.Effect == domain.NativeEffectCommitted {
			outcome.NativeEffect = domain.NativeEffectCommitted
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

func cursorMergeToolPermissions(body []byte, approved cursorToolApprovals) ([]byte, error) {
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("Cursor CLI permission policy exceeds the supported size")
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("preserved malformed Cursor CLI permission policy: %w", err)
	}
	var policy map[string]any
	if existing, found := document["permissions"]; found {
		var ok bool
		policy, ok = existing.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("preserved malformed Cursor CLI permissions")
		}
	} else {
		policy = map[string]any{}
	}
	allow, err := cursorExistingToolRules(policy, "allow")
	if err != nil {
		return nil, err
	}
	deny, err := cursorExistingToolRules(policy, "deny")
	if err != nil {
		return nil, err
	}
	ask, err := cursorExistingToolRules(policy, "ask")
	if err != nil {
		return nil, err
	}
	selected := make([]string, 0)
	for server, tools := range approved {
		for _, tool := range tools {
			rule := "Mcp(" + server + ":" + tool + ")"
			for _, policyRule := range append(slices.Clone(deny), ask...) {
				// A matching exact policy or unknown MCP wildcard must remain
				// authoritative. Never override it with a new allow entry.
				if strings.EqualFold(policyRule, rule) || policyRule == "*" || strings.EqualFold(policyRule, "Mcp") ||
					strings.HasPrefix(strings.ToLower(policyRule), "mcp(") && strings.ContainsAny(policyRule, "*?[]") {
					return nil, fmt.Errorf("preserved Cursor CLI deny/ask rule conflicts with selected MCP tool")
				}
			}
			if !slices.ContainsFunc(allow, func(existing string) bool { return strings.EqualFold(existing, rule) }) {
				selected = append(selected, rule)
			}
		}
	}
	if len(selected) == 0 {
		return body, nil
	}
	slices.Sort(selected)
	for _, rule := range selected {
		allow = append(allow, rule)
	}
	policy["allow"] = allow
	document["permissions"] = policy
	next, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(next, '\n'), nil
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
