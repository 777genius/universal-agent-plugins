package nativeconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
)

type resolvedFile struct {
	path          string
	body          []byte
	mode          os.FileMode
	exists        bool
	jsonc         bool
	alternatePath string
}

func (kernel Kernel) Apply(req Request) (Receipt, error) {
	receipts, err := kernel.ApplyBatch([]Request{req})
	if err != nil && !IsCommittedCleanup(err) {
		return Receipt{}, err
	}
	if len(receipts) == 0 {
		return Receipt{}, err
	}
	return receipts[0], err
}

// ApplyBatch validates and renders related MCP entry mutations into one atomic
// replacement. The batch is all-or-none for cooperating agentplugins writers.
// See conditionalFileIO for the unavoidable portable race with clients that do
// not honor the same locks.
func (kernel Kernel) ApplyBatch(requests []Request) (receipts []Receipt, err error) {
	if kernel.files == nil {
		return nil, fmt.Errorf("native config file IO is required")
	}
	if len(requests) == 0 {
		return nil, nil
	}
	for _, req := range requests {
		if err := validateRequest(req); err != nil {
			return nil, err
		}
		if req.Paths != requests[0].Paths || req.Codec != requests[0].Codec {
			return nil, fmt.Errorf("batched native config requests must share paths and codec")
		}
	}
	acquireLocks := kernel.acquireLocks
	if acquireLocks == nil {
		acquireLocks = kernel.acquireCandidateLocks
	}
	release, err := acquireLocks(requests[0].Paths, requests[0].Codec)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("native config lock acquirer returned no release operation")
	}
	committed := false
	defer func() {
		releaseErr := release()
		if releaseErr == nil {
			return
		}
		cleanupErr := fmt.Errorf("unlock native config: %w", releaseErr)
		if err != nil {
			err = errors.Join(err, cleanupErr)
			return
		}
		if committed {
			err = &CommittedCleanupError{Err: cleanupErr}
			return
		}
		err = cleanupErr
	}()
	file, err := kernel.resolve(requests[0].Paths)
	if err != nil {
		return nil, err
	}
	doc, err := newDocument(file.jsonc)
	if file.exists {
		doc, err = parseDocument(file.body, file.jsonc)
	}
	if err != nil {
		return nil, err
	}
	key := codecCollectionKey(requests[0].Codec)
	create := false
	for _, req := range requests {
		create = create || req.Action == ActionAdd
	}
	entries, err := collection(doc, key, create)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, req := range requests {
		if _, duplicate := seen[req.Name]; duplicate {
			return nil, fmt.Errorf("duplicate native MCP batch entry %q", req.Name)
		}
		seen[req.Name] = struct{}{}
		var current *hujson.ObjectMember
		if entries != nil {
			current, _ = objectMember(entries, req.Name)
		}
		switch req.Action {
		case ActionAdd:
			if current != nil {
				return nil, fmt.Errorf("%w: %s", ErrCollision, req.Name)
			}
			projected, err := projectServer(req.Codec, req.Server, req.Placeholders)
			if err != nil {
				return nil, err
			}
			value, err := jsonValue(projected)
			if err != nil {
				return nil, err
			}
			setEntry(entries, req.Name, value)
		case ActionUpdate:
			if current == nil || verifyReceipt(req.Owned, file.path, req.Codec, req.Name, current) != nil {
				return nil, fmt.Errorf("%w: %s", ErrNotOwned, req.Name)
			}
			projected, err := projectServer(req.Codec, req.Server, req.Placeholders)
			if err != nil {
				return nil, err
			}
			value, err := jsonValue(projected)
			if err != nil {
				return nil, err
			}
			setEntry(entries, req.Name, value)
		case ActionRemove:
			if current == nil || verifyReceipt(req.Owned, file.path, req.Codec, req.Name, current) != nil {
				return nil, fmt.Errorf("%w: %s", ErrNotOwned, req.Name)
			}
			removeEntry(entries, req.Name)
		}
	}

	receipts = make([]Receipt, len(requests))
	for index, req := range requests {
		if req.Action == ActionRemove {
			continue
		}
		desired, err := DesiredReceipt(file.path, req.Codec, req.Name, req.Server, req.Placeholders)
		if err != nil {
			return nil, err
		}
		if req.Desired != nil && *req.Desired != desired {
			return nil, fmt.Errorf("native config desired receipt changed before write: %w", ErrConcurrentChange)
		}
		member, _ := objectMember(entries, req.Name)
		actualDigest, err := entryDigest(req.Codec, req.Name, member)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(actualDigest, desired.Digest) {
			return nil, fmt.Errorf("native config projection differs from desired receipt")
		}
		receipts[index] = desired
	}
	next, err := doc.render()
	if err != nil {
		return nil, err
	}
	if err := kernel.writeVerified(file, requests[0].Codec, next); err != nil {
		return nil, err
	}
	committed = true
	return receipts, nil
}

// Inspect performs a strict read-only ownership check for one native entry.
func (kernel Kernel) Inspect(paths Paths, codec Codec, name string, owned *Receipt) (present bool, exactlyOwned bool, err error) {
	if kernel.files == nil {
		return false, false, fmt.Errorf("native config file IO is required")
	}
	if strings.TrimSpace(name) == "" {
		return false, false, fmt.Errorf("MCP entry name is required")
	}
	probe := Request{Paths: paths, Codec: codec, Action: ActionAdd, Name: name}
	if err := validateRequest(probe); err != nil {
		return false, false, err
	}
	file, err := kernel.resolve(paths)
	if err != nil || !file.exists {
		return false, false, err
	}
	doc, err := parseDocument(file.body, file.jsonc)
	if err != nil {
		return false, false, err
	}
	key := codecCollectionKey(codec)
	entries, err := collection(doc, key, false)
	if err != nil || entries == nil {
		return false, false, err
	}
	member, _ := objectMember(entries, name)
	if member == nil {
		return false, false, nil
	}
	return true, verifyReceipt(owned, file.path, codec, name, member) == nil, nil
}

func (kernel Kernel) resolve(paths Paths) (resolvedFile, error) {
	jsonPath := filepath.Clean(paths.JSON)
	jsonBody, jsonMode, jsonExists, err := kernel.files.ReadNoFollow(jsonPath)
	if err != nil {
		return resolvedFile{}, err
	}
	var jsoncBody []byte
	var jsoncMode os.FileMode
	var jsoncExists bool
	jsoncPath := ""
	if paths.JSONC != "" {
		jsoncPath = filepath.Clean(paths.JSONC)
		jsoncBody, jsoncMode, jsoncExists, err = kernel.files.ReadNoFollow(jsoncPath)
		if err != nil {
			return resolvedFile{}, err
		}
	}
	if jsonExists && jsoncExists {
		return resolvedFile{}, ErrAmbiguousConfig
	}
	if jsoncExists {
		return resolvedFile{path: jsoncPath, body: jsoncBody, mode: jsoncMode, exists: true, jsonc: true, alternatePath: jsonPath}, nil
	}
	return resolvedFile{path: jsonPath, body: jsonBody, mode: jsonMode, exists: jsonExists, alternatePath: jsoncPath}, nil
}

func (kernel Kernel) writeVerified(file resolvedFile, _ Codec, next []byte) (resultErr error) {
	mode := file.mode
	if !file.exists {
		mode = 0o600
	}
	restore := func() error {
		if file.exists {
			return kernel.compareAndSwap(file.path, next, true, file.body, mode)
		}
		return kernel.removeIfUnchanged(file.path, next)
	}
	if err := kernel.verifyAlternateAbsent(file); err != nil {
		return err
	}
	if err := kernel.compareAndSwap(file.path, file.body, file.exists, next, mode); err != nil {
		return kernel.rollbackIfStillOurs(file, next, restore, fmt.Errorf("write native config: %w", err))
	}
	if err := kernel.verifyAlternateAbsent(file); err != nil {
		return kernel.rollbackIfStillOurs(file, next, restore, err)
	}
	written, _, exists, err := kernel.files.ReadNoFollow(file.path)
	if err != nil || !exists || !bytes.Equal(written, next) {
		verifyErr := err
		if verifyErr == nil {
			verifyErr = fmt.Errorf("written bytes differ from requested bytes")
		}
		return kernel.rollbackIfStillOurs(file, next, restore, fmt.Errorf("verify native config: %w", verifyErr))
	}
	return nil
}

func (kernel Kernel) compareAndSwap(path string, expected []byte, expectedExists bool, next []byte, mode os.FileMode) error {
	if files, ok := kernel.files.(conditionalFileIO); ok {
		return files.CompareAndSwap(path, expected, expectedExists, next, mode)
	}
	current, _, exists, err := kernel.files.ReadNoFollow(path)
	if err != nil {
		return fmt.Errorf("re-read native config at replacement boundary: %w", err)
	}
	if exists != expectedExists || !bytes.Equal(current, expected) {
		return ErrConcurrentChange
	}
	return kernel.files.WriteAtomic(path, next, mode)
}

func (kernel Kernel) removeIfUnchanged(path string, expected []byte) error {
	if files, ok := kernel.files.(conditionalFileIO); ok {
		return files.RemoveIfUnchanged(path, expected)
	}
	current, _, exists, err := kernel.files.ReadNoFollow(path)
	if err != nil {
		return fmt.Errorf("re-read native config at removal boundary: %w", err)
	}
	if !exists || !bytes.Equal(current, expected) {
		return ErrConcurrentChange
	}
	return kernel.files.RemoveNoFollow(path)
}

func (kernel Kernel) verifyAlternateAbsent(file resolvedFile) error {
	if file.alternatePath == "" {
		return nil
	}
	_, _, exists, err := kernel.files.ReadNoFollow(file.alternatePath)
	if err != nil {
		return fmt.Errorf("verify alternate native config: %w", err)
	}
	if exists {
		return ErrAmbiguousConfig
	}
	return nil
}

func (kernel Kernel) rollbackIfStillOurs(file resolvedFile, writtenByUs []byte, restore func() error, primary error) error {
	current, _, exists, readErr := kernel.files.ReadNoFollow(file.path)
	if readErr != nil {
		return errors.Join(primary, fmt.Errorf("rollback skipped because current bytes could not be verified: %w", readErr))
	}
	if exists == file.exists && bytes.Equal(current, file.body) {
		return primary
	}
	if !exists || !bytes.Equal(current, writtenByUs) {
		return errors.Join(primary, fmt.Errorf("rollback skipped because native config is no longer our exact write: %w", ErrConcurrentChange))
	}
	if rollbackErr := restore(); rollbackErr != nil {
		return errors.Join(primary, fmt.Errorf("rollback native config: %w", rollbackErr))
	}
	return fmt.Errorf("native config original bytes restored after failure: %w", primary)
}
