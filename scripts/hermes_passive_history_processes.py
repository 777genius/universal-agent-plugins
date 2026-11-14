"""Owned inert children for the bundled passive-history experiment only.

Ledger fields never authorize a signal. POSIX children stay unreaped until all
signal decisions have ended; Windows operations retain the original handle.
"""
import base64
import hashlib
import json
import math
import os
import re
import signal
import stat
import subprocess
import threading
import time


SENTINEL = '''import json, os, sys
nonce = sys.argv[1]
def reply(phase):
    print(json.dumps(dict(nonce=nonce, pid=os.getpid(), parent=os.getppid(), phase=phase)), flush=True)
reply("ready")
for line in sys.stdin:
    phase = line.rstrip("\\n")
    if not phase.isascii() or len(phase) > 40: raise SystemExit(2)
    reply(phase)
'''
REGISTER = '''import json, os, sys
from pathlib import Path
import psutil
from hermes_cli import process_identity as identity
assert Path(identity.__file__).resolve() == Path(sys.argv[1]) / "hermes_cli/process_identity.py"
child = psutil.Process(int(sys.argv[2]))
own = psutil.Process()
assert child.ppid() == own.ppid() == int(sys.argv[3])
recorded = identity.register_child(child.pid, "mcp-helper")
print(json.dumps(dict(nonce=sys.argv[4], recorded=recorded, pid=own.pid, parent=own.ppid(),
    create_time=own.create_time(), child_pid=child.pid, child_create=child.create_time(),
    child_argv=" ".join(child.cmdline()[:10]), install=identity.install_id())), flush=True)
'''
SELECT = '''import json, os, sys
from pathlib import Path
from hermes_cli import process_identity as identity
assert Path(identity.__file__).resolve() == Path(sys.argv[1]) / "hermes_cli/process_identity.py"
recorded = []
selected = identity.reap_orphaned_mcp_helpers(kill_fn=recorded.append)
print(json.dumps(dict(nonce=sys.argv[2], pid=os.getpid(), recorded=recorded, selected=selected)), flush=True)
'''
STREAM_LIMIT = 64 << 10


def require(condition, message):
    if not condition:
        raise RuntimeError("passive history " + message)


def remaining(deadline):
    value = deadline - time.monotonic()
    require(math.isfinite(value) and value > 0, "original deadline expired")
    return value


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def document(body):
    require(type(body) is bytes and 0 < len(body) <= 1 << 20, "JSON bound")
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "duplicate JSON member")
            result[key] = value
        return result
    return json.loads(body, object_pairs_hook=unique,
                      parse_constant=lambda _: require(False, "nonfinite JSON"))


class PipeCapture:
    def __init__(self, stream, limit):
        self.stream, self.limit = stream, limit
        self.body, self.error, self.eof = bytearray(), None, False
        self.changed = threading.Condition()
        self.thread = threading.Thread(target=self.read, daemon=True)
        self.thread.start()

    def read(self):
        try:
            while True:
                part = os.read(self.stream.fileno(), min(1024, self.limit + 1 - len(self.body)))
                with self.changed:
                    if not part:
                        self.eof = True
                        self.changed.notify_all()
                        return
                    self.body.extend(part)
                    require(len(self.body) <= self.limit, "live child stream overflow")
                    self.changed.notify_all()
        except BaseException as error:
            with self.changed:
                self.error = error
                self.changed.notify_all()

    def line(self, offset, deadline):
        with self.changed:
            while True:
                if self.error is not None:
                    raise self.error
                end = self.body.find(b"\n", offset)
                if end >= 0:
                    return bytes(self.body[offset:end]), end + 1
                require(not self.eof, "child stream ended before response")
                self.changed.wait(min(.05, remaining(deadline)))

    def close(self, deadline):
        self.thread.join(max(0, deadline - time.monotonic()))
        require(not self.thread.is_alive(), "child pipe EOF unproved")
        self.stream.close()
        if self.error is not None:
            raise self.error
        require(self.eof, "child pipe closure unproved")
        return bytes(self.body)


def windows_state(child, original=None, *, identity=True):
    import ctypes
    from ctypes import wintypes
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    kernel.GetProcessId.argtypes, kernel.GetProcessId.restype = [wintypes.HANDLE], wintypes.DWORD
    kernel.WaitForSingleObject.argtypes = [wintypes.HANDLE, wintypes.DWORD]
    kernel.WaitForSingleObject.restype = wintypes.DWORD
    state = kernel.WaitForSingleObject(child._handle, 0)
    require(state in (0, 258), "original Windows wait failed")
    if not identity:
        return state == 0, None  # Constructor failure: the Popen handle is still owned.
    kernel.GetProcessTimes.argtypes = [wintypes.HANDLE] + [ctypes.POINTER(wintypes.FILETIME)] * 4
    kernel.GetProcessTimes.restype = wintypes.BOOL
    times = [wintypes.FILETIME() for _ in range(4)]
    require(kernel.GetProcessTimes(child._handle, *(ctypes.byref(item) for item in times)),
            "original Windows process times unavailable")
    identity = (kernel.GetProcessId(child._handle), times[0].dwHighDateTime, times[0].dwLowDateTime)
    require(identity[0] == child.pid and (original is None or identity == original),
            "original Windows handle identity differs")
    return state == 0, identity


class HeldChild:
    def __init__(self, argv, cwd, env, deadline):
        remaining(deadline - 10)
        self.deadline, self.argv, self.offset = deadline, list(argv), 0
        self.lost, self.closed, self.reaped, self.checks = False, False, False, []
        self.windows, self.out, self.err = None, None, None
        self.child = subprocess.Popen(argv, cwd=cwd, env=dict(env), stdin=subprocess.PIPE,
                                      stdout=subprocess.PIPE, stderr=subprocess.PIPE, close_fds=True)
        try:
            self.windows = windows_state(self.child)[1] if os.name == "nt" else None
            self.out = PipeCapture(self.child.stdout, 8192)
            self.err = PipeCapture(self.child.stderr, 4096)
        except BaseException as error:
            try:
                self.close()
            except BaseException as cleanup:
                raise RuntimeError("passive history poststart cleanup incomplete") from cleanup
            raise error

    def dead(self, *, cleanup=False):
        require(not self.reaped and not self.lost, "original child wait authority lost")
        remaining(self.deadline if cleanup else self.deadline - 10)
        if os.name == "nt":
            return windows_state(self.child, self.windows, identity=self.windows is not None)[0]
        try:
            status = os.waitid(os.P_PID, self.child.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
        except ChildProcessError:
            self.lost = True
            raise RuntimeError("passive history original child wait authority lost") from None
        return status is not None

    def wait_dead(self, until=None, *, cleanup=False):
        bound = self.deadline if cleanup else self.deadline - 10
        until = bound if until is None else min(until, bound)
        while not self.dead(cleanup=cleanup):
            if time.monotonic() >= until:
                return False
            time.sleep(min(.01, remaining(bound)))
        return True

    def response(self):
        body, self.offset = self.out.line(self.offset, self.deadline - 10)
        require(not self.err.body and self.err.error is None, "child stderr is not empty")
        return document(body)

    def challenge(self, nonce, phase):
        require(not self.dead(), "original sentinel died before challenge")
        require(type(phase) is str and phase.isascii() and 0 < len(phase) <= 40, "challenge shape")
        if phase != "ready":
            self.child.stdin.write((phase + "\n").encode("ascii"))
            self.child.stdin.flush()
        value = self.response()
        require(value == dict(nonce=nonce, pid=self.child.pid, parent=os.getpid(), phase=phase)
                and type(value["pid"]) is int and type(value["parent"]) is int and not self.dead(),
                "original nonce-responsive sibling differs")
        self.checks.append(value)
        return value

    def signal(self, force=False):
        require(not self.dead(cleanup=True), "signal after original child exit")
        if os.name == "nt":
            self.child.kill()  # Popen uses its retained original process handle.
        else:
            os.kill(self.child.pid, signal.SIGKILL if force else signal.SIGTERM)

    def close(self):
        require(not self.closed, "child already closed")
        if not self.reaped and not self.dead(cleanup=True):
            self.signal()
            if not self.wait_dead(time.monotonic() + .5, cleanup=True):
                self.signal(force=True)
        if not self.reaped:
            require(self.wait_dead(cleanup=True), "original child death unproved")
            self.child.wait(timeout=remaining(self.deadline))  # Sole final reap; no later signal.
            self.reaped = True
        streams, errors = {}, []
        for name, capture in (("stdout", self.out), ("stderr", self.err)):
            try:
                if capture is not None:
                    streams[name] = capture.close(self.deadline)
                else:
                    getattr(self.child, name).close()
            except BaseException as error:
                errors.append(error)
        try:
            self.child.stdin.close()
        except BaseException as error:
            errors.append(error)
        if os.name == "nt" and not self.child._handle.closed:
            self.child._handle.Close()
        require(not errors, "original pipe closure incomplete")
        remaining(self.deadline)
        self.closed = True
        return dict(pid=self.child.pid, exit=self.child.returncode, stdout=encoded(streams.get("stdout", b"")),
                    stderr=encoded(streams.get("stderr", b"")),
                    waited=True, pipes_closed=True, original_handle=True)


def encoded(body):
    require(type(body) is bytes and len(body) <= STREAM_LIMIT, "retained stream bound")
    return dict(bytes=len(body), sha256=hashlib.sha256(body).hexdigest(),
                base64=base64.b64encode(body).decode("ascii"))


def decoded(value):
    require(type(value) is dict and set(value) == {"bytes", "sha256", "base64"}
            and type(value["bytes"]) is int and 0 <= value["bytes"] <= STREAM_LIMIT
            and type(value["base64"]) is str and len(value["base64"]) <= 4 * ((STREAM_LIMIT + 2) // 3),
            "closed stream schema")
    body = base64.b64decode(value["base64"], validate=True)
    require(encoded(body) == value, "retained stream bytes differ")
    return body


def capture_command(api, argv, cwd, env, evidence, label, deadline, *, refused=False, lineage=False):
    budget = remaining(deadline) - 20
    require(budget > 0, "command would consume cleanup reserve")
    api.public_runtime_pins(rehash=False, expected=api.pins)
    result = api.bounded(argv, cwd, env, evidence / label, timeout=min(90, budget), expected_failure=refused,
                         **({"process_identity": api.public_windows_process_identity} if lineage else {}))
    authority = api.evidence_identity
    require(api.directory_identity(evidence) == authority, "original command evidence directory replaced")
    with api.held_census_evidence(evidence, authority) as (descriptor, verify):
        files, bodies = [], {}
        for suffix in ("status.json", "stdout", "stderr"):
            remaining(deadline); verify()
            path = evidence / (label + "." + suffix)
            info = path.lstat()
            body = api.ordinary_bytes(path, info, parent=None if os.name == "nt" else descriptor,
                                      maximum=16 << 10 if suffix == "status.json" else STREAM_LIMIT, deadline=deadline)
            files.append((path, info, body)); bodies[suffix] = body
            verify()
        for path, info, _ in files:
            require(api.stable_file(path.lstat()) == api.stable_file(info), "original command capture replaced")
        verify()
    status = document(bodies["status.json"])
    value = dict(label=label, status=status, status_raw=encoded(bodies["status.json"]),
                 stdout=encoded(bodies["stdout"]), stderr=encoded(bodies["stderr"]))
    validate_command(value, argv, refused=refused, lineage=lineage)
    require(result == bodies["stdout"], "command stdout return differs")
    remaining(deadline)
    return value, (authority, tuple(files))


def validate_command(value, argv=None, *, refused=False, lineage=False):
    require(type(value) is dict and set(value) == {"label", "status", "status_raw", "stdout", "stderr"}, "closed command schema")
    status = value["status"]
    raw = decoded(value["status_raw"])
    require(len(raw) <= 16 << 10 and document(raw) == status, "original status bytes differ")
    fields = {"argv", "pid", "exit", "error", "streams"} | ({"process_identity"} if lineage else set())
    require(type(status) is dict and set(status) == fields and status["error"] is None
            and type(status["pid"]) is int and 0 < status["pid"] < 2**32
            and type(status["exit"]) is int and (status["exit"] != 0 if refused else status["exit"] == 0)
            and type(status["argv"]) is list and all(type(arg) is str and "\0" not in arg for arg in status["argv"])
            and (argv is None or status["argv"] == argv)
            and type(status["streams"]) is dict and set(status["streams"]) == {"stdout", "stderr"},
            "original command status differs")
    for name in ("stdout", "stderr"):
        body = decoded(value[name])
        stream = status["streams"][name]
        require(type(stream) is dict and set(stream) == {"bytes", "sha256"} and type(stream["bytes"]) is int
                and stream == dict(bytes=len(body), sha256=hashlib.sha256(body).hexdigest()),
                "original command stream join differs")
    return value


def rejoin_commands(api, captures, evidence, deadline):
    if not captures:
        return
    all_files = []
    for authority, files in captures.values():
        require(authority == api.evidence_identity, "command evidence authority changed")
        all_files.extend(files)
    with api.held_census_evidence(evidence, api.evidence_identity) as (descriptor, verify):
        for path, info, body in all_files:
            remaining(deadline); verify()
            require(api.stable_file(path.lstat()) == api.stable_file(info)
                    and api.ordinary_bytes(path, info, parent=None if os.name == "nt" else descriptor,
                                           maximum=len(body), deadline=deadline) == body,
                    "original command receipt changed")
        for path, info, _ in all_files:
            remaining(deadline)
            require(api.stable_file(path.lstat()) == api.stable_file(info),
                    "original command changed during receipt rejoin")
        verify()


def open_ledger(path):
    if os.name != "nt":
        return os.open(path, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK)
    import ctypes
    from ctypes import wintypes
    import msvcrt
    kernel = ctypes.WinDLL("kernel32", use_last_error=True)
    create, close = kernel.CreateFileW, kernel.CloseHandle
    create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p,
                       wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
    create.restype = wintypes.HANDLE
    close.argtypes, close.restype = [wintypes.HANDLE], wintypes.BOOL
    handle = create(str(path), 0xC0000000, 7, None, 3, 0x00200000, None)
    if handle is None or handle == ctypes.c_void_p(-1).value:
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        return msvcrt.open_osfhandle(handle, os.O_RDWR | os.O_BINARY)
    except BaseException:
        close(handle)
        raise


class LedgerHandle:
    def __init__(self, api, path, original, deadline):
        self.api, self.path, self.deadline = api, path, deadline
        self.parent_named = path.parent.lstat()
        self.parent = api.open_inventory_directory(path.parent)
        self.fd = None
        try:
            self.parent_held = os.fstat(self.parent)
            api.check_inventory_directory(self.parent_named, self.parent_held, "authored ledger parent open differs")
            self.named = path.lstat()
            require(stat.S_ISREG(self.named.st_mode) and self.named.st_nlink == 1, "ordinary authored ledger required")
            remaining(deadline)
            self.fd = open_ledger(path)
            self.held = os.fstat(self.fd)
            require(api.same_opened_capture(self.named, self.held), "authored ledger open identity differs")
            self.body = original
            self.verify()
        except BaseException:
            self.close()
            raise

    def verify(self):
        remaining(self.deadline)
        self.api.verify_inventory_directory(self.path.parent, self.parent_named, self.parent, self.parent_held)
        require(self.api.stable_file(self.path.lstat()) == self.api.stable_file(self.named)
                and self.api.stable_file(os.fstat(self.fd)) == self.api.stable_file(self.held), "authored ledger replaced")
        os.lseek(self.fd, 0, os.SEEK_SET)
        require(os.read(self.fd, len(self.body) + 1) == self.body, "authored ledger original bytes differ")
        require(self.api.stable_file(self.path.lstat()) == self.api.stable_file(self.named)
                and self.api.stable_file(os.fstat(self.fd)) == self.api.stable_file(self.held), "authored ledger changed during read")
        remaining(self.deadline)

    def purpose(self, purpose):
        self.verify()
        original_named, original_held = self.named, self.held
        value = document(self.body)
        require(type(value) is list and len(value) == 1 and purpose in {"cli", "mcp-helper"}, "authored single-purpose mutation")
        value[0]["purpose"] = purpose
        body = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
        require(0 < len(body) <= 4096, "authored ledger write bound")
        os.lseek(self.fd, 0, os.SEEK_SET)
        require(os.write(self.fd, body) == len(body), "authored ledger short write")
        os.ftruncate(self.fd, len(body))
        os.fsync(self.fd)
        self.named, self.held, self.body = self.path.lstat(), os.fstat(self.fd), body
        require(physical(self.named) == physical(original_named) and physical(self.held) == physical(original_held)
                and self.api.same_opened_capture(self.named, self.held), "authored ledger write identity differs")
        self.verify()
        return body

    def close(self):
        errors = []
        if self.fd is not None:
            descriptor = self.fd
            self.fd = None
            try:
                os.close(descriptor)
            except BaseException as error:
                errors.append(error)
        if self.parent is not None:
            descriptor = self.parent
            self.parent = None
            try:
                os.close(descriptor)
            except BaseException as error:
                errors.append(error)
        require(not errors, "authored ledger handle closure incomplete")


def physical(info):
    keys = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid")
    if os.name == "nt":
        keys += ("st_birthtime_ns", "st_file_attributes", "st_reparse_tag")
    return tuple(getattr(info, key) for key in keys)


def validate_forged(rows, registration, argv, nonce, install, sentinel, registrar, controller):
    fields = set("pid create_time purpose install spawner_pid spawner_create registered_at argv host port profile hermes_home isolated boot_start spawner_boot boot_id".split())
    require(type(rows) is list and len(rows) == 1 and type(rows[0]) is dict and set(rows[0]) == fields,
            "forged ledger closed schema differs")
    expected = set("nonce recorded pid parent create_time child_pid child_create child_argv install".split())
    require(type(registration) is dict and set(registration) == expected and registration["recorded"] is True
            and registration["nonce"] == nonce and registration["pid"] == registrar
            and registration["parent"] == controller and registration["child_pid"] == sentinel,
            "original registration process relation differs")
    require(all(type(pid) is int and 0 < pid < 2**32 for pid in (sentinel, registrar, controller))
            and len({sentinel, registrar, controller}) == 3
            and all(type(registration[key]) is int for key in ("pid", "parent", "child_pid")), "sibling PID types differ")
    row = rows[0]
    require(type(row["pid"]) is int and row["pid"] == sentinel and type(row["spawner_pid"]) is int
            and row["spawner_pid"] == registrar and row["purpose"] == "mcp-helper"
            and row["install"] == registration["install"] == install and re.fullmatch(r"[a-f0-9]{12}", install)
            and row["argv"] == registration["child_argv"] == " ".join(argv[:10])
            and row["host"] == row["profile"] == row["hermes_home"] == ""
            and row["port"] is None and row["isolated"] is False, "forged helper identity/defaults differ")
    for key in ("create_time", "spawner_create", "registered_at"):
        require(type(row[key]) is float and math.isfinite(row[key]) and 0 < row[key] < 2**53, "forged birth/time type differs")
    require(type(registration["create_time"]) is float and type(registration["child_create"]) is float
            and row["create_time"] == registration["child_create"]
            and row["spawner_create"] == registration["create_time"], "original birth-time join differs")
    for key in ("boot_start", "spawner_boot"):
        require(row[key] is None or os.sys.platform == "linux" and type(row[key]) is float
                and math.isfinite(row[key]) and 0 <= row[key] < 2**53, "forged boot time differs")
    require(row["boot_id"] is None or os.sys.platform == "linux" and type(row["boot_id"]) is str
            and re.fullmatch(r"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}", row["boot_id"]),
            "forged boot ID differs")
    return row


def validate_closed_child(value, *, sentinel):
    require(type(value) is dict and set(value) == {"pid", "exit", "stdout", "stderr", "waited", "pipes_closed", "original_handle"}
            and type(value["pid"]) is int and 0 < value["pid"] < 2**32
            and type(value["exit"]) is int and (value["exit"] != 0 if sentinel else value["exit"] == 0)
            and all(value[key] is True for key in ("waited", "pipes_closed", "original_handle")),
            "closed original child schema differs")
    require(decoded(value["stderr"]) == b"", "original child stderr differs")
    return decoded(value["stdout"])
