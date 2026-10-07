package nativeconfig

import (
	"fmt"

	"github.com/tailscale/hujson"
)

// CheckDialectTransition is read-only admission for the closed same-ID provider
// transition. ApplyDialectTransition repeats every ownership/root proof under
// its commit lease with fully staged desired receipts.
func (kernel Kernel) CheckDialectTransition(paths Paths, source, target Codec, owned []Receipt, names []string) (err error) {
	if err := validateTransitionPreflightShape(source, target, owned, names); err != nil {
		return err
	}
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	if err := validateTransitionPreflightReceipts(paths, source, owned, names); err != nil {
		return err
	}
	release, err := kernel.acquireDialectTransitionLease(paths, source)
	if err != nil {
		return err
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
	doc, entries, err := readTransitionPreflightSource(file, source, owned)
	if err != nil {
		return err
	}
	return checkTransitionPreflightTarget(doc, entries, source, target, owned, names)
}

func validateTransitionPreflightShape(source, target Codec, owned []Receipt, names []string) error {
	if source == target || source != CodecOpenCode && source != CodecOpenCodeV2 || target != CodecOpenCode && target != CodecOpenCodeV2 || len(owned) == 0 || len(owned) != len(names) || len(owned) > MaxTransitionEntries {
		return ErrNativeMigrationRequired
	}
	return nil
}

func validateTransitionPreflightReceipts(paths Paths, source Codec, owned []Receipt, names []string) error {
	for i, r := range owned {
		if r.Name != names[i] || i > 0 && names[i-1] >= names[i] {
			return ErrNativeMigrationRequired
		}
		if err := validateRequest(Request{Paths: paths, Codec: source, Name: r.Name, Action: ActionRemove, Owned: &r}); err != nil {
			return err
		}
	}
	return nil
}

func (kernel Kernel) acquireDialectTransitionLease(paths Paths, source Codec) (func() error, error) {
	acquire := kernel.acquireLocks
	if acquire == nil {
		acquire = kernel.acquireCandidateLocks
	}
	release, err := acquire(paths, source)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("missing native lease release")
	}
	return release, nil
}

func readTransitionPreflightSource(file resolvedFile, source Codec, owned []Receipt) (*document, *hujson.Object, error) {
	if !file.exists {
		return nil, nil, ErrNotOwned
	}
	doc, err := parseDocument(file.body, file.jsonc)
	if err != nil {
		return nil, nil, err
	}
	entries, err := codecCollection(doc, source, false)
	if err != nil {
		return nil, nil, err
	}
	if entries == nil {
		return nil, nil, ErrNotOwned
	}
	for _, r := range owned {
		member, _ := objectMember(entries, r.Name)
		if err := verifyReceipt(&r, file.path, source, r.Name, member); err != nil {
			return nil, nil, err
		}
	}
	return doc, entries, nil
}

func checkTransitionPreflightTarget(doc *document, entries *hujson.Object, source, target Codec, owned []Receipt, names []string) error {
	mcp, err := collection(doc, "mcp", false)
	if err != nil {
		return err
	}
	// Check both leaves before editing even this private in-memory document.
	dest, err := codecCollection(doc, target, false)
	if err != nil {
		return err
	}
	if err := checkTransitionPreflightCollisions(dest, target, names); err != nil {
		return err
	}
	for _, r := range owned {
		removeTransitionEntry(entries, r.Name)
	}
	if source == CodecOpenCodeV2 && len(entries.Members) == 0 {
		removeTransitionEntry(mcp, "servers")
	}
	if err := checkTransitionPreflightRoot(doc, mcp, target); err != nil {
		return err
	}
	return checkTransitionPreflightNamespace(doc, target, names)
}

func checkTransitionPreflightCollisions(dest *hujson.Object, target Codec, names []string) error {
	for _, name := range names {
		if dest != nil {
			member, _ := objectMember(dest, name)
			if member != nil && !(target == CodecOpenCode && name == "servers") {
				return ErrCollision
			}
		}
	}
	return nil
}

func checkTransitionPreflightRoot(doc *document, mcp *hujson.Object, target Codec) error {
	if target == CodecOpenCodeV2 {
		return requireOpenCodeV2Root(doc)
	}
	nested, err := transitionNestedServers(mcp)
	if err != nil {
		return err
	}
	if nested != nil {
		return ErrNativeMigrationRequired
	}
	return nil
}

func checkTransitionPreflightNamespace(doc *document, target Codec, names []string) error {
	dest, err := codecCollection(doc, target, false)
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
