//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func geminiCycleFacts(body string, r *geminiCycleRecipe, home string) (nativeFacts, error) {
	var facts nativeFacts
	value, err := projectNativeRuntimeJSON([]byte(body))
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return facts, fmt.Errorf("Gemini cycle exact probe facts")
	}
	d := json.NewDecoder(strings.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(&facts); err != nil {
		return facts, err
	}
	if facts.Nonce != r.Nonce || facts.Revision != r.Revision || facts.CWD != r.Active || facts.Root != r.Active || facts.Data != r.Data || !reflect.DeepEqual(facts.Argv, []string{"argument with spaces", r.Active + "/argument", "${UNKNOWN}"}) || facts.Literal != "literal  "+home || facts.Once != r.Active+"/" || facts.Marker != "retained-"+r.Nonce || facts.PID <= 0 || facts.Session == "" || len(facts.Session) > 256 {
		return facts, fmt.Errorf("Gemini cycle held runtime facts differ")
	}
	return facts, nil
}
func (c *geminiProjectCycle) eventLog() ([]byte, error) {
	body, err := projectNativeRuntimeRead(c.f.base.Events, 1<<20)
	if err != nil || len(body) > 1<<20 {
		return nil, fmt.Errorf("Gemini cycle complete event log framing")
	}
	return body, nil
}
func (c *geminiProjectCycle) advertised(scope string) []*geminiCycleRecipe {
	selected := []*geminiCycleRecipe{}
	for _, key := range []string{"user", scope} {
		r := c.recipes[key]
		if r.Binding.ClientBindingID != "" && !r.Removed && (len(selected) == 0 || selected[len(selected)-1] != r) {
			selected = append(selected, r)
		}
	}
	return selected
}

type geminiCycleProbeGroup struct {
	pid     json.Number
	session string
	methods []string
}

func (c *geminiProjectCycle) events(t *testing.T, scope string, before []byte, start, end time.Time, q *geminiCyclePhase) {
	t.Helper()
	after, err := c.eventLog()
	if err != nil || !bytes.HasPrefix(after, before) {
		t.Fatal("Gemini append-only probe events changed")
	}
	allowed := map[string]*geminiCycleRecipe{}
	for _, r := range c.advertised(scope) {
		allowed[r.Nonce] = r
	}
	groups, err := geminiCycleProbeSegment(after[len(before):], allowed, q, c.f.base.Home, start, end, c.probeSessions, c.probePIDs)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		pid, _ := g.pid.Int64()
		c.probeSessions[g.session] = true
		c.probePIDs[pid] = true
	}
	c.eventsAccepted = bytes.Clone(after)
	c.openEvents = false
}
func geminiCycleProbeSegment(segment []byte, allowed map[string]*geminiCycleRecipe, q *geminiCyclePhase, home string, start, end time.Time, priorSessions map[string]bool, priorPIDs map[int64]bool) (map[string]*geminiCycleProbeGroup, error) {

	if len(segment) > 0 && segment[len(segment)-1] != '\n' {
		return nil, fmt.Errorf("Gemini complete probe event framing")
	}
	rows := [][]byte{}
	if len(segment) > 0 {
		rows = bytes.Split(segment[:len(segment)-1], []byte{'\n'})
	}
	if len(rows) > 6 {
		return nil, fmt.Errorf("Gemini exact advertised component event bound")
	}
	groups := map[string]*geminiCycleProbeGroup{}
	last := map[string]time.Time{}
	currentSessions := map[string]bool{}
	currentPIDs := map[int64]bool{}
	for _, line := range rows {
		value, err := projectNativeRuntimeJSON(line)
		row, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(row, "method pid session nonce timestamp facts", "") != nil {
			return nil, fmt.Errorf("Gemini closed component event fields")
		}
		nonce, ok := row["nonce"].(string)
		r := allowed[nonce]
		if !ok || r == nil {
			return nil, fmt.Errorf("Gemini removed/unselected component started")
		}
		pid, pok := row["pid"].(json.Number)
		number, e := pid.Int64()
		session, sok := row["session"].(string)
		stamp, tok := row["timestamp"].(string)
		when, te := time.Parse(time.RFC3339Nano, stamp)
		if !pok || e != nil || number <= 0 || !sok || session == "" || len(session) > 256 || !tok || te != nil || when.Before(last[nonce]) || when.Before(start) || when.After(end) {
			return nil, fmt.Errorf("Gemini component event time/process/session")
		}
		last[nonce] = when
		g := groups[nonce]
		if g == nil {
			if priorSessions[session] || priorPIDs[number] || currentSessions[session] || currentPIDs[number] {
				return nil, fmt.Errorf("Gemini probe PID/session collision across fresh component processes")
			}
			currentSessions[session] = true
			currentPIDs[number] = true
			g = &geminiCycleProbeGroup{pid: pid, session: session}
			groups[nonce] = g
		}
		if g.pid != pid || g.session != session {
			return nil, fmt.Errorf("Gemini component restarted inside a single native stage")
		}
		method, ok := row["method"].(string)
		if !ok {
			return nil, fmt.Errorf("Gemini component method type")
		}
		want := []string{"initialize", "tools/list"}
		selected := q != nil && q.operation != "none" && q.r == r
		if selected {
			want = append(want, "tools/call")
		}
		index := len(g.methods)
		if index >= len(want) || method != want[index] {
			return nil, fmt.Errorf("Gemini component selected exact method order")
		}
		g.methods = append(g.methods, method)
		if method == "tools/call" {
			body, _ := json.Marshal(row["facts"])
			facts, e := geminiCycleFacts(string(body), r, home)
			if e != nil || !reflect.DeepEqual(facts, q.facts) || facts.PID != int(number) || facts.Session != session {
				return nil, fmt.Errorf("Gemini provider/native/probe facts identity join")
			}
		} else if row["facts"] != nil {
			return nil, fmt.Errorf("Gemini unexpected noncall facts")
		}
	}
	for nonce, r := range allowed {
		g := groups[nonce]
		if g == nil {
			if q != nil {
				return nil, fmt.Errorf("Gemini advertised declaration lacks actual initialization/listing")
			}
			continue
		}
		want := 2
		if q != nil && q.operation != "none" && q.r == r {
			want = 3
		}
		if len(g.methods) != want {
			return nil, fmt.Errorf("Gemini incomplete component process exchange")
		}
	}

	return groups, nil
}
func geminiCycleMarkerDelta(before, after map[string]geminiCycleRecord, prefix string, r *geminiCycleRecipe, write bool) error {
	for name := range geminiCycleKeys(before, after) {
		a, had := before[name]
		b, has := after[name]
		if had && has && a == b {
			continue
		}
		if write && name == prefix {
			if !had || !has || !geminiCycleSameDirectory(a, b) {
				return fmt.Errorf("Gemini selected data root replaced")
			}
			continue
		}
		if write && name == filepath.Join(prefix, "native-marker.txt") {
			if had || !has || b.Mode != 0100000|0600 || b.Size != int64(len("retained-"+r.Nonce)) || b.SHA256 != geminiCycleSHA([]byte("retained-"+r.Nonce)) {
				return fmt.Errorf("Gemini exact selected initial marker creation")
			}
			continue
		}
		return fmt.Errorf("Gemini native changed unselected data/state/payload/cwd authority")
	}
	if write {
		marker, ok := after[filepath.Join(prefix, "native-marker.txt")]
		if !ok || marker.SHA256 != geminiCycleSHA([]byte("retained-"+r.Nonce)) {
			return fmt.Errorf("Gemini selected marker absent")
		}
	}
	return nil
}
