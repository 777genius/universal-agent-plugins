"""Small bounded read/process utilities for pinned hosted client research.

No acquisition, native admission guard or command selection occurs here.
"""
import hashlib
import io
import os
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
    import resource
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))


def kill_group(process):
    group_id = process.pid
    try:
        os.killpg(group_id, signal.SIGKILL)
    except ProcessLookupError:
        pass
    except PermissionError as denied:
        # Darwin may refuse a group while its sole leader is still exiting.
        # Spend the existing cleanup reap window on our child, then prove
        # absence without a destructive PID-reuse retry. A live child or any
        # uncertainty about the original group preserves the signal refusal.
        if process.poll() is None:
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                raise denied
        try:
            os.killpg(group_id, 0)
        except ProcessLookupError:
            return
        except OSError:
            pass
        raise


def capture_output(process, selector, streams, started, timeout, limit, input_complete=None):
    """Drain bounded nonblocking streams under the original process deadline."""
    counts = {"stdout": 0, "stderr": 0}
    group_closed = False
    response = bytearray()
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
            if name == "stdout" and input_complete is not None:
                response.extend(accepted)
                if b"\n" in response:
                    if input_complete(bytes(response).split(b"\n", 1)[0] + b"\n") is not True:
                        return "probe input response refused"
                    process.stdin.close()
                    input_complete = None
    return "probe input response missing" if input_complete is not None else None


def run_probe(argv, cwd, env, output, label, timeout=30, limit=OUTPUT_LIMIT, *, memory_only=False, input_bytes=None, input_complete=None, umask=None):
    """Cap streams live without limiting unrelated native filesystem writes.

    New private collectors use memory_only: no native-writable capture path is
    opened or reread, and the returned bytes come from the bounded held buffers.
    The historical on-disk behavior remains available to preserved collectors.
    """
    if umask is not None and (type(umask) is not int or not 0 <= umask <= 0o777):
        raise ValueError("probe umask must be an exact POSIX permission mask")
    if input_bytes is not None and (type(input_bytes) is not bytes or not 1 <= len(input_bytes) <= 256):
        raise ValueError("probe input must be 1..256 exact bytes")
    if input_complete is not None and (input_bytes is None or not callable(input_complete)):
        raise ValueError("probe response predicate requires bounded input")
    if os.name != "posix":
        raise ValueError("research process capture requires POSIX")
    try:
        __import__("resource")
    except ImportError as error:
        raise ValueError("research process capture requires POSIX resource limits") from error
    stdout_path, stderr_path = output / (label + ".stdout.log"), output / (label + ".stderr.log")
    record = {"probe": label, "argv": argv, "cwd": str(cwd), "error": None}
    if input_bytes is not None:
        record.update(stdin_bytes=len(input_bytes), stdin_sha256=hashlib.sha256(input_bytes).hexdigest(), stdin_written=False)
    if input_complete is not None:
        record["stdin_response_confirmed"] = False
    def confirmed(body):
        try:
            accepted = input_complete(body)
        except Exception as error:
            record["stdin_response_exception"] = {"type": type(error).__name__, "message": str(error)[:512]}
            raise ValueError("probe input response predicate raised " + type(error).__name__) from error
        record["stdin_response_confirmed"] = accepted is True
        return accepted
    started, process = time.monotonic(), None
    if limit < 1 or timeout <= 0:
        raise ValueError("positive probe stream limit and timeout are required")
    stdout_target = io.BytesIO() if memory_only else stdout_path.open("wb")
    stderr_target = io.BytesIO() if memory_only else stderr_path.open("wb")
    with stdout_target as stdout, stderr_target as stderr, selectors.DefaultSelector() as selector:
        try:
            process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=(subprocess.PIPE if input_bytes is not None else subprocess.DEVNULL),
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True,
                                       preexec_fn=child_limits, **({"umask": umask} if umask is not None else {}))
            if input_bytes is not None:
                os.set_blocking(process.stdin.fileno(), False)
                try:
                    if os.write(process.stdin.fileno(), input_bytes) != len(input_bytes):
                        raise OSError("probe input write was incomplete")
                    record["stdin_written"] = True
                finally:
                    if input_complete is None:
                        process.stdin.close()
            for name, pipe in (("stdout", process.stdout), ("stderr", process.stderr)):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            record["error"] = capture_output(process, selector, {"stdout": stdout, "stderr": stderr},
                                             started, timeout, limit, confirmed if input_complete is not None else None)
        except (OSError, subprocess.SubprocessError, ValueError) as exc:
            record["error"] = str(exc)[:512]
        finally:
            if process is not None:
                try:
                    kill_group(process)
                    process.wait(timeout=5)
                finally:
                    for pipe in ((process.stdin,) if input_bytes is not None else ()) + (process.stdout, process.stderr):
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

