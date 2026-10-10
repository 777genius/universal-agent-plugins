package opencode

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Real FlushFileBuffers must succeed without altering writable skill content,
// and readonly data must keep its bytes/attributes when durability is denied.
func TestWindowsNativeTransitionSyncPreservesBytes(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		name, mode := "writable", os.FileMode(0600)
		if readonly {
			name, mode = "readonly", 0400
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "SKILL.md")
			body := []byte("TEST skill preimage must stay intact\n")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0600) })
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			err = syncTransitionTree(root)
			if readonly && !errors.Is(err, windows.ERROR_ACCESS_DENIED) || !readonly && err != nil {
				t.Fatalf("readonly=%v: unexpected synchronization result: %v", readonly, err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, body) || after.Mode() != before.Mode() {
				t.Fatalf("synchronization changed skill bytes or mode: %q, %v", got, err)
			}
		})
	}
}

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
		{"root_inherited_grant", "D:(A;OICI;FA;;;" + sid + ")", true},
		{"root_inherit_only_foreign_grant", "D:P(A;OICI;FA;;;" + sid + ")(A;OICIIO;FR;;;WD)", true},
		{"journal_broad_grant", "D:P(A;;FA;;;" + sid + ")(A;;FR;;;WD)", false},
		{"journal_foreign_grant", "D:P(A;;FA;;;" + sid + ")(A;;FR;;;AU)", false},
		{"journal_null_dacl", "D:NO_ACCESS_CONTROL", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, root := privateTransitionJournalFixture(t, false)
			path := filepath.Join(root, transitionRecordFile)
			if tc.directory {
				path = root
			}
			if tc.name == "root_inherited_grant" {
				// Inheritance must originate from a real parent. A synthetic ID
				// flag on a protected root is normalized by SetNamedSecurityInfo.
				parent := filepath.Dir(root)
				setTransitionTestDACL(t, parent, "D:P(A;OICI;FA;;;"+sid+")(A;OICI;FR;;;WD)")
				t.Cleanup(func() { setTransitionTestDACL(t, parent, "D:P(A;OICI;FA;;;"+sid+")") })
			}
			descriptor := setTransitionTestDACL(t, path, tc.acl)
			restoreFlags := ""
			if tc.directory {
				restoreFlags = "OICI"
			}
			t.Cleanup(func() { setTransitionTestDACL(t, path, "D:P(A;"+restoreFlags+";FA;;;"+sid+")") })
			switch tc.name {
			case "root_inherited_grant":
				assertTransitionTestGrant(t, descriptor, "S-1-1-0", windows.INHERITED_ACE)
			case "root_inherit_only_foreign_grant":
				// Inherit-only grants must be tested on a container: the OS
				// can remove meaningless child-inheritance ACEs from files.
				assertTransitionTestGrant(t, descriptor, "S-1-1-0", windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE|windows.INHERIT_ONLY_ACE)
			}
			if _, err := readTransitionRecord(root); err == nil {
				t.Fatalf("unsafe Windows transition descriptor was accepted: %s", descriptor.String())
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

func setTransitionTestDACL(t *testing.T, path, sddl string) *windows.SECURITY_DESCRIPTOR {
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
	saved, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("persisted Windows descriptor: %s", saved.String())
	return saved
}

func assertTransitionTestGrant(t *testing.T, descriptor *windows.SECURITY_DESCRIPTOR, sidString string, flags uint8) {
	t.Helper()
	sid, err := windows.StringToSid(sidString)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := descriptor.DACL()
	if err != nil || acl == nil {
		t.Fatalf("unsafe grant injection lost DACL: %v, %s", err, descriptor.String())
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			t.Fatal(err)
		}
		if ace.Header.AceType == windows.ACCESS_ALLOWED_ACE_TYPE && ace.Header.AceFlags&flags == flags && ace.Mask != 0 && (*windows.SID)(unsafe.Pointer(&ace.SidStart)).Equals(sid) {
			return
		}
	}
	t.Fatalf("OS did not retain unsafe grant SID=%s flags=%#x: %s", sidString, flags, descriptor.String())
}
