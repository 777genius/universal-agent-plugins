package grok

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// enablementEdit carries only the owned native ID and reversible formatting bytes.
// The original configuration may contain credentials and is never journaled.
type enablementEdit struct {
	Offset       int    `json:"offset"`
	Added        string `json:"added"`
	Removed      string `json:"removed,omitempty"`
	BeforeDigest string `json:"before_digest"`
	AfterDigest  string `json:"after_digest"`
	Mode         uint32 `json:"mode"`
}

func enablementDigest(body []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(body))
}

func prepareEnablementEdit(before nativeconfig.FileSnapshot, id, name string) (enablementEdit, []byte, error) {
	var edit enablementEdit
	if !before.Exists || !before.Mode.IsRegular() || before.Mode.Perm() != 0600 && before.Mode.Perm() != 0644 {
		return edit, nil, fmt.Errorf("grok enablement requires an existing ordinary native config")
	}
	after, err := prepareProjectEnableBytes(before.Body, id, name)
	if err != nil {
		return edit, nil, err
	}
	parsed, lists, err := membershipDocument(before.Body)
	if err != nil {
		return edit, nil, err
	}
	_, hasPlugins := parsed["plugins"]
	position, prefix, suffix, err := enabledInsertion(before.Body, len(lists["enabled"]), hasPlugins)
	if err != nil {
		return edit, nil, err
	}
	edit = enablementEdit{Offset: position, Added: prefix + "\"" + id + "\"" + suffix,
		BeforeDigest: enablementDigest(before.Body), AfterDigest: enablementDigest(after), Mode: uint32(before.Mode.Perm())}
	if err := edit.validate(id); err != nil {
		return enablementEdit{}, nil, err
	}
	return edit, after, nil
}

func prepareDisablementEdit(before nativeconfig.FileSnapshot, id string) (enablementEdit, []byte, error) {
	var edit enablementEdit
	if !before.Exists || !before.Mode.IsRegular() || before.Mode.Perm() != 0600 && before.Mode.Perm() != 0644 {
		return edit, nil, fmt.Errorf("grok removal requires an existing ordinary native config")
	}
	change, err := prepareProjectRemoveBytes(before.Body, id)
	if err != nil {
		return edit, nil, err
	}
	edit = enablementEdit{Offset: change.Offset, Removed: change.Removed, BeforeDigest: enablementDigest(before.Body),
		AfterDigest: enablementDigest(change.Body), Mode: uint32(before.Mode.Perm())}
	return edit, change.Body, edit.validate(id)
}

func (edit enablementEdit) validate(id string) error {
	if edit.Offset < 0 || edit.Offset > grokMembershipLimit || !membershipEncodedID(id) ||
		!enablementHash(edit.BeforeDigest) || !enablementHash(edit.AfterDigest) || (edit.Mode != 0600 && edit.Mode != 0644) {
		return fmt.Errorf("grok enablement edit authority is invalid")
	}
	valid := edit.Added == "" && edit.Removed == "" && edit.Offset == 0 && edit.BeforeDigest == edit.AfterDigest
	if edit.BeforeDigest != edit.AfterDigest {
		valid = edit.Removed == "" && enablementInsertion(edit.Added, id) || edit.Added == "" && enablementRemoval(edit.Removed, id)
	}
	if !valid {
		return fmt.Errorf("grok native membership edit is not one reversible owned span")
	}
	return nil
}

func enablementRemoval(removed, id string) bool {
	if len(removed) > 4096 || strings.ContainsAny(removed, "\r\n") || strings.Count(removed, ",") > 1 {
		return false
	}
	token := strings.Trim(strings.Trim(removed, " \t"), ",")
	token = strings.Trim(token, " \t")
	return token == "\""+id+"\"" || token == "'"+id+"'"
}

func enablementHash(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	for _, character := range value[7:] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func enablementInsertion(added, id string) bool {
	quoted := "\"" + id + "\""
	for _, prefix := range []string{"", " ", ", "} {
		if added == prefix+quoted {
			return true
		}
	}
	for _, newline := range []string{"\n", "\r\n"} {
		for _, prefix := range []string{"", newline, "[plugins]" + newline, newline + "[plugins]" + newline} {
			if added == prefix+"enabled = ["+quoted+"]"+newline {
				return true
			}
		}
	}
	return false
}

// undo reconstructs a predecessor only from the exact still-owned postimage.
// A third version is retained, including edits outside [plugins].enabled.
func (edit enablementEdit) undo(current nativeconfig.FileSnapshot, id string) ([]byte, error) {
	if err := edit.validate(id); err != nil {
		return nil, err
	}
	end := edit.Offset + len(edit.Added)
	if !current.Exists || current.Mode != os.FileMode(edit.Mode) || len(current.Body) > grokMembershipLimit ||
		enablementDigest(current.Body) != edit.AfterDigest || end > len(current.Body) ||
		!bytes.Equal(current.Body[edit.Offset:end], []byte(edit.Added)) {
		return nil, fmt.Errorf("retain concurrent grok config: %w", nativeconfig.ErrConcurrentChange)
	}
	before := make([]byte, 0, len(current.Body)-len(edit.Added)+len(edit.Removed))
	before = append(before, current.Body[:edit.Offset]...)
	before = append(before, edit.Removed...)
	before = append(before, current.Body[end:]...)
	if enablementDigest(before) != edit.BeforeDigest {
		return nil, fmt.Errorf("grok enablement predecessor digest differs")
	}
	return before, nil
}
