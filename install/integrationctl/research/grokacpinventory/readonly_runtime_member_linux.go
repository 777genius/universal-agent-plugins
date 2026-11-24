//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type readonlyResultProof struct {
	Authority json.RawMessage `json:"authority"`
	Stage     string          `json:"stage"`
	Control   *bool           `json:"control_complete"`
	Original  json.RawMessage `json:"original"`
	Marker    json.RawMessage `json:"control_marker"`
	Member    json.RawMessage `json:"runtime_member,omitempty"`
}

// decodeReadonlyProof admits optional metadata only on the existing negative
// runtime stage. The caller still validates all no-progress and cleanup facts.
func decodeReadonlyProof(body []byte) (readonlyResultProof, error) {
	var proof readonlyResultProof
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		return proof, err
	}
	fields := "authority stage control_complete original control_marker"
	_, hasMember := keys["runtime_member"]
	if hasMember {
		fields += " runtime_member"
	}
	if err := closed(body, &proof, fields); err != nil {
		return proof, err
	}
	if hasMember {
		if err := validateReadonlyRuntimeMember(proof.Stage, proof.Member); err != nil {
			return proof, err
		}
	}
	return proof, nil
}

func validateReadonlyRuntimeMember(stage string, body []byte) error {
	if stage != "runtime:readonly-census-writable" {
		return errors.New("readonly-result-refused")
	}
	var row struct {
		Member string  `json:"member"`
		Mode   uint32  `json:"mode"`
		UID    *uint32 `json:"uid"`
		GID    *uint32 `json:"gid"`
		Links  uint64  `json:"links"`
		Bytes  *uint64 `json:"bytes"`
	}
	if err := closed(body, &row, "member mode uid gid links bytes"); err != nil {
		return err
	}
	if !validReadonlyMemberName(row.Member) || row.Links == 0 ||
		row.UID == nil || row.GID == nil || row.Bytes == nil || *row.Bytes >= 1<<63 ||
		row.Mode&0o022 == 0 || row.Mode&unix.S_IFMT == unix.S_IFLNK {
		return errors.New("readonly-result-refused")
	}
	return nil
}

func validReadonlyMemberName(name string) bool {
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) ||
		strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, char := range name {
		if char < 32 || char == 127 {
			return false
		}
	}
	return true
}
