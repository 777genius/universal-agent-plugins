// Package opencodeplugin plans placement of one global OpenCode local JS plugin.
// It has no filesystem or process effects. The caller owns observation and the
// compare-and-swap transaction that applies or removes the planned file.
package opencodeplugin

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// FileKind describes the object at the target path, observed without following
// its final symlink. A regular file's SHA256 is the lowercase hex digest of its
// bytes. A symlink or other object is always a conflict.
type FileKind string

const (
	Regular FileKind = "regular"
	Symlink FileKind = "symlink"
	Other   FileKind = "other"
)

// Existing is the caller's snapshot of the target. Nil means it was absent.
// Path must be the exact path returned by this planner, and SHA256 is required
// for a regular file.
type Existing struct {
	Path   string
	Kind   FileKind
	SHA256 string
}

// Input supplies explicit environment values; Plan never reads process env.
// Override is the OpenCode config root itself. XDGConfigHome is its parent.
// OwnedSHA256 is the digest from the product's trusted ownership record; empty
// means no ownership claim. DesiredSHA256 is the proposed JS bundle's digest.
type Input struct {
	HomeDir       string
	XDGConfigHome string
	Override      string
	FileName      string
	DesiredSHA256 string
	OwnedSHA256   string
	Existing      *Existing
}

type Action string

const (
	Create    Action = "create"
	Replace   Action = "replace"
	Unchanged Action = "unchanged"
	Conflict  Action = "conflict"
)

type ConflictReason string

const (
	ForeignFile ConflictReason = "foreign_file"
	ChangedFile ConflictReason = "changed_file"
	WrongKind   ConflictReason = "wrong_kind"
)

// Placement is a snapshot-based proposal, never authority to mutate the host.
// Existing is copied into the result so the caller can compare it again during
// its transaction. ConflictReason is nonempty only for Action == Conflict.
type Placement struct {
	Root           string
	Target         string
	DesiredSHA256  string
	Existing       *Existing
	Action         Action
	ConflictReason ConflictReason
}

// Plan selects exactly one global root, validates the target and classifies its
// supplied preimage. A nonempty invalid override or XDG value is an error.
func Plan(in Input) (Placement, error) {
	if !validFileName(in.FileName) {
		return Placement{}, errors.New("OpenCode plugin filename must be one .js basename")
	}
	if !validDigest(in.DesiredSHA256) {
		return Placement{}, errors.New("desired SHA256 must be 64 lowercase hex characters")
	}
	if in.OwnedSHA256 != "" && !validDigest(in.OwnedSHA256) {
		return Placement{}, errors.New("owned SHA256 must be 64 lowercase hex characters")
	}
	root, err := configRoot(in)
	if err != nil {
		return Placement{}, err
	}
	target := filepath.Join(root, "plugins", in.FileName)
	if !contained(root, target) {
		return Placement{}, errors.New("plugin target escapes OpenCode config root")
	}
	out := Placement{Root: root, Target: target, DesiredSHA256: in.DesiredSHA256}
	if in.Existing == nil {
		out.Action = Create
		return out, nil
	}
	if in.Existing.Path != target {
		return Placement{}, fmt.Errorf("observed path %q does not match target %q", in.Existing.Path, target)
	}
	observed := *in.Existing
	out.Existing = &observed
	if observed.Kind != Regular {
		if observed.Kind != Symlink && observed.Kind != Other {
			return Placement{}, fmt.Errorf("invalid existing file kind %q", observed.Kind)
		}
		out.Action, out.ConflictReason = Conflict, WrongKind
		return out, nil
	}
	if !validDigest(observed.SHA256) {
		return Placement{}, errors.New("existing regular file requires a lowercase SHA256")
	}
	if in.OwnedSHA256 == "" {
		out.Action, out.ConflictReason = Conflict, ForeignFile
	} else if observed.SHA256 != in.OwnedSHA256 {
		out.Action, out.ConflictReason = Conflict, ChangedFile
	} else if observed.SHA256 == in.DesiredSHA256 {
		out.Action = Unchanged
	} else {
		out.Action = Replace
	}
	return out, nil
}

func configRoot(in Input) (string, error) {
	if in.Override != "" {
		return absoluteClean("override", in.Override)
	}
	if in.XDGConfigHome != "" {
		base, err := absoluteClean("XDG_CONFIG_HOME", in.XDGConfigHome)
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "opencode"), nil
	}
	home, err := absoluteClean("home directory", in.HomeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode"), nil
}

func absoluteClean(label, path string) (string, error) {
	if path == "" || strings.TrimSpace(path) != path || strings.ContainsFunc(path, unicode.IsControl) || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == string(filepath.Separator) {
		return "", fmt.Errorf("%s must be a clean absolute directory path", label)
	}
	return path, nil
}

func validFileName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		filepath.Base(name) == name && !strings.ContainsAny(name, `/\`) && !strings.ContainsFunc(name, unicode.IsControl) &&
		!strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".js")
}

func validDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, ch := range digest {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func contained(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
