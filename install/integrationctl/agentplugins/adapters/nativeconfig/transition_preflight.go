package nativeconfig

import "fmt"

// CheckDialectTransition is read-only admission for the closed same-ID provider
// transition. ApplyDialectTransition repeats every ownership/root proof under
// its commit lease with fully staged desired receipts.
func (kernel Kernel) CheckDialectTransition(paths Paths, source, target Codec, owned []Receipt, names []string) (err error) {
	if source == target || source != CodecOpenCode && source != CodecOpenCodeV2 || target != CodecOpenCode && target != CodecOpenCodeV2 || len(owned) == 0 || len(owned) != len(names) || len(owned) > MaxTransitionEntries {
		return ErrNativeMigrationRequired
	}
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	for i, r := range owned {
		if r.Name != names[i] || i > 0 && names[i-1] >= names[i] {
			return ErrNativeMigrationRequired
		}
		if err := validateRequest(Request{Paths: paths, Codec: source, Name: r.Name, Action: ActionRemove, Owned: &r}); err != nil {
			return err
		}
	}
	acquire := kernel.acquireLocks
	if acquire == nil {
		acquire = kernel.acquireCandidateLocks
	}
	release, err := acquire(paths, source)
	if err != nil {
		return err
	}
	if release == nil {
		return fmt.Errorf("missing native lease release")
	}
	defer func() {
		if e := release(); e != nil && err == nil {
			err = e
		}
	}()
	file, err := kernel.resolve(paths)
	if err != nil {
		return err
	}
	if !file.exists {
		return ErrNotOwned
	}
	doc, err := parseDocument(file.body, file.jsonc)
	if err != nil {
		return err
	}
	entries, err := codecCollection(doc, source, false)
	if err != nil {
		return err
	}
	if entries == nil {
		return ErrNotOwned
	}
	for _, r := range owned {
		member, _ := objectMember(entries, r.Name)
		if err := verifyReceipt(&r, file.path, source, r.Name, member); err != nil {
			return err
		}
	}
	mcp, err := collection(doc, "mcp", false)
	if err != nil {
		return err
	}
	// Check both leaves before editing even this private in-memory document.
	dest, err := codecCollection(doc, target, false)
	if err != nil {
		return err
	}
	for _, name := range names {
		if dest != nil {
			member, _ := objectMember(dest, name)
			if member != nil && !(target == CodecOpenCode && name == "servers") {
				return ErrCollision
			}
		}
	}
	for _, r := range owned {
		removeTransitionEntry(entries, r.Name)
	}
	if source == CodecOpenCodeV2 && len(entries.Members) == 0 {
		removeTransitionEntry(mcp, "servers")
	}
	if target == CodecOpenCodeV2 {
		if err := requireOpenCodeV2Root(doc); err != nil {
			return err
		}
	} else {
		nested, err := transitionNestedServers(mcp)
		if err != nil {
			return err
		}
		if nested != nil {
			return ErrNativeMigrationRequired
		}
	}
	dest, err = codecCollection(doc, target, false)
	if err != nil {
		return err
	}
	var retained []string
	if dest != nil {
		retained, err = openCodeActiveMCPNamesForCodec(dest, target)
		if err != nil {
			return err
		}
	}
	return checkOpenCodeNamespace(retained, names)
}
