package nativeconfig

// TransitionRequest is the private provider handoff for a managed-leaf
// OpenCode V1 <-> V2 transition, not host selection or registry authority.
// Entries must be strictly sorted by Name. No arbitrary document edits occur.
type TransitionRequest struct {
	Paths                    Paths
	SourceCodec, TargetCodec Codec
	Entries                  []TransitionEntry
	// PersistPrepared must durably persist the supplied facts before returning
	// nil. It runs under the native file locks: callers acquire their operation
	// lock first, and must not reenter the kernel here. The provider owns journal
	// IDs, projection/skill linkage and recovery/state publication. A callback is
	// required; kernel success alone does not complete that provider transaction.
	PersistPrepared func(PreparedTransition) error
}

type TransitionEntry struct {
	LogicalID          string
	Name               string
	SourceOwned        Receipt
	TargetOwned        *Receipt
	TargetServer       Server
	TargetPlaceholders Placeholders
	DesiredReceipt     Receipt
}

// PreparedTransition contains bounded native facts only, never registry state.
// All mutable data is detached from both the request and the pending write.
// The recipient may retain/mutate this copy without changing committed bytes
// or returned receipts. TargetHash is SHA-256 of exact TargetBytes, distinct
// from the existing codec/name-domain ownership digest.
type PreparedTransition struct {
	Path                     string
	Original                 FileSnapshot
	TargetBytes              []byte
	TargetHash               string
	SourceCodec, TargetCodec Codec
	Entries                  []PreparedTransitionEntry
}

type PreparedTransitionEntry struct {
	LogicalID             string
	Name                  string
	SourceReceipt         Receipt
	PreviouslyOwnedTarget *Receipt
	TargetReceipt         Receipt
}
