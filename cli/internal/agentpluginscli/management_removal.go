package agentpluginscli

import "github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"

// Adapter-derived requirements stay bound to the fresh retained action set. A
// host must explicitly re-prepare; confirming the blocked review cannot attest.
func (p *managementPrepared) reviewExternalUninstalled() error {
	if p.input.ExternalUninstalled && len(p.externalUninstalls) == 0 {
		return managementRefusal("EXTERNAL_UNINSTALL_NOT_APPLICABLE", "This review has no native removals requiring external confirmation; prepare it without externalUninstalled")
	}
	for _, action := range p.request.prepared {
		if !p.externalUninstalls[action.Alias+"\x00"+action.ClientID] {
			continue
		}
		diagnostic := management.Diagnostic{
			Code: "EXTERNAL_UNINSTALL_REQUIRED", Severity: management.DiagnosticSeverityError,
			Alias: action.Alias, TargetId: p.backend.targetID(action.ClientID),
			Message: "Remove this plugin in the native client, or confirm that it was never activated, then prepare a new review with externalUninstalled",
		}
		if p.input.ExternalUninstalled {
			diagnostic.Code, diagnostic.Severity = "EXTERNAL_UNINSTALL_ATTESTED", management.DiagnosticSeverityWarning
			diagnostic.Message = "The host explicitly confirmed that this plugin was removed in the native client or was never activated"
			p.review.Warnings = append(p.review.Warnings, diagnostic)
		} else {
			p.review.Confirmable = false
			p.review.Conflicts = append(p.review.Conflicts, diagnostic)
		}
	}
	return nil
}
