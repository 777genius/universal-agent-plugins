package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type switchSession struct {
	ctx          context.Context
	cmd          *cobra.Command
	app          App
	opts         *options
	selector     string
	source       string
	installation domain.Installation
	loaded       loadedPackage
	detected     map[domain.ClientID]domain.DetectedClient
	output       switchOutput
}

func runSwitch(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector, source string) error {
	session := &switchSession{ctx: ctx, cmd: cmd, app: app, opts: opts, selector: selector, source: source}
	if err := session.load(); err != nil {
		return err
	}
	if session.loaded.cleanup != nil {
		defer func() { _ = session.loaded.cleanup() }()
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &session.loaded); err != nil {
		return err
	}
	session.initOutput()
	if session.installation.DataRetained && len(session.installation.Clients) == 0 {
		return session.runRetained()
	}
	return session.runGroup()
}

func (session *switchSession) load() error {
	if err := session.selectInstallation(); err != nil {
		return err
	}
	if err := session.preflightTargets(); err != nil {
		return err
	}
	return session.loadPackage()
}

func (session *switchSession) selectInstallation() error {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, session.selector)
	if err != nil {
		return err
	}
	session.installation = installation
	return nil
}

func (session *switchSession) preflightTargets() error {
	targetIDs := installationTargets(session.installation, string(domain.ScopeUser))
	if len(targetIDs) == 0 {
		return nil
	}
	_, detected, err := preflightSelectedTargets(
		session.ctx, session.app, targetIDs, nil,
		!session.opts.dryRun && isDirectorySelector(session.source),
		lifecycleInstallIntents(session.installation, session.opts.scope, nil),
	)
	if err != nil {
		return err
	}
	session.detected = detected
	return nil
}

func (session *switchSession) loadPackage() error {
	targetIDs := installationTargets(session.installation, string(domain.ScopeUser))
	loaded, err := session.app.loadPackageFor(session.ctx, session.source, packageResolutionRequest{
		Targets: targetIDs, Operation: domain.DirectoryInstall, Clients: session.detected,
	})
	if err != nil {
		return err
	}
	session.loaded = loaded
	if loaded.envelope.Manifest.Name != session.installation.DeclaredName {
		return fmt.Errorf("switch source manifest name %q does not match installed product %q", loaded.envelope.Manifest.Name, session.installation.DeclaredName)
	}
	return nil
}

func (session *switchSession) initOutput() {
	session.output = switchOutput{
		DryRun: true, Status: "planned", Source: publicPackageSource(session.loaded.envelope.Source),
		Revision: session.loaded.envelope.Source.ResolvedRevision, TreeDigest: session.loaded.envelope.TreeDigest,
		ManifestDigest: session.loaded.envelope.ManifestDigest, Directory: cloneDirectoryOrigin(session.loaded.directory),
	}
}

func (session *switchSession) runRetained() error {
	service := session.app.Lifecycle
	service.StateStore = session.app.StateStore
	planned, err := service.SwitchRetained(session.ctx, usecase.BindingChangeInput{
		Selector: session.installation.InstallationID, Envelope: session.loaded.envelope,
	}, session.loaded.origin, session.loaded.directory)
	session.output.Retained = &planned
	session.output.PluginData = planned.PluginData
	if err != nil {
		return session.failPreflight(err)
	}
	if session.opts.dryRun {
		return renderSwitchResult(session.cmd.OutOrStdout(), session.opts.format, session.output)
	}
	if err := session.renderHumanPlan(); err != nil {
		return err
	}
	applied, err := service.SwitchRetained(session.ctx, usecase.BindingChangeInput{
		Selector: session.installation.InstallationID, Envelope: session.loaded.envelope, Confirmed: true,
	}, session.loaded.origin, session.loaded.directory)
	session.output.DryRun, session.output.Retained = false, &applied
	session.output.PluginData = applied.PluginData
	status := "completed"
	if err != nil {
		status = "apply_failed"
	}
	return session.finish(err, status)
}

func (session *switchSession) runGroup() error {
	inputs, err := session.groupInputs()
	if err != nil {
		return err
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	service := lifecycleService(session.app, session.detected)
	planned, err := service.SwitchGroup(session.ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true, Switch: true})
	session.output.Group = &planned
	session.output.PluginData = planned.PluginData
	session.output.Targets = switchTargets(planned)
	if err != nil {
		return session.failPreflight(fmt.Errorf("switch preflight failed; no target was changed: %w", err))
	}
	if session.opts.dryRun {
		return renderSwitchResult(session.cmd.OutOrStdout(), session.opts.format, session.output)
	}
	if err := session.renderHumanPlan(); err != nil {
		return err
	}
	applied, err := service.SwitchGroup(session.ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, Confirmed: true, Switch: true})
	session.output.DryRun, session.output.Group = false, &applied
	session.output.PluginData = applied.PluginData
	session.output.Targets = switchTargets(applied)
	status := string(applied.Phase)
	if err != nil {
		status = groupFailureStatus(applied.Phase)
	}
	return session.finish(err, status)
}

func (session *switchSession) groupInputs() ([]usecase.AddInput, error) {
	targetIDs := installationTargets(session.installation, string(domain.ScopeUser))
	inputs := make([]usecase.AddInput, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		client := session.detected[targetID]
		clientPackage := cloneLoadedPackage(session.loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, targetID); err != nil {
			return nil, err
		}
		inputs = append(inputs, usecase.AddInput{
			Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser, Hints: clientPackage.hints,
			InstallationID: session.installation.InstallationID, BackendExecutable: backendExecutable(client, session.detected),
			OriginMode: session.loaded.origin, DirectoryResolution: cloneDirectoryOrigin(session.loaded.directory),
			DistributionSuspended: session.loaded.distributionSuspended, ReleaseRevoked: session.loaded.releaseRevoked,
		})
	}
	return inputs, nil
}

func (session *switchSession) failPreflight(err error) error {
	session.output.Status = "preflight_failed"
	if renderErr := renderSwitchResult(session.cmd.OutOrStdout(), session.opts.format, session.output); renderErr != nil {
		return renderErr
	}
	return err
}

func (session *switchSession) renderHumanPlan() error {
	if session.opts.format != "human" {
		return nil
	}
	return renderSwitchResult(session.cmd.OutOrStdout(), session.opts.format, session.output)
}

func (session *switchSession) finish(err error, status string) error {
	session.output.Status = status
	if renderErr := renderSwitchResult(session.cmd.OutOrStdout(), session.opts.format, session.output); renderErr != nil && err == nil {
		err = renderErr
	}
	return err
}
