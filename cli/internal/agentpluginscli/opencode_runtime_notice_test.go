package agentpluginscli

import (
	"bytes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
	"strings"
	"testing"
)

func TestOpenCodeRuntimeNoticeLifecycleOutputs(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		for _, state := range []string{"success", "nochange", "dryrun"} {
			for _, operation := range []string{"add", "update", "repair"} {
				t.Run(operation+"/"+format+"/"+state, func(t *testing.T) {
					result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "api/server", Support: domain.SupportPrepared}}}, Mutated: state == "success", NoChange: state == "nochange"}
					result.Plan.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityInfo, Code: openCodeNamespaceNotice, Message: "Initial global config server-name preflight passed; project and live tools were not checked."}}
					result.Activation = domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotRequired}
					var output bytes.Buffer
					var err error
					switch operation {
					case "add":
						err = renderAddResult(&output, format, domain.PackageEnvelope{}, result, state == "dryrun")
					case "update":
						err = renderUpdateResult(&output, format, domain.PackageEnvelope{}, result, state == "dryrun")
					case "repair":
						err = renderRepairResult(&output, format, domain.Installation{}, result, state == "dryrun")
					}
					if err != nil {
						t.Fatal(err)
					}
					if state == "success" && format == "human" && operation != "repair" && !strings.Contains(output.String(), "OpenCode MCP configuration") {
						t.Fatal(output.String())
					}
					if strings.Count(output.String(), openCodeNamespaceNotice) != 1 {
						t.Fatalf("notice missing or duplicated: %s", output.String())
					}
					if !strings.Contains(output.String(), "project and live tools were not checked") {
						t.Fatal(output.String())
					}
				})
			}
		}
	}
}

func TestOpenCodeRuntimeNoticeRequiresPreflightEvidence(t *testing.T) {
	result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "api/server", Support: domain.SupportPrepared}}}}
	var output bytes.Buffer
	if err := renderOpenCodeRuntimeNotice(&output, result); err != nil || output.Len() != 0 {
		t.Fatalf("unproven notice: %q, %v", output.String(), err)
	}
	result.Plan.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityInfo, Code: openCodeNamespaceNotice, Message: "preflight passed"}}
	if err := renderOpenCodeRuntimeNotice(&output, result); err != nil || !strings.Contains(output.String(), "Info: "+openCodeNamespaceNotice) {
		t.Fatalf("missing proven notice: %q, %v", output.String(), err)
	}
}

func TestOpenCodeRuntimeNoticeGroupedOutputs(t *testing.T) {
	result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "x", Support: domain.SupportPrepared}}}, NoChange: true}
	result.Plan.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityInfo, Code: openCodeNamespaceNotice, Message: "preflight passed"}}
	output := newAddResultData(domain.PackageEnvelope{}, result, false)
	update := updateMultiResult{Targets: []updateTargetResult{{Target: "opencode", Output: output}}}
	for _, format := range []string{"human", "json"} {
		for _, operation := range []string{"add", "update", "repair", "update-all"} {
			t.Run(operation+"/"+format, func(t *testing.T) {
				var buf bytes.Buffer
				cmd := &cobra.Command{}
				cmd.SetOut(&buf)
				opts := &options{format: format}
				var err error
				switch operation {
				case "add":
					err = renderAddMultiResult(cmd, opts, addMultiResult{Targets: []addTargetResult{{Target: "opencode", Output: output}, {Target: "codex"}}}, domain.PackageEnvelope{})
				case "update":
					err = renderUpdateMultiResult(cmd, opts, update)
				case "repair":
					err = renderRepairMultiResult(cmd, opts, repairMultiResult{Targets: []repairTargetResult{{Target: "opencode", Output: newRepairResultData(domain.Installation{}, result, false)}}})
				case "update-all":
					err = renderUpdateAll(cmd, opts, updateAllResult{Installations: []updateAllInstallation{{Name: "test", Plan: &update}}})
				}
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(buf.String(), openCodeNamespaceNotice) != 1 {
					t.Fatalf("notice missing or duplicated: %s", buf.String())
				}
			})
		}
	}
}
