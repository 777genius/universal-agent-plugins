package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
)

// LocalSkillBackend retains native and local authority in the backend.
type LocalSkillBackend interface {
	PrepareLocalSkill(context.Context, LocalSkillRequest, string) (Prepared, error)
}

func (a *Application) SupportsLocalSkills() bool {
	_, ok := a.opts.Backend.(LocalSkillBackend)
	return ok
}

func (a *Application) PrepareLocalSkill(ctx context.Context, input LocalSkillRequest) (Plan, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Plan{}, err
	}
	defer leave()
	backend, ok := a.opts.Backend.(LocalSkillBackend)
	if !ok {
		return Plan{}, refusal(422, "CAPABILITY_UNAVAILABLE", "Local Skill authoring is not available in this engine", false)
	}
	if !input.Action.Valid() || !ValidID(input.RootId) || input.SkillName == "" || len(input.SkillName) > 255 || len(input.Files) > 200 {
		return Plan{}, refusal(400, "INVALID_REQUEST", "Invalid bounded local Skill selection", false)
	}
	input = cloneLocalSkillRequest(input)
	var restorePoint *LocalSkillBackup
	if input.Action == Restore {
		record, err := a.opts.Store.ByOperation(input.RestoreOperationId)
		if err != nil {
			return Plan{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot inspect the retained local operation", true)
		}
		if record == nil || record.Operation.Action != LocalSkill || record.Operation.LocalSkill == nil || record.Operation.LocalSkill.RootId != input.RootId || record.Operation.LocalSkill.SkillName != input.SkillName {
			return Plan{}, refusal(409, "RECOVERY_CONTEXT_MISMATCH", "Restore must reference the same retained local Skill operation", false)
		}
		restorePoint = localSkillRestorePoint(record.Operation)
	}
	if err := a.reservePreparation(true); err != nil {
		return Plan{}, err
	}
	defer a.releasePreparation()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	operationID := randomID()
	prepared, err := backend.PrepareLocalSkill(ctx, input, operationID)
	if err != nil {
		if prepared != nil {
			prepared.Close()
		}
		return Plan{}, err
	}
	identity := &LocalSkillIdentity{RootId: input.RootId, SkillName: input.SkillName}
	request := ChangeRequest{Action: LocalSkill, OperationId: input.RestoreOperationId}
	return a.retainPrepared(ctx, request, operationID, prepared, identity, restorePoint)
}

// UnmarshalJSON preserves required-nullable semantics: omitted content must not
// accidentally become a reviewed deletion. Generated wire fields remain intact.
func (f *LocalSkillFile) UnmarshalJSON(body []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	for _, key := range []string{"relativePath", "expectedSHA256", "contentBase64"} {
		if _, present := fields[key]; !present {
			return errors.New("local Skill files must explicitly include relativePath, expectedSHA256 and contentBase64")
		}
	}
	type wireFile LocalSkillFile
	var decoded wireFile
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*f = LocalSkillFile(decoded)
	return nil
}
