package management

import "context"

type skillCatalogDisposer interface {
	DiscardSkillsCatalog(context.Context, string) error
}

func (a *Application) SupportsCatalogDiscard() bool {
	_, ok := a.opts.Backend.(skillCatalogDisposer)
	return ok
}
func (a *Application) DiscardSkillsCatalog(ctx context.Context, id string) error {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return err
	}
	defer leave()
	backend, ok := a.opts.Backend.(skillCatalogDisposer)
	if !ok {
		return refusal(422, "CAPABILITY_UNAVAILABLE", "This engine cannot release Skill discovery sessions independently", false)
	}
	if !ValidID(id) {
		return refusal(400, "INVALID_REQUEST", "Invalid catalog identity", false)
	}
	ctx, cancel := operationContext(ctx)
	defer cancel()
	return backend.DiscardSkillsCatalog(ctx, id)
}
