"""Small bounded read/process utilities for pinned hosted client research.

No acquisition, native admission guard or command selection occurs here.
"""
import hashlib
import os
import resource
import signal
import subprocess
import time

OUTPUT_LIMIT = 1 << 20


def bounded_read(stream, limit):
    chunks, size = [], 0
    while True:
        chunk = stream.read(min(64 << 10, limit + 1 - size))
        if not chunk:
            return b"".join(chunks)
        chunks.append(chunk)
        size += len(chunk)
        if size > limit:
            raise ValueError("input exceeds configured byte limit")


def child_limits(limit):
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))


def run_probe(argv, cwd, env, output, label, timeout=30, limit=OUTPUT_LIMIT):
    """File limits apply in the child while running; every process group is closed."""
    stdout_path, stderr_path = output / (label + ".stdout.log"), output / (label + ".stderr.log")
    record = {"probe": label, "argv": argv, "cwd": str(cwd), "error": None}
    started, process = time.monotonic(), None
    with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
        try:
            process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                       stdout=stdout, stderr=stderr, start_new_session=True,
                                       preexec_fn=lambda: child_limits(limit))
            while True:
                if stdout_path.stat().st_size >= limit or stderr_path.stat().st_size >= limit:
                    record["error"] = "output limit reached"
                    break
                if time.monotonic() - started >= timeout:
                    record["error"] = "probe deadline exceeded"
                    break
                if process.poll() is not None:
                    break
                time.sleep(0.02)
        except (OSError, subprocess.SubprocessError) as exc:
            record["error"] = str(exc)[:512]
        finally:
            if process is not None:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait(timeout=5)
    stdout_body, stderr_body = stdout_path.read_bytes(), stderr_path.read_bytes()
    if len(stdout_body) >= limit or len(stderr_body) >= limit:
        record["error"] = "output limit reached"
    record.update({"exit": process.returncode if process is not None else None,
                   "elapsed_seconds": round(time.monotonic() - started, 3),
                   "stdout_file": stdout_path.name, "stderr_file": stderr_path.name,
                   "stdout_bytes": len(stdout_body), "stderr_bytes": len(stderr_body),
                   "stdout_sha256": hashlib.sha256(stdout_body).hexdigest(),
                   "stderr_sha256": hashlib.sha256(stderr_body).hexdigest()})
    return record, stdout_body, stderr_body

