"""Finite private service observation; acquisition and native admission stay in the caller."""
import hashlib
import errno
import http.client
import json
import os
import re
import selectors
import signal
import socket
import subprocess
import threading
import time
import urllib.parse

from native_client_research import child_limits, kill_group

LIMIT = 1 << 20
LABEL = re.compile(r"[a-z][a-z0-9-]{0,90}\Z")


class ServiceDeadline(Exception):
    pass


def group_alive(pid):
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False


def port_closed(port):
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as check:
        check.settimeout(0.2)
        return check.connect_ex(("127.0.0.1", port)) == errno.ECONNREFUSED


class Service:
    """The observer sees snapshots and one loopback origin, never a launcher.

    A single monotonic deadline includes a five-second cleanup reserve. Captures
    are held in memory and remain quarantined unless the whole observation and
    graceful cleanup succeed. A forced kill is always a failed observation.
    """

    def __init__(self, label, argv, cwd, env, timeout, secrets):
        if not LABEL.fullmatch(label) or not 5 < timeout <= 60:
            raise ValueError("invalid bounded service label/deadline")
        self.label, self.argv, self.cwd, self.env = label, argv, cwd, env
        self.started = time.monotonic()
        self.deadline = self.started + timeout
        self.work_deadline = self.deadline - 5
        self._secrets = secrets
        self._lock = threading.Lock()
        self._streams = {"stdout": bytearray(), "stderr": bytearray()}
        self._problem = None
        self._ready = self._shutdown = False
        self._origin = self._port = self._process = None
        self._stop = threading.Event()
        self._http, self._bodies = [], {}

    def remaining(self):
        with self._lock:
            problem = self._problem
        left = self.work_deadline - time.monotonic()
        if problem or left <= 0:
            raise ValueError("service capture or original deadline failed")
        return left

    def snapshot(self):
        self.remaining()
        with self._lock:
            return bytes(self._streams["stdout"]), bytes(self._streams["stderr"])

    def bind_origin(self, origin):
        if self._origin is not None or not isinstance(origin, str):
            raise ValueError("service origin already bound or invalid")
        parsed = urllib.parse.urlsplit(origin)
        if (not re.fullmatch(r"http://127\.0\.0\.1:[0-9]{4,5}", origin)
                or parsed.port is None or not 1024 <= parsed.port <= 65535):
            raise ValueError("only one literal loopback service origin is permitted")
        self._origin, self._port = origin, parsed.port

    def capture_ready(self):
        self.remaining()
        if self._ready or self._origin is None or not self._secrets:
            raise ValueError("private readiness must be validated once before capture admission")
        self._ready = True

    def mark_shutdown(self):
        self.remaining()
        if not self._ready or self._shutdown:
            raise ValueError("unexpected service shutdown acknowledgement")
        self._shutdown = True

    def http(self, label, method, path, token=None, json_body=None, timeout=5):
        """Bound one literal-loopback HTTP exchange; retain no headers or tokens."""
        if (not self._ready or not LABEL.fullmatch(label) or len(self._http) >= 60
                or any(row["label"] == label for row in self._http)
                or method not in ("GET", "POST") or not isinstance(path, str)
                or not re.fullmatch(r"/api/v[12]/[A-Za-z0-9._~/%?:=&+-]{1,2048}", path)
                or "token" in path.lower() or not 0 < timeout <= 5
                or (method == "GET" and json_body is not None)):
            raise ValueError("unregistered or excessive service HTTP request")
        # A path cannot replace the already bound host or introduce an absolute
        # redirect. http.client has no proxy, redirect or credential discovery.
        parsed = urllib.parse.urlsplit(path)
        if parsed.scheme or parsed.netloc or parsed.fragment:
            raise ValueError("service request is not a relative API path")
        headers = {"Accept": "application/json", "Connection": "close"}
        if token is not None:
            if (not isinstance(token, str) or not re.fullmatch(r"[A-Za-z0-9_-]{43}", token)
                    or token.encode() not in self._secrets):
                raise ValueError("unregistered private bearer refused")
            headers["Authorization"] = "Bearer " + token
        body = None if json_body is None else json.dumps(json_body, allow_nan=False, separators=(",", ":")).encode()
        if body is not None:
            if len(body) > LIMIT:
                raise ValueError("service request body exceeds limit")
            headers["Content-Type"] = "application/json"
        request_deadline = time.monotonic() + min(timeout, self.remaining())
        connection = http.client.HTTPConnection("127.0.0.1", self._port, timeout=min(timeout, self.remaining()))
        timer = None
        row = {"label": label, "method": method, "path": path, "authenticated": token is not None,
               "status": None, "error": None, "response_file": self.label + "-" + label + ".http.json"}
        self._http.append(row)
        raw = b""
        try:
            connection.connect()
            held_socket = connection.sock

            def interrupt():
                try:
                    held_socket.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass

            left = request_deadline - time.monotonic()
            if left <= 0:
                raise TimeoutError("request deadline")
            # The timer also bounds slow-dripped HTTP headers and bodies. A
            # socket timeout alone would reset for each successful small read.
            timer = threading.Timer(left, interrupt)
            timer.daemon = True
            timer.start()
            connection.request(method, path, body=body, headers=headers)
            response = connection.getresponse()
            row["status"] = response.status
            if 300 <= response.status < 400:
                raise ValueError("redirect refused")
            lengths = [value for key, value in response.getheaders() if key.lower() == "content-length"]
            if lengths and (len(lengths) != 1 or not re.fullmatch(r"[0-9]+", lengths[0]) or int(lengths[0]) > LIMIT):
                raise ValueError("invalid response length")
            chunks, size = [], 0
            while True:
                self.remaining()
                if time.monotonic() >= request_deadline:
                    raise TimeoutError("request deadline")
                chunk = response.read1(min(64 << 10, LIMIT + 1 - size))
                if not chunk:
                    break
                chunks.append(chunk)
                size += len(chunk)
                if size > LIMIT:
                    raise ValueError("response limit")
            raw = b"".join(chunks)
            if (time.monotonic() >= request_deadline or (lengths and len(raw) != int(lengths[0]))):
                raise ValueError("incomplete or late HTTP response")
            row.update({"response_bytes": len(raw), "response_sha256": hashlib.sha256(raw).hexdigest()})
            self._bodies[row["response_file"]] = raw
            return response.status, raw
        except Exception:
            row["error"] = "bounded service HTTP exchange failed"
            with self._lock:
                self._problem = row["error"]
            raise ValueError(row["error"]) from None
        finally:
            if timer is not None:
                timer.cancel()
            connection.close()

    def _pump(self):
        try:
            with selectors.DefaultSelector() as selector:
                for name, pipe in (("stdout", self._process.stdout), ("stderr", self._process.stderr)):
                    os.set_blocking(pipe.fileno(), False)
                    selector.register(pipe, selectors.EVENT_READ, name)
                while selector.get_map() and not self._stop.is_set():
                    if time.monotonic() >= self.deadline:
                        raise ValueError("capture deadline")
                    for key, _ in selector.select(0.05):
                        with self._lock:
                            allowance = LIMIT - len(self._streams[key.data])
                        try:
                            chunk = os.read(key.fd, min(64 << 10, allowance + 1))
                        except BlockingIOError:
                            continue
                        if not chunk:
                            selector.unregister(key.fileobj)
                            continue
                        with self._lock:
                            self._streams[key.data].extend(chunk[:allowance])
                        if len(chunk) > allowance:
                            raise ValueError("capture limit")
        except Exception:
            with self._lock:
                self._problem = "bounded service capture failed"

    def run(self, observe):
        if threading.current_thread() is not threading.main_thread():
            raise ValueError("bounded service must run on the main thread")
        previous_handler = signal.getsignal(signal.SIGALRM)
        previous_timer = signal.getitimer(signal.ITIMER_REAL)
        alarm_started = time.monotonic()
        if previous_timer[0]:
            self.deadline = min(self.deadline, alarm_started + previous_timer[0])
            self.work_deadline = min(self.work_deadline, self.deadline - 5)
        work_left = self.work_deadline - alarm_started
        if work_left <= 0:
            raise ValueError("insufficient original deadline for service cleanup")

        def deadline(_signal, _frame):
            raise ServiceDeadline("native service original work deadline")

        signal.signal(signal.SIGALRM, deadline)
        # Bound even an observer that blocks without calling snapshot/http. Keep
        # the outer collector timer's absolute expiry when restoring it below.
        signal.setitimer(signal.ITIMER_REAL, work_left)
        try:
            return self._observe_and_reap(observe)
        finally:
            # The inner finally disarms only ordinary observation. Restore the
            # outer absolute expiry AFTER bounded teardown has killed/reaped the
            # owned group; it must not interrupt that teardown and leak a child.
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous_handler)
            if previous_timer[0]:
                left = previous_timer[0] - (time.monotonic() - alarm_started)
                signal.setitimer(signal.ITIMER_REAL, max(0.001, left), previous_timer[1])

    def _observe_and_reap(self, observe):
        record = {"probe": self.label, "argv": self.argv, "cwd": str(self.cwd), "error": None}
        result, thread, complete, forced = None, None, False, False
        cleanup = {"shutdown_acknowledged": False, "sigterm_sent": False, "forced_kill": False,
                   "group_reaped": False, "port_closed": False}
        try:
            self._process = subprocess.Popen(self.argv, cwd=self.cwd, env=self.env, stdin=subprocess.DEVNULL,
                                             stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                             start_new_session=True, preexec_fn=child_limits)
            thread = threading.Thread(target=self._pump, daemon=True)
            thread.start()
            result = observe(self)
            self.remaining()
            if not self._ready or not self._shutdown:
                raise ValueError("incomplete observation")
            complete = True
        except Exception:
            record["error"] = "service observation failed; private streams quarantined"
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            cleanup["shutdown_acknowledged"] = self._shutdown
            if self._process is not None:
                process = self._process
                end = min(self.deadline, time.monotonic() + 5)
                # Native shutdown may finish its HTTP listener before the CLI
                # foreground process. Permit its ordinary SIGTERM handler only.
                try:
                    process.wait(timeout=max(0.001, min(0.15, end - time.monotonic())))
                except subprocess.TimeoutExpired:
                    pass
                if group_alive(process.pid):
                    try:
                        os.killpg(process.pid, signal.SIGTERM)
                        cleanup["sigterm_sent"] = True
                    except ProcessLookupError:
                        pass
                while time.monotonic() < end:
                    process.poll()
                    if not group_alive(process.pid):
                        cleanup["group_reaped"] = True
                        break
                    time.sleep(0.02)
                if not cleanup["group_reaped"]:
                    forced = cleanup["forced_kill"] = True
                    kill_group(process)
                try:
                    process.wait(timeout=0.5)
                except subprocess.TimeoutExpired:
                    pass
                if thread is not None:
                    thread.join(timeout=0.2)
                    if thread.is_alive():
                        self._problem = "service pipes did not close after group cleanup"
                self._stop.set()
                if thread is not None and thread.is_alive():
                    thread.join(timeout=0.2)
                for pipe in (process.stdout, process.stderr):
                    if pipe is not None:
                        pipe.close()
                if self._port is not None:
                    try:
                        cleanup["port_closed"] = port_closed(self._port)
                    except OSError:
                        cleanup["port_closed"] = False
                if (not complete or forced or process.returncode != 0 or not cleanup["port_closed"]
                        or not cleanup["group_reaped"] or (thread is not None and thread.is_alive())
                        or self._problem or time.monotonic() > self.deadline):
                    complete = False
                    record["error"] = "service observation or graceful cleanup failed; private streams quarantined"
            else:
                complete = False
        with self._lock:
            streams = {name: bytes(body) if complete else b"[private service stream quarantined]\n"
                       for name, body in self._streams.items()}
        for name, body in streams.items():
            record.update({name + "_file": self.label + "." + name + ".log", name + "_bytes": len(body),
                           name + "_sha256": hashlib.sha256(body).hexdigest()})
        record.update({"exit": self._process.returncode if self._process else None,
                       "elapsed_seconds": round(time.monotonic() - self.started, 3), "cleanup": cleanup})
        return record, streams["stdout"], streams["stderr"], result if complete else None, self._http if complete else [], self._bodies if complete else {}
