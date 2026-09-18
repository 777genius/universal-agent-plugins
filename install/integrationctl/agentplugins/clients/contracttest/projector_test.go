package contracttest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectingAdapter struct {
	exampleAdapter
	project func(clients.ProjectionInput) ([]domain.NativeObjectOwnership, error)
}

func (a projectingAdapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	return a.project(in)
}

func TestRunProjectorAcceptsAWellBehavedAdapter(t *testing.T) {
	RunProjector(t, projectingAdapter{
		exampleAdapter: exampleAdapter{id: domain.ClientCursor},
		project: func(in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
			if err := os.WriteFile(filepath.Join(in.StagingPath, "projected.txt"), []byte("ok\n"), 0o644); err != nil {
				return nil, err
			}
			return []domain.NativeObjectOwnership{{
				ObjectID: "cursor-note:demo", Kind: "cursor_note", Path: in.Plan.ActivePath,
			}}, nil
		},
	})
}

func TestProjectorViolationsRejectABrokenProjection(t *testing.T) {
	calls := 0
	cases := map[string]clients.Adapter{
		"no projection capability": exampleAdapter{id: domain.ClientCursor},
		"managed package object": projectingAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			project: func(clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
				return []domain.NativeObjectOwnership{{Kind: managedPackageDirectoryKind}}, nil
			},
		},
		"write outside staging": projectingAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			project: func(in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
				return nil, os.WriteFile(filepath.Join(filepath.Dir(in.StagingPath), "escaped.txt"), []byte("no\n"), 0o644)
			},
		},
		"non-deterministic objects": projectingAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			project: func(clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
				calls++
				return []domain.NativeObjectOwnership{{ObjectID: fmt.Sprintf("note-%d", calls), Kind: "note"}}, nil
			},
		},
	}
	for name, adapter := range cases {
		if violations := projectorViolations(t, adapter); len(violations) == 0 {
			t.Errorf("projectorViolations accepted the %q adapter", name)
		}
	}
}
