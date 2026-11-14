package platformexec

import "testing"

func TestMergeRemainingOpenCodeConfigFieldsCreatesExtraMap(t *testing.T) {
	t.Parallel()
	config := importedOpenCodeConfig{}
	mergeRemainingOpenCodeConfigFields(&config, map[string]any{"demo": true}, nil)
	if config.Extra == nil {
		t.Fatal("expected extra map")
	}
	if _, ok := config.Extra["command"]; !ok {
		t.Fatalf("extra = %#v", config.Extra)
	}
}
