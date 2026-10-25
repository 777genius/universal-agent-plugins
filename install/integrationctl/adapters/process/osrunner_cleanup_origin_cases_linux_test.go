//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestOrdinaryCleanupMissingTaskIdentityKeepsUnknownAndGoneDistinct(t *testing.T) {
	for _, present := range []bool{true, false} {
		ops := linuxTaskProcOps{taskExists: func(string) (bool, error) { return present, nil }}
		gone, err := linuxTaskGoneAfterIdentityError("/inert/task", os.ErrNotExist, ops)
		if !present {
			if !gone || err != nil {
				t.Fatal("disappeared task changed its existing accepted outcome")
			}
			continue
		}
		want := fmt.Sprintf("existing proc task identity is unavailable: %s", os.ErrNotExist.Error())
		if gone || err == nil || err.Error() != want || linuxProcessGone(err) || ordinaryCleanupFailureReason(err) != "task_identity_unavailable" {
			t.Fatal("missing identity in a present task changed its original refusal")
		}
		broader := withOrdinaryCleanupOrigin("task_children_observation", err)
		if reflect.ValueOf(broader) != reflect.ValueOf(err) || ordinaryCleanupFailureReason(broader) != "task_identity_unavailable" {
			t.Fatal("broader task annotation replaced the specific existing failure")
		}
	}
}

func TestOrdinaryCleanupTraversalOriginsRetainOriginalRefusalsAndHandles(t *testing.T) {
	readFailure := errors.New("inert identity observation failed")
	for _, tc := range []struct {
		name, reason, message          string
		openErr, childErr, parentErr   error
		changedStart, changedParent    bool
		expired, limit, gone, reparent bool
	}{
		{name: "pidfd open", openErr: syscall.EPERM, reason: "pidfd_open", message: "open stable supervisor descendant handle for 41: operation not permitted"},
		{name: "pidfd identity read", childErr: readFailure, reason: "pidfd_identity_read", message: "revalidate stable supervisor descendant 41: inert identity observation failed"},
		{name: "pidfd identity changed", changedStart: true, reason: "pidfd_identity_changed", message: "revalidate stable supervisor descendant 41: process start time changed"},
		{name: "parent identity read", changedParent: true, parentErr: readFailure, reason: "parent_identity_read", message: "revalidate stable supervisor descendant 41 parent 7: inert identity observation failed"},
		{name: "live parent changed", changedParent: true, reason: "parent_changed", message: "revalidate stable supervisor descendant 41: live parent changed from 7 to 8"},
		{name: "traversal deadline", expired: true, reason: "traversal_deadline", message: "Linux supervisor descendant traversal exceeded cleanup deadline after 1 visit(s)"},
		{name: "traversal limit", limit: true, reason: "traversal_limit", message: "Linux supervisor descendant traversal exceeded 1-process cleanup bound"},
		{name: "identity gone", gone: true},
		{name: "already accepted exiting parent", changedParent: true, reparent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closed, opened := 0, 0
			ops := linuxDescendantTraversalOps{
				directChildren: func(pid int, _ time.Time, _ int) (map[int]uint64, error) {
					if pid == 7 {
						return map[int]uint64{41: 101}, nil
					}
					if pid != 41 {
						t.Fatal("traversed an unrelated fixture identity")
					}
					return nil, nil
				},
				readIdentity: func(pid int) (linuxProcessIdentity, error) {
					if pid == 7 {
						return linuxProcessIdentity{pid: 7, ppid: 1, startTime: 11, zombie: tc.reparent}, tc.parentErr
					}
					if pid != 41 {
						t.Fatal("read an unrelated fixture identity")
					}
					if tc.gone {
						return linuxProcessIdentity{}, os.ErrNotExist
					}
					identity := linuxProcessIdentity{pid: 41, ppid: 7, startTime: 101}
					if tc.changedStart {
						identity.startTime++
					}
					if tc.changedParent {
						identity.ppid = 8
					}
					return identity, tc.childErr
				},
				openPidfd: func(pid, flags int) (int, error) {
					if pid != 41 || flags != 0 {
						t.Fatal("changed fixture pidfd acquisition")
					}
					if tc.openErr != nil {
						return -1, tc.openErr
					}
					opened++
					return 91, nil
				},
				closePidfd: func(fd int) error {
					if fd != 91 {
						t.Fatal("changed fixture handle authority")
					}
					closed++
					return nil
				},
			}
			deadline, visits := time.Now().Add(time.Second), 4
			if tc.expired {
				deadline = time.Now().Add(-time.Second)
			}
			if tc.limit {
				visits = 1
			}
			result, err := linuxDescendantIdentitiesWith(7, deadline, visits, ops)
			if tc.reason != "" {
				if err == nil || err.Error() != tc.message || ordinaryCleanupFailureReason(err) != tc.reason || len(result) != 0 || closed != opened {
					t.Fatal("origin changed original failure/message or leaked a fixture handle")
				}
				if tc.openErr != nil && !errors.Is(err, tc.openErr) || tc.childErr != nil && !errors.Is(err, tc.childErr) || tc.parentErr != nil && !errors.Is(err, tc.parentErr) {
					t.Fatal("origin lost original wrapped failure")
				}
				return
			}
			if err != nil || ordinaryCleanupFailureReason(err) != "" {
				t.Fatal("gone/accepted reparent outcome changed")
			}
			for _, entry := range result {
				if err := ops.closePidfd(entry.process.pidfd); err != nil {
					t.Fatal(err)
				}
			}
			if closed != opened {
				t.Fatal("fixture handle ownership changed")
			}
		})
	}
}
