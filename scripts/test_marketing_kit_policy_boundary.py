"""Signed Marketing Kit boundary checks tolerate unrelated Directory grants."""
import copy
import runpy
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


RUNNER = runpy.run_path(str(Path(__file__).with_name("marketing-kit-project-release-e2e.py")))


class MarketingPolicyBoundaryContracts(unittest.TestCase):
    def policy(self, prepared=True):
        targets = {client: {"scopes": ["user"], "delivery": "managed"}
                   for client in ("codex", "cursor", "copilot", "claude", "gemini", "opencode")}
        # Original failed job 137006042061 observed these additional signed49
        # grants. They do not alter Copilot ownership or VS Code saved support.
        targets.update({client: {"scopes": ["user"], "delivery": "managed"}
                        for client in ("hermes", "antigravity", "pi")})
        if prepared:
            targets["vscode"] = {"scopes": ["user"], "delivery": "prepared"}
        return {"source_mode": "production_default", "snapshot_sequence": 49,
                "snapshot_digest": "sha256:f54f905f55d05477ee01ae6028a8d32e10b8b8db58784f81f624642ae71f2452",
                "root_target_compatibility": targets,
                "closure": copy.deepcopy(RUNNER["FIXED_SIGNED_CLOSURE"])}

    def test_expanded_authenticated_map_selects_same_exact_boundary(self):
        for prepared in (False, True):
            with self.subTest(prepared=prepared):
                policy = self.policy(prepared)
                expected = "saved_management_capability" if prepared else "signed_grant_refusal"
                self.assertEqual(RUNNER["current_vscode_boundary_kind"](policy), expected)
                policy["root_target_compatibility"]["another-client"] = {
                    "scopes": ["user"], "delivery": "managed"}
                self.assertEqual(RUNNER["current_vscode_boundary_kind"](policy), expected)

    def test_required_and_optional_boundary_contracts_still_fail_closed(self):
        for mutation in ("missing-required", "required-scope", "required-delivery", "vscode-scope",
                         "vscode-delivery", "extra-scope", "extra-delivery", "extra-field",
                         "extra-empty-client", "targets-list", "source", "bool-sequence", "pin"):
            with self.subTest(mutation=mutation):
                policy = self.policy()
                targets = policy["root_target_compatibility"]
                if mutation == "missing-required": targets.pop("opencode")
                elif mutation == "required-scope": targets["copilot"]["scopes"] = ["project"]
                elif mutation == "required-delivery": targets["copilot"]["delivery"] = "prepared"
                elif mutation == "vscode-scope": targets["vscode"]["scopes"] = ["project"]
                elif mutation == "vscode-delivery": targets["vscode"]["delivery"] = "managed"
                elif mutation == "extra-scope": targets["pi"]["scopes"] = ["project"]
                elif mutation == "extra-delivery": targets["pi"]["delivery"] = "prepared"
                elif mutation == "extra-field": targets["pi"]["unexpected"] = True
                elif mutation == "extra-empty-client": targets[""] = targets.pop("pi")
                elif mutation == "targets-list": policy["root_target_compatibility"] = list(targets)
                elif mutation == "source": policy["source_mode"] = "fixed_immutable"
                elif mutation == "bool-sequence": policy["snapshot_sequence"] = True
                else: policy["closure"]["777genius/notion"]["manifest_digest"] = "sha256:" + "0" * 64
                with self.assertRaises(AssertionError):
                    RUNNER["current_vscode_boundary_kind"](policy)

    def test_signed49_saved_boundary_executes_exact_released_refusal(self):
        verify = RUNNER["verify_current_vscode_user_boundary"]
        policy = self.policy()
        diagnostic = ("agentplugins: target vscode cannot be saved; retry agentplugins add "
                      "marketing-kit --target vscode --no-save\n")
        with TemporaryDirectory() as directory:
            env, project, *_ = RUNNER["run_case"]("unused", "copilot", Path(directory).resolve(), "E")
            calls, guards = [], []
            def invoke(argv, *_):
                calls.append(argv)
                return {"exit_code": 1, "timeout": False, "stdout_tail": "",
                        "stderr_tail": diagnostic, "_json": None}
            with patch.dict(verify.__globals__, {
                    "verify_pinned_launcher": lambda *_: None,
                    "observe_production_policy": lambda *_: copy.deepcopy(policy),
                    "invoke": invoke, "inventory": lambda *_: []}):
                proof = verify("published-cli", env, project, policy, (lambda: guards.append(True),))
            self.assertEqual(calls, [["published-cli", "add", "marketing-kit", "--target", "vscode",
                                      "--scope", "user", "--yes", "--format", "json"]])
            self.assertEqual(proof["contract"], "saved_management_capability")
            self.assertEqual(proof["exit_code"], 1)
            self.assertEqual(proof["policy"], policy)
            self.assertEqual(proof["native_editor_runtime"], "not_qualified")
            self.assertTrue(guards)

    def test_expanded_map_still_requires_full_policy_continuity(self):
        observe = RUNNER["observe_production_policy"]
        for change in (False, "remove-extra", "add-extra"):
            with self.subTest(change=change), TemporaryDirectory() as directory:
                env, project, *_ = RUNNER["run_case"]("unused", "copilot", Path(directory).resolve(), "E")
                expected = self.policy()
                observed = copy.deepcopy(expected)
                observed.pop("source_mode")
                if change == "remove-extra": observed["root_target_compatibility"].pop("pi")
                elif change == "add-extra": observed["root_target_compatibility"]["other"] = {
                    "scopes": ["user"], "delivery": "managed"}
                with patch.dict(observe.__globals__, {"inspect_signed_closure": lambda *_: observed}):
                    if change:
                        with self.assertRaisesRegex(AssertionError, "policy observation changed"):
                            observe("published-cli", env, project, "copilot", expected)
                    else:
                        self.assertEqual(observe("published-cli", env, project, "copilot", expected), expected)


if __name__ == "__main__":
    unittest.main()
