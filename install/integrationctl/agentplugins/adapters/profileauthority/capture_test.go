package profileauthority_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func linuxFacts() domain.ProfileAuthorityFacts {
	return domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: "/profile", Ancestry: []domain.ProfileAuthorityEntry{
		{CanonicalPath: "/", Scheme: "linux-fsuuid-inode-v1", VolumeID: "0001", ObjectID: "1"},
		{CanonicalPath: "/profile", Scheme: "linux-fsuuid-inode-v1", VolumeID: "0001", ObjectID: "18446744073709551615"},
	}}
}

func compareNeutral(t *testing.T, f domain.ProfileAuthorityFacts, wantValid bool) {
	t.Helper()
	a, err := domain.NewProfileAuthority(f)
	neutralEntries := make([]directoryidentity.Entry, len(f.Ancestry))
	for i, e := range f.Ancestry {
		neutralEntries[i] = directoryidentity.Entry{CanonicalPath: e.CanonicalPath, Scheme: e.Scheme, VolumeID: e.VolumeID, ObjectID: e.ObjectID}
	}
	n, neutralErr := directoryidentity.NewAuthority(directoryidentity.Facts{Version: f.Version, CanonicalRoot: f.CanonicalRoot, Ancestry: neutralEntries})
	if (err == nil) != wantValid || (neutralErr == nil) != wantValid {
		t.Fatalf("valid=%v; domain=%v; neutral=%v", wantValid, err, neutralErr)
	}
	if !wantValid {
		if !a.IsZero() || !n.IsZero() {
			t.Fatal("invalid constructor returned token")
		}
		return
	}
	converted, err := profileauthority.FromNeutral(n)
	if err != nil || !converted.Equal(a) {
		t.Fatalf("neutral conversion: %v", err)
	}
	back, err := profileauthority.ToNeutral(a)
	if err != nil || !back.Equal(n) {
		t.Fatalf("domain conversion: %v", err)
	}
	domainJSON, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	neutralJSON, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(domainJSON, neutralJSON) {
		t.Fatalf("different structural encodings:\n%s\n%s", domainJSON, neutralJSON)
	}
}

// Regression: constructor or bridge rejects a valid known scheme or conflates
// length-preserving Linux UUIDs with Darwin/Windows IDs. Existing domain has none.
func TestKnownSchemesAndBridge(t *testing.T) {
	f := linuxFacts()
	compareNeutral(t, f, true)
	f.Ancestry[0].VolumeID = "01"
	f.Ancestry[1].VolumeID = "00112233445566778899aabbccddeeff"
	compareNeutral(t, f, true) // A real filesystem transition may change volume IDs.
	f = linuxFacts()
	for i := range f.Ancestry {
		f.Ancestry[i].Scheme = "darwin-voluuid-inode-v1"
		f.Ancestry[i].VolumeID = "00112233445566778899aabbccddeeff"
	}
	compareNeutral(t, f, true)
	f = domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: `C:\profile`, Ancestry: []domain.ProfileAuthorityEntry{
		{CanonicalPath: `C:\`, Scheme: "windows-ntfs-volume-fileid-v1", VolumeID: "4294967295", ObjectID: "0:1"},
		{CanonicalPath: `C:\profile`, Scheme: "windows-ntfs-volume-fileid-v1", VolumeID: "1", ObjectID: "4294967295:0"},
	}}
	compareNeutral(t, f, true)
	f = linuxFacts()
	f.CanonicalRoot = "/"
	f.Ancestry = f.Ancestry[:1]
	compareNeutral(t, f, true)
	f = linuxFacts()
	f.CanonicalRoot = "/" + strings.Repeat("x", 4095)
	f.Ancestry[1].CanonicalPath = f.CanonicalRoot
	compareNeutral(t, f, true)
	f = linuxFacts()
	f.CanonicalRoot = "/目录/é"
	f.Ancestry = append(f.Ancestry, f.Ancestry[1])
	f.Ancestry[1].CanonicalPath = "/目录"
	f.Ancestry[1].ObjectID = "2"
	f.Ancestry[2].ObjectID = "3"
	f.Ancestry[2].CanonicalPath = f.CanonicalRoot
	compareNeutral(t, f, true)
}

// Regression: paths or IDs supplied as diagnostic facts can forge a token with
// missing/duplicated ancestors or unknown schemes. No structural constructor existed.
func TestInvalidFactsIndependentCases(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.ProfileAuthorityFacts)
	}{
		{"zero-version", func(f *domain.ProfileAuthorityFacts) { f.Version = 0 }},
		{"future-version", func(f *domain.ProfileAuthorityFacts) { f.Version = 2 }},
		{"empty-ancestry", func(f *domain.ProfileAuthorityFacts) { f.Ancestry = nil }},
		{"unknown-scheme", func(f *domain.ProfileAuthorityFacts) {
			for i := range f.Ancestry {
				f.Ancestry[i].Scheme = "dev-inode"
			}
		}},
		{"mixed-scheme", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].Scheme = "darwin-voluuid-inode-v1" }},
		{"incomplete-start", func(f *domain.ProfileAuthorityFacts) { f.Ancestry = f.Ancestry[1:] }},
		{"missing-middle", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/parent/profile"
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"duplicate-entry", func(f *domain.ProfileAuthorityFacts) { f.Ancestry = append(f.Ancestry, f.Ancestry[1]) }},
		{"duplicate-id", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = f.Ancestry[0].ObjectID }},
		{"unordered", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[0], f.Ancestry[1] = f.Ancestry[1], f.Ancestry[0] }},
		{"mismatched-root", func(f *domain.ProfileAuthorityFacts) { f.CanonicalRoot = "/other" }},
		{"relative-root", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "profile"
			f.Ancestry[1].CanonicalPath = "profile"
		}},
		{"unclean-root", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/profile/.."
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"trailing-slash", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/profile/"
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"double-slash", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "//profile"
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"root-too-long", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/" + strings.Repeat("x", 4096)
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"control", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/profile\u0085"
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"invalid-utf8", func(f *domain.ProfileAuthorityFacts) {
			f.CanonicalRoot = "/profile\xff"
			f.Ancestry[1].CanonicalPath = f.CanonicalRoot
		}},
		{"empty-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = "" }},
		{"zero-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = "0000" }},
		{"odd-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = "001" }},
		{"nonhex-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = "gg" }},
		{"uppercase-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = "AB" }},
		{"seventeen-byte-uuid", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = strings.Repeat("01", 17) }},
		{"uuid-id-cap", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].VolumeID = strings.Repeat("01", 65) }},
		{"zero-inode", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = "0" }},
		{"empty-inode", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = "" }},
		{"signed-inode", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = "+1" }},
		{"leading-zero-inode", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = "01" }},
		{"inode-overflow", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = "18446744073709551616" }},
		{"inode-id-cap", func(f *domain.ProfileAuthorityFacts) { f.Ancestry[1].ObjectID = strings.Repeat("1", 129) }},
		{"darwin-short-uuid", func(f *domain.ProfileAuthorityFacts) {
			for i := range f.Ancestry {
				f.Ancestry[i].Scheme = "darwin-voluuid-inode-v1"
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { f := linuxFacts(); c.mutate(&f); compareNeutral(t, f, false) })
	}
}

// Regression: permissive JSON accepts ambiguous duplicate/unknown fields, trailing
// tokens or oversized messages, and an invalid decode overwrites existing authority.
// Existing state JSON tests have no private ProfileAuthority decoder.
// Case aliases previously bypassed the exact-key duplicate walk: encoding/json
// accepted Version after version:0 and OBJECT_ID after object_id:0. Both field
// orders and aliases alone must reject without replacing either retained token.
func TestStrictJSONAndFailedDecodeRetainsValue(t *testing.T) {
	valid := `{"version":1,"canonical_root":"/profile","ancestry":[{"canonical_path":"/","scheme":"linux-fsuuid-inode-v1","volume_id":"0001","object_id":"1"},{"canonical_path":"/profile","scheme":"linux-fsuuid-inode-v1","volume_id":"0001","object_id":"2"}]}`
	cases := []struct{ name, raw string }{
		{"unpaired-surrogate", strings.ReplaceAll(valid, "profile", `\ud800`)},
		{"lone-low-surrogate", strings.ReplaceAll(valid, "profile", `\udc00`)},

		{"zero-object", `{}`}, {"null", `null`}, {"array", `[]`}, {"empty", ``},
		{"trailing-object", valid + ` {}`}, {"trailing-null", valid + ` null`},
		{"duplicate-root", strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1)},
		{"escaped-duplicate", strings.Replace(valid, `"version":1`, `"version":1,"\u0076ersion":1`, 1)},
		{"case-root-invalid-first", strings.Replace(valid, `"version":1`, `"version":0,"Version":1`, 1)},
		{"case-root-invalid-last", strings.Replace(valid, `"version":1`, `"Version":1,"version":0`, 1)},
		{"case-root-alias-only", strings.Replace(valid, `"version":1`, `"Version":1`, 1)},
		{"case-entry-invalid-first", strings.Replace(valid, `"object_id":"1"`, `"object_id":"0","OBJECT_ID":"1"`, 1)},
		{"case-entry-invalid-last", strings.Replace(valid, `"object_id":"1"`, `"OBJECT_ID":"1","object_id":"0"`, 1)},
		{"case-entry-alias-only", strings.Replace(valid, `"object_id":"1"`, `"OBJECT_ID":"1"`, 1)},
		{"duplicate-entry", strings.Replace(valid, `"object_id":"1"`, `"object_id":"1","object_id":"1"`, 1)},
		{"unknown-root", strings.Replace(valid, `"version":1`, `"version":1,"grant":true`, 1)},
		{"unknown-entry", strings.Replace(valid, `"object_id":"1"`, `"object_id":"1","grant":true`, 1)},
		{"bad-version", strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{"null-ancestry", `{"version":1,"canonical_root":"/","ancestry":null}`},
		{"truncated", valid[:len(valid)-1]},
		{"invalid-utf8", strings.Replace(valid, "profile", "profile\xff", 1)},
		{"size-cap", strings.Repeat(" ", 2*1024*1024+1)},
		{"deep-unknown", `{"future":` + strings.Repeat("[", 20) + `0` + strings.Repeat("]", 20) + `}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var a domain.ProfileAuthority
			var n directoryidentity.Authority
			if err := json.Unmarshal([]byte(valid), &a); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(valid), &n); err != nil {
				t.Fatal(err)
			}
			original, neutralOriginal := a, n
			if err := a.UnmarshalJSON([]byte(c.raw)); err == nil {
				t.Fatal("domain invalid JSON accepted")
			}
			if err := n.UnmarshalJSON([]byte(c.raw)); err == nil {
				t.Fatal("neutral invalid JSON accepted")
			}
			if !a.Equal(original) || !n.Equal(neutralOriginal) {
				t.Fatal("failed decode changed token")
			}
		})
	}
	for _, scalar := range []string{`\ud834\udd1e`, `\ufffd`, `\\ud800`} {
		literal := strings.ReplaceAll(valid, "profile", scalar)
		var d domain.ProfileAuthority
		var neutral directoryidentity.Authority
		if err := d.UnmarshalJSON([]byte(literal)); err != nil {
			t.Fatalf("valid scalar %q: %v", scalar, err)
		}
		if err := neutral.UnmarshalJSON([]byte(literal)); err != nil {
			t.Fatalf("valid neutral scalar %q: %v", scalar, err)
		}
	}

	var a domain.ProfileAuthority
	var n directoryidentity.Authority
	if err := json.Unmarshal([]byte(" \n"+valid+"\t "), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(valid), &n); err != nil {
		t.Fatal(err)
	}
	converted, err := profileauthority.FromNeutral(n)
	if err != nil || !converted.Equal(a) {
		t.Fatalf("valid decoded conversion: %v", err)
	}
}

// Regression: constructor/accessor keeps a caller slice, equality changes after
// edits, or zero encodes '{}'. The diagnostic facts type must never be a grant.
func TestDomainImmutabilityZeroAndOptionalNil(t *testing.T) {
	f := linuxFacts()
	a, err := domain.NewProfileAuthority(f)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	f.Ancestry[0].VolumeID = "ff"
	f.Ancestry[1].ObjectID = "9"
	exposed := a.Facts()
	exposed.Ancestry[0].ObjectID = "88"
	exposed.Ancestry = exposed.Ancestry[:1]
	later, err := json.Marshal(a)
	if err != nil || !bytes.Equal(original, later) {
		t.Fatalf("authority changed: %s %v", later, err)
	}
	var decoded domain.ProfileAuthority
	if err := json.Unmarshal(later, &decoded); err != nil || !a.Equal(decoded) {
		t.Fatalf("immutable roundtrip: %v", err)
	}
	changed := a.Facts()
	changed.Ancestry[1].ObjectID = "7"
	other, err := domain.NewProfileAuthority(changed)
	if err != nil || a.Equal(other) {
		t.Fatalf("different incarnation equal: %v", err)
	}
	var zero domain.ProfileAuthority
	if !zero.IsZero() || zero.Equal(a) {
		t.Fatal("bad zero")
	}
	if _, err := json.Marshal(zero); err == nil {
		t.Fatal("zero encoded grant")
	}
	if _, err := profileauthority.ToNeutral(zero); err == nil {
		t.Fatal("zero converted to neutral grant")
	}
	if _, err := profileauthority.FromNeutral(directoryidentity.Authority{}); err == nil {
		t.Fatal("neutral zero converted")
	}
	carrier := struct {
		Authority *domain.ProfileAuthority `json:"authority,omitempty"`
	}{}
	b, err := json.Marshal(carrier)
	if err != nil || string(b) != "{}" {
		t.Fatalf("optional nil: %s %v", b, err)
	}
	carrier.Authority = &zero
	if _, err := json.Marshal(carrier); err == nil {
		t.Fatal("present zero optional encoded")
	}
}

// Regression: a cap off-by-one rejects complete depth 256, accepts depth 257,
// or Windows paths/IDs accept UNC, DOS aliases, malformed serials or zero file IDs.
// Prior domain paths were ordinary strings, with no cross-OS persisted shape.
func TestAncestryCapAndWindowsShape(t *testing.T) {
	f := linuxFacts()
	f.Ancestry = f.Ancestry[:1]
	for i := 1; i < 256; i++ {
		e := f.Ancestry[0]
		e.CanonicalPath = strings.TrimSuffix(f.Ancestry[i-1].CanonicalPath, "/") + "/a"
		e.ObjectID = strconv.Itoa(i + 1)
		f.Ancestry = append(f.Ancestry, e)
	}
	f.CanonicalRoot = f.Ancestry[len(f.Ancestry)-1].CanonicalPath
	compareNeutral(t, f, true)
	last := f.Ancestry[len(f.Ancestry)-1]
	last.CanonicalPath += "/a"
	f.Ancestry = append(f.Ancestry, last)
	f.CanonicalRoot = last.CanonicalPath
	compareNeutral(t, f, false)
	for _, bad := range []string{`\\server\profile`, `C:profile`, `c:\profile`, `C:\profile\`, `C:\.\profile`, `C:\..\profile`, `C:\profile:stream`, `C:\CON`, `C:\aux.txt`, `C:\LPT1`, `C:\profile.`, `C:\profile `, `C:\a/b`, `C:\a*`, `C:\a\\b`} {
		w := domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: bad, Ancestry: []domain.ProfileAuthorityEntry{{CanonicalPath: `C:\`, Scheme: "windows-ntfs-volume-fileid-v1", VolumeID: "1", ObjectID: "0:1"}, {CanonicalPath: bad, Scheme: "windows-ntfs-volume-fileid-v1", VolumeID: "1", ObjectID: "0:2"}}}
		compareNeutral(t, w, false)
	}
	for _, ids := range [][2]string{{"0", "0:1"}, {"4294967296", "0:1"}, {"01", "0:1"}, {"1", "0:0"}, {"1", "1"}, {"1", "-1:2"}, {"1", "0:01"}, {"1", "4294967296:1"}, {"1", "0:4294967296"}, {"1", "0:1:2"}} {
		w := domain.ProfileAuthorityFacts{Version: 1, CanonicalRoot: `C:\`, Ancestry: []domain.ProfileAuthorityEntry{{CanonicalPath: `C:\`, Scheme: "windows-ntfs-volume-fileid-v1", VolumeID: ids[0], ObjectID: ids[1]}}}
		compareNeutral(t, w, false)
	}
}
