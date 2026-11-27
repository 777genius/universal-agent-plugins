//go:build windows

package scaffold

import (
	"errors"
	"io"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows ignores POSIX mkdir mode. Supply a protected DACL at exclusive
// creation, granting only the current process user full access, inherited by
// payload children. Do not inherit a potentially broad parent DACL.
func makePrivateStage(parent *os.Root, name string) error {
	handle, err := createPrivateStage(parent, name, windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE)
	if err != nil {
		return err
	}
	return errors.Join(windows.CloseHandle(handle))
}

// Hold DELETE access from exclusive creation until Skills removes its owned
// stage. A source reader's ancestry pin denies DELETE, so it cannot descend
// into this stage and acquire a pin that prevents payload publication.
// Sharing remains permissive for the creator's rooted writes and cleanup.
func makePrivateSkillStage(parent *os.Root, name string) (io.Closer, error) {
	handle, err := createPrivateStage(parent, name, windows.FILE_LIST_DIRECTORY|windows.SYNCHRONIZE|windows.DELETE)
	if err != nil {
		return nil, err
	}
	return skillStageGuard{handle}, nil
}

type skillStageGuard struct{ handle windows.Handle }

func (g skillStageGuard) Close() error { return windows.CloseHandle(g.handle) }

func createPrivateStage(parent *os.Root, name string, access uint32) (windows.Handle, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return 0, err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return 0, err
	}
	dir, err := parent.Open(".")
	if err != nil {
		return 0, err
	}
	defer dir.Close()
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return 0, err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(dir.Fd()), ObjectName: objectName, SecurityDescriptor: descriptor}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, access, &oa, &status, nil, windows.FILE_ATTRIBUTE_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_CREATE,
		windows.FILE_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(dir)
	runtime.KeepAlive(descriptor)
	if err != nil {
		return 0, err
	}
	return handle, nil
}
