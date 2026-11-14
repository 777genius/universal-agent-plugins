package jsonstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const supportedSchemaVersion = 1

type Store struct {
	FS   ports.FileSystem
	Path string
}

func (s Store) Load(ctx context.Context) (ports.StateFile, error) {
	if s.Path == "" {
		return ports.StateFile{}, errors.New("state path required")
	}
	data, err := s.FS.ReadFile(ctx, s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ports.StateFile{SchemaVersion: supportedSchemaVersion}, nil
		}
		return ports.StateFile{}, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return ports.StateFile{}, domain.NewError(domain.ErrStateConflict, "decode integration state", err)
	}
	if header.SchemaVersion != 0 && header.SchemaVersion != supportedSchemaVersion {
		return ports.StateFile{}, domain.NewError(domain.ErrStateConflict, fmt.Sprintf("unsupported state schema_version %d (supported: %d)", header.SchemaVersion, supportedSchemaVersion), nil)
	}
	var out ports.StateFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return ports.StateFile{}, domain.NewError(domain.ErrStateConflict, "decode integration state", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ports.StateFile{}, domain.NewError(domain.ErrStateConflict, "integration state contains trailing JSON values", err)
	}
	if out.SchemaVersion == 0 {
		out.SchemaVersion = supportedSchemaVersion
	}
	return out, nil
}

func (s Store) Save(ctx context.Context, state ports.StateFile) error {
	if s.Path == "" {
		return domain.NewError(domain.ErrStateConflict, "state path required", nil)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = supportedSchemaVersion
	}
	if state.SchemaVersion != supportedSchemaVersion {
		return domain.NewError(domain.ErrStateConflict, fmt.Sprintf("refusing to write unsupported state schema_version %d (supported: %d)", state.SchemaVersion, supportedSchemaVersion), nil)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := s.FS.MkdirAll(ctx, filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	return s.FS.WriteFileAtomic(ctx, s.Path, data, 0o644)
}
