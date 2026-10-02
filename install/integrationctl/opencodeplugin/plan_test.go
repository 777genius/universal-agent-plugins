package opencodeplugin

import (
	"path/filepath"
	"runtime"
	"testing"
)

const (
	oldDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func baseInput(t *testing.T) Input {
	t.Helper()
	return Input{HomeDir: filepath.Join(t.TempDir(), "home", "user"), FileName: "agent-notifications.js", DesiredSHA256: newDigest}
}

// Red if XDG is ignored, override loses precedence, or an invalid selected
// root silently falls back to the home directory.
func TestRootPrecedence(t *testing.T) {
	in := baseInput(t)
	base, err := Plan(in)
	if err != nil || base.Root != filepath.Join(in.HomeDir, ".config", "opencode") {
		t.Fatalf("home root: %+v, %v", base, err)
	}
	in.XDGConfigHome = filepath.Join(t.TempDir(), "custom", "config")
	xdg, err := Plan(in)
	if err != nil || xdg.Root != filepath.Join(in.XDGConfigHome, "opencode") {
		t.Fatalf("XDG root: %+v, %v", xdg, err)
	}
	in.Override = filepath.Join(t.TempDir(), "profile", "opencode")
	override, err := Plan(in)
	if err != nil || override.Root != in.Override || override.Target != filepath.Join(in.Override, "plugins", in.FileName) {
		t.Fatalf("override root: %+v, %v", override, err)
	}
	in.Override = "relative/root"
	if _, err := Plan(in); err == nil {
		t.Fatal("relative override accepted")
	}
	in.Override = ""
	in.XDGConfigHome = "relative/config"
	if _, err := Plan(in); err == nil {
		t.Fatal("relative XDG root silently fell back")
	}
}

// Red if a root-only caller needs a filename/digest, if precedence changes, or
// if the resolver's displayed scope differs from the writer's placement.
func TestResolvedScopeMatchesInstallerAuthority(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "TEST home", "用户")
	xdg := filepath.Join(base, "TEST XDG", "配置")
	override := filepath.Join(base, "TEST profile", "OpenCode")
	for _, tc := range []struct {
		name string
		in   Input
		want string
	}{
		{"home", Input{HomeDir: home}, filepath.Join(home, ".config", "opencode")},
		{"xdg", Input{HomeDir: home, XDGConfigHome: xdg}, filepath.Join(xdg, "opencode")},
		{"override", Input{HomeDir: home, XDGConfigHome: xdg, Override: override}, override},
		{"xdg ignores invalid home", Input{HomeDir: "relative/home", XDGConfigHome: xdg}, filepath.Join(xdg, "opencode")},
		{"override ignores invalid parents", Input{HomeDir: "relative/home", XDGConfigHome: "relative/config", Override: override}, override},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ResolveConfigRoot(tc.in)
			if err != nil || root != tc.want {
				t.Fatalf("root = %q, %v; want %q", root, err, tc.want)
			}
			in := tc.in
			in.FileName, in.DesiredSHA256 = "agent-notifications.js", newDigest
			placement, err := Plan(in)
			wantTarget := filepath.Join(tc.want, "plugins", "agent-notifications.js")
			if err != nil || placement.Root != tc.want || placement.Target != wantTarget || placement.Action != Create {
				t.Fatalf("placement = %+v, %v; want root %q, target %q, create", placement, err, tc.want, wantTarget)
			}
		})
	}
	t.Run("placement fields ignored", func(t *testing.T) {
		in := Input{Override: override, FileName: "../bad.js", DesiredSHA256: "invalid", OwnedSHA256: "invalid",
			Existing: &Existing{Path: "relative", Kind: "unknown", SHA256: "invalid"}}
		root, err := ResolveConfigRoot(in)
		if err != nil || root != override {
			t.Fatalf("root-only input validated placement fields: %q, %v", root, err)
		}
	})
}

// Red if the public resolver normalizes an invalid authority, falls back to a
// valid lower-priority root, or changes the existing root diagnostic.
func TestResolveConfigRootRejectsInvalidAuthority(t *testing.T) {
	base := t.TempDir()
	home, xdg := filepath.Join(base, "TEST home"), filepath.Join(base, "TEST XDG")
	sep := string(filepath.Separator)
	for _, invalid := range []struct{ name, path string }{
		{"relative", "relative"}, {"blank", " "}, {"leading space", " " + home}, {"trailing space", home + " "},
		{"newline", home + "\n"}, {"DEL", home + "\x7f"}, {"root", sep}, {"trailing separator", home + sep},
		{"dot segment", home + sep + "."}, {"parent traversal", home + sep + ".." + sep + "other"},
	} {
		for _, tc := range []struct {
			name  string
			in    Input
			label string
		}{
			{"home", Input{HomeDir: invalid.path}, "home directory"},
			{"xdg", Input{HomeDir: home, XDGConfigHome: invalid.path}, "XDG_CONFIG_HOME"},
			{"override", Input{HomeDir: home, XDGConfigHome: xdg, Override: invalid.path}, "override"},
		} {
			t.Run(tc.name+"/"+invalid.name, func(t *testing.T) {
				root, err := ResolveConfigRoot(tc.in)
				wantError := tc.label + " must be a clean absolute directory path"
				if root != "" || err == nil || err.Error() != wantError {
					t.Fatalf("root = %q, %v; want empty root and %q", root, err, wantError)
				}
			})
		}
	}
	t.Run("missing home", func(t *testing.T) {
		root, err := ResolveConfigRoot(Input{})
		if root != "" || err == nil || err.Error() != "home directory must be a clean absolute directory path" {
			t.Fatalf("missing authority: %q, %v", root, err)
		}
	})
}

// Red if routing Plan through the public helper moves root validation before
// filename/digest checks or returns a partial placement on a validation error.
func TestPlanValidationOrder(t *testing.T) {
	for _, tc := range []struct {
		name, fileName, desired, owned, wantError string
	}{
		{"filename", "../bad.js", "invalid", "invalid", "OpenCode plugin filename must be one .js basename"},
		{"desired", "plugin.js", "invalid", "invalid", "desired SHA256 must be 64 lowercase hex characters"},
		{"owned", "plugin.js", newDigest, "invalid", "owned SHA256 must be 64 lowercase hex characters"},
		{"root", "plugin.js", newDigest, oldDigest, "override must be a clean absolute directory path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Plan(Input{Override: "relative", FileName: tc.fileName, DesiredSHA256: tc.desired, OwnedSHA256: tc.owned})
			if got != (Placement{}) || err == nil || err.Error() != tc.wantError {
				t.Fatalf("placement = %+v, %v; want zero placement and %q", got, err, tc.wantError)
			}
		})
	}
}

// Red if traversal, noncanonical roots, or a snapshot from another target can
// be accepted as authority for this plugin file.
func TestRejectsEscapingAndMismatchedPaths(t *testing.T) {
	for _, name := range []string{"../foreign.js", "nested/plugin.js", `nested\plugin.js`, ".hidden.js", "plugin.ts", "bad\nname.js"} {
		in := baseInput(t)
		in.FileName = name
		if _, err := Plan(in); err == nil {
			t.Errorf("accepted invalid filename %q", name)
		}
	}
	in := baseInput(t)
	in.Override = filepath.Join(t.TempDir(), "profile") + string(filepath.Separator) + ".." + string(filepath.Separator) + "other"
	if _, err := Plan(in); err == nil {
		t.Fatal("accepted noncanonical override")
	}
	in.Override = ""
	in.Existing = &Existing{Path: filepath.Join(in.HomeDir, ".config", "opencode", "plugins", "other.js"), Kind: Regular, SHA256: oldDigest}
	if _, err := Plan(in); err == nil {
		t.Fatal("accepted snapshot for another target")
	}
}

// Red if a foreign or edited file is treated as safely replaceable, or if a
// symlink is treated as an owned regular file.
func TestConflictFacts(t *testing.T) {
	in := baseInput(t)
	initial, err := Plan(in)
	if err != nil || initial.Action != Create {
		t.Fatalf("absent target: %+v, %v", initial, err)
	}
	in.Existing = &Existing{Path: initial.Target, Kind: Regular, SHA256: oldDigest}
	foreign, err := Plan(in)
	if err != nil || foreign.Action != Conflict || foreign.ConflictReason != ForeignFile {
		t.Fatalf("foreign file: %+v, %v", foreign, err)
	}
	in.OwnedSHA256 = newDigest
	changed, err := Plan(in)
	if err != nil || changed.Action != Conflict || changed.ConflictReason != ChangedFile {
		t.Fatalf("edited file: %+v, %v", changed, err)
	}
	in.OwnedSHA256 = oldDigest
	replace, err := Plan(in)
	if err != nil || replace.Action != Replace || replace.Existing.SHA256 != oldDigest {
		t.Fatalf("owned replacement: %+v, %v", replace, err)
	}
	in.Existing.Kind = Symlink
	in.Existing.SHA256 = ""
	symlink, err := Plan(in)
	if err != nil || symlink.Action != Conflict || symlink.ConflictReason != WrongKind {
		t.Fatalf("symlink target: %+v, %v", symlink, err)
	}
	in.Existing.Kind = Regular
	in.Existing.SHA256 = newDigest
	in.OwnedSHA256 = newDigest
	unchanged, err := Plan(in)
	if err != nil || unchanged.Action != Unchanged {
		t.Fatalf("matching owned file: %+v, %v", unchanged, err)
	}
	if unchanged.Existing == in.Existing {
		t.Fatal("result did not copy existing identity")
	}
}

// A Windows absolute path needs a volume. The planner must retain the native
// drive/UNC spelling so the product can apply its own filesystem policy.
func TestWindowsNativeRoots(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows filepath semantics")
	}
	in := baseInput(t)
	in.Override = `C:\OpenCode Test\config`
	got, err := Plan(in)
	if err != nil || got.Root != in.Override || got.Target != filepath.Join(in.Override, "plugins", in.FileName) {
		t.Fatalf("drive override: %+v, %v", got, err)
	}
	in.Override = `\OpenCode Test\config`
	if _, err := Plan(in); err == nil {
		t.Fatal("drive-relative override accepted")
	}
	in.Override = `\\server\share\opencode`
	got, err = Plan(in)
	if err != nil || got.Root != in.Override {
		t.Fatalf("UNC override was mangled: %+v, %v", got, err)
	}
}
