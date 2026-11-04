package agentpluginscli

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestScopedDirectoryHighWaterMarksNeverBorrowAnotherProtocol(t *testing.T) {
	state := domain.StateFileV2{Installations: []domain.Installation{
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 1, SnapshotSequence: 43}},
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 2, SnapshotSequence: 1}},
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 2, SnapshotSequence: 3}},
	}}
	if floor := installedDirectoryFloor(state); floor != 43 {
		t.Fatalf("v1 floor was changed: %d", floor)
	}
	if floor := directoryFloorForClient(state, &directoryv1.V2Client{}); floor != 3 {
		t.Fatalf("v2 inherited v1 floor or ignored its own receipt: %d", floor)
	}
	if floor := directoryFloorForClient(state, nil); floor != 43 {
		t.Fatalf("default v1 client changed: %d", floor)
	}
	if floor := installedDirectoryFloorForSchema(state, 2); floor != 3 {
		t.Fatalf("explicit v2 receipt floor mismatch: %d", floor)
	}
}
