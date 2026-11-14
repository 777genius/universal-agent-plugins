package journal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const journalSchemaVersion = 1

type FileJournal struct {
	FS      ports.FileSystem
	BaseDir string
}

func (j FileJournal) Start(ctx context.Context, op domain.OperationRecord) error {
	path, err := j.path(op.OperationID)
	if err != nil {
		return err
	}
	info, err := j.FS.Stat(ctx, path)
	if err != nil {
		return err
	}
	if info.Exists {
		return domain.NewError(domain.ErrStateConflict, "operation journal already exists", nil)
	}
	return j.write(ctx, op)
}

func (j FileJournal) AppendStep(ctx context.Context, operationID string, step domain.JournalStep) error {
	op, err := j.read(ctx, operationID)
	if err != nil {
		return err
	}
	op.Steps = append(op.Steps, step)
	return j.write(ctx, op)
}

func (j FileJournal) Finish(ctx context.Context, operationID, status string) error {
	op, err := j.read(ctx, operationID)
	if err != nil {
		return err
	}
	op.Status = status
	return j.write(ctx, op)
}

func (j FileJournal) ListOpen(ctx context.Context) ([]domain.OperationRecord, error) {
	if err := j.FS.MkdirAll(ctx, j.BaseDir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(j.BaseDir)
	if err != nil {
		return nil, err
	}
	var out []domain.OperationRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		op, err := j.read(ctx, trimExt(entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read operation journal %s: %w", entry.Name(), err)
		}
		if op.Status != "committed" && op.Status != "rolled_back" {
			out = append(out, op)
		}
	}
	return out, nil
}

func (j FileJournal) read(ctx context.Context, operationID string) (domain.OperationRecord, error) {
	path, err := j.path(operationID)
	if err != nil {
		return domain.OperationRecord{}, err
	}
	data, err := j.FS.ReadFile(ctx, path)
	if err != nil {
		return domain.OperationRecord{}, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return domain.OperationRecord{}, domain.NewError(domain.ErrStateConflict, "decode operation journal", err)
	}
	if header.SchemaVersion != 0 && header.SchemaVersion != journalSchemaVersion {
		return domain.OperationRecord{}, domain.NewError(domain.ErrStateConflict, fmt.Sprintf("unsupported operation journal schema_version %d", header.SchemaVersion), nil)
	}
	var op domain.OperationRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&op); err != nil {
		return domain.OperationRecord{}, domain.NewError(domain.ErrStateConflict, "decode operation journal", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.OperationRecord{}, domain.NewError(domain.ErrStateConflict, "operation journal contains trailing JSON values", err)
	}
	if op.SchemaVersion == 0 {
		op.SchemaVersion = journalSchemaVersion
	}
	if err := pathpolicy.ValidateLeafID(op.OperationID); err != nil {
		return domain.OperationRecord{}, domain.NewError(domain.ErrStateConflict, "operation journal has unsafe operation id", err)
	}
	return op, nil
}

func (j FileJournal) write(ctx context.Context, op domain.OperationRecord) error {
	path, err := j.path(op.OperationID)
	if err != nil {
		return err
	}
	if op.SchemaVersion == 0 {
		op.SchemaVersion = journalSchemaVersion
	}
	if op.SchemaVersion != journalSchemaVersion {
		return domain.NewError(domain.ErrStateConflict, fmt.Sprintf("refusing to write unsupported operation journal schema_version %d", op.SchemaVersion), nil)
	}
	if err := j.FS.MkdirAll(ctx, j.BaseDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return j.FS.WriteFileAtomic(ctx, path, data, 0o644)
}

func (j FileJournal) path(operationID string) (string, error) {
	if err := pathpolicy.ValidateLeafID(operationID); err != nil {
		return "", domain.NewError(domain.ErrStateConflict, "unsafe operation journal id", err)
	}
	return filepath.Join(j.BaseDir, operationID+".json"), nil
}
func trimExt(name string) string { return name[:len(name)-len(filepath.Ext(name))] }
