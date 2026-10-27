"""Inert trusted project scanner handoffs; no vendor, acquisition or UID spoofing."""
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("vscode_project_test", ROOT / "research-vscode-project.py")
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)


class TrustedProjectScannerTests(unittest.TestCase):
    def test_actual_acquisition_uses_portable_source_pin_and_unique_member(self):
        member = "lintai-v0.1.3-x86_64-unknown-linux-musl/lintai"
        receipt = {"members": [{"kind": "file", "name": member}]}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "scanner" / member
            contents, budget = object(), object()
            with mock.patch.object(collector.shared, "acquire", return_value=(receipt, contents)) as acquire:
                with mock.patch.object(collector.shared, "materialize", return_value=(binary, {binary: "a" * 64})) as materialize:
                    self.assertEqual(collector.acquire_scanner(root, budget), (binary, "a" * 64, receipt))
            actual_spec, actual_budget = acquire.call_args.args
            self.assertIs(actual_budget, budget)
            self.assertEqual(actual_spec["url"], "https://github.com/777genius/lintai/releases/download/v0.1.3/lintai-v0.1.3-x86_64-unknown-linux-musl.tar.gz")
            self.assertEqual(actual_spec["integrity"], "sha256:3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b")
            self.assertEqual(actual_spec["entrypoint"], member)
            materialize.assert_called_once_with(root / "scanner", actual_spec, receipt, contents, selected_only=True)

    def test_historical_gnu_pin_refuses_before_acquisition(self):
        historical = collector.publisher.native.PINS["linux-amd64"]["lintai"]
        with mock.patch.object(collector.publisher.native, "current_source_scanner_pin", return_value=historical):
            with mock.patch.object(collector.shared, "acquire") as acquire:
                with self.assertRaisesRegex(ValueError, "reviewed scanner archive differs"):
                    collector.acquire_scanner(Path("/inert"), object())
                acquire.assert_not_called()

    def test_ambiguous_or_missing_scanner_member_refuses_before_materialization(self):
        for members in ([], [{"kind": "file", "name": "one/lintai"}, {"kind": "file", "name": "two/lintai"}]):
            with self.subTest(members=members):
                with mock.patch.object(collector.shared, "acquire", return_value=({"members": members}, object())):
                    with mock.patch.object(collector.shared, "materialize") as materialize:
                        with self.assertRaisesRegex(ValueError, "absent or ambiguous"):
                            collector.acquire_scanner(Path("/inert"), object())
                        materialize.assert_not_called()


if __name__ == "__main__":
    unittest.main()
