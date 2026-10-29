//go:build linux

package pluginkitairepo_test

import (
	"testing"
	"time"
)

func copilotCycleProbeInert(methods []string) (*copilotProjectCycle, *copilotCycleRecipe, nativeFacts, []map[string]any, time.Time) {
	c := &copilotProjectCycle{probeIDs: map[string]bool{}, nativeIDs: map[string]bool{}}
	r := &copilotCycleRecipe{Nonce: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	facts := nativeFacts{Nonce: r.Nonce, Revision: "a-one", CWD: "/fixture/plugin", Root: "/fixture/plugin", Data: "/fixture/data",
		Argv: []string{"argument with spaces", "/fixture/plugin/argument", "${UNKNOWN}"}, Literal: "literal ${UNKNOWN} /fixture/home",
		Once: "/fixture/plugin/${UNKNOWN}", Marker: "retained-" + r.Nonce, PID: 123, Session: "123-456"}
	start := time.Date(2026, 10, 29, 13, 51, 30, 0, time.UTC)
	rows := make([]map[string]any, len(methods))
	for i, method := range methods {
		var returned any
		if i == len(methods)-1 {
			returned = facts
		}
		rows[i] = map[string]any{"method": method, "pid": facts.PID, "session": facts.Session, "nonce": r.Nonce,
			"timestamp": start.Add(time.Duration(i+1) * time.Millisecond).Format(time.RFC3339Nano), "facts": returned}
	}
	return c, r, facts, rows, start
}

func TestCopilotCycleProbeAcceptsOnlyMeasuredMethodCounts(t *testing.T) {
	for _, methods := range [][]string{
		{"initialize", "tools/list", "tools/call"},
		{"initialize", "tools/list", "tools/list", "tools/list", "tools/call"},
		{"initialize", "tools/list", "tools/list", "tools/call"},
		{"initialize", "tools/list", "tools/list", "tools/list", "tools/list", "tools/call"},
	} {
		c, r, facts, rows, start := copilotCycleProbeInert(methods)
		err := c.probe(copilotReadinessEncode(t, rows), r, facts, start, start.Add(time.Second), "native", "provider")
		if (err == nil) != (len(methods) == 3 || len(methods) == 5) {
			t.Fatal("unmeasured probe graph count accepted or measured graph refused", methods, err)
		}
	}
}

func TestCopilotCycleProbeExtraListsDoNotWeakenIdentityOrderOrFacts(t *testing.T) {
	for _, fault := range []string{"initialize", "call", "order", "unknown", "pid", "session", "nonce", "future", "backwards", "startup-facts", "final-facts", "field", "newline"} {
		c, r, facts, rows, start := copilotCycleProbeInert([]string{"initialize", "tools/list", "tools/list", "tools/list", "tools/call"})
		switch fault {
		case "initialize":
			rows[3]["method"] = "initialize"
		case "call":
			rows[3]["method"] = "tools/call"
		case "order":
			rows[0], rows[1] = rows[1], rows[0]
		case "unknown":
			rows[3]["method"] = "ping"
		case "pid":
			rows[3]["pid"] = facts.PID + 1
		case "session":
			rows[3]["session"] = "123-789"
		case "nonce":
			rows[3]["nonce"] = "foreign"
		case "future":
			rows[3]["timestamp"] = start.Add(2 * time.Second).Format(time.RFC3339Nano)
		case "backwards":
			rows[3]["timestamp"] = start.Format(time.RFC3339Nano)
		case "startup-facts":
			rows[3]["facts"] = facts
		case "final-facts":
			rows[4]["facts"] = nil
		case "field":
			rows[3]["foreign"] = true
		}
		body := copilotReadinessEncode(t, rows)
		if fault == "newline" {
			body = body[:len(body)-1]
		}
		if c.probe(body, r, facts, start, start.Add(time.Second), "native", "provider") == nil {
			t.Fatal("extra lists weakened exact probe guards", fault)
		}
	}
}

func TestCopilotCycleProbeFreshnessAndNoCallRemainClosed(t *testing.T) {
	c, r, facts, rows, start := copilotCycleProbeInert([]string{"initialize", "tools/list", "tools/list", "tools/list", "tools/call"})
	body := copilotReadinessEncode(t, rows)
	end := start.Add(time.Second)
	if c.probe(nil, nil, nativeFacts{}, start, end, "native", "provider") != nil || c.probe(body, nil, facts, start, end, "native", "provider") == nil {
		t.Fatal("no-call phase gained MCP activity")
	}
	for _, ids := range []map[string]bool{c.nativeIDs, c.probeIDs} {
		ids[facts.Session] = true
		if c.probe(body, r, facts, start, end, "native", "provider") == nil {
			t.Fatal("stale or reused probe identity accepted")
		}
		delete(ids, facts.Session)
	}
	if c.probe(body, r, facts, start, end, facts.Session, "provider") == nil || c.probe(body, r, facts, start, end, "native", facts.Session) == nil {
		t.Fatal("probe borrowed native or provider identity")
	}
	if c.probe(body, r, facts, start, end, "native", "provider") != nil || c.probe(body, r, facts, start, end, "native", "provider") == nil {
		t.Fatal("successful probe was not recorded as a unique session")
	}
}
