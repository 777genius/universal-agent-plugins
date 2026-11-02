"""Admit bounded OpenCode or closed Cline project evidence; never native caches.

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
WINDOWS = os.name == "nt"


def python_runtime():
    return {"version": sys.version[:512], "implementation": sys.implementation.name[:64],
            "executable": sys.executable[:1024]}


def identity(info):
    result = (info.st_dev, info.st_ino, info.st_mode, info.st_nlink,
              info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    if WINDOWS:
        result += (getattr(info, "st_birthtime_ns", None), getattr(info, "st_file_attributes", None),
                   getattr(info, "st_reparse_tag", None))
    return result


def windows_identity(info):
    # Windows path ctime and held-fd ctime have different meanings in CPython
    # 3.12. Keep both complete same-domain identities, and cross-check birth time.
    birth = getattr(info, "st_birthtime_ns", None)
    attributes = getattr(info, "st_file_attributes", None)
    tag = getattr(info, "st_reparse_tag", None)
    if (not all(type(value) is int and value > 0 for value in (birth, info.st_dev, info.st_ino, info.st_nlink)) or not isinstance(attributes, int) or attributes & 0x400
            or not isinstance(tag, int) or tag != 0):
        raise ValueError("Windows envelope identity is unavailable or a reparse point")
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_size,
            info.st_mtime_ns, birth, attributes)


def same_opened(path_info, held_info):
    if WINDOWS:
        return windows_identity(path_info) == windows_identity(held_info)
    return identity(path_info) == identity(held_info)


def open_member(path):
    if not WINDOWS:
        return os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    import ctypes
    from ctypes import wintypes
    import msvcrt

    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    create, close = kernel.CreateFileW, kernel.CloseHandle
    create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p,
                       wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    create.restype = wintypes.HANDLE
    close.argtypes, close.restype = [wintypes.HANDLE], wintypes.BOOL
    # GENERIC_READ, share read/write/delete, OPEN_EXISTING, OPEN_REPARSE_POINT.
    # A null security-attributes pointer creates a noninheritable handle.
    handle = create(str(path), 0x80000000, 7, None, 3, 0x00200000, None)
    if handle is None or handle == ctypes.c_void_p(-1).value:
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        descriptor = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
        if descriptor < 0:
            raise OSError("Windows file descriptor transfer failed")
    except BaseException:
        if not close(handle):
            raise ctypes.WinError(ctypes.get_last_error())
        raise
    # Only this descriptor now owns the handle; the caller closes it once.
    return descriptor


def check_deadline(started):
    if time.monotonic() - started > 4:
        raise TimeoutError("project envelope deadline")


def confirm_paths(packet):
    source, root, members, started = packet
    check_deadline(started)
    current = source.lstat()
    if WINDOWS:
        windows_identity(current)
    if source.resolve() != source or identity(current) != identity(root):
        raise ValueError("project envelope root changed")
    for path, before in members:
        check_deadline(started)
        current = path.lstat()
        if WINDOWS:
            windows_identity(current)
        if identity(current) != identity(before):
            raise ValueError("project envelope member replaced")
    check_deadline(started)


def read_packet(source, *, client="opencode"):
    if client not in ("opencode", "cline"):
        raise ValueError("unknown closed project evidence client")
    started = time.monotonic()
    source = Path(source)
    root = source.lstat()
    if not stat.S_ISDIR(root.st_mode) or source.resolve() != source:
        raise ValueError("project envelope root is not an exact real directory")
    if WINDOWS:
        windows_identity(root)
    names = []
    with os.scandir(source) as entries:
        for entry in entries:
            names.append(entry.name)
            if len(names) > FILES:
                raise ValueError("project envelope file count refused")
    bodies, members = {}, []
    total = 0
    for name in sorted(names):
        check_deadline(started)
        if not NAME.fullmatch(name):
            raise ValueError("project envelope member name refused")
        path = source / name
        before = path.lstat()
        if not stat.S_ISREG(before.st_mode) or before.st_nlink != 1 or not 0 <= before.st_size <= 1 << 20:
            raise ValueError("project envelope member type/size refused")
        if WINDOWS:
            windows_identity(before)
        if total + before.st_size > LIMIT - 65536:
            raise ValueError("project envelope aggregate refused")
        descriptor = open_member(path)
        try:
            held = os.fstat(descriptor)
            if not stat.S_ISREG(held.st_mode) or not same_opened(before, held):
                raise ValueError("project envelope opened identity differs")
            chunks = []
            left = before.st_size + 1
            while left:
                check_deadline(started)
                chunk = os.read(descriptor, min(left, 65536))
                if not chunk:
                    break
                chunks.append(chunk)
                left -= len(chunk)
            body = b"".join(chunks)
            after = os.fstat(descriptor)
            if len(body) != before.st_size or identity(after) != identity(held):
                raise ValueError("project envelope bytes/held identity changed")
        finally:
            os.close(descriptor)
        if identity(path.lstat()) != identity(before):
            raise ValueError("project envelope member replaced")
        members.append((path, before))
        bodies[name] = body
        total += len(body)
    packet = (source, root, members, started)
    confirm_paths(packet)
    if client == "cline":
        from cline_project_envelope import validate
        validate(bodies)
        return bodies, packet
    if "opencode-project-evidence.json" not in bodies:
        raise ValueError("project envelope summary missing")
    # Capture digests are checked again independently from Go's in-memory admission.
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
    return bodies, packet


def read_envelope(source, *, client="opencode"):
    return read_packet(source, client=client)[0]


def write_zip(pending, bodies):
    with zipfile.ZipFile(pending, "x", compression=zipfile.ZIP_STORED) as archive:
        for name, body in sorted(bodies.items()):
            info = zipfile.ZipInfo(name)
            info.external_attr = (stat.S_IFREG | 0o600) << 16
            archive.writestr(info, body)
            if pending.stat().st_size > LIMIT:
                raise ValueError("project ZIP envelope exceeds aggregate cap")
    if pending.stat().st_size > LIMIT:
        raise ValueError("project ZIP overhead exceeds aggregate cap")


def cline_destination_parent(path, expected=None):
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode) or path.resolve() != path:
        raise ValueError("Cline upload parent is not an exact ordinary directory")
    if expected is not None and identity(info) != identity(expected):
        raise ValueError("Cline upload parent changed before publication")
    return info


def write_archive(source, destination, *, client="opencode"):
    if client not in ("opencode", "cline"):
        raise ValueError("unknown closed project evidence client")
    source, destination = Path(source), Path(destination)
    if source.parent.stat().st_dev != destination.parent.stat().st_dev:
        raise ValueError("project envelope publication filesystem differs")
    parent = cline_destination_parent(destination.parent) if client == "cline" else None
    pending = Path(tempfile.mkdtemp(prefix=client + "-envelope-pending-", dir=source.parent)) / "evidence.zip"
    admitted = True
    try:
        bodies, packet = read_packet(source, client=client)
        write_zip(pending, bodies)
        confirm_paths(packet)  # The closed ZIP contains only captured in-memory bytes.
    except (OSError, ValueError, KeyError, TypeError, TimeoutError) as error:
        admitted = False
        failure = {"success": False, "native_discovery_qualified": False,
            "native_mcp_initialization_qualified": False, "native_model_use_qualified": False,
            "artifact_admission_error": str(error)[:512], "artifact_python_runtime": python_runtime()}
        summary_name = "opencode-project-evidence.json"
        if client == "cline":
            from cline_project_envelope import NATIVE, NATIVE_FALSE
            summary_name = NATIVE
            failure.update({key: False for key in NATIVE_FALSE})
            failure.update(collection_complete=False, read_only_unique_marker_discovery_qualified=False)
        bodies = {summary_name: json.dumps(failure).encode()}
        pending.unlink(missing_ok=True)
        write_zip(pending, bodies)
    if client == "cline":
        cline_destination_parent(destination.parent, parent)
    os.replace(pending, destination)
    return admitted


if __name__ == "__main__":
    sys.exit(0 if write_archive(sys.argv[1], sys.argv[2]) else 2)
