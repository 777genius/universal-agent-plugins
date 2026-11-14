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
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const readonlyRuntimeEntryLimit = 262144

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

// Only known runtime-root ownership is translated after the child has verified
// the exact namespace maps. Overflow IDs never establish new ownership.
func readonlyRuntimeFacts(facts readonlyObjectFacts, uid, gid uint32) (readonlyObjectFacts, error) {
	kind := facts.Mode & unix.S_IFMT
	if facts.Inode == 0 || facts.Links == 0 || facts.Size < 0 || facts.UID != uid || facts.GID != gid ||
		facts.Mtime.Nsec < 0 || facts.Mtime.Nsec >= 1e9 || facts.Ctime.Nsec < 0 || facts.Ctime.Nsec >= 1e9 {
		return readonlyObjectFacts{}, errors.New("readonly runtime ownership or metadata differs")
	}
	if kind != unix.S_IFDIR && kind != unix.S_IFREG && kind != unix.S_IFLNK ||
		kind != unix.S_IFLNK && facts.Mode&0022 != 0 || kind == unix.S_IFLNK && facts.Links != 1 {
		return readonlyObjectFacts{}, errors.New("readonly runtime contains an unsafe or special member")
	}
	// Runtime set-id bits are metadata, not authority. The child separately
	// requires NOSUID on every runtime mount and drops all capabilities.
	facts.UID, facts.GID = 0, 0
	return facts, nil
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
	facts, err := readonlyRuntimeFacts(before, census.uid, census.gid)
	if err != nil {
		return err
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
	if _, err := readonlyRuntimeFacts(before, census.uid, census.gid); err != nil {
		return err
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
