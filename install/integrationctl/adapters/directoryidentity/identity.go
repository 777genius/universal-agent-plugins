// Package directoryidentity records directory incarnations, without discovery,
// creation, inventory or effect authorization. A structural Authority is not a
// filesystem observation: only Capture and Revalidate perform that observation.
package directoryidentity

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxEncodedBytes = 2 * 1024 * 1024

var (
	ErrUnsupported       = errors.New("durable directory identity unsupported")
	ErrBootstrapRequired = errors.New("profile_bootstrap_required")
	ErrChanged           = errors.New("directory authority changed")
)

type Entry struct {
	CanonicalPath string `json:"canonical_path"`
	Scheme        string `json:"scheme"`
	VolumeID      string `json:"volume_id"`
	ObjectID      string `json:"object_id"`
}

type Facts struct {
	Version       int     `json:"version"`
	CanonicalRoot string  `json:"canonical_root"`
	Ancestry      []Entry `json:"ancestry"`
}

// An immutable canonical encoding avoids retaining caller-owned DTO slices.
type Authority struct{ encoded string }

func NewAuthority(f Facts) (Authority, error) {
	if err := validateFacts(f); err != nil {
		return Authority{}, err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return Authority{}, err
	}
	if len(b) > MaxEncodedBytes {
		return Authority{}, errors.New("authority encoding size limit")
	}
	return Authority{encoded: string(b)}, nil
}

func (a Authority) Facts() Facts {
	var f Facts
	// Only a validated constructor or decoder can populate encoded.
	_ = json.Unmarshal([]byte(a.encoded), &f)
	return f
}
func (a Authority) IsZero() bool           { return a.encoded == "" }
func (a Authority) Equal(b Authority) bool { return a == b }
func (a Authority) MarshalJSON() ([]byte, error) {
	if a.IsZero() {
		return nil, errors.New("zero directory authority cannot be encoded")
	}
	return []byte(a.encoded), nil
}
func (a *Authority) UnmarshalJSON(b []byte) error {
	if a == nil {
		return errors.New("nil directory authority receiver")
	}
	if len(b) > MaxEncodedBytes || !utf8.Valid(b) {
		return errors.New("invalid authority encoding size or UTF-8")
	}
	if err := validJSONScalars(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := uniqueJSON(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing authority JSON")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f Facts
	if err := d.Decode(&f); err != nil {
		return err
	}
	next, err := NewAuthority(f)
	if err != nil {
		return err
	}
	*a = next
	return nil
}

func uniqueJSON(d *json.Decoder, depth int) error {
	if depth > 8 {
		return errors.New("authority JSON nesting limit")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	start, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if start == '{' {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return errors.New("duplicate authority JSON field")
			}
			// encoding/json also matches case aliases; accept only the exact
			// decoded v1 keys at the root and ancestry-entry object depths.
			known := (depth == 0 && (s == "version" || s == "canonical_root" || s == "ancestry")) ||
				(depth == 2 && (s == "canonical_path" || s == "scheme" || s == "volume_id" || s == "object_id"))
			if !known {
				return errors.New("unknown authority JSON field")
			}
			seen[s] = true
		}
		if err := uniqueJSON(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}

func validateFacts(f Facts) error {
	if f.Version != 1 || len(f.Ancestry) == 0 || len(f.Ancestry) > 256 {
		return errors.New("invalid authority version or ancestry size")
	}
	scheme := f.Ancestry[0].Scheme
	if err := validatePath(f.CanonicalRoot, scheme); err != nil {
		return err
	}
	previous := ""
	seenIDs := make(map[[3]string]bool, len(f.Ancestry))
	for i, e := range f.Ancestry {
		if e.Scheme != scheme {
			return errors.New("mixed authority schemes")
		}
		if err := validatePath(e.CanonicalPath, scheme); err != nil {
			return err
		}
		parent := parentPath(e.CanonicalPath, scheme)
		if (i == 0 && parent != e.CanonicalPath) || (i > 0 && (parent != previous || parent == e.CanonicalPath)) {
			return errors.New("incomplete or unordered authority ancestry")
		}
		if err := validateIDs(e); err != nil {
			return err
		}
		key := [3]string{e.Scheme, e.VolumeID, e.ObjectID}
		if seenIDs[key] {
			return errors.New("duplicate directory authority ID")
		}
		seenIDs[key] = true
		previous = e.CanonicalPath
	}
	if previous != f.CanonicalRoot {
		return errors.New("authority root does not end ancestry")
	}
	return nil
}

func validatePath(p, scheme string) error {
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) {
		return errors.New("invalid authority path length or UTF-8")
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return errors.New("authority path contains controls")
		}
	}
	if scheme == "windows-ntfs-volume-fileid-v1" {
		return validateWindowsPath(p)
	}
	if !path.IsAbs(p) || path.Clean(p) != p {
		return errors.New("authority path must be clean and absolute")
	}
	return nil
}

func validateWindowsPath(p string) error {
	if len(p) < 3 || p[1:3] != `:\` || p[0] < 'A' || p[0] > 'Z' {
		return errors.New("authority requires canonical drive path")
	}
	if len(p) == 3 {
		return nil
	}
	for _, part := range strings.Split(p[3:], `\`) {
		if !validWindowsComponent(part) {
			return errors.New("invalid authority drive component")
		}
	}
	return nil
}

func validWindowsComponent(part string) bool {
	if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `/:*?"<>|`) || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
		return false
	}
	base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CLOCK$":
		return false
	}
	if len(base) != 4 {
		return true
	}
	if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
		return base[3] < '1' || base[3] > '9'
	}
	return true
}

func parentPath(p, scheme string) string {
	if scheme != "windows-ntfs-volume-fileid-v1" {
		return path.Dir(p)
	}
	if i := strings.LastIndex(p, `\`); i > 2 {
		return p[:i]
	}
	return p[:3]
}

func decimal(s string, bits int, allowZero bool) bool {
	v, err := strconv.ParseUint(s, 10, bits)
	return err == nil && strconv.FormatUint(v, 10) == s && (allowZero || v != 0)
}

func validateIDs(e Entry) error {
	if len(e.VolumeID) > 128 || len(e.ObjectID) > 128 {
		return errors.New("authority ID limit")
	}
	switch e.Scheme {
	case "linux-fsuuid-inode-v1", "darwin-voluuid-inode-v1":
		b, err := hex.DecodeString(e.VolumeID)
		if err != nil || len(b) < 1 || len(b) > 16 || hex.EncodeToString(b) != e.VolumeID || (e.Scheme == "darwin-voluuid-inode-v1" && len(b) != 16) || allZero(b) || !decimal(e.ObjectID, 64, false) {
			return errors.New("invalid UUID/inode authority IDs")
		}
	case "windows-ntfs-volume-fileid-v1":
		p := strings.Split(e.ObjectID, ":")
		if !decimal(e.VolumeID, 32, false) || len(p) != 2 || !decimal(p[0], 32, true) || !decimal(p[1], 32, true) || (p[0] == "0" && p[1] == "0") {
			return errors.New("invalid Windows authority IDs")
		}
	default:
		return fmt.Errorf("unknown authority scheme %q", e.Scheme)
	}
	return nil
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// encoding/json replaces unpaired UTF-16 escapes; identity bytes must not be
// silently repaired into a different valid filesystem spelling.
func validJSONScalars(b []byte) error {
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' {
			continue
		}
		i++
		if i >= len(b) || b[i] != 'u' {
			continue
		}
		if i+4 >= len(b) {
			return errors.New("truncated authority Unicode escape")
		}
		v, err := strconv.ParseUint(string(b[i+1:i+5]), 16, 16)
		if err != nil {
			return err
		}
		i += 4
		if v >= 0xdc00 && v <= 0xdfff {
			return errors.New("unpaired authority Unicode surrogate")
		}
		if v < 0xd800 || v > 0xdbff {
			continue
		}
		if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
			return errors.New("unpaired authority Unicode surrogate")
		}
		low, err := strconv.ParseUint(string(b[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return errors.New("unpaired authority Unicode surrogate")
		}
		i += 6
	}
	return nil
}
