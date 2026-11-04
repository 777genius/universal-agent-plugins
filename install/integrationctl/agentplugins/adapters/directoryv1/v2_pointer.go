package directoryv1

import (
	"fmt"
)

// ParseV2Pointer validates the independently versioned signed Directory
// endpoint. It reuses v1's reviewed path/fetch-contract checks, but never
// accepts a v1 pointer as authorization for a schema-2 publication.
func ParseV2Pointer(body []byte) (Pointer, error) {
	var pointer Pointer
	if err := strictDecode(body, &pointer, MaxLatestBytes); err != nil {
		return pointer, fmt.Errorf("%w: v2 pointer: %v", ErrStrictJSON, err)
	}
	if err := requireRootFields(body, "pointer_schema_version", "snapshot_schema_version",
		"sequence", "snapshot_path", "envelope_path", "fetch_contract"); err != nil {
		return pointer, fmt.Errorf("%w: v2 pointer: %v", ErrStrictJSON, err)
	}
	if pointer.PointerSchemaVersion != 2 || pointer.SnapshotSchemaVersion != 2 {
		return pointer, strictError("invalid v2 pointer schema")
	}
	// Only version constants differ. For security, keep exactly one reviewed
	// artifact-path, redirect, credential and size-bounds validator.
	legacyContract := pointer
	legacyContract.PointerSchemaVersion = 1
	legacyContract.SnapshotSchemaVersion = 1
	if err := validatePointer(legacyContract); err != nil {
		return pointer, err
	}
	return pointer, nil
}

// VerifyV2PointerBundle checks both authenticated artifacts and the pointer's
// sequence, but is not a network fetch or a production routing switch.
func VerifyV2PointerBundle(pointer Pointer, snapshot, envelope []byte, trust TrustStore) (VerifiedBundle, error) {
	if pointer.PointerSchemaVersion != 2 || pointer.SnapshotSchemaVersion != 2 {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	bundle, err := VerifyV2Bundle(snapshot, envelope, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if pointer.Sequence != bundle.Snapshot.Sequence || pointer.Sequence != bundle.Envelope.Sequence ||
		bundle.Snapshot.SnapshotSchemaVersion != 2 {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	return bundle, nil
}
