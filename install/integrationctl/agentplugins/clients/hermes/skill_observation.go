package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This is native completion, not a passive read: the official launcher owns
// bootstrap recovery, dependency selection and its generation lease. The
// catalog uses Hermes' discovery and quarantine gates without skill_view,
// Skill preprocessing, secret capture or per-Skill dependency installation.
const hermesSkillBootstrap = `import os, sys
from pathlib import Path
root = Path(sys.argv[1])
sys.path.insert(0, str(root))
from hermes_cli._launchers import runtime_command
argv = runtime_command(root, python=sys.executable, args=sys.argv[3:], code=sys.argv[2])
if os.name == "nt":
    # Windows execv concatenates arguments without quoting inline Python.
    # The supervised parent waits and preserves the selected child's status.
    import subprocess
    raise SystemExit(subprocess.call(argv))
os.execv(argv[0], argv)
`

const hermesSkillObservation = `import hashlib, json, sys
from pathlib import Path
from tools import skills_tool
from agent.skill_utils import find_project_root, is_project_root_trusted
root = Path(sys.argv[2])
if Path(skills_tool.__file__).resolve().parents[1] != root:
    raise RuntimeError("native Hermes source differs")
expected = json.loads(sys.argv[1])
selected = set(expected.values())
captured = {}
read_skill_text = skills_tool._read_skill_text
def capture_skill_text(path):
    text = read_skill_text(path)
    key = str(path.resolve())
    if key in selected:
        body = text.encode("utf-8")
        if len(body) > 1 << 20 or key in captured:
            captured[key] = None
        else:
            captured[key] = hashlib.sha256(body).hexdigest()
    return text
skills_tool._read_skill_text = capture_skill_text
try:
    catalog = skills_tool._skill_catalog()
finally:
    skills_tool._read_skill_text = read_skill_text
rows = []
for entry in catalog:
    if entry["load_name"] in expected and entry["status"] == "unique":
        path = entry["path"].resolve()
        if str(path) != expected[entry["load_name"]] or not captured.get(str(path)):
            continue
        rows.append({"name": entry["load_name"], "path": str(path),
                     "digest": captured[str(path)], "tier": entry["tier"]})
project = find_project_root()
print(json.dumps({"profile": str(skills_tool.get_hermes_home().resolve()),
                  "project": str(project.resolve()) if project else "",
                  "project_trusted": bool(project and is_project_root_trusted(project)), "skills": rows}))
`

func observeHermesSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, cwd string, expected map[string]hermesSkillMetadata) (bool, error) {
	executable := request.Client.ExecutablePath
	if !cleanRoot(executable) || len(expected) == 0 || len(expected) > 256 {
		return false, nil
	}
	version, err := hermesSkillCommand(ctx, env, cwd, []string{executable, "--version"})
	if err != nil {
		return false, err
	}
	root, err := hermesSkillSource(executable, version)
	if err != nil {
		return false, err
	}
	profile, err := hermesSkillCommand(ctx, env, cwd, []string{executable, "config", "path"})
	if err != nil {
		return false, err
	}
	wanted := filepath.Join(request.Client.ConfigRoot, "config.yaml")
	if string(profile) != wanted+"\n" && string(profile) != wanted+"\r\n" {
		return false, nil
	}
	encoded, err := hermesSkillCommand(ctx, env, cwd, []string{executable, "--print-runtime-command"})
	if err != nil {
		return false, err
	}
	var native []string
	if json.Unmarshal(encoded, &native) != nil || len(native) != 4 || !cleanRoot(native[0]) || native[1] != "-I" || native[2] != "-c" || native[3] == "" {
		return false, fmt.Errorf("hermes native runtime command is unavailable")
	}
	paths := make(map[string]string, len(expected))
	for name, skill := range expected {
		paths[name] = skill.path
	}
	selection, err := json.Marshal(paths)
	if err != nil {
		return false, err
	}
	body, err := hermesSkillCommand(ctx, env, cwd, []string{
		native[0], "-I", "-c", hermesSkillBootstrap, root, hermesSkillObservation, string(selection), root,
	})
	if err != nil {
		return false, err
	}
	return hermesSkillsObserved(body, request, expected), nil
}

func hermesSkillCommand(ctx context.Context, env clients.Env, cwd string, argv []string) ([]byte, error) {
	selection := clients.SessionPlan{Environment: map[string]string{"TERMINAL_CWD": cwd}}
	result, err := shared.RunNativeRegistry(ctx, env.Runner, legacyports.Command{
		Argv: argv, Dir: cwd, Env: selection.ChildEnvironment(os.Environ()), StdoutLimitBytes: 1 << 20,
	})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 || len(result.Stderr) != 0 || len(result.Stdout) >= 1<<20 {
		return nil, fmt.Errorf("hermes native Skill observation unavailable")
	}
	return result.Stdout, nil
}

func hermesSkillSource(executable string, version []byte) (string, error) {
	lines := strings.Split(strings.ReplaceAll(string(version), "\r\n", "\n"), "\n")
	root := ""
	for _, line := range lines {
		if value, ok := strings.CutPrefix(line, "Install directory: "); ok {
			if root != "" {
				return "", fmt.Errorf("hermes native installation is ambiguous")
			}
			root = value
		}
	}
	if !cleanRoot(root) {
		return "", fmt.Errorf("hermes native installation directory unavailable")
	}
	physical, err := filepath.EvalSymlinks(root)
	if err != nil || physical != root {
		return "", fmt.Errorf("hermes native installation directory is redirected")
	}
	if lines[0] == "Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d" {
		return root, nil
	}
	bundle, err := qualifyHermesBundle(executable)
	if err != nil || hermesInstallDirectory(version, filepath.Join(bundle.root, "hermes-agent")) != nil {
		return "", fmt.Errorf("hermes native Skill resolver revision is unqualified")
	}
	return root, nil
}

func hermesSkillsObserved(body []byte, request domain.ActivationRequest, expected map[string]hermesSkillMetadata) bool {
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil || len(doc) != 4 || doc["profile"] != request.Client.ConfigRoot {
		return false
	}
	project, trusted, tier := "", false, json.Number("1")
	if request.Plan.Scope == domain.ScopeProject {
		project, trusted, tier = request.Plan.TargetContext.ProjectRoot, true, json.Number("0")
	}
	if doc["project"] != project || doc["project_trusted"] != trusted {
		return false
	}
	rows, ok := doc["skills"].([]any)
	if !ok || len(rows) != len(expected) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 4 {
			return false
		}
		name, named := row["name"].(string)
		skill, selected := expected[name]
		if !named || !selected || seen[name] || row["path"] != skill.path || row["digest"] != skill.digest || row["tier"] != tier {
			return false
		}
		seen[name] = true
	}
	return true
}
