package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// Each case mutates an actual NTFS descriptor on a valid, persisted transition,
// so an ACL-blind Windows fix would accept it and make this regression red.
func TestWindowsNativeTransitionRejectsUnsafePrivacy(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := user.User.Sid.String()
	for _, tc := range []struct {
		name, acl string
		directory bool
	}{
		{"root_broad_grant", "D:P(A;OICI;FA;;;" + sid + ")(A;OICI;FR;;;WD)", true},
		{"root_unprotected", "D:(A;OICI;FA;;;" + sid + ")", true},
		{"root_inherited_grant", "D:P(A;OICIID;FA;;;" + sid + ")", true},
		{"journal_broad_grant", "D:P(A;;FA;;;" + sid + ")(A;;FR;;;WD)", false},
		{"journal_foreign_grant", "D:P(A;;FA;;;" + sid + ")(A;;FR;;;AU)", false},
		{"journal_inherit_only_grant", "D:P(A;;FA;;;" + sid + ")(A;OIIO;FR;;;WD)", false},
		{"journal_null_dacl", "D:NO_ACCESS_CONTROL", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, root := privateTransitionJournalFixture(t, false)
			path := filepath.Join(root, transitionRecordFile)
			if tc.directory {
				path = root
			}
			setTransitionTestDACL(t, path, tc.acl)
			if _, err := readTransitionRecord(root); err == nil {
				t.Fatal("unsafe Windows transition descriptor was accepted")
			}
		})
	}
	for _, directory := range []bool{false, true} {
		name := "journal_foreign_owner"
		if directory {
			name = "root_foreign_owner"
		}
		t.Run(name, func(t *testing.T) {
			_, root := privateTransitionJournalFixture(t, false)
			path := filepath.Join(root, transitionRecordFile)
			if directory {
				path = root
			}
			owner, err := windows.StringToSid("S-1-5-32-544") // Administrators, never the user SID.
			if err != nil {
				t.Fatal(err)
			}
			err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, owner, nil, nil, nil)
			if err == windows.ERROR_INVALID_OWNER || err == windows.ERROR_ACCESS_DENIED || err == windows.ERROR_PRIVILEGE_NOT_HELD {
				t.Skip("changing owner requires an elevated Windows account")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := readTransitionRecord(root); err == nil {
				t.Fatal("foreign transition owner was accepted")
			}
		})
	}
	t.Run("journal_readonly", func(t *testing.T) {
		_, root := privateTransitionJournalFixture(t, false)
		path := filepath.Join(root, transitionRecordFile)
		if err := os.Chmod(path, 0400); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0600) })
		if _, err := readTransitionRecord(root); err == nil {
			t.Fatal("readonly Windows transition journal was accepted")
		}
	})
}

func setTransitionTestDACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	protection := windows.UNPROTECTED_DACL_SECURITY_INFORMATION
	if control&windows.SE_DACL_PROTECTED != 0 {
		protection = windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.SECURITY_INFORMATION(protection), nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}
