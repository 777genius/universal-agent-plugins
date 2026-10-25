"""Exercise finite supervision with inert Python children, never vendor binaries."""
import hashlib
import json
import signal
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch

import project_native_research as research
import project_native_service as live

TOKEN = "a" * 43
CHILD = "import signal,time; signal.signal(signal.SIGTERM,lambda *_:exit(0)); print('ready '+'a'*43,flush=True); time.sleep(30)"


class Response:
    status = 200

    def __init__(self, body=b'{"ok":true}'):
        self.body = body

    def getheaders(self):
        return [("Content-Length", str(len(self.body)))]

    def read1(self, count):
        result, self.body = self.body[:count], self.body[count:]
        return result


class Connection:
    response = Response
    instances = []

    def __init__(self, host, port, timeout):
        self.host, self.port, self.timeout = host, port, timeout
        self.sock, self.closed = self, False
        self.interrupted = threading.Event()
        self.__class__.instances.append(self)

    def connect(self):
        pass

    def request(self, method, path, body=None, headers=None):
        self.request_value = method, path, body, headers

    def getresponse(self):
        return self.response()

    def shutdown(self, _how):
        self.interrupted.set()

    def close(self):
        self.closed = True


class ServiceTests(unittest.TestCase):
    def service(self, root, child=CHILD, timeout=10):
        return live.Service("inert-service", [sys.executable, "-u", "-c", child], root, {}, timeout, [TOKEN.encode()])

    def ready(self, service):
        while True:
            stdout, _ = service.snapshot()
            if b"ready " + TOKEN.encode() + b"\n" in stdout:
                break
            time.sleep(0.01)
        service.bind_origin("http://127.0.0.1:54321")
        service.capture_ready()

    def test_completed_observation_requires_zero_exit_and_graceful_owned_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary, patch.object(live, "port_closed", return_value=True):
            service = self.service(Path(temporary))

            def observe(current):
                self.ready(current)
                current.mark_shutdown()
                return {"observed": True}

            record, stdout, stderr, result, requests, bodies = service.run(observe)
        self.assertIsNone(record["error"])
        self.assertEqual(record["exit"], 0)
        self.assertTrue(record["cleanup"]["group_reaped"])
        self.assertTrue(record["cleanup"]["sigterm_sent"])
        self.assertFalse(record["cleanup"]["forced_kill"])
        self.assertEqual(result, {"observed": True})
        captures = {record["stdout_file"]: stdout, record["stderr_file"]: stderr}
        admitted = research.evidence({"commands": [record], "http_requests": requests}, captures, [TOKEN.encode()])
        self.assertTrue(all(TOKEN.encode() not in body for body in admitted.values()))
        self.assertIn(b"[redacted]", admitted["inert-service.stdout.log"])
        self.assertEqual(bodies, {})

    def test_unknown_startup_and_private_exception_never_publish_raw_streams(self):
        with tempfile.TemporaryDirectory() as temporary, patch.object(live, "port_closed", return_value=True):
            service = self.service(Path(temporary))

            def observe(current):
                while TOKEN.encode() not in current.snapshot()[0]:
                    time.sleep(0.01)
                raise ValueError("unrecognized bearer " + TOKEN)

            record, stdout, stderr, result, requests, bodies = service.run(observe)
        self.assertIn("quarantined", record["error"])
        self.assertIsNone(result)
        self.assertNotIn(TOKEN, json.dumps(record))
        self.assertNotIn(TOKEN.encode(), stdout + stderr)
        self.assertEqual(requests, [])
        self.assertEqual(bodies, {})

    def test_unacknowledged_shutdown_and_live_port_cannot_pass(self):
        for acknowledge, closed in ((False, True), (True, False)):
            with self.subTest(acknowledge=acknowledge, closed=closed), tempfile.TemporaryDirectory() as temporary, patch.object(live, "port_closed", return_value=closed):
                service = self.service(Path(temporary))

                def observe(current):
                    self.ready(current)
                    if acknowledge:
                        current.mark_shutdown()
                    return "not sufficient"

                record, stdout, stderr, result, _, _ = service.run(observe)
            self.assertIsNotNone(record["error"])
            self.assertIsNone(result)
            self.assertNotIn(TOKEN.encode(), stdout + stderr)

    def test_ignored_sigterm_is_forced_cleanup_failure(self):
        child = CHILD.replace("lambda *_:exit(0)", "signal.SIG_IGN")
        with tempfile.TemporaryDirectory() as temporary, patch.object(live, "port_closed", return_value=True):
            service = self.service(Path(temporary), child, timeout=6)

            def observe(current):
                self.ready(current)
                current.mark_shutdown()

            record, stdout, stderr, result, _, _ = service.run(observe)
        self.assertTrue(record["cleanup"]["forced_kill"])
        self.assertIsNotNone(record["error"])
        self.assertIsNone(result)
        self.assertNotIn(TOKEN.encode(), stdout + stderr)

    def test_stream_overflow_is_detected_live_and_quarantined(self):
        child = "import sys; sys.stdout.write('x' * %d); sys.stdout.flush()" % (live.LIMIT + 1)
        with tempfile.TemporaryDirectory() as temporary:
            service = self.service(Path(temporary), child)

            def observe(current):
                while True:
                    current.snapshot()
                    time.sleep(0.01)

            record, stdout, stderr, _, _, _ = service.run(observe)
        self.assertIsNotNone(record["error"])
        self.assertLess(len(stdout + stderr), 128)
        self.assertTrue(record["cleanup"]["group_reaped"])

    def test_blocked_callback_is_interrupted_without_extending_outer_deadline(self):
        previous_handler = signal.getsignal(signal.SIGALRM)
        previous_timer = signal.getitimer(signal.ITIMER_REAL)
        handler = lambda *_: self.fail("outer timer unexpectedly fired")
        signal.signal(signal.SIGALRM, handler)
        signal.setitimer(signal.ITIMER_REAL, 20)
        started = time.monotonic()
        try:
            with tempfile.TemporaryDirectory() as temporary:
                service = self.service(Path(temporary), timeout=6)
                record, stdout, stderr, result, _, _ = service.run(lambda _service: time.sleep(10))
            left, interval = signal.getitimer(signal.ITIMER_REAL)
            self.assertIs(signal.getsignal(signal.SIGALRM), handler)
            self.assertEqual(interval, 0)
            self.assertLess(left, 19.1)
            self.assertGreater(left, 17)
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, previous_handler)
            if previous_timer[0]:
                signal.setitimer(signal.ITIMER_REAL, max(0.001, previous_timer[0] - (time.monotonic() - started)), previous_timer[1])
        self.assertLess(time.monotonic() - started, 2)
        self.assertIsNotNone(record["error"])
        self.assertIsNone(result)
        self.assertTrue(record["cleanup"]["group_reaped"])
        self.assertNotIn(TOKEN.encode(), stdout + stderr)

    def test_http_is_fixed_loopback_bounded_and_keeps_bearer_out_of_receipts(self):
        service = self.service(Path("/tmp"))
        for origin in ("http://localhost:54321", "https://127.0.0.1:54321", "http://127.0.0.1:54321/path", "http://127.0.0.1:22"):
            with self.assertRaises(ValueError):
                service.bind_origin(origin)
        service.bind_origin("http://127.0.0.1:54321")
        service.capture_ready()
        with patch.object(live.http.client, "HTTPConnection", Connection):
            status, body = service.http("health", "GET", "/api/v1/healthz", token=TOKEN)
        self.assertEqual((status, body), (200, b'{"ok":true}'))
        connection = Connection.instances[-1]
        self.assertEqual((connection.host, connection.port), ("127.0.0.1", 54321))
        self.assertEqual(connection.request_value[3]["Authorization"], "Bearer " + TOKEN)
        self.assertTrue(connection.closed)
        self.assertNotIn(TOKEN, json.dumps(service._http))
        self.assertEqual(service._http[0]["response_sha256"], hashlib.sha256(body).hexdigest())
        with self.assertRaises(ValueError):
            service.http("health", "GET", "/api/v1/healthz")
        for path in ("https://outside/api/v1/meta", "//outside/api/v1/meta", "/api/v1/meta#fragment", "/api/v1/meta?token=" + TOKEN):
            with self.assertRaises(ValueError):
                service.http("other", "GET", path)

    def test_http_rejects_redirect_and_oversized_body_before_admission(self):
        class Redirect(Response):
            status = 302

        class Oversized(Response):
            def getheaders(self):
                return [("Content-Length", str(live.LIMIT + 1))]

        for response in (Redirect, Oversized):
            with self.subTest(response=response):
                service = self.service(Path("/tmp"))
                service.bind_origin("http://127.0.0.1:54321")
                service.capture_ready()
                with patch.object(live.http.client, "HTTPConnection", Connection), patch.object(Connection, "response", response):
                    with self.assertRaisesRegex(ValueError, "HTTP exchange failed"):
                        service.http("refused", "GET", "/api/v1/meta")
                self.assertEqual(service._bodies, {})
                with self.assertRaises(ValueError):
                    service.remaining()

    def test_request_timer_bounds_slow_headers_instead_of_resetting_deadline(self):
        class Slow(Connection):
            def getresponse(self):
                if not self.interrupted.wait(1):
                    raise AssertionError("request timer did not interrupt held socket")
                raise OSError("private diagnostics " + TOKEN)

        service = self.service(Path("/tmp"))
        service.bind_origin("http://127.0.0.1:54321")
        service.capture_ready()
        started = time.monotonic()
        with patch.object(live.http.client, "HTTPConnection", Slow):
            with self.assertRaisesRegex(ValueError, "^bounded service HTTP exchange failed$"):
                service.http("slow", "GET", "/api/v1/meta", timeout=0.05)
        self.assertLess(time.monotonic() - started, 0.5)
        self.assertNotIn(TOKEN, json.dumps(service._http))

    def test_http_evidence_identity_and_redaction_are_mandatory(self):
        body = json.dumps({"private": TOKEN}).encode()
        row = {"response_file": "service-meta.http.json", "response_bytes": len(body),
               "response_sha256": hashlib.sha256(body).hexdigest()}
        summary = {"commands": [], "http_requests": [row]}
        admitted = research.evidence(summary, {row["response_file"]: body}, [TOKEN.encode()])
        self.assertTrue(all(TOKEN.encode() not in value for value in admitted.values()))
        with self.assertRaisesRegex(ValueError, "receipt differs"):
            research.evidence(summary, {row["response_file"]: body + b" "}, [TOKEN.encode()])


if __name__ == "__main__":
    unittest.main()
