"""Inert Cursor publisher transport admission; no network or vendor process."""
import gzip
import hashlib
import io
import os
from pathlib import Path
import tarfile
import unittest
from unittest.mock import patch

import project_native_cursor_acquisition as transport
import project_native_research as research


class CursorPublisherTransportTests(unittest.TestCase):
    def setUp(self):
        self.spec = {"url": transport.URL, "format": "tar.gz", "max_bytes": 4096,
                     "transport": transport.TRANSPORT}

    def probe(self, body=b"inert", *, version=b"curl 8.5.0 (inert)\n", mutate=None,
              result=None, output=None):
        def run(argv, cwd, env, directory, label, **limits):
            self.assertEqual(env, {"PATH": "/usr/bin:/bin", "HOME": str(cwd), "LANG": "C", "LC_ALL": "C"})
            self.assertEqual(cwd, directory)
            self.assertEqual(cwd.stat().st_mode & 0o777, 0o700)
            self.assertEqual(limits["limit"], 16384)
            self.assertIs(limits["memory_only"], True)
            self.assertLessEqual(limits["timeout"], 90)
            if label == "cursor-curl-version":
                self.assertEqual(argv, ["/usr/bin/curl", "-q", "--version"])
                return {"error": None, "exit": 0}, version, b""
            self.assertEqual(label, "cursor-curl-download")
            target = Path(argv[argv.index("--output") + 1])
            self.assertEqual(target.parent, cwd)
            self.assertEqual(target.stat().st_mode & 0o777, 0o600)
            self.assertEqual(target.read_bytes(), b"")
            self.assertEqual(argv[:6], ["/usr/bin/curl", "-q", "-fSL", "--silent", "--proto", "=https"])
            self.assertEqual(argv[argv.index("--proto-redir") + 1], "=https")
            self.assertEqual(argv[argv.index("--max-redirs") + 1], "0")
            self.assertEqual(argv[argv.index("--max-filesize") + 1], str(self.spec["max_bytes"]))
            self.assertEqual(argv[-1], transport.URL)
            target.write_bytes(body)
            if mutate is not None:
                mutate(target)
            return result or {"error": None, "exit": 0}, output or b"200\n" + transport.URL.encode() + b"\n", b""
        return run

    def test_supervised_exact_transport_has_closed_environment_and_removes_private_file(self):
        held = []
        original = self.probe()
        def record(*args, **kwargs):
            held.append(args[1])
            return original(*args, **kwargs)
        with patch.dict(os.environ, {"HTTPS_PROXY": "forbidden", "CURL_HOME": "forbidden", "CURSOR_API_KEY": "forbidden"}), patch.object(transport, "run_probe", side_effect=record):
            self.assertEqual(transport.download(self.spec, research.Budget()), (b"inert", transport.URL))
        self.assertTrue(held)
        self.assertTrue(all(not root.exists() for root in held))

    def test_old_curl_failure_redirect_or_process_error_cannot_yield_archive(self):
        variants = ({"version": b"curl 8.3.0 (inert)\n"}, {"version": b"unknown\n"},
                    {"result": {"error": None, "exit": 22}, "output": b"403\n" + transport.URL.encode() + b"\n"},
                    {"result": {"error": "deadline", "exit": -9}},
                    {"output": b"200\nhttps://user:password@other.invalid/archive\n"},
                    {"output": b"302\n" + transport.URL.encode() + b"\n"})
        for variant in variants:
            with self.subTest(variant=variant), patch.object(transport, "run_probe", side_effect=self.probe(**variant)):
                with self.assertRaises(ValueError):
                    transport.download(self.spec, research.Budget())

    def test_replaced_linked_or_oversized_output_is_refused(self):
        def replace(target):
            other = target.with_name("replacement")
            other.write_bytes(b"inert")
            other.chmod(0o600)
            other.replace(target)
        def symlink(target):
            target.unlink()
            target.symlink_to("missing")
        def hardlink(target):
            os.link(target, target.with_name("alias"))
        for mutate in (replace, symlink, hardlink, lambda target: target.write_bytes(b"x" * 4097)):
            with self.subTest(mutate=mutate), patch.object(transport, "run_probe", side_effect=self.probe(mutate=mutate)):
                with self.assertRaises(ValueError):
                    transport.download(self.spec, research.Budget())

    def test_existing_size_digest_and_inventory_gates_still_apply(self):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w:gz") as archive:
            member = tarfile.TarInfo("dist-package/cursor-agent")
            member.size = 5
            archive.addfile(member, io.BytesIO(b"inert"))
        body = output.getvalue()
        self.spec.update(integrity="sha256:" + hashlib.sha256(body).hexdigest(), expected_bytes=len(body))
        with patch.object(transport, "run_probe", side_effect=self.probe(body)):
            receipt, contents = research.acquire(self.spec, research.Budget())
            self.assertIs(receipt["pin_verified"], True)
            self.assertEqual(contents, {"dist-package/cursor-agent": b"inert"})
        for changed in ({"expected_bytes": len(body) + 1}, {"integrity": "sha256:" + "0" * 64}, {"max_expanded_bytes": 4}):
            with self.subTest(changed=changed), patch.object(transport, "run_probe", side_effect=self.probe(body)):
                with self.assertRaises(ValueError):
                    research.acquire({**self.spec, **changed}, research.Budget())

    def test_unregistered_transport_is_refused_before_child_and_urllib_remains_default(self):
        with patch.object(transport, "run_probe", side_effect=AssertionError("child")):
            for changed in ({"url": "https://x.ai/other.tar.gz"}, {"transport": "unknown"}):
                with self.subTest(changed=changed), self.assertRaises(ValueError):
                    research.acquire({**self.spec, **changed}, research.Budget())
        body = gzip.compress(b"inert")
        response = io.BytesIO(body)
        response.url = "https://x.ai/inert.gz"
        with patch.object(transport, "download", side_effect=AssertionError("Cursor transport")), patch.object(research.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = response
            _, contents = research.acquire({"url": response.url, "format": "gzip", "max_bytes": 4096}, research.Budget())
            self.assertEqual(contents, {"payload": b"inert"})


if __name__ == "__main__":
    unittest.main()
