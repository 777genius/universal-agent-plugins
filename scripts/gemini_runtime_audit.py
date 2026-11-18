"""Strict observation of pinned Gemini 0.66.0 list-session startup journals.

Pinned upstream 970413207732f5d4d8c50540c7fc359c44fdaaf2 can also persist
startup environment history and metadata updates during skills-list initialization.
Only exact held startup environment templates can admit three-record startup journals.
Conversation records remain outside this narrow accepted shape.
Keep raw changes and custody; never infer who wrote matching bytes.
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
# Query platform capability independently of wrappers around individual syscalls.
_OPEN_FOR_CAPABILITY = os.open
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
    posix = (sys.platform in ("linux", "darwin") and os.name == "posix"
             and _OPEN_FOR_CAPABILITY in getattr(os, "supports_dir_fd", ())
             and all(type(getattr(os, flag, None)) is int and getattr(os, flag) > 0
                     for flag in ("O_DIRECTORY", "O_NOFOLLOW", "O_CLOEXEC", "O_NONBLOCK")))
    windows = sys.platform == "win32" and os.name == "nt"
    return ((posix or windows) and isinstance(version, dict)
            and version.get("status") == "reported" and type(version.get("exit_code")) is int
            and version["exit_code"] == 0 and version.get("value") == "0.66.0")


def _refused_record_shapes(body):
    """Fixed shapes only; rejected history never gains admission authority."""
    if type(body) is not bytes or not 0 < len(body) <= 4096 or body.count(b"\n") != 3 or not body.endswith(b"\n"):
        return None
    shapes = []
    for line in body.split(b"\n")[:-1]:
        try:
            value = _json(line)
        except Exception:
            shapes.append("json-refused")
            continue
        shape = "other"
        if type(value) is dict:
            keys = set(value)
            metadata = {"sessionId", "projectHash", "startTime", "lastUpdated"}
            if (keys in (metadata, metadata | {"kind"})
                    and all(type(entry) is str for entry in value.values())):
                shape = "metadata-fields"
            elif (keys == {"id", "timestamp", "type", "content"}
                    and type(value["id"]) is str and type(value["timestamp"]) is str
                    and value["type"] == "user"
                    and type(value["content"]) is list and len(value["content"]) == 1
                    and type(value["content"][0]) is dict and set(value["content"][0]) == {"text"}
                    and type(value["content"][0]["text"]) is str):
                shape = "user-text-fields"
            elif (keys == {"$set"} and type(value["$set"]) is dict
                    and set(value["$set"]) == {"lastUpdated"}
                    and type(value["$set"]["lastUpdated"]) is str):
                shape = "last-updated-fields"
        shapes.append(shape)
    return shapes


def _millisecond(value):
    # Gemini Date.toISOString() records exactly milliseconds. Compare the same
    # command bounds at that precision, without an added tolerance; retain raw
    # bounds for command ordering and evidence.
    return value.replace(microsecond=value.microsecond // 1000 * 1000)


def _complete_snapshot(snapshot):
    if (not isinstance(snapshot, dict) or any(
            not isinstance(path, str) or not isinstance(digest, str)
            or digest == "unreadable"
            or path.replace("\\", "/").rsplit("/", 1)[-1] == ".walk-error"
            for path, digest in snapshot.items())):
        raise ValueError("Gemini runtime snapshot is incomplete")


def _metadata(body, filename, project_hash, started, finished, diagnostic=None):
    final_lf, lf_count = body.endswith(b"\n"), body.count(b"\n")
    if not final_lf or lf_count != 1:
        if diagnostic is not None:
            diagnostic.update(operation="record-framing",
                              bytes=len(body) if len(body) <= 4096 else None,
                              lf_count=lf_count if len(body) <= 4096 else None,
                              final_lf=final_lf)
            try:
                shapes = _refused_record_shapes(body)
            except Exception:
                shapes = None
            if shapes is not None:
                diagnostic["record_shapes"] = shapes
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
    if not _millisecond(started) <= first <= last <= _millisecond(finished):
        raise ValueError("Gemini runtime record time is outside the repeat interval")
    match = _NAME.fullmatch(filename)
    if not match:
        raise ValueError("Gemini runtime record filename is not a list session")
    named = datetime.strptime(match[1], "%Y-%m-%dT%H-%M").replace(tzinfo=timezone.utc)
    # The filename is chosen before metadata, and can cross a minute boundary.
    if not started.replace(second=0, microsecond=0) <= named <= first:
        raise ValueError("Gemini runtime filename time is outside the repeat interval")

def _interval(value):
    if (not isinstance(value, tuple) or len(value) != 2
            or any(not isinstance(item, datetime) or item.tzinfo != timezone.utc for item in value)
            or value[0] > value[1]):
        raise ValueError("Gemini startup command interval is invalid")
    return value


def _startup(body, filename, project_hash, interval, project_temp):
    started, finished = _interval(interval)
    if not body.endswith(b"\n") or body.count(b"\n") != 3:
        raise ValueError("Gemini startup record framing is invalid")
    first, message, update = body.splitlines()
    _metadata(first + b"\n", filename, project_hash, started, finished)
    metadata, message, update = _json(first), _json(message), _json(update)
    if (set(message) != {"id", "timestamp", "type", "content"}
            or message["id"] != hashlib.sha256(b"environment-context").hexdigest()[:32]
            or message["type"] != "user" or type(message["content"]) is not list
            or len(message["content"]) != 1 or type(message["content"][0]) is not dict
            or set(message["content"][0]) != {"text"}
            or type(message["content"][0]["text"]) is not str
            or type(update) is not dict or set(update) != {"$set"}
            or type(update["$set"]) is not dict or set(update["$set"]) != {"lastUpdated"}):
        raise ValueError("Gemini startup record contains non-environment data")
    text = message["content"][0]["text"]
    prefix = ("<session_context>\nThis is the Gemini CLI. We are setting up the context for our chat.\n"
              "Today's date is ")
    lines = text.split("\n")
    if (not text.startswith(prefix) or not text.endswith("\n</session_context>")
            or len(lines) < 7 or not lines[2].endswith(" (formatted according to the user's locale).")
            or lines[3] != "My operating system is: " + sys.platform
            or lines[4] != "The project's temporary directory is: " + str(project_temp)
            or text.count("<session_context>") != 1 or text.count("</session_context>") != 1):
        raise ValueError("Gemini startup environment framing differs")
    times = [metadata["startTime"], metadata["lastUpdated"], message["timestamp"],
             update["$set"]["lastUpdated"]]
    if any(type(value) is not str or not _TIME.fullmatch(value) for value in times):
        raise ValueError("Gemini startup record time is invalid")
    parsed = [datetime.fromisoformat(value.replace("Z", "+00:00")) for value in times]
    if not (_millisecond(started) <= parsed[0] <= parsed[1] <= parsed[2]
            <= parsed[3] <= _millisecond(finished)):
        raise ValueError("Gemini startup record time is outside its command interval")
    return message["id"], text


class GeminiRepeatAudit:
    """Hold the pre-repeat namespace and verify both actual public phases."""

    def __init__(self, home, project, before, version):
        self.parents, self.leaves, self.files, self.phases = {}, {}, {}, []
        self.reason, self.complete = None, False
        self.failure_facts = None
        self.phase, self.role = "pre-repeat", "registry"
        self.windows = None
        self.startup_template = None
        self.baseline_observed = False
        self.baseline_record = None
        self.before = self.previous = before
        self.profile, self.project = Path(home) / ".gemini", Path(project)
        self.observed_at = self.command_finished = datetime.now(timezone.utc)
        try:
            if not pinned_version(version):
                raise ValueError("Gemini empty-session proof requires supported custody and native 0.66.0")
            _complete_snapshot(before)
            self.deadline = time.monotonic() + 5
            if sys.platform == "win32":
                backend = run_path(str(Path(__file__).with_name("gemini_windows_custody.py")))
                self.windows = backend["Custody"](lambda: self.deadline)
            self._hold(self.project)
            self._hold(self.profile)
            registry = _json(self._read(self.profile / "projects.json", 65536, before))
            if (not isinstance(registry, dict) or set(registry) != {"projects"}
                    or not isinstance(registry["projects"], dict)):
                raise ValueError("Gemini runtime registry shape is invalid")
            # Pinned ProjectRegistry serializes resolved Windows paths in lowercase.
            registry_project = str(self.project).lower() if sys.platform == "win32" else str(self.project)
            slug = registry["projects"].get(registry_project)
            if (not isinstance(slug, str) or not re.fullmatch(r"[a-z0-9-]{1,128}", slug)
                    or sum(value == slug for value in registry["projects"].values()) != 1):
                slug_is_string = isinstance(slug, str)
                slug_matches_pattern = slug_is_string and bool(re.fullmatch(r"[a-z0-9-]{1,128}", slug))
                self.failure_facts = {
                    "phase": "pre-repeat", "role": "registry", "operation": "project-binding",
                    "exact_key_present": registry_project in registry["projects"],
                    "slug_is_string": slug_is_string, "slug_matches_pattern": slug_matches_pattern,
                    "slug_binding_count": min(2, sum(value == slug for value in registry["projects"].values()))
                    if slug_matches_pattern else None,
                }
                raise ValueError("Gemini runtime registry does not bind the exact project")
            directory = self.profile / "tmp" / slug
            self.chats = directory / "chats"
            self._hold(self.chats)
            self.role = "marker"
            marker = self._read(directory / ".project_root", 4096, before)
            if marker != registry_project.encode("utf-8"):
                raise ValueError("Gemini runtime ownership marker differs")
            self.prefix = f"{self.profile.name}/{self.chats.relative_to(self.profile)}{os.sep}"
            old = [key for key in before if key.startswith(self.prefix)]
            if len(old) > 64:
                raise ValueError("Gemini pre-repeat chat count exceeds its bound")
            self.role = "previous-chat"
            for key in old:
                name = key[len(self.prefix):]
                if (not name or "/" in name or name in (".", "..")
                        or self.windows is not None and "\\" in name):
                    raise ValueError("Gemini pre-repeat chat layout is not flat")
                self._open(self.chats / name, 1 << 20)
            self._guard()
        except (OSError, ValueError, RuntimeError, UnicodeError, TypeError) as error:
            self.reason = str(error)[:512]
            self.close()

    def _hold(self, path):
        if self.windows is not None:
            self.windows.hold(path)
            return
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
        if self.windows is not None:
            self.windows.guard()
            return
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
        if self.windows is not None:
            return self.windows.open_file(path, limit)
        self._guard()
        before = path.lstat()
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or not 0 < before.st_size <= limit or len(self.leaves) >= 82):
            self.failure_facts = {
                "phase": self.phase, "role": self.role, "operation": "file-admission",
                "regular": stat.S_ISREG(before.st_mode), "single_link": before.st_nlink == 1,
                "positive_size": before.st_size > 0, "within_limit": before.st_size <= limit,
                "descriptor_capacity": len(self.leaves) < 82,
                "bytes": before.st_size if 0 <= before.st_size <= limit else None,
            }
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

    def baseline(self, snapshot, *, interval):
        """Join the explicit native list to custody held before that command."""
        if self.reason:
            return
        try:
            self.deadline = time.monotonic() + 5
            self._guard()
            _complete_snapshot(snapshot)
            started, finished = _interval(interval)
            if (self.baseline_observed or self.phases or started < self.observed_at
                    or finished > datetime.now(timezone.utc)):
                raise ValueError("Gemini startup baseline command interval is invalid")
            if any(snapshot.get(path) != digest for path, digest in self.before.items()):
                raise ValueError("Gemini startup baseline changed previous entries")
            added = sorted(set(snapshot) - set(self.before))
            if len(added) > 1:
                raise ValueError("Gemini startup baseline exceeds one fresh list record")
            if sum(key.startswith(self.prefix) for key in snapshot) > 64:
                raise ValueError("Gemini pre-repeat chat count exceeds its bound")
            if added:
                key = added[0]
                name = key[len(self.prefix):] if key.startswith(self.prefix) else ""
                if not _NAME.fullmatch(name):
                    raise ValueError("Gemini startup baseline is outside exact list-session files")
                body = self._read(self.chats / name, 4096, snapshot)
                project_hash = hashlib.sha256(str(self.project).encode()).hexdigest()
                if body.count(b"\n") == 3:
                    self.startup_template = _startup(
                        body, name, project_hash, interval, self.chats.parent)
                else:
                    _metadata(body, name, project_hash, started, finished)
                self.baseline_record = {
                    "path": key, "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest(),
                    "records": body.count(b"\n"), "started": started.isoformat(),
                    "finished": finished.isoformat(), "operation": "native-skills-list-baseline",
                }
            self._guard()
            self.before = self.previous = snapshot
            self.command_finished = finished
            self.baseline_observed = True
        except (OSError, ValueError, RuntimeError, UnicodeError, TypeError) as error:
            self.reason = str(error)[:512]

    def checkpoint(self, label, snapshot, *, interval=None):
        if self.reason:
            return
        try:
            self.deadline = time.monotonic() + 5
            self._guard()
            expected = ("after-public-repeat-add", "after-public-repeat-list")
            if len(self.phases) >= 2 or label != expected[len(self.phases)]:
                raise ValueError("Gemini runtime phase sequence is invalid")
            self.phase, self.role = label, "new-chat"
            _complete_snapshot(snapshot)
            started, finished = _interval(interval)
            if started < self.command_finished or finished > datetime.now(timezone.utc):
                raise ValueError("Gemini startup command intervals overlap or are in the future")
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
                framing = {}
                try:
                    if body.count(b"\n") == 3 and self.startup_template is not None:
                        template = _startup(
                            body, filename, hashlib.sha256(str(self.project).encode()).hexdigest(),
                            interval, self.chats.parent)
                        if template != self.startup_template:
                            raise ValueError("Gemini startup environment differs from held baseline")
                    else:
                        _metadata(body, filename, hashlib.sha256(str(self.project).encode()).hexdigest(),
                                  started, finished, framing)
                except ValueError:
                    if framing:
                        self.failure_facts = {"phase": self.phase, "role": self.role, **framing}
                    raise
                self.files[key] = {"path": key, "bytes": len(body),
                                   "sha256": hashlib.sha256(body).hexdigest(),
                                   "records": body.count(b"\n"), "conversation_records": 0}
            self._guard()
            self.previous = snapshot
            self.command_finished = finished
            self.phases.append(label)
        except (OSError, ValueError, RuntimeError, UnicodeError, TypeError) as error:
            self.reason = str(error)[:512]

    def finish(self, snapshot):
        if not self.reason:
            try:
                self.deadline = time.monotonic() + 5
                self._guard()
                _complete_snapshot(snapshot)
                if len(self.phases) != 2 or snapshot != self.previous:
                    raise ValueError("Gemini runtime final snapshot differs from the public phases")
                self.complete = True
            except (OSError, ValueError, RuntimeError, TypeError) as error:
                self.reason = str(error)[:512]
        return self.complete and not self.reason

    def evidence(self):
        changed = ("startup-environment-new-records" if any(
            value["records"] == 3 for value in self.files.values()) else "metadata-only-new-records")
        status = (changed if self.files else "unchanged") if (
            self.complete and not self.reason) else "unqualified"
        return {"schema_version": 1, "status": status, "reason": self.reason,
                "files": list(self.files.values()), "phases": self.phases,
                "baseline": self.baseline_record,
                "client_version": "0.66.0", "platform": sys.platform,
                "producer_attribution": False, "writer_closure": False,
                "source_reference": "google-gemini/gemini-cli@970413207732f5d4d8c50540c7fc359c44fdaaf2",
                **({"failure_facts": dict(self.failure_facts)}
                   if self.reason and self.failure_facts is not None else {})}

    def close(self):
        if self.windows is not None:
            self.windows.close()
        for fd, _ in (*self.leaves.values(), *reversed(tuple(self.parents.values()))):
            os.close(fd)
        self.leaves.clear()
        self.parents.clear()
