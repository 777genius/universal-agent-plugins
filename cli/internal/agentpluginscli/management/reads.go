package management

import (
	"context"
	"net/http"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	value, err := s.opts.Backend.Session(r.Context())
	if err != nil {
		writeBackendError(w, err)
		return
	}
	value.ApiVersion = N1
	value.ContextId = s.opts.ContextID
	value.EnvironmentId = s.opts.EnvironmentID
	value.EngineVersion = s.opts.EngineVersion
	if value.Capabilities == nil {
		value.Capabilities = []Capability{}
	}
	if value.Targets == nil {
		value.Targets = []Target{}
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if len(query) > 256 {
		writeError(w, 400, "INVALID_REQUEST", "Search query is too long", false)
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	value, err := s.opts.Backend.Catalog(ctx, query)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	if value.Items == nil {
		value.Items = []CatalogEntry{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) catalogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" || len(id) > 256 {
		writeError(w, 400, "INVALID_REQUEST", "Invalid catalog identity", false)
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	value, err := s.opts.Backend.CatalogDetail(ctx, id)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) skillsCatalog(w http.ResponseWriter, r *http.Request) {
	var input SourceRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	if input.Source == "" || len(input.Source) > 2048 || len(input.Ref) > 256 || len(input.Path) > 4096 {
		writeError(w, 400, "INVALID_REQUEST", "Source or selector is missing or too long", false)
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	value, err := s.opts.Backend.SkillsCatalog(ctx, input)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	if value.Candidates == nil {
		value.Candidates = []SkillCandidate{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	loopbackapi.WriteJSON(w, 200, value)
}

type skillPreviewBackend interface {
	SkillPreview(context.Context, string, string) (SkillPreview, error)
}

func (s *Server) skillsPreview(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.opts.Backend.(skillPreviewBackend)
	if !ok {
		writeError(w, 422, "CAPABILITY_UNAVAILABLE", "This engine does not provide Skill previews", false)
		return
	}
	catalogID, candidateID := r.PathValue("catalogId"), r.PathValue("candidateId")
	if !validID(catalogID) || !validID(candidateID) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid catalog or candidate identity", false)
		return
	}
	ctx, cancel := s.requestContext(r)
	defer cancel()
	value, err := backend.SkillPreview(ctx, catalogID, candidateID)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func (s *Server) environment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.requestContext(r)
	defer cancel()
	value, err := s.opts.Backend.Inspect(ctx)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	pending, backups, err := s.environmentOperations(ctx)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Accepted-operation history is unavailable; do not infer an idle environment", true)
		return
	}
	value.ContextId = s.opts.ContextID
	value.PendingOperations = pending
	value.LocalSkillBackups = backups
	if value.Entries == nil {
		value.Entries = []EnvironmentEntry{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	loopbackapi.WriteJSON(w, 200, value)
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 256 {
		return false
	}
	for _, r := range id {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func validRequestID(id string) bool {
	if len(id) < 16 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-_", r) {
			continue
		}
		return false
	}
	return true
}

func validIDs(ids []string, limit int) bool {
	if len(ids) > limit {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
