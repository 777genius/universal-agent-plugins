package installer

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type groupReportingStore struct {
	state domain.StateFileV2
	err   error
}

func (s groupReportingStore) Load() (domain.StateFileV2, error) { return s.state, s.err }
func (groupReportingStore) Save(domain.StateFileV2) error       { panic("reporting must not persist state") }

// Reporting cannot lose a known effect just because fresh inspection or the
// separate Prepared host handoff is incomplete. This table has no mutation seam.
func TestFinishGroupPreservesActualMutationFacts(t *testing.T) {
	cause := errors.New("fixture activation failure")
	loadFailure := errors.New("fixture state read failure")
	for _, tc := range []struct {
		name                          string
		prior, core, preview, pending bool
		loadErr, activationErr        error
		wantOutcome                   Outcome
		wantErr                       bool
	}{
		{name: "core effect", core: true, wantOutcome: OutcomeCompleted},
		{name: "prior host effect", prior: true, wantOutcome: OutcomeCompleted},
		{name: "stale preview with core effect", core: true, preview: true, wantOutcome: OutcomeCompleted},
		{name: "stale preview with prior effect", prior: true, preview: true, wantOutcome: OutcomeCompleted},
		{name: "read only unchanged preview", preview: true, wantOutcome: OutcomeUnchanged},
		{name: "read only unchanged core", wantOutcome: OutcomeUnchanged},
		{name: "effect before activation failure", core: true, activationErr: cause, wantOutcome: OutcomeIncomplete, wantErr: true},
		{name: "effect before prepared handoff", core: true, pending: true, wantOutcome: OutcomeIncomplete, wantErr: true},
		{name: "core effect with unavailable inspection", core: true, loadErr: loadFailure, wantOutcome: OutcomeCompleted},
		{name: "prior effect with unavailable inspection", prior: true, loadErr: loadFailure, wantOutcome: OutcomeIncomplete, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := domain.StateFileV2{Installations: []domain.Installation{{InstallationID: "reporting-fixture", Clients: map[string]domain.ClientBinding{
				"binding": {ClientBindingID: "binding", ClientID: "claude", Scope: "project", Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled},
			}}}}
			engine := Engine{store: groupReportingStore{state: state, err: tc.loadErr}}
			prepared := &PreparedOperation{req: Request{Operation: OpRepair}, plan: Plan{InstallationID: "reporting-fixture", NoChange: tc.preview}, clients: []domain.DetectedClient{{ClientID: domain.ClientClaude}}}
			got := usecase.GroupResult{InstallationID: "reporting-fixture", Mutated: tc.core, Targets: []usecase.AddResult{{NoChange: true}}}
			result, err := engine.finishGroup(prepared, Result{Mutated: tc.prior}, tc.pending, got, tc.activationErr)
			if result.Mutated != (tc.prior || tc.core) || result.Outcome != tc.wantOutcome || (err != nil) != tc.wantErr {
				t.Fatalf("reported=%+v err=%v", result, err)
			}
			if result.Mutated && (result.NoChange || result.Outcome == OutcomeUnchanged) {
				t.Fatalf("actual effect called unchanged=%+v", result)
			}
			if !result.Mutated && tc.wantOutcome == OutcomeUnchanged && !result.NoChange {
				t.Fatalf("read only result lost unchanged=%+v", result)
			}
			if tc.pending && (result.Reason != "committed binding is not activated" || result.Client.Activation != "prepared" || result.Client.Verification != "installation_verified") {
				t.Fatalf("prepared handoff contract changed=%+v", result)
			}
			if tc.activationErr != nil && !errors.Is(err, tc.activationErr) {
				t.Fatalf("activation error lost=%v", err)
			}
		})
	}
}
