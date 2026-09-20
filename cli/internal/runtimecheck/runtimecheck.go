package runtimecheck

import (
	"os/exec"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

var LookPath = exec.LookPath
var RunCommand = defaultRunCommand

type DoctorStatus string

const (
	StatusReady          DoctorStatus = "ready"
	StatusNeedsBootstrap DoctorStatus = "needs_bootstrap"
	StatusNeedsBuild     DoctorStatus = "needs_build"
	StatusBlocked        DoctorStatus = "blocked"
)

type Inputs struct {
	Root     string
	Targets  []string
	Launcher *pluginmanifest.Launcher
}

type Project struct {
	Root               string
	Targets            []string
	Lane               string
	Runtime            string
	Entrypoint         string
	LauncherPath       string
	LauncherExists     bool
	LauncherExecutable bool
	Python             PythonShape
	Node               NodeShape
}

type PythonManager string

const (
	PythonManagerVenv         PythonManager = "venv"
	PythonManagerRequirements PythonManager = "requirements"
	PythonManagerUV           PythonManager = "uv"
	PythonManagerPoetry       PythonManager = "poetry"
	PythonManagerPipenv       PythonManager = "pipenv"
)

type PythonEnvSource string

const (
	PythonEnvSourceMissing      PythonEnvSource = "missing"
	PythonEnvSourceRepoLocal    PythonEnvSource = "repo-local .venv"
	PythonEnvSourceManagerOwned PythonEnvSource = "manager-owned env"
	PythonEnvSourceBroken       PythonEnvSource = "broken"
)

type NodeManager string

const (
	NodeManagerNPM  NodeManager = "npm"
	NodeManagerPNPM NodeManager = "pnpm"
	NodeManagerYarn NodeManager = "yarn"
	NodeManagerBun  NodeManager = "bun"
)

type PythonShape struct {
	Manager          PythonManager
	ManagerBinary    string
	ManifestPath     string
	HasVenv          bool
	VenvPath         string
	VenvRunnable     bool
	ProbedEnvPath    string
	ProbeAttempted   bool
	ProbeAvailable   bool
	ReadySource      PythonEnvSource
	ReadyInterpreter string
	VersionOutput    string
	BrokenReason     string
	ManagerAvailable bool
}

type NodeShape struct {
	Manager          NodeManager
	ManagerBinary    string
	PackageJSONPath  string
	PackageJSON      bool
	LauncherTarget   string
	RuntimeTarget    string
	RuntimeTargetOK  bool
	Installed        bool
	UsesBuiltOutput  bool
	IsTypeScript     bool
	TSConfigPath     string
	TSConfig         bool
	OutputDir        string
	BuildScript      string
	StructuralIssue  string
	ManagerAvailable bool
	PackageManager   string
}

type Diagnosis struct {
	Status DoctorStatus
	Reason string
	Next   []string
}

type packageJSON struct {
	Scripts        map[string]string `json:"scripts"`
	PackageManager string            `json:"packageManager"`
}

type tsConfig struct {
	Extends         string `json:"extends"`
	CompilerOptions struct {
		OutDir string `json:"outDir"`
	} `json:"compilerOptions"`
}

type pyProject struct {
	Tool map[string]map[string]any `toml:"tool"`
}
