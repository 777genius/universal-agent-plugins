//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func taskStatFixture(tid int, start string) []byte {
	fields := strings.Fields("R 1 44 44 0 -1 0 0 0 0 0 0 0 0 0 0 0 1 0 " + start)
	return []byte(fmt.Sprintf("%d (inert fixture) %s\n", tid, strings.Join(fields, " ")))
}

func TestLinuxTaskChildrenMissingInterfaceRequiresPositiveLiveIdentity(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		childErr, errorAfter               error
		before, after                      []byte
		exists                             bool
		wantUnsupported, wantGone, wantErr bool
	}{
		{name: "same live task has no interface", childErr: os.ErrNotExist, exists: true, wantUnsupported: true, wantErr: true},
		{name: "task exits after initial identity", childErr: os.ErrNotExist, errorAfter: os.ErrNotExist, wantGone: true},
		{name: "task is replaced", childErr: os.ErrNotExist, after: taskStatFixture(44, "8"), exists: true, wantErr: true},
		{name: "missing stat inside present task is unknown", childErr: os.ErrNotExist, errorAfter: os.ErrNotExist, exists: true, wantErr: true},
		{name: "permission is not unsupported", childErr: os.ErrPermission, exists: true, wantErr: true},
		{name: "unknown read failure", childErr: syscall.EIO, exists: true, wantErr: true},
		{name: "ESRCH for still live task is not unsupported", childErr: syscall.ESRCH, exists: true, wantErr: true},
		{name: "malformed before identity", before: []byte("unknown"), exists: true, wantErr: true},
		{name: "malformed after identity", childErr: os.ErrNotExist, after: []byte("unknown"), exists: true, wantErr: true},
		{name: "available empty interface", exists: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			ops := linuxTaskProcOps{
				readFile: func(path string, limit int) ([]byte, error) {
					if strings.HasSuffix(path, "/children") {
						if limit != linuxProcChildrenLimit {
							t.Fatal(limit)
						}
						return []byte{}, tc.childErr
					}
					if limit != linuxProcStatLimit {
						t.Fatal(limit)
					}
					reads++
					if reads > 1 {
						if tc.errorAfter != nil {
							return nil, tc.errorAfter
						}
						if tc.after != nil {
							return tc.after, nil
						}
					}
					if tc.before != nil {
						return tc.before, nil
					}
					return taskStatFixture(44, "7"), nil
				},
				taskExists: func(string) (bool, error) { return tc.exists, nil },
			}
			_, present, err := linuxTaskChildrenWith(12, 44, ops)
			if (err != nil) != tc.wantErr || errors.Is(err, ErrOrdinaryContainmentUnsupported) != tc.wantUnsupported || (!present) != tc.wantGone {
				t.Fatalf("present=%t err=%v", present, err)
			}
			if tc.wantUnsupported && linuxProcessGone(err) {
				t.Fatalf("unsupported live interface must not look like a disappeared ancestor: %v", err)
			}
			if errors.Is(tc.childErr, os.ErrPermission) && !errors.Is(err, os.ErrPermission) {
				t.Fatalf("permission cause lost: %v", err)
			}
		})
	}
}

func TestOrdinaryMissingLiveChildrenRefusesBeforeWrappingOrLaunch(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-launch")
	cmd := exec.Command("/bin/sh", "-c", "printf started > \"$1\"", "sh", marker)
	ops := linuxTaskProcOps{
		readFile: func(path string, _ int) ([]byte, error) {
			if strings.HasSuffix(path, "/children") {
				return nil, os.ErrNotExist
			}
			return taskStatFixture(44, "7"), nil
		},
		taskExists: func(string) (bool, error) { return true, nil },
	}
	containment, err := newOrdinaryCommandContainmentWithPreflight(cmd, time.Second, func() error { return ordinaryContainmentPreflightWith(12, 44, ops) })
	if !errors.Is(err, ErrOrdinaryContainmentUnsupported) || containment != nil || cmd.Process != nil || len(cmd.ExtraFiles) != 0 || cmd.Path != "/bin/sh" {
		t.Fatalf("prelaunch refusal=%+v %v cmd=%+v", containment, err, cmd)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unsupported command was launched: %v", err)
	}
}

func TestOrdinaryActualUnsupportedPreflightCannotLaunch(t *testing.T) {
	err := (OS{}).OrdinaryPreflight()
	if err == nil {
		t.Skip("this host supplies the proc prerequisite; supported runtime tests qualify actual shutdown")
	}
	if !errors.Is(err, ErrOrdinaryContainmentUnsupported) {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "must-not-launch")
	_, runErr := (OS{}).Run(t.Context(), ports.Command{Argv: []string{"/bin/sh", "-c", "printf started > \"$1\"", "sh", marker}})
	if !errors.Is(runErr, ErrOrdinaryContainmentUnsupported) {
		t.Fatalf("ordinary launch did not retain unsupported cause: %v", runErr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unsupported command was launched: %v", err)
	}
}

func TestBoundedProcObservationsAndStrictIdentities(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proc-fixture")
	for _, limit := range []int{linuxProcStatLimit, linuxProcChildrenLimit} {
		if err := os.WriteFile(path, []byte(strings.Repeat("x", limit+1)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readBoundedLinuxProcFile(path, limit); err == nil {
			t.Fatalf("accepted proc read over %d bytes", limit)
		}
		if err := os.WriteFile(path, []byte(strings.Repeat("x", limit)), 0600); err != nil {
			t.Fatal(err)
		}
		body, err := readBoundedLinuxProcFile(path, limit)
		if err != nil || len(body) != limit {
			t.Fatalf("exact bound rejected: bytes=%d error=%v", len(body), err)
		}
	}
	for _, body := range [][]byte{[]byte("44 (x) R 1"), taskStatFixture(45, "7"), taskStatFixture(44, "-1"), taskStatFixture(44, "unknown")} {
		if _, err := parseLinuxTaskIdentity(body, 44); err == nil {
			t.Fatalf("accepted malformed task stat %q", body)
		}
	}
	for _, body := range []string{"-1", "0", "+2", "02", "unknown", strings.Repeat("1", linuxProcChildrenLimit+1)} {
		if err := validateLinuxProcChildren([]byte(body)); err == nil {
			t.Fatalf("accepted malformed proc children %q", body[:min(len(body), 40)])
		}
	}
}

// This is an inert constructor check. It supplies the exact prerequisite
// observation through per-call operations and never starts the command.
func TestOrdinaryAvailableEmptyProcInterfacePermitsWrapping(t *testing.T) {
	ops := linuxTaskProcOps{
		readFile: func(path string, _ int) ([]byte, error) {
			if strings.HasSuffix(path, "/children") {
				return []byte{}, nil
			}
			return taskStatFixture(44, "7"), nil
		},
		taskExists: func(string) (bool, error) { return true, nil },
	}
	cmd := exec.Command("/bin/true")
	containment, err := newOrdinaryCommandContainmentWithPreflight(cmd, time.Second, func() error { return ordinaryContainmentPreflightWith(12, 44, ops) })
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := containment.close(); err != nil {
			t.Error(err)
		}
	}()
	if cmd.Process != nil || cmd.Path != "/proc/self/exe" || len(cmd.ExtraFiles) != 3 {
		t.Fatalf("supported prerequisite did not retain ordinary supervision: %+v", cmd)
	}
}

func TestProcTaskEnumerationAndNamesAreBounded(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < linuxProcTaskEntriesLimit+1; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprint(i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := readBoundedLinuxTaskDirectory(root); err == nil {
		t.Fatal("unbounded task directory was accepted")
	}
	if err := os.Remove(filepath.Join(root, fmt.Sprint(linuxProcTaskEntriesLimit))); err != nil {
		t.Fatal(err)
	}
	if entries, err := readBoundedLinuxTaskDirectory(root); err != nil || len(entries) != linuxProcTaskEntriesLimit {
		t.Fatalf("exact task bound: %d %v", len(entries), err)
	}
	for _, name := range []string{"0", "-1", "+1", "01", "unknown"} {
		if _, err := linuxTaskID(name); err == nil {
			t.Fatalf("malformed task directory accepted: %q", name)
		}
	}
}
