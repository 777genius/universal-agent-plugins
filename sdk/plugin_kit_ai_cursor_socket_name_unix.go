//go:build (linux && !386) || darwin

package pluginkitai

import (
	"syscall"
	"unsafe"
)

// Preserve the native uint32 socklen; the public sockaddr wrapper loses it.
func cursorSocketName(fd int, peer bool, address *syscall.RawSockaddrUnix, size *uint32) syscall.Errno {
	call := uintptr(syscall.SYS_GETSOCKNAME)
	if peer {
		call = syscall.SYS_GETPEERNAME
	}
	// The zeroed native address and uint32 length are live fixed-size objects.
	// The caller supplies exact capacity and validates returned length/family.
	// Both conversions are in this synchronous syscall expression, so Go keeps
	// the objects alive and immobile until return (unsafe.Pointer rule 4).
	// The kernel retains neither pointer; no arithmetic or derived slice occurs.
	_, _, errno := syscall.Syscall(call, uintptr(fd), uintptr(unsafe.Pointer(address)), uintptr(unsafe.Pointer(size))) // #nosec G103 -- audited synchronous native sockaddr/socklen boundary above.
	return errno
}
