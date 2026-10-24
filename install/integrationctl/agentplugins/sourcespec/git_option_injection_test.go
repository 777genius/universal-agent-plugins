package sourcespec

import "testing"

// A leading dash in an SCP-style host or user would reach git as an option
// (for example --upload-pack) when the endpoint is passed positionally.
func TestNormalizeGitURLRejectsLeadingDashInSCPHostOrUser(t *testing.T) {
	for _, endpoint := range []string{"--upload-pack=touch.pwned:x", "-x@host.example:repo.git", "user@-host.example:repo.git"} {
		if _, _, err := NormalizeGitURL(endpoint); err == nil {
			t.Fatalf("option-like SCP endpoint accepted: %q", endpoint)
		}
	}
}
