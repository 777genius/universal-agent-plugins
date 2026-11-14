package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type NotifyInput struct {
	Raw    json.RawMessage
	Client string
}

type NotifyOutcome struct{}

type notifyDTO struct {
	Client string `json:"client"`
}

func DecodeNotify(env runtime.Envelope) (any, string, error) {
	if len(env.Args) < 3 {
		return nil, "", fmt.Errorf("decode codex notify input: missing JSON payload argument")
	}
	raw := json.RawMessage(env.Args[2])
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, "", fmt.Errorf("decode codex notify input: empty JSON payload argument")
	}
	if err := runtime.ValidatePayloadSize(raw, "codex notify input"); err != nil {
		return nil, "", err
	}
	var dto notifyDTO
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, "", fmt.Errorf("decode codex notify input: %w", err)
	}
	return &NotifyInput{
		Raw:    raw,
		Client: dto.Client,
	}, "notify", nil
}

func EncodeNotifyOutcome(NotifyOutcome) runtime.Result {
	return runtime.Result{ExitCode: 0}
}

func EncodeNotify(v any) runtime.Result {
	out, ok := v.(NotifyOutcome)
	if !ok {
		return runtime.Result{ExitCode: 1, Stderr: "encode codex notify response: internal outcome type mismatch\n"}
	}
	return EncodeNotifyOutcome(out)
}
