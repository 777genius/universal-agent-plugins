//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type qoderLogRunObservation struct {
	Status string           `json:"status"`
	Run    *qoderLogRunData `json:"run,omitempty"`
}

type qoderLogRunData struct {
	RunID                    string `json:"run_id"`
	LatestTarget             string `json:"latest_target"`
	ManifestBytes            int    `json:"manifest_bytes"`
	ManifestSHA256           string `json:"manifest_sha256"`
	PID                      int64  `json:"pid"`
	PPID                     int64  `json:"ppid"`
	StartedAt                string `json:"started_at"`
	CLIVersion               string `json:"cli_version"`
	Platform                 string `json:"platform"`
	Arch                     string `json:"arch"`
	ProcessRole              string `json:"process_role"`
	PIDProvenance            string `json:"pid_provenance"`
	RunIDMatchesManifest     bool   `json:"run_id_matches_manifest"`
	CWDMatches               bool   `json:"cwd_matches"`
	ArgvMatches              bool   `json:"argv_matches"`
	ExactAddedRunClosure     bool   `json:"exact_added_run_closure"`
	PreexistingRecordsStable bool   `json:"preexisting_records_stable"`
}

var qoderLogRunID = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{3}(?:Z|[+-][0-9]{2}-[0-9]{2}))-([0-9a-z]{0,6})-p([1-9][0-9]{0,9})$`)

const qoderLogTimeLayout = "2006-01-02T15:04:05.000Z07:00"

func qoderLogTimestamp(text string) bool {
	parsed, err := time.Parse(qoderLogTimeLayout, text)
	return err == nil && parsed.Format(qoderLogTimeLayout) == text && (len(text) != 29 || text[24:26] < "24")
}

func qoderLogRunParts(id string) (string, int64, bool) {
	if len(id) > 64 {
		return "", 0, false
	}
	parts := qoderLogRunID.FindStringSubmatch(id)
	if len(parts) != 4 {
		return "", 0, false
	}
	stamp := []byte(parts[1])
	stamp[13], stamp[16], stamp[19] = ':', ':', '.'
	if len(stamp) == 29 {
		stamp[26] = ':'
	}
	pid, pidErr := strconv.ParseInt(parts[3], 10, 32)
	return string(stamp), pid, qoderLogTimestamp(string(stamp)) && pidErr == nil && pid > 0
}

func qoderLogRunObject(value any, keys string) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	want := strings.Fields(keys)
	if !ok || len(object) != len(want) {
		return nil, false
	}
	for _, key := range want {
		if _, present := object[key]; !present {
			return nil, false
		}
	}
	return object, true
}

func qoderLogRunInteger(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(string(number), 10, 32)
	return parsed, err == nil && parsed > 0 && strconv.FormatInt(parsed, 10) == string(number)
}

func qoderLogRunManifest(body []byte, runID, cwd string, argv []string) (*qoderLogRunData, bool) {
	if len(body) == 0 || len(body) > 4096 {
		return nil, false
	}
	value, err := projectNativeRuntimeJSON(body)
	object, ok := qoderLogRunObject(value, "run_id started_at pid ppid process_role cli_version node_version platform arch cwd project_id argv env_flags log_files")
	if err != nil || !ok {
		return nil, false
	}
	for _, key := range strings.Fields("run_id started_at process_role cli_version node_version platform arch cwd project_id") {
		text, valid := object[key].(string)
		if !valid || text == "" || len(text) > 4096 || strings.ContainsRune(text, 0) {
			return nil, false
		}
	}
	stamp, _, validID := qoderLogRunParts(object["run_id"].(string))
	pid, validPID := qoderLogRunInteger(object["pid"])
	ppid, validPPID := qoderLogRunInteger(object["ppid"])
	flags, validFlags := qoderLogRunObject(object["env_flags"], "debug remote_worker ci")
	logs, validLogs := qoderLogRunObject(object["log_files"], "process")
	if !validID || !qoderLogTimestamp(object["started_at"].(string)) || !validPID || !validPPID || !validFlags || !validLogs ||
		object["cli_version"] != "1.1.78" || object["platform"] != "linux" || object["arch"] != "x64" || object["process_role"] != "main" || logs["process"] != "qodercli.log" {
		return nil, false
	}
	for _, flag := range flags {
		if _, valid := flag.(bool); !valid {
			return nil, false
		}
	}
	arguments, valid := object["argv"].([]any)
	if !valid || len(arguments) == 0 || len(arguments) > 32 {
		return nil, false
	}
	actualArgv := make([]string, len(arguments))
	for index, argument := range arguments {
		text, valid := argument.(string)
		if !valid || len(text) > 4096 || strings.ContainsRune(text, 0) {
			return nil, false
		}
		actualArgv[index] = text
	}
	_, runPID, _ := qoderLogRunParts(runID)
	return &qoderLogRunData{RunID: runID, LatestTarget: "runs/" + runID, ManifestBytes: len(body), ManifestSHA256: qoderDeltaHash(string(body)),
		PID: pid, PPID: ppid, StartedAt: object["started_at"].(string), CLIVersion: "1.1.78", Platform: "linux", Arch: "x64", ProcessRole: "main",
		PIDProvenance: "manifest-only", RunIDMatchesManifest: object["run_id"] == runID && stamp == object["started_at"] && runPID == pid,
		CWDMatches: object["cwd"] == cwd, ArgvMatches: reflect.DeepEqual(actualArgv, argv)}, true
}

func qoderLogRunRead(root string, link qoderLogLinkSnapshot, expected string) (body []byte, err error) {
	directory, err := qoderLogOpenRoot(filepath.Join(root, link.TargetRel))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	held, err := directory.Stat()
	if err != nil || !projectNativeRuntimeStableStat(link.Target.Sys(), held.Sys()) {
		return nil, fmt.Errorf("run directory identity")
	}
	fd, err := unix.Openat(int(directory.Fd()), "manifest.json", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "qoder-held-run-manifest")
	defer func() { err = errors.Join(err, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !qoderLogOwned(before, false) || before.Mode() != 0644 || before.Size() <= 0 || before.Size() > 4096 {
		return nil, fmt.Errorf("manifest identity")
	}
	body, err = io.ReadAll(io.LimitReader(file, 4097))
	after, statErr := file.Stat()
	if err != nil || statErr != nil || len(body) > 4096 || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) || expected != fmt.Sprintf("file:%o:%d:%s", before.Mode(), len(body), qoderDeltaHash(string(body))) {
		return nil, fmt.Errorf("manifest held inventory")
	}
	linked, err := qoderLogOpenAt(directory, "manifest.json", false)
	if err != nil {
		return nil, err
	}
	current, statErr := linked.Stat()
	closeErr := linked.Close()
	if statErr != nil || closeErr != nil || !projectNativeRuntimeStableStat(before.Sys(), current.Sys()) {
		return nil, fmt.Errorf("manifest linked identity")
	}
	return body, nil
}

func qoderObserveLogRun(ctx context.Context, root, cwd string, argv []string, before, after map[string]string) qoderLogRunObservation {
	refused := func(status string) qoderLogRunObservation { return qoderLogRunObservation{Status: status} }
	if ctx == nil || ctx.Err() != nil {
		return refused("context-refused")
	}
	if len(before) > 4096 || len(after) > 4096 {
		return refused("profile-refused")
	}
	info, err := os.Lstat(filepath.Join(root, "logs", "latest"))
	if err != nil {
		return refused("profile-refused")
	}
	link, err := qoderReadOriginalLogLink(root, info)
	if err != nil || link.Record != after["logs/latest"] || !link.Target.IsDir() || link.Target.Mode() != os.ModeDir|0755 {
		return refused("profile-refused")
	}
	parts := strings.Split(link.TargetRel, "/")
	if len(parts) != 3 || parts[0] != "logs" || parts[1] != "runs" {
		return refused("layout-refused")
	}
	runID := parts[2]
	if _, _, valid := qoderLogRunParts(runID); !valid {
		return refused("layout-refused")
	}
	for key := range before {
		if key == link.TargetRel || strings.HasPrefix(key, link.TargetRel+"/") {
			return refused("layout-refused")
		}
	}
	manifest := filepath.Join(link.TargetRel, "manifest.json")
	body, err := qoderLogRunRead(root, link, after[manifest])
	if err != nil {
		return refused("manifest-refused")
	}
	current, err := qoderReadOriginalLogLink(root, info)
	if ctx.Err() != nil {
		return refused("context-refused")
	}
	if err != nil || current.Record != link.Record || !projectNativeRuntimeStableStat(link.Profile.Sys(), current.Profile.Sys()) || !projectNativeRuntimeStableStat(link.Logs.Sys(), current.Logs.Sys()) {
		return refused("profile-refused")
	}
	data, valid := qoderLogRunManifest(body, runID, cwd, argv)
	if !valid {
		return refused("manifest-refused")
	}
	data.PreexistingRecordsStable = true
	for key, record := range before {
		if key != "logs/latest" && after[key] != record {
			data.PreexistingRecordsStable = false
		}
	}
	added := map[string]bool{}
	for key := range after {
		if _, exists := before[key]; !exists {
			added[key] = true
		}
	}
	expected := map[string]bool{link.TargetRel: true, manifest: true, filepath.Join(link.TargetRel, "qodercli.log"): true}
	log, validLog := qoderDeltaValue(after[filepath.Join(link.TargetRel, "qodercli.log")], true, false)
	data.ExactAddedRunClosure = reflect.DeepEqual(added, expected) && after[link.TargetRel] == "directory:20000000755" && validLog && log.Kind == "file" && log.Mode == "644"
	observation := qoderLogRunObservation{Status: "captured", Run: data}
	if raw, err := json.Marshal(observation); err != nil || len(raw) > 2048 {
		return refused("observation-bound")
	}
	return observation
}
