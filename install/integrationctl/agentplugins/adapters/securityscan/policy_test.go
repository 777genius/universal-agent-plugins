package securityscan

import "testing"

func TestDefaultPolicyDigestMatchesSignedSecurityIndexContract(t *testing.T) {
	const expected = "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"
	if actual := DefaultRequirement().Policy.Digest; actual != expected {
		t.Fatalf("policy digest = %q, want %q", actual, expected)
	}
}

func TestDispositionKeepsContextualRemoteExecutionAsWarning(t *testing.T) {
	if actual := disposition("SEC102", "warn", "high"); actual != "warning" {
		t.Fatalf("SEC102 disposition = %q, want warning", actual)
	}
	if actual := disposition("SEC330", "warn", "high"); actual != "blocking" {
		t.Fatalf("SEC330 disposition = %q, want blocking", actual)
	}
	if actual := disposition("SEC102", "deny", "high"); actual != "blocking" {
		t.Fatalf("deny disposition = %q, want blocking", actual)
	}
}
