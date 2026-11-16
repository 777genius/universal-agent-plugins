//go:build linux

package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const readonlyRuntimeEntryLimit = 262144

const readonlyRuntimeDiagnosticLimit = 1536

var errReadonlyRuntimeMetadata = errors.New("readonly runtime ownership or metadata differs")

type readonlyRuntimeRow struct {
	facts readonlyObjectFacts
	link  string
}

type readonlyRuntimeAlias struct {
	facts readonlyObjectFacts
	count uint64
}

type readonlyRuntimeCensus struct {
	rows     map[string]readonlyRuntimeRow
	aliases  map[[2]uint64]readonlyRuntimeAlias
	uid, gid uint32
	mount    uint64
}

// This parent-only census retains original host IDs. The child checks the
// translated owners of the separately captured runtime roots, not these rows.
func readonlyRuntimeFacts(facts readonlyObjectFacts, uid, gid uint32) (readonlyObjectFacts, error) {
	kind := facts.Mode & unix.S_IFMT
	if err := readonlyRuntimeMetadataRefusal(facts, uid, gid); err != nil {
		return readonlyObjectFacts{}, err
	}
	if kind != unix.S_IFDIR && kind != unix.S_IFREG && kind != unix.S_IFLNK ||
		kind != unix.S_IFLNK && facts.Mode&0022 != 0 || kind == unix.S_IFLNK && facts.Links != 1 {
		return readonlyObjectFacts{}, readonlyRuntimeUnsafeRefusal(facts)
	}
	// Runtime set-id bits are metadata, not authority. The child separately
	// requires NOSUID on every runtime mount and drops all capabilities.
	return facts, nil
}

var errReadonlyRuntimeUnsafe = errors.New("readonly runtime contains an unsafe or special member")

// Preserve the original three refusal predicates while exposing their bounded
// classes. No pathname or file content is read to classify captured metadata.
func readonlyRuntimeUnsafeRefusal(facts readonlyObjectFacts) error {
	kind := facts.Mode & unix.S_IFMT
	failed := [...]bool{kind != unix.S_IFDIR && kind != unix.S_IFREG && kind != unix.S_IFLNK,
		kind != unix.S_IFLNK && facts.Mode&0022 != 0, kind == unix.S_IFLNK && facts.Links != 1}
	var predicates uint8
	for bit, invalid := range failed {
		if invalid {
			predicates |= 1 << bit
		}
	}
	return fmt.Errorf("%w: unsafe_predicates=0x%02x kind=%#o mode=%#o links=%d", errReadonlyRuntimeUnsafe,
		predicates, kind, facts.Mode, facts.Links)
}

func (census *readonlyRuntimeCensus) memberFacts(path string, facts readonlyObjectFacts) (readonlyObjectFacts, error) {
	// Root-owned, non-writable descendants may have an original package group.
	// Preserve it in the digest; never infer a host group from a child overflow ID.
	// The census root and requireRoots retain their original strict group checks.
	gid := facts.GID
	if path == "/usr" {
		gid = census.gid
	}
	return readonlyRuntimeFacts(facts, census.uid, gid)
}

func readonlyRuntimeMetadataRefusal(facts readonlyObjectFacts, uid, gid uint32) error {
	// This bitmap is the original metadata guard, in its original order.
	failed := [...]bool{facts.Inode == 0, facts.Links == 0, facts.Size < 0, facts.UID != uid, facts.GID != gid,
		facts.Mtime.Nsec < 0 || facts.Mtime.Nsec >= 1e9, facts.Ctime.Nsec < 0 || facts.Ctime.Nsec >= 1e9}
	var predicates uint8
	for bit, invalid := range failed {
		if invalid {
			predicates |= 1 << bit
		}
	}
	if predicates == 0 {
		return nil
	}
	return fmt.Errorf("%w: predicates=0x%02x dev=%d ino=%d mode=%#o links=%d size=%d uid=%d expected_uid=%d gid=%d expected_gid=%d mtime_nsec=%d ctime_nsec=%d",
		errReadonlyRuntimeMetadata, predicates, facts.Device, facts.Inode, facts.Mode, facts.Links, facts.Size,
		facts.UID, uid, facts.GID, gid, facts.Mtime.Nsec, facts.Ctime.Nsec)
}

func readonlyRuntimeMemberError(path string, err error) error {
	if (!errors.Is(err, errReadonlyRuntimeMetadata) && !errors.Is(err, errReadonlyRuntimeUnsafe)) || !readonlyAbsolute(path) || !readonlyWithin(path, "/usr") {
		return err
	}
	relative := "."
	if path != "/usr" {
		relative = strings.TrimPrefix(path, "/usr/")
	}
	preview := relative[:min(len(relative), 128)]
	digest := sha256.Sum256([]byte("agentplugins-readonly-runtime-member-v1\x00" + relative))
	result := fmt.Errorf("%w; member=%s member_bytes=%d member_sha256=%x member_truncated=%t", err,
		strconv.QuoteToASCII(preview), len(relative), digest, len(relative) > len(preview))
	if len(result.Error()) > readonlyRuntimeDiagnosticLimit {
		return err
	}
	return result
}

func censusReadonlyRuntime(ctx context.Context, roots [4]readonlyRuntimeRoot, uid, gid uint32) (result readonlyCensusProof, resultErr error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	original, path, facts := roots[0].object.file, roots[0].object.path, roots[0].object.facts
	if original == nil || path != "/usr" || facts.Mode&unix.S_IFMT != unix.S_IFDIR {
		return result, errors.New("readonly runtime census requires its fixed original root")
	}
	file, err := openReadonlyDirectory(original, ".")
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if err := requireReadonlyHeld(file, facts); err != nil {
		return result, err
	}
	census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias), uid: uid, gid: gid}
	census.mount, err = readonlyMountID(file, "", unix.AT_EMPTY_PATH)
	if err != nil {
		return result, err
	}
	if err := requireReadonlyMount(nil, path, 0, census.mount); err != nil {
		return result, err
	}
	if err := census.directory(ctx, file, path, facts, 0); err != nil {
		return result, err
	}
	if err := census.requireAliases(ctx); err != nil {
		return result, err
	}
	if err := census.requireRoots(ctx, roots); err != nil {
		return result, err
	}
	return digestReadonlyRuntime(ctx, census.rows)
}

func (census *readonlyRuntimeCensus) record(path string, before readonlyObjectFacts, link string) error {
	if !readonlyAbsolute(path) || !readonlyWithin(path, "/usr") || len(census.rows) >= readonlyRuntimeEntryLimit {
		return errors.New("readonly runtime census bound or root differs")
	}
	if _, exists := census.rows[path]; exists {
		return errors.New("readonly runtime census repeats an original path")
	}
	facts, err := census.memberFacts(path, before)
	if err != nil {
		return readonlyRuntimeMemberError(path, err)
	}
	if (facts.Mode&unix.S_IFMT == unix.S_IFLNK) != (link != "") {
		return errors.New("readonly runtime link body is inconsistent")
	}
	if facts.Mode&unix.S_IFMT == unix.S_IFREG {
		key := [2]uint64{facts.Device, facts.Inode}
		alias := census.aliases[key]
		if alias.count != 0 && alias.facts != facts {
			return errors.New("readonly runtime hardlink facts differ")
		}
		alias.facts, alias.count = facts, alias.count+1
		census.aliases[key] = alias
	}
	census.rows[path] = readonlyRuntimeRow{facts: facts, link: link}
	return nil
}

func (census *readonlyRuntimeCensus) directory(ctx context.Context, file *os.File, path string, before readonlyObjectFacts, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 32 {
		return errors.New("readonly runtime directory depth exceeded")
	}
	if err := census.record(path, before, ""); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := file.ReadDir(32)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		for _, entry := range entries {
			if err := census.entry(ctx, file, path, entry.Name(), depth); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if len(entries) == 0 {
			return errors.New("readonly runtime directory read made no progress")
		}
	}
	return errors.Join(requireReadonlyHeld(file, before), requireReadonlyMount(file, "", unix.AT_EMPTY_PATH, census.mount), ctx.Err())
}

func (census *readonlyRuntimeCensus) entry(ctx context.Context, parent *os.File, path, name string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "/\x00\r\n") {
		return errors.New("readonly runtime member name differs")
	}
	if err := requireReadonlyMount(parent, name, 0, census.mount); err != nil {
		return err
	}
	before, err := readonlyNamedFacts(parent, name)
	if err != nil {
		return err
	}
	path = filepath.Join(path, name)
	if before.Mode&unix.S_IFMT == unix.S_IFDIR {
		return census.child(ctx, parent, name, path, before, depth+1)
	}
	link := ""
	if before.Mode&unix.S_IFMT == unix.S_IFLNK {
		link, err = readonlyLinkAt(parent, name)
		if err != nil {
			return err
		}
	}
	if err := census.record(path, before, link); err != nil {
		return err
	}
	return errors.Join(requireReadonlyNamed(parent, name, before), requireReadonlyMount(parent, name, 0, census.mount))
}

func (census *readonlyRuntimeCensus) child(ctx context.Context, parent *os.File, name, path string, before readonlyObjectFacts, depth int) (resultErr error) {
	if _, err := census.memberFacts(path, before); err != nil {
		return readonlyRuntimeMemberError(path, err)
	}
	file, err := openReadonlyDirectory(parent, name)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if err := requireReadonlyHeld(file, before); err != nil {
		return err
	}
	if err := census.directory(ctx, file, path, before, depth); err != nil {
		return err
	}
	return errors.Join(requireReadonlyNamed(parent, name, before), requireReadonlyMount(parent, name, 0, census.mount))
}

func readonlyLinkAt(parent *os.File, name string) (string, error) {
	buffer := make([]byte, 4097)
	n, err := unix.Readlinkat(int(parent.Fd()), name, buffer)
	runtime.KeepAlive(parent)
	if err != nil {
		return "", err
	}
	if n <= 0 || n > 4096 || !utf8.Valid(buffer[:n]) || strings.ContainsAny(string(buffer[:n]), "\x00\r\n") {
		return "", errors.New("readonly runtime link body exceeds its bound")
	}
	return string(buffer[:n]), nil
}

func (census *readonlyRuntimeCensus) requireAliases(ctx context.Context) error {
	for _, alias := range census.aliases {
		if err := ctx.Err(); err != nil {
			return err
		}
		if alias.count != alias.facts.Links {
			return errors.New("readonly runtime has an external hardlink")
		}
	}
	return nil
}

func digestReadonlyRuntime(ctx context.Context, rows map[string]readonlyRuntimeRow) (readonlyCensusProof, error) {
	if len(rows) == 0 || len(rows) > readonlyRuntimeEntryLimit {
		return readonlyCensusProof{}, errors.New("readonly runtime digest count differs")
	}
	keys := make([]string, 0, len(rows))
	for path := range rows {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, path := range keys {
		if err := ctx.Err(); err != nil {
			return readonlyCensusProof{}, err
		}
		row := rows[path]
		facts := row.facts
		_, _ = fmt.Fprintf(hash, "%s\x00%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\x00%s\n", path,
			facts.Device, facts.Inode, facts.Mode, facts.Links, facts.UID, facts.GID, facts.Size,
			facts.Mtime.Sec, facts.Mtime.Nsec, facts.Ctime.Sec, facts.Ctime.Nsec, row.link)
	}
	return readonlyCensusProof{SHA256: hex.EncodeToString(hash.Sum(nil)), Count: len(rows)}, nil
}
