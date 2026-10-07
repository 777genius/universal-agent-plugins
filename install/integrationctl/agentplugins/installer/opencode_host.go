package installer

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// OpenCodeProbe is a trusted composition port. It receives a fresh target copy
// on each call; production defaults to the one bounded explicit-target facade.
type OpenCodeProbe func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error)

var ErrHostTargetRequired = errors.New("host_target_required")
var errOpenCodeTransportUnsupported = opencodehost.ErrNativeTransportUnsupported

func (e *Engine) prepareOpenCodeHost(ctx context.Context, handle *PreparedOperation) error {
	consumer, ok := clients.As[clients.OpenCodeHostProfileConsumer](e.cfg.Registry, handle.client.ClientID)
	if !ok || !consumer.UsesOpenCodeHostProfile() {
		return nil
	}
	skills, transports := clients.OpenCodeNativeRequirements(handle.envelope)
	_, ownedCodec, err := e.previousOpenCodeEffects(handle, consumer)
	if err != nil {
		return err
	}
	executable := handle.req.ClientExecutable
	if executable == "" {
		return ErrHostTargetRequired
	}
	target := clientdetect.ProbeTarget{Executable: executable, Environment: e.cfg.OpenCodeProbeEnvironment}
	evidence, err := e.cfg.OpenCodeProbe(ctx, target.Clone())
	if err != nil {
		return err
	}
	if evidence.ProbeStatus != "ok" || evidence.Source != "executable_version" || evidence.ExecutableIdentity == "" {
		return errors.New("host_target_unverified")
	}
	profile := opencodehost.Resolve(evidence.VersionEvidence)
	// Phase 1 cannot change stored ownership dialect, including cleanup of
	// the last declaration. Durable native/skill/state recovery is phase 2.
	if ownedCodec != "" {
		selected, err := opencode.DesiredOpenCodeCodec(newOpenCodePreparedHost(executable, openCodeRootIdentity(handle.client.ConfigRoot), target.Environment, evidence.VersionEvidence, profile, nil))
		if err != nil {
			return err
		}
		if ownedCodec != selected {
			return nativeconfig.ErrNativeMigrationRequired
		}
	}
	selections, err := selectOpenCodeNative(profile, skills, transports)
	if err != nil {
		return err
	}
	host := newOpenCodePreparedHost(executable, openCodeRootIdentity(handle.client.ConfigRoot), target.Environment, evidence.VersionEvidence, profile, selections)
	if err := host.ValidateNative(skills, transports); err != nil {
		return err
	}
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
	host := handle.openCodeHost
	if host == nil {
		return nil
	}
	executable, environment := host.Target()
	evidence, err := e.cfg.OpenCodeProbe(ctx, clientdetect.ProbeTarget{Executable: executable, Environment: environment})
	if err != nil || evidence.VersionEvidence != host.Evidence() || openCodeRootIdentity(handle.client.ConfigRoot) != host.Root() {
		return ErrPlanChanged
	}
	return nil
}

// Resolve existing ancestors too: a not-yet-created config root can be beneath
// a directory symlink. Creation of ordinary missing directories preserves it.
func openCodeRootIdentity(root string) string {
	return clientdetect.OpenCodeRootIdentity(root)
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
