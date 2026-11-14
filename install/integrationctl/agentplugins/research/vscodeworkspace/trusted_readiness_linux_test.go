//go:build linux

package main

import "testing"

func TestTrustedInventoryReadinessReceipt(t *testing.T) {
	control := map[string]any{"label": "uap-foreign-a", "uri": "http://127.0.0.1:9/preserved-foreign/nonce/a"}
	valid := func() map[string]any {
		return map[string]any{"label": control["label"], "uri": control["uri"], "definitionsReady": true, "gatewayReady": true}
	}
	if err := trustedObservedReadiness(valid(), control); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"label", "uri", "definitionsReady", "gatewayReady", "extra"} {
		row := valid()
		row[key] = "wrong"
		if err := trustedObservedReadiness(row, control); err == nil {
			t.Fatalf("accepted changed %s", key)
		}
	}
	for _, key := range []string{"label", "uri", "definitionsReady", "gatewayReady"} {
		row := valid()
		delete(row, key)
		if err := trustedObservedReadiness(row, control); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
}

func TestTrustedInventoryReadinessRejectsMalformedExpectation(t *testing.T) {
	for _, expected := range []any{nil, map[string]any{}, map[string]any{"label": nil, "uri": nil}, map[string]any{"label": "", "uri": ""}, map[string]any{"label": "a", "uri": 1}} {
		if err := trustedObservedReadiness(map[string]any{"label": nil, "uri": nil, "definitionsReady": true, "gatewayReady": true}, expected); err == nil {
			t.Fatal("accepted malformed expectation")
		}
	}
	f := &trustedLifecycle{Nonce: "nonce", Foreign: map[string]any{}}
	for _, value := range []any{nil, "not-object", map[string]any{"type": "http", "url": "stale"}} {
		f.Foreign["a"] = value
		if _, err := trustedReadinessControl(f, "a"); err == nil {
			t.Fatal("accepted missing/malformed fixture")
		}
	}
	f.Foreign["a"] = map[string]any{"type": "http", "url": "http://127.0.0.1:9/preserved-foreign/nonce/a"}
	if _, err := trustedReadinessControl(f, "a"); err != nil {
		t.Fatal(err)
	}
}
