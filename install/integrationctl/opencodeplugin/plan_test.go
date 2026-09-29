package opencodeplugin

import (
	"path/filepath"
	"testing"
)

const (
	oldDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	newDigest = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func baseInput() Input {
	return Input{HomeDir: filepath.Join(string(filepath.Separator), "home", "user"), FileName: "agent-notifications.js", DesiredSHA256: newDigest}
}

// Red if XDG is ignored, override loses precedence, or an invalid selected
// root silently falls back to the home directory.
func TestRootPrecedence(t *testing.T) {
	in := baseInput()
	base, err := Plan(in)
	if err != nil || base.Root != filepath.Join(in.HomeDir, ".config", "opencode") {
		t.Fatalf("home root: %+v, %v", base, err)
	}
	in.XDGConfigHome = filepath.Join(string(filepath.Separator), "custom", "config")
	xdg, err := Plan(in)
	if err != nil || xdg.Root != filepath.Join(in.XDGConfigHome, "opencode") {
		t.Fatalf("XDG root: %+v, %v", xdg, err)
	}
	in.Override = filepath.Join(string(filepath.Separator), "profile", "opencode")
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

// Red if traversal, noncanonical roots, or a snapshot from another target can
// be accepted as authority for this plugin file.
func TestRejectsEscapingAndMismatchedPaths(t *testing.T) {
	for _, name := range []string{"../foreign.js", "nested/plugin.js", `nested\plugin.js`, ".hidden.js", "plugin.ts", "bad\nname.js"} {
		in := baseInput()
		in.FileName = name
		if _, err := Plan(in); err == nil {
			t.Errorf("accepted invalid filename %q", name)
		}
	}
	in := baseInput()
	in.Override = string(filepath.Separator) + "profile/../other"
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
	in := baseInput()
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
