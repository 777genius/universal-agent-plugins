//go:build linux && 386

package pluginkitai

import (
	"runtime"
	"syscall"
	"unsafe"
)

// Linux/386 uses socketcall subnumbers from linux/net.h, not direct name traps.
func cursorSocketName(fd int, peer bool, address *syscall.RawSockaddrUnix, size *uint32) syscall.Errno {
	call := uintptr(6) // GETSOCKNAME
	if peer {
		call = 7 // GETPEERNAME
	}
	// The kernel follows pointers stored in a uintptr argument array. Direct
	// syscall keepalive protects that array, but not its nested integer values.
	// Pin both native objects BEFORE conversion: Pin makes them escape to the
	// heap and retains them at fixed addresses even across Go stack movement/GC.
	// Deferred Unpin keeps the Pinner live and releases both pins on every exit.
	var pinned runtime.Pinner
	pinned.Pin(address)
	defer pinned.Unpin()
	pinned.Pin(size)
	// Exact-size zeroed sockaddr and uint32 socklen, no pointer arithmetic or
	// slices. Caller supplies capacity and checks returned length/family. These
	// synchronous name calls retain no pointers after return.
	args := [3]uintptr{uintptr(fd), uintptr(unsafe.Pointer(address)), uintptr(unsafe.Pointer(size))} // #nosec G103 -- nested native pointers retained and pinned above through socketcall.
	// Convert the array pointer only in the syscall expression (unsafe rule 4).
	_, _, errno := syscall.Syscall(syscall.SYS_SOCKETCALL, call, uintptr(unsafe.Pointer(&args[0])), 0) // #nosec G103 -- audited synchronous socketcall array boundary above.
	return errno
}
