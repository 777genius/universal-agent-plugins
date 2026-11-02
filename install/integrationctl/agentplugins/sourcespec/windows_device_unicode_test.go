package sourcespec

import "testing"

// One portable source grammar must match archive and snapshot path admission.
func TestGitAndArchiveSubpathRejectWindowsUnicodeDeviceNames(t *testing.T) {
	for _, name := range []string{"COM¹", "skills/com²", "a/LPT³/skill.md", "LPT¹.tar.gz"} {
		if _, err := NormalizeSubpath(name); err == nil {
			t.Errorf("accepted nonportable source subpath %q", name)
		}
	}
	for _, name := range []string{"skills/report¹", "skills/中文", "COM10"} {
		if value, err := NormalizeSubpath(name); err != nil || value != name {
			t.Errorf("changed valid Unicode subpath %q: %q, %v", name, value, err)
		}
	}
}
