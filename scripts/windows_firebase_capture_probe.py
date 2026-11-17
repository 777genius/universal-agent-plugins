#!/usr/bin/env python3
"""Keep the published Firebase probe's Windows family inside its owned TEMP."""
import hashlib
import importlib.util
import json
import ntpath
import os
from pathlib import Path
import secrets
import stat
import shutil
import subprocess
import sys
import tempfile
import time

SCRIPTS = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPTS))
from windows_owned_job import WindowsJob

REPO = SCRIPTS.parent
OUTPUT_LIMIT = 1 << 20
PROBE_TIMEOUT = 900  # Outer custody bound; the existing add budget remains 180 s.
DIAGNOSTIC_LIMIT = 8 * 1024


def process_identity(job, pid, deadline):
    """Read only a held, confirmed member; never retain its full image path."""
    from ctypes import wintypes as W
    if not 0 < pid <= 0xffffffff or time.monotonic() >= deadline:
        return None
    handle = job.api.OpenProcess(0x1000, False, pid)  # QUERY_LIMITED_INFORMATION
    if not handle:
        return None
    try:
        member = W.BOOL()
        if (time.monotonic() >= deadline
                or not job.api.IsProcessInJob(handle, job.handle, job.C.byref(member))
                or not member.value):
            return None
        result = {"pid": pid, "executable": None}
        if time.monotonic() >= deadline:
            return result
        size = W.DWORD(4096)
        image = job.C.create_unicode_buffer(size.value)
        if (job.api.QueryFullProcessImageNameW(handle, 0, image, job.C.byref(size))
                and 0 < size.value < 4096 and len(image.value) == size.value):
            name = ntpath.basename(image.value)
            if (0 < len(name) <= 128 and name not in (".", "..")
                    and not any(ord(c) < 32 or c in "/\\:" for c in name)):
                result["executable"] = name
        return result
    finally:
        # Failure here is diagnostic only; the caller still closes its Job.
        if not job.api.CloseHandle(handle):
            raise RuntimeError("diagnostic process handle close failed")


def process_failure_diagnostic(job, deadline):
    """One bounded query, no PID scan, retries, privilege changes or argv reads."""
    from ctypes import wintypes as W
    deadline = min(deadline, time.monotonic() + 1.0)
    result = {"status": "deadline", "processes": []}
    if time.monotonic() >= deadline:
        return result
    C, api = job.C, job.api
    signatures = {
        "OpenProcess": ([W.DWORD, W.BOOL, W.DWORD], W.HANDLE),
        "IsProcessInJob": ([W.HANDLE, W.HANDLE, C.POINTER(W.BOOL)], W.BOOL),
        "QueryFullProcessImageNameW":
            ([W.HANDLE, W.DWORD, W.LPWSTR, C.POINTER(W.DWORD)], W.BOOL),
    }
    for name, (args, returns) in signatures.items():
        getattr(api, name).argtypes, getattr(api, name).restype = args, returns

    class ProcessIds(C.Structure):
        _fields_ = [("assigned", W.DWORD), ("count", W.DWORD),
                    ("pids", C.c_size_t * 16)]

    ids = ProcessIds()
    if time.monotonic() >= deadline:
        return result
    if not api.QueryInformationJobObject(job.handle, 3, C.byref(ids), C.sizeof(ids), None):
        result["status"] = "query-unavailable"
        return result
    if not 0 <= ids.count <= min(ids.assigned, 16):
        result["status"] = "invalid-count"
        return result
    result.update(status="partial", assigned=int(ids.assigned))
    seen = set()
    for pid in ids.pids[:ids.count]:
        if time.monotonic() >= deadline:
            result["status"] = "deadline"
            break
        if pid in seen:
            continue
        seen.add(pid)
        identity = process_identity(job, int(pid), deadline)
        if identity is not None:
            result["processes"].append(identity)
    else:
        if (len(result["processes"]) == ids.assigned
                and all(p["executable"] is not None for p in result["processes"])):
            result["status"] = "complete"
    if time.monotonic() >= deadline:
        result["status"] = "deadline"
    return result


def temp_failure_diagnostic(error, root, original):
    """Attribute a bounded ordinary member without exposing exception text."""
    result = {key: value if type(value) is int and 0 <= value <= 0xffffffff else None
              for key in ("errno", "winerror")
              for value in (getattr(error, key, None),)}
    result["relative_member"] = None
    try:
        if not isinstance(error.filename, str):
            return result
        member = Path(error.filename)
        relative = member.relative_to(root)
        label = relative.as_posix()
        if (not member.is_absolute() or len(label) > 512 or len(relative.parts) > 16
                or any(part == ".." or ":" in part for part in relative.parts)
                or any(ord(c) < 32 for c in label)):
            return result
        identity = lambda s: (s.st_dev, s.st_ino, s.st_mode)
        if identity(root.lstat()) != identity(original):
            return result
        observed = []
        current = root
        for part in (None, *relative.parts):
            if part is not None:
                current = current / part
            info = current.lstat()
            if (not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode))
                    or getattr(info, "st_file_attributes", 0) & 0x400
                    or (stat.S_ISREG(info.st_mode) and info.st_nlink != 1)):
                return result
            observed.append((current, identity(info)))
        if all(identity(path.lstat()) == expected for path, expected in observed):
            result["relative_member"] = label
    except (OSError, ValueError, TypeError):
        pass
    return result


def add_failure_diagnostics(receipt, diagnostics):
    """The extra serialized field is bounded independently of the primary proof."""
    if not diagnostics:
        return
    original_size = len(json.dumps(receipt, indent=2).encode("utf-8"))
    receipt["failure_diagnostics"] = diagnostics
    if len(json.dumps(receipt, indent=2).encode("utf-8")) - original_size > DIAGNOSTIC_LIMIT:
        receipt["failure_diagnostics"] = {"status": "diagnostic-bound-exceeded"}


CONSOLE_LIMIT = 1800
SUMMARY_STATUSES = frozenset((
    "passed", "repeat-add-unqualified", "incomplete-native-proof", "cleanup-failed",
    "failed-install-residue", "install-failed", "documented-or-source-unsupported",
    "target-unavailable", "host-containment-unavailable", "test-build-version-policy",
    "expected-suspended-policy", "prepared-lifecycle-passed",
    "package-installed-native-unqualified", "external-native-uninstall-required",
    "native-ownership-unverified",
))


def console_exit(value):
    # bool is an int subclass, but never an exit code. Bound before serialization.
    return value if type(value) is int and -(1 << 31) <= value < (1 << 32) else None


def console_flag(value):
    return value if type(value) is bool else None


def selected_probe_summary(proof, temp_removed, temp_error, primary_bytes):
    """Project fixed structured facts; no controller streams or arbitrary labels."""
    family = proof if type(proof) is dict else {}
    summary = {"diagnostic": "firebase-probe-summary-v1",
               "family": {key: console_flag(family.get(key)) for key in (
                   "timeout", "gate_released", "forced", "settled")},
               "temp_removed": console_flag(temp_removed),
               "temp_cleanup_error_present": temp_error is not None,
               "primary": "missing"}
    summary["family"].update(
        child_exit=console_exit(family.get("child_exit")),
        primary_error_present=family.get("primary_error") is not None,
        cleanup_error_present=family.get("cleanup_error") is not None)
    if family.get("settled") is not True:
        summary["primary"] = "unsettled"
    elif type(primary_bytes) is not bytes:
        pass
    elif len(primary_bytes) > OUTPUT_LIMIT:
        summary["primary"] = "oversized"
    else:
        try:
            document = json.loads(primary_bytes)
            row = document.get("case") if type(document) is dict else None
            if type(row) is not dict:
                raise ValueError("not a lifecycle row")
            summary["primary"] = "available"
            status = row.get("status")
            summary["status"] = (status if type(status) is str and len(status) <= 64
                                 and status in SUMMARY_STATUSES else "unrecognized")
            summary["failed_gates"] = [
                key for key in ("repeat_verified", "removal_verified", "sentinels_preserved")
                if row.get(key) is not True]
            if row.get("status") != "passed":
                summary["failed_gates"].append("status_passed")
            if type(row.get("native_residuals")) is not list or row["native_residuals"] != []:
                summary["failed_gates"].append("native_residuals_empty")
            summary["commands"] = {}
            for label in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                command = row.get(label)
                command = command if type(command) is dict else {}
                summary["commands"][label] = {
                    "exit_code": console_exit(command.get("exit_code")),
                    "timeout": console_flag(command.get("timeout"))}
        except (ValueError, TypeError, RecursionError, UnicodeError):
            # No exception text or partially projected lifecycle object escapes.
            summary["primary"] = "malformed"
            for key in ("status", "failed_gates", "commands"):
                summary.pop(key, None)
    line = json.dumps(summary, separators=(",", ":"), ensure_ascii=True)
    if len(line.encode("utf-8")) > CONSOLE_LIMIT:
        return '{"diagnostic":"firebase-probe-summary-v1","primary":"diagnostic-bound"}'
    return line


def emit_probe_summary(proof, temp_removed, temp_error, primary_bytes):
    """Observability cannot override the unchanged native/custody verdict."""
    try:
        line = selected_probe_summary(proof, temp_removed, temp_error, primary_bytes)
    except BaseException:
        line = '{"diagnostic":"firebase-probe-summary-v1","primary":"diagnostic-unavailable"}'
    try:
        print(line, flush=True)
    except BaseException:
        pass


def job_counts(job):
    counts = job.Accounting()
    job.check(job.api.QueryInformationJobObject(
        job.handle, 1, job.C.byref(counts), job.C.sizeof(counts), None))
    return {"total": int(counts.total), "active": int(counts.active),
            "terminated": int(counts.terminated)}


def supervise(command, *, cwd, env, stdout, stderr, timeout):
    """Admit the waiting controller before it can start the unchanged command."""
    proof = {"child_pid": None, "child_exit": None, "timeout": False,
             "gate_released": False, "counts_before": None, "forced": None,
             "settled": False, "primary_error": None, "cleanup_error": None}
    process, job = None, None
    nonce = secrets.token_hex(16)
    controller = [sys.executable, "-I", "-S", "-B", str(Path(__file__).resolve()),
                  "--gate", nonce, *command]
    deadline = time.monotonic() + timeout
    try:
        process = subprocess.Popen(controller, cwd=cwd, env=env,
                                   stdin=subprocess.PIPE, stdout=stdout, stderr=stderr)
        proof["child_pid"] = process.pid
        job = WindowsJob(process)
        process.stdin.write((nonce + "\n").encode("ascii"))
        process.stdin.close()
        proof["gate_released"] = True
        while process.poll() is None:
            if time.monotonic() >= deadline:
                proof["timeout"] = True
                break
            if any(os.fstat(stream.fileno()).st_size > OUTPUT_LIMIT
                   for stream in (stdout, stderr)):
                proof["primary_error"] = "output-bound"
                break
            time.sleep(.05)
        proof["child_exit"] = process.poll()
    except BaseException as error:
        proof["primary_error"] = type(error).__name__
    finally:
        if process is not None and process.stdin is not None:
            try:
                process.stdin.close()
            except OSError:
                pass
        if job is not None:
            try:
                proof["counts_before"] = job_counts(job)
                proof["forced"] = proof["counts_before"]["active"] != 0
            except (OSError, RuntimeError) as error:
                proof["primary_error"] = proof["primary_error"] or type(error).__name__
            if proof["forced"] is True:
                try:
                    proof["process_diagnostics"] = process_failure_diagnostic(job, deadline)
                except BaseException:
                    proof["process_diagnostics"] = {"status": "unavailable", "processes": []}
            try:
                # This existing API returns only after active==0 and handle close.
                # Closing a handle in a failing finally block is not settlement.
                job.close()
                process.wait(timeout=5)
                proof["settled"] = True
            except BaseException as error:
                proof["cleanup_error"] = type(error).__name__
        elif process is not None and not proof["gate_released"]:
            try:
                # The isolated controller cannot create descendants before its gate.
                if process.poll() is None:
                    process.kill()
                process.wait(timeout=5)
                proof["settled"] = True
            except BaseException as error:
                proof["cleanup_error"] = type(error).__name__
        elif process is None:
            proof["settled"] = True
    return proof


def run_case(cli, root, report):
    source = SCRIPTS / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("published_firebase", source)
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    doc = json.loads((REPO / "tests/live-install/fixtures.json").read_text(encoding="utf-8"))
    audit.validate_fixtures(doc)
    assert doc["plugin_aliases"][6] == "firebase"
    case_root_chars = len(str(root / "plugin-06"))
    assert case_root_chars >= 65, "Original Windows TEMP profile was not reproduced"
    row = audit.do_case(cli, "claude", "plugin", "firebase", 6, root,
                        doc["plugin_declared_names"], doc["signed_release_pins"])
    with report.open("x", encoding="utf-8") as output:
        output.write(json.dumps({
            "source_sha": os.environ.get("GITHUB_SHA"),
            "published_cli": "agentplugins 0.1.101", "target": "claude",
            "kind": "plugin", "case_index": 6,
            "temp_profile": "original-windows-default", "case_root_chars": case_root_chars,
            "case": row}, indent=2) + "\n")
    print(json.dumps({"status": row.get("status"),
                      "repeat_verified": row.get("repeat_verified"),
                      "removal_verified": row.get("removal_verified"),
                      "sentinels_preserved": row.get("sentinels_preserved")}))
    if (row.get("status") != "passed" or row.get("repeat_verified") is not True
            or row.get("removal_verified") is not True
            or row.get("sentinels_preserved") is not True
            or row.get("native_residuals") != []):
        raise SystemExit("Published Windows Firebase native lifecycle unqualified")


def run_probe(cli, evidence_dir):
    evidence_dir = Path(evidence_dir)
    report = evidence_dir / "published-firebase-claude.json"
    family_report = evidence_dir / "published-firebase-process-family.json"
    stdout_path = evidence_dir / "published-firebase-controller.stdout"
    stderr_path = evidence_dir / "published-firebase-controller.stderr"
    for path in (report, family_report, stdout_path, stderr_path):
        if path.exists() or path.is_symlink():
            raise RuntimeError("Firebase probe refuses existing evidence")
    # Use the original default Windows TEMP, not the shorter RUNNER_TEMP.
    root = Path(tempfile.mkdtemp(prefix="uap-live-30x30-"))
    original_root = root.lstat()
    proof = {"settled": True, "primary_error": "controller-not-started",
             "child_exit": None, "timeout": False, "forced": None,
             "gate_released": False, "counts_before": None, "cleanup_error": None}
    temp_removed, temp_error = False, None
    diagnostics = {}
    try:
        with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
            proof["settled"] = False
            proof = supervise(
                [sys.executable, "-I", "-S", "-B", str(Path(__file__).resolve()),
                 "--case", str(cli), str(root), str(report)],
                cwd=REPO, env=dict(os.environ), stdout=stdout, stderr=stderr,
                timeout=PROBE_TIMEOUT)
    except BaseException as error:
        proof["primary_error"] = type(error).__name__
    finally:
        if proof.get("settled") is True:
            try:
                current_root = root.lstat()
                if (not stat.S_ISDIR(current_root.st_mode)
                        or current_root.st_mode != original_root.st_mode
                        or (current_root.st_dev, current_root.st_ino)
                        != (original_root.st_dev, original_root.st_ino)):
                    raise OSError("owned TEMP identity changed")
                shutil.rmtree(root)
                temp_removed = True
            except OSError as error:
                temp_error = type(error).__name__
                try:
                    diagnostics["temp_cleanup"] = temp_failure_diagnostic(error, root, original_root)
                except BaseException:
                    diagnostics["temp_cleanup"] = {"status": "unavailable"}
        primary = None
        data = None
        if report.is_file() and not report.is_symlink() and report.stat().st_size <= OUTPUT_LIMIT:
            data = report.read_bytes()
            primary = {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
        processes = proof.pop("process_diagnostics", None)
        if processes is not None:
            diagnostics["process_family"] = processes
        receipt = {"schema_version": 1, "source_sha": os.environ.get("GITHUB_SHA"),
                   "scope": "owned Firebase probe process family; not native qualification",
                   "command_label": "published Firebase/Claude probe", "family": proof,
                   "primary_report": primary, "temp_removed": temp_removed,
                   "temp_cleanup_error": temp_error, "retained_owned_temp": not temp_removed}
        add_failure_diagnostics(receipt, diagnostics)
        with family_report.open("x", encoding="utf-8") as output:
            output.write(json.dumps(receipt, indent=2) + "\n")
        emit_probe_summary(proof, temp_removed, temp_error, data)
    return 0 if (proof.get("child_exit") == 0 and proof.get("timeout") is False
                 and proof.get("gate_released") is True and proof.get("forced") is False
                 and proof.get("settled") is True and proof.get("primary_error") is None
                 and proof.get("cleanup_error") is None and temp_removed and primary) else 1


def main():
    if len(sys.argv) >= 4 and sys.argv[1] == "--gate":
        nonce = sys.argv[2]
        if (len(nonce) != 32 or any(c not in "0123456789abcdef" for c in nonce)
                or sys.stdin.buffer.read(33) != (nonce + "\n").encode("ascii")):
            return 2
        return subprocess.call(sys.argv[3:], stdin=subprocess.DEVNULL)
    if len(sys.argv) == 5 and sys.argv[1] == "--case":
        run_case(sys.argv[2], Path(sys.argv[3]), Path(sys.argv[4]))
        return 0
    if len(sys.argv) != 1 or os.name != "nt":
        raise SystemExit("Firebase native probe requires Windows and no arguments")
    cli = shutil.which("agentplugins.cmd")
    if cli is None:
        raise SystemExit("Pinned published agentplugins.cmd unavailable")
    return run_probe(str(Path(cli).resolve()), Path(os.environ["RUNNER_TEMP"]))


if __name__ == "__main__":
    raise SystemExit(main())
