package platformexec

import "testing"

func TestMarshalOpenCodePluginRefYAMLUsesScalarWhenOptionsEmpty(t *testing.T) {
	t.Parallel()

	got := marshalOpenCodePluginRefYAML(opencodePluginRef{Name: " @acme/demo "})
	if got != "@acme/demo" {
		t.Fatalf("yaml value = %#v", got)
	}
}

func TestValidateOpenCodePluginRefsRejectsEmptyOptionKey(t *testing.T) {
	t.Parallel()

	err := validateOpenCodePluginRefs([]opencodePluginRef{{
		Name:    "@acme/demo",
		Options: map[string]any{"": true},
	}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestJSONValuesForOpenCodePluginsPreservesTupleShape(t *testing.T) {
	t.Parallel()

	values := jsonValuesForOpenCodePlugins([]opencodePluginRef{{
		Name:    "@acme/demo",
		Options: map[string]any{"enabled": true},
	}})
	tuple, ok := values[0].([]any)
	if !ok || len(tuple) != 2 {
		t.Fatalf("values = %#v", values)
	}
}
