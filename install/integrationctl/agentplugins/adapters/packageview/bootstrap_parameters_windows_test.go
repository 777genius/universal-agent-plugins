//go:build windows && amd64

package packageview

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This is a test-only acquisition diagnostic, not a supported-profile fallback.
// Every call uses the same literal fixed-drive root as production, attributes
// plus SYNCHRONIZE only, and identical sharing/reparse/no-recall protections.
// No call lists a directory, reads data, upgrades a handle or changes timestamps.
func bootstrapParameterMatrix(t *testing.T, drive string) {
	t.Helper()
	uDrive, err := windows.UTF16PtrFromString(drive)
	bootstrapCheck(t, "parameters/drive-UTF16", err)
	if windows.GetDriveType(uDrive) != windows.DRIVE_FIXED {
		t.Fatal("parameter diagnostic requires the fixture's fixed drive")
	}
	const access = windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE
	const protected = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_NO_RECALL
	const options = protected | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_DIRECTORY_FILE
	// Pin the numeric domains separately. In particular Win32 OPEN_NO_RECALL is
	// 0x00100000, which in NT CreateOptions means FILE_RESERVE_OPFILTER instead.
	// BACKUP_SEMANTICS is likewise not NT OPEN_FOR_BACKUP_INTENT. These checks
	// describe the pinned bindings; they do not prove kernel acceptance.
	if access != 0x00100080 || options != 0x00604021 || winShare != 7 ||
		windows.FILE_FLAG_OPEN_NO_RECALL != 0x00100000 || windows.FILE_OPEN_NO_RECALL != 0x00400000 ||
		windows.FILE_RESERVE_OPFILTER != windows.FILE_FLAG_OPEN_NO_RECALL ||
		windows.FILE_FLAG_BACKUP_SEMANTICS != 0x02000000 || windows.FILE_OPEN_FOR_BACKUP_INTENT != 0x00004000 {
		t.Fatal("pinned Windows access/options/Win32 flag domains changed")
	}
	u, err := windows.NewNTUnicodeString(`\??\` + drive)
	bootstrapCheck(t, "parameters/NTUnicodeString", err)
	oa := windows.OBJECT_ATTRIBUTES{ObjectName: u, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var iosb windows.IO_STATUS_BLOCK
	if oa.Length != 48 || unsafe.Offsetof(oa.RootDirectory) != 8 || unsafe.Offsetof(oa.ObjectName) != 16 ||
		unsafe.Offsetof(oa.Attributes) != 24 || unsafe.Offsetof(oa.SecurityDescriptor) != 32 || unsafe.Offsetof(oa.SecurityQoS) != 40 ||
		unsafe.Sizeof(*u) != 16 || unsafe.Offsetof(u.Buffer) != 8 || u.Length != 14 || u.MaximumLength != 16 ||
		unsafe.Sizeof(iosb) != 16 || unsafe.Offsetof(iosb.Information) != 8 {
		t.Fatal("Windows amd64 NT acquisition ABI mismatch")
	}
	t.Logf("parameters ABI: OBJECT_ATTRIBUTES=%d UNICODE_STRING=%d IO_STATUS_BLOCK=%d name=%q length=%d maximum=%d", oa.Length, unsafe.Sizeof(*u), unsafe.Sizeof(iosb), u.String(), u.Length, u.MaximumLength)
	// Resolve the same export independently and use Go's SyscallN for the exact
	// eleven arguments, alongside x/sys v0.35.0's generated Syscall12 binding.
	proc := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateFile")
	bootstrapCheck(t, "parameters/NtCreateFile-export", proc.Find())
	for _, tc := range []struct {
		name       string
		options    uint32
		attributes uint32
		objectCase uint32
		allocation bool
		syscallN   bool
	}{
		{"exact-xsys", options, 0, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"exact-SyscallN", options, 0, windows.OBJ_CASE_INSENSITIVE, false, true},
		{"normal-file-attributes", options, windows.FILE_ATTRIBUTE_NORMAL, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"no-directory-constraint", options &^ windows.FILE_DIRECTORY_FILE, 0, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"no-backup-intent", options &^ windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"no-directory-or-backup", options &^ (windows.FILE_DIRECTORY_FILE | windows.FILE_OPEN_FOR_BACKUP_INTENT), 0, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"synchronous-alert", options&^windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_SYNCHRONOUS_IO_ALERT, 0, windows.OBJ_CASE_INSENSITIVE, false, false},
		{"case-sensitive-object", options, 0, 0, false, false},
		{"explicit-zero-allocation", options, 0, windows.OBJ_CASE_INSENSITIVE, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.options&protected != protected {
				t.Fatal("diagnostic must retain reparse and no-recall protections")
			}
			attrs := oa
			attrs.Attributes = tc.objectCase
			var allocation int64
			var size *int64
			if tc.allocation {
				size = &allocation
			}
			var h windows.Handle
			var result windows.IO_STATUS_BLOCK
			t.Logf("NtCreateFile access=0x%08x object-attributes=0x%08x root=0 file-attributes=0x%08x share=0x%08x disposition=0x%08x options=0x%08x allocation-pointer=%t ea=NULL/0", access, attrs.Attributes, tc.attributes, winShare, windows.FILE_OPEN, tc.options, tc.allocation)
			var err error
			if tc.syscallN {
				status, _, _ := syscall.SyscallN(proc.Addr(), uintptr(unsafe.Pointer(&h)), uintptr(access), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&result)), uintptr(unsafe.Pointer(size)), uintptr(tc.attributes), uintptr(winShare), uintptr(windows.FILE_OPEN), uintptr(tc.options), 0, 0)
				// NtCreateFile returns NTSTATUS, never a Win32 last-error value.
				if uint32(status) != 0 {
					err = windows.NTStatus(status)
				}
			} else {
				err = windows.NtCreateFile(&h, access, &attrs, &result, size, tc.attributes, winShare, windows.FILE_OPEN, tc.options, 0, 0)
			}
			runtime.KeepAlive(u)
			if err != nil {
				// Alternate parameter failures are evidence, not assertions that a
				// different production profile should be supported. Keep probing.
				t.Logf("result NTSTATUS=0x%08x raw=%T: %v", uint32(err.(windows.NTStatus)), err, err)
				return
			}
			t.Logf("result NTSTATUS=0x00000000 iosb.status=0x%08x information=%d", uint32(result.Status), result.Information)
			bootstrapCheck(t, "parameters/CloseHandle", windows.CloseHandle(h))
		})
	}
	// Always require the actual production wrapper independently. A successful
	// alternate call is only diagnostic evidence and cannot satisfy this gate.
	f, err := winOpen(0, `\??\`+drive, true)
	bootstrapCheck(t, "parameters/production-winOpen", err)
	bootstrapCheck(t, "parameters/production-close", f.Close())
}
