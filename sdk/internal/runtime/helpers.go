package runtime

import "strings"

func NormalizeHookName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))
}

func InternalHookTypeMismatch(name string) error {
	return &internalHookTypeMismatch{name: name}
}

type internalHookTypeMismatch struct {
	name string
}

func (e *internalHookTypeMismatch) Error() string {
	return "internal hook type mismatch for " + e.name
}
