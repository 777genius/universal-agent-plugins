"""Offline acquisition boundaries and the actual frozen public JS validators."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

SPEC = importlib.util.spec_from_file_location("released_source_npm", Path(__file__).with_name("released_source_npm.py"))
npm = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(npm)
ROOT = Path(__file__).resolve().parents[1]


class AcquisitionBoundaryTests(unittest.TestCase):
    def test_explicit_npm_cli_wins_over_older_path_shim(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            node = root / "node"
            node.write_bytes(b"trusted-node-fixture")
            old = root / "old/npm-cli.js"
            old.parent.mkdir()
            old.write_bytes(b"older-npm-fixture")
            selected = root / "source-audit-npm/node_modules/npm/bin/npm-cli.js"
            selected.parent.mkdir(parents=True)
            selected.write_bytes(b"npm12-fixture")
            with mock.patch.dict(os.environ, {"SOURCE_AUDIT_NPM_CLI": str(selected)}), \
                 mock.patch.object(npm.shutil, "which", side_effect=lambda name: str(node if name == "node" else old)) as discover:
                actual_node, actual_npm = npm.tools()
            self.assertEqual((actual_node, actual_npm), (node.resolve(), selected.resolve()))
            self.assertEqual(discover.call_args_list, [mock.call("node")])

    def test_explicit_npm_cli_invalid_paths_refuse_without_fallback(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            node = root / "node"
            node.write_bytes(b"trusted-node-fixture")
            directory = root / "directory/npm-cli.js"
            directory.mkdir(parents=True)
            for configured in ("npm-cli.js", "", str(root / "missing/npm-cli.js"), str(directory), str(node)):
                with self.subTest(configured=configured), \
                     mock.patch.dict(os.environ, {"SOURCE_AUDIT_NPM_CLI": configured}), \
                     mock.patch.object(npm.shutil, "which", return_value=str(node)) as discover:
                    with self.assertRaisesRegex(ValueError, "SOURCE_AUDIT_NPM_CLI"):
                        npm.tools()
                    self.assertEqual(discover.call_args_list, [mock.call("node")])

    def test_subprocess_failure_reports_only_bounded_npm_code(self):
        result = (1, b"", b"npm error code E401\nnpm error token=private https://user:secret@registry/\n")
        with mock.patch.object(npm, "capture", return_value=result):
            with self.assertRaisesRegex(ValueError, "exit=1; codes=E401") as caught:
                npm.run(["node", "npm-cli.js"], cwd=ROOT, env={})
        self.assertNotIn("private", str(caught.exception))
        self.assertNotIn("secret", str(caught.exception))

    def child(self, code):
        return [sys.executable, "-I", "-c", code]

    def test_capture_preserves_complete_success_and_drains_both_pipes(self):
        # Each stream exceeds pipe capacity; sequential draining would deadlock.
        code = ("import sys, threading; "
                "t=threading.Thread(target=lambda:sys.stderr.buffer.write(b'e'*262144)); "
                "t.start(); sys.stdout.buffer.write(bytes(range(256))*1024); t.join()")
        self.assertEqual(npm.run(self.child(code), cwd=ROOT, env=dict(os.environ)),
                         bytes(range(256)) * 1024)

    def test_exact_stream_limits_preserve_stdout_and_zero_exit_overflow_refuses(self):
        with mock.patch.object(npm, "OUTPUT_LIMIT", 65536):
            code = "import sys; sys.stdout.buffer.write(b'x'*65536); sys.stderr.buffer.write(b'e'*65536)"
            self.assertEqual(npm.run(self.child(code), cwd=ROOT, env=dict(os.environ)), b'x' * 65536)
            for stream in ("stdout", "stderr"):
                with self.subTest(stream=stream), self.assertRaisesRegex(ValueError, "output exceeds bound"):
                    npm.run(self.child(f"import sys; sys.{stream}.buffer.write(b'x'*65537)"),
                            cwd=ROOT, env=dict(os.environ))

    def test_each_capture_stream_overflow_kills_and_reaps_original_child(self):
        for stream in ("stdout", "stderr"):
            with self.subTest(stream=stream):
                children = []
                original = subprocess.Popen
                def start(*args, **kwargs):
                    process = original(*args, **kwargs)
                    children.append(process)
                    return process
                code = ("import sys, time; "
                        f"sys.{stream}.buffer.write(b'x'*65537); sys.{stream}.flush(); "
                        "time.sleep(60)")
                started = time.monotonic()
                with mock.patch.object(npm, "OUTPUT_LIMIT", 65536), \
                     mock.patch.object(npm.subprocess, "Popen", side_effect=start):
                    with self.assertRaisesRegex(ValueError, "output exceeds bound"):
                        npm.run(self.child(code), cwd=ROOT, env=dict(os.environ))
                self.assertLess(time.monotonic() - started, 5)
                self.assertEqual(len(children), 1)
                self.assertIsNotNone(children[0].returncode)
                self.assertTrue(children[0].stdout.closed)
                self.assertTrue(children[0].stderr.closed)

    def test_timeout_kills_and_reaps_original_child_without_raw_output(self):
        children = []
        original = subprocess.Popen
        def start(*args, **kwargs):
            process = original(*args, **kwargs)
            children.append(process)
            return process
        with mock.patch.object(npm, "PROCESS_TIMEOUT", 0.1), \
             mock.patch.object(npm.subprocess, "Popen", side_effect=start):
            with self.assertRaisesRegex(ValueError, "subprocess timed out"):
                npm.run(self.child("import time; time.sleep(60)"), cwd=ROOT, env=dict(os.environ))
        self.assertIsNotNone(children[0].returncode)

    def test_windows_assigns_job_before_gate_and_reuses_owned_job_cleanup(self):
        import io
        events = []
        class Gate(io.BytesIO):
            def write(self, body):
                events.append("gate")
                return super().write(body)
        process = SimpleNamespace(stdin=Gate(), stdout=io.BytesIO(b"complete"), stderr=io.BytesIO(),
                                  poll=lambda: 0, wait=lambda **kwargs: 0, returncode=0)
        job = object()
        def assign(child):
            self.assertIs(child, process)
            events.append("assign")
            return job
        def close(owned, deadline):
            self.assertIs(owned, job)
            self.assertTrue(process.stdin.closed)
            events.append("close")
            return False
        def helpers(path):
            return ({"WindowsJob": assign} if str(path).endswith("windows_owned_job.py")
                    else {"_windows_file_close": close})
        windows_os = SimpleNamespace(**{**os.__dict__, "name": "nt"})
        with mock.patch.object(npm, "os", windows_os), \
             mock.patch.object(npm, "run_path", side_effect=helpers), \
             mock.patch.object(npm.subprocess, "CREATE_NEW_PROCESS_GROUP", 512, create=True), \
             mock.patch.object(npm.subprocess, "Popen", return_value=process) as start:
            self.assertEqual(npm.run(["verifier"], cwd=ROOT, env={}), b"complete")
        self.assertEqual(events, ["assign", "gate", "close"])
        self.assertIn("--windows-owned-file-command", start.call_args.args[0])

    def test_windows_job_assignment_refusal_never_releases_gate_and_closes_pipes(self):
        import io
        gate = mock.Mock(wraps=io.BytesIO())
        process = SimpleNamespace(stdin=gate, stdout=io.BytesIO(), stderr=io.BytesIO(),
                                  poll=lambda: None, wait=mock.Mock(return_value=0), kill=mock.Mock())
        gate.closed = False
        def helpers(path):
            return ({"WindowsJob": mock.Mock(side_effect=ValueError("job assignment refused"))}
                    if str(path).endswith("windows_owned_job.py") else {})
        with mock.patch.object(npm, "os", SimpleNamespace(**{**os.__dict__, "name": "nt"})), \
             mock.patch.object(npm, "run_path", side_effect=helpers), \
             mock.patch.object(npm.subprocess, "CREATE_NEW_PROCESS_GROUP", 512, create=True), \
             mock.patch.object(npm.subprocess, "Popen", return_value=process):
            with self.assertRaisesRegex(ValueError, "job assignment refused"):
                npm.run(["verifier"], cwd=ROOT, env={})
        gate.write.assert_not_called()
        gate.close.assert_called_once()
        process.kill.assert_called_once()
        process.wait.assert_called_once()
        self.assertTrue(process.stdout.closed)
        self.assertTrue(process.stderr.closed)

    @unittest.skipUnless(os.name == "nt", "actual Windows owned job descendant cleanup")
    def test_windows_owned_job_stops_inherited_pipe_descendant(self):
        import threading
        with tempfile.TemporaryDirectory() as temporary:
            marker, born = Path(temporary) / "escaped", Path(temporary) / "born"
            child = ("from pathlib import Path; import time; Path(%r).write_text('ready'); "
                     "time.sleep(5); Path(%r).write_text('escaped')") % (str(born), str(marker))
            leader = ("import subprocess, sys; subprocess.Popen([sys.executable, '-I', '-c', "
                      + repr(child) + "])")
            before = set(threading.enumerate())
            with mock.patch.object(npm, "PROCESS_TIMEOUT", 3):
                with self.assertRaisesRegex(ValueError, "timed out"):
                    npm.run(self.child(leader), cwd=ROOT, env=dict(os.environ))
            self.assertTrue(born.exists(), "fixture must reach the actual owned descendant")
            self.assertEqual(set(threading.enumerate()), before)
            time.sleep(5.2)
            self.assertFalse(marker.exists(), "owned Windows descendant escaped job cleanup")

    @unittest.skipUnless(os.name == "posix", "POSIX owned process group observation")
    def test_leader_exit_stops_inherited_pipe_descendant_and_readers(self):
        import threading
        with tempfile.TemporaryDirectory() as temporary:
            marker = Path(temporary) / "descendant"
            child = "import time; time.sleep(.8); from pathlib import Path; Path(%r).write_text('escaped')" % str(marker)
            leader = ("import subprocess, sys; subprocess.Popen([sys.executable, '-I', '-c', "
                      + repr(child) + "])")
            before = set(threading.enumerate())
            with mock.patch.object(npm, "PROCESS_TIMEOUT", .2):
                with self.assertRaisesRegex(ValueError, "timed out"):
                    npm.run(self.child(leader), cwd=ROOT, env=dict(os.environ))
            self.assertEqual(set(threading.enumerate()), before)
            time.sleep(.9)
            self.assertFalse(marker.exists(), "owned descendant survived timeout cleanup")

    @unittest.skipUnless(os.name == "posix", "POSIX owned process group observation")
    def test_successful_leader_cannot_leave_silent_owned_descendant(self):
        child = "import time; time.sleep(60)"
        leader = ("import subprocess, sys; subprocess.Popen([sys.executable, '-I', '-c', "
                  + repr(child) + "], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)")
        with self.assertRaisesRegex(ValueError, "retained owned descendants"):
            npm.run(self.child(leader), cwd=ROOT, env=dict(os.environ))

    def test_diagnostic_codes_are_capped_and_raw_output_is_redacted(self):
        code = ("import sys; "
                "sys.stderr.write(''.join('npm error code E%03d\\n'%i for i in range(1000))); "
                "sys.stderr.write('private https://user:secret@registry/\\n'); sys.exit(7)")
        with self.assertRaisesRegex(ValueError, "exit=7; codes=") as caught:
            npm.run(self.child(code), cwd=ROOT, env=dict(os.environ))
        details = str(caught.exception).split("codes=", 1)[1].split(",")
        self.assertEqual(len(details), npm.DIAGNOSTIC_CODE_LIMIT)
        self.assertNotIn("secret", str(caught.exception))
        self.assertNotIn("private", str(caught.exception))

    def test_successful_provision_returns_wrapper_and_preserves_case_environment(self):
        self.provision_with_pack_shape(named=False)

    def test_successful_provision_accepts_exact_package_named_pack_record(self):
        self.provision_with_pack_shape(named=True)

    def provision_with_pack_shape(self, *, named):
        # Public processes are replaced at their I/O boundary only. The real
        # provision API and existing bounded archive/package inspector execute.
        # This test does not claim registry/Sigstore or hosted lifecycle proof.
        import io
        import tarfile
        draft_spec = importlib.util.spec_from_file_location("npm_test_draft", ROOT / "scripts/native_client_draft.py")
        draft = importlib.util.module_from_spec(draft_spec)
        draft_spec.loader.exec_module(draft)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = b"test binary bytes"
            asset = {"file": "agentplugins-linux-amd64", "size": len(binary), "sha256": hashlib.sha256(binary).hexdigest()}
            verified = {"version": npm.VERSION, "tag": npm.TAG, "commit": npm.COMMIT,
                        "repository": "777genius/universal-agent-plugins", "manifest_sha256": "a" * 64,
                        "assets": {"linux-amd64": asset}}
            pins = {"schema_version": 2, "version": npm.VERSION, "npm_package": npm.PACKAGE,
                    "assets": verified["assets"], "tag": npm.TAG, "repository": verified["repository"],
                    "producer": {"repository": verified["repository"], "tag": npm.TAG, "commit": npm.COMMIT,
                                 "release_manifest": {"schema_version": 2, "sha256": "a" * 64, "version": npm.VERSION}}}
            files = {"package.json": json.dumps({"name": npm.PACKAGE, "version": npm.VERSION,
                      "bin": {"agentplugins": "bin/agentplugins.js"}}).encode(),
                     "assets.json": json.dumps(pins).encode(), "bin/agentplugins.js": b"// offline fixture\n"}
            def isolated(home, case, data):
                return {"PATH": "/runtime", "HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            audit = SimpleNamespace(ROOT=ROOT, live=SimpleNamespace(isolated_client_env=isolated),
                                    native=SimpleNamespace(draft=draft))
            calls = []
            packed_bytes = []
            def process(command, *, cwd, env):
                argv = list(map(str, command)); calls.append(argv)
                self.assertFalse(any(k.startswith("AGENTPLUGINS_INTERNAL_PROOF_") for k in env))
                if argv[-1] == "--version": return b"12.0.2\n"
                if argv[1].endswith("npm-public-contract.js"):
                    if argv[2] == "download":
                        captured = Path(argv[3]).read_bytes()
                        self.assertEqual(isinstance(json.loads(captured), dict), named)
                        self.assertEqual(captured, packed_bytes[0])
                    return b""
                if argv[1].endswith("release-assets.js"): return json.dumps(verified).encode()
                if argv[2] == "view": return b'[{"dist":{"integrity":"test-integrity","shasum":"test-shasum"}}]'
                if argv[2] == "pack":
                    filename = npm.PACKAGE + "-" + npm.VERSION + ".tgz"
                    path = Path(argv[argv.index("--pack-destination") + 1]) / filename
                    with tarfile.open(path, "w:gz", format=tarfile.USTAR_FORMAT) as archive:
                        for name, body in files.items():
                            info = tarfile.TarInfo("package/" + name); info.size = len(body)
                            archive.addfile(info, io.BytesIO(body))
                    record = {"filename": filename}
                    packed_bytes.append(json.dumps({npm.PACKAGE: record} if named else [record]).encode())
                    return packed_bytes[0]
                if argv[2] == "install":
                    installed = cwd / "node_modules" / npm.PACKAGE
                    for name, body in files.items():
                        path = installed / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(body)
                    self.assertIn("--ignore-scripts", argv)
                    return b""
                if argv[2] == "audit":
                    self.assertEqual(argv[2:], ["audit", "signatures", "--json", "--include-attestations"])
                    return b'{"verified":[{"attestationBundles":[]}]}'
                if argv[-1] == "version":
                    path = Path(env["AGENTPLUGINS_CACHE_DIR"]) / npm.VERSION / "linux-amd64/agentplugins"
                    path.parent.mkdir(parents=True); path.write_bytes(binary)
                    return ("agentplugins " + npm.VERSION + "\n").encode()
                self.fail("unexpected process " + repr(argv))
            context = {"proof": {"acquisition": "public GitHub release download", "version": npm.VERSION,
                                  "tag": npm.TAG, "commit": npm.COMMIT}}
            with mock.patch.object(npm, "tools", return_value=(Path("/trusted/node"), Path("/trusted/npm-cli.js"))), \
                 mock.patch.object(npm, "run", side_effect=process):
                result = npm.provision(audit, root, "linux-amd64", context)
            self.assertEqual((root / "public-npm/pack.json").read_bytes(), packed_bytes[0])
            self.assertEqual(result["env"], {})
            self.assertEqual(result["command"], [str(Path("/trusted/node")), str(root / "public-npm/project/node_modules" / npm.PACKAGE / "bin/agentplugins.js")])
            self.assertEqual(calls[-1], [*result["command"], "version"])
            self.assertTrue(result["proof"]["lifecycle_must_use_wrapper"])

    def test_isolated_npm_environment_discards_config_tokens_and_proof_override(self):
        ambient = {"PATH": "/runtime", "HOME": "/ambient", "NPM_CONFIG_USERCONFIG": "/private",
                   "npm_config_cache": "/private", "NODE_AUTH_TOKEN": "private", "GH_TOKEN": "private",
                   "NODE_OPTIONS": "--require private", "AGENTPLUGINS_INTERNAL_PROOF_BINARY": "/private"}
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            def isolated(home, case, data):
                return {**ambient, "HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            audit = SimpleNamespace(live=SimpleNamespace(isolated_client_env=isolated))
            env = npm.npm_environment(audit, root)
            self.assertNotIn("NODE_AUTH_TOKEN", env)
            self.assertNotIn("GH_TOKEN", env)
            self.assertNotIn("NODE_OPTIONS", env)
            self.assertNotIn("AGENTPLUGINS_INTERNAL_PROOF_BINARY", env)
            self.assertNotIn("npm_config_cache", env)
            self.assertEqual(env["NPM_CONFIG_REGISTRY"], "https://registry.npmjs.org/")
            self.assertEqual(Path(env["NPM_CONFIG_USERCONFIG"]).read_text(), "registry=https://registry.npmjs.org/\n")
            self.assertEqual(Path(env["NPM_CONFIG_GLOBALCONFIG"]).read_bytes(), b"")
            self.assertTrue(Path(env["NPM_CONFIG_CACHE"]).is_relative_to(root))

    def test_wrong_public_release_is_refused_before_subprocess(self):
        with mock.patch.object(npm, "tools") as tools:
            with self.assertRaisesRegex(ValueError, "exact public release"):
                npm.provision(None, Path("/unused"), "linux-amd64", {"proof": {"version": "0.1.115"}})
            tools.assert_not_called()

    def test_npm_metadata_rejects_nonexact_record_shapes(self):
        record = {"dist": {"integrity": "test", "shasum": "test"}}
        for metadata in ([], [record, record], [None], ["record"], [[record]], [{}],
                         [{"dist": []}], None, "record"):
            with self.subTest(metadata=metadata), tempfile.TemporaryDirectory() as temp:
                audit = SimpleNamespace(ROOT=ROOT)
                release = {"proof": {"acquisition": "public GitHub release download",
                                      "tag": npm.TAG, "commit": npm.COMMIT, "version": npm.VERSION}}
                with mock.patch.object(npm, "tools", return_value=(Path("/node"), Path("/npm-cli.js"))), \
                     mock.patch.object(npm, "npm_environment", return_value={"PATH": "/runtime"}), \
                     mock.patch.object(npm, "run", side_effect=[b"12.0.2", json.dumps(metadata).encode()]) as run:
                    with self.assertRaisesRegex(ValueError, "npm metadata is not one exact package"):
                        npm.provision(audit, Path(temp), "linux-amd64", release)
                    self.assertEqual(run.call_count, 2)

    def test_wrong_npm_version_is_refused_before_public_acquisition(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            audit = SimpleNamespace(ROOT=ROOT)
            release = {"proof": {"acquisition": "public GitHub release download", "tag": npm.TAG,
                                  "commit": npm.COMMIT, "version": npm.VERSION}}
            with mock.patch.object(npm, "tools", return_value=(Path("/node"), Path("/npm-cli.js"))), \
                 mock.patch.object(npm, "npm_environment", return_value={"PATH": "/runtime"}), \
                 mock.patch.object(npm, "run", return_value=b"11.0.0\n") as run:
                with self.assertRaisesRegex(ValueError, "npm 12.0.2"):
                    npm.provision(audit, root, "linux-amd64", release)
                self.assertEqual(run.call_count, 1)
                self.assertEqual(run.call_args.args[0][-1], "--version")


@unittest.skipUnless(shutil.which("node"), "Node prerequisite unavailable")
class FrozenValidatorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.contract = ROOT / "npm/agentplugins/scripts/npm-public-contract.js"
        self.assertEqual(hashlib.sha256(self.contract.read_bytes()).hexdigest(), npm.CONTRACT_SHA256)

    def invoke(self, command, *args):
        return subprocess.run([shutil.which("node"), str(self.contract), command, *map(str, args)],
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10)

    def test_actual_audit_validator_rejects_unverified_or_missing_signatures(self):
        attestation = self.root / "attestation.json"
        attestation.write_text('{"attestations": []}')
        for record in ({"invalid": [], "missing": [], "verified": []},
                       {"invalid": [{"name": npm.PACKAGE}], "missing": [], "verified": []}):
            path = self.root / "audit.json"
            path.write_text(json.dumps(record))
            result = self.invoke("audit", path, attestation, npm.VERSION)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b"cryptographically verify", result.stderr)

    def test_actual_attestation_validator_refuses_missing_slsa_bundle(self):
        path = self.root / "attestation.json"
        path.write_text('{"attestations": []}')
        result = self.invoke("attestation", path, npm.VERSION, "sha512-" + "A" * 86 + "==", npm.TAG, npm.COMMIT)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"exactly one SLSA", result.stderr)

    def test_actual_download_validator_checks_public_tarball_bytes(self):
        body = b"offline public tarball identity fixture"
        import base64
        integrity = "sha512-" + base64.b64encode(hashlib.sha512(body).digest()).decode()
        shasum = hashlib.sha1(body).hexdigest()
        filename = npm.PACKAGE + "-" + npm.VERSION + ".tgz"
        tarball = self.root / filename
        tarball.write_bytes(body)
        pack = self.root / "pack.json"
        record = {"id": npm.PACKAGE + "@" + npm.VERSION, "name": npm.PACKAGE,
                  "version": npm.VERSION, "filename": filename, "integrity": integrity, "shasum": shasum}
        for value in ([record], {npm.PACKAGE: record}):
            with self.subTest(value=value):
                pack.write_text(json.dumps(value))
                captured = pack.read_bytes()
                tarball.write_bytes(body)
                result = self.invoke("download", pack, self.root, npm.VERSION, integrity, shasum)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(pack.read_bytes(), captured)
                tarball.write_bytes(body + b"mutated")
                result = self.invoke("download", pack, self.root, npm.VERSION, integrity, shasum)
                self.assertNotEqual(result.returncode, 0)
        tarball.write_bytes(body)
        for value in ({}, {"other-package": record}, {npm.PACKAGE: record, "other": record},
                      {npm.PACKAGE: [record]}, {npm.PACKAGE: None}, [], [record, record],
                      [[record]], record, None, "record"):
            with self.subTest(invalid=value):
                pack.write_text(json.dumps(value))
                result = self.invoke("download", pack, self.root, npm.VERSION, integrity, shasum)
                self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
