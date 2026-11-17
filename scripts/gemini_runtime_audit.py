"""Bounded Linux observation of empty Gemini 0.66.0 skills-list sessions.

Pinned upstream 970413207732f5d4d8c50540c7fc359c44fdaaf2 initializes the
skills-list-session ID and appends one initial JSONL metadata record without a
conversation. Classify only those new records at an unchanged registry binding;
never exclude chats from snapshots or infer who wrote matching bytes.
"""

from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import stat
import sys
import time

_FILES = run_path(str(Path(__file__).with_name("native_client_project_artifacts.py")))
_NAME = re.compile(r"session-([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2})-(?:[1-9][0-9]{0,5}-)?skills-l\.jsonl\Z")
_TIME = re.compile(r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}Z\Z")


def _json(body):
    def unique(pairs):
        value = {}
        for key, entry in pairs:
            if key in value:
                raise ValueError("duplicate JSON member")
            value[key] = entry
        return value

    def invalid_constant(value):
        raise ValueError("nonfinite JSON value")

    return json.loads(body.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=invalid_constant)


def _directory_identity(info):
    if not stat.S_ISDIR(info.st_mode):
        raise ValueError("Gemini runtime parent is not an ordinary directory")
    # New direct children legitimately change directory times and size.
    return info.st_dev, info.st_ino, info.st_mode


def pinned_version(version):
    return (sys.platform == "linux" and isinstance(version, dict)
            and version.get("status") == "reported" and type(version.get("exit_code")) is int
            and version["exit_code"] == 0 and version.get("value") == "0.66.0")


def _metadata(body, filename, project_hash, started, finished):
    if not body.endswith(b"\n") or body.count(b"\n") != 1:
        raise ValueError("Gemini runtime is not one complete initial record")
    record = _json(body)
    keys = {"sessionId", "projectHash", "startTime", "lastUpdated"}
    if (not isinstance(record, dict) or set(record) not in (keys, keys | {"kind"})
            or record.get("kind", "main") != "main"
            or record.get("sessionId") != "skills-list-session"
            or record.get("projectHash") != project_hash):
        raise ValueError("Gemini runtime record contains non-list or conversation data")
    values = []
    for key in ("startTime", "lastUpdated"):
        value = record[key]
        if not isinstance(value, str) or not _TIME.fullmatch(value):
            raise ValueError("Gemini runtime record time is invalid")
        values.append(datetime.fromisoformat(value.replace("Z", "+00:00")))
    first, last = values
    if not started <= first <= last <= finished:
        raise ValueError("Gemini runtime record time is outside the repeat interval")
    match = _NAME.fullmatch(filename)
    if not match:
        raise ValueError("Gemini runtime record filename is not a list session")
    named = datetime.strptime(match[1], "%Y-%m-%dT%H-%M").replace(tzinfo=timezone.utc)
    # The filename is chosen before metadata, and can cross a minute boundary.
    if not started.replace(second=0, microsecond=0) <= named <= first:
        raise ValueError("Gemini runtime filename time is outside the repeat interval")


class GeminiRepeatAudit:
    """Hold the pre-repeat namespace and verify both actual public phases."""

    def __init__(self, home, project, before, version):
        self.parents, self.leaves, self.files, self.phases = {}, {}, {}, []
        self.reason, self.complete = None, False
        self.before = self.previous = before
        self.profile, self.project = Path(home) / ".gemini", Path(project)
        self.started = datetime.now(timezone.utc)
        self.started = self.started.replace(microsecond=self.started.microsecond // 1000 * 1000)
        try:
            if not pinned_version(version):
                raise ValueError("Gemini empty-session proof requires Linux native 0.66.0")
            self.deadline = time.monotonic() + 5
            self._hold(self.project)
            self._hold(self.profile)
            registry = _json(self._read(self.profile / "projects.json", 65536, before))
            if (not isinstance(registry, dict) or set(registry) != {"projects"}
                    or not isinstance(registry["projects"], dict)):
                raise ValueError("Gemini runtime registry shape is invalid")
            slug = registry["projects"].get(str(self.project))
            if (not isinstance(slug, str) or not re.fullmatch(r"[a-z0-9-]{1,128}", slug)
                    or sum(value == slug for value in registry["projects"].values()) != 1):
                raise ValueError("Gemini runtime registry does not bind the exact project")
            directory = self.profile / "tmp" / slug
            self.chats = directory / "chats"
            self._hold(self.chats)
            marker = self._read(directory / ".project_root", 4096, before)
            if marker != str(self.project).encode("utf-8"):
                raise ValueError("Gemini runtime ownership marker differs")
            self.prefix = f"{self.profile.name}/{self.chats.relative_to(self.profile)}/"
            old = [key for key in before if key.startswith(self.prefix)]
            if len(old) > 64:
                raise ValueError("Gemini pre-repeat chat count exceeds its bound")
            for key in old:
                name = key[len(self.prefix):]
                if not name or "/" in name or name in (".", ".."):
                    raise ValueError("Gemini pre-repeat chat layout is not flat")
                self._open(self.chats / name, 1 << 20)
            self._guard()
        except (OSError, ValueError, RuntimeError, UnicodeError, TypeError) as error:
            self.reason = str(error)[:512]
            self.close()

    def _hold(self, path):
        if not path.is_absolute() or len(path.parts) > 64:
            raise ValueError("Gemini runtime directory depth or root is invalid")
        for member in (*reversed(path.parents), path):
            if member in self.parents:
                continue
            parent = self.parents.get(member.parent)
            fd = os.open(member.name if parent else str(member),
                         os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                         dir_fd=parent[0] if parent else None)
            info = os.fstat(fd)
            self.parents[member] = (fd, _directory_identity(info))
        self._guard()

    def _guard(self):
        if time.monotonic() > self.deadline:
            raise ValueError("Gemini runtime checkpoint deadline")
        for path, (fd, identity) in self.parents.items():
            if (_directory_identity(os.fstat(fd)) != identity
                    or _directory_identity(path.lstat()) != identity):
                raise ValueError("Gemini runtime parent replaced or redirected")
        for path, (fd, identity) in self.leaves.items():
            if (_FILES["identity"](os.fstat(fd)) != identity
                    or _FILES["identity"](path.lstat()) != identity):
                raise ValueError("Gemini runtime previous file changed or replaced")

    def _open(self, path, limit):
        self._guard()
        before = path.lstat()
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or not 0 < before.st_size <= limit or len(self.leaves) >= 82):
            raise ValueError("Gemini runtime file is not bounded regular single-link data")
        fd = os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
                     dir_fd=self.parents[path.parent][0])
        self.leaves[path] = (fd, _FILES["identity"](before))
        if not _FILES["same_opened"](before, os.fstat(fd)):
            raise ValueError("Gemini runtime opened file identity changed")
        return fd, before.st_size

    def _read(self, path, limit, snapshot):
        fd, size = self._open(path, limit)
        body = bytearray()
        while len(body) <= size:
            self._guard()
            block = os.read(fd, min(65536, size + 1 - len(body)))
            if not block:
                break
            body.extend(block)
        self._guard()
        key = f"{self.profile.name}/{path.relative_to(self.profile)}"
        if len(body) != size or snapshot.get(key) != hashlib.sha256(body).hexdigest():
            raise ValueError("Gemini runtime file differs from its complete phase snapshot")
        return bytes(body)

    def checkpoint(self, label, snapshot):
        if self.reason:
            return
        try:
            self.deadline = time.monotonic() + 5
            self._guard()
            expected = ("after-public-repeat-add", "after-public-repeat-list")
            if len(self.phases) >= 2 or label != expected[len(self.phases)]:
                raise ValueError("Gemini runtime phase sequence is invalid")
            if any(snapshot.get(path) != digest for path, digest in self.previous.items()):
                raise ValueError("Gemini runtime repeat changed a previous entry")
            added = sorted(set(snapshot) - set(self.before))
            if len(added) > 16:
                raise ValueError("Gemini runtime new-record count exceeds its bound")
            for key in added:
                if key in self.files:
                    continue
                filename = key[len(self.prefix):] if key.startswith(self.prefix) else ""
                if not _NAME.fullmatch(filename):
                    raise ValueError("Gemini runtime delta is outside exact list-session files")
                body = self._read(self.chats / filename, 4096, snapshot)
                _metadata(body, filename, hashlib.sha256(str(self.project).encode()).hexdigest(),
                          self.started, datetime.now(timezone.utc))
                self.files[key] = {"path": key, "bytes": len(body),
                                   "sha256": hashlib.sha256(body).hexdigest(),
                                   "records": 1, "conversation_records": 0}
            self._guard()
            self.previous = snapshot
            self.phases.append(label)
        except (OSError, ValueError, RuntimeError, UnicodeError, TypeError) as error:
            self.reason = str(error)[:512]

    def finish(self, snapshot):
        if not self.reason:
            try:
                self.deadline = time.monotonic() + 5
                self._guard()
                if len(self.phases) != 2 or snapshot != self.previous:
                    raise ValueError("Gemini runtime final snapshot differs from the public phases")
                self.complete = True
            except (OSError, ValueError, RuntimeError, TypeError) as error:
                self.reason = str(error)[:512]
        return self.complete and not self.reason

    def evidence(self):
        status = ("metadata-only-new-records" if self.files else "unchanged") if (
            self.complete and not self.reason) else "unqualified"
        return {"schema_version": 1, "status": status, "reason": self.reason,
                "files": list(self.files.values()), "phases": self.phases,
                "client_version": "0.66.0", "platform": "linux",
                "producer_attribution": False, "writer_closure": False,
                "source_reference": "google-gemini/gemini-cli@970413207732f5d4d8c50540c7fc359c44fdaaf2"}

    def close(self):
        for fd, _ in (*self.leaves.values(), *reversed(tuple(self.parents.values()))):
            os.close(fd)
        self.leaves.clear()
        self.parents.clear()
