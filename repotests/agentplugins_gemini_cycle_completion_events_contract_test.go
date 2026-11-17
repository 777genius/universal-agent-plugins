//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
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
