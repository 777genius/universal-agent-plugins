package vscodelocal_test

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/vscodelocal"
)

// Red: a future change adds a public response control/context field, even if
// the runtime encoder still discards it. This checks the exported API itself;
// process tests separately verify the actual nil/empty wire response.
func TestResponsesHaveNoControlSurface(t *testing.T) {
	for _, response := range []reflect.Type{
		reflect.TypeOf(vscodelocal.StopResponse{}),
		reflect.TypeOf(vscodelocal.SubagentStopResponse{}),
	} {
		if response.NumField() != 0 {
			t.Fatalf("observer response %s exposes fields", response.Name())
		}
	}
}
