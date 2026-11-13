#!/usr/bin/env python3
"""Observe pinned Hermes explicit custom-runtime-root MCP; UAP remains refused."""
import hashlib
from contextlib import contextmanager
import json
import os
from pathlib import Path
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


def ordinary_bytes(path, expected=None, *, parent=None):
    path = Path(path)
    require(parent is not None or (path.is_absolute() and path.resolve() == path), "ordinary file alias")
    def linked():
        return path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
    before = linked()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and 0 <= before.st_size <= LIMIT,
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
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = stream.read(LIMIT + 1)
        require(len(body) == before.st_size <= LIMIT and stable_file(os.fstat(descriptor)) == stable_file(held)
                and stable_file(linked()) == stable_file(before), "file changed during bounded capture")
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
    for name, root in roots.items():
        require(directory_identity(root) == tuple(held[name]), "original custom root directory changed")


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

    Windows path lstat and held directory fstat can synthesize different
    mode and attribute flags for the same file ID. Both domains independently
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


def verify_inventory_directory(path, before, descriptor, held, parent=None):
    linked = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
    require((parent is not None or path.resolve() == path) and stable_file(linked) == stable_file(before)
            and stable_file(os.fstat(descriptor)) == stable_file(held), "inventory directory changed")


def inventory(root, sentinels=()):
    """Capture an ordinary tree, refusing after the bounded number of visits.

    os.walk materializes the whole child-name list *before* the membership
    limit is checked. A lazy scandir walk keeps that limit effective even for
    a directory with an unexpectedly large number of entries.
    """
    root = Path(root)
    identity = directory_identity(root)
    result, total = {}, 0

    def visit(path, depth, parent=None):
        nonlocal total
        require(len(result) < 2048 and depth <= 24, "inventory membership bound")
        relative = path.relative_to(root).as_posix()
        info = path.lstat() if parent is None else os.stat(path.name, dir_fd=parent, follow_symlinks=False)
        if depth == 0:
            require((info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid) == identity,
                    "inventory original root changed before open")
        inventory_member(relative, info)
        record = {"type": "directory" if stat.S_ISDIR(info.st_mode) else "file",
                  "mode": stat.S_IMODE(info.st_mode), "device": info.st_dev, "inode": info.st_ino}
        if record["type"] == "file":
            body = ordinary_bytes(path, info, **({} if parent is None else {"parent": parent}))
            total += len(body)
            require(total <= 16 << 20, "inventory byte bound")
            require(not any(marker.encode() in body for marker in sentinels),
                    "original User settings/data copied into custom root")
            record.update(bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
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
        return
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
print('UAP_PROFILE_RESOLUTION=' + json.dumps(value, sort_keys=True))
"""


def no_user_copies(root, sentinels):
    return inventory(root, sentinels)


def config_document_boundary(body, value, server, url, removed=False, authored_sha=None):
    """Check the exact mapping and bytes independently of the native YAML parser."""
    require(isinstance(value, dict) and set(value) <= {"mcp_servers"}, "unadmitted custom config fields")
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


def observe_profiles(probes):
    evidence = Path(os.environ["HERMES_NATIVE_EVIDENCE"])
    user = Path(os.environ["HOME"])
    outer, nonce = user.parent, os.environ["HERMES_CALL_NONCE"]
    native = json.loads(ordinary_bytes(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])).decode("utf-8-sig"))
    from hermes_constants import get_default_hermes_root
    import hermes_yaml as yaml
    original = get_default_hermes_root(home="").resolve()
    require(original.is_relative_to(user) and original != user and os.environ["USERPROFILE"] == str(user), "actual native User root differs")
    original.mkdir(parents=True, exist_ok=True)
    sentinels = ["USER_" + kind + "_" + nonce for kind in ("MODEL", "PROVIDER", "AUTH", "HISTORY", "FOREIGN")]
    authored = {"config.yaml": "model: " + sentinels[0] + "\nproviders:\n  custom:\n    base_url: http://127.0.0.1:9/" + sentinels[1] + "\nmcp_servers:\n  foreign:\n    command: echo\n    args: [retained]\n",
                "auth.json": json.dumps({"version": 1, "providers": {}, "admission_marker": sentinels[2]}),
                ".env": "# inert admission; no credentials\n",
                "sessions/retained-history.txt": sentinels[3],
                "skills/foreign/SKILL.md": "---\nname: foreign\ndescription: Retained foreign User Skill.\n---\n" + sentinels[4]}
    for name, body in authored.items():
        path = original / name
        require(not path.exists(), "original User fixture collision")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")
        path.chmod(0o600)
    spec, records, shared = {}, [], []
    for scope in ("a", "b"):
        project = outer / ("project-" + scope)
        project.mkdir()
        (project / "foreign.txt").write_text(nonce + scope)
        bounded(["git", "init", "-q", str(project)], outer, dict(os.environ), evidence / ("git-" + scope), timeout=30)
        root = project / ".uap" / "hermes-runtime"
        root.mkdir(parents=True)
        entry = {"url": probes[scope].url}
        config = root / "config.yaml"
        config.write_text(yaml.safe_dump({"mcp_servers": {"uap-root-" + scope: entry}}), encoding="utf-8")
        config.chmod(0o600)
        spec[scope] = {"root": str(root), "project": str(project), "server": "uap-root-" + scope,
                       "url": probes[scope].url, "config_sha256": sha(config), "entry": entry}
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
               "os_home": str(user), "sentinel_sha256": [hashlib.sha256(x.encode()).hexdigest() for x in sentinels]})
    try:
        baseline = inventory(outer)
    except (OSError, RuntimeError, ValueError):
        publish_census_request(evidence, outer, nonce, json.loads(os.environ["HERMES_CENSUS_EVIDENCE_ID"]))
        raise
    protected = [str((original / name).relative_to(outer)).replace('\\', '/') for name in authored]
    controls = [str((outer / ("project-" + scope) / "foreign.txt").relative_to(outer)).replace('\\', '/') for scope in ("a", "b")]
    protected += controls
    write_json(evidence / "original-user-baseline.json", baseline)
    current = baseline

    def command(label, scope, args=None, guard=False, expected_failure=False):
        nonlocal current
        verify_root_identities(root_paths, held_roots)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        before = inventory(outer)
        require(before == current, "unmeasured inter-command mutation")
        selected = spec[scope]
        env = {**os.environ, "HERMES_HOME": selected["root"]}
        require(env["HOME"] == str(user) and env["USERPROFILE"] == str(user), "OS User home changed")
        if guard:
            from hermes_cli._launchers import runtime_command
            argv = runtime_command(Path(os.environ["RUNNER_TEMP"]) / "hermes-agent", python=native[0], code=GUARD_CODE)
        else:
            argv = [*native, *args]
        methods_before = {name: list(probe.methods()) for name, probe in probes.items()}
        failure, output = None, b""
        try:
            output = bounded(argv, Path(selected["project"]), env, evidence / label, timeout=60, expected_failure=expected_failure)
        except BaseException as error:
            failure = error
        verify_root_identities(root_paths, held_roots)
        after = inventory(outer)
        delta = changes(before, after)
        records.append({"label": label, "scope": scope, "runtime_root": selected["root"],
                        "delta": delta, "error": type(failure).__name__ if failure else None})
        current = after
        if label == "remove-mcp-" + scope and failure is None:
            removed.add(scope)
        for name, root in root_paths.items():
            config_boundary(root, spec[name]["server"], spec[name]["url"], name in removed,
                            None if name in removed else spec[name]["config_sha256"])
        shared.extend({"label": label, "path": path, **change} for path, change in delta.items()
                      if not any(path == str(root.relative_to(outer)).replace('\\', '/')
                                 or path.startswith(str(root.relative_to(outer)).replace('\\', '/') + "/") for root in roots))
        protected_unchanged(baseline, after, protected)
        for other in (name for name in ("a", "b") if name != scope):
            prefix = str(Path(spec[other]["root"]).relative_to(outer)).replace('\\', '/')
            require({key: value for key, value in before.items() if key == prefix or key.startswith(prefix + "/")}
                    == {key: value for key, value in after.items() if key == prefix or key.startswith(prefix + "/")},
                    "sibling custom root changed")
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
            records[-1]["resolution"] = receipt
        return output

    try:
        for scope in ("a", "b"):
            command("guard-" + scope, scope, guard=True)
            output = command("list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() in output, "selected native MCP name absent")
            command("test-" + scope, scope, ["mcp", "test", spec[scope]["server"]])
        command("test-a-restart", "a", ["mcp", "test", spec["a"]["server"]])
        output = command("list-user", "user", ["mcp", "list"])
        require(b"foreign" in output and all(spec[x]["server"].encode() not in output for x in ("a", "b")), "original User MCP selection changed")
        for scope in ("a", "b"):
            command("remove-mcp-" + scope, scope, ["mcp", "remove", spec[scope]["server"]])
            output = command("removed-list-" + scope, scope, ["mcp", "list"])
            require(spec[scope]["server"].encode() not in output, "removed MCP still listed")
            command("removed-test-" + scope, scope, ["mcp", "test", spec[scope]["server"]], expected_failure=True)
        write_json(evidence / "original-user-final.json", inventory(outer))
    finally:
        write_json(evidence / "profile-command-deltas.json", {"commands": records, "unadmitted_shared_effects": shared,
                   "production_project_mcp": False, "admission_qualified": False,
                   "methods": {name: list(probe.methods()) for name, probe in probes.items()}})


COMMANDS = ({prefix + x for prefix in ("git-", "guard-", "list-", "test-", "remove-mcp-", "removed-list-", "removed-test-") for x in ("a", "b")}
            | {"test-a-restart", "list-user"})
MEMBERS = ({name + suffix for name in COMMANDS for suffix in (".stdout", ".stderr", ".status.json")}
           | {"original-user-baseline.json", "original-user-final.json", "source-guards.json", "profile-endpoints.json",
              "profile-command-deltas.json", "bootstrap-census-request.json", "bootstrap-census.json", "profile-admission.json", "lifecycle.stdout", "lifecycle.stderr", "owned-start-gate"})


def admit_evidence(directory):
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


def stage_evidence(directory):
    global EVIDENCE_CLOSED
    identity = directory_identity(directory)
    admit_evidence(directory)
    sources = {path.name: path.lstat() for path in directory.iterdir()}
    upload = directory / "upload"
    require(not upload.exists() and not upload.is_symlink(), "upload destination already exists")
    pending = directory / (".upload-pending-" + secrets.token_hex(16))
    pending.mkdir(mode=0o700)
    pending_identity = directory_identity(pending)
    try:
        for name, expected in sorted(sources.items()):
            body = ordinary_bytes(directory / name, expected)
            if name in ("bootstrap-census-request.json", "bootstrap-census.json"):
                validate_census_member(name, body)
            with (pending / name).open("xb") as stream:
                stream.write(body)
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


def safe_failure_evidence(runner, failure_kind="unclosed-native-admission"):
    directory = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-evidence-", dir=runner))
    write_json(directory / "profile-admission.json", {"status": "failed", "error": "native admission aborted before evidence closure", "original_failure_kind": failure_kind,
               "admission_qualified": False, "production_project_mcp": False, "vendor_authentication": False,
               "real_model_inference": False, "tools_call": False})
    stage_evidence(directory)
    return directory


def child_case():
    from contextlib import ExitStack
    from pi_project_components_native_e2e import PiHTTPMCPProbe
    with ExitStack() as stack:
        probes = {name: stack.enter_context(PiHTTPMCPProbe(nonce=os.environ["HERMES_CALL_NONCE"]))
                  for name in ("a", "b")}
        observe_profiles(probes)


def run():
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "genuine GitHub-hosted admission required")
    runner, checkout = Path(os.environ["RUNNER_TEMP"]).resolve(), HERE.parents[1]
    before, official = BASE["source_identity"](checkout), BASE["source_identity"](runner / "hermes-agent")
    require(before["commit"] == os.environ["HERMES_NATIVE_UAP_HEAD"] and not before["dirty"], "exact clean UAP source required")
    require(official["commit"] == BASE["PIN"] and not official["dirty"], "unmodified pinned Hermes required")
    evidence = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-evidence-", dir=runner))
    home = Path(tempfile.mkdtemp(prefix="hermes-profile-admission-home-", dir=runner))
    home_authority = directory_identity(home)
    identity = (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode)
    evidence_identity = (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode)
    evidence_authority = directory_identity(evidence)
    user = home / "user"
    user.mkdir(mode=0o700)
    nonce, child, job, failure = secrets.token_hex(32), None, None, None
    proof = {"source": before, "hermes_source": official, "nonce": nonce, "capture_complete": False,
             "admission_qualified": False, "production_project_mcp": False,
             "vendor_authentication": False, "real_model_inference": False, "tools_call": False}
    runtime_file = Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])
    native = json.loads(ordinary_bytes(runtime_file).decode("utf-8-sig"))
    held = {"runtime": sha(Path(native[0])), "runtime_command": sha(runtime_file), "launcher": sha(Path(os.environ["HERMES_BIN"]))}
    helpers = runpy.run_path(str(HERE / "hermes-native.py"), run_name="hermes_profile_process_helpers")
    lifecycle_deadline = time.monotonic() + 720
    try:
        env = BASE["closed_environment"](user, evidence, nonce, "http://127.0.0.1:9/unused")
        env["HERMES_CENSUS_EVIDENCE_ID"] = json.dumps(evidence_authority)
        env.pop("HERMES_NATIVE_MCP_URL", None)  # This case has no User/legacy-call endpoint.
        gate = evidence / "owned-start-gate"
        env["HERMES_CALL_START_GATE"] = str(gate)
        sys.path.insert(0, str(runner / "hermes-agent"))
        from hermes_cli._launchers import runtime_command
        argv = runtime_command(runner / "hermes-agent", python=native[0], args=["--child"],
                               code=f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name='__main__')")
        flags = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
        with (evidence / "lifecycle.stdout").open("wb") as out, (evidence / "lifecycle.stderr").open("wb") as err:
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
        proof.update(capture_complete=True, methods=methods, runtime_roots=endpoints, deltas=deltas)
        require(not deltas["unadmitted_shared_effects"], "shared metadata effects require independent admission")
    except BaseException as error:
        failure = error
        proof.update(status="failed", error=type(error).__name__ + ": " + str(error)[:300])
    finally:
        process_settled = child is None
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
            endpoint_file = evidence / "profile-endpoints.json"
            root_paths = {name: home / ("project-" + name) / ".uap" / "hermes-runtime" for name in ("a", "b")}
            if endpoint_file.exists():
                selected = json.loads(ordinary_bytes(endpoint_file))["selected"]
                require(all(selected[name]["root"] == str(root) for name, root in root_paths.items()), "cleanup root receipt differs")
                verify_root_identities(root_paths, {name: selected[name]["directory_identity"] for name in root_paths})
            else:
                require(all(not root.exists() and not root.is_symlink() for root in root_paths.values()), "cleanup lacks original custom-root receipt")
            require(home.resolve() == home and not home.is_symlink()
                    and (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode) == identity, "owned outer root changed")
            maybe_census(evidence, home, home_authority, nonce, process_settled, lifecycle_deadline, evidence_authority)
            require(directory_identity(home) == home_authority, "owned outer root changed after census")
            shutil.rmtree(home)
            proof["cleanup"] = "completed"
        except BaseException as error:
            failure = failure or error
            proof.update(status="failed", cleanup="failed", cleanup_error=type(error).__name__)
        if not process_settled:
            safe_failure_evidence(runner, type(failure).__name__ if failure else "process-cleanup-unproved")
            raise RuntimeError("native process cleanup unproved; raw evidence withheld") from failure
        require(evidence.resolve() == evidence and not evidence.is_symlink()
                and (evidence.stat().st_dev, evidence.stat().st_ino, evidence.stat().st_mode) == evidence_identity,
                "evidence directory identity changed")
        admit_evidence(evidence)
        proof.update(source_after=BASE["source_identity"](checkout), hermes_source_after=BASE["source_identity"](runner / "hermes-agent"),
                     streams={p.name: {"bytes": p.stat().st_size, "sha256": hashlib.sha256(ordinary_bytes(p)).hexdigest()} for p in evidence.iterdir() if p.is_file()})
        final = {"runtime": sha(Path(native[0])), "runtime_command": sha(runtime_file), "launcher": sha(Path(os.environ["HERMES_BIN"]))}
        if proof["source_after"] != before or proof["hermes_source_after"] != official or final != held:
            failure = failure or RuntimeError("source/runtime identity changed")
            proof.update(status="failed", source_error="source/runtime identity changed")
        proof.update(runtime_before=held, runtime_after=final, lifecycle_exit=child.returncode if child else None,
                     run_id=os.environ["GITHUB_RUN_ID"], run_attempt=os.environ["GITHUB_RUN_ATTEMPT"], runner_os=os.environ["RUNNER_OS"])
        write_json(evidence / "profile-admission.json", proof)
        try:
            stage_evidence(evidence)
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
