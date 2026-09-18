package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type groupSession struct {
	service            Service
	ctx                context.Context
	input              GroupInput
	replace            bool
	groupID            string
	state              domain.StateFileV2
	first              AddInput
	sourceID           string
	installationIndex  int
	existing           bool
	installationID     string
	lifecycleBaseline  *domain.Installation
	result             GroupResult
	physical           map[string]int
	collisionKey       string
	planned            []plannedGroupTarget
	desired            domain.StateFileV2
	externalCompleted  int
	externalFailed     int
	firstActivationErr error
}

func (session *groupSession) validateGroupInput() error {
	for _, target := range append(append([]AddInput(nil), session.input.Targets...), session.input.CompatibilityChecks...) {
		if err := target.InstallIntent.Validate(target.Client.ClientID); err != nil {
			return err
		}
	}
	if len(session.input.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	if session.service.StateStore == nil || session.service.Paths == nil || session.service.Planner == nil || session.service.Stager == nil || session.service.Activator == nil {
		return fmt.Errorf("agentplugins group dependencies are incomplete")
	}
	return nil
}

func (session *groupSession) ensureGroupID() error {
	groupID := strings.TrimSpace(session.input.OperationGroupID)
	if groupID == "" {
		var err error
		groupID, err = newOperationID()
		if err != nil {
			return err
		}
	}
	session.groupID = groupID
	return nil
}

func (session *groupSession) resolveGroupInstallation() error {
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	session.state = state
	first := session.input.Targets[0]
	if first.OriginMode == "" && first.DirectoryResolution != nil {
		first.OriginMode = domain.OriginModeDirectory
		session.input.Targets[0] = first
	}
	if err := validateOperationOrigin(first.OriginMode, first.DirectoryResolution); err != nil {
		return err
	}
	if first.Envelope.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("group accepts only standard Agent Plugins packages")
	}
	session.first = first
	session.sourceID = domain.ComputeSourceBindingID(first.Envelope.Source)
	installationIndex, existing, sticky, err := findStickyInstallation(state, first, session.sourceID)
	if err != nil {
		return err
	}
	session.installationIndex = installationIndex
	session.existing = existing
	session.installationID = strings.TrimSpace(first.InstallationID)
	if existing {
		if err := session.validateExistingGroupInstallation(sticky); err != nil {
			return err
		}
	} else if session.replace {
		return fmt.Errorf("update requires an existing installation")
	}
	if session.installationID == "" {
		session.installationID, err = domain.NewInstallationID()
		if err != nil {
			return err
		}
	}
	session.result = GroupResult{
		InstallationID:   session.installationID,
		OperationGroupID: session.groupID,
		Targets:          make([]AddResult, len(session.input.Targets)),
		Phase:            GroupPhasePlanned,
	}
	if session.input.Switch && session.lifecycleBaseline != nil {
		session.result.PluginData = switchPluginDataDecision(*session.lifecycleBaseline)
	}
	return nil
}

func (session *groupSession) validateExistingGroupInstallation(sticky bool) error {
	installation := session.state.Installations[session.installationIndex]
	baseline := installation
	session.lifecycleBaseline = &baseline
	session.installationID = installation.InstallationID
	if err := session.validateStickySource(installation, sticky); err != nil {
		return err
	}
	if installation.NeedsRebind {
		return fmt.Errorf("installation requires explicit rebind")
	}
	if session.input.Switch {
		return session.validateGroupSwitch(installation)
	}
	if !session.input.Repair {
		if err := validateDirectoryTransition(installation, session.first); err != nil {
			return err
		}
	}
	if session.replace && !session.input.Switch && !session.input.Repair && installation.OriginMode == domain.OriginModeDirect && immutableDirectGit(installation.Source) {
		return fmt.Errorf("direct full-SHA installations require explicit switch")
	}
	return session.validateAddedTargetPackage(installation)
}

func (session *groupSession) validateStickySource(installation domain.Installation, sticky bool) error {
	if !sticky || installation.Source.SourceBindingID == session.sourceID || session.input.Switch {
		return nil
	}
	sameDistribution := installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && session.first.DirectoryResolution != nil && installation.Directory.DistributionID == session.first.DirectoryResolution.DistributionID
	if sameDistribution {
		return nil
	}
	return fmt.Errorf("installation is source-sticky; use switch")
}

func (session *groupSession) validateAddedTargetPackage(installation domain.Installation) error {
	if session.replace {
		return nil
	}
	first := session.first
	if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && first.DirectoryResolution != nil && first.DirectoryResolution.DesiredReleaseSequence != installation.Directory.DesiredReleaseSequence {
		return fmt.Errorf("adding targets must retain recorded release sequence %d; update separately", installation.Directory.DesiredReleaseSequence)
	}
	if installation.Source.TreeDigest != first.Envelope.TreeDigest {
		return fmt.Errorf("adding targets must use recorded desired package bytes; update separately")
	}
	return nil
}

func (session *groupSession) validateGroupSwitch(installation domain.Installation) error {
	first := session.first
	if installation.DeclaredName != first.Envelope.Manifest.Name {
		return fmt.Errorf("switch must preserve manifest identity")
	}
	if installation.OriginMode == domain.OriginModeDirectory && normalizedOriginMode(first.OriginMode) == domain.OriginModeDirectory && installation.Directory != nil && first.DirectoryResolution != nil && installation.Directory.ProductID != first.DirectoryResolution.ProductID {
		return fmt.Errorf("switch must remain within one Directory product")
	}
	for otherIndex, other := range session.state.Installations {
		if otherIndex != session.installationIndex && other.Source.SourceBindingID == session.sourceID {
			return fmt.Errorf("switch source is already bound to installation %s", other.InstallationID)
		}
	}
	return nil
}
