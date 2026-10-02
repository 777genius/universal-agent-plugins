package installerui

import "context"

// Per-call private seams for measured queue and restoration qualification.
// Normal calls use OS operations; no global hook or additional public config.
type terminalOpsKey struct{}
type terminalOps struct {
	snapshot func(int) (func() error, error)
	queued   func() (int, error)
}

func promptOps(ctx context.Context) *terminalOps {
	ops, _ := ctx.Value(terminalOpsKey{}).(*terminalOps)
	return ops
}
