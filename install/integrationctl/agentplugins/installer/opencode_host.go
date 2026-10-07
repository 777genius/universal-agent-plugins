package installer

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/hostprep"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// OpenCodeProbe is a trusted composition port. It receives a fresh target copy
// on each call; production defaults to the one bounded explicit-target facade.
type OpenCodeProbe = hostprep.Probe

var ErrHostTargetRequired = hostprep.ErrHostTargetRequired
var errOpenCodeTransportUnsupported = hostprep.ErrTransportUnsupported

func (e *Engine) prepareOpenCodeHost(ctx context.Context, handle *PreparedOperation) error {
	consumer, ok := clients.As[clients.OpenCodeHostProfileConsumer](e.cfg.Registry, handle.client.ClientID)
	if !ok || !consumer.UsesOpenCodeHostProfile() {
		return nil
	}
	skills, transports := clients.OpenCodeNativeRequirements(handle.envelope)
	previousEffects, ownedCodec, err := e.previousOpenCodeEffects(handle, consumer)
	if err != nil {
		return err
	}
	if !skills && len(transports) == 0 && !previousEffects {
		return nil
	}
	preparer, err := hostprep.New(e.cfg.Registry, e.cfg.OpenCodeProbe, e.cfg.OpenCodeProbeEnvironment)
	if err != nil {
		return err
	}
	host, err := preparer.Prepare(ctx, handle.req.ClientExecutable, handle.client.ConfigRoot, skills, transports)
	if err != nil {
		return err
	}
	// Preserve the facade's prior-effect and cross-profile cleanup fence.
	if ownedCodec != "" && len(transports) == 0 {
		selected, err := opencode.DesiredOpenCodeCodec(host)
		if err != nil {
			return err
		}
		if selected != ownedCodec {
			return nativeconfig.ErrNativeMigrationRequired
		}
	}
	profile := host.Profile()
	handle.openCodeHost = host
	handle.client.OpenCodeHost = host
	handle.client.Version = profile.Version
	handle.detected[handle.client.ClientID] = handle.client
	return nil
}

func (e *Engine) previousOpenCodeEffects(handle *PreparedOperation, consumer clients.OpenCodeHostProfileConsumer) (effects bool, ownedCodec nativeconfig.Codec, err error) {
	state, err := e.store.Load()
	if err != nil {
		return false, "", err
	}
	sourceID := domain.ComputeSourceBindingID(handle.envelope.Source)
	for _, installation := range state.Installations {
		if handle.req.InstallationID != "" {
			if installation.InstallationID != handle.req.InstallationID {
				continue
			}
		} else if installation.Source.SourceBindingID != sourceID && installation.DeclaredName != handle.envelope.Manifest.Name {
			continue
		}
		// Without an explicit installation ID, conservatively fence any source
		// or name match the existing lifecycle could select. Do not infer a target.
		binding, _, found := findBinding(installation, handle.client.ClientID)
		if !found {
			continue
		}
		ownedSkills, config := consumer.OwnedOpenCodeNativeRequirements(binding.NativeObjects)
		effects = effects || ownedSkills || config || e.cfg.OnCommittedBinding != nil && hostHandoffPending(binding)
		for _, object := range binding.NativeObjects {
			codec, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil {
				return false, "", err
			}
			if !mcp {
				continue
			}
			if ownedCodec != "" && ownedCodec != codec {
				return false, "", nativeconfig.ErrNativeMigrationRequired
			}
			ownedCodec = codec
		}
	}
	return effects, ownedCodec, nil
}

func (e *Engine) revalidateOpenCodeHost(ctx context.Context, handle *PreparedOperation) error {
	if handle.openCodeHost == nil {
		return nil
	}
	preparer, err := hostprep.New(e.cfg.Registry, e.cfg.OpenCodeProbe, e.cfg.OpenCodeProbeEnvironment)
	if err != nil {
		return err
	}
	if err := preparer.RevalidateOpenCodeHost(ctx, handle.client); err != nil {
		return ErrPlanChanged
	}
	return nil
}

func cloneOpenCodeSelections(in []opencodehost.Selection) []opencodehost.Selection {
	if in == nil {
		return nil
	}
	out := make([]opencodehost.Selection, len(in))
	for i, selection := range in {
		out[i] = selection.Clone()
	}
	return out
}

func openCodeRootIdentity(root string) string { return hostprep.RootIdentity(root) }
