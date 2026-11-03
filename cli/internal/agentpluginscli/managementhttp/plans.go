package managementhttp

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	var input management.ChangeRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	value, err := s.application.Prepare(r.Context(), input)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) prepareLocalSkill(w http.ResponseWriter, r *http.Request) {
	if !s.application.SupportsLocalSkills() {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "Local Skill authoring is not available in this engine", false)
		return
	}
	var input management.LocalSkillRequest
	if !decodeLocalSkill(w, r, &input) {
		return
	}
	value, err := s.application.PrepareLocalSkill(r.Context(), input)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	var input management.ApplyRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	value, err := s.application.Apply(r.Context(), r.PathValue("id"), input.RequestId)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, http.StatusAccepted, value)
}

func (s *Server) discardPlan(w http.ResponseWriter, r *http.Request) {
	if err := s.application.DiscardPlan(r.Context(), r.PathValue("id")); err != nil {
		writeBackendError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) operation(w http.ResponseWriter, r *http.Request) {
	value, err := s.application.Operation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) cancelOperation(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		var input struct{}
		if !loopbackapi.Decode(w, r, &input, writeError) {
			return
		}
	}
	value, err := s.application.CancelOperation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, http.StatusAccepted, value)
}

// Binary local imports retain their larger route-specific cap; ordinary JSON
// requests continue to use loopbackapi's 1 MiB limit.
func decodeLocalSkill(w http.ResponseWriter, r *http.Request, value *management.LocalSkillRequest) bool {
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
