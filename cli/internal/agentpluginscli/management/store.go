package management

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

const (
	recordLimit      = 1024
	recordBytesLimit = 1 << 20
	resultRetention  = 30 * 24 * time.Hour
)

var errHistoryFull = errors.New("retained operation history is full; unresolved identities cannot be discarded")

// acceptance is an observation/correlation record, never an executable job.
// One file contains request identity and outcome, avoiding a second index commit.
type acceptance struct {
	EnvironmentID string    `json:"environmentId"`
	IntentRoot    string    `json:"intentRoot"`
	Version       int       `json:"version"`
	ContextID     string    `json:"contextId"`
	Fingerprint   string    `json:"fingerprint"`
	InstallerIDs  []string  `json:"installerIds"`
	Operation     Operation `json:"operation"`
}

type observationStore struct {
	root          string
	stateRoot     string
	environmentID string
}

func newObservationStore(stateRoot, environmentID string) observationStore {
	return observationStore{
		stateRoot:     stateRoot,
		environmentID: environmentID,
		root:          filepath.Join(stateRoot, "management", digest(environmentID)),
	}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s observationStore) path(requestID string) string {
	return filepath.Join(s.root, digest(requestID)+".json")
}

func (s observationStore) byRequest(requestID string) (*acceptance, error) {
	record, err := s.read(s.path(requestID))
	if err != nil || record == nil {
		return record, err
	}
	if record.Operation.RequestId != requestID {
		return nil, errors.New("request identity mismatch")
	}
	return record, nil
}

func (s observationStore) byOperation(id string) (*acceptance, error) {
	records, err := s.list()
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].Operation.OperationId == id {
			return &records[i], nil
		}
	}
	return nil, nil
}

func (s observationStore) read(path string) (*acceptance, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > recordBytesLimit {
		return nil, errors.New("invalid management observation file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var record acceptance
	decoder := json.NewDecoder(io.LimitReader(file, recordBytesLimit+1))
	if err = decoder.Decode(&record); err != nil {
		return nil, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("invalid observation document")
	}
	if record.Version != 1 || record.EnvironmentID != s.environmentID || record.Operation.EnvironmentId != s.environmentID || record.Operation.ContextId != record.ContextID {
		return nil, errors.New("management observation belongs to a different context or version")
	}
	if !validID(record.Operation.OperationId) || !validRequestID(record.Operation.RequestId) || !record.Operation.State.Valid() {
		return nil, errors.New("invalid management observation identity")
	}
	if filepath.Base(path) != filepath.Base(s.path(record.Operation.RequestId)) {
		return nil, errors.New("management observation filename mismatch")
	}
	return &record, nil
}

func (s observationStore) list() ([]acceptance, error) {
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return []acceptance{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > recordLimit*2 {
		return nil, errHistoryFull
	}
	records := make([]acceptance, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		record, err := s.read(filepath.Join(s.root, entry.Name()))
		if err != nil {
			return nil, err
		}
		if record != nil {
			records = append(records, *record)
		}
	}
	return records, nil
}

// save is called only by the owner of the cross-process management lock.
func (s observationStore) save(record acceptance) error {
	if record.EnvironmentID != s.environmentID || record.Operation.EnvironmentId != s.environmentID || record.Operation.ContextId != record.ContextID {
		return errors.New("cannot save observation for a different context")
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(body) > recordBytesLimit {
		return errors.New("management result exceeds the observation limit")
	}
	if err = s.ensureDirectory(); err != nil {
		return err
	}
	return atomicfile.Write(s.path(record.Operation.RequestId), append(body, '\n'), 0o600)
}

func (s observationStore) ensureDirectory() error {
	for _, path := range []string{s.stateRoot, filepath.Join(s.stateRoot, "management"), s.root} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			if err = os.MkdirAll(path, 0o700); err != nil {
				return err
			}
			info, err = os.Lstat(path)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("management observation root must be a real directory")
		}
		if err = os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// reserve prunes only old, terminal, resolved observations. It runs under the
// same process lock as acceptance; recovery identities never age out here.
func (s observationStore) reserve(now time.Time, forget func(acceptance) error) error {
	records, err := s.list()
	if err != nil {
		return err
	}
	retained := 0
	for _, record := range records {
		if prunable(record.Operation, now) {
			if forget != nil {
				if err = forget(record); err != nil {
					retained++ // Unverified undo data stays intact until its owner resolves it.
					continue
				}
			}
			if err = os.Remove(s.path(record.Operation.RequestId)); err != nil {
				return err
			}
		} else {
			retained++
		}
	}
	if retained >= recordLimit {
		return errHistoryFull
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
