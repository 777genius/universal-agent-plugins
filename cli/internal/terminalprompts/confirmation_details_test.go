package terminalprompts

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

func TestConfirmationAdapterForwardsOwnedDetailsAndPlainShowsThem(t *testing.T) {
	request := prompt.ConfirmationRequest{Title: "Apply?", Summary: []string{"One Skill"}, Details: []string{"/exact/owned/path"}}
	converted := confirmationRequest(request)
	converted.Summary[0] = "changed"
	if request.Summary[0] != "One Skill" || request.Details[0] != "/exact/owned/path" {
		t.Fatal("adapter aliases caller-owned details")
	}
	var output strings.Builder
	result, err := (PlainPrompter{Input: strings.NewReader("yes\n"), Output: &output}).Confirm(context.Background(), request)
	if err != nil || !result.Accepted || !strings.Contains(output.String(), "/exact/owned/path") {
		t.Fatalf("plain adapter hid details: %+v %v %q", result, err, output.String())
	}
}

type detailsForbiddenInput struct{ t *testing.T }

func (input detailsForbiddenInput) Read([]byte) (int, error) {
	input.t.Fatal("invalid details consumed prompt input")
	return 0, io.EOF
}

type detailsForbiddenOutput struct{ t *testing.T }

func (output detailsForbiddenOutput) Write([]byte) (int, error) {
	output.t.Fatal("invalid details rendered a question")
	return 0, io.ErrClosedPipe
}

func TestRichDetailsBudgetRefusalIsPreciseAndBeforeUI(t *testing.T) {
	p := HuhPrompter{Input: detailsForbiddenInput{t}, Output: detailsForbiddenOutput{t}, terminal: &installerui.Terminal{}}
	result, err := p.Confirm(context.Background(), prompt.ConfirmationRequest{Title: "Apply?", Details: make([]string, 129)})
	if result.Accepted || !errors.Is(err, prompt.ErrReviewTooLarge) || !errors.Is(err, installerui.ErrInvalidRequest) {
		t.Fatalf("oversized details lost precise refusal/original error: %+v %v", result, err)
	}
	_, err = p.Confirm(context.Background(), prompt.ConfirmationRequest{Title: "Apply?", Details: []string{string([]byte{255})}})
	if !errors.Is(err, installerui.ErrInvalidRequest) || errors.Is(err, prompt.ErrReviewTooLarge) {
		t.Fatalf("malformed details were mistaken for a budget fallback: %v", err)
	}
}
