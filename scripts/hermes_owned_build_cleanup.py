"""Detach only proven aliases in this task's fresh private Hermes build roots.

POSIX unlink is descriptor-relative, not atomic inode-CAS against an arbitrary
same-UID attacker. Authority is the exclusive fresh fixture lifetime after the
official builder returns; any observed drift stops cleanup, retaining the rest.
"""
from contextlib import ExitStack, contextmanager
import os
import json
from pathlib import Path
import runpy
import stat
import subprocess
import sys
import time

API = runpy.run_path(str(Path(__file__).parent / "e2e/hermes-profile-admission.py"),
                     run_name="hermes_owned_cleanup_helpers")
require, stable = API["require"], API["stable_file"]
DOMAIN_MEMBERS, DEPTH, BYTES = API["PAYLOAD_MEMBERS"], API["PAYLOAD_DEPTH"], API["PAYLOAD_BYTES"]
LIMIT, UNION_BYTES = 3 * DOMAIN_MEMBERS, 3 * BYTES


def check_time(deadline):
    require(time.monotonic() < deadline, "owned build cleanup deadline")


class OwnedRoots:
    def __init__(self, roots):
        require(set(roots) == {"payload", "work", "cache"}, "owned build domains differ")
        self.roots, self.held, self.identities = roots, {}, {}
        self.stack = ExitStack()
        try:
            for label, root in roots.items():
                info = root.lstat()
                require(stat.S_ISDIR(info.st_mode) and not root.is_symlink()
                        and (os.name == "nt" or stat.S_IMODE(info.st_mode) == 0o700),
                        "fresh private build root required")
                fd = API["open_inventory_directory"](root)
                self.stack.callback(os.close, fd)
                API["check_inventory_directory"](info, os.fstat(fd), "owned build root changed")
                self.held[label] = fd
                self.identities[label] = API["directory_identity"](root)
        except BaseException:
            self.stack.close()
            raise

    def verify(self):
        for label, root in self.roots.items():
            expected = self.identities[label]
            require(API["directory_identity"](root) == expected, "owned build root replaced")
            held = os.fstat(self.held[label])
            require((held.st_dev, held.st_ino, held.st_mode, held.st_uid, held.st_gid) == expected
                    if os.name != "nt" else API["same_inventory_directory"](root.lstat(), held),
                    "owned build held root changed")

    def close(self):
        self.stack.close()


def census(owned, deadline):
    """Visit the complete disjoint domain union, without reading regular bytes."""
    owned.verify()
    records, total = {}, 0
    counts = {label: {"entries": 0, "regular_bytes": 0} for label in owned.roots}

    def visit(label, path, parent, depth):
        nonlocal total
        check_time(deadline)
        counts[label]["entries"] += 1
        census_bound(counts, label, total, len(records) < LIMIT and depth <= DEPTH
                     and counts[label]["entries"] <= DOMAIN_MEMBERS, "owned build census bound")
        info = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        if depth == 0:
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
                    == owned.identities[label], "owned build original root changed")
        require(info.st_dev == owned.identities[label][0]
                and (info.st_uid, info.st_gid) == owned.identities[label][3:], "owned build member owner/device")
        relative = path.relative_to(owned.roots[label]).as_posix()
        records[label, relative] = info
        if stat.S_ISLNK(info.st_mode) or getattr(info, "st_file_attributes", 0) & 0x400:
            return  # Neither resolve nor traverse links, including junctions.
        if stat.S_ISREG(info.st_mode):
            total += info.st_size
            counts[label]["regular_bytes"] += info.st_size
            census_bound(counts, label, total, info.st_nlink > 0 and info.st_size >= 0
                         and counts[label]["regular_bytes"] <= BYTES
                         and total <= UNION_BYTES, "owned build byte bound")
            return
        require(stat.S_ISDIR(info.st_mode), "owned build nonordinary member")
        fd = API["open_inventory_directory"](path, parent)
        try:
            held = os.fstat(fd)
            API["check_inventory_directory"](info, held, "owned build directory changed")
            with os.scandir(path if os.name == "nt" else fd) as entries:
                for entry in entries:
                    visit(label, path / entry.name, None if os.name == "nt" else fd, depth + 1)
            API["verify_inventory_directory"](path, info, fd, held, parent,
                                              diagnostic=(label, relative))
        finally:
            os.close(fd)

    for label, root in owned.roots.items():
        visit(label, root, None, 0)
    owned.verify()
    print_census(counts, "complete", None, total)
    return records


def print_census(counts, status, role, total):
    print("HERMES_BUILD_CENSUS=" + json.dumps({"schema_version": 1, "status": status,
          "role": role, "domains": counts, "regular_bytes": total,
          "domain_byte_limit": BYTES, "union_byte_limit": UNION_BYTES,
          "domain_member_limit": DOMAIN_MEMBERS}, sort_keys=True), flush=True)


def census_bound(counts, label, total, admitted, reason):
    if not admitted:
        print_census(counts, "refused", label, total)
    require(admitted, reason)


def alias_plan(records):
    groups = {}
    for key, info in records.items():
        if stat.S_ISREG(info.st_mode):
            groups.setdefault((info.st_dev, info.st_ino), []).append((key, info))
    selected = []
    for aliases in groups.values():
        if not any(key[0] == "payload" for key, _ in aliases):
            continue
        first = aliases[0][1]
        require(all(stable(info) == stable(first) for _, info in aliases)
                and first.st_nlink == len(aliases), "owned build unknown external alias")
        selected.extend((key, info) for key, info in aliases if key[0] != "payload")
    return selected


@contextmanager
def held_parent(owned, key, records, deadline):
    label, relative = key
    root, parts = owned.roots[label], Path(relative).parts
    with ExitStack() as stack:
        parent, path = owned.held[label], root
        chain = []
        for part in parts[:-1]:
            check_time(deadline)
            path /= part
            expected = records[label, path.relative_to(root).as_posix()]
            linked = path.lstat() if os.name == "nt" else os.stat(part, dir_fd=parent, follow_symlinks=False)
            require(stable(linked) == stable(expected), "owned build ancestor changed")
            fd = API["open_inventory_directory"](path, None if os.name == "nt" else parent)
            stack.callback(os.close, fd)
            API["check_inventory_directory"](linked, os.fstat(fd), "owned build ancestor changed at open")
            chain.append([path, fd, parent, os.fstat(fd)])
            parent = fd
        def verify():
            check_time(deadline)
            owned.verify()
            for ancestor, fd, parent_fd, held_before in chain:
                expected = records[label, ancestor.relative_to(root).as_posix()]
                linked = ancestor.lstat() if os.name == "nt" else os.stat(ancestor.name, dir_fd=parent_fd, follow_symlinks=False)
                held = os.fstat(fd)
                require(stable(linked) == stable(expected), "owned build ancestor link changed")
                API["check_inventory_directory"](linked, held, "owned build ancestor handle changed")
                require(stable(held) == stable(held_before), "owned build ancestor handle drift")
        def record_parent_change():
            if chain:
                previous, after = chain[-1][3], os.fstat(chain[-1][1])
                fields = ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_nlink")
                require(all(getattr(previous, field) == getattr(after, field) for field in fields),
                        "owned build unlink held parent changed")
                chain[-1][3] = after
        verify.record_parent_change = record_parent_change
        verify()
        yield path, parent, parts[-1], verify
        verify()


def windows_delete(path, expected):
    """Delete the held ordinary leaf itself; never close then DeleteFile(path)."""
    import ctypes
    from ctypes import wintypes
    import msvcrt
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    create = kernel.CreateFileW
    create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p,
                       wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    create.restype = wintypes.HANDLE
    handle = create(str(path), 0x80010000, 3, None, 3, 0x00200000, None)
    if handle is None or handle == ctypes.c_void_p(-1).value:
        raise ctypes.WinError(ctypes.get_last_error())
    fd = None
    try:
        fd = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
        held = os.fstat(fd)
        require(API["same_opened_capture"](expected, held)
                and stable(path.lstat()) == stable(expected), "owned build delete leaf changed")
        class Disposition(ctypes.Structure):
            _fields_ = [("DeleteFile", ctypes.c_ubyte)]  # FILE_DISPOSITION_INFO: BOOLEAN.
        disposition = Disposition(1)
        apply = kernel.SetFileInformationByHandle
        apply.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD]
        apply.restype = wintypes.BOOL
        if not apply(handle, 4, ctypes.byref(disposition), ctypes.sizeof(disposition)):
            raise ctypes.WinError(ctypes.get_last_error())
    finally:
        if fd is not None:
            os.close(fd)
        else:
            close = kernel.CloseHandle
            close.argtypes, close.restype = [wintypes.HANDLE], wintypes.BOOL
            if not close(handle):
                raise ctypes.WinError(ctypes.get_last_error())


def detach(owned, deadline):
    records = census(owned, deadline)
    selected = alias_plan(records)  # Whole union admitted before the first unlink.
    expected = {(info.st_dev, info.st_ino): info for _, info in selected}
    anchors = {(info.st_dev, info.st_ino): key for key, info in records.items()
               if key[0] == "payload" and stat.S_ISREG(info.st_mode)}
    for key, original in selected:
        check_time(deadline)
        inode = original.st_dev, original.st_ino
        previous = expected[inode]
        with held_parent(owned, key, records, deadline) as (path, parent, name, verify):
            linked = (path / name).lstat() if os.name == "nt" else os.stat(name, dir_fd=parent, follow_symlinks=False)
            require(stable(linked) == stable(previous), "owned build delete member changed")
            if os.name == "nt":
                verify()
                windows_delete(path / name, linked)
            else:
                fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
                try:
                    require(stable(os.fstat(fd)) == stable(linked), "owned build delete open changed")
                    verify()
                    os.unlink(name, dir_fd=parent)
                    require(os.fstat(fd).st_nlink == linked.st_nlink - 1, "owned build unlink count changed")
                finally:
                    os.close(fd)
            # Only this successfully unlinked leaf's immediate parent changes.
            # Track that explicit own effect; all other captured parents stay exact.
            updated = path.lstat() if os.name == "nt" else os.fstat(parent)
            before = records[key[0], Path(key[1]).parent.as_posix()]
            require((updated.st_dev, updated.st_ino, updated.st_mode, updated.st_uid, updated.st_gid)
                    == (before.st_dev, before.st_ino, before.st_mode, before.st_uid, before.st_gid),
                    "owned build unlink parent changed")
            records[key[0], Path(key[1]).parent.as_posix()] = updated
            verify.record_parent_change()
        with held_parent(owned, anchors[inode], records, deadline) as (path, parent, name, verify):
            after = (path / name).lstat() if os.name == "nt" else os.stat(name, dir_fd=parent, follow_symlinks=False)
            fields = ("st_dev", "st_ino", "st_mode", "st_uid", "st_gid", "st_size", "st_mtime_ns")
            require(all(getattr(after, field) == getattr(previous, field) for field in fields)
                    and after.st_nlink == previous.st_nlink - 1, "owned build payload changed during unlink")
            expected[inode] = after  # Exact own nlink decrement/ctime transition only.
    final = census(owned, deadline)
    require(not alias_plan(final), "owned build aliases remain")
    API["payload_snapshot"](owned.roots["payload"], owned.identities["payload"], deadline)
    return len(selected)


def build(source, runner, pin):
    roots = {label: runner / ("hermes-profile-" + leaf)
             for label, leaf in (("work", "work"), ("cache", "cache"), ("payload", "payload"))}
    for root in roots.values():
        root.mkdir(mode=0o700)  # Existing names refuse; never adopt another cache.
    owned = OwnedRoots(roots)
    try:
        owned.verify()
        with (roots["work"] / ".native-preparation").open("x", encoding="utf-8") as marker:
            marker.write(str(source))
        owned.verify()
        subprocess.run([sys.executable, "-S", "-B", str(source / "scripts/bundles/native_build.py"),
                        "--source", str(source), "--work", str(roots["work"]),
                        "--cache", str(roots["cache"]), "--out", str(roots["payload"]),
                        "--commit", pin], check=True, timeout=9000)
        return detach(owned, time.monotonic() + 120)
    finally:
        owned.close()  # Keep all remaining build state on every outcome.
