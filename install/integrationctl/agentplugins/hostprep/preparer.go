// Package hostprep shares bounded explicit-target preparation across lifecycle
// compositions. Identity, namespace and projection readers never probe hosts.
package hostprep

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

type Probe func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error)

var ErrHostTargetRequired = errors.New("host_target_required")
var ErrPlanChanged = errors.New("host_plan_changed")

type Preparer struct {
	probe       Probe
	environment []string
}

var _ ports.OpenCodeHostPreparer = (*Preparer)(nil)

// New pins a private launch environment once, at trusted composition.
func New(probe Probe, environment []string) (*Preparer, error) {
	if probe == nil {
		probe = clientdetect.ProbeOpenCodeTarget
	}
	if environment == nil {
		environment = clientdetect.OpenCodeProbeEnvironment()
	}
	env, err := clientdetect.CopyOpenCodeProbeEnvironment(environment)
	if err != nil {
		return nil, err
	}
	return &Preparer{probe: probe, environment: env}, nil
}

func (p *Preparer) Prepare(ctx context.Context, executable, root string, skills bool, transports []string) (*Snapshot, error) {
	if executable == "" {
		return nil, ErrHostTargetRequired
	}
	if !filepath.IsAbs(root) {
		return nil, clientdetect.ErrInvalidProbeTarget
	}
	target := clientdetect.ProbeTarget{Executable: executable, Environment: slices.Clone(p.environment)}
	evidence, err := p.probe(ctx, target.Clone())
	if err != nil {
		return nil, err
	}
	if evidence.ProbeStatus != "ok" || evidence.Source != "executable_version" || evidence.ExecutableIdentity == "" {
		return nil, errors.New("host_target_unverified")
	}
	profile := opencodehost.Resolve(evidence.VersionEvidence)
	selections, err := SelectNative(profile, skills, transports)
	if err != nil {
		return nil, err
	}
	host := newSnapshot(executable, RootIdentity(root), target.Environment, evidence.VersionEvidence, profile, selections)
	if err := host.ValidateNative(skills, transports); err != nil {
		return nil, err
	}
	return host, nil
}

func (p *Preparer) PrepareOpenCodeHost(ctx context.Context, client domain.DetectedClient, envelope domain.PackageEnvelope) (domain.DetectedClient, error) {
	if client.ClientID != domain.ClientOpenCode {
		return client, nil
	}
	// Use the canonical client capability declaration for desired effects;
	// historical unsupported components cannot require a host or grant authority.
	definition, _ := domain.ClientDefinitionFor(client.ClientID)
	selected := envelope
	selected.MCP.Servers = make(map[string]domain.MCPServer)
	for name, server := range envelope.MCP.Servers {
		support, ok := definition.Capabilities.MCPTransports[server.Type]
		if ok && support != domain.SupportUnsupported {
			selected.MCP.Servers[name] = server
		}
	}
	skills, transports := clients.OpenCodeNativeRequirements(selected)
	if !skills && len(transports) == 0 {
		client.OpenCodeHost = nil
		return client, nil
	}
	if retained, ok := client.OpenCodeHost.(*Snapshot); ok {
		if err := p.RevalidateOpenCodeHost(ctx, client); err != nil {
			return client, err
		}
		if err := client.OpenCodeHost.ValidateNative(skills, transports); err != nil {
			return client, err
		}
		client.Version = retained.Profile().Version
		return client, nil
	}
	host, err := p.Prepare(ctx, client.ExecutablePath, client.ConfigRoot, skills, transports)
	if err != nil {
		return client, err
	}
	client.OpenCodeHost = host
	client.Version = host.Profile().Version
	return client, nil
}

func (p *Preparer) RevalidateOpenCodeHost(ctx context.Context, client domain.DetectedClient) error {
	if client.OpenCodeHost == nil {
		return nil
	}
	host, ok := client.OpenCodeHost.(*Snapshot)
	if !ok {
		return ErrPlanChanged
	}
	executable, environment := host.Target()
	if executable != client.ExecutablePath || !slices.Equal(environment, p.environment) || RootIdentity(client.ConfigRoot) != host.Root() {
		return ErrPlanChanged
	}
	evidence, err := p.probe(ctx, clientdetect.ProbeTarget{Executable: executable, Environment: environment})
	if err != nil || evidence.VersionEvidence != host.Evidence() || RootIdentity(client.ConfigRoot) != host.Root() {
		return ErrPlanChanged
	}
	return nil
}

// Resolve existing ancestors too: missing config roots may lie beneath symlinks.
func RootIdentity(root string) string { return clientdetect.OpenCodeRootIdentity(root) }
