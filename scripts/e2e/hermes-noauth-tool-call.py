#!/usr/bin/env python3
"""Real pinned Hermes legacy CLI dispatch with a scripted localhost provider.

Only inert User MCP calls are qualified. No vendor inference, login, source
patch, approval bypass or Project MCP grant is involved.
"""
import hashlib
import json
import os
from pathlib import Path
import runpy
import secrets
import shutil
import subprocess
import sys
import tempfile
import threading
import time

SCRIPTS = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(SCRIPTS))
from hermes_local_provider_fixture import MODEL, ScriptedHermesProvider, result_token

PIN = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"
LIMIT = 1 << 20
NATIVE = runpy.run_path(str(Path(__file__).with_name("hermes-native.py")), run_name="hermes_native_helpers")
source_root = NATIVE["source_root"]


def require(value, message):
    if not value:
        raise RuntimeError(message)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def bounded(command, cwd, env, output, timeout=90, expected_failure=False):
    """Capture both streams without an unbounded file or inherited shell."""
    child = subprocess.Popen([str(arg) for arg in command], cwd=cwd, env=env,
                             stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    buffers, overflow = [bytearray(), bytearray()], threading.Event()
    def read(pipe, target):
        try:
            while body := pipe.read(16384):
                room = LIMIT - len(target)
                target.extend(body[:room])
                if len(body) > room:
                    overflow.set()
        finally:
            pipe.close()
    threads = [threading.Thread(target=read, args=(pipe, buffer), daemon=True)
               for pipe, buffer in zip((child.stdout, child.stderr), buffers)]
    for thread in threads:
        thread.start()
    failure = None
    try:
        deadline = time.monotonic() + timeout
        while child.poll() is None:
            if overflow.is_set() or time.monotonic() >= deadline:
                failure = "output bound" if overflow.is_set() else "command timeout"
                child.terminate()
                try:
                    child.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait(timeout=3)
                break
            time.sleep(.05)
        for thread in threads:
            thread.join(timeout=2)
        if overflow.is_set():
            failure = "output bound"
        if any(thread.is_alive() for thread in threads):
            failure = "native command left inherited capture pipe open"
    finally:
        for suffix, body in zip(("stdout", "stderr"), buffers):
            output.with_suffix("." + suffix).write_bytes(body)
        output.with_suffix(".status.json").write_text(json.dumps({"argv": [str(x) for x in command],
            "pid": child.pid, "exit": child.poll(), "error": failure,
            "streams": {suffix: {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}
                        for suffix, body in zip(("stdout", "stderr"), buffers)}}) + "\n")
    require(not failure and not overflow.is_set() and (child.returncode != 0) == expected_failure,
            output.name + ": native command failed or exceeded its capture bound")
    return bytes(buffers[0])


def child_case():
    helpers = runpy.run_path(str(Path(__file__).with_name("hermes-native-lifecycle.py")), run_name="hermes_lifecycle_helpers")
    Parent = helpers["Lifecycle"]
    source = source_root()
    sys.path.insert(0, str(source))
    from hermes_cli._launchers import runtime_command

    class NativeCallLifecycle(Parent):
        def capture(self, command, name, cwd=None, expected_failure=False, env=None):
            output = self.evidence / name
            body = bounded(command, cwd or self.root, env or dict(os.environ), output,
                           expected_failure=expected_failure)
            output.write_bytes(body)
            return output

        def probe_mcp(self, stage):
            super().probe_mcp(stage)
            if stage not in {"user-add", "user-repair"}:
                return
            original = self.config.read_bytes()
            with ScriptedHermesProvider(os.environ["HERMES_CALL_NONCE"], stage) as provider:
                # Configure only this test-owned profile, before holding its bytes.
                # JSON is a strict YAML subset; preserve both MCP entries exactly.
                # The official isolated runtime does not guarantee PyYAML.
                # Change only this disposable profile's known test model line;
                # preserve the entire UAP-produced MCP mapping byte for byte.
                header = b"model: offline-native-smoke\n"
                require(original.startswith(header) and original.count(header) == 1
                        and original.endswith(b"\n") and b"mcp_servers:\n" in original
                        and b"\ntools:" not in original, "unexpected native YAML baseline")
                selected_model = {"provider": "custom", "base_url": provider.base_url,
                                  "default": MODEL, "context_length": 65536}
                self.config.write_bytes(
                    b"model: " + json.dumps(selected_model, sort_keys=True).encode("utf-8")
                    + b"\n" + original[len(header):]
                    # Official Hermes config explicitly permits an eager tool
                    # surface. It keeps this deterministic probe on direct
                    # native MCP, without modifying installed MCP authority.
                    + b"tools:\n  tool_search:\n    enabled: off\n")
                held = self.config.read_bytes()
                query = "Call the offered inert ping once. nonce=" + provider.nonce + " stage=" + stage
                native = json.loads(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"]).read_text())
                command = runtime_command(source, python=native[0], args=["--base-url", provider.base_url,
                    "--model", MODEL, "--enabled-toolsets", "mcp-uap-native-test-server", "--max-turns", "3",
                    "--query", query], code=(
                        # Legacy run_agent.py omits MCP startup. Discover the
                        # exact UAP-owned server with the pinned vendor API.
                        "from tools.mcp_oauth import suppress_interactive_oauth\n"
                        "from tools.mcp_tool_discovery import discover_mcp_tools\n"
                        "with suppress_interactive_oauth():\n"
                        "    discovered = discover_mcp_tools(allowed_mcp_names=['uap-native-test-server'])\n"
                        "if sorted(discovered) != ['mcp__uap_native_test_server__ping']:\n"
                        "    raise RuntimeError('native selected MCP tool discovery differs')\n"
                        f"runpy.run_path({str(source / 'run_agent.py')!r}, run_name='__main__')"))
                try:
                    output = bounded(command, self.root, dict(os.environ), self.evidence / (stage + "-native-driver"))
                    provider.close()
                    receipt = provider.receipt()
                    require(receipt["request_count"] == 2 and receipt["completed"] is True and receipt["invalid"] == 0,
                            "native provider exchange incomplete")
                    require(receipt["closed"] is True and receipt["active_requests"] == 0
                            and receipt["exchange_count"] == 2 and len(receipt["exchanges"]) == 2
                            and all(x["status"] == 200 and x["delivered"] is True for x in receipt["exchanges"]),
                            "original provider exchange or cleanup incomplete")
                    require(provider.final_token.encode() in output, "native driver omitted final joined result")
                    require(self.config.read_bytes() == held, "native driver changed owned YAML authority")
                finally:
                    provider.close()
                    (self.evidence / (stage + "-provider.json")).write_text(json.dumps(provider.receipt(), indent=2) + "\n")
                    self.config.write_bytes(original)

    with tempfile.TemporaryDirectory(prefix="hermes-call-case-", dir=os.environ["RUNNER_TEMP"]) as temporary:
        suite = NativeCallLifecycle(Path(temporary))
        suite.prepare()
        suite.user_lifecycle()
        suite.project_lifecycle()
        suite.remove_user()


PROFILE_DIRECTORIES = (("XDG_CONFIG_HOME", "config"), ("XDG_CACHE_HOME", "cache"),
                       ("XDG_STATE_HOME", "state"), ("XDG_DATA_HOME", "data"),
                       ("TMPDIR", "tmp"), ("TEMP", "tmp"), ("TMP", "tmp"),
                       ("APPDATA", "AppData/Roaming"), ("LOCALAPPDATA", "AppData/Local"))


def closed_environment(root, evidence, nonce, mcp_url, *, installation=None):
    env = {key: os.environ[key] for key in ("PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT") if key in os.environ}
    # Only explicit official installation outputs and task-owned locations survive.
    if installation is None:
        for key in ("HERMES_BIN", "UAP_BIN", "RUNNER_TEMP", "HERMES_NATIVE_RUNTIME_COMMAND_FILE"):
            env[key] = os.environ[key]
        if "HERMES_NATIVE_SOURCE" in os.environ:
            env["HERMES_NATIVE_SOURCE"] = str(source_root())
    else:
        require(set(installation) == {"HERMES_BIN", "RUNNER_TEMP", "HERMES_PROFILE_RUNTIME_ROOT", "HERMES_PROFILE_PYTHON"},
                "explicit bundled runtime outputs required")
        env.update(installation)
    env.update({"HOME": str(root), "USERPROFILE": str(root), "HERMES_HOME": str(root / "selected-profile"),
                "HERMES_NATIVE_EVIDENCE": str(evidence), "HERMES_CALL_NONCE": nonce,
                "HERMES_NATIVE_MCP_URL": mcp_url, "PYTHONUTF8": "1", "PYTHONDONTWRITEBYTECODE": "1",
                "NO_COLOR": "1", "TERM": "dumb", "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0",
                "NO_PROXY": "127.0.0.1,localhost", "no_proxy": "127.0.0.1,localhost"})
    for key, folder in PROFILE_DIRECTORIES:
        path = root / folder
        path.mkdir(parents=True, exist_ok=True)
        env[key] = str(path)
    return env


def source_identity(source):
    def git(*args):
        return subprocess.check_output(["git", "-C", str(source), *args], text=True, timeout=10,
                                       env={**os.environ, "GIT_OPTIONAL_LOCKS": "0"}).strip()
    return {"commit": git("rev-parse", "HEAD"), "tree": git("rev-parse", "HEAD^{tree}"),
            "dirty": git("status", "--porcelain", "--untracked-files=normal")}


class WindowsJob:
    """Owned job with the same kill-on-close/accounting contract as windows_conpty.py."""
    def __init__(self, process):
        import ctypes as C
        from ctypes import wintypes as W
        class Basic(C.Structure):
            _fields_ = [("process_time", C.c_int64), ("job_time", C.c_int64), ("flags", W.DWORD),
                        ("min_working", C.c_size_t), ("max_working", C.c_size_t), ("process_limit", W.DWORD),
                        ("affinity", C.c_size_t), ("priority", W.DWORD), ("scheduling", W.DWORD)]
        class Limits(C.Structure):
            _fields_ = [("basic", Basic), ("io", C.c_uint64 * 6), ("process_memory", C.c_size_t),
                        ("job_memory", C.c_size_t), ("peak_process", C.c_size_t), ("peak_job", C.c_size_t)]
        class Accounting(C.Structure):
            _fields_ = [("times", C.c_int64 * 4), ("faults", W.DWORD), ("total", W.DWORD),
                        ("active", W.DWORD), ("terminated", W.DWORD)]
        self.C, self.Accounting = C, Accounting
        self.api = C.WinDLL("kernel32", use_last_error=True)
        signatures = {"CreateJobObjectW": ([W.LPVOID, W.LPCWSTR], W.HANDLE),
            "SetInformationJobObject": ([W.HANDLE, C.c_int, W.LPVOID, W.DWORD], W.BOOL),
            "AssignProcessToJobObject": ([W.HANDLE, W.HANDLE], W.BOOL),
            "QueryInformationJobObject": ([W.HANDLE, C.c_int, W.LPVOID, W.DWORD, W.LPVOID], W.BOOL),
            "TerminateJobObject": ([W.HANDLE, W.UINT], W.BOOL), "CloseHandle": ([W.HANDLE], W.BOOL)}
        for name, (args, result) in signatures.items():
            getattr(self.api, name).argtypes, getattr(self.api, name).restype = args, result
        self.handle = self.api.CreateJobObjectW(None, None)
        self.check(self.handle)
        try:
            limits = Limits()
            limits.basic.flags = 0x2000
            self.check(self.api.SetInformationJobObject(self.handle, 9, C.byref(limits), C.sizeof(limits)))
            self.check(self.api.AssignProcessToJobObject(self.handle, int(process._handle)))
        except BaseException:
            self.api.CloseHandle(self.handle)
            raise

    def check(self, result):
        if not result:
            raise self.C.WinError(self.C.get_last_error())

    def close(self):
        try:
            self.check(self.api.TerminateJobObject(self.handle, 97))
            deadline = time.monotonic() + 5
            while True:
                counts = self.Accounting()
                self.check(self.api.QueryInformationJobObject(self.handle, 1, self.C.byref(counts), self.C.sizeof(counts), None))
                if counts.active == 0:
                    return
                require(time.monotonic() < deadline, "owned Windows job still has active processes")
                time.sleep(.05)
        finally:
            self.check(self.api.CloseHandle(self.handle))


def run():
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "actual GitHub-hosted native lane required")
    runner = Path(os.environ["RUNNER_TEMP"]).resolve()
    checkout = Path(__file__).resolve().parents[2]
    before = source_identity(checkout)
    require(before["commit"] == os.environ["HERMES_NATIVE_UAP_HEAD"] and not before["dirty"], "exact clean source required")
    source = source_root()
    official = source_identity(source)
    require(official["commit"] == PIN and not official["dirty"], "unmodified pinned official Hermes source required")
    from pi_project_components_native_e2e import PiHTTPMCPProbe
    helpers = runpy.run_path(str(Path(__file__).with_name("hermes-native.py")), run_name="hermes_process_helpers")
    evidence = Path(tempfile.mkdtemp(prefix="hermes-native-call-evidence-", dir=runner))
    home = Path(tempfile.mkdtemp(prefix="hermes-native-call-home-", dir=runner))
    home_identity = (home.stat().st_dev, home.stat().st_ino, home.stat().st_mode)
    (home / "selected-profile").mkdir(mode=0o700)
    native_argv = json.loads(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"]).read_text())
    require(isinstance(native_argv, list) and len(native_argv) >= 4 and Path(native_argv[0]).is_file(), "official runtime argv missing")
    nonce, child, failure, job = secrets.token_hex(32), None, None, None
    gate = evidence / "owned-start-gate"
    proof = {"source_before": before, "hermes_source": official, "nonce": nonce,
             "run_id": os.environ["GITHUB_RUN_ID"], "run_attempt": os.environ["GITHUB_RUN_ATTEMPT"],
             "runner_os": os.environ["RUNNER_OS"], "runner_arch": os.environ["RUNNER_ARCH"],
             "native_runtime_sha256": sha(Path(native_argv[0])), "driver": "official run_agent.py legacy CLI",
             "runtime_command_sha256": sha(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"])),
             "launcher_sha256": sha(Path(os.environ["HERMES_BIN"])),
             "real_model_inference": False, "vendor_authentication": False, "tools_call": False}
    with PiHTTPMCPProbe(nonce=nonce) as probe:
        try:
            env = closed_environment(home, evidence, nonce, probe.url)
            env["HERMES_CALL_START_GATE"] = str(gate)
            flags = {"creationflags": subprocess.CREATE_NEW_PROCESS_GROUP} if os.name == "nt" else {"start_new_session": True}
            sys.path.insert(0, str(source))
            from hermes_cli._launchers import runtime_command
            command = runtime_command(source, python=native_argv[0], args=["--child"],
                code=f"runpy.run_path({str(Path(__file__).resolve())!r}, run_name='__main__')")
            with (evidence / "lifecycle.stdout").open("wb") as out, (evidence / "lifecycle.stderr").open("wb") as err:
                child = subprocess.Popen(command, env=env,
                                         stdin=subprocess.DEVNULL, stdout=out, stderr=err, **flags)
                if os.name == "nt":
                    job = WindowsJob(child)
                proof["lifecycle_pid"] = child.pid
                proof["process_ownership"] = "Windows kill-on-close job" if job is not None else "dedicated POSIX process group"
                gate.write_text("owned\n")
                deadline = time.monotonic() + 720
                while child.poll() is None:
                    require(time.monotonic() < deadline, "native lifecycle timeout")
                    require(all((evidence / name).stat().st_size <= LIMIT for name in ("lifecycle.stdout", "lifecycle.stderr")),
                            "native lifecycle output bound")
                    time.sleep(.1)
            require(child.returncode == 0, "native lifecycle child failed")
            calls, invalid = probe.call_records()
            require(invalid == 0 and len(calls) == 2 and probe.methods().count("tools/call") == 2,
                    "exact two total native MCP calls required")
            for stage in ("user-add", "user-repair"):
                provider = json.loads((evidence / (stage + "-provider.json")).read_text())
                matched = [call for call in calls if call["arguments"] == {"nonce": nonce, "stage": stage}]
                require(len(matched) == 1 and matched[0]["result"] == result_token(nonce, stage)
                        and provider["completed"] is True and provider["request_count"] == 2 and provider["invalid"] == 0,
                        "provider/native MCP nonce join failed")
            proof.update({"tools_call": True, "native_agent_turn": True, "scripted_provider_exchange": True,
                          "mcp_calls": calls, "mcp_methods": probe.methods(), "status": "pass"})
        except BaseException as error:
            failure = error
            proof.update({"status": "failed", "error": type(error).__name__ + ": " + str(error)[:300]})
        finally:
            if child is not None:
                try:
                    if job is not None:
                        job.close()
                        child.wait(timeout=5)
                    else:
                        helpers["stop_lifecycle"](child)
                    proof["process_cleanup"] = "completed"
                except BaseException as error:
                    failure = error
                    proof.update({"status": "failed", "process_cleanup": "failed", "cleanup_error": type(error).__name__})
            proof["lifecycle_exit"] = child.returncode if child else None
            final_calls, invalid_calls = probe.call_records()
            if proof.get("status") == "pass" and (invalid_calls or final_calls != proof["mcp_calls"]
                                                  or probe.methods().count("tools/call") != 2):
                failure = RuntimeError("MCP calls changed before owned cleanup completed")
                proof["status"] = "failed"
            proof["mcp_calls_after_cleanup"], proof["invalid_mcp_calls"] = final_calls, invalid_calls
            proof["source_after"] = source_identity(checkout)
            if proof["source_after"] != before:
                failure = RuntimeError("source changed")
                proof["status"] = "failed"
            proof["hermes_source_after"] = source_identity(source)
            proof["native_runtime_sha256_after"] = sha(Path(native_argv[0]))
            proof["runtime_command_sha256_after"] = sha(Path(os.environ["HERMES_NATIVE_RUNTIME_COMMAND_FILE"]))
            proof["launcher_sha256_after"] = sha(Path(os.environ["HERMES_BIN"]))
            if (proof["hermes_source_after"] != official
                    or any(proof[name + "_after"] != proof[name] for name in
                           ("native_runtime_sha256", "runtime_command_sha256", "launcher_sha256"))):
                failure = RuntimeError("official Hermes source or installed runtime changed")
                proof["status"] = "failed"
            try:
                require(home.is_dir() and not home.is_symlink() and home.resolve() == home,
                        "owned private home identity changed")
                require((home.stat().st_dev, home.stat().st_ino, home.stat().st_mode) == home_identity,
                        "owned private home was replaced")
                shutil.rmtree(home)
                proof["private_home_cleanup"] = "completed"
            except BaseException as error:
                failure = error
                proof.update({"status": "failed", "private_home_cleanup": "failed"})
            proof["streams"] = {p.name: {"bytes": p.stat().st_size, "sha256": sha(p)}
                                for p in evidence.iterdir() if p.is_file()}
            (evidence / "native-tool-call.json").write_text(json.dumps(proof, indent=2) + "\n")
    if failure:
        raise RuntimeError("native Hermes call failed; original evidence: " + str(evidence)) from failure
    print("PASS: real Hermes legacy driver dispatched two inert User MCP calls; no vendor inference/auth: " + str(evidence))


if __name__ == "__main__":
    if sys.argv[1:] == ["--child"]:
        gate = Path(os.environ["HERMES_CALL_START_GATE"])
        deadline = time.monotonic() + 10
        while not gate.is_file():
            require(time.monotonic() < deadline, "owned process gate did not open")
            time.sleep(.02)
        child_case()
    elif not sys.argv[1:]:
        run()
    else:
        raise SystemExit("no arguments, or private --child")
