"""Offline acquisition boundaries and the actual frozen public JS validators."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
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
        result = SimpleNamespace(returncode=1, stdout=b"", stderr=b"npm error code E401\nnpm error token=private https://user:secret@registry/\n")
        with mock.patch.object(npm.subprocess, "run", return_value=result):
            with self.assertRaisesRegex(ValueError, "exit=1; codes=E401") as caught:
                npm.run(["node", "npm-cli.js"], cwd=ROOT, env={})
        self.assertNotIn("private", str(caught.exception))
        self.assertNotIn("secret", str(caught.exception))

    def test_successful_provision_returns_wrapper_and_preserves_case_environment(self):
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
            def process(command, *, cwd, env):
                argv = list(map(str, command)); calls.append(argv)
                self.assertFalse(any(k.startswith("AGENTPLUGINS_INTERNAL_PROOF_") for k in env))
                if argv[-1] == "--version": return b"12.0.2\n"
                if argv[1].endswith("npm-public-contract.js"): return b""
                if argv[1].endswith("release-assets.js"): return json.dumps(verified).encode()
                if argv[2] == "view": return b'{"dist":{"integrity":"test-integrity","shasum":"test-shasum"}}'
                if argv[2] == "pack":
                    filename = npm.PACKAGE + "-" + npm.VERSION + ".tgz"
                    path = Path(argv[argv.index("--pack-destination") + 1]) / filename
                    with tarfile.open(path, "w:gz", format=tarfile.USTAR_FORMAT) as archive:
                        for name, body in files.items():
                            info = tarfile.TarInfo("package/" + name); info.size = len(body)
                            archive.addfile(info, io.BytesIO(body))
                    return json.dumps([{"filename": filename}]).encode()
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
        pack.write_text(json.dumps([{"id": npm.PACKAGE + "@" + npm.VERSION, "name": npm.PACKAGE,
            "version": npm.VERSION, "filename": filename, "integrity": integrity, "shasum": shasum}]))
        result = self.invoke("download", pack, self.root, npm.VERSION, integrity, shasum)
        self.assertEqual(result.returncode, 0, result.stderr)
        tarball.write_bytes(body + b"mutated")
        result = self.invoke("download", pack, self.root, npm.VERSION, integrity, shasum)
        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
