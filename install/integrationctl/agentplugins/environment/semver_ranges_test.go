package environment

import "testing"

// Frozen expected results for the shared npm-style subset. Node-semver and
// Masterminds are not interchangeable for every syntactic edge; these cases
// describe UAP's explicitly supported canonical contract.
func TestRegistrySemverConstraints(t *testing.T) {
	for _, tc := range []struct {
		expression, version string
		accept              bool
	}{
		{"^1.2", "1.2.0", true},
		{"^1.2", "2.0.0", false},
		{"^0.0", "0.0.8", true},
		{"^0.0.3", "0.0.4", false},
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{">=1.2.3 <2.0.0", "1.9.99", true},
		{">=1.2.3 <2.0.0", "2.0.0", false},
		{"1.2.x", "1.2.8", true},
		{"1.2.x", "1.3.0", false},
		{"1.2", "1.2.7", true},
		{"^1.2 || ^3.0", "3.4.0", true},
		{"^1.2 || ^3.0", "2.4.0", false},
		{"^1.2", "1.3.0-beta.1", false},
		{">=1.3.0-beta.1 <1.3.0", "1.3.0-beta.2", true},
		{">=1.3.0-beta.1 <2.0.0", "1.3.1-beta.1", false},
		{">=1.3.0-beta.1 <2.0.0", "1.3.1", true},
		{"^1.2.3 || ^3.0.0-beta.1", "3.0.1-beta.1", false},
		{"^1.2.3 || ^3.0.0-beta.1", "3.0.0-beta.2", true},
		{"*", "1.3.0-beta.1", false},
		{"*", "1.3.0", true},
		{"1.2.3+build.2", "1.2.3+build.2", true},
		{"1.2.3+build.2", "1.2.3+build.1", true},
		{">=1.2.3", "v1.3.0", false},
		{"^1.2", "01.2.3", false},
	} {
		t.Run(tc.expression+"/"+tc.version, func(t *testing.T) {
			selector, err := ParseRegistryVersionSelector(tc.expression)
			if err != nil {
				t.Fatal(err)
			}
			if got := SelectorMatchesVersion(selector, tc.version); got != tc.accept {
				t.Fatalf("%s against %s = %v, want %v", tc.expression, tc.version, got, tc.accept)
			}
		})
	}
}

func TestRegistryVersionSyntaxRejectsUnsafeInput(t *testing.T) {
	for _, expression := range []string{"", "^", "v1.2.3", "^v1.2.3", ">=v1.2.3", " 1.2.3", "1.2.3 ", "1.02.3", ">=1.2.3 ||"} {
		t.Run(expression, func(t *testing.T) {
			_, err := ParseRegistryVersionSelector(expression)
			if err == nil {
				t.Fatalf("accepted malformed selector %q", expression)
			}
		})
	}
}

func TestRegistryVersionSelectorClassification(t *testing.T) {
	for _, tc := range []struct {
		expression string
		kind       SelectorKind
	}{
		{"1.2.3", SelectorVersion},
		{"1.2.3-rc.1", SelectorVersion},
		{"1.2.3+meta", SelectorVersion},
		{"^1.2", SelectorRange},
		{"1.2", SelectorRange},
		{"~1.2", SelectorRange},
		{">=1.2.0 <2", SelectorRange},
	} {
		s, err := ParseRegistryVersionSelector(tc.expression)
		if err != nil || s.Kind != tc.kind {
			t.Fatalf("%q parsed %+v, %v (want %v)", tc.expression, s, err, tc.kind)
		}
	}
}
