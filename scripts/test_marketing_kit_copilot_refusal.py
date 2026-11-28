"""Strict admission of the released CLI's expected Copilot refusal diagnostic."""
import copy
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import runpy
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


RUNNER = runpy.run_path(str(Path(__file__).with_name("marketing-kit-project-release-e2e.py")))
REASON = "copilot has not remembered the selected project folder; rerun installation to restore folder trust"
STAMP = "2026-11-26T18:00:00Z"
CLOCK = datetime.strptime(STAMP, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc).timestamp()
LOG_NAME = "install-20261126T180000Z-0123abcd.jsonl"


class CopilotConsentDiagnosticContracts(unittest.TestCase):
    def fixture(self, root):
        env, project, *_ = RUNNER["run_case"]("unused", "copilot", root.resolve(), "D")
        for name in ("uap.toml", "uap.lock"):
            (project / name).write_text("portable agreement\n", encoding="utf-8")
        receipts = tuple((alias, "receipt-" + alias) for alias in sorted(RUNNER["UPDATE_ALIASES"]))
        before = RUNNER["refusal_profile_snapshot"](env, project, {})
        data = Path(env["AGENTPLUGINS_HOME"])
        before["data"] = RUNNER["copilot_cold_data_snapshot"](data)
        return env, project, data, before, receipts

    def records(self, receipts):
        header = {"type": "run", "schema_version": 1, "time": STAMP,
                  "command": "agentplugins install", "version": "0.1.116",
                  "scope": "project", "attempt": 1, "os": "linux", "arch": "amd64"}
        bindings = [{"type": "binding", "alias": alias, "client": "copilot", "status": "incomplete",
                     "code": "FAILED", "summary": "The installation did not complete for this agent.",
                     "detail": REASON} for alias, _ in receipts]
        return [header, *bindings]

    def write_log(self, data, receipts, *, records=None, name=LOG_NAME, body=None):
        directory = data / "logs"
        directory.mkdir(mode=0o700)
        path = directory / name
        if body is None:
            body = "".join(json.dumps(row, separators=(",", ":")) + "\n"
                           for row in (self.records(receipts) if records is None else records))
        path.write_text(body, encoding="utf-8")
        path.chmod(0o600)
        return path

    def check(self, data, before, receipts, *, observed=None, context=True, version="0.1.116"):
        if observed is None:
            observed = RUNNER["copilot_cold_data_snapshot"](data)
        consent_log = (receipts, CLOCK + .1, CLOCK + .9) if context else None
        token = RUNNER["_CLI_VERSION"].set(version)
        try:
            RUNNER["check_refusal_data_unchanged"](data, before, observed, consent_log=consent_log)
        finally:
            RUNNER["_CLI_VERSION"].reset(token)

    def refusal(self, project, receipts):
        actions, results = [], []
        for alias, installation_id in receipts:
            pin = RUNNER["FIXED_SIGNED_CLOSURE"][RUNNER["REQUIRED_DISTRIBUTIONS"][alias]]
            actions.append({"alias": alias, "client_id": "copilot", "action": "install",
                            "package_digest": pin["tree_digest"], "revision": pin["resolved_revision"]})
            results.append({"client_id": "copilot", "operation": "install", "outcome": "incomplete",
                            "reason": REASON, "next_actions": [{"kind": "activate", "reason": REASON}],
                            "installation_id": installation_id})
        return {"exit_code": 1, "timeout": False, "_json": {
            "schema_version": 1, "command": "agentplugins install", "result": "failure",
            "outcome": "partial", "scope": "project", "scope_selection_reason": "explicit",
            "actions": actions, "results": results,
            "bindings": [{"alias": action["alias"], "client_id": "copilot",
                          "status": "incomplete", "code": "FAILED"} for action in actions],
            "selection": {"scope": "project", "scope_selection_reason": "explicit",
                          "project_root": str(project), "manifest_path": str(project / "uap.toml"),
                          "lock_path": str(project / "uap.lock")}}}

    def test_actual_refusal_orchestration_accepts_exact_writer_record(self):
        # This same call on a19856c9 reaches the original unexpected-cache-entry
        # failure; the baseline does not need any newly introduced helper.
        verify = RUNNER["verify_copilot_cold_project_refusal"]
        with TemporaryDirectory() as directory:
            env, project, data, _, receipts = self.fixture(Path(directory))
            calls, checkpoints = [], []
            def invoke(argv, actual_env, cwd, timeout):
                calls.append((argv, actual_env, cwd, timeout))
                self.write_log(data, receipts)
                checkpoints.append(RUNNER["copilot_cold_data_snapshot"](data))
                return self.refusal(project, receipts)
            token = RUNNER["_CLI_VERSION"].set("0.1.116")
            try:
                with patch.dict(verify.__globals__, {"invoke": invoke, "time": lambda: CLOCK + .5}):
                    proof = verify("published-cli", env, project)
            finally:
                RUNNER["_CLI_VERSION"].reset(token)
            self.assertEqual(len(calls), 1)
            self.assertEqual(calls[0][0], ["published-cli", "install", "--locked", "--yes", "--scope",
                                          "project", "--project-root", str(project), "--format", "json"])
            self.assertEqual(proof["result"], "expected_refusal")
            self.assertEqual(proof["reason"], "missing_remembered_folder_consent")
            self.assertFalse(proof["cold_native_restore_qualified"])
            self.assertEqual(proof["bindings"], 4)
            self.assertEqual(RUNNER["copilot_cold_data_snapshot"](data), checkpoints[0])
            self.assertTrue((data / "logs" / LOG_NAME).is_file())
            self.assertFalse((Path(env["HOME"]) / ".copilot/config.json").exists())

    def test_prefixed_sync_error_detail_cannot_replace_released_installer_error(self):
        # Released apply_package.go returns the incomplete result together with
        # its error, so coordinator.applyBinding preserves that error verbatim.
        # Its alias/client prefix belongs only to the nil-error fallback branch.
        with TemporaryDirectory() as directory:
            _, _, data, before, receipts = self.fixture(Path(directory))
            records = self.records(receipts)
            for record in records[1:]:
                self.assertEqual(record["detail"], REASON)
                record["detail"] = f'{record["alias"]}/copilot: incomplete: {REASON}'
            self.write_log(data, receipts, records=records)
            with self.assertRaisesRegex(AssertionError, "differs from verified consent refusal"):
                self.check(data, before, receipts)

    def test_valid_log_stays_in_complete_snapshot(self):
        with TemporaryDirectory() as directory:
            _, _, data, before, receipts = self.fixture(Path(directory))
            path = self.write_log(data, receipts)
            observed = RUNNER["copilot_cold_data_snapshot"](data)
            retained = copy.deepcopy(observed)
            self.check(data, before, receipts, observed=observed)
            self.assertEqual(observed, retained)
            self.assertEqual(RUNNER["copilot_cold_data_snapshot"](data), retained)
            self.assertIn("metadata:0:logs/" + LOG_NAME, observed)
            self.assertIn(data.name + "/logs/" + LOG_NAME, observed)
            self.assertEqual(path.stat().st_nlink, 1)

    def test_log_never_authorizes_failed_or_mismatched_receipts(self):
        verify = RUNNER["verify_copilot_cold_project_refusal"]
        for mutation in ("exit", "reason", "closure", "receipt-reuse", "exception"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                env, project, data, _, receipts = self.fixture(Path(directory))
                def invoke(*_):
                    self.write_log(data, receipts)
                    response = self.refusal(project, receipts)
                    if mutation == "exit": response["exit_code"] = 0
                    elif mutation == "reason": response["_json"]["results"][0]["reason"] = "other failure"
                    elif mutation == "closure": response["_json"]["actions"].pop()
                    elif mutation == "receipt-reuse":
                        response["_json"]["results"][1]["installation_id"] = receipts[0][1]
                    else: raise RuntimeError("failed command")
                    return response
                token = RUNNER["_CLI_VERSION"].set("0.1.116")
                try:
                    with patch.dict(verify.__globals__, {"invoke": invoke, "time": lambda: CLOCK + .5}):
                        with self.assertRaisesRegex(AssertionError, "unexpected cache/data entry"):
                            verify("published-cli", env, project)
                finally:
                    RUNNER["_CLI_VERSION"].reset(token)

    def test_other_callers_and_release_versions_gain_no_log_allowance(self):
        for context, version in ((False, "0.1.116"), (True, "0.1.115")):
            with self.subTest(context=context, version=version), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                self.write_log(data, receipts)
                with self.assertRaises(AssertionError):
                    self.check(data, before, receipts, context=context, version=version)

    def test_header_and_binding_changes_fail_closed(self):
        mutations = ("schema", "bool-schema", "command", "version", "scope", "attempt",
                     "bool-attempt", "os", "arch", "timestamp", "old-time", "extra-header",
                     "alias", "client", "status", "code", "summary", "detail",
                     "extra-binding", "duplicate-binding", "missing-binding", "record-order")
        for mutation in mutations:
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                records = self.records(receipts)
                if mutation == "schema": records[0]["schema_version"] = 2
                elif mutation == "bool-schema": records[0]["schema_version"] = True
                elif mutation == "command": records[0]["command"] = "agentplugins add"
                elif mutation == "version": records[0]["version"] = "0.1.115"
                elif mutation == "scope": records[0]["scope"] = "user"
                elif mutation == "attempt": records[0]["attempt"] = 2
                elif mutation == "bool-attempt": records[0]["attempt"] = True
                elif mutation == "os": records[0]["os"] = "darwin"
                elif mutation == "arch": records[0]["arch"] = "arm64"
                elif mutation == "timestamp": records[0]["time"] = "invalid"
                elif mutation == "old-time": records[0]["time"] = "2026-11-26T17:00:00Z"
                elif mutation == "extra-header": records[0]["unrelated"] = "value"
                elif mutation in ("alias", "client", "status", "code", "summary", "detail"):
                    records[1][mutation] = "unrelated"
                elif mutation == "extra-binding": records[1]["unexpected"] = "value"
                elif mutation == "duplicate-binding": records[2] = copy.deepcopy(records[1])
                elif mutation == "missing-binding": records.pop()
                else: records[1], records[2] = records[2], records[1]
                self.write_log(data, receipts, records=records)
                with self.assertRaisesRegex(AssertionError, "differs from verified consent refusal"):
                    self.check(data, before, receipts)

    def test_malformed_jsonl_and_unknown_log_names_fail(self):
        for mutation in ("duplicate-key", "invalid-json", "extra-line", "missing-newline",
                         "oversized", "unknown-name", "malformed-name", "second-log", "extra-child"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                body = "".join(json.dumps(row) + "\n" for row in self.records(receipts))
                name = LOG_NAME
                if mutation == "duplicate-key": body = body.replace('"type": "run"', '"type": "run", "type": "run"', 1)
                elif mutation == "invalid-json": body = "{not-json}\n"
                elif mutation == "extra-line": body += "{}\n"
                elif mutation == "missing-newline": body = body.rstrip("\n")
                elif mutation == "oversized": body += "x" * (17 << 10)
                elif mutation == "unknown-name": name = "foreign.jsonl"
                elif mutation == "malformed-name": name = "install-20261126T180000Z-..jsonl"
                self.write_log(data, receipts, body=body, name=name)
                if mutation in ("second-log", "extra-child"):
                    extra = LOG_NAME.replace("0123abcd", "abcdef01") if mutation == "second-log" else "foreign"
                    (data / "logs" / extra).write_text("unowned", encoding="utf-8")
                with self.assertRaises(AssertionError):
                    self.check(data, before, receipts)

    def test_matching_name_and_header_still_require_this_invocation_window(self):
        with TemporaryDirectory() as directory:
            _, _, data, before, receipts = self.fixture(Path(directory))
            records = self.records(receipts)
            records[0]["time"] = "2026-11-26T17:00:00Z"
            self.write_log(data, receipts, records=records,
                           name=LOG_NAME.replace("180000", "170000"))
            with self.assertRaisesRegex(AssertionError, "differs from verified consent refusal"):
                self.check(data, before, receipts)

    def test_unsafe_files_and_ancestors_never_become_log_allowances(self):
        for mutation in ("file-mode", "directory-mode", "file-symlink", "directory-symlink",
                         "hardlink", "fifo", "root-symlink", "owner"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                env, _, data, before, receipts = self.fixture(Path(directory))
                path = self.write_log(data, receipts)
                if mutation == "file-mode": path.chmod(0o644)
                elif mutation == "directory-mode": path.parent.chmod(0o755)
                elif mutation == "file-symlink":
                    target = data.parent / "outside-log"
                    path.rename(target)
                    path.symlink_to(target)
                elif mutation == "directory-symlink":
                    target = data.parent / "outside-logs"
                    path.parent.rename(target)
                    (data / "logs").symlink_to(target, target_is_directory=True)
                elif mutation == "hardlink": os.link(path, data.parent / "other-link")
                elif mutation == "fifo":
                    if not hasattr(os, "mkfifo"):
                        continue
                    path.unlink()
                    os.mkfifo(path, 0o600)
                elif mutation == "root-symlink":
                    target = data.parent / "outside-data"
                    data.rename(target)
                    data.symlink_to(target, target_is_directory=True)
                if mutation == "owner":
                    if not hasattr(os, "getuid"):
                        continue
                    owner = os.getuid()
                    with patch.object(os, "getuid", return_value=owner + 1):
                        with self.assertRaises(AssertionError):
                            self.check(data, before, receipts)
                else:
                    with self.assertRaises(AssertionError):
                        self.check(data, before, receipts)

    def test_existing_data_and_existing_logs_remain_protected(self):
        for mutation in ("other-bytes", "existing-log"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                sentinel = data / "foreign-user-data"
                sentinel.write_text("keep", encoding="utf-8")
                if mutation == "existing-log":
                    self.write_log(data, receipts)
                before["data"] = RUNNER["copilot_cold_data_snapshot"](data)
                if mutation == "other-bytes":
                    self.write_log(data, receipts)
                    sentinel.write_text("changed", encoding="utf-8")
                else:
                    (data / "logs" / LOG_NAME).write_text("changed", encoding="utf-8")
                with self.assertRaises(AssertionError):
                    self.check(data, before, receipts)

    def test_changed_bytes_or_identity_since_checkpoint_fail(self):
        for mutation in ("bytes", "inode", "directory-inode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                path = self.write_log(data, receipts)
                body = path.read_bytes()
                observed = RUNNER["copilot_cold_data_snapshot"](data)
                if mutation == "bytes": path.write_bytes(body + b" ")
                elif mutation == "inode":
                    replacement = data.parent / "replacement"
                    replacement.write_bytes(body)
                    replacement.chmod(0o600)
                    replacement.replace(path)
                else:
                    path.parent.rename(data.parent / "replaced-logs")
                    self.write_log(data, receipts)
                with self.assertRaises(AssertionError):
                    self.check(data, before, receipts, observed=observed)

    def test_read_time_mutations_fail_without_rebaselining(self):
        audit = RUNNER["audit"]
        read = audit["copilot_settings_read"]
        for mutation in ("bytes", "inode", "new-entry"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, _, data, before, receipts = self.fixture(Path(directory))
                path = self.write_log(data, receipts)
                observed = RUNNER["copilot_cold_data_snapshot"](data)
                def tampered_read(candidate, root):
                    result = read(candidate, root)
                    if candidate == path:
                        if mutation == "bytes": path.write_text(result[0] + " ", encoding="utf-8")
                        elif mutation == "inode":
                            replacement = data.parent / "replacement"
                            replacement.write_text(result[0], encoding="utf-8")
                            replacement.chmod(0o600)
                            replacement.replace(path)
                        else: (data / "foreign-from-reader").write_text("write", encoding="utf-8")
                    return result
                with patch.dict(audit, {"copilot_settings_read": tampered_read}):
                    with self.assertRaisesRegex(AssertionError, "changed during diagnostic custody"):
                        self.check(data, before, receipts, observed=observed)
                self.assertNotEqual(RUNNER["copilot_cold_data_snapshot"](data), observed)

    def test_unexpected_and_uncovered_diagnostics_are_bounded_and_private(self):
        with TemporaryDirectory() as directory:
            _, _, data, before, receipts = self.fixture(Path(directory))
            (data / "unexpected-entry").write_text("DO_NOT_PRINT_VALUE", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "unexpected cache/data entry") as failure:
                self.check(data, before, receipts, context=False)
            message = str(failure.exception)
            self.assertIn("metadata:0:unexpected-entry", message)
            self.assertNotIn("DO_NOT_PRINT_VALUE", message)
            self.assertNotIn(str(data), message)
            observed = copy.deepcopy(before["data"])
            observed.update({f"unsafe\nname-{number}": "DO_NOT_PRINT_VALUE" for number in range(40)})
            with self.assertRaisesRegex(AssertionError, "unexpected data state") as failure:
                self.check(data, before, receipts, observed=observed, context=False)
            message = str(failure.exception)
            self.assertNotIn("unsafe\nname", message)
            self.assertNotIn("DO_NOT_PRINT_VALUE", message)
            delta = json.loads(message.split(": ", 1)[1])
            self.assertEqual(delta["changed_key_count"], 40)
            self.assertEqual(len(delta["items"]), 20)
            self.assertEqual(delta["omitted_key_count"], 20)

    def test_cold_project_refusals_preserve_real_filesystem_and_snapshot_inputs(self):
        check = RUNNER["check_copilot_cold_project_unchanged"]
        snapshot = RUNNER["scoped_snapshot"]
        for mutation in ("existing", "unowned", "link"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as directory:
                _, project, _, _, receipts = self.fixture(Path(directory))
                protected = project / "protected.txt"
                protected.write_text("original", encoding="utf-8")
                before = snapshot([project])
                retained = copy.deepcopy(before)
                if mutation == "existing": protected.write_text("changed", encoding="utf-8")
                elif mutation == "unowned": (project / "unowned.txt").write_text("PRIVATE_CONTENT")
                else: (project / "private-link").symlink_to(project.parent / "PRIVATE_LINK_TARGET")
                after = snapshot([project])
                with patch.dict(check.__globals__, {"scoped_snapshot": lambda roots: after}):
                    with self.assertRaises(AssertionError) as raised:
                        check(project, before, receipts)
                detail = json.loads(str(raised.exception).split(": ", 1)[1])
                allowed = {"existing": {"existing-project-state"}, "unowned": {"unowned-project-state"},
                           "link": {"unsafe-project-state", "unowned-project-state"}}
                self.assertIn(detail["reason"], allowed[mutation])
                self.assertEqual(detail["delta"]["changed_key_count"], 1)
                self.assertEqual(len(detail["delta"]["items"]), 1)
                self.assertEqual(detail["delta"]["omitted_key_count"], 0)
                self.assertEqual(before, retained)
                self.assertEqual(snapshot([project]), after)
                for private in (str(project.parent), "PRIVATE_LINK_TARGET", "PRIVATE_CONTENT"):
                    self.assertNotIn(private, str(raised.exception))

    @unittest.skipUnless(hasattr(os, "mkfifo"), "FIFO fixture requires POSIX")
    def test_cold_project_special_file_reports_captured_class_without_opening_it(self):
        check = RUNNER["check_copilot_cold_project_unchanged"]
        with TemporaryDirectory() as directory:
            _, project, _, _, receipts = self.fixture(Path(directory))
            before = RUNNER["scoped_snapshot"]([project])
            alias, uid = receipts[0]
            physical = alias + "-" + hashlib.sha256(uid.encode()).hexdigest()[:12]
            package = project / ".uap/managed/clients/copilot" / physical
            package.mkdir(parents=True)
            payload = package / "special"
            os.mkfifo(payload, 0o600)
            after = RUNNER["scoped_snapshot"]([project])
            attempts = []
            original_open = os.open
            parent = package.stat()
            def guarded_open(candidate, *args, **kwargs):
                named = Path(os.fsdecode(candidate))
                directory = kwargs.get("dir_fd")
                selected = named == payload
                if named.name == payload.name and directory is not None:
                    held = os.fstat(directory)
                    selected = selected or (held.st_dev, held.st_ino) == (parent.st_dev, parent.st_ino)
                if selected:
                    attempts.append(candidate)
                    raise AssertionError("FIFO must not be opened")
                return original_open(candidate, *args, **kwargs)
            with patch.object(os, "open", side_effect=guarded_open):
                with self.assertRaises(AssertionError) as raised:
                    check(project, before, receipts)
            self.assertEqual(attempts, [])
            detail = json.loads(str(raised.exception).split(": ", 1)[1])
            self.assertIn(detail["reason"], ("unsafe-project-state", "unowned-project-state"))
            self.assertIn(detail["key_prefix"], ("metadata:0:", "file"))
            self.assertIn((detail["value_class"], detail["value_prefix"]),
                          (("string", "non-regular"), ("tuple", "fifo")))
            self.assertEqual(detail["delta"]["changed_key_count"], 1)
            self.assertEqual(detail["delta"]["omitted_key_count"], 0)
            self.assertNotIn(str(project.parent), str(raised.exception))
            self.assertEqual(RUNNER["scoped_snapshot"]([project]), after)

    def test_cold_project_singleton_cannot_omit_failure_among_many_valid_additions(self):
        check = RUNNER["check_copilot_cold_project_unchanged"]
        with TemporaryDirectory() as directory:
            _, project, _, _, receipts = self.fixture(Path(directory))
            before = RUNNER["scoped_snapshot"]([project])
            alias, uid = receipts[0]
            physical = alias + "-" + hashlib.sha256(uid.encode()).hexdigest()[:12]
            package = project / ".uap/managed/clients/copilot" / physical
            package.mkdir(parents=True)
            for number in range(80):
                (package / (str(number) + ".json")).write_text('{"valid":true}')
            link = package / "unsafe\nprivate-link"
            link.symlink_to(project.parent / "PRIVATE_TARGET")
            after = RUNNER["scoped_snapshot"]([project])
            delta = RUNNER["audit"]["copilot_snapshot_delta"]
            calls = []
            def singleton(left, right):
                calls.append((copy.deepcopy(left), copy.deepcopy(right)))
                return delta(left, right)
            with patch.dict(RUNNER["audit"], {"copilot_snapshot_delta": singleton}):
                with self.assertRaises(AssertionError) as raised:
                    check(project, before, receipts)
            self.assertGreater(len(after.keys() - before.keys()), 80)
            self.assertEqual(len(calls), 1)
            left, right = calls[0]
            self.assertEqual(left, {})
            self.assertEqual(len(right), 1)
            self.assertTrue(next(iter(right)).endswith(link.name))
            detail = json.loads(str(raised.exception).split(": ", 1)[1])
            self.assertIn(detail["key_prefix"], ("metadata:0:", "link:0:"))
            self.assertEqual(detail["delta"]["changed_key_count"], 1)
            self.assertEqual(detail["delta"]["omitted_key_count"], 0)
            self.assertLess(len(str(raised.exception)), 1400)
            self.assertNotIn("PRIVATE_TARGET", str(raised.exception))
            self.assertNotIn(str(project.parent), str(raised.exception))
            self.assertEqual(RUNNER["scoped_snapshot"]([project]), after)

    def test_cold_project_diagnostic_failures_keep_original_assertion(self):
        check = RUNNER["check_copilot_cold_project_unchanged"]
        message = "cold Copilot locked install modified native or unowned Project state"
        for failure in ("delta", "serialization"):
            with self.subTest(failure=failure), TemporaryDirectory() as directory:
                _, project, _, _, receipts = self.fixture(Path(directory))
                before = RUNNER["scoped_snapshot"]([project])
                (project / "unowned").write_text("PRIVATE_CONTENT")
                after = RUNNER["scoped_snapshot"]([project])
                def diagnostic(*args):
                    if failure == "delta": raise RuntimeError("PRIVATE_DIAGNOSTIC_FAILURE")
                    return {"unserializable": {object()}}
                with patch.dict(RUNNER["audit"], {"copilot_snapshot_delta": diagnostic}):
                    with self.assertRaises(AssertionError) as raised:
                        check(project, before, receipts)
                self.assertEqual(raised.exception.args, (message,))
                self.assertEqual(RUNNER["scoped_snapshot"]([project]), after)


if __name__ == "__main__":
    unittest.main()
