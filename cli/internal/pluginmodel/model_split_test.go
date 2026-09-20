package pluginmodel

import (
	"reflect"
	"strings"
	"testing"
)

func TestManifestSelectedTargetsNormalizesInput(t *testing.T) {
	manifest := Manifest{Targets: []string{"Claude", " codex-runtime "}}

	all, err := manifest.SelectedTargets(" all ")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude", "codex-runtime"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("SelectedTargets(all) = %#v", all)
	}

	selected, err := manifest.SelectedTargets(" CODEX-RUNTIME ")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"codex-runtime"}; !reflect.DeepEqual(selected, want) {
		t.Fatalf("SelectedTargets(codex-runtime) = %#v", selected)
	}
}

func TestManifestSelectedTargetsRejectsDisabledTarget(t *testing.T) {
	manifest := Manifest{Targets: []string{"claude"}}
	_, err := manifest.SelectedTargets("gemini")
	if err == nil {
		t.Fatal("expected disabled target error")
	}
	if !strings.Contains(err.Error(), `target "gemini" is not enabled in plugin.yaml`) {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeNativeExtraObjectRejectsManagedConflicts(t *testing.T) {
	base := map[string]any{"outer": map[string]any{"other": "keep"}}
	doc := NativeExtraDoc{
		Fields: map[string]any{
			"outer": map[string]any{
				"managed": "blocked",
			},
		},
	}

	err := MergeNativeExtraObject(base, doc, "config.extra", []string{"outer.managed"})
	if err == nil {
		t.Fatal("expected managed conflict")
	}
	if !strings.Contains(err.Error(), `config.extra may not override canonical field "outer.managed"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeNativeExtraObjectMergesNestedFields(t *testing.T) {
	base := map[string]any{
		"outer": map[string]any{
			"existing": "keep",
		},
	}
	doc := NativeExtraDoc{
		Fields: map[string]any{
			"outer": map[string]any{
				"added": "value",
			},
		},
	}

	if err := MergeNativeExtraObject(base, doc, "config.extra", nil); err != nil {
		t.Fatal(err)
	}
	got, ok := base["outer"].(map[string]any)
	if !ok {
		t.Fatalf("outer type = %T", base["outer"])
	}
	if got["existing"] != "keep" || got["added"] != "value" {
		t.Fatalf("outer = %#v", got)
	}
}
