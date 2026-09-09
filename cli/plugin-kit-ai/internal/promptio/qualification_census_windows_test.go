//go:build windows

package promptio

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

var qualificationQueryObject = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryObject")
var qualificationThreadID = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetThreadId")

// PROCESS_HANDLE_SNAPSHOT_INFORMATION / PROCESS_HANDLE_TABLE_ENTRY_INFO.
// Pointer-sized fields keep the native layout on both Windows architectures.
type qualificationHandleEntry struct {
	handle                                  windows.Handle
	handleCount, pointerCount               uintptr
	access, typeIndex, attributes, reserved uint32
}

func qualificationNativeCensus(t *testing.T) {
	t.Helper()
	// Toolhelp owns exactly one diagnostic handle, closed before handle census.
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		t.Errorf("native thread snapshot: %v", err)
		return
	}
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	var tids []uint32
	err = windows.Thread32First(snapshot, &entry)
	for err == nil {
		if entry.OwnerProcessID == windows.GetCurrentProcessId() {
			tids = append(tids, entry.ThreadID)
		}
		err = windows.Thread32Next(snapshot, &entry)
	}
	closeErr := windows.CloseHandle(snapshot)
	t.Logf("native tids=%v diagnostic_snapshot=%#x close=%v", tids, snapshot, closeErr)
	if closeErr != nil {
		t.Errorf("diagnostic snapshot close: %v", closeErr)
	}
	if err != windows.ERROR_NO_MORE_FILES {
		t.Errorf("native thread enumeration: %v", err)
	}
	// Fixed storage: no diagnostic handle is opened for per-handle queries.
	// Snapshot identity is only contemporaneous, unlike the generation ledger.
	var data struct {
		count, reserved uintptr
		entries         [2048]qualificationHandleEntry
	}
	var size uint32
	err = windows.NtQueryInformationProcess(windows.CurrentProcess(), 51, unsafe.Pointer(&data), uint32(unsafe.Sizeof(data)), &size)
	if err != nil {
		t.Errorf("native handle snapshot: %v required_bytes=%d", err, size)
		return
	}
	if data.count > uintptr(len(data.entries)) {
		t.Errorf("native handle snapshot overflow: %d", data.count)
		return
	}
	t.Logf("native handle snapshot count=%d; types/TIDs are metadata, not runtime ownership proof", data.count)
	for _, h := range data.entries[:data.count] {
		var object [512]uintptr // aligned OBJECT_TYPE_INFORMATION, including its name
		status, _, _ := qualificationQueryObject.Call(uintptr(h.handle), 2, uintptr(unsafe.Pointer(&object)), unsafe.Sizeof(object), uintptr(unsafe.Pointer(&size)))
		name := "unknown"
		if status == 0 {
			name = (*windows.NTUnicodeString)(unsafe.Pointer(&object)).String()
		}
		var tid uintptr
		if name == "Thread" {
			tid, _, _ = qualificationThreadID.Call(uintptr(h.handle))
		}
		t.Logf("native handle=%#x type=%s index=%d access=%#x attributes=%#x references=%d/%d target_tid=%d query_status=%#x", h.handle, name, h.typeIndex, h.access, h.attributes, h.handleCount, h.pointerCount, tid, status)
	}
}
