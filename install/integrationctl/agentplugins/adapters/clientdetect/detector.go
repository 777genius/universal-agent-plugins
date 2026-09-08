package clientdetect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type Detector struct {
	HomeDir                string
	GOOS                   string
	Environment            map[string]string
	SystemApplicationsDir  string
	WindowsProgramFiles    []string
	LinuxApplicationDirs   []string
	LookPath               func(string) (string, error)
	Lstat                  func(string) (fs.FileInfo, error)
	ReadDir                func(string) ([]os.DirEntry, error)
	ProbeVersion           func(context.Context, string) (string, error)
	VersionTimeout         time.Duration
	TargetedVersionTimeout time.Duration
}

func NewOS(homeDir string) Detector {
	environment := environmentSnapshot()
	xdgApplications := ""
	if root := strings.TrimSpace(environment["XDG_DATA_HOME"]); root != "" {
		xdgApplications = filepath.Join(root, "applications")
	}
	userApplications := ""
	if root := strings.TrimSpace(homeDir); root != "" {
		userApplications = filepath.Join(root, ".local", "share", "applications")
	}
	return Detector{
		HomeDir:               homeDir,
		GOOS:                  runtime.GOOS,
		Environment:           environment,
		SystemApplicationsDir: "/Applications",
		WindowsProgramFiles:   compactPaths(environment["ProgramFiles"], environment["ProgramW6432"], environment["ProgramFiles(x86)"]),
		LinuxApplicationDirs: compactPaths(
			xdgApplications,
			userApplications,
			"/usr/local/share/applications", "/usr/share/applications",
		),
		LookPath:               exec.LookPath,
		Lstat:                  os.Lstat,
		ReadDir:                os.ReadDir,
		ProbeVersion:           probeExecutableVersion,
		VersionTimeout:         2 * time.Second,
		TargetedVersionTimeout: 10 * time.Second,
	}
}

func (detector Detector) Detect(ctx context.Context) ([]domain.DetectedClient, error) {
	return detector.detect(ctx, false, nil)
}

// DetectWithVersionProbe is reserved for explicit lifecycle resolution that
// needs the installed client version to bind signed Directory evidence. Detect
// itself is strictly observational and never executes a discovered binary.
func (detector Detector) DetectWithVersionProbe(ctx context.Context) ([]domain.DetectedClient, error) {
	return detector.detect(ctx, true, nil)
}

func (detector Detector) DetectTargetsWithVersionProbe(ctx context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	selected := make(map[domain.ClientID]struct{}, len(targets))
	for _, target := range targets {
		selected[target] = struct{}{}
	}
	timeout := detector.TargetedVersionTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	detector.VersionTimeout = timeout
	return detector.detect(ctx, true, selected)
}

func (detector Detector) detect(ctx context.Context, probeVersion bool, selected map[domain.ClientID]struct{}) ([]domain.DetectedClient, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	probe := func(clientID domain.ClientID) bool {
		if !probeVersion {
			return false
		}
		if selected == nil {
			return true
		}
		_, ok := selected[clientID]
		return ok
	}
	if strings.TrimSpace(detector.HomeDir) == "" {
		return nil, fmt.Errorf("client detector home directory is required")
	}
	if detector.LookPath == nil || detector.Lstat == nil {
		return nil, fmt.Errorf("client detector probes are required")
	}
	clients := make([]domain.DetectedClient, 0, len(domain.SupportedClientIDs()))
	for _, definition := range domain.ClientDefinitions() {
		clients = append(clients, detector.detectClient(ctx, definition, probe(definition.ID)))
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].ClientID < clients[j].ClientID })
	return clients, nil
}

func (detector Detector) detectClient(ctx context.Context, definition domain.ClientDefinition, probeVersion bool) domain.DetectedClient {
	switch definition.ID {
	case domain.ClientCodex:
		return detector.detectCodex(ctx, probeVersion)
	case domain.ClientChatGPT:
		return detector.detectChatGPT(ctx, probeVersion)
	case domain.ClientCursor:
		return detector.detectCursor(ctx, probeVersion)
	case domain.ClientCopilot:
		return detector.detectCopilot(ctx, probeVersion)
	case domain.ClientVSCode:
		return detector.detectVSCode(ctx, probeVersion)
	case domain.ClientKiro:
		return detector.detectKiro(ctx, probeVersion)
	case domain.ClientClaude:
		return detector.detectClaude(ctx, probeVersion)
	case domain.ClientGemini:
		return detector.detectGemini(ctx, probeVersion)
	case domain.ClientOpenCode:
		return detector.detectOpenCode(ctx, probeVersion)
	case domain.ClientCline:
		return detector.detectCline(ctx, probeVersion)
	case domain.ClientWindsurf:
		return detector.detectWindsurf(ctx, probeVersion)
	default:
		return domain.DetectedClient{ClientID: definition.ID, DisplayName: definition.DisplayName, Status: domain.DetectionNotDetected}
	}
}

func (detector Detector) detectCodex(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := filepath.Join(detector.HomeDir, ".codex")
	surfaces := []domain.ClientSurface{
		detector.binarySurface("codex_cli", "codex"),
		detector.directorySurface("codex_config", configRoot),
	}
	if detector.GOOS == "darwin" {
		surfaces = append(surfaces, detector.appSurface("codex_desktop", "Codex.app"))
	} else if detector.GOOS == "linux" {
		surfaces = append(surfaces, detector.linuxDesktopSurface("codex_desktop", "codex.desktop"))
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientCodex, "OpenAI Codex", configRoot, detector.lookup("codex"), surfaces)
}

func (detector Detector) detectChatGPT(ctx context.Context, probeVersion bool) domain.DetectedClient {
	var surfaces []domain.ClientSurface
	switch detector.GOOS {
	case "darwin":
		surfaces = append(surfaces, detector.appSurface("chatgpt_desktop", "ChatGPT.app"))
	case "windows":
		surfaces = append(surfaces, detector.windowsAppSurface(
			"chatgpt_desktop",
			filepath.Join("Microsoft", "WindowsApps", "ChatGPT.exe"),
			filepath.Join("WindowsApps", "ChatGPT.exe"),
		))
	case "linux":
		surfaces = append(surfaces, detector.linuxDesktopSurface("chatgpt_desktop", "chatgpt.desktop"))
	}
	// ChatGPT is a remote/manual host. It intentionally has no executable and
	// does not inherit the Codex CLI or config directory.
	return detector.detectedClient(ctx, probeVersion, domain.ClientChatGPT, "ChatGPT", "", "", surfaces)
}

func (detector Detector) detectCursor(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := filepath.Join(detector.HomeDir, ".cursor")
	surfaces := []domain.ClientSurface{
		detector.binarySurface("cursor_cli", "cursor"),
		detector.directorySurface("cursor_config", configRoot),
	}
	if detector.GOOS == "darwin" {
		surfaces = append(surfaces, detector.appSurface("cursor_desktop", "Cursor.app"))
	} else if detector.GOOS == "windows" {
		surfaces = append(surfaces, detector.windowsAppSurface("cursor_desktop", filepath.Join("Programs", "cursor", "Cursor.exe"), filepath.Join("Cursor", "Cursor.exe")))
	} else if detector.GOOS == "linux" {
		surfaces = append(surfaces, detector.linuxDesktopSurface("cursor_desktop", "cursor.desktop"))
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientCursor, "Cursor", configRoot, detector.lookup("cursor"), surfaces)
}

func (detector Detector) detectCopilot(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := filepath.Join(detector.HomeDir, ".copilot")
	surfaces := []domain.ClientSurface{
		detector.binarySurface("copilot_cli", "copilot"),
		detector.directorySurface("copilot_config", configRoot),
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientCopilot, "GitHub Copilot CLI", configRoot, detector.lookup("copilot"), surfaces)
}

func (detector Detector) detectVSCode(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := detector.vscodeConfigRoot()
	surfaces := []domain.ClientSurface{
		detector.binarySurface("vscode_cli", "code"),
		detector.directorySurface("vscode_config", configRoot),
	}
	if detector.GOOS == "darwin" {
		surfaces = append(surfaces, detector.appSurface("vscode_desktop", "Visual Studio Code.app"))
	} else if detector.GOOS == "windows" {
		surfaces = append(surfaces, detector.windowsAppSurface("vscode_desktop", filepath.Join("Programs", "Microsoft VS Code", "Code.exe"), filepath.Join("Microsoft VS Code", "Code.exe")))
	} else if detector.GOOS == "linux" {
		surfaces = append(surfaces, detector.linuxDesktopSurface("vscode_desktop", "code.desktop", "visual-studio-code.desktop"))
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientVSCode, "Visual Studio Code", configRoot, detector.lookup("code"), surfaces)
}

func (detector Detector) detectKiro(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := filepath.Join(detector.HomeDir, ".kiro")
	kiroCLI := detector.lookup("kiro-cli")
	legacyCLI := detector.lookup("kiro")
	surfaces := []domain.ClientSurface{
		{ID: "kiro_cli", Detected: kiroCLI != "", Evidence: evidence(kiroCLI != "", "executable_on_path")},
		{ID: "kiro_legacy_cli", Detected: legacyCLI != "", Evidence: evidence(legacyCLI != "", "legacy_executable_on_path")},
		detector.directorySurface("kiro_config", configRoot),
	}
	if detector.GOOS == "darwin" {
		surfaces = append(surfaces, detector.appSurface("kiro_desktop", "Kiro.app"))
	} else if detector.GOOS == "windows" {
		surfaces = append(surfaces, detector.windowsAppSurface("kiro_desktop", filepath.Join("Programs", "Kiro", "Kiro.exe"), filepath.Join("Kiro", "Kiro.exe")))
	} else if detector.GOOS == "linux" {
		surfaces = append(surfaces, detector.linuxDesktopSurface("kiro_desktop", "kiro.desktop"))
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientKiro, "Kiro", configRoot, firstPath(kiroCLI, legacyCLI), surfaces)
}

func (detector Detector) detectClaude(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := strings.TrimSpace(detector.Environment["CLAUDE_CONFIG_DIR"])
	if configRoot == "" {
		configRoot = filepath.Join(detector.HomeDir, ".claude")
	}
	surfaces := []domain.ClientSurface{
		detector.binarySurface("claude_cli", "claude"),
		detector.directorySurface("claude_config", configRoot),
	}
	return detector.detectedClientWithSelectionSurfaces(ctx, probeVersion, domain.ClientClaude, "Claude Code", configRoot, detector.lookup("claude"), surfaces, "claude_cli")
}

func (detector Detector) detectGemini(ctx context.Context, probeVersion bool) domain.DetectedClient {
	homeRoot := detector.HomeDir
	if configured := strings.TrimSpace(detector.Environment["GEMINI_CLI_HOME"]); configured != "" {
		homeRoot = configured
	}
	configRoot := filepath.Join(homeRoot, ".gemini")
	surfaces := []domain.ClientSurface{
		detector.binarySurface("gemini_cli", "gemini"),
		detector.directorySurface("gemini_config", configRoot),
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientGemini, "Gemini CLI", configRoot, detector.lookup("gemini"), surfaces)
}

func (detector Detector) detectOpenCode(ctx context.Context, probeVersion bool) domain.DetectedClient {
	configRoot := detector.xdgConfigRoot("opencode")
	surfaces := []domain.ClientSurface{
		detector.binarySurface("opencode_cli", "opencode"),
		detector.directorySurface("opencode_config", configRoot),
	}
	switch detector.GOOS {
	case "darwin":
		surfaces = append(surfaces, detector.appSurface("opencode_desktop", "OpenCode.app"))
	case "windows":
		surfaces = append(surfaces, detector.windowsAppSurface("opencode_desktop", filepath.Join("Programs", "OpenCode", "OpenCode.exe"), filepath.Join("OpenCode", "OpenCode.exe")))
	case "linux":
		surfaces = append(surfaces, detector.linuxDesktopSurface("opencode_desktop", "opencode.desktop"))
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientOpenCode, "OpenCode", configRoot, detector.lookup("opencode"), surfaces)
}

func (detector Detector) detectCline(ctx context.Context, probeVersion bool) domain.DetectedClient {
	clineStorageName := "saoudrizwan.claude-dev"
	vscodeConfig := filepath.Join(detector.vscodeConfigRoot(), "globalStorage", clineStorageName)
	cursorConfig := filepath.Join(detector.cursorEditorConfigRoot(), "globalStorage", clineStorageName)
	surfaces := []domain.ClientSurface{
		detector.extensionSurface("cline_vscode_extension", filepath.Join(detector.HomeDir, ".vscode", "extensions"), "saoudrizwan.claude-dev", "cline.cline"),
		detector.directorySurface("cline_vscode_config", vscodeConfig),
		detector.extensionSurface("cline_cursor_extension", filepath.Join(detector.HomeDir, ".cursor", "extensions"), "saoudrizwan.claude-dev", "cline.cline"),
		detector.directorySurface("cline_cursor_config", cursorConfig),
	}
	// Editor globalStorage is only a detection surface. Cline's shared native
	// configuration and skill roots live under its product-owned home.
	configRoot := filepath.Join(detector.HomeDir, ".cline")
	return detector.detectedClient(ctx, probeVersion, domain.ClientCline, "Cline", configRoot, "", surfaces)
}

func (detector Detector) detectWindsurf(ctx context.Context, probeVersion bool) domain.DetectedClient {
	windsurfCLI := detector.lookup("windsurf")
	devinCLI := detector.lookup("devin")
	stableEditorConfig := detector.editorChannelConfigRoot("Windsurf")
	nextEditorConfig := detector.editorChannelConfigRoot("Windsurf - Next")
	devinEditorConfig := detector.editorChannelConfigRoot("Devin")
	devinLocalConfig := detector.xdgConfigRoot("devin")
	stableConfig := filepath.Join(detector.HomeDir, ".codeium", "windsurf")
	nextConfig := filepath.Join(detector.HomeDir, ".codeium", "windsurf-next")
	insidersConfig := filepath.Join(detector.HomeDir, ".codeium", "windsurf-insiders")
	surfaces := []domain.ClientSurface{
		{ID: "windsurf_cli", Detected: windsurfCLI != "", Evidence: evidence(windsurfCLI != "", "executable_on_path")},
		{ID: "devin_cli", Detected: devinCLI != "", Evidence: evidence(devinCLI != "", "executable_on_path")},
		detector.directorySurface("windsurf_config", stableEditorConfig),
		detector.directorySurface("windsurf_next_config", nextEditorConfig),
		detector.directorySurface("devin_config", devinEditorConfig),
		detector.directorySurface("devin_local_config", devinLocalConfig),
		detector.directorySurface("windsurf_legacy_mcp", stableConfig),
		detector.directorySurface("windsurf_next_legacy_mcp", nextConfig),
		detector.directorySurface("windsurf_insiders_legacy_mcp", insidersConfig),
	}
	switch detector.GOOS {
	case "darwin":
		surfaces = append(surfaces,
			detector.appSurface("windsurf_desktop", "Windsurf.app"),
			detector.appSurface("windsurf_next_desktop", "Windsurf - Next.app"),
			detector.appSurface("windsurf_insiders_desktop", "Windsurf - Insiders.app"),
			detector.appSurface("devin_desktop", "Devin.app"),
		)
	case "windows":
		surfaces = append(surfaces,
			detector.windowsAppSurface("windsurf_desktop", filepath.Join("Programs", "Windsurf", "Windsurf.exe"), filepath.Join("Windsurf", "Windsurf.exe")),
			detector.windowsAppSurface("devin_desktop", filepath.Join("Programs", "Devin", "Devin.exe"), filepath.Join("Devin", "Devin.exe")),
		)
	case "linux":
		surfaces = append(surfaces,
			detector.linuxDesktopSurface("windsurf_desktop", "windsurf.desktop"),
			detector.linuxDesktopSurface("devin_desktop", "devin.desktop"),
		)
	}
	// ConfigRoot is a mutation authority, not merely a detection hint. Select
	// exactly one legacy Cascade channel and never guess a cloud-synced Devin
	// location. Multiple installed channels deliberately require the user to
	// narrow the environment before automatic activation is allowed.
	configRoot := ""
	type channel struct {
		root       string
		surfaceIDs []string
	}
	channels := []channel{
		{root: stableConfig, surfaceIDs: []string{"windsurf_legacy_mcp", "windsurf_config", "windsurf_desktop"}},
		{root: nextConfig, surfaceIDs: []string{"windsurf_next_legacy_mcp", "windsurf_next_config", "windsurf_next_desktop"}},
		{root: insidersConfig, surfaceIDs: []string{"windsurf_insiders_legacy_mcp", "windsurf_insiders_desktop"}},
	}
	var selected []string
	for _, candidate := range channels {
		for _, id := range candidate.surfaceIDs {
			if detectedSurface(surfaces, id) {
				selected = append(selected, candidate.root)
				break
			}
		}
	}
	if len(selected) == 1 {
		configRoot = selected[0]
	}
	return detector.detectedClient(ctx, probeVersion, domain.ClientWindsurf, "Windsurf / Devin", configRoot, firstPath(windsurfCLI, devinCLI), surfaces)
}

func detectedSurface(surfaces []domain.ClientSurface, id string) bool {
	for _, surface := range surfaces {
		if surface.ID == id {
			return surface.Detected
		}
	}
	return false
}

func (detector Detector) vscodeConfigRoot() string {
	switch detector.GOOS {
	case "darwin":
		return filepath.Join(detector.HomeDir, "Library", "Application Support", "Code", "User")
	case "windows":
		if root := strings.TrimSpace(detector.Environment["APPDATA"]); root != "" {
			return filepath.Join(root, "Code", "User")
		}
	default:
		if root := strings.TrimSpace(detector.Environment["XDG_CONFIG_HOME"]); root != "" {
			return filepath.Join(root, "Code", "User")
		}
		return filepath.Join(detector.HomeDir, ".config", "Code", "User")
	}
	return filepath.Join(detector.HomeDir, ".config", "Code", "User")
}

func (detector Detector) cursorEditorConfigRoot() string {
	return detector.editorChannelConfigRoot("Cursor")
}

func (detector Detector) editorChannelConfigRoot(channel string) string {
	switch detector.GOOS {
	case "darwin":
		return filepath.Join(detector.HomeDir, "Library", "Application Support", channel, "User")
	case "windows":
		if root := strings.TrimSpace(detector.Environment["APPDATA"]); root != "" {
			return filepath.Join(root, channel, "User")
		}
	}
	return filepath.Join(detector.xdgConfigBase(), channel, "User")
}

func (detector Detector) xdgConfigRoot(name string) string {
	return filepath.Join(detector.xdgConfigBase(), name)
}

func (detector Detector) xdgConfigBase() string {
	if root := strings.TrimSpace(detector.Environment["XDG_CONFIG_HOME"]); root != "" {
		return root
	}
	return filepath.Join(detector.HomeDir, ".config")
}

func (detector Detector) binarySurface(id, binary string) domain.ClientSurface {
	path := detector.lookup(binary)
	return domain.ClientSurface{ID: id, Detected: path != "", Evidence: evidence(path != "", "executable_on_path")}
}

func (detector Detector) directorySurface(id, path string) domain.ClientSurface {
	detected := detector.realDirectory(path)
	return domain.ClientSurface{ID: id, Detected: detected, Evidence: evidence(detected, "configuration_directory")}
}

func (detector Detector) extensionSurface(id, root string, extensionIDs ...string) domain.ClientSurface {
	readDir := detector.ReadDir
	if readDir == nil {
		readDir = os.ReadDir
	}
	if !detector.realDirectory(root) {
		return domain.ClientSurface{ID: id}
	}
	entries, err := readDir(root)
	if err != nil {
		return domain.ClientSurface{ID: id}
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := strings.ToLower(entry.Name())
		for _, extensionID := range extensionIDs {
			extensionID = strings.ToLower(extensionID)
			if name == extensionID || strings.HasPrefix(name, extensionID+"-") {
				return domain.ClientSurface{ID: id, Detected: true, Evidence: "editor_extension"}
			}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (detector Detector) appSurface(id, appName string) domain.ClientSurface {
	paths := []string{filepath.Join(detector.HomeDir, "Applications", appName)}
	if strings.TrimSpace(detector.SystemApplicationsDir) != "" {
		paths = append(paths, filepath.Join(detector.SystemApplicationsDir, appName))
	}
	for _, path := range paths {
		if detector.realDirectory(path) {
			return domain.ClientSurface{ID: id, Detected: true, Evidence: "application_bundle"}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (detector Detector) windowsAppSurface(id, userRelativePath, systemRelativePath string) domain.ClientSurface {
	paths := []string{}
	if root := strings.TrimSpace(detector.Environment["LOCALAPPDATA"]); root != "" {
		paths = append(paths, filepath.Join(root, userRelativePath))
	}
	for _, root := range detector.WindowsProgramFiles {
		if strings.TrimSpace(root) != "" {
			paths = append(paths, filepath.Join(root, systemRelativePath))
		}
	}
	for _, path := range paths {
		if detector.realFile(path) {
			return domain.ClientSurface{ID: id, Detected: true, Evidence: "application_installation"}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (detector Detector) linuxDesktopSurface(id string, filenames ...string) domain.ClientSurface {
	for _, root := range detector.LinuxApplicationDirs {
		for _, filename := range filenames {
			if detector.realFile(filepath.Join(root, filename)) {
				return domain.ClientSurface{ID: id, Detected: true, Evidence: "desktop_entry"}
			}
		}
	}
	return domain.ClientSurface{ID: id}
}

func (detector Detector) lookup(binary string) string {
	path, err := detector.LookPath(binary)
	if err != nil || strings.TrimSpace(path) == "" {
		return ""
	}
	return path
}

func (detector Detector) realDirectory(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := detector.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func (detector Detector) realFile(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := detector.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0
}

func firstPath(paths ...string) string {
	for _, path := range paths {
		if strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func (detector Detector) detectedClient(ctx context.Context, probeVersion bool, clientID domain.ClientID, displayName, configRoot, executablePath string, surfaces []domain.ClientSurface) domain.DetectedClient {
	return detector.detectedClientWithSelectionSurfaces(ctx, probeVersion, clientID, displayName, configRoot, executablePath, surfaces)
}

// detectedClientWithSelectionSurfaces separates read-only discovery evidence
// from the surfaces that make a client safe to select for lifecycle work. An
// empty selection list means any detected surface is actionable. Clients with
// stricter authority requirements can retain ambient evidence without becoming
// an automatic or explicit mutation target.
func (detector Detector) detectedClientWithSelectionSurfaces(ctx context.Context, probeVersion bool, clientID domain.ClientID, displayName, configRoot, executablePath string, surfaces []domain.ClientSurface, selectionSurfaceIDs ...string) domain.DetectedClient {
	status := domain.DetectionNotDetected
	selectionSurfaces := make(map[string]struct{}, len(selectionSurfaceIDs))
	for _, id := range selectionSurfaceIDs {
		selectionSurfaces[id] = struct{}{}
	}
	for _, surface := range surfaces {
		_, selectable := selectionSurfaces[surface.ID]
		if surface.Detected && (len(selectionSurfaces) == 0 || selectable) {
			status = domain.DetectionDetected
			break
		}
	}
	client := domain.DetectedClient{
		ClientID:       clientID,
		DisplayName:    displayName,
		Status:         status,
		Surfaces:       surfaces,
		ExecutablePath: executablePath,
		ConfigRoot:     configRoot,
	}
	if probeVersion && status == domain.DetectionDetected && executablePath != "" && detector.ProbeVersion != nil {
		timeout := detector.VersionTimeout
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if version, err := detector.ProbeVersion(ctx, executablePath); err == nil {
			client.Version = normalizeVersion(version)
		}
	}
	return client
}

const maximumVersionOutput = 4096

type cappedBuffer struct {
	bytes.Buffer
	remaining int
}

func (buffer *cappedBuffer) Write(value []byte) (int, error) {
	if len(value) > buffer.remaining {
		return 0, fmt.Errorf("version output exceeds %d bytes", maximumVersionOutput)
	}
	buffer.remaining -= len(value)
	return buffer.Buffer.Write(value)
}

func probeExecutableVersion(ctx context.Context, executable string) (string, error) {
	isolatedDir, err := os.MkdirTemp("", "agentplugins-version-probe-")
	if err != nil {
		return "", fmt.Errorf("create isolated version probe directory: %w", err)
	}
	defer os.RemoveAll(isolatedDir)

	output := &cappedBuffer{remaining: maximumVersionOutput}
	command := exec.CommandContext(ctx, executable, "--version")
	command.Dir = isolatedDir
	command.Env = []string{}
	if path := os.Getenv("PATH"); strings.TrimSpace(path) != "" {
		// PATH is the sole inherited variable so /usr/bin/env shebangs can
		// resolve their runtime without exposing HOME, tokens, or credentials.
		command.Env = append(command.Env, "PATH="+path)
	}
	command.Stdin = strings.NewReader("")
	command.Stdout, command.Stderr = output, output
	command.WaitDelay = 100 * time.Millisecond
	if err := command.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func normalizeVersion(value string) string {
	for _, field := range strings.Fields(value) {
		candidate := strings.Trim(field, "vV,;()[]{}")
		if strings.HasSuffix(candidate, ".") {
			candidate = strings.TrimSuffix(candidate, ".")
		}
		parts := strings.SplitN(strings.SplitN(candidate, "+", 2)[0], "-", 2)
		core := strings.Split(parts[0], ".")
		if len(core) < 2 {
			continue
		}
		valid := true
		for _, segment := range core {
			if segment == "" || strings.Trim(segment, "0123456789") != "" {
				valid = false
				break
			}
		}
		if valid {
			return candidate
		}
	}
	return ""
}

func evidence(ok bool, value string) string {
	if ok {
		return value
	}
	return ""
}

func environmentSnapshot() map[string]string {
	values := map[string]string{}
	for _, name := range []string{
		"APPDATA", "LOCALAPPDATA", "ProgramFiles", "ProgramW6432", "ProgramFiles(x86)",
		"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
		"CLAUDE_CONFIG_DIR", "GEMINI_CLI_HOME", "CLINE_DATA_DIR", "CLINE_MCP_SETTINGS_PATH",
	} {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		}
	}
	return values
}

func compactPaths(paths ...string) []string {
	result := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func IsNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
