package vscodeprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

// SnapshotDigest binds exact bytes, including foreign comments/whitespace.
func SnapshotDigest(settings []byte) string {
	sum := sha256.Sum256(settings)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validateIdentity(id Identity) error {
	for _, s := range []string{id.SettingsPath, id.ProfileID, id.PluginRoot, id.PackageID} {
		if s == "" || len(s) > MaxIdentityBytes || !utf8.ValidString(s) || strings.ContainsAny(s, "\x00\r\n") || nativeTrim(s) != s {
			return fmt.Errorf("invalid explicit identity")
		}
	}
	for _, p := range []string{id.SettingsPath, id.PluginRoot} {
		normalized, ok := absolutePath(p)
		if !ok || normalized != p {
			return fmt.Errorf("identity requires a clean absolute path")
		}
	}
	if !validDigest(id.PackageDigest) || !validDigest(id.ProjectionDigest) {
		return fmt.Errorf("invalid package/projection digest")
	}
	return nil
}

// The native loader calls JS String.trim: ECMAScript WhiteSpace and
// LineTerminator characters, including U+FEFF but excluding U+0085.
func nativeTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\u0009', '\u000b', '\u000c', '\u0020', '\u00a0', '\u1680',
			'\u202f', '\u205f', '\u3000', '\ufeff', '\u000a', '\u000d', '\u2028', '\u2029':
			return true
		}
		return r >= '\u2000' && r <= '\u200a'
	})
}

func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && strings.ToLower(s) == s
}

// This lexical check has no host-OS dependency. UNC/device roots, relative
// paths and ambiguous slash styles are refused. The caller owns symlink/case/
// filesystem identity qualification and no-follow/CAS writes in the next slice.
func absolutePath(p string) (string, bool) {
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.Contains(p, "\\") {
		return path.Clean(p), true
	}
	if len(p) < 3 || !((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) || p[1] != ':' || p[2] != '\\' || strings.Contains(p, "/") {
		return "", false
	}
	rest := strings.ReplaceAll(p[2:], "\\", "/")
	return p[:2] + strings.ReplaceAll(path.Clean(rest), "/", "\\"), true
}

func sameBinding(a, b Identity) bool {
	return a.SettingsPath == b.SettingsPath && a.ProfileID == b.ProfileID && a.PluginRoot == b.PluginRoot && a.PackageID == b.PackageID
}

func receiptDigest(r Receipt) string {
	r.Digest = ""
	body, _ := json.Marshal(r) // fixed string/bool struct: Marshal cannot fail
	return SnapshotDigest(append([]byte("agentplugins-vscode-profile-v1\x00"), body...))
}

func newReceipt(id Identity, enabled bool) *Receipt {
	r := &Receipt{Version: "1", Selector: selector, Identity: id, Enabled: enabled}
	r.Digest = receiptDigest(*r)
	return r
}

func validatePrevious(req Request) error {
	r := req.Previous
	if r == nil {
		return fmt.Errorf("operation requires a trusted receipt")
	}
	if r.Version != "1" || r.Selector != selector || validateIdentity(r.Identity) != nil || r.Digest != receiptDigest(*r) {
		return fmt.Errorf("invalid receipt")
	}
	if !sameBinding(r.Identity, req.Identity) {
		return fmt.Errorf("recorded selector/profile/package binding differs")
	}
	if req.Action != Update && r.Identity != req.Identity {
		return fmt.Errorf("recorded package/projection differs")
	}
	return nil
}
