package agentpluginscli

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type scopedFloorTestStore struct {
	state domain.StateFileV2
	err error
}

func (s scopedFloorTestStore) Load() (domain.StateFileV2, error) { return s.state, s.err }
func (s scopedFloorTestStore) Save(domain.StateFileV2) error {
	return errors.New("read-only floor test unexpectedly wrote state")
}

type scopedFloorV2Spy struct{ *fixedDirectoryClient }

func (scopedFloorV2Spy) SnapshotSchema() int { return 2 }

func TestDirectoryDependencyLoadKeepsIndependentReceiptHighWaters(t *testing.T) {
	store := scopedFloorTestStore{state: domain.StateFileV2{Installations: []domain.Installation{
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 1, SnapshotSequence: 43}},
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 2, SnapshotSequence: 4}},
	}}}
	v1 := &fixedDirectoryClient{bundle: readModelBundle()}
	a := App{DirectoryClient: v1, StateStore: store}
	if _, err := a.loadAuthorizedDirectory(context.Background()); err != nil || v1.floor != 43 {
		t.Fatalf("v1 restored from wrong sequence: %d %v", v1.floor, err)
	}
	v2 := &fixedDirectoryClient{bundle: readModelBundle()}
	a.DirectoryClient = scopedFloorV2Spy{fixedDirectoryClient: v2}
	if _, err := a.loadAuthorizedDirectory(context.Background()); err != nil || v2.floor != 4 {
		t.Fatalf("v2 borrowed v1 sequence: %d %v", v2.floor, err)
	}
	a.StateStore = scopedFloorTestStore{err: errors.New("unreadable state")}
	before := v2.calls
	if _, err := a.loadAuthorizedDirectory(context.Background()); err == nil || v2.calls != before {
		t.Fatalf("unreadable existing authority permitted Directory fetch: %v", err)
	}
}
