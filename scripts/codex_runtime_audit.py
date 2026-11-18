"""Strict native Codex Skill evidence with independently checked SQLite state.

Codex 0.153.4's supported sqlite_home separates runtime databases from native
registration files. No native-profile file is excluded from the caller's raw
snapshot. SQLite accepts a changed WAL representation only when integrity,
schema and every table's contents still match the pre-install native baseline.
"""

import asyncio
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import signal
import sqlite3
import stat
import subprocess
import sys
import tempfile
import time
import tomllib

_FILES = run_path(str(Path(__file__).with_name("native_client_project_artifacts.py")))
FILE_LIMIT = 32 << 20
TOTAL_LIMIT = 64 << 20
ROW_LIMIT = 20000
TEMPORARY_GIT_HEAD = b"ref: refs/heads/main\n"
TEMPORARY_GIT_LIMIT = 64
DATABASES = frozenset(("state_5.sqlite", "logs_2.sqlite", "goals_1.sqlite",
                      "memories_1.sqlite", "queue_1.sqlite", "thread_history_1.sqlite"))


def _regular_bytes(path, limit):
    before = path.lstat()
    if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
            or not 0 <= before.st_size <= limit or path.resolve(strict=True) != path):
        raise ValueError("Codex audit file is not bounded, regular and exclusively linked")
    descriptor = _FILES["open_member"](path)
    try:
        held = os.fstat(descriptor)
        if not _FILES["same_opened"](before, held):
            raise ValueError("Codex audit opened file identity changed")
        chunks, remaining = [], before.st_size + 1
        while remaining:
            block = os.read(descriptor, min(remaining, 65536))
            if not block:
                break
            chunks.append(block)
            remaining -= len(block)
        body = b"".join(chunks)
        if (len(body) != before.st_size
                or _FILES["identity"](held) != _FILES["identity"](os.fstat(descriptor))):
            raise ValueError("Codex audit bytes changed during the full read")
    finally:
        os.close(descriptor)
    if _FILES["identity"](before) != _FILES["identity"](path.lstat()):
        raise ValueError("Codex audit file changed after the full read")
    return body


def _real_directory(path):
    info = path.lstat()
    if (not stat.S_ISDIR(info.st_mode) or path.is_symlink()
            or getattr(info, "st_file_attributes", 0) & 0x400
            or path.resolve(strict=True) != path):
        raise ValueError("Codex audit directory is redirected")
    return info


def _bounded_member_names(path, limit):
    before = _FILES["identity"](_real_directory(path))
    names = []
    with os.scandir(path) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > limit:
                raise ValueError("Codex temporary Git member count exceeds its bound")
    if _FILES["identity"](_real_directory(path)) != before:
        raise ValueError("Codex temporary Git directory changed during enumeration")
    return sorted(names)


def _temporary_git_name_diagnostic(name, phase):
    """Quote bounded listing evidence without reading the rejected child."""
    return ("Codex temporary Git directory name is not qualified; "
            f"checkpoint={ascii(phase[:32])}; basename={ascii(name[:64])}; "
            f"basename_chars={len(name)}; basename_truncated={len(name) > 64}")


def _temporary_git_state(profile, *, phase="direct-scan"):
    """Verify the pinned client's empty automatic-Git staging repositories.

    Codex 0.153.4 core-plugins/src/git_policy.rs writes this skeleton under
    CODEX_HOME/.tmp. Optional objects/refs must be empty; this structural
    check does not identify the historical writer of a matching residue.
    startup_sync.rs may also create plugins.sync.lock in this shared root.
    Retain an observed empty lock, never accept it as a repeat delta.
    """
    root = profile / ".tmp"
    try:
        root.lstat()
    except FileNotFoundError:
        return {}, {}
    _real_directory(profile)
    files, identities, witnesses = {}, {}, {}

    def directory(path):
        info = _real_directory(path)
        witnesses[path] = _FILES["identity"](info)
        identities[path] = (info.st_dev, info.st_ino, info.st_mode,
                            getattr(info, "st_file_attributes", None))

    directory(root)
    names = _bounded_member_names(root, TEMPORARY_GIT_LIMIT)
    layouts = {}
    for name in names:
        if name == "plugins.sync.lock":
            lock = root / name
            identities[lock] = _FILES["identity"](lock.lstat())
            _regular_bytes(lock, 0)
            continue
        if re.fullmatch(r"git-[A-Za-z0-9]{6}", name) is None:
            raise ValueError(_temporary_git_name_diagnostic(name, phase))
        repository = root / name
        directory(repository)
        members = _bounded_member_names(repository, 3)
        if "HEAD" not in members or not set(members) <= {"HEAD", "objects", "refs"}:
            raise ValueError("Codex temporary Git repository has unexpected members")
        layouts[repository] = members
        for child in (member for member in members if member != "HEAD"):
            path = repository / child
            directory(path)
            _bounded_member_names(path, 0)
            layouts[path] = []
        head = repository / "HEAD"
        before = _FILES["identity"](head.lstat())
        if _regular_bytes(head, len(TEMPORARY_GIT_HEAD)) != TEMPORARY_GIT_HEAD:
            raise ValueError("Codex temporary Git HEAD differs from the pinned native bytes")
        if before != _FILES["identity"](head.lstat()):
            raise ValueError("Codex temporary Git HEAD changed during inspection")
        identities[head] = before
        files[f"{profile.name}/{head.relative_to(profile)}"] = hashlib.sha256(TEMPORARY_GIT_HEAD).hexdigest()
    if _bounded_member_names(root, TEMPORARY_GIT_LIMIT) != names:
        raise ValueError("Codex temporary Git namespace changed during inspection")
    for path, members in layouts.items():
        if _bounded_member_names(path, 3) != members:
            raise ValueError("Codex temporary Git members changed during inspection")
    _confirm_temporary_git_identities(identities, witnesses)
    return files, identities


def _confirm_temporary_git_identities(identities, witnesses):
    for path, previous in identities.items():
        if path.name in ("HEAD", "plugins.sync.lock"):
            current = _FILES["identity"](path.lstat())
        else:
            info = _real_directory(path)
            if _FILES["identity"](info) != witnesses[path]:
                raise ValueError("Codex temporary Git directory changed during inspection")
            current = (info.st_dev, info.st_ino, info.st_mode,
                       getattr(info, "st_file_attributes", None))
        if current != previous:
            raise ValueError("Codex temporary Git identity changed during inspection")


def _repeat_temporary_git_only(profile, before, after, previous):
    """Accept only newly observed empty native Git HEADs; retain raw differences."""
    old_files, old_identities = previous
    files, identities = _temporary_git_state(profile, phase="repeat-comparison")
    lock = profile / ".tmp" / "plugins.sync.lock"
    if (lock in old_identities) != (lock in identities):
        return False
    if any(identities.get(path) != value for path, value in old_identities.items()):
        return False
    if any(before.get(path) != digest or after.get(path) != digest
           for path, digest in old_files.items()):
        return False
    if any(after.get(path) != digest for path, digest in files.items()):
        return False
    changed = {path for path in before.keys() | after.keys() if before.get(path) != after.get(path)}
    return bool(changed) and all(path not in before and path not in old_files
                                and path in files and after[path] == files[path] for path in changed)


def _runtime_files(root):
    _real_directory(root)
    names = []
    with os.scandir(root) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > 3 * len(DATABASES):
                raise ValueError("Codex SQLite runtime member count is invalid")
    names.sort()
    if not names or len(names) > 3 * len(DATABASES):
        raise ValueError("Codex SQLite runtime member count is invalid")
    bodies, total = {}, 0
    for name in names:
        database = next((db for db in DATABASES if name in (db, db + "-wal", db + "-shm")), None)
        if database is None or database not in names:
            raise ValueError("Codex SQLite runtime contains an unexpected member")
        body = _regular_bytes(root / name, min(FILE_LIMIT, TOTAL_LIMIT - total))
        bodies[name] = body
        total += len(body)
    return bodies


def _cell(value):
    return {"bytes": value.hex()} if isinstance(value, bytes) else value


def _table_digest(connection, name):
    quoted = '"' + name.replace('"', '""') + '"'
    encoded, size = [], 0
    for row in connection.execute("SELECT * FROM " + quoted):
        value = json.dumps([_cell(cell) for cell in row], sort_keys=True, separators=(",", ":"))
        encoded.append(value)
        size += len(value)
        if len(encoded) > ROW_LIMIT or size > TOTAL_LIMIT:
            raise ValueError("Codex SQLite table exceeds the semantic audit bound")
    digest = hashlib.sha256()
    for value in sorted(encoded):
        digest.update(value.encode("utf-8") + b"\n")
    return {"rows": len(encoded), "sha256": digest.hexdigest()}


def _database_state(path, expected=None):
    started = time.monotonic()
    connection = sqlite3.connect(path.as_uri() + "?mode=ro", uri=True)
    try:
        connection.setlimit(sqlite3.SQLITE_LIMIT_LENGTH, FILE_LIMIT)
        connection.set_progress_handler(lambda: int(time.monotonic() - started > 5), 1000)
        connection.execute("PRAGMA query_only=ON")
        connection.execute("PRAGMA trusted_schema=OFF")
        schema = connection.execute(
            "SELECT type,name,tbl_name,sql FROM sqlite_schema ORDER BY type,name").fetchall()
        if not schema or expected is not None and schema != expected["schema"]:
            raise ValueError("Codex SQLite runtime schema changed")
        if connection.execute("PRAGMA integrity_check").fetchall() != [("ok",)]:
            raise ValueError("Codex SQLite runtime failed integrity_check")
        tables = {name: _table_digest(connection, name)
                  for kind, name, _, sql in schema if kind == "table" and sql is not None}
        metadata = {name: connection.execute("PRAGMA " + name).fetchone()[0]
                    for name in ("user_version", "application_id")}
        result = {"schema": schema, "tables": tables, "metadata": metadata}
        if expected is not None and result != expected:
            raise ValueError("Codex SQLite runtime table contents changed")
        return result
    finally:
        connection.close()


def runtime_state(root, temporary, baseline=None):
    """Fully read live bytes; interpret WAL only in a private, disposable copy."""
    original_root = _real_directory(root)
    bodies = _runtime_files(root)
    databases = {name for name in bodies if name in DATABASES}
    if baseline is not None and databases != set(baseline):
        raise ValueError("Codex SQLite database inventory changed")
    with tempfile.TemporaryDirectory(prefix="codex-sqlite-proof-", dir=temporary) as scratch:
        copy = Path(scratch)
        for name, body in bodies.items():
            with (copy / name).open("xb") as stream:
                stream.write(body)
        result = {name: _database_state(copy / name, baseline[name] if baseline else None)
                  for name in sorted(databases)}
    if (not os.path.samestat(original_root, _real_directory(root))
            or _runtime_files(root) != bodies):
        raise ValueError("Codex SQLite runtime changed while its copy was inspected")
    files = {name: {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}
             for name, body in bodies.items()}
    return result, files


async def _drain(reader, limit):
    parts, size = [], 0
    while block := await reader.read(65536):
        size += len(block)
        if size > limit:
            raise ValueError("Codex native discovery output exceeds audit bound")
        parts.append(block)
    return b"".join(parts)


def _windows_active(job):
    counts = job.Accounting()
    job.check(job.api.QueryInformationJobObject(
        job.handle, 1, job.C.byref(counts), job.C.sizeof(counts), None))
    return counts.active


def _posix_group_active(pid):
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False


async def _start_native(binary, project, environment):
    windows = os.name == "nt"
    argv = ([sys.executable, str(Path(__file__).resolve()), "--windows-native-start", str(binary)]
            if windows else [str(binary), "app-server"])
    options = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if windows else {"start_new_session": True}
    process = await asyncio.create_subprocess_exec(
        *argv, cwd=project, env=environment,
        stdin=asyncio.subprocess.PIPE, stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE, limit=2 << 20, **options)
    job = None
    try:
        if windows:
            # Reuse the tested job/accounting owner. The bootstrap cannot start
            # npm/native descendants before this assignment and one-byte gate.
            helpers = run_path(str(Path(__file__).parent / "e2e/hermes-noauth-tool-call.py"))
            owner = process._transport.get_extra_info("subprocess")
            job = helpers["WindowsJob"](owner)
            process.stdin.write(b"!")
            await process.stdin.drain()
        return process, job
    except BaseException:
        process.stdin.close()
        try:
            if job is not None:
                await asyncio.to_thread(job.close)
            elif process.returncode is None:
                process.kill()  # The unreleased bootstrap has no native descendants.
        finally:
            try:
                await asyncio.wait_for(process.wait(), 5)
            finally:
                await _close_native_transport(process)
        raise


async def _close_native_transport(process):
    # CPython Process has no public close API. Its owned subprocess transport
    # closes all three pipe transports; one loop turn runs their close callbacks.
    # Use this private adapter only after the bounded process cleanup/reap.
    process._transport.close()
    await asyncio.sleep(0)


async def _finish_native(process, job, completed, readers, diagnostic=None):
    @contextmanager
    def operation(phase):
        try:
            yield
        except BaseException:
            if diagnostic is not None:
                diagnostic["phase"] = phase
            raise

    try:
        try:
            with operation("stdin-close"):
                process.stdin.close()
        finally:
            if job is not None:
                with operation("job-close"):
                    await asyncio.to_thread(job.close)
            elif not completed:
                with operation("group-kill"):
                    try:
                        os.killpg(process.pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
            with operation("process-wait"):
                await asyncio.wait_for(process.wait(), 5)
    finally:
        try:
            with operation("reader-drain"):
                for reader in readers:
                    if not reader.done():
                        reader.cancel()
                await asyncio.gather(*readers, return_exceptions=True)
        finally:
            with operation("transport-close"):
                await _close_native_transport(process)


async def _exchange(binary, project, profile, environment, *,
                    versions=("0.153.4",), with_metadata=False):
    process, job = await _start_native(binary, project, environment)
    errors = asyncio.create_task(_drain(process.stderr, 1 << 20))
    readers, completed = [errors], False
    transcript, total = [], 0

    async def send(message):
        process.stdin.write((json.dumps(message) + "\n").encode())
        await process.stdin.drain()

    async def response(identity):
        nonlocal total
        for _ in range(64):
            line = await process.stdout.readline()
            total += len(line)
            if not line or total > 4 << 20:
                raise ValueError("Codex native discovery ended or exceeded output bound")
            message = json.loads(line)
            transcript.append(message)
            if message.get("id") == identity:
                if "error" in message or not isinstance(message.get("result"), dict):
                    raise ValueError("Codex native discovery returned an error")
                return message["result"]
            if "id" in message or not isinstance(message.get("method"), str):
                raise ValueError("Codex native discovery returned an unexpected response")
        raise ValueError("Codex native discovery notification bound exceeded")

    phase = "initialize-send"
    try:
        async with asyncio.timeout(15):
            await send({"id": 1, "method": "initialize", "params": {
                "clientInfo": {"name": "agentplugins", "version": "1"},
                "capabilities": {"experimentalApi": True}}})
            phase = "initialize-response"
            initialized = await response(1)
            phase = "initialize-check"
            if not _path_equal(initialized.get("codexHome"), profile):
                raise ValueError("Codex native discovery initialized another profile")
            native_version = next((version for version in versions
                if str(initialized.get("userAgent", "")).startswith("agentplugins/" + version + " ")), None)
            if native_version is None:
                raise ValueError("Codex runtime audit requires its pinned native version")
            phase = "skills-send"
            await send({"method": "initialized"})
            await send({"id": 2, "method": "skills/list", "params": {
                "cwds": [str(project)], "forceReload": True}})
            phase = "skills-response"
            listing = await response(2)
            phase = "stdin-close"
            process.stdin.close()
            await process.stdin.wait_closed()
            tail = asyncio.create_task(_drain(process.stdout, (4 << 20) - total))
            readers.append(tail)
            phase = "process-wait"
            await process.wait()
            phase = "reader-drain"
            await tail
            stderr = await errors
            phase = "exit-check"
            if process.returncode != 0 or stderr:
                raise ValueError(
                    "Codex native discovery did not exit cleanly: "
                    f"exit_code={process.returncode}, stderr_bytes={len(stderr)}, "
                    f"stderr_prefix={stderr[:2048]!r}")
            phase = "group-check"
            while _windows_active(job) if job is not None else _posix_group_active(process.pid):
                await asyncio.sleep(.02)  # Still inside the same finite 15-second budget.
            completed = True
            return (native_version, listing) if with_metadata else listing
    except TimeoutError as error:
        # Preserve the strict budget and native process ownership requirements.
        # Expose only bounded failure evidence before the existing owned cleanup.
        raise TimeoutError(
            f"Codex native discovery timed out: phase={phase}; "
            f"launcher_exit_code={process.returncode!r}") from error
    finally:
        failure = sys.exception()
        diagnostic = {}
        try:
            await _finish_native(process, job, completed, readers, diagnostic)
        except Exception as cleanup:
            if failure is not None:
                raise RuntimeError(f"Codex discovery failed (phase={phase}; {failure}); "
                                   f"owned cleanup failed (phase={diagnostic.get('phase', 'unknown')}; {cleanup})") from failure
            raise


def _path_equal(value, expected):
    if not isinstance(value, str) or not Path(value).is_absolute():
        return False
    path = Path(value)
    try:
        # Real Windows paths may have extended-length spelling. Directory/file
        # identity admits that form without accepting redirected aliases.
        return (not any(member.is_symlink() or member.is_junction()
                        for member in (path, *path.parents))
                and os.path.normcase(path.name) == os.path.normcase(expected.name)
                and path.parent.samefile(expected.parent)
                and path.samefile(expected))
    except (OSError, ValueError):
        return False


def _skill_inventory(listing, project):
    rows = listing.get("data")
    if not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict):
        raise ValueError("Codex native discovery returned ambiguous projects")
    row = rows[0]
    if not _path_equal(row.get("cwd"), project) or row.get("errors") not in (None, []):
        raise ValueError("Codex native discovery returned another project or errors")
    skills = row.get("skills")
    if not isinstance(skills, list) or any(not isinstance(skill, dict) for skill in skills):
        raise ValueError("Codex native Skill inventory is malformed")
    return skills


def verify_skill_inventory(listing, project, identity, selected, foreign, present, system=None):
    skills = _skill_inventory(listing, project)
    for name, expected, required in (("foreign-neighbor", foreign, True), (identity, selected, present)):
        matched = [skill for skill in skills if skill.get("name") == name]
        if name == identity and system is not None:
            system_encoded = json.dumps(system, sort_keys=True, allow_nan=False)
            observed = [(skill, json.dumps(skill, sort_keys=True, allow_nan=False))
                        for skill in matched]
            original = [skill for skill, encoded in observed if encoded == system_encoded]
            # A selected project Skill may shadow the System entry. Its original
            # file remains guarded, and absence must restore the exact typed entry.
            if len(original) > 1 or (not present and len(original) != 1):
                raise ValueError("Codex original System Skill inventory changed")
            matched = [skill for skill, encoded in observed if encoded != system_encoded]
        if not required:
            if matched:
                raise ValueError("removed Codex Skill is still natively discoverable")
            continue
        if (len(matched) != 1 or not _path_equal(matched[0].get("path"), expected)
                or matched[0].get("enabled") is not True or matched[0].get("scope") != "repo"
                or matched[0].get("pluginId") is not None):
            raise ValueError("Codex native Skill identity, path or enablement is unqualified")


def public_skill_ready(added, matches, identity):
    if not isinstance(added, dict) or not isinstance(matches, list) or len(matches) != 1:
        return False
    if (added.get("schema_version") != 1 or added.get("command") != "skills add"
            or added.get("result") != "success" or added.get("status") != "succeeded"
            or added.get("scope") != "project" or added.get("readiness") != "ready"
            or added.get("next_actions") not in (None, [])):
        return False
    items = added.get("items")
    if (not isinstance(items, list) or len(items) != 1 or not isinstance(items[0], dict)
            or items[0].get("alias") != identity or items[0].get("targetId") != "codex"
            or items[0].get("status") not in ("succeeded", "unchanged")):
        return False
    clients = matches[0].get("clients") if isinstance(matches[0], dict) else None
    if not isinstance(clients, list) or any(not isinstance(client, dict) for client in clients):
        return False
    selected = [client for client in clients if client.get("client_id") == "codex"]
    return len(selected) == 1 and all(selected[0].get(key) == value for key, value in {
        "scope": "project", "materialization": "materialized", "activation": "active",
        "verification": "installation_verified", "authentication": "not_required", "policy": "allowed",
    }.items())



class CodexPluginSQLiteAudit:
    """Forward plugin measurement: owned SQLite root, unchanged semantic state.

    Plugin registration and removal remain independently checked by the caller.
    Native-profile paths are never excluded from its generic scanner.
    """
    def __init__(self, binary, case, home, project, environment):
        self.case, self.project = case, project
        self.profile, self.temporary = home / ".codex", case / "tmp"
        self.runtime = home / ".local/state/codex-sqlite"
        self.config = self.profile / "config.toml"
        self.pin = str(self.runtime)
        for directory in (home / ".local", home / ".local/state", self.runtime):
            _real_directory(directory.parent)
            if directory == self.runtime or not directory.exists():
                directory.mkdir(mode=0o700, exist_ok=False)
            _real_directory(directory)
        paths = {case, home, project, self.profile, self.temporary, self.runtime}
        paths.update(self.runtime.parents[:3])
        self.anchors = {path: _real_directory(path) for path in paths}
        body = ("sqlite_home = " + json.dumps(self.pin) + "\n").encode()
        with self.config.open("xb") as stream:
            stream.write(body)
        initial_config = _FILES["identity"](self.config.lstat())
        self.guard()
        native_env = {key: value for key, value in environment.items()
                      if key.upper() in {"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR",
                                         "TEMP", "TMP", "TMPDIR"}}
        native_env["CODEX_HOME"] = str(self.profile)
        self.native_version, listing = asyncio.run(_exchange(
            Path(binary), project, self.profile, native_env,
            versions=("0.153.4", "0.162.0"), with_metadata=True))
        self.guard()
        rows = listing.get("data")
        if (not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict)
                or not _path_equal(rows[0].get("cwd"), project)
                or rows[0].get("errors") not in (None, [])
                or not isinstance(rows[0].get("skills"), list)):
            raise ValueError("Codex preinstall inventory is not the selected context")
        # The preinstall profile must still contain only the authored pin.
        if (_regular_bytes(self.config, 65536) != body
                or _FILES["identity"](self.config.lstat()) != initial_config):
            raise ValueError("Codex preinstall changed the authored config")
        self.baseline, files = runtime_state(self.runtime, self.temporary)
        self.primary_anchors = {name: (self.runtime / name).lstat() for name in self.baseline}
        self.guard()
        self.observations = [{"phase": "before-install", "runtime_files_fully_read": files}]

    def guard(self):
        for path, original in self.anchors.items():
            if not os.path.samestat(original, _real_directory(path)):
                raise ValueError("Codex plugin SQLite authority root changed")
        for name, original in getattr(self, "primary_anchors", {}).items():
            if not os.path.samestat(original, (self.runtime / name).lstat()):
                raise ValueError("Codex plugin SQLite primary database identity changed")
        config = tomllib.loads(_regular_bytes(self.config, 65536).decode("utf-8"))
        if config.get("sqlite_home") != self.pin:
            raise ValueError("Codex plugin SQLite config authority changed")

    def checkpoint(self, phase):
        self.guard()
        _, files = runtime_state(self.runtime, self.temporary, self.baseline)
        self.guard()
        self.observations.append({"phase": phase, "runtime_files_fully_read": files})

    def evidence(self):
        return {"native_version": self.native_version,
                "runtime_root": str(self.runtime.relative_to(self.case)),
                "runtime_semantics_unchanged": True, "runtime_baseline": self.baseline,
                "observations": self.observations}


class CodexRuntimeAudit:
    def __init__(self, binary, case, home, project, environment, identity, foreign):
        self.binary, self.case, self.project = Path(binary), case, project
        self.profile, self.temporary = home / ".codex", case / "tmp"
        self.runtime = home / ".local/state/codex-sqlite"
        self.config = self.profile / "config.toml"
        self.identity, self.foreign = identity, foreign
        self.selected = project / ".agents/skills" / identity / "SKILL.md"
        self.environment = {key: value for key, value in environment.items()
                            if key.upper() in {"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR",
                                               "TEMP", "TMP", "TMPDIR"}}
        self.environment["CODEX_HOME"] = str(self.profile)
        for directory in (home / ".local", home / ".local/state", self.runtime):
            _real_directory(directory.parent)
            if directory == self.runtime or not directory.exists():
                directory.mkdir(exist_ok=False)
            _real_directory(directory)
        paths = {case, home, self.profile, project, self.temporary, self.runtime}
        paths.update(self.runtime.parents[:3])
        self.anchors = {path: _real_directory(path) for path in paths}
        self.config_bytes = ("sqlite_home = " + json.dumps(str(self.runtime)) + "\n").encode()
        with self.config.open("xb") as stream:
            stream.write(self.config_bytes)
        self.foreign_bytes = _regular_bytes(foreign, FILE_LIMIT)
        self.baseline, self.observations, self.system_skill = None, [], None
        self.checkpoint("before-install", False)

    def _system_baseline(self, listing):
        skills = _skill_inventory(listing, self.project)
        matched = [skill for skill in skills if skill.get("name") == self.identity
                   and skill.get("scope") == "system"]
        if not matched:
            return None
        path = self.profile / "skills/.system" / self.identity / "SKILL.md"
        if (len(matched) != 1 or Path(self.identity).name != self.identity
                or self.identity in ("", ".", "..")
                or matched[0].get("enabled") is not True
                or matched[0].get("pluginId") is not None
                or not _path_equal(matched[0].get("path"), path)):
            raise ValueError("Codex System Skill is outside its original native namespace")
        directories = (self.profile / "skills", self.profile / "skills/.system", path.parent)
        anchors = {directory: _FILES["identity"](_real_directory(directory))
                   for directory in directories}
        before = _FILES["identity"](path.lstat())
        body = _regular_bytes(path, FILE_LIMIT)
        baseline = {"entry": dict(matched[0]), "path": path, "identity": before,
                    "bytes": body, "anchors": anchors}
        self._guard_system(baseline)
        return baseline

    def _guard_system(self, baseline):
        if baseline is None:
            return
        for directory, original in baseline["anchors"].items():
            if _FILES["identity"](_real_directory(directory)) != original:
                raise ValueError("Codex original System Skill directory changed")
        path = baseline["path"]
        if (_FILES["identity"](path.lstat()) != baseline["identity"]
                or _regular_bytes(path, FILE_LIMIT) != baseline["bytes"]):
            raise ValueError("Codex original System Skill file changed")

    def _inventory_diagnostic(self, listing, phase):
        rows = listing.get("data") if isinstance(listing, dict) else None
        skills = rows[0].get("skills") if (isinstance(rows, list) and len(rows) == 1
                                           and isinstance(rows[0], dict)) else None
        matched = [skill for skill in skills if isinstance(skill, dict)
                   and skill.get("name") == self.identity] if isinstance(skills, list) else []
        facts = []
        for skill in matched[:3]:
            label = "outside-owned-roots"
            raw = skill.get("path")
            if isinstance(raw, str) and Path(raw).is_absolute() and ".." not in Path(raw).parts:
                for root, prefix in ((self.profile, "profile/"), (self.project, "project/")):
                    if Path(raw).is_relative_to(root):
                        label = prefix + str(Path(raw).relative_to(root))
                        break
            facts.append({"scope": str(skill.get("scope"))[:16], "path": label[:160],
                          "enabled": skill.get("enabled") is True,
                          "plugin_absent": skill.get("pluginId") is None})
        return (f"checkpoint={ascii(phase[:32])}; matching_count={len(matched)}; "
                f"matching={json.dumps(facts, ensure_ascii=True, separators=(',', ':'))}")

    def guard(self):
        for path, previous in self.anchors.items():
            if not os.path.samestat(previous, _real_directory(path)):
                raise ValueError("Codex runtime audit root identity changed")
        if _regular_bytes(self.config, 65536) != self.config_bytes:
            raise ValueError("Codex native runtime configuration changed")
        if _regular_bytes(self.foreign, FILE_LIMIT) != self.foreign_bytes:
            raise ValueError("Codex foreign Skill bytes changed")
        self._guard_system(self.system_skill)

    def checkpoint(self, phase, present):
        self.guard()
        selected_bytes = _regular_bytes(self.selected, FILE_LIMIT) if present else None
        listing = asyncio.run(_exchange(self.binary, self.project, self.profile, self.environment))
        self.guard()
        if present and _regular_bytes(self.selected, FILE_LIMIT) != selected_bytes:
            raise ValueError("Codex selected Skill bytes changed during native discovery")
        try:
            system = self._system_baseline(listing) if self.baseline is None else self.system_skill
            verify_skill_inventory(listing, self.project, self.identity, self.selected,
                                   self.foreign, present, system["entry"] if system else None)
        except (OSError, ValueError) as error:
            raise ValueError("Codex native Skill inventory is unqualified; "
                             + self._inventory_diagnostic(listing, phase)) from error
        state, files = runtime_state(self.runtime, self.temporary, self.baseline)
        git_files, _ = _temporary_git_state(self.profile, phase=phase)
        self.guard()
        self._guard_system(system)
        if self.baseline is None:
            self.baseline = state
            self.system_skill = system
        self.observations.append({"phase": phase, "selected_skill_present": present,
                                  "foreign_skill_present": True, "native_exit_code": 0,
                                  "runtime_integrity_verified": True, "runtime_semantics_unchanged": True,
                                  "runtime_files_fully_read": files,
                                  "temporary_git_files_fully_read": git_files})
        return True

    def temporary_git_state(self):
        self.guard()
        state = _temporary_git_state(self.profile, phase="before-repeat")
        self.guard()
        return state

    def repeat_runtime_only(self, before, after, previous):
        self.guard()
        qualified = _repeat_temporary_git_only(self.profile, before, after, previous)
        self.guard()
        return qualified

    def evidence(self):
        self.guard()
        return {"native_version": "0.153.4", "runtime_root": str(self.runtime.relative_to(self.case)),
                "runtime_baseline": self.baseline, "observations": self.observations,
                "system_skill_baseline": ({
                    "path": str(self.system_skill["path"].relative_to(self.case)),
                    "bytes": len(self.system_skill["bytes"]),
                    "sha256": hashlib.sha256(self.system_skill["bytes"]).hexdigest(),
                    "scope": "system", "unchanged": True,
                } if self.system_skill is not None else None)}


if __name__ == "__main__":
    if os.name != "nt" or len(sys.argv) != 3 or sys.argv[1] != "--windows-native-start":
        raise SystemExit("private Windows native observer bootstrap required")
    # Exactly one unbuffered byte: no JSON protocol input can be consumed here.
    if os.read(0, 1) != b"!":
        raise SystemExit("native observer process ownership was not established")
    raise SystemExit(subprocess.call([sys.argv[2], "app-server"], stdin=sys.stdin,
                                    stdout=sys.stdout, stderr=sys.stderr))
