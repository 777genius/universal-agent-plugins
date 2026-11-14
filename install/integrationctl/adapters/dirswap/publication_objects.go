package dirswap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

const publicationObjectsPrefix = "objects-v1:"
const publicationObjectsLimit = 8
const publicationObjectsBytes int64 = 4096

// PreparedPublication is a sealed, bounded proof of the caller's original
// publication objects. It contains no live handles and has no persisted schema.
type PreparedPublication struct {
	identity string
	digest   string
}

func (proof PreparedPublication) IsZero() bool {
	return proof.identity == "" && proof.digest == ""
}

// OriginalPublicationEntry names one original held object. RelativePath uses
// canonical slash separators; "." names the root. Info must come from the
// original live Root.Stat or File.Stat, not a pathname that may be reopened.
type OriginalPublicationEntry struct {
	RelativePath string
	Info         os.FileInfo
}

// CaptureOriginalPublication seals at most eight original entries, depth two,
// and 4 KiB of regular bytes. All original identities are joined before any
// bytes are read. The caller keeps its original handles live until this returns.
func CaptureOriginalPublication(root *os.Root, entries []OriginalPublicationEntry) (PreparedPublication, error) {
	if len(entries) == 0 || len(entries) > publicationObjectsLimit {
		return PreparedPublication{}, fmt.Errorf("original publication entry limit")
	}
	expected := make(map[string]os.FileInfo, len(entries))
	for _, entry := range entries {
		if !publicationRelative(entry.RelativePath) || entry.Info == nil || expected[entry.RelativePath] != nil {
			return PreparedPublication{}, fmt.Errorf("invalid original publication manifest")
		}
		expected[entry.RelativePath] = entry.Info
	}
	if expected["."] == nil || !expected["."].IsDir() {
		return PreparedPublication{}, fmt.Errorf("original publication root unavailable")
	}
	return capturePublicationObjects(root, expected, "")
}

type publicationObject struct {
	relative string
	parent   *os.Root
	name     string
	file     *os.File
	dir      *os.Root
	info     os.FileInfo
	identity string
	facts    string
	names    []string
}

type publicationObjects struct {
	expected map[string]os.FileInfo
	entries  []publicationObject
	closers  []io.Closer
	bytes    int64
}

func publicationRelative(name string) bool {
	return name != "" && utf8.ValidString(name) && path.Clean(name) == name && !strings.ContainsAny(name, "\\:\x00") &&
		strings.Count(name, "/") <= 1 && filepath.IsLocal(filepath.FromSlash(name))
}

func capturePublicationObjects(root *os.Root, expected map[string]os.FileInfo, rootIdentity string) (proof PreparedPublication, resultErr error) {
	if root == nil {
		return proof, fmt.Errorf("held publication root unavailable")
	}
	capture := publicationObjects{expected: expected}
	defer func() {
		for index := len(capture.closers) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, capture.closers[index].Close())
		}
		if resultErr != nil {
			proof = PreparedPublication{}
		}
	}()
	if err := capture.collect(root, ".", ".", rootIdentity); err != nil {
		return proof, err
	}
	if expected != nil && len(expected) != len(capture.entries) {
		return proof, fmt.Errorf("original publication manifest is incomplete")
	}
	if err := capture.revalidate(); err != nil {
		return proof, err
	}
	digest, err := capture.digest()
	if err != nil {
		return proof, err
	}
	if err := capture.revalidate(); err != nil {
		return proof, err
	}
	return PreparedPublication{identity: capture.entries[0].identity, digest: digest}, nil
}

func (capture *publicationObjects) collect(parent *os.Root, name, relative, rootIdentity string) error {
	if len(capture.entries) >= publicationObjectsLimit || !publicationRelative(relative) {
		return fmt.Errorf("publication object census limit")
	}
	named, err := parent.Lstat(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	if !named.IsDir() && !named.Mode().IsRegular() {
		return fmt.Errorf("publication entry must be an ordinary directory or regular file")
	}
	object, err := capture.open(parent, name, relative, named)
	if err != nil {
		return err
	}
	if rootIdentity != "" && object.identity != rootIdentity {
		return fmt.Errorf("publication root was replaced")
	}
	if err := capture.admit(object); err != nil {
		return err
	}
	if object.dir != nil {
		object.names, err = publicationNames(object.dir)
		if err != nil {
			return err
		}
	}
	capture.entries = append(capture.entries, object)
	for _, child := range object.names {
		if err := capture.collect(object.dir, child, path.Join(relative, child), ""); err != nil {
			return err
		}
	}
	return nil
}

func (capture *publicationObjects) open(parent *os.Root, name, relative string, named os.FileInfo) (publicationObject, error) {
	object := publicationObject{relative: relative, parent: parent, name: name}
	var err error
	if named.IsDir() {
		object.dir, err = parent.OpenRoot(filepath.FromSlash(name))
		if err == nil {
			capture.closers = append(capture.closers, object.dir)
			object.file, err = object.dir.Open(".")
		}
	} else {
		object.file, err = atomicfile.OpenRootReadFile(parent, filepath.FromSlash(name))
	}
	if err != nil {
		return object, err
	}
	capture.closers = append(capture.closers, object.file)
	object.info, err = object.file.Stat()
	if err != nil {
		return object, err
	}
	if !samePublicationInfo(named, object.info) {
		return object, fmt.Errorf("publication entry changed while opening")
	}
	object.identity, object.facts, err = publicationEntryFacts(object.file, object.info)
	return object, err
}

func (capture *publicationObjects) admit(object publicationObject) error {
	for _, previous := range capture.entries {
		if previous.identity == object.identity {
			return fmt.Errorf("publication object has multiple names")
		}
	}
	if capture.expected != nil {
		original := capture.expected[object.relative]
		if original == nil || !samePublicationInfo(original, object.info) {
			return fmt.Errorf("original publication object changed")
		}
	}
	if object.info.Mode().IsRegular() {
		size := object.info.Size()
		if size < 0 || size > publicationObjectsBytes || capture.bytes > publicationObjectsBytes-size {
			return fmt.Errorf("publication regular byte limit")
		}
		capture.bytes += size
	}
	return nil
}

func publicationNames(root *os.Root) (names []string, resultErr error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	entries, err := file.ReadDir(publicationObjectsLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > publicationObjectsLimit {
		return nil, fmt.Errorf("publication directory entry limit")
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || !publicationRelative(name) || strings.Contains(name, "/") {
			return nil, fmt.Errorf("publication directory entry name is not canonical")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func samePublicationInfo(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
		!samePublicationMetadata(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	return before.IsDir() || before.Mode().IsRegular() && before.Size() == after.Size()
}

func (capture *publicationObjects) revalidate() error {
	for _, object := range capture.entries {
		if err := object.revalidate(); err != nil {
			return err
		}
	}
	return nil
}

func (object publicationObject) revalidate() error {
	held, err := object.file.Stat()
	if err != nil {
		return err
	}
	named, err := object.parent.Lstat(filepath.FromSlash(object.name))
	if err != nil {
		return err
	}
	if !samePublicationInfo(object.info, held) || !samePublicationInfo(object.info, named) {
		return fmt.Errorf("publication named or held object changed")
	}
	identity, facts, err := publicationEntryFacts(object.file, held)
	if err != nil {
		return err
	}
	if identity != object.identity || facts != object.facts {
		return fmt.Errorf("publication object authority changed")
	}
	if object.dir != nil {
		names, err := publicationNames(object.dir)
		if err != nil {
			return err
		}
		if !slices.Equal(names, object.names) {
			return fmt.Errorf("publication directory contents changed")
		}
	}
	return nil
}

func (capture *publicationObjects) digest() (string, error) {
	h := sha256.New()
	for _, object := range capture.entries {
		publicationObjectField(h, object.relative)
		publicationObjectField(h, object.identity)
		publicationObjectField(h, fmt.Sprint(uint32(object.info.Mode())))
		publicationObjectField(h, object.facts)
		if object.dir != nil {
			continue
		}
		publicationObjectField(h, object.info.ModTime().UTC().Format(time.RFC3339Nano))
		body, err := io.ReadAll(io.LimitReader(object.file, object.info.Size()+1))
		if err != nil {
			return "", err
		}
		if int64(len(body)) != object.info.Size() {
			return "", fmt.Errorf("publication bytes changed while reading")
		}
		publicationObjectField(h, string(body))
	}
	return publicationObjectsPrefix + hex.EncodeToString(h.Sum(nil)), nil
}

func publicationObjectField(h hash.Hash, value string) {
	_ = binary.Write(h, binary.BigEndian, uint64(len(value)))
	_, _ = io.WriteString(h, value)
}

func validPublicationObjectsDigest(digest string) bool {
	if len(digest) != len(publicationObjectsPrefix)+sha256.Size*2 || !strings.HasPrefix(digest, publicationObjectsPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(digest, publicationObjectsPrefix)
	value, err := hex.DecodeString(encoded)
	return err == nil && len(value) == sha256.Size && hex.EncodeToString(value) == encoded
}

func validatePublicationInputRole(input Input) error {
	if input.PublicationParentAuthority != nil && (!input.RequireAbsent || input.Remove) {
		return fmt.Errorf("publication parent requires a fresh swap")
	}
	if !input.ExpectedPublication.IsZero() && input.PublicationParentAuthority == nil {
		return fmt.Errorf("original publication proof requires its original parent")
	}
	return nil
}

func publicationInputProof(input Input, stagingPath string) (string, string, error) {
	proof := input.ExpectedPublication
	if proof.IsZero() {
		if input.Remove {
			return "", "", nil
		}
		return publicationProof(stagingPath)
	}
	if input.Remove || !input.RequireAbsent || input.PublicationParentAuthority == nil || input.PublicationParentAuthority.IsZero() {
		return "", "", fmt.Errorf("original publication proof requires a fresh parent-authorized swap")
	}
	if err := matchesPublicationObjects(stagingPath, proof.identity, proof.digest); err != nil {
		return "", "", err
	}
	return proof.identity, proof.digest, nil
}

func validatePublicationProof(receipt Receipt) error {
	if receipt.Operation == OperationSwap && (receipt.PublishedIdentity == "" || receipt.PublishedDigest == "") {
		return fmt.Errorf("publication ownership proof unavailable")
	}
	if strings.HasPrefix(receipt.BackupDigest, "objects") {
		return fmt.Errorf("original publication proof cannot authorize a backup")
	}
	if !strings.HasPrefix(receipt.PublishedDigest, "objects") {
		return nil
	}
	if !validPublicationObjectsDigest(receipt.PublishedDigest) || receipt.Operation != OperationSwap ||
		receipt.PublicationParentAuthority == nil || receipt.HadActive {
		return fmt.Errorf("invalid original publication proof or role")
	}
	return nil
}

func validatePublicationObjectsRoles(receipt Receipt) error {
	if !strings.HasPrefix(receipt.PublishedDigest, "objects") {
		return nil
	}
	present := ""
	for _, role := range []struct{ label, name string }{
		{"staging", receipt.StagingPath}, {"active", receipt.ActivePath}, {"quarantine", receipt.QuarantinePath},
	} {
		exists, err := realDirectoryExists(role.name)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if present != "" {
			return fmt.Errorf("ambiguous original publication roles")
		}
		present = role.label
		if !validPublicationObjectRole(receipt.Phase, present) {
			return fmt.Errorf("original publication has an unexpected phase role")
		}
		if err := matchesPublicationObjects(role.name, receipt.PublishedIdentity, receipt.PublishedDigest); err != nil {
			return err
		}
	}
	if !validPublicationObjectRole(receipt.Phase, present) {
		return fmt.Errorf("original publication recovery object is missing")
	}
	return nil
}

func validPublicationObjectRole(phase, role string) bool {
	switch phase {
	case PhaseIntent, PhaseBackupPending, PhaseOldBackedUp:
		return role == "staging"
	case PhaseActivationPending:
		return role == "staging" || role == "active"
	case PhaseActivated, PhaseCommitPending, PhaseCommitted:
		return role == "active"
	case PhaseRollbackPending:
		return role == "" || role == "staging" || role == "active" || role == "quarantine"
	case PhaseRolledBack:
		return role == "" || role == "staging"
	default:
		return false
	}
}

func matchesPublicationObjects(name, identity, digest string) (resultErr error) {
	if identity == "" || !validPublicationObjectsDigest(digest) {
		return fmt.Errorf("original publication ownership proof unavailable")
	}
	parent, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.Close()) }()
	named, err := parent.Lstat(filepath.Base(name))
	if err != nil {
		return err
	}
	if !named.IsDir() || named.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("publication root is not an ordinary directory")
	}
	root, err := parent.OpenRoot(filepath.Base(name))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	proof, err := capturePublicationObjects(root, nil, identity)
	if err != nil {
		return err
	}
	after, err := parent.Lstat(filepath.Base(name))
	if err != nil {
		return err
	}
	held, err := root.Stat(".")
	if err != nil {
		return err
	}
	if !samePublicationInfo(named, after) || !samePublicationInfo(after, held) {
		return fmt.Errorf("publication root changed during verification")
	}
	if proof.identity != identity || proof.digest != digest {
		return fmt.Errorf("original publication objects changed or were replaced; recovery required")
	}
	return nil
}

func matchesProof(path, expectedIdentity, expectedDigest string) error {
	if strings.HasPrefix(expectedDigest, "objects") {
		return matchesPublicationObjects(path, expectedIdentity, expectedDigest)
	}
	if expectedIdentity == "" || expectedDigest == "" {
		return fmt.Errorf("directory ownership evidence unavailable for %q; recovery required", path)
	}
	identity, digest, err := publicationProof(path)
	if err != nil {
		return fmt.Errorf("verify directory ownership at %q: %w", path, err)
	}
	if identity != expectedIdentity || digest != expectedDigest {
		return fmt.Errorf("directory changed or replaced at %q; recovery required", path)
	}
	return nil
}
