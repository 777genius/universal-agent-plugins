package main

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type hermesWindowsProcess struct {
	PID          uint32 `json:"pid"`
	ParentPID    uint32 `json:"parent_pid"`
	Created100ns uint64 `json:"created_100ns"`
	Image        string `json:"image"`
}

type hermesWindowsIdentity struct {
	PID          uint32
	Created100ns uint64
}

func init() {
	hermesWindowsParent = func() (any, error) {
		rows, err := captureHermesWindowsChain(windows.GetCurrentProcessId(), openHermesWindowsProcess, readHermesWindowsProcess, windows.CloseHandle)
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "hermes-public-windows-lineage-v1", "processes": rows}, nil
	}
}

func openHermesWindowsProcess(pid uint32) (windows.Handle, error) {
	return windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, pid)
}

// There are exactly four roles: probe, managed launcher, Hermes Python and
// bundled native launcher. Nothing here authorizes process termination.
func captureHermesWindowsChain(pid uint32, open func(uint32) (windows.Handle, error), read func(windows.Handle) (hermesWindowsProcess, error), closeHandle func(windows.Handle) error) (rows []hermesWindowsProcess, err error) {
	var held []windows.Handle
	defer func() {
		for _, handle := range held {
			err = errors.Join(err, closeHandle(handle))
		}
	}()
	for range 4 {
		if pid == 0 {
			return nil, errors.New("hermes Windows parent is unavailable")
		}
		handle, openErr := open(pid)
		if openErr != nil {
			return nil, openErr
		}
		held = append(held, handle)
		row, readErr := read(handle)
		if readErr != nil {
			return nil, readErr
		}
		if row.PID != pid || !hermesWindowsOlderParent(rows, row) {
			return nil, errors.New("hermes Windows process ancestry changed")
		}
		rows = append(rows, row)
		pid = row.ParentPID
	}
	for index, handle := range held {
		row, readErr := read(handle)
		if readErr != nil {
			return nil, readErr
		}
		if row != rows[index] {
			return nil, errors.New("hermes Windows held process changed")
		}
	}
	return rows, nil
}

func hermesWindowsOlderParent(rows []hermesWindowsProcess, parent hermesWindowsProcess) bool {
	if parent.ParentPID == 0 || parent.Created100ns <= 116444736000000000 || parent.Image == "" {
		return false
	}
	for _, child := range rows {
		if parent.PID == child.PID || parent.Created100ns >= child.Created100ns {
			return false
		}
	}
	return true
}

func readHermesWindowsProcess(handle windows.Handle) (hermesWindowsProcess, error) {
	var row hermesWindowsProcess
	original, parent, err := joinHermesWindowsParent(handle, readHermesWindowsIdentity, hermesWindowsSnapshotParent)
	if err != nil {
		return row, err
	}
	image, err := hermesWindowsProcessImage(handle)
	if err != nil {
		return row, err
	}
	row = hermesWindowsProcess{PID: original.PID, ParentPID: parent,
		Created100ns: original.Created100ns, Image: image}
	return row, liveHermesWindowsProcess(handle)
}

func joinHermesWindowsParent(handle windows.Handle, identity func(windows.Handle) (hermesWindowsIdentity, error), lookup func(uint32) (uint32, error)) (hermesWindowsIdentity, uint32, error) {
	before, err := identity(handle)
	if err != nil {
		return hermesWindowsIdentity{}, 0, err
	}
	parent, err := lookup(before.PID)
	if err != nil {
		return hermesWindowsIdentity{}, 0, err
	}
	after, err := identity(handle)
	if err != nil {
		return hermesWindowsIdentity{}, 0, err
	}
	if before != after {
		return hermesWindowsIdentity{}, 0, errors.New("hermes Windows child changed across parent snapshot")
	}
	return before, parent, nil
}

func readHermesWindowsIdentity(handle windows.Handle) (hermesWindowsIdentity, error) {
	var identity hermesWindowsIdentity
	if err := liveHermesWindowsProcess(handle); err != nil {
		return identity, err
	}
	pid, err := windows.GetProcessId(handle)
	if err != nil {
		return identity, err
	}
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return identity, err
	}
	ticks := uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime)
	if pid == 0 || ticks <= 116444736000000000 || ticks >= 1<<63 {
		return identity, errors.New("hermes Windows process identity differs")
	}
	identity = hermesWindowsIdentity{PID: pid, Created100ns: ticks}
	return identity, liveHermesWindowsProcess(handle)
}

// Toolhelp supplies only a hint for an already held/live child. No other row
// is retained, and the caller rejoins that same child before opening its parent.
func hermesWindowsSnapshotParent(pid uint32) (parent uint32, err error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(snapshot)) }()
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for count := range 4096 {
		if count == 0 {
			err = windows.Process32First(snapshot, &entry)
		} else {
			err = windows.Process32Next(snapshot, &entry)
		}
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return 0, errors.New("hermes Windows original child is absent from parent snapshot")
			}
			return 0, err
		}
		if entry.ProcessID == pid {
			if entry.ParentProcessID == 0 || entry.ParentProcessID == pid {
				return 0, errors.New("hermes Windows snapshot parent is unavailable")
			}
			return entry.ParentProcessID, nil
		}
	}
	return 0, errors.New("hermes Windows parent snapshot exceeds row bound")
}

func hermesWindowsProcessImage(handle windows.Handle) (string, error) {
	var buffer [4096]uint16
	length := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &length); err != nil {
		return "", err
	}
	if length == 0 || length >= uint32(len(buffer)) {
		return "", errors.New("hermes Windows process image bound")
	}
	return windows.UTF16ToString(buffer[:length]), nil
}

func liveHermesWindowsProcess(handle windows.Handle) error {
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return err
	}
	if state != uint32(windows.WAIT_TIMEOUT) {
		return errors.New("hermes Windows original process is not live")
	}
	return nil
}
