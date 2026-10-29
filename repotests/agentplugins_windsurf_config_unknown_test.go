//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type windsurfConfigUnknownMember struct {
	Status         string `json:"status"`
	KeySHA256      string `json:"key_sha256,omitempty"`
	Type           string `json:"type,omitempty"`
	SemanticSHA256 string `json:"semantic_json_sha256,omitempty"`
}

func windsurfConfigObserveUnknown(object map[string]any, fields *windsurfConfigAdditionalFields) *windsurfConfigUnknownMember {
	if fields.UnknownCount != 1 {
		return nil
	}
	var name string
	var value any
	count := 0
	for key, item := range object {
		if key == "auto_update" || key == "permissions" {
			continue
		}
		if _, known := fields.Documented[key]; known {
			continue
		}
		name, value, count = key, item, count+1
	}
	observation := &windsurfConfigUnknownMember{Status: "count-refused"}
	if count != 1 {
		return observation
	}
	kind := windsurfConfigValueType(value)
	switch kind {
	case "null", "boolean", "number", "string", "array", "object":
	default:
		observation.Status = "type-refused"
		return observation
	}
	body, err := json.Marshal(value)
	observation.Status = "value-encoding-refused"
	if err != nil {
		return observation
	}
	observation.Status = "value-bound"
	if len(body) > windsurfConfigDiagnosticLimit {
		return observation
	}
	keyDigest, valueDigest := windsurfConfigDigest([]byte(name)), windsurfConfigDigest(body)
	observation.Status = "digest-refused"
	if !windsurfConfigDigestValid(keyDigest) || !windsurfConfigDigestValid(valueDigest) {
		return observation
	}
	return &windsurfConfigUnknownMember{Status: "captured", KeySHA256: keyDigest, Type: kind, SemanticSHA256: valueDigest}
}

func windsurfUnknownFileCapture(t *testing.T, suffix []byte, changedSafety bool) windsurfConfigChange {
	t.Helper()
	f, path, original := newWindsurfConfigDiagnostic(t)
	body := bytes.TrimSpace(original)
	body = append(bytes.Clone(body[:len(body)-1]), suffix...)
	if changedSafety {
		body = bytes.Replace(body, []byte(`"auto_update":false`), []byte(`"auto_update":true`), 1)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	unchanged := windsurfConfigDiagnosticUnchanged(t, path, path)
	observed, err := projectNativeRuntimeDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	f.captureWindsurfConfigChange(path, f.protected[path], observed, nil)
	change := windsurfConfigDiagnosticResult(t, f, path, f.protected[path], "captured")
	if change.AutoUpdate.MatchesAuthored == changedSafety || !change.Permissions.MatchesAuthored || observed == f.protected[path] {
		t.Fatal("unknown observation concealed a safety change or adopted the observed digest")
	}
	unchanged()
	return change
}

func TestWindsurfUnknownMemberReportsOnlyIndependentNameTypeAndSemanticHashes(t *testing.T) {
	for _, tc := range []struct{ kind, raw, digest string }{
		{"null", `null`, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
		{"boolean", `false`, "fcbcf165908dd18a9e49f7ff27810176db8e9f63b4352213741664245224f8aa"},
		{"number", `17`, "4523540f1504cd17100c4835e85b7eefd49911580f8efff0599a8f283be6b9e3"},
		{"string", `"PRIVATE-VALUE /private/path"`, "76e30cb8ae2c7f79081a485a42848d283ed69469ceb909ccfd9bd75f78a0a133"},
		{"array", `["PRIVATE-VALUE",17]`, "ee4d7d63f2fc13bebba6f30344ccbecfe3777e3aa876e9868525cfb2a92c0d3f"},
		{"object", `{ "z": false, "a": "PRIVATE-VALUE" }`, "39ee2397b43808a81eb436b44a01e68a52665e1b45d9a57c1204e1639f168314"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			for _, safety := range []bool{false, true} {
				change := windsurfUnknownFileCapture(t, []byte(",\"PRIVATE-KEY\\n\\u001b\":"+tc.raw+"}"), safety)
				want := &windsurfConfigUnknownMember{Status: "captured", KeySHA256: "998d09234bf34d49973872e6e0a841789fcd54b66c3e16d441dd65e2d5bcddd3", Type: tc.kind, SemanticSHA256: tc.digest}
				if change.AdditionalFields.UnknownCount != 1 || len(change.AdditionalFields.Documented) != 0 || !reflect.DeepEqual(change.AdditionalFields.UnknownObservation, want) {
					t.Fatal("unknown member metadata or independently stated exact semantic digest differs")
				}
			}
		})
	}
}

func TestWindsurfUnknownObservationKeepsKnownZeroMultipleAndNearMissClassification(t *testing.T) {
	for _, additions := range []map[string]any{{}, {"Theme_Mode": true, "theme_mоde": false}, {"theme_mode": nil, "agent": true}} {
		change := windsurfAdditionalFieldsCapture(t, additions)
		if change.AdditionalFields.UnknownObservation != nil {
			t.Fatal("zero or multiple unknown names gained an arbitrary member observation")
		}
	}
	for _, key := range []string{"Theme_Mode", "theme_mоde"} {
		change := windsurfAdditionalFieldsCapture(t, map[string]any{key: true})
		if change.AdditionalFields.UnknownObservation == nil || change.AdditionalFields.UnknownObservation.Status != "captured" || len(change.AdditionalFields.Documented) != 0 {
			t.Fatal("case/homoglyph near-miss became a documented name")
		}
	}
	object := map[string]any{"PRIVATE-KEY": map[string]any{"PRIVATE-VALUE": true}, "theme_mode": "PRIVATE-VALUE"}
	before, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	fields := windsurfConfigClassifyAddedFields(object)
	fields.UnknownObservation.Type = "changed-returned-field"
	fields.Documented["theme_mode"] = "changed-returned-map"
	after, err := json.Marshal(object)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("returned observation aliases the parsed source object")
	}
}

func TestWindsurfUnknownObservationRefusesValueExpansionAndImpossibleInternalTypes(t *testing.T) {
	change := windsurfUnknownFileCapture(t, []byte(",\"PRIVATE-KEY\":\""+strings.Repeat("&", 12000)+"\"}"), false)
	if !reflect.DeepEqual(change.AdditionalFields.UnknownObservation, &windsurfConfigUnknownMember{Status: "value-bound"}) {
		t.Fatal("bounded held input exported an oversized semantic value or partial metadata")
	}
	for _, tc := range []struct {
		object map[string]any
		status string
	}{
		{map[string]any{"PRIVATE-KEY": make(chan int)}, "type-refused"},
		{map[string]any{"PRIVATE-KEY": json.Number("private-invalid-number")}, "value-encoding-refused"},
		{map[string]any{"PRIVATE-KEY": true, "PRIVATE-OTHER": false}, "count-refused"},
	} {
		fields := &windsurfConfigAdditionalFields{Documented: map[string]string{}, UnknownCount: 1}
		if got := windsurfConfigObserveUnknown(tc.object, fields); !reflect.DeepEqual(got, &windsurfConfigUnknownMember{Status: tc.status}) {
			t.Fatal("impossible internal state exposed a seventh type or partial/private metadata")
		}
	}
}
