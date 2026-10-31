package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

// LocalSkillBackend shares the ordinary retained-plan/acceptance lifecycle. It
// does not expose a filesystem lease or allow a caller to supply absolute paths.
type LocalSkillBackend interface {
	PrepareLocalSkill(context.Context, LocalSkillRequest, string) (Prepared, error)
}

func (s *Server) registerLocalSkillsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/skills/local/plans", s.prepareLocalSkill)
}

func (s *Server) prepareLocalSkill(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.opts.Backend.(LocalSkillBackend)
	if !ok {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "Local Skill authoring is not available in this engine", false)
		return
	}
	var input LocalSkillRequest
	if !decodeLocalSkill(w, r, &input) {
		return
	}
	if !input.Action.Valid() || !validID(input.RootId) || input.SkillName == "" || len(input.SkillName) > 255 || len(input.Files) > 200 {
		writeError(w, 400, "INVALID_REQUEST", "Invalid bounded local Skill selection", false)
		return
	}
	var restorePoint *LocalSkillBackup
	if string(input.Action) == "restore" {
		record, err := s.store.byOperation(input.RestoreOperationId)
		if err != nil {
			writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot inspect the retained local operation", true)
			return
		}
		if record == nil || record.Operation.Action != ChangeAction("local_skill") || record.Operation.LocalSkill == nil || record.Operation.LocalSkill.RootId != input.RootId || record.Operation.LocalSkill.SkillName != input.SkillName {
			writeError(w, 409, "RECOVERY_CONTEXT_MISMATCH", "Restore must reference the same retained local Skill operation", false)
			return
		}
		restorePoint = localSkillRestorePoint(record.Operation)
	}
	if !s.reservePreparation(w, true) {
		return
	}
	defer func() { s.mu.Lock(); s.preparing = false; s.mu.Unlock() }()
	ctx, cancel := s.requestContext(r)
	defer cancel()
	operationID := loopbackapi.RandomID()
	prepared, err := backend.PrepareLocalSkill(ctx, input, operationID)
	if err != nil {
		if prepared != nil {
			prepared.Close()
		}
		writeBackendError(w, err)
		return
	}
	identity := &LocalSkillIdentity{RootId: input.RootId, SkillName: input.SkillName}
	request := ChangeRequest{Action: LocalSkill, OperationId: input.RestoreOperationId}
	s.retainPrepared(ctx, w, request, operationID, prepared, identity, restorePoint)
}

// Base64 preserves the editor's binary import feature. The larger cap is local
// to this route; ordinary management requests retain the shared 1 MiB bound.
func decodeLocalSkill(w http.ResponseWriter, r *http.Request, value *LocalSkillRequest) bool {
	if ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || ct != "application/json" {
		writeError(w, 415, "INVALID_REQUEST", "Content-Type must be application/json", false)
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Invalid or oversized local Skill request", false)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, 400, "INVALID_REQUEST", "Expected one JSON document", false)
		return false
	}
	return true
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
