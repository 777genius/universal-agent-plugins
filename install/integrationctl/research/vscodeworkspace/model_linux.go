//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const publisherVersion = "1.143.1"
const publisherCommit = "e646868227c3e3a9e3f24277c8c7220be42c0be7"
const publisherPrefix = "VSCode-linux-x64"
const rgExecutable = "resources/app/node_modules.asar.unpacked/@vscode/ripgrep-universal/bin/linux-x64/rg"
const streamLimit = 1 << 20
const controlLimit = 64 << 10
const resultLimit = 512 << 10
const artifactLimit = 32 << 20
const display = ":93"

var buildCommit string
var probeLabels = []string{"A", "B", "A-restart", "A-after-removal"}
var requiredPublisher = []string{"code", "resources/app/out/cli.js", "resources/app/product.json", rgExecutable}
var observerPins = map[string]string{
	"package.json":  "441f882bfafd76e8e71e9ab59803f51a1e4b2890fb32f993f8298440fa992cb2",
	"extension.cjs": "4c66f495dfe950ae2b32dda642f799ff3a53f319bc56d5149ed2fb850e232574",
	"shape.cjs":     "78a0046fe261156849f03ceb1cb5cc7036e3666400996fbe281b9b3301513802",
	"runner.cjs":    "4858024108996491b5b1701c219e77e56a41013de1dc1feff2727ad5023e711c",
}
var artifactName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$")

type inputRequest struct {
	Mode         string        `json:"mode,omitempty"`
	Trusted      *trustedInput `json:"trusted,omitempty"`
	UID          int           `json:"uid"`
	GID          int           `json:"gid"`
	EUID         int           `json:"euid"`
	EGID         int           `json:"egid"`
	StartNS      int64         `json:"start_monotonic_ns"`
	SourceCommit string        `json:"source_commit"`
	SourceTree   string        `json:"source_tree,omitempty"`
	ObserverRoot string        `json:"observer_root"`
	OutputRoot   string        `json:"output_root"`
	HelperSHA    string        `json:"helper_sha256"`
}
type sealedRequest struct {
	Input            inputRequest      `json:"input"`
	Base             string            `json:"base"`
	ExecutableHashes map[string]string `json:"executable_hashes"`
}
type isolationProof struct {
	PID              int      `json:"pid"`
	UID              int      `json:"uid"`
	GID              int      `json:"gid"`
	EUID             int      `json:"euid"`
	EGID             int      `json:"egid"`
	Namespace        uint64   `json:"network_namespace"`
	ParentNamespace  uint64   `json:"parent_network_namespace"`
	Groups           []int    `json:"groups"`
	Interfaces       []string `json:"interfaces"`
	CapabilitiesZero bool     `json:"capabilities_zero"`
	NoNewPrivileges  bool     `json:"no_new_privileges"`
	LoopbackReady    bool     `json:"loopback_ready,omitempty"`
}
type childRecord struct {
	Label          string         `json:"label"`
	Argv           []string       `json:"argv"`
	Cwd            string         `json:"cwd"`
	Proof          isolationProof `json:"isolation"`
	Diagnostics    string         `json:"diagnostics"`
	MergedFile     string         `json:"merged_file"`
	MergedBytes    int            `json:"merged_bytes"`
	MergedSHA      string         `json:"merged_sha256"`
	MergedOverflow bool           `json:"merged_overflow"`
	PlannedStop    bool           `json:"planned_stop"`
	TreeReaped     bool           `json:"tree_reaped"`
	Error          string         `json:"error"`
}
type strictRunner interface {
	RunDuplexWithPlannedShutdown(context.Context, ports.Command, func(io.Writer, io.Reader) error) error
}
type controller struct {
	request              sealedRequest
	deadline             time.Time
	work                 context.Context
	cancelWork           context.CancelFunc
	runner               strictRunner
	files                map[string][]byte
	bytes                int
	output               *os.File
	hashes               map[string]string
	parentNS             uint64
	cookie               string
	children             []childRecord
	observations         []map[string]any
	displayAssets        map[string]heldDisplayAsset
	createdDisplayParent os.FileInfo
	displayCleanupErr    error
	trustedState         *trustedLifecycle
}

func deadlineAt(start int64, limit time.Duration) (time.Time, error) {
	return deadlineAtMaximum(start, limit, 480*time.Second)
}
func deadlineAtMaximum(start int64, limit, maximum time.Duration) (time.Time, error) {
	var now unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &now); err != nil {
		return time.Time{}, err
	}
	ns := now.Nano()
	if start <= 0 || start > ns || ns-start > int64(maximum) || limit <= 0 || limit > maximum {
		return time.Time{}, fmt.Errorf("monotonic task budget authority refused")
	}
	return time.Now().Add(limit - time.Duration(ns-start)), nil
}
func encode(value any, limit int) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if len(body) > limit {
		return nil, fmt.Errorf("encoded evidence exceeds bound")
	}
	return body, nil
}
func clipped(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > 2048 {
		return text[:2048]
	}
	return text
}
func (c *controller) keep(name string, body []byte) error {
	_, exists := c.files[name]
	if !artifactName.MatchString(name) || (!strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".log")) || exists || len(c.files) >= 64 || len(body) > 1<<20 || c.bytes+len(body) > artifactLimit-(64<<10) {
		return fmt.Errorf("bounded flat evidence admission refused")
	}
	c.files[name] = append([]byte(nil), body...)
	c.bytes += len(body)
	return nil
}
