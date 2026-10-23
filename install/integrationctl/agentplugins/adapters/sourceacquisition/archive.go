package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// AcquireArchive acquires one complete standard package without running its
// content. The caller owns the returned sealed snapshot via packagedigest.Remove;
// the original stream and extraction tree never outlive this call.
func (acquirer Acquirer) AcquireArchive(ctx context.Context, source sourcespec.Archive, expectedArchiveSHA256, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	var empty domain.PackageSnapshot
	coordinates, err := archiveCoordinatesFor(source, expectedArchiveSHA256, expectedTreeDigest)
	if err != nil {
		return empty, err
	}
	ctx, cancel := context.WithTimeout(ctx, archiveTimeout)
	defer cancel()
	workspace, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-archive-*")
	if err != nil {
		return empty, errors.New("create private archive workspace")
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	filename, format, checksum, err := acquirer.archiveRetain(ctx, coordinates, workspace)
	if err != nil {
		return empty, err
	}
	extractionRoot := filepath.Join(workspace, "extracted")
	executables, err := archiveUnpack(ctx, filename, extractionRoot, format)
	if err != nil {
		return empty, err
	}
	selected, logicalPath, err := archiveSelectPackage(extractionRoot, coordinates.packagePath)
	if err != nil {
		return empty, err
	}
	return acquirer.archiveSeal(ctx, coordinates, checksum, extractionRoot, selected, logicalPath, executables)
}

type archiveCoordinates struct {
	endpoint, packagePath, bytePin, treePin string
	explicit                                bool
}

func archiveCoordinatesFor(source sourcespec.Archive, bytePin, treePin string) (archiveCoordinates, error) {
	var empty archiveCoordinates
	packagePath, err := sourcespec.NormalizeSubpath(source.Path)
	if err != nil {
		return empty, err
	}
	for _, checksum := range []string{source.SHA256, bytePin} {
		if checksum != "" && !sourcespec.ValidSHA256(checksum) {
			return empty, errors.New("archive checksum requires 64 lowercase hex characters")
		}
	}
	if source.SHA256 != "" && bytePin != "" && source.SHA256 != bytePin {
		return empty, errors.New("requested and locked archive checksums disagree")
	}
	if bytePin == "" {
		bytePin = source.SHA256
	}
	if treePin != "" && (!strings.HasPrefix(treePin, "sha256:") || !sourcespec.ValidSHA256(strings.TrimPrefix(treePin, "sha256:"))) {
		return empty, errors.New("archive tree pin requires a sha256 digest")
	}
	endpoint, remote, err := archiveEndpoint(source.URL)
	if err != nil {
		return empty, err
	}
	return archiveCoordinates{endpoint: endpoint, packagePath: packagePath, bytePin: bytePin, treePin: treePin,
		explicit: !remote || archiveExplicit(endpoint) || bytePin != ""}, nil
}

func archiveEndpoint(endpoint string) (string, bool, error) {
	remote := strings.Contains(endpoint, "://")
	var err error
	if remote {
		endpoint, err = sourcespec.NormalizeArchiveURL(endpoint)
	} else if endpoint != "" {
		endpoint, err = filepath.Abs(endpoint)
		endpoint = filepath.Clean(endpoint)
	} else {
		err = errors.New("archive source is empty")
	}
	if err != nil {
		return "", false, errors.New("archive source requires a public HTTPS endpoint or explicit local file")
	}
	return endpoint, remote, nil
}

func (acquirer Acquirer) archiveRetain(ctx context.Context, coordinates archiveCoordinates, workspace string) (string, archiveFormat, string, error) {
	filename := filepath.Join(workspace, "download")
	format, checksum, err := acquirer.archiveDownload(ctx, coordinates.endpoint, filename, coordinates.explicit)
	if err != nil {
		return "", format, "", err
	}
	if coordinates.bytePin != "" && checksum != coordinates.bytePin {
		return "", format, "", errors.New("archive original-byte SHA256 does not match the requested or locked checksum")
	}
	if format == archiveGzip {
		tarFilename := filepath.Join(workspace, "stream.tar")
		if err := archiveGunzip(ctx, filename, tarFilename); err != nil {
			return "", format, "", err
		}
		filename, format = tarFilename, archiveTAR
	}
	return filename, format, checksum, nil
}

func archiveUnpack(ctx context.Context, filename, extractionRoot string, format archiveFormat) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, errors.New("open retained archive stream")
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, errors.New("inspect retained archive stream")
	}
	if err := os.Mkdir(extractionRoot, 0o700); err != nil {
		return nil, errors.New("create private archive extraction root")
	}
	root, err := os.OpenRoot(extractionRoot)
	if err != nil {
		return nil, errors.New("open contained archive extraction root")
	}
	defer func() { _ = root.Close() }()
	output := &archiveExtraction{root: root, policy: newArchivePolicy()}
	switch format {
	case archiveZIP:
		err = archiveExtractZIP(ctx, file, info.Size(), output)
	case archiveTAR:
		err = archiveExtractTAR(ctx, file, info.Size(), output)
	default:
		err = errors.New("unsupported archive format")
	}
	return output.executables, err
}

func (acquirer Acquirer) archiveSeal(ctx context.Context, coordinates archiveCoordinates, checksum, extractionRoot, selected, logicalPath string, executablePaths []string) (domain.PackageSnapshot, error) {
	source := domain.SourceIdentity{
		SourceKind: "archive", SourceURL: coordinates.endpoint, ArchiveSHA256: checksum,
		RequestedSource: coordinates.endpoint, CanonicalSource: sourcespec.Key(sourcespec.KindArchive, coordinates.endpoint, logicalPath),
		PackageSubpath: logicalPath, ResolvedRevision: "sha256:" + checksum, SourceBindingHint: "direct-archive",
	}
	prefix, err := filepath.Rel(extractionRoot, selected)
	if err != nil {
		return domain.PackageSnapshot{}, errors.New("resolve selected archive package")
	}
	prefix = filepath.ToSlash(prefix)
	if prefix == "." {
		prefix = ""
	} else {
		prefix += "/"
	}
	executables := make([]string, 0)
	for _, executable := range executablePaths {
		if strings.HasPrefix(executable, prefix) {
			executables = append(executables, strings.TrimPrefix(executable, prefix))
		}
	}
	snapshot, err := acquirer.digester().SnapshotWithExecutables(ctx, selected, source, executables)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("seal acquired archive package: %w", err)
	}
	if coordinates.treePin != "" && snapshot.TreeDigest != coordinates.treePin {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("archive package tree digest does not match the locked digest")
	}
	snapshot.AcquiredAt = acquirer.now()
	return snapshot, nil
}

type archiveExtraction struct {
	root        *os.Root
	policy      *archivePolicy
	executables []string
}

func (output *archiveExtraction) write(ctx context.Context, raw string, directory bool, size int64, executable bool, body io.Reader) error {
	name, err := output.policy.entry(raw, directory, size)
	if err != nil {
		return err
	}
	if directory {
		if name != "" {
			if err := output.root.MkdirAll(filepath.FromSlash(name), 0o700); err != nil {
				return errors.New("create archive directory")
			}
		}
		var probe [1]byte
		if n, err := body.Read(probe[:]); n != 0 || err != io.EOF {
			return errors.New("invalid archive directory payload or checksum")
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := output.root.MkdirAll(filepath.FromSlash(path.Dir(name)), 0o700); err != nil {
		return errors.New("create archive file parent")
	}
	// No links are ever created in this private root. Rooted exclusive writes
	// protect containment and reject overwrites independently of lexical policy.
	file, err := output.root.OpenFile(filepath.FromSlash(name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("create contained archive file")
	}
	n, copyErr := io.Copy(file, io.LimitReader(&archiveContextReader{ctx: ctx, reader: body}, size+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || n != size {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("archive payload size/checksum disagrees with its header")
	}
	if executable {
		output.executables = append(output.executables, name)
	}
	return nil
}

func archiveSelectPackage(extracted, requested string) (string, string, error) {
	logicalRoot := extracted
	if !archiveManifestPresent(logicalRoot) {
		entries, err := os.ReadDir(extracted)
		if err != nil {
			return "", "", errors.New("inspect archive package roots")
		}
		if len(entries) == 1 && entries[0].IsDir() {
			logicalRoot = filepath.Join(extracted, entries[0].Name())
		}
	}
	if requested != "" {
		selected := filepath.Join(logicalRoot, filepath.FromSlash(requested))
		if !archiveManifestAt(selected) {
			return "", "", errors.New("selected archive path has no standard plugin.json")
		}
		return selected, requested, nil
	}
	if archiveManifestPresent(logicalRoot) {
		return logicalRoot, "", nil
	}
	candidates, err := archiveDiscoverPackages(logicalRoot)
	if err != nil {
		return "", "", err
	}
	if len(candidates) == 1 {
		return filepath.Join(logicalRoot, filepath.FromSlash(candidates[0])), candidates[0], nil
	}
	if len(candidates) == 0 {
		return "", "", errors.New("archive contains no complete standard Agent Plugins package")
	}
	return "", "", fmt.Errorf("archive contains multiple packages; choose an explicit path: %s", strings.Join(candidates, ", "))
}

func archiveDiscoverPackages(logicalRoot string) ([]string, error) {
	var candidates []string
	err := filepath.WalkDir(logicalRoot, func(filename string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "plugin.json" {
			return nil
		}
		parent := filepath.Dir(filename)
		if !archivePackageShaped(parent) {
			return nil
		}
		relative, err := filepath.Rel(logicalRoot, parent)
		if err != nil {
			return err
		}
		candidates = append(candidates, filepath.ToSlash(relative))
		if len(candidates) > 16 {
			return errors.New("archive has more than 16 package candidates; choose an explicit path")
		}
		return nil
	})
	sort.Strings(candidates)
	return candidates, err
}

func archiveManifestAt(root string) bool {
	info, err := os.Stat(filepath.Join(root, "plugin.json"))
	return err == nil && info.Mode().IsRegular()
}

func archiveManifestPresent(root string) bool {
	_, err := os.Lstat(filepath.Join(root, "plugin.json"))
	return err == nil
}

func archivePackageShaped(parent string) bool {
	mcp, mcpErr := os.Stat(filepath.Join(parent, "mcp.json"))
	skills, skillsErr := os.Stat(filepath.Join(parent, "skills"))
	return mcpErr == nil && mcp.Mode().IsRegular() || skillsErr == nil && skills.IsDir()
}
