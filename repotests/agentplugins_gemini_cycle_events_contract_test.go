//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestGeminiCycleCurrentAndPriorComponentProcessIdentitiesCannotCollideConsistently(t *testing.T) {
	c := geminiCycleInert(t)
	geminiCycleStageActual(t, c, "user")
	geminiCycleStageActual(t, c, "a")
	u, a := c.recipes["user"], c.recipes["a"]
	start := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	facts := nativeFacts{Nonce: a.Nonce, Revision: a.Revision, CWD: a.Active, Argv: []string{"argument with spaces", a.Active + "/argument", "${UNKNOWN}"}, Root: a.Active, Data: a.Data, Literal: "literal  " + c.f.base.Home, Once: a.Active + "/", Marker: "retained-" + a.Nonce, PID: 2002, Session: "inert-a-process"}
	allowed := map[string]*geminiCycleRecipe{u.Nonce: u, a.Nonce: a}
	for _, kind := range []string{"valid", "current-pid", "current-session", "current-both", "prior-pid", "prior-session", "user-call"} {
		t.Run(kind, func(t *testing.T) {
			selected := facts
			if kind == "current-pid" || kind == "current-both" {
				selected.PID = 1001
			}
			if kind == "current-session" || kind == "current-both" {
				selected.Session = "inert-user-process"
			}
			q := &geminiCyclePhase{r: a, operation: "write", facts: selected}
			var body []byte
			for index, row := range []struct {
				method, nonce, session string
				pid                    int
				facts                  any
			}{{"initialize", u.Nonce, "inert-user-process", 1001, nil}, {"tools/list", u.Nonce, "inert-user-process", 1001, nil}, {"initialize", a.Nonce, selected.Session, selected.PID, nil}, {"tools/list", a.Nonce, selected.Session, selected.PID, nil}, {"tools/call", a.Nonce, selected.Session, selected.PID, selected}} {
				if kind == "user-call" && index == 1 {
					row.method = "tools/call"
				}
				raw, err := json.Marshal(map[string]any{"method": row.method, "pid": row.pid, "session": row.session, "nonce": row.nonce, "timestamp": start.Add(time.Duration(index+1) * time.Second).Format(time.RFC3339Nano), "facts": row.facts})
				if err != nil {
					t.Fatal(err)
				}
				body = append(append(body, raw...), '\n')
			}
			priorSessions := map[string]bool{}
			priorPIDs := map[int64]bool{}
			if kind == "prior-pid" {
				priorPIDs[2002] = true
			}
			if kind == "prior-session" {
				priorSessions[selected.Session] = true
			}
			sessionsBefore := map[string]bool{}
			for k, v := range priorSessions {
				sessionsBefore[k] = v
			}
			pidsBefore := map[int64]bool{}
			for k, v := range priorPIDs {
				pidsBefore[k] = v
			}
			groups, err := geminiCycleProbeSegment(body, allowed, q, c.f.base.Home, start, end, priorSessions, priorPIDs)
			if kind == "valid" {
				if err != nil || len(groups) != 2 {
					t.Fatal("real selected/U facts-shaped distinct process segment refused", err)
				}
			} else if err == nil || groups != nil {
				t.Fatal("consistent component PID/session collision accepted", kind)
			}
			if !reflect.DeepEqual(sessionsBefore, priorSessions) || !reflect.DeepEqual(pidsBefore, priorPIDs) {
				t.Fatal("refused partial identities mutated prior stage authority")
			}
		})
	}
	if bytes.Contains(c.f.files["cycle-probe-events.jsonl"], []byte("inert-a-process")) {
		t.Fatal("inert event controls fabricated a native capture")
	}
}
