// Package managementstate implements Management observation persistence and its
// cross-process mutation lock. Records correlate accepted work; they are not jobs.
package managementstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// Store preserves the version-1 atomic observation format. Application policy
// decides admission, retention and whether private undo data may be forgotten.
type Store struct {
	root          string
	stateRoot     string
	environmentID string
}

var _ management.ObservationStore = (*Store)(nil)

// NewStore captures its partition without creating or changing any files.
func NewStore(stateRoot, environmentID string) (*Store, error) {
	if !filepath.IsAbs(stateRoot) || environmentID == "" {
		return nil, errors.New("management requires an absolute state root and environment identity")
	}
	return &Store{
		stateRoot: stateRoot, environmentID: environmentID,
		root: filepath.Join(stateRoot, "management", digest(environmentID)),
	}, nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Store) path(requestID string) string {
	return filepath.Join(s.root, digest(requestID)+".json")
}

// ByRequest returns an independent observation, or nil when it is absent.
func (s *Store) ByRequest(requestID string) (*management.Observation, error) {
	record, err := s.read(s.path(requestID))
	if err != nil || record == nil {
		return record, err
	}
	if record.Operation.RequestId != requestID {
		return nil, errors.New("request identity mismatch")
	}
	return record, nil
}

func (s *Store) ByOperation(id string) (*management.Observation, error) {
	records, err := s.List()
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

func (s *Store) read(path string) (*management.Observation, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > management.RecordBytesLimit {
		return nil, errors.New("invalid management observation file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var record management.Observation
	decoder := json.NewDecoder(io.LimitReader(file, management.RecordBytesLimit+1))
	if err = decoder.Decode(&record); err != nil {
		return nil, err
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid observation document")
	}
	if record.Version != 1 || record.EnvironmentID != s.environmentID || record.Operation.EnvironmentId != s.environmentID || record.Operation.ContextId != record.ContextID {
		return nil, errors.New("management observation belongs to a different context or version")
	}
	if !management.ValidID(record.Operation.OperationId) || !management.ValidRequestID(record.Operation.RequestId) || !record.Operation.State.Valid() {
		return nil, errors.New("invalid management observation identity")
	}
	if filepath.Base(path) != filepath.Base(s.path(record.Operation.RequestId)) {
		return nil, errors.New("management observation filename mismatch")
	}
	return &record, nil
}

func (s *Store) List() ([]management.Observation, error) {
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return []management.Observation{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > management.RecordLimit*2 {
		return nil, management.ErrHistoryFull
	}
	records := make([]management.Observation, 0, len(entries))
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

// Save is called only by the owner of the cross-process management lock. An
// error after atomic publication may leave a visible record for later observation.
func (s *Store) Save(record management.Observation) error {
	if record.EnvironmentID != s.environmentID || record.Operation.EnvironmentId != s.environmentID || record.Operation.ContextId != record.ContextID {
		return errors.New("cannot save observation for a different context")
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(body) > management.RecordBytesLimit {
		return errors.New("management result exceeds the observation limit")
	}
	if err = s.ensureDirectory(); err != nil {
		return err
	}
	return atomicfile.Write(s.path(record.Operation.RequestId), append(body, '\n'), 0o600)
}

func (s *Store) ensureDirectory() error {
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
			return errors.New("management observation root must be a real directory")
		}
		if err = os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes only the selected observation. The application must hold the
// mutation lock and prove retention/undo eligibility before calling it.
func (s *Store) Delete(requestID string) error {
	return os.Remove(s.path(requestID))
}
