//go:build windows

package scaffold

import (
	"context"
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
