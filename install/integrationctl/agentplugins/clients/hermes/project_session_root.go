package hermes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const projectSessionRootKind = "hermes_project_session_root"

// The marker records source ownership. The existing standalone publication
// receipt separately binds physical root and parent identities; neither owns runtime history.
func projectSessionRoot(project string) string {
	return filepath.Join(project, ".hermes", "uap-session")
}

func sessionRootBody(project string) []byte {
	return []byte("uap-hermes-project-session-v1\n" + project + "\n")
}

func sessionRootObject(project string) domain.NativeObjectOwnership {
	return domain.NativeObjectOwnership{
		ObjectID: projectSessionRootKind, Kind: projectSessionRootKind,
		Path:            filepath.Join(projectSessionRoot(project), ".uap-owner"),
		ManagedDigest:   fmt.Sprintf("%x", sha256.Sum256(sessionRootBody(project))),
		ProtectionClass: "owned",
	}
}

func noAliasDirectory(path string) (os.FileInfo, error) {
	if !cleanRoot(path) {
		return nil, fmt.Errorf("hermes session directory must be canonical and absolute")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil || resolved != path {
		return nil, fmt.Errorf("hermes session directory contains an alias")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("hermes session directory is unavailable")
	}
	return info, nil
}

func verifyProjectSessionRoot(ctx context.Context, project string, owned domain.NativeObjectOwnership) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if owned != sessionRootObject(project) {
		return fmt.Errorf("hermes session root receipt differs")
	}
	root := projectSessionRoot(project)
	before, err := noAliasDirectory(root)
	if err != nil {
		return err
	}
	if err := validateSessionDirectory(before); err != nil {
		return err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	held, err := directory.Stat(".")
	if err != nil || !os.SameFile(before, held) || before.Mode() != held.Mode() {
		return fmt.Errorf("hermes session root changed before marker read")
	}
	if err := validateSessionDirectory(held); err != nil {
		return err
	}
	if err := validateSessionRootHandle(directory); err != nil {
		return err
	}
	body, err := readSessionMember(directory, ".uap-owner", 4096)
	if err != nil {
		return err
	}
	after, err := noAliasDirectory(root)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || !bytes.Equal(body, sessionRootBody(project)) {
		return fmt.Errorf("hermes session root ownership changed")
	}
	return errors.Join(ctx.Err(), validateSessionDirectory(after), validateSessionRootHandle(directory))
}

func validateSessionRootHandle(root *os.Root) (resultErr error) {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return validateSessionDirectoryHandle(file, info)
}

func readSessionMember(root *os.Root, name string, limit int64) (result []byte, resultErr error) {
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() > limit {
		return nil, fmt.Errorf("hermes session member is missing, aliased or oversized")
	}
	file, err := openSessionFile(root, name)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	held, err := file.Stat()
	if err != nil || !os.SameFile(before, held) || held.Mode() != before.Mode() || held.Size() != before.Size() {
		return nil, fmt.Errorf("hermes session member changed before read")
	}
	if err := validateSessionHandle(file, held); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, fmt.Errorf("hermes session member read exceeded bound")
	}
	after, err := root.Lstat(name)
	final, finalErr := file.Stat()
	if err != nil || finalErr != nil || !sameSessionMember(held, after) || !sameSessionMember(held, final) {
		return nil, fmt.Errorf("hermes session member changed during read")
	}
	if err := validateSessionHandle(file, final); err != nil {
		return nil, err
	}
	return body, nil
}

func sameSessionMember(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() &&
		before.ModTime().Equal(after.ModTime())
}
