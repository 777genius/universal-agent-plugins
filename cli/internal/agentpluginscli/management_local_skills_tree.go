package agentpluginscli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const localSkillMaxBytes = 10 << 20
const localSkillMaxFiles = 200

// A bounded complete-tree observation detects omitted/deleted files and edits
// made after review. Symlinks and special files are never authoring inputs.
type localSkillFile struct {
	Path   string `json:"path"`
	Mode   uint32 `json:"mode"`
	SHA256 string `json:"sha256"`
	Bytes  []byte `json:"-"`
}
type localSkillDirectory struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
}
type localSkillTree struct {
	Exists      bool                  `json:"exists"`
	Mode        uint32                `json:"mode"`
	Directories []localSkillDirectory `json:"directories"`
	Files       []localSkillFile      `json:"files"`
}

func (t localSkillTree) digest() string {
	body, _ := json.Marshal(t)
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}
func localSkillDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}
func localSkillName(name string) bool {
	return len(name) <= 255 && pathcontract.ValidatePortablePathSegment(name) == nil && !strings.HasPrefix(name, ".uap-local-") && !strings.HasPrefix(name, ".agentplugins-")
}
func localSkillRelative(name string) bool {
	if len(name) > 1024 || path.Clean(name) != name || strings.Count(name, "/") > 16 {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if !localSkillName(segment) {
			return false
		}
	}
	return true
}

// New modes must match what the host can observe after publication. Existing
// modes, including read-only attributes, remain part of the captured tree.
func localSkillDefaultMode(directory bool) uint32 {
	fileMode, directoryMode := uint32(0644), uint32(0755)
	if runtime.GOOS == "windows" {
		fileMode, directoryMode = 0666, 0777
	}
	if directory {
		return directoryMode
	}
	return fileMode
}
func readLocalSkillTree(root *os.Root, name string, keepBytes bool) (localSkillTree, error) {
	tree := localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return tree, nil
	}
	if err != nil {
		return tree, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return tree, errors.New("local Skill must be a real directory")
	}
	tree.Exists = true
	tree.Mode = uint32(info.Mode().Perm())
	walker := localSkillWalker{root: root, base: name, tree: &tree, keep: keepBytes}
	err = fs.WalkDir(root.FS(), name, walker.visit)
	return tree, err
}

type localSkillWalker struct {
	root  *os.Root
	base  string
	tree  *localSkillTree
	keep  bool
	total int64
}

func (w *localSkillWalker) visit(current string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if current == w.base {
		return nil
	}
	relative := strings.TrimPrefix(current, w.base+"/")
	if !localSkillRelative(relative) {
		return errors.New("local Skill contains an unsupported path")
	}
	if entry.IsDir() {
		if len(w.tree.Directories) >= 400 {
			return errors.New("too many local Skill directories")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		w.tree.Directories = append(w.tree.Directories, localSkillDirectory{Path: relative, Mode: uint32(info.Mode().Perm())})
		return nil
	}
	if !entry.Type().IsRegular() || len(w.tree.Files) >= localSkillMaxFiles {
		return errors.New("local Skill contains symbolic links, special files or too many files")
	}
	return w.read(current, relative)
}
func (w *localSkillWalker) read(current, relative string) error {
	file, err := w.root.Open(current)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("local Skill contains a special file")
	}
	if info.Size() > localSkillMaxBytes-w.total {
		return errors.New("local Skill exceeds 10 MiB")
	}
	body, err := io.ReadAll(io.LimitReader(file, localSkillMaxBytes-w.total+1))
	if err != nil {
		return err
	}
	w.total += int64(len(body))
	if w.total > localSkillMaxBytes {
		return errors.New("local Skill exceeds 10 MiB")
	}
	item := localSkillFile{Path: relative, Mode: uint32(info.Mode().Perm()), SHA256: localSkillDigest(body)}
	if w.keep {
		item.Bytes = body
	}
	w.tree.Files = append(w.tree.Files, item)
	return nil
}
func writeLocalSkillTree(root *os.Root, name string, tree localSkillTree) error {
	if err := root.Mkdir(name, 0700); err != nil {
		return err
	}
	return populateLocalSkillTree(root, name, tree)
}
func populateLocalSkillTree(root *os.Root, name string, tree localSkillTree) error {
	for _, directory := range tree.Directories {
		if err := root.MkdirAll(filepath.Join(name, directory.Path), 0700); err != nil {
			return err
		}
	}
	for _, item := range tree.Files {
		filename := filepath.Join(name, item.Path)
		if err := root.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			return err
		}
		file, err := root.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(item.Mode)&0777)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(item.Bytes)
		if writeErr == nil {
			writeErr = file.Chmod(os.FileMode(item.Mode) & 0777)
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for i := len(tree.Directories) - 1; i >= 0; i-- {
		directory := tree.Directories[i]
		if err := root.Chmod(filepath.Join(name, directory.Path), os.FileMode(directory.Mode)&0777); err != nil {
			return err
		}
		if err := syncLocalSkillRoot(root, filepath.Join(name, directory.Path)); err != nil {
			return err
		}
	}
	if err := root.Chmod(name, os.FileMode(tree.Mode)&0777); err != nil {
		return err
	}
	return syncLocalSkillRoot(root, name)
}
func syncLocalSkillRoot(root *os.Root, name string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return file.Sync()
}
func localSkillPathOverlap(a, b string) bool {
	if !filepath.IsAbs(a) || !filepath.IsAbs(b) {
		return false
	}
	inside := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
	}
	return inside(a, b) || inside(b, a)
}
func sortLocalSkillTree(tree *localSkillTree) {
	sort.Slice(tree.Directories, func(i, j int) bool { return tree.Directories[i].Path < tree.Directories[j].Path })
	sort.Slice(tree.Files, func(i, j int) bool { return tree.Files[i].Path < tree.Files[j].Path })
}
