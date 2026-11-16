//go:build linux

package process

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const readonlyPolicyLimit = 64 << 10
const readonlyPolicySeals = unix.F_SEAL_SEAL | unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE
const readonlyBootstrapEnvironment = "PLUGIN_KIT_AI_READONLY_DUPLEX_BOOTSTRAP=1"

type readonlyPolicyObject struct {
	Path  string
	Facts readonlyObjectFacts
}

type readonlyPolicyDevice struct {
	Facts readonlyObjectFacts
	Rdev  uint64
}

// This is a private sealed, single-invocation message. It is neither a journal
// nor a portable receipt and cannot qualify the caller's discovery closure.
type readonlyPolicy struct {
	Version                             uint32
	Nonce                               string
	Deadline                            int64
	UID, GID, OverflowUID, OverflowGID  uint32
	Groups                              []uint32
	Namespaces                          [7]uint64
	Home, Worktree, Project, Executable readonlyPolicyObject
	Runtime                             [4]readonlyPolicyObject
	Devices                             [6]readonlyPolicyDevice
	Self, Primitive                     readonlyObjectFacts
	SelfSHA, PrimitiveSHA, NativeSHA    string
	Input, RuntimeProof                 readonlyCensusProof
	Argv, Env                           []string
	Dir                                 string
}

func readonlyHex(value string, size int) bool {
	if len(value) != size || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func readonlyMonotonic() (int64, error) {
	var stamp unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &stamp); err != nil {
		return 0, err
	}
	if stamp.Sec < 0 || stamp.Nsec < 0 || stamp.Nsec >= 1e9 || stamp.Sec > (math.MaxInt64-stamp.Nsec)/1e9 {
		return 0, errors.New("readonly original monotonic clock differs")
	}
	return stamp.Sec*1e9 + stamp.Nsec, nil
}

func readonlyOriginalDeadline(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	deadline, exists := ctx.Deadline()
	if !exists {
		return 0, errors.New("readonly exchange requires its original finite deadline")
	}
	now, err := readonlyMonotonic()
	if err != nil {
		return 0, err
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 15*time.Second || int64(remaining) > math.MaxInt64-now {
		return 0, errors.New("readonly original deadline is outside its existing bound")
	}
	// Sampling remaining after CLOCK_MONOTONIC can only shorten this cutoff.
	return now + int64(remaining), nil
}

func (policy readonlyPolicy) validate() error {
	if policy.Version != 1 || !readonlyHex(policy.Nonce, 32) || policy.Deadline <= 0 || policy.UID == 0 || policy.GID == 0 ||
		policy.OverflowUID == 0 || policy.OverflowGID == 0 || policy.OverflowUID == policy.UID || policy.OverflowGID == policy.GID {
		return errors.New("readonly private policy identity differs")
	}
	for _, identity := range policy.Namespaces {
		if identity == 0 {
			return errors.New("readonly private namespace identity is absent")
		}
	}
	if len(policy.Groups) > 64 {
		return errors.New("readonly original supplementary groups exceed their bound")
	}
	if err := policy.validateObjects(); err != nil {
		return err
	}
	if err := policy.validateCommand(); err != nil {
		return err
	}
	if err := policy.validateProofs(); err != nil {
		return err
	}
	return policy.validateDevices()
}

func (policy readonlyPolicy) validateProofs() error {
	if !readonlyHex(policy.SelfSHA, 64) || !readonlyHex(policy.PrimitiveSHA, 64) || !readonlyHex(policy.NativeSHA, 64) ||
		!readonlyHex(policy.Input.SHA256, 64) || !readonlyHex(policy.RuntimeProof.SHA256, 64) ||
		policy.Input.Count <= 0 || policy.Input.Count > readonlyInputEntryLimit ||
		policy.RuntimeProof.Count <= 0 || policy.RuntimeProof.Count > readonlyRuntimeEntryLimit {
		return errors.New("readonly private policy original proofs differ")
	}
	return nil
}

func (policy readonlyPolicy) validateObjects() error {
	for _, object := range []readonlyPolicyObject{policy.Home, policy.Worktree, policy.Project} {
		if !readonlyAbsolute(object.Path) || readonlyWithin(object.Path, "/usr") || readonlyWithin("/usr", object.Path) {
			return errors.New("readonly input and runtime roles overlap")
		}
		if err := admitReadonlyFacts(object.Facts, true, policy.UID, policy.GID); err != nil {
			return err
		}
	}
	if !readonlyWithin(policy.Project.Path, policy.Worktree.Path) || policy.Dir != policy.Project.Path {
		return errors.New("readonly private project role differs")
	}
	for i, object := range policy.Runtime {
		if !readonlyAbsolute(object.Path) || !readonlyWithin(object.Path, "/usr") || i == 0 && object.Path != "/usr" {
			return errors.New("readonly private runtime role differs")
		}
		if err := admitReadonlyFacts(object.Facts, true, 0, 0); err != nil {
			return err
		}
	}
	return policy.validateExecutables()
}

func (policy readonlyPolicy) validateExecutables() error {
	object := policy.Executable
	uid, gid := policy.UID, policy.GID
	if readonlyWithin(object.Path, "/usr") {
		uid, gid = 0, 0
	}
	if !readonlyAbsolute(object.Path) || object.Facts.Mode&0111 == 0 || policy.Self.Mode&0111 == 0 {
		return errors.New("readonly private executable role differs")
	}
	if err := admitReadonlyFacts(object.Facts, false, uid, gid); err != nil {
		return err
	}
	if err := admitReadonlyFacts(policy.Primitive, false, 0, 0); err != nil || policy.Primitive.Mode&0111 == 0 {
		return errors.Join(err, errors.New("readonly private primitive metadata differs"))
	}
	uid, gid = policy.UID, policy.GID
	if policy.Self.UID == 0 && policy.Self.GID == 0 {
		uid, gid = 0, 0
	}
	return admitReadonlyFacts(policy.Self, false, uid, gid)
}

func readonlyCommandStrings(values []string, limit int) error {
	if len(values) == 0 || len(values) > 64 {
		return errors.New("readonly explicit command field count differs")
	}
	total := 0
	for _, value := range values {
		total += len(value)
		if len(value) > 4096 || total > limit || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\r\n") {
			return errors.New("readonly explicit command field exceeds its bound")
		}
	}
	return nil
}

func (policy readonlyPolicy) validateCommand() error {
	if err := errors.Join(readonlyCommandStrings(policy.Argv, 16384), readonlyCommandStrings(policy.Env, 32768)); err != nil {
		return err
	}
	if policy.Argv[0] != policy.Executable.Path {
		return errors.New("readonly native argv does not name the original executable")
	}
	values := make(map[string]string, len(policy.Env))
	for _, field := range policy.Env {
		key, value, exists := strings.Cut(field, "=")
		if !exists || key == "" || len(key) > 128 {
			return errors.New("readonly explicit environment is malformed")
		}
		for _, symbol := range key {
			if symbol != '_' && (symbol < 'A' || symbol > 'Z') && (symbol < '0' || symbol > '9') {
				return errors.New("readonly explicit environment key differs")
			}
		}
		if _, exists := values[key]; exists {
			return errors.New("readonly explicit environment repeats a key")
		}
		values[key] = value
	}
	return policy.validateLocations(values)
}

func (policy readonlyPolicy) validateLocations(values map[string]string) error {
	required := map[string]string{"HOME": policy.Home.Path,
		"XDG_CONFIG_HOME": filepath.Join(policy.Home.Path, ".config"),
		"XDG_DATA_HOME":   filepath.Join(policy.Home.Path, ".local", "share"),
		"XDG_CACHE_HOME":  filepath.Join(policy.Home.Path, ".cache"),
		"XDG_STATE_HOME":  filepath.Join(policy.Home.Path, ".local", "state")}
	for key, expected := range required {
		if values[key] != expected {
			return errors.New("readonly original HOME or XDG role differs")
		}
	}
	if pwd, exists := values["PWD"]; exists && pwd != policy.Dir {
		return errors.New("readonly original working directory differs")
	}
	return nil
}

func (policy readonlyPolicy) validateDevices() error {
	numbers := [6][2]uint32{{1, 3}, {1, 5}, {1, 7}, {1, 8}, {1, 9}, {5, 0}}
	for i, device := range policy.Devices {
		facts := device.Facts
		if facts.Mode&unix.S_IFMT != unix.S_IFCHR || facts.Mode&07000 != 0 || facts.UID != 0 || facts.Links != 1 ||
			facts.Inode == 0 || facts.Size != 0 || unix.Major(device.Rdev) != numbers[i][0] || unix.Minor(device.Rdev) != numbers[i][1] {
			return errors.New("readonly fixed kernel device differs")
		}
	}
	return nil
}

func sealReadonlyPolicy(policy readonlyPolicy) (result *os.File, digest string, resultErr error) {
	if err := policy.validate(); err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(policy)
	if err != nil || len(body) > readonlyPolicyLimit {
		return nil, "", errors.Join(err, errors.New("readonly private policy exceeds its bound"))
	}
	fd, err := unix.MemfdCreate("agentplugins-readonly-policy", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, "", err
	}
	result = os.NewFile(uintptr(fd), "agentplugins-readonly-policy")
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if n, err := result.Write(body); err != nil || n != len(body) {
		return result, "", errors.Join(err, io.ErrShortWrite)
	}
	if _, err := unix.FcntlInt(result.Fd(), unix.F_ADD_SEALS, readonlyPolicySeals); err != nil {
		return result, "", err
	}
	sum := sha256.Sum256(body)
	return result, hex.EncodeToString(sum[:]), nil
}

func readReadonlyPolicy(file *os.File) (readonlyPolicy, string, error) {
	seals, err := unix.FcntlInt(file.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals != readonlyPolicySeals {
		return readonlyPolicy{}, "", errors.Join(err, errors.New("readonly private policy is not immutable"))
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > readonlyPolicyLimit {
		return readonlyPolicy{}, "", errors.Join(err, errors.New("readonly private policy file differs"))
	}
	body := make([]byte, info.Size())
	if n, err := file.ReadAt(body, 0); err != nil || n != len(body) {
		return readonlyPolicy{}, "", errors.Join(err, io.ErrUnexpectedEOF)
	}
	var policy readonlyPolicy
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return readonlyPolicy{}, "", err
	}
	canonical, err := json.Marshal(policy)
	if err != nil || !bytes.Equal(canonical, body) {
		return readonlyPolicy{}, "", errors.Join(err, errors.New("readonly private policy is not canonical"))
	}
	if err := policy.validate(); err != nil {
		return readonlyPolicy{}, "", err
	}
	sum := sha256.Sum256(body)
	return policy, hex.EncodeToString(sum[:]), nil
}
