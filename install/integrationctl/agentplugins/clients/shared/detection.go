package shared

import "strings"

// FirstPath reports the first non-empty path. Clients whose product ships under
// more than one binary name (Kiro's legacy CLI, Devin next to Windsurf) resolve
// each candidate as its own detection surface and then pick one executable.
func FirstPath(paths ...string) string {
	for _, path := range paths {
		if strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}
