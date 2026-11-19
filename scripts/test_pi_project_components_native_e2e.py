"""Inert Pi raw-Skills fixture contracts; never starts a native or Go child."""
import copy
from pathlib import Path
import unittest
from unittest import mock

import pi_project_components_native_e2e as fixture


class PiRawSkillsContracts(unittest.TestCase):
    def ready(self):
        return {"schema_version": 1, "command": "skills add", "result": "success", "readiness": "ready",
                "next_actions": []}

    def inventory(self, scope="project"):
        base = Path("/owned/project/.pi") if scope == "project" else Path("/owned/home/.pi/agent")
        skill = base / "skills/uap-pi-selected/SKILL.md"
        row = {"name": "skill:uap-pi-selected", "description": "Inert selected Pi Skill", "source": "skill",
               "sourceInfo": {"path": str(skill), "source": "auto", "scope": scope, "origin": "top-level",
                              "baseDir": str(base)}}
        receipt = {"passed": True, "closure_proved": True,
                   "response": {"type": "response", "command": "get_commands", "success": True,
                                "data": {"commands": [row]}}}
        return receipt, skill, base

    @mock.patch.object(fixture.subprocess, "run", side_effect=AssertionError("inert contract started subprocess"))
    def test_ready_requires_actual_public_raw_success(self, _run):
        fixture.pi_raw_ready(self.ready())
        for key, value in (("command", "add"), ("readiness", "needs_action"), ("result", "failure"),
                           ("next_actions", [{"code": "verify_native"}]), ("schema_version", 2), ("schema_version", True)):
            with self.subTest(key=key):
                response = self.ready()
                response[key] = value
                with self.assertRaises(RuntimeError):
                    fixture.pi_raw_ready(response)

    @mock.patch.object(fixture.subprocess, "run", side_effect=AssertionError("inert contract started subprocess"))
    def test_exact_auto_project_and_user_origins(self, _run):
        for scope in ("project", "user"):
            with self.subTest(scope=scope):
                receipt, skill, base = self.inventory(scope)
                self.assertEqual(len(fixture.pi_raw_inventory(receipt, skill, scope, base)), 1)
                for field, wrong in (("path", "/foreign/SKILL.md"), ("scope", "temporary"), ("source", "cli"),
                                     ("origin", "package"), ("baseDir", "/foreign")):
                    altered = copy.deepcopy(receipt)
                    altered["response"]["data"]["commands"][0]["sourceInfo"][field] = wrong
                    with self.subTest(field=field), self.assertRaises(RuntimeError):
                        fixture.pi_raw_inventory(altered, skill, scope, base)

    def test_native_closure_and_absence_are_required(self):
        receipt, skill, base = self.inventory()
        for field in ("passed", "closure_proved"):
            altered = copy.deepcopy(receipt)
            altered[field] = False
            with self.subTest(field=field), self.assertRaises(RuntimeError):
                fixture.pi_raw_inventory(altered, skill, "project", base)
        with self.assertRaises(RuntimeError):
            fixture.pi_raw_inventory(receipt, None, "project", base)
        receipt["response"]["data"]["commands"] = []
        fixture.pi_raw_inventory(receipt, None, "project", base)
        with self.assertRaises(RuntimeError):
            fixture.pi_raw_inventory(receipt, skill, "project", base)

    def test_duplicate_selected_rows_are_not_native_proof(self):
        receipt, skill, base = self.inventory()
        rows = receipt["response"]["data"]["commands"]
        rows.append(copy.deepcopy(rows[0]))
        with self.assertRaises(RuntimeError):
            fixture.pi_raw_inventory(receipt, skill, "project", base)

    def test_existing_native_case_and_public_raw_cases_remain_required(self):
        import ast
        source = Path(fixture.__file__).read_text()
        tree = ast.parse(source)
        main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "main")
        calls = [node for node in ast.walk(main) if isinstance(node, ast.Call)]
        self.assertTrue(any(isinstance(node.func, ast.Name) and node.func.id == "pi_raw_skills_case" for node in calls))
        self.assertIn('state["mcp_tools_call_qualified"] = True', source)
        self.assertIn('for scope in ("project", "user")', source)
        self.assertIn('inventories("repeat", True)', source)
        self.assertIn('if repeated != first:', source)
        # The helper cannot silently replace ordinary native startup with an approval override.
        raw = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "pi_raw_skills_case")
        literals = [node.value for node in ast.walk(raw) if isinstance(node, ast.Constant) and isinstance(node.value, str)]
        self.assertNotIn("--approve", literals)
        self.assertNotIn("--no-approve", literals)


if __name__ == "__main__":
    unittest.main()

