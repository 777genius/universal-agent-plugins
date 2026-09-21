package terminalprompts

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

// promptCancellation preserves the caller's cancellation cause for lifecycle
// safety while presenting an active prompt cancellation as a user action.
type promptCancellation struct {
	cause error
}

func (promptCancellation) Error() string { return prompt.ErrPromptCanceled.Error() }

func (err promptCancellation) Unwrap() []error {
	return []error{prompt.ErrPromptCanceled, err.cause}
}

func canceledPrompt(cause error) error {
	if cause == nil || errors.Is(cause, prompt.ErrPromptCanceled) {
		return prompt.ErrPromptCanceled
	}
	return promptCancellation{cause: cause}
}
