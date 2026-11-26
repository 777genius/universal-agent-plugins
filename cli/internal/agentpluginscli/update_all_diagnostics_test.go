package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type diagnosticUpdateReadFailure struct {
	transaction.StateStore
	cause error
}

func (store diagnosticUpdateReadFailure) Load() (domain.StateFileV2, error) {
	return domain.StateFileV2{}, store.cause
}

// Seed real imperative installations and prepare both complete updates before
// introducing a check or stage failure. This exercises the actual session
// apply owner, durable state and remaining-item policy, rather than a renderer.
func TestImperativeUpdateAllFailureUsesSafeDiagnosticsAndStopsLaterWork(t *testing.T) {
	for _, boundary := range []string{"recheck", "stage"} {
		for _, outputFailure := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/output-failed=%t", boundary, outputFailure), func(t *testing.T) {
				client := fixtureClient(t, domain.ClientCursor)
				fixture := newCLIFixture(t, []domain.DetectedClient{client})
				for _, name := range []string{"alpha", "beta"} {
					plugin := writeNamedCLIPlugin(t, name, "1.0.0")
					if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
						t.Fatal(err)
					}
					writeNamedCLIPluginVersion(t, plugin, name, "2.0.0")
				}
				fixture.app.InstallLogDir = diagnosticLogDirectory(t, "unwritable")
				cmd, output, _ := retryTestCommand(t)
				opts := &options{scope: "user", format: "human", yes: true}
				session := updateAllSession{ctx: t.Context(), cmd: cmd, app: fixture.app, opts: opts}
				if empty, err := session.load(); err != nil || empty {
					t.Fatalf("real update session did not load: empty=%t err=%v", empty, err)
				}
				defer session.cleanupPrepared()
				if err := session.preflightAll(); err != nil {
					t.Fatal(err)
				}
				if len(session.preparedItems) != 2 || session.saved != nil {
					t.Fatalf("fixture is not an imperative-only prepared batch: %+v", session.result)
				}
				before := installedProjection(t, fixture)
				cause := fmt.Errorf("opaque-imperative-update-731298: %w", syscall.ENOSPC)
				prepared := session.preparedItems[0].prepared
				stager := &diagnosticStageFailure{PackageStager: prepared.service.Stager, cause: cause}
				if boundary == "recheck" {
					prepared.service.StateStore = diagnosticUpdateReadFailure{StateStore: prepared.service.StateStore, cause: cause}
				} else {
					prepared.service.Stager = stager
				}
				writerFailure := errors.New("opaque-imperative-writer-213987")
				if outputFailure {
					broken := &diagnosticResultWriter{fail: func(body []byte) bool { return bytes.Contains(body, []byte("Update all:")) }, err: writerFailure}
					cmd.SetOut(broken)
				}
				err := session.applyAll()
				if !errors.Is(err, cause) || !errors.Is(err, syscall.ENOSPC) || strings.Contains(err.Error(), "opaque-") {
					t.Fatalf("imperative failure lost its cause or exposed native detail: %v", err)
				}
				if outputFailure && (!errors.Is(err, writerFailure) || !errors.Is(err, errInstallOutput)) {
					t.Fatalf("imperative result output failure was discarded: %v", err)
				}
				if !reflect.DeepEqual(before, installedProjection(t, fixture)) {
					t.Fatal("refused imperative update changed a binding or replayed later work")
				}
				wantStages := 0
				if boundary == "stage" {
					wantStages = 1
				}
				if stager.calls != wantStages || session.result.Updated != 0 || session.result.Failed != 1 || session.result.Installations[1].Status != "not_attempted" {
					t.Fatalf("imperative failure replayed or lost unattempted peer: stages=%d result=%+v", stager.calls, session.result)
				}
				if !outputFailure {
					assertUnavailableDiagnostic(t, output.String(), "unwritable")
					if strings.Contains(output.String(), "opaque-") || !strings.Contains(output.String(), "free disk space") {
						t.Fatalf("imperative human row bypassed classified diagnostic projection: %q", output.String())
					}
				}
				// JSON and retained native outcome fields keep their established
				// details; the human projection must be detached from those facts.
				cmd.SetOut(output)
				output.Reset()
				opts.format = "json"
				if err := renderUpdateAll(cmd, opts, session.result); err != nil {
					t.Fatal(err)
				}
				decodeOneDiagnosticEnvelope(t, output.String())
				if !strings.Contains(output.String(), "opaque-imperative-update-731298") {
					t.Fatal("human projection altered original update-all JSON reason")
				}
			})
		}
	}
}
