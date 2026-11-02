package opencode

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"golang.org/x/sys/windows"
)

// FILE_ALL_ACCESS: standard required rights, synchronization, and file rights.
const transitionWindowsFullAccess windows.ACCESS_MASK = 0x001f01ff

func writePrivateTransitionJournal(path string, body []byte) (resultErr error) {
	if info, err := os.Lstat(path); err == nil {
		if err := validateTransitionPrivacy(path, info, false); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), transitionRecordFile+".tmp-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer func() {
		if file != nil {
			resultErr = errors.Join(resultErr, file.Close())
		}
		if err := os.Remove(temp); err != nil && !os.IsNotExist(err) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	// The empty temp already inherits the protected root's user-only ACL.
	// Elevated tokens default to Administrators ownership; bind its owner to
	// the user before writing any recovery preimage or journal authority.
	if err := windows.SetNamedSecurityInfo(temp, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := validateTransitionPrivacy(temp, info, false); err != nil {
		return err
	}
	if written, err := file.Write(body); err != nil {
		return err
	} else if written != len(body) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	err = file.Close()
	file = nil
	if err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	return atomicfile.SyncDirectory(filepath.Dir(path))
}

func createPrivateTransitionRoot(parent string) (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	// Set owner explicitly: elevated processes otherwise default to the
	// Administrators group. Protected OICI grants let atomic journal temps
	// inherit only this user's access, without a post-creation exposure window.
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return "", err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	for range 10 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		path := filepath.Join(parent, fmt.Sprintf(".agentplugins-native-%x", random))
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return "", err
		}
		err = windows.CreateDirectory(name, &attributes)
		runtime.KeepAlive(sd)
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			continue
		}
		if err != nil {
			return "", &os.PathError{Op: "mkdir", Path: path, Err: err}
		}
		info, err := os.Lstat(path)
		if err == nil {
			err = validateTransitionPrivacy(path, info, true)
		}
		if err != nil {
			return "", errors.Join(err, os.Remove(path))
		}
		return path, nil
	}
	return "", fmt.Errorf("cannot allocate private transition directory")
}

func validateTransitionPrivacy(path string, info os.FileInfo, directory bool) error {
	if directory != info.IsDir() || !directory && (!info.Mode().IsRegular() || info.Mode().Perm()&0200 == 0) {
		return fmt.Errorf("transition path must be writable and private")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return fmt.Errorf("transition owner must be the current user")
	}
	control, _, err := sd.Control()
	if err != nil || directory && control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("transition root must reject inherited access")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount == 0 {
		return fmt.Errorf("transition DACL must grant only the current user")
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		// Fail closed for object/callback/deny ACEs rather than interpreting a
		// different layout as ACCESS_ALLOWED_ACE. No broad or foreign grant,
		// including inherit-only grants, may become journal authority.
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != transitionWindowsFullAccess {
			return fmt.Errorf("unsupported transition access grant")
		}
		flags := ace.Header.AceFlags
		if directory && flags != windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE || !directory && flags&^uint8(windows.INHERITED_ACE) != 0 {
			return fmt.Errorf("unsafe transition access inheritance")
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.IsValid() || !sid.Equals(user.User.Sid) {
			return fmt.Errorf("transition access must belong only to the current user")
		}
	}
	runtime.KeepAlive(sd)
	return nil
}
