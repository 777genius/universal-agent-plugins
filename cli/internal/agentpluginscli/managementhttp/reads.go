package managementhttp

import (
	"net/http"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	value, err := s.application.Session(r.Context())
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	value, err := s.application.Catalog(r.Context(), r.URL.Query().Get("query"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) catalogDetail(w http.ResponseWriter, r *http.Request) {
	value, err := s.application.CatalogDetail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) skillsCatalog(w http.ResponseWriter, r *http.Request) {
	var input management.SourceRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	value, err := s.application.SkillsCatalog(r.Context(), input)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) skillsPreview(w http.ResponseWriter, r *http.Request) {
	if !s.application.SupportsSkillPreview() {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "This engine does not provide Skill previews", false)
		return
	}
	value, err := s.application.SkillPreview(r.Context(), r.PathValue("catalogId"), r.PathValue("candidateId"))
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) discardSkillsCatalog(w http.ResponseWriter, r *http.Request) {
	if !s.application.SupportsCatalogDiscard() {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "This engine cannot release Skill discovery sessions independently", false)
		return
	}
	if err := s.application.DiscardSkillsCatalog(r.Context(), r.PathValue("catalogId")); err != nil {
		writeBackendError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) environment(w http.ResponseWriter, r *http.Request) {
	value, err := s.application.Inspect(r.Context())
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) {
	var input management.UpdateCheckRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	value, err := s.application.CheckUpdates(r.Context(), input)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}
