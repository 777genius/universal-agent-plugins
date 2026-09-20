package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateGuardsRejectUnreadableState(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(t.TempDir(), "state")})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(eng.cfg.StateFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.cfg.StateFile, []byte("invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := testCtx(t)
	prepared := &PreparedOperation{plan: Plan{Operation: OpUpdate, InstallationID: "test"}}
	checks := map[string]func() error{
		"confirm single": func() error { return eng.confirmPreparedPlan(ctx, prepared) },
		"confirm group":  func() error { return eng.confirmGroupPlan(ctx, prepared) },
		"install digest": func() error { return eng.refuseRecordedDigestRewrite("test", "digest") },
		"repair digest":  func() error { return eng.refuseRepairRevisionRewrite("test", "codex", "digest") },
		"group handoff": func() error {
			_, err := eng.reconcileGroupHostHandoff(ctx, prepared, &Result{})
			return err
		},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil || !strings.Contains(err.Error(), "read installation state") {
				t.Fatalf("corrupt state must prevent guarded operation: %v", err)
			}
		})
	}
}
