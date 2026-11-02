package packagedigest

import "testing"

// Go package-tree admission uses pathpolicy for every segment. This verifies
// the source walker contract without creating forbidden names on Windows.
func TestSnapshotPathValidationRejectsSuperscriptDevices(t *testing.T) {
	for _, name := range []string{"COM¹", "a/com².txt", "a/b/LPT³/readme.md"} {
		t.Run(name, func(t *testing.T) {
			if err := validatePath(name, 64, map[string]string{}); err == nil {
				t.Fatalf("snapshot accepted Windows device alias %q", name)
			}
		})
	}
	for _, good := range []string{"COM10", "a/report¹.txt", "a/中文/readme.md"} {
		t.Run(good, func(t *testing.T) {
			if err := validatePath(good, 64, map[string]string{}); err != nil {
				t.Fatalf("snapshot rejected a portable Unicode path %q: %v", good, err)
			}
		})
	}
}
