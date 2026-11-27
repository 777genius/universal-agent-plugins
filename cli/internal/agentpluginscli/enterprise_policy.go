package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// A session belongs to one command/review, never to the management daemon.
// Loading is lazy so malformed install policy cannot prevent safe removal.
func (app App) withEnterprisePolicySession() App {
	if app.InstallationPolicy == nil {
		return app
	}
	return app.withEnterprisePolicyGate(enterprisepolicy.NewSession(app.InstallationPolicy))
}

func (app App) withEnterprisePolicyGate(gate ports.InstallationPolicy) App {
	app.InstallationPolicy = gate
	app.Lifecycle.InstallationPolicy = app.InstallationPolicy
	if app.Installer != nil {
		app.Installer = app.Installer.WithInstallationPolicy(app.InstallationPolicy)
	}
	return app
}

func (app App) checkEnterprisePolicy(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	if app.InstallationPolicy == nil {
		return "", nil
	}
	return app.InstallationPolicy.Check(ctx, request)
}

func enterprisePolicyRequest(loaded loadedPackage, scope domain.InstallScope, targets []domain.ClientID) domain.InstallPolicyRequest {
	git, archive := requestedPolicyPins(loaded.envelope.Source)
	return domain.InstallPolicyRequest{Envelope: &loaded.envelope, OriginMode: loaded.origin,
		Scope: scope, Targets: targets, GitPinned: git, ArchivePinned: archive}
}

// RequestedSource is original intent on imperative paths. Locked dependency
// paths supply their separate authoritative selector instead of this helper.
func requestedPolicyPins(source domain.SourceIdentity) (git, archive *bool) {
	spec, err := sourcespec.Parse(source.RequestedSource)
	if err != nil {
		return nil, nil
	}
	if spec.Git != nil {
		pinned := sourcespec.ValidCommit(spec.Git.Commit)
		git = &pinned
	}
	if spec.Archive != nil {
		pinned := sourcespec.ValidSHA256(spec.Archive.SHA256)
		archive = &pinned
	}
	return git, archive
}

func (app App) checkEnterpriseSource(ctx context.Context, source domain.SourceIdentity, origin domain.OriginMode) error {
	git, archive := requestedPolicyPins(source)
	_, err := app.checkEnterprisePolicy(ctx, domain.InstallPolicyRequest{Source: source,
		OriginMode: origin, Scope: app.scope(), GitPinned: git, ArchivePinned: archive})
	return err
}
