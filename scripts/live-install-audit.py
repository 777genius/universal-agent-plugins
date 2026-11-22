#!/usr/bin/env python3
"""Live, non-mocked UAP install/remove evidence. Runs only on disposable profiles."""
import argparse
from contextlib import ExitStack
import hashlib
import json
import os
import platform
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import tomllib
import traceback
import uuid
from runpy import run_path
from datetime import datetime, timezone
from urllib.parse import urlsplit, urlunsplit

ROOT = Path(__file__).resolve().parents[1]
FIXTURES = ROOT / "tests/live-install/fixtures.json"
CODEX_RUNTIME_AUDIT = run_path(str(ROOT / "scripts/codex_runtime_audit.py"))
HISTORICAL_CODEX = run_path(str(ROOT / "scripts/published_codex_baseline.py"))
CLAUDE_RETIREMENT_AUDIT = run_path(str(ROOT / "scripts/claude_cache_retirement_audit.py"))
GEMINI_RUNTIME_AUDIT = run_path(str(ROOT / "scripts/gemini_runtime_audit.py"))
OPENCODE_BOOTSTRAP_AUDIT = run_path(str(ROOT / "scripts/opencode_bootstrap_audit.py"))
NATIVE_ARTIFACTS = run_path(str(ROOT / "scripts/native_client_project_artifacts.py"))
PREPARED_INSTALLATION = run_path(str(ROOT / "scripts/prepared_installation_audit.py"))
WINDSURF_INSTALLATION = run_path(str(ROOT / "scripts/windsurf_installation_audit.py"))

# One immutable authority for the signed Marketing Kit and its dependencies.
MARKETING_KIT_DISTRIBUTIONS = {
    "marketing-kit": "777genius/marketing-kit",
    "notion": "777genius/notion",
    "hubspot-crm": "777genius/hubspot-crm",
    "playwright": "777genius/playwright-bridge",
}
MARKETING_KIT_SIGNED_CLOSURE = {
    "777genius/marketing-kit": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins-registry",
        "package_path": "plugins/marketing-kit",
        "resolved_revision": "c5bcedd4ecf12585a8a50401852e57a44891ebec",
        "tree_digest": "sha256:8918c0f0222bd735fd0eef3cacf90b5a9351fb213483d1558a65b9911624e68a",
        "manifest_digest": "sha256:40b18a6b33c6392134aff7beafb9a91eaeecf46ce765e0e888cdc743dd284ea2"
    },
    "777genius/notion": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins",
        "package_path": "plugins/notion",
        "resolved_revision": "2ddbb99dd190c1792b79904f9875e6322bccd243",
        "tree_digest": "sha256:860fa6dd1830d0da2b8fed38195fa6ad6307082ac12d708118e5eb1c5d10f9b7",
        "manifest_digest": "sha256:bbab30a4f062218d7a5fce2f6c818680d3c7adfaebdbaa3e523e15942a2f57e5"
    },
    "777genius/hubspot-crm": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins",
        "package_path": "plugins/hubspot-crm",
        "resolved_revision": "2ddbb99dd190c1792b79904f9875e6322bccd243",
        "tree_digest": "sha256:09a0b151c384235c3d7b3929398695d24cd7b07c75993a0a73f1a218198ededd",
        "manifest_digest": "sha256:20334e86a1b599d2759e04bfaef37dc2ddb7f6b0f3994b5d66d4eca12fbec941"
    },
    "777genius/playwright-bridge": {
        "release_sequence": 2,
        "package_version": "0.0.80",
        "repository": "777genius/universal-agent-plugins-registry",
        "package_path": "plugins/playwright",
        "resolved_revision": "d9c8d778200d51ae941d86bb69af23534cd1c8f7",
        "tree_digest": "sha256:ff3c74bb5760fea47003130f3984d48234892330325a7cad35a7bc973acc9cf6",
        "manifest_digest": "sha256:8419b5c9e67894c2312048c4fe30e3ea920d00101d02641230183dd7ab4d3b31"
    }
}

NATIVE_ROOTS = {
    "codex": [".codex", ".agents"], "chatgpt": [],
    "cursor": [".cursor"], "copilot": [".copilot", ".config/copilot"],
    "vscode": [".vscode", ".config/Code"], "kiro": [".kiro"],
    "claude": [".claude"], "gemini": [".gemini"],
    "opencode": [".config/opencode", ".opencode"], "cline": [".cline"],
    "windsurf": [".windsurf", ".devin"], "grok": [".grok"],
    "kimi": [".kimi-code", ".kimi"], "qoder": [".qoder"],
    "pi": [".pi"],
}
PROJECT_SKILL_ROOTS = {
    "codex": [".agents/skills"], "cursor": [".cursor/skills"],
    "copilot": [".github/skills"], "vscode": [".github/skills"],
    "kiro": [".kiro/skills"], "claude": [".claude/skills"],
    "gemini": [".gemini/skills"], "opencode": [".opencode/skills"],
    "cline": [".cline/skills"], "windsurf": [".devin/skills"],
    "grok": [".grok/plugins"], "kimi": [".kimi-code/skills"],
    "qoder": [".uap/managed/clients/qoder"], "pi": [".pi/skills"],
}
EXECUTABLE = {
    "codex": ("codex",), "copilot": ("copilot",), "cline": ("cline",),
    "claude": ("claude",), "gemini": ("gemini",),
    "opencode": ("opencode",), "pi": ("pi",),
    "cursor": ("cursor-agent", "cursor"), "vscode": ("code",),
    "kiro": ("kiro-cli", "kiro"), "grok": ("grok",),
    "kimi": ("kimi",), "qoder": ("qoder", "qodercli"),
    "windsurf": ("devin", "windsurf"),
}

# Bind native client commands before running any untrusted package lifecycle.
# In particular, Codex removal attestation must not resolve a new PATH entry.
AGENT_BINARIES = {
    name: str(Path(path).resolve())
    for candidates in EXECUTABLE.values() for name in candidates
    if (path := shutil.which(name))
}
# Git is a host test prerequisite, not a client plugin or an acquired package.
# Resolve it before executing any third-party installation source.
GIT_EXECUTABLE = str(Path(path).resolve()) if (path := shutil.which("git")) else None


def validate_signed_release_pin(alias, pin):
    if not isinstance(pin, dict) or set(pin) != {
            "distribution", "repository", "path", "revision",
            "sequence", "tree_digest", "manifest_digest"}:
        raise ValueError(f"Invalid frozen signed release pin: {alias}")
    if (not all(isinstance(pin[key], str) and pin[key]
                for key in ("distribution", "repository", "path"))
            or type(pin["sequence"]) is not int or pin["sequence"] < 1
            or not isinstance(pin["revision"], str)
            or not re.fullmatch(r"[0-9a-f]{40}", pin["revision"])
            or any(not isinstance(pin[key], str) or
                   re.fullmatch(r"sha256:[0-9a-f]{64}", pin[key]) is None
                   for key in ("tree_digest", "manifest_digest"))):
        raise ValueError(f"Malformed frozen signed release pin: {alias}")


def effective_signed_release_pins(doc, native_target):
    """Freeze one selected client's source expectations without mutating defaults."""
    pins = {alias: dict(pin) for alias, pin in doc["signed_release_pins"].items()}
    for alias, pin in doc.get("signed_release_pins_by_target", {}).get(native_target, {}).items():
        pins[alias] = dict(pin)
    return pins


def validate_fixtures(doc):
    names = doc["plugin_aliases"]
    skills = doc["skills"]
    targets = doc["targets"]
    if len(names) != 30 or len(set(names)) != 30:
        raise ValueError("Must have exactly 30 distinct plugin sources")
    if len(skills) != 30 or len(set((v["repo"], v["name"]) for v in skills)) != 30:
        raise ValueError("Must have exactly 30 distinct Skills")
    if set(targets) != set(NATIVE_ROOTS):
        raise ValueError("All 15 advertised adapters must be represented")
    direct = doc["plugin_declared_names"]
    exact_git = [v for v in names if v.startswith("github:")]
    if set(direct) != set(exact_git + ["777genius/chrome-devtools-bridge", "777genius/github-bridge"]):
        raise ValueError("All external manifest identities must be supplied and no stale alias mappings")
    pins = doc.get("signed_release_pins")
    signed_aliases = set(names) - set(exact_git)
    if not isinstance(pins, dict) or set(pins) != signed_aliases:
        raise ValueError("Missing frozen Registry release pins for signed aliases")
    for alias, pin in pins.items():
        validate_signed_release_pin(alias, pin)
    if "signed_release_pins_by_target" in doc:
        overrides = doc["signed_release_pins_by_target"]
        canonical_targets = {audit_client_id(target, "plugin") for target in targets}
        if not isinstance(overrides, dict) or not overrides or not set(overrides) <= canonical_targets:
            raise ValueError("Invalid target-specific signed release pin maps")
        for target, target_pins in overrides.items():
            if (not isinstance(target_pins, dict) or not target_pins
                    or not set(target_pins) <= signed_aliases):
                raise ValueError(f"Invalid target-specific signed release pins: {target}")
            for alias, pin in target_pins.items():
                validate_signed_release_pin(alias, pin)
    for plugin in exact_git:
        if not re.fullmatch(r"github:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[a-f0-9]{40}", plugin):
            raise ValueError("Direct plugin source is not a pinned root package")
        if not re.fullmatch(r"[a-z][a-z0-9-]*", direct[plugin]):
            raise ValueError("Invalid direct package declared identity")
    for alias in ("777genius/chrome-devtools-bridge", "777genius/github-bridge"):
        if not re.fullmatch(r"[a-z][a-z0-9-]*", direct[alias]):
            raise ValueError("Invalid namespaced distribution manifest identity")
    for skill in skills:
        if not re.fullmatch(r"[0-9a-f]{40}", skill["ref"]):
            raise ValueError("Skill lacks a complete immutable commit")
        canonical = f'https://github.com/{skill["repo"]}/tree/{skill["ref"]}/skills/{skill["folder"]}'
        if (skill["source"] != canonical or
                not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", skill["name"]) or
                not re.fullmatch(r"[0-9a-f]{40}", skill.get("skill_md_git_blob_sha1", ""))):
            raise ValueError("Invalid Skill identity/source or pinned blob")
    if len(set(s["repo"] for s in skills)) < 3:
        raise ValueError("Insufficiently diverse publishers")


def _windows_file_close(job, deadline):
    """Wait original current-member handles; a job count alone is not exit."""
    import time
    C, api = job.C, job.api
    handles, failure, generation = [], None, None
    def accounting():
        info = job.Accounting()
        job.check(api.QueryInformationJobObject(
            job.handle, 1, C.byref(info), C.sizeof(info), None))
        return info
    try:
        class Members(C.Structure):
            _fields_ = [("assigned", C.c_uint32), ("listed", C.c_uint32),
                        ("pids", C.c_size_t * 64)]
        for name, args, result in (
                ("OpenProcess", [C.c_uint32, C.c_int, C.c_uint32], C.c_void_p),
                ("IsProcessInJob", [C.c_void_p, C.c_void_p, C.POINTER(C.c_int)], C.c_int),
                ("WaitForSingleObject", [C.c_void_p, C.c_uint32], C.c_uint32)):
            getattr(api, name).argtypes, getattr(api, name).restype = args, result
        generation = accounting().total
        members = Members()
        job.check(api.QueryInformationJobObject(
            job.handle, 3, C.byref(members), C.sizeof(members), None))
        if members.assigned != members.listed or members.listed > 64:
            raise OSError("owned Windows job member list is incomplete")
        pids = list(members.pids[:members.listed])
        if len(set(pids)) != len(pids) or any(not 0 < pid <= 0xffffffff for pid in pids):
            raise OSError("owned Windows job member identities are invalid")
        for pid in pids:
            if time.monotonic() >= deadline:
                raise OSError("owned Windows job cleanup deadline")
            handle = api.OpenProcess(0x00101000, False, pid)
            job.check(handle)
            handles.append(handle)
            belongs = C.c_int()
            job.check(api.IsProcessInJob(handle, job.handle, C.byref(belongs)))
            if not belongs.value:
                raise OSError("opened process is not in the owned Windows job")
        if accounting().total != generation:
            raise OSError("owned Windows job membership changed during capture")
    except BaseException as error:
        failure = error
    try:
        # Even an incomplete observation must terminate only this owned job.
        try:
            job.check(api.TerminateJobObject(job.handle, 97))
        except BaseException as error:
            failure = failure or error
        try:
            if generation is None or accounting().total != generation:
                raise OSError("owned Windows job membership changed during termination")
        except BaseException as error:
            failure = failure or error
        for handle in handles:
            try:
                remaining = max(0, int((deadline - time.monotonic()) * 1000))
                if api.WaitForSingleObject(handle, remaining) != 0:
                    raise OSError("owned Windows job process did not signal exit")
            except BaseException as error:
                failure = failure or error
        try:
            final = accounting()
            while final.active:
                if time.monotonic() >= deadline:
                    raise OSError("owned Windows job cleanup deadline")
                time.sleep(min(.05, max(0, deadline - time.monotonic())))
                final = accounting()
            if generation is None or final.total != generation:
                raise OSError("owned Windows job membership changed during termination")
            if time.monotonic() > deadline:
                raise OSError("owned Windows job cleanup deadline")
        except BaseException as error:
            failure = failure or error
    finally:
        for handle in (*handles, job.handle):
            try:
                job.check(api.CloseHandle(handle))
            except BaseException as error:
                failure = failure or error
    if failure is not None:
        raise failure
    return bool(pids)


def _windows_file_call(argv, *, cwd, env, stdout, stderr, stdin, timeout, check):
    """Own the blocked bootstrap before it can start any command descendants."""
    import time
    started, child, job = time.monotonic(), None, None
    cleanup_deadline = None
    helpers = run_path(str(ROOT / "scripts/e2e/hermes-noauth-tool-call.py"))
    try:
        child = subprocess.Popen(
            [sys.executable, str(Path(__file__).resolve()), "--windows-owned-file-command",
             json.dumps(argv)], cwd=cwd, env=env, stdin=subprocess.PIPE,
            stdout=stdout, stderr=stderr,
            creationflags=subprocess.CREATE_NEW_PROCESS_GROUP)
        job = helpers["WindowsJob"](child)
        remaining = timeout - (time.monotonic() - started)
        if remaining <= 0:
            raise subprocess.TimeoutExpired(argv, timeout)
        child.stdin.write(b"!")
        child.stdin.close()
        try:
            code = child.wait(timeout=remaining)
        except subprocess.TimeoutExpired:
            # Observe only the already-owned job, inside its existing cleanup
            # budget. Diagnostic refusal never bypasses mandatory termination.
            cleanup_deadline = time.monotonic() + 5
            error = subprocess.TimeoutExpired(argv, timeout)
            error.owned_timeout_diagnostic = {"status": "refused", "qualified": False}
            try:
                observer = run_path(str(ROOT / "scripts/windows_owned_timeout.py"))["snapshot"]
                error.owned_timeout_diagnostic = observer(
                    job, child, argv[0], started, cleanup_deadline)
            except Exception:
                pass
            raise error from None
        counts = job.Accounting()
        job.check(job.api.QueryInformationJobObject(
            job.handle, 1, job.C.byref(counts), job.C.sizeof(counts), None))
        if counts.active:
            raise OSError("owned Windows command descendants remained after launcher exit")
        return subprocess.CompletedProcess(argv, code)
    finally:
        if child is not None:
            deadline = cleanup_deadline if cleanup_deadline is not None else time.monotonic() + 5
            try:
                try:
                    if child.stdin is not None and not child.stdin.closed:
                        child.stdin.close()
                finally:
                    if job is not None:
                        if _windows_file_close(job, deadline) and child.returncode is not None:
                            raise OSError("owned Windows command descendants remained after launcher exit")
                    elif child.poll() is None:
                        child.kill()  # The unreleased bootstrap cannot have descendants.
            finally:
                child.wait(timeout=max(0, deadline - time.monotonic()))


def _windows_owned_file_command(argv):
    if os.name != "nt" or sys.stdin.buffer.read(1) != b"!":
        raise SystemExit("owned Windows command gate was not released")
    command = json.loads(argv)
    if (not isinstance(command, list) or not command
            or not all(isinstance(value, str) and "\0" not in value for value in command)):
        raise SystemExit("owned Windows command arguments are invalid")
    return subprocess.call(command, stdin=subprocess.DEVNULL)


def call(argv, env, cwd, timeout=105, capture_files=False, capture_full=False):
    try:
        if capture_files or capture_full:
            # A Windows native client may spawn a child which inherits stdout
            # or stderr. Pipes stay open after the UAP process exits, causing
            # subprocess.run(capture_output=True) to time out after a real add.
            # Capture in private regular files so the parent exit is observed.
            scratch = Path(env.get("TEMP", ""))
            if not scratch.is_absolute() or not scratch.is_dir() or scratch.is_symlink():
                raise OSError("private output capture root unavailable")
            with (tempfile.TemporaryFile(mode="w+b", dir=scratch) as stdout_file,
                  tempfile.TemporaryFile(mode="w+b", dir=scratch) as stderr_file):
                runner = (_windows_file_call if sys.platform == "win32" and not capture_full
                          else subprocess.run)
                try:
                    p = runner(argv, cwd=cwd, env=env, stdout=stdout_file,
                               stderr=stderr_file, stdin=subprocess.DEVNULL,
                               timeout=timeout, check=False)
                except subprocess.TimeoutExpired as error:
                    if capture_full or sys.platform != "win32":
                        raise
                    stdout_file.seek(0)
                    stderr_file.seek(0)
                    output, errors = stdout_file.read((1 << 20) + 1), stderr_file.read((1 << 20) + 1)
                    if len(output) > 1 << 20 or len(errors) > 1 << 20:
                        output, errors = b"", b"bounded native output capture exceeded"
                    diagnostic = ({"owned_timeout_diagnostic": error.owned_timeout_diagnostic}
                                  if hasattr(error, "owned_timeout_diagnostic") else {})
                    return {"exit_code": None, "stdout_tail": output.decode("utf-8", errors="replace")[-1800:],
                            "stderr_tail": errors.decode("utf-8", errors="replace")[-1800:],
                            "timeout": True, "_json": None, **diagnostic}
                stdout_file.seek(0)
                stderr_file.seek(0)
                limit = 1 << 20
                output, errors = stdout_file.read(limit + 1), stderr_file.read(limit + 1)
            if len(output) > limit or len(errors) > limit:
                return {"exit_code": None, "stdout_tail": "",
                        "stderr_tail": "bounded native output capture exceeded",
                        "timeout": False, "_json": None}
            decoding = "strict" if capture_full else "replace"
            stdout, stderr = output.decode("utf-8", errors=decoding), errors.decode("utf-8", errors=decoding)
        else:
            p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True,
                               text=True, errors="replace", stdin=subprocess.DEVNULL,
                               timeout=timeout, check=False)
            stdout, stderr = p.stdout, p.stderr
        try:
            parsed_json = (json.loads(stdout, object_pairs_hook=_unique_json_object,
                                      parse_constant=_invalid_json_constant)
                           if capture_full else json.loads(stdout))
        except ValueError:
            parsed_json = None
        if (capture_files and not capture_full and p.returncode == 0
                and (not isinstance(parsed_json, dict)
                     or parsed_json.get("result") != "success")):
            # An inherited child may append to the file after UAP exits.
            # Never promote a truncated/corrupt add response to native PASS.
            return {"exit_code": None, "stdout_tail": stdout[-1800:],
                    "stderr_tail": "native add success JSON unavailable",
                    "timeout": False, "_json": None}
        complete = {"stdout_full": stdout, "stderr_full": stderr} if capture_full else {}
        return {"exit_code": p.returncode, "stdout_tail": stdout[-1800:],
                "stderr_tail": stderr[-1800:], "timeout": False,
                "_json": parsed_json, **complete}
    except UnicodeError:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": "native output capture was not UTF-8",
                "timeout": False, "_json": None}
    except subprocess.TimeoutExpired as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": True}
    except OSError as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": False}


def install_case_timeout(target, kind, item, *, repeat=False):
    if (sys.platform == "win32" and kind == "plugin"
            and item == "firebase"):
        # A fresh Claude installation includes runLockedNPM's existing
        # 10-minute npm deadline and sequential native probes. Use a bounded
        # cold-install window; expiration still fails the lifecycle and kills
        # the owned job. The warm repeat retains the original bound.
        if target == "claude" and not repeat:
            return 900
        return 180
    return 105


def version_evidence(argv, env, cwd):
    """Record a short, observable version; a successful path alone is not proof."""
    observed = call(argv, env, cwd, 25)
    value = (observed["stdout_tail"] or observed["stderr_tail"]).strip()
    if observed["timeout"]:
        return {"status": "timeout", "exit_code": observed["exit_code"]}
    if observed["exit_code"] != 0:
        return {"status": "probe-failed", "exit_code": observed["exit_code"]}
    if not value:
        return {"status": "empty-output", "exit_code": 0}
    return {"status": "reported", "value": value[:200], "exit_code": 0}


def native_file_digest(path):
    """Hash one held regular file; a pathname swap cannot redirect or block the read."""
    try:
        initial = path.lstat()
        if stat.S_ISLNK(initial.st_mode):
            return "symlink"
        if not stat.S_ISREG(initial.st_mode):
            return "non-regular"
        descriptor = NATIVE_ARTIFACTS["open_member"](path)
        try:
            opened = os.fstat(descriptor)
            if not stat.S_ISREG(opened.st_mode):
                return "non-regular"
            if not NATIVE_ARTIFACTS["same_opened"](initial, opened):
                return "unreadable"
            sha, remaining = hashlib.sha256(), opened.st_size + 1
            while remaining:
                chunk = os.read(descriptor, min(remaining, 1024 * 1024))
                if not chunk:
                    break
                sha.update(chunk)
                remaining -= len(chunk)
            identity = NATIVE_ARTIFACTS["identity"]
            if (remaining != 1 or identity(opened) != identity(os.fstat(descriptor))
                    or identity(initial) != identity(path.lstat())):
                return "unreadable"
            return sha.hexdigest()
        finally:
            os.close(descriptor)
    except (OSError, ValueError):
        return "unreadable"


def snapshot(root, *, include_directories=False):
    """Record native entries; Gemini repeat also preserves ordinary directory identities."""
    found = {}
    for target_root in root:
        if target_root.is_symlink():
            found[f"{target_root.name}/.native-root"] = "symlink"
            continue
        if not target_root.is_dir():
            continue

        def walk_error(error):
            try:
                relative = Path(error.filename).relative_to(target_root)
                found[f"{target_root.name}/{relative}/.walk-error"] = "unreadable"
            except (TypeError, ValueError):
                found[f"{target_root.name}/.walk-error"] = "unreadable"

        for base, dirs, files in os.walk(target_root, followlinks=False, onerror=walk_error):
            for name in dirs[:]:
                path = Path(base) / name
                if path.is_symlink():
                    found[f"{target_root.name}/{path.relative_to(target_root)}"] = "symlink"
                    dirs.remove(name)
                elif include_directories:
                    # An empty directory is a repeat mutation too. Ignore
                    # timestamps: native journals may touch existing parents.
                    try:
                        info = path.lstat()
                        identity = f"{info.st_mode}:{info.st_dev}:{info.st_ino}" if stat.S_ISDIR(info.st_mode) else "non-regular"
                    except OSError:
                        identity = "unreadable"
                    found[f"directory:{target_root.name}/{path.relative_to(target_root).as_posix()}"] = identity
            for name in files:
                path = Path(base) / name
                relative = str(path.relative_to(target_root))
                found[f"{target_root.name}/{relative}"] = native_file_digest(path)
    return found


def snapshot_change_evidence(before, after):
    """Describe existing snapshot differences without reading or exporting file bytes."""
    changed = sorted(path for path in before.keys() | after.keys()
                     if before.get(path) != after.get(path))
    entries = [{"path": path[:2048], "path_truncated": len(path) > 2048,
                "before": before.get(path), "after": after.get(path)}
               for path in changed[:100]]
    return {"total": len(changed), "omitted": max(0, len(changed) - len(entries)),
            "entries": entries}


def repeat_native_phase_evidence(before, after_add, after_list, after_native):
    """Compare every closed phase to the same original snapshot; no new baseline."""
    return [{"phase": phase, **snapshot_change_evidence(before, after)}
            for phase, after in (("after-public-repeat-add", after_add),
                                 ("after-public-repeat-list", after_list),
                                 ("after-native-checkpoint", after_native))]



def bounded_error_traceback(error):
    """Keep the failure frames even when a message or note fills the budget."""
    formatted = "".join(traceback.format_exception(error, limit=-8))
    if len(formatted) <= 8192:
        return formatted
    frames = "".join(
        f"  File {repr(frame.filename)[-320:]}, line {str(frame.lineno)[:24]}, in {frame.name[:96]}\n"
        for frame in traceback.extract_tb(error.__traceback__, limit=-8))
    try:
        message = str(error)[:2048]
    except Exception:
        message = "<exception message unavailable>"
    notes = getattr(error, "__notes__", ())
    notes = notes if isinstance(notes, (list, tuple)) else ()
    notes = "\n".join(note[:192] for note in notes[:8] if isinstance(note, str))[:1536]
    return ("Traceback (most recent call last):\n" + frames
            + type(error).__name__[:256] + ": " + message + "\n"
            + ("Notes:\n" + notes + "\n" if notes else "")
            + "[traceback truncated; message, notes and earlier context bounded]\n")


def failed_case_diagnostics(row):
    """Expose existing failed-case evidence in logs; never change qualification."""
    keys = ("codex_public_add_ready", "codex_public_repeat_ready", "public_skill_ready",
            "repeat_native_unchanged", "repeat_native_runtime_only", "repeat_native_changes",
            "repeat_native_phases", "repeat_verified", "kimi_repeat_log_facts",
            "removal_verified", "sentinels_preserved", "public_target_states")
    evidence = {key: row[key] for key in keys if row.get(key) is not None}
    for key in ("error", "error_traceback"):
        if key in row:
            evidence[key] = str(row[key])[:8192]
    proof = row.get("gemini_runtime_proof")
    if isinstance(proof, dict):
        # Project only bounded facts already emitted by the strict observer.
        # Never print journal text, native CLI output, or full record/path lists.
        summary = {key: value[:512] if isinstance(value := proof.get(key), str)
                   else None if value is None else "invalid"
                   for key in ("status", "reason")}
        phases = proof.get("phases")
        summary["phases"] = [
            phase if phase in ("after-public-repeat-add", "after-public-repeat-list") else "invalid"
            for phase in phases[:2]] if isinstance(phases, list) else []
        summary["phases_truncated"] = isinstance(phases, list) and len(phases) > 2
        fields = {
            "baseline": ("bytes", "records", "started", "finished"),
            "failure_facts": ("phase", "role", "operation", "exact_key_present",
                              "slug_is_string", "slug_matches_pattern", "slug_binding_count",
                              "regular", "single_link", "positive_size", "within_limit",
                              "descriptor_capacity", "bytes", "lf_count", "final_lf"),
        }
        for section, keys in fields.items():
            facts = proof.get(section)
            summary[section] = ({
                key: value[:64] if isinstance(value := facts[key], str)
                else value if value is None or type(value) is bool
                or type(value) is int and 0 <= value <= 1 << 20 else "invalid"
                for key in keys if key in facts
            } if isinstance(facts, dict) else None)
        facts = proof.get("failure_facts")
        shapes = facts.get("record_shapes") if isinstance(facts, dict) else None
        if isinstance(shapes, list):
            allowed = ("json-refused", "metadata-fields", "user-text-fields",
                       "last-updated-fields", "other")
            summary["failure_facts"]["record_shapes"] = [
                shape if isinstance(shape, str) and shape in allowed else "invalid"
                for shape in shapes[:3]]
            summary["failure_facts"]["record_shapes_truncated"] = len(shapes) > 3
        evidence["gemini_runtime_proof"] = summary
    return evidence


def scoped_snapshot(roots, *, snapshotter=None):
    """Record protected file and directory identities without following links."""
    roots = tuple(Path(root) for root in roots)
    state = (snapshot if snapshotter is None else snapshotter)(roots)
    # The shared audit records failed file reads and walk errors as
    # 'unreadable'. Identical failures before/after must not prove isolation.
    if any(digest == "unreadable" for digest in state.values()):
        raise AssertionError("protected scope snapshot unreadable: native entry")

    def record_metadata(index, path, relative):
        """Preserve directory and file inode identity, not only permissions."""
        try:
            entry = path.lstat()
        except OSError as error:
            raise AssertionError("protected scope snapshot unreadable: metadata") from error
        identity = (entry.st_mode, entry.st_dev, entry.st_ino)
        # Equal bytes and inode do not prove a peer left file timestamps alone.
        if stat.S_ISREG(entry.st_mode):
            identity += (entry.st_mtime_ns,)
        state[f"metadata:{index}:{relative}"] = identity

    def refuse_walk_error(error):
        raise AssertionError(f"protected scope snapshot unreadable: {error}") from error

    for index, root in enumerate(roots):
        if root.is_symlink():
            state[f"root-link:{index}"] = os.readlink(root)
            record_metadata(index, root, ".")
            continue
        # An empty protected profile must not disappear or turn into a file
        # unnoticed: audit.snapshot records children, not root existence.
        if not root.is_dir():
            state[f"root-kind:{index}"] = "file" if root.exists() else "absent"
            continue
        state[f"root-kind:{index}"] = "directory"
        record_metadata(index, root, ".")
        for directory, dirs, files in os.walk(root, followlinks=False, onerror=refuse_walk_error):
            for name in dirs:
                path = Path(directory) / name
                relative = path.relative_to(root).as_posix()
                if path.is_symlink():
                    state[f"link:{index}:{relative}"] = os.readlink(path)
                    record_metadata(index, path, relative)
                else:
                    state[f"directory:{index}:{relative}"] = "present"
                    record_metadata(index, path, relative)
            for name in files:
                path = Path(directory) / name
                if path.is_symlink():
                    relative = path.relative_to(root).as_posix()
                    state[f"link:{index}:{relative}"] = os.readlink(path)
                    record_metadata(index, path, relative)
                else:
                    record_metadata(index, path, path.relative_to(root).as_posix())
    return state

def check_copilot_snapshot_unchanged(home, case_root, before, after, stage, log_limit):
    """Only this real read may create bounded new vendor process logs."""
    prefix = home.name + "/" + str(Path(".copilot") / "logs") + os.sep
    created = [key for key in after if key not in before and key.startswith(prefix)]
    if len(created) > log_limit:
        raise AssertionError(stage + ": excessive new Copilot process logs")
    for key in created:
        relative = key[len(home.name) + 1:]
        path = home / relative
        info = path.lstat()
        if (not re.fullmatch(r"process-[0-9]+-[0-9]+\.log", path.name)
                or path.parent != home / ".copilot/logs"
                or not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > 1 << 20
                or not path_inside_case(path, case_root)):
            raise AssertionError(stage + ": unsafe new Copilot process log")
        after.pop(key)
        after.pop("metadata:0:" + Path(relative).as_posix(), None)
    if after != before:
        raise AssertionError(stage + ": native query modified protected state")

def exact_native_identity(text, identity):
    """Recognize an identity token in native data, never a URL substring."""
    expected = identity.casefold()

    def contains(value):
        if isinstance(value, str):
            return value.casefold() == expected
        if isinstance(value, dict):
            return any(str(key).casefold() == expected or contains(child)
                       for key, child in value.items())
        if isinstance(value, list):
            return any(contains(child) for child in value)
        return False

    try:
        return contains(json.loads(text))
    except ValueError:
        escaped = re.escape(identity)
        # TOML/YAML-style native registrations: exact keys or identity fields.
        # An unrelated URL such as https://github.com cannot qualify "github".
        return bool(
            re.search(r"""(?mi)^\s*['"]?""" + escaped + r"""['"]?\s*[:=]""", text)
            or re.search(
                r"""(?mi)^\s*(?:name|id|plugin|plugin_id|identity)\s*[:=]\s*"""
                r"""['"]?""" + escaped + r"""['"]?\s*(?:$|[,#}])""", text)
        )


def codex_owned_native_config(path, identity, case_root):
    """Codex stores exact plugin@marketplace keys, not bare plugin names."""
    try:
        state = tomllib.loads(path.read_text(encoding="utf-8"))
        plugins = state.get("plugins", {})
        markets = state.get("marketplaces", {})
        if not isinstance(plugins, dict) or not isinstance(markets, dict):
            return False
        for spec in plugins:
            if not isinstance(spec, str) or not spec.startswith(identity + "@"):
                continue
            marketplace = spec[len(identity) + 1:]
            if not re.fullmatch(r"agentplugins-[a-f0-9]{12}", marketplace):
                continue
            entry = markets.get(marketplace)
            if not isinstance(entry, dict) or entry.get("source_type", "local") != "local":
                continue
            source = entry.get("source")
            if not isinstance(source, str) or not Path(source).is_absolute():
                continue
            if Path(source).resolve().is_relative_to(case_root.resolve()):
                return True
    except (OSError, TypeError, ValueError):
        return False
    return False


def native_components_from_add(parsed):
    """Read exact natively supported component identities from the released CLI.

    Metadata is a candidate list only; each name still needs independent,
    changed on-disk native evidence inside the disposable client profile.
    """
    if not isinstance(parsed, dict) or not isinstance(parsed.get("data"), dict):
        return ()
    actions = parsed["data"].get("actions")
    if not isinstance(actions, list):
        return ()
    names = set()
    for action in actions:
        if not isinstance(action, dict) or not isinstance(action.get("component_support"), list):
            continue
        for component in action["component_support"]:
            if not isinstance(component, dict) or component.get("support") != "native":
                continue
            name = component.get("name")
            if isinstance(name, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}", name):
                names.add(name)
    return tuple(sorted(names))


def prepared_only_add(parsed):
    """Never call a prepared-only package a native installation."""
    if not isinstance(parsed, dict) or not isinstance(parsed.get("data"), dict):
        return False
    actions = parsed["data"].get("actions")
    if not isinstance(actions, list) or not actions:
        return False
    found = False
    for action in actions:
        if not isinstance(action, dict) or not isinstance(action.get("component_support"), list):
            return False
        for component in action["component_support"]:
            if not isinstance(component, dict) or component.get("support") != "prepared":
                return False
            found = True
    return found


def attributable_native_changes(native_roots, changes, identity, target, component_names=()):
    """Credit only identity-attributed active native state, not cached downloads."""
    found = []
    identities = tuple(dict.fromkeys((identity, *component_names)))
    normalized = {name.casefold() for name in identities}
    for root in native_roots:
        if not root.is_dir() or root.is_symlink():
            continue
        for relpath, _digest in changes:
            prefix = root.name + "/"
            if not relpath.startswith(prefix):
                continue
            relative = relpath[len(prefix):]
            if passive_native_path(target, relative):
                continue
            leaf = root / relative
            if leaf.is_symlink() or not leaf.is_file():
                continue
            try:
                # A complete name component (or file stem) is an attributable
                # managed native path; arbitrary substring matches are not.
                owned_path = any(
                    component.casefold() in normalized
                    or Path(component).stem.casefold() in normalized
                    for component in Path(relative).parts
                )
                # Larger files without an attributable path stay unqualified.
                owned_content = False
                if not owned_path and leaf.stat().st_size <= 1024 * 1024:
                    if target == "codex" and relative == "config.toml":
                        owned_content = codex_owned_native_config(
                            leaf, identity, root.parent.parent)
                    else:
                        content = leaf.read_text(encoding="utf-8", errors="replace")
                        owned_content = any(exact_native_identity(content, name)
                                            for name in identities)
                if owned_path or owned_content:
                    found.append(relpath)
            except OSError:
                continue
    return sorted(set(found))


def source_target_incompatibility(diagnostic, target=None):
    """Recognize narrowly known client/source-policy refusals, never PASS."""
    diagnostic = diagnostic.casefold()
    return ("release does not support complete target set" in diagnostic
            or (target == "pi" and
                "pi package manifest overrides selected extension discovery" in diagnostic))


def is_cli_refusal(output):
    """Diagnostic words cannot turn a successful add or timeout into a refusal."""
    return type(output.get("exit_code")) is int and output["exit_code"] != 0


def host_containment_refusal(diagnostic):
    """Selected native process custody was explicitly unavailable, not a bad source."""
    diagnostic = diagnostic.casefold()
    return ("manual_activation_required" in diagnostic and
            ("cgroup" in diagnostic or
             "safe duplex process containment unavailable" in diagnostic))


def passive_native_path(target, relative):
    """Downloaded caches and run logs are not active native registrations."""
    normalized = "/".join(Path(relative).parts)
    return ((target == "codex" and normalized.startswith("plugins/cache/"))
            or (target == "qoder" and normalized.startswith(("plugins/cache/", "logs/runs/")))
            # Official Grok user-guide Markdown is passive vendor documentation,
            # not a managed plugin registration. No other paths are excluded.
            or (target == "grok" and normalized in (
                "docs/user-guide/05-configuration.md",
                "docs/user-guide/16-subagents.md")))


def qoder_passive_latest_log_link(root, file):
    """Qualify only Qoder's real, contained run-log pointer, never a plugin link."""
    logs = root / "logs"
    if file != logs / "latest" or root.is_symlink() or logs.is_symlink():
        return False
    try:
        relative = file.readlink()
        if relative.is_absolute() or not path_inside_case(logs, root):
            return False
        destination = Path(os.path.normpath(str(logs / relative)))
        if not any(destination.is_relative_to(logs / leaf)
                   for leaf in ("runs", "sessions")):
            return False
        return (path_inside_case(destination, logs)
                and (destination.is_file() or destination.is_dir()))
    except (OSError, RuntimeError, ValueError):
        return False


def qoder_native_plugin_state(home, env, cwd, identity, expected_version=None):
    """Read real Qoder 1.1 native JSON; never qualify from a package cache."""
    profile = home / ".qoder"
    executable = next((AGENT_BINARIES[name] for name in EXECUTABLE["qoder"]
                       if AGENT_BINARIES.get(name)), None)
    # Both native profile selectors must remain inside the disposable HOME.
    # The executable was pinned before any untrusted source installation.
    if (not executable or not Path(executable).is_absolute()
            or not profile.is_dir() or profile.is_symlink()
            or not path_inside_case(profile, home)
            or env.get("QODER_CONFIG_DIR") != str(profile.resolve(strict=True))):
        return {"status": "unqualified-prerequisites"}
    observed = call([executable, "--config-dir", str(profile),
                     "plugins", "list", "--json"], env, cwd, 30)
    entries = observed.get("_json")
    if observed.get("exit_code") != 0 or not isinstance(entries, list) or len(entries) > 1024:
        return {"status": "unqualified-list"}
    seen = set()
    claimed = []
    for entry in entries:
        if not isinstance(entry, dict) or not isinstance(entry.get("id"), str):
            return {"status": "unqualified-schema"}
        native_id = entry["id"]
        if (native_id.count("@") != 1 or not all(native_id.split("@", 1))
                or native_id in seen
                or ("version" in entry and not isinstance(entry["version"], str))):
            return {"status": "unqualified-schema"}
        seen.add(native_id)
        if native_id.split("@", 1)[0] == identity:
            claimed.append(entry)
    if not claimed:
        return {"status": "absent"}
    if (len(claimed) == 1 and claimed[0]["id"] == identity + "@local"
            and isinstance(expected_version, str) and expected_version
            and claimed[0].get("version") == expected_version):
        return {"status": "registered"}
    return {"status": "unqualified-identity-or-version"}


def copilot_json_document(text):
    def unique(pairs):
        if len({key for key, _ in pairs}) != len(pairs):
            raise ValueError("ambiguous Copilot JSON object")
        return dict(pairs)
    def reject_constant(_):
        raise ValueError("invalid Copilot JSON constant")
    return json.loads(text, object_pairs_hook=unique, parse_constant=reject_constant)

COPILOT_BUILTIN_JSON = [{"name": "browser-use", "marketplace": "", "version": "0.0.4",
                         "enabled": False, "source": "builtin"}]
COPILOT_BUILTIN_TEXT = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"
COPILOT_EMPTY_TEXT = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n"


def copilot_native_entries_state(entries, owners):
    """Check the complete observed .105 live JSON identity and source schema."""
    if not isinstance(entries, list) or len(entries) != len(owners) + 1:
        raise AssertionError("Copilot native plugin listing malformed or incomplete")
    found, builtin = set(), []
    for entry in entries:
        if (not isinstance(entry, dict) or not isinstance(entry.get("name"), str)
                or type(entry.get("enabled")) is not bool or not isinstance(entry.get("source"), str)
                or (entry.get("version") is not None and not isinstance(entry["version"], str))):
            raise AssertionError("Copilot native plugin entry malformed")
        name = entry["name"]
        if name == "browser-use":
            builtin.append(entry)
            continue
        if (name not in owners or name in found or entry.get("marketplace") != owners[name]["marketplace"]
                or entry["enabled"] is not True
                or entry.get("version") != owners[name]["version"]
                or entry["source"] != "live" or entry.get("installedFrom") != owners[name]["target_locator"]):
            raise AssertionError("Copilot native managed identity is foreign or duplicated")
        found.add(name)
    if builtin != COPILOT_BUILTIN_JSON or found != set(owners):
        raise AssertionError("Copilot native closure or observed builtin baseline changed")
    return {"status": "installed" if owners else "absent", "managed_count": len(found)}


def copilot_native_text_state(text, owners):
    """Consume complete .105 live listing, including each exact physical path."""
    document = text.replace("\r\n", "\n")
    if not owners:
        if document != COPILOT_EMPTY_TEXT + COPILOT_BUILTIN_TEXT:
            raise AssertionError("Copilot native empty text differs from observed baseline")
        return
    if not document.startswith(COPILOT_BUILTIN_TEXT):
        raise AssertionError("Copilot native builtin prefix changed")
    document = document[len(COPILOT_BUILTIN_TEXT):].removesuffix("\n")
    lines = document.split("\n")
    if (not lines or lines[0] != "Live Plugins (loaded from a local marketplace directory, never copied):"
            or len(lines) != 1 + 2 * len(owners)):
        raise AssertionError("Copilot native full live listing incomplete")
    found = set()
    for index in range(1, len(lines), 2):
        entry = re.fullmatch(r"  • ([A-Za-z0-9][A-Za-z0-9._-]*)@(agentplugins-[0-9a-f]{12}) \(v([^() ]+)\) \(enabled\)", lines[index])
        if (entry is None or entry[1] not in owners or entry[1] in found
                or entry[2] != owners[entry[1]]["marketplace"] or entry[3] != owners[entry[1]]["version"]
                or lines[index + 1] != "      from " + owners[entry[1]]["target_locator"]):
            raise AssertionError("Copilot native physical identity/version/source differs")
        found.add(entry[1])
    if found != set(owners):
        raise AssertionError("Copilot native physical closure incomplete")

def marketing_kit_signed_pins(fixture_pins):
    """Extend the reviewed 30-source catalogue only with its signed kit child."""
    if not isinstance(fixture_pins, dict):
        return None
    pins = {}
    for alias, distribution in MARKETING_KIT_DISTRIBUTIONS.items():
        anchor = MARKETING_KIT_SIGNED_CLOSURE[distribution]
        pin = {"distribution": distribution, "repository": anchor["repository"],
               "path": anchor["package_path"], "revision": anchor["resolved_revision"],
               "sequence": anchor["release_sequence"], "tree_digest": anchor["tree_digest"],
               "manifest_digest": anchor["manifest_digest"]}
        # HubSpot is a kit dependency, not a 31st top-level catalogue case.
        # Never silently replace a changed or missing reviewed catalogue pin.
        if alias != "hubspot-crm" or alias in fixture_pins:
            supplied = fixture_pins.get(alias)
            if (not isinstance(supplied, dict) or type(supplied.get("sequence")) is not int
                    or supplied != pin):
                return None
        pins[alias] = pin
    return pins


def copilot_signed_kit_owners(list_json, pins, data, case_root):
    """Qualify all four signed User bindings before trusting native siblings."""
    aliases = tuple(MARKETING_KIT_DISTRIBUTIONS)
    pins = marketing_kit_signed_pins(pins)
    if pins is None:
        return None
    text, status = copilot_settings_read(data / "state-v2.json", case_root)
    if status is not None:
        return None
    try:
        state = json.loads(text, object_pairs_hook=_unique_json_object)
        records = state["installations"]
        receipts = state["environment_receipts"]
        public_records = list_json["data"]["installations"]
        if (not isinstance(public_records, list) or len(public_records) != 4
                or not isinstance(records, list) or len(records) != 4
                or not isinstance(receipts, list) or len(receipts) != 4):
            return None
        owners, receipt_keys, physical_ids, binding_ids = {}, set(), set(), set()
        for alias in aliases:
            pin = pins[alias]
            matches = matching_installations(list_json, alias, "copilot", "plugin", alias, pins)
            if len(matches) != 1:
                return None
            public = matches[0]
            public_clients = public["clients"]
            if (not isinstance(public_clients, list) or len(public_clients) != 1
                    or public_clients[0].get("client_id") != "copilot"):
                return None
            version = MARKETING_KIT_SIGNED_CLOSURE[pin["distribution"]]["package_version"]
            public_revision = public_clients[0]["package_revision"]
            if (public_revision.get("version") != version
                    or type(public_revision.get("release_sequence")) is not int):
                return None
            selected = [v for v in records if v["directory"].get("distribution_id") == pin["distribution"]]
            if len(selected) != 1:
                return None
            record = selected[0]
            directory, source, package = (record[k] for k in ("directory", "source", "package"))
            if (record["installation_id"] != public["installation_id"]
                    or record.get("origin_mode") != "directory"
                    or type(directory.get("desired_release_sequence")) is not int
                    or directory.get("desired_release_sequence") != pin["sequence"]
                    or package.get("version") != version
                    or package.get("manifest_digest") != pin["manifest_digest"]
                    or any(source.get(key) != pin[field] for key, field in (
                        ("repository", "repository"), ("package_subpath", "path"),
                        ("resolved_revision", "revision"), ("tree_digest", "tree_digest")))):
                return None
            bindings = record["clients"]
            if not isinstance(bindings, dict) or len(bindings) != 1:
                return None
            binding_id, binding = next(iter(bindings.items()))
            physical = binding.get("physical_artifact_id")
            target = Path(binding.get("target_locator", ""))
            revision = binding.get("package_revision", {})
            if (not isinstance(binding_id, str) or not binding_id or binding_id in binding_ids
                    or binding.get("client_binding_id") != binding_id
                    or binding.get("client_id") != "copilot" or binding.get("scope") != "user"
                    or binding.get("materialization") != "materialized"
                    or binding.get("verification") != "installation_verified"
                    or binding.get("affected_surfaces") != ["copilot", "vscode"]
                    or not isinstance(physical, str) or not physical.strip()
                    or physical != physical.strip() or physical in physical_ids
                    or physical in (".", "..") or Path(physical).name != physical
                    or not target.is_absolute() or not target.is_dir()
                    or not path_inside_case(target, case_root)
                    or not isinstance(revision, dict)
                    or type(revision.get("release_sequence")) is not int
                    or any(revision.get(key) != expected for key, expected in (
                        ("distribution_id", pin["distribution"]), ("release_sequence", pin["sequence"]),
                        ("version", version), ("resolved_revision", pin["revision"]),
                        ("tree_digest", pin["tree_digest"]),
                        ("manifest_digest", pin["manifest_digest"])))):
                return None
            binding_receipts = binding.get("receipts")
            if (not isinstance(binding_receipts, list) or not binding_receipts
                    or any(not isinstance(v, dict) or v.get("phase") != "committed"
                           or v.get("client_binding_id") != binding_id
                           or v.get("active_path") != str(target)
                           or not isinstance(v.get("operation_id"), str) or not v["operation_id"]
                           for v in binding_receipts)
                    or len({v["operation_id"] for v in binding_receipts}) != len(binding_receipts)):
                return None
            marketplace = "agentplugins-" + hashlib.sha256(physical.encode()).hexdigest()[:12]
            manifest_text, manifest_status = copilot_settings_read(
                target / ".github/plugin/marketplace.json", case_root)
            if manifest_status is not None:
                return None
            manifest = json.loads(manifest_text, object_pairs_hook=_unique_json_object)
            if (not isinstance(manifest, dict) or manifest.get("name") != marketplace
                    or not isinstance(manifest.get("plugins"), list)
                    or len(manifest["plugins"]) != 1
                    or any(manifest["plugins"][0].get(key) != value for key, value in (
                        ("name", alias), ("version", version), ("source", ".")))):
                return None
            owners[alias] = {"marketplace": marketplace, "version": version,
                             "target_locator": str(target)}
            receipt_keys.add((alias, "copilot", public["installation_id"],
                              binding_id, "directory:" + pin["distribution"], pin["tree_digest"]))
            physical_ids.add(physical)
            binding_ids.add(binding_id)
        observed = {tuple(item.get(key) for key in (
            "alias", "client_id", "installation_id", "client_binding_id",
            "source_key", "package_digest")) for item in receipts}
        if (observed != receipt_keys or len(observed) != 4
                or len({v.get("receipt_id") for v in receipts}) != 4
                or any(v.get("phase") != "committed" or v.get("environment_id") != "user-default"
                       or not isinstance(v.get("receipt_id"), str) or not v["receipt_id"]
                       for v in receipts)):
            return None
        return owners
    except (KeyError, IndexError, TypeError, ValueError, AttributeError):
        return None


def copilot_signed_kit_settings(home, case_root, owners, enabled):
    """Require exactly four physical preferences and only owned marketplaces."""
    if type(enabled) is not bool or not isinstance(owners, dict) or set(owners) != {
            "marketing-kit", "notion", "hubspot-crm", "playwright"}:
        return False
    text, status = copilot_settings_read(home / ".copilot/settings.json", case_root)
    if status is not None:
        return False
    try:
        document = copilot_json_document(text)
        expected = {name + "@" + owner["marketplace"] for name, owner in owners.items()}
        preferences = document.get("enabledPlugins") if isinstance(document, dict) else None
        if (not isinstance(document, dict)
                or set(document) not in ({"enabledPlugins"},
                                        {"enabledPlugins", "extraKnownMarketplaces"})
                or not isinstance(preferences, dict) or set(preferences) != expected
                or any(value is not enabled for value in preferences.values())):
            return False
        marketplaces = document.get("extraKnownMarketplaces", {})
        if not isinstance(marketplaces, dict):
            return False
        if not enabled:
            return marketplaces == {}
        # Copilot .105 retains owned directory registrations while installed.
        # Complete native JSON and TEXT independently require all four plugins.
        expected_sources = {
            owner["marketplace"]: {"source": {"source": "directory",
                                               "path": owner["target_locator"]}}
            for owner in owners.values()}
        return all(name in expected_sources and value == expected_sources[name]
                   for name, value in marketplaces.items())
    except (TypeError, ValueError, KeyError, AttributeError):
        return False


def copilot_native_roots_guard(home, env, project):
    """Capture owned root identities before native queries or protected reads."""
    case = project.parent

    def stamp(path):
        value = path.lstat()
        if not stat.S_ISDIR(value.st_mode):
            raise AssertionError("Copilot native protected root is not a directory")
        return value.st_mode, value.st_dev, value.st_ino

    try:
        if case.is_symlink():
            raise AssertionError("Copilot native case is a symlink")
        canonical = case.resolve(strict=True)
        case_identity = stamp(case)
        roots = (home, Path(env["AGENTPLUGINS_HOME"]), project, home / ".copilot")

        def inside(path):
            # A trusted macOS temporary ancestor may use /var or /private/var.
            # Neither spelling may redefine the captured case authority.
            return path_inside_case(path, case) or path_inside_case(path, canonical)

        expected = []
        for path in roots:
            if not inside(path):
                raise AssertionError("Copilot native protected root escaped case")
            expected.append((path, stamp(path)))
    except (OSError, RuntimeError, ValueError) as error:
        raise AssertionError("Copilot native protected root unavailable") from error

    def guard():
        try:
            if (case.is_symlink() or case.resolve(strict=True) != canonical
                    or stamp(case) != case_identity or stamp(canonical) != case_identity):
                raise AssertionError("Copilot native case redirected")
            for path, identity in expected:
                if not inside(path) or stamp(path) != identity:
                    raise AssertionError("Copilot native protected root redirected")
        except (OSError, RuntimeError, ValueError) as error:
            raise AssertionError("Copilot native protected root unavailable") from error

    guard()
    return guard


def copilot_native_plugin_state(home, env, cwd, identity, expected_version=None,
                                owners=None, owned_call=None):
    """Read the original profile; signed kits require complete JSON and TEXT."""
    profile = home / ".copilot"
    executable = AGENT_BINARIES.get("copilot")
    if (not executable or not Path(executable).is_absolute()
            or not profile.is_dir() or profile.is_symlink()
            or not path_inside_case(profile, home)):
        return {"status": "unqualified-prerequisites"}
    native_env = dict(env, COPILOT_HOME=str(profile))
    if owners is not None:
        if (not isinstance(owners, dict)
                or (owners and set(owners) != set(MARKETING_KIT_DISTRIBUTIONS))):
            return {"status": "unqualified-signed-closure"}
        project = Path(cwd)
        case_root = project.parent
        roots = (home, Path(env["AGENTPLUGINS_HOME"]), project)
        assert_owned_roots = copilot_native_roots_guard(home, env, project)

        def observe_owned():
            assert_owned_roots()
            return scoped_snapshot(roots)

        before = observe_owned()
        try:
            responses = []
            invoke_owned = call if owned_call is None else owned_call
            for suffix in (["--json"], []):
                assert_owned_roots()
                if (not profile.is_dir() or profile.is_symlink()
                        or not path_inside_case(profile, home)):
                    return {"status": "unqualified-prerequisites"}
                try:
                    result = invoke_owned([executable, "plugin", "list", *suffix],
                                          native_env, cwd, 30, capture_full=True)
                finally:
                    assert_owned_roots()
                if (type(result.get("exit_code")) is not int or result["exit_code"] != 0
                        or result.get("timeout") is not False or result.get("stderr_full") != ""
                        or not isinstance(result.get("stdout_full"), str)
                        or len(result["stdout_full"].encode("utf-8")) > 1 << 20):
                    return {"status": "unqualified-list"}
                responses.append(result["stdout_full"])
            try:
                state = copilot_native_entries_state(copilot_json_document(responses[0]), owners)
                copilot_native_text_state(responses[1], owners)
                return state
            except (AssertionError, ValueError, TypeError, KeyError, AttributeError):
                return {"status": "unqualified-signed-closure"}
        finally:
            check_copilot_snapshot_unchanged(
                home, case_root, before, observe_owned(), "Copilot native read-only query", 2)
    observed = call([executable, "plugin", "list", "--json"], native_env, cwd, 30)
    entries = observed.get("_json")
    if observed.get("exit_code") != 0 or not isinstance(entries, list) or len(entries) > 1024:
        return {"status": "unqualified-list"}
    found = 0
    for entry in entries:
        if (not isinstance(entry, dict)
                or not isinstance(entry.get("name"), str)
                or not isinstance(entry.get("enabled"), bool)
                or not isinstance(entry.get("source"), str)
                or ("version" in entry and entry["version"] is not None
                    and not isinstance(entry["version"], str))
                or ("marketplace" in entry and entry["marketplace"] is not None
                    and not isinstance(entry["marketplace"], str))):
            return {"status": "unqualified-schema"}
        market = entry.get("marketplace")
        managed = (isinstance(market, str)
                   and re.fullmatch(r"agentplugins-[0-9a-f]{12}", market))
        if entry["name"] != identity:
            # A managed marketplace under a reminted name is not native absence.
            if managed:
                return {"status": "unqualified-reminted-managed"}
            continue
        if not managed:
            return {"status": "unqualified-foreign-name"}
        if (entry["enabled"] is not True
                or (entry.get("version") is not None
                    and entry["version"] != expected_version)):
            return {"status": "unqualified-activation-or-version"}
        found += 1
    if found == 0:
        return {"status": "absent"}
    if found == 1 and isinstance(expected_version, str) and expected_version:
        return {"status": "installed"}
    return {"status": "unqualified-identity"}


def copilot_disabled_preference_only(before, after_add, after_remove,
                                    native_before, native_after, residuals):
    """Accept no active registration, without deleting a vendor's disabled preference."""
    if (not isinstance(before, dict) or before.get("status") != "absent"
            or not isinstance(after_add, dict) or after_add.get("status") != "parsed"
            or not isinstance(after_remove, dict) or after_remove.get("status") != "parsed"
            or not isinstance(native_before, dict) or native_before.get("status") != "installed"
            or not isinstance(native_after, dict) or native_after.get("status") != "absent"
            or residuals != [".copilot/settings.json"]
            or after_remove.get("allowed_sections") != ["enabledPlugins"]):
        return False
    expected_after = {"exact_key": 0, "qualified_key": 1, "reference_key": 0,
                      "exact_value": 0, "qualified_value": 0, "reference_value": 0}
    return (after_add.get("enabled_plugin_value_counts") ==
            {"true": 1, "false": 0, "non_boolean": 0}
            and after_remove.get("enabled_plugin_value_counts") ==
            {"true": 0, "false": 1, "non_boolean": 0}
            and after_remove.get("match_counts") == expected_after)


def _codex_system_skill_checkpoint(home, original):
    """Join the full pinned bundle proof to the original objects and fresh bytes."""
    profile = home / ".codex"
    files, _, metadata = CODEX_RUNTIME_AUDIT["system_skill_initialization"](profile)
    if metadata != original:
        raise ValueError("Codex pinned system Skill authority changed")
    observed = scoped_snapshot((home,))
    for path, held in metadata.items():
        key = "metadata:0:" + path.relative_to(home).as_posix()
        if observed.get(key) != held:
            raise ValueError("Codex pinned system Skill snapshot identity changed")
    for path, digest in files.items():
        if observed.get(home.name + "/" + str(path.relative_to(home))) != digest:
            raise ValueError("Codex pinned system Skill snapshot bytes changed")
    return {profile.name + "/" + str(path.relative_to(profile)): digest
            for path, digest in files.items()}


def native_identity_residuals(native_roots, identity, observed_paths, target,
                              component_names=(), *, client_version=None,
                              bootstrap_proof=None):
    """After remove, reject any remaining named native file/config entry.

    Codex may retain its downloaded source cache after its native plugin and
    marketplace are removed; that cache is not active client configuration.
    """
    leftover = []
    identities = tuple(dict.fromkeys((identity, *component_names)))
    normalized = {name.casefold() for name in identities}

    def named_path(relative):
        return any(
            part.casefold() in normalized or Path(part).stem.casefold() in normalized
            or any(re.fullmatch(re.escape(name) + r"@agentplugins-[0-9a-f]{12}",
                                Path(part).stem, re.IGNORECASE) for name in identities)
            for part in Path(relative).parts)

    def named_content(contents):
        qualified = any(re.search(
            r"(?<![A-Za-z0-9_./:-])" + re.escape(name)
            + r"@agentplugins-[0-9a-f]{12}(?![A-Za-z0-9_./-])",
            contents, re.IGNORECASE) for name in identities)
        return qualified or any(exact_native_identity(contents, name) for name in identities)

    for root in native_roots:
        prefix = root.name + "/"
        if root.is_symlink():
            leftover.append(root.name + ": symlink")
            continue
        for observed in observed_paths:
            if not observed.startswith(prefix):
                continue
            relative = observed[len(prefix):]
            # Synthetic os.walk error markers have no on-disk file to lstat.
            # Retain the error as negative proof instead of treating it as gone.
            if relative == ".walk-error" or relative.endswith("/.walk-error"):
                leftover.append(observed + ": unreadable-walk")
                continue
            file = root / relative
            try:
                mode = file.lstat().st_mode
                if target == "opencode" and OPENCODE_BOOTSTRAP_AUDIT["candidate"](relative):
                    proof = (OPENCODE_BOOTSTRAP_AUDIT["qualify"](
                        root, relative, client_version, named_path, named_content)
                        if all(name.strip() for name in identities) and not named_path(relative)
                        else None)
                    if proof is None:
                        leftover.append(observed + ": unverified-native-bootstrap")
                    elif bootstrap_proof is not None:
                        bootstrap_proof.append(proof)
                    continue
                if stat.S_ISLNK(mode):
                    if (target != "qoder" or "/".join(Path(relative).parts) != "logs/latest"
                            or not qoder_passive_latest_log_link(root, file)):
                        leftover.append(observed + ": symlink")
                elif not stat.S_ISREG(mode):
                    leftover.append(observed + ": non-regular")
                elif passive_native_path(target, relative):
                    continue
                elif named_path(relative):
                    leftover.append(observed)
                elif file.stat().st_size > 1024 * 1024:
                    # Unread content must never count as clean native state.
                    leftover.append(observed + ": unscanned-too-large")
                else:
                    contents = file.read_text(encoding="utf-8", errors="replace")
                    if named_content(contents):
                        leftover.append(observed)
            except FileNotFoundError:
                continue
            except OSError:
                leftover.append(observed + ": unreadable")
    return sorted(set(leftover))


def path_inside_case(path, case_root):
    """Reject intermediate symlinks as well as escaping or unreadable roots."""
    try:
        if case_root.is_symlink() or not path.is_relative_to(case_root):
            return False
        if not path.resolve().is_relative_to(case_root.resolve(strict=True)):
            return False
        cursor = path
        while cursor != case_root:
            if cursor.is_symlink():
                return False
            cursor = cursor.parent
        return True
    except (OSError, RuntimeError, ValueError):
        return False


def project_skill_scan_roots(target):
    """Scan owned projections without changing independent foreign fixtures."""
    return PROJECT_SKILL_ROOTS.get(target, []) + (
        [".uap/managed/clients/copilot"] if target == "copilot" else [])


def skill_removal_leftovers(project, target, skill_name, installed_path=None):
    """Reject surviving Skills and substituted intermediate project roots."""
    leftovers = []
    case_root = project.parent
    for relative_root in project_skill_scan_roots(target):
        base = project / relative_root
        named = base / skill_name
        if not path_inside_case(base, case_root) or not path_inside_case(named, case_root):
            leftovers.append(str(base.relative_to(project)) + ": unsafe ancestor")
        elif named.exists() or named.is_symlink():
            leftovers.append(str(named.relative_to(project)))
    if installed_path:
        path = project / installed_path
        if not path_inside_case(path, case_root):
            leftovers.append(str(installed_path) + ": unsafe ancestor")
        elif path.exists() or path.is_symlink():
            leftovers.append(str(path.relative_to(project)))
    return sorted(set(leftovers))


def skill_file_present(project, target, skill_name):
    case_root = project.parent
    for relative_root in project_skill_scan_roots(target):
        base = project / relative_root
        if not path_inside_case(base, case_root) or not base.is_dir():
            continue
        for f in base.rglob("SKILL.md"):
            if not path_inside_case(f, case_root) or not f.is_file():
                continue
            try:
                head = f.read_text(encoding="utf-8", errors="replace")[:3000]
            except OSError:
                continue
            if re.search(r"(?m)^name:\s*['\"]?" + re.escape(skill_name) + r"['\"]?\s*$", head):
                return str(f.relative_to(project))
    return None


def copilot_project_skill_path(project, fixture, receipt, matches, *, repeated=False):
    """Find only the exact raw Skill selected by public source/IID receipts.

    This is a file/source proof, not a Ready qualifier. The existing public
    add/repeat readiness gate remains mandatory. A recursive same-name match
    anywhere else in the managed root cannot substitute for this destination.
    """
    if (type(fixture) is not dict or type(receipt) is not dict
            or type(matches) is not list or len(matches) != 1
            or type(matches[0]) is not dict
            or type(receipt.get("schema_version")) is not int
            or receipt.get("schema_version") != 1
            or receipt.get("command") != "skills add"
            or receipt.get("result") != "success"
            or receipt.get("status") != "succeeded"
            or receipt.get("scope") != "project"):
        return None
    selected = receipt.get("items")
    public = matches[0]
    if (type(selected) is not list or len(selected) != 1
            or type(selected[0]) is not dict):
        return None
    identities = PREPARED_INSTALLATION["grok_skill_identity"](
        fixture, public.get("installation_id"))
    if identities is None:
        return None
    candidate, name, physical = identities
    identity, source_path = fixture["name"], "skills/" + fixture["folder"]
    if (identity in (".", "..") or Path(identity).name != identity
            or "\\" in identity or any(c in identity for c in "\x00\r\n")):
        return None
    destination = project / ".uap/managed/clients/copilot" / physical
    chosen = selected[0]
    if (public.get("name") != name
            or public.get("source") != fixture["repo"] + "/" + source_path
            or chosen.get("alias") != identity or chosen.get("targetId") != "copilot"
            or chosen.get("status") != ("unchanged" if repeated else "succeeded")
            or chosen.get("candidateId") != candidate
            or chosen.get("destinationPath") != str(destination)):
        return None
    clients = public.get("clients")
    if (type(clients) is not list or len(clients) != 1 or type(clients[0]) is not dict
            or any(clients[0].get(key) != value for key, value in (
                ("client_id", "copilot"), ("scope", "project"),
                ("materialization", "materialized"), ("policy", "allowed"),
                ("verification", "installation_verified")))):
        return None
    revision = clients[0].get("package_revision")
    if type(revision) is not dict or revision.get("resolved_revision") != fixture["ref"]:
        return None
    case_root = project.parent
    text, status = copilot_settings_read(project / "uap.lock", case_root)
    if status is not None:
        return None
    try:
        lock = json.loads(text, object_pairs_hook=_unique_json_object,
                          parse_constant=_invalid_json_constant)
        packages = lock["packages"]
        if type(packages) is not list or any(type(p) is not dict for p in packages):
            return None
        selected_lock = [p for p in packages if p.get("alias") == identity]
        if len(selected_lock) != 1:
            return None
        package = selected_lock[0]
        git = package["source"]["git"]
        targets = package["targets"]
        if (type(git) is not dict or type(targets) is not list
                or targets != ["copilot"]
                or git.get("kind") != "skill" or git.get("skill") != identity
                or type(git.get("repository")) is not str
                or git["repository"].casefold() != fixture["repo"].casefold()
                or git.get("commit") != fixture["ref"]
                or git.get("path") != source_path
                or git.get("resolved_skill_path") != source_path):
            return None
        for key in ("tree_digest", "manifest_digest"):
            value = revision.get(key)
            if (type(value) is not str or re.fullmatch(r"sha256:[0-9a-f]{64}", value) is None
                    or package.get(key) != value):
                return None
    except (KeyError, TypeError, ValueError):
        return None
    skill = destination / "skills" / identity / "SKILL.md"
    text, status = copilot_settings_read(skill, case_root)
    if status is not None:
        return None
    body = text.encode("utf-8")
    blob = b"blob " + str(len(body)).encode("ascii") + b"\0" + body
    if hashlib.sha1(blob).hexdigest() != fixture.get("skill_md_git_blob_sha1"):
        return None
    return str(skill.relative_to(project))


def copilot_skill_success_response(response, command):
    """Validate the public envelope for this fresh Copilot Skill lifecycle."""
    return (type(response) is dict and type(response.get("schema_version")) is int
            and response.get("schema_version") == 1 and response.get("command") == command
            and response.get("result") == "success")


def copilot_skill_list_ready(response, matches):
    """A Ready receipt must agree with its fresh, single-target public binding.

    Normalized raw Skills have no catalog authentication requirement. Copilot's
    observed pure-Skill qualifier yields not_required after native discovery.
    This acceptance check does not prevent finding/removing unready files.
    """
    if not copilot_skill_success_response(response, "list"):
        return False
    data = response.get("data")
    nodes = data.get("installations") if type(data) is dict else None
    if (type(nodes) is not list or len(nodes) != 1 or type(nodes[0]) is not dict
            or type(matches) is not list or nodes != matches):
        return False
    clients = nodes[0].get("clients")
    return (type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
            and all(clients[0].get(key) == value for key, value in (
                ("client_id", "copilot"), ("scope", "project"),
                ("materialization", "materialized"), ("activation", "active"),
                ("authentication", "not_required"), ("policy", "allowed"),
                ("verification", "installation_verified"))))


def locked_skill_is_pinned(project, item, target):
    """Require the exact source, commit, selected folder and target in the real lock."""
    try:
        lock = json.loads((project / "uap.lock").read_text(encoding="utf-8"))
        selected = [p for p in lock["packages"] if p.get("alias") == item["name"]]
        if len(selected) != 1:
            return False
        package = selected[0]
        source = package["source"]["git"]
        location = "skills/" + item["folder"]
        return bool(
            source.get("kind") == "skill" and source.get("skill") == item["name"]
            and source.get("repository", "").casefold() == item["repo"].casefold()
            and source.get("commit") == item["ref"]
            and (source.get("resolved_skill_path") == location or source.get("path") == location)
            and target in package["targets"]
            and re.fullmatch(r"sha256:[0-9a-f]{64}", package.get("tree_digest", "")) is not None
            and re.fullmatch(r"sha256:[0-9a-f]{64}", package.get("manifest_digest", "")) is not None
        )
    except (OSError, ValueError, KeyError, TypeError):
        return False


def matches_pinned_skill_bytes(project, skill_path, blob_sha1):
    if skill_path is None:
        return False
    try:
        data = (project / skill_path).read_bytes()
        git_blob = b"blob " + str(len(data)).encode("ascii") + b"\0" + data
        return hashlib.sha1(git_blob).hexdigest() == blob_sha1
    except OSError:
        return False


def has_target_registration(response, identity, target, *, skill_source=None):
    """Inspect failed add's list without requiring a verified/active binding."""
    if not isinstance(response, dict):
        return False
    data = response.get("data")
    installations = data.get("installations") if isinstance(data, dict) else None
    if not isinstance(installations, list):
        return False
    return any(
        isinstance(node, dict) and (node.get("name") == identity
                                   or skill_source is not None and node.get("source") == skill_source)
        and isinstance(node.get("clients"), list)
        and any(isinstance(client, dict) and client.get("client_id") == target
                for client in node["clients"])
        for node in installations
    )


def recorded_signed_release_matches(directory, revision):
    """Bind a verified client's package to the recorded signed release."""
    if not isinstance(directory, dict) or not isinstance(revision, dict):
        return False
    distribution = directory.get("recorded_distribution")
    commit = directory.get("recorded_revision")
    sequence = directory.get("recorded_release_sequence")
    if (not isinstance(distribution, str) or not distribution
            or not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit)
            or type(sequence) is not int or sequence < 1):
        return False
    return bool(
        revision.get("distribution_id") == distribution
        and revision.get("resolved_revision") == commit
        and revision.get("release_sequence") == sequence
        and all(isinstance(revision.get(key), str)
                and re.fullmatch(r"sha256:[0-9a-f]{64}", revision[key])
                for key in ("tree_digest", "manifest_digest"))
    )


def signed_release_public_observation(response, identity, target):
    """Only immutable public provenance fields, never raw list/account content."""
    if not isinstance(response, dict) or not isinstance(response.get("data"), dict):
        return []
    nodes = response["data"].get("installations", [])
    if not isinstance(nodes, list):
        return []
    def public_string(value, pattern):
        return value if isinstance(value, str) and re.fullmatch(pattern, value) else None

    # Signed source paths include owner/repo/subdirectories, not just distribution IDs.
    selector = r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+){1,8}"
    sha = r"[0-9a-f]{40}"
    digest = r"sha256:[0-9a-f]{64}"
    evidence = []
    for node in nodes:
        if not isinstance(node, dict) or node.get("name") != identity:
            continue
        directory = node.get("directory")
        directory = directory if isinstance(directory, dict) else {}
        clients = node.get("clients")
        if not isinstance(clients, list):
            continue
        for client in clients:
            if not isinstance(client, dict) or client.get("client_id") != target:
                continue
            revision = client.get("package_revision")
            revision = revision if isinstance(revision, dict) else {}
            evidence.append({
                "source": public_string(node.get("source"), selector),
                "recorded_distribution": public_string(
                    directory.get("recorded_distribution"), selector),
                "recorded_revision": public_string(directory.get("recorded_revision"), sha),
                "recorded_release_sequence": directory.get("recorded_release_sequence")
                if type(directory.get("recorded_release_sequence")) is int else None,
                "resolved_revision": public_string(revision.get("resolved_revision"), sha),
                "release_sequence": revision.get("release_sequence")
                if type(revision.get("release_sequence")) is int else None,
                "tree_digest": public_string(revision.get("tree_digest"), digest),
                "manifest_digest": public_string(revision.get("manifest_digest"), digest),
            })
    return evidence[:3]


def public_target_state_observation(response, identity, target, *,
                                    installation_ids=None, skill_source=None):
    """Bounded public enum projection; never installation acceptance evidence."""
    if response is None:
        return {"status": "unobserved"}
    data = response.get("data") if isinstance(response, dict) else None
    nodes = data.get("installations") if isinstance(data, dict) else None
    if not isinstance(nodes, list):
        return {"status": "invalid"}
    enums = {
        "materialization": ("absent", "staged", "materialized", "degraded"),
        "activation": ("not_required", "prepared", "manual_activation_required", "active", "failed"),
        "authentication": ("not_required", "not_checked", "auth_pending", "authenticated", "failed"),
        "policy": ("allowed", "blocked", "approval_required"),
        "verification": ("not_run", "package_validated", "installation_verified", "runtime_verified", "failed"),
    }
    selected_nodes = selected_clients = invalid_rows = 0
    truncated = len(nodes) > 32
    states = []
    for node in nodes[:32]:
        if not isinstance(node, dict):
            invalid_rows += 1
            continue
        selected = (node.get("name") == identity if installation_ids is None else (
            (isinstance(node.get("installation_id"), str)
             and node["installation_id"] in installation_ids)
            or (skill_source is not None and node.get("source") == skill_source)))
        if not selected:
            continue
        selected_nodes += 1
        clients = node.get("clients")
        if not isinstance(clients, list):
            invalid_rows += 1
            continue
        truncated = truncated or len(clients) > 32
        for client in clients[:32]:
            if not isinstance(client, dict):
                invalid_rows += 1
                continue
            if client.get("client_id") != target:
                continue
            if selected_clients == 32:
                truncated = True
                continue
            selected_clients += 1
            state = {key: value if isinstance(value := client.get(key), str) and value in allowed
                     else "invalid" for key, allowed in enums.items()}
            if state not in states:
                if len(states) < 4:
                    states.append(state)
                else:
                    truncated = True
    return {"status": "observed", "selected_installations": selected_nodes,
            "selected_bindings": selected_clients, "invalid_rows": min(invalid_rows, 32),
            "states": states, "truncated": truncated}


def prepared_package_binding(client, target):
    """A materialized User package can have a lifecycle without native discovery."""
    return (isinstance(client, dict) and client.get("client_id") == target
            and client.get("scope") == "user"
            and client.get("materialization") == "materialized"
            and client.get("activation") == "prepared"
            and client.get("verification") == "package_validated"
            and client.get("policy") == "allowed")


def kiro_direct_prepared_installation(node, identity, target, kind, item):
    """Bind Kiro's non-native User preparation to one exact Git source and UUID."""
    if target != "kiro" or kind != "plugin" or type(item) is not str or type(node) is not dict:
        return False
    repository = r"[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*"
    selector = re.fullmatch(r"github:(" + repository + r")@([0-9a-f]{40})", item)
    source, uid, clients = node.get("source"), node.get("installation_id"), node.get("clients")
    if (selector is None or type(identity) is not str or not identity or node.get("name") != identity
            or type(source) is not str or re.fullmatch(repository, source) is None
            or source.lower() != selector[1].lower() or type(uid) is not str
            or re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", uid) is None
            or type(clients) is not list or len(clients) != 1):
        return False
    client = clients[0]
    if not prepared_package_binding(client, "kiro") or client.get("authentication") != "not_checked":
        return False
    revision = client.get("package_revision")
    return (type(revision) is dict and revision.get("resolved_revision") == selector[2]
            and all(type(revision.get(key)) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", revision[key])
                    for key in ("tree_digest", "manifest_digest")))


def matching_installations(response, identity, target, kind, item, signed_release_pins=None, *, kiro_prepared=False):
    """Verify the frozen Registry release, never only live self-consistency."""
    signed_release_pins = signed_release_pins or {}
    if not isinstance(response, dict):
        return []
    try:
        installations = response["data"]["installations"]
        if not isinstance(installations, list):
            return []
    except (KeyError, TypeError):
        return []
    expected_source = (item if kind == "plugin" else
                       f'{item["repo"]}/skills/{item["folder"]}')
    result = []
    for node in installations:
        if not isinstance(node, dict):
            continue
        if kiro_prepared and kiro_direct_prepared_installation(node, identity, target, kind, item):
            result.append(node)
            continue
        clients = node.get("clients", [])
        if not isinstance(clients, list):
            continue
        verified_clients = [
            client for client in clients
            if isinstance(client, dict) and client.get("client_id") == target and
            (client.get("verification") == "installation_verified" or
             (kind == "plugin" and client.get("materialization") == "materialized" and
              client.get("verification") == "package_validated" and
              (client.get("activation") == "manual_activation_required" or
               (isinstance(item, str) and item in signed_release_pins
                and prepared_package_binding(client, target)))))
        ]
        if not verified_clients:
            continue
        source = node.get("source", "")
        if kind == "skill":
            accepted = source == expected_source
        elif item in ("777genius/chrome-devtools-bridge", "777genius/github-bridge"):
            accepted = node.get("name") == identity and isinstance(source, str)
        elif isinstance(item, str) and item.startswith("github:"):
            expected_repo = item.split("@", 1)[0][len("github:"):].casefold()
            recorded_repo = (re.sub(r"^(?:github:|https://github\.com/|git@github\.com:)",
                                    "", source, flags=re.IGNORECASE)
                             .split("@", 1)[0].removesuffix(".git").rstrip("/")
                             if isinstance(source, str) else "")
            accepted = node.get("name") == identity and recorded_repo.casefold() == expected_repo
        else:
            accepted = node.get("name") == identity
        if accepted and node.get("installation_id"):
            # An exact-SHA Git selector must resolve to that SHA, not merely to
            # a repository with a matching display name. Validate the native
            # binding's recorded immutable commit, not CLI stdout prose.
            if kind == "plugin" and isinstance(item, str) and item.startswith("github:"):
                commit = item.rsplit("@", 1)[-1]
                if not any(
                    isinstance(client.get("package_revision"), dict)
                    and client["package_revision"].get("resolved_revision") == commit
                    for client in verified_clients
                ):
                    continue
            # A catalogue name is not enough to prove the selected signed
            # distribution: tie reviewed aliases to the signed directory
            # product and ensure immutable source revision/digests exist.
            if kind == "plugin" and isinstance(item, str) and not item.startswith("github:"):
                pin = signed_release_pins.get(item)
                directory = node.get("directory", {})
                # The published signed snapshot must provide an immutable SHA.
                # Even a self-consistent native receipt cannot replace that pin.
                if not isinstance(pin, dict) or not isinstance(pin.get("revision"), str):
                    continue
                # CLI `list.source` serializes the canonical repository/subpath;
                # the signed Directory distribution is an independent authority.
                # Never compare those two different source-identifying fields.
                expected_public_source = (
                    pin["repository"].strip("/") + "/" + pin["path"].strip("/"))
                if (source != expected_public_source or
                        not isinstance(directory, dict) or
                        directory.get("recorded_distribution") != pin["distribution"] or
                        directory.get("recorded_revision") != pin["revision"] or
                        directory.get("recorded_release_sequence") != pin["sequence"] or
                        (item != "777genius/chrome-devtools-bridge" and
                         directory.get("product_id") !=
                         ("github" if item == "777genius/github-bridge" else item))):
                    continue
                if not any(
                        recorded_signed_release_matches(
                            directory, client.get("package_revision"))
                        and client["package_revision"].get("tree_digest") == pin["tree_digest"]
                        and client["package_revision"].get("manifest_digest") == pin["manifest_digest"]
                        for client in verified_clients):
                    continue
            result.append(node)
    return result



def _unique_json_object(pairs):
    """Reject ambiguous duplicate keys in local audit receipts."""
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON field")
        result[key] = value
    return result


def _invalid_json_constant(_value):
    raise ValueError("non-finite JSON constant")


def cursor_preparation_source_matches(response, identity, kind, item, signed_release_pins):
    """Reuse immutable source matching for every Cursor Marketing dependency."""
    if kind != "plugin" or item != "marketing-kit":
        return matching_installations(response, identity, "cursor", kind, item, signed_release_pins)
    pins = dict(signed_release_pins or {})
    verified = []
    for alias, distribution in MARKETING_KIT_DISTRIBUTIONS.items():
        release = MARKETING_KIT_SIGNED_CLOSURE[distribution]
        pins[alias] = {"repository": release["repository"], "path": release["package_path"],
                       "distribution": distribution, "revision": release["resolved_revision"],
                       "sequence": release["release_sequence"], "tree_digest": release["tree_digest"],
                       "manifest_digest": release["manifest_digest"]}
        matches = matching_installations(response, alias, "cursor", "plugin", alias, pins)
        if len(matches) != 1:
            return []
        selected = [client for client in matches[0].get("clients", [])
                    if type(client) is dict and client.get("client_id") == "cursor"]
        if (len(selected) != 1 or type(selected[0].get("package_revision")) is not dict
                or type(selected[0]["package_revision"].get("release_sequence")) is not int):
            return []
        verified.extend(matches)
    return verified


def grok_prepared_skill_installations(list_json, item, skill_path, project, add_result, *, repeated=False,
                                     full_receipt=Ellipsis):
    """Qualify exact Grok project Skill preparation without claiming native activation."""
    if (not isinstance(list_json, dict) or not isinstance(item, dict)
            or not isinstance(skill_path, str) or not isinstance(add_result, dict)):
        return []
    data = list_json.get("data")
    entries = data.get("installations") if isinstance(data, dict) else None
    if (not isinstance(entries, list) or len(entries) != 1
            or not isinstance(entries[0], dict)):
        return []
    node = entries[0]
    parts = Path(skill_path).parts
    if (len(parts) != 6 or parts[:2] != (".grok", "plugins")
            or parts[3:] != ("skills", item.get("name"), "SKILL.md")
            or re.fullmatch(r"skill-[a-f0-9]{45}-[a-f0-9]{12}", parts[2]) is None):
        return []
    root, plugins = project / ".grok", project / ".grok" / "plugins"
    bundle = plugins / parts[2]
    skill = bundle / "skills" / item["name"] / "SKILL.md"
    if (any(p.is_symlink() for p in (project, root, plugins, bundle,
            bundle / "skills", skill.parent, skill))
            or not plugins.is_dir() or not bundle.is_dir() or not skill.is_file()
            or not bundle.resolve().is_relative_to(project.resolve())):
        return []
    try:
        receipt = (json.loads(add_result.get("stdout_tail", ""), object_pairs_hook=_unique_json_object)
                   if full_receipt is Ellipsis else full_receipt)
        if (not isinstance(receipt, dict) or receipt.get("status") != "succeeded"
                or not isinstance(receipt.get("items"), list) or len(receipt["items"]) != 1):
            return []
        selected = receipt["items"][0]
        if (not isinstance(selected, dict)
                or selected.get("alias") != item["name"]
                or selected.get("targetId") != "grok"
                or selected.get("status") != ("unchanged" if repeated else "succeeded")
                or selected.get("destinationPath") != str(bundle)
                or not isinstance(selected.get("candidateId"), str)
                or re.fullmatch(r"sha256:[0-9a-f]{64}", selected["candidateId"]) is None):
            return []
    except (TypeError, ValueError, KeyError, IndexError):
        return []
    name = node.get("name")
    if (node.get("source") != f'{item["repo"]}/skills/{item["folder"]}'
            or not isinstance(name, str) or re.fullmatch(r"skill-[a-f0-9]{56}", name) is None
            or not parts[2].startswith(name[:51] + "-")):
        return []
    uid = node.get("installation_id")
    if (not isinstance(uid, str)
            or re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", uid) is None):
        return []
    clients = node.get("clients")
    if (not isinstance(clients, list) or len(clients) != 1
            or not isinstance(clients[0], dict)):
        return []
    client = clients[0]
    if any(client.get(k) != v for k, v in (
            ("client_id", "grok"), ("scope", "project"),
            ("materialization", "materialized"), ("verification", "package_validated"),
            ("activation", "prepared"), ("policy", "allowed"))):
        return []
    revision = client.get("package_revision")
    if not isinstance(revision, dict) or revision.get("resolved_revision") != item.get("ref"):
        return []
    lock = project / "uap.lock"
    if lock.is_symlink() or not lock.is_file():
        return []
    try:
        packages = json.loads(lock.read_text(encoding="utf-8"),
                              object_pairs_hook=_unique_json_object)["packages"]
        matches = [p for p in packages if isinstance(p, dict) and p.get("alias") == item["name"]]
    except (OSError, TypeError, KeyError, ValueError):
        return []
    if len(matches) != 1:
        return []
    for key in ("tree_digest", "manifest_digest"):
        value = revision.get(key)
        if (not isinstance(value, str) or re.fullmatch(r"sha256:[a-f0-9]{64}", value) is None
                or matches[0].get(key) != value):
            return []
    return [node]


def grok_prepared_source_survives(response, item, prior_name, prior_uid):
    """Fail closed on retained or reminted Grok source after removal."""
    data = response.get("data") if isinstance(response, dict) else None
    nodes = data.get("installations") if isinstance(data, dict) else None
    if not isinstance(nodes, list) or not all(isinstance(n, dict) for n in nodes):
        return True
    source = f'{item["repo"]}/skills/{item["folder"]}'
    return any(n.get("source") == source or n.get("installation_id") == prior_uid
               or n.get("name") == prior_name
               or (isinstance(n.get("name"), str) and n["name"].startswith(prior_name[:51]))
               for n in nodes)

def codex_marketplace_source_in_case(source, case_root):
    """Prove physical containment, including Windows extended-length path aliases.

    Compare real directory identity; never permit a lexical prefix or an
    unresolved source to authorize native plugin removal.
    """
    if not source.is_absolute():
        return False
    try:
        root = case_root.resolve(strict=True)
        selected = source.resolve(strict=True)
        if selected.is_relative_to(root):
            return True
        # Go/Codex may write \\?\-prefixed Windows paths into TOML while
        # Python's temporary root is spelled as a normal drive path.
        return any(os.path.samefile(parent, root)
                   for parent in (selected, *selected.parents))
    except (OSError, ValueError, RuntimeError):
        return False


def uninstall_native_codex_plugin(identity, home, case_root, env, cwd):
    """Perform the documented *actual* Codex uninstall before attestation.

    This never touches an ambient account: the config and managed marketplace
    must both be contained within this case's disposable profile.
    """
    codex_bin = AGENT_BINARIES.get("codex")
    if not codex_bin or not Path(codex_bin).is_absolute() or not Path(codex_bin).is_file():
        return {"completed": False, "reason": "pinned Codex executable unavailable"}
    config = home / ".codex/config.toml"
    if not config.is_file() or config.is_symlink():
        return {"completed": False, "reason": "managed Codex native config absent"}
    try:
        state = tomllib.loads(config.read_text(encoding="utf-8"))
        plugins = state.get("plugins", {})
        candidates = [name for name in plugins
                      if isinstance(name, str) and name.startswith(identity + "@")]
        if len(candidates) != 1:
            return {"completed": False, "reason": "cannot identify one managed plugin"}
        spec = candidates[0]
        marketplace = spec.split("@", 1)[1]
        entry = state.get("marketplaces", {}).get(marketplace)
        if not isinstance(entry, dict) or entry.get("source_type", "local") != "local":
            return {"completed": False, "reason": "managed local marketplace cannot be verified"}
        source = Path(entry.get("source", ""))
        if not codex_marketplace_source_in_case(source, case_root):
            return {"completed": False, "reason": "marketplace source outside disposable case"}
    except (OSError, TypeError, ValueError):
        return {"completed": False, "reason": "cannot parse owned Codex config"}
    isolated = dict(env, CODEX_HOME=str(home / ".codex"))
    remove_plugin = call([codex_bin, "plugin", "remove", spec, "--json"], isolated, cwd, 40)
    remove_plugin.pop("_json", None)
    remove_market = call([codex_bin, "plugin", "marketplace", "remove", marketplace, "--json"],
                         isolated, cwd, 40) if remove_plugin["exit_code"] == 0 else None
    if remove_market:
        remove_market.pop("_json", None)
    try:
        after = tomllib.loads(config.read_text(encoding="utf-8")) if config.exists() else {}
        no_plugin = spec not in after.get("plugins", {})
        no_market = marketplace not in after.get("marketplaces", {})
    except (OSError, ValueError, TypeError):
        no_plugin = no_market = False
    return {"completed": bool(remove_plugin["exit_code"] == 0 and no_plugin and no_market),
            "plugin_remove": remove_plugin, "marketplace_remove": remove_market,
            "native_plugin_absent": no_plugin, "native_marketplace_absent": no_market}


# These native-client overrides supersede HOME/XDG in real UAP client adapters.
# Strip the few additional explicit-config knobs before creating one case's
# exact roots. Never allow a caller's existing AI client profile into this E2E.
PROFILE_ENV_KEYS = {
    "CODEX_HOME", "CLAUDE_CONFIG_DIR", "CLAUDE_CODE_CONFIG_DIR",
    "GEMINI_CLI_HOME", "QODER_CONFIG_DIR", "PI_CODING_AGENT_DIR",
    "KIMI_CODE_HOME", "KIMI_CODE_CACHE_DIR", "GROK_HOME",
    "CLINE_DATA_DIR", "CLINE_MCP_SETTINGS_PATH",
    "OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_DATA_DIR",
    "CURSOR_USER_DATA_DIR",
}



# A third-party package may launch processes during a live installation.
# The audit requires public network access, not the runner's account tokens.
# An explicit allowlist is safer than blacklisting secret-name families.
INHERITED_RUNTIME_ENV_KEYS = frozenset({
    "PATH", "PATHEXT", "SystemRoot", "SYSTEMROOT", "WINDIR", "COMSPEC",
    "OS", "USER", "LOGNAME",
    "LANG", "LANGUAGE", "LC_ALL", "LC_CTYPE", "LC_MESSAGES", "TZ", "TERM",
    "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
    "http_proxy", "https_proxy", "all_proxy", "no_proxy",
    "SSL_CERT_FILE", "SSL_CERT_DIR", "CURL_CA_BUNDLE",
    "REQUESTS_CA_BUNDLE", "NODE_EXTRA_CA_CERTS", "GIT_SSL_CAINFO",
})

def isolated_client_env(home, case_root, data):
    """Every native client authority root must belong to this disposable case."""
    home, case_root, data = (p.resolve(strict=True) for p in (home, case_root, data))
    if not home.is_relative_to(case_root) or not data.is_relative_to(case_root):
        raise ValueError("audit profile paths are not inside the disposable case")
    # Native profile overrides, Git config injection and host SSH helpers must
    # not escape the case even if a maintainer runs this script locally.
    env = {key: value for key, value in os.environ.items()
           if key in INHERITED_RUNTIME_ENV_KEYS}
    # Public proxy routing is useful, but embedded proxy credentials must not
    # reach untrusted package install subprocesses.
    for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
                "http_proxy", "https_proxy", "all_proxy"):
        if key not in env or "@" not in env[key]:
            continue
        try:
            parsed = urlsplit(env[key])
            if "@" in parsed.netloc:
                env[key] = urlunsplit(parsed._replace(netloc=parsed.netloc.rsplit("@", 1)[1]))
            else:
                env.pop(key)
        except ValueError:
            env.pop(key)
    temp = case_root / "tmp"
    temp.mkdir(exist_ok=True)
    if not temp.resolve().is_relative_to(case_root):
        raise ValueError("child temp root would escape disposable case")
    roots = {
        "CODEX_HOME": home / ".codex",
        "CLAUDE_CONFIG_DIR": home / ".claude",
        "GEMINI_CLI_HOME": home,  # Gemini appends /.gemini itself
        "QODER_CONFIG_DIR": home / ".qoder",
        "PI_CODING_AGENT_DIR": home / ".pi/agent",
        "KIMI_CODE_HOME": home / ".kimi-code",
        "KIMI_CODE_CACHE_DIR": home / ".cache/kimi-code",
        # Grok uses disposable HOME/.grok by default. Setting GROK_HOME
        # makes a normal profile look custom and disqualifies project Skills.
        "CLINE_DATA_DIR": home / ".cline/data",
        "CLINE_MCP_SETTINGS_PATH": home / ".cline/data/settings/cline_mcp_settings.json",
    }
    if any(not path.resolve().is_relative_to(home) for path in roots.values()):
        raise ValueError("native profile would escape disposable home")
    env.update({
        "HOME": str(home), "USERPROFILE": str(home),
        "XDG_CONFIG_HOME": str(home / ".config"),
        "XDG_CACHE_HOME": str(home / ".cache"),
        "XDG_DATA_HOME": str(home / ".local/share"),
        "XDG_STATE_HOME": str(home / ".local/state"),
        "APPDATA": str(home / "AppData/Roaming"),
        "LOCALAPPDATA": str(home / "AppData/Local"),
        "AGENTPLUGINS_HOME": str(data),
        "TMPDIR": str(temp), "TEMP": str(temp), "TMP": str(temp),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": str(case_root / "empty.gitconfig"),
        "GIT_TERMINAL_PROMPT": "0", "GCM_INTERACTIVE": "never",
        "KIMI_CODE_NO_AUTO_UPDATE": "1",
        "CI": "true", "DO_NOT_TRACK": "1",
        **{key: str(path) for key, path in roots.items()},
    })
    return env


def preserved_regular_text(path, expected, case_root):
    """Require a real in-case foreign sentinel, not a symlinked impersonation."""
    if path is None:
        return True
    try:
        return (path_inside_case(path, case_root) and path.is_file()
                and path.read_text(encoding="utf-8") == expected)
    except (OSError, UnicodeError):
        return False



def copilot_settings_windows_name(value):
    """Normalize only Win32 drive/UNC prefixes, never resolve a candidate path."""
    import ntpath
    value = str(value).replace("/", "\\")
    if value.startswith("\\\\?\\UNC\\"):
        value = "\\\\" + value[8:]
    elif value.startswith("\\\\?\\"):
        value = value[4:]
    value = ntpath.normpath(value)
    # Preserve component case, Unicode and short-name aliases exactly. Only a
    # drive letter is normalized; do not invent filesystem equivalence keys.
    return value[0].upper() + value[1:] if len(value) >= 2 and value[1] == ":" else value


def copilot_settings_windows_descriptor(path, initial, expected):
    """Reuse the owned Win32 handle seam and verify its final name before bytes."""
    import ctypes
    from ctypes import wintypes
    import msvcrt
    from runpy import run_path
    helpers = run_path(str(ROOT / "scripts/native_client_project_artifacts.py"))
    descriptor = helpers["open_member"](path)
    try:
        if not helpers["same_opened"](initial, os.fstat(descriptor)):
            raise ValueError("Windows settings identity changed")
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        query = kernel.GetFinalPathNameByHandleW
        query.argtypes = [wintypes.HANDLE, wintypes.LPWSTR, wintypes.DWORD, wintypes.DWORD]
        query.restype = wintypes.DWORD
        buffer = ctypes.create_unicode_buffer(4101)
        size = query(msvcrt.get_osfhandle(descriptor), buffer, len(buffer), 0)
        if (size == 0 or size >= len(buffer)
                or copilot_settings_windows_name(buffer.value) != copilot_settings_windows_name(expected)):
            raise ValueError("Windows settings handle left canonical case path")
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def copilot_settings_read(path, case_root):
    """Read bounded owned bytes through descriptors, never a replaced symlink."""
    descriptors = []
    try:
        # Resolve only the trusted root, before inspecting candidate metadata.
        # Windows compares the actual held-handle name with this lexical path.
        relative = path.relative_to(case_root)
        canonical_root = case_root.resolve(strict=True)
        root_stat = canonical_root.lstat()
        if not stat.S_ISDIR(root_stat.st_mode):
            return None, "unsafe-path"
        expected = canonical_root / relative
        initial = path.lstat()
        if not stat.S_ISREG(initial.st_mode) or initial.st_size > 1 << 20:
            return None, "not-regular-or-too-large"
        if initial.st_nlink != 1 or not path_inside_case(path, case_root):
            return None, "unsafe-path"
        flags = (os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
                 | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_CLOEXEC", 0))
        # POSIX walks verified directory descriptors. A renamed ancestor cannot
        # redirect the final open, and no candidate component is resolved.
        if (os.open in os.supports_dir_fd and hasattr(os, "O_DIRECTORY")
                and hasattr(os, "O_NOFOLLOW")):
            parent = os.open(canonical_root, flags | os.O_DIRECTORY)
            descriptors.append(parent)
            opened_root = os.fstat(parent)
            if (not stat.S_ISDIR(root_stat.st_mode) or not stat.S_ISDIR(opened_root.st_mode)
                    or (root_stat.st_dev, root_stat.st_ino) != (opened_root.st_dev, opened_root.st_ino)):
                return None, "unsafe-path"
            for name in relative.parts[:-1]:
                parent = os.open(name, flags | os.O_DIRECTORY, dir_fd=parent)
                descriptors.append(parent)
            descriptor = os.open(path.name, flags, dir_fd=parent)
        elif os.name == "nt":
            descriptor = copilot_settings_windows_descriptor(path, initial, expected)
        else:
            return None, "unsafe-path"  # No pathname-only read fallback.
        descriptors.append(descriptor)
        opened = os.fstat(descriptor)
        current = path.lstat()
        current_root = canonical_root.lstat()
        identity = (initial.st_dev, initial.st_ino)
        if (not stat.S_ISREG(opened.st_mode) or not stat.S_ISREG(current.st_mode)
                or not stat.S_ISDIR(current_root.st_mode)
                or (root_stat.st_dev, root_stat.st_ino) != (current_root.st_dev, current_root.st_ino)
                or opened.st_nlink != 1 or current.st_nlink != 1
                or identity != (opened.st_dev, opened.st_ino)
                or identity != (current.st_dev, current.st_ino)
                or not path_inside_case(path, case_root)):
            return None, "unsafe-path"
        if opened.st_size > 1 << 20:
            return None, "not-regular-or-too-large"
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = stream.read((1 << 20) + 1)
        if len(body) > 1 << 20:
            return None, "not-regular-or-too-large"
        return body.decode("utf-8"), None
    except FileNotFoundError:
        return None, "absent"
    except (OSError, RuntimeError, UnicodeError, ValueError):
        return None, "unsafe-path"
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def kimi_log_fact(facts, reason, changed=None):
    """Fixed diagnostic enums/counts only; never change the proof decision."""
    if isinstance(facts, dict):
        facts["reason"] = reason
        if changed is not None:
            facts["changed_entries_capped"] = min(changed, 100000)
    return None


def kimi_workspace_lock_require_no_companion(path):
    """Check the final native lock entry without following even broken links."""
    try:
        path.with_name(path.name + ".lock").lstat()
    except FileNotFoundError:
        return
    raise ValueError("Kimi workspace coordinator remains locked")


def kimi_workspace_lock_capture(home, case_root, scope):
    """Bind the native empty coordination target to the held scoped snapshot."""
    relative = ".kimi-code/.projects-write-lock"
    path = home / relative
    metadata_key = "metadata:0:" + relative
    content_key = home.name + "/" + relative
    # proper-lockfile's live companion must not survive native completion.
    if "metadata:0:" + relative + ".lock" in scope:
        raise ValueError("Kimi workspace coordinator remains locked")
    try:
        initial = path.lstat()
    except FileNotFoundError:
        if metadata_key in scope or content_key in scope:
            raise ValueError("Kimi workspace coordinator disappeared")
        kimi_workspace_lock_require_no_companion(path)
        return None
    fields = ("st_dev", "st_mode", "st_nlink", "st_size",
              "st_uid", "st_gid", "st_ino", "st_mtime_ns")
    identity = tuple(getattr(initial, field) for field in fields)
    if (not stat.S_ISREG(initial.st_mode) or stat.S_IMODE(initial.st_mode) != 0o600
            or initial.st_nlink != 1 or initial.st_size != 0
            or initial.st_uid != home.lstat().st_uid):
        raise ValueError("Kimi workspace coordinator is not private and empty")
    body, error = copilot_settings_read(path, case_root)
    if (error is not None or body != ""
            or identity != tuple(getattr(path.lstat(), field) for field in fields)
            or scope.get(metadata_key) != (initial.st_mode, initial.st_dev,
                                           initial.st_ino, initial.st_mtime_ns)
            or scope.get(content_key) != hashlib.sha256(b"").hexdigest()):
        raise ValueError("Kimi workspace coordinator changed during capture")
    kimi_workspace_lock_require_no_companion(path)
    return identity


def kimi_repeat_log_capture(home, project, case_root, facts=None):
    """Private bounded bytes; only hashes leave the isolated audit case."""
    path = home / ".kimi-code/logs/kimi-code.log"
    try:
        parents = (case_root, home, home / ".kimi-code", path.parent)
        def parent_facts():
            values = [directory.lstat() for directory in parents]
            if any(not stat.S_ISDIR(value.st_mode) for value in values):
                raise ValueError("Kimi log parent is not ordinary")
            return [(value.st_dev, value.st_ino, value.st_mode) for value in values]
        before_parents, initial = parent_facts(), path.lstat()
        body, error = copilot_settings_read(path, case_root)
        final = path.lstat()
        fields = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_size", "st_mtime_ns")
        identity = tuple(getattr(initial, key) for key in fields)
        if error is not None:
            reasons = {"absent": "log-absent", "unsafe-path": "log-read-unsafe",
                       "not-regular-or-too-large": "log-read-type-or-bound"}
            return kimi_log_fact(facts, reasons.get(error, "log-read-refused") if isinstance(error, str) else "log-read-refused")
        if before_parents != parent_facts():
            return kimi_log_fact(facts, "log-parent-changed")
        if identity != tuple(getattr(final, key) for key in fields):
            return kimi_log_fact(facts, "log-identity-changed")
        scope = scoped_snapshot((home, project / ".kimi-code", project / ".uap/managed"))
        log_key = home.name + "/.kimi-code/logs/kimi-code.log"
        body = body.encode("utf-8")
        if scope.get(log_key) != hashlib.sha256(body).hexdigest():
            return kimi_log_fact(facts, "log-hash-mismatch")
        workspace_lock = kimi_workspace_lock_capture(home, case_root, scope)
        if identity != tuple(getattr(path.lstat(), key) for key in fields):
            return kimi_log_fact(facts, "log-identity-after-scope-changed")
        if before_parents != parent_facts():
            return kimi_log_fact(facts, "log-parent-after-scope-changed")
        # Derive the native comparison from the same captured bytes. A separate
        # earlier/later snapshot must match before this can explain its delta.
        prefix = home.name + "/"
        native = {key[len(prefix):]: value for key, value in scope.items()
                  if any(key.startswith(prefix + root + "/") for root in NATIVE_ROOTS["kimi"])}
        kimi_log_fact(facts, "captured")
        if isinstance(facts, dict):
            facts["log_bytes"] = len(body)
        return {"body": body, "identity": identity, "native": native,
                "parents": before_parents, "scope": scope, "log_key": log_key,
                "workspace_lock": workspace_lock}
    except OSError:
        return kimi_log_fact(facts, "filesystem-read-refused")
    except ValueError:
        return kimi_log_fact(facts, "parent-or-path-refused")
    except AssertionError:
        return kimi_log_fact(facts, "scope-unreadable")


def kimi_scope_change_facts(before, after, home_prefix):
    """Describe held snapshot differences without exposing keys or values."""
    changed = sorted(key for key in before.keys() | after.keys()
                     if key not in before or key not in after or before[key] != after[key])
    records = []
    scopes = {"0": "home", "1": "project-native", "2": "managed"}
    for key in changed[:8]:
        parts = key.split(":", 2)
        kind = parts[0] if parts[0] in ("metadata", "directory", "link", "root-kind", "root-link") else "file-hash"
        scope = scopes.get(parts[1], "unknown") if kind != "file-hash" and len(parts) > 1 else "unknown"
        if kind == "file-hash":
            scope = {home_prefix: "home", ".kimi-code": "project-native", "managed": "managed"}.get(key.split("/", 1)[0], "unknown")
        record = {"key_sha256": hashlib.sha256(key.encode("utf-8", errors="surrogatepass")).hexdigest(),
                  "scope": scope, "key_kind": kind,
                  "change_kind": "added" if key not in before else "removed" if key not in after else "changed"}
        if kind == "metadata" and key in before and key in after:
            left, right = before[key], after[key]
            if isinstance(left, tuple) and isinstance(right, tuple) and len(left) == len(right) and len(left) in (3, 4):
                record["metadata_fields"] = [name for index, name in enumerate(("mode", "device", "inode", "mtime_ns")[:len(left)])
                                             if left[index] != right[index]]
            else:
                record["metadata_fields"] = ["shape"]
        records.append(record)
    return {"scope_changes": records, "scope_changes_omitted": min(max(0, len(changed) - 8), 100000)}


def kimi_repeat_log_append(before, after, listed, added, repeated, command, native_before, native_after, facts=None):
    """Admit one same-inode append, not rotation or other native mutations."""
    for phase, value in (("before", before), ("after-add", after), ("post-list", listed)):
        if value is None:
            return kimi_log_fact(facts, "capture-unavailable-" + phase)
    if after != listed:
        return kimi_log_fact(facts, "post-list-changed")
    if (type(command.get("exit_code")) is not int or command["exit_code"] != 0
            or command.get("timeout") is not False):
        return kimi_log_fact(facts, "repeat-command-refused")
    if before["native"] != native_before:
        return kimi_log_fact(facts, "native-before-mismatch")
    if listed["native"] != native_after:
        return kimi_log_fact(facts, "native-after-mismatch")
    for index, response in enumerate((added, repeated)):
        items = response.get("items") if isinstance(response, dict) else None
        if (not isinstance(response, dict) or response.get("schema_version") != 1
                or type(response.get("schema_version")) is not int
                or response.get("command") != "skills add" or response.get("result") != "success"
                or response.get("status") != "succeeded" or response.get("scope") != "project"
                or response.get("readiness") != "ready" or response.get("next_actions") not in (None, [])
                or not isinstance(items, list) or len(items) != 1 or not isinstance(items[0], dict)
                or items[0].get("targetId") != "kimi"
                or items[0].get("status") not in (("succeeded", "unchanged") if index else ("succeeded",))):
            return kimi_log_fact(facts, "fresh-public-refused" if index == 0 else "repeat-public-refused")
    if before["identity"][:4] != after["identity"][:4]:
        return kimi_log_fact(facts, "log-identity-changed")
    if before["parents"] != after["parents"]:
        return kimi_log_fact(facts, "log-parent-changed")
    if before["log_key"] != after["log_key"]:
        return kimi_log_fact(facts, "log-key-changed")
    if not after["body"].startswith(before["body"]):
        return kimi_log_fact(facts, "log-prefix-changed")
    if len(after["body"]) > 1 << 20:
        return kimi_log_fact(facts, "log-over-one-mib")
    if len(after["body"]) <= len(before["body"]):
        return kimi_log_fact(facts, "log-not-append")
    ignored = {before["log_key"], "metadata:0:.kimi-code/logs/kimi-code.log"}
    left, right = before["workspace_lock"], after["workspace_lock"]
    if ((left is None) != (right is None)
            or left is not None and left[:6] != right[:6]):
        return kimi_log_fact(facts, "workspace-lock-authority-changed")
    if left is not None:
        # Native 2.5.2 atomically rewrites this empty target before taking its lock.
        ignored.add("metadata:0:.kimi-code/.projects-write-lock")
    original = {key: value for key, value in before["scope"].items() if key not in ignored}
    current = {key: value for key, value in after["scope"].items() if key not in ignored}
    if original != current:
        changed = sum(original.get(key) != current.get(key) for key in original.keys() | current.keys())
        if isinstance(facts, dict):
            facts.update(kimi_scope_change_facts(original, current, before["log_key"].split("/", 1)[0]))
        return kimi_log_fact(facts, "protected-scope-changed", changed)
    kimi_log_fact(facts, "verified-append")
    return {"status": "verified-owned-log-append", "path": ".kimi-code/logs/kimi-code.log",
            "before_bytes": len(before["body"]), "after_bytes": len(after["body"]),
            "before_sha256": hashlib.sha256(before["body"]).hexdigest(),
            "after_sha256": hashlib.sha256(after["body"]).hexdigest(),
            "append_sha256": hashlib.sha256(after["body"][len(before["body"]):]).hexdigest(),
            "same_inode_mode_parents": True, "post_list_unchanged": True,
            **({"workspace_lock": "verified-empty-native-coordinator"} if left is not None else {})}


def copilot_settings_sanitized_shape(home, case_root, identity):
    """Observe *only* token shape, never export raw client settings or secrets."""
    path = home / ".copilot/settings.json"
    if not path_inside_case(path, case_root) or path.is_symlink():
        return {"status": "unsafe-path"}
    if not path.exists():
        return {"status": "absent"}
    try:
        text, status = copilot_settings_read(path, case_root)
        if status is not None:
            return {"status": status}
        doc = json.loads(text)
    except (OSError, UnicodeError, ValueError):
        return {"status": "unreadable-or-invalid-json"}
    if not isinstance(doc, (dict, list)):
        return {"status": "unexpected-json-root"}
    names = ("exact_key", "qualified_key", "reference_key",
             "exact_value", "qualified_value", "reference_value")
    counts = dict.fromkeys(names, 0)
    qualified = re.compile(re.escape(identity) + r"@agentplugins-[0-9a-f]{12}", re.I)
    allowed_sections = {"plugins", "installedPlugins", "enabledPlugins",
                        "marketplaces", "pluginMarketplaces"}
    sections = sorted(set(doc) & allowed_sections) if isinstance(doc, dict) else []
    # Native enablement evidence is separate from a mere matching key. Export
    # aggregate booleans only, never a key, provider value or raw settings JSON.
    enabled_values = {"true": 0, "false": 0, "non_boolean": 0}
    enabled = doc.get("enabledPlugins") if isinstance(doc, dict) else None
    if isinstance(enabled, dict):
        for key, value in enabled.items():
            if isinstance(key, str) and qualified.fullmatch(key):
                bucket = ("true" if value is True else
                          "false" if value is False else "non_boolean")
                enabled_values[bucket] += 1
    def match(value, role):
        if value.casefold() == identity.casefold():
            counts["exact_" + role] += 1
        elif qualified.fullmatch(value):
            counts["qualified_" + role] += 1
        elif identity.casefold() in value.casefold():
            counts["reference_" + role] += 1
    stack = [(doc, 0)]
    nodes = 0
    depth_truncated = False
    while stack and nodes < 10000:
        value, depth = stack.pop()
        nodes += 1
        if depth >= 32:
            depth_truncated = True
            continue
        if isinstance(value, dict):
            for key, child in value.items():
                match(key, "key")
                stack.append((child, depth + 1))
        elif isinstance(value, list):
            stack.extend((child, depth + 1) for child in value)
        elif isinstance(value, str):
            match(value, "value")
    return {"status": "truncated" if stack or depth_truncated else "parsed",
            "allowed_sections": sections, "match_counts": counts,
            "enabled_plugin_value_counts": enabled_values}


def initialize_project_skill_git_root(project, env, case_root):
    """Create an isolated real Git worktree for native project Skill discovery."""
    if (not GIT_EXECUTABLE or not Path(GIT_EXECUTABLE).is_file()
            or not project.is_dir() or project.is_symlink()
            or not project.resolve().is_relative_to(case_root.resolve())):
        raise RuntimeError("genuine isolated project Git prerequisite unavailable")
    # Git init follows pre-existing .git symlinks and writes to their targets.
    # Refuse an existing marker before invoking any external Git process.
    marker = project / ".git"
    if marker.exists() or marker.is_symlink():
        raise RuntimeError("disposable project Git metadata must be initially absent")
    # Git for Windows has no POSIX /dev/null. Disable hooks with an empty,
    # case-owned directory on every OS, never through host Git config.
    disabled_hooks = case_root / "empty-git-hooks"
    if disabled_hooks.is_symlink():
        raise RuntimeError("disposable Git hooks directory is redirected")
    disabled_hooks.mkdir(exist_ok=True)
    if (not disabled_hooks.is_dir() or disabled_hooks.is_symlink()
            or not disabled_hooks.resolve().is_relative_to(case_root.resolve())):
        raise RuntimeError("disposable Git hooks directory escaped its case")
    result = call([GIT_EXECUTABLE, "-c", f"core.hooksPath={disabled_hooks.resolve()}",
                   "init", "--quiet"], env, project, 30)
    if (result["exit_code"] != 0 or not marker.is_dir() or marker.is_symlink()
            or not marker.resolve().is_relative_to(project.resolve())):
        raise RuntimeError("genuine isolated project Git init failed")


def audit_client_id(target, kind):
    """Public Desktop arguments persist as the canonical local Codex client."""
    return "codex" if (target, kind) == ("chatgpt", "plugin") else target


def do_case(cli, target, kind, item, index, runroot, plugin_declared_names,
            signed_release_pins=None, *, client_version=None, historical_baseline=False,
            historical_plugins=False, installation_audit=None, installation_fixtures=None):
    with ExitStack() as audits:
        return _do_case(cli, target, kind, item, index, runroot, plugin_declared_names,
                        signed_release_pins, audits, client_version,
                        **({"historical_baseline": True} if historical_baseline else {}),
                        **({"historical_plugins": True} if historical_plugins else {}),
                        **({"installation_audit": installation_audit,
                            "installation_fixtures": installation_fixtures} if installation_audit else {}))


def _do_case(cli, target, kind, item, index, runroot, plugin_declared_names,
             signed_release_pins, audits, client_version, *, historical_baseline=False,
             historical_plugins=False, installation_audit=None, installation_fixtures=None):
    if historical_plugins and ((target, kind) != ("codex", "plugin") or historical_baseline):
        raise ValueError("historical archived feed requires its distinct Codex plugin lane")
    installation_routes = dict(PREPARED_INSTALLATION["ROUTES"])
    installation_routes.update(PREPARED_INSTALLATION["KIRO_ROUTES"])
    installation_routes[WINDSURF_INSTALLATION["ACCEPTANCE"]] = ("windsurf", "plugin")
    if installation_audit and installation_routes.get(installation_audit) != (target, kind):
        raise ValueError("installation audit does not match selected client and scope")
    if installation_audit == WINDSURF_INSTALLATION["ACCEPTANCE"] and type(installation_fixtures) is not dict:
        raise ValueError("Windsurf installation requires the validated complete fixture set")
    kiro_installation_case = installation_audit in PREPARED_INSTALLATION["KIRO_ACCEPTANCES"]
    if kiro_installation_case and type(installation_fixtures) is not dict:
        raise ValueError("Kiro installation requires the validated complete fixture set")
    native_target = audit_client_id(target, kind)
    case_root = runroot / f"{kind}-{index:02d}"
    home = case_root / "home"
    project = case_root / "project"
    data = case_root / "uap-data"
    # A previous case can plant a symlink at the *case root* itself. Resolving
    # that link and using its destination as the trust anchor is not containment.
    if runroot.is_symlink() or runroot.parent.is_symlink() or not runroot.parent.is_dir():
        raise ValueError("audit run root is redirected or unavailable")
    runroot.mkdir(exist_ok=True)
    if runroot.is_symlink() or not runroot.is_dir():
        raise ValueError("audit run root is redirected")
    canonical_runroot = runroot.resolve(strict=True)
    if case_root.is_symlink():
        raise ValueError("audit case root is redirected")
    case_root.mkdir(exist_ok=True)
    if (runroot.is_symlink() or case_root.is_symlink() or not case_root.is_dir()
            or case_root.resolve(strict=True).parent != canonical_runroot):
        raise ValueError("audit case root escaped disposable run root")
    canonical_case = case_root.resolve(strict=True)
    for path in (home, project, data, home / ".config", home / ".cache", home / ".local/share"):
        # Never create a descendant through an already redirected parent.
        if (path.is_symlink() or
                not path.parent.resolve().is_relative_to(canonical_case)):
            raise ValueError("nested audit directory escapes disposable case")
        # Directory caches and state-v2 protect the owned data root as 0700.
        # Seed that mode before the Kiro/Windsurf immutable baseline; retain
        # strict detection of any later mode, owner or inode change.
        strict_data_root = kiro_installation_case or installation_audit == WINDSURF_INSTALLATION["ACCEPTANCE"]
        mode = 0o700 if strict_data_root and path == data else 0o777
        path.mkdir(parents=True, exist_ok=True, mode=mode)
        if path.is_symlink() or not path.resolve().is_relative_to(canonical_case):
            raise ValueError("nested audit directory escapes disposable case")
    native_roots = [home / v for v in NATIVE_ROOTS[native_target]]
    if any(not root.resolve().is_relative_to(case_root.resolve()) for root in native_roots):
        raise ValueError("native root escapes disposable case")
    sentinel = project / "foreign-untouched.txt"
    sentinel.write_text("foreign content never belongs to UAP\n")
    user_sentinel = home / "foreign-untouched.txt"
    user_sentinel.write_text("foreign profile content never belongs to UAP\n")
    # A foreign file lives adjacent to installed native user artifacts.
    # Its content and position are independent from UAP-managed receipts.
    native_foreign = (native_roots[0] / "foreign-neighbor.txt") if native_roots else None
    if native_foreign:
        native_foreign.parent.mkdir(parents=True, exist_ok=True)
        native_foreign.write_text("unmanaged native neighbor must survive\n")
    # A pre-existing, unrelated project Skill is an independently owned neighbor.
    project_skill_roots = project_skill_scan_roots(target)
    project_foreign = None
    if kind == "skill" and project_skill_roots:
        # Qoder's UAP-managed bundle is not an unmanaged native Skill location.
        # Grok's plugins/ children require complete plugin manifests. A
        # foreign standalone Skill belongs to its separate native skills/ root.
        foreign_root = (".qoder/skills" if target == "qoder" else
                        ".grok/skills" if target == "grok" else
                        project_skill_roots[0])
        # Pi trusts only UAP-owned executable Skills in the selected project.
        # Keep the independent foreign Skill in B, alongside A's ordinary
        # foreign-file sentinel, without bypassing Pi's project trust refusal.
        foreign_project = case_root / "project-b" if target == "pi" else project
        project_foreign = foreign_project / foreign_root / "foreign-neighbor" / "SKILL.md"
    if project_foreign:
        if not path_inside_case(project_foreign, case_root):
            raise ValueError("foreign Skill neighbor escapes disposable case")
        project_foreign.parent.mkdir(parents=True, exist_ok=True)
        if target == "pi":
            # Recheck after creating parents; never overwrite a prior foreign
            # file or a hardlink when seeding the independent Pi project.
            if not path_inside_case(project_foreign, case_root):
                raise ValueError("Pi foreign Skill neighbor was redirected before fixture write")
            with project_foreign.open("x", encoding="utf-8") as stream:
                stream.write("---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
        else:
            project_foreign.write_text("---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
    pi_foreign_project = None
    if target == "pi" and kind == "skill":
        foreign_project = case_root / "project-b"
        if (not foreign_project.is_dir() or foreign_project.is_symlink()
                or not path_inside_case(foreign_project, case_root)):
            raise ValueError("Pi foreign project is not an ordinary contained directory")
        pi_foreign_project = scoped_snapshot((foreign_project,))
    # An installed Skill must leave the complete project Skill tree unchanged
    # after remove, not just its old SKILL.md pathname.
    before_skill_tree = (snapshot([project / name for name in project_skill_roots])
                         if kind == "skill" else {})
    # Presence of an actual binary comes from the workflow; a config directory is
    # only an empty isolated profile and is never counted as agent detection.
    if native_target in ("codex", "claude", "opencode", "gemini", "copilot", "pi"):
        native_roots[0].mkdir(parents=True, exist_ok=True)
    env = isolated_client_env(home, case_root, data)
    kiro_installation = None
    if kiro_installation_case:
        kiro_installation = PREPARED_INSTALLATION["KiroInstallationAudit"](
            cli, env, project, case_root, globals(), kind)
        audits.callback(kiro_installation.close)
    windsurf_installation = None
    if installation_audit == WINDSURF_INSTALLATION["ACCEPTANCE"]:
        windsurf_installation = WINDSURF_INSTALLATION["WindsurfInstallationAudit"](
            cli, env, project, case_root, globals())
        native_roots = list(windsurf_installation.native_roots)
    codex_skill_case = target == "codex" and kind == "skill"
    codex_runtime = None
    gemini_runtime = None
    gemini_native_baseline = None
    gemini_settings = gemini_settings_identity = None
    gemini_settings_text = '{"context":{"includeDirectoryTree":false}}\n'
    gemini_skill_case = (target == "gemini" and kind == "skill"
                         and AGENT_BINARIES.get("gemini")
                         and GEMINI_RUNTIME_AUDIT["pinned_version"](client_version))
    codex_plugin_sqlite = None
    codex_system_metadata = None
    codex_system_skill_proof = None
    codex_add_ready = False
    codex_repeat_ready = False
    historical_observer = (HISTORICAL_CODEX["PublishedBaseline"](cli, env, project)
                           if historical_baseline or historical_plugins else None)
    historical_plugin_feed = None
    historical_add_verified = historical_repeat_add_verified = False
    historical_repeat_lifecycle_verified = False
    def assert_case_unchanged():
        # Native clients and acquired packages execute as subprocesses.
        # Re-anchor before and after each call; otherwise an ancestor symlink
        # can make os.walk/read_text silently inspect a foreign profile.
        try:
            if (runroot.is_symlink() or case_root.is_symlink()
                    or runroot.resolve(strict=True) != canonical_runroot
                    or case_root.resolve(strict=True) != canonical_case):
                raise ValueError("audit case root changed after external execution")
            for path in (home, project, data):
                if not path.is_dir() or not path_inside_case(path, case_root):
                    raise ValueError("audit case profile/project root was redirected")
            for path in (*native_roots,
                         *(project / rel for rel in project_skill_roots)):
                if not path_inside_case(path, case_root):
                    raise ValueError("audit case native/project ancestor was redirected")
            if (pi_foreign_project is not None
                    and scoped_snapshot((case_root / "project-b",)) != pi_foreign_project):
                raise ValueError("Pi foreign project changed after external execution")
            if gemini_settings is not None:
                if (GEMINI_RUNTIME_AUDIT["_FILES"]["identity"](gemini_settings.lstat())
                        != gemini_settings_identity
                        or copilot_settings_read(gemini_settings, case_root) != (gemini_settings_text, None)):
                    raise ValueError("Gemini audit context settings changed after external execution")
            if codex_runtime is not None:
                codex_runtime.guard()
            if codex_plugin_sqlite is not None:
                codex_plugin_sqlite.guard()
            if windsurf_installation is not None:
                windsurf_installation.guard()
            if kiro_installation is not None:
                kiro_installation.guard()
        except (OSError, RuntimeError) as error:
            raise ValueError("audit case root became unavailable") from error

    def audited_call(*args, **kwargs):
        assert_case_unchanged()
        requested_full = kwargs.get("capture_full", False)
        if installation_audit or historical_plugins:
            # Explicit installation qualification needs complete, bounded and
            # unambiguous public receipts; preserve the existing call deadline.
            kwargs["capture_full"] = True
        outcome = call(*args, **kwargs)
        assert_case_unchanged()
        if historical_observer is not None:
            historical_observer.after_command()
            assert_case_unchanged()
        if codex_plugin_sqlite is not None:
            codex_plugin_sqlite.checkpoint("after-public-command")
        if windsurf_installation is not None:
            windsurf_installation.observe(list(args[0]), outcome)
        if kiro_installation is not None:
            kiro_installation.observe(list(args[0]), outcome)
        if (installation_audit or historical_plugins) and not requested_full:
            outcome.pop("stdout_full", None)
            outcome.pop("stderr_full", None)
        return outcome

    if historical_plugins:
        # Observe the unchanged npm CLI's real default-feed refusal first.
        # The explicit archive is authenticated by that same pinned CLI before
        # the ordinary strict native lifecycle and its SQLite checkpoints run.
        historical_plugin_feed = HISTORICAL_CODEX["prepare_plugin_archive"](
            historical_observer, audited_call, item, (signed_release_pins or {})[item])

    if gemini_skill_case:
        # The pinned native list initializes a model context even without a model call.
        # Its optional tree includes transient UAP package snapshots; discovery is unchanged.
        assert_case_unchanged()
        gemini_settings = home / ".gemini/settings.json"
        with gemini_settings.open("xb") as stream:
            stream.write(gemini_settings_text.encode("utf-8"))
        gemini_settings_identity = GEMINI_RUNTIME_AUDIT["_FILES"]["identity"](gemini_settings.lstat())
        assert_case_unchanged()
    if kind == "skill" and target in ("kimi", "grok", "copilot"):
        # Native project discovery requires an actual Git worktree. Never
        # weaken the exact project identity or owned-plugin discovery gates.
        initialize_project_skill_git_root(project, env, case_root)
    if codex_skill_case and AGENT_BINARIES.get("codex"):
        codex_runtime = CODEX_RUNTIME_AUDIT["CodexRuntimeAudit"](
            AGENT_BINARIES["codex"], case_root, home, project, env, item["name"], project_foreign)
        assert_case_unchanged()
        _, _, codex_system_metadata = CODEX_RUNTIME_AUDIT["system_skill_initialization"](
            home / ".codex")
        _codex_system_skill_checkpoint(home, codex_system_metadata)
    if native_target == "codex" and kind == "plugin" and AGENT_BINARIES.get("codex"):
        # Pinned 0.153.4 retains a valid installation UUID and empty arg0 parents.
        # Author these fixture objects before protecting every byte and identity.
        assert_case_unchanged()
        profile = home / ".codex"
        for directory in (profile / "tmp", profile / "tmp/arg0"):
            directory.mkdir(mode=0o700)
        with (profile / "installation_id").open("xb") as stream:
            if os.name != "nt":
                os.fchmod(stream.fileno(), 0o644)
            stream.write(str(uuid.uuid4()).encode("ascii"))
        protected_home = scoped_snapshot((home,))
        protected_delivery = scoped_snapshot((project, data))
        codex_plugin_sqlite = CODEX_RUNTIME_AUDIT["CodexPluginSQLiteAudit"](
            AGENT_BINARIES["codex"], case_root, home, project, env)
        assert_case_unchanged()
        bundle_files, bundle_directories, bundle_metadata = (
            CODEX_RUNTIME_AUDIT["plugin_initialization"](home / ".codex"))
        system_root = home / ".codex/skills/.system"
        codex_system_metadata = {
            path: fact for path, fact in bundle_metadata.items()
            if path == system_root.parent or path.is_relative_to(system_root)}
        initialized_home = scoped_snapshot((home,))
        # Admit only exact artifacts validated by the bounded Codex initialization
        # proofs. Every other old or newly created key is protected.
        initializer = codex_plugin_sqlite
        sqlite_root = home / ".local/state/codex-sqlite"
        config_path = home / ".codex/config.toml"
        if initializer.runtime != sqlite_root or initializer.config != config_path:
            raise ValueError("Codex preinstall initialization changed protected fixture authority")
        owned_files = {
            config_path: hashlib.sha256(
                CODEX_RUNTIME_AUDIT["_sqlite_config"](initializer.pin)).hexdigest(),
            **{sqlite_root / name: fact["sha256"] for name, fact in
               initializer.observations[0]["runtime_files_fully_read"].items()},
            **bundle_files,
        }
        expected_home = dict(protected_home)
        owned_directories = (home / ".local", home / ".local/state", sqlite_root,
                             *bundle_directories)
        for path in (*owned_directories, *owned_files):
            directory = path in owned_directories
            relative = path.relative_to(home)
            metadata_key = "metadata:0:" + relative.as_posix()
            metadata = initialized_home.get(metadata_key)
            if (not isinstance(metadata, tuple) or not metadata
                    or not (stat.S_ISDIR(metadata[0]) if directory
                            else stat.S_ISREG(metadata[0]))
                    or (path in bundle_metadata and metadata != bundle_metadata[path])):
                raise ValueError("Codex preinstall initialization changed protected fixture authority")
            entries = {metadata_key: metadata}
            if directory:
                entries["directory:0:" + relative.as_posix()] = "present"
            else:
                entries[home.name + "/" + str(relative)] = owned_files[path]
            for key, value in entries.items():
                if key not in protected_home:
                    expected_home[key] = value
        initializer.guard()
        if (initialized_home != expected_home
                or scoped_snapshot((project, data)) != protected_delivery):
            raise ValueError("Codex preinstall initialization changed protected fixture authority")
    before = snapshot(native_roots)
    claude_before = (scoped_snapshot(native_roots)
                     if (target, kind, sys.platform) == ("claude", "skill", "linux") else None)
    if kind == "plugin":
        identity = plugin_declared_names.get(item, item)
        args = ["add", item, "--target", target, "--scope", "user", "--format", "json"]
        # VS Code's unchanged user adapter only accepts an imperative add.
        if target == "vscode":
            args.append("--no-save")
        scope = ["--scope", "user"]
    else:
        identity = item["name"]
        args = ["skills", "add", item["source"], "--skill", item["name"],
                "--target", target, "--scope", "project", "--project-root",
                str(project), "--no-ui", "--yes", "--format", "json"]
        scope = ["--scope", "project", "--project-root", str(project)]
    copilot_before = (copilot_settings_sanitized_shape(home, case_root, identity)
                      if kind == "plugin" and target == "copilot" else None)
    add_timeout = install_case_timeout(target, kind, item)
    # Keep the original strict add/list/repeat/remove evidence; only avoid
    # inherited Windows Firebase child pipe handles masking parent exit.
    file_capture = (sys.platform == "win32" and kind == "plugin" and item == "firebase")
    capture_option = {"capture_files": True} if file_capture else {}
    output = audited_call([cli, *args], env, project, add_timeout, **capture_option)
    added_json = output.pop("_json", None)
    native_components = (native_components_from_add(added_json)
                         if kind == "plugin" else ())
    prepared_components_only = (prepared_only_add(added_json)
                                if kind == "plugin" else False)
    copilot_after_add = (copilot_settings_sanitized_shape(home, case_root, identity)
                         if copilot_before is not None else None)
    changed = set(snapshot(native_roots).items()) - set(before.items())
    native_attribution = attributable_native_changes(
        native_roots, changed, identity, native_target, native_components)
    copilot_skill_case = target == "copilot" and kind == "skill"
    skill_path = (skill_file_present(project, target, identity)
                  if kind == "skill" and not copilot_skill_case else None)
    locked = (project / "uap.toml").is_file() and (project / "uap.lock").is_file()
    lock_contains = locked and identity in (project / "uap.lock").read_text(encoding="utf-8")
    lock_pinned = bool(kind == "skill" and locked and locked_skill_is_pinned(project, item, target))
    skill_bytes_match = (kind == "skill" and matches_pinned_skill_bytes(
        project, skill_path, item["skill_md_git_blob_sha1"]))
    listed = (audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                   env, project, 30))
    listed_json = listed.pop("_json", None) if listed else None
    matching = matching_installations(listed_json, identity, native_target, kind, item,
                                     signed_release_pins, **({"kiro_prepared": True} if kiro_installation_case else {}))
    if copilot_skill_case:
        skill_path = copilot_project_skill_path(project, item, added_json, matching)
        # The bounded receipt-bound read already verified the exact Git blob.
        skill_bytes_match = skill_path is not None
    kiro_direct_prepared = bool(kiro_installation_case and len(matching) == 1
        and kiro_direct_prepared_installation(matching[0], identity, native_target, kind, item))
    grok_prepared = (grok_prepared_skill_installations(
        listed_json, item, skill_path, project, output,
        **({"full_receipt": added_json} if installation_audit else {}))
        if target == "grok" and kind == "skill" and not matching
        and listed["exit_code"] == 0 else [])
    if grok_prepared:
        matching = grok_prepared
    signed_observation = (
        signed_release_public_observation(listed_json, identity, native_target)
        if kind == "plugin" and isinstance(item, str) and not item.startswith("github:")
        else [])
    listed_identity = len(matching) == 1
    failed_install_registered = has_target_registration(listed_json, identity, native_target) if (
        output["exit_code"] != 0) else False
    failed_install_residue = bool(output["exit_code"] != 0 and (
        failed_install_registered or native_attribution or skill_path))
    prepared_binding = bool(listed_identity and (
        grok_prepared or (kind == "plugin" and any(
            (client.get("client_id") == native_target and client.get("verification") == "package_validated"
             and client.get("activation") == "manual_activation_required")
            or prepared_package_binding(client, native_target)
            for client in matching[0].get("clients", []) if isinstance(client, dict)))))
    # Name and file presence alone are insufficient. A true Skill pass also
    # proves the immutable source in the portable lock AND exact upstream bytes.
    installed = bool(
        output["exit_code"] == 0 and listed["exit_code"] == 0 and listed_identity and
        (kind == "plugin" or
         (bool(skill_path) and lock_pinned and skill_bytes_match))
    )
    if gemini_skill_case and installed:
        gemini_baseline_before = snapshot(native_roots, include_directories=True)
        gemini_runtime = GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
            home, project, gemini_baseline_before, client_version,
            profile_snapshot=scoped_snapshot)
        audits.callback(gemini_runtime.close)
        if gemini_runtime.reason is None:
            # Match runGeminiInventory: locale/timezone and other inherited variables
            # are absent from its explicit child environment, including model context.
            native_env = {"GEMINI_CLI_HOME": env["GEMINI_CLI_HOME"]}
            native_env.update({key: env[key] for key in (
                "PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR",
                "GEMINI_RESTRICTED_MODE", "GEMINI_CLI_TRUST_WORKSPACE") if key in env})
            native_env.update(NO_COLOR="1", TERM="dumb")
            native_started = datetime.now(timezone.utc)
            native_observed = audited_call(
                [AGENT_BINARIES["gemini"], "skills", "list", "--all"], native_env, project, 30)
            native_finished = datetime.now(timezone.utc)
            native_observed.pop("_json", None)
            gemini_baseline_after = snapshot(native_roots, include_directories=True)
            gemini_native_baseline = {
                "operation": "native-skills-list-baseline",
                "started": native_started.isoformat(), "finished": native_finished.isoformat(),
                "observation": native_observed, "selected_skill_discovery_claimed": False,
                "native_changes": snapshot_change_evidence(
                    gemini_baseline_before, gemini_baseline_after),
            }
            if native_observed["exit_code"] == 0 and not native_observed["timeout"]:
                gemini_runtime.baseline(gemini_baseline_after,
                                        interval=(native_started, native_finished))
            else:
                gemini_runtime.reason = "Gemini native baseline command did not complete cleanly"
    if codex_runtime is not None and installed:
        codex_runtime.checkpoint("installed", True)
        codex_add_ready = CODEX_RUNTIME_AUDIT["public_skill_ready"](added_json, matching, identity)
        if historical_baseline:
            historical_add_verified = HISTORICAL_CODEX["historical_add"](
                added_json, matching, item, project / skill_path)
    # BUG-016: a single successful add/remove cannot attest idempotence.
    # Repeat the same source and target only after the first registered add;
    # keep native, managed and foreign state inside the same disposable case.
    repeat_add = None
    repeat_add_json = None
    repeat_matches = []
    repeat_list = None
    repeat_list_json = None
    repeat_identity_ids = []
    repeat_native_unchanged = None
    repeat_native_runtime_only = False
    repeat_native_changes = None
    repeat_native_phases = None
    repeat_lock_unchanged = None
    repeat_project_unchanged = None
    repeat_verified = False
    kimi_repeat_log_proof = None
    kimi_log_case = (target == "kimi" and kind == "skill" and sys.platform == "linux"
                     and isinstance(client_version, dict) and client_version.get("status") == "reported"
                     and client_version.get("value") == "2.5.2"
                     and type(client_version.get("exit_code")) is int and client_version["exit_code"] == 0)
    kimi_repeat_log_facts = ({"captures": {phase: {} for phase in ("before", "after-add", "post-list")},
                              "append": {"reason": "not-reached"}} if kimi_log_case else None)
    if installed:
        before_repeat_native = snapshot(native_roots, include_directories=gemini_runtime is not None)
        before_repeat_runtime = (codex_runtime.temporary_git_state()
                                 if codex_runtime is not None else None)
        repeat_project_snapshot = scoped_snapshot if copilot_skill_case else snapshot
        before_repeat_project = (repeat_project_snapshot([project / root for root in project_skill_roots])
                                 if grok_prepared or copilot_skill_case else None)
        lock_root = project if kind == "skill" else data
        lock_paths = (lock_root / "uap.toml", lock_root / "uap.lock")
        def lock_state():
            return tuple(
                (path.is_file() and not path.is_symlink(),
                 path.read_bytes() if path.is_file() and not path.is_symlink() else None)
                for path in lock_paths
            )
        before_repeat_lock = lock_state()
        kimi_log_before = (kimi_repeat_log_capture(home, project, case_root,
                            kimi_repeat_log_facts["captures"]["before"]) if kimi_log_case else None)
        repeat_add_started = datetime.now(timezone.utc)
        repeat_timeout = install_case_timeout(target, kind, item, repeat=True)
        repeat_add = audited_call([cli, *args], env, project, repeat_timeout,
                                  **capture_option)
        repeat_add_finished = datetime.now(timezone.utc)
        kimi_log_after = (kimi_repeat_log_capture(home, project, case_root,
                            kimi_repeat_log_facts["captures"]["after-add"]) if kimi_log_case else None)
        after_repeat_add = (snapshot(native_roots, include_directories=gemini_runtime is not None)
                            if codex_runtime is not None or gemini_runtime is not None else None)
        if gemini_runtime is not None:
            gemini_runtime.checkpoint("after-public-repeat-add", after_repeat_add,
                                      interval=(repeat_add_started, repeat_add_finished))
        repeat_add_json = repeat_add.pop("_json", None)
        repeat_list_started = datetime.now(timezone.utc)
        repeat_list = audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                           env, project, 30)
        repeat_list_finished = datetime.now(timezone.utc)
        after_repeat_list = (snapshot(native_roots, include_directories=gemini_runtime is not None)
                             if codex_runtime is not None or gemini_runtime is not None else None)
        if gemini_runtime is not None:
            gemini_runtime.checkpoint("after-public-repeat-list", after_repeat_list,
                                      interval=(repeat_list_started, repeat_list_finished))
        repeat_list_json = repeat_list.pop("_json", None)
        repeat_matches = (grok_prepared_skill_installations(
            repeat_list_json, item, skill_path, project, repeat_add, repeated=True,
            **({"full_receipt": repeat_add_json} if installation_audit else {}))
            if grok_prepared else matching_installations(
                repeat_list_json, identity, native_target, kind, item, signed_release_pins,
                **({"kiro_prepared": True} if kiro_installation_case else {})))
        if kiro_direct_prepared:
            repeat_matches = [node for node in repeat_matches
                if kiro_direct_prepared_installation(node, identity, native_target, kind, item)
                and node["clients"][0]["package_revision"] == matching[0]["clients"][0]["package_revision"]]
        copilot_repeat_path = (copilot_project_skill_path(
            project, item, repeat_add_json, repeat_matches, repeated=True)
            if copilot_skill_case else None)
        repeat_identity_ids = sorted(
            entry["installation_id"] for entry in repeat_matches)
        if codex_runtime is not None:
            codex_runtime.checkpoint("repeat-installed", True)
            codex_repeat_ready = CODEX_RUNTIME_AUDIT["public_skill_ready"](
                repeat_add_json, repeat_matches, identity)
        after_repeat_native = snapshot(native_roots, include_directories=gemini_runtime is not None)
        kimi_log_listed = (kimi_repeat_log_capture(home, project, case_root,
                            kimi_repeat_log_facts["captures"]["post-list"]) if kimi_log_case else None)
        gemini_repeat_valid = (gemini_runtime.finish(after_repeat_native)
                              if gemini_runtime is not None else False)
        if codex_runtime is not None or gemini_runtime is not None:
            repeat_native_phases = repeat_native_phase_evidence(
                before_repeat_native, after_repeat_add, after_repeat_list, after_repeat_native)
        repeat_native_unchanged = after_repeat_native == before_repeat_native
        if not repeat_native_unchanged:
            repeat_native_changes = snapshot_change_evidence(
                before_repeat_native, after_repeat_native)
            if codex_runtime is not None:
                repeat_native_runtime_only = codex_runtime.repeat_runtime_only(
                    before_repeat_native, after_repeat_native, before_repeat_runtime)
            elif gemini_runtime is not None:
                repeat_native_runtime_only = gemini_repeat_valid
            elif kimi_log_case:
                kimi_repeat_log_proof = kimi_repeat_log_append(
                    kimi_log_before, kimi_log_after, kimi_log_listed, added_json, repeat_add_json, repeat_add,
                    before_repeat_native, after_repeat_native, kimi_repeat_log_facts["append"])
                repeat_native_runtime_only = kimi_repeat_log_proof is not None
        repeat_project_unchanged = (repeat_project_snapshot([project / root for root in project_skill_roots])
                                    == before_repeat_project if grok_prepared or copilot_skill_case else None)
        repeat_lock_unchanged = lock_state() == before_repeat_lock
        historical_repeat_lifecycle_verified = bool(
            repeat_add["exit_code"] == 0 and repeat_list["exit_code"] == 0
            and len(repeat_matches) == 1
            and repeat_identity_ids == sorted(entry["installation_id"] for entry in matching)
            and (repeat_native_unchanged or repeat_native_runtime_only) and repeat_lock_unchanged
            and (not (grok_prepared or copilot_skill_case) or repeat_project_unchanged)
            and (not copilot_skill_case or copilot_repeat_path == skill_path)
        )
        repeat_verified = bool(historical_repeat_lifecycle_verified
                               and (not codex_skill_case or codex_repeat_ready)
                               and (gemini_runtime is None or gemini_repeat_valid))
        if historical_baseline:
            historical_repeat_add_verified = HISTORICAL_CODEX["historical_add"](
                repeat_add_json, repeat_matches, item, project / skill_path, repeated=True)
    # BUG-039: independent native ID+version proof before UAP removes it.
    qoder_native_before_remove = None
    if target == "qoder" and kind == "plugin" and installed and repeat_verified:
        qoder_native_before_remove = qoder_native_plugin_state(
            home, env, project, identity, matching[0].get("version"))
        assert_case_unchanged()
    copilot_native_before_remove = None
    kit_owners = None
    if target == "copilot" and kind == "plugin" and installed and repeat_verified:
        if item == "marketing-kit":
            kit_owners = copilot_signed_kit_owners(listed_json, signed_release_pins or {},
                                                   data, case_root)
        if item == "marketing-kit" and (kit_owners is None
                or not copilot_signed_kit_settings(home, case_root, kit_owners, True)):
            copilot_native_before_remove = {"status": "unqualified-signed-closure"}
        else:
            copilot_native_before_remove = copilot_native_plugin_state(
                home, env, project, identity, matching[0].get("version"),
                owners=kit_owners, owned_call=audited_call)
        assert_case_unchanged()
    claude_retirement = None
    claude_retirement_proof = None
    if claude_before is not None and installed and repeat_verified:
        def claude_document(path):
            text, error = copilot_settings_read(path, case_root)
            if error is not None:
                raise ValueError("Claude retirement metadata unavailable")
            return json.loads(text, object_pairs_hook=_unique_json_object)
        try:
            claude_retirement = CLAUDE_RETIREMENT_AUDIT["CacheRetirement"](
                case_root, home, project, matching[0], repeat_add_json, item,
                claude_document, scoped_snapshot, claude_before,
                lambda observed: native_identity_residuals(
                    native_roots, identity, observed, native_target, native_components))
            audits.callback(claude_retirement.close)
        except (OSError, ValueError, TypeError, KeyError, IndexError):
            claude_retirement_proof = {"status": "unresolved-cache-retirement"}
    cleanup = None
    cleanup_json = None
    native_uninstall = None
    if installed:
        # Codex explicitly requires external uninstall before UAP can settle a
        # user-scope plugin removal. We execute and verify the real native
        # plugin + marketplace commands, never attest on a guessed state.
        if kind == "plugin" and native_target == "codex":
            native_uninstall = uninstall_native_codex_plugin(
                identity, home, case_root, env, project)
            assert_case_unchanged()
            if codex_plugin_sqlite is not None:
                codex_plugin_sqlite.checkpoint("after-native-uninstall")
        attestation = (["--external-uninstalled"] if
                       native_uninstall and native_uninstall["completed"] else [])
        # Grok project Skills have generated bundle names; never select by raw alias.
        selector = matching[0]["installation_id"] if grok_prepared or kiro_installation_case else identity
        cleanup = audited_call([cli, "remove", selector, "--target", target, *scope,
                        *attestation, "--format", "json"], env, project, 60)
        cleanup_json = cleanup.pop("_json", None)
    qoder_native_after_remove = None
    if target == "qoder" and kind == "plugin" and cleanup and cleanup["exit_code"] == 0:
        qoder_native_after_remove = qoder_native_plugin_state(
            home, env, project, identity)
        assert_case_unchanged()
    sentinels_preserved = (
        preserved_regular_text(sentinel, "foreign content never belongs to UAP\n", case_root)
        and preserved_regular_text(user_sentinel, "foreign profile content never belongs to UAP\n", case_root)
        and preserved_regular_text(native_foreign, "unmanaged native neighbor must survive\n", case_root)
        and preserved_regular_text(
            project_foreign, "---\nname: foreign-neighbor\ndescription: unrelated\n---\n", case_root)
    )
    post_list = (audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                      env, project, 30) if cleanup and cleanup["exit_code"] == 0 else None)
    post_list_json = post_list.pop("_json", None) if post_list else None
    if codex_runtime is not None and cleanup and cleanup["exit_code"] == 0:
        codex_runtime.checkpoint("removed", False)
        assert_case_unchanged()
    copilot_native_after_remove = None
    if target == "copilot" and kind == "plugin" and cleanup and cleanup["exit_code"] == 0:
        copilot_native_after_remove = copilot_native_plugin_state(
            home, env, project, identity, owners={} if item == "marketing-kit" else None,
            owned_call=audited_call)
        assert_case_unchanged()
    copilot_after_remove = (copilot_settings_sanitized_shape(home, case_root, identity)
                            if copilot_before is not None and cleanup else None)
    post_skill_tree_delta = []
    if kind == "skill" and cleanup:
        after_skill_tree = snapshot([project / name for name in project_skill_roots])
        post_skill_tree_delta = sorted(
            path for path in before_skill_tree.keys() | after_skill_tree.keys()
            if before_skill_tree.get(path) != after_skill_tree.get(path))
    post_removal_skill = skill_file_present(project, target, identity) if kind == "skill" and cleanup else None
    post_removal_leftovers = (
        skill_removal_leftovers(project, target, identity, skill_path)
        if kind == "skill" and cleanup else [])
    if kind == "skill" and cleanup:
        sentinels_preserved = sentinels_preserved and (
            preserved_regular_text(sentinel, "foreign content never belongs to UAP\n", case_root)
            and preserved_regular_text(user_sentinel, "foreign profile content never belongs to UAP\n", case_root)
            and preserved_regular_text(native_foreign, "unmanaged native neighbor must survive\n", case_root)
            and preserved_regular_text(
                project_foreign, "---\nname: foreign-neighbor\ndescription: unrelated\n---\n", case_root)
        )
    remaining_ids = set()
    post_list_schema_valid = False
    post_list_empty = False
    if isinstance(post_list_json, dict):
        try:
            entries = post_list_json["data"]["installations"]
            if isinstance(entries, list) and all(isinstance(v, dict) for v in entries):
                remaining_ids = {entry.get("installation_id") for entry in entries}
                post_list_schema_valid = True
                post_list_empty = not entries
        except (KeyError, TypeError):
            pass
    if copilot_skill_case:
        post_list_schema_valid = (post_list_schema_valid
                                  and copilot_skill_success_response(post_list_json, "list"))
    old_ids = {entry["installation_id"] for entry in matching}
    # A successful remove must not re-register this identity under a new ID.
    post_remove_identity_registered = bool(
        post_list_schema_valid and (
            has_target_registration(
                post_list_json, identity, native_target,
                skill_source=f'{item["repo"]}/skills/{item["folder"]}'
                if kind == "skill" and target == "claude" else None)
            or (grok_prepared and grok_prepared_source_survives(
                post_list_json, item, matching[0]["name"], matching[0]["installation_id"]))))
    post_lock_pinned = bool(kind == "skill" and (project / "uap.lock").exists()
                            and locked_skill_is_pinned(project, item, target))
    unsafe_paths = [path for path, digest in changed
                    if digest in ("symlink", "unreadable", "non-regular")]
    # The remove step may create new native registrations after the add snapshot.
    # Rescan the disposable roots; preserve earlier observed unsafe paths too.
    codex_system_files = None
    if codex_system_metadata is not None and cleanup and cleanup["exit_code"] == 0:
        assert_case_unchanged()
        codex_system_files = _codex_system_skill_checkpoint(home, codex_system_metadata)
    opencode_bootstrap_proof = []
    native_residuals = (
        native_identity_residuals(
            native_roots, identity,
            sorted(set(snapshot(native_roots)) | set(native_attribution) | set(unsafe_paths)),
            native_target, native_components, client_version=client_version,
            bootstrap_proof=opencode_bootstrap_proof)
        if cleanup and cleanup["exit_code"] == 0 else []
    )
    codex_system_residuals_verified = False
    if codex_system_files is not None:
        if _codex_system_skill_checkpoint(home, codex_system_metadata) != codex_system_files:
            raise ValueError("Codex pinned system Skill inventory changed during residual scan")
        assert_case_unchanged()
        codex_system_residuals_verified = bool(
            native_residuals and all(path in codex_system_files for path in native_residuals)
            and (codex_skill_case or (native_uninstall
                 and native_uninstall.get("completed") is True
                 and native_uninstall.get("native_plugin_absent") is True
                 and native_uninstall.get("native_marketplace_absent") is True)))
        if codex_system_residuals_verified:
            codex_system_skill_proof = {
                "status": "verified-unchanged-pinned-system-files-at-checkpoint",
                "native_version": "0.153.4",
                "residuals": {path: codex_system_files[path] for path in native_residuals},
                "atomic_snapshot_claimed": False, "writer_closure_claimed": False}
    # Preserve truthful disk-state evidence without equating passive cached
    # source/log files to an active Qoder/Codex plugin registration.
    passive_retained = []
    qoder_passive_log_link_verified = None
    if target == "qoder" and kind == "plugin" and cleanup:
        qoder_passive_log_link_verified = qoder_passive_latest_log_link(
            native_roots[0], native_roots[0] / "logs/latest")
    if cleanup and cleanup["exit_code"] == 0:
        for root in native_roots:
            prefix = root.name + "/"
            for observed in native_attribution:
                if not observed.startswith(prefix):
                    continue
                relative = observed[len(prefix):]
                path = root / relative
                if passive_native_path(native_target, relative) and path.is_file() and not path.is_symlink():
                    passive_retained.append(observed)
    copilot_disabled_setting_retained = bool(
        target == "copilot" and kind == "plugin"
        and copilot_disabled_preference_only(
            copilot_before, copilot_after_add, copilot_after_remove,
            copilot_native_before_remove, copilot_native_after_remove,
            native_residuals)
        and (item != "marketing-kit" or copilot_signed_kit_settings(
            home, case_root, kit_owners, False)))
    claude_retired_residuals_verified = False
    if (claude_retirement is not None and cleanup and cleanup["exit_code"] == 0
            and post_list and post_list["exit_code"] == 0 and post_list_schema_valid
            and old_ids.isdisjoint(remaining_ids) and not post_remove_identity_registered
            and post_removal_skill is None and not post_removal_leftovers
            and not post_lock_pinned and not post_skill_tree_delta and sentinels_preserved):
        claude_retirement_proof = claude_retirement.finish(native_residuals, cleanup_json)
        claude_retired_residuals_verified = (
            claude_retirement_proof["status"] == "verified-cache-retirement-at-checkpoint")
    removal_verified = bool(
        cleanup and cleanup["exit_code"] == 0 and post_list and post_list["exit_code"] == 0
        and post_list_schema_valid and old_ids.isdisjoint(remaining_ids)
        and (not copilot_skill_case or (post_list_empty
             and copilot_skill_success_response(cleanup_json, "remove")))
        # This fresh case owns the whole signed kit closure. An original or
        # reminted child record must not survive even when native state is empty.
        and (kind != "plugin" or item != "marketing-kit" or post_list_empty)
        and (not kiro_direct_prepared or post_list_empty)
        and (target != "qoder" or kind != "plugin"
             or (qoder_native_after_remove is not None
                 and qoder_native_after_remove["status"] == "absent"))
        and (target != "copilot" or kind != "plugin" or item != "marketing-kit"
             or (copilot_native_after_remove is not None
                 and copilot_native_after_remove["status"] == "absent"))
        and not post_remove_identity_registered
        and (not native_residuals or copilot_disabled_setting_retained
             or claude_retired_residuals_verified or codex_system_residuals_verified)
        and (kind != "skill" or (post_removal_skill is None
                                  and not post_removal_leftovers and not post_lock_pinned
                                  and not post_skill_tree_delta))
    )
    diagnostics = (output["stdout_tail"] + "\n" + output["stderr_tail"]).lower()
    install_refused = is_cli_refusal(output)
    # Still remove an installed-but-unready Skill, but never call its lifecycle
    # PASS. The public ready states and fresh native inventory are both needed.
    installed = installed and (not codex_skill_case or codex_add_ready)
    if installed and removal_verified and sentinels_preserved and not repeat_verified:
        status = "repeat-add-unqualified"
    elif installed and removal_verified and sentinels_preserved and repeat_verified:
        status = ("prepared-lifecycle-passed" if (
                       kind == "plugin" and prepared_components_only)
                  else "package-installed-native-unqualified" if (
                       kind == "plugin" and (
                           (target == "qoder" and
                            (qoder_native_before_remove is None or
                             qoder_native_before_remove["status"] != "registered"))
                           or (target == "copilot" and
                               (copilot_native_before_remove is None or
                                copilot_native_before_remove["status"] != "installed"))
                           or (native_target == "codex" and not (
                               native_uninstall and native_uninstall.get("completed")))
                           or (native_target not in ("codex", "qoder", "copilot")
                               and not native_attribution)))
                  else "prepared-lifecycle-passed" if prepared_binding else "passed")
    elif installed and cleanup and (
            "external removal requires explicit attestation" in
            (cleanup.get("stderr_tail", "") + cleanup.get("stdout_tail", "")).lower()
            or (target == "vscode" and kind == "plugin" and prepared_binding
                and repeat_verified and cleanup.get("exit_code") not in (None, 0)
                and "removal requires a manual client step for [vscode]" in
                cleanup.get("stderr_tail", "").lower()
                and "--external-uninstalled" in
                cleanup.get("stderr_tail", "").lower())):
        # A prepared VS Code package is not an activated native plugin.
        # Do not counterfeit the external-uninstalled attestation.
        status = "external-native-uninstall-required"
    elif installed:
        status = "cleanup-failed"
    elif failed_install_residue:
        status = "failed-install-residue"
    elif install_refused and host_containment_refusal(diagnostics):
        status = "host-containment-unavailable"
    elif install_refused and ("not detected" in diagnostics or "not available" in diagnostics
                             or "target_unavailable" in diagnostics
                             or (target == "windsurf" and kind == "plugin"
                                 and "clientconfigroot must be an explicit absolute clean path"
                                 in diagnostics)):
        status = "target-unavailable"
    elif install_refused and ("unsupported" in diagnostics or "not supported" in diagnostics
                              or source_target_incompatibility(diagnostics, target)):
        status = "documented-or-source-unsupported"
    elif install_refused and "minimum installer version" in diagnostics:
        status = "test-build-version-policy"
    elif install_refused and "distribution is suspended" in diagnostics:
        status = "expected-suspended-policy"
    elif (install_refused and target == "chatgpt" and kind == "plugin"
          and listed["exit_code"] == 0 and not failed_install_registered
          and not changed and sentinels_preserved
          and "group apply preflight failed; no target was changed" in output["stderr_tail"].lower()
          and "native identity ownership is indeterminate; refusing mutation"
          in output["stderr_tail"].lower()):
        # The native ownership preflight refused before mutating the disposable
        # profile. Never promote this or a source-unsupported case to native PASS.
        status = "native-ownership-unverified"
    elif output["exit_code"] == 0:
        status = "incomplete-native-proof"
    else:
        status = "install-failed"
    # A clean file lifecycle cannot override the installer's public setup
    # result. This is a necessary condition only; every native gate above
    # remains required, including Codex's independent inventory checkpoints.
    skill_public_ready = ({phase: bool(
        isinstance(response, dict) and response.get("readiness") == "ready"
        and response.get("next_actions") in (None, []))
        for phase, response in (("after-add", added_json), ("repeat-add", repeat_add_json))}
        if kind == "skill" else None)
    if copilot_skill_case:
        for phase, response, matches in (("after-add", listed_json, matching),
                                          ("repeat-add", repeat_list_json, repeat_matches)):
            skill_public_ready[phase] = (skill_public_ready[phase]
                                         and copilot_skill_list_ready(response, matches))
    if status == "passed" and skill_public_ready is not None and not all(skill_public_ready.values()):
        status = "incomplete-native-proof"
    # Pi component support and owned files do not provide independent
    # before/add/repeat/remove native inventories in this catalog lane.
    if status == "passed" and (native_target, kind) == ("pi", "plugin"):
        status = "package-installed-native-unqualified"
    historical_archive_status = status
    if historical_plugins and status == "passed":
        status = "historical-archive-lifecycle-observed"
    if kiro_installation_case and status == "passed":
        status = "installation-lifecycle-observed"
    row = {"kind": kind, "index": index, "identity": identity, "target": target,
            **({target + "_preparation_envelopes": PREPARED_INSTALLATION["observe"](
                kind, identity, {"install": added_json, "list": listed_json,
                                 "repeat_add": repeat_add_json, "repeat_list": repeat_list_json,
                                 "cleanup": cleanup_json, "post_remove_list": post_list_json},
                matching if target == "grok" else
                    cursor_preparation_source_matches(listed_json, identity, kind, item, signed_release_pins),
                repeat_matches if target == "grok" else
                    cursor_preparation_source_matches(repeat_list_json, identity, kind, item, signed_release_pins),
                target=target, **({"skill_fixture": item} if target == "grok" else {}))}
               if installation_audit in PREPARED_INSTALLATION["ACCEPTANCES"] else {}),
            **({"canonical_client_id": native_target} if target == "chatgpt" else {}),
            "source": item if kind == "plugin" else item["source"],
            "status": status, "install": output, "list": listed,
            "native_file_delta": len(changed),
            "native_identity_files": native_attribution,
            "native_component_candidates": list(native_components),
            "prepared_components_only": prepared_components_only,
            "skill_path": skill_path,
            "lock_written": bool(locked), "lock_contains_identity": bool(lock_contains),
            "lock_exact_source_pinned": lock_pinned, "skill_bytes_match": bool(skill_bytes_match),
            "listed_identity": listed_identity, "prepared_binding": prepared_binding,
            "signed_release_observation": signed_observation,
            "public_target_states": {
                "after-add": public_target_state_observation(
                    listed_json, identity, native_target,
                    installation_ids=old_ids if kind == "skill" else None,
                    skill_source=f'{item["repo"]}/skills/{item["folder"]}' if kind == "skill" else None),
                "repeat-add": public_target_state_observation(
                    repeat_list_json, identity, native_target,
                    installation_ids=repeat_identity_ids if kind == "skill" else None,
                    skill_source=f'{item["repo"]}/skills/{item["folder"]}' if kind == "skill" else None),
                "after-remove": public_target_state_observation(
                    post_list_json, identity, native_target,
                    installation_ids=old_ids | set(repeat_identity_ids) if kind == "skill" else None,
                    skill_source=f'{item["repo"]}/skills/{item["folder"]}' if kind == "skill" else None)},
            "public_skill_ready": skill_public_ready,
            "repeat_add": repeat_add, "repeat_list": repeat_list,
            "repeat_installation_ids": repeat_identity_ids,
            "repeat_native_unchanged": repeat_native_unchanged,
            "repeat_native_runtime_only": repeat_native_runtime_only,
            "kimi_repeat_log_proof": kimi_repeat_log_proof,
            "kimi_repeat_log_facts": kimi_repeat_log_facts,
            "repeat_native_changes": repeat_native_changes,
            "repeat_native_phases": repeat_native_phases,
            "repeat_lock_unchanged": repeat_lock_unchanged,
            "repeat_project_unchanged": repeat_project_unchanged,
            "repeat_verified": repeat_verified,
            "failed_install_registered": failed_install_registered,
            "failed_install_residue": failed_install_residue,
            "matched_installation_ids": sorted(old_ids),
            "native_uninstall": native_uninstall, "cleanup": cleanup, "post_remove_list": post_list,
            "copilot_settings_proof": ({"before": copilot_before,
                                        "after_add": copilot_after_add,
                                        "after_remove": copilot_after_remove}
                                       if copilot_before is not None else None),
            "post_removal_skill": post_removal_skill,
            "post_removal_leftovers": post_removal_leftovers,
            "post_skill_tree_delta": post_skill_tree_delta,
            "post_lock_pinned": post_lock_pinned,
            "removal_verified": removal_verified, "native_residuals": native_residuals,
            **({"claude_cache_retirement": claude_retirement_proof}
               if claude_retirement_proof is not None else {}),
            **({"codex_system_skill_retention": codex_system_skill_proof}
               if codex_system_skill_proof is not None else {}),
            **({"opencode_bootstrap_retention": opencode_bootstrap_proof}
               if opencode_bootstrap_proof else {}),
            "retained_passive_native_files": sorted(set(passive_retained)),
            "qoder_passive_log_link_verified": qoder_passive_log_link_verified,
            "qoder_native_after_repeat": qoder_native_before_remove,
            "qoder_native_after_remove": qoder_native_after_remove,
            "copilot_native_after_repeat": copilot_native_before_remove,
            "copilot_native_after_remove": copilot_native_after_remove,
            **({"historical_npm_proof": historical_observer.proof}
               if historical_observer is not None else {}),
            **({"historical_add_verified": historical_add_verified,
                "historical_repeat_add_verified": historical_repeat_add_verified,
                "historical_repeat_lifecycle_verified": historical_repeat_lifecycle_verified}
               if historical_baseline else {}),
            **({"historical_plugin_feed": historical_plugin_feed,
                "historical_archive_status": historical_archive_status}
               if historical_plugins else {}),
            "codex_public_add_ready": codex_add_ready if codex_skill_case else None,
            "codex_public_repeat_ready": codex_repeat_ready if codex_skill_case else None,
            "codex_runtime_proof": codex_runtime.evidence() if codex_runtime is not None else None,
            "gemini_runtime_proof": gemini_runtime.evidence() if gemini_runtime is not None else None,
            "gemini_native_baseline": gemini_native_baseline,
            **({"codex_plugin_sqlite_proof": codex_plugin_sqlite.evidence()}
               if codex_plugin_sqlite is not None else {}),
            "copilot_disabled_setting_retained": copilot_disabled_setting_retained,
            "post_list_schema_valid": post_list_schema_valid,
            "post_remove_identity_registered": post_remove_identity_registered,
            "sentinels_preserved": sentinels_preserved}
    if windsurf_installation is not None:
        row["windsurf_installation_audit"] = windsurf_installation.finish(
            row, item, index, installation_fixtures)
    if kiro_installation is not None:
        row["kiro_installation_custody"] = kiro_installation.finish(row, item, index, installation_fixtures)
    return row


def expected_negative_coverage(row, cli_version=None):
    """Accept only verified *non-native* outcomes, never relabel them PASS.

    The published release has frozen unsupported target combinations, and
    OpenCode can complete a prepared package lifecycle without proving native
    activation. Fail closed on side effects, missing cleanup, or new sources.
    """
    if not row.get("sentinels_preserved") or row.get("failed_install_residue"):
        return False
    target = row.get("target")
    if row.get("kind") != "plugin":
        return False

    # A completed OpenCode package lifecycle can be non-native even when
    # prepared Skills are materialized under the isolated native profile.
    # The entire identity must be removed; a surviving file is never accepted.
    native_files = row.get("native_identity_files")
    if (target == "opencode"
            and isinstance(native_files, list)
            and all(isinstance(path, str) and path.startswith("opencode/")
                    and ".." not in Path(path).parts for path in native_files)
            and row.get("native_component_candidates") == []
            and row.get("listed_identity") is True
            and row.get("repeat_verified") is True
            and row.get("removal_verified") is True
            and row.get("post_remove_identity_registered") is False
            and row.get("native_residuals") == []
            and isinstance(row.get("install"), dict)
            and row["install"].get("exit_code") == 0
            and isinstance(row.get("cleanup"), dict)
            and row["cleanup"].get("exit_code") == 0):
        if (row.get("status") == "prepared-lifecycle-passed"
                and row.get("prepared_components_only") is True):
            return True
        # Only these immutable sources in the verified 0.1.95 / 0.1.101 / 0.1.102
        # catalog have a clean lifecycle but no attributable native component.
        # An unknown or newly unqualified source must still fail this audit.
        non_native_direct_sources = {
            "github:itential/builder-skills@2ccdf77358b018ad4df125422a067fe84e63de1d",
            "github:GoogleCloudPlatform/google-cloud-storage-plugin@b346e39698fcdfdc711ca3ee35e41c8dbaa1aa2b",
            "github:cometchat/cometchat-skills@8652f1ea41d12e5058123db1e4e6babef9c918f1",
            "github:uizze/uizze@fc401e9e654cbafe427b0c98142fc8bccd0c4e7e",
        }
        if (row.get("status") == "package-installed-native-unqualified"
                and cli_version in ("agentplugins 0.1.95", "agentplugins 0.1.101", "agentplugins 0.1.102",
                                    "agentplugins 0.1.115")
                and row.get("prepared_components_only") is False
                and native_files == []
                and row.get("source") in non_native_direct_sources):
            return True

    if (row.get("status") != "documented-or-source-unsupported"
            or row.get("native_file_delta") != 0
            or row.get("native_identity_files")
            or row.get("listed_identity")
            or row.get("failed_install_registered")
            or not isinstance(row.get("install"), dict)
            or type(row["install"].get("exit_code")) is not int
            or row["install"]["exit_code"] == 0):
        return False
    diagnostic = row["install"].get("stderr_tail", "")
    if not isinstance(diagnostic, str):
        return False
    if (row.get("source") == "marketing-kit"
            and target in ("cline", "copilot", "kimi", "qoder", "grok")
            and "release does not support complete target set" in diagnostic
            and ("missing " + target) in diagnostic):
        return True
    # Pi's generic plugin route cannot install these frozen sources when the
    # signed release explicitly lacks pi or its own manifest prevents native
    # extension discovery. Refusals are negative, never a native installation.
    return bool(target == "pi"
                and (("release does not support complete target set" in diagnostic
                      and "missing pi" in diagnostic)
                     or "pi package manifest overrides selected extension discovery"
                        in diagnostic))


def unexecuted_source_rows(selected, kind, target, status, declared_names):
    """Record exact fixture slots on a refused bootstrap, never native PASS."""
    if status not in ("client-unavailable", "documented-unsupported"):
        raise ValueError("only non-execution status permitted")
    return [{"target": target, "kind": kind, "index": index,
             "source": item if kind == "plugin" else item["source"],
             "identity": declared_names.get(item, item) if kind == "plugin" else item["name"],
             "status": status}
            for index, item in enumerate(selected)]


def vendor_bootstrap_attested():
    """A failed vendor bootstrap cannot qualify an unrelated runner binary."""
    return os.environ.get("AUDIT_VENDOR_OUTCOME") == "success"


def published_audit_temp_parent():
    """Keep GitHub Windows deep fixtures inside the real short runner scratch.

    The published 0.1.101 CLI has a separately documented long-path bug.
    This qualifies an ordinary bounded Windows project, not MAX_PATH support.
    """
    if sys.platform != "win32" or os.environ.get("GITHUB_ACTIONS") != "true":
        return None
    candidate = os.environ.get("RUNNER_TEMP", "")
    parent = Path(candidate)
    if not candidate or not parent.is_absolute() or parent.is_symlink() or not parent.is_dir():
        raise RuntimeError("Windows GitHub runner scratch is unavailable or redirected")
    return str(parent)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--cli", required=True)
    p.add_argument("--kind", choices=("plugin", "skill"), required=True)
    p.add_argument("--target", required=True)
    p.add_argument("--limit", type=int, default=30)
    p.add_argument("--report-dir", required=True)
    p.add_argument("--acceptance", choices=("native", HISTORICAL_CODEX["ACCEPTANCE"],
                                         HISTORICAL_CODEX["PLUGIN_ACCEPTANCE"],
                                         *PREPARED_INSTALLATION["ACCEPTANCES"],
                                         *PREPARED_INSTALLATION["KIRO_ACCEPTANCES"],
                                         WINDSURF_INSTALLATION["ACCEPTANCE"]),
                   default="native")
    args = p.parse_args()
    historical_baseline = args.acceptance == HISTORICAL_CODEX["ACCEPTANCE"]
    if historical_baseline and (args.target, args.kind, sys.platform, platform.machine(), args.limit) != (
            "codex", "skill", "linux", "x86_64", 2):
        p.error("Historical npm observation requires exactly two Codex Skills on Linux x86_64")
    historical_plugins = args.acceptance == HISTORICAL_CODEX["PLUGIN_ACCEPTANCE"]
    if historical_plugins and (args.target, args.kind, sys.platform, platform.machine(), args.limit) != (
            "codex", "plugin", "linux", "x86_64", 2):
        p.error("Historical archived feed requires exactly two Codex plugins on Linux x86_64")
    prepared_installation = args.acceptance in PREPARED_INSTALLATION["ACCEPTANCES"]
    kiro_installation = args.acceptance in PREPARED_INSTALLATION["KIRO_ACCEPTANCES"]
    if kiro_installation and (args.target, args.kind, sys.platform, platform.machine(), args.limit) != (
            *PREPARED_INSTALLATION["KIRO_ROUTES"][args.acceptance], "darwin", "arm64", 30):
        p.error("Kiro installation requires its complete pinned macOS arm64 lane")
    if prepared_installation and (args.target, args.kind, sys.platform, platform.machine(), args.limit) != (
            *PREPARED_INSTALLATION["ROUTES"][args.acceptance], "linux", "x86_64", 30):
        p.error("Preparation requires its exact pinned Linux x86_64 complete installation lane")
    windsurf_installation = args.acceptance == WINDSURF_INSTALLATION["ACCEPTANCE"]
    if windsurf_installation and (args.target, args.kind, sys.platform, platform.machine(), args.limit) != (
            "windsurf", "plugin", "linux", "x86_64", 30):
        p.error("Windsurf Global installation requires the complete pinned Linux x86_64 plugin lane")
    doc = json.loads(FIXTURES.read_text())
    validate_fixtures(doc)
    if args.target not in doc["targets"] or not 1 <= args.limit <= 30:
        p.error("unknown target or invalid limit")
    report_dir = Path(args.report_dir)
    report_dir.mkdir(parents=True, exist_ok=True)
    cases = doc["plugin_aliases"] if args.kind == "plugin" else doc["skills"]
    selected = cases[:args.limit]
    source_sha = os.environ.get("GITHUB_SHA", "")
    pr_head_sha = os.environ.get("AUDIT_PR_HEAD_SHA", "")
    workflow_run = os.environ.get("GITHUB_RUN_ID", "")
    report = {"checked_at": datetime.now(timezone.utc).isoformat(),
              "target": args.target, "kind": args.kind, "cli": args.cli,
              "github_sha": source_sha if re.fullmatch(r"[a-f0-9]{40}", source_sha) else None,
              "pr_head_sha": pr_head_sha if re.fullmatch(r"[a-f0-9]{40}", pr_head_sha) else None,
              "github_run_id": workflow_run if workflow_run.isdigit() else None,
              "fixture_sha256": hashlib.sha256(FIXTURES.read_bytes()).hexdigest(),
              "platform": sys.platform, "architecture": platform.machine(),
              "client_binary": None, "client_version": {"status": "not-probed"},
              "cli_version": {"status": "not-probed"},
              "fixture_count": len(cases), "cases_attempted": 0,
              "source_selection_complete": args.limit == 30,
              "full_coverage": False, "results": []}
    if historical_baseline:
        report["acceptance"] = args.acceptance
        report.update(HISTORICAL_CODEX["summary"](report, doc))
    if historical_plugins:
        report["acceptance"] = args.acceptance
        report.update(HISTORICAL_CODEX["plugin_summary"](report, doc))
    if prepared_installation:
        report["acceptance"] = args.acceptance
        report.update(PREPARED_INSTALLATION["summary"](report, doc, args.acceptance))
    if kiro_installation:
        report["acceptance"] = args.acceptance
        report.update(PREPARED_INSTALLATION["kiro_summary"](report, doc, args.acceptance))
    if windsurf_installation:
        report["acceptance"] = args.acceptance
        report.update(WINDSURF_INSTALLATION["summary"](report, doc, fixture_sha256=report["fixture_sha256"]))
    report_file = report_dir / f"{args.target}-{args.kind}.json"
    def save():
        report_file.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    native_target = audit_client_id(args.target, args.kind)
    signed_release_pins = effective_signed_release_pins(doc, native_target)
    if args.target == "chatgpt":
        report["canonical_client_id"] = native_target
    eligible_binaries = EXECUTABLE.get(native_target, ())
    if not vendor_bootstrap_attested():
        report["blocker"] = "Vendor bootstrap was not verified; no native-client PASS is permitted"
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "client-unavailable",
            doc["plugin_declared_names"])
        save()
        print(report["blocker"], file=sys.stderr)
        return 2
    if eligible_binaries and not any(AGENT_BINARIES.get(name) for name in eligible_binaries):
        report["blocker"] = (
            "Genuine client executable not installed; tried " + ", ".join(eligible_binaries)
        )
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "client-unavailable",
            doc["plugin_declared_names"])
        save()
        print(report["blocker"], file=sys.stderr)
        return 2
    if args.target == "chatgpt" and args.kind == "skill":
        report["blocker"] = "ChatGPT has no native project skill filesystem adapter"
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "documented-unsupported",
            doc["plugin_declared_names"])
        save()
        return 2
    cli = str(Path(args.cli).resolve(strict=True))
    # Real Windows runner scratch avoids accidental >260-character internal
    # preview paths for deep immutable Skills (tracked separately in PR #780).
    # This does not convert a refused installation into native PASS.
    temp_parent = published_audit_temp_parent()
    report["temp_parent_kind"] = ("github-windows-runner" if temp_parent else "python-default")
    # Resolve only the selected parent before creating any owned case; strict
    # profile/foreign-path checks still reject redirected existing directories.
    physical_parent = Path(temp_parent or tempfile.gettempdir()).resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="uap-live-30x30-", dir=physical_parent) as temp:
        runroot = Path(temp)
        probe_case = runroot / "version-probe"
        probe_home, probe_data = probe_case / "home", probe_case / "uap-data"
        probe_home.mkdir(parents=True)
        probe_data.mkdir()
        probe_env = isolated_client_env(probe_home, probe_case, probe_data)
        report["cli_version"] = version_evidence([cli, "version"], probe_env, probe_case)
        if eligible_binaries:
            native = next(AGENT_BINARIES[name] for name in eligible_binaries
                          if AGENT_BINARIES.get(name))
            report["client_binary"] = native
            report["client_version"] = version_evidence([native, "--version"], probe_env, probe_case)
        save()
        for i, item in enumerate(selected):
            try:
                result = do_case(cli, args.target, args.kind, item, i, runroot,
                                 doc["plugin_declared_names"],
                                 signed_release_pins,
                                 **({"client_version": report.get("client_version")}
                                    if (args.target, args.kind) in (("gemini", "skill"), ("kimi", "skill"))
                                    or args.target == "opencode" else {}),
                                 **({"historical_baseline": True} if historical_baseline else {}),
                                 **({"historical_plugins": True} if historical_plugins else {}),
                                 **({"installation_audit": args.acceptance} if prepared_installation else {}),
                                 **({"installation_audit": args.acceptance, "installation_fixtures": doc}
                                    if windsurf_installation or kiro_installation else {}))
            except Exception as e:
                result = {"kind": args.kind, "target": args.target, "index": i,
                          "source": item if args.kind == "plugin" else item["source"],
                          "identity": (doc["plugin_declared_names"].get(item, item)
                                       if args.kind == "plugin" else item["name"]),
                          "status": "harness-error", "error": repr(e),
                          "error_traceback": bounded_error_traceback(e)}
                if historical_plugins and isinstance(e, HISTORICAL_CODEX["ArchiveUnavailable"]):
                    result["historical_plugin_feed"] = e.observation
            report["results"].append(result)
            report["cases_attempted"] += 1
            save()
            line = {"target": args.target, "kind": args.kind, "index": i,
                    "status": result["status"]}
            if result["status"] != "passed":
                line.update(failed_case_diagnostics(result))
            if result["status"] == "incomplete-native-proof" and args.kind == "plugin":
                line["observed_signed_release"] = result.get("signed_release_observation", [])
                line["expected_signed_release"] = signed_release_pins.get(item)
            print(json.dumps(line), flush=True)
    passed = sum(r["status"] == "passed" for r in report["results"])
    report["passed"] = passed
    report["prepared_lifecycle_passed"] = sum(
        r["status"] == "prepared-lifecycle-passed" for r in report["results"])
    accepted_negatives = [
        r for r in report["results"]
        if expected_negative_coverage(r, report["cli_version"].get("value"))
    ]
    report["verified_negative_coverage"] = len(accepted_negatives)
    report["full_coverage"] = args.limit == 30 and report["cases_attempted"] == 30
    save()
    print(f'{args.target} {args.kind}: {passed}/{args.limit} native PASS; '
          f'verified_negative={len(accepted_negatives)}; attempted={report["cases_attempted"]}')
    if historical_baseline:
        report.update(HISTORICAL_CODEX["summary"](report, doc))
        save()
        print(f'Historical npm lifecycle observations: {report["historical_lifecycle_observed"]}/2; '
              'modern public readiness remains unavailable; native PASS count unchanged')
        return 0 if report["historical_observation_complete"] else 1
    if historical_plugins:
        report.update(HISTORICAL_CODEX["plugin_summary"](report, doc))
        save()
        print(f'Historical archived plugin lifecycles: {report["archived_plugin_lifecycles_observed"]}/2; '
              f'current default-feed refusals={report["current_default_refusals_observed"]}; '
              'current native PASS count remains zero')
        return 0 if report["historical_plugin_observation_complete"] else 1
    if prepared_installation:
        report.update(PREPARED_INSTALLATION["summary"](report, doc, args.acceptance))
        save()
        print(f'{args.target} prepared installation: {report["installation_passed"]}/30; '
              'native discovery, activation and authenticated execution remain unqualified')
        return 0 if report["installation_acceptance_passed"] else 1
    if kiro_installation:
        report.update(PREPARED_INSTALLATION["kiro_summary"](report, doc, args.acceptance))
        save()
        print(f'Kiro installation: {report["installation_passed"]}/30; '
              f'exact signed refusals={report["installation_verified_negative_coverage"]}; '
              'native discovery, activation, runtime, authentication and tools remain unqualified')
        return 0 if report["installation_acceptance_passed"] else 1
    if windsurf_installation:
        report.update(WINDSURF_INSTALLATION["summary"](report, doc, fixture_sha256=report["fixture_sha256"]))
        save()
        print(f'Windsurf Global installation: {report["installation_passed"]}/30 installed; '
              f'exact refusals={report["installation_verified_negative_coverage"]}; '
              'native runtime, authentication, model and MCP qualification remain false')
        return 0 if report["installation_acceptance_passed"] else 1
    return 0 if (report["cases_attempted"] == args.limit
                 and passed + len(accepted_negatives) == args.limit) else 1


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--windows-owned-file-command":
        sys.exit(_windows_owned_file_command(sys.argv[2]))
    sys.exit(main())
