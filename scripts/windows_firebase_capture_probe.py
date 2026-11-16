#!/usr/bin/env python3
"""Keep the published Firebase probe's Windows family inside its owned TEMP."""
import hashlib
import importlib.util
import json
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
        primary = None
        if report.is_file() and not report.is_symlink() and report.stat().st_size <= OUTPUT_LIMIT:
            data = report.read_bytes()
            primary = {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
        receipt = {"schema_version": 1, "source_sha": os.environ.get("GITHUB_SHA"),
                   "scope": "owned Firebase probe process family; not native qualification",
                   "command_label": "published Firebase/Claude probe", "family": proof,
                   "primary_report": primary, "temp_removed": temp_removed,
                   "temp_cleanup_error": temp_error, "retained_owned_temp": not temp_removed}
        with family_report.open("x", encoding="utf-8") as output:
            output.write(json.dumps(receipt, indent=2) + "\n")
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
