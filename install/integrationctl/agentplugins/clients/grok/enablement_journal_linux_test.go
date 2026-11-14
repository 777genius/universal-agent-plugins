//go:build linux

package grok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type enablementCrashWire struct {
	Record                   enablementRecord
	Desired                  domain.StateFileV2
	StatePath, OperationsDir string
}

func TestEnablementJournalProcessDeathReopensFiniteStates(t *testing.T) {
	for _, point := range []string{"stage_created", "record_written", "record_synced", "record_published", "publication_synced", "before_record_remove", "record_removed"} {
		t.Run(point, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			if point == "before_record_remove" || point == "record_removed" {
				fixture.attempt(t, true, false)
			} else {
				path, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			runEnablementCrashChild(t, fixture, point)
			if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err != nil {
				t.Fatal("reopen after real process exit", err)
			}
			if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err != nil {
				t.Fatal("second recovery", err)
			}
			body, err := os.ReadFile(fixture.config)
			if err != nil || !bytes.Equal(body, fixture.original) {
				t.Fatal("native predecessor differs", err)
			}
			if enablementStateDigest(fixture.loaded(t)) != fixture.record.BeforeStateDigest {
				t.Fatal("complete persisted predecessor differs")
			}
			entries, err := os.ReadDir(fixture.recovery.OperationsDir)
			if err != nil || len(entries) != 0 {
				t.Fatal("known unpublished or settled record remains", entries, err)
			}
		})
	}
}

func runEnablementCrashChild(t *testing.T, fixture enablementFixture, point string, desired ...domain.StateFileV2) {
	t.Helper()
	wire := enablementCrashWire{Record: fixture.record, StatePath: fixture.state.Path, OperationsDir: fixture.recovery.OperationsDir}
	if len(desired) != 0 {
		wire.Desired = desired[0]
	}
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(fixture.state.Path), "TEST-journal-child.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestEnablementJournalCrashChild$", "--", path, point)
	cmd.Env = []string{"UAP_GROK_JOURNAL_CHILD=1"}
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 81 {
		t.Fatalf("child did not reach crash boundary: %v %s", err, output)
	}
}

func TestEnablementJournalCrashChild(t *testing.T) {
	if os.Getenv("UAP_GROK_JOURNAL_CHILD") != "1" {
		return
	}
	if len(os.Args) < 3 {
		t.Fatal("missing bounded fixture arguments")
	}
	path, point := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wire enablementCrashWire
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	record := wire.Record
	recovery := EnablementRecovery{ProjectRoot: record.ProjectRoot, UserRoot: record.UserRoot, OperationsDir: wire.OperationsDir,
		State: transaction.Kernel{StateStore: statev2.Store{Path: wire.StatePath, ProjectRoot: record.ProjectRoot}}, Kernel: nativeconfig.New(),
		PackageVerifier: enablementTestPackage{path: record.ArtifactPath, digest: record.PackageDigest},
		journalPoint: func(observed string) error {
			if point == observed {
				os.Exit(81)
			}
			return nil
		}}
	if record.Action == "remove" {
		err = runRemovalCrashBoundary(t, recovery, wire, point)
	} else if point == "before_record_remove" || point == "record_removed" {
		err = recoverPrototypeRecords(t.Context(), recovery)
	} else {
		err = recovery.writeRecord(record)
	}
	t.Fatal("crash boundary was not reached", err)
}

func TestEnablementJournalPublicationErrorsRetainOnlyCompleteAuthority(t *testing.T) {
	for _, point := range []string{"stage_created", "record_written", "record_synced", "record_published", "publication_synced"} {
		t.Run(point, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			path, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			fixture.recovery.journalPoint = func(observed string) error {
				if observed == point {
					return errors.New("inert publication failure")
				}
				return nil
			}
			if err := fixture.recovery.writeRecord(fixture.record); err == nil {
				t.Fatal("failed publication reported success")
			}
			fixture.recovery.journalPoint = nil
			_, err := fixture.recovery.readRecord(fixture.record.OperationID)
			published := point == "record_published" || point == "publication_synced"
			if published && err != nil || !published && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failure left incomplete recognized authority", err)
			}
			if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEnablementJournalParentAndLeafReplacementNeverTouchesReplacement(t *testing.T) {
	for _, target := range []string{"parent", "read-leaf", "remove-leaf", "publish-stage"} {
		t.Run(target, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			recordPath, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			path, point := recordPath, "record_read"
			switch target {
			case "parent":
				path = fixture.recovery.OperationsDir
			case "remove-leaf":
				point = "before_record_remove"
			case "publish-stage":
				point, path = "record_synced", filepath.Join(fixture.recovery.OperationsDir, enablementStagePrefix+fixture.record.OperationID)
				if err := os.Remove(recordPath); err != nil {
					t.Fatal(err)
				}
			}
			fixture.recovery.journalPoint = replacementHook(t, path, point, target == "parent")
			var err error
			switch target {
			case "publish-stage":
				err = fixture.recovery.writeRecord(fixture.record)
			case "remove-leaf":
				body, readErr := os.ReadFile(recordPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				err = fixture.recovery.removeRecord(fixture.record.OperationID, enablementDigest(body))
			default:
				_, err = fixture.recovery.readRecord(fixture.record.OperationID)
			}
			if err == nil {
				t.Fatal("replaced journal authority admitted")
			}
			if target == "parent" {
				path = filepath.Join(path, "foreign")
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil || string(body) != "foreign retained" {
				t.Fatal("replacement was changed or removed", readErr)
			}
		})
	}
}

func replacementHook(t *testing.T, path, point string, directory bool) func(string) error {
	t.Helper()
	replaced := false
	return func(observed string) error {
		if observed != point || replaced {
			return nil
		}
		replaced = true
		if err := os.Rename(path, path+"-retained"); err != nil {
			return err
		}
		if directory {
			if err := os.Mkdir(path, 0700); err != nil {
				return err
			}
			path = filepath.Join(path, "foreign")
		}
		return os.WriteFile(path, []byte("foreign retained"), 0600)
	}
}

func TestEnablementJournalUnknownStageAndFullStateDriftRefuse(t *testing.T) {
	for _, change := range []string{"partial-stage", "unrelated-state", "binding-time", "installation-time"} {
		t.Run(change, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			fixture.attempt(t, true, false)
			state := fixture.loaded(t)
			switch change {
			case "partial-stage":
				path := filepath.Join(fixture.recovery.OperationsDir, enablementStagePrefix+"op-unknown")
				if err := os.WriteFile(path, []byte("{\"unknown\":"), 0600); err != nil {
					t.Fatal(err)
				}
			case "unrelated-state":
				other := state.Installations[0]
				other.InstallationID, other.Source.SourceBindingID = "foreign-installation", "foreign-source"
				other.Clients = map[string]domain.ClientBinding{}
				state.Installations = append(state.Installations, other)
			case "binding-time":
				binding := state.Installations[0].Clients[fixture.record.BindingID]
				binding.UpdatedAt = "2026-11-13T23:00:00Z"
				state.Installations[0].Clients[fixture.record.BindingID] = binding
			case "installation-time":
				state.Installations[0].UpdatedAt = "2026-11-13T23:00:00Z"
			}
			if err := fixture.state.Save(state); err != nil {
				t.Fatal(err)
			}
			if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err == nil {
				t.Fatal("unknown stage or third state admitted")
			}
			if enablementStateDigest(fixture.loaded(t)) != enablementStateDigest(state) {
				t.Fatal("third state was normalized away")
			}
			body, err := os.ReadFile(fixture.config)
			if err != nil || !bytes.Equal(body, fixture.next) {
				t.Fatal("native config changed after refusal", err)
			}
		})
	}
}

func runRemovalCrashBoundary(t *testing.T, recovery EnablementRecovery, wire enablementCrashWire, point string) error {
	t.Helper()
	if point == "before_callback" {
		os.Exit(81)
	}
	if point == "short_footer" {
		path, _ := recovery.recordRoot(wire.Record.OperationID)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		var limit unix.Rlimit
		if err := unix.Getrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			return err
		}
		limit.Cur = uint64(info.Size() + 17)
		signal.Ignore(unix.SIGXFSZ)
		if err := unix.Setrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			return err
		}
	}
	before, err := recovery.State.Load()
	if err != nil {
		return err
	}
	handled, err := recovery.PrepareNativeRemovalDecision(t.Context(), wire.Record.InstallationID, wire.Record.BindingID, before, wire.Desired)
	if point == "short_footer" && handled && err != nil {
		os.Exit(81)
	}
	if err != nil || !handled {
		return errors.Join(err, errors.New("TEST removal decision was not handled"))
	}
	if point == "decision_cleared" {
		return recoverPrototypeRecords(t.Context(), recovery)
	}
	disposition, err := recovery.State.PersistStateDecisionWithDisposition(before, wire.Desired)
	if err != nil || disposition != domain.StateDecisionDesired {
		return errors.Join(err, errors.New("TEST removal decision was not durable"))
	}
	if point == "state_committed" {
		os.Exit(81)
	}
	if err := recoverPrototypeRecords(t.Context(), recovery); err != nil {
		return err
	}
	if point == "before_package_delete" {
		os.Exit(81)
	}
	return errors.New("TEST removal crash point not reached")
}
