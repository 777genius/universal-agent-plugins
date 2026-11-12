#!/usr/bin/env python3
"""Bounded REAL Codex native secrecy spike; synthetic secret only, never logs its value.

Pass requires an actual Codex shell tool call, then an MCP stdio tools/call
through name-only env_vars; inspect the entire isolated Codex/home/project tree.
This is not a product feature, and never reads real user credentials.
"""
import base64
import contextlib
import errno
import hashlib
import http.server
import json
import ntpath
import os
import re
from pathlib import Path
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile
import threading
import time
import urllib.parse

ENV_NAME = "UAP_E2E_SYNTHETIC_TOKEN"
FOREIGN_ENV_NAME = "UAP_E2E_FOREIGN_PASSWORD"
# Explicit restricted-token sandbox only for disposable native Windows CI.
# Official Codex 0.167.1 package smoke tests use windows.sandbox="unelevated".
WINDOWS_NATIVE_SANDBOX_CONFIG = '[windows]\nsandbox = "unelevated"\n'
SHELL_CALL = "call_uap_shell_snapshot"
SHELL_MARKER = "UAP_SHELL_COMPLETED"
SHELL_ENV_PRESENT = "UAP_SHELL_ENV_PRESENT"
SHELL_FOREIGN_PRESENT = "UAP_SHELL_FOREIGN_PRESENT"
MCP_CALL = "call_uap_env_mcp"
SEARCH_CALL = "call_uap_tool_search"

# Vendor-supplied tool names are untrusted. Report only this fixed vocabulary;
# never serialize arbitrary advertised names or model/provider metadata.
KNOWN_NATIVE_TOOL_NAMES = frozenset({
    "exec_command", "shell_command", "tool_search",
    "mcp__env_spike", "mcp__env_spike__confirm_env",
    "create_goal", "get_goal", "update_goal",
    "list_mcp_resource_templates", "list_mcp_resources",
    "multi_agent_v1", "read_mcp_resource", "request_user_input",
    "view_image", "write_stdin",
})

CHILD = r'''
import hashlib, json, os, sys
name = "UAP_E2E_SYNTHETIC_TOKEN"
for line in sys.stdin:
    try:
        q = json.loads(line)
        method = q.get("method")
        if method == "initialize":
            out = {"protocolVersion": "2025-03-26",
                   "capabilities": {"tools": {}},
                   "serverInfo": {"name": "uap-env-spike", "version": "1"}}
        elif method == "tools/list":
            out = {"tools": [{"name": "confirm_env",
                  "description": "Return inert value-presence proof, no plaintext",
                  "inputSchema": {"type": "object", "properties": {}},
                  "annotations": {"readOnlyHint": True, "destructiveHint": False,
                                  "idempotentHint": True, "openWorldHint": False}}]}
        elif method == "tools/call":
            value = os.environ.get(name)
            evidence = {"observed": bool(value),
                        "sha256": hashlib.sha256(value.encode()).hexdigest() if value else ""}
            out = {"content": [{"type": "text",
                        "text": json.dumps(evidence, sort_keys=True)}]}
        elif method == "ping":
            out = {}
        else:
            continue
        if q.get("id") is not None:
            print(json.dumps({"jsonrpc": "2.0", "id": q["id"], "result": out}), flush=True)
    except Exception:
        break
'''



def structured_mcp_receipt_matches(payload, expected_digest):
    """Require the child's structured observed=true + exact digest, not text hits.

    The genuine Codex model/direct tool responses wrap an MCP text content
    element in dicts/lists. Parse only complete bounded JSON strings. A raw
    diagnostic, rejected tool response or incidental digest is never a PASS.
    """
    if not expected_digest:
        return False

    def visit(item, depth):
        if depth > 8:
            return False
        if isinstance(item, dict):
            if (item.get("isError") not in (None, False)
                    or item.get("error") not in (None, False)):
                return False
            if "observed" in item or "sha256" in item:
                return (item.get("observed") is True
                        and item.get("sha256") == expected_digest)
            return any(
                visit(item[key], depth + 1)
                for key in ("content", "output", "result", "text")
                if key in item
            )
        if isinstance(item, list):
            return any(visit(value, depth + 1) for value in item[:32])
        if isinstance(item, str) and len(item) <= 65536:
            try:
                return visit(json.loads(item), depth + 1)
            except (TypeError, ValueError):
                return False
        return False

    return visit(payload, 0)


def fixed_windows_spawn_error_category(response):
    """Return only a fixed Win32 error class, never the native tool response."""
    if not isinstance(response, str):
        return "not_reported"
    text = response.lower()
    # Only parse numeric codes labelled as operating-system errors. A number
    # elsewhere in model output is NOT vendor evidence.
    match = re.search(r"\b(?:os error|win32 error|error code)\s*[:=]?\s*(\d{1,5})\b", text)
    if match is None:
        return "not_reported"
    return {
        "2": "file_not_found",
        "3": "path_not_found",
        "5": "access_denied",
        "87": "invalid_parameter",
        "123": "invalid_path_name",
        "193": "invalid_executable",
        "267": "invalid_directory",
        "740": "elevation_required",
        "1314": "privilege_not_held",
    }.get(match.group(1), "other_win32_error")


def isolated_root(prefix):
    """Keep disposable Codex project trust keys canonical on symlinked temp dirs.

    macOS's /var/folders alias can resolve to /private/var/folders. The Codex
    CLI may canonicalize cwd before matching [projects.<path>] trust keys.
    Canonicalize our NEW, isolated root before generating either config; no
    real user profile is visited or modified.
    """
    return Path(tempfile.mkdtemp(prefix=prefix)).resolve(strict=True)


def isolated_env(root, home, secret, foreign_secret):
    """Disposable Codex profile: no real runner secrets are inherited."""
    env = {"HOME": str(home), "CODEX_HOME": str(home / ".codex"),
           "TMPDIR": str(root / "tmp"), "TEMP": str(root / "tmp"),
           "TMP": str(root / "tmp"), "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
           "LANG": "C.UTF-8", "NO_COLOR": "1", ENV_NAME: secret,
           FOREIGN_ENV_NAME: foreign_secret}
    if os.name == "nt":
        # Only vendor-documented nonsecret Windows core process fields.
        # Never copy credentials or the runner's actual user profile paths.
        env.update({key: os.environ[key] for key in
                    ("SYSTEMROOT", "WINDIR", "SYSTEMDRIVE", "COMSPEC", "PATHEXT",
                     "OS", "PROGRAMFILES", "PROGRAMFILES(X86)", "PROGRAMW6432",
                     "PROGRAMDATA", "USERNAME", "USERDOMAIN")
                    if key in os.environ})
        env["USERPROFILE"] = str(home)
        env["APPDATA"] = str(home / "AppData" / "Roaming")
        env["LOCALAPPDATA"] = str(home / "AppData" / "Local")
        (home / "AppData" / "Roaming").mkdir(parents=True, exist_ok=True)
        (home / "AppData" / "Local").mkdir(parents=True, exist_ok=True)
    return env


def _split_runtime_marker(marker):
    """Never embed a complete proof marker in tool arguments."""
    half = len(marker) // 2
    return marker[:half], marker[half:]


def _sh_print_marker(marker):
    left, right = _split_runtime_marker(marker)
    return f"printf '%s%s' '{left}' '{right}'"


def _ps_print_marker(marker):
    left, right = _split_runtime_marker(marker)
    return f"Write-Output ('{left}' + '{right}')"


def windows_shell_command():
    """PowerShell checks name presence only, never reads a canary value."""
    return ("Start-Sleep -Seconds 7; "
            f"if (Test-Path Env:{ENV_NAME}) {{ {_ps_print_marker(SHELL_ENV_PRESENT)} }}; "
            f"if (Test-Path Env:{FOREIGN_ENV_NAME}) {{ {_ps_print_marker(SHELL_FOREIGN_PRESENT)} }}; "
            f"{_ps_print_marker(SHELL_MARKER)}")

class Model(http.server.BaseHTTPRequestHandler):
    shell_only = False
    requests = 0
    shell_requested = False
    mcp_requested = False
    search_requested = False
    mcp_output_seen = False
    mcp_digest_verified = False
    mcp_output_indicators = []
    mcp_output_type = "not_observed"
    expected_digest = None
    shell_output_seen = False
    shell_execution_verified = False
    shell_secret_visible = False
    shell_foreign_visible = False
    shell_response_category = "not_observed"
    shell_response_indicators = []
    shell_win32_failure_category = "not_reported"
    shell_requested_at = 0.0
    error = ""
    advertised_tool_names = set()

    def log_message(self, *_args):
        pass

    def do_POST(self):
        if self.path != "/v1/responses":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", 0))
        if length <= 0 or length > 1 << 20:
            self.send_error(413)
            return
        try:
            req = json.loads(self.rfile.read(length))
        except ValueError:
            self.send_error(400)
            return
        cls = type(self)
        cls.requests += 1
        outputs = [x for x in req.get("input", []) if isinstance(x, dict)]
        for response in outputs:
            if response.get("type") == "function_call_output" and response.get("call_id") == SHELL_CALL:
                cls.shell_output_seen = True
                # A native tool returning an error is not proof the command ran.
                # Require an execution-only sentinel AND elapsed sleep time.
                rendered = json.dumps(response.get("output"), sort_keys=True)
                # Native tool errors may echo the command arguments, including
                # our fixed presence markers. Only a *verified completed*
                # seven-second command can prove observed shell environment.
                # Missing shell proof is independently a hard FAIL, never PASS.
                # A delayed native tool error can echo all fixed markers after
                # the wait window; never promote that response to a shell proof.
                # The fixed command produces no failure text on real completion.
                lower = rendered.lower()
                explicit_tool_failure = any(term in lower for term in (
                    "failed", "error", "denied", "not supported",
                    "operation not permitted", "timed out",
                ))
                if (SHELL_MARKER in rendered and
                        time.monotonic() - cls.shell_requested_at >= 6.0 and
                        not explicit_tool_failure):
                    cls.shell_execution_verified = True
                    cls.shell_secret_visible |= SHELL_ENV_PRESENT in rendered
                    cls.shell_foreign_visible |= SHELL_FOREIGN_PRESENT in rendered
                    cls.shell_response_category = "completed"
                else:
                    # Only fixed categories leave this synthetic-only harness;
                    # never persist raw native tool responses or environment.
                    # Bounded, fixed-vocabulary diagnostics. Never persist the
                    # native response, command output, paths or synthetic value.
                    indicators = (
                        "sandbox", "permission denied", "operation not permitted",
                        "read-only", "landlock", "bwrap", "seccomp", "namespace",
                        "not supported", "not found", "invalid", "unrecognized",
                        "failed", "error", "denied", "session id", "session_id",
                        "running", "yield_time_ms", "tool_call", "exec_command",
                        "expected", "arguments", "json", "timeout", "exited",
                        "createprocess", "processfailed", "sandboxdenied",
                        "failed to create unified exec process", "invalidcwd",
                        "exec_server", "shell_snapshot", "access is denied",
                        "failed to spawn", "file not found", "path is not valid",
                    )
                    cls.shell_response_indicators = [
                        word for word in indicators if word in lower
                    ]
                    cls.shell_win32_failure_category = fixed_windows_spawn_error_category(rendered)
                    cls.shell_response_category = (
                        "sandbox_error" if "sandbox" in lower else
                        "permission_error" if "permission" in lower else
                        "tool_error" if "error" in lower or "failed" in lower else
                        "completion_unverified"
                    )
        for response in outputs:
            if (response.get("type") == "function_call_output"
                    and response.get("call_id") == MCP_CALL):
                cls.mcp_output_seen = True
                # SHA256 of a fresh synthetic-only canary, never real credentials.
                # Correlate the ACTUAL native tool result rather than relying
                # on a sandbox-blocked child sidecar receipt.
                output = response.get("output")
                rendered = json.dumps(output, sort_keys=True)
                cls.mcp_output_type = (
                    "text" if isinstance(output, str) else
                    "object" if isinstance(output, dict) else
                    "array" if isinstance(output, list) else
                    "other"
                )
                # Fixed vocabulary only: never record the child's output,
                # synthetic canary, digest string, process paths or argv.
                cls.mcp_output_indicators = [
                    term for term in (
                        "sha256", "observed", "unknown", "not found", "unsupported",
                        "not registered", "unavailable", "not connected", "failed",
                        "error", "permission denied", "denied", "namespace",
                        "server", "mcp", "tool", "timeout", "call", "result",
                    ) if term in rendered.lower()
                ]
                if structured_mcp_receipt_matches(output, cls.expected_digest):
                    cls.mcp_digest_verified = True
        tools = req.get("tools", [])
        function_names = {
            name for x in tools if isinstance(x, dict)
            for name in (x.get("name"),) if isinstance(name, str)
        }
        # The raw names are only used for in-memory routing during this
        # request. Reports never include names outside the reviewed allowlist.
        cls.advertised_tool_names.update(
            name for name in function_names if name in KNOWN_NATIVE_TOOL_NAMES
        )
        namespace = next((x for x in tools if isinstance(x, dict)
                          and x.get("type") == "namespace"
                          and x.get("name") == "mcp__env_spike"), None)
        # Versions using deferred tool discovery expose the namespace in input.
        if namespace is None:
            for output in outputs:
                if output.get("type") == "tool_search_output":
                    namespace = next((t for t in output.get("tools", [])
                                      if isinstance(t, dict)
                                      and t.get("name") == "mcp__env_spike"), None)
                    if namespace is not None:
                        break
        if cls.requests > 10:
            cls.error = "model request budget"
            self.send_error(409)
            return
        if not cls.shell_requested:
            shell = "exec_command" if "exec_command" in function_names else (
                "shell_command" if "shell_command" in function_names else None
            )
            if shell is None:
                cls.error = "no advertised shell tool"
                self.send_error(409)
                return
            if cls.shell_only:
                # Genuinely credential-name-free Windows/Unix control:
                # do not even mention TOKEN/PASSWORD env names in tool args.
                command = (
                    "Start-Sleep -Seconds 7; " + _ps_print_marker(SHELL_MARKER)
                    if os.name == "nt" else
                    "sleep 7; " + _sh_print_marker(SHELL_MARKER)
                )
            else:
                command = ("sleep 7; "
                           f'if test -n "${{{ENV_NAME}+x}}"; then {_sh_print_marker(SHELL_ENV_PRESENT)}; fi; '
                           f'if test -n "${{{FOREIGN_ENV_NAME}+x}}"; then {_sh_print_marker(SHELL_FOREIGN_PRESENT)}; fi; '
                           f'{_sh_print_marker(SHELL_MARKER)}')
                if os.name == "nt":
                    command = windows_shell_command()
            # Codex exec_command can yield before a long command has finished.
            # Request a full bounded wait so the completion marker is observed
            # from the real native shell rather than mistaking an async
            # session-creation response for command execution.
            args = ({"cmd": command, "yield_time_ms": 10000} if shell == "exec_command"
                    else {"command": command})
            item = {"type": "function_call", "name": shell,
                    "call_id": SHELL_CALL, "arguments": json.dumps(args)}
            cls.shell_requested = True
            cls.shell_requested_at = time.monotonic()
        elif not cls.shell_output_seen:
            cls.error = "shell tool output missing"
            self.send_error(409)
            return
        elif cls.shell_only and cls.shell_output_seen:
            # No-secret host diagnostic only. The calling harness MUST check
            # shell_execution_verified; returning a model message is not PASS.
            item = {"type": "message", "id": "msg_uap_shell_only",
                    "role": "assistant",
                    "content": [{"type": "output_text", "text": "Probe complete."}]}
        elif cls.mcp_output_seen:
            item = {"type": "message", "id": "msg_uap_spike", "role": "assistant",
                    "content": [{"type": "output_text", "text": "Proof complete."}]}
        elif namespace and any(x.get("name") == "confirm_env"
                               for x in namespace.get("tools", [])):
            item = {"type": "function_call", "namespace": "mcp__env_spike",
                    "name": "confirm_env", "call_id": MCP_CALL, "arguments": "{}"}
            cls.mcp_requested = True
        elif "mcp__env_spike__confirm_env" in function_names:
            item = {"type": "function_call", "name": "mcp__env_spike__confirm_env",
                    "call_id": MCP_CALL, "arguments": "{}"}
            cls.mcp_requested = True
        elif not cls.search_requested and "tool_search" in function_names:
            item = {"type": "tool_search_call", "call_id": SEARCH_CALL,
                    "execution": "client",
                    "arguments": {"query": "env_spike confirm_env", "limit": 8}}
            cls.search_requested = True
        else:
            cls.error = "MCP tool not advertised"
            self.send_error(409)
            return
        events = [
            ("response.created", {"type": "response.created",
                                  "response": {"id": "resp_uap_spike"}}),
            ("response.output_item.done", {"type": "response.output_item.done",
                                           "item": item}),
            ("response.completed", {"type": "response.completed",
                                    "response": {"id": "resp_uap_spike",
                                                 "usage": {"input_tokens": 1,
                                                           "output_tokens": 1,
                                                           "total_tokens": 2}}}),
        ]
        body = "".join(f"event: {event}\ndata: {json.dumps(payload)}\n\n"
                       for event, payload in events).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

def _scan_stat_identity(info, include_change_time=True):
    """Path-vs-handle identity: Windows stat ctime has incompatible semantics."""
    shared = (info.st_dev, info.st_ino, info.st_size, info.st_mtime_ns)
    return shared + ((info.st_ctime_ns,) if include_change_time else ())


# Capture capability before unit tests mock os.open. Without dir_fd and
# O_NOFOLLOW on POSIX, refuse to inspect (never fall back to path-only reads).
_SCAN_DIRFD_SUPPORTED = os.open in os.supports_dir_fd


def _capture_scan_root(root):
    """Pin immutable lexical root + filesystem identity at fixture creation."""
    path = Path(root)
    observed = path.lstat()
    if (not stat.S_ISDIR(observed.st_mode) or observed.st_ino == 0
            or path.is_symlink()
            or (hasattr(path, "is_junction") and path.is_junction())):
        raise OSError(errno.EPERM, "untrusted_disposable_scan_root")
    return (str(path), observed.st_dev, observed.st_ino)


def _scan_root_is_original(root, pinned):
    """Reject junction, symlink, directory replacement and renamed roots."""
    try:
        if str(Path(root)) != pinned[0]:
            return False
        return _capture_scan_root(root) == pinned
    except (OSError, ValueError):
        return False


def _root_changed_finding():
    return {"path_sha256": hashlib.sha256(b".").hexdigest(),
            "result": "unscanned_directory", "io_reason": "root_identity_changed",
            "scope": "other"}


def _windows_path_within_root(opened_path, root_path):
    """Accept only fully qualified local-drive paths beneath the isolated root."""
    def canonical(value):
        if value.startswith("\\\\?\\") and not value.startswith("\\\\?\\UNC\\"):
            value = value[4:]
        return ntpath.normcase(ntpath.normpath(value))
    opened = canonical(opened_path)
    root = canonical(root_path)
    # UNC and device/volume GUID forms are deliberately unsupported: fail closed.
    if (opened.startswith("\\\\") or root.startswith("\\\\")
            or not ntpath.isabs(opened) or not ntpath.isabs(root)):
        return False
    try:
        return ntpath.commonpath((root, opened)) == root
    except ValueError:
        return False


def _windows_fd_within_root(fd, root):
    """Check the resolved *opened handle* before reading any bytes on Windows."""
    import ctypes
    import msvcrt
    from ctypes import wintypes
    get_path = ctypes.WinDLL("kernel32", use_last_error=True).GetFinalPathNameByHandleW
    get_path.argtypes = (wintypes.HANDLE, wintypes.LPWSTR,
                         wintypes.DWORD, wintypes.DWORD)
    get_path.restype = wintypes.DWORD
    handle = msvcrt.get_osfhandle(fd)
    length = get_path(handle, None, 0, 0)
    if not 0 < length <= 32768:
        raise OSError(errno.EIO, "unverifiable_opened_file_path")
    buffer = ctypes.create_unicode_buffer(length + 1)
    written = get_path(handle, buffer, len(buffer), 0)
    if (not 0 < written < len(buffer)
            or not _windows_path_within_root(
                buffer.value, str(root))):
        raise OSError(errno.EPERM, "file_not_proven_inside_root")


def _read_regular_bounded(path, observed, root, root_anchor):
    """Read only from a verified descendant with stable file identity.

    POSIX pins each parent with O_DIRECTORY|O_NOFOLLOW and opens relative
    to that handle; Windows verifies the kernel-resolved opened file handle
    before reading. No unsafe fallback if verification is unsupported.
    """
    limit = 32 * 1024 * 1024
    flags = (os.O_RDONLY | getattr(os, "O_BINARY", 0)
             | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_NOFOLLOW", 0))
    # A previously trusted temporary root cannot be swapped to a Windows
    # junction or an ordinary new directory and silently re-anchored.
    if not _scan_root_is_original(root, root_anchor):
        raise OSError(errno.EPERM, "changed_disposable_scan_root")
    with contextlib.ExitStack() as descriptors:
        if os.name == "nt":
            fd = os.open(path, flags)
            try:
                _windows_fd_within_root(fd, root_anchor[0])
                if not _scan_root_is_original(root, root_anchor):
                    raise OSError(errno.EPERM, "changed_disposable_scan_root")
            except BaseException:
                os.close(fd)
                raise
        else:
            if (not _SCAN_DIRFD_SUPPORTED or not getattr(os, "O_DIRECTORY", 0)
                    or not getattr(os, "O_NOFOLLOW", 0)):
                raise OSError(errno.ENOSYS, "secure_directory_open_unavailable")
            try:
                relative = Path(path).relative_to(Path(root))
            except ValueError:
                raise OSError(errno.EPERM, "file_outside_scan_root") from None
            if (not relative.parts
                    or any(part in (".", "..") for part in relative.parts)):
                raise OSError(errno.EPERM, "invalid_scan_relative_path")
            dir_flags = (os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
                         | getattr(os, "O_CLOEXEC", 0))
            directory_fd = os.open(root, dir_flags)
            descriptors.callback(os.close, directory_fd)
            for component in relative.parts[:-1]:
                opened_dir = os.open(component, dir_flags, dir_fd=directory_fd)
                descriptors.callback(os.close, opened_dir)
                directory_fd = opened_dir
            fd = os.open(relative.parts[-1], flags, dir_fd=directory_fd)
            if not _scan_root_is_original(root, root_anchor):
                os.close(fd)
                raise OSError(errno.EPERM, "changed_disposable_scan_root")
        with os.fdopen(fd, "rb") as reader:
            opened = os.fstat(reader.fileno())
            # Windows path stat reports creation ctime; handle stat reports
            # change ctime. Still keep full fstat-to-fstat before/after proof.
            def path_identity(info):
                return _scan_stat_identity(
                    info, include_change_time=os.name != "nt"
                )
            if (not stat.S_ISREG(opened.st_mode) or opened.st_ino == 0
                    or path_identity(opened) != path_identity(observed)
                    or opened.st_size > limit):
                raise OSError(errno.EIO, "unstable_native_scan")
            data = reader.read(limit + 1)
            after = os.fstat(reader.fileno())
            if (len(data) != opened.st_size
                    or _scan_stat_identity(after) != _scan_stat_identity(opened)
                    or not _scan_root_is_original(root, root_anchor)):
                raise OSError(errno.EIO, "unstable_native_scan")
            return data


def inspect(root, secret, root_anchor=None):
    if root_anchor is None:
        try:
            root_anchor = _capture_scan_root(root)
        except (OSError, ValueError):
            return ([_root_changed_finding()], 0)
    if not _scan_root_is_original(root, root_anchor):
        return ([_root_changed_finding()], 0)
    values = (secret,) if isinstance(secret, str) else secret
    patterns = []
    for value in values:
        raw = value.encode()
        # Common encodings across shell/snapshot formats; no canary bytes leave
        # the isolated scan. Hex can use either case; UTF-16BE may be Base64.
        patterns.extend([raw, raw.hex().encode(), raw.hex().upper().encode(),
                         base64.b64encode(raw), base64.urlsafe_b64encode(raw),
                         urllib.parse.quote(value).encode(),
                         value.encode("utf-16le"), value.encode("utf-16be"),
                         base64.b64encode(value.encode("utf-16le")),
                         base64.b64encode(value.encode("utf-16be"))])
    found, symlinks = [], 0
    root_path = Path(root).resolve(strict=True)

    def bounded_io_reason(exc):
        # Fixed categories only: never serialize file paths or OS messages.
        winerror = getattr(exc, "winerror", None)
        if winerror in (32, 33):
            return "sharing_violation"
        if winerror == 5 or getattr(exc, "errno", None) in (errno.EACCES, errno.EPERM):
            return "access_denied"
        if winerror in (2, 3) or getattr(exc, "errno", None) == errno.ENOENT:
            return "vanished_during_walk"
        return "other_io_error"

    def bounded_scan_scope(path):
        try:
            # Use the same traversal spelling as os.walk and path hashes.
            # On Windows resolve() can canonicalize the temporary root into
            # a different drive/alias representation; it is only needed for
            # resolving external symlink targets, not fixed scope labels.
            parts = Path(path).relative_to(Path(root)).parts
        except (TypeError, ValueError, OSError):
            return "unknown"
        if parts[:2] == ("home", ".codex"):
            return "codex_home"
        if parts[:1] == ("project",):
            return "project"
        if parts[:1] == ("tmp",):
            return "temporary"
        return "other"

    def record_symlink(path):
        nonlocal symlinks
        symlinks += 1
        try:
            outside = not path.resolve(strict=False).is_relative_to(root_path)
        except (OSError, RuntimeError, ValueError):
            outside = True
        if outside:
            # An outside or unresolvable link cannot be proved safe by our
            # no-follow scanner. Fail closed; never read or log its target.
            found.append({
                "path_sha256": hashlib.sha256(
                    str(path.relative_to(root)).encode()).hexdigest(),
                "result": "unscanned_external_symlink",
            })

    def record_walk_error(exc):
        # os.walk otherwise silently drops unreadable directories. An omitted
        # subtree invalidates a no-leak proof even if all scanned files pass.
        # Record only a path digest and fixed category, never errno text/paths.
        try:
            failed = Path(exc.filename) if exc.filename else root_path
            # Match the original lexical os.walk root, not the resolved
            # Windows drive/alias spelling (which can collapse path hashes).
            relative = failed.relative_to(Path(root))
        except (TypeError, ValueError, OSError):
            relative = Path(".")
        found.append({
            "path_sha256": hashlib.sha256(str(relative).encode()).hexdigest(),
            "result": "unscanned_directory",
            "io_reason": bounded_io_reason(exc),
            "scope": bounded_scan_scope(failed),
        })

    # Older Windows Python lacks junction detection. Missing proof is FAIL.
    if os.name == "nt" and not hasattr(Path, "is_junction"):
        return ([{"path_sha256": hashlib.sha256(b".").hexdigest(),
                  "result": "unscanned_junction_capability"}], 0)

    def is_link_or_junction(path):
        # Windows junctions are not necessarily symlinks. Prune before walk.
        return path.is_symlink() or (
            path.is_junction() if hasattr(path, "is_junction") else False
        )

    for directory, subdirs, files in os.walk(
            root, followlinks=False, onerror=record_walk_error):
        for name in list(subdirs):
            path = Path(directory) / name
            if is_link_or_junction(path):
                record_symlink(path)
                subdirs.remove(name)
        for name in files:
            path = Path(directory) / name
            if is_link_or_junction(path):
                record_symlink(path)
                continue
            try:
                metadata = path.lstat()
                # A FIFO/device/socket can block indefinitely or be unreadable.
                # Never open nonregular filesystem objects; lack of proof is FAIL.
                if not stat.S_ISREG(metadata.st_mode):
                    found.append({
                        "path_sha256": hashlib.sha256(
                            str(path.relative_to(root)).encode()).hexdigest(),
                        "result": "unscanned_special_file",
                    })
                    continue
                if metadata.st_size > 32 * 1024 * 1024:
                    found.append({"path_sha256": hashlib.sha256(str(path.relative_to(root)).encode()).hexdigest(),
                                  "result": "unscanned_oversize"})
                    continue
                data = _read_regular_bounded(path, metadata, root, root_anchor)
            except OSError as exc:
                # Vanished and locked files both leave incomplete evidence.
                # Never treat a file disappearing between lstat/read as proof
                # that its transient content was safe; never exempt SQLite -shm.
                # Expose only fixed diagnostics, not messages or pathnames.
                found.append({
                    "path_sha256": hashlib.sha256(
                        str(path.relative_to(root)).encode()).hexdigest(),
                    "result": "unscanned_file",
                    "io_reason": bounded_io_reason(exc),
                    "scope": bounded_scan_scope(path),
                    "sqlite_shm_candidate": path.name.lower().endswith(".sqlite-shm"),
                })
                continue
            if any(p and p in data for p in patterns):
                found.append({"path_sha256": hashlib.sha256(str(path.relative_to(root)).encode()).hexdigest(),
                              "result": "synthetic_value_persisted",
                              "shell_snapshot_path": "shell_snapshots" in path.parts})
    if not _scan_root_is_original(root, root_anchor):
        found.append(_root_changed_finding())
    return found, symlinks

def aggregate_findings(*batches):
    """Union observations by path AND finding category, preserving fail-closed proof.

    Live and post-exit reports may describe the same path differently as it
    changes. Never replace a canary finding with an unsafe-symlink finding (or
    vice versa). Counts are untruncated; only presentation samples are bounded.
    """
    by_category = {}
    for batch in batches:
        for finding in batch:
            key = (finding["path_sha256"], finding["result"])
            if key not in by_category:
                by_category[key] = dict(finding)
            elif finding.get("shell_snapshot_path"):
                by_category[key]["shell_snapshot_path"] = True
    all_findings = [by_category[key] for key in sorted(by_category)]
    counts = {}
    for finding in all_findings:
        category = finding["result"]
        counts[category] = counts.get(category, 0) + 1
    return all_findings, counts

def main():
    codex = Path(sys.argv[1]).resolve(strict=True)
    report_path = Path(sys.argv[2]).resolve()
    report_path.parent.mkdir(parents=True, exist_ok=True)
    mode = sys.argv[3] if len(sys.argv) > 3 else "default"
    if mode not in ("default", "filtered", "default_excludes", "core_inherit"):
        raise ValueError("unsupported native shell policy control mode")
    if len(sys.argv) > 4:
        raise ValueError("unexpected native spike parameter")
    secret = "uap_spike_" + secrets.token_hex(24)
    foreign_secret = "uap_foreign_spike_" + secrets.token_hex(24)
    Model.expected_digest = hashlib.sha256(secret.encode()).hexdigest()
    root = isolated_root("uap-codex-spike-")
    root_anchor = _capture_scan_root(root)
    home, project = root / "home", root / "project"
    (home / ".codex").mkdir(parents=True)
    (project / ".codex").mkdir(parents=True)
    (project / ".git").mkdir()
    (root / "tmp").mkdir()
    child = root / "child.py"
    child.write_text(CHILD)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Model)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    config = (
        'cli_auth_credentials_store = "file"\n'
        'mcp_oauth_credentials_store = "file"\n'
        'model_provider = "fixture"\nmodel = "fixture-model"\n'
        '[model_providers.fixture]\nname = "Fixture"\n'
        f'base_url = "http://127.0.0.1:{server.server_port}/v1"\n'
        'wire_api = "responses"\nrequires_openai_auth = false\n'
        '[analytics]\nenabled = false\n[feedback]\nenabled = false\n'
        f'[projects.{json.dumps(str(project))}]\ntrust_level = "trusted"\n'
        f'[shell_environment_policy]\nexclude = ["{FOREIGN_ENV_NAME}"]\n'
    )
    if os.name == "nt":
        config += WINDOWS_NATIVE_SANDBOX_CONFIG
    (home / ".codex" / "config.toml").write_text(config)
    project_server = (
        '[mcp_servers.env_spike]\n'
        f'command = {json.dumps(sys.executable)}\n'
        f'args = [{json.dumps(str(child))}]\n'
        f'env_vars = ["{ENV_NAME}"]\n'
    )
    # Research-only per-project control. It tests whether a native Codex
    # shell policy could protect the shell while stdio MCP receives env_vars.
    # Never install this policy in real user files without foreign-policy proof.
    if mode == "filtered":
        project_server += (
            '[shell_environment_policy.filters]\n'
            f'{json.dumps(ENV_NAME)} = "exclude"\n'
        )
    # Research-only stricter native shell policy: requires a credential-like
    # variable NAME and deliberately does not replace lower-layer legacy
    # exclude/include_only arrays. Never install without scoped consent/proof.
    if mode == "default_excludes":
        project_server += (
            '[shell_environment_policy]\n'
            'ignore_default_excludes = false\n'
        )
    # Research-only scalar inheritance control. It preserves standard core
    # shell variables but prevents arbitrary parent environment forwarding to
    # shell tools; stdio MCP env_vars must continue to work independently.
    if mode == "core_inherit":
        project_server += (
            '[shell_environment_policy]\n'
            'inherit = "core"\n'
        )
    (project / ".codex" / "config.toml").write_text(project_server)
    env = isolated_env(root, home, secret, foreign_secret)
    commands = [
        str(codex), "exec", "--skip-git-repo-check",
        "--sandbox", "read-only", "Call the available shell tool once and then confirm_env."
    ]
    report = {"policy_mode": mode, "sandbox_backend": ("bwrap" if sys.platform.startswith("linux") else "native"), "version": "", "real_codex_shell_called": False,
              "real_codex_mcp_called": False, "mcp_child_saw_secret": False,
              "leak": False, "secrecy_pass": False, "failure": None}
    try:
        version = subprocess.run([str(codex), "--version"], env=env,
                                 capture_output=True, timeout=15, text=True)
        report["version"] = version.stdout.strip()[:100]
        # Observe LIVE named snapshots during the deliberately held shell tool.
        # Post-exit scans alone miss transient files deleted by normal cleanup.
        monitor_stop = threading.Event()
        live = {"samples": 0, "hits": {}, "symlinks": 0, "errors": 0,
                "snapshot_files_seen": False}
        def monitor():
            while not monitor_stop.is_set():
                try:
                    matches, links = inspect(
                        root, (secret, foreign_secret), root_anchor=root_anchor
                    )
                    live["samples"] += 1
                    live["symlinks"] = max(live["symlinks"], links)
                    for match in matches:
                        live["hits"][(match["path_sha256"], match["result"])] = match
                    snapshot_dir = home / ".codex" / "shell_snapshots"
                    if snapshot_dir.is_dir() and any(snapshot_dir.iterdir()):
                        live["snapshot_files_seen"] = True
                except OSError:
                    live["errors"] += 1
                monitor_stop.wait(0.035)
        observer = threading.Thread(target=monitor, daemon=True)
        observer.start()
        try:
            result = subprocess.run(commands, cwd=project, env=env,
                                    capture_output=True, timeout=115)
        finally:
            monitor_stop.set()
            observer.join(timeout=3)
            observer_stopped = not observer.is_alive()
        report["live_observer_stopped"] = observer_stopped
        report["codex_exit_code"] = result.returncode
        report["live_scan_samples"] = live["samples"]
        report["live_snapshot_files_seen"] = live["snapshot_files_seen"]
        report["live_scan_errors"] = live["errors"]
        report["shell_tool_output_seen"] = Model.shell_output_seen
        report["shell_secret_visible"] = Model.shell_secret_visible
        report["shell_foreign_visible"] = Model.shell_foreign_visible
        report["shell_response_category"] = Model.shell_response_category
        report["shell_response_indicators"] = Model.shell_response_indicators
        report["real_codex_shell_called"] = Model.shell_requested and Model.shell_execution_verified
        report["real_codex_mcp_called"] = Model.mcp_requested and Model.mcp_output_seen
        report["native_mcp_digest_seen"] = Model.mcp_digest_verified
        report["mcp_output_type"] = Model.mcp_output_type
        report["mcp_output_indicators"] = Model.mcp_output_indicators
        report["requests"] = Model.requests
        report["advertised_tool_names"] = sorted(Model.advertised_tool_names)[:35]
        report["mcp_child_saw_secret"] = Model.mcp_digest_verified
        found, symlinks = inspect(
            root, (secret, foreign_secret), root_anchor=root_anchor
        )
        # Never aggregate a live dictionary while the observer may still be
        # scanning; an incomplete join is itself a mandatory security FAIL.
        combined, counts = aggregate_findings(
            found, tuple(live["hits"].values()) if observer_stopped else ()
        )
        report["leak"] = bool(combined)  # legacy name: any unsafe finding
        report["canary_persistence_observed"] = counts.get("synthetic_value_persisted", 0) > 0
        report["unsafe_scan_observed"] = (not observer_stopped or any(
            category.startswith("unscanned_") and total > 0
            for category, total in counts.items()
        ))
        report["finding_counts"] = counts
        report["finding_total"] = len(combined)
        report["matches"] = combined[:10]
        report["symlinks"] = max(symlinks, live["symlinks"])
        if result.returncode != 0:
            report["failure"] = Model.error or "native_codex_exit_nonzero"
        elif not all([report["real_codex_shell_called"], report["real_codex_mcp_called"],
                      report["mcp_child_saw_secret"]]):
            report["failure"] = Model.error or "native_proof_incomplete"
        elif (not observer_stopped or combined or live["errors"]
              or live["samples"] < 20 or Model.shell_secret_visible
              or Model.shell_foreign_visible):
            report["failure"] = "live_canary_persisted_or_unsafe_scan"
        else:
            report["secrecy_pass"] = True
    except (OSError, subprocess.TimeoutExpired, ValueError) as exc:
        report["failure"] = type(exc).__name__
    finally:
        server.shutdown()
        report_path.write_text(json.dumps(report, indent=2, sort_keys=True))
        shutil.rmtree(root, ignore_errors=True)
    print(json.dumps({k: v for k, v in report.items() if k != "matches"}, sort_keys=True))
    return 0 if report["secrecy_pass"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
