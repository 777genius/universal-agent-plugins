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
	inputs *enablementInventoryInputs
}

func (recovery EnablementRecovery) openInventoryProfile(ctx context.Context, executable, home string) (*enablementProfile, error) {
	profile, err := recovery.openEnablementProfile(ctx)
	if err != nil {
		return nil, err
	}
	profile.inputs, err = captureEnablementInventoryInputs(ctx, profile, executable, recovery.ProjectRoot, home)
	if err != nil {
		return nil, errors.Join(err, profile.close())
	}
	return profile, nil
}

func (recovery EnablementRecovery) openEnablementProfile(ctx context.Context) (*enablementProfile, error) {
	parent, err := captureEnablementParent(ctx, recovery.UserRoot)
	if err != nil {
		return nil, err
	}
	return recovery.openHeldProfile(ctx, parent)
}

// Cold recovery must retain the recorded parent, including before the native
// lock or config is opened. A fresh capture cannot replace that authority.
func (recovery EnablementRecovery) openRecordedProfile(ctx context.Context, record enablementRecord) (*enablementProfile, error) {
	if err := record.validateParent(); err != nil {
		return nil, err
	}
	if record.UserRoot != recovery.UserRoot {
		return nil, errors.New("grok recovery User root differs from recorded authority")
	}
	return recovery.openHeldProfile(ctx, record.UserParentAuthority)
}

func (recovery EnablementRecovery) openHeldProfile(ctx context.Context, parent directoryidentity.Authority) (*enablementProfile, error) {
	if err := directoryidentity.Revalidate(ctx, parent); err != nil {
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
	var err error
	if profile.inputs != nil {
		err = profile.inputs.close()
	}
	return errors.Join(err, profile.config.Close(), profile.unlock())
}
