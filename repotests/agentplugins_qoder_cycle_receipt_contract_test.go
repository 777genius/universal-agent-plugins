//go:build linux

package pluginkitairepo_test

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestQoderCycleReceiptRefusesForeignAuthorityPathsAndDigests(t *testing.T) {
	c := qoderCycleInert(t)
	qoderCycleStageInert(t, c, "a")
	b := c.recipes["a"].Binding
	group := "op-0123456789abcdef0123456789abcdef"
	operation := group + "-001"
	suffix := qoderDeltaHash(operation)[:16]
	digest := b.NativeObjects[0].ManagedDigest
	valid := domain.MutationReceipt{OperationID: operation, OperationGroupID: group, Sequence: 1, MutationType: "directory_swap", ClientBindingID: b.ClientBindingID, ActivePath: b.TargetLocator, StagingPath: filepath.Join(b.NativeProfileRoot, ".agentplugins-staging-"+suffix), BackupPath: filepath.Join(b.NativeProfileRoot, ".agentplugins-backup-"+suffix), AfterDigest: digest, Phase: "committed"}
	if err := qoderCycleReceipt(valid, b, 1, "directory_swap", "", digest); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*domain.MutationReceipt){
		func(r *domain.MutationReceipt) { r.ActivePath = c.f.other },
		func(r *domain.MutationReceipt) { r.StagingPath = filepath.Join(c.f.base.Home, "foreign") },
		func(r *domain.MutationReceipt) { r.BackupPath += "-other" },
		func(r *domain.MutationReceipt) { r.DataReceiptID = "foreign" },
		func(r *domain.MutationReceipt) {
			r.ProfileOwners = []domain.PhysicalProfileOwner{{Namespace: "foreign"}}
		},
		func(r *domain.MutationReceipt) { r.DirectoryProof = []byte(`{}`) },
		func(r *domain.MutationReceipt) { r.EnvironmentAuthority = &domain.EnvironmentAuthority{} },
		func(r *domain.MutationReceipt) { r.BeforeDigest = digest },
		func(r *domain.MutationReceipt) { r.AfterDigest = "sha256:foreign" },
		func(r *domain.MutationReceipt) { r.Sequence++ },
		func(r *domain.MutationReceipt) { r.OperationID = group + "-002" },
	} {
		wrong := valid
		mutate(&wrong)
		if qoderCycleReceipt(wrong, b, 1, "directory_swap", "", digest) == nil {
			t.Fatal("foreign/extra receipt authority was admitted")
		}
	}
}
