package management

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

const (
	RecordLimit      = 1024
	RecordBytesLimit = 1 << 20
	resultRetention  = 30 * 24 * time.Hour
)

var ErrHistoryFull = errors.New("retained operation history is full; unresolved identities cannot be discarded")

// Observation is the version-1 correlation record. No persisted value grants
// authority to execute an operation after restart.
type Observation struct {
	EnvironmentID string    `json:"environmentId"`
	IntentRoot    string    `json:"intentRoot"`
	Version       int       `json:"version"`
	ContextID     string    `json:"contextId"`
	Fingerprint   string    `json:"fingerprint"`
	InstallerIDs  []string  `json:"installerIds"`
	Operation     Operation `json:"operation"`
}

// ObservationStore returns independent owned snapshots and nil for absent
// lookups. Save must not retain mutable caller references. All writes/deletes
// are invoked while the application owns the management mutation lock.
type ObservationStore interface {
	ByRequest(string) (*Observation, error)
	ByOperation(string) (*Observation, error)
	List() ([]Observation, error)
	Save(Observation) error
	Delete(requestID string) error
}

// MutationLock is nonblocking. IsActive distinguishes another live owner from
// failures to determine ownership, including errors returned by the backend.
type MutationLock interface {
	Acquire(context.Context) (func() error, error)
	IsActive(error) bool
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// reserveHistory owns retention policy; the adapter only persists validated
// records. The caller retains the mutation lock through acceptance.
func (a *Application) reserveHistory(now time.Time) error {
	records, err := a.opts.Store.List()
	if err != nil {
		return err
	}
	retained := 0
	for _, record := range records {
		if !prunable(record.Operation, now) {
			retained++
			continue
		}
		if err := a.forgetObservation(record); err != nil {
			retained++
			continue
		}
		if err := a.opts.Store.Delete(record.Operation.RequestId); err != nil {
			return err
		}
	}
	if retained >= RecordLimit {
		return ErrHistoryFull
	}
	return nil
}
func prunable(op Operation, now time.Time) bool {
	if op.ResolvedBy != "" {
		return now.Sub(op.UpdatedAt) >= resultRetention
	}
	if pendingOperation(op) {
		return false
	}
	if op.Result == nil || len(op.Result.NextActions) != 0 {
		return false
	}
	return now.Sub(op.UpdatedAt) >= resultRetention
}
func terminal(state OperationState) bool { return state != Queued && state != Running }
