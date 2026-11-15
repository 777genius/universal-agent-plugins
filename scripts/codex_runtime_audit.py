"""Strict native Codex Skill evidence with independently checked SQLite state.

Codex 0.153.4's supported sqlite_home separates runtime databases from native
registration files. No native-profile file is excluded from the caller's raw
snapshot. SQLite accepts a changed WAL representation only when integrity,
schema and every table's contents still match the pre-install native baseline.
"""

import asyncio
import hashlib
import json
import os
from pathlib import Path
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
            await asyncio.wait_for(process.wait(), 5)
        raise


async def _finish_native(process, job, completed, readers):
    try:
        try:
            process.stdin.close()
        finally:
            if job is not None:
                await asyncio.to_thread(job.close)
            elif not completed:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            await asyncio.wait_for(process.wait(), 5)
    finally:
        for reader in readers:
            if not reader.done():
                reader.cancel()
        await asyncio.gather(*readers, return_exceptions=True)


async def _exchange(binary, project, profile, environment):
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

    try:
        async with asyncio.timeout(15):
            await send({"id": 1, "method": "initialize", "params": {
                "clientInfo": {"name": "agentplugins", "version": "1"},
                "capabilities": {"experimentalApi": True}}})
            initialized = await response(1)
            if not _path_equal(initialized.get("codexHome"), profile):
                raise ValueError("Codex native discovery initialized another profile")
            if not str(initialized.get("userAgent", "")).startswith("agentplugins/0.153.4 "):
                raise ValueError("Codex runtime audit requires the qualified native 0.153.4")
            await send({"method": "initialized"})
            await send({"id": 2, "method": "skills/list", "params": {
                "cwds": [str(project)], "forceReload": True}})
            listing = await response(2)
            process.stdin.close()
            await process.stdin.wait_closed()
            tail = asyncio.create_task(_drain(process.stdout, (4 << 20) - total))
            readers.append(tail)
            await process.wait()
            await tail
            stderr = await errors
            if process.returncode != 0 or stderr:
                raise ValueError("Codex native discovery did not exit cleanly")
            while _windows_active(job) if job is not None else _posix_group_active(process.pid):
                await asyncio.sleep(.02)  # Still inside the same finite 15-second budget.
            completed = True
            return listing
    finally:
        failure = sys.exception()
        try:
            await _finish_native(process, job, completed, readers)
        except Exception as cleanup:
            if failure is not None:
                raise RuntimeError(f"Codex discovery failed ({failure}); owned cleanup failed ({cleanup})") from failure
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


def verify_skill_inventory(listing, project, identity, selected, foreign, present):
    rows = listing.get("data")
    if not isinstance(rows, list) or len(rows) != 1 or not isinstance(rows[0], dict):
        raise ValueError("Codex native discovery returned ambiguous projects")
    row = rows[0]
    if not _path_equal(row.get("cwd"), project) or row.get("errors") not in (None, []):
        raise ValueError("Codex native discovery returned another project or errors")
    skills = row.get("skills")
    if not isinstance(skills, list) or any(not isinstance(skill, dict) for skill in skills):
        raise ValueError("Codex native Skill inventory is malformed")
    for name, expected, required in (("foreign-neighbor", foreign, True), (identity, selected, present)):
        matched = [skill for skill in skills if skill.get("name") == name]
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


class CodexRuntimeAudit:
    def __init__(self, binary, case, home, project, environment, identity, foreign, path_in_case):
        self.binary, self.case, self.project = Path(binary), case, project
        self.profile, self.temporary = home / ".codex", case / "tmp"
        self.runtime = home / ".local/state/codex-sqlite"
        self.config = self.profile / "config.toml"
        self.identity, self.foreign = identity, foreign
        self.path_in_case = path_in_case
        self.trust_authorized, self.trust_observed = False, False
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
        self.baseline, self.observations = None, []
        self.checkpoint("before-install", False)

    def begin_install(self):
        """Authorize only the upcoming confirmed project Skill installation."""
        self.guard()
        self.trust_authorized = True

    def _guard_config(self, require_trust):
        body = _regular_bytes(self.config, 65536)
        expected = tomllib.loads(self.config_bytes.decode("utf-8"))
        observed = tomllib.loads(body.decode("utf-8"))
        projects = observed.pop("projects", None)
        if observed != expected:
            raise ValueError("Codex native runtime configuration changed")
        if projects is None:
            if require_trust or self.trust_observed:
                raise ValueError("Codex exact project trust is missing or was revoked")
            if not self.trust_authorized and body != self.config_bytes:
                raise ValueError("Codex native runtime configuration changed before installation")
            return
        if not self.trust_authorized or not isinstance(projects, dict) or len(projects) != 1:
            raise ValueError("Codex project trust changed outside the confirmed installation")
        selected, trust = next(iter(projects.items()))
        if (trust != {"trust_level": "trusted"}
                or not self.path_in_case(Path(selected), self.case)
                or not _path_equal(selected, self.project)):
            raise ValueError("Codex project trust differs from the exact selected project")
        self.trust_observed = True

    def guard(self, require_trust=False):
        for path, previous in self.anchors.items():
            if not os.path.samestat(previous, _real_directory(path)):
                raise ValueError("Codex runtime audit root identity changed")
        self._guard_config(require_trust)
        if _regular_bytes(self.foreign, FILE_LIMIT) != self.foreign_bytes:
            raise ValueError("Codex foreign Skill bytes changed")

    def checkpoint(self, phase, present):
        self.guard(require_trust=present)
        selected_bytes = _regular_bytes(self.selected, FILE_LIMIT) if present else None
        listing = asyncio.run(_exchange(self.binary, self.project, self.profile, self.environment))
        self.guard()
        if present and _regular_bytes(self.selected, FILE_LIMIT) != selected_bytes:
            raise ValueError("Codex selected Skill bytes changed during native discovery")
        verify_skill_inventory(listing, self.project, self.identity, self.selected, self.foreign, present)
        state, files = runtime_state(self.runtime, self.temporary, self.baseline)
        self.guard()
        if self.baseline is None:
            self.baseline = state
        self.observations.append({"phase": phase, "selected_skill_present": present,
                                  "foreign_skill_present": True, "native_exit_code": 0,
                                  "runtime_integrity_verified": True, "runtime_semantics_unchanged": True,
                                  "runtime_files_fully_read": files})
        return True

    def evidence(self):
        return {"native_version": "0.153.4", "runtime_root": str(self.runtime.relative_to(self.case)),
                "runtime_baseline": self.baseline, "observations": self.observations}


if __name__ == "__main__":
    if os.name != "nt" or len(sys.argv) != 3 or sys.argv[1] != "--windows-native-start":
        raise SystemExit("private Windows native observer bootstrap required")
    # Exactly one unbuffered byte: no JSON protocol input can be consumed here.
    if os.read(0, 1) != b"!":
        raise SystemExit("native observer process ownership was not established")
    raise SystemExit(subprocess.call([sys.argv[2], "app-server"], stdin=sys.stdin,
                                    stdout=sys.stdout, stderr=sys.stderr))
