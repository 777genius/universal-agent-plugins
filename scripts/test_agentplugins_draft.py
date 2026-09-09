"""Offline draft verification: mock all gh calls, use the real byte verifier."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "draft", Path(__file__).with_name("verify-agentplugins-draft.py"))
draft = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(draft)
SOURCE = Path(__file__).resolve().parents[1]


class DraftTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        source = self.root / "source"
        script = source / "npm/agentplugins/scripts/release-assets.js"
        script.parent.mkdir(parents=True)
        shutil.copyfile(SOURCE / "npm/agentplugins/scripts/release-assets.js", script)
        (script.parent.parent / "THIRD_PARTY_NOTICES.txt").write_text("fixture notices\n")
        self.assets = self.root / "assets"
        self.assets.mkdir()
        for target in ("darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64",
                       "windows_amd64.exe", "windows_arm64.exe"):
            (self.assets / f"agentplugins_1.2.3_{target}").write_text(f"fixture {target}\n")
        self.args = argparse.Namespace(repository="777genius/universal-agent-plugins", tag="agentplugins-v1.2.3",
            commit="a" * 40, release_id=123, run_id=456, run_attempt=2,
            source=str(source), receipt=str(self.root / "receipt.json"))
        subprocess.run(["node", str(script), "prepare", str(self.assets),
                        self.args.tag, self.args.commit], check=True, capture_output=True)
        self.args.asset_set_digest = draft.sha256(self.assets / "checksums.txt")
        self.metadata = {"id": 123, "tag_name": self.args.tag, "draft": True,
                         "prerelease": False, "updated_at": "2026-09-08T00:00:00Z"}
        self.asset_metadata = [{"id": index, "name": path.name, "size": path.stat().st_size,
                               "state": "uploaded", "digest": "sha256:" + draft.sha256(path)}
                              for index, path in enumerate(sorted(self.assets.iterdir()), 1)]
        self.lookup_id = 123
        self.commit = self.args.commit
        self.reads = 0
        self.change = None
        self.attestation_failure = False
        self.calls = []
        self.real_run = draft.run

    def fake_run(self, command, **kwargs):
        self.calls.append(command)
        if command[0] == "node":
            self.assertEqual(command[:3], ["node", str(Path(self.args.source) /
                "npm/agentplugins/scripts/release-assets.js"), "verify"])
            self.assertEqual(command[4:], [self.args.tag, self.args.commit])
            return self.real_run(command, stderr=subprocess.PIPE, **kwargs)
        self.assertEqual(command[0], "gh")
        if command[1:3] == ["attestation", "verify"]:
            self.assertEqual(command[4:], ["--repo", "777genius/universal-agent-plugins", "--signer-workflow",
                "github.com/777genius/universal-agent-plugins/.github/workflows/agentplugins-release.yml",
                "--source-digest", self.args.commit])
            if self.attestation_failure:
                raise subprocess.CalledProcessError(1, command)
            return b"verified"
        self.assertEqual(command[1], "api")
        # A visibility-capable token must never turn this verifier into an API writer.
        self.assertFalse(set(command) & {"-X", "--method", "--input"})
        endpoint = command[2]
        if endpoint != "graphql":
            self.assertFalse(set(command) & {"-f", "-F", "--field", "--raw-field"})
        if endpoint != "graphql":
            self.assertTrue(endpoint.startswith("repos/777genius/universal-agent-plugins/"))
        if endpoint == "graphql":
            self.assertEqual(command[3:5], ["-f",
                "query=query($owner:String!,$name:String!,$tag:String!){"
                "repository(owner:$owner,name:$name){release(tagName:$tag){databaseId}}}"])
            self.assertEqual(len(command), 11)
            self.assertIn("owner=777genius", command)
            self.assertIn("name=universal-agent-plugins", command)
            self.reads += 1
            if self.reads == 2 and self.change:
                self.change()
            release = None if self.lookup_id is None else {"databaseId": self.lookup_id}
            data = {"data": {"repository": {"release": release}}}
        elif "/commits/" in endpoint:
            data = {"sha": self.commit}
        elif "/releases/assets/" in endpoint:
            asset = next(item for item in self.asset_metadata if item["id"] == int(endpoint.rsplit("/", 1)[1]))
            return (self.assets / asset["name"]).read_bytes()
        elif endpoint.endswith("/assets?per_page=100"):
            self.assertEqual(command[3:], ["--paginate", "--slurp"])
            data = [self.asset_metadata[:4], self.asset_metadata[4:]]
        elif endpoint.endswith("/releases/123"):
            data = self.metadata
        else:
            self.fail(f"unexpected gh operation: {command}")
        return json.dumps(data).encode()

    def verify(self):
        with patch.object(draft, "run", side_effect=self.fake_run):
            return draft.verify(self.args)

    def rejects(self):
        with self.assertRaises((ValueError, KeyError, FileNotFoundError,
                                subprocess.CalledProcessError)):
            self.verify()
        self.assertFalse(Path(self.args.receipt).exists())

    def test_success_binds_all_nine_subjects_and_rereads(self):
        result = self.verify()
        self.assertEqual(self.reads, 2)
        self.assertEqual(len(result["subjects"]), 9)
        self.assertEqual(result["producer_run_attempt"], 2)
        self.assertEqual(result["release"]["id"], 123)
        self.assertEqual(result["repository"], "777genius/universal-agent-plugins")
        self.assertEqual(result["signer_workflow"],
            "github.com/777genius/universal-agent-plugins/.github/workflows/agentplugins-release.yml")
        self.assertEqual(result["asset_set_digest"], self.args.asset_set_digest)
        self.assertEqual(json.loads(Path(self.args.receipt).read_text()), result)
        self.assertEqual(sum(c[1:3] == ["attestation", "verify"] for c in self.calls), 9)

    def test_invalid_inputs_fail_before_network(self):
        for field, value in (("repository", "other/repo"),
                             ("repository", "777genius/plugin-kit-ai"),
                             ("tag", "agentplugins-v1.2.3-rc1"),
                             ("commit", "main"), ("asset_set_digest", "ABC"),
                             ("release_id", 0), ("run_id", 0), ("run_attempt", 0)):
            with self.subTest(field=field):
                args = copy.copy(self.args)
                setattr(self.args, field, value)
                self.rejects()
                self.assertEqual(self.calls, [])
                self.args = args

    def test_historical_asset_producer_identity(self):
        verified = json.loads(self.real_run([
            "node", str(Path(self.args.source) / "npm/agentplugins/scripts/release-assets.js"),
            "verify", str(self.assets), self.args.tag, self.args.commit]))
        self.assertEqual(verified["repository"], "777genius/plugin-kit-ai")
        self.assertTrue(verified["gate_eligible"])
        self.verify()

    def test_wrong_verifier_producer_or_ineligible_assets_rejected(self):
        for overrides in ({"repository": "other/repo"},
                          {"repository": "777genius/universal-agent-plugins"},
                          {"gate_eligible": False}):
            with self.subTest(overrides=overrides):
                def changed_verifier(command, **kwargs):
                    output = self.fake_run(command, **kwargs)
                    if command[0] == "node":
                        verified = json.loads(output)
                        verified.update(overrides)
                        return json.dumps(verified).encode()
                    return output
                self.calls = []
                with patch.object(draft, "run", side_effect=changed_verifier):
                    with self.assertRaisesRegex(ValueError, "ineligible release assets"):
                        draft.verify(self.args)
                self.assertFalse(Path(self.args.receipt).exists())
                self.assertFalse(any(c[1:3] == ["attestation", "verify"] for c in self.calls))

    def test_non_draft_or_wrong_identity(self):
        for field, value in (("draft", False), ("prerelease", True), ("id", 999),
                             ("tag_name", "agentplugins-v9.9.9")):
            with self.subTest(field=field):
                original = self.metadata[field]
                self.metadata[field] = value
                self.rejects()
                self.metadata[field] = original

    def test_missing_recreated_or_moved_tag(self):
        for value in (None, 999):
            self.lookup_id = value
            self.rejects()
        self.lookup_id = 123
        self.commit = "b" * 40
        self.rejects()

    def test_draft_hidden_by_api_visibility_fails_closed(self):
        self.lookup_id = None
        with patch.object(draft, "run", side_effect=self.fake_run):
            with self.assertRaisesRegex(ValueError, "release missing or recreated"):
                draft.verify(self.args)
        self.assertEqual(len(self.calls), 1)
        self.assertEqual(self.calls[0][1:3], ["api", "graphql"])
        self.assertFalse(Path(self.args.receipt).exists())

    def test_changes_during_verification(self):
        changes = [lambda: self.metadata.update(draft=False),
                   lambda: self.metadata.update(prerelease=True),
                   lambda: self.metadata.update(updated_at="later"),
                   lambda: setattr(self, "lookup_id", None),
                   lambda: setattr(self, "lookup_id", 999),
                   lambda: setattr(self, "commit", "b" * 40),
                   lambda: self.asset_metadata[0].update(id=999),
                   lambda: self.asset_metadata[0].update(updated_at="later"),
                   lambda: self.asset_metadata[0].update(digest="sha256:" + "b" * 64)]
        for change in changes:
            with self.subTest(change=change):
                metadata, assets = copy.deepcopy((self.metadata, self.asset_metadata))
                self.reads = 0
                self.change = change
                self.rejects()
                self.metadata, self.asset_metadata = metadata, assets
                self.lookup_id, self.commit = 123, self.args.commit

    def test_api_failure_on_final_read(self):
        def fail():
            raise subprocess.CalledProcessError(1, ["gh", "api"])
        self.change = fail
        self.rejects()

    def test_attestation_rejection(self):
        # gh is the signature/source/signer/subject authority; any rejection is fatal.
        self.attestation_failure = True
        self.rejects()
        self.assertEqual(self.reads, 1)

    def test_wrong_frozen_digest_and_source_notices(self):
        digest = self.args.asset_set_digest
        self.args.asset_set_digest = "b" * 64
        self.rejects()
        self.args.asset_set_digest = digest
        (Path(self.args.source) / "npm/agentplugins/THIRD_PARTY_NOTICES.txt").write_text("changed")
        self.rejects()

    def test_missing_extra_duplicate_and_traversal_assets(self):
        original = copy.deepcopy(self.asset_metadata)
        self.asset_metadata.pop()
        self.rejects()
        self.asset_metadata = copy.deepcopy(original)
        self.asset_metadata.append(dict(original[0]))
        self.rejects()
        self.asset_metadata = copy.deepcopy(original)
        self.asset_metadata[0]["name"] = "../escape"
        self.rejects()
        self.asset_metadata = copy.deepcopy(original)
        (self.assets / "extra.txt").write_text("extra")
        self.asset_metadata.append({"id": 100, "name": "extra.txt", "size": 5, "state": "uploaded"})
        self.rejects()

    def test_modified_bytes_checksums_manifest_notices(self):
        for path in self.assets.iterdir():
            with self.subTest(asset=path.name):
                data = path.read_bytes()
                path.write_bytes(b"X" * len(data))
                self.rejects()
                path.write_bytes(data)

    def test_alias_rejected_by_reused_verifier(self):
        original = self.fake_run
        def alias(command, **kwargs):
            if command[0] == "node":
                root = Path(command[3])
                path = next(root.glob("agentplugins_*"))
                data = path.read_bytes()
                path.unlink()
                external = self.root / "aliased"
                external.write_bytes(data)
                path.symlink_to(external)
            return original(command, **kwargs)
        with patch.object(draft, "run", side_effect=alias):
            with self.assertRaises(subprocess.CalledProcessError):
                draft.verify(self.args)
        self.assertFalse(Path(self.args.receipt).exists())


class ProducerPermissionTests(unittest.TestCase):
    def setUp(self):
        self.workflow = (SOURCE / ".github/workflows/agentplugins-release.yml").read_text()

    def job(self, name):
        return re.search(r"(?ms)^  " + re.escape(name) +
                         r":\n.*?(?=^  [\w-]+:\n|\Z)", self.workflow).group()

    def test_write_is_limited_to_trusted_producer_jobs(self):
        header = self.workflow.split("\njobs:\n", 1)[0]
        self.assertIn("\npermissions:\n  contents: read\n", header)
        self.assertNotIn(": write", header)
        for name in ("validate", "build", "platform-proof"):
            self.assertNotIn(": write", self.job(name))
        platform = (SOURCE / ".github/workflows/agentplugins-platform-proof.yml").read_text()
        self.assertNotIn(": write", platform)
        job = self.job("verified-draft")
        permissions, steps = job.split("    steps:\n", 1)
        self.assertIn("    permissions:\n", permissions)
        grants = re.findall(r"^      ([\w-]+): (read|write)$", permissions, re.M)
        self.assertEqual(grants, [("contents", "write"), ("attestations", "read")])
        self.assertNotIn("env:", permissions)
        self.assertEqual(job.count("GH_TOKEN:"), 1)
        self.assertIn("        env:\n          GH_TOKEN: ${{ github.token }}", steps)
        self.assertIn("          persist-credentials: false\n", steps)
        self.assertIn("          ref: ${{ needs.validate.outputs.commit }}\n", steps)
        # Only the frozen-source verifier runs; no installer, plugin, npm, or mutation step.
        runs = re.findall(r"(?ms)^        run: \|\n(.*?)(?=^      -|\Z)", steps)
        self.assertEqual(len(runs), 1)
        commands = runs[0].replace("\\\n", "").strip().splitlines()
        self.assertEqual(len(commands), 1)
        self.assertTrue(commands[0].startswith("python3 scripts/verify-agentplugins-draft.py "))
        self.assertNotRegex(commands[0], r"[;`]|&&|\|\||\$\(")
        self.assertIn('--source "${GITHUB_WORKSPACE}"', commands[0])

    def test_producer_trust_and_promotion_defaults_remain_fail_closed(self):
        self.assertRegex(self.workflow, r"publish_release:\n(?:        .*\n)*?        type: boolean\n        default: false\n")
        validate = self.job("validate")
        for guard in ('test "${WORKFLOW_REF}" = "refs/heads/main"',
                      '"${WORKFLOW_COMMIT}" != "${head_commit}"',
                      'test "${head_commit}" = "$(git rev-parse refs/remotes/origin/main)"',
                      'test "${head_commit}" = "$(git rev-list -n 1 "refs/tags/${TAG}")"',
                      '.merged_at != null and .base.ref == "main"',
                      'require_check dependency-review', 'require_check "${name}"'):
            self.assertIn(guard, validate)
        self.assertIn("needs: [validate, stage-draft, platform-proof]", self.job("verified-draft"))
        promote = self.job("promote-release")
        self.assertIn("if: ${{ inputs.publish_release == true }}", promote)
        self.assertIn("needs: [validate, stage-draft, platform-proof, verified-draft]", promote)
        for name in ("stage-draft", "verified-draft", "promote-release"):
            self.assertNotRegex(self.job(name), r"always\(\)|!cancelled\(\)|failure\(\)|continue-on-error:")
        self.assertNotIn("if:", self.job("verified-draft"))


if __name__ == "__main__":
    unittest.main()
