package management

import (
	"context"
	"errors"
	"strings"
)

type skillPreviewBackend interface {
	SkillPreview(context.Context, string, string) (SkillPreview, error)
}

func (a *Application) SupportsSkillPreview() bool {
	_, ok := a.opts.Backend.(skillPreviewBackend)
	return ok
}

func (a *Application) Session(ctx context.Context) (Session, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Session{}, err
	}
	defer leave()
	value, err := a.opts.Backend.Session(ctx)
	if err != nil {
		return Session{}, err
	}
	value.ApiVersion = N1
	value.ContextId = a.opts.ContextID
	value.EnvironmentId = a.opts.EnvironmentID
	value.EngineVersion = a.opts.EngineVersion
	if value.Capabilities == nil {
		value.Capabilities = []Capability{}
	}
	if value.Targets == nil {
		value.Targets = []Target{}
	}
	return value, nil
}

func (a *Application) Catalog(ctx context.Context, query string) (Catalog, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Catalog{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	if len(query) > 256 {
		return Catalog{}, refusal(400, "INVALID_REQUEST", "Search query is too long", false)
	}
	value, err := a.opts.Backend.Catalog(ctx, query)
	if err != nil {
		return Catalog{}, err
	}
	if value.Items == nil {
		value.Items = []CatalogEntry{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	return value, nil
}

func (a *Application) CatalogDetail(ctx context.Context, id string) (CatalogEntry, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return CatalogEntry{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	if id == "" || len(id) > 256 {
		return CatalogEntry{}, refusal(400, "INVALID_REQUEST", "Invalid catalog identity", false)
	}
	return a.opts.Backend.CatalogDetail(ctx, id)
}

func (a *Application) SkillsCatalog(ctx context.Context, input SourceRequest) (SkillCatalog, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return SkillCatalog{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	if input.Source == "" || len(input.Source) > 2048 || len(input.Ref) > 256 || len(input.Path) > 4096 {
		return SkillCatalog{}, refusal(400, "INVALID_REQUEST", "Source or selector is missing or too long", false)
	}
	value, err := a.opts.Backend.SkillsCatalog(ctx, input)
	if err != nil {
		return SkillCatalog{}, err
	}
	if value.Candidates == nil {
		value.Candidates = []SkillCandidate{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	return value, nil
}

func (a *Application) SkillPreview(ctx context.Context, catalogID, candidateID string) (SkillPreview, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return SkillPreview{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	backend, ok := a.opts.Backend.(skillPreviewBackend)
	if !ok {
		return SkillPreview{}, refusal(422, "CAPABILITY_UNAVAILABLE", "This engine does not provide Skill previews", false)
	}
	if !ValidID(catalogID) || !ValidID(candidateID) {
		return SkillPreview{}, refusal(400, "INVALID_REQUEST", "Invalid catalog or candidate identity", false)
	}
	return backend.SkillPreview(ctx, catalogID, candidateID)
}

func (a *Application) Inspect(ctx context.Context) (Environment, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Environment{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	value, err := a.opts.Backend.Inspect(ctx)
	if err != nil {
		return Environment{}, err
	}
	pending, backups, err := a.environmentOperations(ctx)
	if err != nil {
		return Environment{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Accepted-operation history is unavailable; do not infer an idle environment", true)
	}
	value.ContextId = a.opts.ContextID
	value.PendingOperations = pending
	value.LocalSkillBackups = backups
	if value.Entries == nil {
		value.Entries = []EnvironmentEntry{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	return value, nil
}

func (a *Application) CheckUpdates(ctx context.Context, input UpdateCheckRequest) (UpdateCheck, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return UpdateCheck{}, err
	}
	defer leave()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	aliases := []string{}
	if input.Aliases != nil {
		aliases = append(aliases, (*input.Aliases)...)
		input.Aliases = &aliases
	}
	if !validIDs(aliases, 2000) || input.All && len(aliases) > 0 || !input.All && len(aliases) == 0 {
		return UpdateCheck{}, refusal(400, "INVALID_REQUEST", "Choose unique declared aliases or all", false)
	}
	value, err := a.opts.Backend.CheckUpdates(ctx, input)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return UpdateCheck{}, refusal(504, "UPDATE_CHECK_TIMEOUT", "The update check timed out; retry the read-only check", true)
		}
		return UpdateCheck{}, err
	}
	value.ContextId = a.opts.ContextID
	if value.Entries == nil {
		value.Entries = []UpdateCheckEntry{}
	}
	if value.Diagnostics == nil {
		value.Diagnostics = []Diagnostic{}
	}
	return value, nil
}

// ValidID is the shared bounded identifier rule for intent and durable records.
func ValidID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	for _, r := range id {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func ValidRequestID(id string) bool {
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
		if !ValidID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
