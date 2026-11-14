package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func hermesWindowsChainFixture() []hermesWindowsProcess {
	return []hermesWindowsProcess{
		{PID: 40, ParentPID: 30, Created100ns: 116444737000000004, Image: "probe.exe"},
		{PID: 30, ParentPID: 20, Created100ns: 116444737000000003, Image: "managed.exe"},
		{PID: 20, ParentPID: 10, Created100ns: 116444737000000002, Image: "python.exe"},
		{PID: 10, ParentPID: 5, Created100ns: 116444737000000001, Image: "hermes.exe"},
	}
}

func TestHermesWindowsChainKeepsAllHandlesUntilRejoin(t *testing.T) {
	want := hermesWindowsChainFixture()
	var opened, closed []uint32
	reads := 0
	open := func(pid uint32) (windows.Handle, error) {
		opened = append(opened, pid)
		return windows.Handle(pid), nil
	}
	read := func(handle windows.Handle) (hermesWindowsProcess, error) {
		reads++
		if len(closed) != 0 || reads > 4 && len(opened) != 4 {
			t.Fatal("original process handles were released before the final rejoin")
		}
		for _, row := range want {
			if row.PID == uint32(handle) {
				return row, nil
			}
		}
		t.Fatal("reader left the fixed process chain")
		return hermesWindowsProcess{}, nil
	}
	got, err := captureHermesWindowsChain(40, open, read, func(handle windows.Handle) error {
		closed = append(closed, uint32(handle))
		return nil
	})
	if err != nil || !reflect.DeepEqual(got, want) || reads != 8 ||
		!reflect.DeepEqual(opened, []uint32{40, 30, 20, 10}) || !reflect.DeepEqual(closed, opened) {
		t.Fatalf("original chain/closure differs: got=%+v opened=%v closed=%v reads=%d err=%v", got, opened, closed, reads, err)
	}
}

func TestHermesWindowsChainRefusesReusedChangedAndUnavailableParents(t *testing.T) {
	for _, effect := range []string{"wrong-pid", "same-birth", "newer-birth", "reused-pid", "missing-parent", "read", "open", "rejoin", "close"} {
		t.Run(effect, func(t *testing.T) {
			rows := hermesWindowsChainFixture()
			opened, closed, reads := 0, 0, 0
			open := func(pid uint32) (windows.Handle, error) {
				if effect == "open" && opened == 2 {
					return 0, errors.New("owned open refusal")
				}
				opened++
				return windows.Handle(pid), nil
			}
			read := func(handle windows.Handle) (hermesWindowsProcess, error) {
				reads++
				row := rows[(40-int(handle))/10]
				if reads == 2 {
					switch effect {
					case "wrong-pid":
						row.PID++
					case "same-birth":
						row.Created100ns = rows[0].Created100ns
					case "newer-birth":
						row.Created100ns = rows[0].Created100ns + 1
					case "reused-pid":
						row.ParentPID = rows[0].PID
					case "missing-parent":
						row.ParentPID = 0
					case "read":
						return row, errors.New("original process stopped")
					}
				}
				if effect == "rejoin" && reads == 7 {
					row.Created100ns++
				}
				return row, nil
			}
			_, err := captureHermesWindowsChain(40, open, read, func(windows.Handle) error {
				closed++
				if effect == "close" {
					return errors.New("held handle close refused")
				}
				return nil
			})
			if err == nil || closed != opened || opened > 4 {
				t.Fatalf("uncertain process chain accepted or handles leaked: %d/%d %v", opened, closed, err)
			}
		})
	}
}

func TestHermesWindowsReadJoinsOriginalProcessAndRefusesExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(ctx, executable, "-test.run=^TestHermesWindowsProcessWaitHelper$")
	child.Env = append(os.Environ(), "UAP_HERMES_WINDOWS_WAIT=1")
	input, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	handle, err := openHermesWindowsProcess(uint32(child.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
	first, err := readHermesWindowsProcess(handle)
	if err != nil || first.PID != uint32(child.Process.Pid) || first.ParentPID != uint32(os.Getpid()) || first.Image != executable {
		t.Fatalf("original child process differs: %+v %v", first, err)
	}
	second, err := readHermesWindowsProcess(handle)
	if err != nil || first != second {
		t.Fatalf("held child changed: %+v %v", second, err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatal(err)
	}
	if _, err := readHermesWindowsProcess(handle); err == nil {
		t.Fatal("terminated handle accepted as live")
	}
}

func TestHermesWindowsParentSnapshotRejoinsTheSameLiveChild(t *testing.T) {
	for _, effect := range []string{"valid", "dead-before", "dead-after", "pid", "birth", "snapshot"} {
		t.Run(effect, func(t *testing.T) {
			want := hermesWindowsIdentity{PID: 40, Created100ns: 116444737000000004}
			reads, lookups := 0, 0
			identity := func(handle windows.Handle) (hermesWindowsIdentity, error) {
				if handle != 713 {
					t.Fatal("parent snapshot reopened the original child")
				}
				reads++
				if effect == "dead-before" || effect == "dead-after" && reads == 2 {
					return hermesWindowsIdentity{}, errors.New("original child stopped")
				}
				got := want
				if reads == 2 && effect == "pid" {
					got.PID++
				}
				if reads == 2 && effect == "birth" {
					got.Created100ns++
				}
				return got, nil
			}
			lookup := func(pid uint32) (uint32, error) {
				lookups++
				if pid != want.PID || reads != 1 {
					t.Fatal("snapshot lacks its original held child identity")
				}
				if effect == "snapshot" {
					return 0, errors.New("snapshot unavailable")
				}
				return 30, nil
			}
			got, parent, err := joinHermesWindowsParent(713, identity, lookup)
			if effect == "valid" {
				if err != nil || got != want || parent != 30 || reads != 2 || lookups != 1 {
					t.Fatalf("original child/parent join differs: %+v %d %d/%d %v", got, parent, reads, lookups, err)
				}
			} else if err == nil || got != (hermesWindowsIdentity{}) || parent != 0 || lookups > 1 ||
				effect == "dead-before" && lookups != 0 {
				t.Fatalf("uncertain snapshot child accepted: %+v %d %d/%d %v", got, parent, reads, lookups, err)
			}
		})
	}
}

func TestHermesWindowsProcessWaitHelper(t *testing.T) {
	if os.Getenv("UAP_HERMES_WINDOWS_WAIT") != "1" {
		return
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
}
