package grok

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// Both prototype directions retain the same native lock and exact config
// transaction after capturing the original parent, before native inventory.
type enablementProfile struct {
	parent directoryidentity.Authority
	config *nativeconfig.ExactFile
	unlock func() error
}

func (recovery EnablementRecovery) openEnablementProfile(ctx context.Context) (*enablementProfile, error) {
	parent, err := captureEnablementParent(ctx, recovery.UserRoot)
	if err != nil {
		return nil, err
	}
	unlock, err := lockNativeEnablement(recovery.UserRoot)
	if err != nil {
		return nil, err
	}
	if err := directoryidentity.Revalidate(ctx, parent); err != nil {
		return nil, errors.Join(err, unlock())
	}
	file, err := recovery.Kernel.BeginExactFile(filepath.Join(recovery.UserRoot, "config.toml"))
	if err != nil {
		return nil, errors.Join(err, unlock())
	}
	if err := directoryidentity.Revalidate(ctx, parent); err != nil {
		return nil, errors.Join(err, file.Close(), unlock())
	}
	return &enablementProfile{parent: parent, config: file, unlock: unlock}, nil
}

func (profile *enablementProfile) close() error {
	return errors.Join(profile.config.Close(), profile.unlock())
}
