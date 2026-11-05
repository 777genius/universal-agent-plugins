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
		return pointer, fmt.Errorf("%w: v2 pointer: %w", ErrStrictJSON, err)
	}
	if err := requireRootFields(body, "pointer_schema_version", "snapshot_schema_version",
		"sequence", "snapshot_path", "envelope_path", "fetch_contract"); err != nil {
		return pointer, fmt.Errorf("%w: v2 pointer: %w", ErrStrictJSON, err)
	}
	if pointer.PointerSchemaVersion != 2 || pointer.SnapshotSchemaVersion != 2 {
		return pointer, strictError("invalid v2 pointer schema")
	}
	if err := validateV2PointerContract(pointer); err != nil {
		return pointer, err
	}
	return pointer, nil
}

// Validate the full path/fetch contract for both decoded pointers and direct
// bundle verification callers. Only the wire version constants differ from v1.
func validateV2PointerContract(pointer Pointer) error {
	legacyContract := pointer
	legacyContract.PointerSchemaVersion = 1
	legacyContract.SnapshotSchemaVersion = 1
	return validatePointer(legacyContract)
}

// VerifyV2PointerBundle checks both authenticated artifacts and the pointer's
// sequence, but is not a network fetch or a production routing switch.
func VerifyV2PointerBundle(pointer Pointer, snapshot, envelope []byte, trust TrustStore) (VerifiedBundle, error) {
	if pointer.PointerSchemaVersion != 2 || pointer.SnapshotSchemaVersion != 2 {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	if err := validateV2PointerContract(pointer); err != nil {
		return VerifiedBundle{}, err
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
