//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func taskReleaseStatFixture(tid int, start string, changes map[int]string) []byte {
	fields := make([]string, 50)
	for index := range fields {
		fields[index] = "0"
	}
	fields[0], fields[2], fields[3], fields[5], fields[19] = "R", "-1", "-1", "-1", start
	for index, value := range changes {
		fields[index] = value
	}
	return []byte(fmt.Sprintf("%d (inert released task) %s\n", tid, strings.Join(fields, " ")))
}

func TestLinuxTaskReleaseSnapshotRequiresProvenDisappearance(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		after, exists, changed, zombie bool
		observationErr                 error
		wantGone, wantErr              bool
		wantProbes                     int
	}{
		{name: "initial_absent", wantGone: true, wantProbes: 1},
		{name: "initial_present", exists: true, wantErr: true, wantProbes: 1},
		{name: "initial_observation_error", observationErr: os.ErrPermission, wantErr: true, wantProbes: 1},
		{name: "after_absent", after: true, wantGone: true, wantProbes: 1},
		{name: "after_present", after: true, exists: true, wantErr: true, wantProbes: 1},
		{name: "after_observation_error", after: true, observationErr: syscall.EIO, wantErr: true, wantProbes: 1},
		{name: "after_changed_identity_absent", after: true, changed: true, wantErr: true},
		{name: "zombie_release_absent", zombie: true, wantGone: true, wantProbes: 1},
		{name: "zombie_release_present", zombie: true, exists: true, wantErr: true, wantProbes: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statReads, childrenReads, probes := 0, 0, 0
			ops := linuxTaskProcOps{
				readFile: func(path string, limit int) ([]byte, error) {
					switch path {
					case "/proc/12/task/44/stat":
						if limit != linuxProcStatLimit {
							t.Fatal("stat observation bound changed")
						}
						statReads++
						if tc.after && statReads == 1 {
							return taskStatFixture(44, "7"), nil
						}
						start, changes := "7", map[int]string{}
						if tc.changed {
							start = "8"
						}
						if tc.zombie {
							changes[0] = "Z"
						}
						return taskReleaseStatFixture(44, start, changes), nil
					case "/proc/12/task/44/children":
						if !tc.after || limit != linuxProcChildrenLimit {
							t.Fatal("unexpected children observation or changed bound")
						}
						childrenReads++
						return nil, os.ErrNotExist
					default:
						t.Fatal("observation escaped the exact task")
						return nil, nil
					}
				},
				taskExists: func(path string) (bool, error) {
					if path != "/proc/12/task/44" {
						t.Fatal("absence observation escaped the exact task")
					}
					probes++
					return tc.exists, tc.observationErr
				},
			}
			body, present, err := linuxTaskChildrenWith(12, 44, ops)
			if len(body) != 0 || present == tc.wantGone || (err != nil) != tc.wantErr {
				t.Fatalf("present=%t err=%v; want gone=%t error=%t", present, err, tc.wantGone, tc.wantErr)
			}
			wantStats, wantChildren := 1, 0
			if tc.after {
				wantStats, wantChildren = 2, 1
			}
			if statReads != wantStats || childrenReads != wantChildren || probes != tc.wantProbes {
				t.Fatalf("observation counts: stat=%d children=%d directory=%d", statReads, childrenReads, probes)
			}
			if linuxProcessGone(err) || errors.Is(err, ErrOrdinaryContainmentUnsupported) {
				t.Fatal("release or uncertain observation acquired unrelated errno authority")
			}
			if tc.changed && err.Error() != "proc task changed identity while its children interface was missing" {
				t.Fatal("observed replacement lost the original identity refusal")
			}
		})
	}
}

func TestLinuxTaskReleaseRecordRequiresCanonicalCompleteStat(t *testing.T) {
	_, validErr := parseLinuxTaskIdentity(taskReleaseStatFixture(44, "7", nil), 44)
	var released *linuxTaskReleaseIdentityError
	if !errors.As(validErr, &released) || released == nil || released.identity.pid != 44 || released.identity.startTime != 7 || released.identity.ppid != 0 || released.identity.pgid != -1 {
		t.Fatal("complete release record lost its private stable identity")
	}
	if validErr.Error() != "malformed proc task stable identity" || linuxProcessGone(validErr) || errors.Is(validErr, ErrOrdinaryContainmentUnsupported) {
		t.Fatal("release marker changed original error text or acquired errno authority")
	}
	complete := strings.TrimSpace(string(taskReleaseStatFixture(44, "7", nil)))
	truncated := []byte(complete[:strings.LastIndexByte(complete, ' ')])
	for _, tc := range []struct {
		name    string
		body    []byte
		changes map[int]string
	}{
		{name: "wrong_tid", body: taskReleaseStatFixture(45, "7", nil)},
		{name: "short_state", body: []byte("44 (inert) R 0 -1 -1")},
		{name: "missing_final_field", body: truncated},
		{name: "parent_negative_zero", changes: map[int]string{1: "-0"}},
		{name: "other_parent", changes: map[int]string{1: "1"}},
		{name: "group_leading_zero", changes: map[int]string{2: "-01"}},
		{name: "other_negative_group", changes: map[int]string{2: "-2"}},
		{name: "session_leading_zero", changes: map[int]string{3: "-01"}},
		{name: "other_session", changes: map[int]string{3: "0"}},
		{name: "start_leading_zero", changes: map[int]string{19: "07"}},
		{name: "negative_start", changes: map[int]string{19: "-1"}},
		{name: "start_plus", changes: map[int]string{19: "+7"}},
		{name: "unknown_start", changes: map[int]string{19: "unknown"}},
		{name: "counter_leading_zero", changes: map[int]string{20: "00"}},
		{name: "counter_negative_zero", changes: map[int]string{20: "-0"}},
		{name: "counter_plus", changes: map[int]string{20: "+1"}},
		{name: "unknown_counter", changes: map[int]string{20: "unknown"}},
		{name: "counter_overflow", changes: map[int]string{20: "18446744073709551616"}},
		{name: "unknown_state", changes: map[int]string{0: "?"}},
		{name: "malformed_state", changes: map[int]string{0: "RR"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if body == nil {
				body = taskReleaseStatFixture(44, "7", tc.changes)
			}
			_, err := parseLinuxTaskIdentity(body, 44)
			var marker *linuxTaskReleaseIdentityError
			if err == nil || errors.As(err, &marker) || linuxProcessGone(err) {
				t.Fatal("malformed record acquired release/disappearance authority")
			}
			ops := linuxTaskProcOps{taskExists: func(string) (bool, error) {
				t.Fatal("malformed record attempted an absence probe")
				return false, nil
			}}
			gone, retained := linuxTaskGoneAfterIdentityError("/inert/task", err, ops)
			if gone || reflect.ValueOf(retained) != reflect.ValueOf(err) {
				t.Fatal("ordinary malformed failure changed")
			}
		})
	}
	identity, err := parseLinuxTaskIdentity(taskStatFixture(44, "007"), 44)
	if err != nil || identity.pid != 44 || identity.startTime != 7 || identity.pgid != 44 {
		t.Fatal("special release admission changed the existing ordinary parser")
	}
}

func TestLinuxTaskReleaseNestedErrorsNeverAuthorizeDisappearance(t *testing.T) {
	_, marker := parseLinuxTaskIdentity(taskReleaseStatFixture(44, "7", nil), 44)
	var nilMarker *linuxTaskReleaseIdentityError
	for _, input := range []error{
		fmt.Errorf("wrapped release: %w", marker),
		errors.Join(marker, os.ErrNotExist),
		errors.Join(syscall.ESRCH, marker),
		errors.Join(marker, errors.Join(os.ErrPermission, os.ErrNotExist)),
		nilMarker,
	} {
		ops := linuxTaskProcOps{taskExists: func(string) (bool, error) {
			t.Fatal("unadmitted marker attempted an absence probe")
			return false, nil
		}}
		gone, err := linuxTaskGoneAfterIdentityError("/inert/task", input, ops)
		var uncertainty *linuxTaskIdentityUncertainty
		if gone || err == nil || !errors.As(err, &uncertainty) || uncertainty == nil || reflect.ValueOf(uncertainty.cause) != reflect.ValueOf(input) {
			t.Fatal("unadmitted marker lost its exact uncertain cause")
		}
		if err.Error() != input.Error() {
			t.Fatal("opaque uncertainty changed the original error text")
		}
		assertTaskReleaseRemainsUnknown(t, err)
	}
	for _, input := range []error{errors.New(marker.Error()), supervisorDiagnosticTextTrap{}} {
		ops := linuxTaskProcOps{taskExists: func(string) (bool, error) {
			t.Fatal("unrelated error attempted an absence probe")
			return false, nil
		}}
		gone, err := linuxTaskGoneAfterIdentityError("/inert/task", input, ops)
		if gone || reflect.ValueOf(err) != reflect.ValueOf(input) {
			t.Fatal("unrelated error acquired marker authority")
		}
		assertTaskReleaseRemainsUnknown(t, err)
	}
}

func TestLinuxTaskReleaseObserverErrorsRetainCausesWithoutGoneErrnos(t *testing.T) {
	_, marker := parseLinuxTaskIdentity(taskReleaseStatFixture(44, "7", nil), 44)
	for _, observation := range []error{
		os.ErrPermission, errors.New("unknown observation"), os.ErrNotExist, syscall.ESRCH,
		fmt.Errorf("wrapped missing directory: %w", os.ErrNotExist),
		errors.Join(os.ErrPermission, os.ErrNotExist), supervisorDiagnosticTextTrap{},
	} {
		for _, exists := range []bool{false, true} {
			probes := 0
			ops := linuxTaskProcOps{taskExists: func(path string) (bool, error) {
				if path != "/inert/task" {
					t.Fatal("observer path changed")
				}
				probes++
				return exists, observation
			}}
			gone, err := linuxTaskGoneAfterIdentityError("/inert/task", marker, ops)
			var uncertainty *linuxTaskIdentityUncertainty
			if gone || err == nil || probes != 1 || !errors.Is(err, marker) || !errors.As(err, &uncertainty) || uncertainty == nil {
				t.Fatal("failed observer lost release cause or granted disappearance")
			}
			if reflect.ValueOf(uncertainty.cause) != reflect.ValueOf(observation) || errors.Is(err, observation) {
				t.Fatal("observer cause was replaced or its errno escaped quarantine")
			}
			assertTaskReleaseRemainsUnknown(t, err)
		}
	}
	ops := linuxTaskProcOps{taskExists: func(string) (bool, error) { return true, nil }}
	gone, err := linuxTaskGoneAfterIdentityError("/inert/task", marker, ops)
	if gone || reflect.ValueOf(err) != reflect.ValueOf(marker) {
		t.Fatal("present directory changed the exact release refusal")
	}
}

func assertTaskReleaseRemainsUnknown(t *testing.T, cause error) {
	t.Helper()
	contextual := withOrdinaryCleanupOrigin("task_children_observation", cause)
	joined := errors.Join(errors.New("earlier non-gone failure"), contextual)
	for _, err := range []error{cause, contextual, joined} {
		if linuxProcessGone(err) || errors.Is(err, ErrOrdinaryContainmentUnsupported) {
			t.Fatal("uncertainty became disappearance at an outer boundary")
		}
	}
	if ordinaryCleanupFailureReason(joined) != "task_children_observation" {
		t.Fatal("opaque uncertainty changed bounded classification")
	}
}

func TestLinuxTaskReleaseCannotAuthorizeOrdinaryPreflight(t *testing.T) {
	for _, exists := range []bool{false, true} {
		reads, probes := 0, 0
		ops := linuxTaskProcOps{
			readFile: func(path string, limit int) ([]byte, error) {
				if path != "/proc/12/task/44/stat" || limit != linuxProcStatLimit {
					t.Fatal("preflight attempted an unadmitted observation")
				}
				reads++
				return taskReleaseStatFixture(44, "7", nil), nil
			},
			taskExists: func(path string) (bool, error) {
				if path != "/proc/12/task/44" {
					t.Fatal("preflight absence probe changed task")
				}
				probes++
				return exists, nil
			},
		}
		cmd := exec.CommandContext(t.Context(), "/bin/true")
		containment, err := newOrdinaryCommandContainmentWithPreflight(cmd, time.Second, func() error {
			return ordinaryContainmentPreflightWith(12, 44, ops)
		})
		if err == nil || containment != nil || cmd.Process != nil || cmd.Path != "/bin/true" || len(cmd.ExtraFiles) != 0 || reads != 1 || probes != 1 {
			t.Fatal("release snapshot authorized wrapping or command launch")
		}
		if linuxProcessGone(err) || errors.Is(err, ErrOrdinaryContainmentUnsupported) {
			t.Fatal("release preflight acquired unrelated errno authority")
		}
	}
}

func TestLinuxTaskDirectoryObserverNormalizesOnlyActualAbsence(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path                string
		wantExists, wantErr bool
	}{
		{path: filepath.Join(root, "absent")},
		{path: root, wantExists: true},
		{path: file, wantErr: true},
		{path: link, wantErr: true},
	} {
		exists, err := linuxTaskDirectoryExists(tc.path)
		if exists != tc.wantExists || (err != nil) != tc.wantErr || linuxProcessGone(err) {
			t.Fatal("directory observer confused absence, presence or uncertainty")
		}
	}
}
