package report

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

// commandsAssessment reports command diagnostics as installer policy. They
// never pass through conformance facts: a command is an installer extension,
// so it must not change normative conformance or host safety, only readiness.
// Entry names stay opaque.
func (r *Report) commandsAssessment(pkg *domain.PackageEnvelope) Assessment {
	result := assessment(Pass)
	if pkg == nil {
		return result
	}
	for _, diagnostic := range pkg.Diagnostics {
		if diagnostic.Boundary != domain.BoundaryCommand {
			continue
		}
		id := r.add(Finding{Code: diagnostic.Code, Layer: "installer_policy", Rule: "installer/commands", Location: commandext.RelativeDirectory, ItemID: opaque("command:" + diagnostic.Item), Severity: string(diagnostic.Severity)})
		result.FindingIDs = append(result.FindingIDs, id)
		if diagnostic.Severity == domain.SeverityError {
			result.Status = Fail
		}
	}
	return result
}

// addCommandCompatibility handles the rows of commands a client withholds,
// either explicit-only ones it cannot enforce or every command when its
// registry entry withholds them. Withholding is the safe outcome, so it is a
// warning while the client still gets something; with nothing deliverable
// left it fails like any unsupported component.
func (r *Report) addCommandCompatibility(c planner.ClientCompatibility, component planner.ComponentCompatibility) bool {
	if !planner.CommandWithheld(component) {
		return false
	}
	deliverable := slices.ContainsFunc(c.Components, func(row planner.ComponentCompatibility) bool { return row.Support != domain.SupportUnsupported })
	severity := "warning"
	if !deliverable {
		severity = "error"
		r.Compatibility.Status = Fail
	}
	id := r.add(Finding{Code: "target_command_withheld", Layer: "compatibility", Rule: "planner/static-support", ItemID: opaque(string(c.ClientID) + ":command"), Severity: severity})
	r.Compatibility.FindingIDs = append(r.Compatibility.FindingIDs, id)
	return true
}
