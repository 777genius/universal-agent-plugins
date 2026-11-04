package management

import "slices"

func cloneChange(input ChangeRequest) ChangeRequest {
	input.Aliases = slices.Clone(input.Aliases)
	input.TargetIds = slices.Clone(input.TargetIds)
	input.CandidateIds = slices.Clone(input.CandidateIds)
	return input
}
func cloneLocalSkillRequest(input LocalSkillRequest) LocalSkillRequest {
	input.Files = slices.Clone(input.Files)
	for i := range input.Files {
		if value := input.Files[i].ContentBase64; value != nil {
			copied := *value
			input.Files[i].ContentBase64 = &copied
		}
		if value := input.Files[i].ExpectedSHA256; value != nil {
			copied := *value
			input.Files[i].ExpectedSHA256 = &copied
		}
	}
	return input
}
func cloneBackup(backup *LocalSkillBackup) *LocalSkillBackup {
	if backup == nil {
		return nil
	}
	out := *backup
	if backup.RecordedAt != nil {
		at := *backup.RecordedAt
		out.RecordedAt = &at
	}
	return &out
}
func clonePlan(plan Plan) Plan {
	plan.Actions = slices.Clone(plan.Actions)
	plan.Warnings = slices.Clone(plan.Warnings)
	plan.Conflicts = slices.Clone(plan.Conflicts)
	plan.RestorePoint = cloneBackup(plan.RestorePoint)
	return plan
}
func cloneOperation(op Operation) Operation {
	op.Items = slices.Clone(op.Items)
	op.Diagnostics = slices.Clone(op.Diagnostics)
	if op.LocalSkill != nil {
		identity := *op.LocalSkill
		op.LocalSkill = &identity
	}
	if op.Result != nil {
		result := *op.Result
		result.Items = slices.Clone(result.Items)
		result.NextActions = slices.Clone(result.NextActions)
		result.LocalSkillBackup = cloneBackup(result.LocalSkillBackup)
		op.Result = &result
	}
	return op
}
func cloneObservation(record Observation) Observation {
	record.InstallerIDs = slices.Clone(record.InstallerIDs)
	record.Operation = cloneOperation(record.Operation)
	return record
}
