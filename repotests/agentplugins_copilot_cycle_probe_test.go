//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"time"
)

var copilotCycleProbeID = regexp.MustCompile(`^[1-9]\d{0,9}-[1-9]\d{0,19}$`)

func (c *copilotProjectCycle) events() ([]byte, error) {
	raw, err := projectNativeRuntimeRead(c.f.base.Events, 1<<20)
	if os.IsNotExist(err) {
		return []byte{}, nil
	}
	return raw, err
}

func (c *copilotProjectCycle) facts(r *copilotCycleRecipe, body string) (nativeFacts, error) {
	var facts nativeFacts
	value, err := projectNativeRuntimeJSON([]byte(body))
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return facts, fmt.Errorf("Copilot cycle exact returned facts shape")
	}
	if err = json.Unmarshal([]byte(body), &facts); err != nil {
		return facts, err
	}
	if facts.Nonce != r.Nonce || facts.Revision != r.Revision || facts.Root != r.Active || facts.Data != r.Data || facts.CWD != r.Active || !reflect.DeepEqual(facts.Argv, []string{"argument with spaces", r.Active + "/argument", "${UNKNOWN}"}) || facts.Literal != "literal ${UNKNOWN} "+c.f.base.Home || facts.Once != r.Active+"/${UNKNOWN}" || facts.Marker != "retained-"+r.Nonce || facts.PID <= 0 || !copilotCycleProbeID.MatchString(facts.Session) {
		return facts, fmt.Errorf("Copilot cycle returned facts differ from held recipe")
	}
	return facts, nil
}

func (c *copilotProjectCycle) probe(delta []byte, r *copilotCycleRecipe, facts nativeFacts, start, end time.Time, session, call string) error {
	if r == nil {
		if len(delta) != 0 {
			return fmt.Errorf("Copilot cycle no-call phase started MCP")
		}
		return nil
	}
	if !bytes.HasSuffix(delta, []byte{'\n'}) || c.probeIDs[facts.Session] || session == facts.Session || call == facts.Session || c.nativeIDs[facts.Session] {
		return fmt.Errorf("Copilot cycle probe is stale or shares native/provider identity")
	}
	lines := bytes.Split(bytes.TrimSuffix(delta, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 3 {
		return fmt.Errorf("Copilot cycle exact initialize/list/call triple")
	}
	last := start
	for i, method := range []string{"initialize", "tools/list", "tools/call"} {
		value, err := projectNativeRuntimeJSON(lines[i])
		row, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(row, "method pid session nonce timestamp facts", "") != nil || row["method"] != method || row["pid"] != json.Number(fmt.Sprint(facts.PID)) || row["session"] != facts.Session || row["nonce"] != r.Nonce {
			return fmt.Errorf("Copilot cycle probe method/session/scope")
		}
		stamp, ok := row["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if !ok || err != nil || when.Before(last) || when.After(end) {
			return fmt.Errorf("Copilot cycle probe interval")
		}
		last = when
		if i < 2 {
			if row["facts"] != nil {
				return fmt.Errorf("Copilot cycle startup has returned facts")
			}
			continue
		}
		want, _ := json.Marshal(facts)
		actual, _ := json.Marshal(row["facts"])
		if !bytes.Equal(actual, want) {
			a, _ := projectNativeRuntimeJSON(actual)
			b, _ := projectNativeRuntimeJSON(want)
			if !reflect.DeepEqual(a, b) {
				return fmt.Errorf("Copilot cycle probe and native/provider facts differ")
			}
		}
	}
	c.probeIDs[facts.Session] = true
	return nil
}
