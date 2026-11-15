package hermes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

	"gopkg.in/yaml.v3"
)

type sessionRuntimeSelection struct {
	inspect     bool
	bundle      hermesBundle
	environment []string
}

type sessionSourceSnapshot struct {
	body   []byte
	digest string
}

type sessionRuntimeMember struct {
	name, leaf, identity string
	limit                int64
	mode                 os.FileMode
	parent               *sessionHeldDirectory
	file                 *os.File
	info                 os.FileInfo
	body                 []byte
}

type sessionRuntimeSnapshot struct {
	root, logs *sessionHeldDirectory
	members    []*sessionRuntimeMember
	passive    *sessionPassiveState
}

func captureSessionSource(ctx context.Context, project string, owner domain.NativeObjectOwnership, names []string, selection sessionRuntimeSelection) (sessionSourceSnapshot, error) {
	result, state, err := acquireSessionSource(ctx, project, owner, names, selection)
	if state != nil {
		err = errors.Join(err, state.close())
	}
	return result, err
}

func acquireSessionSource(ctx context.Context, project string, owner domain.NativeObjectOwnership, names []string, selection sessionRuntimeSelection) (result sessionSourceSnapshot, state *sessionRuntimeSnapshot, resultErr error) {
	if err := verifyProjectSessionRoot(ctx, project, owner); err != nil {
		return result, nil, err
	}
	directory, err := openSessionDirectory(projectSessionRoot(project))
	if err != nil {
		return result, nil, err
	}
	state = &sessionRuntimeSnapshot{root: directory}
	captured := state
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, captured.close())
			state = nil
		}
	}()
	if err := validateSessionDirectoryHandle(directory.file, directory.info); err != nil {
		return result, nil, err
	}
	if err := state.capture(ctx, selection.inspect); err != nil {
		return result, nil, err
	}
	if len(state.admitted()) != 0 {
		state.passive, err = captureSessionPassive(ctx, selection.bundle, selection.environment)
		if err != nil {
			return result, nil, err
		}
	}
	if err := verifySessionBootstrap(ctx, directory.root, directory.info, state.admitted()); err != nil {
		return result, nil, err
	}
	result.body, err = readSessionMember(directory.root, "config.yaml", 1<<20)
	if err != nil {
		return result, nil, err
	}
	if err := exactProjectSessionConfig(result.body, names); err != nil {
		return result, nil, err
	}
	if err := ctx.Err(); err != nil {
		return result, nil, err
	}
	if err := state.validate(result.body); err != nil {
		return result, nil, err
	}
	if err := state.rejoin(ctx); err != nil {
		return result, nil, err
	}
	result.digest, err = state.commitment(result.body, selection)
	return result, state, errors.Join(err, ctx.Err())
}

func (state *sessionRuntimeSnapshot) capture(ctx context.Context, inspect bool) error {
	if _, err := state.root.root.Lstat("logs"); err == nil {
		state.logs, err = state.root.child("logs")
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	state.members = []*sessionRuntimeMember{
		{name: "spawn-ledger.json", leaf: "spawn-ledger.json", limit: 4096, mode: 0o600, parent: state.root},
		{name: "spawn-ledger.json.lock", leaf: "spawn-ledger.json.lock", mode: 0o644, parent: state.root},
		{name: "logs/mcp-stderr.log", leaf: "mcp-stderr.log", limit: 512, mode: 0o644, parent: state.logs},
	}
	if runtime.GOOS == "windows" {
		state.members = append(state.members, &sessionRuntimeMember{name: "logs/.__agent.lock", leaf: ".__agent.lock", mode: 0o666, parent: state.logs})
	}
	for _, member := range state.members {
		if err := ctx.Err(); err != nil {
			return err
		}
		if member.parent == nil {
			continue
		}
		info, err := member.parent.root.Lstat(member.leaf)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !inspect {
			return fmt.Errorf("hermes runtime history is restricted to passive inspection")
		}
		member.info = info
		if err := member.capture(ctx); err != nil {
			return err
		}
	}
	if (state.members[0].file == nil) != (state.members[1].file == nil) {
		return fmt.Errorf("hermes runtime registry pair is incomplete")
	}
	return nil
}

func (member *sessionRuntimeMember) capture(ctx context.Context) error {
	if !member.info.Mode().IsRegular() || member.info.Size() < 0 || member.info.Size() > member.limit ||
		(runtime.GOOS != "windows" || member.name == "logs/.__agent.lock") && member.info.Mode() != member.mode {
		return fmt.Errorf("hermes runtime history member is not bounded and ordinary")
	}
	var err error
	member.file, err = openSessionFile(member.parent.root, member.leaf)
	if err != nil {
		return err
	}
	held, err := member.file.Stat()
	if err != nil || !sameSessionMember(member.info, held) {
		return fmt.Errorf("hermes runtime history changed before read")
	}
	if err := validateSessionOwnedHandle(member.file, held); err != nil {
		return err
	}
	member.identity, err = sessionPhysicalIdentity(member.file, held)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	member.body, err = member.readBody()
	if err != nil || int64(len(member.body)) != held.Size() || int64(len(member.body)) > member.limit {
		return fmt.Errorf("hermes runtime history read exceeded its original bound")
	}
	return member.rejoin(ctx)
}

func (member *sessionRuntimeMember) readBody() ([]byte, error) {
	return io.ReadAll(io.LimitReader(member.file, member.limit+1))
}

func (member *sessionRuntimeMember) rejoin(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if member.parent == nil {
		return nil
	}
	named, err := member.parent.root.Lstat(member.leaf)
	if member.file == nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("hermes inspect history appeared after original absence")
		}
		return nil
	}
	if err != nil || !sameSessionMember(member.info, named) {
		return fmt.Errorf("hermes original runtime history name changed")
	}
	if _, err := member.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	body, readErr := member.readBody()
	held, statErr := member.file.Stat()
	if readErr != nil || statErr != nil || !bytes.Equal(body, member.body) || !sameSessionMember(member.info, held) {
		return fmt.Errorf("hermes original held runtime history changed")
	}
	if err := validateSessionOwnedHandle(member.file, held); err != nil {
		return err
	}
	identity, identityErr := sessionPhysicalIdentity(member.file, held)
	named, nameErr := member.parent.root.Lstat(member.leaf)
	if identityErr != nil || nameErr != nil || identity != member.identity || !sameSessionMember(member.info, named) {
		return fmt.Errorf("hermes runtime history authority changed during read")
	}
	return ctx.Err()
}

func (state *sessionRuntimeSnapshot) rejoin(ctx context.Context) error {
	return state.rejoinOriginal(ctx, false)
}

func (state *sessionRuntimeSnapshot) rejoinExecution(ctx context.Context) error {
	return state.rejoinOriginal(ctx, true)
}

func (state *sessionRuntimeSnapshot) rejoinOriginal(ctx context.Context, ordinaryBootstrap bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, directory := range []*sessionHeldDirectory{state.root, state.logs} {
		if directory == nil {
			continue
		}
		if err := directory.rejoin(); err != nil {
			return err
		}
	}
	if state.logs == nil && !ordinaryBootstrap {
		if _, err := state.root.root.Lstat("logs"); !os.IsNotExist(err) {
			return fmt.Errorf("hermes runtime log parent absence changed")
		}
	}
	for _, member := range state.members {
		if member.parent == nil && ordinaryBootstrap {
			if _, err := state.root.root.Lstat(filepath.FromSlash(member.name)); !os.IsNotExist(err) {
				return fmt.Errorf("hermes inspect history appeared after original absence")
			}
		}
		if err := member.rejoin(ctx); err != nil {
			return err
		}
	}
	if state.passive != nil {
		if err := state.passive.rejoin(ctx); err != nil {
			return err
		}
	}
	if state.logs != nil {
		if err := state.logs.rejoin(); err != nil {
			return err
		}
	}
	return errors.Join(state.root.rejoin(), ctx.Err())
}

func (state *sessionRuntimeSnapshot) close() error {
	var result error
	for _, member := range state.members {
		if member.file != nil {
			result = errors.Join(result, member.file.Close())
		}
	}
	for _, directory := range []*sessionHeldDirectory{state.logs, state.root} {
		if directory != nil {
			result = errors.Join(result, directory.close())
		}
	}
	if state.passive != nil {
		result = errors.Join(result, state.passive.close())
	}
	return result
}

func (state *sessionRuntimeSnapshot) admitted() map[string]os.FileInfo {
	result := map[string]os.FileInfo{}
	for _, member := range state.members {
		if member.file != nil {
			result[member.name] = member.info
		}
	}
	return result
}

func (state *sessionRuntimeSnapshot) commitment(body []byte, selection sessionRuntimeSelection) (string, error) {
	fields := []string{"hermes-original-passive-source-v1", strconv.FormatBool(selection.inspect), state.root.path,
		state.root.identity, state.root.parentIdentity, hermesSessionDigest(body, selection.bundle)}
	logPresent := false
	for _, member := range state.members {
		seconds, nanos := "", ""
		if member.file != nil {
			seconds, nanos = strconv.FormatInt(member.info.ModTime().Unix(), 10), strconv.Itoa(member.info.ModTime().Nanosecond())
			logPresent = logPresent || member.parent == state.logs
		}
		fields = append(fields, member.name, strconv.FormatBool(member.file != nil), member.identity, seconds, nanos,
			strconv.Itoa(len(member.body)), fmt.Sprintf("%x", sha256.Sum256(member.body)))
	}
	if logPresent {
		fields = append(fields, state.logs.identity, state.logs.parentIdentity)
	}
	if state.passive != nil {
		fields = append(fields, state.passive.selectors...)
		for _, directory := range state.passive.directories {
			fields = append(fields, directory.path, directory.identity, directory.parentIdentity)
		}
		for _, missing := range state.passive.missing {
			fields = append(fields, missing.directory.path, missing.name)
		}
	}
	encoded, err := json.Marshal(fields)
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), err
}

func (state *sessionRuntimeSnapshot) validate(body []byte) error {
	if state.members[0].file != nil {
		if err := validateSessionLedger(state.members[0].body, state.passive.install); err != nil {
			return err
		}
	}
	if state.members[2].file != nil {
		var config struct {
			Servers map[string]map[string]any `yaml:"mcp_servers"`
		}
		if err := yaml.Unmarshal(body, &config); err != nil {
			return fmt.Errorf("hermes passive log lacks a selected configuration")
		}
		var names []string
		for name, server := range config.Servers {
			if command, ok := server["command"].(string); ok && command != "" {
				names = append(names, name)
			}
		}
		return validateSessionStdioLog(state.members[2].body, names)
	}
	return nil
}

func validateSessionLedger(body []byte, install string) error {
	if !utf8.Valid(body) {
		return fmt.Errorf("hermes passive registry is not UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return fmt.Errorf("hermes passive registry is malformed")
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("hermes passive registry has trailing content")
	}
	rows, ok := value.([]any)
	if !ok || len(rows) != 1 {
		return fmt.Errorf("hermes passive registry must contain one helper")
	}
	row, ok := rows[0].(map[string]any)
	fields := strings.Fields("pid create_time purpose install spawner_pid spawner_create registered_at argv host port profile hermes_home isolated boot_start spawner_boot boot_id")
	if !ok || len(row) != len(fields) {
		return fmt.Errorf("hermes passive registry schema differs")
	}
	for _, field := range fields {
		if _, present := row[field]; !present {
			return fmt.Errorf("hermes passive registry field is absent")
		}
	}
	if err := validateSessionLedgerIdentity(row, install); err != nil {
		return err
	}
	return validateSessionLedgerTimes(row)
}

func validateSessionLedgerIdentity(row map[string]any, install string) error {
	if !sessionLedgerPID(row["pid"]) || !sessionLedgerPID(row["spawner_pid"]) || row["purpose"] != "mcp-helper" ||
		row["install"] != install || row["host"] != "" || row["profile"] != "" || row["hermes_home"] != "" || row["port"] != nil || row["isolated"] != false {
		return fmt.Errorf("hermes passive registry purpose or selected install differs")
	}
	argv, ok := row["argv"].(string)
	if !ok || argv == "" || len(argv) > 4096 || strings.ContainsRune(argv, 0) {
		return fmt.Errorf("hermes passive registry argv is not bounded data")
	}
	return nil
}

func sessionLedgerPID(value any) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	pid, err := number.Int64()
	return err == nil && pid > 0 && pid < 1<<32
}

func sessionLedgerTime(value any, zero bool) bool {
	number, ok := value.(json.Number)
	if !ok || !strings.ContainsAny(string(number), ".eE") {
		return false
	}
	stamp, err := number.Float64()
	return err == nil && stamp < 1<<53 && (stamp > 0 || zero && stamp == 0)
}

func validateSessionLedgerTimes(row map[string]any) error {
	for _, key := range []string{"create_time", "spawner_create", "registered_at"} {
		if !sessionLedgerTime(row[key], false) {
			return fmt.Errorf("hermes passive registry epoch is invalid")
		}
	}
	for _, key := range []string{"boot_start", "spawner_boot"} {
		if row[key] != nil && (runtime.GOOS != "linux" || !sessionLedgerTime(row[key], true)) {
			return fmt.Errorf("hermes passive registry boot time is invalid")
		}
	}
	if row["boot_id"] != nil {
		boot, ok := row["boot_id"].(string)
		if !ok || runtime.GOOS != "linux" || !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`).MatchString(boot) {
			return fmt.Errorf("hermes passive registry boot identifier is invalid")
		}
	}
	return nil
}

func validateSessionStdioLog(body []byte, names []string) error {
	if len(body) == 0 || !utf8.Valid(body) {
		return fmt.Errorf("hermes passive stdio log is empty or invalid")
	}
	newline := "\n"
	if runtime.GOOS == "windows" {
		newline = "\r\n"
	}
	remaining := string(body)
	for remaining != "" {
		if !strings.HasPrefix(remaining, newline) || len(remaining) < len(newline)+23 {
			return fmt.Errorf("hermes passive stdio log contains child output")
		}
		remaining = remaining[len(newline):]
		stamp := remaining[:23]
		if _, err := time.Parse("2006-01-02 15:04:05,000", stamp); err != nil || stamp[:4] == "0000" ||
			!regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3}$`).MatchString(stamp) {
			return fmt.Errorf("hermes passive stdio log timestamp is invalid")
		}
		remaining = remaining[23:]
		matched := false
		for _, name := range names {
			header := " ===== starting MCP server '" + name + "' =====" + newline
			if strings.HasPrefix(remaining, header) {
				remaining, matched = strings.TrimPrefix(remaining, header), true
				break
			}
		}
		if !matched {
			return fmt.Errorf("hermes passive stdio log names an unowned server")
		}
	}
	return nil
}
