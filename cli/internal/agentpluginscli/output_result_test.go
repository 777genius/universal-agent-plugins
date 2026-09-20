package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestSwitchResultEnvelopeMatchesStructuredStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     string
		groupPhase usecase.GroupPhase
		wantResult string
	}{
		{name: "preflight failure", status: "preflight_failed", groupPhase: usecase.GroupPhasePlanned, wantResult: outputResultFailure},
		{name: "controlled rollback", status: string(usecase.GroupPhaseManagedRolledBack), groupPhase: usecase.GroupPhaseManagedRolledBack, wantResult: outputResultFailure},
		{name: "managed commit activation failure", status: string(usecase.GroupPhaseManagedActivationFailed), groupPhase: usecase.GroupPhaseManagedActivationFailed, wantResult: outputResultFailure},
		{name: "partial external outcome", status: string(usecase.GroupPhaseExternalPartialFailure), groupPhase: usecase.GroupPhaseExternalPartialFailure, wantResult: outputResultFailure},
		{name: "retained apply failure", status: "apply_failed", wantResult: outputResultFailure},
		{name: "real success", status: string(usecase.GroupPhaseCompleted), groupPhase: usecase.GroupPhaseCompleted, wantResult: outputResultSuccess},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := switchOutput{Status: test.status}
			if test.groupPhase != "" {
				result.Group = &usecase.GroupResult{Phase: test.groupPhase}
			} else {
				result.Retained = &usecase.BindingChangeResult{}
			}

			var jsonOutput bytes.Buffer
			if err := renderSwitchResult(&jsonOutput, "json", result); err != nil {
				t.Fatal(err)
			}
			assertOutputEnvelopeResult(t, jsonOutput.Bytes(), "switch", test.wantResult)
			var structured struct {
				Data switchOutput `json:"data"`
			}
			if err := json.Unmarshal(jsonOutput.Bytes(), &structured); err != nil {
				t.Fatal(err)
			}
			if structured.Data.Status != test.status {
				t.Fatalf("structured status = %q, want %q", structured.Data.Status, test.status)
			}

			var humanOutput bytes.Buffer
			if err := renderSwitchResult(&humanOutput, "human", result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(humanOutput.String(), "Switch: "+test.status) {
				t.Fatalf("human output = %q", humanOutput.String())
			}
			if test.wantResult != outputResultSuccess && strings.Contains(humanOutput.String(), "Switch: completed") {
				t.Fatalf("failure claimed completion: %q", humanOutput.String())
			}
		})
	}
}

func TestSwitchJSONContractCarriesPluginDataAndPerTargetNextActions(t *testing.T) {
	t.Parallel()
	retained := domain.PluginDataDecision{
		Disposition: domain.PluginDataRetained, Present: true, ReceiptCount: 1,
		Ownership: domain.PluginDataOwnershipOwned, Compatibility: domain.PluginDataCompatibilityNotProven,
		Warning: domain.PluginDataCompatibilityWarning,
	}
	tests := []struct {
		name       string
		result     switchOutput
		wantClient domain.ClientID
		wantAction string
		wantData   domain.PluginDataDisposition
	}{
		{
			name: "active stateful upstream to bridge preview", wantClient: domain.ClientCodex,
			wantAction: "finish installation in Codex Plugins, then start a new session",
			wantData:   domain.PluginDataRetained,
			result: switchOutput{DryRun: true, Status: "planned", PluginData: retained, Targets: []switchTargetOutput{{
				ClientID: domain.ClientCodex, Status: "manual_activation_required", NextAction: "finish installation in Codex Plugins, then start a new session",
			}}},
		},
		{
			name: "reverse bridge to upstream with manual Kiro activation", wantClient: domain.ClientKiro,
			wantAction: "import the prepared package as a custom Power in Kiro, then verify it is active",
			wantData:   domain.PluginDataRetained,
			result: switchOutput{Status: string(usecase.GroupPhaseCompleted), PluginData: retained, Targets: []switchTargetOutput{{
				ClientID: domain.ClientKiro, Status: string(usecase.GroupTargetExternalCompleted), NextAction: "import the prepared package as a custom Power in Kiro, then verify it is active",
			}}},
		},
		{
			name: "controlled rollback retains decision", wantClient: domain.ClientCodex,
			wantAction: "finish activation in Codex, then verify the original plugin remains active",
			wantData:   domain.PluginDataRetained,
			result: switchOutput{Status: string(usecase.GroupPhaseManagedRolledBack), PluginData: retained, Targets: []switchTargetOutput{{
				ClientID: domain.ClientCodex, Status: string(usecase.GroupTargetManagedRolledBack), NextAction: "finish activation in Codex, then verify the original plugin remains active",
			}}},
		},
		{
			name: "no data", wantClient: domain.ClientCodex,
			wantAction: "start a new Codex session and verify the switched plugin is available",
			wantData:   domain.PluginDataNone,
			result: switchOutput{Status: string(usecase.GroupPhaseCompleted), PluginData: domain.PluginDataDecision{
				Disposition: domain.PluginDataNone, Ownership: domain.PluginDataOwnershipNone, Compatibility: domain.PluginDataCompatibilityNotApplicable,
			}, Targets: []switchTargetOutput{{ClientID: domain.ClientCodex, Status: string(usecase.GroupTargetExternalCompleted), NextAction: "start a new Codex session and verify the switched plugin is available"}}},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := renderSwitchResult(&output, "json", test.result); err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data switchOutput `json:"data"`
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Data.PluginData.Disposition != test.wantData || len(envelope.Data.Targets) != 1 || envelope.Data.Targets[0].ClientID != test.wantClient || envelope.Data.Targets[0].NextAction != test.wantAction {
				t.Fatalf("switch contract = %+v", envelope.Data)
			}
			if test.wantData == domain.PluginDataRetained {
				if envelope.Data.PluginData.Ownership != domain.PluginDataOwnershipOwned || envelope.Data.PluginData.Compatibility != domain.PluginDataCompatibilityNotProven || envelope.Data.PluginData.Warning != domain.PluginDataCompatibilityWarning {
					t.Fatalf("retained-data contract = %+v", envelope.Data.PluginData)
				}
			} else if envelope.Data.PluginData.Present || envelope.Data.PluginData.Warning != "" {
				t.Fatalf("no-data contract = %+v", envelope.Data.PluginData)
			}
		})
	}
}

func TestSwitchHumanOutputSurfacesRetainedDataRiskAndPendingAction(t *testing.T) {
	t.Parallel()
	result := switchOutput{Status: string(usecase.GroupPhaseCompleted), PluginData: domain.PluginDataDecision{
		Disposition: domain.PluginDataRetained, Present: true, ReceiptCount: 1,
		Ownership: domain.PluginDataOwnershipOwned, Compatibility: domain.PluginDataCompatibilityNotProven,
		Warning: domain.PluginDataCompatibilityWarning,
	}, Targets: []switchTargetOutput{{ClientID: domain.ClientCodex, Status: string(usecase.GroupTargetExternalCompleted), NextAction: "install the prepared plugin in Codex, then verify it"}}}
	var output bytes.Buffer
	if err := renderSwitchResult(&output, "human", result); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, required := range []string{"completed; follow-up required", "PLUGIN_DATA: retained", domain.PluginDataCompatibilityWarning, "Next: install the prepared plugin in Codex, then verify it"} {
		if !strings.Contains(body, required) {
			t.Fatalf("human switch output omitted %q: %q", required, body)
		}
	}
}

func TestSingleTargetAddResultEnvelopeMatchesCommandOutcome(t *testing.T) {
	t.Parallel()
	success := domain.ActivationOutcome{
		Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled,
	}
	activationFailure := domain.ActivationOutcome{
		Activation: domain.ActivationFailed, Authentication: domain.AuthenticationNotChecked,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
	}
	tests := []struct {
		name       string
		result     usecase.AddResult
		commandErr error
		wantResult string
		wantHuman  string
	}{
		{
			name: "add activation failure", result: usecase.AddResult{Mutated: true, Activation: activationFailure},
			commandErr: errors.New("activate client"), wantResult: outputResultFailure, wantHuman: "Add: activation_failed",
		},
		{
			name: "resume activation failure", result: usecase.AddResult{Mutated: false, Activation: activationFailure},
			commandErr: errors.New("resume client activation"), wantResult: outputResultFailure, wantHuman: "Add: activation_failed",
		},
		{
			name: "controlled rollback without activation outcome", result: usecase.AddResult{},
			commandErr: errors.New("transaction rolled back"), wantResult: outputResultFailure, wantHuman: "Add: failed",
		},
		{
			name: "real success", result: usecase.AddResult{Mutated: true, Activation: success},
			wantResult: outputResultSuccess, wantHuman: "Installed and verified for the selected client.",
		},
	}
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"}}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var jsonOutput bytes.Buffer
			if err := renderAddResultError(&jsonOutput, "json", envelope, test.result, false, test.commandErr); err != nil {
				t.Fatal(err)
			}
			assertOutputEnvelopeResult(t, jsonOutput.Bytes(), "add", test.wantResult)

			var humanOutput bytes.Buffer
			if err := renderAddResultError(&humanOutput, "human", envelope, test.result, false, test.commandErr); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(humanOutput.String(), test.wantHuman) {
				t.Fatalf("human output = %q", humanOutput.String())
			}
			if test.wantResult != outputResultSuccess && strings.Contains(strings.ToLower(humanOutput.String()), "completed") {
				t.Fatalf("failure claimed completion: %q", humanOutput.String())
			}
		})
	}
}

func assertOutputEnvelopeResult(t *testing.T, body []byte, command, result string) {
	t.Helper()
	var envelope outputEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode output %q: %v", body, err)
	}
	if envelope.SchemaVersion != outputSchemaVersion || envelope.Command != command || envelope.Result != result {
		t.Fatalf("output envelope = %+v", envelope)
	}
}
