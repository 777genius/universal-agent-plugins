"""Bounded Qoder 1.1.81 Project Skill observations, never native qualification.

The official npm entrypoint sources support ordinary-cwd ``skills list``.
Its initialization can write runtime state; every observed delta stays unqualified.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import stat


PINS = {
    "package.json": (1177, "efca34ac491934199f468649a18d3d2b34ffcf0a2cabc510ddfaca9aa0512418"),
    "bundle/qoder-npm-dispatcher.cjs": (8262, "37cc389f07b046d78a2c80ff8d9ab54f433d38f94bf1f221a9e7e1dea297d120"),
    "bundle/qodercli.js": (35653383, "dcabbcaece1c0683ee4929ac7c0cf0ea00bbea4d319f5abeaae8216088b9888f"),
}
ARCHIVE_SHA256 = "be9358de5600e3a93925709cb0e7e979b38e67dcde5d028128bfd0b16892f357"
PHASES = ("before", "add", "repeat", "remove")
STREAM_LIMIT = 1 << 20
ENTRY_LIMIT, FILE_LIMIT, TOTAL_LIMIT = 4096, 8 << 20, 64 << 20


class DiagnosticBudget(ValueError):
    pass


class QoderDiagnosticCustodyError(ValueError):
    pass


class QoderSkillDiagnostic:
    def __init__(self, case_root, home, project, data, executable, version, *,
                 artifacts, run_probe, guard, env):
        self.case, self.home, self.project, self.data = map(Path, (case_root, home, project, data))
        self.executable = Path(executable) if executable else None
        self.artifacts, self.run_probe, self.guard_case, self.env = artifacts, run_probe, guard, env
        self.descriptors, self.pin_files, self.parent_facts, self.bound_dirs = [], [], {}, []
        self.last_phase = -1
        self.capture_stopped = False
        self.fact = {
            "schema_version": 1, "diagnostic_only": True,
            "native_discovery_qualified": False, "project_lifecycle_qualified": False,
            "native_session_restart_qualified": False, "authentication_qualified": False,
            "model_execution_qualified": False, "mcp_runtime_qualified": False,
            "hook_runtime_qualified": False,
            "case_root": str(self.case), "project": str(self.project), "home": str(self.home),
            "reported_client_version": version, "observations": [], "closed": False,
            "source_archive": {"url": "https://registry.npmjs.org/@qoder-ai/qodercli/-/qodercli-1.1.81.tgz",
                               "sha256": ARCHIVE_SHA256, "bytes": 31942266},
            "provenance_scope": "three npm entrypoint files only; interpreter and transitive dependencies are not attested",
            "filesystem_scope": "bounded disposable case only; no escaped-writer or network isolation claim",
            "execution_scope": "executed means the probe returned an integer CLI process exit; no Skill or model execution is claimed",
            "limits": {"entries": ENTRY_LIMIT, "file_bytes": FILE_LIMIT,
                       "total_file_bytes": TOTAL_LIMIT, "stream_bytes": STREAM_LIMIT, "timeout_seconds": 30},
        }
        self.reason = None
        self.case_fd = None
        try:
            if (type(version) is not dict or version.get("status") != "reported"
                    or version.get("value") != "1.1.81"
                    or type(version.get("exit_code")) is not int or version["exit_code"] != 0):
                raise ValueError("reported Qoder version is not the supported 1.1.81")
            self.guard_case()
            if self.case.resolve(strict=True) != self.case:
                raise ValueError("Qoder diagnostic case is not canonical")
            if (self.env.get("HOME") != str(self.home)
                    or self.env.get("QODER_CONFIG_DIR") != str(self.home / ".qoder")
                    or self.env.get("AGENTPLUGINS_HOME") != str(self.data)
                    or any(self.env.get(key) != str(self.case / "tmp") for key in ("TMPDIR", "TMP", "TEMP"))):
                raise ValueError("Qoder diagnostic environment does not select the disposable case")
            self.case_fd = self._open(self.case, directory=True)
            self.case_identity = self._directory_identity(os.fstat(self.case_fd))
            for path in (self.home, self.project, self.data, self.case / "tmp",
                         self.home / ".qoder", self.project / ".qoder", self.project / ".qoder/skills"):
                if not path.is_relative_to(self.case) or path.resolve(strict=True) != path:
                    raise ValueError("Qoder diagnostic authority is not a canonical case directory")
                fd = self._open(path, directory=True)
                self.bound_dirs.append((path, fd, self._directory_identity(os.fstat(fd))))
            self.fact["authority_directories"] = [
                {"path": str(path), "identity": identity} for path, _, identity in self.bound_dirs]
            self._pin_entrypoints()
            self.fact["entrypoint_pins_verified"] = True
        except (OSError, ValueError) as error:
            self.reason = str(error)[:512]
            self.fact.update(entrypoint_pins_verified=False, prerequisite_error=self.reason)

    def _open(self, path, *, directory=False, parent=None):
        flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
        if directory:
            flags |= os.O_DIRECTORY
        fd = os.open(path, flags, dir_fd=parent)
        self.descriptors.append(fd)
        return fd

    def _identity(self, info):
        return (*self.artifacts["identity"](info), info.st_uid, info.st_gid)

    @staticmethod
    def _directory_identity(info):
        return info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid

    def _pin_entrypoints(self):
        executable = self.executable
        if (executable is None or not executable.is_absolute()
                or executable.name not in ("qoder-npm-dispatcher.cjs", "qodercli.js")
                or executable.parent.name != "bundle"
                or executable.resolve(strict=True) != executable):
            raise ValueError("Qoder diagnostic requires the exact npm entrypoint")
        package = executable.parent.parent
        for path in (package, executable.parent, *package.parents):
            info = path.lstat()
            if not stat.S_ISDIR(info.st_mode):
                raise ValueError("Qoder entrypoint ancestor is not an ordinary directory")
            self.parent_facts[path] = self._directory_identity(info)
        package_fd = self._open(package, directory=True)
        bundle_fd = self._open("bundle", directory=True, parent=package_fd)
        entries = []
        for relative, (size, digest) in PINS.items():
            path = package / relative
            fd = self._open(Path(relative).name, parent=bundle_fd if "/" in relative else package_fd)
            info = os.fstat(fd)
            if (not stat.S_ISREG(info.st_mode) or info.st_nlink != 1
                    or info.st_mode & 0o022 or info.st_size != size or size > 40 << 20):
                raise ValueError("Qoder entrypoint file shape differs from the fixed source pin")
            sha, remaining = hashlib.sha256(), size + 1
            while remaining:
                chunk = os.read(fd, min(1 << 20, remaining))
                if not chunk:
                    break
                sha.update(chunk)
                remaining -= len(chunk)
            identity = self._identity(info)
            if (remaining != 1 or sha.hexdigest() != digest
                    or identity != self._identity(os.fstat(fd))
                    or identity != self._identity(path.lstat())):
                raise ValueError("Qoder entrypoint bytes or custody differ from the fixed source pin")
            self.pin_files.append((path, fd, identity))
            entries.append({"path": str(path), "bytes": size, "sha256": digest, "identity": identity})
        self.fact["entrypoint_files"] = entries
        self.fact["selected_executable"] = str(executable)

    def _guard(self):
        self.guard_case()
        if (self.case.resolve(strict=True) != self.case
                or self._directory_identity(self.case.lstat()) != self.case_identity
                or self._directory_identity(os.fstat(self.case_fd)) != self.case_identity):
            raise ValueError("Qoder diagnostic case directory was replaced")
        for path, identity in self.parent_facts.items():
            if self._directory_identity(path.lstat()) != identity:
                raise ValueError("Qoder entrypoint ancestor changed")
        for path, fd, identity in self.bound_dirs:
            if (path.resolve(strict=True) != path
                    or self._directory_identity(path.lstat()) != identity
                    or self._directory_identity(os.fstat(fd)) != identity):
                raise ValueError("Qoder held project/profile directory changed")
        for path, fd, identity in self.pin_files:
            if (self._identity(path.lstat()) != identity
                    or self._identity(os.fstat(fd)) != identity):
                raise ValueError("Qoder held entrypoint changed")

    def _frame(self):
        """Read a closed, bounded case tree through held no-follow directory fds."""
        frame, total = {}, 0

        def walk(directory, prefix, depth):
            nonlocal total
            if depth > 64:
                raise DiagnosticBudget("Qoder diagnostic tree depth exceeds budget")
            frame[prefix or "."] = ("directory", *self._identity(os.fstat(directory)))
            os.lseek(directory, 0, os.SEEK_SET)
            with os.scandir(directory) as entries:
                for entry in entries:
                    name = prefix + "/" + entry.name if prefix else entry.name
                    if len(frame) >= ENTRY_LIMIT or len(name.encode("utf-8")) > 4096:
                        raise DiagnosticBudget("Qoder diagnostic entry budget exceeded")
                    info = os.stat(entry.name, dir_fd=directory, follow_symlinks=False)
                    if stat.S_ISLNK(info.st_mode):
                        target = os.readlink(entry.name, dir_fd=directory)
                        if len(target.encode("utf-8")) > 4096:
                            raise DiagnosticBudget("Qoder diagnostic link budget exceeded")
                        if self._identity(info) != self._identity(os.stat(
                                entry.name, dir_fd=directory, follow_symlinks=False)):
                            raise ValueError("Qoder diagnostic link changed during read")
                        frame[name] = ("symlink", *self._identity(info), target)
                        continue
                    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
                    if stat.S_ISDIR(info.st_mode):
                        flags |= os.O_DIRECTORY
                    elif not stat.S_ISREG(info.st_mode):
                        raise ValueError("Qoder diagnostic tree contains a special file")
                    fd = os.open(entry.name, flags, dir_fd=directory)
                    try:
                        if self._identity(info) != self._identity(os.fstat(fd)):
                            raise ValueError("Qoder diagnostic member changed during open")
                        if stat.S_ISDIR(info.st_mode):
                            walk(fd, name, depth + 1)
                        else:
                            total += info.st_size
                            if info.st_size > FILE_LIMIT or total > TOTAL_LIMIT:
                                raise DiagnosticBudget("Qoder diagnostic byte budget exceeded")
                            sha, remaining = hashlib.sha256(), info.st_size + 1
                            while remaining:
                                chunk = os.read(fd, min(1 << 20, remaining))
                                if not chunk:
                                    break
                                sha.update(chunk)
                                remaining -= len(chunk)
                            if remaining != 1:
                                raise ValueError("Qoder diagnostic member length changed")
                            frame[name] = ("file", *self._identity(info), sha.hexdigest())
                        if (self._identity(info) != self._identity(os.fstat(fd))
                                or self._identity(info) != self._identity(os.stat(
                                    entry.name, dir_fd=directory, follow_symlinks=False))):
                            raise ValueError("Qoder diagnostic member changed during read")
                    finally:
                        os.close(fd)
        walk(self.case_fd, "", 0)
        return frame

    def observe(self, phase):
        if phase not in PHASES or PHASES.index(phase) <= self.last_phase:
            raise ValueError("Qoder diagnostic phase order differs")
        self.last_phase = PHASES.index(phase)
        observation = {"phase": phase, "status": "unqualified", "capture_complete": False,
                       "probe_invoked": False, "executed": False}
        self.fact["observations"].append(observation)
        if self.reason:
            observation.update(status="unexecuted-prerequisites", reason=self.reason)
            return
        if self.capture_stopped:
            observation.update(status="unexecuted-after-incomplete-capture")
            return
        try:
            self._guard()
            before = self._frame()
            if phase == "before":
                self.fact["original_authority"] = before
            argv = [str(self.executable), "--config-dir", str(self.home / ".qoder"), "skills", "list"]
            label = "qoder-skill-diagnostic-" + phase
            observation["probe_invoked"] = True
            record, stdout, stderr = self.run_probe(
                argv, self.project, self.env, self.case, label,
                timeout=30, limit=STREAM_LIMIT, memory_only=True)
            if (type(record) is not dict or type(stdout) is not bytes or type(stderr) is not bytes
                    or len(stdout) > STREAM_LIMIT or len(stderr) > STREAM_LIMIT):
                raise ValueError("Qoder diagnostic transport returned unbounded or non-byte capture")
            # These are retained raw bytes, not a parsed/qualified Skill inventory.
            observation.update(record=record, capture_storage="memory-only",
                               executed=type(record.get("exit")) is int,
                               stdout_base64=base64.b64encode(stdout).decode("ascii"),
                               stderr_base64=base64.b64encode(stderr).decode("ascii"))
            valid = (record.get("probe") == label and "error" in record
                     and record.get("argv") == argv and record.get("cwd") == str(self.project)
                     and all(type(body) is bytes and len(body) <= STREAM_LIMIT
                             and type(record.get(key + "_bytes")) is int
                             and record.get(key + "_bytes") == len(body)
                             and record.get(key + "_sha256") == hashlib.sha256(body).hexdigest()
                             for key, body in (("stdout", stdout), ("stderr", stderr))))
            complete = (valid and record.get("error") is None
                        and type(record.get("exit")) is int and record["exit"] >= 0)
            observation.update(capture_record_consistent=valid, capture_complete=complete,
                               stream_scope="complete-observed-streams" if complete else "retained-prefixes")
            # A timed-out/broken transport is already decisive diagnostic data.
            # Do not spend three more full deadlines in the same case.
            self.capture_stopped = not complete
            self._guard()
            after = self._frame()
            changed = sorted(path for path in before.keys() | after.keys() if before.get(path) != after.get(path))
            observation["case_changes"] = {
                "total": len(changed), "omitted": max(0, len(changed) - 100),
                "entries": [{"path": path, "before": before.get(path), "after": after.get(path)}
                            for path in changed[:100]],
                "unchanged": not changed,
                "before_sha256": hashlib.sha256(json.dumps(before, sort_keys=True).encode()).hexdigest(),
                "after_sha256": hashlib.sha256(json.dumps(after, sort_keys=True).encode()).hexdigest(),
            }
            self._guard()
        except DiagnosticBudget as error:
            observation.update(status="unqualified-budget", reason=str(error))
        except (OSError, ValueError, AssertionError) as error:
            observation.update(status="unqualified-custody", reason=str(error)[:512])
            refused = QoderDiagnosticCustodyError(str(error))
            refused.audit_evidence = self.evidence()
            raise refused from error

    def evidence(self):
        observed = {entry["phase"] for entry in self.fact["observations"] if entry["executed"] is True}
        self.fact["unexecuted_phases"] = [phase for phase in PHASES if phase not in observed]
        return self.fact

    def close(self):
        for fd in reversed(self.descriptors):
            os.close(fd)
        self.descriptors.clear()
        self.fact["closed"] = True
