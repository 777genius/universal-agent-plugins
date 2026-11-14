//go:build !linux

package grok

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func captureEnablementInventoryInputs(ctx context.Context, _ *enablementProfile, _, _, _ string) (*enablementInventoryInputs, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errEnablementInventoryInputs
}

func (*enablementInventoryInputs) revalidate(ctx context.Context, _ nativeconfig.FileSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errEnablementInventoryInputs
}
