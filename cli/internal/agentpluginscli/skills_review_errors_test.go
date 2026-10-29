package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
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
