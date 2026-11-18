package environment

import "testing"

func TestDevelopmentClassificationRejectsStaleLockedIntent(t *testing.T) {
	historical := requirementsFixture()
	oldDigest, err := ManifestDigest(historical)
	if err != nil {
		t.Fatal(err)
	}
	current := requirementsFixture()
	current.Development = map[string]bool{"docs": true}
	newDigest, err := ManifestDigest(current)
	if err != nil || newDigest == oldDigest {
		t.Fatalf("dev-only intent did not change digest: %v", err)
	}
	stale := Lock{LockVersion: LockVersion, ManifestDigest: oldDigest}
	if err := stale.Validate(current); err == nil {
		t.Fatal("stale locked manifest accepted")
	}
	current.Development["missing"] = true
	if err := current.Validate(); err == nil {
		t.Fatal("unknown development alias accepted")
	}
}

func TestDevelopmentClassificationDoesNotChangeNormalIntent(t *testing.T) {
	baseline := requirementsFixture()
	old, err := ManifestDigest(baseline)
	if err != nil {
		t.Fatal(err)
	}
	same := requirementsFixture()
	same.Development = map[string]bool{}
	got, err := ManifestDigest(same)
	if err != nil || got != old {
		t.Fatalf("empty development field changed v1 hash %s -> %s: %v", old, got, err)
	}
}
