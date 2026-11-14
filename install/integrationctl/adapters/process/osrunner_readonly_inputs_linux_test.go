//go:build linux

package process

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type readonlyAlteredInfo struct {
	os.FileInfo
	stat syscall.Stat_t
}

func (info readonlyAlteredInfo) Sys() any { return &info.stat }

func TestReadonlyOriginalRefusesSameObjectChangedOwnerGroupOrLinksBeforeCapture(t *testing.T) {
	for _, field := range []string{"owner", "group", "links"} {
		t.Run(field, func(t *testing.T) {
			path := t.TempDir()
			original, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = original.Close() }()
			info, err := original.Stat()
			if err != nil {
				t.Fatal(err)
			}
			altered := readonlyAlteredInfo{FileInfo: info, stat: *info.Sys().(*syscall.Stat_t)}
			uid, gid := altered.stat.Uid, altered.stat.Gid
			switch field {
			case "owner":
				altered.stat.Uid++
				uid = altered.stat.Uid
			case "group":
				altered.stat.Gid++
				gid = altered.stat.Gid
			case "links":
				altered.stat.Nlink++
			}
			_, err = admitReadonlyOriginal(t.Context(), ports.ReadonlyOriginal{Path: path, File: original, Info: altered}, true, uid, gid)
			if !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatal("same original identity re-baselined its metadata", err)
			}
			after, err := original.Stat()
			if err != nil || !os.SameFile(info, after) || *info.Sys().(*syscall.Stat_t) != *after.Sys().(*syscall.Stat_t) {
				t.Fatal("metadata refusal changed or closed the borrowed directory", err)
			}
		})
	}
}

func TestReadonlyOriginalNamedReplacementCannotAdoptSameBytes(t *testing.T) {
	original := readonlyBorrowedFile(t)
	info, err := original.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original.Name(), original.Name()+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original.Name(), []byte("original authority"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = admitReadonlyOriginal(t.Context(), ports.ReadonlyOriginal{Path: original.Name(), File: original, Info: info}, false, uint32(os.Getuid()), uint32(os.Getgid()))
	if !errors.Is(err, directoryidentity.ErrChanged) {
		t.Fatal("same-byte replacement was adopted", err)
	}
	requireReadonlyBorrowStillOpen(t, original)
	body, err := os.ReadFile(original.Name())
	if err != nil || string(body) != "original authority" {
		t.Fatal("refusal changed the named replacement", err)
	}
}

func TestReadonlyInputRoleAndCanceledRequestsRefuseBeforeOpening(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := admitReadonlyInputs(ctx, ports.Command{}, ports.ReadonlyDuplexInputs{}); !errors.Is(err, context.Canceled) {
		t.Fatal("original cancellation was lost", err)
	}
	for _, inputs := range []ports.ReadonlyDuplexInputs{
		{},
		{Executable: ports.ReadonlyOriginal{Path: "/selected/executable"}, Project: ports.ReadonlyOriginal{Path: "/outside"}, Worktree: ports.ReadonlyOriginal{Path: "/original"}},
		{Executable: ports.ReadonlyOriginal{Path: "/different"}, Project: ports.ReadonlyOriginal{Path: "/original"}, Worktree: ports.ReadonlyOriginal{Path: "/original"}},
	} {
		_, err := admitReadonlyInputs(t.Context(), ports.Command{Argv: []string{"/selected/executable"}, Dir: "/original"}, inputs)
		if err == nil {
			t.Fatal("missing or unrelated original role was admitted")
		}
	}
}

func TestReadonlyCensusRefusesSpecialAliasOrWritableInput(t *testing.T) {
	for _, fault := range []string{"none", "fifo", "symlink", "hardlink", "writable"} {
		t.Run(fault, func(t *testing.T) {
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, "ordinary"), []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch fault {
			case "fifo":
				err = unix.Mkfifo(filepath.Join(path, "special"), 0600)
			case "symlink":
				err = os.Symlink("ordinary", filepath.Join(path, "alias"))
			case "hardlink":
				err = os.Link(filepath.Join(path, "ordinary"), filepath.Join(path, "alias"))
			case "writable":
				err = os.Chmod(filepath.Join(path, "ordinary"), 0620)
			}
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = file.Close() }()
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			facts, err := readonlyFacts(info)
			if err != nil {
				t.Fatal(err)
			}
			census := readonlyInputCensus{rows: make(map[string]readonlyObjectFacts), uid: facts.UID, gid: facts.GID}
			err = census.directory(t.Context(), file, path, facts, 0)
			if (fault == "none") != (err == nil) {
				t.Fatal("ordinary input/special-member decision differs", fault, err)
			}
			if fault == "none" && len(census.rows) != 2 {
				t.Fatal("ordinary input census omitted a real member", census.rows)
			}
			body, err := os.ReadFile(filepath.Join(path, "ordinary"))
			if err != nil || string(body) != "unchanged" {
				t.Fatal("metadata-only census changed original file bytes", err)
			}
		})
	}
}

func TestReadonlyCensusOpenDoesNotAdvanceBorrowedDirectoryOffset(t *testing.T) {
	path := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	original, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = original.Close() }()
	first, err := original.ReadDir(1)
	if err != nil || len(first) != 1 {
		t.Fatal("original iterator setup failed", err)
	}
	iterator, err := openReadonlyDirectory(original, ".")
	if err != nil {
		t.Fatal(err)
	}
	all, readErr := iterator.ReadDir(-1)
	if err := errors.Join(readErr, iterator.Close()); err != nil || len(all) != 3 {
		t.Fatal("readonly iterator shared the original file description", err, len(all))
	}
	rest, err := original.ReadDir(-1)
	if err != nil || len(rest) != 2 {
		t.Fatal("readonly iterator consumed borrowed owner state", err, len(rest))
	}
}

func TestReadonlyCensusCountCancellationAndDigestKeepFiniteOriginalFacts(t *testing.T) {
	original := readonlyBorrowedFile(t)
	info, err := original.Stat()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		t.Fatal(err)
	}
	census := readonlyInputCensus{rows: make(map[string]readonlyObjectFacts), uid: facts.UID, gid: facts.GID}
	for i := range readonlyInputEntryLimit {
		census.rows[filepath.Join("/original", strconv.Itoa(i))] = facts
	}
	if err := census.record(original.Name(), facts); err == nil || len(census.rows) != readonlyInputEntryLimit {
		t.Fatal("census bound changed the admitted set", err)
	}
	rows := map[string]readonlyObjectFacts{original.Name(): facts}
	before, err := digestReadonlyCensus(t.Context(), rows)
	if err != nil || before.Count != 1 || len(before.SHA256) != 64 {
		t.Fatal("original census digest differs", before, err)
	}
	changed := facts
	changed.Ctime.Nsec = (changed.Ctime.Nsec + 1) % 1e9
	rows[original.Name()] = changed
	after, err := digestReadonlyCensus(t.Context(), rows)
	if err != nil || after.SHA256 == before.SHA256 {
		t.Fatal("same-name original metadata drift was omitted", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := digestReadonlyCensus(ctx, rows); !errors.Is(err, context.Canceled) {
		t.Fatal("census renewed the original context", err)
	}
	requireReadonlyBorrowStillOpen(t, original)
}

type readonlyOpaqueInfo struct {
	os.FileInfo
	system any
}

func (info readonlyOpaqueInfo) Sys() any { return info.system }

func TestReadonlyOriginalMalformedInfoRefusesWithoutClosingBorrow(t *testing.T) {
	original := readonlyBorrowedFile(t)
	info, err := original.Stat()
	if err != nil {
		t.Fatal(err)
	}
	var absentStat *syscall.Stat_t
	for _, malformed := range []os.FileInfo{
		nil,
		readonlyOpaqueInfo{FileInfo: info, system: nil},
		readonlyOpaqueInfo{FileInfo: info, system: absentStat},
		readonlyOpaqueInfo{FileInfo: info, system: false},
	} {
		_, err := admitReadonlyOriginal(t.Context(), ports.ReadonlyOriginal{Path: original.Name(), File: original, Info: malformed}, false, uint32(os.Getuid()), uint32(os.Getgid()))
		if err == nil {
			t.Fatal("malformed original metadata was admitted")
		}
		requireReadonlyBorrowStillOpen(t, original)
	}
}

func readonlyCompleteInputs(t *testing.T) (ports.Command, ports.ReadonlyDuplexInputs) {
	t.Helper()
	root := t.TempDir()
	home, worktree := filepath.Join(root, "home"), filepath.Join(root, "worktree")
	project, executable := filepath.Join(worktree, "project"), filepath.Join(root, "bin", "native")
	for _, directory := range []string{filepath.Join(home, ".grok"), filepath.Dir(executable), project} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, mode := range map[string]os.FileMode{
		filepath.Join(home, ".grok", "config.toml"): 0600,
		filepath.Join(project, "original"):          0600,
		executable:                                  0700,
	} {
		if err := os.WriteFile(path, []byte("original authority"), mode); err != nil {
			t.Fatal(err)
		}
	}
	inputs := ports.ReadonlyDuplexInputs{}
	for _, role := range []struct {
		path string
		out  *ports.ReadonlyOriginal
	}{{home, &inputs.Home}, {worktree, &inputs.Worktree}, {project, &inputs.Project}, {executable, &inputs.Executable}} {
		file, err := os.Open(role.path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		*role.out = ports.ReadonlyOriginal{Path: role.path, File: file, Info: info}
	}
	return ports.Command{Argv: []string{executable, "agent", "--no-leader", "stdio"}, Dir: project, Env: []string{"HOME=" + home}}, inputs
}

func TestReadonlyCompleteInputAdmissionKeepsOriginalAuthoritiesAndOffsets(t *testing.T) {
	command, inputs := readonlyCompleteInputs(t)
	if entries, err := inputs.Worktree.File.ReadDir(1); err != nil || len(entries) != 1 {
		t.Fatal("borrowed directory iterator setup differs", err)
	}
	if _, err := inputs.Executable.File.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	admitted, err := admitReadonlyInputs(t.Context(), command, inputs)
	if err != nil {
		t.Fatal("actual original P1 capture/admission failed", err)
	}
	for _, role := range []readonlyOriginalObject{admitted.home, admitted.worktree, admitted.project, admitted.executable} {
		if role.parent.IsZero() || role.revalidate(t.Context()) != nil {
			t.Fatal("full input admission lacks its original physical ancestry")
		}
	}
	proof, err := censusReadonlyInputs(t.Context(), admitted)
	if err != nil || proof.Count != 6 || len(proof.SHA256) != 64 {
		t.Fatal("complete original fixed-role census differs", proof, err)
	}
	if rest, err := inputs.Worktree.File.ReadDir(-1); err != nil || len(rest) != 0 {
		t.Fatal("census changed the caller's existing directory offset", err)
	}
	if offset, err := inputs.Executable.File.Seek(0, io.SeekCurrent); err != nil || offset != 3 {
		t.Fatal("admission changed the caller's executable offset", offset, err)
	}
	for _, original := range []ports.ReadonlyOriginal{inputs.Home, inputs.Worktree, inputs.Project, inputs.Executable} {
		current, err := original.File.Stat()
		if err != nil || !os.SameFile(original.Info, current) || current.Mode() != original.Info.Mode() {
			t.Fatal("admission adopted, closed or changed a borrowed original", err)
		}
	}
	requireReadonlyBorrowStillOpen(t, inputs.Executable.File)
}

func TestReadonlyReturnedAuthorityRefusesLateReplacementMetadataAndParentDrift(t *testing.T) {
	for _, fault := range []string{"same-byte-replacement", "mode-drift", "parent-replacement-with-original-child"} {
		t.Run(fault, func(t *testing.T) {
			command, inputs := readonlyCompleteInputs(t)
			admitted, err := admitReadonlyInputs(t.Context(), command, inputs)
			if err != nil {
				t.Fatal("actual original P1 capture/admission failed", err)
			}
			if _, err := censusReadonlyInputs(t.Context(), admitted); err != nil {
				t.Fatal("original census prerequisite failed", err)
			}
			selected := admitted.executable
			switch fault {
			case "same-byte-replacement":
				if err := os.Rename(inputs.Executable.Path, inputs.Executable.Path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(inputs.Executable.Path, []byte("original authority"), 0700); err != nil {
					t.Fatal(err)
				}
			case "mode-drift":
				if err := os.Chmod(inputs.Executable.Path, 0400); err != nil {
					t.Fatal(err)
				}
			case "parent-replacement-with-original-child":
				old := inputs.Worktree.Path + "-original"
				if err := os.Rename(inputs.Worktree.Path, old); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(inputs.Worktree.Path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(old, "project"), inputs.Project.Path); err != nil {
					t.Fatal(err)
				}
				selected = admitted.project
			}
			if err := selected.revalidate(t.Context()); !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatal("returned original authority adopted late drift", fault, err)
			}
			if _, err := censusReadonlyInputs(t.Context(), admitted); err == nil {
				t.Fatal("whole input observation accepted late original drift")
			}
			requireReadonlyBorrowStillOpen(t, inputs.Executable.File)
		})
	}
}
