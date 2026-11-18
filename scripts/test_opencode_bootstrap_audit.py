"""The eleven-leaf exception must fail closed on changed bytes or authority."""
from contextlib import ExitStack
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
from tempfile import TemporaryDirectory
import unittest
from unittest import mock


def load_audit():
    path = Path(__file__).with_name("live-install-audit.py")
    spec = importlib.util.spec_from_file_location("opencode_live_audit", path)
    audit = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(audit)
    return audit


@unittest.skipUnless(os.name == "posix", "OpenCode bootstrap proof requires POSIX descriptor custody")
class OpenCodeBootstrapAuditTest(unittest.TestCase):
    def setUp(self):
        self.audit = load_audit()
        self.helper = self.audit.OPENCODE_BOOTSTRAP_AUDIT
        self.namespace = self.helper["qualify"].__globals__
        if not self.helper["supported"]():
            self.skipTest("required no-follow descriptor primitives unavailable")
        self.stack = ExitStack()
        self.addCleanup(self.stack.close)
        self.root = Path(self.stack.enter_context(TemporaryDirectory())).resolve() / "opencode"
        self.root.mkdir()
        self.version = {"status": "reported", "value": "1.18.34"}
        self.bundle = "node_modules/effect/dist/unstable/httpapi/internal/httpApiScalar.js"
        self.link = "node_modules/.bin/uuid"
        self.target = "node_modules/uuid/dist-node/bin/uuid"
        self.link_text = "../uuid/dist-node/bin/uuid"
        self.bodies = {
            "package.json": b'{\n  "dependencies": {\n    "@opencode-ai/plugin": "1.18.34"\n  }\n}\n',
            "node_modules/@opencode-ai/plugin/package.json":
                b'{"name":"@opencode-ai/plugin","version":"1.18.34"}\n',
            "node_modules/effect/package.json": b'{"name":"effect","version":"4.0.0-beta.83"}\n',
            "node_modules/uuid/package.json": b'{"name":"uuid","version":"14.0.3"}\n',
            self.bundle: b"// bounded vendor source\n" * 50000,
            self.target: b"// bounded vendor executable\n",
        }
        for relative, body in self.bodies.items():
            path = self.root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
        (self.root / self.link).parent.mkdir()
        (self.root / self.link).symlink_to(self.link_text)
        # Small deterministic unit data exercise the same reader. Production
        # pins are separately checked against the genuine native/official bytes.
        packages = dict(self.namespace["PACKAGE_PINS"])
        for name in ("@opencode-ai/plugin", "effect", "uuid"):
            body = self.bodies["node_modules/" + name + "/package.json"]
            packages[name] = (packages[name][0], *self.pin(body))
        bundles = dict(self.namespace["BUNDLES"])
        bundles[self.bundle] = ("effect", *self.pin(self.bodies[self.bundle]))
        links = dict(self.namespace["BIN_LINKS"])
        links[self.link] = ("uuid", self.link_text, self.target, *self.pin(self.bodies[self.target]))
        self.stack.enter_context(mock.patch.dict(self.namespace, {
            "PACKAGE_PINS": packages, "BUNDLES": bundles, "BIN_LINKS": links}))

    @staticmethod
    def pin(body):
        return len(body), hashlib.sha256(body).hexdigest()

    def scan(self, identity="docker-hub", names=(), observed=None, version="default"):
        proof = []
        residuals = self.audit.native_identity_residuals(
            [self.root], identity,
            observed if observed is not None else ["opencode/" + path for path in (self.link, self.bundle)],
            "opencode", names, client_version=self.version if version == "default" else version,
            bootstrap_proof=proof)
        return residuals, proof

    def test_exact_selected_bytes_are_read_and_bounded_proof_is_reported(self):
        residuals, proof = self.scan()
        self.assertEqual(residuals, [])
        self.assertEqual(len(proof), 2)
        self.assertTrue(all(row["source_bytes_fully_read"] and not row["symlink_followed"] for row in proof))
        self.assertEqual(proof[0]["link_text"], self.link_text)
        self.assertEqual(proof[0]["regular_target"], self.target)
        self.assertGreater(proof[1]["bytes"], 1 << 20)

    def test_native_version_and_platform_are_required(self):
        for version in (None, {}, {"status": "reported", "value": "1.18.35"},
                        {"status": "probe-failed", "value": "1.18.34"}):
            with self.subTest(version=version):
                residuals, proof = self.scan(version=version)
                self.assertEqual(len(residuals), 2)
                self.assertEqual(proof, [])
        with mock.patch.dict(self.namespace, {"supported": lambda: False}):
            self.assertEqual(len(self.scan()[0]), 2)

    def test_main_carries_observed_native_version_to_the_actual_case_wrapper(self):
        cli = self.root.parent / "inert-cli"
        cli.write_text("unit fixture only; never executed")
        observed = []

        def capture(*args):
            observed.append(args[-1])
            return {"status": "passed"}

        with mock.patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                mock.patch("sys.argv", ["audit", "--cli", str(cli), "--target", "opencode",
                                        "--kind", "plugin", "--limit", "1",
                                        "--report-dir", str(self.root.parent)]), \
                mock.patch.dict(self.audit.AGENT_BINARIES, {"opencode": str(cli)}), \
                mock.patch.object(self.audit, "version_evidence", side_effect=[
                    {"status": "reported", "value": "agentplugins 0.1.95"}, self.version]), \
                mock.patch.object(self.audit, "_do_case", side_effect=capture), \
                mock.patch("builtins.print"):
            self.assertEqual(self.audit.main(), 0)
        self.assertEqual(observed, [self.version])

    def test_modified_short_or_oversized_bundle_never_uses_generic_fallback(self):
        path = self.root / self.bundle
        for body in (b"truncated", self.bodies[self.bundle][:-1] + b"!", b"x" * ((4 << 20) + 1)):
            with self.subTest(bytes=len(body)):
                path.write_bytes(body)
                residuals, _ = self.scan()
                self.assertIn("opencode/" + self.bundle + ": unverified-native-bootstrap", residuals)

    def test_target_bytes_and_all_manifests_are_authorities(self):
        for relative in (self.target, "package.json", "node_modules/@opencode-ai/plugin/package.json",
                         "node_modules/uuid/package.json"):
            with self.subTest(path=relative):
                path = self.root / relative
                original = path.read_bytes()
                path.write_bytes(original + b" ")
                residuals, _ = self.scan()
                self.assertIn("opencode/" + self.link + ": unverified-native-bootstrap", residuals)
                path.write_bytes(original)

    def test_link_cannot_escape_or_point_at_another_equal_file(self):
        link = self.root / self.link
        foreign = self.root.parent / "foreign"
        foreign.write_bytes(self.bodies[self.target])
        for target in (str(foreign), "../../../foreign", "../uuid/package.json"):
            with self.subTest(target=target):
                link.unlink()
                link.symlink_to(target)
                residuals, _ = self.scan()
                self.assertIn("opencode/" + self.link + ": unverified-native-bootstrap", residuals)
                self.assertEqual(foreign.read_bytes(), self.bodies[self.target])

    def test_redirected_parent_or_target_and_hardlinks_are_refused(self):
        target = self.root / self.target
        target.unlink()
        foreign = self.root.parent / "foreign"
        foreign.write_bytes(self.bodies[self.target])
        target.symlink_to(foreign)
        self.assertEqual(len(self.scan()[0]), 1)
        target.unlink()
        os.link(foreign, target)
        self.assertEqual(len(self.scan()[0]), 1)
        target.unlink()
        target.write_bytes(self.bodies[self.target])
        owner = self.root / "node_modules/uuid"
        moved = self.root.parent / "foreign-owner"
        owner.rename(moved)
        owner.symlink_to(moved, target_is_directory=True)
        self.assertEqual(len(self.scan()[0]), 1)

    def test_equal_byte_target_replacement_during_open_is_refused(self):
        target = self.root / self.target
        original_open = os.open
        changed = []

        def replace(name, flags, *args, **kwargs):
            descriptor = original_open(name, flags, *args, **kwargs)
            if name == "uuid" and not flags & os.O_DIRECTORY and not changed:
                target.rename(target.with_name("held-original"))
                target.write_bytes(self.bodies[self.target])
                changed.append(True)
            return descriptor

        with mock.patch.dict(self.namespace, {"supported": lambda: True}), mock.patch.object(os, "open", replace):
            residuals, _ = self.scan()
        self.assertEqual(changed, [True])
        self.assertIn("opencode/" + self.link + ": unverified-native-bootstrap", residuals)

    def test_equal_byte_ancestor_replacement_during_read_is_refused(self):
        owner = self.root / "node_modules/uuid"
        original_open = os.open
        changed = []

        def replace(name, flags, *args, **kwargs):
            descriptor = original_open(name, flags, *args, **kwargs)
            if name == "uuid" and not flags & os.O_DIRECTORY and not changed:
                old = owner.with_name("uuid-original")
                owner.rename(old)
                shutil.copytree(old, owner)
                changed.append(True)
            return descriptor

        with mock.patch.dict(self.namespace, {"supported": lambda: True}), mock.patch.object(os, "open", replace):
            residuals, _ = self.scan()
        self.assertEqual(changed, [True])
        self.assertIn("opencode/" + self.link + ": unverified-native-bootstrap", residuals)

    def test_native_identity_and_component_names_take_precedence_over_pins(self):
        for identity, names in (("effect", ()), ("docker-hub", ("uuid",)),
                                ("@opencode-ai/plugin", ()), ("4.0.0-beta.83", ()), ("", ())):
            with self.subTest(identity=identity, names=names):
                self.assertTrue(self.scan(identity, names)[0])

    def test_qualified_identity_inside_pinned_fixture_content_is_still_rejected(self):
        body = b"identity: docker-hub@agentplugins-0123456789ab\n"
        (self.root / self.target).write_bytes(body)
        links = dict(self.namespace["BIN_LINKS"])
        links[self.link] = ("uuid", self.link_text, self.target, *self.pin(body))
        with mock.patch.dict(self.namespace, {"BIN_LINKS": links}):
            self.assertIn("opencode/" + self.link + ": unverified-native-bootstrap", self.scan()[0])

    def test_unknown_objects_and_active_config_keep_original_failure_classes(self):
        extra = {
            "node_modules/.bin/unknown": ": symlink",
            "node_modules/effect/unknown.js": ": unscanned-too-large",
            "opencode.json": "",
        }
        (self.root / "node_modules/.bin/unknown").symlink_to(self.link_text)
        (self.root / "node_modules/effect/unknown.js").write_bytes(b"x" * ((1 << 20) + 1))
        (self.root / "opencode.json").write_text(json.dumps({"mcp": {"docker-hub": {"enabled": True}}}))
        observed = ["opencode/" + path for path in (*extra, self.link, self.bundle)]
        residuals, proof = self.scan(observed=observed)
        self.assertEqual(residuals, sorted("opencode/" + path + suffix for path, suffix in extra.items()))
        self.assertEqual(len(proof), 2)


if __name__ == "__main__":
    unittest.main()
