package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type listedPlugin struct {
	ID      string `json:"id"`
	Scope   string `json:"scope"`
	Enabled bool   `json:"enabled"`
}

func (a Adapter) Inspect(ctx context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	state, err := a.inspectState(ctx, in)
	if err != nil {
		return ports.InspectResult{}, err
	}
	return state.result(a.ID()), nil
}

func scopeForInspect(in ports.InspectInput) string {
	if in.Record != nil {
		return in.Record.Policy.Scope
	}
	return in.Scope
}
