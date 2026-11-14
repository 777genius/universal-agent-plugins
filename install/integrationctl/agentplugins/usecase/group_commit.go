package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) buildDesiredGroupState() {
	session.desired = session.state
	for _, target := range session.planned {
		if target.noChange {
			continue
		}
		session.upsertGroupTargetState(target)
	}
	session.restoreGroupOrigin()
}

func (session *groupSession) upsertGroupTargetState(target plannedGroupTarget) {
	session.desired, session.installationIndex = upsertPreparedInstallation(session.desired, session.installationIndex, session.existing, target.input, target.plan, session.installationID, target.clientBindingID, session.sourceID, session.service.now())
	session.existing = true
	installation := session.desired.Installations[session.installationIndex]
	client := installation.Clients[target.clientBindingID]
	if target.managed != nil {
		// Addressing a shared backend through another logical surface must not
		// rewrite the persisted physical binding identity (and thereby make its
		// deterministic binding ID inconsistent).
		client.ClientID = target.managed.ClientID
		client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.AffectedSurfaces...)
		client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.ClientID)
	}
	if sharesPhysicalBackend(target.input.Client.ClientID, target.plan.SelectedDelivery, target.input.Scope) {
		client.AffectedSurfaces = append(client.AffectedSurfaces, string(target.input.Client.ClientID))
		for _, sibling := range domain.BackendSiblingsForScope(target.input.Client.ClientID, target.input.Scope) {
			client.AffectedSurfaces = append(client.AffectedSurfaces, string(sibling))
		}
	}
	for _, resultIndex := range target.resultIndexes {
		client.AffectedSurfaces = append(client.AffectedSurfaces, string(session.input.Targets[resultIndex].Client.ClientID))
	}
	client.AffectedSurfaces = uniqueSortedSurfaces(client.AffectedSurfaces)
	if session.input.Repair && target.managed != nil {
		// A repair commit becomes authoritative before directory/receipt
		// finalization. Preserve prior authentication evidence in that commit
		// decision so a late failure cannot reset completed authentication.
		client.Authentication = preservedGroupAuthentication(target.plan.Authentication, target.managed.Authentication)
	}
	if target.dataReceipt.DataReceiptID != "" {
		if installation.DataReceipts == nil {
			installation.DataReceipts = map[string]domain.DataReceipt{}
		}
		installation.DataReceipts[target.dataReceipt.DataReceiptID] = target.dataReceipt
		client.DataReceiptID = target.dataReceipt.DataReceiptID
	} else if session.input.Switch && target.managed != nil {
		// A destination that does not consume PLUGIN_DATA must not sever the
		// retained receipt from the active binding. A later reverse switch
		// can therefore recover the same owned directory.
		client.DataReceiptID = target.managed.DataReceiptID
	}
	installation.DataRetained = false
	installation.OperationGroupID = session.groupID
	installation.Clients[target.clientBindingID] = client
	session.desired.Installations[session.installationIndex] = installation
}

func (session *groupSession) restoreGroupOrigin() {
	if session.input.Repair && session.lifecycleBaseline != nil {
		// Repair restores each binding's own recorded projection. It must not let
		// iteration order move the installation-wide desired release/source back
		// to whichever heterogeneous binding happened to be repaired last.
		installation := session.desired.Installations[session.installationIndex]
		installation.DeclaredName = session.lifecycleBaseline.DeclaredName
		installation.Source = session.lifecycleBaseline.Source
		installation.Package = session.lifecycleBaseline.Package
		installation.OriginMode = session.lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(session.lifecycleBaseline.Directory)
		session.desired.Installations[session.installationIndex] = installation
	}
	if session.input.Switch {
		installation := session.desired.Installations[session.installationIndex]
		first := session.first
		installation.Source = sourceBindingFromEnvelope(first.Envelope, session.sourceID)
		installation.OriginMode = normalizedOriginMode(first.OriginMode)
		installation.Directory = cloneDirectoryOrigin(first.DirectoryResolution)
		session.desired.Installations[session.installationIndex] = installation
		return
	}
	if session.input.Repair && session.lifecycleBaseline != nil {
		installation := session.desired.Installations[session.installationIndex]
		installation.Source = session.lifecycleBaseline.Source
		installation.Package = session.lifecycleBaseline.Package
		installation.OriginMode = session.lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(session.lifecycleBaseline.Directory)
		session.desired.Installations[session.installationIndex] = installation
	}
}
