from __future__ import annotations

import importlib.util
import io
import json
import sys
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = (
    Path(__file__).resolve().parents[1]
    / "scripts"
    / "request_launch_runtime_observations.py"
)
sys.path.insert(0, str(MODULE_PATH.parent))
SPEC = importlib.util.spec_from_file_location(
    "request_launch_runtime_observations", MODULE_PATH
)
assert SPEC and SPEC.loader
observer_request = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(observer_request)


ENDPOINT = "https://observer.example/v1/observations"
TOKEN = "secret-oidc-token"


class FakeResponse:
    def __init__(
        self,
        body: bytes,
        *,
        url: str = ENDPOINT,
        status: int = 200,
        length: str | None = None,
    ) -> None:
        self._body = io.BytesIO(body)
        self._url = url
        self.status = status
        self.headers = {} if length is None else {"Content-Length": length}

    def read(self, size: int = -1) -> bytes:
        return self._body.read(size)

    def geturl(self) -> str:
        return self._url

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return None


class FakeOpener:
    def __init__(self, response: FakeResponse) -> None:
        self.response = response
        self.requests = []
        self.timeouts = []

    def open(self, request, timeout=None):
        self.requests.append(request)
        self.timeouts.append(timeout)
        return self.response


class RedirectingOpener:
    """Simulate urllib receiving a redirect without making a network request."""

    def __init__(self, location: str) -> None:
        self.location = location
        self.requests = []

    def open(self, request, timeout=None):
        self.requests.append(request)
        return observer_request.NoRedirect().redirect_request(
            request, None, 302, "Found", {"Location": self.location}, self.location
        )


class ObserverTransportTests(unittest.TestCase):
    def request(self, opener):
        return observer_request.request_observer_bundle(
            ENDPOINT, b'{"challenge":"abc"}', TOKEN, opener
        )

    def test_normal_success_uses_bounded_timeout_and_expected_headers(self) -> None:
        value = {"schema_version": 1, "artifacts": {}}
        opener = FakeOpener(FakeResponse(json.dumps(value).encode()))

        self.assertEqual(self.request(opener), value)

        self.assertEqual(opener.timeouts, [observer_request.REQUEST_TIMEOUT_SECONDS])
        self.assertEqual(len(opener.requests), 1)
        request = opener.requests[0]
        self.assertEqual(request.full_url, ENDPOINT)
        self.assertEqual(request.get_method(), "POST")
        self.assertEqual(request.get_header("Authorization"), "Bearer " + TOKEN)

    def test_default_opener_installs_redirect_rejection_handler(self) -> None:
        opener = FakeOpener(FakeResponse(b"{}"))
        with mock.patch.object(
            observer_request.urllib.request,
            "build_opener",
            return_value=opener,
        ) as build_opener:
            observer_request.request_observer_bundle(ENDPOINT, b"{}", TOKEN)

        handler = build_opener.call_args.args[0]
        self.assertIsInstance(handler, observer_request.NoRedirect)

    def test_rejects_every_redirect_without_leaking_token(self) -> None:
        locations = {
            "same origin": "https://observer.example/v2/observations",
            "cross origin": "https://attacker.example/steal",
            "HTTP downgrade": "http://observer.example/steal",
            "malformed": "https://[",
        }
        for label, location in locations.items():
            opener = RedirectingOpener(location)
            with self.subTest(label=label), self.assertRaisesRegex(
                observer_request.ObserverTransportError,
                "redirects are forbidden",
            ) as caught:
                self.request(opener)
            self.assertNotIn(TOKEN, str(caught.exception))
            self.assertNotIn(location, str(caught.exception))
            self.assertEqual(len(opener.requests), 1)
            self.assertEqual(
                opener.requests[0].get_header("Authorization"), "Bearer " + TOKEN
            )

    def test_rejects_unexpected_final_url_even_with_injected_opener(self) -> None:
        opener = FakeOpener(
            FakeResponse(b"{}", url="https://attacker.example/steal")
        )
        with self.assertRaisesRegex(
            observer_request.ObserverTransportError, "failed closed"
        ):
            self.request(opener)

    def test_enforces_content_length_and_streamed_size_bounds(self) -> None:
        oversized = str(observer_request.MAX_RESPONSE_BYTES + 1)
        for length in (oversized, "-1", "not-a-number"):
            with self.subTest(length=length), self.assertRaisesRegex(
                observer_request.ObserverTransportError, "size bound"
            ):
                self.request(FakeOpener(FakeResponse(b"{}", length=length)))

        with mock.patch.object(observer_request, "MAX_RESPONSE_BYTES", 4):
            with self.assertRaisesRegex(
                observer_request.ObserverTransportError, "size bound"
            ):
                self.request(FakeOpener(FakeResponse(b"12345")))

    def test_enforces_total_response_time_bound(self) -> None:
        with mock.patch.object(
            observer_request.time,
            "monotonic",
            side_effect=[0, observer_request.RESPONSE_TOTAL_SECONDS + 1],
        ):
            with self.assertRaisesRegex(
                observer_request.ObserverTransportError, "time bound"
            ):
                self.request(FakeOpener(FakeResponse(b"{}")))

    def test_transport_and_json_errors_are_sanitized(self) -> None:
        class FailingOpener:
            def open(self, request, timeout=None):
                raise OSError(
                    f"remote failure at https://attacker.example/?token={TOKEN}"
                )

        for opener in (FailingOpener(), FakeOpener(FakeResponse(b"not-json"))):
            with self.subTest(opener=type(opener).__name__), self.assertRaises(
                observer_request.ObserverTransportError
            ) as caught:
                self.request(opener)
            message = str(caught.exception)
            self.assertNotIn(TOKEN, message)
            self.assertNotIn("attacker.example", message)
            self.assertIsNone(caught.exception.__cause__)

    def test_rejects_malformed_or_non_https_initial_endpoints(self) -> None:
        for endpoint in (
            "http://observer.example/v1",
            "https://user:password@observer.example/v1",
            "https://observer.example/v1?token=secret",
            "https://observer.example:99999/v1",
            "https://[/v1",
        ):
            with self.subTest(endpoint=endpoint), self.assertRaises(
                observer_request.ObserverTransportError
            ) as caught:
                observer_request.validate_endpoint(endpoint)
            self.assertNotIn(endpoint, str(caught.exception))


if __name__ == "__main__":
    unittest.main()
