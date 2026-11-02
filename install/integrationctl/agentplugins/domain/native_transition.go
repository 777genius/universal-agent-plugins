package domain

// NativeAttemptIdentity is transient authority from the existing durable fence,
// never package input or a second operation allocator.
type NativeAttemptIdentity struct {
	OperationID, InstallationID, BindingID, NativeRoot string
}

type StateDecisionDisposition string

const (
	StateDecisionOld     StateDecisionDisposition = "old"
	StateDecisionDesired StateDecisionDisposition = "desired"
	StateDecisionUnknown StateDecisionDisposition = "unknown"
)

// OpenCodeTransitionPrepared is a host-neutral detached handoff. The private
// provider record adds exact state and skill intent under the outer operation lock.
type OpenCodeTransitionPrepared struct {
	Path                                 string
	OriginalBytes, TargetBytes           []byte
	OriginalMode                         uint32
	OriginalExists                       bool
	TargetHash, SourceCodec, TargetCodec string
	Entries                              []OpenCodeTransitionEntry
}
type OpenCodeTransitionReceipt struct{ Version, Path, Codec, Name, Digest string }
type OpenCodeTransitionEntry struct {
	LogicalID, Name              string
	SourceReceipt, TargetReceipt OpenCodeTransitionReceipt
	PreviouslyOwnedTarget        *OpenCodeTransitionReceipt
}
