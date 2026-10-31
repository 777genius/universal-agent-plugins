package domain

import "strings"

// ReservedStdioEnvKey identifies the client-managed environment keys.
// Windows compares environment variable names without regard to case;
// other platforms may use distinct lowercase names.
func ReservedStdioEnvKey(env any, windows bool) string {
	present := func(want string) bool {
		switch values := env.(type) {
		case map[string]any:
			for key := range values {
				if key == want || (windows && strings.EqualFold(key, want)) {
					return true
				}
			}
		case map[string]string:
			for key := range values {
				if key == want || (windows && strings.EqualFold(key, want)) {
					return true
				}
			}
		}
		return false
	}
	for _, reserved := range []string{"PLUGIN_ROOT", "PLUGIN_DATA"} {
		if present(reserved) {
			return reserved
		}
	}
	return ""
}
