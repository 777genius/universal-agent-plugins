//go:build windows

package scaffold

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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
	_, err = Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(ctx context.Context, root string, _ *os.Root) error {
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

// Real access checks on one fresh parent, with only a duplicated thread token's
// privileges disabled. All probes and Apply stay on that locked thread.
func TestDeniedParent(t *testing.T) {
	parent := tempRoot(t)
	plan, validate := planFor(t, "skill"), realValidation(t)
	deniedParentWithoutPrivileges(t, func(sid string) {
		original, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		restore, _, err := original.DACL()
		if err != nil {
			t.Fatal(err)
		}
		control, _, err := original.Control()
		if err != nil {
			t.Fatal(err)
		}
		restoreFlags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.UNPROTECTED_DACL_SECURITY_INFORMATION)
		if control&windows.SE_DACL_PROTECTED != 0 {
			restoreFlags = windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION
		}
		restored := false
		restoreDACL := func() {
			if restored {
				return
			}
			if err := windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, restoreFlags, nil, nil, restore, nil); err != nil {
				t.Errorf("restore parent DACL: %v", err)
				return
			}
			restored = true
		}
		// Runs even on Fatal, before reverting the thread and TempDir cleanup.
		defer restoreDACL()
		deniedParentProbeCreates(t, parent, "before-deny", false)
		descriptor, err := windows.SecurityDescriptorFromString("D:P(D;;0x00000006;;;" + sid + ")(A;;FA;;;" + sid + ")")
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
		actual, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := actual.DACL()
		if err != nil || acl == nil || acl.AceCount != 2 {
			t.Fatalf("unexpected installed DACL: %v, %v", acl, err)
		}
		actualControl, _, err := actual.Control()
		if err != nil || actualControl&windows.SE_DACL_PROTECTED == 0 {
			t.Fatalf("installed DACL not protected: %v", err)
		}
		// FILE_ADD_FILE|FILE_ADD_SUBDIRECTORY, FILE_ALL_ACCESS.
		for i, mask := range []windows.ACCESS_MASK{0x6, 0x001f01ff} {
			var ace *windows.ACCESS_ALLOWED_ACE // deny/allow ACEs have the same layout
			if err := windows.GetAce(acl, uint32(i), &ace); err != nil {
				t.Fatal(err)
			}
			if ace.Header.AceType != []byte{windows.ACCESS_DENIED_ACE_TYPE, windows.ACCESS_ALLOWED_ACE_TYPE}[i] || ace.Header.AceFlags != 0 || ace.Mask != mask || (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() != sid {
				t.Fatalf("installed ACE %d differs from fixture: %s", i, actual.String())
			}
		}
		t.Logf("installed parent DACL: %s", actual.String())
		deniedParentProbeCreates(t, parent, "denied", true)
		validated := false
		r, err := Apply(context.Background(), plan, ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: func(ctx context.Context, root string, _ *os.Root) error {
			validated = true
			return validate(ctx, root)
		}})
		// makePrivateStage returns raw NTSTATUS; an unrelated validation failure
		// must not satisfy this fixture (nor does a fabricated callback error).
		if !errors.Is(err, windows.STATUS_ACCESS_DENIED) || r.Committed || validated {
			t.Fatalf("Apply: want native stage denial, uncommitted, no validation; result=%+v validated=%v err=%v", r, validated, err)
		}
		t.Logf("Apply: STATUS_ACCESS_DENIED, committed=%v, validated=%v", r.Committed, validated)
		assertOnly(t, parent)
		restoreDACL()
		if !restored {
			t.Fatal("parent DACL not restored")
		}
		deniedParentProbeCreates(t, parent, "after-restore", false)
		assertOnly(t, parent)
	})
}

// No subtests, parallel calls, or goroutines: rooted handles are acquired afresh
// in this context, and each API targets the same parent/probe pathname.
func deniedParentProbeCreates(t *testing.T, parent, phase string, denied bool) {
	t.Helper()
	root, err := openParent(parent)
	if err != nil {
		t.Fatalf("%s openParent: %v", phase, err)
	}
	defer root.Close()
	for _, probe := range []struct {
		name   string
		create func() error
		denial error
	}{
		{"os.Mkdir/CreateDirectoryW", func() error { return os.Mkdir(filepath.Join(parent, "probe"), 0700) }, fs.ErrPermission},
		{"Root.Mkdir/NtCreateFile", func() error { return root.Mkdir("probe", 0700) }, fs.ErrPermission},
		{"makePrivateStage/NtCreateFile", func() error { return makePrivateStage(root, "probe") }, windows.STATUS_ACCESS_DENIED},
	} {
		err := probe.create()
		t.Logf("%s %s: %v", phase, probe.name, err)
		if denied {
			if !errors.Is(err, probe.denial) {
				t.Fatalf("%s %s: expected actual access denial, got %v", phase, probe.name, err)
			}
		} else {
			if err != nil {
				t.Fatalf("%s %s: allowed control failed: %v", phase, probe.name, err)
			}
			if err := root.Remove("probe"); err != nil {
				t.Fatal(err)
			}
		}
	}
	assertOnly(t, parent)
}

func deniedParentWithoutPrivileges(t *testing.T, run func(sid string)) {
	t.Helper()
	runtime.LockOSThread()
	impersonating := false
	defer func() {
		if impersonating {
			if err := windows.RevertToSelf(); err != nil {
				// Never return an impersonating thread to Go's thread pool.
				// Go terminates a still-locked thread when this test goroutine exits.
				t.Errorf("RevertToSelf failed; retiring locked thread: %v", err)
				return
			}
			var remaining windows.Token
			if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &remaining); err != windows.ERROR_NO_TOKEN {
				if err == nil {
					remaining.Close()
				}
				t.Errorf("thread token remained after revert; retiring locked thread: %v", err)
				return
			}
			t.Log("thread token reverted before unlock")
		}
		runtime.UnlockOSThread()
	}()
	var ambient windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &ambient); err != windows.ERROR_NO_TOKEN {
		if err == nil {
			ambient.Close()
		}
		t.Fatalf("expected no ambient impersonation token: %v", err)
	}
	var source, duplicate windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &source); err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	user, err := source.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	deniedParentPrivileges(t, source, "process", false)
	if err := windows.DuplicateTokenEx(source, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_IMPERSONATE, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &duplicate); err != nil {
		t.Fatal(err)
	}
	defer duplicate.Close()
	// Adjust ONLY the private duplicate; no process/user/global policy changes.
	if err := windows.AdjustTokenPrivileges(duplicate, true, nil, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := windows.SetThreadToken(nil, duplicate); err != nil {
		t.Fatal(err)
	}
	impersonating = true
	effective := windows.GetCurrentThreadEffectiveToken()
	effectiveUser, err := effective.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if !effectiveUser.User.Sid.Equals(user.User.Sid) {
		t.Fatal("effective token and DACL trustee differ")
	}
	deniedParentPrivileges(t, effective, "effective", true)
	t.Logf("%s thread=%d process/effective/DACL SID=%s", runtime.Version(), windows.GetCurrentThreadId(), user.User.Sid.String())
	run(user.User.Sid.String())
}

func deniedParentPrivileges(t *testing.T, token windows.Token, label string, requireDisabled bool) {
	t.Helper()
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size); err != windows.ERROR_INSUFFICIENT_BUFFER {
		t.Fatalf("%s privilege size: %v", label, err)
	}
	buf := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buf[0], size, &size); err != nil {
		t.Fatal(err)
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buf[0])).AllPrivileges()
	for _, name := range []string{"SeBackupPrivilege", "SeRestorePrivilege"} {
		var luid windows.LUID
		if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
			t.Fatal(err)
		}
		present, enabled := false, false
		for _, p := range privileges {
			if p.Luid == luid {
				present, enabled = true, p.Attributes&windows.SE_PRIVILEGE_ENABLED != 0
			}
		}
		t.Logf("%s %s: present=%v enabled=%v", label, name, present, enabled)
	}
	if requireDisabled {
		for _, p := range privileges {
			if p.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
				t.Fatalf("effective privilege still enabled: %+v", p)
			}
		}
		t.Logf("effective token: all %d privileges disabled", len(privileges))
	}
}
