package hermes

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

func validateSessionHandle(file *os.File, _ os.FileInfo) error {
	var metadata syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &metadata); err != nil {
		return err
	}
	if metadata.NumberOfLinks != 1 || metadata.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("hermes session source contains a physical alias")
	}
	return validateSessionWindowsOwner(file)
}

func validateSessionOwnedHandle(file *os.File, info os.FileInfo) error {
	return validateSessionHandle(file, info)
}

func validateSessionOwnedDirectory(file *os.File, info os.FileInfo) error {
	return validateSessionDirectoryHandle(file, info)
}

func validateSessionDirectory(info os.FileInfo) error {
	metadata, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || !info.IsDir() || metadata.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("hermes selected directory is not ordinary")
	}
	return nil
}

func validateSessionDirectoryHandle(file *os.File, info os.FileInfo) error {
	if err := validateSessionDirectory(info); err != nil {
		return err
	}
	return validateSessionWindowsOwner(file)
}

func validateSessionWindowsOwner(file *os.File) error {
	expected, err := sessionTokenDefaultOwner(windows.GetCurrentProcessToken(), windows.GetTokenInformation)
	if err != nil {
		return err
	}
	descriptor, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.Equals(expected) {
		return fmt.Errorf("hermes selected source owner differs from the current token default owner")
	}
	return nil
}

// os.Root creates objects with the token's default owner. On elevated Windows
// tokens that can be an eligible group SID rather than TokenUser. Require that
// exact TokenOwner, without accepting other groups or changing existing ACLs.
func sessionTokenDefaultOwner(token windows.Token, query func(windows.Token, uint32, *byte, uint32, *uint32) error) (*windows.SID, error) {
	var size uint32
	if err := query(token, windows.TokenOwner, nil, 0, &size); !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, errors.Join(fmt.Errorf("hermes token default owner size unavailable"), err)
	}
	const maxOwnerBytes = 65536
	const header = uint32(strconv.IntSize / 8)
	if size < header+8 || size > maxOwnerBytes {
		return nil, fmt.Errorf("hermes token default owner exceeds bounds")
	}
	allocated := size
	body := make([]byte, size)
	var pinned runtime.Pinner
	pinned.Pin(&body[0])
	defer pinned.Unpin()
	if err := query(token, windows.TokenOwner, &body[0], allocated, &size); err != nil {
		return nil, err
	}
	if size < header+8 || size > allocated {
		return nil, fmt.Errorf("hermes token default owner changed size")
	}
	return sessionTokenOwnerSID(body[:size])
}

// Decode only offsets into the pinned, owned response. The ABI's pointer is
// never converted back into a Go pointer or used to read foreign memory.
func sessionTokenOwnerSID(body []byte) (*windows.SID, error) {
	const header = strconv.IntSize / 8
	if len(body) < header+8 {
		return nil, fmt.Errorf("hermes token default owner header is truncated")
	}
	selected := uint64(binary.LittleEndian.Uint32(body[:header]))
	if header == 8 {
		selected = binary.LittleEndian.Uint64(body[:header])
	}
	start := uint64(reflect.ValueOf(body).Pointer())
	if selected < start || selected-start < header || selected-start > uint64(len(body))-8 {
		return nil, fmt.Errorf("hermes token default owner is outside the returned buffer")
	}
	sid := body[selected-start:]
	count := int(sid[1])
	if sid[0] != 1 || count > 15 || len(sid) < 8+4*count {
		return nil, fmt.Errorf("hermes token default owner is invalid")
	}
	var authority uint64
	for _, part := range sid[2:8] {
		authority = authority<<8 | uint64(part)
	}
	value := fmt.Sprintf("S-1-%d", authority)
	if authority >= 1<<32 {
		value = fmt.Sprintf("S-1-0x%012x", authority)
	}
	for index := range count {
		value += fmt.Sprintf("-%d", binary.LittleEndian.Uint32(sid[8+4*index:]))
	}
	owner, err := windows.StringToSid(value)
	if err != nil || owner == nil || !owner.IsValid() || owner.Len() != 8+4*count {
		return nil, errors.Join(fmt.Errorf("hermes token default owner did not round-trip"), err)
	}
	if owner.IdentifierAuthority().Value != [6]byte(sid[2:8]) || int(owner.SubAuthorityCount()) != count {
		return nil, fmt.Errorf("hermes token default owner components differ")
	}
	for index := range count {
		if owner.SubAuthority(uint32(index)) != binary.LittleEndian.Uint32(sid[8+4*index:]) {
			return nil, fmt.Errorf("hermes token default owner sub-authority differs")
		}
	}
	return owner, nil
}

func openSessionFile(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
