package sourceacquisition

import (
	"context"
	"io"
)

func (acquirer *Acquirer) BindReporter(reporter Reporter) {
	if acquirer == nil {
		return
	}
	acquirer.Reporter = reporter
}

func (acquirer Acquirer) gitCommand(ctx context.Context, dir string) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		return acquirer.runner().Run(ctx, Command{Dir: dir, Args: args, Progress: acquirer.gitProgressSink(args)})
	}
}

func (acquirer Acquirer) gitProgressSink(args []string) io.Writer {
	if acquirer.Reporter == nil || !gitReportsProgress(args) {
		return nil
	}
	return newGitProgressWriter(acquirer.Reporter)
}

func gitReportsProgress(args []string) bool {
	for _, argument := range args {
		if argument == "fetch" || argument == "checkout" {
			return true
		}
	}
	return false
}

func gitFetchArgs(repoRoot, revision string, progress bool) []string {
	return []string{"-C", repoRoot, "fetch", gitQuietOrProgress(progress), "--depth=1", "--filter=blob:none", "--no-tags", "--no-recurse-submodules", "origin", revision}
}

func gitCheckoutArgs(repoRoot, revision string, progress bool) []string {
	return []string{"-C", repoRoot, "checkout", gitQuietOrProgress(progress), "--detach", revision}
}

func gitQuietOrProgress(progress bool) string {
	if progress {
		return "--progress"
	}
	return "--quiet"
}
