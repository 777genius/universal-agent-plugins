//go:build linux

package process

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// validateReadonlyChild checks the already constructed, private view. Original
// input census and descriptor/hash authority remain with their existing owners.
func validateReadonlyChild(ctx context.Context, policy readonlyPolicy) error {
	if err := readonlyChildCutoff(ctx, policy.Deadline); err != nil {
		return err
	}
	if err := policy.validate(); err != nil {
		return err
	}
	namespaces, err := captureReadonlyNamespaces(ctx)
	if err != nil {
		return err
	}
	if err := readonlyChildNamespaces(policy.Namespaces, namespaces); err != nil {
		return err
	}
	if err := readonlyChildIdentity(ctx, policy); err != nil {
		return err
	}
	body, err := readReadonlyProc(ctx, "/proc/self/mountinfo")
	if err != nil {
		return err
	}
	if err := readonlyChildMounts(body, policy); err != nil {
		return err
	}
	if err := readonlyChildObjects(ctx, policy); err != nil {
		return err
	}
	return readonlyChildCutoff(ctx, policy.Deadline)
}

func readonlyChildCutoff(ctx context.Context, deadline int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := readonlyMonotonic()
	if err != nil {
		return err
	}
	if deadline <= 0 || now >= deadline {
		return context.DeadlineExceeded
	}
	return nil
}

func readonlyChildNamespaces(original, current [7]uint64) error {
	for i, identity := range current {
		if identity == 0 || original[i] == 0 || (i == 6) != (identity == original[i]) {
			return errors.New("readonly child namespace identity differs")
		}
	}
	return nil
}

func readonlyChildIDMap(body []byte, identity uint32) error {
	line := strings.TrimSuffix(string(body), "\n")
	fields := strings.Fields(line)
	if identity == 0 || len(body) == 0 || len(body) > 128 || line+"\n" != string(body) || strings.Trim(line, " \t0123456789") != "" || len(fields) != 3 {
		return errors.New("readonly child identity map differs")
	}
	if fields[0] != strconv.FormatUint(uint64(identity), 10) || fields[1] != "0" || fields[2] != "1" {
		return errors.New("readonly child identity map differs")
	}
	return nil
}

func readonlyChildIdentity(ctx context.Context, policy readonlyPolicy) error {
	for _, field := range []struct {
		path string
		id   uint32
	}{{"/proc/self/uid_map", policy.UID}, {"/proc/self/gid_map", policy.GID}} {
		body, err := readReadonlyProc(ctx, field.path)
		if err != nil {
			return err
		}
		if err := readonlyChildIDMap(body, field.id); err != nil {
			return err
		}
	}
	groups, err := readReadonlyProc(ctx, "/proc/self/setgroups")
	if err != nil || string(groups) != "deny\n" {
		return errors.Join(err, errors.New("readonly child setgroups permission differs"))
	}
	uid, err := readonlyOverflow(ctx, "/proc/sys/kernel/overflowuid")
	if err != nil || uid != policy.OverflowUID {
		return errors.Join(err, errors.New("readonly child overflow uid differs"))
	}
	gid, err := readonlyOverflow(ctx, "/proc/sys/kernel/overflowgid")
	if err != nil || gid != policy.OverflowGID {
		return errors.Join(err, errors.New("readonly child overflow gid differs"))
	}
	body, err := readReadonlyProc(ctx, "/proc/self/status")
	if err != nil {
		return err
	}
	return readonlyChildStatus(body, policy)
}

func readonlyChildStatusFields(body []byte) (map[string][]string, error) {
	fields := map[string][]string{"Uid": nil, "Gid": nil, "Groups": nil, "CapInh": nil, "CapPrm": nil,
		"CapEff": nil, "CapBnd": nil, "CapAmb": nil, "NoNewPrivs": nil}
	if len(body) == 0 || len(body) > 65536 || body[len(body)-1] != '\n' || strings.ContainsAny(string(body), "\x00\r") {
		return nil, errors.New("readonly child status exceeds its fixed shape")
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, errors.New("readonly child status field is malformed")
		}
		previous, wanted := fields[key]
		if !wanted {
			continue
		}
		if previous != nil {
			return nil, errors.New("readonly child status repeats an authority field")
		}
		fields[key] = append([]string{}, strings.Fields(value)...)
	}
	for _, value := range fields {
		if value == nil {
			return nil, errors.New("readonly child status lacks an authority field")
		}
	}
	return fields, nil
}

func readonlyChildStatus(body []byte, policy readonlyPolicy) error {
	fields, err := readonlyChildStatusFields(body)
	if err != nil {
		return err
	}
	for _, field := range []struct {
		key string
		id  uint32
	}{{"Uid", policy.UID}, {"Gid", policy.GID}} {
		values := fields[field.key]
		if len(values) != 4 {
			return errors.New("readonly child credential count differs")
		}
		for _, value := range values {
			if value != strconv.FormatUint(uint64(field.id), 10) {
				return errors.New("readonly child credentials differ")
			}
		}
	}
	for _, key := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"} {
		if values := fields[key]; len(values) != 1 || values[0] != "0000000000000000" {
			return errors.New("readonly child retained a capability")
		}
	}
	if values := fields["NoNewPrivs"]; len(values) != 1 || values[0] != "1" {
		return errors.New("readonly child privilege escalation is not closed")
	}
	return readonlyChildGroups(fields["Groups"], policy)
}

func readonlyChildGroups(values []string, policy readonlyPolicy) error {
	if len(values) != len(policy.Groups) || len(values) > 64 {
		return errors.New("readonly child supplementary group count differs")
	}
	for i, original := range policy.Groups {
		mapped := policy.OverflowGID
		if original == policy.GID {
			mapped = policy.GID
		}
		if values[i] != strconv.FormatUint(uint64(mapped), 10) {
			return errors.New("readonly child supplementary group translation differs")
		}
	}
	return nil
}

func readonlyChildObjects(ctx context.Context, policy readonlyPolicy) error {
	objects := make([]readonlyPolicyObject, 0, 9)
	objects = append(objects, policy.Home, policy.Worktree, policy.Project, policy.Executable,
		readonlyPolicyObject{Path: readonlyBootstrapPath, Facts: policy.Self})
	for i, visible := range readonlyRuntimePaths() {
		objects = append(objects, readonlyPolicyObject{Path: visible, Facts: policy.Runtime[i].Facts})
	}
	for _, object := range objects {
		if _, err := readonlyChildObject(ctx, object.Path, readonlyMappedFacts(policy, object.Facts)); err != nil {
			return err
		}
	}
	for i, name := range readonlyDeviceNames() {
		device, err := readonlyChildObject(ctx, "/dev/"+name, readonlyMappedFacts(policy, policy.Devices[i].Facts))
		if err != nil || device != policy.Devices[i].Rdev {
			return errors.Join(err, errors.New("readonly child original device differs"))
		}
	}
	return ctx.Err()
}

func readonlyChildObject(ctx context.Context, path string, expected readonlyObjectFacts) (device uint64, resultErr error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	fd, err := unix.Open(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	file := os.NewFile(uintptr(fd), "readonly-child-object")
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	held, err := file.Stat()
	if err != nil {
		return 0, err
	}
	facts, err := readonlyFacts(held)
	if err != nil || facts != expected {
		return 0, errors.Join(err, errors.New("readonly child held object differs from its original"))
	}
	named, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	facts, err = readonlyFacts(named)
	if err != nil || facts != expected {
		return 0, errors.Join(err, errors.New("readonly child named object differs from its original"))
	}
	return held.Sys().(*syscall.Stat_t).Rdev, ctx.Err()
}

const (
	readonlyMountRO uint8 = 1 << iota
	readonlyMountNosuid
	readonlyMountNodev
	readonlyMountNoexec
)

type readonlyChildMount struct {
	id, parent, device uint64
	path, root, fs     string
	flags              uint8
}

type readonlyChildMountRule struct {
	fs, root string
	flags    uint8
	device   uint64
	original bool
}

func readonlyChildMountRules(policy readonlyPolicy) (map[string]readonlyChildMountRule, error) {
	base := readonlyMountRO | readonlyMountNosuid | readonlyMountNodev
	rules := map[string]readonlyChildMountRule{
		"/": {fs: "tmpfs", root: "/", flags: base}, "/dev": {fs: "tmpfs", root: "/", flags: base},
		"/proc":    {fs: "proc", root: "/", flags: base | readonlyMountNoexec},
		"/dev/pts": {fs: "devpts", root: "/", flags: readonlyMountRO | readonlyMountNosuid | readonlyMountNoexec},
	}
	objects := make([]readonlyPolicyObject, 0, 8)
	objects = append(objects, readonlyMountedInputs(policy)...)
	objects = append(objects, readonlyPolicyObject{Path: readonlyBootstrapPath, Facts: policy.Self})
	for i, visible := range readonlyRuntimePaths() {
		objects = append(objects, readonlyPolicyObject{Path: visible, Facts: policy.Runtime[i].Facts})
	}
	if readonlyNativeSeparate(policy) {
		objects = append(objects, policy.Executable)
	}
	for _, object := range objects {
		if _, duplicate := rules[object.Path]; duplicate {
			return nil, errors.New("readonly child mount roles overlap")
		}
		rules[object.Path] = readonlyChildMountRule{flags: base, device: object.Facts.Device, original: true}
	}
	for i, name := range readonlyDeviceNames() {
		path := "/dev/" + name
		if _, duplicate := rules[path]; duplicate {
			return nil, errors.New("readonly child mount roles overlap")
		}
		rules[path] = readonlyChildMountRule{flags: readonlyMountRO | readonlyMountNosuid,
			device: policy.Devices[i].Facts.Device, original: true}
	}
	return rules, nil
}

func readonlyChildMountOptions(value string) (uint8, error) {
	var flags uint8
	seen := make(map[string]bool)
	for _, option := range strings.Split(value, ",") {
		if seen[option] {
			return 0, errors.New("readonly child mount repeats an option")
		}
		seen[option] = true
		switch option {
		case "ro":
			flags |= readonlyMountRO
		case "nosuid":
			flags |= readonlyMountNosuid
		case "nodev":
			flags |= readonlyMountNodev
		case "noexec":
			flags |= readonlyMountNoexec
		case "noatime", "nodiratime", "relatime", "strictatime", "lazytime", "nosymfollow":
		default:
			return 0, errors.New("readonly child mount has an unsupported option")
		}
	}
	return flags, nil
}

func readonlyChildMountPath(value string) (string, error) {
	if value == "" || len(value) > 16384 {
		return "", errors.New("readonly child mount path exceeds its bound")
	}
	decoded := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			decoded = append(decoded, value[i])
			continue
		}
		if len(value)-i < 4 {
			return "", errors.New("readonly child mount escape is malformed")
		}
		switch value[i+1 : i+4] {
		case "040":
			decoded = append(decoded, ' ')
		case "011":
			decoded = append(decoded, '\t')
		case "012":
			decoded = append(decoded, '\n')
		case "134":
			decoded = append(decoded, '\\')
		default:
			return "", errors.New("readonly child mount escape differs")
		}
		i += 3
	}
	path := string(decoded)
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || !utf8.ValidString(path) || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("readonly child mount path is not canonical")
	}
	return path, nil
}

func readonlyChildMountNumber(value string, bits int) (uint64, error) {
	number, err := strconv.ParseUint(value, 10, bits)
	if err != nil || strconv.FormatUint(number, 10) != value {
		return 0, errors.Join(err, errors.New("readonly child mount number differs"))
	}
	return number, nil
}

func readonlyChildMountIDs(fields []string) (readonlyChildMount, error) {
	var row readonlyChildMount
	id, idErr := readonlyChildMountNumber(fields[0], 64)
	parent, parentErr := readonlyChildMountNumber(fields[1], 64)
	majorText, minorText, found := strings.Cut(fields[2], ":")
	major, majorErr := readonlyChildMountNumber(majorText, 32)
	minor, minorErr := readonlyChildMountNumber(minorText, 32)
	if err := errors.Join(idErr, parentErr, majorErr, minorErr); err != nil || !found || id == 0 || parent == 0 || major > math.MaxUint32 || minor > math.MaxUint32 {
		return row, errors.Join(err, errors.New("readonly child mount identity differs"))
	}
	row.id, row.parent, row.device = id, parent, unix.Mkdev(uint32(major), uint32(minor))
	return row, nil
}

func readonlyChildMountLine(line string) (readonlyChildMount, error) {
	fields := strings.Split(line, " ")
	var row readonlyChildMount
	if len(fields) < 10 || len(fields) > 16 {
		return row, errors.New("readonly child mount row has an unexpected shape")
	}
	separator := 6
	if fields[separator] == "unbindable" {
		separator++
	}
	if len(fields) != separator+4 || fields[separator] != "-" || fields[separator+1] == "" || fields[separator+2] == "" || fields[separator+3] == "" {
		return row, errors.New("readonly child mount propagation or filesystem fields differ")
	}
	var err error
	row, err = readonlyChildMountIDs(fields)
	if err != nil {
		return row, err
	}
	row.root, err = readonlyChildMountPath(fields[3])
	if err != nil {
		return row, err
	}
	row.path, err = readonlyChildMountPath(fields[4])
	if err != nil {
		return row, err
	}
	row.flags, err = readonlyChildMountOptions(fields[5])
	row.fs = fields[separator+1]
	return row, err
}

func readonlyChildMountAllowed(row readonlyChildMount, rules map[string]readonlyChildMountRule) error {
	rule, found := rules[row.path]
	if !found {
		switch row.path {
		case "/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus":
			rule = readonlyChildMountRule{fs: "proc", root: strings.TrimPrefix(row.path, "/proc"),
				flags: readonlyMountRO | readonlyMountNosuid | readonlyMountNodev | readonlyMountNoexec}
		default:
			return errors.New("readonly child has an undeclared mount")
		}
	}
	if row.flags&rule.flags != rule.flags || rule.fs != "" && row.fs != rule.fs || rule.root != "" && row.root != rule.root ||
		rule.original && row.device != rule.device {
		return errors.New("readonly child mount does not match its fixed role")
	}
	return nil
}

func readonlyChildMounts(body []byte, policy readonlyPolicy) error {
	if len(body) == 0 || len(body) > 262144 || body[len(body)-1] != '\n' || strings.ContainsAny(string(body), "\x00\r") {
		return errors.New("readonly child mount table exceeds its bound")
	}
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	rules, err := readonlyChildMountRules(policy)
	if err != nil {
		return err
	}
	if len(lines) < len(rules) || len(lines) > len(rules)+4 {
		return errors.New("readonly child mount count differs")
	}
	rows := make(map[string]readonlyChildMount, len(lines))
	identities := make(map[uint64]readonlyChildMount, len(lines))
	for _, line := range lines {
		row, err := readonlyChildMountLine(line)
		if err != nil {
			return err
		}
		if _, exists := rows[row.path]; exists {
			return errors.New("readonly child mount point is repeated")
		}
		if _, exists := identities[row.id]; exists {
			return errors.New("readonly child mount identity is repeated")
		}
		if err := readonlyChildMountAllowed(row, rules); err != nil {
			return err
		}
		rows[row.path], identities[row.id] = row, row
	}
	for path := range rules {
		if _, exists := rows[path]; !exists {
			return errors.New("readonly child lacks a required mount")
		}
	}
	return readonlyChildMountTopology(rows["/"].id, identities)
}

func readonlyChildMountTopology(root uint64, rows map[uint64]readonlyChildMount) error {
	for _, row := range rows {
		for steps := 0; row.id != root; steps++ {
			parent, exists := rows[row.parent]
			if !exists || steps >= len(rows) || parent.id == row.id || parent.path != "/" && !readonlyWithin(row.path, parent.path) {
				return errors.New("readonly child mount ancestry differs")
			}
			row = parent
		}
	}
	return nil
}
