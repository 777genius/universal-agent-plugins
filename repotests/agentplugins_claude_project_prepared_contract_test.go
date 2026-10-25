package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
)

func TestClaudePreparedEvidenceCannotCertifyNativeDiscovery(t *testing.T) {
	valid := func() map[string]nativeStage {
		stages := claudeProjectPreparedStages()
		for name, stage := range stages {
			if stage.Reason == "not reached" {
				stage.Status, stage.Reason = "passed", "fixture contract exercised"
				stages[name] = stage
			}
		}
		return stages
	}
	if err := validateClaudePreparedStages(valid()); err != nil {
		t.Fatal(err)
	}
	for name := range valid() {
		t.Run(name, func(t *testing.T) {
			stages := valid()
			stage := stages[name]
			if stage.Status == "passed" {
				stage.Status = "not_evaluated"
			} else {
				stage.Status = "passed"
			}
			stages[name] = stage
			if err := validateClaudePreparedStages(stages); err == nil {
				t.Fatal("changed native/owned-file qualification contract was accepted")
			}
		})
	}
	missing := valid()
	delete(missing, "native_skill_discovery")
	if err := validateClaudePreparedStages(missing); err == nil {
		t.Fatal("missing native discovery boundary accepted")
	}
}

func TestClaudePreparedNativeSuppressionRequiresExactObservedShape(t *testing.T) {
	sentinel := func() map[string]any {
		return map[string]any{"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project", "enabled": false, "installPath": "", "notes": []string{"1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies."}}
	}
	for _, change := range []string{"exact", "absent", "wrong-path", "enabled", "scope", "reason", "duplicate", "loaded"} {
		t.Run(change, func(t *testing.T) {
			row := sentinel()
			rows := make([]map[string]any, 1, 2)
			rows[0] = row
			want := true
			switch change {
			case "absent":
				rows = []map[string]any{}
				want = false
			case "wrong-path":
				row["installPath"] = "/untrusted"
			case "enabled":
				row["enabled"] = true
			case "scope":
				row["scope"] = "user"
			case "reason":
				row["notes"] = []string{"different reason"}
			case "duplicate":
				rows = append(rows, sentinel())
			case "loaded":
				rows = append(rows, map[string]any{"id": claudeCommandsPluginID})
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			err = validateClaudeProjectSuppression(body, want)
			if (err == nil) != (change == "exact" || change == "absent") {
				t.Fatalf("native qualification change=%s err=%v", change, err)
			}
		})
	}
}

func TestClaudePreparedNativeSuppressionRejectsUnknownJSON(t *testing.T) {
	for _, body := range []string{"null", `{}`, `[] {}`, `[{"id":"x","id":"y"}]`, `[false]`} {
		if err := validateClaudeProjectSuppression([]byte(body), false); err == nil {
			t.Fatalf("malformed native list accepted: %s", body)
		}
	}
}

func TestClaudePreparedAssertionSupportsPublicFirstAddAndReplayShapes(t *testing.T) {
	outcome := map[string]any{"activation": map[string]any{"activation": "prepared", "verification": "installation_verified", "user_actions": []any{"native suppression"}}}
	for _, envelope := range []map[string]any{
		{"data": map[string]any{"targets": []any{map[string]any{"output": map[string]any{"result": outcome}}}}},
		{"data": map[string]any{"result": outcome}},
	} {
		before, _ := json.Marshal(envelope)
		requireClaudePreparedResult(t, envelope)
		after, _ := json.Marshal(envelope)
		if string(before) != string(after) {
			t.Fatal("assertion view changed the recorded public envelope")
		}
	}
}
