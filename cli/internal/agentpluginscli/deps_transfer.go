package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type dependencyTransferFile struct {
	path string
	body []byte
	info os.FileInfo
}

type dependencyTransferPair struct {
	directory     string
	directoryInfo os.FileInfo
	files         [2]dependencyTransferFile
	requirements  environment.Requirements
	lock          environment.Lock
	diagnostic    string
}

type dependencyTransfer struct {
	source, destination dependencyTransferPair
	output              dependencyTransferOutput
	prepared            []dependencyAction
}

func newDependencyTransferCommand(app App, opts *options, operation string) *cobra.Command {
	var yes, install bool
	command := &cobra.Command{
		Use: operation + " [directory]", Short: operation + " the exact global dependency agreement",
		Long: "Transfer only uap.toml and uap.lock between the global user environment and an exchange directory (default: current directory). Import never resolves pins. Import with --install requires strict locked preflight to succeed before saving; existing drift can require importing alone followed by install --locked.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			transfer, err := prepareDependencyTransfer(app, opts, operation, args, install)
			if err != nil {
				return err
			}
			if install {
				return runDependencyTransferInstallOutput(cmd, app, opts, &transfer, yes)
			}
			return runDependencyTransfer(cmd, app, opts, &transfer, yes)
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the complete transfer without an interactive prompt")
	if operation == "import" {
		command.Flags().BoolVar(&install, "install", false, "restore the imported agreement using strict locked installation")
	}
	return command
}

func prepareDependencyTransfer(app App, opts *options, operation string, args []string, install bool) (dependencyTransfer, error) {
	transfer := dependencyTransfer{}
	exchange := "."
	if len(args) > 0 {
		if args[0] == "" {
			return transfer, errors.New("exchange directory must not be empty")
		}
		exchange = args[0]
	}
	exchange, err := filepath.Abs(exchange)
	if err != nil {
		return transfer, err
	}
	manifest, _ := app.dependencyPaths()
	global, err := filepath.Abs(filepath.Dir(manifest))
	if err != nil {
		return transfer, err
	}
	source, destination := global, exchange
	if operation == "import" {
		source, destination = exchange, global
	}
	transfer.source, err = readDependencyTransferPair(source, false)
	if err != nil {
		return transfer, err
	}
	if err = transfer.source.validate(app); err != nil {
		return transfer, fmt.Errorf("invalid source agreement: %w", err)
	}
	transfer.destination, err = readDependencyTransferPair(destination, true)
	if err != nil {
		return transfer, err
	}
	if err = transfer.destination.validate(app); err != nil {
		transfer.destination.diagnostic = err.Error()
	}
	transfer.output = newDependencyTransferOutput(transfer, operation, opts.dryRun, install)
	return transfer, nil
}

func readDependencyTransferPair(directory string, allowMissing bool) (dependencyTransferPair, error) {
	pair := dependencyTransferPair{directory: directory}
	info, err := os.Stat(directory)
	if err != nil && (!allowMissing || !os.IsNotExist(err)) {
		return pair, err
	}
	if err == nil && !info.IsDir() {
		return pair, fmt.Errorf("exchange path %s is not a directory", directory)
	}
	pair.directoryInfo = info
	for i, name := range []string{"uap.toml", "uap.lock"} {
		pair.files[i], err = readDependencyTransferFile(filepath.Join(directory, name))
		if err != nil {
			return pair, err
		}
		if !allowMissing && pair.files[i].info == nil {
			return pair, fmt.Errorf("source agreement requires %s", pair.files[i].path)
		}
	}
	return pair, nil
}

func readDependencyTransferFile(path string) (dependencyTransferFile, error) {
	file := dependencyTransferFile{path: path}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return file, nil
	}
	if err != nil {
		return file, err
	}
	if !info.Mode().IsRegular() {
		return file, fmt.Errorf("%s must be a regular non-symlink file", path)
	}
	file.body, err = os.ReadFile(path)
	if err != nil {
		return file, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return file, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(info, after) {
		return file, installer.ErrPlanChanged
	}
	file.info = after
	return file, nil
}

func (pair *dependencyTransferPair) validate(app App) error {
	for _, file := range pair.files {
		if file.info == nil || len(file.body) == 0 {
			return fmt.Errorf("incomplete agreement: %s is missing or empty", file.path)
		}
	}
	requirements, err := readDependencyManifestBytes(pair.files[0].path, pair.files[0].body)
	if err != nil {
		return err
	}
	lock, err := decodeDependencyLockBytes(pair.files[1].body, requirements, false)
	if err != nil {
		return err
	}
	canonicalizeRequirementsFromLock(&requirements, lock)
	if err := validateDependencyTargets(app, requirements); err != nil {
		return err
	}
	pair.requirements, pair.lock = requirements, lock
	return nil
}

func (pair dependencyTransferPair) check() error {
	current, err := readDependencyTransferPair(pair.directory, true)
	if err != nil {
		return fmt.Errorf("%w: %w", installer.ErrPlanChanged, err)
	}
	if pair.directoryInfo != nil && (current.directoryInfo == nil || !os.SameFile(pair.directoryInfo, current.directoryInfo)) {
		return installer.ErrPlanChanged
	}
	if pair.directoryInfo == nil && current.directoryInfo != nil {
		return installer.ErrPlanChanged
	}
	for i := range pair.files {
		if !sameDependencyTransferFile(pair.files[i], current.files[i]) {
			return installer.ErrPlanChanged
		}
	}
	return nil
}

func sameDependencyTransferFile(expected, current dependencyTransferFile) bool {
	if expected.info == nil || current.info == nil {
		return expected.info == nil && current.info == nil
	}
	return os.SameFile(expected.info, current.info) && expected.info.Mode() == current.info.Mode() && bytes.Equal(expected.body, current.body)
}

func (transfer dependencyTransfer) check() error {
	if err := transfer.source.check(); err != nil {
		return err
	}
	return transfer.destination.check()
}
