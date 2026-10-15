package installer

import (
	"context"
	"errors"
	"testing"
)

func TestCompletionAttestationRejectsRemoveAndGroupsBeforeEffects(t *testing.T) {
	for _, req := range []Request{
		{Operation: OpRemove, AuthComplete: true},
		{Operation: OpInstall, ActivationComplete: true, Targets: []ClientTarget{{ClientID: "codex"}}},
	} {
		var engine *Engine
		if _, err := engine.Prepare(context.Background(), req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected invalid request: %v", err)
		}
	}
}
