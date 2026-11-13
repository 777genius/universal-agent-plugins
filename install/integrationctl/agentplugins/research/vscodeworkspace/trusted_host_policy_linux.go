//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const stockCodeProfileSHA = "9fd4a3cc5f3bf64aefbed6ff4cab30fd60563d655ae2896197f152d07b726e9a"
const hostPolicyArtifact = "trusted-host-policy.json"

type policyIdentity struct {
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
	Mode    uint32 `json:"mode"`
	UID     uint32 `json:"uid"`
	GID     uint32 `json:"gid"`
	Links   uint64 `json:"links"`
	Size    int64  `json:"size"`
	MtimeNS int64  `json:"mtime_ns"`
	CtimeNS int64  `json:"ctime_ns"`
}
type policyFileFact struct {
	State    string          `json:"state"`
	SHA256   string          `json:"sha256"`
	Identity *policyIdentity `json:"identity"`
}
type policyDirectoryFact struct {
	Identity *policyIdentity `json:"identity"`
	MountID  uint64          `json:"mount_id"`
}
type hostPolicySnapshot struct {
	Files             map[string]policyFileFact      `json:"files"`
	Directories       map[string]policyDirectoryFact `json:"directories"`
	Entries           map[string]string              `json:"entries"`
	Sysctls           map[string]*int64              `json:"sysctls"`
	Grammar           string                         `json:"grammar"`
	KnownProfileMatch bool                           `json:"known_profile_match"`
	LoadedMode        string                         `json:"loaded_vscode_mode"`
	Enabled           *bool                          `json:"apparmor_enabled"`
	CodeAbsent        *bool                          `json:"code_absent"`
}
type hostPolicyReceipt struct {
	Schema           int                `json:"schema_version"`
	Status           string             `json:"status"`
	Reason           string             `json:"reason"`
	ReferenceSHA     string             `json:"reference_profile_sha256"`
	Before           hostPolicySnapshot `json:"before"`
	After            hostPolicySnapshot `json:"after"`
	NativeQualified  bool               `json:"native_qualified"`
	LoadedEqualsDisk bool               `json:"loaded_equals_disk_qualified"`
}
type policyHeld struct {
	file     *os.File
	path     string
	identity policyIdentity
}
type hostPolicyMeasurement struct {
	before hostPolicySnapshot
	reason string
	held   []policyHeld
}
type policyInput struct {
	key, path string
	optional  bool
}

var policyFiles = []policyInput{
	{"code", "/etc/apparmor.d/code", false}, {"local_code", "/etc/apparmor.d/local/code", true},
	{"abi", "/etc/apparmor.d/abi/4.0", true}, {"global", "/etc/apparmor.d/tunables/global", true}, {"alias", "/etc/apparmor.d/tunables/alias", true},
}
var policySysctls = []policyInput{
	{"apparmor_userns", "/proc/sys/kernel/apparmor_restrict_unprivileged_userns", false},
	{"apparmor_unconfined", "/proc/sys/kernel/apparmor_restrict_unprivileged_unconfined", false},
	{"userns_clone", "/proc/sys/kernel/unprivileged_userns_clone", false},
	{"max_user_namespaces", "/proc/sys/user/max_user_namespaces", false},
}

func policyStat(info os.FileInfo) (policyIdentity, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return policyIdentity{}, fmt.Errorf("policy-stat-refused")
	}
	return policyIdentity{st.Dev, st.Ino, st.Mode, st.Uid, st.Gid, uint64(st.Nlink), st.Size, st.Mtim.Nano(), st.Ctim.Nano()}, nil
}
func policyReason(reason *string, next string) {
	if *reason == "none" {
		*reason = next
	}
}

// Opening is nonblocking before type admission. Ordinary and virtual files keep
// the same identity checks; only the latter explicitly permit size-zero streams.
func policyRead(path string, limit int64, virtual bool, uid uint32) ([]byte, *os.File, policyIdentity, error) {
	return policyReadUsing(path, limit, virtual, uid, unix.Open)
}
func policyReadUsing(path string, limit int64, virtual bool, uid uint32, open func(string, int, uint32) (int, error)) ([]byte, *os.File, policyIdentity, error) {
	before, err := anchoredInfo(path)
	if err != nil {
		return nil, nil, policyIdentity{}, err
	}
	identity, err := policyStat(before)
	statLimit := limit
	if virtual {
		statLimit = 1 << 20
	}
	if err != nil || !regular(before, statLimit) || identity.UID != uid || before.Mode().Perm()&022 != 0 {
		return nil, nil, identity, fmt.Errorf("policy-file-refused")
	}
	fd, err := open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, identity, err
	}
	file := os.NewFile(uintptr(fd), path)
	fail := func(err error) ([]byte, *os.File, policyIdentity, error) {
		return nil, nil, identity, errors.Join(err, file.Close())
	}
	held, err := file.Stat()
	if err != nil || !sameFile(before, held) {
		return fail(fmt.Errorf("policy-open-identity-refused"))
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return fail(err)
	}
	after, e := file.Stat()
	linked, pe := anchoredInfo(path)
	if e != nil || pe != nil || int64(len(body)) > limit || !sameFile(before, after) || !sameFile(before, linked) || (!virtual && int64(len(body)) != before.Size()) {
		return fail(fmt.Errorf("policy-capture-refused"))
	}
	return body, file, identity, nil
}
func stockPolicyGrammar(body []byte) bool {
	if len(body) > 16<<10 || !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		return false
	}
	expected := []string{"abi <abi/4.0>,", "include <tunables/global>", "profile vscode /usr/share/code{/bin,}/code flags=(unconfined) {", "userns,", "include if exists <local/code>", "}"}
	var actual []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		// #include is a directive, not an ignorable comment.
		if strings.HasPrefix(line, "#") {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(line, "#")), "include") {
				return false
			}
			continue
		}
		for _, char := range line {
			if char > 127 {
				return false
			}
		}
		actual = append(actual, line)
	}
	return reflect.DeepEqual(actual, expected)
}
func loadedVSCodeMode(body []byte) (string, error) {
	if len(body) > 256<<10 {
		return "", fmt.Errorf("policy-loaded-bound")
	}
	found := "absent"
	for _, line := range strings.Split(string(body), "\n") {
		if len(line) > 4096 {
			return "", fmt.Errorf("policy-loaded-line-bound")
		}
		if line != "vscode" && !strings.HasPrefix(line, "vscode ") {
			continue
		}
		if found != "absent" {
			return "", fmt.Errorf("policy-loaded-duplicate")
		}
		switch line {
		case "vscode (unconfined)":
			found = "unconfined"
		case "vscode (enforce)":
			found = "enforce"
		case "vscode (complain)":
			found = "complain"
		case "vscode (kill)":
			found = "kill"
		default:
			return "", fmt.Errorf("policy-loaded-shape")
		}
	}
	return found, nil
}
func policySysctl(body []byte, maximum int64) (int64, error) {
	if len(body) > 32 {
		return 0, fmt.Errorf("policy-sysctl-bound")
	}
	value := strings.TrimSuffix(string(body), "\n")
	if value == "" || strings.Trim(value, "0123456789") != "" || (len(value) > 1 && value[0] == '0') {
		return 0, fmt.Errorf("policy-sysctl-shape")
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number > maximum {
		return 0, fmt.Errorf("policy-sysctl-range")
	}
	return number, nil
}
func emptyPolicySnapshot() hostPolicySnapshot {
	s := hostPolicySnapshot{Files: map[string]policyFileFact{}, Directories: map[string]policyDirectoryFact{}, Entries: map[string]string{}, Sysctls: map[string]*int64{}, Grammar: "unmeasured", LoadedMode: "unmeasured"}
	for _, input := range policyFiles {
		s.Files[input.key] = policyFileFact{State: "unmeasured"}
	}
	for _, name := range []string{"root", "usr", "usr_share"} {
		s.Directories[name] = policyDirectoryFact{}
	}
	for _, name := range []string{"disable_code", "force_complain_code"} {
		s.Entries[name] = "unmeasured"
	}
	for _, input := range policySysctls {
		s.Sysctls[input.key] = nil
	}
	return s
}
func policyOptionalAbsent(path string) bool {
	// Admit missing optional leaves only through safe existing ancestors.
	if _, err := anchoredInfo(path); !os.IsNotExist(err) {
		return false
	}
	parent := filepath.Dir(path)
	for {
		if _, err := anchoredInfo(parent); os.IsNotExist(err) {
			parent = filepath.Dir(parent)
			continue
		}
		return immutableAncestors(parent) == nil
	}
}
func (m *hostPolicyMeasurement) fileFact(input policyInput) (policyFileFact, []byte) {
	if input.optional && policyOptionalAbsent(input.path) {
		return policyFileFact{State: "absent"}, nil
	}
	if immutableAncestors(filepath.Dir(input.path)) != nil {
		policyReason(&m.reason, "file-unsafe")
		return policyFileFact{State: "refused"}, nil
	}
	body, file, identity, err := policyRead(input.path, 16<<10, false, 0)
	if err != nil {
		policyReason(&m.reason, "file-capture-refused")
		return policyFileFact{State: "refused"}, nil
	}
	sum := sha256.Sum256(body)
	m.held = append(m.held, policyHeld{file, input.path, identity})
	return policyFileFact{State: "present", SHA256: hex.EncodeToString(sum[:]), Identity: &identity}, body
}
func policyEntry(path string) (string, error) {
	if policyOptionalAbsent(path) {
		return "absent", nil
	}
	if immutableAncestors(filepath.Dir(path)) != nil {
		return "", fmt.Errorf("policy-entry-parent")
	}
	info, err := anchoredInfo(path)
	if err != nil {
		return "", err
	}
	id, err := policyStat(info)
	if err != nil || id.UID != 0 {
		return "", fmt.Errorf("policy-entry-owner")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "symlink", nil
	}
	if info.Mode().IsRegular() {
		return "ordinary", nil
	}
	return "other", nil
}
func (m *hostPolicyMeasurement) directoryFact(path string) policyDirectoryFact {
	info, err := exactDirectory(path, 0)
	if err != nil {
		policyReason(&m.reason, "directory-refused")
		return policyDirectoryFact{}
	}
	fd, err := unix.Open(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		policyReason(&m.reason, "directory-refused")
		return policyDirectoryFact{}
	}
	file := os.NewFile(uintptr(fd), path)
	held, e := file.Stat()
	linked, pe := anchoredInfo(path)
	identity, se := policyStat(info)
	mount, me := scannerMountID(path)
	if e != nil || pe != nil || se != nil || me != nil || !sameFile(info, held) || !sameFile(info, linked) {
		_ = file.Close()
		policyReason(&m.reason, "directory-refused")
		return policyDirectoryFact{}
	}
	m.held = append(m.held, policyHeld{file, path, identity})
	return policyDirectoryFact{&identity, mount}
}
func policyVirtual(path string, limit int64) ([]byte, error) {
	if immutableAncestors(filepath.Dir(path)) != nil {
		return nil, fmt.Errorf("policy-virtual-parent")
	}
	body, file, _, err := policyRead(path, limit, true, 0)
	if err != nil {
		return nil, err
	}
	return body, file.Close()
}
func (m *hostPolicyMeasurement) snapshot() hostPolicySnapshot {
	s := emptyPolicySnapshot()
	m.snapshotFiles(&s)
	for key, path := range map[string]string{"root": "/", "usr": "/usr", "usr_share": "/usr/share"} {
		s.Directories[key] = m.directoryFact(path)
	}
	if _, err := anchoredInfo("/usr/share/code"); os.IsNotExist(err) {
		absent := true
		s.CodeAbsent = &absent
	} else if err == nil {
		absent := false
		s.CodeAbsent = &absent
	} else {
		policyReason(&m.reason, "code-leaf-refused")
	}
	for key, path := range map[string]string{"disable_code": "/etc/apparmor.d/disable/code", "force_complain_code": "/etc/apparmor.d/force-complain/code"} {
		state, err := policyEntry(path)
		if err != nil {
			state = "refused"
			policyReason(&m.reason, "entry-refused")
		}
		s.Entries[key] = state
	}
	m.snapshotKernel(&s)
	return s
}
func (m *hostPolicyMeasurement) snapshotFiles(s *hostPolicySnapshot) {
	for _, input := range policyFiles {
		fact, body := m.fileFact(input)
		s.Files[input.key] = fact
		if input.key == "code" && fact.State == "present" {
			s.KnownProfileMatch = fact.SHA256 == stockCodeProfileSHA
			if stockPolicyGrammar(body) {
				s.Grammar = "known-ubuntu-vscode-userns"
			} else {
				s.Grammar = "refused"
				policyReason(&m.reason, "grammar-refused")
			}
		}
		clear(body)
	}
}
func (m *hostPolicyMeasurement) snapshotKernel(s *hostPolicySnapshot) {
	body, err := policyVirtual("/sys/kernel/security/apparmor/profiles", 256<<10)
	if err == nil {
		s.LoadedMode, err = loadedVSCodeMode(body)
		if err == nil && s.LoadedMode == "absent" {
			policyReason(&m.reason, "loaded-profile-absent")
		}
	}
	clear(body)
	if err != nil {
		policyReason(&m.reason, "loaded-profile-refused")
	}
	body, err = policyVirtual("/sys/module/apparmor/parameters/enabled", 8)
	if err == nil && (bytes.Equal(body, []byte("Y\n")) || bytes.Equal(body, []byte("N\n"))) {
		enabled := body[0] == 'Y'
		s.Enabled = &enabled
		if !enabled {
			policyReason(&m.reason, "apparmor-disabled")
		}
	} else {
		policyReason(&m.reason, "enabled-fact-refused")
	}
	clear(body)
	for _, input := range policySysctls {
		body, err := policyVirtual(input.path, 32)
		maximum := int64(1)
		if input.key == "max_user_namespaces" {
			maximum = 2147483647
		}
		if err == nil {
			var value int64
			value, err = policySysctl(body, maximum)
			if err == nil {
				s.Sysctls[input.key] = &value
			}
		}
		clear(body)
		if err != nil {
			policyReason(&m.reason, "sysctl-refused")
		}
	}
}
func beginHostPolicyMeasurement() *hostPolicyMeasurement {
	m := &hostPolicyMeasurement{reason: "none"}
	m.before = m.snapshot()
	return m
}
func (m *hostPolicyMeasurement) finish(c *controller, after hostPolicySnapshot, nativeErr error) error {
	for _, held := range m.held {
		info, err := held.file.Stat()
		linked, pe := anchoredInfo(held.path)
		var id, other policyIdentity
		var e, le error
		if err == nil {
			id, e = policyStat(info)
		}
		if pe == nil {
			other, le = policyStat(linked)
		}
		if err != nil || pe != nil || e != nil || le != nil || id != held.identity || other != held.identity {
			policyReason(&m.reason, "held-identity-changed")
		}
		if held.file.Close() != nil {
			policyReason(&m.reason, "descriptor-close-refused")
		}
	}
	if !reflect.DeepEqual(m.before, after) {
		policyReason(&m.reason, "measurement-changed")
	}
	status := "observed"
	if m.reason != "none" {
		status = "refused"
	}
	receipt := hostPolicyReceipt{Schema: 1, Status: status, Reason: m.reason, ReferenceSHA: stockCodeProfileSHA, Before: m.before, After: after}
	body, err := encode(receipt, 16<<10)
	if err != nil {
		return errors.Join(nativeErr, fmt.Errorf("host policy receipt bound refused"))
	}
	if err = c.keep(hostPolicyArtifact, body); err != nil {
		return errors.Join(nativeErr, err)
	}
	if m.reason != "none" {
		return errors.Join(nativeErr, fmt.Errorf("host policy measurement refused: %s", m.reason))
	}
	return nativeErr
}
