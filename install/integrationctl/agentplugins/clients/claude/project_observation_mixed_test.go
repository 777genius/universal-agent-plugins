package claude

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"testing"
)

func TestProjectObservationPreservesCompleteMixedRowResults(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []string
		want projectListObservation
	}{
		{"user-trust", []string{"user", "suppressed"}, projectListObservation{state: projectListSuppressed, untrusted: true, otherSource: true}},
		{"user-opaque", []string{"user", "opaque"}, projectListObservation{state: projectListSuppressed, otherSource: true}},
		{"foreign-trust", []string{"foreign", "suppressed"}, projectListObservation{state: projectListForeign, untrusted: true}},
		{"foreign-user", []string{"foreign", "user"}, projectListObservation{state: projectListForeign}},
		{"foreign-trust-user", []string{"foreign", "suppressed", "user"}, projectListObservation{state: projectListForeign, untrusted: true}},
		{"foreign-opaque-user", []string{"foreign", "opaque", "user"}, projectListObservation{state: projectListForeign}},
		{"selected-trust", []string{"selected", "suppressed"}, projectListObservation{}},
		{"selected-user", []string{"selected", "user"}, projectListObservation{}},
		{"selected-foreign", []string{"selected", "foreign"}, projectListObservation{}},
		{"duplicate-unrelated", []string{"local", "local"}, projectListObservation{}},
		{"two-user-paths", []string{"user", "second-user"}, projectListObservation{}},
		{"suppressed-malformed-unrelated", []string{"suppressed", "malformed"}, projectListObservation{}},
		{"unrelated-local", []string{"local"}, projectListObservation{state: projectListAbsent}},
		{"unrelated-managed", []string{"managed"}, projectListObservation{state: projectListAbsent}},
		{"selected-unrelated", []string{"selected", "local", "managed"}, projectListObservation{state: projectListEnabled}},
		{"same-name-local", []string{"same-local"}, projectListObservation{}},
		{"same-name-managed", []string{"same-managed"}, projectListObservation{}},
		{"disabled-foreign", []string{"disabled-foreign"}, projectListObservation{state: projectListForeign}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, user, active, fixtures := mixedProjectListFixtures(t)
			// Rotations and reversals cover all orders for the two/three-row cases.
			for start := range len(test.rows) {
				for _, reverse := range []bool{false, true} {
					t.Run(fmt.Sprintf("start-%d-reverse-%t", start, reverse), func(t *testing.T) {
						rows := mixedProjectListOrder(test.rows, fixtures, start, reverse)
						body, err := json.Marshal(rows)
						if err != nil {
							t.Fatal(err)
						}
						if got := observeClaudeProjectList(body, "demo", active, root, user); got != test.want {
							t.Fatalf("observation=%+v,want=%+v", got, test.want)
						}
					})
				}
			}
		})
	}
}

func mixedProjectListFixtures(t *testing.T) (string, string, string, map[string]map[string]any) {
	t.Helper()
	root, user, active, selected, other, suppressed := projectListFixture(t)
	foreign := maps.Clone(selected)
	foreign["installPath"] = active + "-foreign"
	disabledForeign := maps.Clone(foreign)
	disabledForeign["enabled"] = false
	opaque := maps.Clone(suppressed)
	opaque["notes"] = []string{"A future native suppression reason."}
	secondUser := maps.Clone(other)
	secondUser["installPath"] = other["installPath"].(string) + "-second"
	local := maps.Clone(selected)
	local["id"], local["scope"], local["installPath"] = "unrelated", "local", filepath.Join(root, "unrelated")
	managed := maps.Clone(local)
	managed["scope"] = "managed"
	malformed := maps.Clone(local)
	malformed["notes"] = "unrecognized"
	sameLocal, sameManaged := maps.Clone(local), maps.Clone(managed)
	sameLocal["id"], sameManaged["id"] = "demo@skills-dir", "demo@skills-dir"
	return root, user, active, map[string]map[string]any{
		"selected": selected, "user": other, "suppressed": suppressed, "opaque": opaque,
		"foreign": foreign, "disabled-foreign": disabledForeign, "second-user": secondUser,
		"local": local, "managed": managed, "malformed": malformed, "same-local": sameLocal, "same-managed": sameManaged,
	}
}

func mixedProjectListOrder(names []string, fixtures map[string]map[string]any, start int, reverse bool) []map[string]any {
	rows := make([]map[string]any, 0, len(names))
	for step := range names {
		index := (start + step) % len(names)
		if reverse {
			index = (start + len(names) - step) % len(names)
		}
		rows = append(rows, fixtures[names[index]])
	}
	return rows
}
