package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type scopedFloorTestStore struct {
	state domain.StateFileV2
	err   error
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

type rollbackSensitiveLocalDirectory struct {
	*fixedDirectoryClient
	floor uint64
}

func (*rollbackSensitiveLocalDirectory) SnapshotSchema() int { return 2 }

func (c *rollbackSensitiveLocalDirectory) LoadLocal(floor uint64) (directoryv1.VerifiedBundle, error) {
	c.floor = floor
	if floor > c.bundle.Snapshot.Sequence {
		return directoryv1.VerifiedBundle{}, directoryv1.ErrRollback
	}
	return c.bundle, nil
}

func TestDirectRevocationWarningUsesSelectedDirectorySchemaFloor(t *testing.T) {
	bundle := readModelBundle()
	bundle.Snapshot.SnapshotSchemaVersion = 2
	bundle.Snapshot.Sequence = 4
	distribution := &bundle.Snapshot.Distributions[0]
	policy := &distribution.ReleasePolicies[0]
	policy.Status = domain.ReleaseRevoked
	var digest string
	for _, release := range distribution.Releases {
		if release.Sequence == policy.ReleaseSequence {
			digest = release.TreeDigest
			break
		}
	}
	if digest == "" {
		t.Fatal("revocation fixture must match an existing release")
	}

	client := &rollbackSensitiveLocalDirectory{fixedDirectoryClient: &fixedDirectoryClient{bundle: bundle}}
	state := domain.StateFileV2{Installations: []domain.Installation{
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 1, SnapshotSequence: 43}},
		{Directory: &domain.DirectoryOrigin{SnapshotSchema: 2, SnapshotSequence: 4}},
	}}
	var warnings bytes.Buffer
	app := App{DirectoryClient: client, StateStore: scopedFloorTestStore{state: state}, ErrorOutput: &warnings}
	app.warnDirectRevocation(digest)
	if client.floor != 4 || !strings.Contains(warnings.String(), "WARNING [directory_release_revoked]") {
		t.Fatalf("v2 direct-source warning lost to another protocol's floor: floor=%d warnings=%q", client.floor, warnings.String())
	}
}
