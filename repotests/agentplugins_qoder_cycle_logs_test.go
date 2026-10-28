//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

type qoderCycleLogs struct {
	ctx                context.Context
	root               string
	original, expected map[string]string
	held               map[string]os.FileInfo
	runs               []string
}

type qoderCycleLogTransition struct {
	Kind         string             `json:"kind"`
	Before       string             `json:"before_sha256"`
	After        string             `json:"after_sha256"`
	Form         string             `json:"argv_form,omitempty"`
	LatestRecord string             `json:"latest_record,omitempty"`
	Run          *qoderLogRunData   `json:"run,omitempty"`
	Delta        *qoderProfileDelta `json:"delta,omitempty"`
	LogBytes     int64              `json:"log_bytes,omitempty"`
	LogSHA256    string             `json:"log_sha256,omitempty"`
}

func qoderCycleInventoryHash(value map[string]string) string {
	raw, _ := json.Marshal(value)
	return qoderDeltaHash(string(raw))
}

func newQoderCycleLogs(ctx context.Context, root string, recipe map[string]string) (*qoderCycleLogs, error) {
	actual, err := projectNativeCLIQoderProfile(root)
	if err != nil || ctx == nil || ctx.Err() != nil || !reflect.DeepEqual(actual, recipe) {
		return nil, fmt.Errorf("Qoder original profile differs from pre-authored closed recipe")
	}
	if _, err := os.Lstat(filepath.Join(root, "logs")); !os.IsNotExist(err) {
		return nil, fmt.Errorf("Qoder logs must be absent before any native child")
	}
	l := &qoderCycleLogs{ctx: ctx, root: root, original: map[string]string{}, expected: map[string]string{}, held: map[string]os.FileInfo{}, runs: []string{}}
	for path, record := range recipe {
		l.original[path], l.expected[path] = record, record
	}
	return l, nil
}

func (l *qoderCycleLogs) check() error {
	if l.ctx.Err() != nil {
		return fmt.Errorf("Qoder log authority deadline")
	}
	actual, err := projectNativeCLIQoderProfile(l.root)
	if err != nil || !reflect.DeepEqual(actual, l.expected) {
		return fmt.Errorf("Qoder original/admitted log inventory changed")
	}
	for path, before := range l.held {
		after, err := os.Lstat(filepath.Join(l.root, path))
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return fmt.Errorf("Qoder admitted old log identity changed")
		}
		a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
		if a.Uid != b.Uid || a.Gid != b.Gid || before.Mode().IsRegular() && b.Nlink != 1 {
			return fmt.Errorf("Qoder admitted old log ownership changed")
		}
	}
	return nil
}

// The embedded path stays private; the normalized argument authority is exact.
func qoderCycleArgvForm(actual, expected []string) string {
	if len(expected) == 0 {
		return "refused"
	}
	if reflect.DeepEqual(actual, expected) {
		return "direct"
	}
	if len(actual) != len(expected)+1 || actual[0] != expected[0] || !reflect.DeepEqual(actual[2:], expected[1:]) {
		return "refused"
	}
	embedded := actual[1]
	if len(embedded) > 4096 || !strings.HasPrefix(embedded, "/$bunfs/") || !filepath.IsAbs(embedded) || filepath.Clean(embedded) != embedded {
		return "refused"
	}
	for _, char := range embedded {
		if char < 32 || char == 127 {
			return "refused"
		}
	}
	return "compiled-embedded"
}

func (l *qoderCycleLogs) unchanged() (qoderCycleLogTransition, error) {
	before := qoderCycleInventoryHash(l.expected)
	if err := l.check(); err != nil {
		return qoderCycleLogTransition{Kind: "refused", Before: before}, err
	}
	return qoderCycleLogTransition{Kind: "unchanged", Before: before, After: before}, nil
}

func (l *qoderCycleLogs) native(cwd string, argv []string, start, end time.Time) (qoderCycleLogTransition, error) {
	transition := qoderCycleLogTransition{Kind: "refused", Before: qoderCycleInventoryHash(l.expected)}
	actual, err := projectNativeCLIQoderProfile(l.root)
	if err != nil || l.ctx.Err() != nil {
		return transition, fmt.Errorf("Qoder native complete log inventory refused")
	}
	info, err := os.Lstat(filepath.Join(l.root, "logs", "latest"))
	if err != nil {
		return transition, err
	}
	link, err := qoderReadOriginalLogLink(l.root, info)
	if err != nil || link.Record != actual["logs/latest"] || link.Target.Mode() != os.ModeDir|0755 {
		return transition, fmt.Errorf("Qoder latest does not join the held source-defined run")
	}
	parts := strings.Split(link.TargetRel, "/")
	if len(parts) != 3 || parts[0] != "logs" || parts[1] != "runs" {
		return transition, fmt.Errorf("Qoder source run path")
	}
	runID := parts[2]
	for _, old := range l.runs {
		if old == runID {
			return transition, fmt.Errorf("Qoder log run identity reused")
		}
	}
	for path := range l.expected {
		if path == link.TargetRel || strings.HasPrefix(path, link.TargetRel+"/") {
			return transition, fmt.Errorf("Qoder source run prefix preexists")
		}
	}
	manifest := filepath.Join(link.TargetRel, "manifest.json")
	body, err := qoderLogRunRead(l.root, link, actual[manifest])
	if err != nil {
		return transition, err
	}
	data, valid := qoderLogRunManifest(body, runID, cwd, argv)
	if !valid || !data.RunIDMatchesManifest || !data.CWDMatches {
		return transition, fmt.Errorf("Qoder complete manifest source/cwd/run identity")
	}
	stamp, err := time.Parse(qoderLogTimeLayout, data.StartedAt)
	if err != nil || end.Before(start) || end.Sub(start) > 35*time.Second || stamp.Before(start.Truncate(time.Millisecond)) || stamp.After(end) {
		return transition, fmt.Errorf("Qoder run timestamp does not belong to this command interval")
	}
	value, _ := projectNativeRuntimeJSON(body)
	packet := value.(map[string]any)["argv"].([]any)
	args := make([]string, len(packet))
	for i, arg := range packet {
		args[i] = arg.(string)
	}
	transition.Form = qoderCycleArgvForm(args, argv)
	transition.Run = data
	if transition.Form == "refused" {
		return transition, fmt.Errorf("Qoder manifest has an unmeasured private argv form")
	}
	logPath := filepath.Join(link.TargetRel, "qodercli.log")
	log, valid := qoderDeltaValue(actual[logPath], true, false)
	if !valid || log.Kind != "file" || log.Mode != "644" || log.Bytes == nil || *log.Bytes <= 0 {
		return transition, fmt.Errorf("Qoder source log ordinary mode")
	}
	if err := qoderCycleLogLeaf(l.root, logPath, actual[logPath]); err != nil {
		return transition, err
	}
	expected := map[string]string{}
	for path, record := range l.expected {
		expected[path] = record
	}
	first := len(l.runs) == 0
	if first {
		expected["logs"], expected["logs/runs"] = "directory:20000000755", "directory:20000000755"
	}
	expected[link.TargetRel] = "directory:20000000755"
	expected[manifest] = fmt.Sprintf("file:644:%d:%s", data.ManifestBytes, data.ManifestSHA256)
	expected[logPath] = actual[logPath]
	expected["logs/latest"] = link.Record
	if !reflect.DeepEqual(actual, expected) {
		return transition, fmt.Errorf("Qoder native changed a foreign/preexisting/unrecorded profile entry")
	}
	again, err := qoderReadOriginalLogLink(l.root, info)
	if err != nil || again.Record != link.Record || !projectNativeRuntimeStableStat(link.Profile.Sys(), again.Profile.Sys()) || !projectNativeRuntimeStableStat(link.Logs.Sys(), again.Logs.Sys()) {
		return transition, fmt.Errorf("Qoder held log link/profile changed during classification")
	}
	delta := qoderOriginalProfileDelta(l.expected, actual)
	data.ExactAddedRunClosure, data.PreexistingRecordsStable = true, true
	transition.Kind, transition.After, transition.Delta = "one-run", qoderCycleInventoryHash(expected), &delta
	transition.LogBytes, transition.LogSHA256 = *log.Bytes, log.ContentSHA256
	transition.LatestRecord = link.Record
	// Only this independently validated recipe is added, never an observed map.
	l.expected = expected
	l.runs = append(l.runs, runID)
	held := []string{link.TargetRel, manifest, logPath}
	if first {
		held = append(held, "logs", "logs/runs")
	}
	for _, path := range held {
		record, err := os.Lstat(filepath.Join(l.root, path))
		if err != nil {
			return transition, err
		}
		l.held[path] = record
	}
	if err := l.check(); err != nil {
		return transition, err
	}
	return transition, nil
}

func qoderCycleLogLeaf(root, relative, expected string) (err error) {
	directory, err := qoderLogOpenRoot(filepath.Join(root, filepath.Dir(relative)))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	file, err := qoderLogOpenAt(directory, filepath.Base(relative), false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !qoderLogOwned(info, false) || info.Mode() != 0644 {
		return fmt.Errorf("Qoder log held ordinary ownership")
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(root, relative), 64<<20)
	after, statErr := file.Stat()
	if err != nil || statErr != nil || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) || expected != fmt.Sprintf("file:644:%d:%s", len(raw), qoderDeltaHash(string(raw))) {
		return fmt.Errorf("Qoder log leaf does not join the held inventory")
	}
	linked, err := qoderLogOpenAt(directory, filepath.Base(relative), false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, linked.Close()) }()
	current, err := linked.Stat()
	if err != nil || !projectNativeRuntimeStableStat(info.Sys(), current.Sys()) {
		return fmt.Errorf("Qoder log linked identity changed")
	}
	return nil
}
