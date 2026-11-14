#!/usr/bin/env python3
"""Genuine Windows Codex model→shell diagnostic without credentials/canaries.

Uses the exact local model fixture and real read-only native Codex exec path,
but removes BOTH synthetic variables before spawning Codex. No raw tool,
subprocess or environment output is ever logged or stored.
"""
import http.server
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading

from codex_env_names_native_spike import (
    ENV_NAME, FOREIGN_ENV_NAME, Model, isolated_env,
    WINDOWS_NATIVE_SANDBOX_CONFIG
)


class ShellOnlyModel(Model):
    shell_only = True
    expected_digest = None
    requests = 0
    shell_requested = False
    shell_requested_at = 0.0
    shell_output_seen = False
    shell_execution_verified = False
    shell_secret_visible = False
    shell_foreign_visible = False
    shell_response_category = "not_observed"
    shell_response_indicators = []
    shell_win32_failure_category = "not_reported"
    advertised_tool_names = set()
    error = ""


def main():
    if len(sys.argv) != 3:
        raise SystemExit("usage: control.py CODEX_BINARY REPORT_JSON")
    codex = Path(sys.argv[1]).resolve(strict=True)
    report_path = Path(sys.argv[2]).resolve()
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report = {
        "control": "native_model_shell_no_secret",
        "shell_requested": False,
        "shell_output_seen": False,
        "shell_completed": False,
        "codex_exit_zero": False,
        "pass": False,
        "failure_code": "not_run",
    }
    root = Path(tempfile.mkdtemp(prefix="uap-codex-win-no-secret-"))
    server = None
    thread = None
    try:
        home, project = root / "home", root / "project"
        (home / ".codex").mkdir(parents=True)
        (project / ".codex").mkdir(parents=True)
        (project / ".git").mkdir()
        (root / "tmp").mkdir()
        env = isolated_env(root, home, "", "")
        env.pop(ENV_NAME, None)
        env.pop(FOREIGN_ENV_NAME, None)
        if (ENV_NAME in env or FOREIGN_ENV_NAME in env or
                "GH_TOKEN" in env or "GITHUB_TOKEN" in env):
            raise RuntimeError("isolation_violation")
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ShellOnlyModel)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        config = (
            'model_provider = "fixture"\nmodel = "fixture-model"\n'
            '[model_providers.fixture]\nname = "Fixture"\n'
            f'base_url = "http://127.0.0.1:{server.server_port}/v1"\n'
            'wire_api = "responses"\nrequires_openai_auth = false\n'
            '[analytics]\nenabled = false\n[feedback]\nenabled = false\n'
            f'[projects.{json.dumps(str(project))}]\ntrust_level = "trusted"\n'
            f'[shell_environment_policy]\nexclude = ["{FOREIGN_ENV_NAME}"]\n'
        )
        config += WINDOWS_NATIVE_SANDBOX_CONFIG
        (home / ".codex" / "config.toml").write_text(config)
        (project / ".codex" / "config.toml").write_text(
            '[shell_environment_policy]\ninherit = "core"\n'
        )
        proc = subprocess.run(
            [str(codex), "exec", "--skip-git-repo-check", "--sandbox",
             "read-only", "Use the available shell tool once."],
            cwd=project, env=env, capture_output=True, timeout=95,
        )
        report["codex_exit_zero"] = proc.returncode == 0
        report["shell_requested"] = ShellOnlyModel.shell_requested
        report["shell_output_seen"] = ShellOnlyModel.shell_output_seen
        report["shell_completed"] = (
            ShellOnlyModel.shell_requested and
            ShellOnlyModel.shell_execution_verified
        )
        report["shell_response_category"] = ShellOnlyModel.shell_response_category
        report["shell_response_indicators"] = ShellOnlyModel.shell_response_indicators[:24]
        report["shell_win32_failure_category"] = ShellOnlyModel.shell_win32_failure_category
        report["model_requests"] = ShellOnlyModel.requests
        report["pass"] = (
            report["codex_exit_zero"] and report["shell_completed"]
            and not ShellOnlyModel.shell_secret_visible
            and not ShellOnlyModel.shell_foreign_visible
        )
        report["failure_code"] = "none" if report["pass"] else "model_shell_unproven"
    except (OSError, RuntimeError, subprocess.TimeoutExpired, ValueError) as exc:
        report["failure_code"] = (
            str(exc) if isinstance(exc, RuntimeError) and
            str(exc) == "isolation_violation" else type(exc).__name__
        )
    finally:
        if server is not None:
            server.shutdown()
            server.server_close()
        if thread is not None:
            thread.join(timeout=3)
        shutil.rmtree(root, ignore_errors=True)
        report_path.write_text(json.dumps(report, sort_keys=True, indent=2))
    print(json.dumps(report, sort_keys=True))
    return 0 if report["pass"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
