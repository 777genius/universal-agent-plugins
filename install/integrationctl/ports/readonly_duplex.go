package ports

import "os"

// ReadonlyOriginal is one original, still-held input. Info was observed through
// File by the caller before the invocation. The caller keeps File open until
// the invocation returns; a runner never adopts or closes it. Neither a path
// nor this data record alone attests to complete discovery or readonly effects.
type ReadonlyOriginal struct {
	Path string
	File *os.File
	Info os.FileInfo
}

// ReadonlyDuplexInputs names fixed native-discovery roles, not arbitrary mount
// destinations. Project is the original command working directory within the
// original Worktree. The runner admits its own fixed runtime and primitive.
// Home retains the original HOME and all four existing HOME-relative XDG roots.
// The client must establish that these roles contain its influencing inputs;
// the runner refuses an unsupported physical view of those original roles.
type ReadonlyDuplexInputs struct {
	Home       ReadonlyOriginal
	Worktree   ReadonlyOriginal
	Project    ReadonlyOriginal
	Executable ReadonlyOriginal
}
