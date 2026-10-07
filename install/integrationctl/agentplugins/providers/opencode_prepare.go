package providers

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// OpenCodeClientPreparation is the trusted host-preparation port for existing
// CLI lifecycle paths. New captures launch environment once at composition.
type OpenCodeClientPreparation struct {
	registry    *clients.Registry
	environment []string
	probe       func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error)
}

func NewOpenCodeClientPreparation(registry *clients.Registry) *OpenCodeClientPreparation {
	return &OpenCodeClientPreparation{registry: registry, environment: clientdetect.OpenCodeProbeEnvironment(), probe: clientdetect.ProbeOpenCodeTarget}
}

type preparedOpenCodeClient struct {
	*opencodehost.NativePrepared
	namespace  string
	authority  *domain.ProfileAuthority
	skills     bool
	transports []string
}

func (p *OpenCodeClientPreparation) PrepareClient(ctx context.Context, envelope domain.PackageEnvelope, client domain.DetectedClient, previous []domain.NativeObjectOwnership, executable string, processInert bool) (domain.OpenCodeHostAuthority, error) {
	if p.registry == nil {
		return nil, clients.ErrRegistryRequired
	}
	consumer, ok := clients.As[clients.OpenCodeHostProfileConsumer](p.registry, client.ClientID)
	if !ok || !consumer.UsesOpenCodeHostProfile() {
		return client.OpenCodeHost, nil
	}
	skills, transports := desiredOpenCodeNativeRequirements(envelope, client.ClientID)
	oldSkills, _ := consumer.OwnedOpenCodeNativeRequirements(previous)
	// Even a manifest-only install emits a versioned native projection. Its
	// codec selection is a desired effect and needs qualified host authority.
	if processInert {
		return offlineOpenCodeHost(client.OpenCodeHost, skills, transports)
	}
	if frozen, ok := client.OpenCodeHost.(*preparedOpenCodeClient); ok {
		return p.reuseOpenCodeHost(ctx, client, frozen, executable, skills || oldSkills, transports)
	}
	host, err := p.prepareOpenCodeTarget(ctx, client, executable, skills || oldSkills, transports)
	if err != nil {
		return nil, err
	}
	if err := validatePreviousOpenCodeCodec(host, previous); err != nil {
		return nil, err
	}
	return host, nil
}

func desiredOpenCodeNativeRequirements(envelope domain.PackageEnvelope, id domain.ClientID) (bool, []string) {
	// Only supported desired effects enter qualification. Historical unsupported
	// entries remain available to the existing selection and removal policy.
	definition, _ := domain.ClientDefinitionFor(id)
	var transports []string
	for _, server := range envelope.MCP.Servers {
		if support := definition.Capabilities.MCPTransports[server.Type]; support != "" && support != domain.SupportUnsupported {
			transports = append(transports, server.Type)
		}
	}
	slices.Sort(transports)
	return len(envelope.Skills) > 0, transports
}

func offlineOpenCodeHost(host domain.OpenCodeHostAuthority, skills bool, transports []string) (domain.OpenCodeHostAuthority, error) {
	if host == nil {
		return nil, fmt.Errorf("OpenCode prepared profile is required for offline planning; prepare an explicit qualified host first")
	}
	if err := host.ValidateNative(skills, transports); err != nil {
		return nil, err
	}
	return host, nil
}

func (p *OpenCodeClientPreparation) reuseOpenCodeHost(ctx context.Context, client domain.DetectedClient, frozen *preparedOpenCodeClient, executable string, skills bool, transports []string) (domain.OpenCodeHostAuthority, error) {
	if executable != "" && executable != client.ExecutablePath {
		return nil, clientdetect.ErrProbeTargetChanged
	}
	if err := frozen.ValidateNative(skills, transports); err != nil {
		return nil, err
	}
	plan := (domain.DeliveryPlan{OpenCodeHost: frozen}).WithProfileAuthority(client.ProfileAuthority, client.ProfileNamespace)
	if err := p.RevalidateClient(ctx, client, plan); err != nil {
		return nil, err
	}
	return frozen, nil
}

func (p *OpenCodeClientPreparation) prepareOpenCodeTarget(ctx context.Context, client domain.DetectedClient, executable string, skills bool, transports []string) (*preparedOpenCodeClient, error) {
	if executable == "" {
		executable = client.ExecutablePath
	}
	if executable == "" || executable != client.ExecutablePath {
		return nil, fmt.Errorf("host_target_required: select an explicit detected OpenCode executable")
	}
	environment, err := clientdetect.CopyOpenCodeProbeEnvironment(p.environment)
	if err != nil {
		return nil, err
	}
	target := clientdetect.ProbeTarget{Executable: executable, Environment: environment}
	evidence, err := p.probe(ctx, target.Clone())
	if err != nil {
		return nil, err
	}
	if evidence.ProbeStatus != "ok" || evidence.Source != "executable_version" || evidence.ExecutableIdentity == "" {
		return nil, clientdetect.ErrUnverifiedProbeTarget
	}
	profile := opencodehost.Resolve(evidence.VersionEvidence)
	selections, err := opencodehost.SelectNative(profile, skills, transports)
	if err != nil {
		return nil, err
	}
	root := clientdetect.OpenCodeRootIdentity(client.ConfigRoot)
	if root == "" {
		return nil, fmt.Errorf("OpenCode user config root is unavailable")
	}
	return &preparedOpenCodeClient{NativePrepared: opencodehost.NewNativePrepared(executable, root, environment, evidence.VersionEvidence, profile, selections), namespace: client.ProfileNamespace, authority: domain.CloneProfileAuthority(client.ProfileAuthority), skills: skills, transports: slices.Clone(transports)}, nil
}

func validatePreviousOpenCodeCodec(host domain.OpenCodeHostAuthority, previous []domain.NativeObjectOwnership) error {
	codec, err := clients.DesiredOpenCodeCodec(host)
	if err != nil {
		return err
	}
	for _, object := range previous {
		stored, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return err
		}
		if mcp && stored != codec {
			return nativeconfig.ErrNativeMigrationRequired
		}
	}
	return nil
}

func (p *OpenCodeClientPreparation) RevalidateClient(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan) error {
	frozen, prepared := client.OpenCodeHost.(*preparedOpenCodeClient)
	if !prepared {
		return nil
	}
	host, ok := plan.OpenCodeHost.(*preparedOpenCodeClient)
	if !ok || host != frozen {
		return clientdetect.ErrProbeTargetChanged
	}
	executable, environment := host.Target()
	if executable != client.ExecutablePath || clientdetect.OpenCodeRootIdentity(client.ConfigRoot) != host.Root() || client.ProfileNamespace != host.namespace || !reflect.DeepEqual(client.ProfileAuthority, host.authority) || plan.ProfileNamespace() != host.namespace || !reflect.DeepEqual(plan.ProfileAuthority(), host.authority) {
		return clientdetect.ErrProbeTargetChanged
	}
	evidence, err := p.probe(ctx, clientdetect.ProbeTarget{Executable: executable, Environment: environment})
	if err != nil || evidence.VersionEvidence != host.Evidence() || clientdetect.OpenCodeRootIdentity(client.ConfigRoot) != host.Root() {
		return errors.Join(clientdetect.ErrProbeTargetChanged, err)
	}
	return nil
}

func (host *preparedOpenCodeClient) ValidateNative(skills bool, transports []string) error {
	if skills && !host.skills {
		return fmt.Errorf("OpenCode desired skill effects differ from prepared selection")
	}
	for _, transport := range transports {
		if !slices.Contains(host.transports, transport) {
			return fmt.Errorf("OpenCode desired MCP effects differ from prepared selection")
		}
	}
	return host.NativePrepared.ValidateNative(skills, transports)
}
