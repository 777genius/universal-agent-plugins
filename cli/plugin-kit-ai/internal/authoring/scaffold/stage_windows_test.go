//go:build windows

package scaffold

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsPrivateStageDACL(t *testing.T) {
	validate := realValidation(t)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tempRoot(t), "output")
	_, err = Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, root string) error {
		descriptor, err := windows.GetNamedSecurityInfo(filepath.Dir(root), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			return err
		}
		control, _, err := descriptor.Control()
		if err != nil {
			return err
		}
		if control&windows.SE_DACL_PROTECTED == 0 {
			t.Fatal("private stage inherited parent DACL")
		}
		acl, _, err := descriptor.DACL()
		if err != nil {
			return err
		}
		if acl == nil || acl.AceCount != 1 {
			t.Fatal("private stage must grant exactly the process user")
		}
		var ace *windows.ACCESS_ALLOWED_ACE
		if err = windows.GetAce(acl, 0, &ace); err != nil {
			return err
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || sid.String() != user.User.Sid.String() {
			t.Fatal("unexpected private stage trustee")
		}
		return validate(ctx, root)
	}})
	if err != nil {
		t.Fatal(err)
	}
}

// Real NTFS access denial, not POSIX chmod emulation. Only this new fixture's
// protected DACL is changed; restore it before t.TempDir cleanup.
func TestDeniedParent(t *testing.T) {
	parent := tempRoot(t)
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	original, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	restore, _, err := original.DACL()
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(D;;0x00000006;;;" + user.User.Sid.String() + ")(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	deny, _, err := descriptor.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, deny, nil); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, restore, nil); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Mkdir(filepath.Join(parent, "probe"), 0700); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("ACL denial not established: %v", err)
	}
	r, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: realValidation(t)})
	if err == nil || r.Committed {
		t.Fatal("denied parent accepted")
	}
	assertOnly(t, parent)
}
