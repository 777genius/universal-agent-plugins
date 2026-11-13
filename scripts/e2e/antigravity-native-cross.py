#!/usr/bin/env python3
"""Real agy 1.5.3 + UAP native lifecycle on Windows and macOS.

No browser/model authentication, no foreign profile mutation, no IDE activation claim.
All fixtures, binaries and state live below RUNNER_TEMP.
"""
import hashlib
import math
import base64
import stat
import time
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tempfile
import threading


NAME = "uap-agy-native-smoke"
REMOTE_URL = "https://example.invalid/uap-native-smoke"

# Exact members of the official 0.1.3 archives pinned by securityscan/release.go.
# Only this immutable UAP runtime leaf has a separate bound from profile data.
LINTAI_PINS = {
    "linux/amd64": (9989584, "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba"),
    "darwin/arm64": (7949216, "37342414aa7feadc09b8473d1bfafe95fd79ab99f284d37518c930fcb7986c57"),
    "darwin/amd64": (8417744, "6899e59bab426565fa2433b798a0b6e8accb114386b5d0def30a8a071be8de13"),
    "windows/amd64": (7620608, "9d486e5b66875384c12a147b8a19007beebe2691bc6d23a7867bf019b70be453"),
}


def lintai_member():
    system = {"linux": "linux", "darwin": "darwin", "win32": "windows"}.get(sys.platform)
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    target = f"{system}/{arch}"
    if target not in LINTAI_PINS: raise AssertionError("Unqualified scanner platform")
    size, digest = LINTAI_PINS[target]
    filename = "lintai.exe" if system == "windows" else "lintai"
    return f"uap-state/security/lintai/0.1.3/{system}-{arch}/{filename}", size, digest


def run(binary, *args, env, cwd):
    result = subprocess.run(
        [str(binary), *map(str, args)],
        cwd=str(cwd),
        env=env,
        text=True,
        encoding="utf-8",
        errors="replace",
        capture_output=True,
        timeout=240,
        check=False,
    )
    if result.returncode:
        raise RuntimeError(
            f"{Path(binary).name} {args[0] if args else ''}: exit {result.returncode}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result.stdout


def profile(base, executable_dir):
    home = base / "home"
    env = dict(os.environ)
    env.update(
        HOME=str(home),
        USERPROFILE=str(home),
        APPDATA=str(home / "AppData" / "Roaming"),
        LOCALAPPDATA=str(home / "AppData" / "Local"),
        XDG_CONFIG_HOME=str(base / "config"),
        XDG_CACHE_HOME=str(base / "cache"),
        PATH=str(executable_dir) + os.pathsep + os.environ.get("PATH", ""),
    )
    for key in ("HOME", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME"):
        Path(env[key]).mkdir(parents=True, exist_ok=True)
    return env


def checked_list(cli, env, project):
    rows = json.loads(run(cli, "plugin", "list", "--json", env=env, cwd=project))
    if not isinstance(rows, list):
        raise AssertionError("agy plugin list did not return its documented array")
    return [item.get("name") for item in rows if isinstance(item, dict)]


def checked_plugin(cli, env, project, path, manifest, enabled=None):
    """Observe the exact physical artifact, never a same-name registration."""
    rows = json.loads(run(cli, "plugin", "list", "--json", env=env, cwd=project))
    if not isinstance(rows, list):
        raise AssertionError("Native inventory is not an array")
    matches = [row for row in rows if isinstance(row, dict)
               and (row.get("path") == str(path) or row.get("name") == path.name)]
    if len(matches) != 1:
        raise AssertionError("Native inventory omitted or duplicated the exact owned path")
    row = matches[0]
    if (row.get("path") != str(path) or row.get("name") != path.name
            or row.get("version") != manifest["version"]):
        raise AssertionError("Native inventory changed physical identity or version")
    if not isinstance(row.get("enabled"), bool):
        raise AssertionError("Native inventory omitted boolean enablement")
    if enabled is not None and row["enabled"] is not enabled:
        raise AssertionError("Native enablement readback differs from requested state")
    if row.get("skills") != ["native-smoke"] or sorted(row.get("mcpServers", [])) != ["native-http", "native-stdio"]:
        raise AssertionError("Native inventory did not discover the selected components")
    return row


def tree_snapshot(root):
    """Record every fixture entry, including empty directories and file bytes."""
    result = {}
    for entry in sorted(root.rglob("*")):
        if entry.is_symlink():
            raise AssertionError("Unexpected symlink in the disposable native fixture")
        result[entry.relative_to(root).as_posix()] = (
            "directory" if entry.is_dir() else hashlib.sha256(entry.read_bytes()).hexdigest()
        )
    return result


def assert_native_mcp(plugin, fixture_root):
    config = json.loads((plugin / "mcp_config.json").read_text(encoding="utf-8"))
    servers = config.get("mcpServers", {})
    if set(servers) != {"native-stdio", "native-http"}:
        raise AssertionError("Native MCP projection lost or added a selected server")
    stdio = servers["native-stdio"]
    if stdio.get("command") != "node" or "type" in stdio or "url" in stdio or "serverUrl" in stdio:
        raise AssertionError("Native stdio transport was not projected unambiguously")
    if Path(stdio["cwd"]).resolve() != plugin.resolve():
        raise AssertionError("Native stdio working directory is not the active plugin")
    if {"PLUGIN_ROOT", "PLUGIN_DATA"}.intersection(stdio["env"]):
        raise AssertionError("Native config overrides vendor-managed process environment")
    data = Path(stdio["env"]["UAP_SMOKE_DATA"])
    if not data.is_absolute() or not data.is_dir() or fixture_root.resolve() not in data.resolve().parents:
        raise AssertionError("Native stdio data directory is not an owned disposable directory")
    if (plugin / "mcp.json").exists():
        raise AssertionError("Ignored portable MCP manifest remained in native package")
    args = stdio.get("args", [])
    if (len(args) != 3 or Path(args[0]).resolve() != (plugin / "scripts" / "mcp-smoke.cjs").resolve()
            or args[1] != "--data-dir" or Path(args[2]).resolve() != data.resolve()):
        raise AssertionError("Native stdio arguments do not reference committed plugin/data paths")
    remote = servers["native-http"]
    if remote != {"serverUrl": REMOTE_URL, "headers": {"X-UAP-Smoke": "native-e2e"}}:
        raise AssertionError("Native Streamable HTTP URL or headers were not preserved")
    if "${PLUGIN_" in json.dumps(config):
        raise AssertionError("A client-managed placeholder remained in native MCP configuration")


def require_windows_token_owner(owner, token, security):
    """Compare to this token's exact default object owner, not TokenUser/groups."""
    import ctypes
    from ctypes import wintypes as w
    needed = w.DWORD()
    security.GetTokenInformation(token, 4, None, 0, ctypes.byref(needed))
    if not ctypes.sizeof(ctypes.c_void_p) <= needed.value <= 65536:
        raise AssertionError("Windows default-owner token bound")
    allocated = needed.value
    body = ctypes.create_string_buffer(allocated)
    if not security.GetTokenInformation(token, 4, body, allocated, ctypes.byref(needed)):
        raise AssertionError("Windows default-owner token unavailable")
    if not ctypes.sizeof(ctypes.c_void_p) <= needed.value <= allocated:
        raise AssertionError("Windows default-owner token changed size")
    selected = ctypes.cast(body, ctypes.POINTER(ctypes.c_void_p))[0]
    if not owner or not selected or not security.EqualSid(owner, selected):
        raise AssertionError("Windows owner differs from task token default owner")


def owned_handle(path, directory=False, parent=None):
    """Open the owned fixture without following aliases; Windows holds directories."""
    if os.name != "nt":
        return os.open(path.name if parent is not None else path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | (os.O_DIRECTORY if directory else 0), dir_fd=parent)
    import ctypes
    from ctypes import wintypes as w
    import msvcrt
    kernel, security = ctypes.WinDLL("kernel32", use_last_error=True), ctypes.WinDLL("advapi32", use_last_error=True)
    kernel.CreateFileW.argtypes = [w.LPCWSTR, w.DWORD, w.DWORD, ctypes.c_void_p, w.DWORD, w.DWORD, w.HANDLE]
    kernel.CreateFileW.restype = w.HANDLE
    kernel.CloseHandle.argtypes = [w.HANDLE]
    kernel.CloseHandle.restype = w.BOOL
    # READ_CONTROL + GENERIC_READ; held directory denies deletion/replacement.
    handle = kernel.CreateFileW(str(path), 0x80020000, 3 if directory else 7, None, 3, 0x02200000, None)
    if handle in (None, ctypes.c_void_p(-1).value):
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        security.GetSecurityInfo.argtypes = [w.HANDLE, w.DWORD, w.DWORD, ctypes.POINTER(ctypes.c_void_p), ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.POINTER(ctypes.c_void_p)]
        security.GetSecurityInfo.restype = w.DWORD
        owner, descriptor, token = ctypes.c_void_p(), ctypes.c_void_p(), w.HANDLE()
        security.OpenProcessToken.argtypes = [w.HANDLE, w.DWORD, ctypes.POINTER(w.HANDLE)]
        security.GetTokenInformation.argtypes = [w.HANDLE, w.DWORD, ctypes.c_void_p, w.DWORD, ctypes.POINTER(w.DWORD)]
        security.EqualSid.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
        kernel.LocalFree.argtypes = [ctypes.c_void_p]
        if security.GetSecurityInfo(handle, 1, 1, ctypes.byref(owner), None, None, None, ctypes.byref(descriptor)):
            raise AssertionError("Owned Windows security identity unavailable")
        try:
            if not security.OpenProcessToken(w.HANDLE(-1), 8, ctypes.byref(token)):
                raise ctypes.WinError(ctypes.get_last_error())
            try:
                require_windows_token_owner(owner, token, security)
            finally:
                if not kernel.CloseHandle(token): raise ctypes.WinError(ctypes.get_last_error())
        finally:
            if kernel.LocalFree(descriptor): raise AssertionError("Windows security release failed")
        fd = msvcrt.open_osfhandle(handle, os.O_RDONLY | os.O_BINARY)
    except BaseException:
        kernel.CloseHandle(handle)
        raise
    return fd


def object_identity(info, directory=False):
    if stat.S_ISLNK(info.st_mode) or not (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)):
        raise AssertionError("Owned ordinary object required")
    if os.name == "nt":
        if not info.st_ino or not info.st_dev or getattr(info, "st_reparse_tag", None) != 0 or getattr(info, "st_file_attributes", 0x400) & 0x400:
            raise AssertionError("Windows owned identity unavailable")
    elif (info.st_uid, info.st_gid) != (os.getuid(), os.getgid()):
        raise AssertionError("Owned UID/GID differs")
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)


def stable_file(info):
    # Reading may legitimately advance atime; it grants no mutation authority.
    return (*object_identity(info), info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns,
            getattr(info, "st_birthtime_ns", None), getattr(info, "st_file_attributes", None), getattr(info, "st_reparse_tag", None))


def profile_file_refusal(root, child, info, root_device):
    """Bounded metadata for one refused owned-fixture entry; never open it."""
    relative = child.relative_to(root).as_posix().encode("utf-8")
    values = {"bytes": info.st_size, "links": info.st_nlink, "mode": info.st_mode}
    if not all(type(value) is int and value >= 0 for value in values.values()):
        raise AssertionError("Profile file bound/alias/device: metadata unavailable")
    cap = (1 << 53) - 1
    facts = {"type": "antigravity-profile-file-refusal",
             "reason": "links" if info.st_nlink != 1 else "size" if info.st_size > 1 << 20 else "device",
             "member": relative[:128].decode("utf-8", "ignore"),
             "member_sha256": hashlib.sha256(relative).hexdigest(),
             "member_truncated": len(relative) > 128,
             "same_device": info.st_dev == root_device,
             "overflow": any(value > cap for value in values.values())}
    facts.update({key: min(value, cap) for key, value in values.items()})
    raise AssertionError("Profile file bound/alias/device: " + json.dumps(facts, ensure_ascii=True, sort_keys=True))


class ProfileLedger:
    def __init__(self, root, *, lintai=False):
        self.root, self.held = root, {}
        self.scanner = lintai_member() if lintai else None
        try:
            self.base = self.scan()
        except BaseException:
            self.close()
            raise

    def directory(self, path, parent=None):
        linked = object_identity(path.lstat(), True)
        if path not in self.held:
            fd = owned_handle(path, True, parent)
            opened = object_identity(os.fstat(fd), True)
            if opened[:2] != linked[:2]: os.close(fd); raise AssertionError("Opened directory differs")
            self.held[path] = (fd, linked, opened)
        fd, original, opened = self.held[path]
        if path != self.root and linked[0] != self.held[self.root][1][0]:
            raise AssertionError("Directory crossed original volume")
        if linked != original or object_identity(os.fstat(fd), True) != opened:
            raise AssertionError("Original directory replaced")
        return linked

    def scan(self, *, strict=False):
        result, aggregate = {}, 0
        def visit(path, depth, parent=None):
            nonlocal aggregate
            if depth > 24 or len(result) >= 4096: raise AssertionError("Profile membership bound")
            identity = self.directory(path, parent)
            result[path.relative_to(self.root).as_posix()] = ("directory", identity, self.directory_metadata(path)) if strict else ("directory", identity)
            descriptor = self.held[path][0]
            with os.scandir(path if os.name == "nt" else descriptor) as entries:
                for entry in entries:
                    if len(result) >= 4096: raise AssertionError("Profile membership bound")
                    child = path / entry.name
                    info = child.lstat() if os.name == "nt" else os.stat(entry.name, dir_fd=descriptor, follow_symlinks=False)
                    if stat.S_ISDIR(info.st_mode): visit(child, depth + 1, None if os.name == "nt" else descriptor); continue
                    identity = object_identity(info)
                    relative = child.relative_to(self.root).as_posix()
                    scanner = self.scanner is not None and relative == self.scanner[0]
                    limit = self.scanner[1] if scanner else 1 << 20
                    if info.st_nlink != 1 or info.st_size > limit or info.st_dev != identity_root[0]:
                        profile_file_refusal(self.root, child, info, identity_root[0])
                    if scanner and info.st_size != limit: raise AssertionError("Pinned scanner length differs")
                    if scanner and os.name != "nt" and stat.S_IMODE(info.st_mode) != 0o700:
                        raise AssertionError("Pinned scanner mode differs")
                    fd = owned_handle(child, parent=None if os.name == "nt" else descriptor)
                    try:
                        opened = os.fstat(fd)
                        if object_identity(opened)[:2] != identity[:2]: raise AssertionError("Opened file differs")
                        if scanner and os.name != "nt" and stat.S_IMODE(opened.st_mode) != 0o700:
                            raise AssertionError("Opened pinned scanner mode differs")
                        before = stable_file(opened)
                        digest, size, body = hashlib.sha256(), 0, None if scanner else bytearray()
                        while size <= limit:
                            chunk = os.read(fd, min(65536, limit + 1 - size))
                            if not chunk: break
                            size += len(chunk); digest.update(chunk)
                            if body is not None: body.extend(chunk)
                        after = os.fstat(fd)
                        if size != info.st_size or before != stable_file(after) or stable_file(child.lstat() if os.name == "nt" else os.stat(entry.name, dir_fd=descriptor, follow_symlinks=False)) != stable_file(info):
                            raise AssertionError("Profile file changed during read")
                    finally: os.close(fd)
                    if scanner and digest.hexdigest() != self.scanner[2]: raise AssertionError("Pinned scanner bytes differ")
                    if not scanner: aggregate += size
                    if aggregate > 16 << 20: raise AssertionError("Profile bytes bound")
                    result[relative] = ("file", identity, info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns, digest.hexdigest(), bytes(body) if body is not None else None)
                    if strict: result[relative] += (stable_file(info), stable_file(opened))
            self.directory(path)
            if strict and result[path.relative_to(self.root).as_posix()][2] != self.directory_metadata(path):
                raise AssertionError("Directory metadata changed during snapshot")
        identity_root = self.directory(self.root)
        visit(self.root, 0)
        for path in self.held: self.directory(path)
        if self.scanner is not None and self.scanner[0] not in result: raise AssertionError("Ordinary UAP scanner is missing")
        return result

    def directory_metadata(self, path):
        def metadata(info):
            return (*object_identity(info, True), info.st_nlink, info.st_size,
                    info.st_mtime_ns, info.st_ctime_ns, getattr(info, "st_birthtime_ns", None),
                    getattr(info, "st_file_attributes", None), getattr(info, "st_reparse_tag", None))
        # Compare each API domain with itself; Windows path/handle attributes may differ.
        return metadata(path.lstat()), metadata(os.fstat(self.held[path][0]))

    def observe(self, command):
        before = self.scan(strict=True)
        body = command()  # The existing bridge returns only after proved family settlement.
        if self.scan(strict=True) != before:
            raise AssertionError("Read-only native command changed original profile")
        return body

    def verify(self, expected, config):
        current = self.scan()
        if set(current) != set(self.base): raise AssertionError("Unknown profile path effect")
        for key, value in self.base.items():
            if key != config and current[key] != value: raise AssertionError("Foreign profile effect")
        before, after = self.base[config], current[config]
        if before[0] != "file" or after[0] != "file" or before[1][0] != after[1][0] or before[1][2:] != after[1][2:] or after[2] != 1 or after[3] > 16384:
            raise AssertionError("Known config replacement authority differs")
        if json.dumps(closed_config(after[-1]), sort_keys=True) != json.dumps(expected, sort_keys=True): raise AssertionError("Unknown config semantic effect")
        return hashlib.sha256(after[-1]).hexdigest()

    def close(self):
        for fd, _linked, _opened in self.held.values(): os.close(fd)
        self.held.clear()


def closed_config(body):
    def unique(pairs):
        if len(pairs) > 128: raise AssertionError("Config member bound")
        value = {}
        for key, item in pairs:
            if key in value: raise AssertionError("Duplicate config field")
            value[key] = item
        return value
    if len(body) > 16384: raise AssertionError("Config byte bound")
    value = json.loads(body, object_pairs_hook=unique, parse_constant=lambda _: (_ for _ in ()).throw(AssertionError("Config nonfinite")))
    def depth(item, level=0):
        if level > 8 or isinstance(item, float) and not math.isfinite(item): raise AssertionError("Config depth/number bound")
        if isinstance(item, (dict, list)):
            for child in item.values() if isinstance(item, dict) else item: depth(child, level + 1)
    depth(value)
    if not isinstance(value, dict): raise AssertionError("Config object required")
    return value


def bridge_exchange(helper, frame, *, timeout=35):
    """Bound the parent transport; only the Go receipt attests native cleanup."""
    if len(frame) > 16384 or not frame.endswith(b"\n") or frame.count(b"\n") != 1:
        raise AssertionError("Supervised request frame refused")
    deadline = time.monotonic() + timeout
    cleanup_deadline = deadline + 5
    child = subprocess.Popen([str(helper)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, bufsize=0)
    captures = [bytearray(), bytearray()]
    failed = threading.Event()
    def drain(pipe, target, bound):
        try:
            while chunk := pipe.read(16384):
                room = bound - len(target)
                target.extend(chunk[:room])
                if len(chunk) > room:
                    failed.set()  # Continue draining, without retaining excess bytes.
        except (OSError, ValueError):
            failed.set()
        finally:
            try:
                pipe.close()
            except (OSError, ValueError):
                failed.set()
    def send():
        try:
            pending = memoryview(frame)
            while pending:
                sent = child.stdin.write(pending)
                if not sent:
                    raise OSError("request pipe closed")
                pending = pending[sent:]
        except (OSError, ValueError):
            failed.set()
    threads = [threading.Thread(target=drain, args=(pipe, target, bound), daemon=True)
               for pipe, target, bound in ((child.stdout, captures[0], 2 << 20),
                                           (child.stderr, captures[1], 65536))]
    threads.append(threading.Thread(target=send, daemon=True))
    failure, cleanup_failed = None, False
    try:
        for thread in threads:
            thread.start()
        while child.poll() is None or any(thread.is_alive() for thread in threads):
            if failed.is_set() or time.monotonic() >= deadline:
                raise AssertionError("Supervised transport bound; owned profile retained")
            time.sleep(.01)
        if failed.is_set() or child.returncode != 0 or captures[1]:
            raise AssertionError("Supervised command receipt refused; owned profile retained")
    except BaseException as exc:
        failure = exc
    finally:
        # Raw FileIO avoids a buffered writer lock. EOF cancels the Go bridge's
        # original context; keep draining while its existing supervisor settles.
        try:
            child.stdin.close()
        except (OSError, ValueError):
            cleanup_failed = True
        if child.poll() is None:
            try:
                child.wait(timeout=max(0, min(4, cleanup_deadline - time.monotonic() - 1)))
            except subprocess.TimeoutExpired:
                cleanup_failed = True
                try:
                    child.kill()  # Exact helper only; never a native cleanup attestation.
                except OSError:
                    cleanup_failed = True
                try:
                    child.wait(timeout=max(0, cleanup_deadline - time.monotonic()))
                except subprocess.TimeoutExpired:
                    cleanup_failed = True
        for thread in threads:
            if thread.ident is not None:
                thread.join(timeout=max(0, cleanup_deadline - time.monotonic()))
        cleanup_failed |= child.poll() is None or any(thread.is_alive() for thread in threads)
    if failure is not None:
        if cleanup_failed:
            raise failure from AssertionError("Helper cleanup unproved; owned profile retained")
        raise failure
    if cleanup_failed or failed.is_set():
        raise AssertionError("Supervised helper cleanup refused; owned profile retained")
    return bytes(captures[0])


def supervised_action(helper, agy, root, selector, action, source_identity, original_pins):
    if tuple(hashlib.sha256(path.read_bytes()).hexdigest() for path in (agy, helper)) != original_pins:
        raise AssertionError("Original executable pin changed")
    pin = original_pins[0]
    req = dict(binary=str(agy), binary_sha256=pin, root=str(root), selector=selector, action=action,
               source_commit=source_identity[0], source_tree=source_identity[1], deadline_unix_ns=time.time_ns() + 29_000_000_000)
    output = bridge_exchange(helper, json.dumps(req).encode() + b"\n")
    if not output.endswith(b"\n") or output.count(b"\n") != 1:
        raise AssertionError("Supervised command receipt refused; owned profile retained")
    if tuple(hashlib.sha256(path.read_bytes()).hexdigest() for path in (agy, helper)) != original_pins:
        raise AssertionError("Executable runtime pin changed")
    value = closed_config(output)
    keys = {"source_commit", "source_tree", "root", "selector", "action", "binary_sha256", "cleanup_complete", "stdout_base64", "stdout_bytes", "stdout_sha256"}
    if set(value) != keys or any(value[key] != req[key] for key in ("source_commit", "source_tree", "root", "selector", "action", "binary_sha256")) or value["cleanup_complete"] is not True:
        raise AssertionError("Command receipt identity differs")
    body = base64.b64decode(value["stdout_base64"], validate=True)
    if type(value["stdout_bytes"]) is not int or len(body) != value["stdout_bytes"] or hashlib.sha256(body).hexdigest() != value["stdout_sha256"]:
        raise AssertionError("Command receipt stream differs")
    return body


def direct_user_enable(agy, uap, source, executable_dir, root_parent):
    helper = Path(os.environ["AGY_ADMISSION_HELPER"]).resolve(strict=True)
    source_identity = (os.environ["AGY_ADMISSION_COMMIT"], os.environ["AGY_ADMISSION_TREE"])
    original_pins = tuple(hashlib.sha256(path.read_bytes()).hexdigest() for path in (agy, helper))
    for initial in (False, True):
        root = Path(tempfile.mkdtemp(prefix="uap-agy-direct-", dir=root_parent))
        print('{"fixtures_retained":true,"scope":"disposable-cli-user-only"}', flush=True)
        env = profile(root, executable_dir)
        env["AGENTPLUGINS_HOME"] = str(root / "uap-state")
        for path in (root / "tmp", root / "project"): path.mkdir()
        run(uap, "add", source, "--target", "antigravity", "--scope", "user", "--no-save", "--yes", "--format", "json", env=env, cwd=root / "project")
        plugins = Path(env["HOME"]) / ".gemini" / "config" / "plugins"
        candidates = list(plugins.iterdir())
        if len(candidates) != 1: raise AssertionError("Exact ordinary UAP directory missing")
        selected = candidates[0]
        manifest = json.loads((selected / "plugin.json").read_bytes())
        for name in ("same-manifest-foreign", "foreign-disabled"):
            target = plugins / name
            shutil.copytree(selected, target)
        config = plugins.parent / "config.json"
        baseline = {"plugins": {selected.name: {"enabled": initial}, "same-manifest-foreign": {"enabled": False}, "foreign-disabled": {"enabled": False}}}
        config.write_text(json.dumps(baseline) + "\n"); config.chmod(0o600)
        held = ProfileLedger(root, lintai=True)
        config_key = config.relative_to(root).as_posix()
        expected = json.loads(json.dumps(baseline))
        def action(kind):
            held.verify(expected, config_key)
            command = lambda: supervised_action(helper, agy, root, selected.name, kind, source_identity, original_pins)
            body = held.observe(command) if kind in ("list", "version") else command()
            if kind in ("enable", "disable"): expected["plugins"][selected.name]["enabled"] = kind == "enable"
            digest = held.verify(expected, config_key)
            print(json.dumps({"scope": "disposable-cli-user-only", "action": kind, "initial_enabled": initial, "config_sha256": digest, "native_project_qualified": False}), flush=True)
            return body
        def inventory():
            rows = json.loads(action("list"))
            if not isinstance(rows, list) or not all(isinstance(row, dict) for row in rows):
                raise AssertionError("Native inventory array/object required")
            for path, state in ((selected, expected["plugins"][selected.name]["enabled"]), (plugins / "same-manifest-foreign", False), (plugins / "foreign-disabled", False)):
                matches = [row for row in rows if row.get("path") == str(path) or row.get("name") == path.name]
                if len(matches) != 1 or matches[0].get("path") != str(path) or matches[0].get("name") != path.name or matches[0].get("enabled") is not state or matches[0].get("version") != manifest["version"] or matches[0].get("skills") != ["native-smoke"] or sorted(matches[0].get("mcpServers", [])) != ["native-http", "native-stdio"]:
                    raise AssertionError("Selected or foreign physical identity differs")
        try:
            if action("version").strip() != b"1.5.3": raise AssertionError("Selected-profile pinned version differs")
            inventory()
            action("enable"); inventory()
            action("disable"); inventory()
            if initial: action("enable"); inventory()
            held.verify(baseline, config_key)
        except BaseException:
            held.close()  # Retain the owned profile on every unproved/unknown effect.
            raise
        held.close()
        # These independent ephemeral-host fixtures are retained, never path-deleted.


def admission_contracts():
    """Inert filesystem and receipt refusals; no native binary or account."""
    import unittest
    from unittest.mock import Mock, patch
    class Contracts(unittest.TestCase):
        def exercise_bridge(self, code, frame=b'{}\n', *, interrupt=False, timeout=5, partial=False, expected_exit=None):
            actual_popen, actual_thread, actual_sleep = subprocess.Popen, threading.Thread, time.sleep
            children, readers, writes = [], [], []
            def spawn(argv, **kwargs):
                self.assertEqual(argv, ["inert-helper"])
                child = actual_popen([sys.executable, "-I", "-c", code], **kwargs)
                if partial:
                    original = child.stdin
                    class PartialWriter:
                        def write(self, body):
                            writes.append(len(body)); return original.write(body[:1024])
                        def close(self): return original.close()
                        @property
                        def closed(self): return original.closed
                    child.stdin = PartialWriter()
                children.append(child)
                return child
            def thread(*args, **kwargs):
                value = actual_thread(*args, **kwargs); readers.append(value); return value
            raised = False
            def tick(seconds):
                nonlocal raised
                if interrupt and not raised:
                    raised = True
                    raise KeyboardInterrupt("inert cancellation")
                actual_sleep(seconds)
            try:
                with patch.object(subprocess, "Popen", side_effect=spawn), patch.object(threading, "Thread", side_effect=thread), patch.object(time, "sleep", side_effect=tick):
                    return bridge_exchange("inert-helper", frame, timeout=timeout)
            finally:
                for child in children:
                    self.assertIsNotNone(child.returncode, "helper was not reaped")
                    self.assertTrue(all(pipe.closed for pipe in (child.stdin, child.stdout, child.stderr)))
                    if expected_exit is not None: self.assertEqual(child.returncode, expected_exit)
                self.assertFalse(any(reader.is_alive() for reader in readers), "bridge thread survived")
                if partial: self.assertGreater(len(writes), 1)

        def test_bridge_drains_both_streams_and_reaps_every_refusal(self):
            prefix = "import sys,time; sys.stdin.buffer.readline(); "
            self.assertEqual(self.exercise_bridge(prefix + "sys.stdout.buffer.write(b'bounded\\n')"), b'bounded\n')
            cases = {
                "stderr-first": prefix + "sys.stderr.buffer.write(b'e' * (1 << 20)); sys.stderr.flush(); sys.stdout.buffer.write(b'late\\n')",
                "stdout-flood": prefix + "sys.stdout.buffer.write(b'x' * ((2 << 20) + 1)); sys.stdout.flush()",
                "nonempty-stderr": prefix + "sys.stderr.buffer.write(b'e'); sys.stdout.buffer.write(b'bounded\\n')",
                "early-exit": "import sys; sys.exit(3)",
                "await-control-eof": prefix + "sys.stdin.buffer.read(); sys.exit(3)",
                "ignores-control": "import time; time.sleep(60)",
            }
            for name, code in cases.items():
                with self.subTest(name=name), self.assertRaises(AssertionError):
                    self.exercise_bridge(code, timeout=.3 if name in {"await-control-eof", "ignores-control"} else 5,
                                         expected_exit=3 if name in {"early-exit", "await-control-eof"} else None)
            with self.assertRaises(KeyboardInterrupt):
                self.exercise_bridge(prefix + "sys.stdin.buffer.read(); sys.exit(3)", interrupt=True, expected_exit=3)

        def test_bridge_refuses_request_overflow_before_spawn_and_handles_partial_writes(self):
            with patch.object(subprocess, "Popen") as spawn, self.assertRaises(AssertionError):
                bridge_exchange("inert-helper", b'x' * 16384 + b'\n')
            spawn.assert_not_called()
            frame = b'x' * 16383 + b'\n'
            code = "import sys; body=sys.stdin.buffer.readline(); sys.stdout.buffer.write((str(len(body)) + '\\n').encode('ascii'))"
            self.assertEqual(self.exercise_bridge(code, frame, partial=True, expected_exit=0), b'16384\n')
            with self.assertRaises(AssertionError):
                self.exercise_bridge("import sys; sys.exit(3)", frame)

        def test_supervised_receipt_still_requires_exact_identity_and_cleanup(self):
            actual_popen = subprocess.Popen
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary); agy, helper = root / "agy", root / "helper"
                agy.write_bytes(b'inert-agy'); helper.write_bytes(b'inert-helper')
                pins = tuple(hashlib.sha256(path.read_bytes()).hexdigest() for path in (agy, helper))
                for change in ("", "r['cleanup_complete']=False", "r['source_tree']='wrong'", "r['stdout_bytes']=4", "close-stdout", "close-stderr"):
                    code = ("import sys,json,base64,hashlib; r=json.loads(sys.stdin.buffer.readline()); del r['binary']; del r['deadline_unix_ns']; "
                            "r.update(cleanup_complete=True,stdout_base64=base64.b64encode(b'[]\\n').decode(),stdout_bytes=3,stdout_sha256=hashlib.sha256(b'[]\\n').hexdigest()); "
                            + (change + "; " if change and not change.startswith("close-") else "") + "print(json.dumps(r))")
                    def spawn(_argv, **kwargs):
                        child = actual_popen([sys.executable, "-I", "-c", code], **kwargs)
                        if change.startswith("close-"):
                            stream = change.removeprefix("close-"); pipe = getattr(child, stream)
                            def refuse_close():
                                pipe.close()
                                raise OSError("inert stream close refusal")
                            setattr(child, stream, Mock(read=pipe.read, close=refuse_close))
                        return child
                    with self.subTest(change=change), patch.object(subprocess, "Popen", side_effect=spawn):
                        if change:
                            with self.assertRaises(AssertionError): supervised_action(helper, agy, root, "owned", "list", ("a" * 40, "b" * 40), pins)
                        else:
                            self.assertEqual(supervised_action(helper, agy, root, "owned", "list", ("a" * 40, "b" * 40), pins), b'[]\n')

        def test_ledger_accepts_only_selected_atomic_config_delta(self):
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                config = root / "config.json"
                before = {"plugins": {"owned": {"enabled": False}, "foreign": {"enabled": False}}}
                config.write_text(json.dumps(before))
                held = ProfileLedger(root)
                try:
                    after = json.loads(json.dumps(before)); after["plugins"]["owned"]["enabled"] = True
                    replacement = root / "pending"
                    replacement.write_text(json.dumps(after)); replacement.chmod(config.stat().st_mode & 0o777)
                    replacement.replace(config)
                    held.verify(after, "config.json")
                    config.write_text(json.dumps(before))
                    held.verify(before, "config.json")
                    for wrong in ({"plugins": {"owned": {"enabled": False}}}, {"unexpected": True}, {"plugins": {"owned": {"enabled": 0}, "foreign": {"enabled": False}}}):
                        config.write_text(json.dumps(wrong))
                        with self.assertRaises(AssertionError): held.verify(before, "config.json")
                finally: held.close()

        def test_ledger_refuses_unknown_alias_large_parent_and_late_race(self):
            for kind in ("new", "alias", "hardlink", "large", "parent", "race"):
                with self.subTest(kind=kind), tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary) / "owned"; root.mkdir()
                    config = root / "config.json"; config.write_text('{}')
                    held = ProfileLedger(root)
                    try:
                        if kind == "new": (root / "new").write_text('private')
                        elif kind == "alias": config.unlink(); config.symlink_to(root / "outside")
                        elif kind == "hardlink": os.link(config, root / "linked")
                        elif kind == "large": config.write_bytes(b'x' * ((1 << 20) + 1))
                        elif kind == "parent":
                            if os.name == "nt": continue  # Held no-delete directory already fences replacement.
                            root.rename(root.with_name('old')); root.mkdir(); (root / 'foreign').write_text('preserve')
                        if kind == "race":
                            original = os.read
                            def raced(fd, count):
                                body = original(fd, count); config.write_text('{"changed":true}'); return body
                            with patch('os.read', side_effect=raced), self.assertRaises(AssertionError): held.verify({}, 'config.json')
                        else:
                            with self.assertRaises((AssertionError, OSError)): held.verify({}, 'config.json')
                        if kind == 'parent': self.assertEqual((root / 'foreign').read_text(), 'preserve')
                    finally: held.close()

        def test_read_only_atime_advance_does_not_refuse(self):
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary); config = root / "config.json"; config.write_text('{}')
                os.utime(config, ns=(1, config.stat().st_mtime_ns))
                held = ProfileLedger(root)
                try: held.verify({}, "config.json")
                finally: held.close()
                before = config.stat()
                from types import SimpleNamespace
                changed = SimpleNamespace(**{name: getattr(before, name) for name in dir(before) if name.startswith('st_')})
                changed.st_atime_ns += 1
                self.assertEqual(stable_file(before), stable_file(changed))

        def test_only_exact_immutable_scanner_has_a_separate_byte_bound(self):
            body = b's' * ((1 << 20) + 17)
            pins = {key: (len(body), hashlib.sha256(body).hexdigest()) for key in LINTAI_PINS}
            for effect in ("none", "bytes", "unknown-copy", "missing", "race", "config-aggregate"):
                with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary, patch.dict(globals(), {"LINTAI_PINS": pins}):
                    root = Path(temporary); (root / "config.json").write_text('{}')
                    scanner = root / lintai_member()[0]
                    scanner.parent.mkdir(parents=True); scanner.write_bytes(body); scanner.chmod(0o700)
                    held = ProfileLedger(root, lintai=True)
                    try:
                        held.verify({}, "config.json")
                        if effect == "none": continue
                        if effect == "bytes": scanner.write_bytes(b't' + body[1:])
                        elif effect == "unknown-copy": (root / "other-scanner").write_bytes(body)
                        elif effect == "missing": scanner.unlink()
                        elif effect == "config-aggregate":
                            for index in range(17): (root / str(index)).write_bytes(b'x' * (1 << 20))
                        if effect == "race":
                            original = os.read
                            def raced(fd, count):
                                chunk = original(fd, count)
                                if os.fstat(fd).st_ino == scanner.stat().st_ino:
                                    with scanner.open('r+b') as stream: stream.write(b't')
                                return chunk
                            with patch('os.read', side_effect=raced), self.assertRaises(AssertionError): held.verify({}, "config.json")
                        else:
                            with self.assertRaises(AssertionError): held.verify({}, "config.json")
                    finally: held.close()
                    if effect == "bytes":
                        with self.assertRaises(AssertionError): ProfileLedger(root, lintai=True)
                    if effect == "missing":
                        with self.assertRaises(AssertionError): ProfileLedger(root, lintai=True)

        @unittest.skipIf(os.name == "nt", "Windows chmod has a different documented permission domain")
        def test_scanner_initial_posix_mode_is_exact_before_hashing(self):
            body = b's' * ((1 << 20) + 17)
            pins = {key: (len(body), hashlib.sha256(body).hexdigest()) for key in LINTAI_PINS}
            for mode in (0o600, 0o755, 0o777):
                with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary, patch.dict(globals(), {"LINTAI_PINS": pins}):
                    root = Path(temporary); scanner = root / lintai_member()[0]
                    scanner.parent.mkdir(parents=True); scanner.write_bytes(body); scanner.chmod(mode)
                    original = os.read
                    def guarded(fd, count):
                        if os.fstat(fd).st_ino == scanner.stat().st_ino:
                            raise AssertionError("scanner bytes read before mode admission")
                        return original(fd, count)
                    with patch('os.read', side_effect=guarded), self.assertRaisesRegex(AssertionError, "Pinned scanner mode differs"):
                        ProfileLedger(root, lintai=True)

        @unittest.skipIf(os.name == "nt", "Windows chmod has a different documented permission domain")
        def test_scanner_same_inode_mode_race_refuses_before_read(self):
            body = b's' * ((1 << 20) + 17)
            pins = {key: (len(body), hashlib.sha256(body).hexdigest()) for key in LINTAI_PINS}
            for mode in (0o600, 0o755, 0o777):
                with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary, patch.dict(globals(), {"LINTAI_PINS": pins}):
                    root = Path(temporary).resolve(); scanner = root / lintai_member()[0]
                    scanner.parent.mkdir(parents=True); scanner.write_bytes(body); scanner.chmod(0o700)
                    original_handle, original_read = owned_handle, os.read
                    identity = (scanner.stat().st_dev, scanner.stat().st_ino)
                    changed, reads = [], []
                    def raced(path, *args, **kwargs):
                        if path == scanner:
                            scanner.chmod(mode); changed.append(True)
                            self.assertEqual((scanner.stat().st_dev, scanner.stat().st_ino), identity)
                        return original_handle(path, *args, **kwargs)
                    def guarded(fd, count):
                        info = os.fstat(fd)
                        if (info.st_dev, info.st_ino) == identity:
                            reads.append(True)
                            raise AssertionError("scanner bytes read before held mode admission")
                        return original_read(fd, count)
                    with patch.dict(globals(), {"owned_handle": raced}), patch('os.read', side_effect=guarded), self.assertRaisesRegex(AssertionError, "Opened pinned scanner mode differs"):
                        ProfileLedger(root, lintai=True)
                    self.assertEqual(changed, [True]); self.assertEqual(reads, [])

        def test_unknown_first_effect_prevents_every_later_mutation(self):
            with tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary); binary = parent / "inert"; binary.write_text("never executed")
                calls = []
                def deliver(_binary, *_args, env, cwd):
                    plugin = Path(env["HOME"]) / ".gemini/config/plugins/owned-physical"
                    plugin.mkdir(parents=True)
                    (plugin / "plugin.json").write_text('{"name":"inert","version":"1.0.0"}')
                    scanner = Path(env["AGENTPLUGINS_HOME"]).parent / lintai_member()[0]
                    scanner.parent.mkdir(parents=True); scanner.write_bytes(b'scanner'); scanner.chmod(0o700)
                    return '[]'
                def command(_helper, _agy, root, selector, action, _source, _pins):
                    calls.append(action)
                    if action == "version": return b"1.5.3\n"
                    if action != "list":
                        (root / "unexpected").write_text("unknown")
                        return b''
                    return json.dumps([{"name": name, "path": str(root / "home/.gemini/config/plugins" / name),
                                        "enabled": False, "version": "1.0.0", "skills": ["native-smoke"],
                                        "mcpServers": ["native-http", "native-stdio"]}
                                       for name in (selector, "same-manifest-foreign", "foreign-disabled")]).encode()
                pins = {key: (7, hashlib.sha256(b'scanner').hexdigest()) for key in LINTAI_PINS}
                with patch.dict(os.environ, {"AGY_ADMISSION_HELPER": str(binary), "AGY_ADMISSION_COMMIT": "a" * 40, "AGY_ADMISSION_TREE": "b" * 40}), patch.dict(globals(), {"run": deliver, "supervised_action": command, "LINTAI_PINS": pins}), self.assertRaises(AssertionError):
                    direct_user_enable(binary, binary, parent, parent, parent)
                self.assertEqual(calls, ["version", "list", "enable"])
                retained = list(parent.glob("uap-agy-direct-*"))
                self.assertEqual(len(retained), 1)
                self.assertEqual((retained[0] / "unexpected").read_text(), "unknown")

        def test_refused_member_diagnostic_is_bounded_and_never_reads_content(self):
            from types import SimpleNamespace
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary); child = root / ("s" * 150)
                child.write_bytes(b"PRIVATE-CONTENT" + b"x" * (1 << 20))
                original_open = owned_handle
                def guarded(path, *args, **kwargs):
                    if path == child: raise AssertionError("Refused member was opened")
                    return original_open(path, *args, **kwargs)
                with patch.dict(globals(), {"owned_handle": guarded}), self.assertRaises(AssertionError) as caught:
                    ProfileLedger(root)
                message = str(caught.exception)
                facts = json.loads(message.split(": ", 1)[1])
                self.assertEqual(set(facts), {"type", "reason", "member", "member_sha256", "member_truncated", "same_device", "overflow", "bytes", "links", "mode"})
                self.assertEqual(facts["reason"], "size")
                self.assertEqual(facts["member_sha256"], hashlib.sha256(child.name.encode()).hexdigest())
                self.assertTrue(facts["member_truncated"])
                self.assertLessEqual(len(facts["member"].encode()), 128)
                self.assertFalse(facts["overflow"])
                self.assertNotIn("PRIVATE-CONTENT", message)
                self.assertNotIn(str(root), message)
                fake = SimpleNamespace(st_size=1 << 54, st_nlink=1, st_mode=stat.S_IFREG | 0o600, st_dev=root.stat().st_dev)
                with self.assertRaises(AssertionError) as overflow:
                    profile_file_refusal(root, child, fake, fake.st_dev)
                capped = json.loads(str(overflow.exception).split(": ", 1)[1])
                self.assertEqual(capped["bytes"], (1 << 53) - 1)
                self.assertTrue(capped["overflow"])

        def test_windows_owner_uses_only_exact_token_default_owner(self):
            import ctypes
            from ctypes import wintypes as w
            class TokenAPI:
                def __init__(self, selected=202, bound=32, returned=32, fail=False):
                    self.selected, self.bound, self.returned, self.fail = selected, bound, returned, fail
                    self.classes, self.equal_calls = [], []
                def GetTokenInformation(self, _token, information_class, body, size, needed):
                    self.classes.append(information_class)
                    ctypes.cast(needed, ctypes.POINTER(w.DWORD))[0] = self.bound if body is None else self.returned
                    if body is None or self.fail: return 0
                    self.asserted_size = size
                    ctypes.cast(body, ctypes.POINTER(ctypes.c_void_p))[0] = self.selected
                    return 1
                def EqualSid(self, owner, selected):
                    self.equal_calls.append((owner, selected))
                    return owner == selected
            # Distinct synthetic user/default-owner IDs: only exact default owner202 passes.
            api = TokenAPI()
            require_windows_token_owner(202, object(), api)
            self.assertEqual(api.classes, [4, 4])
            self.assertEqual(api.equal_calls, [(202, 202)])
            for owner, changes in ((101, {}), (303, {}), (0, {}), (202, {'selected': 0}),
                                   (202, {'bound': 0}), (202, {'bound': 65537}),
                                   (202, {'returned': 33}), (202, {'returned': 0}), (202, {'fail': True})):
                with self.subTest(owner=owner, changes=changes):
                    api = TokenAPI(**changes)
                    with self.assertRaises(AssertionError): require_windows_token_owner(owner, object(), api)
                    self.assertTrue(api.classes and all(value == 4 for value in api.classes))

        def test_observation_requires_complete_unchanged_profile(self):
            for effect in ("none", "file-mtime", "directory-mtime", "replace", "new-member", "content"):
                with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                    root = Path(temporary).resolve(); folder = root / "home"; folder.mkdir()
                    target = folder / "config.json"; target.write_bytes(b'{"plugins":{}}')
                    ledger = ProfileLedger(root)
                    calls = []
                    def command():
                        calls.append("version")
                        if effect == "file-mtime":
                            info = target.stat(); os.utime(target, ns=(info.st_atime_ns, info.st_mtime_ns + 1000000000))
                        elif effect == "directory-mtime":
                            info = folder.stat(); os.utime(folder, ns=(info.st_atime_ns, info.st_mtime_ns + 1000000000))
                        elif effect == "replace":
                            pending = folder / "pending"; pending.write_bytes(target.read_bytes()); pending.replace(target)
                        elif effect == "new-member": (folder / "new").write_bytes(b"unexpected")
                        elif effect == "content": target.write_bytes(b'{"plugins":{ },"new":true}')
                        return b"1.5.3\n"
                    try:
                        original = ledger.base.copy()
                        if effect == "none":
                            self.assertEqual(ledger.observe(command), b"1.5.3\n")
                        else:
                            with self.assertRaisesRegex(AssertionError, "Read-only native command"):
                                ledger.observe(command)
                                calls.append("later-mutation")
                        self.assertEqual(calls, ["version"])
                        self.assertEqual(ledger.base, original)
                    finally: ledger.close()

        def test_observation_excludes_only_read_atime_and_propagates_unsettled(self):
            with tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); target = root / "file"; target.write_bytes(b"same")
                ledger = ProfileLedger(root)
                original_stat, original_fstat = os.stat, os.fstat
                offset = [0]
                class AccessTimeView:
                    def __init__(self, info): self.info = info
                    def __getattr__(self, name):
                        if name == "st_atime": return self.info.st_atime + offset[0]
                        if name == "st_atime_ns": return self.info.st_atime_ns + offset[0] * 1000000000
                        return getattr(self.info, name)
                def path_stat(*args, **kwargs): return AccessTimeView(original_stat(*args, **kwargs))
                def held_stat(*args, **kwargs): return AccessTimeView(original_fstat(*args, **kwargs))
                try:
                    with patch("os.stat", side_effect=path_stat), patch("os.fstat", side_effect=held_stat):
                        before = ledger.scan(strict=True)
                        offset[0] = 1
                        self.assertEqual(ledger.scan(strict=True), before)
                        def command():
                            offset[0] = 2
                            return b"1.5.3\n"
                        self.assertEqual(ledger.observe(command), b"1.5.3\n")
                    def unsettled(): raise AssertionError("cleanup unproved")
                    with self.assertRaisesRegex(AssertionError, "cleanup unproved"): ledger.observe(unsettled)
                finally: ledger.close()

        def test_config_is_closed_and_bounded(self):
            for body in (b'{"x":1,"x":2}', b'{} {}', b'{"x":NaN}', b'{"x":1e999}', b'[' * 10 + b'0' + b']' * 10, b'x' * 16385):
                with self.assertRaises((AssertionError, ValueError)): closed_config(body)
    result = unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(Contracts))
    if not result.wasSuccessful(): raise SystemExit(1)


def main():
    agy = Path(os.environ["AGY_BIN"]).resolve(strict=True)
    uap = Path(os.environ["UAP_BIN"]).resolve(strict=True)
    root_parent = Path(os.environ["RUNNER_TEMP"]).resolve(strict=True)
    with tempfile.TemporaryDirectory(prefix="uap-antigravity-native-", dir=root_parent) as temp:
        root = Path(temp)
        executable_dir = root / "bin"
        executable_dir.mkdir()
        shim = executable_dir / ("agy.exe" if os.name == "nt" else "agy")
        shutil.copy2(agy, shim)
        if os.name != "nt":
            shim.chmod(0o700)
        user_env = profile(root / "uap-user", executable_dir)
        vendor_env = profile(root / "agy-vendor-only", executable_dir)
        project = root / "project-a"
        project_b = root / "project-b"
        source = root / "source"
        for directory in (project, project_b, source / "skills" / "native-smoke"):
            directory.mkdir(parents=True)
        portable = {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": NAME,
            "version": "1.0.0",
            "description": "Isolated cross-platform Antigravity native lifecycle smoke",
        }
        manifest = source / "plugin.json"
        manifest.write_text(json.dumps(portable, indent=2) + "\n", encoding="utf-8")
        source_hash = hashlib.sha256(manifest.read_bytes()).hexdigest()
        (source / "skills" / "native-smoke" / "SKILL.md").write_text(
            "---\nname: native-smoke\n"
            "description: Disposable native plugin validation.\n"
            "---\nDo not run commands or connect to external services.\n",
            encoding="utf-8",
        )
        scripts = source / "scripts"
        scripts.mkdir()
        helper = scripts / "mcp-smoke.cjs"
        helper.write_text(
            "// Native validation fixture; no network access or model calls.\n"
            "const lines = require('node:readline').createInterface({ input: process.stdin });\n"
            "lines.on('line', line => {\n"
            "  const request = JSON.parse(line);\n"
            "  if (request.id === undefined) return;\n"
            "  const result = request.method === 'initialize'\n"
            "    ? { protocolVersion: request.params.protocolVersion, capabilities: { tools: {} },\n"
            "        serverInfo: { name: 'uap-native-smoke', version: '1.0.0' } }\n"
            "    : { tools: [] };\n"
            "  process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }) + '\\n');\n"
            "});\n",
            encoding="utf-8",
        )
        if os.name != "nt":
            helper.chmod(0o755)
        (source / "mcp.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {
                "native-stdio": {
                    "type": "stdio", "command": "node",
                    "args": ["${PLUGIN_ROOT}/scripts/mcp-smoke.cjs", "--data-dir", "${PLUGIN_DATA}"],
                    "cwd": "${PLUGIN_ROOT}", "env": {"UAP_SMOKE_DATA": "${PLUGIN_DATA}"},
                },
                "native-http": {
                    "type": "streamable-http", "url": REMOTE_URL,
                    "headers": {"X-UAP-Smoke": "native-e2e"},
                },
            },
        }, indent=2) + "\n", encoding="utf-8")
        source_snapshot = tree_snapshot(source)
        foreign = Path(user_env["HOME"]) / ".gemini" / "config" / "plugins" / "existing-foreign-plugin"
        foreign.mkdir(parents=True)
        sentinel = foreign / "sentinel"
        sentinel.write_text("foreign must survive\n", encoding="utf-8")
        print("Pinned agy:", run(agy, "--version", env=user_env, cwd=project).strip(), flush=True)

        output = run(
            uap, "add", source, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--no-save", "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(output):
            raise AssertionError("UAP returned an empty add result")
        plugin_root = project / ".agents" / "plugins"
        if not plugin_root.is_dir():
            raise AssertionError("UAP did not create its project-native plugins root")
        owned = [p for p in plugin_root.iterdir()
                 if p.is_dir() and not p.is_symlink()
                 and not p.name.startswith(".agentplugins-staging-")]
        if len(owned) != 1:
            raise AssertionError(f"Expected one owned native plugin, found {len(owned)}")
        plugin = owned[0]
        vendor_manifest = plugin / "plugin.json"
        if (plugin / ".antigravity-plugin").exists() or not vendor_manifest.is_file():
            raise AssertionError("UAP emitted missing or duplicate native plugin manifest")
        native = json.loads(vendor_manifest.read_text(encoding="utf-8"))
        if native.get("name") != NAME or native.get("$schema") != "https://antigravity.google/schemas/v1/plugin.json":
            raise AssertionError("Native schema or identity was not projected")
        if not (plugin / "skills" / "native-smoke" / "SKILL.md").is_file():
            raise AssertionError("Selected native Skill was not staged")
        projected_hash = hashlib.sha256(vendor_manifest.read_bytes()).hexdigest()
        assert_native_mcp(plugin, root)
        projected_snapshot = tree_snapshot(plugin)

        # Use the public CLI with a genuine tracked native Project installation.
        # Repeated update must preserve the exact source and native manifest.
        updated = run(
            uap, "update", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(updated):
            raise AssertionError("UAP update produced no structured result")
        if hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("Idempotent Project update changed the native manifest")
        skill_path = plugin / "skills" / "native-smoke" / "SKILL.md"
        original_skill = (source / "skills" / "native-smoke" / "SKILL.md").read_bytes()
        skill_path.unlink()
        damaged_snapshot = tree_snapshot(plugin)
        repair_args = (
            "repair", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
        )
        # Grouped repair deliberately refuses partially changed packages.
        # Preserve this ownership gate and prove refusal does not touch bytes.
        try:
            run(uap, *repair_args, env=user_env, cwd=project)
        except RuntimeError as error:
            if "native identity ownership is indeterminate; refusing repair" not in str(error):
                raise
        else:
            raise AssertionError("Repair silently replaced a partially changed native package")
        if tree_snapshot(plugin) != damaged_snapshot or tree_snapshot(source) != source_snapshot:
            raise AssertionError("Refused partial-package repair changed fixture bytes")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Refused partial-package repair changed a foreign User plugin")
        # The exact-owned bundle is disposable test data. A fully absent bundle
        # is the supported recovery case, with its receipt and revision intact.
        shutil.rmtree(plugin)
        repaired = run(uap, *repair_args, env=user_env, cwd=project)
        if not json.loads(repaired) or skill_path.read_bytes() != original_skill:
            raise AssertionError("Exact-revision absent Project repair did not restore owned Skill bytes")
        if tree_snapshot(plugin) != projected_snapshot:
            raise AssertionError("Absent Project repair did not restore the exact native package")
        assert_native_mcp(plugin, root)
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Update or repair changed a foreign User plugin")

        run(agy, "plugin", "validate", "--strict", plugin, env=user_env, cwd=project)
        if NAME in checked_list(agy, vendor_env, project):
            raise AssertionError("CLI profile unexpectedly owns a project-only plugin")
        run(agy, "plugin", "install", plugin, env=vendor_env, cwd=project)
        if NAME not in checked_list(agy, vendor_env, project):
            raise AssertionError("Pinned agy did not list its explicitly imported plugin")
        imported = Path(vendor_env["HOME"]) / ".gemini" / "config" / "plugins" / NAME
        imported_manifest = json.loads((imported / "plugin.json").read_text(encoding="utf-8"))
        imported_snapshot = tree_snapshot(imported)
        foreign_import = imported.parent / "foreign-disabled"
        foreign_import.mkdir()
        (foreign_import / "plugin.json").write_text(json.dumps({
            "$schema": "https://antigravity.google/schemas/v1/plugin.json",
            "name": "foreign-disabled", "version": "1.0.0",
            "description": "Foreign disabled plugin must retain its state",
        }), encoding="utf-8")
        (foreign_import / "sentinel").write_text("foreign must survive\n", encoding="utf-8")
        run(agy, "plugin", "disable", "foreign-disabled", env=vendor_env, cwd=project)
        foreign_snapshot = tree_snapshot(foreign_import)
        for state in (False, True):
            run(agy, "plugin", "enable" if state else "disable", NAME,
                env=vendor_env, cwd=project)
            checked_plugin(agy, vendor_env, project, imported, imported_manifest, state)
            if tree_snapshot(imported) != imported_snapshot or tree_snapshot(plugin) != projected_snapshot:
                raise AssertionError("Vendor enablement changed imported or UAP-owned package bytes")
            rows = json.loads(run(agy, "plugin", "list", "--json", env=vendor_env, cwd=project))
            foreign_rows = [row for row in rows if row.get("path") == str(foreign_import)]
            if (len(foreign_rows) != 1 or foreign_rows[0].get("enabled") is not False
                    or tree_snapshot(foreign_import) != foreign_snapshot):
                raise AssertionError("Selected vendor toggle changed foreign plugin or disablement")
        run(agy, "plugin", "uninstall", NAME, env=vendor_env, cwd=project)
        if NAME in checked_list(agy, vendor_env, project):
            raise AssertionError("Pinned agy retained the uninstalled test plugin")
        rows = json.loads(run(agy, "plugin", "list", "--json", env=vendor_env, cwd=project))
        foreign_rows = [row for row in rows if row.get("path") == str(foreign_import)]
        if (len(foreign_rows) != 1 or foreign_rows[0].get("enabled") is not False
                or tree_snapshot(foreign_import) != foreign_snapshot):
            raise AssertionError("Vendor uninstall changed foreign plugin or disablement")

        if hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("Native CLI changed UAP-owned staged source")
        if hashlib.sha256(manifest.read_bytes()).hexdigest() != source_hash:
            raise AssertionError("UAP or native CLI changed the portable source")
        if list((project_b / ".agents").glob("plugins/*")) if (project_b / ".agents").exists() else False:
            raise AssertionError("Project A installation mutated independent project B")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("Project installation modified foreign User plugin")
        if (Path(user_env["HOME"]) / ".gemini" / "config" / "plugins" / NAME).exists():
            raise AssertionError("Project installation registered global User plugin")

        # Exercise the public UAP User lifecycle independently of the Project
        # receipt and the separate vendor-only import/uninstall profile.
        user_native_env = profile(root / "uap-native-user-scope", executable_dir)
        native_user_root = Path(user_native_env["HOME"]) / ".gemini" / "config" / "plugins"
        user_result = run(
            uap, "add", source, "--target", "antigravity", "--scope", "user",
            "--no-save", "--yes", "--format", "json",
            env=user_native_env, cwd=project_b,
        )
        if not json.loads(user_result) or not native_user_root.is_dir():
            raise AssertionError("UAP User installation did not create native plugin root")
        user_plugins = [
            entry for entry in native_user_root.iterdir()
            if entry.is_dir() and not entry.is_symlink()
            and not entry.name.startswith(".agentplugins-staging-")
        ]
        if len(user_plugins) != 1:
            raise AssertionError("UAP User scope did not create exactly one owned plugin")
        user_plugin = user_plugins[0]
        user_manifest = json.loads((user_plugin / "plugin.json").read_text(encoding="utf-8"))
        if (user_manifest.get("name") != NAME or
                user_manifest.get("$schema") != "https://antigravity.google/schemas/v1/plugin.json"):
            raise AssertionError("UAP User scope did not project a native manifest")
        if not (user_plugin / "skills" / "native-smoke" / "SKILL.md").is_file():
            raise AssertionError("UAP User scope omitted the selected Skill")
        assert_native_mcp(user_plugin, root)
        run(agy, "plugin", "validate", "--strict", user_plugin, env=user_native_env, cwd=project_b)
        # Pinned agy discovers direct User bundles by physical directory name,
        # which differs from UAP's declared package name. No import is needed.
        # Inventory enablement is not IDE loading or live MCP execution.
        before_inventory = tree_snapshot(Path(user_native_env["HOME"]).parent)
        observed = checked_plugin(agy, user_native_env, project_b, user_plugin, user_manifest)
        if tree_snapshot(Path(user_native_env["HOME"]).parent) != before_inventory:
            raise AssertionError("Read-only native inventory changed the selected User profile")
        print(f"Native UAP User bundle discovered at exact owned path; "
              f"enabled={observed['enabled']}; IDE activation was not attested.", flush=True)
        user_removed = run(
            uap, "remove", NAME, "--target", "antigravity", "--scope", "user",
            "--yes", "--format", "json", env=user_native_env, cwd=project_b,
        )
        if not json.loads(user_removed) or user_plugin.exists():
            raise AssertionError("UAP User-scoped plugin survived exact-owned removal")
        rows = json.loads(run(agy, "plugin", "list", "--json", env=user_native_env, cwd=project_b))
        if not isinstance(rows, list) or any(
                row.get("path") == str(user_plugin) or row.get("name") == user_plugin.name
                for row in rows if isinstance(row, dict)):
            raise AssertionError("Native inventory retained the removed physical User identity")
        if not plugin.is_dir() or hashlib.sha256(vendor_manifest.read_bytes()).hexdigest() != projected_hash:
            raise AssertionError("UAP User-scoped removal changed Project-scoped plugin")

        result = run(
            uap, "remove", NAME, "--target", "antigravity", "--scope", "project",
            "--project-root", project, "--yes", "--format", "json",
            env=user_env, cwd=project,
        )
        if not json.loads(result) or plugin.exists():
            raise AssertionError("UAP did not remove only its exact-owned project plugin")
        if sentinel.read_text(encoding="utf-8") != "foreign must survive\n":
            raise AssertionError("UAP removal modified a foreign User plugin")
        if tree_snapshot(source) != source_snapshot:
            raise AssertionError("Native lifecycle changed the verified portable source")
        if os.environ.get("AGY_ADMISSION_HELPER"):
            direct_user_enable(agy, uap, source, executable_dir, root_parent)
        print("PASS: real agy strict validation of Skills + stdio/HTTP MCP, separate CLI "
              "import/list/uninstall; UAP User/Project add/remove, idempotent update, "
              "partial-drift refusal and exact absent-bundle repair. Foreign User state "
              "preserved. IDE activation and live MCP execution were not observed.", flush=True)


if __name__ == "__main__":
    try:
        admission_contracts() if sys.argv[1:] == ["--contracts"] else main()
    except (AssertionError, KeyError, OSError, RuntimeError, subprocess.TimeoutExpired,
            json.JSONDecodeError) as error:
        print(f"Antigravity native E2E failed: {error}", file=sys.stderr)
        sys.exit(1)
