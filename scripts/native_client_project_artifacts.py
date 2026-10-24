"""Admit only the new bounded OpenCode project envelope; never native caches.

Invoked by the matrix in a five-second child. Pending files stay outside the
workflow upload directory. No recursive failure cleanup or target traversal.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import tempfile
import time
import zipfile

LIMIT = 32 << 20
FILES = 128
NAME = re.compile(r"[A-Za-z0-9_.-]{1,128}\.(?:json|log)\Z")


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
            info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def read_envelope(source):
    started = time.monotonic()
    source = Path(source)
    root = source.lstat()
    if not stat.S_ISDIR(root.st_mode) or source.resolve() != source:
        raise ValueError("project envelope root is not an exact real directory")
    names = []
    with os.scandir(source) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > FILES:
                raise ValueError("project envelope file count refused")
    bodies = {}
    total = 0
    for name in sorted(names):
        if time.monotonic() - started > 4:
            raise TimeoutError("project envelope deadline")
        if not NAME.fullmatch(name):
            raise ValueError("project envelope member name refused")
        path = source / name
        before = path.lstat()
        if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or not 0 <= before.st_size <= 1 << 20:
            raise ValueError("project envelope member type/size refused")
        if total + before.st_size > LIMIT - 65536:
            raise ValueError("project envelope aggregate refused")
        fd = os.open(path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0))
        try:
            if identity(os.fstat(fd)) != identity(before):
                raise ValueError("project envelope opened identity differs")
            chunks = []
            left = before.st_size + 1
            while left:
                if time.monotonic() - started > 4:
                    raise TimeoutError("project envelope deadline")
                chunk = os.read(fd, min(left, 65536))
                if not chunk:
                    break
                chunks.append(chunk)
                left -= len(chunk)
            body = b"".join(chunks)
            if len(body) != before.st_size or identity(os.fstat(fd)) != identity(before):
                raise ValueError("project envelope bytes/identity changed")
        finally:
            os.close(fd)
        if identity(path.lstat()) != identity(before):
            raise ValueError("project envelope member replaced")
        bodies[name] = body
        total += len(body)
    if identity(source.lstat()) != identity(root) or "opencode-project-evidence.json" not in bodies:
        raise ValueError("project envelope root changed or summary missing")
    # Capture digests are checked again here, independently from Go's in-memory
    # admission. A malformed claimed stream cannot enter the actual upload ZIP.
    summary = json.loads(bodies["opencode-project-evidence.json"].decode("utf-8"))
    commands = summary.get("commands", [])
    if not isinstance(commands, list) or len(commands) > 60:
        raise ValueError("project command receipt count refused")
    expected = {"opencode-project-evidence.json"}
    expected.update(name for name in ("foreign-snapshots.json", "same-name-winners.json") if name in bodies)
    for command in commands:
        for stream in ("stdout", "stderr"):
            name = command[stream + "_file"]
            if name != command["label"] + "." + stream + ".log" or name in expected:
                raise ValueError("project stream name/uniqueness refused")
            expected.add(name)
            body = bodies[name]
            if len(body) > 128 << 10:
                raise ValueError("project captured stream exceeds limit")
            if len(body) != command[stream + "_bytes"] or hashlib.sha256(body).hexdigest() != command[stream + "_sha256"]:
                raise ValueError("project envelope stream receipt differs")
    if set(bodies) != expected:
        raise ValueError("unrecorded project envelope member refused")
    return bodies


def write_archive(source, destination):
    source, destination = Path(source), Path(destination)
    # Both are on the existing runner temporary filesystem, permitting atomic
    # publication. A cross-filesystem operation fails instead of copying partial.
    if source.parent.stat().st_dev != destination.parent.stat().st_dev:
        raise ValueError("project envelope publication filesystem differs")
    pending = Path(tempfile.mkdtemp(prefix="opencode-envelope-pending-", dir=source.parent)) / "evidence.zip"
    admitted = True
    try:
        bodies = read_envelope(source)
    except (OSError, ValueError, KeyError, TypeError, TimeoutError) as error:
        admitted = False
        bodies = {"opencode-project-evidence.json": json.dumps({"success": False,
            "native_discovery_qualified": False, "native_mcp_initialization_qualified": False,
            "native_model_use_qualified": False, "artifact_admission_error": str(error)[:512]}).encode()}
    with zipfile.ZipFile(pending, "x", compression=zipfile.ZIP_STORED) as archive:
        for name, body in sorted(bodies.items()):
            info = zipfile.ZipInfo(name)
            info.external_attr = (stat.S_IFREG | 0o600) << 16
            archive.writestr(info, body)
            if pending.stat().st_size > LIMIT:
                raise ValueError("project ZIP envelope exceeds aggregate cap")
    if pending.stat().st_size > LIMIT:
        raise ValueError("project ZIP overhead exceeds aggregate cap")
    os.replace(pending, destination)
    return admitted


if __name__ == "__main__":
    sys.exit(0 if write_archive(sys.argv[1], sys.argv[2]) else 2)
