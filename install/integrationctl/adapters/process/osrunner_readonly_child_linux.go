//go:build linux

package process

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func init() {
	if os.Getenv("PLUGIN_KIT_AI_READONLY_DUPLEX_BOOTSTRAP") != "1" {
		return
	}
	_ = runReadonlyChild()
	_, _ = io.WriteString(os.Stderr, "readonly native child gate refused\n")
	os.Exit(2)
}

func readonlyMappedFacts(policy readonlyPolicy, facts readonlyObjectFacts) readonlyObjectFacts {
	if facts.UID != policy.UID {
		facts.UID = policy.OverflowUID
	}
	if facts.GID != policy.GID {
		facts.GID = policy.OverflowGID
	}
	return facts
}

func readonlyChildContext(policy readonlyPolicy) (context.Context, context.CancelFunc, error) {
	now, err := readonlyMonotonic()
	if err != nil {
		return nil, nil, err
	}
	remaining := policy.Deadline - now
	if remaining <= 0 || remaining > int64(15*time.Second) {
		return nil, nil, errors.New("readonly child original deadline has elapsed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remaining))
	return ctx, cancel, nil
}

func readonlyChildDeadline(ctx context.Context, policy readonlyPolicy) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := readonlyMonotonic()
	if err != nil || now >= policy.Deadline {
		return errors.Join(err, errors.New("readonly child original deadline has elapsed"))
	}
	return nil
}

func readonlyChildEnvironment(policy readonlyPolicy) error {
	if len(os.Args) != 2 || os.Args[0] != readonlyBootstrapPath || os.Args[1] != "--agentplugins-readonly-child" {
		return errors.New("readonly child entry differs")
	}
	want := map[string]string{"LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "PLUGIN_KIT_AI_READONLY_DUPLEX_BOOTSTRAP": "1", "PWD": policy.Dir}
	actual := os.Environ()
	if len(actual) != len(want) {
		return errors.New("readonly child wrapper environment differs")
	}
	for _, field := range actual {
		key, value, exists := strings.Cut(field, "=")
		expected, present := want[key]
		if !exists || !present || value != expected {
			return errors.New("readonly child wrapper environment differs")
		}
		delete(want, key)
	}
	return nil
}

func runReadonlyChild() (resultErr error) {
	policyFile := os.NewFile(3, "readonly-private-policy")
	primitive := os.NewFile(4, "readonly-original-primitive")
	defer func() {
		if policyFile != nil {
			resultErr = errors.Join(resultErr, policyFile.Close())
		}
		if primitive != nil {
			resultErr = errors.Join(resultErr, primitive.Close())
		}
	}()
	policy, digest, err := readReadonlyPolicy(policyFile)
	if err != nil {
		return err
	}
	ctx, cancel, err := readonlyChildContext(policy)
	if err != nil {
		return err
	}
	defer cancel()
	if err := errors.Join(readonlyChildEnvironment(policy), requireReadonlyHeld(primitive, readonlyMappedFacts(policy, policy.Primitive)),
		readonlyNoFileCapability(primitive)); err != nil {
		return err
	}
	if err := validateReadonlyChild(ctx, policy); err != nil {
		return err
	}
	if err := verifyReadonlySelf(ctx, policy); err != nil {
		return err
	}
	native, err := openReadonlyNative(ctx, policy)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, native.Close()) }()
	err = errors.Join(policyFile.Close(), primitive.Close())
	policyFile, primitive = nil, nil
	if err != nil {
		return err
	}
	return execReadonlyNative(ctx, policy, digest, native)
}

func verifyReadonlySelf(ctx context.Context, policy readonlyPolicy) (resultErr error) {
	if err := readonlyChildDeadline(ctx, policy); err != nil {
		return err
	}
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	expected := readonlyMappedFacts(policy, policy.Self)
	object := readonlyOriginalObject{path: readonlyBootstrapPath, file: file, facts: expected}
	if err := errors.Join(object.joinMetadata(), readonlyNoFileCapability(file)); err != nil {
		return err
	}
	digest, err := hashReadonlyHeld(ctx, file, expected)
	if err != nil || digest != policy.SelfSHA {
		return errors.Join(err, errors.New("readonly child executable differs from the original bootstrap"))
	}
	return readonlyChildDeadline(ctx, policy)
}

func openReadonlyNative(ctx context.Context, policy readonlyPolicy) (result *os.File, resultErr error) {
	if err := readonlyChildDeadline(ctx, policy); err != nil {
		return nil, err
	}
	fd, err := unix.Open(policy.Executable.Path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	result = os.NewFile(uintptr(fd), "readonly-original-native")
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	expected := readonlyMappedFacts(policy, policy.Executable.Facts)
	object := readonlyOriginalObject{path: policy.Executable.Path, file: result, facts: expected}
	if err := errors.Join(object.joinMetadata(), readonlyNoFileCapability(result)); err != nil {
		return result, err
	}
	digest, err := hashReadonlyHeld(ctx, result, expected)
	if err != nil || digest != policy.NativeSHA {
		return result, errors.Join(err, errors.New("readonly original native bytes differ"))
	}
	var magic [4]byte
	if _, err := result.ReadAt(magic[:], 0); err != nil || !bytes.Equal(magic[:], []byte{0x7f, 'E', 'L', 'F'}) {
		return result, errors.Join(err, errors.New("readonly native requires the selected Linux binary"))
	}
	return result, readonlyChildDeadline(ctx, policy)
}

func readonlyHeader(policy readonlyPolicy, digest string) string {
	return "agentplugins-readonly-v1:" + policy.Nonce + ":" + digest + "\n"
}

func readReadonlyHeader(ctx context.Context, stdout io.Reader, policy readonlyPolicy, digest string) error {
	if err := readonlyChildDeadline(ctx, policy); err != nil {
		return err
	}
	want := readonlyHeader(policy, digest)
	body := make([]byte, len(want))
	if _, err := io.ReadFull(stdout, body); err != nil {
		return err
	}
	if string(body) != want {
		return errors.New("readonly child did not join the original private view")
	}
	return readonlyChildDeadline(ctx, policy)
}

func execReadonlyNative(ctx context.Context, policy readonlyPolicy, digest string, native *os.File) error {
	if err := checkReadonlyExecDescriptors(ctx); err != nil {
		return err
	}
	if err := readonlyChildDeadline(ctx, policy); err != nil {
		return err
	}
	header := readonlyHeader(policy, digest)
	if n, err := io.WriteString(os.Stdout, header); err != nil || n != len(header) {
		return errors.Join(err, io.ErrShortWrite)
	}
	if err := readonlyChildDeadline(ctx, policy); err != nil {
		return err
	}
	return syscall.Exec("/proc/self/fd/"+strconv.FormatUint(uint64(native.Fd()), 10), policy.Argv, policy.Env)
}

func checkReadonlyExecDescriptors(ctx context.Context) error {
	for fd := range 3 {
		var stat unix.Stat_t
		if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFIFO {
			return errors.Join(err, errors.New("readonly child stdio is not the original pipe interface"))
		}
	}
	file, err := os.Open("/proc/self/fd")
	if err != nil {
		return err
	}
	names, readErr := file.Readdirnames(65)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	err = errors.Join(readErr, file.Close())
	if err != nil || len(names) > 64 {
		return errors.Join(err, errors.New("readonly child descriptor census differs"))
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		fd, err := strconv.ParseUint(name, 10, 31)
		if err != nil || name != strconv.FormatUint(fd, 10) {
			return errors.New("readonly child descriptor name differs")
		}
		if fd > 2 {
			flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
			if errors.Is(err, unix.EBADF) {
				continue
			}
			if err != nil || flags&unix.FD_CLOEXEC == 0 {
				return errors.Join(err, errors.New("readonly native would inherit an extra descriptor"))
			}
		}
	}
	return ctx.Err()
}
