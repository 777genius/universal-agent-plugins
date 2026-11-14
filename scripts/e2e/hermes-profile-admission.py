#!/usr/bin/env python3
"""Observe pinned bundled Hermes and its explicit UAP Project MCP composition."""
import base64
import hashlib
from contextlib import ExitStack, contextmanager
import json
import math
import os
from pathlib import Path
import platform
import re
import runpy
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile
import time

HERE = Path(__file__).resolve().parent
BASE = runpy.run_path(str(HERE / "hermes-noauth-tool-call.py"), run_name="hermes_admission_helpers")
require, bounded, sha = (BASE[name] for name in ("require", "bounded", "sha"))
import native_client_project_artifacts as artifacts

LIMIT = 1 << 20
EVIDENCE_CLOSED = False
STABLE_FIELDS = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid",
                 "st_size", "st_mtime_ns", "st_ctime_ns")


def stable_file(info):
    # Access time may change merely because this verifier read the file.
    return artifacts.identity(info) + (info.st_uid, info.st_gid)


def stable_named_entry(info):
    """Normalize only NTFS's redundant directory bit on named directory stats.

    CPython issue 126253 records 0x10000010 versus 0x10 for the same ordinary
    NTFS directory. Held-handle snapshots and every other attribute stay exact.
    """
    identity = stable_file(info)
    attributes = getattr(info, "st_file_attributes", None)
    if (artifacts.WINDOWS and stat.S_ISDIR(info.st_mode)
            and type(attributes) is int and 0 <= attributes < 2**32
            and attributes & 0x10 and not attributes & 0x400
            and getattr(info, "st_reparse_tag", None) == 0):
        # artifacts.identity places Windows attributes after birth time.
        return identity[:8] + (attributes & ~0x10000000,) + identity[9:]
    return identity


def same_opened_capture(before, held):
    """Join Windows path/fd by stable file ID, not synthetic cross-API mode.

    A path stat and an opened Windows handle may synthesize permission/execute
    bits differently. The complete mode is still compared within each API
    domain before and after capture; only cross-domain mode equality is omitted.
    """
    if not artifacts.WINDOWS:
        return artifacts.same_opened(before, held)
    require(stat.S_ISREG(before.st_mode) and stat.S_ISREG(held.st_mode),
            "opened Windows member is not an ordinary file")
    from_path, from_handle = artifacts.windows_identity(before), artifacts.windows_identity(held)
    return from_path[:2] == from_handle[:2] and from_path[3:] == from_handle[3:]


def ordinary_bytes(path, expected=None, *, parent=None, pin=None, package_digest=None, deadline=None):
    path = Path(path)
    require(parent is not None or (path.is_absolute() and path.resolve() == path), "ordinary file alias")
    def linked():
        return path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
    before = linked()
    require(pin is None or (expected is not None and type(pin) is tuple and len(pin) == 2
            and type(pin[0]) is int and 0 < pin[0] <= 128 << 20
            and type(pin[1]) is str and re.fullmatch(r"[a-f0-9]{64}", pin[1])), "fixed runtime pin required")
    maximum = LIMIT if pin is None else pin[0]
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 <= before.st_size <= maximum
            and (pin is None or before.st_size == maximum),
            "bounded ordinary single-link file required")
    require(expected is None or stable_file(before) == stable_file(expected), "file changed before open")
    # Reuse native artifact admission's Windows OPEN_REPARSE_POINT handle and
    # positive cross-API identity. Path/fd ctime domains remain separately held.
    descriptor = (artifacts.open_member(path) if parent is None else
                  os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent))
    try:
        held = os.fstat(descriptor)
        require(same_opened_capture(before, held)
                and (held.st_uid, held.st_gid) == (before.st_uid, before.st_gid),
                "file changed at open: " + ",".join(
                    field for field in STABLE_FIELDS
                    if not (artifacts.WINDOWS and field == "st_ctime_ns")
                    and getattr(held, field) != getattr(before, field)))
        if pin is None:
            with os.fdopen(os.dup(descriptor), "rb") as stream:
                body = stream.read(LIMIT + 1)
            count = len(body)
            if package_digest is not None: package_digest.update(body)
        else:
            digest, count = hashlib.sha256(), 0
            while count <= maximum:
                require(deadline is None or time.monotonic() < deadline, "pinned runtime read deadline")
                chunk = os.read(descriptor, min(65536, maximum + 1 - count))
                if not chunk: break
                count += len(chunk)
                require(count <= maximum, "pinned runtime grew")
                digest.update(chunk)
                if package_digest is not None: package_digest.update(chunk)
            body = digest.hexdigest()
            require(body == pin[1], "pinned runtime bytes differ")
        require(count == before.st_size <= maximum and stable_file(os.fstat(descriptor)) == stable_file(held)
                and stable_file(linked()) == stable_file(before), "file changed during bounded capture")
        require(deadline is None or time.monotonic() < deadline, "pinned runtime read deadline")
        return body
    finally:
        os.close(descriptor)


def directory_identity(path):
    require(path.is_absolute() and path.resolve() == path and not path.is_symlink(), "directory alias")
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode), "ordinary directory required")
    if artifacts.WINDOWS:
        artifacts.windows_identity(info)
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def verify_root_identities(roots, held):
    # Keep the refusal exact, reporting only fixed field labels, never values or paths.
    fields = ("device", "inode", "mode", "uid", "gid")
    for name, root in roots.items():
        actual, expected = directory_identity(root), tuple(held[name])
        if actual != expected:
            differences = ",".join(field for field, left, right in zip(fields, actual, expected) if left != right)
            raise RuntimeError("original custom root directory changed (" + (differences or "shape") + ")")


def inventory_member(relative, info):
    """Keep the original bounds; report only bounded fixture-relative metadata."""
    if stat.S_ISDIR(info.st_mode) or (stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                                     and 0 <= info.st_size <= LIMIT):
        return
    encoded = json.dumps(relative, ensure_ascii=True)
    details = {"member": relative if len(encoded) <= 512 else None,
               "member_sha256": hashlib.sha256(relative.encode("utf-8")).hexdigest(),
               "mode": info.st_mode, "links": info.st_nlink, "bytes": info.st_size}
    raise RuntimeError("inventory member refused: " + json.dumps(details, ensure_ascii=True, sort_keys=True))


def open_inventory_directory(path, parent=None):
    if not artifacts.WINDOWS:
        return os.open(path if parent is None else path.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NONBLOCK, **({} if parent is None else {"dir_fd": parent}))
    import ctypes
    from ctypes import wintypes
    import msvcrt
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    create, close = kernel.CreateFileW, kernel.CloseHandle
    create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p,
                       wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    create.restype = wintypes.HANDLE
    close.argtypes, close.restype = [wintypes.HANDLE], wintypes.BOOL
    # Directory + no reparse traversal; omit FILE_SHARE_DELETE so the held
    # directory cannot be renamed while Windows' path-only scandir enumerates.
    handle = create(str(path), 0x80000000, 3, None, 3, 0x02200000, None)
    if handle is None or handle == ctypes.c_void_p(-1).value:
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        descriptor = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
        require(descriptor >= 0, "Windows directory descriptor transfer failed")
    except BaseException:
        if not close(handle):
            raise ctypes.WinError(ctypes.get_last_error())
        raise
    return descriptor


def same_inventory_directory(before, held):
    require(stat.S_ISDIR(before.st_mode) and stat.S_ISDIR(held.st_mode), "held inventory directory required")
    if artifacts.WINDOWS:
        left, right = artifacts.windows_identity(before), artifacts.windows_identity(held)
        # Cross-API attributes may differ for the same ordinary directory.
        # Both windows_identity calls reject reparse points, while each
        # domain's full attributes are checked again before and after use.
        return left[:2] == right[:2] and left[3:7] == right[3:7]
    return artifacts.same_opened(before, held)


def check_inventory_directory(before, held, label):
    """Fail closed with bounded cross-API field *names*, never paths or values.

    Windows path lstat and held directory fstat use distinct OS queries and
    can report different mode and attribute flags for the same file ID. Both domains independently
    enforce non-reparse identity and preserve full before/after snapshots.
    """
    if same_inventory_directory(before, held):
        return
    detail = ""
    if artifacts.WINDOWS:
        left, right = artifacts.windows_identity(before), artifacts.windows_identity(held)
        fields = ((0, "device"), (1, "inode"), (3, "links"), (4, "bytes"),
                  (5, "mtime"), (6, "birth"))
        detail = " (Windows metadata: " + ",".join(
            name for index, name in fields if left[index] != right[index]) + ")"
    raise RuntimeError(label + detail)


def verify_inventory_directory(path, before, descriptor, held, parent=None, *, diagnostic=None):
    linked = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
    require(parent is not None or path.resolve() == path, "inventory directory changed (canonical)")
    opened = os.fstat(descriptor)
    if stable_named_entry(linked) == stable_named_entry(before) and stable_file(opened) == stable_file(held):
        return
    fields = STABLE_FIELDS + (("st_birthtime_ns", "st_file_attributes", "st_reparse_tag") if artifacts.WINDOWS else ())
    details = []
    for domain, previous, current in (("path", before, linked), ("handle", held, opened)):
        names = [name for name in fields if getattr(previous, name, None) != getattr(current, name, None)]
        if names:
            details.append(domain + ":" + ",".join(names))
    if artifacts.WINDOWS and type(diagnostic) is tuple and len(diagnostic) == 2:
        role, relative = diagnostic
        values = [getattr(info, "st_file_attributes", None) for info in (before, linked, held, opened)]
        if (type(role) is str and role in {"work", "cache", "payload"}
                and type(relative) is str and 0 < len(relative) <= 4096
                and "\\" not in relative and ":" not in relative
                and (relative == "." or not any(part in {"", ".", ".."} for part in relative.split("/")))
                and all(type(value) is int and 0 <= value < 2**32 for value in values)
                and (values[0] != values[1] or values[2] != values[3])):
            try:
                report = {"kind": "owned_census_windows_attributes", "role": role,
                          "member_sha256": hashlib.sha256(relative.encode("utf-8")).hexdigest()}
                report.update(zip(("path_before", "path_after", "handle_before", "handle_after"), values))
                encoded = json.dumps(report, sort_keys=True, separators=(",", ":"))
                if len(encoded) <= 512:
                    details.append("attributes:" + encoded)
            except (TypeError, ValueError, UnicodeError):
                pass  # A diagnostic cannot replace the original refusal.
    raise RuntimeError("inventory directory changed (" + ";".join(details) + ")")


# Same official archive members already admitted by the Antigravity/Devin
# public fixtures. This is one fixed immutable leaf, never a cache subtree.
PUBLIC_SCANNERS = {
    "linux/amd64": (9989584, "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba"),
    "darwin/arm64": (7949216, "37342414aa7feadc09b8473d1bfafe95fd79ab99f284d37518c930fcb7986c57"),
    "windows/amd64": (7620608, "9d486e5b66875384c12a147b8a19007beebe2691bc6d23a7867bf019b70be453"),
}


def public_scanner_pin():
    system = {"linux": "linux", "darwin": "darwin", "win32": "windows"}.get(sys.platform)
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    require(f"{system}/{arch}" in PUBLIC_SCANNERS, "unqualified public scanner platform")
    leaf = "lintai.exe" if system == "windows" else "lintai"
    return (f"public-uap-state/security/lintai/0.1.3/{system}-{arch}/{leaf}", *PUBLIC_SCANNERS[f"{system}/{arch}"])


def inventory(root, sentinels=(), *, launchers=None, public=False, launcher_mode=None):
    """Capture an ordinary tree, refusing after the bounded number of visits.

    os.walk materializes the whole child-name list *before* the membership
    limit is checked. A lazy scandir walk keeps that limit effective even for
    a directory with an unexpectedly large number of entries.
    """
    root = Path(root)
    identity = directory_identity(root)
    result, total, copies, binary_total = {}, 0, 0, 0
    launchers = launchers or {}
    require(len(launchers) <= 4 and all(re.fullmatch(
        r"public-[ab]/\.uap/managed/clients/hermes/hermes-public-[ab]-[a-f0-9]{12}/"
        r"io\.github\.777genius\.agentplugins/managed-stdio-v1/agentplugins(?:\.exe)?", name)
        for name in launchers), "finite receipt-owned launcher paths required")
    require(not artifacts.WINDOWS or not launchers or type(launcher_mode) is int and 0 <= launcher_mode <= 0o777,
            "original named Windows launcher mode required")
    scanner = public_scanner_pin() if public else None

    def visit(path, depth, parent=None):
        nonlocal total, copies, binary_total
        require(len(result) < 2048 and depth <= 24, "inventory membership bound")
        relative = path.relative_to(root).as_posix()
        info = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        if depth == 0:
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == identity,
                    "inventory original root changed before open")
        pin = launchers.get(relative)
        if scanner is not None and relative == scanner[0]: pin = scanner[1:]
        if pin is None:
            inventory_member(relative, info)
        else:
            require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and info.st_size == pin[0]
                    and info.st_dev == identity[0] and (info.st_uid, info.st_gid) == identity[3:]
                    and (artifacts.WINDOWS or stat.S_IMODE(info.st_mode) == 0o700)
                    and (not artifacts.WINDOWS or relative not in launchers or stat.S_IMODE(info.st_mode) == launcher_mode),
                    "pinned runtime metadata differs")
        record = {"type": "directory" if stat.S_ISDIR(info.st_mode) else "file",
                  "mode": stat.S_IMODE(info.st_mode), "device": info.st_dev, "inode": info.st_ino}
        if record["type"] == "file":
            body = ordinary_bytes(path, info, parent=parent, pin=pin)
            if pin is None:
                total += len(body)
                require(total <= 16 << 20, "inventory byte bound")
                require(public and not relative.startswith("public-") or not any(marker.encode() in body for marker in sentinels),
                        "original User settings/data copied into custom root")
                record.update(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
            else:
                copies += int(relative in launchers)
                binary_total += pin[0] if relative in launchers else 0
                require(copies <= 4 and binary_total <= 4 * (128 << 20), "managed launcher count/byte bound")
                record.update(bytes=pin[0], sha256=body, identity=list(stable_file(info)))
        result[relative] = record
        if record["type"] == "directory":
            descriptor = open_inventory_directory(path, parent)
            try:
                held = os.fstat(descriptor)
                check_inventory_directory(info, held, "inventory directory changed at open")
                verify_inventory_directory(path, info, descriptor, held, parent)
                with os.scandir(path if artifacts.WINDOWS else descriptor) as entries:
                    for entry in entries:
                        visit(path / entry.name, depth + 1, None if artifacts.WINDOWS else descriptor)
                verify_inventory_directory(path, info, descriptor, held, parent)
            finally:
                os.close(descriptor)

    visit(root, 0)
    require(directory_identity(root) == identity, "inventory root replaced")
    return result


# Official standalone payload authority is separate from the small mutable
# profile ledger. These finite ceilings include every shipped byte and link.
PAYLOAD_MEMBERS, PAYLOAD_DEPTH, PAYLOAD_BYTES = 262144, 64, 8 << 30


def payload_links(root, records, deadline, source=None, *, resolve=None):
    """Resolve only the captured link graph; never traverse a filesystem link."""
    lookup = {Path(name): record for name, record in records.items()}
    spellings = {Path(name): name for name in records} if source is not None else {}
    require(len(lookup) == len(records), "ambiguous payload path spelling")
    require(resolve is None or (source is not None and type(resolve) is tuple and 0 < len(resolve) <= 4
            and all(type(name) is str and name in records for name in resolve)), "closed captured link selection required")
    resolved_inputs = {} if resolve is not None else None
    def parts(target, member, record):
        original_target = target
        extended = artifacts.WINDOWS and target.startswith("\\\\?\\")
        if extended:
            target = target[4:]
        path = Path(target)
        require(not extended or (path.is_absolute() and len(path.drive) == 2
                and path.drive[0] in "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
                and path.drive[1] == ":"), "payload device namespace link")
        require(not path.drive or path.is_absolute(), "payload drive-relative link")
        if path.is_absolute():
            try:
                require(path.is_relative_to(root), "payload link escapes runtime")
            except RuntimeError as error:
                if source is not None:
                    source_payload_refusal(error, member, record, target=original_target)
                raise
            return True, list(path.relative_to(root).parts)
        return False, list(path.parts)

    for name, record in records.items():
        if resolve is not None and name not in resolve:
            continue
        if record["type"] != "link":
            if resolved_inputs is not None:
                resolved_inputs[name] = (name, ())
            continue
        traversed = [name]
        absolute, tail = parts(record["target"], name, record)
        pending = ([] if absolute else list(Path(name).parent.parts)) + tail
        resolved, count = [], 0
        while pending:
            require(time.monotonic() < deadline, "payload scan deadline")
            component, *pending = pending
            if component in ("", "."):
                continue
            if component == "..":
                require(bool(resolved), "payload link escapes runtime")
                resolved.pop()
                continue
            resolved.append(component)
            selected = lookup.get(Path(*resolved))
            require(selected is not None, "payload link target missing")
            if source is not None:
                source_link_boundary(name, spellings[Path(*resolved)], source, intermediate=True)
            if selected["type"] == "link":
                count += 1
                require(count <= PAYLOAD_DEPTH, "payload link cycle/depth")
                member = spellings[Path(*resolved)] if source is not None else ""
                traversed.append(member)
                absolute, tail = parts(selected["target"], member, selected)
                resolved = [] if absolute else resolved[:-1]
                pending = tail + pending
            elif pending:
                require(selected["type"] == "directory", "payload link traverses non-directory")
        if source is not None:
            source_link_boundary(name, spellings[Path(*resolved)], source)
        if resolved_inputs is not None:
            resolved_inputs[name] = (spellings[Path(*resolved)], tuple(traversed))
    return resolved_inputs



class PayloadHardlinkRefusal(RuntimeError):
    """Retain one bounded metadata observation without authorizing a file read."""
    def __init__(self, identity, names):
        super().__init__("payload hardlink escapes runtime")
        self.diagnostic = None
        try:
            require(type(identity) is tuple and len(identity) in (9, 12)
                    and type(names) is list and 0 < len(names) <= PAYLOAD_MEMBERS
                    and all(type(name) is str for name in names), "diagnostic shape")
            member = min(names)
            require(type(member) is str and member and ":" not in member and "\\" not in member
                    and all(part not in ("", ".", "..") for part in member.split("/")), "diagnostic member")
            encoded = json.dumps(member, ensure_ascii=True).encode("ascii")
            values = dict(device=identity[0], inode=identity[1], links=identity[3],
                          captured_aliases=len(names), uid=identity[-2], gid=identity[-1])
            bounded = {key: value if type(value) is int and 0 <= value <= (1 << 64) - 1 else None
                       for key, value in values.items()}
            diagnostic = dict(schema_version=1, measurement_only=True,
                              member=member if len(encoded) <= 128 else None,
                              member_sha256=hashlib.sha256(member.encode("utf-8", "surrogatepass")).hexdigest(),
                              hash_encoding="utf-8-surrogatepass", **bounded,
                              integer_saturated=any(value is None for value in bounded.values()))
            require(len(json.dumps(diagnostic, ensure_ascii=True).encode("ascii")) <= 1024, "diagnostic bound")
            self.diagnostic = diagnostic
        except (IndexError, TypeError, ValueError, UnicodeError, RuntimeError):
            pass  # Diagnostics never replace the original alias refusal.


def payload_hardlink_closure(records, deadline, source=None):
    """Close every regular inode's complete alias set before any content read."""
    groups = {}
    for name, record in records.items():
        require(time.monotonic() < deadline, "payload scan deadline")
        if record["type"] != "file":
            continue
        identity = record["identity"]
        require(all(type(value) is int and value > 0 for value in (identity[0], identity[1], identity[3])),
                "payload file identity unavailable")
        key = tuple(identity[:2])
        if key not in groups:
            groups[key] = (identity, [])
        original, names = groups[key]
        require(identity == original, "payload hardlink metadata differs")
        names.append(name)
    for identity, names in groups.values():
        require(time.monotonic() < deadline, "payload scan deadline")
        if not (type(identity[3]) is int and len(names) == identity[3]):
            raise PayloadHardlinkRefusal(identity, names)
        if source is not None:
            source_hardlink_boundary(names, source)


def source_bound_scope(member):
    for scope, root in (("selected-a", "project-a/.uap/hermes-runtime"),
                        ("selected-b", "project-b/.uap/hermes-runtime"), ("original-user", "user")):
        if below(member, root):
            return scope
    return "fixture-other"


def source_bound_predicate(kind, size, count):
    count_failed, size_failed = count > 2048, kind == "file" and size > LIMIT
    if count_failed and size_failed:
        return "count-and-file-bytes"
    return "count" if count_failed else "file-bytes" if size_failed else None


def source_payload_refusal(error, member, record, *, bytes_read=None, target=None, profile_bound=None):
    """Attach captured metadata only; an unsuccessful observation is no authority."""
    try:
        identity = record["identity"]
        require(type(identity) is tuple and len(identity) in (9, 12) and type(member) is str
                and ":" not in member and "\\" not in member
                and all(part not in ("", ".", "..") for part in member.split("/")), "source refusal metadata shape")
        numbers = dict(device=identity[0], inode=identity[1], mode=identity[2], links=identity[3], bytes=identity[4],
                       uid=identity[-2], gid=identity[-1], attributes=identity[8] if len(identity) == 12 else None,
                       reparse_tag=identity[9] if len(identity) == 12 else None)
        bounded = lambda value: value if type(value) is int and 0 <= value < 2**64 else None
        encoded = member.encode("utf-8", "surrogatepass")
        target_bytes = target.encode("utf-8", "surrogatepass") if target is not None else None
        value = dict(schema_version=1, measurement_only=True, admission_qualified=False,
                     operation="ordinary-profile-bound" if profile_bound is not None else
                               "file-read" if bytes_read is not None else "absolute-link-target",
                     member=member if len(json.dumps(member, ensure_ascii=True).encode("ascii")) <= 512 else None,
                     member_sha256=hashlib.sha256(encoded).hexdigest(), hash_encoding="utf-8-surrogatepass",
                     metadata={key: bounded(number) for key, number in numbers.items()},
                     integer_saturated=any(number is not None and bounded(number) is None for number in numbers.values()),
                     bytes_read=bounded(bytes_read), errno=bounded(getattr(error, "errno", None)),
                     winerror=bounded(getattr(error, "winerror", None)),
                     target_sha256=hashlib.sha256(target_bytes).hexdigest() if target_bytes is not None else None,
                     target_bytes=len(target_bytes) if target_bytes is not None else None)
        if profile_bound is not None:
            value["profile_bound"] = dict(profile_bound)
        source_refusal_document(json.dumps(dict(value, nonce="0" * 64, stage="observer-profile-snapshot")).encode())
        error.source_payload_refusal = value
    except (AttributeError, KeyError, IndexError, TypeError, ValueError, UnicodeError, RuntimeError):
        pass  # The original exception, including its errno and traceback, wins.


def source_refusal_document(body):
    require(type(body) is bytes and 0 < len(body) <= 2048, "source refusal evidence bound")
    value = census_document(body)
    fields = set("schema_version measurement_only admission_qualified nonce stage operation member member_sha256 hash_encoding metadata integer_saturated bytes_read errno winerror target_sha256 target_bytes".split())
    if type(value) is dict and value.get("operation") == "ordinary-profile-bound":
        fields.add("profile_bound")
    require(type(value) is dict and set(value) == fields, "closed source refusal schema")
    digest = lambda text: type(text) is str and re.fullmatch(r"[0-9a-f]{64}", text) is not None
    number = lambda item: type(item) is int and 0 <= item < 2**64
    require(type(value["schema_version"]) is int and value["schema_version"] == 1
            and value["measurement_only"] is True and value["admission_qualified"] is False and digest(value["nonce"])
            and value["stage"] in ("parent-before-observer", "observer-profile-initial", "observer-profile-snapshot", "parent-after-settlement")
            and value["operation"] in ("file-read", "absolute-link-target", "ordinary-profile-bound") and value["hash_encoding"] == "utf-8-surrogatepass"
            and type(value["integer_saturated"]) is bool and digest(value["member_sha256"]), "source refusal identity differs")
    member, metadata = value["member"], value["metadata"]
    require(member is None or (type(member) is str and ":" not in member and "\\" not in member
            and all(part not in ("", ".", "..") for part in member.split("/"))
            and len(json.dumps(member, ensure_ascii=True).encode("ascii")) <= 512
            and hashlib.sha256(member.encode("utf-8", "surrogatepass")).hexdigest() == value["member_sha256"]), "source refusal member differs")
    require(type(metadata) is dict and set(metadata) == set("device inode mode links bytes uid gid attributes reparse_tag".split())
            and all(number(item) or item is None for item in metadata.values())
            and (value["integer_saturated"] or all(number(metadata[key]) for key in ("device", "inode", "mode", "links", "bytes", "uid", "gid"))), "source refusal metadata differs")
    require(all(value[key] is None or number(value[key]) for key in ("bytes_read", "errno", "winerror", "target_bytes")), "source refusal counters differ")
    if value["operation"] == "file-read":
        require(number(value["bytes_read"]) and number(metadata["bytes"])
                and value["bytes_read"] <= metadata["bytes"] <= PAYLOAD_BYTES
                and value["target_sha256"] is None and value["target_bytes"] is None, "source read refusal differs")
    elif value["operation"] == "absolute-link-target":
        require(all(value[key] is None for key in ("bytes_read", "errno", "winerror"))
                and digest(value["target_sha256"]) and number(value["target_bytes"]) and 0 < value["target_bytes"] <= 16384, "source link refusal differs")
    else:
        bound = value["profile_bound"]
        require(type(bound) is dict and set(bound) == {"scope", "kind", "ordinary_count", "predicate"}
                and bound["scope"] in ("selected-a", "selected-b", "original-user", "fixture-other")
                and (member is None or bound["scope"] == source_bound_scope(member))
                and bound["kind"] in ("file", "directory", "link")
                and number(bound["ordinary_count"]) and 0 < bound["ordinary_count"] <= PAYLOAD_MEMBERS
                and number(metadata["mode"]) and number(metadata["bytes"]) and metadata["bytes"] <= PAYLOAD_BYTES
                and ((bound["kind"] == "link" and
                      (stat.S_ISLNK(metadata["mode"]) or (number(metadata["attributes"]) and metadata["attributes"] & 0x400
                       and metadata["reparse_tag"] in (0xA0000003, 0xA000000C))))
                     or (bound["kind"] != "link" and not (metadata["attributes"] or 0) & 0x400
                         and stat.S_IFMT(metadata["mode"]) == {"file": stat.S_IFREG, "directory": stat.S_IFDIR}[bound["kind"]])),
                "source ordinary refusal observation differs")
        require(bound["predicate"] in ("count", "file-bytes", "count-and-file-bytes")
                and bound["predicate"] == source_bound_predicate(bound["kind"], metadata["bytes"], bound["ordinary_count"])
                and all(value[key] is None for key in ("bytes_read", "errno", "winerror", "target_sha256", "target_bytes")),
                "source ordinary refusal predicate differs")
    return value


def retain_source_refusal(error, evidence, expected, nonce, stage):
    try:
        observed = getattr(error, "source_payload_refusal", None)
        if observed is None:
            return
        value = dict(observed, nonce=nonce, stage=stage)
        source_refusal_document(json.dumps(value).encode() + b"\n")
        with held_census_evidence(evidence, expected) as (descriptor, verify):
            verify()
            write_census_member(evidence / "source-payload-refusal.json", value, None if artifacts.WINDOWS else descriptor)
    except (OSError, TypeError, ValueError, RuntimeError):
        pass  # First O_EXCL record only; unavailable diagnostics never mask refusal.


def payload_hash_files(root, records, expected, deadline, source=None):
    """Rejoin the metadata-first authority through held ancestors before bytes."""
    children, documents = {}, {}
    requested = tuple(source.get("documents", ())) if source is not None else ()
    require(len(requested) <= 2 and len(set(requested)) == len(requested), "bounded source documents required")
    for name in records:
        if name != ".":
            children.setdefault(Path(name).parent.as_posix(), set()).add(name)

    def visit(path, parent=None):
        require(time.monotonic() < deadline, "payload scan deadline")
        linked = lambda: path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        name, info = path.relative_to(root).as_posix(), linked()
        record = records.get(name)
        require(record is not None and stable_named_entry(info) == record["identity"], "payload metadata changed before hashing")
        if name == ".":
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == tuple(expected), "payload original root changed")
        if record["type"] == "link":
            return
        if record["type"] == "directory":
            descriptor = open_inventory_directory(path, parent)
            try:
                held = os.fstat(descriptor)
                check_inventory_directory(info, held, "payload hash directory changed at open")
                verify_inventory_directory(path, info, descriptor, held, parent)
                names = set()
                with os.scandir(path if artifacts.WINDOWS else descriptor) as entries:
                    for entry in entries:
                        member = (path / entry.name).relative_to(root).as_posix()
                        require(member in children.get(name, set()), "payload new member before hashing")
                        names.add(member)
                        visit(path / entry.name, None if artifacts.WINDOWS else descriptor)
                require(names == children.get(name, set()), "payload member disappeared before hashing")
                verify_inventory_directory(path, info, descriptor, held, parent)
            finally:
                os.close(descriptor)
            return
        descriptor = (artifacts.open_member(path) if parent is None else
                      os.open(path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent))
        try:
            held = os.fstat(descriptor)
            require(same_opened_capture(info, held) and (info.st_uid, info.st_gid) == (held.st_uid, held.st_gid), "payload file changed at open")
            digest, size, tail = hashlib.sha256(), 0, b""
            captured = bytearray() if name in requested else None
            require(captured is None or (info.st_nlink == 1 and info.st_size <= LIMIT), "bounded source document required")
            sentinels = source["sentinels"] if source is not None and source_project(name, source) is not None else ()
            overlap = max((len(value) for value in sentinels), default=1) - 1
            while size <= info.st_size:
                require(time.monotonic() < deadline, "payload scan deadline")
                # A readable, held empty source file has no byte to request;
                # Windows leases may lock a byte beyond EOF. Keep both final fences.
                if source is not None and info.st_size == 0:
                    break
                try:
                    chunk = os.read(descriptor, min(LIMIT, info.st_size + 1 - size))
                except OSError as error:
                    if source is not None:
                        source_payload_refusal(error, name, record, bytes_read=size)
                    raise
                if not chunk:
                    break
                size += len(chunk)
                digest.update(chunk)
                if captured is not None:
                    captured.extend(chunk)
                require(not any(value in tail + chunk for value in sentinels), "original User sentinel copied into selected runtime")
                tail = (tail + chunk)[-overlap:] if overlap else b""
            require(size == info.st_size and stable_file(os.fstat(descriptor)) == stable_file(held)
                    and stable_file(linked()) == stable_file(info), "payload file changed during capture")
            record["sha256"] = digest.hexdigest()
            if captured is not None:
                documents[name] = bytes(captured)
        finally:
            os.close(descriptor)

    visit(root)
    require(set(documents) == set(requested), "captured source document missing")
    return documents

def payload_snapshot(root, expected, deadline, *, source=None):
    """Stream all payload bytes with held no-follow directory/file authority."""
    require(directory_identity(root) == tuple(expected), "payload original root changed")
    records, links, total = {}, 0, 0

    def visit(path, depth, parent=None):
        nonlocal links, total
        require(time.monotonic() < deadline, "payload scan deadline")
        require(len(records) < PAYLOAD_MEMBERS and depth <= PAYLOAD_DEPTH, "payload membership/depth bound")
        linked = lambda: path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        info = linked()
        if depth == 0:
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == tuple(expected), "payload original root changed")
        name = path.relative_to(root).as_posix()
        record = {"identity": stable_named_entry(info)}
        reparse = getattr(info, "st_file_attributes", 0) & 0x400
        if stat.S_ISLNK(info.st_mode) or reparse:
            require(not reparse or getattr(info, "st_reparse_tag", 0) in (0xA0000003, 0xA000000C), "unknown payload reparse point")
            target = os.readlink(path) if parent is None else os.readlink(path.name, dir_fd=parent)
            require(len(target) <= 4096 and stable_file(linked()) == stable_file(info), "payload link changed/bound")
            record.update(type="link", target=target)
            links += 1
        elif stat.S_ISREG(info.st_mode):
            require(0 <= info.st_size <= PAYLOAD_BYTES - total and info.st_nlink > 0, "payload byte bound")
            total += info.st_size
            record.update(type="file")
        else:
            require(stat.S_ISDIR(info.st_mode), "nonordinary payload member")
            record["type"] = "directory"
        records[name] = record
        if record["type"] == "directory":
            descriptor = open_inventory_directory(path, parent)
            try:
                held = os.fstat(descriptor)
                check_inventory_directory(info, held, "payload directory changed at open")
                verify_inventory_directory(path, info, descriptor, held, parent)
                with os.scandir(path if artifacts.WINDOWS else descriptor) as entries:
                    for entry in entries:
                        visit(path / entry.name, depth + 1, None if artifacts.WINDOWS else descriptor)
                verify_inventory_directory(path, info, descriptor, held, parent)
            finally:
                os.close(descriptor)

    visit(root, 0)
    source_options = {"source": source} if source is not None else {}
    payload_links(root, records, deadline, **source_options)
    payload_hardlink_closure(records, deadline, **source_options)
    documents = payload_hash_files(root, records, expected, deadline, **source_options)
    digest = hashlib.sha256()
    for name, record in sorted(records.items()):
        require(time.monotonic() < deadline, "payload scan deadline")
        digest.update(json.dumps([name, record], sort_keys=True, ensure_ascii=True).encode() + b"\n")
    require(directory_identity(root) == tuple(expected), "payload original root changed")
    result = {"sha256": digest.hexdigest(), "entries": len(records), "logical_regular_bytes": total, "links": links}
    if source is not None:
        result["ledger"] = source_ledger(records, source, deadline)
        result["_records"] = records  # Private join authority, never an evidence member.
        result["_documents"] = documents  # Only the two requested bounded PM facts.
    return result


def below(name, root):
    return name == root or name.startswith(root + "/")


def source_project(name, source):
    return next((root for root in source["projects"] if below(name, root)), None)


def source_link_boundary(origin, target, source, *, intermediate=False):
    """A shared capture envelope is never shared link authority."""
    project = source_project(origin, source)
    allowed = [project, source["source"], source["tools"]] if project else [source["user"]]
    require(project is not None or below(origin, source["user"]), "source link outside an owned runtime")
    require(any(below(target, root) or (intermediate and (target == "." or below(root, target))) for root in allowed),
            "source link crosses runtime authority")


def source_hardlink_boundary(names, source):
    projects = {source_project(name, source) for name in names} - {None}
    require(len(projects) <= 1, "source hardlink crosses Project roots")
    if projects:
        allowed = [*projects, source["source"], source["tools"]]
        require(all(any(below(name, root) for root in allowed) for name in names),
                "source hardlink crosses original User authority")
    elif len(names) > 1:
        require(all(below(name, source["user"]) for name in names), "source hardlink crosses fixture authority")


def source_layout(outer, user, original, source, *, protected=(), sentinels=()):
    require(user == outer / "user" and source == original / "hermes-agent"
            and original.is_relative_to(user) and source.resolve() == source,
            "source installation layout differs")
    relative = lambda path: path.relative_to(outer).as_posix()
    return {"user": relative(user), "source": relative(source), "tools": relative(original / "tools"),
            "installs": relative(original / "installs"), "protected": tuple(protected),
            "projects": [relative(outer / ("project-" + role) / ".uap/hermes-runtime") for role in ("a", "b")],
            "sentinels": tuple(value.encode("utf-8") for value in sentinels)}


def source_user_digest(records, user, deadline=None):
    digest, entries = hashlib.sha256(), 0
    for name, record in sorted(records.items()):
        require(deadline is None or time.monotonic() < deadline, "payload scan deadline")
        if below(name, user):
            digest.update(json.dumps([name, record], sort_keys=True, ensure_ascii=True).encode() + b"\n")
            entries += 1
    require(0 < entries <= PAYLOAD_MEMBERS, "source User census absent or oversized")
    return {"sha256": digest.hexdigest(), "entries": entries}


SOURCE_DELTA_FIELDS = tuple("device inode mode links bytes mtime_ns ctime_ns birthtime_ns attributes reparse_tag uid gid".split())


def source_delta_record(record):
    if record is None:
        return None
    identity = record["identity"]
    require(type(identity) is tuple and len(identity) in (9, 12), "source delta identity shape")
    values = (*identity[:7], *(identity[7:10] if len(identity) == 12 else (None, None, None)), *identity[-2:])
    bounded = lambda number: number if type(number) is int and 0 <= number < 2**64 else None
    return {"kind": record["type"], "metadata": {key: bounded(number) for key, number in zip(SOURCE_DELTA_FIELDS, values)},
            "integer_saturated": any(number is not None and bounded(number) is None for number in values),
            "sha256": record.get("sha256") if record["type"] == "file" else None,
            "target_sha256": hashlib.sha256(record["target"].encode("utf-8", "surrogatepass")).hexdigest() if record["type"] == "link" else None}


def source_user_delta(before, after, user, nonce, before_user, after_user, deadline):
    """Compare accepted captures only; this never reopens a payload member."""
    require(user == "user" and all(type(rows) is dict and 0 < len(rows) <= PAYLOAD_MEMBERS for rows in (before, after)), "source delta captured maps required")
    require(source_user_digest(before, user, deadline) == before_user
            and source_user_digest(after, user, deadline) == after_user, "source delta captured digest differs")
    rows, total = [], 0
    for name in sorted(before.keys() | after.keys()):
        require(time.monotonic() < deadline, "source delta deadline")
        if not below(name, user) or before.get(name) == after.get(name):
            continue
        total += 1
        if len(rows) < 8:
            rows.append({"member": name if len(json.dumps(name, ensure_ascii=True).encode("ascii")) <= 512 else None,
                         "member_sha256": hashlib.sha256(name.encode("utf-8", "surrogatepass")).hexdigest(),
                         "before": source_delta_record(before.get(name)), "after": source_delta_record(after.get(name))})
    value = dict(schema_version=1, measurement_only=True, admission_qualified=False, nonce=nonce,
                 stage="parent-after-settlement", hash_encoding="utf-8-surrogatepass",
                 before=dict(before_user), after=dict(after_user), total_changes=total, truncated=total > len(rows), changes=rows)
    while len(json.dumps(value).encode() + b"\n") > 16 << 10 and rows:
        rows.pop(); value["truncated"] = True
    source_delta_document(json.dumps(value).encode() + b"\n")
    require(time.monotonic() < deadline, "source delta deadline")
    return value


def source_delta_document(body):
    require(type(body) is bytes and 0 < len(body) <= 16 << 10, "source delta evidence bound")
    value = census_document(body)
    closed = lambda row, keys: type(row) is dict and set(row) == set(keys.split())
    digest = lambda text: type(text) is str and re.fullmatch(r"[0-9a-f]{64}", text) is not None
    number = lambda item: type(item) is int and 0 <= item < 2**64
    require(closed(value, "schema_version measurement_only admission_qualified nonce stage hash_encoding before after total_changes truncated changes")
            and type(value["schema_version"]) is int and value["schema_version"] == 1 and value["measurement_only"] is True
            and value["admission_qualified"] is False and digest(value["nonce"]) and value["stage"] == "parent-after-settlement"
            and value["hash_encoding"] == "utf-8-surrogatepass", "source delta schema differs")
    require(all(closed(value[key], "sha256 entries") and digest(value[key]["sha256"])
                and number(value[key]["entries"]) and 0 < value[key]["entries"] <= PAYLOAD_MEMBERS for key in ("before", "after")), "source delta summaries differ")
    rows, total = value["changes"], value["total_changes"]
    require(type(rows) is list and len(rows) <= 8 and number(total) and len(rows) <= total <= 2 * PAYLOAD_MEMBERS
            and type(value["truncated"]) is bool and value["truncated"] == (total > len(rows))
            and (total == 0) == (value["before"] == value["after"]), "source delta counters differ")
    for row in rows:
        require(closed(row, "member member_sha256 before after") and digest(row["member_sha256"]), "source delta row differs")
        name = row["member"]
        require(name is None or (type(name) is str and below(name, "user") and ":" not in name and "\\" not in name
                and all(part not in ("", ".", "..") for part in name.split("/")) and len(json.dumps(name).encode()) <= 512
                and hashlib.sha256(name.encode("utf-8", "surrogatepass")).hexdigest() == row["member_sha256"]), "source delta member differs")
        require(row["before"] is not None or row["after"] is not None, "source delta empty row")
        for record in (item for item in (row["before"], row["after"]) if item is not None):
            require(closed(record, "kind metadata integer_saturated sha256 target_sha256")
                    and record["kind"] in ("file", "directory", "link") and type(record["integer_saturated"]) is bool
                    and type(record["metadata"]) is dict and set(record["metadata"]) == set(SOURCE_DELTA_FIELDS)
                    and all(item is None or number(item) for item in record["metadata"].values())
                    and (record["integer_saturated"] or all(number(record["metadata"][key]) for key in SOURCE_DELTA_FIELDS if key not in ("birthtime_ns", "attributes", "reparse_tag")))
                    and (digest(record["sha256"]) if record["kind"] == "file" else record["sha256"] is None)
                    and (digest(record["target_sha256"]) if record["kind"] == "link" else record["target_sha256"] is None), "source delta record differs")
        require(row["before"] != row["after"] or row["before"]["integer_saturated"], "source delta unchanged row")
    require(len({row["member_sha256"] for row in rows}) == len(rows), "source delta duplicate member")
    return value


def retain_source_user_delta(evidence, expected, nonce, prepared, after, before_records, after_records, user, deadline, *, failure, settled):
    if not isinstance(failure, BaseException) or settled is not True or prepared is None or after is None or not before_records or not after_records:
        return
    try:
        value = source_user_delta(before_records, after_records, user, nonce, prepared["user"], after["user"], deadline)
        with held_census_evidence(evidence, expected) as (descriptor, verify):
            verify()
            require(time.monotonic() < deadline, "source delta deadline")
            write_census_member(evidence / "source-user-delta.json", value, None if artifacts.WINDOWS else descriptor)
    except (OSError, KeyError, TypeError, ValueError, RuntimeError):
        pass  # No repaired baseline or replaced original failure, including partial writes.


def source_ledger(records, source, deadline):
    """Partition every captured member once; large runtime trees stay hashed.

    Original User data is covered even when it is outside the three native PM
    trees. Protected fixture members are separate rows from the same capture,
    never independently reread or silently excluded from the authority ledger.
    """
    partitions = [source[key] for key in ("user", "source", "tools", "installs")]
    partitions += [root + suffix for root in source["projects"] for suffix in ("/tools", "/installs", "/cache/uv")]
    groups, ledger, ordinary = {}, {}, 0
    for name, record in sorted(records.items()):
        require(time.monotonic() < deadline, "payload scan deadline")
        matches = [root for root in partitions if below(name, root)] if name not in source["protected"] else []
        if not matches:
            ordinary += 1
            try:
                require(ordinary <= 2048 and (record["type"] != "file" or record["identity"][4] <= LIMIT),
                        "source ordinary profile bound")
            except RuntimeError as error:
                source_payload_refusal(error, name, record, profile_bound=dict(
                    scope=source_bound_scope(name), kind=record["type"], ordinary_count=ordinary,
                    predicate=source_bound_predicate(record["type"], record["identity"][4], ordinary)))
                raise
            ledger[name] = record
            continue
        prefix = max(matches, key=len)
        if prefix not in groups:
            groups[prefix] = {"digest": hashlib.sha256(), "entries": 0, "logical_regular_bytes": 0, "links": 0}
        group = groups[prefix]
        group["digest"].update(json.dumps([name, record], sort_keys=True, ensure_ascii=True).encode() + b"\n")
        group["entries"] += 1
        group["logical_regular_bytes"] += record["identity"][4] if record["type"] == "file" else 0
        group["links"] += record["type"] == "link"
    for prefix, group in groups.items():
        digest = group.pop("digest")
        ledger[prefix] = dict(type="runtime-tree", sha256=digest.hexdigest(), **group)
    require(sum(group["entries"] for group in groups.values()) + ordinary == len(records), "source partition coverage differs")
    require(len(json.dumps(ledger, ensure_ascii=True).encode()) <= LIMIT, "source ledger output bound")
    return ledger


def source_capture_mode():
    value = os.environ.get("HERMES_PROFILE_SOURCE_CAPTURE")
    require(value in (None, "1"), "unknown source capture mode")
    return value == "1"


def source_inputs(runner, source):
    """Read the actual official installer outputs; never synthesize PM facts."""
    home, user = runner / "hermes-native-home", runner / "hermes-native-home/user"
    original = user / ("AppData/Local/hermes" if os.name == "nt" else ".hermes")
    require(os.environ["HOME"] == str(user) and os.environ["USERPROFILE"] == str(user)
            and os.environ["HERMES_HOME"] == str(original) and source == original / "hermes-agent",
            "preserved installed User/source authority differs")
    for path in (home, user, original, source, original / "tools", original / "installs"):
        directory_identity(path)
    command_file = Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])
    require(command_file.is_absolute() and command_file.is_relative_to(runner), "official runtime command receipt escapes fixture")
    command_parent, command_info = directory_identity(command_file.parent), command_file.lstat()
    with held_census_evidence(command_file.parent, command_parent) as (descriptor, verify):
        body = ordinary_bytes(command_file, command_info, parent=None if artifacts.WINDOWS else descriptor)
        verify()
    argv = json.loads(body.decode("utf-8-sig"))
    require(type(argv) is list and len(argv) == 4 and all(type(value) is str for value in argv)
            and argv[1:3] == ["-I", "-c"], "official source runtime command differs")
    native, python = Path(os.environ["HERMES_BIN"]), Path(argv[0])
    require(native.is_absolute() and native.is_relative_to(user) and native.resolve().is_relative_to(user)
            and python.is_absolute() and python.is_relative_to(original / "tools")
            and python.resolve().is_relative_to(original / "tools"), "installed source launcher/interpreter escaped owner")
    install_key = hashlib.sha256(str(source).encode()).hexdigest()[:16]
    facts = original / "installs" / install_key / "facts.json"
    provenance_members = sorted({path.relative_to(home).as_posix() for path in
                                 (native, native.resolve(), python, python.resolve(), facts, original / "tools/facts.json")})
    layout = source_layout(home, user, original, source)
    return {"home": home, "user": user, "original": original, "source": source, "native": native,
            "python": python, "facts": facts, "layout": layout, "runtime_argv": argv, "provenance_members": provenance_members,
            "command_file": command_file, "command_parent": command_parent, "command_info": command_info,
            "command_sha256": hashlib.sha256(body).hexdigest()}


def verify_source_capture_paths(home, expected, records, names, deadline, *, hash_members=()):
    """Rejoin captured ancestors/leaves and optionally verify their pinned bytes."""
    require(type(hash_members) is tuple and set(hash_members) <= set(names), "captured byte verification differs")
    with held_census_evidence(home, expected) as (root, verify):
        require(stable_named_entry(home.lstat()) == records["."]["identity"], "source captured outer changed")
        for name in names:
            require(time.monotonic() < deadline, "payload scan deadline")
            parts = Path(name).parts
            require(parts and not Path(name).is_absolute() and not any(part in {"", ".", ".."} for part in parts), "source captured member spelling differs")
            with ExitStack() as stack:
                parent, path, held_directories = root, home, []
                for index, part in enumerate(parts):
                    verify()
                    path = path / part
                    record = records.get(path.relative_to(home).as_posix())
                    linked_parent = None if artifacts.WINDOWS else parent
                    info = path.lstat() if linked_parent is None else os.stat(part, dir_fd=parent, follow_symlinks=False)
                    require(record is not None and stable_named_entry(info) == record["identity"], "source input changed after capture")
                    if index < len(parts) - 1 or record["type"] == "directory":
                        require(record["type"] == "directory", "source captured ancestor is aliased")
                        descriptor = open_inventory_directory(path, linked_parent)
                        stack.callback(os.close, descriptor)
                        held = os.fstat(descriptor)
                        check_inventory_directory(info, held, "source captured ancestor changed at open")
                        verify_inventory_directory(path, info, descriptor, held, linked_parent)
                        held_directories.append((path, info, descriptor, held, linked_parent))
                        parent = descriptor
                    elif record["type"] == "link":
                        target = os.readlink(path) if linked_parent is None else os.readlink(part, dir_fd=parent)
                        after = path.lstat() if linked_parent is None else os.stat(part, dir_fd=parent, follow_symlinks=False)
                        require(target == record["target"] and stable_named_entry(after) == record["identity"], "source captured link target changed")
                    elif name in hash_members:
                        require(record["type"] == "file", "source captured byte verification is not a file")
                        for authority in reversed(held_directories):
                            verify_inventory_directory(*authority)
                        verify()
                        ordinary_bytes(path, info, parent=linked_parent,
                                       pin=(record["identity"][4], record["sha256"]), deadline=deadline)
                for authority in reversed(held_directories):
                    verify_inventory_directory(*authority)
                verify()
        verify()


def source_runtime_snapshot(inputs, expected, *, retain=None, deadline=None):
    home, layout = inputs["home"], inputs["layout"]
    tools = inputs["original"] / "tools"
    document_names = tuple(path.relative_to(home).as_posix() for path in (inputs["facts"], tools / "facts.json"))
    deadline = time.monotonic() + 120 if deadline is None else deadline
    snapshot = payload_snapshot(home, expected, deadline, source=dict(layout, documents=document_names))
    records = snapshot.pop("_records")
    documents = snapshot.pop("_documents")
    verify_source_capture_paths(home, expected, records, document_names, deadline)
    facts, store = (json.loads(documents[name].decode("utf-8-sig")) for name in document_names)
    entry = store.get("packages", {}).get("python", {}).get("entry")
    require(type(entry) is str and entry and not Path(entry).is_absolute()
            and ".." not in Path(entry).parts, "installed Python tool selection differs")
    require(inputs["python"] == tools / entry / ("python.exe" if os.name == "nt" else "bin/python3"),
            "installed interpreter does not match committed tool facts")
    environment = facts.get("packages", {}).get("venv", {}).get("environment")
    require(type(environment) is str, "official installed dependency selection absent")
    environment = Path(environment)
    require(environment.is_absolute() and os.path.normpath(str(environment)) == str(environment)
            and environment.is_relative_to(inputs["facts"].parent / "environments"), "installed dependency generation escaped owner")
    generation, venv = (path.relative_to(home).as_posix() for path in (environment, environment / "pyvenv.cfg"))
    require(records.get(generation, {}).get("type") == "directory", "installed dependency generation is not a captured directory")
    metadata = records.get(venv, {})
    require(metadata.get("type") == "file" and metadata["identity"][3] == 1
            and metadata["identity"][4] <= LIMIT, "installed pyvenv metadata differs")
    ledger = snapshot.pop("ledger")
    provenance_members = inputs["provenance_members"]
    require(all(name in records and records[name]["type"] in {"file", "link"} for name in provenance_members), "installed provenance member missing")
    immutable = {name: record for name, record in ledger.items()
                 if any(below(name, layout[key]) for key in ("source", "tools", "installs"))}
    # These provenance views join the same capture; the ledger partition still
    # counts each member once, including large interpreter binaries and links.
    immutable.update({name: records[name] for name in provenance_members})
    user = source_user_digest(records, layout["user"], deadline)
    verify_source_capture_paths(home, expected, records, (*provenance_members, generation, venv), deadline)
    if retain is not None:
        require(type(retain) is dict and not retain, "source private census destination differs")
        retain.update(records)
    return {"inventory": snapshot, "immutable": immutable, "user": user,
            "provenance": {"source": str(inputs["source"]), "native_command": [str(inputs["native"])],
                           "python": str(inputs["python"]), "selected_environment": str(environment),
                           "runtime_command_sha256": inputs["command_sha256"],
                           "native_launch": {"driver": "official-runtime-command", "bytecode_disabled": True,
                                             "argv_prefix": list(source_native_prefix(inputs))},
                           "captured_members": {name: records[name] for name in provenance_members}}}


def prepare_user_fixture(original, authored, *, source_capture=False):
    """Retain official config preimages before authoring this disposable fixture."""
    retained = {}
    for name, body in authored.items():
        path = original / name
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists() or path.is_symlink():
            require(source_capture and name in {"config.yaml", ".env"}, "original User fixture collision")
            before = path.lstat()
            previous = ordinary_bytes(path, before)
            preserved = original / ("uap-installer-preimage-" + name.lstrip("."))
            require(not preserved.exists() and not preserved.is_symlink(), "installer preimage destination collision")
            path.rename(preserved)
            after = preserved.lstat()
            old, new = stable_file(before), stable_file(after)
            require(old[:6] + old[7:] == new[:6] + new[7:] and ordinary_bytes(preserved, after) == previous,
                    "installer preimage changed during fixture preparation")
            retained[name] = {"member": preserved.name, "bytes": len(previous), "sha256": hashlib.sha256(previous).hexdigest(),
                              "before_identity": old, "retained_identity": new}
        with path.open("xb") as stream:
            stream.write(body.encode("utf-8"))
        path.chmod(0o600)
    return retained


def user_fixture_content(nonce):
    sentinels = ["USER_" + kind + "_" + nonce for kind in ("MODEL", "PROVIDER", "AUTH", "HISTORY", "FOREIGN")]
    authored = {"config.yaml": "model: " + sentinels[0] + "\nproviders:\n  custom:\n    base_url: http://127.0.0.1:9/" + sentinels[1] + "\nmcp_servers:\n  foreign:\n    command: echo\n    args: [retained]\n",
                "auth.json": json.dumps({"version": 1, "providers": {}, "admission_marker": sentinels[2]}),
                ".env": "# inert admission; no credentials\n",
                "sessions/retained-history.txt": sentinels[3],
                "skills/foreign/SKILL.md": "---\nname: foreign\ndescription: Retained foreign User Skill.\n---\n" + sentinels[4]}
    return sentinels, authored


def source_preparation_directories(inputs, authored):
    home, user, original = (inputs[key] for key in ("home", "user", "original"))
    targets = [user / folder for _, folder in BASE["PROFILE_DIRECTORIES"]]
    targets += [(original / name).parent for name in authored]
    result = set()
    for directory in targets:
        require(directory.is_relative_to(user), "source fixture preparation escaped original User")
        while directory.is_relative_to(user):
            result.add(directory.relative_to(home).as_posix())
            if directory == user:
                break
            directory = directory.parent
    return result


def source_preparation_relation(before, after, inputs, authored, retained, directories, deadline):
    """Account for every original record and only the declared fixture writes."""
    home, original = inputs["home"], inputs["original"]
    relative = lambda path: path.relative_to(home).as_posix()
    files = {relative(original / name): body.encode("utf-8") for name, body in authored.items()}
    renames = {relative(original / name): relative(original / ("uap-installer-preimage-" + name.lstrip(".")))
               for name in ("config.yaml", ".env") if relative(original / name) in before}
    require(set(retained) == {Path(name).name for name in renames}, "source fixture preimage set differs")
    require(not (set(renames.values()) & before.keys()), "source fixture preimage overwrote an original member")
    require(after.keys() == before.keys() | files.keys() | set(renames.values()) | directories,
            "source fixture preparation changed undeclared membership")
    def child_names(records):
        children = {name: set() for name in directories}
        for name in records:
            require(time.monotonic() < deadline, "payload scan deadline")
            parent = Path(name).parent.as_posix()
            if name != "." and parent in children:
                children[parent].add(name)
        return children
    old_children, new_children = child_names(before), child_names(after)
    changed = set(files) | set(renames.values()) | directories
    for name, record in before.items():
        require(time.monotonic() < deadline, "payload scan deadline")
        if name not in changed:
            require(after.get(name) == record, "source fixture preparation changed an original User/runtime member")
    for name, body in files.items():
        require(name not in before or name in renames, "source fixture preparation adopted an original protected member")
        record = after[name]
        mode = 0o666 if os.name == "nt" else 0o600
        require(record["type"] == "file" and record["identity"][3] == 1
                and stat.S_IMODE(record["identity"][2]) == mode and record["identity"][4] == len(body)
                and record["sha256"] == hashlib.sha256(body).hexdigest()
                and record["identity"][-2:] == after[Path(name).parent.as_posix()]["identity"][-2:],
                "source authored fixture postimage differs")
    for old_name, new_name in renames.items():
        old, new = before[old_name], after[new_name]
        require(old["type"] == new["type"] == "file" and old["identity"][3] == new["identity"][3] == 1
                and old["sha256"] == new["sha256"]
                and old["identity"][:6] + old["identity"][7:] == new["identity"][:6] + new["identity"][7:],
                "source installer preimage changed during declared rename")
        report = retained[Path(old_name).name]
        require(relative(original / report["member"]) == new_name and report["sha256"] == old["sha256"]
                and report["bytes"] == old["identity"][4]
                and tuple(report["before_identity"]) == old["identity"]
                and tuple(report["retained_identity"]) == new["identity"], "source preimage receipt differs from captured records")
    for name in directories:
        record = after[name]
        require(record["type"] == "directory", "source preparation directory was replaced")
        if name in before:
            previous = before[name]
            require(previous["type"] == "directory", "source preparation adopted a non-directory")
            if old_children[name] == new_children[name]:
                require(record == previous, "source preparation changed an untouched original directory")
            else:
                require(record["identity"][:3] + record["identity"][7:] == previous["identity"][:3] + previous["identity"][7:],
                        "source preparation replaced original directory authority")
        else:
            require(stat.S_IMODE(record["identity"][2]) == (0o777 if os.name == "nt" else 0o700),
                    "source preparation directory mode differs")
            require(record["identity"][-2:] == after[Path(name).parent.as_posix()]["identity"][-2:],
                    "source preparation directory owner differs")


def prepare_source_user(inputs, original_records, evidence, nonce, expected, *, retain=None):
    deadline = time.monotonic() + 120
    _, authored = user_fixture_content(nonce)
    directories = source_preparation_directories(inputs, authored)
    existing = [name for name in directories if name in original_records]
    require(all(original_records[name]["type"] == "directory" for name in existing), "source preparation parent is not an original ordinary directory")
    verify_source_capture_paths(inputs["home"], expected, original_records, existing, deadline)
    for name in sorted(directories, key=lambda value: (len(Path(value).parts), value)):
        path = inputs["home"] / name
        if name not in original_records:
            path.mkdir(mode=0o700)
    env = BASE["closed_environment"](inputs["user"], evidence, nonce, "http://127.0.0.1:9/unused")
    retained = prepare_user_fixture(inputs["original"], authored, source_capture=True)
    prepared = {}
    snapshot = source_runtime_snapshot(inputs, expected, retain=prepared, deadline=deadline)
    source_preparation_relation(original_records, prepared, inputs, authored, retained, directories, deadline)
    receipt = {"schema_version": 1, "nonce": nonce, "outer_identity": expected, "admission_qualified": False,
               "original_user": source_user_digest(original_records, inputs["layout"]["user"], deadline),
               "prepared_user": snapshot["user"], "installer_preimages": retained,
               "declared_directories": sorted(directories), "authored_members": sorted(authored)}
    env["HERMES_SOURCE_PREPARATION_SHA256"] = write_json(evidence / "source-user-preparation.json", receipt)
    if retain is not None:
        require(type(retain) is dict and not retain, "source prepared capture destination differs")
        retain.update(prepared)
    return env, snapshot, receipt


def require_source_user_baseline(captured, prepared):
    require(captured == prepared, "original source User changed before the first Project command or across the native family")


def source_preparation_document(body):
    """Only fixed fixture names, hashes and captured identities may be exported."""
    value = census_document(body)
    require(type(value) is dict and set(value) == {
        "schema_version", "nonce", "outer_identity", "admission_qualified", "original_user",
        "prepared_user", "installer_preimages", "declared_directories", "authored_members"},
        "source preparation receipt schema differs")
    def digest(text):
        return type(text) is str and re.fullmatch(r"[0-9a-f]{64}", text) is not None
    require(type(value["schema_version"]) is int and value["schema_version"] == 1
            and digest(value["nonce"]) and value["admission_qualified"] is False,
            "source preparation receipt qualification differs")
    require(type(value["outer_identity"]) is list and len(value["outer_identity"]) == 5
            and all(type(number) is int and 0 <= number < 2**64 for number in value["outer_identity"]),
            "source preparation root identity differs")
    for name in ("original_user", "prepared_user"):
        record = value[name]
        require(type(record) is dict and set(record) == {"sha256", "entries"}
                and type(record["entries"]) is int and 0 < record["entries"] <= PAYLOAD_MEMBERS
                and digest(record["sha256"]), "source preparation User digest differs")
    _, authored = user_fixture_content(value["nonce"])
    require(value["authored_members"] == sorted(authored), "source preparation authored names differ")
    # Validate both portable artifact spellings without touching a filesystem.
    home = Path("fixture")
    directory_sets = [sorted(source_preparation_directories(
        {"home": home, "user": home / "user", "original": home / "user" / relative}, authored))
        for relative in (".hermes", "AppData/Local/hermes")]
    require(value["declared_directories"] in directory_sets, "source preparation declared directories differ")
    preimages = value["installer_preimages"]
    require(type(preimages) is dict and set(preimages) <= {"config.yaml", ".env"},
            "source preparation preimage names differ")
    for name, record in preimages.items():
        require(type(record) is dict and set(record) == {"member", "bytes", "sha256", "before_identity", "retained_identity"}
                and record["member"] == "uap-installer-preimage-" + name.lstrip(".")
                and type(record["bytes"]) is int and 0 <= record["bytes"] <= LIMIT and digest(record["sha256"]),
                "source preparation preimage metadata differs")
        before, retained = record["before_identity"], record["retained_identity"]
        for identity in (before, retained):
            require(type(identity) is list and len(identity) in (9, 12)
                    and all(type(number) is int and -(2**63) <= number < 2**64 for number in identity)
                    and all(number >= 0 for index, number in enumerate(identity)
                            if index not in ((5, 6, 7) if len(identity) == 12 else (5, 6)))
                    and stat.S_ISREG(identity[2]) and identity[3] == 1 and identity[4] == record["bytes"],
                    "source preparation preimage identity differs")
            if len(identity) == 12:
                require(identity[7] > 0 and 0 <= identity[8] < 2**32 and not identity[8] & 0x400
                        and identity[9] == 0, "source preparation preimage reparse metadata differs")
        require(before[:6] + before[7:] == retained[:6] + retained[7:], "source preparation preimage rename differs")
    return value


def read_source_preparation(evidence, expected, nonce, digest):
    with held_census_evidence(evidence, expected) as (descriptor, verify):
        body = ordinary_bytes(evidence / "source-user-preparation.json", parent=None if artifacts.WINDOWS else descriptor)
        require(hashlib.sha256(body).hexdigest() == digest, "source preparation receipt changed")
        value = source_preparation_document(body)
        require(value["nonce"] == nonce, "source preparation receipt nonce differs")
        verify()
        return value


def payload_contract(payload, source, target):
    """Join official builder inputs and completed contained runtime selection."""
    manifest_bytes = ordinary_bytes(payload / "manifest.json")
    prepared_path = payload.with_name(payload.name + ".prepared.json")
    prepared_identity = prepared_path.lstat()
    prepared_bytes = ordinary_bytes(prepared_path, prepared_identity)
    manifest, prepared = json.loads(manifest_bytes), json.loads(prepared_bytes)
    require(manifest["schema"] == prepared["schema"] == 1 and manifest["ref"] == prepared["revision"] == BASE["PIN"]
            and prepared["out"] == str(payload) and prepared["source"] == str(source)
            and prepared["lock"] == hashlib.sha256(ordinary_bytes(source / "pm/lock.json")).hexdigest(), "payload source/preparation differs")
    inputs, runtime = prepared["inputs"], manifest["runtime"]
    require(manifest["target"] == inputs["target"] == target and inputs["ref"] == BASE["PIN"]
            and manifest["repo"] == inputs["repo"] == runtime["repoDir"] == "hermes-agent"
            and manifest["venv"] == "venv" and manifest["store"] == runtime["toolsDir"] == "tools"
            and inputs["placement"] == "contained" and not (payload / "hermes-agent/.git").exists(), "payload layout/target differs")
    selected = {"code": runtime["repoDir"], "python": runtime["storePython"],
                "site_packages": runtime["sitePackages"], "environment": "venv", "tools": "tools"}
    for key, relative in selected.items():
        require(isinstance(relative, str) and relative and Path(relative).as_posix() == relative
                and not Path(relative).is_absolute() and ".." not in Path(relative).parts, "payload selection is not relative")
        path = payload / relative
        require(inputs[key] == str(path) and path.resolve().is_relative_to(payload)
                and (path.is_file() if key == "python" else path.is_dir()), "payload selected input escapes/differs")
    require(selected["python"].startswith("tools/") and selected["site_packages"].startswith("venv/"), "payload interpreter/dependencies misplaced")
    launcher = "bin/hermes" + (".exe" if target.startswith("win32-") else "")
    require(runtime["commands"]["hermes"] == launcher and "hermes" in manifest["launchers"], "payload native launcher differs")
    launcher_path = payload / launcher
    require(launcher_path.resolve() == launcher_path and stat.S_ISREG(launcher_path.lstat().st_mode), "payload launcher alias")
    return {"runtime_distribution": "bundled", "source_install_custom_root_qualified": False,
            "ref": manifest["ref"], "target": target, "selected": selected, "native_command": [str(launcher_path)],
            "launcher_sha256": hashlib.sha256(ordinary_bytes(launcher_path)).hexdigest(),
            "manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
            "prepared_sha256": hashlib.sha256(prepared_bytes).hexdigest(), "prepared_identity": stable_file(prepared_identity),
            "lock_sha256": prepared["lock"]}


CENSUS_MEMBERS = 131072
CENSUS_CAP = (1 << 53) - 1
CENSUS_GROUPS = ("cache", "tools", "installs", "bootstrap-other", "projectA", "projectB", "User-other", "outer-other")
CENSUS_KINDS = ("directory", "regular", "symlink", "reparse", "other")


def census_bucket(parts):
    if parts[:2] == ("user", "selected-profile"):
        return parts[2] if len(parts) > 2 and parts[2] in ("cache", "tools", "installs") else "bootstrap-other"
    return {"project-a": "projectA", "project-b": "projectB", "user": "User-other"}.get(parts[0] if parts else "", "outer-other")


def metadata_census(root, expected, deadline):
    """Measurement only: no file contents, link targets or pathname records."""
    groups = {name: dict(entries=0, kinds=dict.fromkeys(CENSUS_KINDS, 0), logical_regular_bytes=0,
                         max_regular_bytes=0, max_links=0, regular_hardlinked_entries=0) for name in CENSUS_GROUPS}
    result = dict(schema_version=1, measurement_only=True, admission_qualified=False, complete=False,
                  status="refused", reason="authority", overflow=False, groups=groups)
    recorded = 0
    class Partial(Exception):
        pass
    def saturate(value):
        if value > CENSUS_CAP:
            result["overflow"] = True
        return min(value, CENSUS_CAP)
    def budget(depth):
        if time.monotonic() >= deadline:
            raise Partial("time-limit")
        if recorded >= CENSUS_MEMBERS:
            raise Partial("member-limit")
        if depth > 32:
            raise Partial("depth-limit")
    def visit(path, parts, parent=None):
        nonlocal recorded
        budget(len(parts))
        info = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        if not parts:
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == tuple(expected), "census original root changed")
        reparse = artifacts.WINDOWS and bool(getattr(info, "st_file_attributes", 0) & 0x400)
        kind = ("reparse" if reparse else "symlink" if stat.S_ISLNK(info.st_mode) else
                "directory" if stat.S_ISDIR(info.st_mode) else "regular" if stat.S_ISREG(info.st_mode) else "other")
        group = groups[census_bucket(parts)]
        require(info.st_nlink >= 0 and info.st_size >= 0, "census invalid metadata")
        group["entries"] += 1; group["kinds"][kind] += 1; recorded += 1
        group["max_links"] = saturate(max(group["max_links"], info.st_nlink))
        if kind == "regular":
            group["regular_hardlinked_entries"] += int(info.st_nlink > 1)
            group["logical_regular_bytes"] = saturate(group["logical_regular_bytes"] + info.st_size)
            group["max_regular_bytes"] = saturate(max(group["max_regular_bytes"], info.st_size))
        if kind == "directory":
            descriptor = open_inventory_directory(path, parent)
            try:
                held = os.fstat(descriptor)
                check_inventory_directory(info, held, "census directory changed at open")
                verify_inventory_directory(path, info, descriptor, held, parent)
                with os.scandir(path if artifacts.WINDOWS else descriptor) as entries:
                    for entry in entries:
                        visit(path / entry.name, parts + (entry.name,), None if artifacts.WINDOWS else descriptor)
                verify_inventory_directory(path, info, descriptor, held, parent)
            finally:
                os.close(descriptor)
    try:
        require(directory_identity(root) == tuple(expected), "census original root differs")
        visit(root, ())
        require(directory_identity(root) == tuple(expected), "census original root changed")
        if time.monotonic() >= deadline:
            raise Partial("time-limit")
        result.update(status="complete" if not result["overflow"] else "partial", reason="none" if not result["overflow"] else "overflow")
        result["complete"] = not result["overflow"]
    except Partial as error:
        result.update(status="partial", reason=error.args[0])
    except (OSError, RuntimeError, ValueError):
        result.update(status="refused", reason="authority")
    result["entries"] = sum(group["entries"] for group in groups.values())
    result["logical_regular_bytes"] = saturate(sum(group["logical_regular_bytes"] for group in groups.values()))
    if result["overflow"]:
        result.update(complete=False, reason="overflow" if result["reason"] == "none" else result["reason"], status="partial" if result["status"] == "complete" else result["status"])
    require(result["entries"] == recorded and all(sum(group["kinds"].values()) == group["entries"] for group in groups.values()), "census counter join")
    return result


@contextmanager
def held_census_evidence(evidence, expected):
    require(directory_identity(evidence) == tuple(expected), "original census evidence changed")
    before = evidence.lstat()
    descriptor = open_inventory_directory(evidence)
    try:
        held = os.fstat(descriptor)
        require(same_inventory_directory(before, held), "census evidence changed at open")
        key = lambda info: (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
        def verify():
            require(directory_identity(evidence) == tuple(expected) and key(os.fstat(descriptor)) == key(held), "original census evidence changed")
        verify()
        yield descriptor, verify
        verify()
    finally:
        os.close(descriptor)


def publish_census_request(evidence, outer, nonce, expected):
    try:
        request = dict(schema_version=1, reason="initial-inventory-refused", nonce=nonce, root_identity=directory_identity(outer))
        with held_census_evidence(evidence, expected) as (descriptor, verify):
            verify()
            write_census_member(evidence / "bootstrap-census-request.json", request, None if artifacts.WINDOWS else descriptor)
    except (OSError, RuntimeError, ValueError):
        pass  # The original inventory exception remains authoritative.


def write_census_member(path, value, parent=None):
    body = json.dumps(value, sort_keys=True).encode() + b"\n"
    require(len(body) <= 16 << 10, "census evidence bound")
    descriptor = (os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600) if parent is None else
                  os.open(path.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent))
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(body)


def census_document(body):
    require(len(body) <= 16 << 10, "census evidence bound")
    def unique(pairs):
        value = {}
        for key, item in pairs:
            require(key not in value, "duplicate census request key")
            value[key] = item
        return value
    return json.loads(body, object_pairs_hook=unique)


def validate_census_member(name, body):
    value = census_document(body)
    def closed(row, keys):
        require(type(row) is dict and set(row) == set(keys.split()), "closed census schema")
    def integer(number, maximum=CENSUS_CAP):
        require(type(number) is int and 0 <= number <= maximum, "census integer")
    if name == "bootstrap-census-request.json":
        closed(value, "schema_version reason nonce root_identity")
        require(type(value["schema_version"]) is int and value["schema_version"] == 1
                and value["reason"] == "initial-inventory-refused"
                and type(value["nonce"]) is str and len(value["nonce"]) == 64
                and all(char in "0123456789abcdef" for char in value["nonce"])
                and type(value["root_identity"]) is list and len(value["root_identity"]) == 5, "census request schema")
        for number in value["root_identity"]: integer(number, (1 << 64) - 1)
        return
    minimal = "schema_version measurement_only admission_qualified complete status reason"
    detailed = " overflow groups entries logical_regular_bytes"
    require(type(value) is dict, "census object")
    closed(value, minimal + (detailed if "groups" in value else ""))
    require(type(value["schema_version"]) is int and value["schema_version"] == 1
            and value["measurement_only"] is True and value["admission_qualified"] is False
            and type(value["complete"]) is bool, "census qualification")
    if "groups" not in value:
        require(value["complete"] is False and value["status"] == "refused" and value["reason"] == "request", "census refusal")
        return value
    require(type(value["overflow"]) is bool and (value["status"], value["reason"]) in
            (("complete", "none"), ("partial", "time-limit"), ("partial", "member-limit"), ("partial", "depth-limit"), ("partial", "overflow"), ("refused", "authority")), "census status")
    require(value["complete"] == (value["status"] == "complete") and not (value["complete"] and value["overflow"]), "census completeness")
    closed(value["groups"], " ".join(CENSUS_GROUPS))
    for row in value["groups"].values():
        closed(row, "entries kinds logical_regular_bytes max_regular_bytes max_links regular_hardlinked_entries")
        closed(row["kinds"], " ".join(CENSUS_KINDS))
        for number in row["kinds"].values(): integer(number, CENSUS_MEMBERS)
        for key, number in row.items():
            if key != "kinds": integer(number)
        require(sum(row["kinds"].values()) == row["entries"] <= CENSUS_MEMBERS
                and row["regular_hardlinked_entries"] <= row["kinds"]["regular"], "census group join")
    integer(value["entries"], CENSUS_MEMBERS); integer(value["logical_regular_bytes"])
    require(value["reason"] != "member-limit" or value["entries"] == CENSUS_MEMBERS, "census member limit join")
    require(sum(row["entries"] for row in value["groups"].values()) == value["entries"]
            and min(CENSUS_CAP, sum(row["logical_regular_bytes"] for row in value["groups"].values())) == value["logical_regular_bytes"], "census total join")
    return value


def maybe_census(evidence, home, expected, nonce, settled, deadline, evidence_expected):
    with held_census_evidence(evidence, evidence_expected) as (descriptor, verify):
        parent = None if artifacts.WINDOWS else descriptor
        request = evidence / "bootstrap-census-request.json"
        try:
            info = request.lstat() if parent is None else os.stat(request.name, dir_fd=parent, follow_symlinks=False)
        except FileNotFoundError:
            return
        result = dict(schema_version=1, measurement_only=True, admission_qualified=False, complete=False, status="refused", reason="request")
        try:
            require(settled is True and info.st_size <= 16 << 10, "census settlement/request bound")
            body = ordinary_bytes(request, info, **({} if parent is None else {"parent": parent}))
            validate_census_member(request.name, body)
            value = census_document(body)
            require(value["nonce"] == nonce and tuple(value["root_identity"]) == tuple(expected), "census request identity")
            result = metadata_census(home, expected, min(deadline, time.monotonic() + 5))
        except (OSError, RuntimeError, ValueError):
            pass
        verify()
        write_census_member(evidence / "bootstrap-census.json", result, parent)


def changes(before, after):
    return {name: {"before": before.get(name), "after": after.get(name)}
            for name in sorted(before.keys() | after.keys()) if before.get(name) != after.get(name)}


def protected_unchanged(before, after, paths):
    for name in paths:
        require(before.get(name) == after.get(name), "protected original User changed: " + name)


def profile_tree(snapshot, prefix):
    return {name: record for name, record in snapshot.items() if name == prefix or name.startswith(prefix + "/")}


def user_control_boundary(before, after, prefix):
    """Admit only the pinned cold User control, after every Project action."""
    old, new = profile_tree(before, prefix), profile_tree(after, prefix)
    private_dir, public_dir = (0o777, 0o777) if os.name == "nt" else (0o700, 0o755)
    private_file, log_mode = (0o666, 0o666) if os.name == "nt" else (0o600, 0o644)
    secured = {prefix + suffix for suffix in ("", "/sessions", "/skills")}
    require(secured <= old.keys() and all(old[name]["type"] == "directory" and old[name]["mode"] == public_dir for name in secured),
            "User control directory preimage differs")
    for name, record in old.items():
        require(new.get(name) == ({**record, "mode": private_dir} if name in secured else record),
                "User control changed original object")
    directories = {name: private_dir for name in ("cron", "logs", "logs/curator", "memories", "pairing", "hooks", "image_cache", "audio_cache")}
    directories.update({"backups": public_dir, "backups/config": public_dir})
    added = {name[len(prefix) + 1:]: record for name, record in new.items() if name not in old}
    backups = [name for name in added if re.fullmatch(r"backups/config/config\.yaml\.good\.[0-9]{8}-[0-9]{6}", name)]
    require(len(backups) == 1, "User control backup count/name differs")
    config = old.get(prefix + "/config.yaml", {})
    require(config.get("type") == "file" and config.get("bytes") == 279 and config.get("mode") == private_file,
            "User control config preimage differs")
    files = {"SOUL.md": (private_file, 667, "36c1f5a2e92cd1d018311eaf4c8f1e8886672eae78212e033c681d0e3d5d506f"),
             "logs/agent.log": (log_mode, 0, hashlib.sha256(b"").hexdigest()),
             "logs/errors.log": (log_mode, 0, hashlib.sha256(b"").hexdigest()),
             backups[0]: (private_file, config["bytes"], config["sha256"])}
    # The pinned Windows concurrent handler opens lazily. Admit only the
    # observed absent pair; a partial pair or nonempty log still refuses.
    if (os.name == "nt" and "logs/agent.log" not in added
            and "logs/errors.log" not in added):
        del files["logs/agent.log"], files["logs/errors.log"]
    require(set(added) == set(directories) | set(files), "User control unknown/missing bootstrap member")
    for name, record in added.items():
        expected = ({"type": "directory", "mode": directories[name]} if name in directories else
                    dict(zip(("mode", "bytes", "sha256"), files[name]), type="file"))
        require({key: value for key, value in record.items() if key not in {"device", "inode"}} == expected,
                "User control bootstrap metadata differs")
    require((added[backups[0]]["device"], added[backups[0]]["inode"]) != (config["device"], config["inode"]),
            "User control backup aliases original config")


def profile_effect_boundary(label, scope, baseline, before, after, user, selected, *, source_capture=False):
    require(profile_tree(before, user) == profile_tree(baseline, user), "original User changed before command")
    delta = changes(before, after)
    if scope == "user":
        require(label == "list-user" and all(name == user or name.startswith(user + "/") for name in delta),
                "User control changed outside original User")
        if source_capture:
            # The source fixture is already installed. Unexpected User effects
            # are recorded by the caller, then refused; never renew a baseline.
            require(profile_tree(after, user) == profile_tree(baseline, user), "source User control changed the measured original User")
        else:
            user_control_boundary(before, after, user)
    else:
        require(scope in {"a", "b"} and profile_tree(after, user) == profile_tree(baseline, user),
                "Project command changed original User")
        require(all(name == selected or name.startswith(selected + "/") for name in delta),
                "Project command changed outside selected runtime")


def source_version_readout(body, source):
    """Join the complete pinned fast-version output, not just its banner."""
    newline = b"\r\n" if os.name == "nt" else b"\n"
    require(0 < len(body) <= LIMIT and body.endswith(newline), "source version output is incomplete")
    lines = body[:-len(newline)].split(newline)
    expected = ["Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d".encode(),
                ("Install directory: " + str(source)).encode(), b"Install method: git"]
    require(len(lines) in (5, 6) and lines[:3] == expected, "pinned source native version differs")
    number = rb"(?:0|[1-9][0-9]{0,3})"
    require(re.fullmatch(rb"Python: 3\." + number + rb"\." + number, lines[3]) is not None
            and re.fullmatch(rb"OpenAI SDK: " + number + rb"\." + number + rb"\." + number, lines[4]) is not None,
            "source version runtime metadata differs")
    require(len(lines) == 5 or lines[5] == "Update available — run 'hermes update'".encode(),
            "source version has an unobserved trailing notice")


def config_readout(kind, body, selected):
    if kind == "path":
        expected = str(Path(selected["root"]) / "config.yaml").encode("utf-8")
        require(body in (expected + b"\n", expected + b"\r\n"), "native config path differs")
        return
    require(kind == "mcp", "unknown native config readout")
    def unique(pairs):
        value = {}
        for name, item in pairs:
            require(name not in value, "native config duplicate JSON key")
            value[name] = item
        return value
    try:
        value = json.loads(body.decode("utf-8"), object_pairs_hook=unique)
    except (UnicodeError, ValueError) as error:
        raise RuntimeError("native config JSON framing differs") from error
    require(type(value) is dict and value == {selected["server"]: selected["entry"]}, "native config MCP source differs")


def validate_guards(value):
    require(set(value) == {"gateway_running", "default_gateway_pid", "served_profiles", "service_manager", "memory_provider"},
            "source guard fields differ")
    require(value["gateway_running"] is False and value["default_gateway_pid"] is None
            and value["served_profiles"] is None and value["service_manager"] != "s6"
            and value["memory_provider"] is None, "live shared gateway/service/memory authority refused")


def validate_roots(user_home, native_home, roots):
    user_home, native_home = Path(user_home), Path(native_home)
    for root in roots:
        root = Path(root)
        require(root.is_absolute() and root.resolve() == root and not root.is_symlink()
                and root.parent.name != "profiles" and not root.is_relative_to(native_home)
                and not native_home.is_relative_to(root) and root != user_home,
                "custom root is not independent original-User authority")
    require(len(set(map(str, roots))) == 2 and not roots[0].is_relative_to(roots[1])
            and not roots[1].is_relative_to(roots[0]), "A/B runtime roots overlap")


def observer_runtime_command(runtime_command, root, python, *, code, args=()):
    """Keep UAP observer imports from writing into the measured payload.

    The pinned runtime API uses -I, which ignores PYTHONDONTWRITEBYTECODE.
    Preserve its complete bootstrap and arguments for observer/resolution
    probes. Source native CLI commands have their separate frozen-prefix join;
    bundled native commands continue to use the literal published launcher.
    """
    argv = runtime_command(root, python=python, code=code, args=args)
    require(type(argv) is list and len(argv) == 4 + len(args)
            and all(type(value) is str for value in argv)
            and argv[:3] == [str(python), "-I", "-c"]
            and argv[4:] == list(args), "observer runtime command shape changed")
    return [argv[0], "-I", "-B", *argv[2:]]


def source_observer_runtime_command(api, inputs, snapshot, records, authority, *, code, args, deadline):
    """Run only our observer through PM's already-selected dependency interpreter."""
    home, source = inputs["home"], inputs["source"]
    provenance = snapshot["provenance"]
    require(provenance["source"] == str(source) and provenance["python"] == str(inputs["python"])
            and provenance["native_command"] == [str(inputs["native"])], "source observer provenance differs")
    facts = inputs["facts"].relative_to(home).as_posix()
    require(provenance["captured_members"][facts] == records[facts], "source observer facts are not the captured selection")
    environment = Path(provenance["selected_environment"])
    require(environment.is_absolute() and os.path.normpath(str(environment)) == str(environment)
            and environment.is_relative_to(inputs["facts"].parent / "environments"), "source observer generation escaped owner")
    interpreter = environment / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    api_source = source / "pm/environments.py"
    require(Path(api.__code__.co_filename) == api_source and api.__code__.co_name == "venv_command"
            and api.__globals__.get("__name__") == "pm.environments"
            and api.__globals__.get("__file__") == str(api_source), "source observer API is not pinned-source-owned")
    executable, config, api_name = (path.relative_to(home).as_posix() for path in (interpreter, environment / "pyvenv.cfg", api_source))
    resolved, links = payload_links(home, records, deadline, source=inputs["layout"], resolve=(executable,))[executable]
    require(records[resolved]["type"] == "file" and records[config]["type"] == "file"
            and records[api_name]["type"] == "file", "source observer inputs are not captured regular files")
    names = tuple(dict.fromkeys((facts, api_name, config, executable, *links, resolved)))
    hashed = tuple(dict.fromkeys((facts, api_name, config, resolved)))
    verify_source_capture_paths(home, authority, records, names, deadline, hash_members=hashed)
    prefix = api(source, environment, options=("-I", "-B"))
    require(type(prefix) is list and all(type(value) is str for value in prefix)
            and prefix == [str(interpreter), "-I", "-B"], "source observer argv is not the selected source interpreter")
    verify_source_capture_paths(home, authority, records, names, deadline, hash_members=hashed)
    require(type(code) is str and type(args) is tuple and all(type(value) is str for value in args), "source observer code/args shape")
    entry = f"import sys, runpy; sys.path.insert(0, {str(source)!r}); " + code
    argv = [*prefix, "-c", entry, *args]
    return argv, {"driver": "official pm.environments.venv_command", "selected_environment": str(environment),
                  "interpreter": str(interpreter), "argv_sha256": hashlib.sha256(json.dumps(argv).encode()).hexdigest(),
                  "captured_members": {name: records[name] for name in names}, "admission_qualified": False}


def source_native_prefix(inputs):
    """Keep the installation's complete exported CLI bootstrap; add only -B."""
    original = inputs["runtime_argv"]
    require(type(original) is list and len(original) == 4 and all(type(value) is str for value in original)
            and original[:3] == [str(inputs["python"]), "-I", "-c"], "source native original runtime shape differs")
    prefix = (original[0], "-I", "-B", *original[2:])
    require(len(json.dumps(prefix).encode("ascii")) <= 16384, "source native runtime prefix bound")
    return prefix


def source_native_runtime_command(api, inputs, records, authority, prefix, *, args, deadline, guard=False):
    """Rejoin the same installed CLI before each native or fixed resolution probe."""
    require(type(prefix) is tuple and prefix == source_native_prefix(inputs)
            and type(args) is tuple and all(type(value) is str for value in args)
            and type(guard) is bool and (not guard or not args), "source native frozen command differs")
    home, source = inputs["home"], inputs["source"]
    api_source = source / "hermes_cli/_launchers.py"
    require(Path(api.__code__.co_filename) == api_source and api.__code__.co_name == "runtime_command"
            and api.__globals__.get("__name__") == "hermes_cli._launchers"
            and api.__globals__.get("__file__") == str(api_source), "source native API is not pinned-source-owned")
    selected = tuple(path.relative_to(home).as_posix() for path in (inputs["native"], inputs["python"]))
    resolved = payload_links(home, records, deadline, source=inputs["layout"], resolve=selected)
    fixed = tuple(path.relative_to(home).as_posix() for path in
                  (inputs["facts"], inputs["original"] / "tools/facts.json", api_source,
                   source / "hermes_bootstrap.py", source / "hermes_cli/main.py"))
    targets = tuple(resolved[name][0] for name in selected)
    require(all(records.get(name, {}).get("type") == "file" for name in (*fixed, *targets)),
            "source native input is not a captured regular file")
    names = tuple(dict.fromkeys((*fixed, *selected, *(link for name in selected for link in resolved[name][1]), *targets)))
    hashed = tuple(dict.fromkeys((*fixed, *targets)))
    readout = inputs["command_file"]
    with held_census_evidence(readout.parent, inputs["command_parent"]) as (descriptor, verify):
        for phase in ("before", "after"):
            verify()
            ordinary_bytes(readout, inputs["command_info"], parent=None if artifacts.WINDOWS else descriptor,
                           pin=(inputs["command_info"].st_size, inputs["command_sha256"]), deadline=deadline)
            verify_source_capture_paths(home, authority, records, names, deadline, hash_members=hashed)
            verify()
            if phase == "before":
                argv = api(source, python=inputs["python"], args=args)
                require(type(argv) is list and all(type(value) is str for value in argv)
                        and argv == [prefix[0], prefix[1], *prefix[3:], *args], "source native exported bootstrap or tail differs")
                command = (observer_runtime_command(api, source, inputs["python"], code=GUARD_CODE) if guard else
                           [argv[0], "-I", "-B", *argv[2:]])
    return command


GUARD_CODE = r"""
import json, os
from pathlib import Path
p = Path(os.environ['HERMES_HOME'])
if any((p / n).exists() for n in ('active_profile', 'gateway.pid', 'gateway.lock', 'gateway_state.json')):
    raise RuntimeError('existing profile redirect/gateway signal')
import hermes_cli.main
from hermes_constants import get_default_hermes_root, get_hermes_home
from hermes_cli.auth import _global_auth_file_path
from hermes_cli.profiles import _check_gateway_running
from hermes_cli.gateway_multiplex_served import live_default_gateway_pid, recorded_served_profiles
from hermes_cli.service_manager import detect_service_manager
from hermes_cli.profile_memory_config import active_memory_provider
from hermes_cli.config import load_config_readonly
value = {'home': str(get_hermes_home().resolve()), 'default_root': str(get_default_hermes_root().resolve()),
         'native_root': str(get_default_hermes_root(home='').resolve()),
         'auth_fallback': str(_global_auth_file_path()) if _global_auth_file_path() is not None else None,
         'os_home': str(Path.home().resolve()), 'userprofile': os.environ['USERPROFILE'],
         'gateway_running': _check_gateway_running(p), 'default_gateway_pid': live_default_gateway_pid(),
         'served_profiles': recorded_served_profiles(), 'service_manager': detect_service_manager(),
         'memory_provider': active_memory_provider(load_config_readonly())}
if os.environ.get('HERMES_PROFILE_SOURCE_CAPTURE') == '1':
    from pm.environments import owning_home_root
    owner = owning_home_root(Path(os.environ['HERMES_NATIVE_SOURCE']))
    value['source_owner'] = str(owner) if owner is not None else None
print('UAP_PROFILE_RESOLUTION=' + json.dumps(value, sort_keys=True))
"""


def no_user_copies(root, sentinels):
    return inventory(root, sentinels)


def config_document_boundary(body, value, server, url, removed=False, authored_sha=None):
    """Check the exact mapping and bytes independently of the native YAML parser."""
    allowed = {"mcp_servers"}
    if removed and type(value) is dict and value == {"_config_version": 52}:
        # Exact pinned native final-server removal postimage, never a new input.
        stamp = b"_config_version: 52\r\n" if os.name == "nt" else b"_config_version: 52\n"
        require(type(value["_config_version"]) is int and body == stamp,
                "unadmitted removed config stamp")
        allowed.add("_config_version")
    require(isinstance(value, dict) and set(value) <= allowed, "unadmitted custom config fields")
    expected = {} if removed else {server: {"url": url}}
    require(value.get("mcp_servers", {}) == expected, "selected native MCP mapping differs")
    digest = hashlib.sha256(body).hexdigest()
    if authored_sha is not None:
        require(digest == authored_sha, "authored custom config bytes changed")
    return digest


def config_boundary(root, server, url, removed=False, authored_sha=None):
    import hermes_yaml as yaml
    body = ordinary_bytes(root / "config.yaml")
    return config_document_boundary(body, yaml.safe_load(body), server, url, removed, authored_sha)


def protocol_join(before, after, selected, expected):
    delta = {}
    for name in before:
        require(after[name][:len(before[name])] == before[name], "MCP method ledger rewritten")
        delta[name] = after[name][len(before[name]):]
    require(all(not methods for name, methods in delta.items() if name != selected), "sibling/default probe contacted")
    methods = delta.get(selected, [])
    require(methods.count("initialize") == expected and methods.count("tools/list") == expected
            and all(method in {"initialize", "notifications/initialized", "tools/list"} for method in methods)
            and (not expected or methods.index("initialize") < methods.index("tools/list")),
            "selected native initialize/list differs")
    return delta


def write_json(path, value):
    body = json.dumps(value, indent=2).encode() + b"\n"
    require(len(body) <= LIMIT and not path.exists(), "fresh bounded evidence required")
    path.write_bytes(body)
    return hashlib.sha256(body).hexdigest()


def observe_profiles(probes):
    evidence = Path(os.environ["HERMES_NATIVE_EVIDENCE"])
    user = Path(os.environ["HOME"])
    outer, nonce = user.parent, os.environ["HERMES_CALL_NONCE"]
    source_capture = source_capture_mode()
    native = [os.environ["HERMES_BIN"]]
    from hermes_constants import get_default_hermes_root
    import hermes_yaml as yaml
    original = get_default_hermes_root(home="").resolve()
    require(original.is_relative_to(user) and original != user and os.environ["USERPROFILE"] == str(user), "actual native User root differs")
    inputs = source_inputs(Path(os.environ["RUNNER_TEMP"]).resolve(), BASE["source_root"]()) if source_capture else None
    require(inputs is None or original == inputs["original"], "native source default User differs before fixture preparation")
    native_prefix, source_records = None, {}
    if source_capture:
        encoded = os.environ["HERMES_SOURCE_NATIVE_PREFIX"]
        require(len(encoded.encode("ascii")) <= 16384, "source native frozen prefix bound")
        value = json.loads(encoded)
        expected = {"argv": list(source_native_prefix(inputs)), "readout_sha256": inputs["command_sha256"],
                    "readout_identity": list(stable_file(inputs["command_info"])), "readout_parent": list(inputs["command_parent"])}
        require(type(value) is dict and json.dumps(value, sort_keys=True) == json.dumps(expected, sort_keys=True),
                "source native parent prefix differs")
        native_prefix = tuple(value["argv"])
    sentinels, authored = user_fixture_content(nonce)
    preparation, evidence_authority = None, None
    if source_capture:
        evidence_authority = json.loads(os.environ["HERMES_CENSUS_EVIDENCE_ID"])
        preparation = read_source_preparation(evidence, evidence_authority, nonce,
                                              os.environ["HERMES_SOURCE_PREPARATION_SHA256"])
        require(directory_identity(outer) == tuple(preparation["outer_identity"]), "original source outer changed before Project setup")
        retained = preparation["installer_preimages"]
    else:
        original.mkdir(parents=True, exist_ok=True)
        retained = prepare_user_fixture(original, authored)
    spec, records, shared = {}, [], []
    for scope in ("a", "b"):
        project = outer / ("project-" + scope)
        project.mkdir()
        (project / "foreign.txt").write_text(nonce + scope)
        bounded(["git", "init", "-q", str(project)], outer, dict(os.environ), evidence / ("git-" + scope), timeout=30)
        root = project / ".uap" / "hermes-runtime"
        # The selected disposable credential/config root starts private.
        root.mkdir(mode=0o700, parents=True)
        entry = {"url": probes[scope].url}
        config = root / "config.yaml"
        config.write_text(yaml.safe_dump({"mcp_servers": {"uap-root-" + scope: entry}}), encoding="utf-8")
        config.chmod(0o600)
        spec[scope] = {"root": str(root), "project": str(project), "server": "uap-root-" + scope,
                       "url": probes[scope].url, "config_sha256": sha(config), "entry": entry}
        if source_capture:
            temporary = root / "tmp"
            temporary.mkdir(mode=0o700)
            spec[scope].update(temporary=str(temporary), temporary_identity=directory_identity(temporary))
    roots = [Path(spec[scope]["root"]) for scope in ("a", "b")]
    validate_roots(user, original, roots)
    root_paths = {scope: Path(spec[scope]["root"]) for scope in ("a", "b")}
    held_roots = {scope: directory_identity(root) for scope, root in root_paths.items()}
    removed = set()
    for scope, root in root_paths.items():
        spec[scope]["directory_identity"] = held_roots[scope]
        config_boundary(root, spec[scope]["server"], spec[scope]["url"], authored_sha=spec[scope]["config_sha256"])
    spec["user"] = {"root": str(original), "project": str(user)}
    # This receipt records our original created roots, independently of whether
    # the subsequent whole-tree inventory admits native bootstrap effects.
    write_json(evidence / "profile-endpoints.json", {"selected": spec, "original_native_root": str(original),
               "os_home": str(user), "sentinel_sha256": [hashlib.sha256(x.encode()).hexdigest() for x in sentinels],
               "installer_preimages": retained, "source_capture": source_capture})
    protected = [str((original / name).relative_to(outer)).replace('\\', '/') for name in authored]
    controls = [str((outer / ("project-" + scope) / "foreign.txt").relative_to(outer)).replace('\\', '/') for scope in ("a", "b")]
    protected += controls
    outer_identity = tuple(preparation["outer_identity"]) if preparation is not None else directory_identity(outer)
    layout = None
    if source_capture:
        layout = dict(inputs["layout"], protected=(*inputs["layout"]["protected"], *protected),
                      sentinels=tuple(value.encode("utf-8") for value in sentinels))
    def capture(*, initial=False):
        if layout is None:
            return inventory(outer)
        deadline = time.monotonic() + 120
        try:
            snapshot = payload_snapshot(outer, outer_identity, deadline, source=layout)
        except (OSError, RuntimeError) as error:
            retain_source_refusal(error, evidence, evidence_authority, nonce,
                                  "observer-profile-initial" if initial else "observer-profile-snapshot")
            raise
        if initial:
            require_source_user_baseline(source_user_digest(snapshot["_records"], layout["user"], deadline), preparation["prepared_user"])
            require(not source_records, "source native original capture already retained")
            source_records.update(snapshot["_records"])
        return snapshot["ledger"]
    try:
        baseline = capture(initial=True)
    except (OSError, RuntimeError, ValueError):
        publish_census_request(evidence, outer, nonce, json.loads(os.environ["HERMES_CENSUS_EVIDENCE_ID"]))
        raise
    write_json(evidence / "original-user-baseline.json", baseline)
    current = baseline

    def command(label, scope, args=None, guard=False, expected_failure=False):
        nonlocal current
        verify_root_identities(root_paths, held_roots)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        before = capture()
        require(before == current, "unmeasured inter-command mutation")
        user_prefix = (user if source_capture else original).relative_to(outer).as_posix()
        require(profile_tree(before, user_prefix) == profile_tree(baseline, user_prefix), "original User changed before command")
        require(scope != "user" or (label == "list-user" and removed == {"a", "b"}), "User control must follow Project removal")
        selected = spec[scope]
        env = {**os.environ, "HERMES_HOME": selected["root"]}
        require(env["HOME"] == str(user) and env["USERPROFILE"] == str(user), "OS User home changed")
        if source_capture:
            if scope != "user":
                temporary = Path(selected["temporary"])
                require(temporary == Path(selected["root"]) / "tmp"
                        and directory_identity(temporary) == selected["temporary_identity"], "source native original temporary root differs")
                env.update({key: str(temporary) for key in ("TMPDIR", "TEMP", "TMP")})
            from hermes_cli._launchers import runtime_command
            argv = source_native_runtime_command(runtime_command, inputs, source_records, outer_identity, native_prefix,
                args=tuple(args or ()), guard=guard, deadline=float(os.environ["HERMES_PROFILE_DEADLINE"]))
            require(all(directory_identity(Path(spec[name]["temporary"])) == spec[name]["temporary_identity"]
                        for name in root_paths), "source native original temporary root changed before launch")
        elif guard:
            from hermes_cli._launchers import runtime_command
            argv = observer_runtime_command(runtime_command, Path(os.environ["HERMES_PROFILE_RUNTIME_ROOT"]),
                                            os.environ["HERMES_PROFILE_PYTHON"], code=GUARD_CODE)
        else:
            argv = [*native, *args]
        methods_before = {name: list(probe.methods()) for name, probe in probes.items()}
        failure, output = None, b""
        try:
            output = bounded(argv, Path(selected["project"]), env, evidence / label, timeout=60, expected_failure=expected_failure)
        except BaseException as error:
            failure = error
        verify_root_identities(root_paths, held_roots)
        if source_capture:
            require(all(directory_identity(Path(spec[name]["temporary"])) == spec[name]["temporary_identity"]
                        for name in root_paths), "source native original temporary root changed")
        after = capture()
        delta = changes(before, after)
        records.append({"label": label, "scope": scope, "runtime_root": selected["root"],
                        "delta": delta, "error": type(failure).__name__ if failure else None})
        if source_capture:
            records[-1].update(driver="fixed-resolution-probe" if guard else "official-runtime-command",
                               bytecode_disabled=True, temporary_root=env["TMPDIR"])
        current = after
        if label == "remove-mcp-" + scope and failure is None:
            removed.add(scope)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        shared_delta = [{"label": label, "path": path, **change} for path, change in delta.items()
                        if not any(path == root.relative_to(outer).as_posix()
                                   or path.startswith(root.relative_to(outer).as_posix() + "/") for root in roots)]
        try:
            profile_effect_boundary(label, scope, baseline, before, after, user_prefix,
                                    Path(selected["root"]).relative_to(outer).as_posix(), source_capture=source_capture)
        except BaseException:
            shared.extend(shared_delta)
            raise
        protected_unchanged(baseline, after, protected)
        for other in (name for name in ("a", "b") if name != scope):
            prefix = str(Path(spec[other]["root"]).relative_to(outer)).replace('\\', '/')
            require({key: value for key, value in before.items() if key == prefix or key.startswith(prefix + "/")}
                    == {key: value for key, value in after.items() if key == prefix or key.startswith(prefix + "/")},
                    "sibling custom root changed")
        if not source_capture:
            for root in roots:
                no_user_copies(root, sentinels)
        if failure:
            raise failure
        methods_after = {name: list(probe.methods()) for name, probe in probes.items()}
        tested = label in ("test-a", "test-b", "test-a-restart")
        records[-1]["native_methods"] = protocol_join(methods_before, methods_after, scope if tested else None, 1 if tested else 0)
        if guard:
            prefix = b"UAP_PROFILE_RESOLUTION="
            rows = [line[len(prefix):] for line in output.splitlines() if line.startswith(prefix)]
            require(len(rows) == 1, "native resolution receipt missing/ambiguous")
            receipt = json.loads(rows[0])
            validate_guards({key: receipt[key] for key in ("gateway_running", "default_gateway_pid", "served_profiles", "service_manager", "memory_provider")})
            require(receipt["home"] == selected["root"] == receipt["default_root"]
                    and receipt["native_root"] == str(original) and receipt["auth_fallback"] is None
                    and receipt["os_home"] == str(user) and receipt["userprofile"] == str(user), "native custom/User root authority differs")
            if source_capture:
                require(receipt.get("source_owner") == str(original), "native source checkout owner differs")
            records[-1]["resolution"] = receipt
        return output

    try:
        if not source_capture:
            require(b"hermes" in command("version-a", "a", ["--version"]).lower(), "bundled native version absent")
        for scope in ("a", "b"):
            if source_capture:
                # The full official source bootstrap prepares its own environment.
                # --version skips that preparation at the pinned revision.
                config_readout("path", command("config-path-" + scope, scope, ["config", "path"]), spec[scope])
                if scope == "a":
                    version = command("version-a", "a", ["--version"])
                    source_version_readout(version, inputs["source"])
            command("guard-" + scope, scope, guard=True)
            # Bounded native observation may initialize only its selected home.
            if not source_capture:
                config_readout("path", command("config-path-" + scope, scope, ["config", "path"]), spec[scope])
            config_readout("mcp", command("config-mcp-" + scope, scope, ["config", "get", "mcp_servers", "--json"]), spec[scope])
            output = command("list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() in output, "selected native MCP name absent")
            command("test-" + scope, scope, ["mcp", "test", spec[scope]["server"]])
        command("test-a-restart", "a", ["mcp", "test", spec["a"]["server"]])
        for scope in ("a", "b"):
            command("remove-mcp-" + scope, scope, ["mcp", "remove", spec[scope]["server"]])
            output = command("removed-list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() not in output, "removed MCP still listed")
            command("removed-test-" + scope, scope, ["mcp", "test", spec[scope]["server"]], expected_failure=True)
        output = command("list-user", "user", ["mcp", "list"])
        require(b"foreign" in output and all(spec[x]["server"].encode() not in output for x in ("a", "b")), "original User MCP selection changed")
        write_json(evidence / "original-user-final.json", capture())
    finally:
        write_json(evidence / "profile-command-deltas.json", {"commands": records, "unadmitted_shared_effects": shared,
                   "production_project_mcp": False, "admission_qualified": False,
                   "methods": {name: list(probe.methods()) for name, probe in probes.items()}})


def public_json(body):
    require(type(body) is bytes and 0 < len(body) <= LIMIT, "public JSON bound")
    def unique(pairs):
        value = {}
        for key, item in pairs:
            require(key not in value, "public JSON duplicate key")
            value[key] = item
        return value
    return json.loads(body, object_pairs_hook=unique, parse_constant=lambda _: require(False, "public JSON nonfinite"))


def public_mcp_digest(name, server):
    require(type(name) is str and bool(name) and type(server) is dict, "public MCP object shape differs")
    canonical = json.dumps(server, sort_keys=True, separators=(",", ":"), ensure_ascii=False, allow_nan=False).encode()
    return "sha256:" + hashlib.sha256(b"agentplugins-native-mcp-v1\0hermes-mcp-servers-yaml\0" + name.encode() + b"\0" + canonical).hexdigest()


def public_parent_receipt(receipt, selected):
    """Join recorded publication facts; product Load/Kernel revalidate the OS IDs."""
    parent, authority = Path(selected).parent, receipt.get("publication_parent_authority")
    require(type(authority) is dict and set(authority) == {"version", "canonical_root", "ancestry"}
            and type(authority["version"]) is int and authority["version"] == 1
            and authority["canonical_root"] == str(parent) and receipt.get("before_digest", "") == "",
            "public publication parent role differs")
    ancestry = authority["ancestry"]
    require(type(ancestry) is list and 1 <= len(ancestry) <= 256, "public publication ancestry bound")
    for entry in ancestry:
        require(type(entry) is dict and set(entry) == {"canonical_path", "scheme", "volume_id", "object_id"}
                and all(type(entry[key]) is str and 0 < len(entry[key]) <= (4096 if key == "canonical_path" else 128)
                        for key in entry), "public publication ancestry shape differs")
    require(ancestry[-1]["canonical_path"] == str(parent), "public publication ancestry root differs")
    for key in ("active_path", "staging_path", "backup_path"):
        path = receipt.get(key)
        require(type(path) is str and 0 < len(path) <= 4096 and Path(path).is_absolute()
                and os.path.normpath(path) == path and Path(path).parent == parent,
                "public publication path differs from original parent")


def public_runtime_pins(expected=None, *, rehash=True):
    pins = {}
    for role, path_key, hash_key in (("uap", "UAP_BIN", "UAP_HERMES_COLD_UAP_SHA256"),
            ("cold", "UAP_HERMES_COLD_TEST_BIN", "UAP_HERMES_COLD_TEST_SHA256"),
            ("probe", "UAP_HERMES_MCP_PROBE_BIN", "UAP_HERMES_MCP_PROBE_SHA256")):
        path, digest = Path(os.environ[path_key]), os.environ[hash_key]
        require(path.is_absolute() and path.resolve() == path and re.fullmatch(r"[a-f0-9]{64}", digest), "exact public runtime required")
        info = path.lstat()
        if expected is not None:
            require(expected[role] == {"path": str(path), "bytes": info.st_size, "sha256": digest,
                    "identity": list(stable_file(info))}, "original public runtime changed")
        if rehash:
            ordinary_bytes(path, info, pin=(info.st_size, digest))
        pins[role] = {"path": str(path), "bytes": info.st_size, "sha256": digest, "identity": list(stable_file(info))}
    require(len({Path(pin["path"]).parent for pin in pins.values()}) == 1, "public runtimes lack one authored build root")
    return pins


def public_binding(project, name):
    """Derive only this authored Project's prospective package read authority."""
    state_path = project / ".uap/state-v2.json"
    if not state_path.exists():
        return None
    body = ordinary_bytes(state_path)
    state = public_json(body)
    require(type(state.get("schema_version")) is int and state["schema_version"] == 5
            and type(state.get("installations")) is list and len(state["installations"]) == 1, "public state shape")
    installed = state["installations"][0]
    require(installed.get("declared_name") == name and type(installed.get("clients")) is dict, "public installation differs")
    if not installed["clients"]:
        return None
    require(len(installed["clients"]) == 1, "ambiguous public binding")
    binding = next(iter(installed["clients"].values()))
    physical = name + "-" + hashlib.sha256(installed["installation_id"].encode()).hexdigest()[:12]
    target, selected = project / ".uap/managed/clients/hermes" / physical, project / ".hermes/uap-session"
    require(binding.get("client_id") == "hermes" and binding.get("scope") == "project"
            and binding.get("target_locator") == str(target) and binding.get("physical_artifact_id") == physical
            and binding.get("native_profile_root") == str(project / ".hermes")
            and binding.get("activation") == "prepared" and binding.get("verification") == "installation_verified"
            and (binding.get("native_activation_attempt") is None or type(binding["native_activation_attempt"]) is str and binding["native_activation_attempt"] == "")
            and binding.get("pending_native_intent") is None,
            "public binding authority/status differs")
    objects = binding.get("native_objects", [])
    require(len(objects) == 5 and {item.get("kind") for item in objects} ==
            {"managed_package_directory", "hermes_project_session_root", "hermes_project_session_mcp", "hermes_project_skill"},
            "public native ownership differs")
    mcp = [item for item in objects if item["kind"] == "hermes_project_session_mcp"]
    require(len(mcp) == 2 and {item.get("logical_name") for item in mcp} == {name + "-http", name + "-stdio"}
            and all(item == dict(object_id="hermes_project_session_mcp:" + item["logical_name"],
                    kind="hermes_project_session_mcp", logical_name=item["logical_name"], path=str(selected / "config.yaml"),
                    managed_digest=item.get("managed_digest"), protection_class="managed")
                    and re.fullmatch(r"sha256:[a-f0-9]{64}", item["managed_digest"]) for item in mcp), "public MCP object selection differs")
    skills = [item for item in objects if item["kind"] == "hermes_project_skill"]
    require(len(skills) == 1 and skills[0] == dict(object_id="hermes_project_skill:" + name,
            kind="hermes_project_skill", logical_name=name, path=str(project / ".hermes/skills" / name),
            source_relative="skills/" + name, managed_digest=skills[0].get("managed_digest"), protection_class="managed")
            and re.fullmatch(r"sha256:[a-f0-9]{64}", skills[0]["managed_digest"]), "public Skill object selection differs")
    package = [item for item in objects if item["kind"] == "managed_package_directory"]
    require(len(package) == 1 and package[0] == dict(object_id="package:hermes:" + physical,
            kind="managed_package_directory", logical_name=name, path=str(target),
            managed_digest=package[0].get("managed_digest"), protection_class="managed")
            and re.fullmatch(r"sha256:[a-f0-9]{64}", package[0]["managed_digest"]), "public package receipt differs")
    receipts = binding.get("receipts", [])
    require(1 <= len(receipts) <= 8 and all(item.get("phase") == "committed" and item.get("mutation_type") == "directory_swap"
            and item.get("client_binding_id") == binding["client_binding_id"] and item.get("active_path") == str(target)
            and type(item.get("sequence")) is int and item["sequence"] == index + 1 for index, item in enumerate(receipts))
            and receipts[-1].get("after_digest") == package[0]["managed_digest"], "public committed package receipt differs")
    root_receipts = [item for item in state.get("transaction_receipts", []) if str(item.get("after_digest", "")).startswith("native-root-v1:")]
    require(len(root_receipts) == 1 and root_receipts[0].get("phase") == "committed"
            and root_receipts[0].get("mutation_type") == "directory_swap" and root_receipts[0].get("active_path") == str(selected)
            and root_receipts[0].get("client_binding_id") == binding["client_binding_id"]
            and re.fullmatch(r"native-root-v1:[a-f0-9]{64}", root_receipts[0]["after_digest"]), "public durable root receipt differs")
    public_parent_receipt(root_receipts[0], selected)
    marker = ("uap-hermes-project-session-v1\n" + str(project) + "\n").encode()
    roots = [item for item in objects if item["kind"] == "hermes_project_session_root"]
    require(roots == [dict(object_id="hermes_project_session_root", kind="hermes_project_session_root", path=str(selected / ".uap-owner"),
            managed_digest=hashlib.sha256(marker).hexdigest(), protection_class="owned")]
            and ordinary_bytes(selected / ".uap-owner") == marker, "public root marker differs")
    data = installed["data_receipts"][binding["data_receipt_id"]]
    data_path = project / ".uap/plugin-data" / physical
    require(len(installed["data_receipts"]) == 1 and data.get("data_receipt_id") == binding["data_receipt_id"]
            and data.get("physical_backend_id") == physical and data.get("scope") == "project" and data.get("state") == "owned"
            and data.get("locator") == str(data_path)
            and public_json(ordinary_bytes(data_path / ".agentplugins-data-owner.json")) == data, "public retained data receipt differs")
    revision, source = binding["package_revision"], installed["source"]
    require(revision["tree_digest"] == source["tree_digest"] and revision.get("resolved_revision", "") == source.get("resolved_revision", "")
            and revision["manifest_digest"] == installed["package"]["manifest_digest"]
            and revision["version"] == installed["package"]["version"], "public source/package revision differs")
    return dict(state=state, state_sha256=hashlib.sha256(body).hexdigest(), installed=installed, binding=binding,
                target=target, selected=selected, data=data_path, digest=package[0]["managed_digest"],
                root_receipt=root_receipts[0], root_identity=directory_identity(selected), parent_identity=directory_identity(selected.parent),
                data_identity=directory_identity(data_path))


def public_same_record(left, right):
    return (json.dumps(left, sort_keys=True, separators=(",", ":"), allow_nan=False)
            == json.dumps(right, sort_keys=True, separators=(",", ":"), allow_nan=False))


def public_continuity(before, after, *, advance=False):
    """Compare with the accepted installation, never a newly observed baseline."""
    previous, current = before["installed"], after["installed"]
    require(public_same_record({key: value for key, value in previous.items() if key not in {"source", "package", "clients", "updated_at", "operation_group_id"}},
                 {key: value for key, value in current.items() if key not in {"source", "package", "clients", "updated_at", "operation_group_id"}}),
            "public original installation/data authority changed")
    source, package = dict(previous["source"]), dict(previous["package"])
    if advance:
        require(current["source"]["tree_digest"] != source["tree_digest"], "public update did not advance its local source")
        # This warm family has one authored local source. No Git/archive or
        # canonical/requested source field may change with its new tree.
        source["tree_digest"] = current["source"]["tree_digest"]
        package.update(version=current["package"]["version"], manifest_digest=current["package"]["manifest_digest"])
    require(public_same_record(source, current["source"]) and public_same_record(package, current["package"]), "public original source/package identity changed")
    left, right = dict(before["binding"]), dict(after["binding"])
    require(public_same_record(left["receipts"], right["receipts"][:len(left["receipts"])]), "public original committed receipt history changed")
    for key in ("receipts", "updated_at"):
        left.pop(key, None); right.pop(key, None)
    if advance:
        revision = dict(left["package_revision"])
        revision.update(tree_digest=source["tree_digest"], version=package["version"], manifest_digest=package["manifest_digest"])
        left["package_revision"] = revision
        # Complete current digests are independently recomputed by snapshot.
        # Every object's identity, kind, path and ownership role must continue.
        for binding in (left, right):
            binding["native_objects"] = [{key: value for key, value in item.items() if key != "managed_digest"}
                                         for item in binding["native_objects"]]
    require(public_same_record(left, right) and before["data"] == after["data"] and before["data_identity"] == after["data_identity"],
            "public original binding or physical data directory changed")


def public_package_digest(outer, proof, image, pin):
    target, digest = proof["target"], hashlib.sha256()
    prefix = target.relative_to(outer).as_posix()
    rows = profile_tree(image, prefix)
    require(prefix in rows and len(rows) <= 128, "public package membership differs")
    for path in sorted(rows):
        if path == prefix:
            continue
        name, row = path[len(prefix) + 1:], rows[path]
        require(name.isascii() and ".git" not in name.split("/") and Path(name).name not in
                {".uap-integrations.lock", ".plugin-kit-ai.lock"}, "public snapshot-excluded member")
        file = outer / path
        kind = "dir" if row["type"] == "directory" else "file"
        executable = kind == "file" and os.name != "nt" and bool(row["mode"] & 0o111)
        digest.update(("\0".join((kind, name, "true" if executable else "false", str(row.get("bytes", 0)))) + "\0").encode())
        info = file.lstat()
        require((info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode)) == (row["device"], row["inode"], row["mode"]), "public package member replaced")
        if kind == "file":
            held = open_inventory_directory(file.parent)
            try:
                parent = image[file.parent.relative_to(outer).as_posix()]
                opened = os.fstat(held)
                require((opened.st_dev, opened.st_ino) == (parent["device"], parent["inode"]), "public package parent replaced")
                body = ordinary_bytes(file, info, parent=None if artifacts.WINDOWS else held,
                        pin=pin if name == "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins" + (".exe" if os.name == "nt" else "") else None,
                        package_digest=digest)
                actual = body if type(body) is str else hashlib.sha256(body).hexdigest()
                require(actual == row["sha256"] and info.st_size == row["bytes"], "public package bytes changed")
                require(directory_identity(file.parent)[:2] == (parent["device"], parent["inode"]), "public package parent changed")
            finally:
                os.close(held)
    require("sha256:" + digest.hexdigest() == proof["digest"], "public package digest differs from committed receipt")


def public_snapshot(outer, projects, pins):
    proofs = {scope: public_binding(project, "hermes-public-" + scope) for scope, project in projects.items()}
    pin = (pins["uap"]["bytes"], pins["uap"]["sha256"])
    suffix = ".exe" if os.name == "nt" else ""
    launchers = {(proof["target"] / ("io.github.777genius.agentplugins/managed-stdio-v1/agentplugins" + suffix)).relative_to(outer).as_posix(): pin
                 for proof in proofs.values() if proof is not None}
    sentinels = ["USER_" + kind + "_" + os.environ.get("HERMES_CALL_NONCE", "") for kind in ("MODEL", "PROVIDER", "AUTH", "HISTORY", "FOREIGN")]
    image = inventory(outer, sentinels, launchers=launchers, public=True, launcher_mode=stat.S_IMODE(pins["uap"]["identity"][2]))
    for scope, proof in proofs.items():
        if proof is None:
            continue
        require(image[(projects[scope] / ".uap/state-v2.json").relative_to(outer).as_posix()]["sha256"] == proof["state_sha256"], "public state changed during capture")
        public_package_digest(outer, proof, image, pin)
        skill = next(item for item in proof["binding"]["native_objects"] if item["kind"] == "hermes_project_skill")
        source = proof["target"] / skill["source_relative"]
        prefix = source.relative_to(outer).as_posix()
        require(set(profile_tree(image, prefix)) == {prefix, prefix + "/SKILL.md"}, "public Skill source membership differs")
        body = ordinary_bytes(source / "SKILL.md")
        row = image[prefix + "/SKILL.md"]
        executable = os.name != "nt" and bool(row["mode"] & 0o111)
        skill_digest = hashlib.sha256(b"f\0SKILL.md\0" + (b"x\0" if executable else b"-\0") + body + b"\0").hexdigest()
        require(hashlib.sha256(body).hexdigest() == row["sha256"] and skill["managed_digest"] == "sha256:" + skill_digest,
                "public Skill digest differs from staged source")
        data = proof["data"].relative_to(outer).as_posix()
        require(set(profile_tree(image, data)) <= {data, data + "/.agentplugins-data-owner.json", data + "/hermes-events.jsonl"}, "public data has unmeasured members")
        require((directory_identity(proof["selected"]), directory_identity(proof["selected"].parent)) ==
                (proof["root_identity"], proof["parent_identity"]), "public root authority changed during capture")
        require(directory_identity(proof["data"]) == proof["data_identity"], "public data authority changed during capture")
    return image, proofs


def public_bootstrap_detail(value):
    """Validate only closed captured counts and a hash-only unexpected member."""
    fields = {"scope", "predicate", "member_count", "backup_count", "unmeasured_count", "first_unmeasured"}
    require(type(value) is dict and all(type(key) is str for key in value) and set(value) == fields,
            "closed public bootstrap detail")
    require(type(value["scope"]) is str and value["scope"] in {"a", "b"}
            and type(value["predicate"]) is str, "public bootstrap detail scope")
    count, backups, unknown = (value[key] for key in ("member_count", "backup_count", "unmeasured_count"))
    require(all(type(number) is int for number in (count, backups, unknown))
            and 0 < count < 2048 and 0 <= backups <= count and 0 <= unknown <= count - backups,
            "public bootstrap detail counts")
    predicate = ("both" if backups > 5 and unknown else "backup-count" if backups > 5
                 else "unmeasured-member" if unknown else None)
    require(predicate is not None and value["predicate"] == predicate, "public bootstrap detail predicate")
    first = value["first_unmeasured"]
    if unknown == 0:
        require(first is None, "public bootstrap unexpected member without count")
    else:
        require(type(first) is dict and all(type(key) is str for key in first)
                and set(first) == {"member_sha256", "type", "mode", "bytes"},
                "closed public bootstrap member")
        digest = lambda text: type(text) is str and re.fullmatch(r"[a-f0-9]{64}", text) is not None
        require(digest(first["member_sha256"]) and type(first["type"]) is str
                and first["type"] in {"file", "directory"} and type(first["mode"]) is int
                and 0 <= first["mode"] <= 0o7777, "public bootstrap member metadata")
        if first["type"] == "directory":
            require(first["bytes"] is None, "public bootstrap directory content")
        else:
            require(type(first["bytes"]) is int and 0 <= first["bytes"] <= LIMIT,
                    "public bootstrap file metadata")
    return value


def capture_public_bootstrap(root, outer, rows, backups, allowed):
    """Use the already captured ledger; neither names nor content leave this helper."""
    try:
        require(type(rows) is dict and 0 < len(rows) < 2048 and all(type(name) is str for name in rows)
                and type(backups) is set and type(allowed) is set
                and all(type(name) is str for name in backups | allowed)
                and backups <= set(rows) and backups <= allowed, "public bootstrap captured ledger")
        scope = {"public-a/.hermes/uap-session": "a", "public-b/.hermes/uap-session": "b"}.get(
            root.relative_to(outer).as_posix())
        unknown = set(rows) - allowed
        first = None
        if unknown:
            name = min(unknown)
            encoded = name.encode("utf-8", "surrogatepass")
            require(0 < len(encoded) <= 16384 and "\0" not in name and ":" not in name and "\\" not in name
                    and all(part not in ("", ".", "..") for part in name.split("/")),
                    "public bootstrap relative member")
            row = rows[name]
            require(type(row) is dict and all(type(key) is str for key in row), "public bootstrap captured row")
            kind = row.get("type")
            require(type(kind) is str and kind in {"file", "directory"}, "public bootstrap captured kind")
            keys = {"type", "mode", "device", "inode"} | ({"bytes", "sha256"} if kind == "file" else set())
            require(set(row) == keys
                    and all(type(row[key]) is int and 0 <= row[key] < 2**64 for key in ("device", "inode")),
                    "public bootstrap captured metadata")
            first = dict(member_sha256=hashlib.sha256(b"uap-hermes-public-bootstrap-member-v1\0" + encoded).hexdigest(),
                         type=kind, mode=row["mode"], bytes=row.get("bytes"))
        predicate = ("both" if len(backups) > 5 and unknown else "backup-count" if len(backups) > 5
                     else "unmeasured-member")
        return public_bootstrap_detail(dict(scope=scope, predicate=predicate, member_count=len(rows),
            backup_count=len(backups), unmeasured_count=len(unknown), first_unmeasured=first))
    except BaseException:
        return None  # Optional metadata never replaces the existing refusal.


def public_stdio_log(root, row, server, sessions):
    """The pinned client's header-only log joins the fixture's completed sessions."""
    from datetime import datetime
    require(server in {"hermes-public-a-stdio", "hermes-public-b-stdio"}
            and type(sessions) is int and 0 < sessions <= (3 if server == "hermes-public-a-stdio" else 1),
            "public stdio log lacks completed owned sessions")
    path = root / "logs/mcp-stderr.log"
    before = path.lstat()
    require((before.st_dev, before.st_ino, stat.S_IMODE(before.st_mode), before.st_size) ==
            (row["device"], row["inode"], row["mode"], row["bytes"]) and 0 < row["bytes"] <= 512,
            "public stdio log identity or bound differs")
    body = ordinary_bytes(path, before)
    require(len(body) == row["bytes"] and hashlib.sha256(body).hexdigest() == row["sha256"],
            "public stdio log differs from captured bytes")
    newline = b"\r\n" if os.name == "nt" else b"\n"
    pattern = (re.escape(newline) + rb"([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3})"
               + re.escape((" ===== starting MCP server '" + server + "' =====").encode() + newline))
    matches = list(re.finditer(pattern, body))
    require(len(matches) == sessions and b"".join(match.group() for match in matches) == body,
            "public stdio log contains an unmeasured session or child output")
    for match in matches:
        datetime.strptime(match.group(1).decode("ascii"), "%Y-%m-%d %H:%M:%S,%f")


PUBLIC_SPAWN_NAMES = ("spawn-ledger.json", "spawn-ledger.json.lock")
# Pinned concurrent-log-handler 0.9.30: only an exact empty Windows
# lock born during this Project's successful native HTTP test is admissible.
PUBLIC_NATIVE_LOG_LOCK = "logs/.__agent.lock"


def public_native_log_lock(root, row):
    path = root / PUBLIC_NATIVE_LOG_LOCK
    require(os.name == "nt" and type(row) is dict
            and set(row) == {"type", "mode", "device", "inode", "bytes", "sha256"}
            and row["type"] == "file" and row["mode"] == 0o666
            and type(row["bytes"]) is int and row["bytes"] == 0
            and row["sha256"] == hashlib.sha256(b"").hexdigest(),
            "public native log lock has unadmitted metadata")
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
            and (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), info.st_size)
            == (row["device"], row["inode"], row["mode"], 0)
            and ordinary_bytes(path, info) == b"",
            "public native log lock is not the captured empty regular file")




def public_spawn_install(root, native):
    require(root.is_absolute() and root.resolve() == root and root.name == "hermes-agent"
            and native.is_absolute() and native.resolve() == native
            and native == root.parent / "bin" / ("hermes.exe" if artifacts.WINDOWS else "hermes"),
            "public spawn captured bundled code root differs")
    return directory_identity(root), hashlib.sha256(str(root).lower().encode("utf-8", "replace")).hexdigest()[:12]


def public_spawn_rows(image, outer, root):
    prefix = root.relative_to(outer).as_posix() + "/"
    return {name: dict(image[prefix + name]) for name in PUBLIC_SPAWN_NAMES if prefix + name in image}


def public_parent_facts(value):
    require(type(value) is dict and set(value) == {"kind", "parent_pid"}
            and value["kind"] == "hermes-public-parent-v1"
            and type(value["parent_pid"]) is int and 0 < value["parent_pid"] < 2**32,
            "public stdio original parent receipt differs")
    return value["parent_pid"]


def public_spawn_status(evidence, label, argv, stdout):
    require(label in {"public-stdio-a", "public-stdio-b", "public-stdio-restart-a", "public-stdio-repaired-a"}
            and type(argv) is list and len(argv) == 4 and argv[1:3] == ["mcp", "test"],
            "public spawn command role differs")
    prefix = evidence / label
    value = public_json(ordinary_bytes(prefix.with_suffix(".status.json")))
    require(type(value) is dict and set(value) == {"argv", "pid", "exit", "error", "streams"}
            and value["argv"] == argv and type(value["pid"]) is int and 0 < value["pid"] < 2**32
            and type(value["exit"]) is int and value["exit"] == 0 and value["error"] is None
            and type(value["streams"]) is dict and set(value["streams"]) == {"stdout", "stderr"},
            "public spawn command status differs")
    for suffix in ("stdout", "stderr"):
        body, stream = ordinary_bytes(prefix.with_suffix("." + suffix)), value["streams"][suffix]
        require(type(stream) is dict and set(stream) == {"bytes", "sha256"}
                and type(stream["bytes"]) is int and stream["bytes"] == len(body)
                and stream["sha256"] == hashlib.sha256(body).hexdigest()
                and (suffix != "stdout" or body == stdout), "public spawn command stream differs")
    return value["pid"]


def public_spawn_launcher(proof, recipe, probe):
    target, data, executable = proof["target"], proof["data"], Path(probe)
    launcher = target / "io.github.777genius.agentplugins/managed-stdio-v1" / ("agentplugins.exe" if artifacts.WINDOWS else "agentplugins")
    expected = [str(launcher), "--internal-stdio-v1", str(target), str(data), str(target), "plugin",
                "--argv-base64-v1", base64.b64encode(executable.name.encode()).decode("ascii")]
    require(type(recipe) is dict and type(recipe.get("command")) is str and type(recipe.get("args")) is list
            and [recipe["command"], *recipe["args"]] == expected, "public spawn managed launcher recipe differs")
    return expected


def public_spawn_continuity(previous, current, *, stdio=False):
    if not stdio:
        require(current == previous, "public retained spawn ledger or lock changed")
    elif previous is not None:
        lock = PUBLIC_SPAWN_NAMES[1]
        require(all(current[field][lock] == previous[field][lock] for field in ("rows", "identities")),
                "public stdio command replaced its original spawn lock")


def public_spawn_entry(body, before, after, nonce, native_pid, install_id, launcher_argv, probe, system):
    require(type(body) is bytes and 0 < len(body) <= 4096 and system in {"linux", "darwin", "win32"},
            "public spawn ledger bound/platform differs")
    entries = public_json(body)
    fields = set("pid create_time purpose install spawner_pid spawner_create registered_at argv host port profile hermes_home isolated boot_start spawner_boot boot_id".split())
    require(type(entries) is list and len(entries) == 1 and type(entries[0]) is dict
            and set(entries[0]) == fields, "public spawn ledger schema or membership differs")
    entry = entries[0]
    require(type(native_pid) is int and 0 < native_pid < 2**32
            and type(entry["pid"]) is int and 0 < entry["pid"] < 2**32
            and type(entry["spawner_pid"]) is int and entry["spawner_pid"] == native_pid
            and entry["pid"] != native_pid and entry["purpose"] == "mcp-helper"
            and type(install_id) is str and re.fullmatch(r"[a-f0-9]{12}", install_id)
            and entry["install"] == install_id and entry["host"] == entry["profile"] == entry["hermes_home"] == ""
            and entry["port"] is None and entry["isolated"] is False, "public spawn ledger identity/defaults differ")
    for key in ("create_time", "spawner_create", "registered_at"):
        require(type(entry[key]) is float and math.isfinite(entry[key]) and 0 < entry[key] < 2**53,
                "public spawn ledger time differs")
    for key in ("boot_start", "spawner_boot"):
        require(entry[key] is None or system == "linux" and type(entry[key]) is float
                and math.isfinite(entry[key]) and 0 <= entry[key] < 2**53, "public spawn ledger boot time differs")
    boot = entry["boot_id"]
    require(boot is None or system == "linux" and type(boot) is str and re.fullmatch(
        r"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}", boot),
        "public spawn ledger boot identifier differs")
    require(type(launcher_argv) is list and len(launcher_argv) == 8
            and all(type(arg) is str and arg and "\0" not in arg for arg in launcher_argv)
            and type(probe) is str and probe and "\0" not in probe
            and type(entry["argv"]) is str and entry["argv"] in
            ({" ".join(launcher_argv)} if system == "win32" else {" ".join(launcher_argv), probe}),
            "public spawn ledger argv differs")
    require(type(before) is list and type(after) is list and after[:len(before)] == before
            and len(after) == len(before) + 2 and type(nonce) is str and re.fullmatch(r"[a-f0-9]{64}", nonce),
            "public spawn ledger lacks this command's new events")
    first, second = after[len(before):]
    require(all(type(row) is dict for row in before + [first, second]), "public spawn event schema differs")
    require([first.get("method"), second.get("method")] == ["initialize", "tools/list"]
            and first.get("session") not in {row.get("session") for row in before},
            "public spawn ledger reused a prior session")
    parent = public_parent_facts(first.get("facts"))
    for row in (first, second):
        require(set(row) == {"method", "pid", "session", "nonce", "timestamp", "facts"}
                and row["nonce"] == nonce and type(row["pid"]) is int and 0 < row["pid"] < 2**32
                and type(row["session"]) is str and 0 < len(row["session"]) <= 128
                and row["session"] == first["session"] and row["pid"] == first["pid"]
                and type(row["timestamp"]) is str and 0 < len(row["timestamp"]) <= 40
                and public_parent_facts(row["facts"]) == parent, "public spawn ledger event identity differs")
    require((entry["pid"] == parent and first["pid"] not in {parent, native_pid}) if system == "win32" else
            (entry["pid"] == first["pid"] and parent == native_pid), "public spawn ledger process chain differs")
    return entry


def public_spawn_read(root, outer, image, authorities):
    require(root.relative_to(outer).as_posix() in {"public-a/.hermes/uap-session", "public-b/.hermes/uap-session"}
            and len(authorities) == 4, "public spawn root authority differs")
    rows = public_spawn_rows(image, outer, root)
    require(set(rows) == set(PUBLIC_SPAWN_NAMES), "public spawn ledger pair is missing")
    bodies, identities, held = {}, {}, []
    with ExitStack() as handles:
        parent = None
        for path, authority in zip((outer, root.parent.parent, root.parent, root), authorities):
            info = path.lstat() if parent is None or artifacts.WINDOWS else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == tuple(authority)
                    and image.get(path.relative_to(outer).as_posix()) == {
                        "type": "directory", "mode": stat.S_IMODE(info.st_mode), "device": info.st_dev, "inode": info.st_ino},
                    "public spawn original directory differs")
            descriptor = open_inventory_directory(path, None if artifacts.WINDOWS else parent)
            handles.callback(os.close, descriptor)
            opened = os.fstat(descriptor)
            check_inventory_directory(info, opened, "public spawn held directory differs")
            held.append((path, info, descriptor, opened, None if artifacts.WINDOWS else parent))
            parent = descriptor
        def verify():
            for item in held:
                verify_inventory_directory(*item)
        try:
            for name, maximum, mode in ((PUBLIC_SPAWN_NAMES[0], 4096, 0o600), (PUBLIC_SPAWN_NAMES[1], 0, 0o644)):
                verify()
                path, row = root / name, rows[name]
                info = path.lstat() if artifacts.WINDOWS else os.stat(name, dir_fd=parent, follow_symlinks=False)
                require(set(row) == {"type", "mode", "device", "inode", "bytes", "sha256"}
                        and row["type"] == "file" and row["mode"] == (0o666 if artifacts.WINDOWS else mode)
                        and type(row["bytes"]) is int and 0 <= row["bytes"] <= maximum
                        and (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), info.st_size) ==
                            (row["device"], row["inode"], row["mode"], row["bytes"])
                        and (info.st_uid, info.st_gid) == tuple(authorities[-1])[-2:],
                        "public spawn captured member differs")
                body = ordinary_bytes(path, info, parent=None if artifacts.WINDOWS else parent)
                require(len(body) == row["bytes"] and hashlib.sha256(body).hexdigest() == row["sha256"],
                        "public spawn captured bytes differ")
                bodies[name], identities[name] = body, stable_file(info)
        finally:
            verify()
    require(bodies[PUBLIC_SPAWN_NAMES[1]] == b"", "public spawn lock is not empty")
    return {"rows": rows, "identities": identities}, bodies


def public_profile(root, image, outer, config_hashes, *, stdio=None, spawn=None, log_lock=None):
    prefix = root.relative_to(outer).as_posix()
    rows = {name[len(prefix) + 1:]: row for name, row in profile_tree(image, prefix).items() if name != prefix}
    marker = ("uap-hermes-project-session-v1\n" + str(root.parent.parent) + "\n").encode()
    owner = rows.get(".uap-owner", {})
    require(owner.get("type") == "file" and owner.get("bytes") == len(marker)
            and owner.get("sha256") == hashlib.sha256(marker).hexdigest(), "public retained root marker differs")
    directories = {"cron", "sessions", "logs", "logs/curator", "memories", "pairing", "hooks", "image_cache", "audio_cache", "skills", "backups", "backups/config"}
    files = {".uap-owner", "config.yaml", "config.yaml.agentplugins.lock", "SOUL.md", "logs/agent.log", "logs/errors.log"}
    if log_lock is not None:
        require(rows.get(PUBLIC_NATIVE_LOG_LOCK) == log_lock,
                "public native log lock changed from its current-command receipt")
        public_native_log_lock(root, log_lock)
        files.add(PUBLIC_NATIVE_LOG_LOCK)
    if stdio is not None and stdio[1]:
        require("logs/mcp-stderr.log" in rows, "public stdio log is missing")
        public_stdio_log(root, rows["logs/mcp-stderr.log"], *stdio)
        files.add("logs/mcp-stderr.log")
    if spawn is not None:
        require(type(spawn) is dict and set(spawn) == {"rows", "identities"}
                and set(spawn["rows"]) == set(spawn["identities"]) == set(PUBLIC_SPAWN_NAMES)
                and all(spawn["rows"][name] == rows.get(name) for name in PUBLIC_SPAWN_NAMES),
                "public spawn admission differs from captured image")
        files.update(PUBLIC_SPAWN_NAMES)
    backups = {name for name in rows if re.fullmatch(r"backups/config/config\.yaml\.good\.[0-9]{8}-[0-9]{6}", name)}
    try:
        require(len(backups) <= 5 and set(rows) <= directories | files | backups, "public native bootstrap member is unmeasured")
    except RuntimeError as error:
        detail = capture_public_bootstrap(root, outer, rows, backups, directories | files | backups)
        if detail is not None and type(error) is RuntimeError:
            error.__dict__["_uap_public_bootstrap"] = detail
        raise
    for name, row in rows.items():
        directory = name in directories
        mode = 0o755 if name in {"backups", "backups/config"} else 0o700 if directory else 0o644 if name.startswith("logs/") or name == "spawn-ledger.json.lock" else 0o600
        require(row["type"] == ("directory" if directory else "file") and
                row["mode"] == ((0o777 if directory else 0o666) if os.name == "nt" else mode), "public native bootstrap metadata differs")
        if name == "config.yaml.agentplugins.lock": require(row["bytes"] == 0, "public config lock is not empty")
        if name == "SOUL.md": require((row["bytes"], row["sha256"]) == (667, "36c1f5a2e92cd1d018311eaf4c8f1e8886672eae78212e033c681d0e3d5d506f"), "public native SOUL differs")
        if name in backups: require(row["sha256"] in config_hashes, "public native backup has unowned bytes")


def public_stdio_events(path, nonce, *, captured=None):
    if not path.exists():
        require(captured is None, "public stdio captured events disappeared")
        return []
    info = path.lstat()
    body = ordinary_bytes(path, info)
    if captured is not None:
        require(type(captured) is dict and set(captured) == {"type", "mode", "device", "inode", "bytes", "sha256"}
                and captured["type"] == "file"
                and (info.st_dev, info.st_ino, stat.S_IMODE(info.st_mode), len(body), hashlib.sha256(body).hexdigest()) ==
                    (captured["device"], captured["inode"], captured["mode"], captured["bytes"], captured["sha256"]),
                "public stdio events differ from captured image")
    require(body.endswith(b"\n") and len(body.splitlines()) <= 32, "public stdio event bound")
    rows = [public_json(line) for line in body.splitlines()]
    require(len(rows) % 2 == 0, "public stdio exchange incomplete")
    sessions = set()
    for first, second in zip(rows[::2], rows[1::2]):
        require([first.get("method"), second.get("method")] == ["initialize", "tools/list"]
                and first.get("session") not in sessions, "public stdio exchange order/restart differs")
        sessions.add(first["session"])
        for row in (first, second):
            require(set(row) == {"method", "pid", "session", "nonce", "timestamp", "facts"}
                    and row["nonce"] == nonce and (row["facts"] is None or public_parent_facts(row["facts"]))
                    and type(row["pid"]) is int and row["pid"] > 0
                    and row["session"] == first["session"] and row["pid"] == first["pid"]
                    and type(row["timestamp"]) is str and len(row["timestamp"]) <= 40, "public stdio event identity differs")
    return rows


def cold_report(value, pins, bundle, head, temporary):
    require(type(value) is dict and set(value) == set("kind entrypoint installer_executed source_head installer_sha256 bundle_executable runner_os architecture original_source restored_source restored_binding commands root_publications removed_state activation native_tools_call vendor_authentication model_invoked".split()), "cold report schema")
    require(value["kind"] == "hermes-bundle-cold-restore" and value["source_head"] == head
            and value["entrypoint"] == "public-cobra-in-process" and value["installer_executed"] is False
            and value["installer_sha256"] == pins["uap"]["sha256"] and value["bundle_executable"] == str(bundle)
            and value["activation"] == "prepared" and all(value[key] is False for key in ("native_tools_call", "vendor_authentication", "model_invoked"))
            and value["original_source"] == value["restored_source"], "cold report source/qualification differs")
    require(value["runner_os"] == {"linux": "linux", "darwin": "darwin", "win32": "windows"}.get(sys.platform)
            and value["architecture"] == {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower()), "cold actual platform differs")
    binding, source = value["restored_binding"], value["restored_source"]
    require(re.fullmatch(r"[a-f0-9]{40}", source.get("resolved_revision", ""))
            and source.get("tree_digest") == binding["package_revision"]["tree_digest"]
            and source["resolved_revision"] == binding["package_revision"]["resolved_revision"]
            and binding["client_id"] == "hermes" and binding["scope"] == "project"
            and binding["activation"] == "prepared" and binding["verification"] == "installation_verified"
            and binding.get("pending_native_intent") is None
            and (binding.get("native_activation_attempt") is None or type(binding["native_activation_attempt"]) is str and binding["native_activation_attempt"] == ""), "cold exact restored binding differs")
    selected = str(Path(binding["native_profile_root"]) / "uap-session")
    objects = [item for item in binding["native_objects"] if item.get("kind") == "hermes_project_session_mcp"]
    require(len(objects) == 1 and objects[0].get("logical_name") == "docs"
            and objects[0].get("object_id") == "hermes_project_session_mcp:docs"
            and objects[0].get("path") == str(Path(selected) / "config.yaml")
            and objects[0].get("protection_class") == "managed", "cold owned MCP selector differs")
    rows = value["commands"]
    require(type(rows) is list and [row.get("step") for row in rows] == ["original-inspect", "cold-inspect", "repaired-inspect"], "cold actual command sequence differs")
    inventories = []
    for row in rows:
        require(set(row) == set("step argv public_argv directory selected_root exit_code stdout_base64 stderr_base64 stdout_bytes stderr_bytes stdout_sha256 stderr_sha256 cleanup_complete no_child_started".split()), "cold command schema")
        project = Path(row["directory"])
        require(project.is_absolute() and project.resolve() == project and project.is_relative_to(temporary)
                and row["argv"] == [str(bundle), "config", "get", "mcp_servers", "--json"]
                and row["public_argv"] == ["run", "--target", "hermes", "--scope", "project", "--project-root", str(project), "--inspect", "--confirm-client-start"]
                and row["selected_root"] == str(project / ".hermes/uap-session")
                and type(row["exit_code"]) is int and row["exit_code"] == 0 and row["cleanup_complete"] is True
                and row["no_child_started"] is False, "cold actual native command/cleanup differs")
        for stream in ("stdout", "stderr"):
            encoded = row[stream + "_base64"]
            # The existing Go producer marshals a nil []byte as null. Only
            # an empty stderr can use that spelling; count and hash still join.
            require(type(encoded) is str or (stream == "stderr" and encoded is None), "cold stream encoding differs")
            body = b"" if encoded is None else base64.b64decode(encoded, validate=True)
            require(type(row[stream + "_bytes"]) is int and len(body) == row[stream + "_bytes"] <= (32768 if stream == "stderr" else LIMIT)
                    and hashlib.sha256(body).hexdigest() == row[stream + "_sha256"], "cold original stream join differs")
            require(stream != "stdout" or bool(body), "cold native inventory is empty")
            if stream == "stdout":
                inventory = public_json(body)
                require(type(inventory) is dict and set(inventory) == {"docs"}
                        and public_mcp_digest("docs", inventory["docs"]) == objects[0].get("managed_digest"),
                        "cold native inventory differs from owned receipt")
                inventories.append(inventory)
    require(inventories[0] == inventories[1] == inventories[2], "cold selected inventories differ")
    require(rows[0]["directory"] != rows[1]["directory"], "cold restored environment was not fresh")
    roots = [row for row in value["root_publications"] if str(row.get("after_digest", "")).startswith("native-root-v1:")]
    require(len(value["root_publications"]) == len(roots) == 1 and roots[0].get("active_path") == selected and roots[0].get("phase") == "committed"
            and roots[0].get("mutation_type") == "directory_swap" and type(roots[0].get("sequence")) is int and roots[0]["sequence"] > 0
            and re.fullmatch(r"native-root-v1:[a-f0-9]{64}", roots[0]["after_digest"])
            and roots[0].get("client_binding_id") == binding["client_binding_id"]
            and roots[0] in value["removed_state"].get("transaction_receipts", [])
            and all(not item.get("clients") for item in value["removed_state"]["installations"])
            and all(str(Path(row["directory"]) / ".hermes/uap-session") == selected for row in rows[1:]), "cold retained root receipt differs")
    public_parent_receipt(roots[0], Path(selected))
    return value


def public_cleanup_complete(home, proof):
    # A killed Go owner may leave a separate native process group. Once this
    # family starts, only the fully validated Cold report permits home cleanup.
    if all(not (home / name).exists() and not (home / name).is_symlink()
           for name in ("public-a", "public-b", "public-cold-tmp")):
        return True
    cold = proof.get("cold_restore")
    return (proof.get("public_project_lifecycle_complete") is True and type(cold) is dict
            and set(cold) == {"source", "commands", "sha256"}
            and cold["source"] == proof["source"]["commit"]
            and type(cold["commands"]) is int and cold["commands"] == 3
            and type(cold["sha256"]) is str and re.fullmatch(r"[a-f0-9]{64}", cold["sha256"]) is not None)


def public_cold(evidence, temporary, env, pins, deadline, head, bundle):
    public_runtime_pins(pins)
    report = evidence / "cold-restore.json"
    require(not report.exists() and not report.is_symlink() and temporary.resolve() == temporary,
            "cold report/temporary root is not fresh")
    require(env.get("GITHUB_ACTIONS") == "true" and env.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "cold invocation lacks genuine hosted environment")
    owned, descriptors, named, held = temporary / "hermes-cold", {}, {}, {}
    def members():
        with os.scandir(temporary) as entries:
            return [entry.name for _, entry in zip(range(2), entries)]
    def opened_identity(descriptor):
        info = os.fstat(descriptor)
        return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
    try:
        for path in (temporary, owned):
            if path == owned:
                require(not members(), "cold parent contains an unowned member")
                path.mkdir(mode=0o700)
            named[path] = directory_identity(path)
            before = path.lstat()
            descriptors[path] = open_inventory_directory(path)
            check_inventory_directory(before, os.fstat(descriptors[path]), "cold directory changed at open")
            held[path] = opened_identity(descriptors[path])
        # Choose once from the original monotonic budget. Go soft-cancels 30s
        # before this same absolute deadline; Python keeps its outer hard fence.
        budget = min(360, deadline - time.monotonic() - 10)
        require(budget > 40, "cold original budget lacks cancellation reserve")
        cold_env = dict(env, UAP_HERMES_PROJECT_BUNDLE_E2E="1", UAP_HERMES_PROJECT_BUNDLE_EXE=str(bundle),
                        UAP_HERMES_COLD_REPORT=str(report), TMPDIR=str(owned), TEMP=str(owned), TMP=str(owned),
                        UAP_HERMES_COLD_DEADLINE_UNIX_MS=str(int((time.time() + budget) * 1000)))
        output = bounded([pins["cold"]["path"], "-test.run=^TestProjectHermesBundleColdRestoreAndRepair$", "-test.count=1", "-test.v", "-test.timeout=6m"],
                         HERE.parents[1], cold_env, evidence / "public-cold", timeout=budget)
        require(re.search(rb"(?m)^--- PASS: TestProjectHermesBundleColdRestoreAndRepair \([0-9]+(?:\.[0-9]+)?s\)$", output)
                and b"--- SKIP:" not in output
                and ("HERMES_COLD_SOURCE " + head + " " + pins["uap"]["sha256"] + " (3 native selected commands)").encode() in output,
                "cold native test did not execute completely")
        value = cold_report(public_json(ordinary_bytes(report)), pins, bundle, head, owned)
        public_runtime_pins(pins)
        require(time.monotonic() < deadline and members() == [owned.name], "cold cleanup budget/membership differs")
        for path in (temporary, owned):
            require(directory_identity(path) == named[path] and opened_identity(descriptors[path]) == held[path], "cold original directory changed")
        # Windows cannot remove an open directory; its unchanged parent stays
        # held. Reuse the existing owned-root cleanup after native closure only.
        os.close(descriptors.pop(owned))
        shutil.rmtree(owned)
        require(directory_identity(temporary) == named[temporary] and not members(), "cold parent closure differs")
        return value
    finally:
        for descriptor in descriptors.values(): os.close(descriptor)


def public_stdio_recipe(project, probe, nonce):
    executable = Path(probe["path"])
    return {"type": "stdio", "command": executable.name, "args": [], "env": {
        "PATH": str(executable.parent), "UAP_TEST_ROOT": str(project),
        "UAP_TEST_EVENTS": "${PLUGIN_DATA}/hermes-events.jsonl", "UAP_TEST_NONCE": nonce,
        "UAP_TEST_HERMES_PUBLIC_PARENT": "1"}}


def public_lock_record(body):
    require(type(body) is bytes and 0 < len(body) <= 256, "public control lock body bound")
    value = public_json(body)
    require(type(value) is dict and set(value) == {"pid", "acquired_at"}
            and type(value["pid"]) is int and 0 < value["pid"] < 2**32
            and type(value["acquired_at"]) is str and re.fullmatch(
                r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-5][0-9](?:\.[0-9]{0,8}[1-9])?Z",
                value["acquired_at"]), "public control lock metadata differs")
    try:
        time.strptime(value["acquired_at"][:19], "%Y-%m-%dT%H:%M:%S")
    except ValueError as error:
        raise RuntimeError("public control lock timestamp differs") from error
    canonical = json.dumps({"pid": value["pid"], "acquired_at": value["acquired_at"]}, separators=(",", ":")).encode() + b"\n"
    require(body == canonical, "public control lock encoding differs")


def public_lock_identity(info):
    if artifacts.WINDOWS:
        artifacts.windows_identity(info)
    # Only size/mtime/ctime can change when the existing processlock writer
    # refreshes its record. Keep complete mode and owner in each stat domain.
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid, info.st_gid,
            getattr(info, "st_birthtime_ns", None), getattr(info, "st_birthtime", None),
            getattr(info, "st_file_attributes", None), getattr(info, "st_reparse_tag", None))


@contextmanager
def public_inspect_lock(outer, project, before, snapshot):
    require(project.parent == outer and project.name in {"public-a", "public-b"}, "public inspect project differs")
    lock, directories = project / ".uap/mutation.lock", []
    key = lock.relative_to(outer).as_posix()
    with ExitStack() as handles:
        parent = None
        for path in (outer, project, lock.parent):
            require(path.is_absolute() and path.resolve() == path, "public control parent alias")
            named = path.lstat()
            row = {"type": "directory", "device": named.st_dev, "inode": named.st_ino, "mode": stat.S_IMODE(named.st_mode)}
            require(stat.S_ISDIR(named.st_mode) and public_same_record(before.get(path.relative_to(outer).as_posix()), row),
                    "public control parent differs from original image")
            descriptor = open_inventory_directory(path, parent)
            handles.callback(os.close, descriptor)
            held = os.fstat(descriptor)
            check_inventory_directory(named, held, "public control parent changed at open")
            directories.append((path, named, descriptor, held))
            parent = None if artifacts.WINDOWS else descriptor
        named = lock.lstat()
        require(stat.S_ISREG(named.st_mode) and named.st_nlink == 1 and 0 < named.st_size <= 256
                and (named.st_uid, named.st_gid) == (directories[-1][1].st_uid, directories[-1][1].st_gid),
                "public control lock is not an original bounded ordinary file")
        metadata = {"type": "file", "mode": stat.S_IMODE(named.st_mode), "device": named.st_dev,
                    "inode": named.st_ino, "bytes": named.st_size}
        expected = before.get(key)
        require(type(expected) is dict and set(expected) == set(metadata) | {"sha256"}
                and public_same_record({field: expected[field] for field in metadata}, metadata),
                "public control lock differs from original image")
        descriptor = (artifacts.open_member(lock) if artifacts.WINDOWS else
                      os.open(lock.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent))
        handles.callback(os.close, descriptor)
        held = os.fstat(descriptor)
        require(same_opened_capture(named, held) and (named.st_uid, named.st_gid) == (held.st_uid, held.st_gid),
                "public control lock changed at open")

        def original():
            for path, previous, fd, opened in directories:
                require(path.resolve() == path and public_lock_identity(path.lstat()) == public_lock_identity(previous)
                        and public_lock_identity(os.fstat(fd)) == public_lock_identity(opened),
                        "public control original parent changed")
            current, opened = lock.lstat(), os.fstat(descriptor)
            require(public_lock_identity(current) == public_lock_identity(named)
                    and public_lock_identity(opened) == public_lock_identity(held)
                    and same_opened_capture(current, opened), "public control original lock changed")
            return current, opened

        def capture():
            current, opened = original()
            require(0 < current.st_size <= 256, "public control lock body bound")
            os.lseek(descriptor, 0, os.SEEK_SET)
            with os.fdopen(os.dup(descriptor), "rb") as stream:
                body = stream.read(257)
            public_lock_record(body)
            linked, final = original()
            require(len(body) == current.st_size and stable_file(linked) == stable_file(current)
                    and stable_file(final) == stable_file(opened), "public control lock changed during capture")
            row = {"type": "file", "mode": stat.S_IMODE(current.st_mode), "device": current.st_dev,
                   "inode": current.st_ino, "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}
            return row, current, opened

        row, _, _ = capture()
        require(public_same_record(before.get(key), row), "public control lock preimage differs")
        finished = False

        def finish():
            nonlocal finished
            require(not finished, "public control lock postimage already captured")
            finished = True
            row, current, opened = capture()
            result = snapshot()
            linked, final = original()
            require(stable_file(linked) == stable_file(current) and stable_file(final) == stable_file(opened)
                    and public_same_record(result[0].get(key), row), "public control lock postimage differs")
            return result

        yield finish


def observe_public(probes):
    import hermes_yaml as yaml
    evidence, user = Path(os.environ["HERMES_NATIVE_EVIDENCE"]), Path(os.environ["HOME"])
    outer, nonce = user.parent, os.environ["HERMES_CALL_NONCE"]
    deadline = float(os.environ["HERMES_PROFILE_DEADLINE"])
    pins = public_json(os.environ["HERMES_PUBLIC_PINS"].encode())
    public_runtime_pins(pins)
    code_root = Path(os.environ["HERMES_PROFILE_RUNTIME_ROOT"])
    code_authority, install_id = public_spawn_install(code_root, Path(os.environ["HERMES_BIN"]))
    outer_authority = directory_identity(outer)
    original = Path(public_json(ordinary_bytes(evidence / "profile-endpoints.json"))["original_native_root"])
    raw_final = public_json(ordinary_bytes(evidence / "original-user-final.json"))
    require(inventory(outer) == raw_final, "raw profile boundary changed before public lifecycle")
    projects = {scope: outer / ("public-" + scope) for scope in ("a", "b")}
    sources = {scope: outer / ("public-source-" + scope) for scope in projects}
    state_root, cold_tmp = outer / "public-uap-state", outer / "public-cold-tmp"
    for directory in (*projects.values(), *sources.values(), state_root, cold_tmp): directory.mkdir(mode=0o700)
    env = dict(os.environ, HERMES_HOME=str(original), AGENTPLUGINS_HOME=str(state_root),
               PATH=str(Path(os.environ["HERMES_BIN"]).parent) + os.pathsep + os.environ["PATH"])
    for scope, project in projects.items():
        bounded(["git", "init", "-q", str(project)], outer, env, evidence / ("public-git-" + scope), timeout=min(30, deadline-time.monotonic()))
        foreign = project / ".hermes/skills/foreign/SKILL.md"
        foreign.parent.mkdir(parents=True)
        foreign.write_bytes(("---\nname: foreign\ndescription: Retained Project Skill.\n---\nFOREIGN_" + nonce + scope + "\n").encode())
        (project / "foreign.txt").write_bytes((nonce + scope).encode())
        source = sources[scope]
        name = "hermes-public-" + scope
        (source / "skills" / name).mkdir(parents=True)
        (source / "skills" / name / "SKILL.md").write_bytes(("---\nname: " + name + "\ndescription: Inert public lifecycle Skill.\n---\nREVISION_1\n").encode())
        (source / "plugin.json").write_bytes(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": name, "version": "1.0.0", "description": "Disposable Hermes public lifecycle"}).encode())
        (source / "mcp.json").write_bytes(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": {
            name + "-http": {"type": "streamable-http", "url": probes[scope].url},
            name + "-stdio": public_stdio_recipe(project, pins["probe"], nonce)}}).encode())
    authorities = {scope: directory_identity(project) for scope, project in projects.items()}
    write_json(evidence / "public-endpoints.json", {scope: {"project": str(project), "identity": authorities[scope],
                "selected": str(project / ".hermes/uap-session"), "url": probes[scope].url} for scope, project in projects.items()})
    current, proofs = public_snapshot(outer, projects, pins)
    require(all(current.get(key) == value for key, value in raw_final.items()), "public authoring changed raw original")
    protected = {key: value for key, value in current.items() if key.startswith("user/") or key.startswith("project-")
                 or key.endswith("foreign.txt") or "/skills/foreign" in key}
    configs, records, born, complete = {scope: set() for scope in projects}, [], {}, False
    stdio_completed = {scope: 0 for scope in projects}
    spawn_states = {scope: None for scope in projects}
    native_log_locks = {scope: None for scope in projects}

    def snapshot():
        image, bindings = public_snapshot(outer, projects, pins)
        verify_root_identities(projects, authorities)
        protected_unchanged(protected, image, protected)
        for scope, proof in bindings.items():
            if proof is not None:
                identity = (proof["root_identity"], proof["parent_identity"], proof["root_receipt"])
                require(scope not in born or born[scope] == identity, "public durable root was replaced or renewed")
                born.setdefault(scope, identity)
        for scope, identity in born.items():
            selected = projects[scope] / ".hermes/uap-session"
            require((directory_identity(selected), directory_identity(selected.parent)) == identity[:2], "public retained root/parent changed")
        return image, bindings

    def spawn_capture(scope, image):
        require(scope in born, "public spawn root has no original receipt")
        root = projects[scope] / ".hermes/uap-session"
        identities = (outer_authority, authorities[scope], born[scope][1], born[scope][0])
        return public_spawn_read(root, outer, image, identities)

    def spawn_preserved(scope, image):
        if spawn_states[scope] is None:
            require(not public_spawn_rows(image, outer, projects[scope] / ".hermes/uap-session"),
                    "public spawn files appeared outside an owned stdio command")
        else:
            public_spawn_continuity(spawn_states[scope], spawn_capture(scope, image)[0])

    def captured_events(proof, image):
        if proof is None:
            return []
        path = proof["data"] / "hermes-events.jsonl"
        row = image.get(path.relative_to(outer).as_posix())
        events = public_stdio_events(path, nonce, captured=row)
        require(bool(events) == (row is not None), "public stdio event capture membership differs")
        return events

    def authored(paths, change):
        nonlocal current, proofs
        before, _ = snapshot()
        require(before == current, "unmeasured public inter-command effect")
        change()
        after, proofs = snapshot()
        require(set(changes(before, after)) == {path.relative_to(outer).as_posix() for path in paths}, "authored public effect differs")
        records.append({"label": "authored", "paths": [str(path.relative_to(outer)) for path in paths], "delta": changes(before, after)})
        current = after

    def command(label, scope, args, *, native=False, inspect=False, expected_failure=False, test=None):
        nonlocal current, proofs
        public_runtime_pins(pins, rehash=False)
        before, previous = snapshot()
        require(before == current and deadline-time.monotonic() > 10, "public command original budget/baseline differs")
        project, name = projects[scope], "hermes-public-" + scope
        selected = project / ".hermes/uap-session"
        http_before = {key: list(probe.methods()) for key, probe in probes.items()}
        for key in projects:
            spawn_preserved(key, before)
        events_before = {key: captured_events(proof, before) for key, proof in previous.items()}
        argv = [os.environ["HERMES_BIN"], *args] if native else [pins["uap"]["path"], *args, "--target", "hermes", "--scope", "project", "--project-root", str(project)]
        if not native and not inspect: argv += ["--yes", "--format", "json"]
        with ExitStack() as handles:
            finish = snapshot
            if inspect:
                require(not native and (scope, label) in {("a", "public-run-a"), ("b", "public-run-b"),
                                                       ("a", "public-repaired-run-a")}
                        and args == ["run", "--inspect", "--confirm-client-start"] and not expected_failure and test is None,
                        "public inspect control role differs")
                finish = handles.enter_context(public_inspect_lock(outer, project, before, snapshot))
            output = bounded(argv, project, dict(env, HERMES_HOME=str(selected)) if native else env,
                             evidence / label, timeout=min(90, deadline-time.monotonic()-5), expected_failure=expected_failure)
            after, proofs = finish()
        for key, proof in previous.items():
            if proof is not None and proofs[key] is not None:
                public_continuity(proof, proofs[key], advance=key == scope and label == "public-update-a")
        delta = changes(before, after)
        selected_prefix = selected.relative_to(outer).as_posix()
        permitted = [selected_prefix]
        if native or inspect:
            require(previous[scope] is not None or scope in born and label in
                    {"public-removed-list-" + scope, "public-removed-test-" + scope}, "native public action lacks owned source")
            if previous[scope]: permitted.append((previous[scope]["data"] / "hermes-events.jsonl").relative_to(outer).as_posix())
        else:
            permitted += [(project / ".uap").relative_to(outer).as_posix(), (project / ".hermes/skills" / name).relative_to(outer).as_posix(), "public-uap-state"]
        control_lock = (project / ".uap/mutation.lock").relative_to(outer).as_posix() if inspect else None
        require(all(path == control_lock or any(path == prefix or path.startswith(prefix + "/") for prefix in permitted)
                    for path in delta), "public action changed an unselected or unmeasured path")
        other = "b" if scope == "a" else "a"
        require(profile_tree(before, projects[other].name) == profile_tree(after, projects[other].name), "public sibling changed")
        if not native and not inspect:
            require(public_json(output).get("result") == "success", "public command did not return success")
        config = selected / "config.yaml"
        servers = {}
        if config.exists():
            body = ordinary_bytes(config); value = yaml.safe_load(body)
            require(type(value) is dict and set(value) <= {"mcp_servers"}, "public selected config has unowned settings")
            servers = value.get("mcp_servers", {})
            require(type(servers) is dict, "public selected config collection differs")
            configs[scope].add(hashlib.sha256(body).hexdigest())
        if proofs[scope] is not None:
            require(set(servers) == {name + "-http", name + "-stdio"} and servers[name + "-http"] == {"url": probes[scope].url}, "public selected config source differs")
            for obj in proofs[scope]["binding"]["native_objects"]:
                if obj["kind"] == "hermes_project_session_mcp":
                    require(obj["path"] == str(config) and obj["managed_digest"] == public_mcp_digest(obj["logical_name"], servers[obj["logical_name"]]),
                            "public owned MCP digest differs")
            skill_prefix = (project / ".hermes/skills" / name).relative_to(outer).as_posix()
            skill_source = (proofs[scope]["target"] / "skills" / name / "SKILL.md").relative_to(outer).as_posix()
            require(set(profile_tree(after, skill_prefix)) == {skill_prefix, skill_prefix + "/SKILL.md"}
                    and after[skill_prefix + "/SKILL.md"]["sha256"] == after[skill_source]["sha256"], "public delivered Skill differs")
            if inspect: require(public_json(output) == servers, "actual public selected inventory differs")
        else:
            require(not servers and not (project / ".hermes/skills" / name).exists(), "public removal retained owned sources")
        http_after = {key: list(probe.methods()) for key, probe in probes.items()}
        protocol_join(http_before, http_after, scope if test == "http" else None, 1 if test == "http" else 0)
        events_after = {}
        for key, proof in previous.items():
            retained = proofs[key] or proof
            events = captured_events(retained, after)
            events_after[key] = events
            require(events[:len(events_before[key])] == events_before[key]
                    and len(events)-len(events_before[key]) == (2 if key == scope and test == "stdio" else 0), "public stdio dispatch differs")
            if retained is not None:
                stdio_completed[key] = len(events) // 2
            if proof is not None and not (key == scope and test == "stdio"):
                prefix = proof["data"].relative_to(outer).as_posix()
                require(profile_tree(before, prefix) == profile_tree(after, prefix), "public operation changed retained data")
        prospective = dict(spawn_states)
        for key in projects:
            if key != scope or test != "stdio":
                spawn_preserved(key, after)
                continue
            require(native and not inspect and not expected_failure and previous[scope] is not None
                    and args == ["mcp", "test", name + "-stdio"]
                    and directory_identity(code_root) == code_authority,
                    "public spawn current command/bundled code authority differs")
            receipt, bodies = spawn_capture(scope, after)
            launcher = public_spawn_launcher(previous[scope], servers[name + "-stdio"], pins["probe"]["path"])
            public_spawn_entry(bodies[PUBLIC_SPAWN_NAMES[0]], events_before[scope], events_after[scope], nonce,
                public_spawn_status(evidence, label, argv, output), install_id, launcher, pins["probe"]["path"], sys.platform)
            public_spawn_continuity(spawn_states[scope], receipt, stdio=True)
            prospective[scope] = receipt
        log_key = selected_prefix + "/logs/mcp-stderr.log"
        if test != "stdio":
            require(before.get(log_key) == after.get(log_key), "public non-stdio action changed the stdio log")
        native_lock_key = selected_prefix + "/" + PUBLIC_NATIVE_LOG_LOCK
        observed_lock = after.get(native_lock_key)
        if native_log_locks[scope] is None:
            if observed_lock is not None:
                require(os.name == "nt" and native and not inspect and not expected_failure
                        and test == "http" and previous[scope] is not None
                        and before.get(native_lock_key) is None
                        and args == ["mcp", "test", name + "-http"],
                        "public native log lock appeared without this owned HTTP command")
                public_native_log_lock(selected, observed_lock)
                native_log_locks[scope] = dict(observed_lock)
        else:
            require(before.get(native_lock_key) == native_log_locks[scope] == observed_lock,
                    "public native log lock changed after its admitted HTTP command")
        if selected.exists():
            public_profile(selected, after, outer, configs[scope], stdio=(name + "-stdio", stdio_completed[scope]),
                           spawn=prospective[scope], log_lock=native_log_locks[scope])
        records.append({"label": label, "scope": scope, "delta": delta,
                        "http_methods": http_after, "stdio_counts": {key: len(events_before[key]) for key in projects}})
        spawn_states.update(prospective)
        current = after
        return output

    try:
        for scope in projects:
            command("public-add-" + scope, scope, ["add", str(sources[scope]), "--no-save"])
        before_repeat = proofs["a"]["state_sha256"]
        command("public-repeat-a", "a", ["add", str(sources["a"]), "--no-save"])
        require(proofs["a"]["state_sha256"] == before_repeat, "public repeat rewrote receipts")
        for scope in projects:
            command("public-run-" + scope, scope, ["run", "--inspect", "--confirm-client-start"], inspect=True)
            for transport in ("http", "stdio"):
                command("public-" + transport + "-" + scope, scope, ["mcp", "test", "hermes-public-" + scope + "-" + transport], native=True, test=transport)
        for transport in ("http", "stdio"):
            command("public-" + transport + "-restart-a", "a", ["mcp", "test", "hermes-public-a-" + transport], native=True, test=transport)
        manifest, skill = sources["a"] / "plugin.json", sources["a"] / "skills/hermes-public-a/SKILL.md"
        def advance():
            value = public_json(ordinary_bytes(manifest)); value["version"] = "1.1.0"
            manifest.write_bytes(json.dumps(value).encode()); skill.write_bytes(ordinary_bytes(skill).replace(b"REVISION_1", b"REVISION_2"))
        prior = proofs["a"]
        authored([manifest, skill], advance)
        command("public-update-a", "a", ["update", "hermes-public-a"])
        require(proofs["a"]["installed"]["source"]["tree_digest"] != prior["installed"]["source"]["tree_digest"]
                and proofs["a"]["installed"]["package"]["version"] == "1.1.0"
                and proofs["a"]["installed"]["installation_id"] == prior["installed"]["installation_id"]
                and proofs["a"]["data"] == prior["data"], "public changed update lost revision or retained data")
        selected_skill = projects["a"] / ".hermes/skills/hermes-public-a/SKILL.md"
        selected_config = proofs["a"]["selected"] / "config.yaml"
        authored([selected_skill, selected_config], lambda: (selected_skill.unlink(), selected_config.unlink()))
        command("public-repair-a", "a", ["repair", "hermes-public-a"])
        require(ordinary_bytes(selected_skill) == ordinary_bytes(skill), "public missing repair did not restore recorded Skill")
        command("public-repaired-run-a", "a", ["run", "--inspect", "--confirm-client-start"], inspect=True)
        for transport in ("http", "stdio"):
            command("public-" + transport + "-repaired-a", "a", ["mcp", "test", "hermes-public-a-" + transport], native=True, test=transport)
        for scope in projects:
            prior = proofs[scope]
            command("public-remove-" + scope, scope, ["remove", "hermes-public-" + scope])
            state = public_json(ordinary_bytes(projects[scope] / ".uap/state-v2.json"))
            retained = state["installations"][0]
            mutable = {"clients", "data_retained", "updated_at", "operation_group_id"}
            require(retained.get("clients") == {} and retained.get("data_retained") is True
                    and public_same_record({key: value for key, value in retained.items() if key not in mutable},
                                           {key: value for key, value in prior["installed"].items() if key not in mutable})
                    and directory_identity(prior["data"]) == prior["data_identity"],
                    "public removal changed the accepted installation/source/data authority")
            require(prior["root_receipt"] in state.get("transaction_receipts", []) and not prior["target"].exists()
                    and directory_identity(prior["selected"]) == prior["root_identity"]
                    and directory_identity(prior["selected"].parent) == prior["parent_identity"]
                    and public_json(ordinary_bytes(prior["data"] / ".agentplugins-data-owner.json")) == prior["installed"]["data_receipts"][prior["binding"]["data_receipt_id"]], "public removal lost retained root/data")
            command("public-removed-list-" + scope, scope, ["mcp", "list"], native=True, expected_failure=False)
            command("public-removed-test-" + scope, scope, ["mcp", "test", "hermes-public-" + scope + "-http"], native=True, expected_failure=True)
        public_cold(evidence, cold_tmp, env, pins, deadline, os.environ["EXPECTED_HEAD"], Path(os.environ["HERMES_BIN"]))
        after, _ = snapshot(); require(after == current and not list(cold_tmp.iterdir()), "cold fixture changed retained public/User tree")
        for scope in projects:
            spawn_preserved(scope, after)
        complete = True
    finally:
        write_json(evidence / "public-command-deltas.json", {"complete": complete, "commands": records, "root_authorities": born,
                   "production_project_mcp": False, "admission_qualified": False,
                   "http_methods": {name: list(probe.methods()) for name, probe in probes.items()}})


COMMANDS = ({prefix + x for prefix in ("git-", "guard-", "config-path-", "config-mcp-", "list-", "test-", "remove-mcp-", "removed-list-", "removed-test-") for x in ("a", "b")}
            | {"test-a-restart", "list-user", "version-a"})
COMMANDS |= ({"public-" + prefix + scope for prefix in ("git-", "add-", "run-", "http-", "stdio-", "remove-", "removed-list-", "removed-test-") for scope in ("a", "b")}
             | {"public-" + name for name in ("repeat-a", "http-restart-a", "stdio-restart-a", "update-a", "repair-a", "repaired-run-a", "http-repaired-a", "stdio-repaired-a", "cold")})
MEMBERS = ({name + suffix for name in COMMANDS for suffix in (".stdout", ".stderr", ".status.json")}
           | {"original-user-baseline.json", "original-user-final.json", "source-guards.json", "profile-endpoints.json",
              "profile-command-deltas.json", "bootstrap-census-request.json", "bootstrap-census.json", "source-user-preparation.json",
              "source-payload-refusal.json", "source-user-delta.json",
              "profile-admission.json", "lifecycle.stdout", "lifecycle.stderr", "owned-start-gate",
              "public-endpoints.json", "public-command-deltas.json", "cold-restore.json"})


def admit_evidence(directory, *, failure=None):
    try:
        total = 0
        for path in directory.iterdir():
            info = path.lstat()
            require(path.name in MEMBERS and stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                    and info.st_size <= LIMIT, "closed admission evidence member refused")
            total += info.st_size
            require(total <= 16 << 20, "admission evidence aggregate bound")
            if path.name in ("bootstrap-census-request.json", "bootstrap-census.json"):
                require(info.st_size <= 16 << 10, "census evidence bound")
                validate_census_member(path.name, ordinary_bytes(path, info))
            elif path.name == "source-user-preparation.json":
                source_preparation_document(ordinary_bytes(path, info))
            elif path.name == "source-payload-refusal.json":
                require(info.st_size <= 2048, "source refusal evidence bound")
                source_refusal_document(ordinary_bytes(path, info))
            elif path.name == "source-user-delta.json":
                require(info.st_size <= 16 << 10, "source delta evidence bound")
                source_delta_document(ordinary_bytes(path, info))
    except BaseException:
        if failure is not None:
            raise RuntimeError("original Hermes refusal retained; evidence publication refused") from failure
        raise


def census_readout(value):
    """Project only an already validated census into a measurement statement."""
    result = {"schema_version": 1, "kind": "hermes-bootstrap-census-readout",
              "measurement_only": True, "admission_qualified": False, "present": value is not None}
    if value is not None:
        names = ("schema_version", "measurement_only", "admission_qualified", "complete", "status", "reason")
        if "groups" in value:
            names += ("overflow", "groups", "entries", "logical_regular_bytes")
        result["census"] = {name: value[name] for name in names}
    return result


def source_user_failure_readout(line, result, proof, value, context):
    """Append only captured User change hashes; any optional failure keeps the exact original line."""
    if value is None:
        return line
    try:
        require(json.dumps(result, sort_keys=True, separators=(",", ":")) == line
                and len(line.encode("ascii")) + len("HERMES_ADMISSION_FAILURE ") + 1 <= 2048,
                "original failure readout differs")
        context = native_failure_context(context)
        require(context["primary_family_settled"] is True
                and proof.get("runtime_distribution") == "source" and proof.get("status") == "failed"
                and all(proof.get(key) is False for key in ("capture_complete", "admission_qualified", "production_project_mcp",
                    "vendor_authentication", "real_model_inference", "tools_call", "source_install_custom_root_qualified",
                    "source_bootstrap_recovery_qualified")), "settled source User readout required")
        source = proof.get("source")
        require(type(source) is dict and set(source) == {"commit", "tree", "dirty"} and source["dirty"] == ""
                and type(source["tree"]) is str and re.fullmatch(r"[a-f0-9]{40}", source["tree"]) is not None
                and source["commit"] == context["source_head"] == result["source_head"]
                and proof.get("source_after") == source and proof.get("nonce") == context["nonce"]
                and result["nonce_sha256"] == hashlib.sha256(context["nonce"].encode("ascii")).hexdigest(),
                "source User readout identity differs")
        value = source_delta_document(json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii"))
        require(value["nonce"] == context["nonce"], "source User delta nonce differs")
        prepared, after = proof.get("source_prepared_runtime"), proof.get("runtime_after")
        require(type(prepared) is dict and type(after) is dict
                and all(type(item.get("user")) is dict and type(item["user"].get("entries")) is int for item in (prepared, after))
                and prepared["user"] == value["before"] and after["user"] == value["after"],
                "source User delta summaries differ")
        rows = value["changes"]
        require(value["total_changes"] > 0 and rows, "source User delta has no retained change")
        non_directory = next((index for index, row in enumerate(rows)
            if any(record is not None and record["kind"] in ("file", "link")
                   for record in (row["before"], row["after"]))), None)
        def selected(index):
            row = rows[index]
            left, right = row["before"], row["after"]
            fields = ("kind", *SOURCE_DELTA_FIELDS, "integer_saturated", "sha256", "target_sha256")
            def part(record, key):
                if record is None:
                    return None
                return record["metadata"][key] if key in SOURCE_DELTA_FIELDS else record[key]
            return dict(index=index, member_sha256=row["member_sha256"],
                        before_kind=part(left, "kind"), after_kind=part(right, "kind"),
                        before_integer_saturated=part(left, "integer_saturated"),
                        after_integer_saturated=part(right, "integer_saturated"),
                        changed_fields=[key for key in fields if part(left, key) != part(right, key)])
        indices = [0] if non_directory in (None, 0) else [0, non_directory]
        projection = dict(stage=value["stage"], hash_encoding=value["hash_encoding"],
                          total_changes=value["total_changes"], retained_changes=len(rows), truncated=value["truncated"],
                          before_entries=value["before"]["entries"], after_entries=value["after"]["entries"],
                          selection="first-retained-and-first-retained-nondirectory",
                          first_non_directory_index=non_directory, selected_changes=[selected(index) for index in indices])
        extended = json.dumps(dict(result, source_user_delta=projection), sort_keys=True, separators=(",", ":"))
        require(len(extended.encode("ascii")) + len("HERMES_ADMISSION_FAILURE ") + 1 <= 2048, "source User readout bound")
        return extended
    except BaseException:
        return line  # Never replace a refusal, its existing diagnostics, or an original exception.


def admission_failure_readout(body, source_refusal, *, native_failure=None, source_delta=None):
    """Project already staged failure bytes; never read or authorize native state."""
    try:
        proof = public_json(body)
        flags = ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference",
                 "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")
        require(type(proof) is dict and proof.get("status") == "failed"
                and proof.get("runtime_distribution") in ("source", "bundled")
                and all(proof.get(key) is False for key in flags), "failure readout is not unqualified")
        context, native, refusal = None, None, None
        if source_refusal is not None or native_failure is not None:
            source = proof.get("source")
            require(proof.get("capture_complete") is False and "native_failure" not in proof
                    and type(source) is dict and set(source) == {"commit", "tree", "dirty"} and source["dirty"] == ""
                    and all(type(source[key]) is str and re.fullmatch(r"[a-f0-9]{40}", source[key]) is not None
                            for key in ("commit", "tree"))
                    and proof.get("source_after") == source, "failure readout lacks unchanged source")
            head, nonce = source["commit"], proof.get("nonce")
            require(type(nonce) is str and re.fullmatch(r"[a-f0-9]{64}", nonce) is not None, "failure readout nonce differs")
            if source_refusal is not None:
                require(type(source_refusal) is dict and proof["runtime_distribution"] == "source", "source failure readout differs")
                value = source_refusal_document(json.dumps(source_refusal, sort_keys=True, separators=(",", ":")).encode("ascii"))
                require(nonce == value["nonce"], "source failure readout nonce differs")
                names = ("stage", "operation", "member", "member_sha256", "hash_encoding", "metadata", "integer_saturated",
                         "bytes_read", "errno", "winerror", "target_sha256", "target_bytes")
                refusal = {name: value[name] for name in names}
                if "profile_bound" in value:
                    refusal["profile_bound"] = value["profile_bound"]
            if native_failure is not None:
                context = native_failure_context(native_failure)
                require(context["source_head"] == head and context["nonce"] == nonce
                        and context["primary_family_settled"] is True, "settled failure context differs")
        else:
            context = native_failure_context(proof.get("native_failure"))
            head, nonce = context["source_head"], context["nonce"]
        if context is not None:
            native = {name: context[name] for name in ("cleanup_phase", "primary_family_settled", "observer_exit")}
            observer = context["observer"]
            native["observer"] = ({name: observer[name] for name in ("phase", "reason", "exception_kind", "command", "caller")}
                                  if observer is not None else None)
            if observer is not None and "public_bootstrap" in observer:
                native["observer"]["public_bootstrap"] = observer["public_bootstrap"]
        result = dict(schema_version=1, kind="hermes-admission-failure-readout", measurement_only=True,
                      admission_qualified=False, native_qualified=False, runtime_distribution=proof["runtime_distribution"],
                      source_head=head, nonce_sha256=hashlib.sha256(nonce.encode("ascii")).hexdigest(),
                      source_refusal=refusal, native_failure=native)
        line = json.dumps(result, sort_keys=True, separators=(",", ":"))
        require(len(line.encode("ascii")) + len("HERMES_ADMISSION_FAILURE ") + 1 <= 2048, "failure readout bound")
        return source_user_failure_readout(line, result, proof, source_delta, context)
    except (AttributeError, KeyError, IndexError, TypeError, ValueError, UnicodeError, RuntimeError):
        return None  # Optional diagnostics never replace the original native refusal.


def stage_evidence(directory, *, native_failure=None):
    global EVIDENCE_CLOSED
    identity = directory_identity(directory)
    admit_evidence(directory)
    sources = {path.name: path.lstat() for path in directory.iterdir()}
    upload = directory / "upload"
    require(not upload.exists() and not upload.is_symlink(), "upload destination already exists")
    pending = directory / (".upload-pending-" + secrets.token_hex(16))
    pending.mkdir(mode=0o700)
    pending_identity = directory_identity(pending)
    census, source_refusal, source_delta, source_proof = None, None, None, None
    profile_body = None
    try:
        for name, expected in sorted(sources.items()):
            body = ordinary_bytes(directory / name, expected)
            if name == "profile-admission.json":
                profile_body = body
            if name in ("bootstrap-census-request.json", "bootstrap-census.json"):
                value = validate_census_member(name, body)
                if name == "bootstrap-census.json":
                    census = value
            elif name == "source-user-preparation.json":
                source_preparation_document(body)
            elif name == "source-payload-refusal.json":
                source_refusal = source_refusal_document(body)
            elif name == "source-user-delta.json":
                source_delta = source_delta_document(body)
            elif name == "profile-admission.json" and set(sources) & {"source-payload-refusal.json", "source-user-delta.json"}:
                source_proof = public_json(body)
            with (pending / name).open("xb") as stream:
                stream.write(body)
        observations = [item for item in (source_refusal, source_delta) if item is not None]
        if observations:
            require(type(source_proof) is dict and all(source_proof.get("nonce") == item["nonce"] for item in observations)
                    and source_proof.get("runtime_distribution") == "source" and source_proof.get("status") == "failed"
                    and all(source_proof.get(key) is False for key in ("capture_complete", "admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference", "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")), "source refusal proof differs")
        if source_delta is not None:
            prepared, after = source_proof.get("source_prepared_runtime"), source_proof.get("runtime_after")
            require(type(prepared) is dict and type(after) is dict
                    and all(type(item.get("user")) is dict and type(item["user"].get("entries")) is int for item in (prepared, after))
                    and prepared.get("user") == source_delta["before"]
                    and after.get("user") == source_delta["after"], "source delta proof differs")
        require(directory_identity(directory) == identity
                and {path.name for path in directory.iterdir()} == set(sources) | {pending.name},
                "evidence directory/membership changed before publication")
        for name, expected in sources.items():
            require(stable_file((directory / name).lstat()) == stable_file(expected), "evidence changed before publication")
        pending.rename(upload)
        EVIDENCE_CLOSED = True
    except BaseException:
        # Only this fresh private staging directory is ours; no visible partial upload.
        require(directory_identity(pending) == pending_identity, "private evidence staging directory changed")
        shutil.rmtree(pending)
        raise
    # This optional log statement owns no artifact or native success decision.
    try:
        print("HERMES_BOOTSTRAP_CENSUS " + json.dumps(census_readout(census), sort_keys=True, separators=(",", ":")), flush=True)
    except (OSError, ValueError):
        pass
    try:
        failure_line = admission_failure_readout(profile_body, source_refusal, native_failure=native_failure, source_delta=source_delta)
        if failure_line is not None:
            print("HERMES_ADMISSION_FAILURE " + failure_line, flush=True)
    except (OSError, ValueError):
        pass


OBSERVER_PHASES = ("profile-probes", "profiles", "public-probes", "public", "probe-cleanup")
OBSERVER_ERRORS = (RuntimeError, ValueError, TypeError, KeyError, IndexError, AttributeError,
                   OSError, PermissionError, FileNotFoundError, FileExistsError, TimeoutError,
                   subprocess.TimeoutExpired, AssertionError, ImportError, ModuleNotFoundError,
                   SystemExit, KeyboardInterrupt)
CLEANUP_PHASES = ("primary-family", "public-family", "profile-roots", "public-roots", "outer-home", "census", "remove-home")


def observer_source_codes():
    """Only code objects already loaded from this exact observer, never traceback paths."""
    pending = [function.__code__ for function in globals().values()
               if type(function) is type(observer_source_codes) and function.__globals__ is globals()]
    result = {}
    for _ in range(2048):
        if not pending:
            return result
        code = pending.pop()
        if code.co_filename == __file__:
            result[(code.co_name, code.co_firstlineno)] = code
            pending.extend(item for item in code.co_consts if type(item) is type(code))
    return {}  # No partial caller authority if the finite own-source walk overflows.


def observer_failure_document(body, nonce, source):
    require(type(body) is bytes and 0 < len(body) <= 2048, "observer diagnostic bound")
    value = census_document(body)
    fields = set("schema_version measurement_only admission_qualified nonce source_head phase reason exception_kind command caller".split())
    if type(value) is dict and "public_bootstrap" in value:
        fields.add("public_bootstrap")
    require(type(value) is dict and set(value) == fields, "closed observer diagnostic")
    require(type(value["schema_version"]) is int and value["schema_version"] == 1
            and value["measurement_only"] is True and value["admission_qualified"] is False
            and type(nonce) is str and re.fullmatch(r"[a-f0-9]{64}", nonce) is not None
            and type(source) is str and re.fullmatch(r"[a-f0-9]{40}", source) is not None
            and value["nonce"] == nonce and value["source_head"] == source
            and value["phase"] in OBSERVER_PHASES and value["reason"] == "uncaught-observer-exception"
            and value["exception_kind"] in tuple(kind.__name__ for kind in OBSERVER_ERRORS) + ("other",), "observer diagnostic identity")
    require(value["command"] is None or type(value["command"]) is str and value["command"] in COMMANDS, "observer command is not closed")
    caller = value["caller"]
    if caller is not None:
        require(type(caller) is dict and set(caller) == {"function", "function_line", "line"}
                and type(caller["function"]) is str and type(caller["function_line"]) is int
                and type(caller["line"]) is int, "observer caller shape")
        code = observer_source_codes().get((caller["function"], caller["function_line"]))
        require(code is not None and caller["line"] in {line for _, _, line in code.co_lines()}, "observer caller is not own source")
    if "public_bootstrap" in value:
        detail = public_bootstrap_detail(value["public_bootstrap"])
        require(value["phase"] == "public" and value["exception_kind"] == "RuntimeError"
                and caller is not None and code is public_profile.__code__
                and type(value["command"]) is str and value["command"].startswith("public-")
                and value["command"].endswith("-" + detail["scope"]), "public bootstrap observer binding")
    require(body == json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n", "observer diagnostic is not one canonical record")
    return value


def emit_observer_failure(error, phase):
    try:
        nonce, source = os.environ["HERMES_CALL_NONCE"], os.environ["HERMES_PROFILE_SOURCE_HEAD"]
        codes, caller, command, trace = observer_source_codes(), None, None, error.__traceback__
        commands = {code for owner in (observe_profiles, observe_public) for code in owner.__code__.co_consts
                    if type(code) is type(owner.__code__) and code.co_name == "command"}
        for _ in range(256):
            if trace is None:
                break
            code = trace.tb_frame.f_code
            if trace.tb_frame.f_globals is globals() and codes.get((code.co_name, code.co_firstlineno)) is code:
                caller = dict(function=code.co_name, function_line=code.co_firstlineno, line=trace.tb_lineno)
                if code in commands:
                    label = trace.tb_frame.f_locals.get("label")
                    command = label if type(label) is str and label in COMMANDS else None
            trace = trace.tb_next
        if trace is not None:
            caller, command = None, None
        value = dict(schema_version=1, measurement_only=True, admission_qualified=False, nonce=nonce, source_head=source,
                     phase=phase, reason="uncaught-observer-exception",
                     exception_kind=type(error).__name__ if type(error) in OBSERVER_ERRORS else "other", command=command, caller=caller)
        if type(error) is RuntimeError and "_uap_public_bootstrap" in error.__dict__:
            try:
                value["public_bootstrap"] = public_bootstrap_detail(error.__dict__["_uap_public_bootstrap"])
            except BaseException:
                pass
        try:
            body = json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n"
            observer_failure_document(body, nonce, source)
        except BaseException:
            if "public_bootstrap" not in value:
                raise
            value.pop("public_bootstrap")
            body = json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n"
            observer_failure_document(body, nonce, source)
        sys.stdout.buffer.write(body)
        sys.stdout.buffer.flush()
    except BaseException:
        pass  # Optional diagnosis must never replace the original child exception.


def read_observer_failure(descriptor, original, nonce, source, *, primary_settled):
    if primary_settled is not True or descriptor is None or original is None:
        return None
    try:
        before = os.fstat(descriptor)
        fields = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid")
        require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 < before.st_size <= 2048
                and all(getattr(before, field) == getattr(original, field) for field in fields), "retained observer descriptor changed")
        os.lseek(descriptor, 0, os.SEEK_SET)
        body = os.read(descriptor, 2049)
        require(stable_file(os.fstat(descriptor)) == stable_file(before) and len(body) == before.st_size,
                "retained observer diagnostic changed while reading")
        return observer_failure_document(body, nonce, source)
    except (OSError, TypeError, ValueError, RuntimeError, RecursionError):
        return None  # No reopen, raw fallback, or authority from incomplete diagnostics.


def native_failure_context(value):
    require(type(value) is dict and set(value) == {"source_head", "nonce", "cleanup_phase", "primary_family_settled", "observer_exit", "observer"}, "closed native failure context")
    require(type(value["source_head"]) is str and re.fullmatch(r"[a-f0-9]{40}", value["source_head"]) is not None
            and type(value["nonce"]) is str and re.fullmatch(r"[a-f0-9]{64}", value["nonce"]) is not None
            and value["cleanup_phase"] in CLEANUP_PHASES and type(value["primary_family_settled"]) is bool
            and (value["observer_exit"] is None or type(value["observer_exit"]) is int and -(2**32) <= value["observer_exit"] < 2**32), "native failure context identity")
    if value["observer"] is not None:
        require(value["primary_family_settled"] is True, "unsettled observer diagnostic")
        observer_failure_document(json.dumps(value["observer"], sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n", value["nonce"], value["source_head"])
    return value


def safe_failure_evidence(runner, failure_kind="unclosed-native-admission", *, native_failure=None):
    source_capture = os.environ.get("HERMES_PROFILE_SOURCE_CAPTURE") == "1"
    directory = Path(tempfile.mkdtemp(prefix="hermes-profile-source-evidence-" if source_capture else "hermes-profile-admission-evidence-", dir=runner))
    proof = {"status": "failed", "error": "native admission aborted before evidence closure", "original_failure_kind": failure_kind,
               "admission_qualified": False, "production_project_mcp": False, "vendor_authentication": False,
               "real_model_inference": False, "tools_call": False,
               "runtime_distribution": "source" if source_capture else "bundled", "source_install_custom_root_qualified": False,
               "source_bootstrap_recovery_qualified": False}
    if native_failure is not None:
        try:
            proof["native_failure"] = native_failure_context(native_failure)
        except (TypeError, ValueError, RuntimeError):
            pass
    write_json(directory / "profile-admission.json", proof)
    stage_evidence(directory)
    return directory


def child_case():
    from contextlib import ExitStack
    from pi_project_components_native_e2e import PiHTTPMCPProbe
    phase = "profile-probes"
    try:
        with ExitStack() as stack:
            probes = {name: stack.enter_context(PiHTTPMCPProbe(nonce=os.environ["HERMES_CALL_NONCE"]))
                      for name in ("a", "b")}
            phase = "profiles"
            observe_profiles(probes)
            if not source_capture_mode():
                phase = "public-probes"
                public = {name: stack.enter_context(PiHTTPMCPProbe(nonce=os.environ["HERMES_CALL_NONCE"])) for name in ("a", "b")}
                phase = "public"
                observe_public(public)
            phase = "probe-cleanup"
    except BaseException as error:
        emit_observer_failure(error, phase)
        raise


def run():
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "genuine GitHub-hosted admission required")
    runner, checkout = Path(os.environ["RUNNER_TEMP"]).resolve(), HERE.parents[1]
    source_capture = source_capture_mode()
    official_source = BASE["source_root"]() if source_capture else runner / "hermes-agent"
    before, official = BASE["source_identity"](checkout), BASE["source_identity"](official_source)
    require(before["commit"] == os.environ["HERMES_NATIVE_UAP_HEAD"] and not before["dirty"], "exact clean UAP source required")
    require(official["commit"] == BASE["PIN"] and not official["dirty"], "unmodified pinned Hermes required")
    evidence = Path(tempfile.mkdtemp(prefix="hermes-profile-source-evidence-" if source_capture else "hermes-profile-admission-evidence-", dir=runner))
    home = runner / "hermes-native-home" if source_capture else Path(tempfile.mkdtemp(prefix="hermes-profile-admission-home-", dir=runner))
    home_authority = directory_identity(home)
    identity = (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode)
    evidence_identity = (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode)
    evidence_authority = directory_identity(evidence)
    user = home / "user"
    if source_capture:
        directory_identity(user)
    else:
        user.mkdir(mode=0o700)
    nonce, child, job, failure = secrets.token_hex(32), None, None, None
    proof = {"source": before, "hermes_source": official, "nonce": nonce, "capture_complete": False,
             "admission_qualified": False, "production_project_mcp": False,
             "vendor_authentication": False, "real_model_inference": False, "tools_call": False,
             "runtime_distribution": "source" if source_capture else "bundled", "source_install_custom_root_qualified": False,
             "source_bootstrap_recovery_qualified": False}
    payload = runner / "hermes-profile-payload"
    target = {"Linux": "linux", "Windows": "win32", "macOS": "darwin"}[os.environ["RUNNER_OS"]] + "-" + os.environ["RUNNER_ARCH"].lower()
    payload_descriptor, runtime_before, native_started, source_installation = None, None, None, None
    source_prepared, preparation, public_pins = None, None, None
    prepared_records, after_records, source_after_capture, source_after_deadline = {}, {}, None, None
    observer_descriptor, observer_original = None, None
    helpers = runpy.run_path(str(HERE / "hermes-native.py"), run_name="hermes_profile_process_helpers")
    lifecycle_deadline = time.monotonic() + 720
    try:
        proof.update(native_family_timeout_seconds=720, payload_scan_timeout_seconds=120)
        if source_capture:
            source_installation = source_inputs(runner, official_source)
            original_records = {}
            runtime_before = source_runtime_snapshot(source_installation, home_authority, retain=original_records)
            proof["runtime_before"] = runtime_before
            env, source_prepared, preparation = prepare_source_user(source_installation, original_records, evidence, nonce, home_authority, retain=prepared_records)
            runtime_root, python = official_source, source_installation["python"]
            env.update(HERMES_PROFILE_SOURCE_CAPTURE="1", HERMES_HOME=str(source_installation["original"]),
                       HERMES_PROFILE_RUNTIME_ROOT=str(runtime_root), HERMES_PROFILE_PYTHON=str(python))
            proof.update(source_prepared_runtime=source_prepared, source_user_preparation=preparation)
        else:
            build = json.loads(ordinary_bytes(runner / "hermes-profile-build.json"))
            require(set(build) == {"commit", "exit_code", "timeout_seconds", "elapsed_seconds"}
                    and build["commit"] == BASE["PIN"] and build["exit_code"] == 0 and build["timeout_seconds"] == 9000
                    and type(build["elapsed_seconds"]) in (int, float) and 0 < build["elapsed_seconds"] <= 9000, "official payload build receipt differs")
            proof["bootstrap"] = build
            payload_identity, payload_before = directory_identity(payload), payload.lstat()
            require(not any(payload.is_relative_to(root) or root.is_relative_to(payload) for root in (home, evidence, checkout, official_source)), "runtime/configuration roots overlap")
            payload_descriptor = open_inventory_directory(payload)
            payload_held = os.fstat(payload_descriptor)
            check_inventory_directory(payload_before, payload_held, "payload root changed at open")
            verify_inventory_directory(payload, payload_before, payload_descriptor, payload_held)
            runtime_before = {"inventory": payload_snapshot(payload, payload_identity, time.monotonic() + 120),
                              "provenance": payload_contract(payload, official_source, target)}
            selected = runtime_before["provenance"]["selected"]
            runtime_root, python = payload / selected["code"], payload / selected["python"]
            env = BASE["closed_environment"](user, evidence, nonce, "http://127.0.0.1:9/unused", installation={
                "HERMES_BIN": runtime_before["provenance"]["native_command"][0], "RUNNER_TEMP": str(runner),
                "HERMES_PROFILE_RUNTIME_ROOT": str(runtime_root), "HERMES_PROFILE_PYTHON": str(python)})
        proof["runtime_before"] = runtime_before
        if not source_capture:
            require(os.environ.get("EXPECTED_HEAD") == before["commit"], "compiled public source differs")
            public_pins = public_runtime_pins()
            require(all(not Path(pin["path"]).is_relative_to(root) for pin in public_pins.values()
                        for root in (home, evidence, payload, checkout, official_source)), "public executable roots overlap native/config source")
            proof["public_runtimes_before"] = public_pins
        env["HERMES_CENSUS_EVIDENCE_ID"] = json.dumps(evidence_authority)
        env["HERMES_PROFILE_SOURCE_HEAD"] = before["commit"]
        if not source_capture:
            for key in ("UAP_BIN", "UAP_HERMES_COLD_UAP_SHA256", "UAP_HERMES_COLD_TEST_BIN", "UAP_HERMES_COLD_TEST_SHA256",
                        "UAP_HERMES_MCP_PROBE_BIN", "UAP_HERMES_MCP_PROBE_SHA256", "EXPECTED_HEAD", "GITHUB_ACTIONS", "RUNNER_ENVIRONMENT"):
                env[key] = os.environ[key]
            env["HERMES_PUBLIC_PINS"] = json.dumps(public_pins)
        env.pop("HERMES_NATIVE_MCP_URL", None)  # This case has no User/legacy-call endpoint.
        gate = evidence / "owned-start-gate"
        env["HERMES_CALL_START_GATE"] = str(gate)
        sys.path.insert(0, str(runtime_root))
        from hermes_cli._launchers import runtime_command
        require(Path(runtime_command.__code__.co_filename).resolve() == runtime_root / "hermes_cli/_launchers.py", "observer runtime API is not selected-source-owned")
        if source_capture:
            native_prefix = source_native_runtime_command(runtime_command, source_installation, prepared_records, home_authority,
                source_native_prefix(source_installation), args=(), deadline=lifecycle_deadline)
            env["HERMES_SOURCE_NATIVE_PREFIX"] = json.dumps({"argv": native_prefix,
                "readout_sha256": source_installation["command_sha256"],
                "readout_identity": list(stable_file(source_installation["command_info"])),
                "readout_parent": list(source_installation["command_parent"])})
        observer_code = f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name='__main__')"
        if source_capture:
            from pm.environments import venv_command
            argv, proof["source_observer"] = source_observer_runtime_command(
                venv_command, source_installation, source_prepared, prepared_records, home_authority,
                code=observer_code, args=("--child",), deadline=lifecycle_deadline)
        else:
            argv = observer_runtime_command(runtime_command, runtime_root, python, args=["--child"], code=observer_code)
        if payload_descriptor is not None:
            verify_inventory_directory(payload, payload_before, payload_descriptor, payload_held)
        native_started = time.monotonic()
        lifecycle_deadline = native_started + 720
        env["HERMES_PROFILE_DEADLINE"] = str(lifecycle_deadline)
        flags = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
        with (evidence / "lifecycle.stdout").open("x+b") as out, (evidence / "lifecycle.stderr").open("wb") as err:
            observer_descriptor = os.dup(out.fileno())
            observer_original = os.fstat(observer_descriptor)
            child = subprocess.Popen(argv, env=env, stdin=subprocess.DEVNULL, stdout=out, stderr=err, **flags)
            if os.name == "nt":
                job = BASE["WindowsJob"](child)
            gate.write_text("owned\n")
            while child.poll() is None:
                require(time.monotonic() < lifecycle_deadline, "admission deadline")
                require(all((evidence / name).stat().st_size <= LIMIT for name in ("lifecycle.stdout", "lifecycle.stderr")), "admission output bound")
                time.sleep(.1)
        require(child.returncode == 0, "native profile observation failed")
        deltas = json.loads(ordinary_bytes(evidence / "profile-command-deltas.json"))
        methods = deltas["methods"]
        require(methods["a"].count("initialize") == 2 and methods["a"].count("tools/list") == 2
                and methods["b"].count("initialize") == 1 and methods["b"].count("tools/list") == 1
                and all("tools/call" not in value for value in methods.values()), "native profile MCP ledger differs")
        endpoints = json.loads(ordinary_bytes(evidence / "profile-endpoints.json"))
        proof.update(methods=methods, runtime_roots=endpoints, deltas=deltas)
        require(not deltas["unadmitted_shared_effects"], "shared metadata effects require independent admission")
        if not source_capture:
            public = public_json(ordinary_bytes(evidence / "public-command-deltas.json"))
            require(public["complete"] is True, "public Hermes lifecycle incomplete")
            cold = cold_report(public_json(ordinary_bytes(evidence / "cold-restore.json")), public_pins,
                               Path(env["HERMES_BIN"]), before["commit"], home / "public-cold-tmp/hermes-cold")
            proof.update(public_project_lifecycle_complete=True, public_deltas=public,
                         cold_restore={"source": cold["source_head"], "commands": len(cold["commands"]), "sha256": sha(evidence / "cold-restore.json")})
    except BaseException as error:
        failure = error
        if source_capture and native_started is None:
            retain_source_refusal(error, evidence, evidence_authority, nonce, "parent-before-observer")
        if type(error) is PayloadHardlinkRefusal and native_started is None and error.diagnostic is not None:
            proof["payload_hardlink_refusal"] = error.diagnostic
        proof.update(status="failed", error=type(error).__name__ + ": " + str(error)[:300])
    finally:
        process_settled = child is None
        primary_settled, cleanup_phase = child is None, "primary-family"
        try:
            if child is not None:
                if job is not None:
                    job.close()
                    child.wait(timeout=5)
                else:
                    require(os.name != "nt", "Windows child lacks owned job accounting")
                    helpers["stop_lifecycle"](child)
                    deadline = time.monotonic() + 5
                    while True:
                        try:
                            os.killpg(child.pid, 0)
                        except ProcessLookupError:
                            break
                        require(time.monotonic() < deadline, "owned process group still present")
                        time.sleep(.05)
                process_settled = True
                primary_settled = True
            cleanup_phase = "public-family"
            if not source_capture and not public_cleanup_complete(home, proof):
                process_settled = False
                raise RuntimeError("public native cleanup unproved; disposable home retained")
            cleanup_phase = "profile-roots"
            endpoint_file = evidence / "profile-endpoints.json"
            root_paths = {name: home / ("project-" + name) / ".uap" / "hermes-runtime" for name in ("a", "b")}
            if endpoint_file.exists():
                selected = json.loads(ordinary_bytes(endpoint_file))["selected"]
                require(all(selected[name]["root"] == str(root) for name, root in root_paths.items()), "cleanup root receipt differs")
                verify_root_identities(root_paths, {name: selected[name]["directory_identity"] for name in root_paths})
            else:
                require(all(not root.exists() and not root.is_symlink() for root in root_paths.values()), "cleanup lacks original custom-root receipt")
            cleanup_phase = "public-roots"
            public_endpoints = evidence / "public-endpoints.json"
            if not source_capture and public_endpoints.exists():
                endpoints = public_json(ordinary_bytes(public_endpoints))
                public_roots = {name: home / ("public-" + name) for name in ("a", "b")}
                require(set(endpoints) == set(public_roots) and all(endpoints[name]["project"] == str(root) for name, root in public_roots.items()), "public cleanup root receipt differs")
                verify_root_identities(public_roots, {name: endpoints[name]["identity"] for name in public_roots})
                public = public_json(ordinary_bytes(evidence / "public-command-deltas.json"))
                for name, root in public_roots.items():
                    selected = root / ".hermes/uap-session"
                    if selected.exists():
                        selected_identity = public["root_authorities"][name]
                        require(directory_identity(selected) == tuple(selected_identity[0]) and directory_identity(selected.parent) == tuple(selected_identity[1]), "public cleanup selected root changed")
            cleanup_phase = "outer-home"
            require(home.resolve() == home and not home.is_symlink()
                    and (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode) == identity, "owned outer root changed")
            cleanup_phase = "census"
            maybe_census(evidence, home, home_authority, nonce, process_settled, lifecycle_deadline, evidence_authority)
            require(directory_identity(home) == home_authority, "owned outer root changed after census")
            if source_capture:
                proof["cleanup"] = "owned-processes-settled; installed source and measured client-owned state retained"
            else:
                cleanup_phase = "remove-home"
                shutil.rmtree(home)
                proof["cleanup"] = "completed"
        except BaseException as error:
            failure = failure or error
            proof.update(status="failed", cleanup="failed", cleanup_error=type(error).__name__)
        try:
            observer = (read_observer_failure(observer_descriptor, observer_original, nonce, before["commit"], primary_settled=primary_settled)
                        if failure is not None else None)
        finally:
            if observer_descriptor is not None:
                try:
                    os.close(observer_descriptor)
                except OSError:
                    pass  # Optional retained diagnostics never replace native refusal.
        proof["native_family_elapsed_seconds"] = round(time.monotonic() - native_started, 3) if native_started is not None else None
        try:
            if source_prepared is not None and process_settled:
                require(read_source_preparation(evidence, evidence_authority, nonce,
                                               env["HERMES_SOURCE_PREPARATION_SHA256"]) == json.loads(json.dumps(preparation)),
                        "source preparation envelope changed after native settlement")
        except BaseException as error:
            failure = failure or error
            proof.update(status="failed", preparation_error=type(error).__name__ + ": " + str(error)[:300])
        try:
            if public_pins is not None and process_settled:
                proof["public_runtimes_after"] = public_runtime_pins(public_pins)
            if runtime_before is not None and process_settled:
                if source_capture:
                    source_after_deadline = time.monotonic() + 120
                    runtime_after = source_runtime_snapshot(source_inputs(runner, official_source), home_authority,
                                                            retain=after_records, deadline=source_after_deadline)
                    source_after_capture = runtime_after
                    proof["runtime_after"] = runtime_after
                    require(runtime_after["immutable"] == runtime_before["immutable"]
                            and runtime_after["provenance"] == runtime_before["provenance"], "immutable source installation changed")
                    if source_prepared is not None:
                        require_source_user_baseline(runtime_after["user"], source_prepared["user"])
                else:
                    verify_inventory_directory(payload, payload_before, payload_descriptor, payload_held)
                    runtime_after = {"inventory": payload_snapshot(payload, payload_identity, time.monotonic() + 120),
                                     "provenance": payload_contract(payload, official_source, target)}
                    proof["runtime_after"] = runtime_after
                    require(runtime_after == runtime_before, "immutable bundled runtime changed")
        except BaseException as error:
            failure = failure or error
            if source_capture and process_settled:
                retain_source_refusal(error, evidence, evidence_authority, nonce, "parent-after-settlement")
            proof.update(status="failed", runtime_error=type(error).__name__ + ": " + str(error)[:300])
        finally:
            if payload_descriptor is not None:
                os.close(payload_descriptor)
        if source_capture:
            retain_source_user_delta(evidence, evidence_authority, nonce, source_prepared, source_after_capture,
                                     prepared_records, after_records, "user", source_after_deadline, failure=failure, settled=process_settled)
        failure_context = {"source_head": before["commit"], "nonce": nonce, "cleanup_phase": cleanup_phase,
                           "primary_family_settled": primary_settled, "observer_exit": child.returncode if child else None, "observer": observer}
        if not process_settled:
            safe_failure_evidence(runner, type(failure).__name__ if failure else "process-cleanup-unproved", native_failure=failure_context)
            raise RuntimeError("native process cleanup unproved; raw evidence withheld") from failure
        require(evidence.resolve() == evidence and not evidence.is_symlink()
                and (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode) == evidence_identity,
                "evidence directory identity changed")
        admit_evidence(evidence, failure=failure)
        proof.update(source_after=BASE["source_identity"](checkout), hermes_source_after=BASE["source_identity"](official_source),
                     streams={p.name: {"bytes": p.stat().st_size, "sha256": hashlib.sha256(ordinary_bytes(p)).hexdigest()} for p in evidence.iterdir() if p.is_file()})
        if proof["source_after"] != before or proof["hermes_source_after"] != official:
            failure = failure or RuntimeError("source/runtime identity changed")
            proof.update(status="failed", source_error="source/runtime identity changed")
        proof["capture_complete"] = failure is None
        proof.update(lifecycle_exit=child.returncode if child else None,
                     run_id=os.environ["GITHUB_RUN_ID"], run_attempt=os.environ["GITHUB_RUN_ATTEMPT"], runner_os=os.environ["RUNNER_OS"])
        write_json(evidence / "profile-admission.json", proof)
        try:
            stage_evidence(evidence, native_failure=failure_context if failure is not None else None)
        except BaseException:
            if failure is not None:
                raise RuntimeError("original Hermes refusal retained; evidence publication refused") from failure
            raise
    if failure:
        raise RuntimeError("Hermes profile admission failed; inspect " + str(evidence)) from failure
    print("Captured unqualified explicit Hermes profile metadata/MCP evidence: " + str(evidence))


if __name__ == "__main__":
    if sys.argv[1:] == ["--child"]:
        gate, deadline = Path(os.environ["HERMES_CALL_START_GATE"]), time.monotonic() + 10
        while not gate.is_file():
            require(time.monotonic() < deadline, "owned process gate missing")
            time.sleep(.02)
        child_case()
    elif not sys.argv[1:]:
        try:
            run()
        except BaseException as error:
            # Any early failure before run() closes its envelope gets only an
            # inert public report; never upload a partially admitted directory.
            if not EVIDENCE_CLOSED:
                safe_failure_evidence(Path(os.environ["RUNNER_TEMP"]).resolve(), type(error.__cause__ or error).__name__)
            raise
    else:
        raise SystemExit("no arguments, or private --child")
