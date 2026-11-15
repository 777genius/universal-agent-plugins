//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func kiroCycleEventsFile(path string) ([]byte, os.FileInfo, error) {
	snapshot, err := kiroCycleCapture(path)
	if os.IsNotExist(err) {
		return nil, nil, kiroCycleAbsent(path)
	}
	if err != nil || snapshot.info.Mode().Perm() != 0600 {
		return nil, nil, fmt.Errorf("Kiro original probe event file refused")
	}
	return snapshot.body, snapshot.info, nil
}

func (c *kiroNativeCycle) eventPrefix(t *testing.T) os.FileInfo {
	t.Helper()
	body, info, err := kiroCycleEventsFile(c.f.base.Events)
	if err != nil || !bytes.Equal(body, c.events) || (info == nil) != (c.eventInfo == nil) || info != nil && (!os.SameFile(c.eventInfo, info) || !projectNativeRuntimeStableStat(c.eventInfo.Sys(), info.Sys())) {
		t.Fatal("Kiro probe history changed outside the selected native command")
	}
	return info
}

func (c *kiroNativeCycle) native(t *testing.T, label, key, operation string) {
	t.Helper()
	r := c.roles[key]
	if r.removed != (operation == "") {
		t.Fatal("Kiro native stage does not match exact installed/removed role")
	}
	if operation == "write" {
		if err := kiroCycleAbsent(r.markerPath()); err != nil {
			t.Fatal(err)
		}
	}
	before := c.eventPrefix(t)
	c.run(t, label)
	body, info, err := kiroCycleEventsFile(c.f.base.Events)
	if err != nil || before != nil && !os.SameFile(before, info) || !bytes.HasPrefix(body, c.events) || operation == "" && before != nil && !projectNativeRuntimeStableStat(before.Sys(), info.Sys()) {
		t.Fatal("Kiro native event history replaced or changed")
	}
	meta, err := projectNativeRuntimeJSON(c.f.files[label+".command.json"])
	if err != nil {
		t.Fatal(err)
	}
	row := meta.(map[string]any)
	start, errStart := time.Parse(time.RFC3339Nano, row["started_at"].(string))
	end, errEnd := time.Parse(time.RFC3339Nano, row["ended_at"].(string))
	if errStart != nil || errEnd != nil {
		t.Fatal("Kiro native command time receipt")
	}
	facts, err := kiroCycleEvents(body[len(c.events):], c.f.base.Nonce, r, start, end, operation != "")
	if err != nil || operation != "" && c.sessions[facts.Session] {
		t.Fatal("Kiro native MCP event/call proof refused", err)
	}
	if operation != "" {
		c.sessions[facts.Session] = true
		snapshot, captureErr := kiroCycleCapture(r.markerPath())
		if captureErr != nil || snapshot.info.Mode().Perm() != 0600 || string(snapshot.body) != r.marker {
			t.Fatal("Kiro native DATA effect is not the exact observed marker")
		}
		if operation == "write" {
			c.pins[r.markerPath()] = snapshot
		}
	}
	c.events, c.eventInfo = bytes.Clone(body), info
	c.check(t)
}

func kiroCycleEvents(raw []byte, nonce string, role *kiroCycleRole, start, end time.Time, positive bool) (nativeFacts, error) {
	var facts nativeFacts
	if !positive {
		if len(raw) != 0 {
			return facts, fmt.Errorf("removed Kiro registration still started or called the probe")
		}
		return facts, nil
	}
	if len(raw) == 0 || len(raw) > 1<<20 || raw[len(raw)-1] != '\n' || end.Before(start) {
		return facts, fmt.Errorf("Kiro bounded full event stream required")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) < 3 || len(lines) > 6 {
		return facts, fmt.Errorf("Kiro one initialize, one to four list, one call required")
	}
	session, pid, last := "", 0, start
	for index, line := range lines {
		value, err := projectNativeRuntimeJSON(line)
		row, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(row, "method pid session nonce timestamp facts", "") != nil {
			return facts, fmt.Errorf("Kiro closed native probe event shape")
		}
		number, ok := row["pid"].(json.Number)
		currentPID, numberErr := strconv.Atoi(string(number))
		currentSession, sessionOK := row["session"].(string)
		timestamp, timeOK := row["timestamp"].(string)
		when, timeErr := time.Parse(time.RFC3339Nano, timestamp)
		if !ok || numberErr != nil || currentPID <= 0 || currentPID > 1<<31-1 || !sessionOK || !strings.HasPrefix(currentSession, strconv.Itoa(currentPID)+"-") || !timeOK || timeErr != nil || when.Before(last) || when.After(end) || row["nonce"] != nonce {
			return facts, fmt.Errorf("Kiro original command/session/time/nonce differs")
		}
		birth, birthErr := strconv.ParseInt(strings.TrimPrefix(currentSession, strconv.Itoa(currentPID)+"-"), 10, 64)
		if birthErr != nil || birth < start.UnixNano() || birth > when.UnixNano() {
			return facts, fmt.Errorf("Kiro probe session creation is outside the original command")
		}
		if index == 0 {
			session, pid = currentSession, currentPID
		}
		if currentSession != session || currentPID != pid {
			return facts, fmt.Errorf("Kiro multiple native probe sessions in one command")
		}
		last = when
		expected := "tools/list"
		if index == 0 {
			expected = "initialize"
		} else if index == len(lines)-1 {
			expected = "tools/call"
		}
		if row["method"] != expected || expected != "tools/call" && row["facts"] != nil {
			return facts, fmt.Errorf("Kiro native MCP event order differs")
		}
		if expected == "tools/call" {
			facts, err = kiroCycleFacts(row["facts"], nonce, role, pid, session)
			if err != nil {
				return facts, err
			}
		}
	}
	return facts, nil
}

func kiroCycleFacts(value any, nonce string, r *kiroCycleRole, pid int, session string) (nativeFacts, error) {
	var facts nativeFacts
	row, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return facts, fmt.Errorf("Kiro exact tool facts schema differs")
	}
	body, err := json.Marshal(row)
	if err != nil || json.Unmarshal(body, &facts) != nil {
		return facts, fmt.Errorf("Kiro tool facts decode failed")
	}
	active := r.binding.TargetLocator
	if facts.Nonce != nonce || facts.Revision != "a-one" || facts.CWD != active || facts.Root != active || facts.Data != r.data || facts.Literal != "" || facts.Once != "" || facts.Marker != r.marker || facts.PID != pid || facts.Session != session || !reflect.DeepEqual(facts.Argv, []string{"Owned kiro", active + "/argument"}) {
		return facts, fmt.Errorf("Kiro native selected package/DATA/argv/marker differs")
	}
	return facts, nil
}

func TestKiroCycleExactToolProfileAndCommandScope(t *testing.T) {
	agent := kiroCycleAgent("uap-kiro-native-a", "uap-kiro-attempt")
	selector := []string{"@uap-kiro-attempt/inspect_runtime"}
	if !reflect.DeepEqual(agent["tools"], selector) || !reflect.DeepEqual(agent["allowedTools"], selector) || len(agent["mcpServers"].(map[string]any)) != 0 || len(agent["resources"].([]string)) != 0 || len(agent["hooks"].(map[string]any)) != 0 || len(agent["toolAliases"].(map[string]string)) != 0 {
		t.Fatal("Kiro agent grants more than one disposable probe tool")
	}
	c := &kiroNativeCycle{f: &projectNativeCLIAttempt{client: "/native/kiro-cli", installer: "/uap", outside: "/outside", name: "uap-kiro-attempt", base: &nativeFixture{Nonce: "nonce", Package: "/source"}}, roles: map[string]*kiroCycleRole{"a": {key: "a", project: "/a", marker: "marker"}, "b": {key: "b", project: "/b", marker: "other"}}}
	for _, label := range kiroCycleOrder {
		cwd, argv, authenticated, err := c.args(label)
		if err != nil || cwd == "" || len(argv) == 0 {
			t.Fatal("registered Kiro stage is not constructible", label)
		}
		isNative := strings.HasPrefix(label, "native-") || strings.HasPrefix(label, "validate-")
		if authenticated != isNative {
			t.Fatal("key crossed the fixed authenticated command boundary")
		}
		for _, argument := range argv {
			if argument == "--trust-all-tools" {
				t.Fatal("broad tool trust is forbidden")
			}
		}
	}
	if _, _, _, err := c.args("other"); err == nil {
		t.Fatal("unknown Kiro native command admitted")
	}
}

func kiroCycleTestEvents(t *testing.T) (*kiroCycleRole, []byte, time.Time, time.Time) {
	t.Helper()
	r := &kiroCycleRole{data: "/a/data", marker: "marker"}
	r.binding.TargetLocator = "/a/package"
	start := time.Unix(100, 0).UTC()
	end := start.Add(time.Second)
	facts := nativeFacts{Nonce: "nonce", Revision: "a-one", CWD: "/a/package", Root: "/a/package", Data: "/a/data", Argv: []string{"Owned kiro", "/a/package/argument"}, Marker: "marker", PID: 31, Session: "31-100000000001"}
	var raw []byte
	for index, method := range []string{"initialize", "tools/list", "tools/call"} {
		var value any
		if method == "tools/call" {
			value = facts
		}
		line, err := json.Marshal(map[string]any{"method": method, "pid": 31, "session": facts.Session, "nonce": "nonce", "timestamp": start.Add(time.Duration(index+1) * time.Millisecond).Format(time.RFC3339Nano), "facts": value})
		if err != nil {
			t.Fatal(err)
		}
		raw = append(append(raw, line...), '\n')
	}
	return r, raw, start, end
}

func TestKiroCycleActualEventOrderScopeAndNegativeControl(t *testing.T) {
	r, raw, start, end := kiroCycleTestEvents(t)
	if _, err := kiroCycleEvents(raw, "nonce", r, start, end, true); err != nil {
		t.Fatal(err)
	}
	attacks := [][]byte{
		bytes.Replace(raw, []byte("\"initialize\""), []byte("\"tools/list\""), 1),
		bytes.Replace(raw, []byte("\"plugin_data\":\"/a/data\""), []byte("\"plugin_data\":\"/b/data\""), 1),
		bytes.Replace(raw, []byte("\"pid\":31"), []byte("\"pid\":32"), 1),
		bytes.Replace(raw, []byte("\"nonce\":\"nonce\""), []byte("\"nonce\":\"old\""), 1),
		bytes.Replace(raw, []byte("\"marker\":\"marker\""), []byte("\"marker\":\"foreign\""), 1),
		append(bytes.Clone(raw), raw...),
	}
	for index, attack := range attacks {
		if _, err := kiroCycleEvents(attack, "nonce", r, start, end, true); err == nil {
			t.Fatal("unjoined native event admitted", index)
		}
	}
	if _, err := kiroCycleEvents(raw, "nonce", r, end, end.Add(time.Second), true); err == nil {
		t.Fatal("stale events admitted")
	}
	if _, err := kiroCycleEvents(nil, "nonce", r, start, end, false); err != nil {
		t.Fatal(err)
	}
	if _, err := kiroCycleEvents(raw, "nonce", r, start, end, false); err == nil {
		t.Fatal("removed server event was ignored")
	}
}

func TestKiroCycleSnapshotRetainsOriginalIdentityAcrossSameByteReplacement(t *testing.T) {
	path := t.TempDir() + "/original"
	if err := os.WriteFile(path, []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := kiroCycleCapture(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := kiroCycleCapture(path)
	if err != nil || os.SameFile(before.info, after.info) || !bytes.Equal(before.body, after.body) {
		t.Fatal("same-byte replacement test did not retain distinct original authority", err)
	}
}
