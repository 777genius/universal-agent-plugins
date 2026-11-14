package codex

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type codexInspectInputs struct {
	Scope         string
	IntegrationID string
	WorkspaceRoot string
	Paths         codexSurfacePaths
	Record        *domain.InstallationRecord
}

func (a Adapter) inspectInputs(in ports.InspectInput) codexInspectInputs {
	scope := scopeForInspect(in)
	workspaceRoot := workspaceRootFromInspectInput(in)
	paths := a.pathsForScope(scope, workspaceRoot, integrationIDForInspect(in.Record))
	if in.Record != nil {
		paths = a.pathsForRecord(*in.Record)
	}
	return codexInspectInputs{
		Scope:         scope,
		IntegrationID: integrationIDForInspect(in.Record),
		WorkspaceRoot: workspaceRoot,
		Paths:         paths,
		Record:        in.Record,
	}
}

func scopeForInspect(in ports.InspectInput) string {
	if in.Record != nil {
		return in.Record.Policy.Scope
	}
	return in.Scope
}

func integrationIDForInspect(record *domain.InstallationRecord) string {
	if record == nil {
		return ""
	}
	return strings.TrimSpace(record.IntegrationID)
}
