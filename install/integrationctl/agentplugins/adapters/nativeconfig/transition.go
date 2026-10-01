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
	committed := false
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			cleanup := fmt.Errorf("unlock native config: %w", releaseErr)
			if err != nil {
				err = errors.Join(err, cleanup)
			} else if committed {
				err = &CommittedCleanupError{Err: cleanup}
			} else {
				err = cleanup
			}
		}
	}()
	file, err := kernel.resolve(req.Paths)
	if err != nil {
		return nil, err
	}
	if !file.exists {
		return nil, ErrNotOwned
	}
	doc, err := parseDocument(file.body, file.jsonc)
	if err != nil {
		return nil, err
	}
	mcp, err := collection(doc, "mcp", false)
	if err != nil {
		return nil, err
	}
	if mcp == nil {
		return nil, ErrNotOwned
	}
	// A V1 leaf literally named servers is not a V2 container. Classify it
	// before looking up either dialect; do not reinterpret its entry fields.
	nested, err := transitionNestedServers(mcp)
	if err != nil {
		return nil, err
	}
	source, destination := mcp, nested
	if req.SourceCodec == CodecOpenCodeV2 {
		source, destination = nested, mcp
	}
	if source == nil {
		return nil, ErrNotOwned
	}
	receipts = make([]Receipt, len(req.Entries))
	preparedEntries := make([]PreparedTransitionEntry, len(req.Entries))
	values := make([]hujson.Value, len(req.Entries))
	compatibleTargets := make([]bool, len(req.Entries))
	// Prove every source and destination before editing even the private AST.
	for i, entry := range req.Entries {
		member, _ := objectMember(source, entry.Name)
		if member == nil || req.SourceCodec == CodecOpenCode && entry.Name == "servers" && nested != nil ||
			verifyReceipt(&entry.SourceOwned, file.path, req.SourceCodec, entry.Name, member) != nil {
			return nil, fmt.Errorf("%w: source %s", ErrNotOwned, entry.Name)
		}
		desired, err := DesiredReceipt(file.path, req.TargetCodec, entry.Name, entry.TargetServer, entry.TargetPlaceholders)
		if err != nil {
			return nil, err
		}
		if entry.DesiredReceipt != desired {
			return nil, fmt.Errorf("transition desired receipt changed: %w", ErrConcurrentChange)
		}
		var target *hujson.ObjectMember
		if destination != nil {
			target, _ = objectMember(destination, entry.Name)
			if req.TargetCodec == CodecOpenCode && entry.Name == "servers" && nested != nil {
				target = nil // The source container, not a destination V1 leaf.
			}
		}
		if target != nil {
			if entry.TargetOwned == nil {
				return nil, fmt.Errorf("%w: target %s", ErrCollision, entry.Name)
			}
			if verifyReceipt(entry.TargetOwned, file.path, req.TargetCodec, entry.Name, target) != nil {
				return nil, fmt.Errorf("%w: target %s", ErrNotOwned, entry.Name)
			}
			if verifyReceipt(&desired, file.path, req.TargetCodec, entry.Name, target) != nil {
				return nil, fmt.Errorf("%w: incompatible owned target %s", ErrCollision, entry.Name)
			}
			compatibleTargets[i] = true
		} else if entry.TargetOwned != nil {
			return nil, fmt.Errorf("%w: missing target %s", ErrNotOwned, entry.Name)
		}
		projected, err := projectServer(req.TargetCodec, entry.TargetServer, entry.TargetPlaceholders)
		if err != nil {
			return nil, err
		}
		values[i], err = jsonValue(projected)
		if err != nil {
			return nil, err
		}
		receipts[i] = desired
		preparedEntries[i] = PreparedTransitionEntry{LogicalID: entry.LogicalID, Name: entry.Name, SourceReceipt: entry.SourceOwned, TargetReceipt: desired}
		if entry.TargetOwned != nil {
			owned := *entry.TargetOwned
			preparedEntries[i].PreviouslyOwnedTarget = &owned
		}
	}
	for _, entry := range req.Entries {
		removeTransitionEntry(source, entry.Name)
	}
	if req.SourceCodec == CodecOpenCodeV2 && len(source.Members) == 0 {
		// Only the emptied source container is removed. Retain its comments.
		member, _ := objectMember(mcp, "servers")
		member.Value.AfterExtra = append(bytes.Clone(source.AfterExtra), member.Value.AfterExtra...)
		removeTransitionEntry(mcp, "servers")
	}
	if req.TargetCodec == CodecOpenCodeV2 {
		if err := requireOpenCodeV2Root(doc); err != nil {
			return nil, err
		}
	} else {
		remaining, err := transitionNestedServers(mcp)
		if err != nil {
			return nil, err
		}
		if remaining != nil {
			return nil, ErrNativeMigrationRequired
		}
	}
	destination, err = codecCollection(doc, req.TargetCodec, true)
	if err != nil {
		return nil, err
	}
	proposed := map[string]bool{}
	for i, entry := range preparedEntries {
		if !compatibleTargets[i] {
			setEntry(destination, entry.Name, values[i])
		}
		proposed[entry.Name] = true
	}
	active, err := openCodeActiveMCPNamesForCodec(destination, req.TargetCodec)
	if err != nil {
		return nil, err
	}
	var retained, added []string
	for _, name := range active {
		if proposed[name] {
			added = append(added, name)
		} else {
			retained = append(retained, name)
		}
	}
	if err := checkOpenCodeNamespace(retained, added); err != nil {
		return nil, err
	}
	next, err := doc.render()
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(next)
	prepared := PreparedTransition{Path: file.path, Original: FileSnapshot{Body: bytes.Clone(file.body), Mode: file.mode, Exists: file.exists},
		TargetBytes: bytes.Clone(next), TargetHash: fmt.Sprintf("sha256:%x", hash), SourceCodec: req.SourceCodec, TargetCodec: req.TargetCodec, Entries: preparedEntries}
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
