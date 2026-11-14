package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the real production CLI composition and command routing against a
// real disposable HTTPS Skill source. Detection/scanner fixtures do not certify
// native application loading or a model session.
func TestComposedSavedFreshnessUpdateAllAndLockedReplay(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			source := newProjectSkillsSource(t, "https")
			journey := newProjectJourney(t, source.transport)
			root, intent := "", journey.cache
			if scope == "project" {
				root, intent = journey.a, journey.a
			}
			journey.run(t, root, "skills", "add", source.input, "--skill", "docs", "--target", "gemini", "--no-ui", "--yes")
			before := securityFixtureTree(t, intent)
			current := journey.run(t, root, "outdated", "docs")
			if !strings.Contains(current, `"status":"current"`) {
				t.Fatal(current)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, intent)) {
				t.Fatal("current check wrote desired/store/managed files")
			}
			source.advance()
			available := journey.run(t, root, "outdated", "docs")
			if !strings.Contains(available, `"status":"outdated"`) || !strings.Contains(available, `"changed":true`) {
				t.Fatal(available)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, intent)) {
				t.Fatal("available check wrote desired/store/managed files")
			}
			preview := journey.run(t, root, "update", "--all", "--dry-run")
			if !strings.Contains(preview, `"planned":1`) {
				t.Fatal(preview)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, intent)) {
				t.Fatal("update preview wrote desired/store/managed files")
			}
			updated := journey.run(t, root, "update", "--all")
			if !strings.Contains(updated, `"updated":1`) {
				t.Fatal(updated)
			}
			assertProjectSkill(t, journey, root, projectJourneyAdvancedSkill)
			lock := projectJourneyRead(t, filepath.Join(intent, "uap.lock"))
			stable := securityFixtureTree(t, intent)
			journey.run(t, root, "deps", "sync", "--locked", "--yes")
			journey.run(t, root, "update", "--all")
			if lock != projectJourneyRead(t, filepath.Join(intent, "uap.lock")) || !reflect.DeepEqual(stable, securityFixtureTree(t, intent)) {
				t.Fatal("locked replay or current update changed a stable agreement")
			}
		})
	}
}
