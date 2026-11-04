package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkillsReviewTimeoutGuidance(t *testing.T) {
	session := &skillsCatalogSession{}
	for _, cause := range []error{
		fmt.Errorf("source credential=private-secret: %w", context.DeadlineExceeded),
		fmt.Errorf("native client credential=private-secret: %w", context.DeadlineExceeded),
	} {
		var refusal *skillspicker.ReviewRefusal
		if !errors.As(session.reviewRefusal(cause), &refusal) {
			t.Fatal("expected public review refusal")
		}
		if refusal.Code != "REVIEW_TIMEOUT" || !refusal.Retryable {
			t.Fatalf("timeout contract changed: %+v", refusal)
		}
		if !strings.Contains(refusal.Message, "source or client check timed out") || !strings.Contains(refusal.Message, "prepare a new review") {
			t.Fatalf("missing actionable check timeout guidance: %q", refusal.Message)
		}
		if strings.Contains(refusal.Message, "private-secret") || strings.Contains(refusal.Message, "fewer Skills") || strings.Contains(refusal.Message, "Review exceeded") {
			t.Fatalf("unsafe or misleading timeout guidance: %q", refusal.Message)
		}
	}
}

func TestSkillsReviewTimeoutPreservesSpecificRefusal(t *testing.T) {
	// A target-facts failure can wrap a probe deadline. Keep the actionable
	// target diagnosis instead of replacing it with generic timeout guidance.
	cause := fmt.Errorf("private-secret: %w: %w", installer.ErrTargetFactsUnavailable, context.DeadlineExceeded)
	var refusal *skillspicker.ReviewRefusal
	if !errors.As((&skillsCatalogSession{}).reviewRefusal(cause), &refusal) || refusal.Code != "TARGET_UNAVAILABLE" {
		t.Fatalf("lost target diagnosis: %+v", refusal)
	}
	if strings.Contains(refusal.Message, "private-secret") {
		t.Fatalf("provider output leaked: %q", refusal.Message)
	}
}

func TestSkillsReviewForeignDestinationSafeGuidance(t *testing.T) {
	for _, cause := range []error{nativeconfig.ErrCollision, nativeconfig.ErrNotOwned, installer.ErrForeignSkillDestination} {
		// A lower-level adapter may embed arbitrary provider paths or secrets.
		unsafe := fmt.Errorf("private-secret: /private/home: %w", cause)
		var refusal *skillspicker.ReviewRefusal
		if !errors.As((&skillsCatalogSession{}).reviewRefusal(unsafe), &refusal) {
			t.Fatal("expected public review refusal")
		}
		if refusal.Code != "FOREIGN_DESTINATION" || refusal.Retryable {
			t.Fatalf("wrong collision guidance: %+v", refusal)
		}
		for _, expected := range []string{"does not own", "rename or remove", "will not overwrite"} {
			if !strings.Contains(refusal.Message, expected) {
				t.Fatalf("missing actionable collision guidance %q: %s", expected, refusal.Message)
			}
		}
		if strings.Contains(refusal.Message, "private-secret") || strings.Contains(refusal.Message, "/private/home") {
			t.Fatalf("private adapter diagnostics escaped to HTTP: %s", refusal.Message)
		}
	}
}

func TestSkillsReviewManagedSkillChanged(t *testing.T) {
	cause := fmt.Errorf("secret plugin path: %w", installer.ErrManagedSkillChanged)
	var refusal *skillspicker.ReviewRefusal
	if !errors.As((&skillsCatalogSession{}).reviewRefusal(cause), &refusal) || refusal.Code != "MANAGED_SKILL_CHANGED" {
		t.Fatalf("managed edit lost its safe guidance: %+v", refusal)
	}
	if strings.Contains(refusal.Message, "secret plugin path") || !strings.Contains(refusal.Message, "edited outside UAP") {
		t.Fatalf("unsafe or unhelpful managed-edits refusal: %+v", refusal)
	}
}
