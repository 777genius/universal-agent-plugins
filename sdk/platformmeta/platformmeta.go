package platformmeta

import "strings"

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
