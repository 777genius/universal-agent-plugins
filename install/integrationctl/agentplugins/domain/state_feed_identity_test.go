package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDirectoryFeedIdentityPreservesLegacyReceipts(t *testing.T) {
	const historical = `{"product_id":"tool","distribution_id":"owner/tool","distribution_kind":"upstream","desired_release_sequence":2,"snapshot_schema":1,"snapshot_sequence":43,"snapshot_digest":"sha256:old"}`
	var receipt DirectoryOrigin
	if err := json.Unmarshal([]byte(historical), &receipt); err != nil {
		t.Fatalf("decode historical directory receipt: %v", err)
	}
	if receipt.FeedID != "" || receipt.SnapshotSequence != 43 {
		t.Fatalf("historical receipt was reclassified: %+v", receipt)
	}
	legacy, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacy), `"feed_id"`) {
		t.Fatalf("legacy receipt gained new feed identity: %s", legacy)
	}
	receipt.FeedID = "modern-v1"
	modern, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var restored DirectoryOrigin
	if err := json.Unmarshal(modern, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.FeedID != "modern-v1" || restored.SnapshotSequence != 43 {
		t.Fatalf("modern receipt lost signed feed identity: %+v", restored)
	}
}
