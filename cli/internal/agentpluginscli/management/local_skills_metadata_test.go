package management

import "testing"

func TestLocalRestorePointMetadataRequiresMatchingIdentity(t *testing.T) {
	op := Operation{Action: LocalSkill, OperationId: "original", LocalSkill: &LocalSkillIdentity{RootId: "user", SkillName: "mine"}, Result: &OperationResult{LocalSkillBackup: &LocalSkillBackup{
		OperationId: "original", RootId: "user", SkillName: "mine", Path: "/private/backup",
	}}}
	point := localSkillRestorePoint(op)
	if point == nil || point.RecordedAt != nil || point.OriginalAction != "" {
		t.Fatal("missing metadata was invented", point)
	}
	for _, modify := range []func(*LocalSkillBackup){
		func(p *LocalSkillBackup) { p.OperationId = "different-operation" },
		func(p *LocalSkillBackup) { p.RootId = "different-root" },
		func(p *LocalSkillBackup) { p.SkillName = "different-skill" },
	} {
		backup := *op.Result.LocalSkillBackup
		modify(&backup)
		mismatch := op
		mismatch.Result = &OperationResult{LocalSkillBackup: &backup}
		if got := localSkillRestorePoint(mismatch); got != nil {
			t.Fatal("unrelated metadata was presented as the selected restore point", got)
		}
	}
}
