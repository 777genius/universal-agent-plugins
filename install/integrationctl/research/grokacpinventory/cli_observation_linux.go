//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
)

const cliMode = "plugin-cli-observation-v1"

func decodeResearchResult(body []byte, got *result, mode string) error {
	if e := closed(body, got, lockResultFields(mode, sessionFields(mode, "schema_version source_commit source_tree scope nonce semantic_complete reason isolation commands rpc", "mode provider"))); e != nil {
		return e
	}
	if mode == cliMode {
		var flag struct {
			Complete *bool `json:"semantic_complete"`
		}
		if e := json.Unmarshal(body, &flag); e != nil || flag.Complete == nil {
			return errors.New("cli-completion-type-refused")
		}
	}
	return nil
}

// The parent must first join this fixed selector to the same-run native ACP row.
func cliArguments(r request) []string {
	root := "/tmp/uap-grok-acp-input-" + r.Nonce + "/a/.grok/plugins/uap-acp-collision"
	sum := sha256.Sum256([]byte(root))
	return map[string][]string{
		"details-help":    {"plugin", "details", "--help"},
		"list-json":       {"plugin", "list", "--json"},
		"details-full-id": {"plugin", "details", "project/" + hex.EncodeToString(sum[:])[:8] + "/uap-acp-collision"},
		"details-name":    {"plugin", "details", "uap-acp-collision"},
		"inspect-project": {"inspect", "--json"},
	}[r.Phase]
}

func validateCLIResult(got result, r request) error {
	if e := validateResearchMode(r); e != nil {
		return e
	}
	if got.Mode != cliMode || got.Phase != r.Phase || got.Commands == nil || len(got.Commands) > 1 ||
		got.Complete && len(got.Commands) != 1 || got.LockHeld != nil || len(got.Provider) != 0 {
		return errors.New("cli-result-refused")
	}
	var rpc map[string]json.RawMessage
	if e := closed(got.RPC, &rpc, "initialize plugins notifications"); e != nil {
		return e
	}
	if !bytes.Equal(bytes.TrimSpace(rpc["initialize"]), []byte("null")) ||
		!bytes.Equal(bytes.TrimSpace(rpc["plugins"]), []byte("null")) ||
		!bytes.Equal(bytes.TrimSpace(rpc["notifications"]), []byte("[]")) {
		return errors.New("cli-rpc-refused")
	}
	if len(got.Commands) == 1 {
		return validateCLICommand(got.Commands[0], got.Complete, r)
	}
	return nil
}

func validateCLICommand(raw []byte, complete bool, r request) error {
	var row struct {
		Label   string   `json:"label"`
		Argv    []string `json:"argv"`
		Exit    *int     `json:"exit"`
		Error   *string  `json:"error"`
		Encoded string   `json:"merged_base64"`
		Bytes   *int     `json:"merged_bytes"`
		SHA     string   `json:"merged_sha256"`
	}
	if e := closed(raw, &row, "label argv exit error merged_base64 merged_bytes merged_sha256"); e != nil {
		return e
	}
	if row.Label != r.Phase || row.Exit == nil || *row.Exit < -255 || *row.Exit > 255 ||
		row.Bytes == nil || *row.Bytes < 0 || *row.Bytes > 16<<10 ||
		len(row.Encoded) != base64.StdEncoding.EncodedLen(*row.Bytes) || !validCLIArgv(row.Argv, r) {
		return errors.New("cli-command-refused")
	}
	if row.Error != nil && (complete || (*row.Error != "deadline" && *row.Error != "stream-bound" &&
		*row.Error != "process-refused" && *row.Error != "protocol-refused")) {
		return errors.New("cli-command-error-refused")
	}
	body, e := base64.StdEncoding.DecodeString(row.Encoded)
	sum := sha256.Sum256(body)
	if e != nil || len(body) != *row.Bytes || hex.EncodeToString(sum[:]) != row.SHA {
		return errors.New("cli-stream-refused")
	}
	return nil
}

func validCLIArgv(argv []string, r request) bool {
	if len(argv) == 0 {
		return false
	}
	return reflect.DeepEqual(argv[1:], cliArguments(r)) && filepath.Clean(argv[0]) == argv[0] &&
		filepath.Base(argv[0]) == "grok" && filepath.Dir(filepath.Dir(argv[0])) == "/tmp" &&
		strings.HasPrefix(filepath.Base(filepath.Dir(argv[0])), "uap-grok-acp-sealed-")
}
