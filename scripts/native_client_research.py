"""Small bounded read/process utilities for pinned hosted client research.

No acquisition, native admission guard or command selection occurs here.
"""
import hashlib
import io
import os
import resource
import selectors
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


def child_limits():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def kill_group(process):
    group_id = process.pid
    try:
        os.killpg(group_id, signal.SIGKILL)
    except ProcessLookupError:
        pass
    except PermissionError:
        # Darwin may refuse a group whose sole leader has already exited.
        # After reaping, prove absence without a destructive PID-reuse retry.
        if process.poll() is not None:
            try:
                os.killpg(group_id, 0)
            except ProcessLookupError:
                return
            except OSError:
                pass
        raise


def capture_output(process, selector, streams, started, timeout, limit):
    """Drain bounded nonblocking streams under the original process deadline."""
    counts = {"stdout": 0, "stderr": 0}
    group_closed = False
    while selector.get_map() or process.poll() is None:
        remaining = timeout - (time.monotonic() - started)
        if remaining <= 0:
            return "probe deadline exceeded"
        if process.poll() is not None and not group_closed:
            kill_group(process)
            group_closed = True
        for key, _ in selector.select(min(0.05, remaining)):
            name = key.data
            allowance = limit - counts[name]
            try:
                chunk = os.read(key.fd, min(64 << 10, allowance + 1))
            except BlockingIOError:
                continue
            if not chunk:
                selector.unregister(key.fileobj)
                key.fileobj.close()
                continue
            accepted = chunk[:allowance]
            streams[name].write(accepted)
            counts[name] += len(accepted)
            if len(chunk) > allowance:
                return "output limit reached"
    return None


def run_probe(argv, cwd, env, output, label, timeout=30, limit=OUTPUT_LIMIT, *, memory_only=False):
    """Cap streams live without limiting unrelated native filesystem writes.

    New private collectors use memory_only: no native-writable capture path is
    opened or reread, and the returned bytes come from the bounded held buffers.
    The historical on-disk behavior remains available to preserved collectors.
    """
    stdout_path, stderr_path = output / (label + ".stdout.log"), output / (label + ".stderr.log")
    record = {"probe": label, "argv": argv, "cwd": str(cwd), "error": None}
    started, process = time.monotonic(), None
    if limit < 1 or timeout <= 0:
        raise ValueError("positive probe stream limit and timeout are required")
    stdout_target = io.BytesIO() if memory_only else stdout_path.open("wb")
    stderr_target = io.BytesIO() if memory_only else stderr_path.open("wb")
    with stdout_target as stdout, stderr_target as stderr, selectors.DefaultSelector() as selector:
        try:
            process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True,
                                       preexec_fn=child_limits)
            for name, pipe in (("stdout", process.stdout), ("stderr", process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            record["error"] = capture_output(process, selector, {"stdout": stdout, "stderr": stderr},
                                             started, timeout, limit)
        except (OSError, subprocess.SubprocessError) as exc:
            record["error"] = str(exc)[:512]
        finally:
            if process is not None:
                try:
                    kill_group(process)
                    process.wait(timeout=5)
                finally:
                    for pipe in (process.stdout, process.stderr):
                        if pipe is not None:
                            pipe.close()
        if memory_only:
            stdout_body, stderr_body = stdout.getvalue(), stderr.getvalue()
    if not memory_only:
        stdout_body, stderr_body = stdout_path.read_bytes(), stderr_path.read_bytes()
    record.update({"exit": process.returncode if process is not None else None,
                   "elapsed_seconds": round(time.monotonic() - started, 3),
                   "stdout_file": stdout_path.name, "stderr_file": stderr_path.name,
                   "stdout_bytes": len(stdout_body), "stderr_bytes": len(stderr_body),
                   "stdout_sha256": hashlib.sha256(stdout_body).hexdigest(),
                   "stderr_sha256": hashlib.sha256(stderr_body).hexdigest()})
    return record, stdout_body, stderr_body

