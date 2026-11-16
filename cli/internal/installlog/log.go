// Package installlog writes a durable, machine-readable record of an
// installation that did not fully succeed. The terminal shows a short cause;
// this file holds the full technical chain so a person or another agent can
// diagnose it later without the console being flooded.
package installlog

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// SchemaVersion identifies the JSON Lines record layout.
const SchemaVersion = 1

const (
	filePrefix = "install-"
	fileSuffix = ".jsonl"
	// Retain enough history to compare a failure with its retry, no more.
	maxFiles = 20
	// Bound any single technical detail so one pathological error cannot fill the disk.
	maxDetailBytes = 16 << 10
)

// Run describes one installation attempt.
type Run struct {
	Command     string
	Version     string
	Scope       string
	OperationID string
	// Attempt is 1 for the first try and increases for each retry in one session.
	Attempt int
	Entries []Entry
}

// Entry is one agent/package result. Detail may contain provider output and
// local paths; it never reaches the terminal.
type Entry struct {
	Alias   string `json:"alias,omitempty"`
	Client  string `json:"client"`
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Summary string `json:"summary,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Failed reports whether the run is worth recording.
func (run Run) Failed() bool {
	for _, entry := range run.Entries {
		switch entry.Status {
		case "failed", "not_attempted", "incomplete", "conflict", "recovery_required", string(installer.OutcomeCancelled):
			return true
		}
	}
	return false
}

type header struct {
	Type          string `json:"type"`
	SchemaVersion int    `json:"schema_version"`
	Time          string `json:"time"`
	Command       string `json:"command"`
	Version       string `json:"version,omitempty"`
	Scope         string `json:"scope,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
	Attempt       int    `json:"attempt"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
}

type record struct {
	Type string `json:"type"`
	Entry
}

// Write stores run under dir and returns the file path. An empty dir disables
// logging. It never overwrites an existing file and keeps the directory private.
func Write(dir string, run Run, now time.Time) (string, error) {
	if dir == "" {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create install log directory: %w", err)
	}
	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	name := filePrefix + now.UTC().Format("20060102T150405Z") + "-" + suffix + fileSuffix
	path := filepath.Join(dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create install log: %w", err)
	}
	encoder := json.NewEncoder(file)
	writeErr := encoder.Encode(header{
		Type: "run", SchemaVersion: SchemaVersion, Time: now.UTC().Format(time.RFC3339), Command: run.Command,
		Version: run.Version, Scope: run.Scope, OperationID: run.OperationID, Attempt: max(run.Attempt, 1),
		OS: runtime.GOOS, Arch: runtime.GOARCH,
	})
	for _, entry := range run.Entries {
		if writeErr != nil {
			break
		}
		entry.Detail = Sanitize(entry.Detail)
		writeErr = encoder.Encode(record{Type: "binding", Entry: entry})
	}
	writeErr = errors.Join(writeErr, file.Close())
	if writeErr != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write install log: %w", writeErr)
	}
	prune(dir)
	return path, nil
}

func randomSuffix() (string, error) {
	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("install log name: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// prune removes the oldest of our own files beyond the retention bound. Names
// sort chronologically, and only files this package could have created match.
func prune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && ownedLogName.MatchString(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for len(names) > maxFiles {
		_ = os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}

var (
	ownedLogName   = regexp.MustCompile(`^install-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{8}\.jsonl$`)
	urlCredentials = regexp.MustCompile(`(?i)(\b[a-z][a-z0-9+.-]*://)[^/\s:@]+(?::[^/\s@]*)?@`)
	// Any identifier that names a secret, in env, flag, YAML or JSON form:
	// MY_SERVICE_TOKEN=x, client_secret: x, "api_key": "x".
	secretAssign = regexp.MustCompile(`(?i)(["']?[a-z0-9_.-]*(?:token|secret|passw(?:or)?d|api[_-]?key|private[_-]?key|credential)[a-z0-9_.-]*["']?\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s"',;]+)`)
	secretFlag   = regexp.MustCompile(`(?i)(--?[a-z0-9-]*(?:token|secret|passw(?:or)?d|api-?key)\s+)([^\s-][^\s]*)`)
	authHeader   = regexp.MustCompile(`(?i)(["']?authorization["']?\s*[=:]\s*)("[^"]*"|'[^']*'|(?:(?:bearer|basic|token)\s+)?[^\s"',;]+)`)
	bearerValue  = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`)
	// Credentials recognizable by their issuer prefix even without a key name.
	knownTokens = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9_-]{16,}|npm_[A-Za-z0-9]{20,}|xox[abprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16})\b`)
)

// Sanitize removes credentials that commonly appear in provider output and
// bounds the size. Local paths are intentionally kept: they are what a
// diagnosing agent needs.
func Sanitize(detail string) string {
	detail = urlCredentials.ReplaceAllString(detail, "${1}***@")
	detail = authHeader.ReplaceAllString(detail, "${1}***")
	detail = secretAssign.ReplaceAllString(detail, "${1}***")
	detail = secretFlag.ReplaceAllString(detail, "${1}***")
	detail = bearerValue.ReplaceAllString(detail, "${1} ***")
	detail = knownTokens.ReplaceAllString(detail, "***")
	if len(detail) > maxDetailBytes {
		detail = detail[:maxDetailBytes] + "…[truncated]"
	}
	return detail
}
