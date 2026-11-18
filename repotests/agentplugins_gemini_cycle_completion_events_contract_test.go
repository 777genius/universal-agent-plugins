//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"reflect"
	"testing"
	"time"
)

func TestGeminiPublicCompletionRequiresBothExactFreshInventoryPhases(t *testing.T) {
	start := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	allowed := map[string]*geminiCycleRecipe{"user": {Nonce: "user"}, "project": {Nonce: "project"}}
	for _, mutation := range []string{"valid", "missing", "extra", "reordered", "foreign", "call", "facts", "same-pid", "same-session", "prior-pid", "prior-session", "late", "comma-clock", "overprecision-clock"} {
		t.Run(mutation, func(t *testing.T) {
			rows := []map[string]any{}
			for i, item := range []struct {
				method, nonce, session string
				pid                    int
			}{
				{"initialize", "user", "skills-user", 1}, {"initialize", "project", "skills-project", 2},
				{"tools/list", "user", "skills-user", 1}, {"tools/list", "project", "skills-project", 2},
				{"initialize", "user", "mcp-user", 3}, {"initialize", "project", "mcp-project", 4},
			} {
				rows = append(rows, map[string]any{"method": item.method, "nonce": item.nonce, "session": item.session, "pid": item.pid, "timestamp": start.Add(time.Duration(i+1) * time.Second).Format(time.RFC3339Nano), "facts": nil})
			}
			sessions, pids := map[string]bool{}, map[int64]bool{}
			switch mutation {
			case "missing":
				rows = rows[:5]
			case "extra":
				rows = append(rows, rows[5])
			case "reordered":
				rows[0]["method"], rows[2]["method"] = "tools/list", "initialize"
			case "foreign":
				rows[5]["nonce"] = "foreign"
			case "call":
				rows[5]["method"] = "tools/call"
			case "facts":
				rows[5]["facts"] = map[string]any{"unearned": true}
			case "same-pid":
				rows[4]["pid"] = 1
			case "same-session":
				rows[4]["session"] = "skills-user"
			case "prior-pid":
				pids[1] = true
			case "prior-session":
				sessions["skills-user"] = true
			case "late":
				rows[5]["timestamp"] = start.Add(time.Hour).Format(time.RFC3339Nano)
			case "comma-clock":
				for _, row := range rows {
					row["timestamp"] = "2026-11-17T00:00:01,123Z"
				}
			case "overprecision-clock":
				for _, row := range rows {
					row["timestamp"] = "2026-11-17T00:00:01.0000000001Z"
				}
			}
			var segment bytes.Buffer
			for _, row := range rows {
				if err := json.NewEncoder(&segment).Encode(row); err != nil {
					t.Fatal(err)
				}
			}
			beforeSessions, beforePIDs := len(sessions), len(pids)
			gotSessions, gotPIDs, err := geminiCyclePublicInventory(segment.Bytes(), "add-a", allowed, "", start, start.Add(time.Minute), sessions, pids)
			if mutation == "valid" {
				if err != nil || len(gotSessions) != 4 || len(gotPIDs) != 4 {
					t.Fatal("exact two-phase inventory refused", err)
				}
			} else if err == nil {
				t.Fatal("nonexact completion accepted", mutation)
			}
			if !reflect.DeepEqual([]int{len(sessions), len(pids)}, []int{beforeSessions, beforePIDs}) {
				t.Fatal("validation mutated prior authority")
			}
		})
	}
}

func TestGeminiPublicCompletionPhasesMatchOperations(t *testing.T) {
	for _, test := range []struct {
		labels []string
		want   []int
	}{
		{[]string{"add-a", "add-b", "update-a", "intact-repair-a", "missing-repair-a"}, []int{2, 1}},
		{[]string{"repeat-add-a", "unchanged-update-a"}, []int{2, 2, 2, 1}},
		{[]string{"git-a", "git-b", "git-user", "version", "add-user", "remove-a", "repeat-remove-a", "remove-b", "remove-user"}, nil},
	} {
		for _, label := range test.labels {
			t.Run(label, func(t *testing.T) {
				if got := geminiCyclePublicInventoryPhases(label); !reflect.DeepEqual(got, test.want) {
					t.Fatalf("public inventory phases = %v, want %v", got, test.want)
				}
			})
		}
	}
}

func TestGeminiRepeatedPublicCompletionRequiresFourExactFreshInventoryPhases(t *testing.T) {
	start := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	allowed := map[string]*geminiCycleRecipe{"user": {Nonce: "user"}, "project": {Nonce: "project"}}
	for _, label := range []string{"repeat-add-a", "unchanged-update-a"} {
		t.Run(label, func(t *testing.T) {
			for _, mutation := range []string{"valid", "missing-phase", "missing", "extra", "reordered", "foreign", "call", "facts", "same-pid", "same-session", "prior-pid", "prior-session", "phase-clock", "late"} {
				t.Run(mutation, func(t *testing.T) {
					rows := []map[string]any{}
					for i, item := range []struct {
						method, nonce, session string
						pid                    int
					}{
						{"initialize", "user", "phase-1-user", 1}, {"initialize", "project", "phase-1-project", 2},
						{"tools/list", "user", "phase-1-user", 1}, {"tools/list", "project", "phase-1-project", 2},
						{"initialize", "user", "phase-2-user", 3}, {"initialize", "project", "phase-2-project", 4},
						{"tools/list", "user", "phase-2-user", 3}, {"tools/list", "project", "phase-2-project", 4},
						{"initialize", "user", "phase-3-user", 5}, {"initialize", "project", "phase-3-project", 6},
						{"tools/list", "user", "phase-3-user", 5}, {"tools/list", "project", "phase-3-project", 6},
						{"initialize", "user", "phase-4-user", 7}, {"initialize", "project", "phase-4-project", 8},
					} {
						rows = append(rows, map[string]any{"method": item.method, "nonce": item.nonce, "session": item.session, "pid": item.pid, "timestamp": start.Add(time.Duration(i+1) * time.Second).Format(time.RFC3339Nano), "facts": nil})
					}
					sessions, pids := map[string]bool{}, map[int64]bool{}
					switch mutation {
					case "missing-phase":
						rows = append(rows[:4], rows[8:]...)
					case "missing":
						rows = rows[:13]
					case "extra":
						rows = append(rows, rows[13])
					case "reordered":
						rows[8]["method"], rows[10]["method"] = "tools/list", "initialize"
					case "foreign":
						rows[8]["nonce"], rows[10]["nonce"] = "foreign", "foreign"
					case "call":
						rows[12]["method"] = "tools/call"
					case "facts":
						rows[12]["facts"] = map[string]any{"unearned": true}
					case "same-pid":
						rows[8]["pid"], rows[10]["pid"] = 1, 1
					case "same-session":
						rows[8]["session"], rows[10]["session"] = "phase-1-user", "phase-1-user"
					case "prior-pid":
						pids[1] = true
					case "prior-session":
						sessions["phase-1-user"] = true
					case "phase-clock":
						rows[8]["timestamp"] = start.Add(2 * time.Second).Format(time.RFC3339Nano)
					case "late":
						rows[13]["timestamp"] = start.Add(time.Hour).Format(time.RFC3339Nano)
					}
					var segment bytes.Buffer
					for _, row := range rows {
						if err := json.NewEncoder(&segment).Encode(row); err != nil {
							t.Fatal(err)
						}
					}
					beforeSessions, beforePIDs := maps.Clone(sessions), maps.Clone(pids)
					gotSessions, gotPIDs, err := geminiCyclePublicInventory(segment.Bytes(), label, allowed, "", start, start.Add(time.Minute), sessions, pids)
					if mutation == "valid" {
						if len(rows) != 14 || err != nil || len(gotSessions) != 8 || len(gotPIDs) != 8 {
							t.Fatal("exact four-phase inventory refused", err)
						}
					} else if err == nil {
						t.Fatal("nonexact repeated completion accepted", mutation)
					}
					if mutation == "missing-phase" && len(rows) != 10 {
						t.Fatal("missing-phase control does not contain ten events")
					}
					if !reflect.DeepEqual(sessions, beforeSessions) || !reflect.DeepEqual(pids, beforePIDs) {
						t.Fatal("validation mutated prior authority")
					}
				})
			}
		})
	}
}
