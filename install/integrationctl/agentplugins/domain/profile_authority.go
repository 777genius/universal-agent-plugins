// ProfileAuthority is structural evidence for future opted-in consumers, not
// an authorization grant or an OS observation. It deliberately uses only stdlib.
package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const profileAuthorityMaxEncodedBytes = 2 * 1024 * 1024

type ProfileAuthorityEntry struct {
	CanonicalPath string `json:"canonical_path"`
	Scheme        string `json:"scheme"`
	VolumeID      string `json:"volume_id"`
	ObjectID      string `json:"object_id"`
}

type ProfileAuthorityFacts struct {
	Version       int                     `json:"version"`
	CanonicalRoot string                  `json:"canonical_root"`
	Ancestry      []ProfileAuthorityEntry `json:"ancestry"`
}

// An immutable canonical encoding avoids retaining caller-owned DTO slices.
type ProfileAuthority struct{ encoded string }

func NewProfileAuthority(f ProfileAuthorityFacts) (ProfileAuthority, error) {
	if err := validateProfileAuthorityFacts(f); err != nil {
		return ProfileAuthority{}, err
	}
	b, err := json.Marshal(f)
	if err != nil {
		return ProfileAuthority{}, err
	}
	if len(b) > profileAuthorityMaxEncodedBytes {
		return ProfileAuthority{}, errors.New("authority encoding size limit")
	}
	return ProfileAuthority{encoded: string(b)}, nil
}

func (a ProfileAuthority) Facts() ProfileAuthorityFacts {
	var f ProfileAuthorityFacts
	// Only a validated constructor or decoder can populate encoded.
	_ = json.Unmarshal([]byte(a.encoded), &f)
	return f
}
func (a ProfileAuthority) IsZero() bool                  { return a.encoded == "" }
func (a ProfileAuthority) Equal(b ProfileAuthority) bool { return a == b }
func (a ProfileAuthority) MarshalJSON() ([]byte, error) {
	if a.IsZero() {
		return nil, errors.New("zero directory authority cannot be encoded")
	}
	return []byte(a.encoded), nil
}
func (a *ProfileAuthority) UnmarshalJSON(b []byte) error {
	if a == nil {
		return errors.New("nil directory authority receiver")
	}
	if len(b) > profileAuthorityMaxEncodedBytes || !utf8.Valid(b) {
		return errors.New("invalid authority encoding size or UTF-8")
	}
	if err := validProfileAuthorityJSONScalars(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if err := profileAuthorityUniqueJSON(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing authority JSON")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f ProfileAuthorityFacts
	if err := d.Decode(&f); err != nil {
		return err
	}
	next, err := NewProfileAuthority(f)
	if err != nil {
		return err
	}
	*a = next
	return nil
}

func profileAuthorityUniqueJSON(d *json.Decoder, depth int) error {
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
		if err := profileAuthorityUniqueJSON(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
