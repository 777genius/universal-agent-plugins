//go:build linux

package grok

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func recordedObservation(t *testing.T, recovery EnablementRecovery) ports.NativeTransitionObservation {
	t.Helper()
	items, err := recovery.Pending()
	if err != nil || len(items) != 1 {
		t.Fatalf("one original recovery observation required: %v %v", items, err)
	}
	return items[0]
}

func assertRecordedSettlement(t *testing.T, fixture enablementFixture, want []byte, stateDigest string) {
	t.Helper()
	body, err := os.ReadFile(fixture.config)
	if err != nil || !bytes.Equal(body, want) || enablementStateDigest(fixture.loaded(t)) != stateDigest {
		t.Fatal("recorded config or complete state decision differs", err)
	}
	info, err := os.Stat(fixture.config)
	if err != nil || info.Mode().Perm() != os.FileMode(fixture.record.Edit.Mode) {
		t.Fatal("recorded config mode differs", err)
	}
	path, err := fixture.recovery.recordRoot(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("settled owned journal remains", err)
	}
	if _, err := os.Stat(fixture.record.ArtifactPath); err != nil {
		t.Fatal("recovery removed the retained package", err)
	}
}

func TestRecordedRecoveryV3EnablementMatrix(t *testing.T) {
	for _, stage := range []string{"predecessor", "attempt", "config", "committed"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			if stage != "predecessor" {
				fixture.attempt(t, stage != "attempt", stage == "committed")
			}
			observed := recordedObservation(t, fixture.recovery)
			cold := coldEnablementRecovery(fixture)
			if err := cold.recoverObservedRecord(t.Context(), observed); err != nil {
				t.Fatal("cold recorded recovery refused an exact boundary", err)
			}
			want, digest := fixture.original, fixture.record.BeforeStateDigest
			if stage == "committed" {
				want, digest = fixture.next, fixture.record.DesiredStateDigest
			}
			assertRecordedSettlement(t, fixture, want, digest)
		})
	}
}

func TestRecordedRecoveryV3CompletionRequiresExactConfig(t *testing.T) {
	for _, change := range []string{"before-image", "foreign-image", "mode"} {
		t.Run(change, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			fixture.attempt(t, true, true)
			body := fixture.next
			switch change {
			case "before-image":
				body = fixture.original
			case "foreign-image":
				body = append(bytes.Clone(body), []byte("# foreign late edit\n")...)
			}
			if err := os.WriteFile(fixture.config, body, 0600); err != nil {
				t.Fatal(err)
			}
			if change == "mode" {
				if err := os.Chmod(fixture.config, 0644); err != nil {
					t.Fatal(err)
				}
			}
			observed := recordedObservation(t, fixture.recovery)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, observed.Root)
			if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); !errors.Is(err, nativeconfig.ErrConcurrentChange) {
				t.Fatal("committed state discarded a different config image", err)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func recordedRemovalBoundary(t *testing.T, footer, state string, applied bool) (enablementFixture, domain.StateFileV2) {
	t.Helper()
	fixture, desired := removalEnablementFixture(t, 3)
	if applied {
		applyRemovalFixture(t, fixture)
	}
	if footer != "none" {
		tail, err := decisionTail(fixture.record, enablementStateDigest(desired))
		if err != nil {
			t.Fatal(err)
		}
		if footer == "partial" {
			tail = tail[:len(enablementDecisionPrefix)+7]
		}
		appendFixtureFooter(t, fixture, tail)
	}
	current := fixture.loaded(t)
	switch state {
	case "target":
		current = desired
	case "predecessor":
		binding := current.Installations[0].Clients[fixture.record.BindingID]
		binding.NativeActivationAttempt = ""
		current = enablementState(current, 0, binding)
	}
	if err := fixture.state.Save(current); err != nil {
		t.Fatal(err)
	}
	return fixture, desired
}

func TestRecordedRecoveryV3RemovalMatrix(t *testing.T) {
	for _, item := range []struct {
		footer, state string
		applied, pass bool
	}{
		{"none", "attempt", false, true}, {"none", "attempt", true, true},
		{"partial", "attempt", false, true}, {"partial", "attempt", true, true},
		{"complete", "attempt", false, true}, {"complete", "attempt", true, true},
		{"complete", "target", true, true}, {"complete", "target", false, false},
		{"none", "target", true, false}, {"partial", "target", true, false},
		{"none", "predecessor", false, true}, {"none", "predecessor", true, false},
		{"partial", "predecessor", false, false}, {"complete", "predecessor", false, false},
	} {
		t.Run(fmt.Sprintf("%s/%s/applied-%t", item.footer, item.state, item.applied), func(t *testing.T) {
			fixture, desired := recordedRemovalBoundary(t, item.footer, item.state, item.applied)
			journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			err := coldEnablementRecovery(fixture).recoverRecord(t.Context(), fixture.record.OperationID)
			if (err == nil) != item.pass {
				t.Fatal("recorded removal boundary result differs", err)
			}
			if !item.pass {
				assertPrototypeFilesUnchanged(t, originals)
				return
			}
			want, digest := fixture.original, fixture.record.BeforeStateDigest
			if item.state == "target" {
				want, digest = fixture.next, enablementStateDigest(desired)
			}
			assertRecordedSettlement(t, fixture, want, digest)
		})
	}
}

func TestRecordedRecoveryV3CodecDoesNotOpenPublicRecovery(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			fixture := newEnablementFixture(t, version)
			fixture.attempt(t, true, false)
			observed := recordedObservation(t, fixture.recovery)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, observed.Root)
			cold := coldEnablementRecovery(fixture)
			if err := cold.Recover(t.Context()); !errors.Is(err, errEnablementUnqualified) {
				t.Fatal("private format opened public recovery", err)
			}
			if err := cold.ReconcileObserved(t.Context(), observed, cold.State); !errors.Is(err, errEnablementUnqualified) {
				t.Fatal("private format opened observed public recovery", err)
			}
			if version == 2 {
				if err := cold.recoverObservedRecord(t.Context(), observed); !errors.Is(err, errEnablementUnqualified) {
					t.Fatal("private v3 admission accepted legacy auxiliary effects", err)
				}
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}
