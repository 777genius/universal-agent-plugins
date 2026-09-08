"""Offline regression for Linux run34015662605/job101438843915 output."""
import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location(
    "checker", Path(__file__).with_name("check-authoring-native-results.py"))
checker = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(checker)
# Exact downloaded output events; fixture data, never native execution proof.
ACTUAL = [{'Time': '2026-09-06T06:08:38.158231092Z',
  'Action': 'output',
  'Package': 'github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills',
  'Test': 'TestSkillContainmentAndSourceGate/incomplete',
  'Output': '=== RUN   TestSkillContainmentAndSourceGate/incomplete\n'},
 {'Time': '2026-09-06T06:08:38.161863805Z',
  'Action': 'output',
  'Package': 'github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills',
  'Test': 'TestSkillContainmentAndSourceGate/incomplete',
  'Output': '--- PASS: TestSkillContainmentAndSourceGate/incomplete (0.00s)\n'}]


class UnavailableDiagnosticTests(unittest.TestCase):
    def test_actual_failing_events(self):
        for event in ACTUAL:
            with self.subTest(output=event["Output"]):
                self.assertFalse(checker.unavailable_diagnostic(
                    event["Output"], event["Test"]))

    def test_go_status_names(self):
        for name in (ACTUAL[0]["Test"], "TestCapability/platform_unavailable",
                     "TestCapability/not-available", "TestGate/unproven"):
            for line in (f"=== RUN   {name}", f"=== PAUSE {name}",
                         f"=== CONT  {name}", f"--- PASS: {name} (0.00s)",
                         f"--- FAIL: {name} (1.23s)", f"--- SKIP: {name} (0.00s)"):
                with self.subTest(line=line):
                    self.assertFalse(checker.unavailable_diagnostic("    " + line + "\n", name))

    def test_real_diagnostics_still_rejected(self):
        name = ACTUAL[0]["Test"]
        for warning in ("platform_unavailable", "not available", "not_available",
                        "not-available", "NOT AVAILABLE", "gate incomplete",
                        "gate unproven", name):
            for text in (warning, "    native_test.go:42: " + warning + "\n",
                         ACTUAL[0]["Output"] + warning + "\n" + ACTUAL[1]["Output"],
                         ACTUAL[1]["Output"].rstrip() + " " + warning):
                with self.subTest(text=text):
                    self.assertTrue(checker.unavailable_diagnostic(text, name))

    def test_structure_must_match_whole_line_and_event_identity(self):
        for event in ACTUAL:
            text, name = event["Output"], event["Test"]
            for output, test in ((text, ""), (text, "TestOther"),
                                 ("native_test.go:42: " + text, name),
                                 (text.rstrip() + " warning\n", name)):
                with self.subTest(output=output, test=test):
                    self.assertTrue(checker.unavailable_diagnostic(output, test))

    def test_test_names_are_literal(self):
        name = "TestGate/incomplete[1]"
        self.assertFalse(checker.unavailable_diagnostic(f"=== RUN   {name}\n", name))
        self.assertTrue(checker.unavailable_diagnostic("=== RUN   TestGate/incomplete1\n", name))


if __name__ == "__main__":
    unittest.main()
