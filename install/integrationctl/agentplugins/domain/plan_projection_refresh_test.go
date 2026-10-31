package domain

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNativeProjectionRefreshDecisionIsEphemeral(t *testing.T) {
	plain, err := json.Marshal(DeliveryPlan{})
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := json.Marshal(DeliveryPlan{}.WithNativeProjectionRefresh(true))
	if err != nil || string(plain) != string(refresh) {
		t.Fatalf("operational decision changed public JSON: %s, %v", refresh, err)
	}
	var restored DeliveryPlan
	if err := json.Unmarshal([]byte(`{"RequiresNativeProjectionRefresh":true,"requires_native_projection_refresh":true}`), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.RequiresNativeProjectionRefresh() {
		t.Fatal("JSON restored an adapter planning decision")
	}
}

func TestNativeProjectionRefreshDecisionCopiesAndResets(t *testing.T) {
	zero := DeliveryPlan{ClientID: ClientOpenCode, PreviousNativeObjects: []NativeObjectOwnership{{Kind: "owned"}}}
	refresh := zero.WithNativeProjectionRefresh(true)
	copied := refresh
	reset := copied.WithNativeProjectionRefresh(false)
	if zero.RequiresNativeProjectionRefresh() || !refresh.RequiresNativeProjectionRefresh() || !copied.RequiresNativeProjectionRefresh() || reset.RequiresNativeProjectionRefresh() {
		t.Fatal("decision lost zero, copy, receiver, or reset semantics")
	}
	if !reflect.DeepEqual(zero, reset) {
		t.Fatal("decision changed other plan facts")
	}
}
