package loader

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// withCommands records the package's commands, checked against every name
// under skills/, on a freshly loaded envelope.
func withCommands(ctx context.Context, envelope domain.PackageEnvelope, root string) (domain.PackageEnvelope, error) {
	envelope.SkillsDirEntries = skillsDirEntries(filepath.Join(root, "skills"))
	return commandext.WithCommands(ctx, envelope, readCommandListing(root))
}

// readCommandListing observes the commands directory without following any
// link and reads each candidate file only up to one byte past the format
// limit. Interpretation is left to commandext.
func readCommandListing(root string) commandext.Listing {
	namespace := filepath.Join(root, domain.ExtensionNamespace)
	if state := directoryState(namespace); state != commandext.RootDirectory {
		return commandext.Listing{Root: state}
	}
	directory := filepath.Join(namespace, domain.CommandsDir)
	if state := directoryState(directory); state != commandext.RootDirectory {
		return commandext.Listing{Root: state}
	}
	handle, err := os.Open(directory)
	if err != nil {
		return commandext.Listing{Root: commandext.RootInvalid}
	}
	defer func() { _ = handle.Close() }()
	limits := conformance.DefaultLimits()
	entries, err := handle.ReadDir(limits.Members + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return commandext.Listing{Root: commandext.RootInvalid}
	}
	listing := commandext.Listing{Root: commandext.RootDirectory, Truncated: len(entries) > limits.Members}
	for _, entry := range entries {
		listing.Entries = append(listing.Entries, commandEntry(directory, entry, int64(limits.SkillBytes)+1))
	}
	return listing
}

func directoryState(path string) commandext.RootState {
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return commandext.RootAbsent
	case err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0:
		return commandext.RootInvalid
	}
	return commandext.RootDirectory
}

func commandEntry(directory string, entry os.DirEntry, limit int64) commandext.Entry {
	result := commandext.Entry{Name: entry.Name()}
	switch mode := entry.Type(); {
	case mode&os.ModeSymlink != 0:
		result.Kind = commandext.EntrySymlink
	case mode.IsDir():
		result.Kind = commandext.EntryDirectory
	case !mode.IsRegular():
		result.Kind = commandext.EntryOther
	case strings.HasPrefix(entry.Name(), "."):
		// Ignored by the format, so never read.
	default:
		body, err := readBounded(filepath.Join(directory, entry.Name()), limit)
		result.Body, result.Unreadable = body, err != nil
	}
	return result
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(io.LimitReader(file, limit))
}

// skillsDirEntries lists every physical name under a real skills directory.
// A skills path that is not one is removed before delivery, so it takes no
// names.
func skillsDirEntries(root string) []string {
	if directoryState(root) != commandext.RootDirectory {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
