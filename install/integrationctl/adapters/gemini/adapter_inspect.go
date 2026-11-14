package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) Inspect(ctx context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	state, err := a.inspectState(ctx, in)
	if err != nil {
		return ports.InspectResult{}, err
	}
	return state.result(a.ID()), nil
}
