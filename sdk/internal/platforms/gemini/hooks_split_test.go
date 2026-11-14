package gemini

import "testing"

func TestNormalizeFunctionNamesDeduplicatesAndTrims(t *testing.T) {
	got := normalizeFunctionNames([]string{" read_file ", "read_file", "", "list_directory"})
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0] != "read_file" || got[1] != "list_directory" {
		t.Fatalf("names = %#v", got)
	}
}

func TestValidateDecisionRejectsUnknownValue(t *testing.T) {
	err := validateDecision("pause")
	if err == nil || err.Error() != `unknown decision "pause"` {
		t.Fatalf("err = %v", err)
	}
}
