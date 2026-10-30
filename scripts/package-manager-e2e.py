#!/usr/bin/env python3
"""Run the portable UAP package-manager acceptance catalog and retain evidence.

Requires Python 3.10+, Go (the repository toolchain), and Git. No native agent,
model account, or production security/trust override is used. See
docs/testing/package-manager-e2e.md for proof boundaries and the macOS command.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time

from package_manager_binary import run_installations


REPO = Path(__file__).resolve().parents[1]
CATALOG = REPO / "cli/internal/agentpluginscli/testdata/package_manager_e2e_cases.json"
INSTALL_CATALOG = REPO / "cli/cmd/agentplugins/testdata/package_manager_install_cases.json"
CLI_PACKAGE = "github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
MAIN_PACKAGE = "github.com/777genius/plugin-kit-ai/cli/cmd/agentplugins"
ROOT_TEST = "TestPackageManagerE2E"
INSTALL_ROOT_TEST = "TestPackageManagerInstallE2E"
CATALOG_GROUPS = ("archive", "registry", "agreement", "transport", "evolution", "adversarial")
ARCHIVE_GROUPS = tuple(group for group in CATALOG_GROUPS if group != "registry")
COMPOSED_TESTS = [
    "TestProjectComposedSourceLifecycleSeparatesUserAndProjects",
    "TestProjectComposedAgreementColdRestoreUsesPortablePins",
    "TestProjectCursorComposedSourceLifecycle",
]


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def validate_case_ids(cases):
    if (not isinstance(cases, list) or not cases
            or any(not isinstance(case, str) or not re.fullmatch(r"[a-z][a-z0-9-]*", case) for case in cases)
            or len(set(cases)) != len(cases)):
        raise ValueError("case inventory must contain unique nonempty canonical case IDs")
    return cases


def load_case_inventories():
    catalog = json.loads(CATALOG.read_text(encoding="utf-8"))
    if not isinstance(catalog, dict) or set(catalog) != set(CATALOG_GROUPS):
        raise ValueError("catalog groups changed; explicitly classify new groups before qualification")
    for cases in catalog.values():
        validate_case_ids(cases)
    installations = validate_case_ids(json.loads(INSTALL_CATALOG.read_text(encoding="utf-8")))
    return catalog, installations


def validate_go_events(path, required, package, exit_code):
    """Accept only complete successful executions, including every selected case."""
    passed, skipped, failed, package_pass = set(), set(), set(), False
    with path.open(encoding="utf-8") as stream:
        for number, line in enumerate(stream, 1):
            try:
                event = json.loads(line)
            except (ValueError, TypeError) as error:
                raise ValueError(f"invalid Go JSON at line {number}") from error
            if not isinstance(event, dict) or not isinstance(event.get("Action"), str):
                raise ValueError(f"invalid Go event at line {number}")
            if event.get("Package") != package:
                continue
            name, action = event.get("Test"), event["Action"]
            if name and action == "pass":
                passed.add(name)
            if action == "skip":
                skipped.add(name or "<package>")
            if action == "fail":
                failed.add(name or "<package>")
            if not name and action == "pass":
                package_pass = True
    missing = sorted(set(required) - passed)
    if exit_code != 0 or not package_pass or missing or skipped or failed:
        raise ValueError(
            f"Go proof incomplete: exit={exit_code}, package_pass={package_pass}, "
            f"missing={missing}, skipped={sorted(skipped)}, failed={sorted(failed)}"
        )
    return sorted(passed)


def isolated_environment(root, caches):
    # Build tools keep explicit cache/tool/proxy settings; test children receive
    # no account tokens, agent configuration, repository overrides or Git hooks.
    allowed = {
        "PATH", "SYSTEMROOT", "WINDIR", "COMSPEC", "PATHEXT", "NUMBER_OF_PROCESSORS",
        "GOROOT", "GOTOOLCHAIN", "GOPROXY", "GOSUMDB", "GONOSUMDB", "GONOPROXY",
        "GOPRIVATE", "CGO_ENABLED", "CC", "CXX", "SDKROOT", "DEVELOPER_DIR",
        "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "SSL_CERT_FILE",
        "SSL_CERT_DIR", "GOMAXPROCS",
    }
    env = {}
    for key, value in sorted(os.environ.items()):
        canonical = key.upper()
        if canonical in allowed and (canonical not in env or key == canonical):
            env[canonical] = value
    env.update(caches)
    paths = {
        "HOME": "home", "USERPROFILE": "home", "APPDATA": "appdata",
        "LOCALAPPDATA": "localappdata", "XDG_CONFIG_HOME": "config",
        "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache", "XDG_STATE_HOME": "state",
        "XDG_RUNTIME_DIR": "runtime", "TMPDIR": "tmp", "TMP": "tmp", "TEMP": "tmp",
        "AGENTPLUGINS_HOME": "uap", "CODEX_HOME": "codex",
        "CLAUDE_CONFIG_DIR": "claude", "GEMINI_CLI_HOME": "gemini",
    }
    for key, relative in paths.items():
        target = root / relative
        target.mkdir(parents=True, exist_ok=True)
        env[key] = str(target)
    hooks = root / "empty-hooks"
    hooks.mkdir()
    env.update({
        "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_CONFIG_COUNT": "3", "GIT_CONFIG_KEY_0": "core.hooksPath",
        "GIT_CONFIG_VALUE_0": str(hooks), "GIT_CONFIG_KEY_1": "core.autocrlf",
        "GIT_CONFIG_VALUE_1": "false", "GIT_CONFIG_KEY_2": "core.fsmonitor",
        "GIT_CONFIG_VALUE_2": "false", "GIT_TERMINAL_PROMPT": "0",
        "GIT_LFS_SKIP_SMUDGE": "1", "NO_COLOR": "1", "TERM": "dumb",
        "PYTHONDONTWRITEBYTECODE": "1", "LANG": "C.UTF-8",
    })
    return env


def output(argv, env):
    return subprocess.check_output(argv, cwd=REPO, env=env, timeout=60).decode("utf-8").strip()


def source_identity(env):
    status = output(["git", "status", "--porcelain=v1", "--untracked-files=normal"], env)
    diff = subprocess.check_output(["git", "diff", "--binary", "HEAD"], cwd=REPO, env=env, timeout=60)
    digest = hashlib.sha256(diff)
    untracked = subprocess.check_output(
        ["git", "ls-files", "--others", "--exclude-standard", "-z"], cwd=REPO, env=env, timeout=60)
    for name in sorted(filter(None, untracked.split(b"\0"))):
        path = REPO / os.fsdecode(name)
        digest.update(name + b"\0")
        digest.update(os.fsencode(os.readlink(path)) if path.is_symlink() else path.read_bytes())
    return {
        "commit": output(["git", "rev-parse", "HEAD"], env),
        "tree": output(["git", "rev-parse", "HEAD^{tree}"], env),
        "dirty": bool(status), "working_changes_sha256": digest.hexdigest(),
    }


def stop_owned_process(process):
    if os.name == "nt":
        if process.poll() is None:
            subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"],
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20, check=False)
    else:
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            process.kill()
    finally:
        if os.name != "nt":
            # A TERM-ignoring descendant can outlive an already-exited leader.
            # This PGID was created exclusively for our Popen invocation.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
    process.wait(timeout=10)


class Run:
    def __init__(self, directory, env, suite, catalog, installations):
        self.directory, self.env = directory, env
        self.groups = CATALOG_GROUPS if suite == "full" else ARCHIVE_GROUPS
        self.catalog_required = [ROOT_TEST] + [f"{ROOT_TEST}/{group}" for group in self.groups]
        self.catalog_required += [f"{ROOT_TEST}/{group}/{case}" for group in self.groups for case in catalog[group]]
        self.install_required = [INSTALL_ROOT_TEST] + [f"{INSTALL_ROOT_TEST}/{case}" for case in installations]
        self.composed_required = [name + "/zip" for name in COMPOSED_TESTS] + COMPOSED_TESTS
        self.required = self.catalog_required + self.install_required + self.composed_required
        self.report = {
            "schema_version": 2, "suite": suite, "status": "running",
            "full_qualification": False, "source": source_identity(env),
            "platform": {"os": platform.system(), "architecture": platform.machine()},
            "tools": {"python": platform.python_version(), "go": output(["go", "version"], env),
                      "git": output(["git", "--version"], env)},
            "required_tests": self.required,
            "catalog_sha256": sha256(CATALOG), "installation_catalog_sha256": sha256(INSTALL_CATALOG),
            "excluded_tests": [f"{ROOT_TEST}/{group}/{case}" for group in CATALOG_GROUPS if group not in self.groups
                               for case in catalog[group]],
            "boundaries": ["Security assessment and verified Directory inputs are fixtures in composed cases.",
                           "Native agent sessions, authentication, MCP execution and model calls are not qualified.",
                           "The unchanged executable separately installs a local synthetic plugin using the real pinned production scanner.",
                           "The scanner is downloaded from its public release into a cold disposable profile; no scanner assessment is preseeded."],
            "phases": [],
        }
        self.save()

    def save(self):
        (self.directory / "report.json").write_text(json.dumps(self.report, indent=2) + "\n", encoding="utf-8")

    def command(self, name, argv, proof_class, cwd=REPO, expected_success=True, required=None, package=None, env=None):
        row = {"name": name, "argv": [str(arg) for arg in argv], "proof_class": proof_class,
               "status": "running"}
        if required is not None:
            row["required_tests"] = list(required)
        self.report["phases"].append(row)
        self.save()
        started = time.monotonic()
        stdout, stderr = self.directory / f"{name}.stdout", self.directory / f"{name}.stderr"
        print(f"Running {name}...", flush=True)
        process = None
        try:
            kwargs = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
            with stdout.open("wb") as out, stderr.open("wb") as err:
                process = subprocess.Popen(row["argv"], cwd=cwd, env=self.env if env is None else env,
                                           stdin=subprocess.DEVNULL, stdout=out, stderr=err, **kwargs)
                try:
                    timeout = (780 if required else 60 if proof_class == "production_binary_smoke"
                               else 120 if proof_class == "production_binary_installation" else 600)
                    row["exit_code"] = process.wait(timeout=timeout)
                except subprocess.TimeoutExpired:
                    stop_owned_process(process)
                    raise RuntimeError("owned command timed out") from None
            if required is not None:
                row["passed_tests"] = validate_go_events(stdout, required, package, row["exit_code"])
            elif (row["exit_code"] == 0) != expected_success:
                raise RuntimeError(f"unexpected exit code {row['exit_code']}")
            row["status"] = "pass"
        except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
            row.update(status="fail", error=str(error))
            if process is not None:
                stop_owned_process(process)
        except BaseException:
            if process is not None:
                stop_owned_process(process)
            row.update(status="fail", error="interrupted", elapsed_seconds=round(time.monotonic() - started, 3))
            self.save()
            raise
        row["elapsed_seconds"] = round(time.monotonic() - started, 3)
        for key, path in [("stdout", stdout), ("stderr", stderr)]:
            if path.exists():
                row[key + "_sha256"] = sha256(path)
        self.save()
        print(f"{name}: {row['status']} ({row['elapsed_seconds']}s)", flush=True)
        if row["status"] != "pass":
            print(row.get("error", "command failed"), file=sys.stderr)
            for path in [stdout, stderr]:
                if path.exists():
                    print(path.read_text(encoding="utf-8", errors="replace")[-6000:], file=sys.stderr)
        return row["status"] == "pass"

    def smoke(self, binary, scratch):
        cwd = scratch / "workspace with spaces Ω"
        cwd.mkdir()
        home = Path(self.env["AGENTPLUGINS_HOME"])
        if not self.command("binary-help", [binary, "deps", "--help"], "production_binary_smoke", cwd):
            return False
        if not self.command("binary-init", [binary, "deps", "init", "--scope", "user"], "production_binary_smoke", cwd):
            return False
        manifest = home / "uap.toml"
        before = manifest.read_bytes()
        refused = self.command("binary-missing-lock", [binary, "install", "--scope", "user", "--locked", "--yes"],
                               "production_binary_smoke", cwd, expected_success=False)
        diagnostic = (self.directory / "binary-missing-lock.stderr").read_text(encoding="utf-8")
        if not refused or "uap.lock" not in diagnostic or manifest.read_bytes() != before or (home / "uap.lock").exists():
            raise RuntimeError("missing-lock smoke did not preserve the agreement or return the expected diagnostic")
        for name in ["state-v2.json", "managed", "store"]:
            if (home / name).exists():
                raise RuntimeError(f"preflight smoke published {name}")
        malformed = cwd / "invalid-plugin"
        malformed.mkdir()
        invalid = b'{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":false}'
        (malformed / "plugin.json").write_bytes(invalid)
        if not self.command("binary-malformed-package", [binary, "validate", malformed, "--scope", "user"],
                            "production_binary_smoke", cwd, expected_success=False):
            return False
        diagnostic = (self.directory / "binary-malformed-package.stderr").read_text(encoding="utf-8")
        if "plugin.json does not conform" not in diagnostic or "'/name'" not in diagnostic:
            raise RuntimeError("malformed-package smoke did not reach standard manifest validation")
        if (malformed / "plugin.json").read_bytes() != invalid:
            raise RuntimeError("malformed-package refusal changed source bytes")
        for name in ["state-v2.json", "managed", "store"]:
            if (home / name).exists():
                raise RuntimeError(f"validation refusal published {name}")
        locked_bytes = None
        for name, args in [("lock", ["deps", "lock"]), ("sync", ["install", "--locked", "--yes"]),
                           ("status", ["deps", "status"])]:
            if not self.command("binary-empty-" + name, [binary, *args, "--scope", "user", "--format", "json"],
                                "production_binary_smoke", cwd):
                return False
            json.loads((self.directory / f"binary-empty-{name}.stdout").read_text(encoding="utf-8"))
            if name == "lock":
                locked_bytes = (home / "uap.lock").read_bytes()
            elif (home / "uap.lock").read_bytes() != locked_bytes:
                raise RuntimeError("empty-environment sync/status rewrote its lock")
        lock_result = json.loads((self.directory / "binary-empty-lock.stdout").read_text(encoding="utf-8"))
        synced = json.loads((self.directory / "binary-empty-sync.stdout").read_text(encoding="utf-8"))
        status = json.loads((self.directory / "binary-empty-status.stdout").read_text(encoding="utf-8"))
        if not all(isinstance(value, dict) for value in [lock_result, synced, status]) or not isinstance(lock_result.get("lock"), dict):
            raise RuntimeError("empty-environment lifecycle returned an unexpected JSON shape")
        locked = lock_result["lock"]
        if (locked.get("lock_version") != 1 or locked.get("packages") != []
                or synced.get("outcome") != "completed" or synced.get("actions") != [] or synced.get("results") != []
                or not status.get("manifest_digest")
                or status["manifest_digest"] != status.get("lock_digest")
                or status.get("pending_recovery") is not False or status.get("facts") != []
                or manifest.read_bytes() != before
                or json.loads((home / "uap.lock").read_text(encoding="utf-8")) != locked):
            raise RuntimeError("empty-environment lifecycle did not preserve a converged agreement")
        return True

    def execute(self, scratch):
        binary = self.directory / ("agentplugins.exe" if os.name == "nt" else "agentplugins")
        built = self.command("build", ["go", "build", "-o", binary, "./cli/cmd/agentplugins"], "build")
        if built:
            self.report["binary_sha256"] = sha256(binary)
            self.save()
        smoke = False
        if built:
            try:
                smoke = self.smoke(binary, scratch)
            except (OSError, ValueError, KeyError, IndexError, TypeError, AttributeError, RuntimeError) as error:
                # A failed smoke proof must remain visible without suppressing
                # the independent graph, installation and composition phases.
                self.report["smoke_error"] = str(error)
                self.save()
                print(str(error), file=sys.stderr)
        binary_installations = False
        if built:
            try:
                install_env = isolated_environment(scratch / "binary-install-profile",
                                                   {key: self.env[key] for key in ["GOPATH", "GOCACHE"]})
                binary_installations = run_installations(self, binary, scratch, install_env)
            except (OSError, ValueError, KeyError, IndexError, TypeError, AttributeError, RuntimeError) as error:
                # Retain independent catalog results even when public scanner
                # acquisition or an executable postcondition fails.
                print(str(error), file=sys.stderr)
        selection = "^TestPackageManagerE2E$" if self.report["suite"] == "full" else "^TestPackageManagerE2E/(" + "|".join(self.groups) + ")$"
        catalog = self.command("catalog", ["go", "test", "-tags=uappm_e2e", "-count=1", "-timeout=12m", "-json",
                                           "./cli/internal/agentpluginscli", "-run", selection],
                               "composed_cli", required=self.catalog_required, package=CLI_PACKAGE)
        installations = self.command("installations", ["go", "test", "-tags=uappm_e2e", "-count=1", "-timeout=12m", "-json",
                                                        "./cli/cmd/agentplugins", "-run", "^" + INSTALL_ROOT_TEST + "$"],
                                     "production_composed", required=self.install_required, package=MAIN_PACKAGE)
        composed = self.command("composition", ["go", "test", "-count=1", "-timeout=12m", "-json",
                                                  "./cli/cmd/agentplugins", "-run", "^(" + "|".join(COMPOSED_TESTS) + ")$/^zip$"],
                                "production_composed", required=self.composed_required, package=MAIN_PACKAGE)
        stable = self.report["source"] == source_identity(self.env)
        self.report["source_unchanged_during_run"] = stable
        if not stable:
            self.report["error"] = "Source changed during execution; rerun against a stable checkout."
            print(self.report["error"], file=sys.stderr)
        passed = built and smoke and binary_installations and catalog and installations and composed and stable
        self.report.update(status="pass" if passed else "fail", full_qualification=passed and self.report["suite"] == "full")
        self.save()
        return passed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--suite", choices=["full", "archive"], default="full")
    parser.add_argument("--output", type=Path, help="new evidence directory outside the checkout; default: unique temporary directory")
    args = parser.parse_args()
    directory = args.output.resolve() if args.output else Path(tempfile.mkdtemp(prefix="uap-package-manager-e2e-"))
    if directory == REPO or REPO in directory.parents:
        parser.error("--output must be outside the source checkout")
    if args.output:
        directory.mkdir(parents=True, exist_ok=False)
    run = None
    try:
        for command in ["go", "git"]:
            if not shutil.which(command):
                raise RuntimeError(f"{command} is required on PATH")
        caches = json.loads(subprocess.check_output(["go", "env", "-json", "GOPATH", "GOCACHE"], cwd=REPO, timeout=60))
        catalog, installations = load_case_inventories()
        with tempfile.TemporaryDirectory(prefix="uap pm Ω ") as temporary:
            scratch = Path(temporary).resolve()
            run = Run(directory, isolated_environment(scratch, caches), args.suite, catalog, installations)
            passed = run.execute(scratch)
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, KeyboardInterrupt) as error:
        report = run.report if run else {"schema_version": 2, "suite": args.suite, "phases": []}
        report.update(status="fail", full_qualification=False, error=str(error) or "interrupted")
        (directory / "report.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
        print(str(error), file=sys.stderr)
        passed = False
    print(f"{args.suite}: {'PASS' if passed else 'FAIL'}; evidence: {directory}", flush=True)
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
