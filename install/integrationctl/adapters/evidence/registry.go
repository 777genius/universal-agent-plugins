package evidence

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Registry struct {
	FS   ports.FileSystem
	Path string
}

//go:embed embedded.json
var embeddedRegistry []byte

type document struct {
	SchemaVersion int                   `json:"schema_version"`
	Entries       []ports.EvidenceEntry `json:"entries"`
}

func (r Registry) Get(ctx context.Context, key string) (ports.EvidenceEntry, error) {
	entries, err := r.List(ctx)
	if err != nil {
		return ports.EvidenceEntry{}, err
	}
	for _, entry := range entries {
		if entry.Key == key {
			return entry, nil
		}
	}
	return ports.EvidenceEntry{}, fmt.Errorf("evidence key not found: %s", key)
}

func (r Registry) List(ctx context.Context) ([]ports.EvidenceEntry, error) {
	data, err := r.FS.ReadFile(ctx, r.Path)
	if err != nil {
		if os.IsNotExist(err) || r.Path == "" {
			data = embeddedRegistry
		} else {
			return nil, err
		}
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return append([]ports.EvidenceEntry(nil), doc.Entries...), nil
}
