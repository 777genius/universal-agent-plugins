package domain

// NativeEffectState records observed configuration effects, never inferred
// merely from an activation or verification error.
type NativeEffectState string

const (
	NativeEffectUnchanged NativeEffectState = "unchanged"
	NativeEffectCommitted NativeEffectState = "committed"
	NativeEffectUncertain NativeEffectState = "uncertain"
)
