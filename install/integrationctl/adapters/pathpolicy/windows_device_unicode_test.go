package pathpolicy

import (
	"strings"
	"testing"
)

// The Win32 file namespace treats superscript 1/2/3 (ISO-8859-1) as
// device digits, even for files carrying a normal extension. Host OS must
// not affect the package's admission result.
func TestPortableSegmentsRejectSuperscriptDeviceAliases(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"COM", "LPT"} {
		for _, digit := range []string{"¹", "²", "³"} {
			for _, suffix := range []string{"", ".txt", ".tar.gz"} {
				bad := prefix + digit + suffix
				for _, candidate := range []string{bad, strings.ToLower(bad)} {
					t.Run(candidate, func(t *testing.T) {
						if err := ValidatePortablePathSegment(candidate); err == nil {
							t.Fatalf("Windows device alias admitted as a portable package segment: %q", candidate)
						}
					})
				}
			}
		}
	}
	for _, good := range []string{"COM10", "LPT10", "COM⁴", "report¹.txt", "中文", "skills-é", "valid.md"} {
		if err := ValidatePortablePathSegment(good); err != nil {
			t.Fatalf("non-device portable name %q was rejected: %v", good, err)
		}
	}
}
