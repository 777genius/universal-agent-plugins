package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// A closed, fixture-only inventory operation uses the same unchanged production
// process owner. It never installs files, grants trust or runs a model prompt.
func capturePiCatalog(args []string) (err error) {
	if err := validateCatalogFixturePaths(args); err != nil {
		return err
	}
	binary, user, cwd, output := args[0], args[1], args[2], args[3]
	before, err := catalogBinaryHash(binary)
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	id := "uap-inventory-" + hex.EncodeToString(nonce[:])
	request, _ := json.Marshal(map[string]string{"id": id, "type": "get_commands"})
	request = append(request, '\n')
	record := map[string]any{"schema_version": 1, "argv": []string{binary, "--mode", "rpc", "--no-session", "--offline"},
		"cwd": cwd, "user_profile": user, "deadline_seconds": 30, "stdout_limit_bytes": 1 << 20,
		"request_base64": base64.StdEncoding.EncodeToString(request), "native_binary_sha256": before, "passed": false, "closure_proved": false}
	var raw []byte
	started := time.Now()
	defer func() { err = errors.Join(err, writeCatalogFixtureReceipt(output, record, started, raw, err)) }()
	environment := append([]string(nil), os.Environ()...)
	environment = replaceCatalogEnvironment(environment, "PI_CODING_AGENT_DIR", user)
	environment = replaceCatalogEnvironment(environment, "PI_OFFLINE", "1")
	command := legacyports.Command{Argv: []string{binary, "--mode", "rpc", "--no-session", "--offline"}, Dir: cwd, Env: environment, StdoutLimitBytes: 1 << 20}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = (process.OS{}).RunDuplexToCompletion(ctx, command, func(stdin io.Writer, stdout io.Reader) error {
		if _, writeErr := stdin.Write(request); writeErr != nil {
			return writeErr
		}
		var response map[string]any
		var exchangeErr error
		raw, response, exchangeErr = fixturePiCatalogResponse(stdout, id)
		if response != nil {
			record["response"] = response
		}
		return exchangeErr // only now does the unchanged runner close stdin
	})
	if err != nil {
		return err
	}
	record["closure_proved"] = true
	after, hashErr := catalogBinaryHash(binary)
	if hashErr != nil {
		return hashErr
	}
	if before != after {
		return errors.New("native binary changed during inventory")
	}
	record["binary_unchanged"] = true
	record["closure_proved"] = true
	record["passed"] = true
	return nil
}

func replaceCatalogEnvironment(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !bytes.HasPrefix([]byte(entry), []byte(prefix)) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

func catalogBinaryHash(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path || !before.Mode().IsRegular() {
		return "", errors.New("native binary fixture is not canonical regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	held, statErr := file.Stat()
	hash := sha256.New()
	count, readErr := io.Copy(hash, io.LimitReader(file, (175<<20)+1))
	closeErr := file.Close()
	if err = errors.Join(statErr, readErr, closeErr); err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if count > 175<<20 || count != before.Size() || !os.SameFile(before, held) || !os.SameFile(before, after) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || before.ModTime() != after.ModTime() {
		return "", errors.New("native binary fixture changed")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fixturePiCatalogResponse(stdout io.Reader, id string) ([]byte, map[string]any, error) {
	raw, err := bufio.NewReader(io.LimitReader(stdout, (1<<20)+1)).ReadBytes('\n')
	if err != nil {
		return raw, nil, err
	}
	if len(raw) > 1<<20 {
		return raw, nil, errors.New("native catalog exceeded live bound")
	}
	response, err := shared.DecodeStrictJSONObject(raw)
	if err != nil {
		return raw, nil, err
	}
	if response["type"] != "response" || response["id"] != id || response["command"] != "get_commands" || response["success"] != true {
		return raw, nil, errors.New("native inventory response differs")
	}
	data, ok := response["data"].(map[string]any)
	if !ok {
		return raw, nil, errors.New("native inventory missing data")
	}
	rows, ok := data["commands"].([]any)
	if !ok || len(rows) > 512 {
		return raw, nil, errors.New("native inventory missing bounded commands")
	}
	return raw, response, nil
}

func validateCatalogFixturePaths(args []string) error {
	if len(args) != 4 {
		return errors.New("catalog requires PI USER_PROFILE CWD OUTPUT")
	}
	for _, path := range args {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return errors.New("catalog fixture paths must be absolute and clean")
		}
	}
	return nil
}

func writeCatalogFixtureReceipt(output string, record map[string]any, started time.Time, raw []byte, failure error) error {
	record["elapsed_seconds"] = time.Since(started).Seconds()
	record["stdout_base64"] = base64.StdEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	record["stdout_sha256"] = hex.EncodeToString(hash[:])
	record["stdout_bytes"] = len(raw)
	if failure != nil {
		record["error"] = failure.Error()
	}
	body, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, append(body, '\n'), 0600)
}
