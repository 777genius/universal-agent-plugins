//go:build windows

package bootstrap_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
)

func TestWindowsApplyFailsClosedBeforeStagingOrProcess(t *testing.T) {
	root, project := generated(t)
	plan, err := (bootstrap.Service{}).Plan(root, project)
	if err != nil {
		t.Fatal(err)
	}
	proveUnsupportedApply(t, root, plan)
}
