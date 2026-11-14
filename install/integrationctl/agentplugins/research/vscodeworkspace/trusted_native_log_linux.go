//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"syscall"
	"time"
)

// Pinned e646: environmentService.logsHome/windowLogsPath, logger toResource,
// installedMcpServersDiscovery definition ID and spdlogLog's exact pattern.
var nativeLogSession = regexp.MustCompile(`^\d{8}T\d{6}$`)
var nativeLogWindow = regexp.MustCompile(`^window[1-9]\d{0,5}$`)
var nativeLogLabel = regexp.MustCompile(`^[a-z][a-z0-9-]{0,127}$`)
var nativeLogFrame = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}) \[info\] (.*)$`)

type nativeLogFacts struct {
	Status         string `json:"status"`
	TransportStart bool   `json:"transport_start"`
	Last           string `json:"last_state"`
	Stopped        int    `json:"stopped"`
	Starting       int    `json:"starting"`
	Running        int    `json:"running"`
	Error          int    `json:"error"`
}

// Private predicate names diagnose inert guards, never public artifact fields.
type nativeLogCheck string
type nativeLogCheckError struct{ code nativeLogCheck }

func (e nativeLogCheckError) Error() string      { return "native log authority refused" }
func nativeLogFailure(code nativeLogCheck) error { return nativeLogCheckError{code} }
func nativeLogCode(err error) nativeLogCheck {
	var refusal nativeLogCheckError
	if errors.As(err, &refusal) {
		return refusal.code
	}
	switch err.Error() {
	case "native log directory bound":
		return "member-bound"
	case "native log directory name":
		return "name"
	case "native log directory ambiguous":
		return "ambiguous"
	case "native log directory missing":
		return "missing"
	default:
		return "io"
	}
}
func nativeLogDifference(a, b os.FileInfo) nativeLogCheck {
	if a == nil || b == nil {
		return "stat"
	}
	left, ok := a.Sys().(*syscall.Stat_t)
	right, rok := b.Sys().(*syscall.Stat_t)
	if !ok || !rok {
		return "stat-type"
	}
	if left.Dev != right.Dev || left.Ino != right.Ino {
		return "identity"
	}
	if a.Mode() != b.Mode() {
		return "mode"
	}
	if a.Size() != b.Size() {
		return "size"
	}
	if !a.ModTime().Equal(b.ModTime()) {
		return "mtime"
	}
	if left.Nlink != right.Nlink {
		return "links"
	}
	if left.Ctim != right.Ctim {
		return "ctime"
	}
	if left.Uid != right.Uid {
		return "uid"
	}
	if left.Gid != right.Gid {
		return "gid"
	}
	if !os.SameFile(a, b) {
		return "identity-type"
	}
	return "none"
}
func unavailableNativeLog() nativeLogFacts {
	return nativeLogFacts{Status: "unavailable", Last: "none"}
}

// Unknown lines refuse the entire projection: multiline stderr/error suffixes
// must not inject apparently native state frames. Nothing from a suffix leaves.
func parseNativeLog(body []byte, label string) (nativeLogFacts, error) {
	out := unavailableNativeLog()
	if !nativeLogLabel.MatchString(label) || len(body) > 64<<10 || len(body) == 0 || body[len(body)-1] != '\n' {
		return out, fmt.Errorf("native log bound")
	}
	lines := bytes.Split(body[:len(body)-1], []byte{'\n'})
	if len(lines) > 255 {
		return out, fmt.Errorf("native log line bound")
	}
	for index, line := range lines {
		match := nativeLogFrame.FindSubmatch(line)
		if match == nil {
			return out, fmt.Errorf("native log frame")
		}
		if _, err := time.Parse("2006-01-02 15:04:05.000", string(match[1])); err != nil {
			return out, err
		}
		message := string(match[2])
		switch message {
		case "Starting server " + label:
			out.TransportStart = true
		case "Stopping server " + label, "Starting server from LocalProcess extension host":
		case "Connection state: Stopped":
			out.Last = "Stopped"
			out.Stopped++
		case "Connection state: Starting":
			out.Last = "Starting"
			out.Starting++
		case "Connection state: Running":
			out.Last = "Running"
			out.Running++
		default:
			if index != len(lines)-1 || !bytes.HasPrefix(match[2], []byte("Connection state: Error ")) || len(match[2]) > 4096 {
				return unavailableNativeLog(), fmt.Errorf("unadmitted native log message")
			}
			out.Last = "Error"
			out.Error++
		}
	}
	out.Status = "observed"
	return out, nil
}

type nativeLogBaseline struct {
	root         *os.Root
	profile      string
	original     os.FileInfo
	mount        uint64
	sessions     map[string]bool
	logsOriginal os.FileInfo
}

func boundedLogNames(root *os.Root) ([]string, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(17)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(names) > 16 {
		return nil, fmt.Errorf("native log directory bound")
	}
	return names, nil
}
func beginNativeLog(profile string, owner int) (*nativeLogBaseline, error) {
	info, err := exactDirectory(profile, owner)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(profile)
	if err != nil {
		return nil, err
	}
	held, err := root.Stat(".")
	if err != nil || !sameFile(info, held) {
		_ = root.Close()
		return nil, fmt.Errorf("native log original root changed")
	}
	b := &nativeLogBaseline{root: root, profile: profile, original: info, sessions: map[string]bool{}}
	b.mount, err = nativeLogRootMount(root)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	logs, err := holdNativeLogDirectory(root, "logs", info.Sys().(*syscall.Stat_t).Uid, info.Sys().(*syscall.Stat_t).Gid, info.Sys().(*syscall.Stat_t).Dev)
	if os.IsNotExist(err) {
		return b, nil
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	defer func() { _ = logs.root.Close() }()
	b.logsOriginal = logs.original
	names, err := boundedLogNames(logs.root)
	for _, name := range names {
		if !nativeLogSession.MatchString(name) {
			err = fmt.Errorf("native log session name")
			break
		}
		b.sessions[name] = true
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return b, nil
}

type nativeLogDirectory struct {
	parent   *os.Root
	name     string
	root     *os.Root
	original os.FileInfo
}

func holdNativeLogDirectory(parent *os.Root, name string, owner, group uint32, device uint64) (nativeLogDirectory, error) {
	d := nativeLogDirectory{parent: parent, name: name}
	info, err := parent.Lstat(name)
	if err != nil {
		return d, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || st.Uid != owner || st.Gid != group || st.Dev != device {
		switch {
		case !ok:
			return d, nativeLogFailure("stat-type")
		case !info.IsDir() || info.Mode()&os.ModeSymlink != 0:
			return d, nativeLogFailure("type")
		case st.Uid != owner:
			return d, nativeLogFailure("uid")
		case st.Gid != group:
			return d, nativeLogFailure("gid")
		default:
			return d, nativeLogFailure("device")
		}
	}
	d.original = info
	d.root, err = parent.OpenRoot(name)
	if err == nil {
		err = d.verify()
	}
	if err != nil && d.root != nil {
		_ = d.root.Close()
	}
	return d, err
}
func (d nativeLogDirectory) verify() error {
	named, err := d.parent.Lstat(d.name)
	if err != nil {
		return err
	}
	held, err := d.root.Stat(".")
	if err != nil || !sameFile(d.original, named) || !sameFile(d.original, held) {
		if err != nil {
			return nativeLogFailure("held-stat")
		}
		if code := nativeLogDifference(d.original, named); code != "none" {
			return nativeLogFailure("named-" + code)
		}
		return nativeLogFailure("held-" + nativeLogDifference(d.original, held))
	}
	return nil
}
func selectNativeLogName(root *os.Root, pattern *regexp.Regexp, prior map[string]bool, strict bool) (string, error) {
	names, err := boundedLogNames(root)
	if err != nil {
		return "", err
	}
	selected := ""
	for _, name := range names {
		if !pattern.MatchString(name) {
			if strict {
				return "", fmt.Errorf("native log directory name")
			}
			continue
		}
		if prior[name] {
			continue
		}
		if selected != "" {
			return "", fmt.Errorf("native log directory ambiguous")
		}
		selected = name
	}
	if selected == "" {
		return "", fmt.Errorf("native log directory missing")
	}
	return selected, nil
}
func (b *nativeLogBaseline) collectChecked(settled bool, deadline time.Time, label string) (nativeLogFacts, nativeLogCheck) {
	unavailable := unavailableNativeLog()
	if b == nil || !settled || !time.Now().Before(deadline) || !nativeLogLabel.MatchString(label) {
		return unavailable, "initial"
	}
	current, err := anchoredInfo(b.profile)
	st := b.original.Sys().(*syscall.Stat_t)
	if err != nil || !os.SameFile(current, b.original) || current.Mode() != b.original.Mode() {
		return unavailable, "profile"
	}
	now, ok := current.Sys().(*syscall.Stat_t)
	if !ok || now.Uid != st.Uid || now.Gid != st.Gid {
		return unavailable, "profile-owner"
	}
	logs, err := holdNativeLogDirectory(b.root, "logs", st.Uid, st.Gid, st.Dev)
	if err != nil {
		return unavailable, "logs-" + nativeLogCode(err)
	}
	defer func() { _ = logs.root.Close() }()
	if b.logsOriginal != nil && (!os.SameFile(b.logsOriginal, logs.original) || b.logsOriginal.Mode() != logs.original.Mode()) {
		return unavailable, "logs-original"
	}
	session, err := selectNativeLogName(logs.root, nativeLogSession, b.sessions, true)
	if err != nil {
		return unavailable, "session-name-" + nativeLogCode(err)
	}
	run, err := holdNativeLogDirectory(logs.root, session, st.Uid, st.Gid, st.Dev)
	if err != nil {
		return unavailable, "session-" + nativeLogCode(err)
	}
	defer func() { _ = run.root.Close() }()
	window, err := selectNativeLogName(run.root, nativeLogWindow, nil, false)
	if err != nil {
		return unavailable, "window-name-" + nativeLogCode(err)
	}
	view, err := holdNativeLogDirectory(run.root, window, st.Uid, st.Gid, st.Dev)
	if err != nil {
		return unavailable, "window-" + nativeLogCode(err)
	}
	defer func() { _ = view.root.Close() }()
	return b.readNativeLogChecked(deadline, label, current, logs, run, view)
}
func (b *nativeLogBaseline) readNativeLogChecked(deadline time.Time, label string, current os.FileInfo, logs, run, view nativeLogDirectory) (nativeLogFacts, nativeLogCheck) {
	unavailable := unavailableNativeLog()
	st := b.original.Sys().(*syscall.Stat_t)
	name := "mcpServer.mcp.config.ws0." + label + ".log"
	leaf, reason := holdNativeLogLeaf(view.root, name, st.Uid, st.Gid)
	if reason != "none" {
		return unavailable, reason
	}
	defer leaf.close()
	verify := func() bool {
		if !b.sameNativeLogMounts(leaf.parent, leaf.file) {
			reason = "leaf-mount"
			return false
		}
		reason = nativeLogFileJoins(b.profile, current, leaf.original, view, name, leaf.file)
		if reason != "none" {
			return false
		}
		for _, join := range []struct {
			prefix    nativeLogCheck
			directory nativeLogDirectory
		}{{"logs-", logs}, {"session-", run}, {"window-", view}} {
			if err := join.directory.verify(); err != nil {
				reason = join.prefix + nativeLogCode(err)
				return false
			}
		}
		if !time.Now().Before(deadline) {
			reason = "deadline"
			return false
		}
		return true
	}
	reason = "body"
	body, err := readNativeLogBody(leaf.file, leaf.original.Size(), verify)
	if err != nil {
		if reason == "body" || reason == "none" {
			reason = "body-" + nativeLogCode(err)
		}
		return unavailable, reason
	}
	reason = "parser-or-deadline"
	out, err := parseNativeLog(body, label)
	if err != nil || !time.Now().Before(deadline) {
		return unavailable, reason
	}
	return out, "none"
}

func nativeLogFileJoins(profilePath string, current, original os.FileInfo, view nativeLogDirectory, name string, file *os.File) nativeLogCheck {
	named, e := view.root.Lstat(name)
	held, he := file.Stat()
	profile, pe := anchoredInfo(profilePath)
	if e != nil || he != nil || pe != nil {
		return "verify-stat"
	}
	for _, join := range []struct {
		prefix nativeLogCheck
		a, b   os.FileInfo
	}{{"profile-", current, profile}, {"leaf-named-", original, named}, {"leaf-held-", original, held}} {
		if !sameFile(join.a, join.b) {
			return join.prefix + nativeLogDifference(join.a, join.b)
		}
	}

	return "none"
}

// Only fixed strings are exported; no source log bytes, labels or paths.
func (b *nativeLogBaseline) close() {
	if b != nil {
		_ = b.root.Close()
	}
}

func readNativeLogBody(reader io.Reader, expected int64, verify func() bool) ([]byte, error) {
	if !verify() {
		return nil, nativeLogFailure("pre-read")
	}
	body, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil {
		return nil, nativeLogFailure("read")
	}
	if int64(len(body)) != expected || len(body) > 64<<10 {
		return nil, nativeLogFailure("size")
	}
	if !verify() {
		return nil, nativeLogFailure("post-read")
	}
	return body, nil
}

var nativeLogRefusalCode = regexp.MustCompile(`^(initial|profile|profile-owner|logs-original|leaf-stat-type-link-size|leaf-owner-device|leaf-mount|leaf-open|verify-stat|deadline|parser-or-deadline|body-(pre-read|read|size|post-read|io)|(session|window)-name-(member-bound|name|ambiguous|missing|io)|(profile|leaf-named|leaf-held)-(stat|stat-type|identity|mode|size|mtime|links|ctime|uid|gid|identity-type|none)|(logs|session|window)-(io|stat-type|type|uid|gid|device|held-stat|(named|held)-(stat|stat-type|identity|mode|size|mtime|links|ctime|uid|gid|identity-type|none)))$`)

func nativeLogRefusalFrame(code nativeLogCheck) []byte {
	if len(code) > 96 || !nativeLogRefusalCode.MatchString(string(code)) {
		return nil
	}
	return []byte("UAP_VSCODE_NATIVE_LOG_REFUSAL_V1 " + code + "\n")
}
