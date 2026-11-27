package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type installationPolicyReview struct {
	fingerprint string
}

func installationPolicyRequest(input AddInput) domain.InstallPolicyRequest {
	request := domain.InstallPolicyRequest{
		Source: input.Envelope.Source, OriginMode: input.OriginMode,
		Envelope: &input.Envelope, Scope: input.Scope,
		GitPinned: input.GitPinned, ArchivePinned: input.ArchivePinned,
	}
	if input.Client.ClientID != "" {
		request.Targets = []domain.ClientID{input.Client.ClientID}
	}
	return request
}

// Admit the complete operation before preparing any host. Preserve the first
// fingerprint through nested update/refresh calls rather than approving a new
// policy halfway through an operation.
func (service *Service) admitInstallationInputs(ctx context.Context, inputs []AddInput) ([]AddInput, error) {
	out := append([]AddInput(nil), inputs...)
	if service.InstallationPolicy == nil {
		return out, nil
	}
	var groupFingerprint string
	for i := range out {
		fingerprint, err := service.InstallationPolicy.Check(ctx, installationPolicyRequest(out[i]))
		if err != nil {
			return nil, err
		}
		if i > 0 && fingerprint != groupFingerprint {
			return nil, domain.ErrInstallPolicyChanged
		}
		groupFingerprint = fingerprint
		if prior := out[i].installationPolicyReview; prior != nil && prior.fingerprint != fingerprint {
			return nil, domain.ErrInstallPolicyChanged
		}
		out[i].installationPolicyReview = &installationPolicyReview{fingerprint: fingerprint}
	}
	service.installationPolicyInputs = out
	return out, nil
}

func (service Service) recheckInstallationPolicy(ctx context.Context) error {
	if service.InstallationPolicy == nil {
		return nil
	}
	for _, input := range service.installationPolicyInputs {
		fingerprint, err := service.InstallationPolicy.Check(ctx, installationPolicyRequest(input))
		if err != nil {
			return err
		}
		if input.installationPolicyReview == nil || input.installationPolicyReview.fingerprint != fingerprint {
			return domain.ErrInstallPolicyChanged
		}
	}
	return nil
}

func (service *Service) admitGroupPolicy(ctx context.Context, input *GroupInput) error {
	inputs := append(append([]AddInput(nil), input.Targets...), input.CompatibilityChecks...)
	admitted, err := service.admitInstallationInputs(ctx, inputs)
	if err != nil {
		return err
	}
	input.Targets, input.CompatibilityChecks = admitted[:len(input.Targets)], admitted[len(input.Targets):]
	return nil
}

func (session *groupSession) prepareGroupHosts() error {
	inputs := append(append([]AddInput(nil), session.input.Targets...), session.input.CompatibilityChecks...)
	prepared, err := session.service.prepareHostInputs(session.ctx, inputs, session.input.DryRun)
	if err != nil {
		return err
	}
	session.input.Targets, session.input.CompatibilityChecks = prepared[:len(session.input.Targets)], prepared[len(session.input.Targets):]
	return nil
}

func (session *groupSession) admitGroupMutation() error {
	if err := session.revalidateHosts(); err != nil {
		return err
	}
	return session.service.recheckInstallationPolicy(session.ctx)
}

func (session *groupSession) recheckGroupCommit() error {
	if err := session.reobserveGroupIdentity(); err != nil {
		return err
	}
	return session.service.recheckInstallationPolicy(session.ctx)
}

func (service *Service) admitBindingPolicy(ctx context.Context, input BindingChangeInput, origin domain.OriginMode) error {
	_, err := service.admitInstallationInputs(ctx, []AddInput{{
		Envelope: input.Envelope, OriginMode: origin, Scope: service.scope(),
		GitPinned: input.GitPinned, ArchivePinned: input.ArchivePinned,
	}})
	return err
}
