package nativeconfig

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/tailscale/hujson"
)

func validateTransition(req TransitionRequest) error {
	if (req.SourceCodec != CodecOpenCode && req.SourceCodec != CodecOpenCodeV2) ||
		(req.TargetCodec != CodecOpenCode && req.TargetCodec != CodecOpenCodeV2) || req.SourceCodec == req.TargetCodec {
		return fmt.Errorf("native transition requires distinct OpenCode V1/V2 codecs")
	}
	if len(req.Entries) == 0 || req.PersistPrepared == nil {
		return fmt.Errorf("native transition requires entries and durable PersistPrepared")
	}
	for i, entry := range req.Entries {
		if err := validateRequest(Request{Paths: req.Paths, Codec: req.SourceCodec, Action: ActionRemove, Name: entry.Name, Owned: &entry.SourceOwned}); err != nil {
			return err
		}
		if entry.LogicalID != "opencode-mcp:"+entry.Name || i > 0 && req.Entries[i-1].Name >= entry.Name {
			return fmt.Errorf("native transition requires consistent logical IDs and strictly sorted unique names")
		}
	}
	return nil
}

// ApplyDialectTransition uses the same candidate locks, document and verified
// conditional writer as ApplyBatch. The one replacement switches all selected
// leaves; rollback is conditional on our exact output. The noncooperating
// writer syscall race documented on conditionalFileIO remains unchanged.
// Errors without IsCommittedCleanup must not publish target ownership; unknown
// write/rollback status requires provider recovery from the prepared facts.
func (kernel Kernel) ApplyDialectTransition(req TransitionRequest) (receipts []Receipt, err error) {
	if err := kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	if err := validateTransition(req); err != nil {
		return nil, err
	}
	release, err := kernel.acquireTransitionLease(req)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() { err = releaseTransitionLease(release, committed, err) }()
	file, err := kernel.resolve(req.Paths)
	if err != nil {
		return nil, err
	}
	prepared, next, receipts, err := prepareDialectTransition(file, req)
	if err != nil {
		return nil, err
	}
	if err := req.PersistPrepared(prepared); err != nil {
		return nil, fmt.Errorf("persist prepared native transition: %w", err)
	}
	// Original preimage/existence CAS also rejects callback edits. Callback
	// copies cannot influence next, file.body or the returned receipts.
	if err := kernel.writeVerified(file, req.TargetCodec, next); err != nil {
		return nil, err
	}
	committed = true
	return receipts, nil
}

func (kernel Kernel) acquireTransitionLease(req TransitionRequest) (func() error, error) {
	acquire := kernel.acquireLocks
	if acquire == nil {
		acquire = kernel.acquireCandidateLocks
	}
	release, err := acquire(req.Paths, req.SourceCodec)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("native config lock acquirer returned no release operation")
	}
	return release, nil
}

func releaseTransitionLease(release func() error, committed bool, operationErr error) error {
	releaseErr := release()
	if releaseErr == nil {
		return operationErr
	}
	cleanup := fmt.Errorf("unlock native config: %w", releaseErr)
	if operationErr != nil {
		return errors.Join(operationErr, cleanup)
	}
	if committed {
		return &CommittedCleanupError{Err: cleanup}
	}
	return cleanup
}

type transitionDocument struct {
	document                         *document
	mcp, nested, source, destination *hujson.Object
}

type transitionEntry struct {
	prepared         PreparedTransitionEntry
	value            hujson.Value
	compatibleTarget bool
}

func prepareDialectTransition(file resolvedFile, req TransitionRequest) (PreparedTransition, []byte, []Receipt, error) {
	doc, err := readTransitionDocument(file, req.SourceCodec)
	if err != nil {
		return PreparedTransition{}, nil, nil, err
	}
	entries := make([]transitionEntry, len(req.Entries))
	// Prove every source and destination before editing even the private AST.
	for i, entry := range req.Entries {
		entries[i], err = prepareTransitionEntry(doc, file.path, req, entry)
		if err != nil {
			return PreparedTransition{}, nil, nil, err
		}
	}
	if err := rewriteTransitionDocument(doc, req, entries); err != nil {
		return PreparedTransition{}, nil, nil, err
	}
	next, err := doc.document.render()
	if err != nil {
		return PreparedTransition{}, nil, nil, err
	}
	preparedEntries := make([]PreparedTransitionEntry, len(entries))
	receipts := make([]Receipt, len(entries))
	for i, entry := range entries {
		preparedEntries[i], receipts[i] = entry.prepared, entry.prepared.TargetReceipt
	}
	hash := sha256.Sum256(next)
	prepared := PreparedTransition{Path: file.path, Original: FileSnapshot{Body: bytes.Clone(file.body), Mode: file.mode, Exists: file.exists},
		TargetBytes: bytes.Clone(next), TargetHash: fmt.Sprintf("sha256:%x", hash), SourceCodec: req.SourceCodec, TargetCodec: req.TargetCodec, Entries: preparedEntries}
	return prepared, next, receipts, nil
}

func readTransitionDocument(file resolvedFile, sourceCodec Codec) (transitionDocument, error) {
	if !file.exists {
		return transitionDocument{}, ErrNotOwned
	}
	doc, err := parseDocument(file.body, file.jsonc)
	if err != nil {
		return transitionDocument{}, err
	}
	mcp, err := collection(doc, "mcp", false)
	if err != nil {
		return transitionDocument{}, err
	}
	if mcp == nil {
		return transitionDocument{}, ErrNotOwned
	}
	// A V1 leaf literally named servers is not a V2 container. Classify it
	// before looking up either dialect; do not reinterpret its entry fields.
	nested, err := transitionNestedServers(mcp)
	if err != nil {
		return transitionDocument{}, err
	}
	source, destination := mcp, nested
	if sourceCodec == CodecOpenCodeV2 {
		source, destination = nested, mcp
	}
	if source == nil {
		return transitionDocument{}, ErrNotOwned
	}
	return transitionDocument{document: doc, mcp: mcp, nested: nested, source: source, destination: destination}, nil
}

func prepareTransitionEntry(doc transitionDocument, path string, req TransitionRequest, entry TransitionEntry) (transitionEntry, error) {
	member, _ := objectMember(doc.source, entry.Name)
	if member == nil || req.SourceCodec == CodecOpenCode && entry.Name == "servers" && doc.nested != nil ||
		verifyReceipt(&entry.SourceOwned, path, req.SourceCodec, entry.Name, member) != nil {
		return transitionEntry{}, fmt.Errorf("%w: source %s", ErrNotOwned, entry.Name)
	}
	desired, err := DesiredReceipt(path, req.TargetCodec, entry.Name, entry.TargetServer, entry.TargetPlaceholders)
	if err != nil {
		return transitionEntry{}, err
	}
	if entry.DesiredReceipt != desired {
		return transitionEntry{}, fmt.Errorf("transition desired receipt changed: %w", ErrConcurrentChange)
	}
	compatible, err := validateTransitionTarget(doc, path, req.TargetCodec, entry, desired)
	if err != nil {
		return transitionEntry{}, err
	}
	projected, err := projectServer(req.TargetCodec, entry.TargetServer, entry.TargetPlaceholders)
	if err != nil {
		return transitionEntry{}, err
	}
	value, err := jsonValue(projected)
	if err != nil {
		return transitionEntry{}, err
	}
	prepared := PreparedTransitionEntry{LogicalID: entry.LogicalID, Name: entry.Name, SourceReceipt: entry.SourceOwned, TargetReceipt: desired}
	if entry.TargetOwned != nil {
		owned := *entry.TargetOwned
		prepared.PreviouslyOwnedTarget = &owned
	}
	return transitionEntry{prepared: prepared, value: value, compatibleTarget: compatible}, nil
}

func validateTransitionTarget(doc transitionDocument, path string, codec Codec, entry TransitionEntry, desired Receipt) (bool, error) {
	var target *hujson.ObjectMember
	if doc.destination != nil {
		target, _ = objectMember(doc.destination, entry.Name)
		if codec == CodecOpenCode && entry.Name == "servers" && doc.nested != nil {
			target = nil // The source container, not a destination V1 leaf.
		}
	}
	if target == nil {
		if entry.TargetOwned != nil {
			return false, fmt.Errorf("%w: missing target %s", ErrNotOwned, entry.Name)
		}
		return false, nil
	}
	if entry.TargetOwned == nil {
		return false, fmt.Errorf("%w: target %s", ErrCollision, entry.Name)
	}
	if verifyReceipt(entry.TargetOwned, path, codec, entry.Name, target) != nil {
		return false, fmt.Errorf("%w: target %s", ErrNotOwned, entry.Name)
	}
	if verifyReceipt(&desired, path, codec, entry.Name, target) != nil {
		return false, fmt.Errorf("%w: incompatible owned target %s", ErrCollision, entry.Name)
	}
	return true, nil
}

func rewriteTransitionDocument(doc transitionDocument, req TransitionRequest, entries []transitionEntry) error {
	for _, entry := range req.Entries {
		removeTransitionEntry(doc.source, entry.Name)
	}
	if req.SourceCodec == CodecOpenCodeV2 && len(doc.source.Members) == 0 {
		// Only the emptied source container is removed. Retain its comments.
		member, _ := objectMember(doc.mcp, "servers")
		member.Value.AfterExtra = append(bytes.Clone(doc.source.AfterExtra), member.Value.AfterExtra...)
		removeTransitionEntry(doc.mcp, "servers")
	}
	if err := validateTransitionRoot(doc, req.TargetCodec); err != nil {
		return err
	}
	destination, err := codecCollection(doc.document, req.TargetCodec, true)
	if err != nil {
		return err
	}
	proposed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.compatibleTarget {
			setEntry(destination, entry.prepared.Name, entry.value)
		}
		proposed[entry.prepared.Name] = true
	}
	return validateTransitionNamespace(destination, req.TargetCodec, proposed)
}

func validateTransitionRoot(doc transitionDocument, target Codec) error {
	if target == CodecOpenCodeV2 {
		return requireOpenCodeV2Root(doc.document)
	}
	remaining, err := transitionNestedServers(doc.mcp)
	if err != nil {
		return err
	}
	if remaining != nil {
		return ErrNativeMigrationRequired
	}
	return nil
}

func validateTransitionNamespace(destination *hujson.Object, codec Codec, proposed map[string]bool) error {
	active, err := openCodeActiveMCPNamesForCodec(destination, codec)
	if err != nil {
		return err
	}
	retained, added := make([]string, 0, len(active)), make([]string, 0, len(active))
	for _, name := range active {
		if proposed[name] {
			added = append(added, name)
		} else {
			retained = append(retained, name)
		}
	}
	return checkOpenCodeNamespace(retained, added)
}

func transitionNestedServers(mcp *hujson.Object) (*hujson.Object, error) {
	member, _ := objectMember(mcp, "servers")
	if member == nil {
		return nil, nil
	}
	obj, ok := member.Value.Value.(*hujson.Object)
	if !ok {
		return nil, ErrNativeMigrationRequired
	}
	for _, key := range []string{"type", "enabled"} {
		field, _ := objectMember(obj, key)
		if field != nil {
			if _, object := field.Value.Value.(*hujson.Object); !object {
				return nil, nil // Existing flat V1 servers leaf.
			}
		}
	}
	return obj, nil
}

// Keep trivia outside deleted managed values, including comments attached to
// an emptied source container. Foreign values themselves are never rebuilt.
func removeTransitionEntry(obj *hujson.Object, name string) {
	member, index := objectMember(obj, name)
	if member == nil {
		return
	}
	extra := bytes.Clone(member.Name.BeforeExtra)
	extra = append(extra, member.Name.AfterExtra...)
	extra = append(extra, member.Value.BeforeExtra...)
	extra = append(extra, member.Value.AfterExtra...)
	removeEntry(obj, name)
	if index < len(obj.Members) {
		obj.Members[index].Name.BeforeExtra = append(extra, obj.Members[index].Name.BeforeExtra...)
	} else {
		obj.AfterExtra = append(extra, obj.AfterExtra...)
	}
}
