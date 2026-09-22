#!/usr/bin/env python3
"""Black-box resilience matrix using disposable client homes only.

The supplied agentplugins binary is executed as a child process. No real client,
user HOME, remote package source, or product test bypass is used.
"""

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import platform
import shlex
import signal
import shutil
import subprocess
import sys
import tempfile
import time
import unicodedata

from harness import Fixture, check, hashes, prepare_scanner, scanner_options
from selection_matrix import prepare_codex_registry

if os.name == "posix":
    import fcntl
    import resource
    import select


CASES = (
    "corrupt-state",
    "future-state",
    "duplicate-installation",
    "tampered-receipt",
    "corrupt-journal",
    "migrate-state-v2",
    "migrate-state-v3",
    "stale-lock-file",
    "held-process-lock",
    "killed-lock-owner",
    "concurrent-add",
    "doctor-during-add",
    "add-remove-race",
    "scanner-failure",
    "permission-denied",
    "closed-stdout",
    "closed-stderr",
    "file-size-limit",
    "foreign-collision",
    "dangling-symlink",
    "source-path-traversal",
    "fresh-multi-client-install",
    "per-client-lifecycle",
    "sigint-during-activation",
    "unicode-normalization",
    "unicode-long-path",
)

POSIX_ONLY = frozenset({
    "held-process-lock", "killed-lock-owner", "scanner-failure",
    "permission-denied", "closed-stdout", "dangling-symlink",
    "closed-stderr", "file-size-limit", "source-path-traversal",
    "sigint-during-activation",
})
UNIX_PROFILE_ONLY = frozenset({
    "fresh-multi-client-install", "per-client-lifecycle"})

ALL_CLIENTS = (
    "codex", "chatgpt", "cursor", "copilot", "vscode", "kiro", "claude",
    "gemini", "opencode", "cline", "windsurf")


def unsupported_reason(name):
    if name in POSIX_ONLY and os.name != "posix":
        return "requires POSIX filesystem or descriptor semantics"
    if name in UNIX_PROFILE_ONLY and platform.system() not in ("Linux", "Darwin"):
        return "synthetic multi-client profile is qualified on Linux and Darwin"
    return ""


def expect_process_line(process, expected, timeout, message):
    check(process.stdout is not None, "process stdout pipe is required")
    readable, _, _ = select.select(
        [process.stdout], [], [], max(0.1, min(timeout, 5)))
    if not readable:
        process.kill()
        process.wait(timeout=2)
        raise AssertionError(message + " before handshake deadline")
    check(process.stdout.readline().strip() == expected, message)


def seed_ten_clients(fixture):
    config = Path(fixture.env["XDG_CONFIG_HOME"])
    if platform.system() == "Darwin":
        editor = fixture.home / "Library/Application Support"
    else:
        editor = config
    paths = (
        fixture.home / ".copilot",
        fixture.home / ".kiro",
        Path(fixture.env["CLAUDE_CONFIG_DIR"]),
        Path(fixture.env["GEMINI_CLI_HOME"]) / ".gemini",
        editor / "Code/User/globalStorage/saoudrizwan.claude-dev",
        config / "opencode",
        fixture.home / ".codeium/windsurf",
    )
    for path in paths:
        path.mkdir(parents=True, exist_ok=True)
    for name in (
            "copilot", "code", "kiro-cli", "claude", "gemini",
            "opencode", "windsurf"):
        shutil.copy2(fixture.bin / "cursor", fixture.bin / name)


def prepare_copilot_registry(fixture):
    state = shlex.quote(str(fixture.root / "copilot-plugin.state"))
    path = shlex.quote(str(fixture.root / "copilot-plugin.path"))
    log = shlex.quote(str(fixture.root / "copilot-plugin.log"))
    copilot = fixture.bin / "copilot"
    copilot.write_text(
        "#!/bin/sh\n"
        f"printf '%s\\n' \"$*\" >> {log}\n"
        "if [ \"$#\" = 1 ] && [ \"$1\" = --version ]; then\n"
        "  printf 'synthetic 99.0.0\\n'; exit 0\n"
        "fi\n"
        "if [ \"$*\" = 'plugin list' ]; then\n"
        f"  if [ -s {state} ] && [ -s {path} ]; then\n"
        f"    IFS= read -r spec < {state}\n"
        f"    IFS= read -r active < {path}\n"
        '    printf "Live Plugins (loaded from a local marketplace '
        'directory, never copied):\\n  \\342\\200\\242 %s (v1.0.0) '
        '(enabled)\\n      from %s\\n" "$spec" "$active"\n'
        "  else\n"
        "    printf \"No plugins installed.\\n\\nUse 'copilot plugin install <source>' to install a plugin.\\n\"\n"
        "  fi\n"
        "  exit 0\n"
        "fi\n"
        "case \"$1 $2\" in\n"
        f"  'plugin install'|'plugin update') printf '%s\\n' \"$3\" > {state}; exit 0 ;;\n"
        f"  'plugin uninstall') : > {state}; exit 0 ;;\n"
        "  'plugin marketplace')\n"
        f"    case \"$3\" in add) printf '%s\\n' \"$4\" > {path}; exit 0 ;;\n"
        "      update) exit 0 ;;\n"
        f"      remove) : > {path}; exit 0 ;; esac ;;\n"
        "esac\n"
        "exit 97\n",
        encoding="utf-8")
    copilot.chmod(0o700)
    shutil.copy2(copilot, fixture.bin / "code")


def prepare_claude_registry(fixture):
    claude = fixture.bin / "claude"
    log = shlex.quote(str(fixture.root / "claude-plugin.log"))
    claude.write_text(
        "#!/bin/sh\n"
        f"printf '%s\\n' \"$*\" >> {log}\n"
        "if [ \"$#\" = 1 ] && [ \"$1\" = --version ]; then "
        "printf 'synthetic 99.0.0\\n'; exit 0; fi\n"
        "if [ \"$*\" = 'plugin list --json' ]; then\n"
        "  for item in \"$CLAUDE_CONFIG_DIR\"/skills/*; do\n"
        "    [ -d \"$item\" ] || continue\n"
        "    [ -f \"$item/.claude-plugin/plugin.json\" ] || continue\n"
        "    printf '[{\"id\":\"pty-synthetic@skills-dir\","
        "\"scope\":\"user\",\"enabled\":true,"
        "\"installPath\":\"%s\"}]\\n' \"$item\"\n"
        "    exit 0\n"
        "  done\n"
        "  printf '[]\\n'; exit 0\n"
        "fi\n"
        "exit 97\n",
        encoding="utf-8")
    claude.chmod(0o700)


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def parse_envelope(stdout):
    if not stdout.strip():
        return None
    try:
        value = json.loads(stdout)
    except json.JSONDecodeError as error:
        raise AssertionError(
            f"stdout is not one JSON envelope: {error}: {stdout!r}") from error
    check(isinstance(value, dict), "JSON output is not an object")
    check(value.get("schema_version") == 1, "JSON schema version changed")
    check(value.get("command"), "JSON command is missing")
    check(value.get("result") in ("success", "failure"),
          "JSON result is invalid")
    return value


class Matrix:
    def __init__(self, binary, timeout, scanner_binary=None):
        self.binary = binary
        self.timeout = timeout
        self.scanner_binary = scanner_binary

    def command(self, fixture, evidence, label, *args, expect=None,
                closed_stdout=False, closed_stderr=False, preexec_fn=None):
        argv = [str(self.binary), *args, "--format=json"]
        env = dict(fixture.env, NO_COLOR="1")
        started = time.monotonic()
        if closed_stdout or closed_stderr:
            check(os.name == "posix", "closed output pipe requires POSIX")
            stdout_pipe = stderr_pipe = None
            try:
                if closed_stdout:
                    read_fd, stdout_pipe = os.pipe()
                    os.close(read_fd)
                if closed_stderr:
                    read_fd, stderr_pipe = os.pipe()
                    os.close(read_fd)
                result = subprocess.run(
                    argv, cwd=fixture.project, env=env,
                    stdout=stdout_pipe if closed_stdout else subprocess.PIPE,
                    stderr=stderr_pipe if closed_stderr else subprocess.PIPE,
                    text=True, timeout=self.timeout, preexec_fn=preexec_fn)
            finally:
                if stdout_pipe is not None:
                    os.close(stdout_pipe)
                if stderr_pipe is not None:
                    os.close(stderr_pipe)
            out = "" if closed_stdout else result.stdout
            err = "" if closed_stderr else result.stderr
        else:
            result = subprocess.run(
                argv, cwd=fixture.project, env=env, capture_output=True,
                text=True, timeout=self.timeout, preexec_fn=preexec_fn)
            out, err = result.stdout, result.stderr
        evidence.mkdir(parents=True, exist_ok=True)
        write_json(evidence / f"{label}.command.json", argv)
        (evidence / f"{label}.stdout").write_text(out, encoding="utf-8")
        (evidence / f"{label}.stderr").write_text(err, encoding="utf-8")
        write_json(evidence / f"{label}.result.json", {
            "exit": result.returncode,
            "elapsed_seconds": round(time.monotonic() - started, 3),
        })
        if expect is not None:
            check((result.returncode == 0) == expect,
                  f"{label}: exit={result.returncode}, "
                  f"expected success={expect}; stderr={err!r}")
        envelope = parse_envelope(out) if out else None
        if envelope is not None:
            check((result.returncode == 0) ==
                  (envelope["result"] == "success"),
                  f"{label}: exit/result disagreement")
        return result, envelope

    def add(self, fixture, evidence, label="add", expect=True):
        return self.command(
            fixture, evidence, label, "add", str(fixture.package),
            "--target=cursor", expect=expect)

    def remove(self, fixture, evidence, label="remove"):
        return self.command(
            fixture, evidence, label, "remove", "pty-synthetic",
            "--target=cursor", "--external-uninstalled", "--purge-data",
            expect=True)

    def doctor(self, fixture, evidence, label="doctor", expect=True):
        return self.command(
            fixture, evidence, label, "doctor", expect=expect)

    @staticmethod
    def state_path(fixture):
        return fixture.data / "state-v2.json"

    @staticmethod
    def protected(fixture):
        state = fixture.data / "state-v2.json"
        return {
            "home": hashes(fixture.home),
            "project": hashes(fixture.project),
            "managed": hashes(fixture.data / "managed"),
            "operations": hashes(fixture.data / "operations-v2"),
            "state": state.read_bytes().hex() if state.is_file() else None,
        }

    @staticmethod
    def assert_no_open_operations(fixture):
        operations = fixture.data / "operations-v2"
        check(not operations.exists() or not any(operations.iterdir()),
              "open operation journal remained")

    def assert_healthy(self, fixture, evidence, installations):
        _, envelope = self.doctor(fixture, evidence)
        data = envelope["data"]
        check(data["installation_count"] == installations,
              f"doctor installation_count={data['installation_count']}, "
              f"want {installations}")
        check(data["open_operation_count"] == 0,
              "doctor reports an open operation")
        self.assert_no_open_operations(fixture)

    def invalid_state(self, fixture, evidence, body, reason):
        state = self.state_path(fixture)
        state.parent.mkdir(parents=True, exist_ok=True)
        state.write_text(body, encoding="utf-8")
        before = self.protected(fixture)
        result, envelope = self.add(fixture, evidence, expect=False)
        check(envelope is None or envelope["result"] == "failure",
              "invalid state reported success")
        diagnostic = (result.stdout + "\n" + result.stderr).lower()
        check(any(word in diagnostic for word in
                  ("state", "schema", "json", "decode")),
              f"{reason}: missing state diagnostic")
        check(self.protected(fixture) == before,
              f"{reason}: invalid state caused mutation")
        self.assert_no_open_operations(fixture)

    def case_corrupt_state(self, fixture, evidence):
        self.invalid_state(
            fixture, evidence, '{"schema_version":', "corrupt state")

    def case_future_state(self, fixture, evidence):
        self.invalid_state(
            fixture, evidence,
            '{"schema_version":999,"installations":[]}\n', "future state")

    def installed_state(self, fixture, evidence):
        self.add(fixture, evidence)
        state = json.loads(
            self.state_path(fixture).read_text(encoding="utf-8"))
        check(len(state.get("installations", [])) == 1,
              "fresh add did not create one installation")
        return state

    def reject_tampered_installed_state(
            self, fixture, evidence, mutate, reason):
        state = self.installed_state(fixture, evidence)
        mutate(state)
        write_json(self.state_path(fixture), state)
        before = self.protected(fixture)
        result, envelope = self.doctor(
            fixture, evidence, label="doctor-tampered", expect=False)
        check(envelope is None or envelope["result"] == "failure",
              f"{reason}: doctor reported success")
        check(self.protected(fixture) == before,
              f"{reason}: read-only doctor mutated state")
        diagnostic = (result.stdout + "\n" + result.stderr).lower()
        check(any(word in diagnostic for word in
                  ("state", "duplicate", "receipt", "schema", "invalid")),
              f"{reason}: missing integrity diagnostic")
        self.assert_no_open_operations(fixture)

    def case_duplicate_installation(self, fixture, evidence):
        def mutate(state):
            state["installations"].append(
                copy.deepcopy(state["installations"][0]))
        self.reject_tampered_installed_state(
            fixture, evidence, mutate, "duplicate installation")

    def case_tampered_receipt(self, fixture, evidence):
        def mutate(state):
            binding = next(iter(
                state["installations"][0]["clients"].values()))
            check(binding.get("receipts"),
                  "installed binding has no receipts")
            binding["receipts"][0]["phase"] = "tampered"
        self.reject_tampered_installed_state(
            fixture, evidence, mutate, "tampered receipt")

    def case_corrupt_journal(self, fixture, evidence):
        journal = fixture.data / "operations-v2"
        journal.mkdir(parents=True, exist_ok=True)
        (journal / "broken.json").write_text("{", encoding="utf-8")
        before = self.protected(fixture)
        result, _ = self.add(fixture, evidence, expect=False)
        diagnostic = (result.stdout + result.stderr).lower()
        check(any(word in diagnostic for word in
                  ("journal", "receipt", "decode")),
              "corrupt journal diagnostic missing")
        check(self.protected(fixture) == before,
              "corrupt journal allowed mutation")
    def migrate_legacy_state(self, fixture, evidence, schema):
        state = self.state_path(fixture)
        state.parent.mkdir(parents=True, exist_ok=True)
        legacy = {
            "schema_version": schema,
            "installations": [{
                "installation_id": "00000000-0000-4000-8000-000000000001",
                "declared_name": "demo",
                "source": {
                    "source_binding_id": "src_demo",
                    "requested_source": "./demo",
                    "canonical_source": "/fixtures/demo",
                    "resolved_revision": "local",
                    "tree_digest": "sha256:tree",
                },
                "package": {
                    "loader_kind": "agent_plugins",
                    "format_id": "agent-plugins/1.0.0",
                    "schema_uri": (
                        "https://agent-plugins.org/schemas/1.0.0/"
                        "plugin.schema.json"),
                    "declared_name": "demo",
                    "manifest_digest": "sha256:manifest",
                    "inventory": {},
                },
                "clients": {},
                "created_at": "2026-08-01T00:00:00Z",
                "updated_at": "2026-08-01T00:00:00Z",
            }],
        }
        original = (json.dumps(legacy, separators=(",", ":")) + "\n").encode()
        state.write_bytes(original)
        _, plan = self.command(
            fixture, evidence, "migrate-dry-run", "migrate-state",
            "--dry-run", expect=True)
        check(plan["data"]["source_schema"] == schema,
              "migration plan reported the wrong source schema")
        check(state.read_bytes() == original,
              "migration dry-run rewrote legacy state")
        self.command(
            fixture, evidence, "migrate-apply", "migrate-state", expect=True)
        migrated = json.loads(state.read_text(encoding="utf-8"))
        check(migrated["schema_version"] == 4,
              "migration did not fix state forward to schema 4")
        check(len(migrated["installations"]) == 1,
              "migration lost the legacy installation")
        backups = list(state.parent.glob(
            f"{state.name}.schema{schema}.backup-agentplugins-*"))
        check(len(backups) == 1 and backups[0].read_bytes() == original,
              "migration backup did not preserve reviewed legacy bytes")
        # Doctor reports tracked client bindings; the historical fixture keeps
        # one installation record but intentionally contains no client binding.
        self.assert_healthy(fixture, evidence / "health", 0)

    def case_migrate_state_v2(self, fixture, evidence):
        self.migrate_legacy_state(fixture, evidence, 2)

    def case_migrate_state_v3(self, fixture, evidence):
        self.migrate_legacy_state(fixture, evidence, 3)

    def case_stale_lock_file(self, fixture, evidence):
        lock = fixture.data / "mutation.lock"
        lock.write_text("stale-file-without-kernel-lock\n", encoding="utf-8")
        self.add(fixture, evidence)
        self.assert_healthy(fixture, evidence / "health", 1)
        self.remove(fixture, evidence)
        self.assert_healthy(fixture, evidence / "clean", 0)

    def case_held_process_lock(self, fixture, evidence):
        check(os.name == "posix", "held process lock requires POSIX")
        lock_path = fixture.data / "mutation.lock"
        lock_path.parent.mkdir(parents=True, exist_ok=True)
        with lock_path.open("a+b") as lock:
            fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            before = self.protected(fixture)
            result, _ = self.add(
                fixture, evidence, label="add-while-locked", expect=False)
            diagnostic = (result.stdout + result.stderr).lower()
            for phrase in (
                    "another agentplugins mutation is active",
                    "wait for it to finish", "retry the same command",
                    "no target was changed"):
                check(phrase in diagnostic,
                      f"busy lock diagnostic missing {phrase!r}")
            check(self.protected(fixture) == before,
                  "busy lock allowed mutation")
            fcntl.flock(lock.fileno(), fcntl.LOCK_UN)
        self.add(fixture, evidence, label="retry-after-unlock")
        self.assert_healthy(fixture, evidence / "health", 1)

    def case_killed_lock_owner(self, fixture, evidence):
        check(os.name == "posix", "killed lock owner requires POSIX")
        lock_path = fixture.data / "mutation.lock"
        lock_path.parent.mkdir(parents=True, exist_ok=True)
        script = (
            "import fcntl,sys,time; "
            "handle=open(sys.argv[1], 'a+b'); "
            "fcntl.flock(handle.fileno(), fcntl.LOCK_EX); "
            "print('locked', flush=True); time.sleep(60)"
        )
        holder = subprocess.Popen(
            [sys.executable, "-c", script, str(lock_path)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        try:
            expect_process_line(
                holder, "locked", self.timeout,
                "lock owner did not acquire the kernel lock")
            holder.kill()
            holder.wait(timeout=2)
        finally:
            if holder.poll() is None:
                holder.kill()
                holder.wait(timeout=2)
            if holder.stdout is not None:
                holder.stdout.close()
            if holder.stderr is not None:
                holder.stderr.close()
        self.add(fixture, evidence, label="add-after-owner-kill")
        self.assert_healthy(fixture, evidence / "health", 1)

    def spawn(self, fixture, *args):
        return subprocess.Popen(
            [str(self.binary), *args, "--format=json"],
            cwd=fixture.project, env=dict(fixture.env, NO_COLOR="1"),
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)

    def collect(self, process, evidence, label):
        try:
            out, err = process.communicate(timeout=self.timeout)
        except subprocess.TimeoutExpired:
            process.kill()
            process.communicate(timeout=2)
            raise AssertionError(f"{label} timed out")
        evidence.mkdir(parents=True, exist_ok=True)
        (evidence / f"{label}.stdout").write_text(out, encoding="utf-8")
        (evidence / f"{label}.stderr").write_text(err, encoding="utf-8")
        envelope = parse_envelope(out) if out else None
        if envelope is not None:
            check((process.returncode == 0) ==
                  (envelope["result"] == "success"),
                  f"{label}: exit/result disagreement")
        return process.returncode, envelope, err

    def case_concurrent_add(self, fixture, evidence):
        argv = ("add", str(fixture.package), "--target=cursor")
        processes = [self.spawn(fixture, *argv) for _ in range(4)]
        rows = [self.collect(process, evidence, f"add-{index}")
                for index, process in enumerate(processes)]
        check(any(code == 0 for code, _, _ in rows),
              "no concurrent add succeeded")
        self.assert_healthy(fixture, evidence / "health", 1)
        self.add(fixture, evidence, label="convergent-retry")
        self.assert_healthy(fixture, evidence / "retry-health", 1)

    def case_doctor_during_add(self, fixture, evidence):
        add = self.spawn(
            fixture, "add", str(fixture.package), "--target=cursor")
        doctors = [self.spawn(fixture, "doctor") for _ in range(8)]
        add_row = self.collect(add, evidence, "add")
        check(add_row[0] == 0,
              f"add failed during doctor race: {add_row[2]}")
        for index, doctor in enumerate(doctors):
            code, envelope, err = self.collect(
                doctor, evidence, f"doctor-{index}")
            check(code == 0, f"doctor race failed: {err}")
            check(envelope["data"]["installation_count"] in (0, 1),
                  "doctor observed a torn installation count")
            check(envelope["data"]["open_operation_count"] == 0,
                  "doctor exposed a partial operation")
        self.assert_healthy(fixture, evidence / "health", 1)

    def case_add_remove_race(self, fixture, evidence):
        self.add(fixture, evidence, label="initial-add")
        add = self.spawn(
            fixture, "add", str(fixture.package), "--target=cursor")
        remove = self.spawn(
            fixture, "remove", "pty-synthetic", "--target=cursor",
            "--external-uninstalled", "--purge-data")
        rows = [self.collect(add, evidence, "racing-add"),
                self.collect(remove, evidence, "racing-remove")]
        check(any(code == 0 for code, _, _ in rows),
              "both racing mutations failed")
        self.add(fixture, evidence, label="convergent-add")
        self.assert_healthy(fixture, evidence / "health", 1)
        self.remove(fixture, evidence, label="final-remove")
        self.assert_healthy(fixture, evidence / "clean", 0)

    def case_scanner_failure(self, fixture, evidence):
        scanner = next(
            (fixture.data / "security" / "lintai" / "0.1.3").rglob(
                "lintai*"))
        scanner.write_text(
            "#!/bin/sh\nprintf 'synthetic scanner failure\\n' >&2\nexit 41\n",
            encoding="utf-8")
        scanner.chmod(0o700)
        before = self.protected(fixture)
        result, _ = self.add(fixture, evidence, expect=False)
        diagnostic = (result.stdout + result.stderr).lower()
        check("scanner" in diagnostic or "security" in diagnostic,
              "scanner failure diagnostic missing")
        check(self.protected(fixture) == before,
              "scanner failure allowed mutation")

    def case_permission_denied(self, fixture, evidence):
        check(os.name == "posix", "permission case requires POSIX")
        target = fixture.home / ".cursor" / "plugins" / "local"
        target.mkdir(parents=True, exist_ok=True)
        target.chmod(0o500)
        before = self.protected(fixture)
        try:
            result, _ = self.add(fixture, evidence, expect=False)
            diagnostic = (result.stdout + result.stderr).lower()
            check(any(word in diagnostic for word in
                      ("permission", "denied", "read-only")),
                  "permission diagnostic missing")
            check(self.protected(fixture) == before,
                  "permission failure allowed mutation")
        finally:
            target.chmod(0o700)

    def case_closed_stdout(self, fixture, evidence):
        result, _ = self.command(
            fixture, evidence, "closed-stdout", "add", str(fixture.package),
            "--target=cursor", expect=False, closed_stdout=True)
        check(result.returncode != 0,
              "closed stdout pipe reported success")
        self.assert_healthy(fixture, evidence / "health", 1)
        self.remove(fixture, evidence)
        self.assert_healthy(fixture, evidence / "clean", 0)
    def case_closed_stderr(self, fixture, evidence):
        before = self.protected(fixture)
        result, _ = self.command(
            fixture, evidence, "closed-stderr", "add",
            str(fixture.root / "missing-package"), "--target=cursor",
            expect=False, closed_stderr=True)
        check(result.returncode != 0, "closed stderr reported success")
        check(self.protected(fixture) == before,
              "closed stderr failure allowed mutation")

    def case_file_size_limit(self, fixture, evidence):
        check(os.name == "posix", "file size limit requires POSIX")

        def limit_writes():
            signal.signal(signal.SIGXFSZ, signal.SIG_IGN)
            resource.setrlimit(resource.RLIMIT_FSIZE, (1, 1))

        before = self.protected(fixture)
        result, _ = self.command(
            fixture, evidence, "file-size-limit", "add",
            str(fixture.package), "--target=cursor", expect=False,
            preexec_fn=limit_writes)
        diagnostic = (result.stdout + result.stderr).lower()
        check(any(word in diagnostic for word in (
            "file too large", "space", "write", "persist", "journal",
            "snapshot package content")),
            "file size failure diagnostic missing")
        check(self.protected(fixture) == before,
              "file size failure allowed partial mutation")
        self.assert_no_open_operations(fixture)


    def case_foreign_collision(self, fixture, evidence):
        state = self.installed_state(fixture, evidence)
        binding = next(iter(
            state["installations"][0]["clients"].values()))
        target = Path(binding["target_locator"])
        self.remove(fixture, evidence)
        target.mkdir(parents=True, exist_ok=True)
        marker = target / "foreign.txt"
        marker.write_text("unowned sentinel\n", encoding="utf-8")
        before = self.protected(fixture)
        result, _ = self.add(
            fixture, evidence, label="collision-add", expect=False)
        diagnostic = (result.stdout + result.stderr).lower()
        check(any(word in diagnostic for word in (
            "ownership", "unmanaged", "collision", "indeterminate",
            "manifest", "authoritative")),
            "foreign collision diagnostic missing")
        check(marker.read_text(encoding="utf-8") == "unowned sentinel\n",
              "foreign collision content changed")
        check(self.protected(fixture) == before,
              "foreign collision allowed mutation")

    def case_dangling_symlink(self, fixture, evidence):
        state = self.installed_state(fixture, evidence)
        binding = next(iter(
            state["installations"][0]["clients"].values()))
        target = Path(binding["target_locator"])
        self.remove(fixture, evidence)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.symlink_to(
            target.parent / "missing-foreign-target", target_is_directory=True)
        before = self.protected(fixture)
        result, _ = self.add(
            fixture, evidence, label="symlink-add", expect=False)
        diagnostic = (result.stdout + result.stderr).lower()
        check(any(word in diagnostic for word in
                  ("symlink", "ownership", "identity", "indeterminate")),
              "dangling symlink diagnostic missing")
        check(target.is_symlink(), "dangling symlink was replaced")
        check(self.protected(fixture) == before,
              "dangling symlink allowed mutation")

    def case_source_path_traversal(self, fixture, evidence):
        check(os.name == "posix", "source traversal case requires POSIX")
        outside = fixture.root / "outside-sentinel"
        outside.write_text("foreign\n", encoding="utf-8")
        skills = fixture.package / "skills"
        skills.mkdir()
        (skills / "escape").symlink_to(outside)
        before = self.protected(fixture)
        result, _ = self.add(
            fixture, evidence, label="source-traversal", expect=False)
        diagnostic = (result.stdout + result.stderr).lower()
        check(any(word in diagnostic for word in (
            "symlink", "path", "traversal", "package")),
            "source traversal diagnostic missing")
        check(outside.read_text(encoding="utf-8") == "foreign\n",
              "source traversal changed external content")
        check(self.protected(fixture) == before,
              "source traversal allowed mutation")
    def case_sigint_during_activation(self, fixture, evidence):
        check(os.name == "posix", "SIGINT activation case requires POSIX")
        seed_ten_clients(fixture)
        claude = fixture.bin / "claude"
        claude_log = fixture.root / "claude-sigint.log"
        activation_marker = fixture.root / "claude-activation-started"
        log_path = shlex.quote(str(claude_log))
        marker_path = shlex.quote(str(activation_marker))
        sleep_path = shlex.quote(shutil.which("sleep") or "/bin/sleep")

        def write_claude(slow):
            delay = (
                f"    : > {marker_path}\n"
                f"    {sleep_path} 60\n"
            ) if slow else ""
            claude.write_text(
                "#!/bin/sh\n"
                f"printf '%s\\n' \"$*\" >> {log_path}\n"
                "if [ \"$#\" = 1 ] && [ \"$1\" = --version ]; then "
                "printf 'synthetic 99.0.0\\n'; exit 0; fi\n"
                "if [ \"$*\" = 'plugin list --json' ]; then\n"
                "  for item in \"$CLAUDE_CONFIG_DIR\"/skills/*; do\n"
                "    [ -d \"$item\" ] || continue\n"
                "    [ -f \"$item/.claude-plugin/plugin.json\" ] || continue\n"
                + delay +
                "    printf '[{\"id\":\"pty-synthetic@skills-dir\","
                "\"scope\":\"user\",\"enabled\":true,"
                "\"installPath\":\"%s\"}]\\n' \"$item\"\n"
                "    exit 0\n"
                "  done\n"
                "  printf '[]\\n'; exit 0\n"
                "fi\n"
                "exit 97\n",
                encoding="utf-8")
            claude.chmod(0o700)

        write_claude(True)
        fixture.before = fixture.mutations()
        process = self.spawn(
            fixture, "add", str(fixture.package), "--target=cursor,claude")
        deadline = time.monotonic() + self.timeout
        while (not activation_marker.exists() and process.poll() is None and
               time.monotonic() < deadline):
            time.sleep(0.02)
        if not activation_marker.exists():
            process.kill()
            code, _, err = self.collect(
                process, evidence, "sigint-before-marker")
            raise AssertionError(
                f"activation marker was not reached: exit={code}, stderr={err!r}")
        process.send_signal(signal.SIGINT)
        code, envelope, _ = self.collect(
            process, evidence, "sigint-during-activation")
        check(code != 0, "SIGINT during activation reported success")
        check(envelope is None or envelope["result"] == "failure",
              "SIGINT failure envelope reported success")
        self.assert_healthy(fixture, evidence / "interrupted-health", 1)

        write_claude(False)
        self.command(
            fixture, evidence, "retry-after-sigint", "add",
            str(fixture.package), "--target=cursor,claude", expect=True)
        self.assert_healthy(fixture, evidence / "retry-health", 1)

    def case_fresh_multi_client_install(self, fixture, evidence):
        targets = (
            "cursor", "kiro", "claude", "gemini", "opencode", "cline",
            "windsurf")
        seed_ten_clients(fixture)
        claude_skills = Path(fixture.env["CLAUDE_CONFIG_DIR"]) / "skills"
        claude_skills.mkdir()
        unrelated_dangling = claude_skills / "ccc"
        unrelated_dangling.symlink_to(fixture.root / "removed-ccc")
        claude_log = fixture.root / "claude-probe.log"
        claude = fixture.bin / "claude"
        log_path = shlex.quote(str(claude_log))
        claude.write_text(
            "#!/bin/sh\n"
            f"printf '%s\\n' \"$*\" >> {log_path}\n"
            "if [ \"$#\" = 1 ] && [ \"$1\" = --version ]; then "
            "printf 'synthetic 99.0.0\\n'; exit 0; fi\n"
            "if [ \"$*\" = 'plugin list --json' ]; then\n"
            "  for item in \"$CLAUDE_CONFIG_DIR\"/skills/*; do\n"
            "    [ -d \"$item\" ] || continue\n"
            "    [ -f \"$item/.claude-plugin/plugin.json\" ] || continue\n"
            "    printf '[{\"id\":\"pty-synthetic@skills-dir\","
            "\"scope\":\"user\",\"enabled\":true,"
            "\"installPath\":\"%s\"}]\\n' \"$item\"\n"
            "    exit 0\n"
            "  done\n"
            "  printf '[]\\n'; exit 0\n"
            "fi\n"
            "exit 97\n",
            encoding="utf-8")
        claude.chmod(0o700)
        fixture.before = fixture.mutations()
        result, envelope = self.command(
            fixture, evidence, "fresh-multi-client-add", "add",
            str(fixture.package), "--target=" + ",".join(targets))
        if claude_log.is_file():
            shutil.copy2(claude_log, evidence / "claude-probe.log")
        check(result.returncode == 0,
              f"fresh multi-client add failed: {result.stderr!r}")
        rows = envelope["data"]["targets"]
        check({row["target"] for row in rows} == set(targets),
              "multi-client result lost a selected logical target")
        check(len(rows) == len(targets),
              "multi-client result duplicated a target")
        probes = claude_log.read_text(encoding="utf-8").splitlines()
        check(probes.count("plugin list --json") >= 2 and
              set(probes).issubset({"--version", "plugin list --json"}),
              f"unexpected Claude probe sequence: {probes}")
        self.assert_healthy(fixture, evidence / "health", 1)
        check(unrelated_dangling.is_symlink(),
              "unrelated dangling Claude skill was mutated")
        self.command(
            fixture, evidence, "fresh-multi-client-remove", "remove",
            "pty-synthetic", "--target=" + ",".join(targets),
            "--external-uninstalled", "--purge-data", expect=True)
        self.assert_healthy(fixture, evidence / "removed-health", 0)
        check(unrelated_dangling.is_symlink(),
              "remove mutated unrelated dangling Claude skill")

    def case_per_client_lifecycle(self, fixture, evidence):
        for target in ALL_CLIENTS:
            client_fixture = Fixture(
                fixture.root / ("client-" + target),
                scanner_binary=self.scanner_binary)
            seed_ten_clients(client_fixture)
            if target == "codex":
                prepare_codex_registry(client_fixture)
            if target in ("copilot", "vscode"):
                prepare_copilot_registry(client_fixture)
            if target == "claude":
                prepare_claude_registry(client_fixture)
            target_evidence = evidence / target
            if target == "chatgpt":
                before = self.protected(client_fixture)
                result, _ = self.command(
                    client_fixture, target_evidence, "add-fail-closed",
                    "add", str(client_fixture.package),
                    "--target=chatgpt", expect=False)
                diagnostic = (result.stdout + "\n" + result.stderr).lower()
                check(
                    "identity" in diagnostic or "directory" in diagnostic,
                    "chatgpt: unsigned local source lacked a clear "
                    "identity/directory diagnostic")
                check(
                    self.protected(client_fixture) == before,
                    "chatgpt: rejected unsigned source mutated protected state")
                write_json(target_evidence / "coverage.json", {
                    "mode": "fail_closed_unsigned_local_source",
                    "signed_directory_lifecycle_test": (
                        "install/integrationctl/agentplugins/usecase/"
                        "manual_remote_lifecycle_test.go::"
                        "TestSignedChatGPTPreparationSupportsAddUpdateAnd"
                        "RepairWhileRemoteActivationIsPending"),
                })
                continue
            _, added = self.command(
                client_fixture, target_evidence, "add", "add",
                str(client_fixture.package), "--target=" + target,
                expect=True)
            rows = added["data"].get("targets", [])
            check(len(rows) == 1 and rows[0]["target"] == target,
                  f"{target}: add lost logical target identity")
            self.assert_healthy(
                client_fixture, target_evidence / "after-add", 1)
            for action in ("update", "repair"):
                _, result = self.command(
                    client_fixture, target_evidence, action, action,
                    "pty-synthetic", "--target=" + target, expect=True)
                rows = result["data"].get("targets", [])
                check(len(rows) == 1 and rows[0]["target"] == target,
                      f"{target}: {action} lost logical target identity")
                self.assert_healthy(
                    client_fixture, target_evidence / ("after-" + action), 1)
            self.command(
                client_fixture, target_evidence, "remove", "remove",
                "pty-synthetic", "--target=" + target,
                "--external-uninstalled", "--purge-data", expect=True)
            self.assert_healthy(
                client_fixture, target_evidence / "after-remove", 0)
            if target not in ("codex", "copilot", "vscode", "claude"):
                client_fixture.validate_stubs()

    def case_unicode_normalization(self, fixture, evidence):
        nfc = unicodedata.normalize("NFC", "cafe\u0301-package")
        nfd = unicodedata.normalize("NFD", "caf\u00e9-package")
        check(nfc != nfd, "normalization fixture collapsed in memory")
        observed_requested = []
        for index, name in enumerate((nfc, nfd)):
            target = fixture.root / (f"unicode-{index}-" + name)
            fixture.package.rename(target)
            fixture.package = target
            self.add(fixture, evidence / str(index))
            self.assert_healthy(fixture, evidence / str(index) / "health", 1)
            state = json.loads(
                self.state_path(fixture).read_text(encoding="utf-8"))
            installations = state.get("installations", [])
            check(len(installations) == 1,
                  "unicode source lost installation identity")
            source = installations[0].get("source", {})
            requested = source.get("requested_source")
            observed_requested.append(requested)
            check(requested == str(target),
                  "unicode requested source was normalized or rewritten")
            canonical = Path(source.get("canonical_source", ""))
            check(canonical.exists() and canonical.samefile(target),
                  "unicode canonical source does not resolve to package")
            self.remove(fixture, evidence / str(index))
            self.assert_healthy(fixture, evidence / str(index) / "clean", 0)
        check(observed_requested[0] != observed_requested[1],
              "NFC and NFD requested sources collided in state")

    def case_unicode_long_path(self, fixture, evidence):
        nested = fixture.root / ("длинный-путь-" + "x" * 80) / "插件"
        nested.parent.mkdir(parents=True)
        fixture.package.rename(nested)
        fixture.package = nested
        self.add(fixture, evidence)
        self.assert_healthy(fixture, evidence / "health", 1)
        self.remove(fixture, evidence)
        self.assert_healthy(fixture, evidence / "clean", 0)

    def run_case(self, name, evidence):
        method = getattr(self, "case_" + name.replace("-", "_"))
        with tempfile.TemporaryDirectory(
                prefix="uap-resilience-") as temp:
            fixture = Fixture(temp, scanner_binary=self.scanner_binary)
            method(fixture, evidence)
            fixture.validate_stubs()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--case", action="append", choices=CASES)
    parser.add_argument("--timeout", type=float, default=20)
    scanner_options(parser)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    check(binary.is_file(), "binary must be a regular file")
    args.artifacts.mkdir(parents=True, exist_ok=False)
    scanner_evidence = prepare_scanner(args)
    matrix = Matrix(binary, args.timeout, args.scanner_path)
    report = {
        "platform": platform.platform(),
        "binary": str(binary),
        "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
        "source_sha": subprocess.check_output(
            ["git", "rev-parse", "HEAD"],
            cwd=Path(__file__).resolve().parents[2], text=True).strip(),
        "isolation": (
            "fresh disposable HOME, clients, state and package per case"),
        "scanner": scanner_evidence,
        "cases": [],
    }
    for name in args.case or CASES:
        row = {"case": name, "status": "pass"}
        started = time.monotonic()
        reason = unsupported_reason(name)
        if reason:
            row.update(status="skip", reason=reason)
        else:
            try:
                matrix.run_case(name, args.artifacts / name)
            except Exception as error:
                row.update(status="fail", error=repr(error))
        row["elapsed_seconds"] = round(time.monotonic() - started, 3)
        report["cases"].append(row)
        write_json(args.artifacts / "report.json", report)
        print(json.dumps(row), flush=True)
    report["evidence_hashes"] = hashes(args.artifacts)
    write_json(args.artifacts / "report.json", report)
    return int(any(
        row["status"] == "fail" for row in report["cases"]))


if __name__ == "__main__":
    raise SystemExit(main())
