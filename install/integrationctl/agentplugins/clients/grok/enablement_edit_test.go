package grok

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func TestEnablementEditRecoversExactPredecessorWithoutPersistingForeignBytes(t *testing.T) {
	for _, original := range []string{
		"# PRIVATE_TOKEN in original\n[plugins]\nenabled=[]\n",
		"# PRIVATE_TOKEN in original\r\n[plugins]\r\nenabled=['foreign', ]\r\n",
		"# PRIVATE_TOKEN in original\n[plugins]\ndisabled=['foreign']\n[models]\nendpoint='PRIVATE_TOKEN'\n",
		"# PRIVATE_TOKEN in original without final newline",
	} {
		before := nativeconfig.FileSnapshot{Body: []byte(original), Mode: 0600, Exists: true}
		edit, after, err := prepareEnablementEdit(before, rawMemberID, "owned")
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(edit)
		if err != nil || bytes.Contains(body, []byte("PRIVATE_TOKEN")) || !bytes.Contains(body, []byte(rawMemberID)) {
			t.Fatalf("journal leaks predecessor or omits insertion: %s, %v", body, err)
		}
		var restarted enablementEdit
		if err := json.Unmarshal(body, &restarted); err != nil {
			t.Fatal(err)
		}
		restored, err := restarted.undo(nativeconfig.FileSnapshot{Body: after, Mode: 0600, Exists: true}, rawMemberID)
		if err != nil || !bytes.Equal(restored, before.Body) {
			t.Fatalf("recovery differs: %q, %v", restored, err)
		}
	}
}

func TestEnablementEditRefusesThirdVersionAndModeDrift(t *testing.T) {
	before := nativeconfig.FileSnapshot{Body: []byte("[plugins]\nenabled=[]\n"), Mode: 0644, Exists: true}
	edit, after, err := prepareEnablementEdit(before, rawMemberID, "owned")
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []nativeconfig.FileSnapshot{
		{Body: append(bytes.Clone(after), []byte("# concurrent edit\n")...), Mode: 0644, Exists: true},
		{Body: bytes.Clone(after), Mode: 0600, Exists: true},
		{Body: bytes.Clone(after), Mode: 0644},
		before,
	} {
		saved := bytes.Clone(changed.Body)
		if restored, err := edit.undo(changed, rawMemberID); !errors.Is(err, nativeconfig.ErrConcurrentChange) || restored != nil {
			t.Fatalf("concurrent version authorized rollback: %q, %v", restored, err)
		}
		if !bytes.Equal(changed.Body, saved) {
			t.Fatal("concurrent input changed")
		}
	}
}

func TestEnablementEditRejectsCorruptRecoverySpan(t *testing.T) {
	before := nativeconfig.FileSnapshot{Body: []byte("[plugins]\nenabled=[]\n"), Mode: 0600, Exists: true}
	edit, after, err := prepareEnablementEdit(before, rawMemberID, "owned")
	if err != nil {
		t.Fatal(err)
	}
	current := nativeconfig.FileSnapshot{Body: after, Mode: 0600, Exists: true}
	for _, mutate := range []func(*enablementEdit){
		func(value *enablementEdit) { value.Offset = -1 },
		func(value *enablementEdit) { value.Offset = grokMembershipLimit + 1 },
		func(value *enablementEdit) { value.Offset++ },
		func(value *enablementEdit) { value.Added += "PRIVATE_TOKEN" },
		func(value *enablementEdit) { value.Added = "\"foreign-id\"" },
		func(value *enablementEdit) { value.BeforeDigest = value.AfterDigest },
		func(value *enablementEdit) { value.Mode = 0666 },
	} {
		changed := edit
		mutate(&changed)
		if restored, err := changed.undo(current, rawMemberID); err == nil || restored != nil {
			t.Fatal("corrupt recovery span admitted")
		}
	}
}
