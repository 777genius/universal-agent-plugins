#!/usr/bin/env python3
"""Native Hermes observers and the existing bounded loopback MCP fixture."""
import json
import os
from pathlib import Path
from contextlib import suppress
import signal
import subprocess
import sys
import tempfile

HERMES_SHA = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"
USER_SKILL = "native-smoke-user"
PROJECT_SKILL = "native-smoke-project"
FOREIGN_SKILL = "native-smoke-foreign"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def observe_skills(user_marker, project_count):
    # runtime_command uses the installed Hermes Python/bootstrap/dependencies.
    # No copied parser or mocked loader.
    from tools import skills_tool
    from hermes_cli.mcp_config import _get_mcp_servers
    from agent.skill_utils import (find_project_root,
                                  get_untrusted_project_skills_root,
                                  is_project_root_trusted)

    source = Path(os.environ["RUNNER_TEMP"]) / "hermes-agent"
    require(Path(skills_tool.__file__).resolve().parents[1] == source.resolve(),
            "native Skills observer imported another Hermes checkout")
    require(_get_mcp_servers().get("foreign") == {"command": "echo", "args": ["foreign"]},
            "native Hermes observed a changed foreign MCP entry")
    catalog = {item["name"] for item in skills_tool.find_all_skills()}
    require(FOREIGN_SKILL in catalog, "native Hermes lost the foreign User Skill")
    view = json.loads(skills_tool.skill_view(USER_SKILL, preprocess=False))
    if user_marker == "absent":
        require(USER_SKILL not in catalog and not view.get("success"),
                "removed User Skill remains natively loadable")
    else:
        require(USER_SKILL in catalog and view.get("success") is True,
                "native Hermes did not discover/read the User Skill")
        require(user_marker in view.get("content", ""), "native Skill content is stale")
        expected = Path(os.environ["HERMES_HOME"]) / "skills" / USER_SKILL
        require(Path(view["skill_dir"]).resolve() == expected.resolve(),
                "native Hermes selected a different Skill instance")
    require(PROJECT_SKILL not in catalog, "untrusted Project Skill became loadable")
    root = find_project_root()
    if root is not None:
        require(root.resolve() == Path.cwd().resolve(), "Hermes selected another Git root")
        require(not is_project_root_trusted(root), "UAP granted native project trust")
    pending = get_untrusted_project_skills_root()
    if project_count:
        require(pending is not None and pending[0].resolve() == Path.cwd().resolve()
                and pending[1] == project_count,
                "native Hermes did not detect the selected untrusted Project Skill")
    else:
        require(pending is None, "removed Project Skill remains natively discoverable")
    print(json.dumps({"user_marker": user_marker, "project_skill_count": project_count,
                      "project_trusted": False, "native_skill_observation": "passed"}))


def invoke_skills(user_marker, project_count):
    source = Path(os.environ["RUNNER_TEMP"]) / "hermes-agent"
    # This official pre-bootstrap API is stdlib-only. The host Python builds
    # argv; only the installation-bound interpreter executes the observation.
    sys.path.insert(0, str(source))
    from hermes_cli._launchers import runtime_command

    runtime_file = os.environ.get("HERMES_NATIVE_RUNTIME_COMMAND_FILE")
    if runtime_file:
        native_argv = json.loads(Path(runtime_file).read_text(encoding="utf-8-sig"))
    else:
        native_argv = json.loads(subprocess.check_output(
            [os.environ["HERMES_BIN"], "--print-runtime-command"],
            text=True, timeout=30))
    require(isinstance(native_argv, list) and len(native_argv) >= 4
            and isinstance(native_argv[0], str) and Path(native_argv[0]).is_absolute(),
            "Hermes did not resolve its installation-bound interpreter")
    command = runtime_command(
        source, args=["skills", user_marker, str(project_count)], python=native_argv[0],
        code=f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name='__main__')")
    subprocess.run(command, check=True, timeout=60)


def check_project_refusal(path):
    data = json.loads(Path(path).read_text())["data"]
    if "targets" in data:
        require(len(data["targets"]) == 1 and data["targets"][0]["target"] == "hermes",
                "Project refusal did not select exactly Hermes")
        data = data["targets"][0]["output"]
    result = data["result"]
    plan = result["plan"]
    require(result["mutated"] is False and plan["client_id"] == "hermes"
            and plan["scope"] == "project" and plan["status"] == "unsupported",
            "Project MCP failed without the expected non-mutating unsupported plan")
    require(any(item["kind"] == "mcp_server" and item["name"] == "uap-native-test-server"
                and item["support"] == "unsupported" for item in plan["components"]),
            "Project MCP refusal did not identify the unsupported MCP component")


def stop_lifecycle(child):
    if os.name == "nt":
        # This process owns the child PID and its disposable native test tree.
        # taskkill handles descendants when a Windows CLI starts subprocesses.
        subprocess.run(["taskkill", "/PID", str(child.pid), "/T", "/F"],
                       check=False, timeout=15, stdout=subprocess.DEVNULL,
                       stderr=subprocess.DEVNULL)
        child.wait(timeout=5)
        return
    # The child owns a new POSIX process group. Also kill surviving descendants
    # when the shell leader exits promptly after SIGTERM.
    for signum in (signal.SIGTERM, signal.SIGKILL):
        with suppress(ProcessLookupError):
            os.killpg(child.pid, signum)
        try:
            child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            continue


def run_lifecycle(script):
    runner = Path(os.environ["RUNNER_TEMP"]).resolve()
    profile = Path(os.environ["HERMES_HOME"]).resolve()
    require(profile.is_relative_to(runner) and profile != runner,
            "Hermes profile must belong to the disposable runner")
    source = runner / "hermes-agent"
    actual = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"],
                                     text=True, timeout=15).strip()
    require(actual == HERMES_SHA, "Hermes source pin differs")
    checkout_sha = subprocess.check_output(
        ["git", "-C", str(script.parents[2]), "rev-parse", "HEAD"],
        text=True, timeout=15).strip()
    # This fixture implements only inert MCP messages; no Pi/model process is run.
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
    from pi_project_components_native_e2e import PiHTTPMCPProbe

    evidence = Path(tempfile.mkdtemp(prefix="hermes-native-evidence-", dir=runner))
    with PiHTTPMCPProbe() as probe:
        env = dict(os.environ, HERMES_NATIVE_MCP_URL=probe.url,
                   HERMES_NATIVE_EVIDENCE=str(evidence), HOME=str(profile.parent))
        env["PATH"] = str(Path(env["HERMES_BIN"]).parent) + os.pathsep + env.get("PATH", "")
        if os.name == "nt":
            env["USERPROFILE"] = str(profile.parent)
            for key, folder in (("APPDATA", "Roaming"), ("LOCALAPPDATA", "Local")):
                path = profile.parent / "AppData" / folder
                path.mkdir(parents=True, exist_ok=True)
                env[key] = str(path)
        child, timed_out = None, False
        try:
            options = ({"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP}
                       if os.name == "nt" else {"start_new_session": True})
            child = subprocess.Popen([sys.executable, str(script)], env=env, **options)
            try:
                child.wait(timeout=600)
            except subprocess.TimeoutExpired:
                timed_out = True
                stop_lifecycle(child)
        finally:
            methods = probe.methods()
            proof = {"hermes_sha": actual, "uap_checkout_sha": checkout_sha,
                     "uap_head_sha": os.environ.get("HERMES_NATIVE_UAP_HEAD", checkout_sha),
                     "github_sha": os.environ.get("GITHUB_SHA"), "timed_out": timed_out,
                     "runner_os": os.environ.get("RUNNER_OS", sys.platform),
                     "runner_arch": os.environ.get("RUNNER_ARCH"),
                     "lifecycle_exit": child.returncode if child else None,
                     "mcp_methods": methods, "model_turn": False,
                     "tools_call": "tools/call" in methods}
            (evidence / "summary.json").write_text(json.dumps(proof, indent=2) + "\n")
        require(not timed_out and child.returncode == 0,
                "Hermes lifecycle failed; inspect " + str(evidence))
        require(methods.count("initialize") >= 4 and methods.count("tools/list") >= 4,
                "native Hermes MCP exchange missing after a User lifecycle stage")
        require("tools/call" not in methods, "discovery smoke unexpectedly invoked a tool")
    print("Hermes native lifecycle, Skill read, project trust and MCP discovery passed: "
          + str(evidence))


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "run":
        run_lifecycle(Path(sys.argv[2]).resolve())
    elif len(sys.argv) == 4 and sys.argv[1] == "skills":
        observe_skills(sys.argv[2], int(sys.argv[3]))
    elif len(sys.argv) == 4 and sys.argv[1] == "observe":
        invoke_skills(sys.argv[2], int(sys.argv[3]))
    elif len(sys.argv) == 3 and sys.argv[1] == "refusal":
        check_project_refusal(sys.argv[2])
    else:
        raise SystemExit("Usage: hermes-native.py run <script> | observe <marker|absent> <count>")
