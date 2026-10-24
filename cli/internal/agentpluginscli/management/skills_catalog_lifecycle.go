package management

import (
	"context"
	"net/http"
)

type skillCatalogDisposer interface {
	DiscardSkillsCatalog(context.Context, string) error
}

func (s *Server) discardSkillsCatalog(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.opts.Backend.(skillCatalogDisposer)
	if !ok {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "This engine cannot release Skill discovery sessions independently", false)
		return
	}
	id := r.PathValue("catalogId")
	if !validID(id) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid catalog identity", false)
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	if err := backend.DiscardSkillsCatalog(ctx, id); err != nil {
		writeBackendError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
