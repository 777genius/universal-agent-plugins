//go:build linux

package process

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strconv"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type readonlyPreparedView struct {
	inputs readonlyOriginalInputs
	system *readonlySystemInputs
	kernel readonlyKernelInputs
	policy readonlyPolicy
	sealed *os.File
	digest string
}

func prepareReadonlyView(ctx context.Context, command ports.Command, originals ports.ReadonlyDuplexInputs) (view *readonlyPreparedView, resultErr error) {
	deadline, err := readonlyOriginalDeadline(ctx)
	if err != nil {
		return nil, err
	}
	if os.Getuid() <= 0 || os.Getgid() <= 0 || os.Getuid() != os.Geteuid() || os.Getgid() != os.Getegid() ||
		command.DiscardStdout || command.StdoutLimitBytes != 0 {
		return nil, errors.New("readonly native transport requires ordinary identity and duplex output")
	}
	inputs, err := admitReadonlyInputs(ctx, command, originals)
	if err != nil {
		return nil, err
	}
	system, err := openReadonlySystem(ctx)
	if err != nil {
		return nil, err
	}
	view = &readonlyPreparedView{inputs: inputs, system: system}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, system.close())
			view = nil
		}
	}()
	view.kernel, err = captureReadonlyKernel(ctx, system)
	if err != nil {
		return view, err
	}
	if err := view.capturePolicy(ctx, command, deadline); err != nil {
		return view, err
	}
	view.sealed, view.digest, err = sealReadonlyPolicy(view.policy)
	if err != nil {
		return view, err
	}
	system.owned = append(system.owned, view.sealed)
	return view, ctx.Err()
}

func readonlyWireObject(original readonlyOriginalObject) readonlyPolicyObject {
	return readonlyPolicyObject{Path: original.path, Facts: original.facts}
}

func (view *readonlyPreparedView) capturePolicy(ctx context.Context, command ports.Command, deadline int64) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	uid, gid, err := readonlyCallerIDs()
	if err != nil {
		return err
	}
	view.policy = readonlyPolicy{Version: 1, Nonce: hex.EncodeToString(nonce[:]), Deadline: deadline,
		UID: uid, GID: gid, OverflowUID: view.kernel.overflowUID, OverflowGID: view.kernel.overflowGID,
		Groups:     append([]uint32(nil), view.kernel.groups...),
		Namespaces: view.kernel.namespaces, Devices: view.kernel.devices,
		Home: readonlyWireObject(view.inputs.home), Worktree: readonlyWireObject(view.inputs.worktree),
		Project: readonlyWireObject(view.inputs.project), Executable: readonlyWireObject(view.inputs.executable),
		Self: view.system.selfFacts, Primitive: view.system.primitive.facts, SelfSHA: view.system.selfSHA, PrimitiveSHA: view.system.primitiveSHA,
		Argv: append([]string(nil), command.Argv...), Env: append([]string(nil), command.Env...), Dir: command.Dir}
	for i, root := range view.system.runtime {
		view.policy.Runtime[i] = readonlyWireObject(root.object)
	}
	view.policy.Input, err = censusReadonlyInputs(ctx, view.inputs)
	if err != nil {
		return err
	}
	view.policy.RuntimeProof, err = censusReadonlyRuntime(ctx, view.system.runtime, 0, 0)
	if err != nil {
		return err
	}
	view.policy.NativeSHA, err = hashReadonlyHeld(ctx, view.inputs.executable.file, view.inputs.executable.facts)
	return errors.Join(err, ctx.Err())
}

func readonlyMountedInputs(policy readonlyPolicy) []readonlyPolicyObject {
	if readonlyWithin(policy.Worktree.Path, policy.Home.Path) {
		return []readonlyPolicyObject{policy.Home}
	}
	if readonlyWithin(policy.Home.Path, policy.Worktree.Path) {
		return []readonlyPolicyObject{policy.Worktree}
	}
	return []readonlyPolicyObject{policy.Home, policy.Worktree}
}

func readonlyNativeSeparate(policy readonlyPolicy) bool {
	return !readonlyWithin(policy.Executable.Path, policy.Home.Path) &&
		!readonlyWithin(policy.Executable.Path, policy.Worktree.Path) && !readonlyWithin(policy.Executable.Path, "/usr")
}

func (view *readonlyPreparedView) revalidate(ctx context.Context, complete bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, original := range []readonlyOriginalObject{view.inputs.home, view.inputs.worktree, view.inputs.project, view.inputs.executable, view.system.primitive} {
		if err := original.revalidate(ctx); err != nil {
			return err
		}
	}
	for _, root := range view.system.runtime {
		if err := root.revalidate(ctx); err != nil {
			return err
		}
	}
	if err := errors.Join(view.kernel.revalidate(ctx), requireReadonlyHeld(view.system.self, view.system.selfFacts),
		readonlyNoFileCapability(view.system.primitive.file), readonlyNoFileCapability(view.inputs.executable.file), readonlyNoFileCapability(view.system.self)); err != nil {
		return err
	}
	if err := view.revalidateExecutableBytes(ctx); err != nil {
		return err
	}
	if complete {
		return view.revalidateCensus(ctx)
	}
	return ctx.Err()
}

func (view *readonlyPreparedView) revalidateExecutableBytes(ctx context.Context) error {
	for _, item := range []struct {
		file  *os.File
		facts readonlyObjectFacts
		want  string
	}{{view.system.primitive.file, view.system.primitive.facts, view.policy.PrimitiveSHA},
		{view.inputs.executable.file, view.inputs.executable.facts, view.policy.NativeSHA}} {
		actual, err := hashReadonlyHeld(ctx, item.file, item.facts)
		if err != nil || actual != item.want {
			return errors.Join(err, errors.New("readonly selected executable bytes changed"))
		}
	}
	return nil
}

func (view *readonlyPreparedView) revalidateCensus(ctx context.Context) error {
	input, err := censusReadonlyInputs(ctx, view.inputs)
	if err != nil || input != view.policy.Input {
		return errors.Join(err, errors.New("readonly original input census changed"))
	}
	runtime, err := censusReadonlyRuntime(ctx, view.system.runtime, 0, 0)
	if err != nil || runtime != view.policy.RuntimeProof {
		return errors.Join(err, errors.New("readonly original runtime census changed"))
	}
	return ctx.Err()
}

func (view *readonlyPreparedView) prepare(ctx context.Context, command *exec.Cmd) (*readonlyExportLease, error) {
	lease := newReadonlyExportLease(command)
	if command == nil || len(command.ExtraFiles) != 0 || !reflect.DeepEqual(command.Args, view.policy.Argv) ||
		!reflect.DeepEqual(command.Env, view.policy.Env) || command.Dir != view.policy.Dir {
		return lease, errors.New("readonly process command changed before export")
	}
	if err := view.revalidate(ctx, false); err != nil {
		return lease, err
	}
	policyFD, err := lease.export(view.sealed)
	if err != nil || policyFD != 3 {
		return lease, errors.Join(err, errors.New("readonly private policy descriptor differs"))
	}
	primitiveFD, err := lease.export(view.system.primitive.file)
	if err != nil || primitiveFD != 4 {
		return lease, errors.Join(err, errors.New("readonly primitive descriptor differs"))
	}
	argv, err := view.arguments(lease)
	if err != nil {
		return lease, err
	}
	command.Path = "/proc/self/fd/4"
	command.Args = argv
	command.Env = []string{"LANG=C.UTF-8", "LC_ALL=C.UTF-8", readonlyBootstrapEnvironment}
	command.Dir = "/"
	return lease, ctx.Err()
}

func (view *readonlyPreparedView) arguments(lease *readonlyExportLease) ([]string, error) {
	argv := []string{readonlyPrimitivePath, "--unshare-user", "--uid", strconv.FormatUint(uint64(view.policy.UID), 10),
		"--gid", strconv.FormatUint(uint64(view.policy.GID), 10), "--unshare-pid", "--unshare-net", "--unshare-ipc",
		"--unshare-uts", "--disable-userns", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--tmpfs", "/"}
	for _, root := range view.system.runtime {
		fd, err := lease.export(root.object.file)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--ro-bind-fd", strconv.Itoa(fd), root.visible)
	}
	for _, object := range readonlyMountedInputs(view.policy) {
		original := view.inputs.home
		if object.Path == view.inputs.worktree.path {
			original = view.inputs.worktree
		}
		fd, err := lease.export(original.file)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--ro-bind-fd", strconv.Itoa(fd), object.Path)
	}
	if readonlyNativeSeparate(view.policy) {
		fd, err := lease.export(view.inputs.executable.file)
		if err != nil {
			return nil, err
		}
		argv = append(argv, "--ro-bind-fd", strconv.Itoa(fd), view.policy.Executable.Path)
	}
	fd, err := lease.export(view.system.self)
	if err != nil {
		return nil, err
	}
	argv = append(argv, "--ro-bind-fd", strconv.Itoa(fd), readonlyBootstrapPath, "--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev")
	for _, name := range readonlyDeviceNames() {
		argv = append(argv, "--remount-ro", "/dev/"+name)
	}
	return append(argv, "--remount-ro", "/dev/pts", "--remount-ro", "/dev", "--remount-ro", "/",
		"--chdir", view.policy.Dir, "--", readonlyBootstrapPath, "--agentplugins-readonly-child"), nil
}
