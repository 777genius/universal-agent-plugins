"""Prevent the live acceptance harness from touching ambient client profiles."""
import importlib.util
import json
import os
import subprocess
import sys
import textwrap
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch


def load_audit():
    file = Path(__file__).resolve().parent / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("live_install_audit", file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class IsolatedClientEnvironmentTest(unittest.TestCase):

    def test_marketing_kit_rejects_non_success_json_even_with_zero_exit(self):
        # A zero process exit with a refusal/partial response is not lifecycle PASS.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        accepted = {"exit_code": 0, "timeout": False,
                    "_json": {"result": "success", "data": {}}}
        self.assertEqual(check(accepted, "list", "list"), accepted["_json"])
        completed = {**accepted, "_json": {"result": "success",
                                          "outcome": "completed", "data": {}}}
        self.assertEqual(check(completed, "list", "list"), completed["_json"])
        for invalid in (
                {"exit_code": 1},
                {"timeout": True},
                {"_json": None},
                {"_json": {"data": {}}},
                {"_json": {"result": "failure", "error": "not authorized"}},
                {"_json": {"result": "partial", "data": {}}},
                # 'success' must not override an explicitly incomplete outcome.
                {"_json": {"result": "success", "outcome": "pending"}},
                {"_json": {"result": "success", "outcome": "manual_action_required"}},
                {"_json": {"result": "success", "outcome": "unexpected"}},
                {"_json": {"result": "success", "outcome": 42}}):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                    check({**accepted, **invalid}, "add")

    def test_marketing_kit_add_requires_scoped_dependency_receipts(self):
        # A published add response has two JSON layers. The outer success is
        # not proof that the inner signed dependency closure was applied.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        closure = {"result": "success", "outcome": "completed",
                   "scope": "project", "actions": actions, "results": results}
        envelope = {"result": "success", "data": closure}
        wrapped = {"exit_code": 0, "timeout": False, "_json": envelope}
        self.assertEqual(check(wrapped, "add", "add", "codex", "project"), envelope)
        # A caller-less or scope-less result cannot qualify an add even if
        # every nested signed dependency action otherwise looks complete.
        for chosen_target, chosen_scope in ((None, None), ("codex", None),
                                            (None, "project")):
            with self.subTest(target=chosen_target, scope=chosen_scope):
                with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                    check(wrapped, "add", "add", chosen_target, chosen_scope)
        # A public wrapper may itself report completed, but not pending/failure.
        completed = {**envelope, "outcome": "completed"}
        self.assertEqual(check({**wrapped, "_json": completed},
                               "add", "add", "codex", "project"), completed)
        # Repeated add can legitimately converge without active actions.
        no_op = {**closure, "actions": [{**a, "action": "unchanged"} for a in actions],
                 "results": []}
        self.assertEqual(check({**wrapped, "_json": {**envelope, "data": no_op}},
                               "repeat add", "add", "codex", "project"),
                         {**envelope, "data": no_op})
        for bad in (
                *({**envelope, "outcome": status} for status in
                  ("failure", "pending", "manual_action_required", "unexpected", 42)),
                {"result": "success", "data": {}},
                {"result": "success"},
                {**envelope, "data": {**closure, "outcome": "partial"}},
                {**envelope, "data": {**closure, "result": "failure"}},
                {**envelope, "data": {**closure, "scope": "user"}},
                {**envelope, "data": {**closure, "results": results[:-1]}},
                # Duplicate receipts for two different signed child aliases
                # cannot establish that each dependency was actually applied.
                {**envelope, "data": {**closure, "results": [
                    results[0], {**results[1],
                                 "installation_id": results[0]["installation_id"]},
                    *results[2:]]}},
                {**envelope, "data": {**closure, "actions": actions[:-1]}},
                {**envelope, "data": {**closure, "actions": [
                    {**actions[0], "client_id": "claude"}, *actions[1:]]}},
        ):
            with self.subTest(bad=bad), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**wrapped, "_json": bad}, "add", "add", "codex", "project")

    def test_marketing_kit_add_receipts_match_native_scope_inventory(self):
        # Cold add plans do not have installation IDs. Even four unique
        # successful result IDs cannot prove they belong to the four installs.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        distributions = runner["REQUIRED_DISTRIBUTIONS"]
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        for scope in ("user", "project"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "data": {
                    "result": "success", "outcome": "completed",
                    "scope": scope, "actions": actions, "results": results}}
                receipt = {"exit_code": 0, "timeout": False, "_json": envelope}
                installed = {distributions[alias]: alias + "-id"
                             for alias in aliases}
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *args: receipt,
                        "inventory": lambda *args: None,
                        "identities": lambda *args: installed}):
                    args = ("add", "marketing-kit", "--target", "codex",
                            "--scope", scope)
                    if scope == "project":
                        args += ("--project-root", str(Path.cwd()),)
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "add", *args)["result"], "passed")
                    for changed in (
                            # Unique, plausible receipts swapped between
                            # dependencies were previously accepted.
                            [results[1], results[0], *results[2:]],
                            [{**results[0], "installation_id": "foreign-id"},
                             *results[1:]]):
                        bad_receipt = {**receipt, "_json": {**envelope, "data": {
                            **envelope["data"], "results": changed}}}
                        with patch.dict(execute.__globals__, {
                                "invoke": lambda *args: bad_receipt}):
                            with self.assertRaisesRegex(
                                    AssertionError, "receipt does not match"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "add", *args)

    def test_marketing_kit_unchanged_dependency_actions_match_scoped_inventory(self):
        # A no-op command must not claim four foreign durable receipts while
        # the real selected-scope inventory and signed lock remain unchanged.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        aliases = tuple(runner["REQUIRED_DISTRIBUTIONS"])
        installed = {distribution: alias + "-id"
                     for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items()}
        actions = [{"alias": alias, "client_id": "codex", "action": "unchanged",
                    "installation_id": alias + "-id"} for alias in aliases]
        for scope in ("user", "project"):
            for operation in ("add", "update"):
                with self.subTest(scope=scope, operation=operation):
                    data = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": actions, "results": []}
                    envelope = ({"result": "success", "data": data}
                                if operation == "add" else data)
                    response = {"exit_code": 0, "timeout": False, "_json": envelope}
                    args = (operation, "marketing-kit", "--target", "codex",
                            "--scope", scope)
                    if scope == "project":
                        args += ("--project-root", str(Path.cwd()),)
                    with patch.dict(execute.__globals__, {
                            "invoke": lambda *argv: response,
                            "inventory": lambda *argv: [],
                            "identities": lambda *argv: installed}):
                        self.assertEqual(execute(
                            "agentplugins", {}, Path.cwd(), "codex",
                            "no-op", *args)["result"], "passed")
                        for wrong_id in (None, "", "foreign-installation",
                                         actions[0]["installation_id"]):
                            changed = [dict(action) for action in actions]
                            changed[1]["installation_id"] = wrong_id
                            changed_data = {**data, "actions": changed}
                            bad = ({"result": "success", "data": changed_data}
                                   if operation == "add" else changed_data)
                            with patch.dict(execute.__globals__, {
                                    "invoke": lambda *argv: {
                                        **response, "_json": bad}}):
                                with self.assertRaisesRegex(
                                        AssertionError, "unchanged action does not match"):
                                    execute("agentplugins", {}, Path.cwd(),
                                            "codex", "no-op", *args)

    def test_marketing_kit_lock_requires_real_selected_scope_dependency_graph(self):
        # Four correct source keys can still conceal detached mandatory children.
        from copy import deepcopy
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["lock_ids"]
        required = runner["REQUIRED_DISTRIBUTIONS"]
        selectors = runner["REQUIRED_CHILD_SELECTORS"]
        pins = []
        for alias, distribution in required.items():
            pins.append({
                "alias": alias, "source_key": "directory:" + distribution,
                "source": {"registry": {"distribution": distribution}},
                "targets": ["codex"], "tree_digest": "sha256:" + "a" * 64,
                "manifest_digest": "sha256:" + "b" * 64,
                "required_by": ([] if alias == "marketing-kit" else
                                [{"alias": "marketing-kit",
                                  "selector": dict(selectors[alias])}]),
            })

        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            lock = root / "uap.lock"

            def check(packages, target="codex"):
                lock.write_text(json.dumps({"packages": packages}), encoding="utf-8")
                return verify(root, target)

            self.assertEqual(check(pins),
                             {"directory:" + d for d in required.values()})

            def swap_source_keys(rows):
                # The overall set remains correct, but each signed pin points
                # to a different distribution than its declared alias/source.
                rows[0]["source_key"], rows[1]["source_key"] = (
                    rows[1]["source_key"], rows[0]["source_key"])

            # The old graph check accepted this connected but reparented
            # closure even though Marketing Kit no longer directly requires
            # Notion as recorded in the signed package manifest.
            chained = deepcopy(pins)
            chained[1]["required_by"][0]["alias"] = "hubspot-crm"
            with self.assertRaisesRegex(AssertionError, "direct required dependencies"):
                check(chained)

            # A redundant child-to-child edge still leaves all three signed
            # children direct under Marketing Kit; reachability alone passes.
            extra_edge = deepcopy(pins)
            extra_edge[2]["required_by"].append({"alias": "notion", "selector": {
                "kind": "release", "value": "1"}})
            with self.assertRaisesRegex(AssertionError, "child dependency edge"):
                check(extra_edge)

            for mutate in (
                    swap_source_keys,
                    lambda rows: rows[1].update(required_by=[]),
                    lambda rows: rows[1].update(targets=["claude"]),
                    lambda rows: rows[1]["source"]["registry"].update(
                        distribution="someone/foreign"),
                    lambda rows: rows[1].update(tree_digest="sha256:invalid"),
                    lambda rows: rows[1]["required_by"][0].update(alias="missing"),
                    lambda rows: rows[1]["required_by"][0]["selector"].update(value="0.2.0"),
                    lambda rows: rows[2]["required_by"][0]["selector"].update(
                        kind="release", value="1"),
                    lambda rows: rows[3]["required_by"][0].pop("selector"),
                    lambda rows: rows[1].update(alias="marketing-kit"),
                    lambda rows: rows[0].update(required_by=[
                        {"alias": "notion", "selector": {
                            "kind": "release", "value": "1"}}]),
            ):
                with self.subTest(change=mutate):
                    bad = deepcopy(pins)
                    mutate(bad)
                    with self.assertRaises(AssertionError):
                        check(bad)
            with self.assertRaisesRegex(AssertionError, "signed lock pin"):
                check(pins, "claude")

    def test_marketing_kit_negative_locked_restore_fixture_changes_one_digest(self):
        # The refusal must originate from a genuinely corrupt pin, not a
        # missing lock, invalid command or unavailable upstream package.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        alter = runner["corrupt_locked_child_digest"]
        original = {"packages": [
            {"alias": "marketing-kit", "tree_digest": "sha256:" + "a" * 64},
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64},
            {"alias": "hubspot-crm", "tree_digest": "sha256:" + "c" * 64},
            {"alias": "playwright", "tree_digest": "sha256:" + "d" * 64}]}
        with TemporaryDirectory() as temp:
            locked = Path(temp) / "uap.lock"
            locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
            alter(locked)
            changed = json.loads(locked.read_text(encoding="utf-8"))
            self.assertEqual(
                changed["packages"][1]["tree_digest"], "sha256:0" + "b" * 63)
            self.assertEqual(changed["packages"][0], original["packages"][0])
            self.assertEqual(changed["packages"][2:], original["packages"][2:])
            for bad in ({"packages": []}, {"packages": [
                    {"alias": "notion", "tree_digest": "not-sha256"}]}):
                locked.write_text(json.dumps(bad), encoding="utf-8")
                with self.assertRaisesRegex(AssertionError, "negative.test|signed child"):
                    alter(locked)


    def test_marketing_kit_negative_lock_corruption_rejects_replaced_paths(self):
        # The fixture itself must not overwrite a foreign file between its
        # preflight lstat and the deliberate one-digest corruption.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        alter = runner["corrupt_locked_child_digest"]
        original = json.dumps({"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}) + "\n"
        for scope in ("user", "project"):
            for variant in ("replaced", "symlink", "hardlink"):
                with self.subTest(scope=scope, variant=variant), TemporaryDirectory() as temp:
                    root = Path(temp)
                    locked = root / scope / "uap.lock"
                    locked.parent.mkdir()
                    locked.write_text(original, encoding="utf-8")
                    expected_identity = locked.lstat()
                    foreign = root / "foreign"
                    if variant == "replaced":
                        locked.replace(root / "old-lock")
                        locked.write_text(original, encoding="utf-8")
                    elif variant == "symlink":
                        locked.replace(root / "old-lock")
                        foreign.write_text("foreign bytes", encoding="utf-8")
                        try:
                            locked.symlink_to(foreign)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create symlinks: {error}")
                    else:
                        try:
                            os.link(locked, foreign)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create hardlinks: {error}")
                    with self.assertRaisesRegex(AssertionError, "negative-test lock path"):
                        alter(locked, expected_identity)
                    if variant == "symlink":
                        self.assertEqual(foreign.read_text(encoding="utf-8"), "foreign bytes")
                    else:
                        self.assertEqual(locked.read_text(encoding="utf-8"), original)
                        if variant == "hardlink":
                            self.assertEqual(foreign.read_text(encoding="utf-8"), original)


    def test_marketing_kit_corrupt_locked_restore_refuses_user_and_project(self):
        # Both scopes must reach the real locked CLI refusal (not an invented
        # missing-file error), restore exact portable bytes, and run guards.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "marketing-kit", "tree_digest": "sha256:" + "a" * 64},
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for path in (home, data, project):
                    path.mkdir()
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                lock_file = (project if scope == "project" else data) / "uap.lock"
                lock_file.write_text(json.dumps(original) + "\n", encoding="utf-8")
                original_bytes = lock_file.read_bytes()
                invoked, checked = [], []

                def refuse(command, *_):
                    invoked.append(command)
                    self.assertEqual(json.loads(lock_file.read_text())["packages"][1][
                        "tree_digest"], "sha256:0" + "b" * 63)
                    return {"exit_code": 1, "timeout": False,
                            "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "b" * 64 + " does not match expected "
                                + "sha256:0" + "b" * 63)}

                def preserved():
                    checked.append(lock_file.read_bytes())

                with patch.dict(verify.__globals__, {
                        "invoke": refuse,
                        "inventory": lambda *args: [],
                        "identities": lambda *args: {}}):
                    self.assertEqual(verify("agentplugins", env, project, "codex",
                                            scope, (preserved,)), 1)
                    self.assertEqual(lock_file.read_bytes(), original_bytes)
                    self.assertEqual(checked, [original_bytes])
                    self.assertEqual(invoked[0].count("--scope"), 1)
                    self.assertEqual(invoked[0][invoked[0].index("--scope") + 1], scope)
                    self.assertEqual("--project-root" in invoked[0], scope == "project")
                    for failed in (
                            {"exit_code": 0, "stderr_tail": "digest mismatch"},
                            {"exit_code": 1, "timeout": True,
                             "stderr_tail": "digest mismatch"},
                            {"exit_code": 1, "stderr_tail": "native client unavailable"},
                            {"exit_code": 1, "stderr_tail": "launcher cache lock timed out"},
                            {"exit_code": 1, "stderr_tail": "signed lock digest mismatch"},
                            {"exit_code": 1, "stderr_tail": "verify notion pinned manifest: remote download failed"},
                            {"exit_code": 1, "stderr_tail": "verify hubspot-crm pinned manifest; signed Directory publication differs from exact lock"},
                            {"exit_code": 1, "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "a" * 64 + " does not match expected "
                                + "sha256:0" + "b" * 63)},
                            {"exit_code": 1, "stderr_tail": (
                                "preflight notion/codex: acquired package tree digest "
                                + "sha256:" + "b" * 64 + " does not match expected "
                                + "sha256:1" + "b" * 63)}):
                        with self.subTest(failed=failed), patch.dict(
                                verify.__globals__, {
                                    "invoke": lambda *args: failed}):
                            with self.assertRaisesRegex(AssertionError, "corrupt"):
                                verify("agentplugins", env, project, "codex",
                                       scope, (preserved,))
                            self.assertEqual(lock_file.read_bytes(), original_bytes)
                    self.assertEqual(len(checked), 10)

    def test_marketing_kit_refusal_allows_only_derived_own_cache_additions(self):
        from runpy import run_path
        import hashlib

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        snapshot, check = runner["refusal_profile_snapshot"], runner["check_refusal_profile_unchanged"]
        for absent in (False, True):
            with self.subTest(absent_data=absent), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                home.mkdir()
                project.mkdir()
                if not absent:
                    data.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                pin = {"tree_digest": "sha256:" + "b" * 64,
                       "manifest_digest": "sha256:" + "c" * 64}
                before = snapshot(env, project, {"packages": [pin]})
                own_before = runner["profile_snapshot"](env, project)
                if absent:
                    data.mkdir(mode=0o700)
                name, subject = next(iter(before["subjects"].items()))
                assessment = {"schema_version": 1, "subject": subject,
                              "scanner": {"id": "lintai", "version": "0.1.3"},
                              "policy": runner["REFUSAL_CACHE_POLICY"]}
                scanner = data / runner["REFUSAL_SCANNER_PATH"]
                files = {data / "directory-v1-cache.json": b"verified signed cache",
                         data / "directory-v1-cache.json.lock": b"",
                         data / name: json.dumps(assessment).encode(), scanner: b"pinned test binary"}
                for path, body in files.items():
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    path.write_bytes(body)
                    path.chmod(0o700 if path == scanner else 0o600)
                with patch.dict(check.__globals__, {
                        "REFUSAL_SCANNER_SHA256": hashlib.sha256(files[scanner]).hexdigest()}):
                    check(env, project, before)
                    # The peer guard keeps its full snapshot contract; even
                    # legitimate additions must fail if attributed to a peer.
                    with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                        runner["check_profile_unchanged"](env, project, own_before, "peer")
                    protected = snapshot(env, project, {"packages": [pin]})
                    cache = data / name
                    replacement = data / "replacement"
                    replacement.write_bytes(cache.read_bytes())
                    replacement.chmod(0o600)
                    replacement.replace(cache)
                    with self.assertRaisesRegex(AssertionError, "existing data entry"):
                        check(env, project, protected)

    def test_marketing_kit_refusal_rejects_state_native_foreign_and_unknown_cache(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        mutations = (
            "data/state-v2.json", "data/store/package", "data/managed/package",
            "data/operations-v2/receipt", "data/uap.toml", "data/uap.lock",
            "home/.codex/config.toml", "project/.uap/state-v2.json",
            "home/foreign.txt", "project/foreign.txt",
            "data/security/unknown", "data/security/assessments/" + "a" * 64 + ".json",
            "data/security/assessments/.assessment-temp.tmp",
            "data/security/lintai/0.1.4/linux-amd64/lintai",
        )
        for relative in mutations:
            with self.subTest(path=relative), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for directory in (home, data, project):
                    directory.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                path = root / relative
                if path.name in ("foreign.txt", "uap.toml", "uap.lock"):
                    path.write_text("original")
                before = runner["refusal_profile_snapshot"](env, project, {})
                path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                path.write_text("unexpected")
                path.chmod(0o600)
                with self.assertRaises(AssertionError):
                    runner["check_refusal_profile_unchanged"](env, project, before)

    def test_marketing_kit_refusal_rejects_unsafe_cache_entries(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for mutation in ("symlink-file", "symlink-directory", "fifo", "scanner",
                         "assessment-subject", "assessment-policy", "mode", "root-mode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, data, project = (root / name for name in ("home", "data", "project"))
                for directory in (home, data, project):
                    directory.mkdir(mode=0o700)
                env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                pin = {"tree_digest": "sha256:" + "b" * 64,
                       "manifest_digest": "sha256:" + "c" * 64}
                before = runner["refusal_profile_snapshot"](env, project, {"packages": [pin]})
                if mutation == "root-mode":
                    data.chmod(0o755)
                elif mutation == "symlink-directory":
                    outside = root / "outside"
                    outside.mkdir()
                    try:
                        (data / "security").symlink_to(outside, target_is_directory=True)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create symlinks: {error}")
                elif mutation.startswith("assessment-"):
                    name, subject = next(iter(before["subjects"].items()))
                    path = data / name
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    body = {"schema_version": 1, "subject": subject,
                            "scanner": {"id": "lintai", "version": "0.1.3"},
                            "policy": dict(runner["REFUSAL_CACHE_POLICY"])}
                    if mutation == "assessment-subject":
                        body["subject"] = {**subject, "tree_digest": "sha256:" + "d" * 64}
                    else:
                        body["policy"]["version"] = 99
                    path.write_text(json.dumps(body))
                    path.chmod(0o600)
                else:
                    path = data / (runner["REFUSAL_SCANNER_PATH"] if mutation == "scanner"
                                   else "directory-v1-cache.json")
                    for parent in reversed(path.relative_to(data).parents):
                        (data / parent).mkdir(mode=0o700, exist_ok=True)
                    if mutation == "symlink-file":
                        outside = root / "outside"
                        outside.write_text("foreign")
                        try:
                            path.symlink_to(outside)
                        except (OSError, NotImplementedError) as error:
                            self.skipTest(f"host cannot create symlinks: {error}")
                    elif mutation == "fifo":
                        if not hasattr(os, "mkfifo"):
                            self.skipTest("host cannot create FIFO")
                        os.mkfifo(path, mode=0o600)
                    else:
                        path.write_text("untrusted")
                        path.chmod(0o700 if mutation == "scanner" else 0o644)
                with self.assertRaises(AssertionError):
                    runner["check_refusal_profile_unchanged"](env, project, before)

    def test_marketing_kit_copilot_user_policy_refusal_requires_exact_diagnostic(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_copilot_user_policy_refusal"]
        diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                      '777genius/marketing-kit: release does not support complete '
                      'target set; missing vscode')
        accepted = {"exit_code": 1, "timeout": False, "stderr_tail": diagnostic + "\n",
                    "stdout_tail": "", "_json": None}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            (project / "uap.toml").write_text("retained Project agreement")
            (project / "uap.lock").write_text("retained exact Project lock")
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            before = runner["profile_snapshot"](env, project)
            commands, guards = [], []

            def invoke(command, *_):
                commands.append(command)
                return accepted

            with patch.dict(verify.__globals__, {"invoke": invoke,
                    "inventory": lambda *_: []}):
                stage = verify("agentplugins", env, project,
                               (lambda: guards.append("checked"),))
            self.assertEqual(commands, [["agentplugins", "add", "marketing-kit",
                "--target", "copilot", "--scope", "user", "--yes", "--format", "json"]])
            self.assertEqual(stage["scope"], "user")
            self.assertEqual(stage["contract"], "negative_policy")
            self.assertEqual(stage["result"], "expected_refusal")
            self.assertNotEqual(stage["result"], "passed")
            self.assertEqual(guards, ["checked"])
            self.assertEqual(runner["profile_snapshot"](env, project), before)
            for ending in ("", "\r\n"):
                with patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, "stderr_tail": diagnostic + ending},
                        "inventory": lambda *_: []}):
                    self.assertEqual(verify("agentplugins", env, project, ())["result"], "expected_refusal")
            for invalid in ({"exit_code": 0}, {"exit_code": -9}, {"exit_code": None},
                            {"exit_code": True}, {"timeout": True}, {"timeout": None},
                            {"stderr_tail": "network unavailable"},
                            {"stderr_tail": diagnostic.replace("vscode", "copilot")},
                            {"stderr_tail": diagnostic.replace('"marketing-kit"', '"notion"')},
                            {"stderr_tail": diagnostic + "\nremote download failed"},
                            {"stderr_tail": " " + diagnostic},
                            {"stderr_tail": diagnostic + " "},
                            {"stdout_tail": '{"result":"success"}'},
                            {"stdout_tail": " "}, {"stdout_tail": None},
                            {"_json": {"result": "success"}}):
                with self.subTest(invalid=invalid), patch.dict(verify.__globals__, {
                        "invoke": lambda *_: {**accepted, **invalid},
                        "inventory": lambda *_: []}):
                    checked = []
                    with self.assertRaisesRegex(AssertionError, "exact missing-vscode"):
                        verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])
                    self.assertEqual(runner["profile_snapshot"](env, project), before)

    def test_marketing_kit_copilot_user_policy_refusal_rejects_own_mutations(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_copilot_user_policy_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode')}
        for relative in ("home/.copilot/settings.json", "home/foreign.txt", "project/uap.lock",
                         "data/uap.toml", "data/uap.lock", "data/state-v2.json",
                         "data/managed/plugin", "data/store/package", "data/operations-v2/receipt",
                         "data/directory-v1-cache.json"):
            for during_inventory in (False, True):
                with self.subTest(path=relative, inventory=during_inventory), TemporaryDirectory() as temporary:
                    case = Path(temporary)
                    home, data, project = (case / name for name in ("home", "data", "project"))
                    for directory in (home, data, project):
                        directory.mkdir(mode=0o700)
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    path = case / relative
                    if path.name in ("foreign.txt", "uap.lock", "state-v2.json"):
                        path.write_text("original")
                    checked = []

                    def change():
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_text("unexpected")

                    def invoke(*_):
                        if not during_inventory:
                            change()
                        return response

                    def inventory(*_):
                        if during_inventory:
                            change()
                        return []

                    with patch.dict(verify.__globals__, {"invoke": invoke, "inventory": inventory}):
                        with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                            verify("agentplugins", env, project, (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])

    def test_marketing_kit_copilot_user_policy_refusal_checks_inventory_and_peers(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_copilot_user_policy_refusal"]
        response = {"exit_code": 1, "timeout": False, "stdout_tail": "", "stderr_tail": (
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing vscode')}
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            env, project, foreign, *_ = runner["run_case"]("unused", "copilot", case, "A")
            peer_env, peer_project, *_ = runner["run_case"]("unused", "copilot", case, "B")
            peer_before = runner["profile_snapshot"](peer_env, peer_project)
            pinned = runner["pin_foreign"]([(foreign, "unmanaged user data\n")], case)
            with patch.dict(verify.__globals__, {"invoke": lambda *_: response,
                    "inventory": lambda *_: ["unexpected"], "identities": lambda *_: {"installed": "id"}}):
                with self.assertRaisesRegex(AssertionError, "retained an installation"):
                    verify("agentplugins", env, project, ())
            marker = Path(peer_env["HOME"]) / "unexpected-native.json"

            def mutate_peer(*_):
                marker.write_text("changed")
                return response

            with patch.dict(verify.__globals__, {"invoke": mutate_peer, "inventory": lambda *_: []}):
                with self.assertRaisesRegex(AssertionError, "isolated profile changed"):
                    verify("agentplugins", env, project, (
                        lambda: runner["check_foreign"](pinned, case),
                        lambda: runner["check_profile_unchanged"](
                            peer_env, peer_project, peer_before, "User refusal"),))

    def test_marketing_kit_copilot_user_refusal_rejects_padded_real_capture(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_copilot_user_policy_refusal"]
        diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                      '777genius/marketing-kit: release does not support complete '
                      'target set; missing vscode')
        padded_stdout = 'unrelated prefix {"result":"success"}' + " " * 2200
        padded_stderr = "unrelated error\n" + " " * 2200 + diagnostic
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            home, data, project = (case / name for name in ("home", "data", "project"))
            for directory in (home, data, project):
                directory.mkdir(mode=0o700)
            env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
            for stdout, stderr in ((padded_stdout, padded_stderr),
                                   ("", padded_stderr), (padded_stdout, diagnostic)):
                with self.subTest(stdout=bool(stdout), stderr_padded=stderr != diagnostic):
                    command = [sys.executable, "-c", "import sys; "
                               f"sys.stdout.write({stdout!r}); sys.stderr.write({stderr!r}); sys.exit(1)"]
                    captured = runner["audit"]["call"](command, env, project, 10)
                    self.assertEqual(captured["exit_code"], 1)
                    self.assertIsNone(captured["_json"])
                    if stdout:
                        self.assertEqual(captured["stdout_tail"], " " * 1800)
                    if stderr == padded_stderr:
                        self.assertEqual(len(captured["stderr_tail"]), 1800)
                        self.assertTrue(captured["stderr_tail"].startswith(" "))
                    with patch.dict(verify.__globals__, {"invoke": lambda *_: captured,
                            "inventory": lambda *_: []}):
                        with self.assertRaisesRegex(AssertionError, "exact missing-vscode"):
                            verify("agentplugins", env, project, ())

    def test_marketing_kit_cleanup_exception_overrides_successful_report(self):
        from io import StringIO
        from runpy import run_path
        from types import SimpleNamespace

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        main = runner["main"]
        installed = {}
        phases = []

        class FailedCleanup:
            def __init__(self, **kwargs):
                self.inner = TemporaryDirectory(**kwargs)

            def __enter__(self):
                return self.inner.__enter__()

            def __exit__(self, exc_type, exc, traceback):
                self.inner.__exit__(exc_type, exc, traceback)
                if exc_type is not None:
                    return False  # Never mask a pre-cleanup failure.
                if phases != ["user-refusal"]:
                    raise AssertionError("cleanup fixture did not reach both scope phases")
                raise OSError("disposable cleanup failed")

        def execute(_cli, _env, project, _target, label, *args):
            if args[0] in ("add", "install"):
                for name in ("uap.toml", "uap.lock"):
                    if not (project / name).exists():
                        (project / name).write_text("retained portable agreement")
                installed[project] = {name: project.parent.name + name for name in runner["EXPECTED"]}
            elif args[0] == "remove":
                installed[project] = {}
            return {"stage": label, "exit_code": 0, "result": "passed"}

        def user_refusal(*_):
            phases.append("user-refusal")
            return {"stage": "User negative contract", "reason": "missing_vscode_grant"}

        with TemporaryDirectory() as output:
            report_path = Path(output) / "report.json"
            with patch.dict(main.__globals__, {
                    "execute": execute, "verify_pinned_launcher": lambda *_: None,
                    "inventory": lambda _cli, _env, project, *_: installed.get(project, {}),
                    "identities": lambda items, *_: items,
                    "lock_ids": lambda *_: {"directory:" + name for name in runner["EXPECTED"]},
                    "settled_profile_snapshot": runner["profile_snapshot"],
                    "verify_corrupt_lock_refusal": lambda *_: 1,
                    "verify_copilot_user_policy_refusal": user_refusal,
                    # Patch only the runner's binding, not the process-wide
                    # tempfile module used by unrelated setup helpers.
                    "tempfile": SimpleNamespace(TemporaryDirectory=FailedCleanup),
                    }), \
                    patch.object(sys, "argv", ["runner", "--target", "copilot", "--cli", "unused",
                                               "--report", str(report_path)]), \
                    patch.object(sys, "stdout", StringIO()), patch.object(sys, "stderr", StringIO()):
                self.assertEqual(main(), 1)
            report = json.loads(report_path.read_text())
            self.assertEqual(phases, ["user-refusal"])
            self.assertEqual(report["scope_results"]["project"]["status"], "passed")
            self.assertEqual(report["scope_results"]["user"]["status"], "refused_as_expected")
            self.assertFalse(report["two_scope_lifecycle_qualified"])
            self.assertEqual(report["status"], "failed")
            self.assertEqual(report["failure"], "disposable cleanup failed")

    def test_marketing_kit_corrupt_lock_restore_rejects_foreign_symlink(self):
        # A malicious native invocation may swap the fixture lock for a link to
        # an outside file. Cleanup must not follow it to write original bytes.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                with TemporaryDirectory() as external:
                    root = Path(temporary)
                    home, data, project = (root / name for name in ("home", "data", "project"))
                    for path in (home, data, project):
                        path.mkdir()
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    locked = (project if scope == "project" else data) / "uap.lock"
                    locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
                    foreign = Path(external) / "foreign.json"
                    foreign.write_bytes(b"foreign must remain intact")
                    # Skip before invoking the guard: its finally block must
                    # never mask a requested test skip on symlink-hostile OSes.
                    probe = root / "symlink-probe"
                    try:
                        probe.symlink_to(foreign)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create file symlinks: {error}")
                    else:
                        probe.unlink()
                    guards = []

                    def attacker(*_):
                        locked.unlink()
                        locked.symlink_to(foreign)
                        return {"exit_code": 1, "timeout": False,
                                "stderr_tail": ("verify notion pinned manifest; run agentplugins deps lock: "
                                            "signed Directory publication differs from exact lock")}

                    with patch.dict(verify.__globals__, {"invoke": attacker}):
                        with self.assertRaisesRegex(AssertionError, "lock path replaced"):
                            verify("agentplugins", env, project, "codex", scope,
                                   (lambda: guards.append(True),))
                    self.assertEqual(foreign.read_bytes(), b"foreign must remain intact")
                    self.assertEqual(guards, [True])

    def test_marketing_kit_corrupt_lock_restore_rejects_foreign_hardlink(self):
        # An outside hardlink to the original inode makes inode-only cleanup
        # overwrite a foreign path when the corrupt fixture is restored.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_corrupt_lock_refusal"]
        original = {"packages": [
            {"alias": "notion", "tree_digest": "sha256:" + "b" * 64}]}
        for scope in ("project", "user"):
            with self.subTest(scope=scope), TemporaryDirectory() as temporary:
                with TemporaryDirectory() as external:
                    root = Path(temporary)
                    home, data, project = (root / name
                                           for name in ("home", "data", "project"))
                    for path in (home, data, project):
                        path.mkdir()
                    env = {"HOME": str(home), "AGENTPLUGINS_HOME": str(data)}
                    locked = (project if scope == "project" else data) / "uap.lock"
                    locked.write_text(json.dumps(original) + "\n", encoding="utf-8")
                    original_bytes = locked.read_bytes()
                    foreign = Path(external) / "linked-lock"
                    try:
                        os.link(locked, foreign)
                    except (OSError, NotImplementedError) as error:
                        self.skipTest(f"host cannot create hardlinks: {error}")
                    # A preexisting hardlink must be rejected before corruption.
                    with self.assertRaisesRegex(
                            AssertionError, "invalid original signed lock path"):
                        verify("agentplugins", env, project, "codex", scope, ())
                    self.assertEqual(foreign.read_bytes(), original_bytes)
                    foreign.unlink()
                    checked = []

                    def attacker(*_):
                        os.link(locked, foreign)
                        return {"exit_code": 1, "timeout": False,
                                "stderr_tail": (
                                    "verify notion pinned manifest: "
                                    "signed Directory publication differs from exact lock")}

                    with patch.dict(verify.__globals__, {
                            "invoke": attacker, "inventory": lambda *args: [],
                            "identities": lambda *args: {}}):
                        with self.assertRaisesRegex(
                                AssertionError, "lock path replaced"):
                            verify("agentplugins", env, project, "codex", scope,
                                   (lambda: checked.append(True),))
                    self.assertEqual(checked, [True])
                    self.assertNotEqual(foreign.read_bytes(), original_bytes)
                    self.assertEqual(foreign.read_bytes(), locked.read_bytes())

    def test_marketing_kit_cold_locked_restore_receipts_match_installed_scope(self):
        # A real cold restore must bind each reported applied result to the
        # installed distribution, not merely return four plausible IDs.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        required = runner["REQUIRED_DISTRIBUTIONS"]
        aliases = tuple(required)
        actions = [{"alias": alias, "client_id": "codex", "action": "install"}
                   for alias in aliases]
        results = [{"client_id": "codex", "operation": "install",
                    "installation_id": alias + "-id", "outcome": "completed"}
                   for alias in aliases]
        installed = {required[alias]: alias + "-id" for alias in aliases}
        for scope in ("project", "user"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": actions, "results": results}
                receipt = {"exit_code": 0, "timeout": False, "_json": envelope}
                args = ("install", "--locked", "--yes", "--scope", scope)
                if scope == "project":
                    args += ("--project-root", str(Path.cwd()))
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *argv: receipt,
                        "inventory": lambda *argv: None,
                        "identities": lambda *argv: installed}):
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "cold restore", *args)["result"], "passed")
                    for incorrect in (
                            # A swapped child identity was formerly accepted
                            # because the inventory itself remained correct.
                            [results[1], results[0], *results[2:]],
                            [{**results[0], "installation_id": "other-profile-id"},
                             *results[1:]]):
                        malformed = {**receipt, "_json": {
                            **envelope, "results": incorrect}}
                        with patch.dict(execute.__globals__, {
                                "invoke": lambda *argv: malformed}):
                            with self.assertRaisesRegex(
                                    AssertionError, "receipt does not match"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "cold restore", *args)

    def test_marketing_kit_repair_receipt_matches_selected_scope_inventory(self):
        # A plausible repair receipt cannot attest to a different durable
        # installation merely because the action itself reports the same ID.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        distribution = runner["REQUIRED_DISTRIBUTIONS"]["marketing-kit"]
        action = {"alias": "marketing-kit", "client_id": "codex",
                  "action": "repair", "installation_id": "root-id"}
        receipt = {"client_id": "codex", "operation": "repair",
                   "installation_id": "root-id", "outcome": "completed"}
        for scope in ("project", "user"):
            with self.subTest(scope=scope):
                envelope = {"result": "success", "outcome": "completed",
                            "scope": scope, "actions": [action],
                            "results": [receipt]}
                response = {"exit_code": 0, "timeout": False, "_json": envelope}
                args = ("repair", "marketing-kit", "--target", "codex",
                        "--scope", scope)
                if scope == "project":
                    args += ("--project-root", str(Path.cwd()))
                with patch.dict(execute.__globals__, {
                        "invoke": lambda *argv: response,
                        "inventory": lambda *argv: [],
                        "identities": lambda *argv: {distribution: "root-id"}}):
                    self.assertEqual(
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "repair", *args)["result"], "passed")
                    with patch.dict(execute.__globals__, {
                            "identities": lambda *argv: {distribution: "foreign-id"}}):
                        with self.assertRaisesRegex(
                                AssertionError, "receipt does not match installed"):
                            execute("agentplugins", {}, Path.cwd(),
                                    "codex", "repair", *args)

    def test_marketing_kit_accepts_only_complete_legacy_noop_update(self):
        # Published 0.1.102 emits result=success alongside outcome/actions/results.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        actions = [{"alias": alias, "client_id": "codex",
                    "installation_id": alias + "-installation",
                    "binding_id": alias + "-binding", "action": "unchanged"}
                   for alias in aliases]
        envelope = {"result": "success", "outcome": "completed",
                    "actions": actions, "results": []}
        valid = {"exit_code": 0, "timeout": False, "_json": envelope}
        with_success = {**envelope, "result": "success"}
        self.assertEqual(check({**valid, "_json": with_success},
                               "update", "update", "codex"), with_success)
        for stage in ("update", "User update"):
            self.assertEqual(check(valid, stage, "update"), envelope)
        for operation in ("add", "repair", "install", "remove"):
            with self.subTest(operation=operation), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check(valid, "update", operation)
        missing_status = {key: value for key, value in envelope.items()
                          if key != "result"}
        with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
            check({**valid, "_json": missing_status}, "update", "update")
        for changed in (
                {"outcome": "partial"},
                {"outcome": "failure"},
                {"result": "failure"},
                {"result": "partial"},
                {"results": [{"outcome": "failed"}]},
                {"actions": actions[:-1]},
                {"actions": [{**actions[0], "action": "failed"}, *actions[1:]]},
                {"actions": [{**actions[0], "alias": "foreign"}, *actions[1:]]},
                {"actions": [{**actions[0], "client_id": ""}, *actions[1:]]},
        ):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**valid, "_json": {**envelope, **changed}}, "update", "update")
        for changed in ({"exit_code": 1}, {"timeout": True}):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**valid, **changed}, "update", "update")

    def test_marketing_kit_dependency_reports_require_selected_client_and_receipts(self):
        # A zero exit and generic success must not certify another client's actions.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        aliases = ("marketing-kit", "notion", "hubspot-crm", "playwright")
        for operation, action_type, selected in (
                ("update", "unchanged", aliases),
                ("repair", "repair", ("marketing-kit",)),
                ("install", "install", aliases),
                ("remove", "remove", ("marketing-kit",))):
            with self.subTest(operation=operation):
                actions = [{"alias": alias, "client_id": "codex",
                            "installation_id": alias + "-id", "action": action_type}
                           for alias in selected]
                results = [{"operation": action_type, "client_id": "codex",
                            "installation_id": action["installation_id"],
                            "outcome": "completed"}
                           for action in actions if action_type != "unchanged"]
                envelope = {"result": "success", "outcome": "completed",
                            "actions": actions, "results": results}
                valid = {"exit_code": 0, "timeout": False, "_json": envelope}
                self.assertEqual(check(valid, operation, operation, "codex"), envelope)
                released = {**envelope, "result": "success"}
                self.assertEqual(check({**valid, "_json": released},
                                       operation, operation, "codex"), released)
                # Both actions and results can consistently attest a *different*
                # client while this target's native state remains untouched.
                foreign = {**envelope,
                           "actions": [{**action, "client_id": "claude"} for action in actions],
                           "results": [{**result, "client_id": "claude"} for result in results]}
                for invalid in (foreign, {**foreign, "result": "success"},
                                {"result": "success"},
                                {key: value for key, value in envelope.items()
                                 if key != "result"},
                                {**envelope, "result": "partial"},
                                {**envelope, "result": "failure"}):
                    with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                        check({**valid, "_json": invalid}, operation, operation, "codex")
                if results:
                    # A completed native operation without a durable installation
                    # identity cannot be attributed to this signed dependency.
                    # Cold install plans may omit the *action* ID; an applied
                    # result must still return its actual installation ID.
                    for missing_id in (None, "", "   ", 42):
                        malformed_results = [dict(result) for result in results]
                        malformed_results[0]["installation_id"] = missing_id
                        with self.subTest(operation=operation, missing_id=missing_id):
                            with self.assertRaisesRegex(
                                    AssertionError, "CLI did not succeed"):
                                check({**valid, "_json": {**envelope,
                                      "results": malformed_results}},
                                      operation, operation, "codex")
                    # The released installer legitimately returns "unchanged"
                    # when an active repair/install/remove converges with no
                    # filesystem mutation. Preserve this exact public contract,
                    # but never accept an unrelated client's unchanged receipt.
                    no_op_receipts = {**envelope, "results": [
                        {**result, "outcome": "unchanged"} for result in results]}
                    for output in (no_op_receipts,
                                   {**no_op_receipts, "result": "success"}):
                        self.assertEqual(check({**valid, "_json": output},
                                               operation, operation, "codex"), output)
                        foreign_no_op = {**output, "results": [
                            {**result, "client_id": "claude"}
                            for result in output["results"]]}
                        with self.assertRaisesRegex(AssertionError,
                                                    "CLI did not succeed"):
                            check({**valid, "_json": foreign_no_op},
                                  operation, operation, "codex")

    def test_marketing_kit_future_profiles_refuse_preplanted_paths(self):
        # A third-party package can access sibling paths under the temporary
        # case root. It must not redirect a later cold B/C bootstrap.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            root = Path(temporary).resolve()
            outside = Path(external).resolve()
            occupied = root / "B"
            occupied.mkdir()
            with self.assertRaisesRegex(AssertionError, "isolated profile root"):
                runner["run_case"]("agentplugins", "codex", root, "B")
            self.assertEqual(list(occupied.iterdir()), [])
            occupied.rmdir()
            try:
                occupied.symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            with self.assertRaisesRegex(AssertionError, "isolated profile root"):
                runner["run_case"]("agentplugins", "codex", root, "B")
            self.assertEqual(list(outside.iterdir()), [])

    def test_marketing_kit_project_git_init_uses_pinned_audit_prerequisite(self):
        # The second profile is created after running untrusted packages; it
        # must not start PATH-resolved Git or use ambient Git hooks/config.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        audit = runner["audit"]
        pinned_git = audit["initialize_project_skill_git_root"]
        calls = []

        def checked_git(project, env, root):
            calls.append((project, root))
            return pinned_git(project, env, root)

        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "A"
            with patch.dict(audit, {"initialize_project_skill_git_root": checked_git}):
                # Any fallback to the original subprocess.run(["git", ...])
                # would now fail rather than silently choosing PATH's binary.
                with patch.dict(runner["run_case"].__globals__,
                                {"subprocess": None}):
                    env, project, _, _, _ = runner["run_case"](
                        "agentplugins", "codex", root, "A")
            self.assertEqual(calls, [(project, case)])
            self.assertEqual(env["HOME"], str(case / "home"))
            self.assertTrue((project / ".git").is_dir())
            self.assertTrue((case / "empty-git-hooks").is_dir())

    def test_marketing_kit_dependency_execution_checks_reported_scope(self):
        # Native/portable state can already exist, so a successful command
        # targeting the wrong scope must be rejected from its own JSON receipt.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        stable = {distribution: alias + "-stable-id"
                  for alias, distribution in runner["REQUIRED_DISTRIBUTIONS"].items()}
        actions = [{"alias": alias, "client_id": "codex", "action": "unchanged",
                    "installation_id": alias + "-stable-id"}
                   for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright")]
        original = {"result": "success", "outcome": "completed",
                    "actions": actions, "results": []}
        for scope in ("user", "project"):
            with self.subTest(scope=scope):
                for reported in (scope, "project" if scope == "user" else "user",
                                 None, "unexpected"):
                    receipt = {**original, "scope": reported}
                    response = {"exit_code": 0, "timeout": False, "_json": receipt}
                    with patch.dict(execute.__globals__,
                                    {"invoke": lambda *a: response,
                                     "inventory": lambda *a: [],
                                     "identities": lambda *a: stable}):
                        command = ("update", "marketing-kit", "--scope", scope)
                        if reported == scope:
                            self.assertEqual(
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "update", *command)["result"], "passed")
                        else:
                            with self.assertRaisesRegex(
                                    AssertionError, "CLI did not succeed"):
                                execute("agentplugins", {}, Path.cwd(), "codex",
                                        "update", *command)
                for malformed in (("update", "marketing-kit"),
                                  ("update", "marketing-kit", "--scope", "invalid"),
                                  ("update", "marketing-kit", "--scope", "user",
                                   "--scope", "project")):
                    with self.assertRaisesRegex(
                            AssertionError, "explicit scope"):
                        execute("agentplugins", {}, Path.cwd(), "codex",
                                "update", *malformed)

    def test_marketing_kit_noop_update_rejects_rekey_and_lock_rewrite(self):
        # Released 0.1.102 reports unchanged; do not trust that envelope alone.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_noop_update"]
        original = {"marketing-kit": "root-id", "notion": "notion-id",
                    "hubspot-crm": "hubspot-id", "playwright": "playwright-id"}
        signed_lock = b'{"immutable": "signed"}\n'
        for stage in ("Project update", "User update"):
            with self.subTest(stage=stage):
                check(dict(original), original, signed_lock, signed_lock, stage)
                rekeyed = {**original, "notion": "unexpected-new-id"}
                with self.assertRaisesRegex(AssertionError,
                                            "no-op update changed installation identities"):
                    check(rekeyed, original, signed_lock, signed_lock, stage)
                with self.assertRaisesRegex(AssertionError,
                                            "no-op update changed signed lock"):
                    check(original, original, signed_lock + b" ", signed_lock, stage)

    def test_marketing_kit_legacy_repair_install_remove_require_real_results(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["verified"]
        actions = [{"alias": alias, "client_id": "codex",
                    "installation_id": alias + "-installation",
                    "action": "install"}
                   for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright")]
        def report(action):
            return {"installation_id": action["installation_id"],
                    "client_id": action["client_id"],
                    "operation": action["action"], "outcome": "completed"}
        def accepted(operation, planned, results):
            envelope = {"result": "success", "outcome": "completed",
                        "actions": planned, "results": results}
            wrapped = {"exit_code": 0, "timeout": False, "_json": envelope}
            self.assertEqual(check(wrapped, operation, operation), envelope)
            return wrapped
        installed = accepted("install", actions, [report(action) for action in actions])
        repair = {**actions[0], "action": "repair"}
        intact = {**report(repair), "outcome": "unchanged",
                  "manual_actions": ["native MCP initialization remains unobserved"]}
        accepted("repair", [repair], [intact])
        removal = {**actions[0], "action": "remove"}
        accepted("remove", [removal], [report(removal)])
        for changed in (
                {"result": None},
                {"outcome": "partial"},
                {"results": []},
                {"results": [{**report(actions[0]), "outcome": "incomplete"},
                             *[report(action) for action in actions[1:]]]},
                {"results": [{**report(actions[0]), "client_id": "foreign"},
                             *[report(action) for action in actions[1:]]]},
                {"actions": [{**actions[0], "action": "unknown"}, *actions[1:]]},
        ):
            with self.subTest(changed=changed), self.assertRaisesRegex(
                    AssertionError, "CLI did not succeed"):
                check({**installed, "_json": {**installed["_json"], **changed}},
                      "cold locked restore", "install")

    def test_marketing_kit_requires_exact_published_cli_version(self):
        # The substring "0.1.102" alone also matches prereleases and 0.1.1021.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        require = runner["require_exact_published_version"]
        valid = {"exit_code": 0, "timeout": False,
                 "stdout_tail": "agentplugins 0.1.102\n"}
        require(valid)
        for version in ("agentplugins 0.1.102-dev",
                        "agentplugins 0.1.1021",
                        "agentplugins 0.1.103",
                        "other-launcher 0.1.102",
                        "agentplugins 0.1.102\nagentplugins 0.1.102",
                        ""):
            with self.subTest(version=version):
                with self.assertRaisesRegex(AssertionError, "not exact published"):
                    require({**valid, "stdout_tail": version})
        for invalid in ({"exit_code": 1}, {"timeout": True},
                        {"stdout_tail": None}):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(AssertionError, "not exact published"):
                    require({**valid, **invalid})

    def test_marketing_kit_private_cache_version_retry_is_bounded_and_strict(self):
        # The workflow's initial version check warms a different launcher cache.
        # Retry acquisition inside the disposable profile, not mismatched releases.
        from runpy import run_path
        from unittest.mock import Mock

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        verify = runner["verify_pinned_launcher"]
        failed = {"exit_code": 1, "timeout": False, "stdout_tail": ""}
        timed_out = {"exit_code": None, "timeout": True, "stdout_tail": ""}
        valid = {"exit_code": 0, "timeout": False,
                 "stdout_tail": "agentplugins 0.1.102\n"}
        probe, pause = Mock(side_effect=[failed, timed_out, valid]), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 3)
        self.assertEqual([item.args for item in pause.call_args_list], [(5,), (10,)])

        probe, pause = Mock(return_value=failed), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            with self.assertRaisesRegex(AssertionError, "not exact published"):
                verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 3)
        self.assertEqual([item.args for item in pause.call_args_list], [(5,), (10,)])

        probe, pause = Mock(return_value={**valid, "stdout_tail": "agentplugins 0.1.102-dev"}), Mock()
        with patch.dict(verify.__globals__, {"invoke": probe, "sleep": pause}):
            with self.assertRaisesRegex(AssertionError, "not exact published"):
                verify("agentplugins", {}, Path("."))
        self.assertEqual(probe.call_count, 1)
        pause.assert_not_called()

    def test_marketing_kit_released_binary_cache_preserves_profile_isolation(self):
        # A and B share only the SHA-checked npm launcher executable cache;
        # neither project nor native profile state may cross scopes.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        bind = runner["bind_pinned_launcher_cache"]
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            sandbox = Path(temporary).resolve()
            cache = sandbox / "released-cli-cache"
            cache.mkdir(mode=0o700)
            env_a, *_ = runner["run_case"]("unused", "codex", sandbox, "A")
            env_b, *_ = runner["run_case"]("unused", "codex", sandbox, "B")
            bind(env_a, cache, sandbox)
            bind(env_b, cache, sandbox)
            self.assertEqual(env_a["AGENTPLUGINS_CACHE_DIR"], str(cache))
            self.assertEqual(env_b["AGENTPLUGINS_CACHE_DIR"], str(cache))
            for key in ("HOME", "AGENTPLUGINS_HOME", "XDG_CACHE_HOME", "CODEX_HOME"):
                self.assertNotEqual(env_a[key], env_b[key], key)
            with self.assertRaisesRegex(AssertionError, "in-sandbox"):
                bind(env_a, Path(external), sandbox)
            self.assertEqual(env_a["AGENTPLUGINS_CACHE_DIR"], str(cache))
            link = sandbox / "untrusted-cache-link"
            try:
                link.symlink_to(Path(external), target_is_directory=True)
            except (OSError, NotImplementedError):
                pass
            else:
                with self.assertRaisesRegex(AssertionError, "in-sandbox"):
                    bind(env_b, link, sandbox)
                self.assertEqual(env_b["AGENTPLUGINS_CACHE_DIR"], str(cache))

    def test_marketing_kit_foreign_guard_rejects_symlinked_ancestor(self):
        # Regression: a native-profile directory can be replaced after add.
        # A matching marker behind an outside symlink must not be read as proof.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        guard = runner["check_foreign"]
        with TemporaryDirectory() as temporary, TemporaryDirectory() as external:
            sandbox = Path(temporary).resolve()
            outside = Path(external).resolve()
            native = sandbox / "A" / "home" / ".copilot"
            native.mkdir(parents=True)
            marker = native / "unmanaged-native-do-not-change.txt"
            content = "unmanaged native settings\\n"
            marker.write_text(content, encoding="utf-8")
            entries = runner["pin_foreign"]([(marker, content)], sandbox)
            guard(entries, sandbox)

            native.rename(native.parent / "original-profile")
            outside_marker = outside / marker.name
            outside_marker.write_text(content, encoding="utf-8")
            try:
                native.symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                guard(entries, sandbox)
            self.assertEqual(outside_marker.read_text(encoding="utf-8"), content)
    def test_marketing_kit_foreign_guard_rejects_replaced_skill_ancestors(self):
        # Moving the original SKILL.md preserves its inode/bytes; replacing
        # either owning directory must still fail foreign-state isolation.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            skill = case / "A" / "home" / ".agents" / "skills" / "foreign" / "SKILL.md"
            skill.parent.mkdir(parents=True)
            original = "---\nname: foreign\n---\n"
            skill.write_text(original, encoding="utf-8")
            pinned = runner["pin_foreign"]([(skill, original)], case)
            before = skill.lstat()

            for replaced in (skill.parent, skill.parent.parent.parent):
                with self.subTest(replaced=str(replaced)):
                    saved = replaced.with_name(replaced.name + "-saved")
                    replaced.rename(saved)
                    replaced.mkdir()
                    if replaced == skill.parent:
                        (saved / "SKILL.md").rename(skill)
                    else:
                        (saved / "skills").rename(replaced / "skills")
                    try:
                        after = skill.lstat()
                        self.assertEqual((after.st_dev, after.st_ino),
                                         (before.st_dev, before.st_ino))
                        with self.assertRaisesRegex(
                                AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                    finally:
                        if replaced == skill.parent:
                            skill.rename(saved / "SKILL.md")
                        else:
                            (replaced / "skills").rename(saved / "skills")
                        replaced.rmdir()
                        saved.rename(replaced)
                    runner["check_foreign"](pinned, case)

    def test_marketing_kit_scoped_snapshot_fails_closed_on_walk_error(self):
        # os.walk skips unreadable descendants without onerror; an unchanged
        # unreadable tree is not proof that foreign project state was preserved.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / "project"
            root.mkdir()
            self.assertEqual(runner["scoped_snapshot"]([root])["root-kind:0"],
                             "directory")

            def inaccessible(_root, **kwargs):
                onerror = kwargs.get("onerror")
                if onerror:
                    onerror(PermissionError("unreadable protected subtree"))
                return iter(())

            with patch.dict(runner["audit"], {"snapshot": lambda _roots: {}}):
                with patch("os.walk", side_effect=inaccessible):
                    with self.assertRaisesRegex(
                            AssertionError, "protected scope snapshot unreadable"):
                        runner["scoped_snapshot"]([root])

    def test_marketing_kit_scoped_snapshot_rejects_unreadable_file_hash(self):
        # The shared audit returns a stable 'unreadable' sentinel for failed
        # file reads. Two identical sentinels are not byte-for-byte evidence.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / "project"
            root.mkdir()
            with patch.dict(runner["audit"], {
                    "snapshot": lambda _roots: {"project/foreign": "unreadable"}}):
                with self.assertRaisesRegex(
                        AssertionError, "protected scope snapshot unreadable"):
                    runner["scoped_snapshot"]([root])

    def test_marketing_kit_foreign_guard_rejects_newline_rewrite(self):
        # Text-mode newline normalization can hide a byte-changing rewrite.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            marker = root / "foreign-user-do-not-change.txt"
            expected = "unmanaged user data\n"
            marker.write_bytes(expected.encode("utf-8"))
            pinned = runner["pin_foreign"]([(marker, expected)], root)
            runner["check_foreign"](pinned, root)
            allowed = root / "new-owned-config.json"
            allowed.write_text("{}", encoding="utf-8")
            runner["check_foreign"](pinned, root)
            allowed.unlink()

            marker.write_bytes(b"unmanaged user data\r\n")
            self.assertEqual(marker.read_text(encoding="utf-8"), expected)
            with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                runner["check_foreign"](pinned, root)

            # Even identical bytes must not excuse a chmod or inode swap of
            # unmanaged state in the profile being actively installed.
            marker.write_bytes(expected.encode("utf-8"))
            # Re-pin after the deliberate byte-corruption case above.
            pinned = runner["pin_foreign"]([(marker, expected)], root)
            original_stat = marker.stat()
            try:
                os.utime(marker, ns=(original_stat.st_atime_ns,
                                     original_stat.st_mtime_ns - 2_000_000_000))
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                os.utime(marker, ns=(original_stat.st_atime_ns,
                                     original_stat.st_mtime_ns))
            runner["check_foreign"](pinned, root)
            original_mode = marker.stat().st_mode & 0o7777
            try:
                marker.chmod(original_mode ^ 0o200)
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                marker.chmod(original_mode)
            runner["check_foreign"](pinned, root)

            saved = root / "saved-original-foreign"
            marker.replace(saved)
            try:
                marker.write_bytes(expected.encode("utf-8"))
                with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                    runner["check_foreign"](pinned, root)
            finally:
                marker.unlink(missing_ok=True)
                saved.replace(marker)
            runner["check_foreign"](pinned, root)

    def test_marketing_kit_identity_guard_rejects_missing_or_shared_ids(self):
        # A missing ID can make the repeat/repair comparison falsely pass.
        # Distinct signed distributions must also retain distinct owned IDs.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        def row(distribution, installation_id):
            return {"installation_id": installation_id,
                    "directory": {"recorded_distribution": distribution},
                    "clients": [{"client_id": "codex", "scope": "project",
                                 "verification": "installation_verified",
                                 "materialization": "materialized",
                                 "package_revision": "pinned"}]}

        with patch.dict(runner["audit"],
                        {"recorded_signed_release_matches": lambda *_: True}):
            check = runner["identities"]
            for bad in (None, "", "  "):
                with self.assertRaisesRegex(AssertionError, "missing installation ID"):
                    check([row("root", bad)], "codex")
            with self.assertRaisesRegex(AssertionError, "duplicate installation ID"):
                check([row("root", "same"), row("dependency", "same")], "codex")
            self.assertEqual(check([row("root", "one"), row("dependency", "two")], "codex"),
                             {"root": "one", "dependency": "two"})

    def test_marketing_kit_identity_guard_rejects_cross_scope_provenance(self):
        # A signed User binding must never authorize an unsigned Project binding.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        def binding(scope, revision):
            return {"client_id": "codex", "scope": scope,
                    "verification": "installation_verified", "materialization": "materialized",
                    "package_revision": revision}
        row = {"installation_id": "owned", "directory": {"recorded_distribution": "root"},
               "clients": [binding("project", "unsigned"), binding("user", "signed")]}

        with patch.dict(runner["audit"],
                        {"recorded_signed_release_matches":
                         lambda _, revision: revision == "signed"}):
            check = runner["identities"]
            with self.assertRaisesRegex(AssertionError, "project binding lacks exact"):
                check([row], "codex")
            row["clients"] = [binding("project", "signed"), binding("user", "unsigned")]
            self.assertEqual(check([row], "codex"), {"root": "owned"})
            row["clients"].append(binding("project", "unsigned"))
            with self.assertRaisesRegex(AssertionError, "project binding lacks exact"):
                check([row], "codex")

    def test_marketing_kit_user_identity_guard_rejects_project_only_signature(self):
        # A signed Project binding cannot authorize an unsigned User binding.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        row = {"installation_id": "owned",
               "directory": {"recorded_distribution": "root"},
               "clients": [{"client_id": "codex", "scope": "user",
                            "verification": "installation_verified",
                            "materialization": "materialized",
                            "package_revision": "unsigned"},
                           {"client_id": "codex", "scope": "project",
                            "verification": "installation_verified",
                            "materialization": "materialized",
                            "package_revision": "signed"}]}
        with patch.dict(runner["audit"],
                        {"recorded_signed_release_matches":
                         lambda _, revision: revision == "signed"}):
            with self.assertRaisesRegex(AssertionError, "user binding lacks exact"):
                runner["identities"]([row], "codex", "user")
            row["clients"][0]["package_revision"] = "signed"
            self.assertEqual(runner["identities"]([row], "codex", "user"),
                             {"root": "owned"})

    def test_marketing_kit_requires_installed_verification_in_selected_scope(self):
        # Signed provenance and a persisted list entry do not prove delivery.
        # Prepared/package-only and failed bindings cannot qualify E2E.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        binding = {"client_id": "codex", "scope": "user",
                   "package_revision": "signed",
                   "verification": "installation_verified",
                   "materialization": "materialized"}
        row = {"installation_id": "owned",
               "directory": {"recorded_distribution": "root"},
               "clients": [binding]}
        with patch.dict(runner["audit"],
                        {"recorded_signed_release_matches":
                         lambda _, revision: revision == "signed"}):
            check = runner["identities"]
            self.assertEqual(check([row], "codex", "user"), {"root": "owned"})
            for invalid in (None, "", "package_validated", "not_checked",
                            "verification_failed", "manual_activation_required"):
                with self.subTest(verification=invalid):
                    row["clients"] = [{**binding, "verification": invalid}]
                    with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
                        check([row], "codex", "user")
            row["clients"] = [binding, {**binding, "verification": "package_validated"}]
            with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
                check([row], "codex", "user")
            row["clients"] = [{**binding, "materialization": "staged"}]
            with self.assertRaisesRegex(AssertionError, "lacks materialized delivery"):
                check([row], "codex", "user")
            # Cursor User deliberately prepares a package with an explicit
            # manual-activation requirement, not an installed native MCP.
            prepared = {**binding, "client_id": "cursor",
                        "verification": "package_validated",
                        "activation": "manual_activation_required",
                        "policy": "allowed"}
            row["clients"] = [prepared]
            self.assertEqual(check([row], "cursor", "user"), {"root": "owned"})
            for change in ({"activation": "prepared"}, {"policy": "blocked"},
                           {"materialization": "degraded"},
                           {"verification": "failed"}):
                row["clients"] = [{**prepared, **change}]
                with self.subTest(change=change), self.assertRaises(AssertionError):
                    check([row], "cursor", "user")
            # Only Cursor User can take the documented manual preparation path.
            row["clients"] = [{**prepared, "client_id": "codex"}]
            with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
                check([row], "codex", "user")
            row["clients"] = [{**prepared, "scope": "project"}]
            with self.assertRaisesRegex(AssertionError, "lacks installed verification"):
                check([row], "cursor", "project")
            # An unverified foreign scope must not overwrite the selected
            # User binding's verified status.
            row["clients"] = [binding, {**binding, "scope": "project",
                                       "verification": "package_validated"}]
            self.assertEqual(check([row], "codex", "user"), {"root": "owned"})

    def test_marketing_kit_user_remove_rejects_active_native_residuals(self):
        # An empty CLI list alone cannot prove native client removal.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            native = home / ".copilot"
            native.mkdir(parents=True)
            settings = native / "settings.json"
            check = runner["check_removed_native_components"]
            for identity in ("marketing-kit", "notion", "hubspot-crm",
                             "playwright-bridge", "playwright"):
                settings.write_text(
                    '{"enabledPlugins":{"' + identity +
                    '@agentplugins-aaaaaaaaaaaa":true}}', encoding="utf-8")
                with self.subTest(identity=identity), self.assertRaisesRegex(
                        AssertionError, "native Marketing Kit remnants"):
                    check("copilot", home, case, "User remove")
            settings.write_text('{"enabledPlugins":{"other@foreign":true}}',
                                encoding="utf-8")
            check("copilot", home, case, "User remove")
            # Project cleanup must cover native configuration under both
            # Project and HOME, without conflating their separate roots.
            project = case / "project"
            native_project = project / ".copilot"
            native_project.mkdir(parents=True)
            project_settings = native_project / "settings.json"
            project_settings.write_text(
                '{"enabledPlugins":{"marketing-kit@agentplugins-aaaaaaaaaaaa":true}}',
                encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            project_settings.write_text('{"enabledPlugins":{"other@foreign":true}}',
                                        encoding="utf-8")
            check("copilot", project, case, "Project remove")
            # Checking Project does not silently cover HOME; caller must do both.
            settings.write_text(
                '{"enabledPlugins":{"marketing-kit@agentplugins-aaaaaaaaaaaa":true}}',
                encoding="utf-8")
            check("copilot", project, case, "Project remove")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", home, case, "Project native HOME remove")
            settings.unlink()
            native.rmdir()
            try:
                native.symlink_to(case / "untrusted", target_is_directory=True)
            except (OSError, NotImplementedError):
                return
            with self.assertRaisesRegex(AssertionError, "unsafe native client root"):
                check("copilot", home, case, "User remove")

    def test_marketing_kit_removed_native_root_rejects_file_ancestors(self):
        # A non-directory native root or intermediate path cannot prove removal.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            home.mkdir()
            for unsafe in (home / ".copilot", home / ".config",
                           home / ".config" / "copilot"):
                unsafe.parent.mkdir(parents=True, exist_ok=True)
                unsafe.write_text("occupied", encoding="utf-8")
                with self.subTest(unsafe=unsafe), self.assertRaisesRegex(
                        AssertionError, "invalid native client root"):
                    check("copilot", home, case, "remove")
                unsafe.unlink()
            check("copilot", home, case, "remove")

    def test_marketing_kit_native_cleanup_refuses_in_sandbox_profile_aliases(self):
        # Reuse the shared path_inside_case guard: it must reject symlinks
        # even when they resolve to another profile *inside* the sandbox.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            a = case / "A"
            a.mkdir()
            (a / "home").mkdir()
            b = case / "B"
            b.mkdir()
            alias = b / "home"
            try:
                alias.symlink_to(a / "home", target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            self.assertTrue(alias.is_dir())  # Symlink resolves within sandbox.
            with self.assertRaisesRegex(AssertionError, "native HOME left disposable case"):
                check("copilot", alias, case, "remove B")
            alias.unlink()
            b.rmdir()
            b.symlink_to(a, target_is_directory=True)
            with self.assertRaisesRegex(AssertionError, "native HOME left disposable case"):
                check("copilot", b / "home", case, "remove B")
            b.unlink()
            b.mkdir()
            (b / "home").mkdir()
            check("copilot", b / "home", case, "remove B")

    def test_marketing_kit_user_and_project_foreign_skills_are_pinned(self):
        # Protect real native User Skills independently of Project Skills;
        # Codex, OpenCode, Cursor, Gemini and Copilot also discover compatibility Skills.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        # These expected paths are independent of the harness root selector:
        # otherwise the test could replicate an incorrect native mapping.
        for target, project_root, user_roots in (
                ("claude", ".claude/skills", (".claude/skills",)),
                ("copilot", ".github/skills", (".copilot/skills", ".agents/skills")),
                ("codex", ".agents/skills", (".codex/skills", ".agents/skills")),
                ("opencode", ".opencode/skills",
                 (".config/opencode/skills", ".agents/skills", ".claude/skills")),
                ("cursor", ".cursor/skills",
                 (".cursor/skills", ".agents/skills", ".claude/skills",
                  ".codex/skills")),
                ("gemini", ".gemini/skills",
                 (".gemini/skills", ".agents/skills"))):
            with self.subTest(target=target), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                env, project, _, _, entries = runner["run_case"](
                    "agentplugins", target, case, "A")
                name = "unmanaged-uap-foreign-skill"
                expected_skills = {
                    project / project_root / name / "SKILL.md",
                    *(Path(env["HOME"]) / root / name / "SKILL.md"
                      for root in user_roots),
                }
                compatible_project = {
                    "opencode": (".agents/skills", ".claude/skills"),
                    "cursor": (".agents/skills", ".claude/skills", ".codex/skills"),
                    "gemini": (".agents/skills",),
                    "copilot": (".agents/skills", ".claude/skills"),
                }
                expected_skills.update({
                    project / root / name / "SKILL.md"
                    for root in compatible_project.get(target, ())
                })
                skills = [(path, content) for path, content in entries
                          if path.name == "SKILL.md"]
                self.assertEqual({path for path, _ in skills}, expected_skills)
                references = [(path, content) for path, content in entries
                              if path.name == "unmanaged-context.md"]
                self.assertEqual(
                    {path for path, _ in references},
                    {skill.parent / "references" / "unmanaged-context.md"
                     for skill in expected_skills})
                pinned = runner["pin_foreign"](entries, case)
                runner["check_foreign"](pinned, case)
                for skill, original in skills:
                    with self.subTest(skill=str(skill)):
                        original_stat = skill.stat()
                        skill.write_text(original + "rewritten\n", encoding="utf-8")
                        with self.assertRaisesRegex(AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                        skill.write_text(original, encoding="utf-8")
                        os.utime(skill, ns=(original_stat.st_atime_ns,
                                            original_stat.st_mtime_ns))
                        runner["check_foreign"](pinned, case)
                        injected = skill.parent / "unexpected-native-file.txt"
                        injected.write_text("unexpected", encoding="utf-8")
                        try:
                            with self.assertRaisesRegex(
                                    AssertionError, "foreign content modified"):
                                runner["check_foreign"](pinned, case)
                        finally:
                            injected.unlink()
                        runner["check_foreign"](pinned, case)
                for reference, original in references:
                    with self.subTest(reference=str(reference)):
                        original_stat = reference.stat()
                        reference.write_text("tampered\n", encoding="utf-8")
                        with self.assertRaisesRegex(
                                AssertionError, "foreign content modified"):
                            runner["check_foreign"](pinned, case)
                        reference.write_text(original, encoding="utf-8")
                        os.utime(reference, ns=(original_stat.st_atime_ns,
                                                original_stat.st_mtime_ns))
                        runner["check_foreign"](pinned, case)
                        injected = reference.parent / "unmanaged-extra.md"
                        injected.write_text("unexpected\n", encoding="utf-8")
                        try:
                            with self.assertRaisesRegex(
                                    AssertionError, "foreign content modified"):
                                runner["check_foreign"](pinned, case)
                        finally:
                            injected.unlink()
                        runner["check_foreign"](pinned, case)

    def test_marketing_kit_opencode_compat_skills_must_be_removed(self):
        # Official OpenCode discovery includes .agents/.claude Skills in
        # both scopes; empty owned native directories must not be a false PASS.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                for compatible in (".agents", ".claude"):
                    with self.subTest(scope=scope, compatible=compatible):
                        skills = base / compatible / "skills"
                        foreign = skills / "unmanaged-uap-foreign-skill"
                        foreign.mkdir(parents=True)
                        (foreign / "SKILL.md").write_text(
                            "---\nname: unmanaged-uap-foreign-skill\n"
                            "description: keep unchanged\n---\n", encoding="utf-8")
                        check("opencode", base, case, "remove")
                        for owned_name in ("marketing-kit",
                                           "playwright@agentplugins-aaaaaaaaaaaa"):
                            owned = skills / owned_name
                            owned.mkdir()
                            with self.assertRaisesRegex(
                                    AssertionError, "native Marketing Kit remnants"):
                                check("opencode", base, case, "remove")
                            owned.rmdir()
                        check("opencode", base, case, "remove")

    def test_marketing_kit_cursor_compat_skills_must_be_removed(self):
        # Cursor documents .agents/.claude/.codex compatibility Skills in
        # User and Project scopes, not only the primary .cursor/skills tree.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                for compatible in (".agents", ".claude", ".codex"):
                    with self.subTest(scope=scope, compatible=compatible):
                        skills = base / compatible / "skills"
                        foreign = skills / "unmanaged-uap-foreign-skill"
                        foreign.mkdir(parents=True)
                        (foreign / "SKILL.md").write_text(
                            "---\nname: unmanaged-uap-foreign-skill\n"
                            "description: keep unchanged\n---\n", encoding="utf-8")
                        check("cursor", base, case, "remove")
                        owned = skills / "marketing-kit"
                        owned.mkdir()
                        with self.assertRaisesRegex(
                                AssertionError, "native Marketing Kit remnants"):
                            check("cursor", base, case, "remove")
                        owned.rmdir()
                        check("cursor", base, case, "remove")

    def test_marketing_kit_gemini_alias_skill_cleanup(self):
        # Gemini's documented native .agents/skills alias is discovered
        # independently in User and Project scope.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            for scope in ("home", "project"):
                base = case / scope
                base.mkdir()
                skills = base / ".agents" / "skills"
                foreign = skills / "unmanaged-uap-foreign-skill"
                foreign.mkdir(parents=True)
                (foreign / "SKILL.md").write_text(
                    "---\nname: unmanaged-uap-foreign-skill\n"
                    "description: keep unchanged\n---\n", encoding="utf-8")
                check("gemini", base, case, "remove")
                owned = skills / "marketing-kit"
                owned.mkdir()
                with self.subTest(scope=scope), self.assertRaisesRegex(
                        AssertionError, "native Marketing Kit remnants"):
                    check("gemini", base, case, "remove")
                owned.rmdir()
                check("gemini", base, case, "remove")

    def test_marketing_kit_copilot_project_skill_removal_scans_github_root(self):
        # Copilot Project Skills live in .github/skills, outside MCP config roots.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            owned = project / ".github" / "skills" / "marketing-kit"
            other = project / ".github" / "skills" / "foreign-skill"
            owned.mkdir(parents=True)
            other.mkdir()
            (other / "SKILL.md").write_text("name: foreign-skill\n", encoding="utf-8")
            file = owned / "SKILL.md"
            file.write_text("name: marketing-kit\n", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            file.unlink()
            # Empty named Skill directories are absent from audit.snapshot()
            # yet remain native residue after a claimed successful remove.
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            owned.rmdir()
            owned_with_receipt = owned.parent / "marketing-kit@agentplugins-aaaaaaaaaaaa"
            owned_with_receipt.mkdir()
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", project, case, "Project remove")
            owned_with_receipt.rmdir()
            # Removing owned Skills must not require deleting a foreign Skill.
            check("copilot", project, case, "Project remove", project_scope=True)
            for compatible in (".agents", ".claude"):
                with self.subTest(compatible=compatible):
                    skill_root = project / compatible / "skills"
                    other_skill = skill_root / "foreign-skill"
                    other_skill.mkdir(parents=True)
                    (other_skill / "SKILL.md").write_text(
                        "---\nname: foreign-skill\ndescription: preserve\n---\n",
                        encoding="utf-8")
                    check("copilot", project, case, "Project remove", project_scope=True)
                    owned_skill = skill_root / "marketing-kit"
                    owned_skill.mkdir()
                    with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                        check("copilot", project, case, "Project remove", project_scope=True)
                    owned_skill.rmdir()
                    check("copilot", project, case, "Project remove", project_scope=True)

    def test_marketing_kit_copilot_project_mcp_cleanup_checks_both_locations(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            (project / ".github").mkdir(parents=True)
            paths = [project / name for name in (".github/mcp.json", ".mcp.json")]
            # Foreign members may refer to a closure alias in their own values;
            # server member identity, rather than whole-file text, is decisive.
            foreign = {"mcpServers": {"foreign-neighbor": {
                "command": "notion", "args": ["playwright", "marketing-kit"]}}}
            original = json.dumps(foreign).encode()
            for path in paths:
                path.write_bytes(original)
            check("copilot", project, case, "remove", project_scope=True)
            for path in paths:
                for alias in ("notion", "playwright", "playwright-bridge",
                              "playwright@agentplugins-aaaaaaaaaaaa"):
                    with self.subTest(path=path.name, alias=alias):
                        changed = {"mcpServers": {**foreign["mcpServers"], alias: {"command": "probe"}}}
                        path.write_text(json.dumps(changed))
                        with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                            check("copilot", project, case, "remove", project_scope=True)
                        path.write_bytes(original)
                        check("copilot", project, case, "remove", project_scope=True)
                        self.assertTrue(all(item.read_bytes() == original for item in paths))
            for path in paths:
                for clean in ({}, {"mcpServers": {}}):
                    path.write_text(json.dumps(clean))
                    check("copilot", project, case, "remove", project_scope=True)
                path.unlink()
            check("copilot", project, case, "remove", project_scope=True)

    def test_marketing_kit_copilot_project_mcp_cleanup_rejects_malformed_config(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            project = case / "project"
            (project / ".github").mkdir(parents=True)
            for relative in (".github/mcp.json", ".mcp.json"):
                path = project / relative
                for body in ('invalid', '[]', '{"mcpServers":[]}',
                             '{"notion":{}}', '{"mcpServers":{"foreign":null}}',
                             '{"mcpServers":{"foreign":{"value":NaN}}}',
                             '{"mcpServers":{"notion":{},"notion":{}}}',
                             '{"mcpServers":{"notion":{}},"mcpServers":{}}',
                             '{"mcpServers":{}} trailing'):
                    with self.subTest(relative=relative, body=body):
                        path.write_text(body)
                        with self.assertRaisesRegex(AssertionError, "malformed Copilot Project MCP"):
                            runner["check_removed_native_components"](
                                "copilot", project, case, "remove", project_scope=True)
                path.unlink()

    def test_marketing_kit_copilot_project_mcp_cleanup_rejects_unsafe_paths(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for relative in (".github/mcp.json", ".mcp.json"):
            with self.subTest(relative=relative), TemporaryDirectory() as temporary:
                case = Path(temporary).resolve()
                project, outside = case / "project", case / "outside"
                (project / ".github").mkdir(parents=True)
                outside.mkdir()
                external = outside / "foreign.json"
                original = b'{"mcpServers":{"foreign":{}}}'
                external.write_bytes(original)
                path = project / relative
                try:
                    path.symlink_to(external)
                except (OSError, NotImplementedError) as error:
                    self.skipTest(f"host cannot create symlinks: {error}")
                with self.assertRaisesRegex(AssertionError, "unsafe Copilot Project MCP"):
                    runner["check_removed_native_components"](
                        "copilot", project, case, "remove", project_scope=True)
                self.assertEqual(external.read_bytes(), original)
                path.unlink()
                path.mkdir()
                with self.assertRaisesRegex(AssertionError, "invalid Copilot Project MCP"):
                    runner["check_removed_native_components"](
                        "copilot", project, case, "remove", project_scope=True)
                path.rmdir()
                if relative.startswith(".github/"):
                    (project / ".github").rmdir()
                    (project / ".github").symlink_to(outside, target_is_directory=True)
                    with self.assertRaisesRegex(AssertionError, "unsafe native client root"):
                        runner["check_removed_native_components"](
                            "copilot", project, case, "remove", project_scope=True)

    def test_marketing_kit_copilot_personal_agents_skill_cleanup(self):
        # GitHub's Copilot CLI documents ~/.agents/skills as a personal root.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            skills = home / ".agents" / "skills"
            other = skills / "foreign-skill"
            other.mkdir(parents=True)
            (other / "SKILL.md").write_text(
                "---\nname: foreign-skill\ndescription: preserve\n---\n",
                encoding="utf-8")
            check("copilot", home, case, "User remove")
            owned = skills / "marketing-kit"
            owned.mkdir()
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("copilot", home, case, "User remove")
            owned.rmdir()
            check("copilot", home, case, "User remove")

    def test_marketing_kit_codex_attestation_requires_native_uninstall_proof(self):
        # A signed Project/User binding is not proof of native uninstallation.
        # Only the existing pinned native removal plus fresh absence grants the flag.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            case = Path(temporary) / "A"
            home, project = case / "home", case / "project"
            home.mkdir(parents=True)
            project.mkdir()
            env = {"HOME": str(home), "CODEX_HOME": str(home / ".codex")}
            check = runner["codex_user_native_remove_attestation"]
            calls = []

            def native_uninstall(identity, native_home, native_root, native_env, cwd):
                calls.append((identity, native_home, native_root, cwd))
                return {"completed": False, "reason": "native plugin still registered"}

            with patch.dict(runner["audit"], {
                    "uninstall_native_codex_plugin": native_uninstall}):
                with self.assertRaisesRegex(AssertionError, "native plugin still registered"):
                    check(env, project)
            self.assertEqual(calls, [("hubspot-crm", home, case, project)])

            # A merely truthy field cannot authorize a security attestation.
            for unverified in ({}, {"completed": "true"}, {"completed": False}):
                with patch.dict(runner["audit"], {
                        "uninstall_native_codex_plugin": lambda *_: unverified}):
                    with self.assertRaisesRegex(AssertionError, "unverified"):
                        check(env, project)
            with patch.dict(runner["audit"], {
                    "uninstall_native_codex_plugin": lambda *_: {"completed": True}}):
                self.assertEqual(check(env, project), ("--external-uninstalled",))
                with self.assertRaisesRegex(AssertionError, "exact disposable profile"):
                    check({**env, "CODEX_HOME": str(case / "foreign")}, project)

    def test_marketing_kit_project_a_guard_rejects_rekey_and_lock_rewrite(self):
        # A complete set of distributions is insufficient if B silently
        # replaces A's installation identities or changes A's signed lock.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_project_a_unchanged"]
        baseline = {"root": "id-a", "dependency": "id-child"}
        check(dict(baseline), baseline, b"pinned", b"pinned", "restore B")
        with self.assertRaisesRegex(AssertionError, "installation identities changed"):
            check({"root": "rekeyed", "dependency": "id-child"},
                  baseline, b"pinned", b"pinned", "restore B")
        with self.assertRaisesRegex(AssertionError, "signed lock changed"):
            check(dict(baseline), baseline, b"rewritten", b"pinned", "remove B")

    def test_marketing_kit_peer_baseline_waits_for_own_security_cache(self):
        # Own post-command writes may settle before the peer starts. No writes
        # observed *after* a peer operation are ever excused or rebaselined.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        settle = runner["settled_profile_snapshot"]
        states = iter(({"security": "starting"},
                       {"security": "completed"},
                       {"security": "completed"},
                       {"security": "completed"}))
        with patch.dict(settle.__globals__, {
                "profile_snapshot": lambda *_: next(states),
                "sleep": lambda _: None}):
            self.assertEqual(settle({}, Path("."), attempts=4, interval=0),
                             {"security": "completed"})

        changing = iter(({"cache": index} for index in range(5)))
        with patch.dict(settle.__globals__, {
                "profile_snapshot": lambda *_: next(changing),
                "sleep": lambda _: None}):
            with self.assertRaisesRegex(AssertionError, "did not quiesce"):
                settle({}, Path("."), attempts=5, interval=0)

    def test_marketing_kit_cross_profile_snapshot_detects_hidden_rewrites(self):
        # Sentinels and stable installation IDs do not prove that another
        # profile left native, HOME or UAP state untouched.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "A")
            before = runner["profile_snapshot"](env, project)
            runner["check_profile_unchanged"](env, project, before, "restore B")
            native = Path(env["HOME"]) / ".codex" / "managed.json"
            native.write_bytes(b"before")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, before, "restore B")
            after = runner["profile_snapshot"](env, project)
            original_stat = native.stat()
            native.write_bytes(b"after")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "remove B")
            native.write_bytes(b"before")
            os.utime(native, ns=(original_stat.st_atime_ns,
                                  original_stat.st_mtime_ns))
            runner["check_profile_unchanged"](env, project, after, "remove B")

            # Changes outside the selected client's native roots must fail too.
            sibling = Path(env["HOME"]) / ".config" / "other-app" / "settings.json"
            sibling.parent.mkdir(parents=True)
            sibling.write_bytes(b"foreign")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            sibling.unlink()
            # Clean the newly created empty ancestors as well; they are now
            # intentionally observable foreign-state mutations.
            sibling.parent.rmdir()
            sibling.parent.parent.rmdir()
            runner["check_profile_unchanged"](env, project, after, "restore B")

            # A's durable UAP receipt/data state must remain untouched by B/C.
            receipt = Path(env["AGENTPLUGINS_HOME"]) / "foreign-receipt.json"
            receipt.write_bytes(b"other installation")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "remove C")
            receipt.unlink()
            runner["check_profile_unchanged"](env, project, after, "remove C")

            # A peer must not create unexpected files inside Project A either.
            extra = project / "unexpected-peer-state.json"
            extra.write_bytes(b"foreign")
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            extra.unlink()
            runner["check_profile_unchanged"](env, project, after, "restore B")

            # Empty directories are foreign state too; no file hash would catch them.
            empty = project / "unexpected-empty-peer-dir"
            empty.mkdir()
            with self.assertRaisesRegex(AssertionError, "original isolated profile changed"):
                runner["check_profile_unchanged"](env, project, after, "restore B")
            empty.rmdir()
            runner["check_profile_unchanged"](env, project, after, "restore B")

    def test_marketing_kit_scoped_snapshot_rejects_metadata_only_changes(self):
        # Equal bytes alone cannot prove that a foreign native file was kept.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "peer")
            foreign = Path(env["HOME"]) / ".codex" / "foreign-settings.json"
            foreign.write_bytes(b"same bytes")
            before = runner["profile_snapshot"](env, project)
            check = runner["check_profile_unchanged"]
            original_mode = foreign.stat().st_mode & 0o7777
            try:
                foreign.chmod(original_mode ^ 0o200)
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer chmod")
            finally:
                foreign.chmod(original_mode)
            check(env, project, before, "peer mode restored")

            # A timestamp-only touch is a foreign change even with identical
            # bytes, inode and permissions.
            original_stat = foreign.stat()
            try:
                os.utime(foreign, ns=(original_stat.st_atime_ns,
                                      original_stat.st_mtime_ns - 2_000_000_000))
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer timestamp changed")
            finally:
                os.utime(foreign, ns=(original_stat.st_atime_ns,
                                      original_stat.st_mtime_ns))
            check(env, project, before, "peer timestamp restored")

            # An atomically replaced file with identical bytes is not the
            # original foreign file, even when its permissions are equal.
            saved = root / "saved-foreign-native"
            foreign.replace(saved)
            try:
                foreign.write_bytes(b"same bytes")
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env, project, before, "peer replacement")
            finally:
                foreign.unlink(missing_ok=True)
                saved.replace(foreign)
            check(env, project, before, "peer identity restored")

    def test_marketing_kit_snapshot_rejects_same_mode_directory_replacement(self):
        # The complete protected tree can look identical after swapping an
        # empty directory: bytes, names and permissions alone do not prove
        # foreign-state isolation. Check the profile root and a nested root.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        for nested in (False, True):
            with self.subTest(nested=nested), TemporaryDirectory() as temporary:
                base = Path(temporary)
                protected = base / "protected"
                protected.mkdir()
                victim = protected / "empty" if nested else protected
                if nested:
                    victim.mkdir()
                before = runner["scoped_snapshot"]([protected])
                saved = base / "saved-original"
                victim.rename(saved)
                try:
                    victim.mkdir(mode=saved.stat().st_mode & 0o7777)
                    self.assertNotEqual(runner["scoped_snapshot"]([protected]), before)
                finally:
                    victim.rmdir()
                    saved.rename(victim)
                self.assertEqual(runner["scoped_snapshot"]([protected]), before)

    def test_marketing_kit_native_uninstall_checks_peer_on_success_and_failure(self):
        # The audited native Codex uninstall runs outside the UAP CLI wrapper.
        # Both success and error paths must detect mutations in a peer HOME.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env, project, *_ = runner["run_case"]("unused", "codex", root, "peer")
            before = runner["profile_snapshot"](env, project)
            marker = Path(env["HOME"]) / ".codex" / "unexpected-native-write.json"

            def verify_peer():
                runner["check_profile_unchanged"](env, project, before,
                                                   "native Codex uninstall")

            for must_fail in (False, True):
                def native_command():
                    marker.write_text("unexpected cross-profile write", encoding="utf-8")
                    if must_fail:
                        raise RuntimeError("native CLI failed")
                    return ("--external-uninstalled",)

                try:
                    with self.assertRaisesRegex(AssertionError,
                                                "original isolated profile changed"):
                        runner["guarded_native_attestation"](native_command,
                                                              (verify_peer,))
                finally:
                    marker.unlink(missing_ok=True)
            self.assertEqual(
                runner["guarded_native_attestation"](
                    lambda: ("--external-uninstalled",), (verify_peer,)),
                ("--external-uninstalled",))

            def failed_without_mutation():
                raise RuntimeError("native CLI failed")

            with self.assertRaisesRegex(RuntimeError, "native CLI failed"):
                runner["guarded_native_attestation"](failed_without_mutation,
                                                      (verify_peer,))

    def test_marketing_kit_removed_user_c_profile_is_not_mutated_by_user_a(self):
        # The final User A remove happens *after* User C has been removed.
        # It must not recreate a file in C's HOME or durable UAP data.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            env_c, project_c, *_ = runner["run_case"]("unused", "codex", root, "C")
            before = runner["profile_snapshot"](env_c, project_c)
            check = runner["check_profile_unchanged"]
            check(env_c, project_c, before, "User A remove")
            for path in (Path(env_c["HOME"]) / ".codex" / "unexpected-peer.json",
                         Path(env_c["AGENTPLUGINS_HOME"]) / "unexpected-receipt.json"):
                path.write_text("cross-profile mutation", encoding="utf-8")
                with self.assertRaisesRegex(AssertionError,
                                            "original isolated profile changed"):
                    check(env_c, project_c, before, "User A remove")
                path.unlink()
                check(env_c, project_c, before, "User A remove")

    def test_marketing_kit_snapshot_rejects_symlink_retarget_without_following(self):
        # The shared file/dir snapshot records only 'symlink', which cannot
        # distinguish a link retargeted to a different foreign profile.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            project = root / "project"
            left, right = root / "left", root / "right"
            for directory in (project, left, right):
                directory.mkdir()
            link = project / "external-native-profile"
            try:
                link.symlink_to(left, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            before = runner["scoped_snapshot"]([project])
            # Preserve the original inode while probing both a retarget and
            # a recreated link to the SAME target; neither is foreign-state
            # preservation even when the final target string is unchanged.
            saved = root / "saved-original-native-link"
            link.rename(saved)
            try:
                link.symlink_to(right, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([project]), before)
                link.unlink()
                link.symlink_to(left, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([project]), before)
            finally:
                link.unlink(missing_ok=True)
                saved.rename(link)
            self.assertEqual(runner["scoped_snapshot"]([project]), before)

    def test_marketing_kit_snapshot_rejects_recreated_root_symlink(self):
        # A protected symlink root must retain its inode, not just its target.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            base = Path(temporary)
            target = base / "actual-profile"
            target.mkdir()
            link = base / "protected-profile"
            try:
                link.symlink_to(target, target_is_directory=True)
            except (OSError, NotImplementedError) as error:
                self.skipTest(f"host cannot create directory symlinks: {error}")
            before = runner["scoped_snapshot"]([link])
            saved = base / "saved-original-link"
            link.rename(saved)
            try:
                link.symlink_to(target, target_is_directory=True)
                self.assertNotEqual(runner["scoped_snapshot"]([link]), before)
            finally:
                link.unlink(missing_ok=True)
                saved.rename(link)
            self.assertEqual(runner["scoped_snapshot"]([link]), before)

    def test_marketing_kit_user_scope_cannot_write_untracked_project_state(self):
        # A User command may succeed while creating a new file in the current
        # Project, leaving its signed lock, manifest and sentinels unchanged.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_user_project_unchanged"]
        snapshot = runner["scoped_snapshot"]
        with TemporaryDirectory() as temporary:
            project = Path(temporary) / "project"
            project.mkdir()
            (project / "uap.toml").write_text("original", encoding="utf-8")
            foreign = project / "other-tool.txt"
            foreign.write_text("owned by another tool", encoding="utf-8")
            baseline = snapshot([project])
            check(project, baseline, "User add")

            untracked = project / "unexpected-user-scope-output.json"
            untracked.write_text("unexpected", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User add")
            untracked.unlink()

            original_stat = foreign.stat()
            foreign.write_text("overwritten by User scope", encoding="utf-8")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")
            foreign.write_text("owned by another tool", encoding="utf-8")
            os.utime(foreign, ns=(original_stat.st_atime_ns,
                                   original_stat.st_mtime_ns))
            check(project, baseline, "User remove")

            empty = project / "unexpected-empty-user-dir"
            empty.mkdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")
            empty.rmdir()
            check(project, baseline, "User remove")

    def test_marketing_kit_snapshot_rejects_removed_or_replaced_empty_root(self):
        # A snapshot with no child files previously equated an empty protected
        # Project with a missing root or a regular file. These are mutations.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        snapshot = runner["scoped_snapshot"]
        check = runner["check_user_project_unchanged"]
        with TemporaryDirectory() as temporary:
            project = Path(temporary) / "empty-project"
            project.mkdir()
            baseline = snapshot([project])
            check(project, baseline, "User add")
            project.rmdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User remove")

            missing = snapshot([project])
            project.mkdir()
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, missing, "User add")
            project.rmdir()
            project.write_bytes(b"replaced with a regular file")
            with self.assertRaisesRegex(AssertionError, "User lifecycle modified Project A"):
                check(project, baseline, "User repair")

    def test_marketing_kit_user_project_receipt_ids_are_scope_isolated(self):
        # Fresh User A and cold User C must not inherit receipt identities
        # from removed Project A or B, even with the same signed closure.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_scope_id_isolation"]
        project_a = {"root": "project-a", "child": "project-a-child"}
        project_b = {"root": "project-b", "child": "project-b-child"}
        user = {"root": "user-a", "child": "user-a-child"}
        check(user, (project_a, project_b), "User add")
        for copied in (project_a["root"], project_b["child"]):
            with self.assertRaisesRegex(AssertionError,
                                        "reused installation IDs across scopes"):
                check({"root": copied, "child": "fresh-user"},
                      (project_a, project_b), "User cold restore")

    def test_marketing_kit_cold_restore_guard_requires_full_disjoint_lock(self):
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        check = runner["check_cold_restore"]
        original = {dist: f"A-{i}"
                    for i, dist in enumerate(sorted(runner["EXPECTED"]))}
        restored = {dist: f"B-{i}"
                    for i, dist in enumerate(sorted(runner["EXPECTED"]))}
        check(restored, original, b"pinned", b"pinned",
              b"portable", b"portable", "User B")
        incomplete = dict(restored)
        incomplete.pop(next(iter(incomplete)))
        with self.assertRaisesRegex(AssertionError, "mandatory signed closure"):
            check(incomplete, original, b"pinned", b"pinned",
                  b"portable", b"portable", "User B")
        reused = {**restored, next(iter(restored)): next(iter(original.values()))}
        with self.assertRaisesRegex(AssertionError, "reused installation IDs"):
            check(reused, original, b"pinned", b"pinned",
                  b"portable", b"portable", "User B")
        with self.assertRaisesRegex(AssertionError, "modified pinned lock"):
            check(restored, original, b"pinned", b"tampered",
                  b"portable", b"portable", "User B")
        with self.assertRaisesRegex(AssertionError, "modified portable manifest"):
            check(restored, original, b"pinned", b"pinned",
                  b"portable", b"tampered", "User B")

    def test_marketing_kit_lock_guard_rejects_duplicate_or_unidentified_pins(self):
        # Comparing lock keys as a set can hide a duplicate signed package.
        from runpy import run_path

        runner = run_path(str(Path(__file__).resolve().parent /
                              "marketing-kit-project-release-e2e.py"))
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            def check(packages):
                (project / "uap.lock").write_text(
                    json.dumps({"packages": packages}), encoding="utf-8")
                return runner["lock_ids"](project)

            root = {"source_key": "directory:777genius/marketing-kit"}
            child = {"source_key": "directory:777genius/notion"}
            self.assertEqual(check([root, child]),
                             {root["source_key"], child["source_key"]})
            with self.assertRaisesRegex(AssertionError, "duplicate signed lock"):
                check([root, root])
            for invalid in (None, "", 123):
                with self.assertRaisesRegex(AssertionError, "invalid signed lock"):
                    check([{"source_key": invalid}])
            with self.assertRaisesRegex(AssertionError, "invalid signed lock"):
                check([{"other": "directory:777genius/notion"}])

    def test_published_windows_runner_scratch_is_real_absolute_and_ci_only(self):
        audit = load_audit()
        with TemporaryDirectory() as scratch:
            with patch.object(audit.sys, "platform", "win32"):
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": scratch}):
                    self.assertEqual(audit.published_audit_temp_parent(), scratch)
                    with patch.object(audit.Path, "is_symlink", return_value=True):
                        with self.assertRaisesRegex(RuntimeError, "redirected"):
                            audit.published_audit_temp_parent()
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "false", "RUNNER_TEMP": scratch}):
                    self.assertIsNone(audit.published_audit_temp_parent())
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": "relative/scratch"}):
                    with self.assertRaisesRegex(RuntimeError, "redirected"):
                        audit.published_audit_temp_parent()
            with patch.object(audit.sys, "platform", "darwin"):
                with patch.dict(audit.os.environ, {
                        "GITHUB_ACTIONS": "true", "RUNNER_TEMP": scratch}):
                    self.assertIsNone(audit.published_audit_temp_parent())

    def test_bootstrap_refusal_preserves_index_source_and_real_identity(self):
        # BUG-053: the original blocked ZIPs had 30 status placeholders but
        # omitted their source and index, so fixture slots could not be audited.
        audit = load_audit()
        source = "github:sample/agent-plugin@" + ("a" * 40)
        rows = audit.unexecuted_source_rows(
            [source, "atlassian"], "plugin", "grok", "client-unavailable",
            {source: "sample-agent-plugin"})
        self.assertEqual([v["index"] for v in rows], [0, 1])
        self.assertEqual([v["source"] for v in rows], [source, "atlassian"])
        self.assertEqual([v["identity"] for v in rows],
                         ["sample-agent-plugin", "atlassian"])
        self.assertTrue(all(v["status"] == "client-unavailable"
                            and "install" not in v for v in rows))
        skill = {"name": "pdf", "source": "github:sample/skills@" + ("b" * 40)}
        blocked = audit.unexecuted_source_rows(
            [skill], "skill", "chatgpt", "documented-unsupported", {})
        self.assertEqual(blocked[0]["source"], skill["source"])
        self.assertEqual(blocked[0]["index"], 0)
        self.assertEqual(blocked[0]["identity"], "pdf")
        with self.assertRaisesRegex(ValueError, "non-execution"):
            audit.unexecuted_source_rows([], "plugin", "grok", "passed", {})

    def test_rebinds_all_client_overrides_and_strips_git_injection(self):
        audit = load_audit()
        hostile = {key: "/outside/actual-user-profile" for key in audit.PROFILE_ENV_KEYS}
        hostile.update({
            "HOME": "/outside/actual-user-home",
            "USERPROFILE": "/outside/actual-user-home",
            "XDG_CONFIG_HOME": "/outside/config",
            "XDG_DATA_HOME": "/outside/data",
            "APPDATA": "/outside/real/windows-roaming",
            "LOCALAPPDATA": "/outside/real/windows-local",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "GIT_SSH_COMMAND": "/outside/host-ssh",
            "SSH_ASKPASS": "/outside/host-askpass",
        })
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, hostile):
                env = audit.isolated_client_env(home, case, data)
            for key in audit.PROFILE_ENV_KEYS:
                if key in env:
                    self.assertTrue(Path(env[key]).resolve().is_relative_to(home.resolve()), (key, env[key]))
                else:
                    self.assertNotIn(key, env)
            for key in ("HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME",
                        "XDG_CACHE_HOME", "XDG_STATE_HOME", "GIT_CONFIG_GLOBAL",
                        "APPDATA", "LOCALAPPDATA", "AGENTPLUGINS_HOME", "TMPDIR"):
                self.assertTrue(Path(env[key]).resolve().is_relative_to(case.resolve()), (key, env[key]))
            self.assertEqual(env["GEMINI_CLI_HOME"], str(home))
            for key in ("GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
                        "GIT_SSH_COMMAND", "SSH_ASKPASS"):
                self.assertNotIn(key, env)

    def test_drops_unrelated_credentials_but_retains_public_runtime_plumbing(self):
        audit = load_audit()
        trusted = {
            "PATH": "/usr/local/bin:/usr/bin:/bin",
            "HTTPS_PROXY": "http://proxy.invalid:3128",
            "LANG": "C.UTF-8",
        }
        canaries = {
            "OPENAI_API_KEY": "never-forward-openai-canary",
            "ANTHROPIC_API_KEY": "never-forward-anthropic-canary",
            "GH_TOKEN": "never-forward-gh-canary",
            "GITHUB_TOKEN": "never-forward-github-canary",
            "ACTIONS_RUNTIME_TOKEN": "never-forward-actions-canary",
            "AWS_SECRET_ACCESS_KEY": "never-forward-aws-canary",
            "NPM_TOKEN": "never-forward-npm-canary",
            "SSH_AUTH_SOCK": "/outside/ssh-agent.sock",
            "GIT_CONFIG_COUNT": "1",
            "GIT_CONFIG_KEY_0": "core.hooksPath",
            "GIT_CONFIG_VALUE_0": "/outside/hooks",
            "CODEX_HOME": "/outside/real-codex-profile",
        }
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {**trusted, **canaries}, clear=True):
                env = audit.isolated_client_env(home, case, data)
        for key, value in trusted.items():
            self.assertEqual(env[key], value)
        for key in canaries:
            if key != "CODEX_HOME":
                self.assertNotIn(key, env)
        self.assertTrue(Path(env["CODEX_HOME"]).is_relative_to(home))
        self.assertEqual(env["CI"], "true")

    def test_grok_uses_disposable_default_home_without_custom_override(self):
        # BUG-057: even a HOME/.grok GROK_HOME value makes the genuine Grok
        # adapter select grok_custom_home and refuse normal project Skills.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {
                "GROK_HOME": "/outside/real-grok-profile",
                "HOME": "/outside/real-home",
                "XDG_CONFIG_HOME": "/outside/real-config",
            }):
                env = audit.isolated_client_env(home, case, data)
            self.assertNotIn("GROK_HOME", env)
            self.assertEqual(env["HOME"], str(home))
            self.assertEqual(env["XDG_CONFIG_HOME"], str(home / ".config"))
            self.assertTrue((Path(env["HOME"]) / ".grok").is_relative_to(case))
            self.assertEqual(list(home.iterdir()), [])

    def test_version_evidence_is_short_and_never_claims_failed_probe(self):
        audit = load_audit()
        with patch.object(audit, "call", return_value={
            "stdout_tail": "codex-cli 0.153.4\n" + ("A" * 500),
            "stderr_tail": "", "exit_code": 0, "timeout": False,
        }) as run:
            reported = audit.version_evidence(["codex", "--version"], {"HOME": "/isolated"}, "/tmp")
            self.assertEqual(reported["status"], "reported")
            self.assertLessEqual(len(reported["value"]), 200)
            self.assertEqual(run.call_args.args[1], {"HOME": "/isolated"})
        for exit_code, stdout, timeout, status in [
            (2, "version 1", False, "probe-failed"),
            (0, "", False, "empty-output"),
            (None, "", True, "timeout"),
        ]:
            with patch.object(audit, "call", return_value={
                "stdout_tail": stdout, "stderr_tail": "",
                "exit_code": exit_code, "timeout": timeout,
            }):
                self.assertEqual(audit.version_evidence(["client", "--version"], {}, "/tmp")["status"], status)

    def test_refuses_case_data_outside_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            case, home, outside = parent / "case", parent / "case/home", parent / "outside"
            home.mkdir(parents=True)
            outside.mkdir()
            with self.assertRaisesRegex(ValueError, "disposable case"):
                audit.isolated_client_env(home, case, outside)

    def test_success_diagnostics_and_timeout_are_never_source_refusals(self):
        audit = load_audit()
        for exit_code in (0, None):
            self.assertFalse(audit.is_cli_refusal({
                "exit_code": exit_code,
                "stdout_tail": '{"result":"success","component_support":"unsupported"}',
                "stderr_tail": "target_unavailable",
            }))
        self.assertTrue(audit.is_cli_refusal({
            "exit_code": 1,
            "stderr_tail": "release does not support complete target set",
        }))

    def test_kiro_darwin_containment_refusal_remains_nonpass(self):
        # BUG-048: a real Kiro 3.0.0 macOS CLI returns this exact
        # fail-closed diagnostic before any native profile mutation.
        audit = load_audit()
        darwin = ("manual_activation_required: automatic native Kiro MCP lifecycle "
                  "containment preflight failed: safe duplex process containment "
                  "unavailable on Darwin; manual activation required")
        linux = "manual_activation_required: cgroup v2 delegation unavailable"
        for diagnostic in (darwin, linux):
            with self.subTest(diagnostic=diagnostic):
                self.assertTrue(audit.host_containment_refusal(diagnostic))
                refused = {"exit_code": 1, "stdout_tail": "",
                           "stderr_tail": diagnostic, "timeout": False,
                           "_json": {"data": {"installations": []}}}
                with TemporaryDirectory() as temporary:
                    with patch.object(audit, "call", return_value=refused):
                        evidence = audit.do_case(
                            "/unused/audit-cli", "kiro", "plugin", "atlassian",
                            0, Path(temporary).resolve(), {})
                self.assertEqual(evidence["status"], "host-containment-unavailable")
                self.assertFalse(evidence["listed_identity"])
                self.assertFalse(evidence["failed_install_residue"])
        for diagnostic in (
            "manual_activation_required: native Kiro registry invalid",
            "safe duplex process containment unavailable on Darwin",
            "native process failed; unexpected exception",
        ):
            self.assertFalse(audit.host_containment_refusal(diagnostic))
        # The same words in an otherwise successful response must not
        # qualify a refusal. The CLI's nonzero exit remains mandatory.
        self.assertFalse(audit.is_cli_refusal({
            "exit_code": 0, "stderr_tail": darwin, "stdout_tail": "",
            "timeout": False}))

    def test_chatgpt_identity_ownership_refusal_requires_nonzero_exit_and_no_mutation(self):
        audit = load_audit()
        ownership_error = (
            "agentplugins: group apply preflight failed; no target was changed "
            "(selected targets: [chatgpt]): target chatgpt identity preflight: "
            "native identity ownership is indeterminate; refusing mutation")
        empty = {"exit_code": 0,
                 "stdout_tail": '{"schema_version":1,"command":"list",'
                                '"result":"success","data":{"installations":[]}}',
                 "stderr_tail": "", "timeout": False,
                 "_json": {"data": {"installations": []}}}
        cases = [
            ("chatgpt", 1, ownership_error, "", "native-ownership-unverified"),
            ("codex", 1, ownership_error, "", "install-failed"),
            ("chatgpt", 0, ownership_error, "", "incomplete-native-proof"),
            ("chatgpt", 1, "generic failure", ownership_error, "install-failed"),
            ("chatgpt", 1, ownership_error,
             '{"components":[{"support":"unsupported"}]}',
             "documented-or-source-unsupported"),
        ]
        for target, exit_code, stderr, stdout, expected in cases:
            with self.subTest(target=target, exit_code=exit_code, expected=expected):
                refused = {"exit_code": exit_code, "stdout_tail": stdout,
                           "stderr_tail": stderr, "timeout": False, "_json": None}
                def source_call(argv, *_args, **_kwargs):
                    return dict(empty if len(argv) > 1 and argv[1] == "list" else refused)
                with TemporaryDirectory() as temporary:
                    with patch.object(audit, "call", side_effect=source_call):
                        result = audit.do_case(
                            "/unused/audit-cli", target, "plugin", "atlassian",
                            0, Path(temporary).resolve(), {})
                self.assertEqual(result["status"], expected)
                self.assertFalse(result["failed_install_registered"])
                self.assertFalse(result["failed_install_residue"])
                self.assertTrue(result["sentinels_preserved"])

    def test_every_native_plugin_client_preserves_foreign_profile_file(self):
        audit = load_audit()
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": None}
        with TemporaryDirectory() as temporary:
            with patch.object(audit, "call", return_value=refused):
                for target, roots in audit.NATIVE_ROOTS.items():
                    if not roots:
                        continue  # ChatGPT is remote/manual, no fabricated native profile
                    with self.subTest(target=target):
                        runroot = Path(temporary).resolve() / target
                        evidence = audit.do_case(
                            "/unused/audit-cli", target, "plugin", "atlassian", 0,
                            runroot, {}
                        )
                        foreign = runroot / "plugin-00/home" / roots[0] / "foreign-neighbor.txt"
                        self.assertTrue(foreign.is_file())
                        self.assertEqual(foreign.read_text(),
                                         "unmanaged native neighbor must survive\n")
                        self.assertTrue(evidence["sentinels_preserved"])


    def test_relocated_skill_payload_never_qualifies_as_removed(self):
        # BUG-027: dropping SKILL.md and moving a payload used to evade
        # both the identity scan and the known installation path check.
        import shutil
        audit = load_audit()
        item = {"name": "demo-skill", "source": "https://github.com/example/repo",
                "skill_md_git_blob_sha1": "a" * 40}
        installed = [{"name": "demo-skill", "installation_id": "owned-install",
                      "clients": [{"client_id": "claude"}]}]
        def success(rows=None):
            return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": rows or []}}}

        for relocate in (True, False):
            with self.subTest(relocate=relocate), TemporaryDirectory() as tmp:
                state = {"added": 0, "removed": False}
                project = Path(tmp) / "skill-00/project"

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    skill_dir = project / ".claude/skills/demo-skill"
                    if operation == "skills":
                        if state["added"] == 0:
                            (skill_dir / "scripts").mkdir(parents=True)
                            (skill_dir / "SKILL.md").write_text("name: demo-skill\n")
                            (skill_dir / "scripts/payload.sh").write_text("echo payload\n")
                            (project / "uap.toml").write_text("demo-skill")
                            (project / "uap.lock").write_text("demo-skill")
                        state["added"] += 1
                    elif operation == "remove":
                        if relocate:
                            moved = skill_dir.with_name("other")
                            skill_dir.rename(moved)
                            (moved / "SKILL.md").unlink()
                        else:
                            shutil.rmtree(skill_dir)
                        (project / "uap.toml").unlink()
                        (project / "uap.lock").unlink()
                        state["removed"] = True
                    elif operation == "list":
                        return success([] if state["removed"] else installed)
                    else:
                        raise AssertionError(argv)
                    return success()

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=installed), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True):
                    evidence = audit.do_case(
                        "/unused/agentplugins", "claude", "skill", item, 0,
                        Path(tmp), {})

                self.assertTrue(evidence["sentinels_preserved"])
                self.assertIsNone(evidence["post_removal_skill"])
                self.assertEqual(evidence["post_removal_leftovers"], [])
                if relocate:
                    self.assertIn("skills/other/scripts/payload.sh",
                                  evidence["post_skill_tree_delta"])
                    self.assertFalse(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "cleanup-failed")
                else:
                    self.assertEqual(evidence["post_skill_tree_delta"], [])
                    self.assertTrue(evidence["removal_verified"])
                    self.assertEqual(evidence["status"], "passed")

    def test_codex_remove_attests_only_pinned_absolute_executable(self):
        # BUG-028: an untrusted package must not shadow codex on PATH.
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            case = Path(tmp).resolve() / "case"
            home = case / "home"
            conf = home / ".codex/config.toml"
            conf.parent.mkdir(parents=True)
            market_dir = case / "managed-marketplace"
            market_dir.mkdir(parents=True)
            market = "agentplugins-012345abcdef"
            conf.write_text(
                '[plugins."demo@' + market + '"]\nenabled = true\n'
                '[marketplaces.' + market + ']\nsource_type = "local"\n'
                'source = "' + str(market_dir) + '"\n')
            commands = []
            def fake_call(argv, env, cwd, timeout=40):
                commands.append(argv)
                if len(commands) == 2:
                    conf.write_text("")
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False, "_json": None}
            pinned = case / "trusted-codex"
            pinned.write_text("test executable")
            with patch.dict(audit.AGENT_BINARIES, {"codex": str(pinned)}, clear=True), \
                 patch.object(audit, "call", side_effect=fake_call):
                result = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {"HOME": str(home)}, case)
            self.assertTrue(result["completed"])
            self.assertEqual(len(commands), 2)
            self.assertTrue(all(cmd[0] == str(pinned) for cmd in commands))
            with patch.dict(audit.AGENT_BINARIES, {}, clear=True), \
                 patch.object(audit, "call") as forbidden:
                refused = audit.uninstall_native_codex_plugin(
                    "demo", home, case, {}, case)
                forbidden.assert_not_called()
            self.assertFalse(refused["completed"])
            self.assertIn("pinned", refused["reason"])

    def test_chatgpt_prepared_requires_registration_and_verified_cleanup(self):
        # BUG-013: a zero-exit CLI can still leave no managed installation.
        audit = load_audit()
        source = "github:example/chatgpt-demo@" + ("a" * 40)
        identity = "chatgpt-demo"
        matched = [{"name": identity, "installation_id": "isolated-owned-install",
                    "clients": [{"client_id": "chatgpt",
                                 "verification": "installation_verified"}]}]

        def response(exit_code, installations=None):
            return {"exit_code": exit_code, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False,
                    "_json": {"data": {"installations": installations or []}}}

        def run_case(outputs, matches):
            with TemporaryDirectory() as temporary:
                with patch.object(audit, "call", side_effect=outputs), \
                     patch.object(audit, "matching_installations", return_value=matches):
                    return audit.do_case(
                        "/unused/agentplugins", "chatgpt", "plugin", source, 0,
                        Path(temporary).resolve(), {source: identity})

        empty = run_case([response(0), response(0)], [])
        self.assertEqual(empty["status"], "incomplete-native-proof")
        self.assertFalse(empty["listed_identity"])
        self.assertIsNone(empty["cleanup"])

        # Initial add/list, repeat add/list, then failed remove.
        failed_remove = run_case(
            [response(0), response(0), response(0), response(0), response(1)], matched)
        self.assertEqual(failed_remove["status"], "cleanup-failed")
        self.assertFalse(failed_remove["removal_verified"])

        # Full lifecycle may qualify only with an exact, successful repeat.
        failed_repeat = run_case(
            [response(0), response(0), response(1), response(0), response(0), response(0)],
            matched)
        self.assertEqual(failed_repeat["status"], "repeat-add-unqualified")
        self.assertFalse(failed_repeat["repeat_verified"])
        self.assertTrue(failed_repeat["removal_verified"])

        prepared = run_case(
            [response(0), response(0), response(0), response(0), response(0), response(0)],
            matched)
        self.assertEqual(prepared["status"], "manual-setup-prepared")
        self.assertTrue(prepared["repeat_verified"])
        self.assertTrue(prepared["repeat_native_unchanged"])
        self.assertTrue(prepared["repeat_lock_unchanged"])
        self.assertEqual(prepared["repeat_installation_ids"], ["isolated-owned-install"])
        self.assertTrue(prepared["listed_identity"])
        self.assertTrue(prepared["removal_verified"])
        self.assertTrue(prepared["sentinels_preserved"])

    def test_both_native_matrices_bind_real_bootstrap_outcomes(self):
        # BUG-014: present binary + failed native --version must never produce PASS.
        # This static guard joins the GitHub step outcome to the harness refusal.
        workflows = (
            (".github/workflows/live-install-audit.yml", "native-install"),
            (".github/workflows/live-install-audit.yml", "native-install-pilot"),
            (".github/workflows/live-install-audit.yml", "published-install"),
            (".github/workflows/live-install-remaining-clients.yml", "vendor-install"),
        )
        for relative_path, step_id in workflows:
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn(f"id: {step_id}", workflow)
                self.assertIn(
                    "AUDIT_VENDOR_OUTCOME: ${{ steps." + step_id + ".outcome }}",
                    workflow,
                )

    def test_harness_exception_records_exact_source_slot_without_network(self):
        # BUG-059: an exception must not erase the source/identity evidence.
        audit = load_audit()
        fixtures = json.loads(audit.FIXTURES.read_text())
        source = fixtures["plugin_aliases"][0]
        with TemporaryDirectory() as temporary:
            cli = Path(temporary) / "inert-cli"
            cli.write_text("no native execution")
            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch("sys.argv", ["live-install-audit.py", "--cli", str(cli),
                                       "--target", "chatgpt", "--kind", "plugin",
                                       "--limit", "1", "--report-dir", temporary]), \
                    patch.object(audit, "version_evidence", return_value={"status": "reported"}), \
                    patch.object(audit, "do_case", side_effect=ValueError("injected")):
                self.assertEqual(audit.main(), 1)
            report = json.loads((Path(temporary) / "chatgpt-plugin.json").read_text())
        self.assertEqual(report["cases_attempted"], 1)
        case = report["results"][0]
        self.assertEqual(case["status"], "harness-error")
        self.assertEqual(case["index"], 0)
        self.assertEqual(case["source"], source)
        self.assertEqual(case["identity"], fixtures["plugin_declared_names"].get(source, source))
        self.assertEqual((case["kind"], case["target"]), ("plugin", "chatgpt"))

    def test_reports_distinct_pr_head_and_checked_merge_sha(self):
        # BUG-015: pull_request GITHUB_SHA is a merge checkout, not PR head.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            with patch.dict(os.environ, {
                "GITHUB_SHA": "a" * 40,
                "AUDIT_PR_HEAD_SHA": "b" * 40,
                "AUDIT_VENDOR_OUTCOME": "failure",
            }), patch("sys.argv", [
                "live-install-audit.py", "--cli", "/unused/agentplugins",
                "--target", "codex", "--kind", "skill", "--limit", "1",
                "--report-dir", temporary,
            ]):
                self.assertEqual(audit.main(), 2)
            report = json.loads((Path(temporary) / "codex-skill.json").read_text())
        self.assertEqual(report["github_sha"], "a" * 40)
        self.assertEqual(report["pr_head_sha"], "b" * 40)
        self.assertEqual(report["results"][0]["status"], "client-unavailable")

    def test_both_matrix_workflows_export_pr_source_head(self):
        for relative_path in (
            ".github/workflows/live-install-audit.yml",
            ".github/workflows/live-install-remaining-clients.yml",
        ):
            with self.subTest(workflow=relative_path):
                workflow = (Path(__file__).resolve().parents[1] / relative_path).read_text()
                self.assertIn("AUDIT_PR_HEAD_SHA: ${{ github.event.pull_request.head.sha || '' }}", workflow)

    def test_failed_vendor_bootstrap_cannot_inherit_unrelated_host_binary(self):
        audit = load_audit()
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "failure"}):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}):
            self.assertTrue(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {}, clear=True):
            self.assertFalse(audit.vendor_bootstrap_attested())
        with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": ""}):
            self.assertFalse(audit.vendor_bootstrap_attested())

    def test_native_launcher_refuses_unreviewed_path_shadow_without_execution(self):
        # BUG-058: matching the artifact digest does not authenticate an
        # unrelated executable that wins runner PATH resolution.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        marker = 'python3 - "$BIN" "$executable" <<\'PY\'\n'
        self.assertIn(marker, workflow)
        source = textwrap.dedent(workflow.split(marker, 1)[1].split("\n          PY", 1)[0])
        with TemporaryDirectory() as temporary:
            root = Path(temporary)
            untrusted = root / "runner-tools"
            verified = root / "vendor"
            untrusted.mkdir()
            verified.mkdir()
            for path in (untrusted / "grok", verified / "grok"):
                path.write_text("#!/bin/sh\nexit 0\n")
                path.chmod(0o755)
            for first, allowed in ((untrusted, False), (verified, True)):
                second = verified if first == untrusted else untrusted
                env = dict(os.environ, PATH=f"{first}:{second}")
                result = subprocess.run(
                    [sys.executable, "-c", source, "grok", str(verified / "grok")],
                    env=env, capture_output=True, text=True, check=False)
                self.assertEqual(result.returncode == 0, allowed)
                if not allowed:
                    self.assertIn("unverified native PATH executable", result.stderr)

    def test_vendor_bootstrap_requires_reviewed_bytes_and_preserves_runner_toolchain(self):
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn("sha256_file -c -", workflow)
        self.assertIn("observed-untrusted.sha256", workflow)
        self.assertIn("AUDIT_VENDOR_OUTCOME", workflow)
        self.assertIn("shasum -a 256", workflow)
        self.assertIn("verification-blocked.txt", workflow)
        self.assertIn("dpkg-deb --extract", workflow)
        self.assertNotIn("sudo DEBIAN_FRONTEND", workflow)
        self.assertNotIn('export PATH="$HOME/.local/bin:$PATH"', workflow)
        self.assertIn(":/usr/bin:/sbin:/bin:", workflow)

    def test_grok_build_uses_independently_sha_pinned_binary_not_installer(self):
        # BUG-056: Grok Build's official immutable Homebrew Cask publishes
        # a Linux x64 ELF digest; a mutable shell installer cannot qualify E2E.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn('https://x.ai/cli/grok-1.0.70-linux-x86_64', workflow)
        self.assertIn('SHA256="22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9"', workflow)
        self.assertIn('Homebrew/homebrew-cask Casks/g/grok-build.rb @ b75a8e94456970fa349c75b59ca5e4183dde8e28', workflow)
        self.assertNotIn('https://x.ai/cli/install.sh', workflow)
        self.assertNotIn('bash "$artifact"', workflow)
        self.assertIn('artifact="$RUNNER_TEMP/grok-1.0.70-linux-x86_64"', workflow)
        self.assertIn('executable="$RUNNER_TEMP/vendor-root/grok"', workflow)
        self.assertIn('install -m 0755 "$artifact" "$executable"', workflow)
        self.assertIn('ln -s "$executable" "$HOME/.local/bin/grok"', workflow)
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('install -m 0755 "$artifact" "$executable"'))
        self.assertIn('env -i PATH="$PATH" HOME="$probe_home"', workflow)

    def test_devin_project_skills_use_exact_reviewed_native_version(self):
        # BUG-054: actual 30/30 project Skills refused with Devin 3000.11.234;
        # production requireCurrentDevin accepts only 3000.11.205.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertEqual(workflow.count("            windsurf)"), 2)
        vendor_block, sha_block = (
            block.split("              ;;", 1)[0]
            for block in workflow.split("            windsurf)")[1:]
        )
        self.assertIn('if [[ "$KIND" == skill ]]; then', vendor_block)
        self.assertIn('if [[ "$KIND" == skill ]]; then', sha_block)
        self.assertIn("devin-3000.11.205-x86_64-unknown-linux.tar.gz", vendor_block)
        self.assertIn("devin-3000.11.234-x86_64-unknown-linux.tar.gz", vendor_block)
        self.assertIn('SHA256="2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4"', sha_block)
        self.assertIn('SHA256="201370cddd2924294e3ca6b46fd8ece49b5763953b644a1dd823fce5490cb41a"', sha_block)
        self.assertLess(vendor_block.index("3000.11.205"), vendor_block.index("3000.11.234"))
        self.assertLess(sha_block.index("2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4"),
                        sha_block.index("201370cddd2924294e3ca6b46fd8ece49b5763953b644a1dd823fce5490cb41a"))
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('archive.extractall(path=sys.argv[2], filter="data")'))
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertIn('env -i PATH="$PATH" HOME="$probe_home"', workflow)

    def test_vscode_deb_uses_exact_verified_staged_launcher(self):
        # BUG-055: /usr/bin/code may be an absolute .deb symlink, not a
        # runnable binary after extraction into an unprivileged temp root.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn('executable="$RUNNER_TEMP/vendor-root/usr/share/code/bin/code"', workflow)
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertIn('test "$(readlink -f "$executable")" = "$executable"', workflow)
        self.assertIn('ln -s "$executable" "$HOME/.local/bin/code"', workflow)
        self.assertIn('python3 - "$BIN" "$executable" <<\'PY\'', workflow)
        self.assertIn('Path(selected).resolve(strict=True) != Path(sys.argv[2]).resolve(strict=True)', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('dpkg-deb --extract "$artifact"'))
        self.assertIn('vscode) SHA256="b8fbff92d8d95908990eed0acf004c980f29003725469e794ddbd017ea511a20"', workflow)

    def test_devin_user_plugin_without_selected_windsurf_channel_is_nonpass(self):
        # BUG-055: a real devin CLI does not select a legacy Windsurf user
        # profile, so fail-closed root selection is not a broken package.
        audit = load_audit()
        refused = {"exit_code": 1, "stdout_tail": "",
                   "stderr_tail": ("preflight atlassian/windsurf: installer request "
                                   "rejected: ClientConfigRoot must be an explicit "
                                   "absolute clean path"),
                   "timeout": False, "_json": None}
        empty = {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                 "timeout": False, "_json": {"data": {"installations": []}}}

        def fake_call(argv, env, cwd, timeout=105):
            return refused if argv[1] == "add" else empty

        with TemporaryDirectory() as temporary, patch.object(audit, "call", side_effect=fake_call):
            evidence = audit.do_case("/unused/uap", "windsurf", "plugin",
                                     "atlassian", 0, Path(temporary), {})
        self.assertEqual(evidence["status"], "target-unavailable")
        self.assertEqual(evidence["install"]["exit_code"], 1)
        self.assertFalse(evidence["listed_identity"])
        self.assertFalse(evidence["failed_install_residue"])
        self.assertTrue(evidence["sentinels_preserved"])
        self.assertIsNone(evidence["cleanup"])

    def test_kiro_native_archive_is_pinned_before_extraction(self):
        # Independently pinned Homebrew Cask 3.0.0, not a mutable install script.
        workflow = (Path(__file__).resolve().parents[1] /
                    ".github/workflows/live-install-remaining-clients.yml").read_text()
        self.assertIn("https://desktop-release.q.us-east-1.amazonaws.com/3.0.0/kirocli-universal-darwin.zip", workflow)
        self.assertIn('kiro) SHA256="0a8f067c4fe6a23a231aa3a18e3e4332120ef9ed3d2d7b6eea907beb8b4cdd8b"', workflow)
        self.assertNotIn('kiro) URL="https://cli.kiro.dev/install"', workflow)
        self.assertLess(workflow.index('sha256_file -c -'),
                        workflow.index('ditto -x -k "$artifact"'))
        self.assertIn('test -f "$executable" && test -x "$executable" && test ! -L "$executable"', workflow)
        self.assertNotIn('export PATH="$HOME/.local/bin:$PATH"', workflow)

    def test_failed_add_inspects_pending_registration_without_false_matches(self):
        audit = load_audit()
        pending = {"data": {"installations": [
            {"name": "atlassian", "clients": [
                {"client_id": "grok", "verification": "pending"}]},
            {"name": "other", "clients": [{"client_id": "codex"}]},
        ]}}
        self.assertTrue(audit.has_target_registration(pending, "atlassian", "grok"))
        self.assertFalse(audit.has_target_registration(pending, "atlassian", "codex"))
        self.assertFalse(audit.has_target_registration(pending, "other", "grok"))
        for malformed in (None, [], {}, {"data": {"installations": "bad"}},
                          {"data": {"installations": [None, "bad"]}}):
            self.assertFalse(audit.has_target_registration(malformed, "atlassian", "grok"))

    def test_signed_release_observation_only_exposes_public_allowlisted_fields(self):
        # BUG-025: diagnose revision drift without logging credentials.
        audit = load_audit()
        response = {"data": {"installations": [
            {"name": "atlassian", "source": "777genius/universal-agent-plugins/plugins/atlassian",
             "api_key": "secret-from-account",
             "directory": {"recorded_distribution": "777genius/atlassian",
                           "recorded_revision": "a" * 40,
                           "recorded_release_sequence": 1,
                           "private_config": "secret"},
             "clients": [
                 {"client_id": "codex",
                  "package_revision": {
                      "resolved_revision": "b" * 40, "release_sequence": 1,
                      "tree_digest": "sha256:" + "c" * 64,
                      "manifest_digest": "sha256:" + "d" * 64,
                      "credential": "secret"}},
                 {"client_id": "claude",
                  "package_revision": {"credential": "other-secret"}},
             ]}
        ]}}
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertEqual(len(observed), 1)
        self.assertEqual(observed[0]["source"], "777genius/universal-agent-plugins/plugins/atlassian")
        self.assertEqual(observed[0]["resolved_revision"], "b" * 40)
        self.assertNotIn("secret", repr(observed))
        response["data"]["installations"][0]["source"] = (
            "https://alice:secret@host.invalid/private")
        observed = audit.signed_release_public_observation(
            response, "atlassian", "codex")
        self.assertIsNone(observed[0]["source"])
        self.assertEqual(audit.signed_release_public_observation(
            response, "atlassian", "grok"), [])


    def test_signed_release_requires_same_verified_target_and_immutable_metadata(self):
        audit = load_audit()
        directory = {
            "product_id": "atlassian",
            "recorded_distribution": "777genius/atlassian",
            "recorded_revision": "a" * 40,
            "recorded_release_sequence": 1,
        }
        revision = {
            "distribution_id": "777genius/atlassian",
            "resolved_revision": "a" * 40,
            "release_sequence": 1,
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        verified = {"client_id": "codex", "verification": "installation_verified",
                    "package_revision": revision}
        node = {"name": "atlassian", "installation_id": "valid-id",
                "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                "clients": [verified]}
        pins = {"atlassian": {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }}
        def matching():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "atlassian", "codex",
                "plugin", "atlassian", pins,
            )
        self.assertEqual(len(matching()), 1)
        for key, value in (("resolved_revision", "d" * 40),
                           ("distribution_id", "wrong/package"),
                           ("release_sequence", 2),
                           ("tree_digest", "sha256:short"),
                           ("manifest_digest", None)):
            with self.subTest(key=key):
                broken = dict(revision, **{key: value})
                node["clients"] = [dict(verified, package_revision=broken)]
                self.assertEqual(matching(), [])
        node["clients"] = [
            dict(verified, package_revision={}),
            {"client_id": "codex", "verification": "pending",
             "package_revision": revision},
        ]
        self.assertEqual(matching(), [])
        node["clients"] = [dict(verified, client_id="grok")]
        self.assertEqual(matching(), [])

    def test_signed_release_rejects_forged_source_even_when_self_consistent(self):
        audit = load_audit()
        pin = {
            "distribution": "777genius/atlassian",
            "repository": "777genius/universal-agent-plugins",
            "path": "plugins/atlassian", "revision": "a" * 40,
            "sequence": 1, "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64,
        }
        directory = {"product_id": "atlassian", "recorded_distribution": pin["distribution"],
                     "recorded_revision": pin["revision"], "recorded_release_sequence": 1}
        revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                    "release_sequence": 1, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}
        record = {"name": "atlassian", "installation_id": "proof",
                  "source": "777genius/universal-agent-plugins/plugins/atlassian", "directory": directory,
                  "clients": [{"client_id": "codex", "verification": "installation_verified",
                               "package_revision": revision}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [record]}}, "atlassian", "codex",
                "plugin", "atlassian", {"atlassian": pin})
        self.assertEqual(len(qualify()), 1)
        for fake in ("evil/atlassian-fork", pin["distribution"], None):
            record["source"] = fake
            self.assertEqual(qualify(), [])
        record["source"] = "777genius/universal-agent-plugins/plugins/atlassian"
        original = record["directory"]
        record["directory"] = dict(original, recorded_revision="d" * 40)
        record["clients"][0]["package_revision"] = dict(
            revision, resolved_revision="d" * 40)
        self.assertEqual(qualify(), [])
        record["directory"] = original
        record["clients"][0]["package_revision"] = dict(
            revision, tree_digest="sha256:" + "d" * 64)
        self.assertEqual(qualify(), [])
        self.assertEqual(audit.matching_installations(
            {"data": {"installations": [record]}}, "atlassian", "codex",
            "plugin", "atlassian"), [])

    def test_frozen_inventory_uses_published_sha_and_refuses_null_revision(self):
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        audit.validate_fixtures(doc)
        self.assertEqual(len(doc["signed_release_pins"]), 21)
        self.assertTrue(all(
            isinstance(pin["revision"], str) and len(pin["revision"]) == 40
            for pin in doc["signed_release_pins"].values()))
        missing = json.loads(json.dumps(doc))
        del missing["signed_release_pins"]["atlassian"]
        with self.assertRaisesRegex(ValueError, "Missing frozen Registry"):
            audit.validate_fixtures(missing)

        # A published SHA binds a read-only native receipt, but a future
        # unbound review release never becomes acceptance evidence.
        pin = doc["signed_release_pins"]["cloudflare"]
        node = {"name": "cloudflare",
                "source": pin["repository"] + "/" + pin["path"],
                "installation_id": "verified-published",
                "directory": {"product_id": "cloudflare",
                              "recorded_distribution": pin["distribution"],
                              "recorded_revision": pin["revision"],
                              "recorded_release_sequence": pin["sequence"]},
                "clients": [{"client_id": "claude",
                             "verification": "installation_verified",
                             "package_revision": {
                                 "distribution_id": pin["distribution"],
                                 "resolved_revision": pin["revision"],
                                 "release_sequence": pin["sequence"],
                                 "tree_digest": pin["tree_digest"],
                                 "manifest_digest": pin["manifest_digest"]}}]}
        response = {"data": {"installations": [node]}}
        self.assertEqual(len(audit.matching_installations(
            response, "cloudflare", "claude", "plugin", "cloudflare",
            doc["signed_release_pins"])), 1)

        malformed = json.loads(json.dumps(doc))
        malformed["signed_release_pins"]["cloudflare"]["revision"] = None
        with self.assertRaisesRegex(ValueError, "Malformed frozen signed release pin"):
            audit.validate_fixtures(malformed)
        self.assertEqual(audit.matching_installations(
            response, "cloudflare", "claude", "plugin", "cloudflare",
            malformed["signed_release_pins"]), [])


    def test_namespaced_github_bridge_is_a_single_pinned_distribution(self):
        # BUG-038: default product "github" selects per-target eligible releases;
        # pin one real explicitly named distribution for reproducible 30x30 E2E.
        audit = load_audit()
        doc = json.loads(audit.FIXTURES.read_text())
        pin = doc["signed_release_pins"]["777genius/github-bridge"]
        self.assertEqual(doc["plugin_declared_names"]["777genius/github-bridge"], "github")
        self.assertNotIn("github", doc["plugin_aliases"])
        self.assertEqual(doc["plugin_aliases"].count("777genius/github-bridge"), 1)
        revision = {
            "distribution_id": pin["distribution"],
            "resolved_revision": pin["revision"],
            "release_sequence": pin["sequence"],
            "tree_digest": pin["tree_digest"],
            "manifest_digest": pin["manifest_digest"],
        }
        directory = {
            "product_id": "github",
            "recorded_distribution": pin["distribution"],
            "recorded_revision": pin["revision"],
            "recorded_release_sequence": pin["sequence"],
        }
        node = {
            "name": "github", "installation_id": "native-owned-bridge",
            "source": pin["repository"] + "/" + pin["path"],
            "directory": directory,
            "clients": [{"client_id": "opencode", "verification": "installation_verified",
                         "package_revision": revision}],
        }
        def selected():
            return audit.matching_installations(
                {"data": {"installations": [node]}}, "github", "opencode",
                "plugin", "777genius/github-bridge", doc["signed_release_pins"])
        self.assertEqual(len(selected()), 1)
        for key, value in (("product_id", "forged-unrelated-product"),
                            ("recorded_distribution", "github/github"),
                           ("recorded_revision", "0" * 40),
                           ("recorded_release_sequence", 1)):
            with self.subTest(key=key):
                node["directory"] = dict(directory, **{key: value})
                self.assertEqual(selected(), [])
        node["directory"] = directory
        node["clients"][0]["package_revision"] = dict(revision, tree_digest="sha256:" + "0" * 64)
        self.assertEqual(selected(), [])
        node["clients"][0]["package_revision"] = revision
        node["clients"][0]["client_id"] = "codex"
        self.assertEqual(selected(), [])

    def test_direct_git_revision_must_belong_to_verified_client(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style", "source": "obra/the-elements-of-style",
                "installation_id": "valid-id", "clients": [
                    {"client_id": "kiro", "verification": "installation_verified",
                     "package_revision": {}},
                    {"client_id": "kiro", "verification": "pending",
                     "package_revision": {"resolved_revision": sha}},
                ]}
        response = {"data": {"installations": [node]}}
        self.assertEqual(audit.matching_installations(
            response, "elements-of-style", "kiro", "plugin", item), [])

    def test_real_signed_release_target_refusal_is_not_an_installer_bug(self):
        audit = load_audit()
        reproduced = (
            'agentplugins: no eligible directory release for "marketing-kit": '
            '777genius/marketing-kit: release does not support complete target set; missing grok'
        )
        self.assertTrue(audit.source_target_incompatibility(reproduced))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: unexpected failure while writing native plugin configuration"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: lintai security scan failed: exit status 2"))



    def test_pi_manifest_discovery_override_is_source_unsupported(self):
        # BUG-062: exact Pi source-layout safety refusal, not arbitrary errors.
        audit = load_audit()
        self.assertTrue(audit.source_target_incompatibility(
            "agentplugins: pi package manifest overrides selected extension discovery", "pi"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: pi package manifest overrides selected extension discovery", "codex"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: pi package manifest is unreadable"))
        self.assertFalse(audit.source_target_incompatibility(
            "agentplugins: unexpected Pi native registry configuration failure"))

    def test_native_snapshot_keeps_symlinks_and_unreadable_paths(self):
        # BUG-017: untrusted native entries must not disappear from evidence.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            root = home / ".codex"
            root.mkdir()
            owned = root / "managed.yaml"
            owned.write_text("name: managed\n")
            outside = home / "unrelated.txt"
            outside.write_text("foreign\n")
            (root / "unknown-link").symlink_to(outside)
            (root / "linked-dir").symlink_to(home, target_is_directory=True)
            initial = audit.snapshot([root])
            self.assertEqual(initial[".codex/unknown-link"], "symlink")
            self.assertEqual(initial[".codex/linked-dir"], "symlink")
            self.assertIn(".codex/managed.yaml", initial)
            self.assertNotIn(".codex/linked-dir/unrelated.txt", initial)
            with patch.object(Path, "open", side_effect=OSError("unreadable")):
                invalid = audit.snapshot([root])
            self.assertEqual(invalid[".codex/managed.yaml"], "unreadable")
            self.assertIn("unknown-link: symlink", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/unknown-link"], "codex")))
            self.assertIn("managed.yaml", " ".join(
                audit.native_identity_residuals(
                    [root], "managed", [".codex/managed.yaml"], "codex")))
            (root / "unknown-link").unlink()
            self.assertFalse(audit.native_identity_residuals(
                [root], "managed", [".codex/unknown-link"], "codex"))

    def test_unreadable_walk_marker_cannot_disappear_from_cleanup_evidence(self):
        # BUG-017: os.walk onerror creates a synthetic path; lstat cannot find
        # that marker, but a failed enumeration must not be counted as clean.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".codex"
            root.mkdir()
            reported = audit.native_identity_residuals(
                [root], "atlassian",
                [".codex/restricted-dir/.walk-error"], "codex")
            self.assertEqual(reported, [
                ".codex/restricted-dir/.walk-error: unreadable-walk"])

    def test_foreign_sentinel_symlink_cannot_fake_preservation(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary)
            foreign = home / "foreign.txt"
            matching = home / "matching.txt"
            foreign.write_text("keep\n")
            matching.write_text("keep\n")
            self.assertTrue(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            foreign.symlink_to(matching)
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))
            foreign.unlink()
            self.assertFalse(audit.preserved_regular_text(foreign, "keep\n", home))

    def test_foreign_sentinel_invalid_utf8_is_tampered_not_harness_error(self):
        # BUG-026: corrupt bytes must not abort the remaining lifecycle proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            foreign = case / "foreign.txt"
            foreign.write_bytes(b"\xff\xfe")
            self.assertFalse(audit.preserved_regular_text(foreign, "untouched\n", case))

    def test_copilot_settings_depth_limit_is_reported_as_truncated(self):
        # BUG-026: a sole depth-limited leaf must not be called fully parsed.
        import json
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve()
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"plugins": {"atlassian": "enabled"}}),
                                encoding="utf-8")
            shallow = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(shallow["status"], "parsed")
            self.assertEqual(shallow["match_counts"]["exact_key"], 1)
            nested = {"atlassian": "enabled"}
            for _ in range(32):
                nested = {"nested": nested}
            settings.write_text(json.dumps({"plugins": nested}), encoding="utf-8")
            deep = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(deep["status"], "truncated")
            self.assertEqual(deep["match_counts"]["exact_key"], 0)

    def test_project_skill_intermediate_symlink_cannot_fake_cleanup(self):
        # BUG-023: project/.claude can be swapped even when SKILL.md is regular.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            project = case / "project"
            foreign = root / "foreign"
            project.mkdir(parents=True)
            foreign_skill_root = foreign / "skills"
            (foreign_skill_root / "foreign-neighbor").mkdir(parents=True)
            (foreign_skill_root / "demo-skill").mkdir()
            marker = foreign_skill_root / "foreign-neighbor/SKILL.md"
            marker.write_text("unmanaged identical sentinel\n")
            (foreign_skill_root / "demo-skill/SKILL.md").write_text(
                "---\nname: demo-skill\n---\n")
            (project / ".claude").symlink_to(foreign, target_is_directory=True)
            spoofed = project / ".claude/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(spoofed.is_file())
            self.assertFalse(audit.preserved_regular_text(
                spoofed, "unmanaged identical sentinel\n", case))
            self.assertFalse(audit.path_inside_case(spoofed, case))
            self.assertIsNone(audit.skill_file_present(project, "claude", "demo-skill"))
            leftovers = audit.skill_removal_leftovers(
                project, "claude", "demo-skill", ".claude/skills/demo-skill/SKILL.md")
            self.assertIn(".claude/skills: unsafe ancestor", leftovers)
            self.assertTrue(any("unsafe ancestor" in entry for entry in leftovers))
            # No helper is permitted to mutate the foreign user's Skill.
            self.assertEqual(marker.read_text(), "unmanaged identical sentinel\n")

    def test_skill_lock_requires_two_complete_sha256_digests(self):
        import json
        audit = load_audit()
        item = {"name": "standalone-skill", "folder": "standalone-skill",
                "repo": "upstream/skills", "ref": "a" * 40}
        package = {"alias": item["name"], "source": {"git": {
            "kind": "skill", "skill": item["name"],
            "repository": item["repo"], "commit": item["ref"],
            "resolved_skill_path": "skills/" + item["folder"]}},
            "targets": ["codex"],
            "tree_digest": "sha256:" + "b" * 64,
            "manifest_digest": "sha256:" + "c" * 64}
        with TemporaryDirectory() as temporary:
            project = Path(temporary)
            def qualified():
                (project / "uap.lock").write_text(
                    json.dumps({"packages": [package]}))
                return audit.locked_skill_is_pinned(project, item, "codex")
            self.assertTrue(qualified())
            for malformed in ("sha256:", "sha256:bad", "sha256:" + "g" * 64,
                              "sha256:" + "A" * 64, "sha256:" + "0" * 65):
                for field in ("tree_digest", "manifest_digest"):
                    with self.subTest(digest=malformed, field=field):
                        original = package[field]
                        package[field] = malformed
                        self.assertFalse(qualified())
                        package[field] = original

    def test_direct_git_repository_identity_must_be_exact(self):
        audit = load_audit()
        sha = "a" * 40
        item = "github:obra/the-elements-of-style@" + sha
        node = {"name": "elements-of-style",
                "source": "obra/the-elements-of-style",
                "installation_id": "verified",
                "clients": [{"client_id": "cline",
                             "verification": "installation_verified",
                             "package_revision": {"resolved_revision": sha}}]}
        def qualify():
            return audit.matching_installations(
                {"data": {"installations": [node]}},
                "elements-of-style", "cline", "plugin", item)
        self.assertEqual(len(qualify()), 1)
        for valid in ("github:obra/the-elements-of-style",
                      "https://github.com/obra/the-elements-of-style"):
            node["source"] = valid
            self.assertEqual(len(qualify()), 1)
        for spoof in ("mirror/obra/the-elements-of-style-fork",
                      "obra/the-elements-of-style-other",
                      "evil/obra/the-elements-of-style",
                      "obra/the-elements-of-style?unverified=true"):
            with self.subTest(source=spoof):
                node["source"] = spoof
                self.assertEqual(qualify(), [])

    def test_proxy_url_userinfo_is_not_inherited_by_child(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary) / "case"
            home, data = case / "home", case / "uap-data"
            home.mkdir(parents=True)
            data.mkdir()
            with patch.dict(os.environ, {
                "HTTPS_PROXY": "https://alice:secret@proxy.invalid:8443",
                "http_proxy": "http://user:pw@proxy.invalid:3128",
                "ALL_PROXY": "socks5://proxy.invalid:1080",
                "NO_PROXY": "localhost",
                "OPENAI_API_KEY": "never-pass",
            }, clear=True):
                env = audit.isolated_client_env(home, case, data)
            self.assertEqual(env["HTTPS_PROXY"], "https://proxy.invalid:8443")
            self.assertEqual(env["http_proxy"], "http://proxy.invalid:3128")
            self.assertEqual(env["ALL_PROXY"], "socks5://proxy.invalid:1080")
            self.assertEqual(env["NO_PROXY"], "localhost")
            self.assertNotIn("OPENAI_API_KEY", env)


    def test_copilot_native_registry_requires_exact_managed_identity(self):
        # BUG-045: local CLI JSON evidence, not a settings-file string match.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary) / "home"
            profile = home / ".copilot"
            profile.mkdir(parents=True)
            native = str(Path(temporary) / "trusted-copilot")
            env = {"HOME": str(home)}
            valid = {"name": "atlassian", "marketplace": "agentplugins-aaaaaaaaaaaa",
                     "version": "0.1.0", "enabled": True, "source": "local"}
            def response(items, exit_code=0):
                return {"exit_code": exit_code, "_json": items}
            with patch.dict(audit.AGENT_BINARIES, {"copilot": native}, clear=True):
                with patch.object(audit, "call", return_value=response([valid])) as executed:
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian", "0.1.0"), {"status": "installed"})
                    self.assertEqual(executed.call_args.args[0],
                                     [native, "plugin", "list", "--json"])
                    self.assertEqual(executed.call_args.args[1]["COPILOT_HOME"], str(profile))
                with patch.object(audit, "call", return_value=response([])):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                wrong = [
                    dict(valid, marketplace="foreign"),
                    dict(valid, version="2.0.0"),
                    dict(valid, enabled=False),
                    dict(valid, name="atlassian@agentplugins-aaaaaaaaaaaa"),
                ]
                for row in wrong:
                    with self.subTest(row=row), patch.object(audit, "call", return_value=response([row])):
                        self.assertNotEqual(audit.copilot_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "installed")
                for listing in ([valid, valid], {"plugins": [valid]}, ["malformed"]):
                    with self.subTest(listing=listing), patch.object(audit, "call", return_value=response(listing)):
                        self.assertNotEqual(audit.copilot_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "installed")
                with patch.object(audit, "call", return_value=response([
                        dict(valid, name="renamed-plugin")])):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"],
                        "unqualified-reminted-managed")
                with patch.object(audit, "call", return_value=response([], exit_code=1)):
                    self.assertNotEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                with patch.dict(audit.AGENT_BINARIES, {}, clear=True):
                    self.assertEqual(audit.copilot_native_plugin_state(
                        home, env, home, "atlassian")["status"], "unqualified-prerequisites")

    def test_copilot_disabled_setting_needs_native_absence_and_exact_owned_shape(self):
        audit = load_audit()
        before = {"status": "absent"}
        after_add = {"status": "parsed",
                     "enabled_plugin_value_counts": {"true": 1, "false": 0, "non_boolean": 0}}
        after_remove = {
            "status": "parsed", "allowed_sections": ["enabledPlugins"],
            "enabled_plugin_value_counts": {"true": 0, "false": 1, "non_boolean": 0},
            "match_counts": {"exact_key": 0, "qualified_key": 1, "reference_key": 0,
                             "exact_value": 0, "qualified_value": 0, "reference_value": 0},
        }
        installed, absent = {"status": "installed"}, {"status": "absent"}
        residuals = [".copilot/settings.json"]
        def qualified(*args):
            return audit.copilot_disabled_preference_only(*args)
        self.assertTrue(qualified(before, after_add, after_remove,
                                  installed, absent, residuals))
        for native_after, left in ((installed, residuals),
                                   (absent, [".copilot/settings.json", ".copilot/other.json"]),
                                   ({"status": "unqualified-list"}, residuals)):
            self.assertFalse(qualified(before, after_add, after_remove,
                                       installed, native_after, left))
        for key, value in (("true", 1), ("false", 0), ("non_boolean", 1)):
            bad = json.loads(json.dumps(after_remove))
            bad["enabled_plugin_value_counts"][key] = value
            self.assertFalse(qualified(before, after_add, bad,
                                       installed, absent, residuals))
        for key, value in (("qualified_key", 2), ("reference_value", 1)):
            bad = json.loads(json.dumps(after_remove))
            bad["match_counts"][key] = value
            self.assertFalse(qualified(before, after_add, bad,
                                       installed, absent, residuals))
        self.assertFalse(qualified({"status": "parsed"}, after_add, after_remove,
                                   installed, absent, residuals))


    def test_qoder_native_cli_proves_owned_registration_and_removal(self):
        # BUG-039: exact native local ID+version only, never cache evidence.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            home = Path(temporary) / "home"
            profile = home / ".qoder"
            profile.mkdir(parents=True)
            # macOS /var may resolve to /private/var; the production probe
            # requires the same canonical directory as isolated_client_env.
            env = {"QODER_CONFIG_DIR": str(profile.resolve(strict=True))}
            native = str(Path(temporary) / "trusted-qodercli")
            def probe(rows, code=0):
                return {"exit_code": code, "_json": rows}
            with patch.dict(audit.AGENT_BINARIES, {"qodercli": native}, clear=True):
                with patch.object(audit, "call", return_value=probe([
                    {"id": "atlassian@local", "version": "0.1.0"}])) as called:
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian", "0.1.0"), {"status": "registered"})
                    self.assertEqual(called.call_args.args[0],
                                     [native, "--config-dir", str(profile), "plugins", "list", "--json"])
                    self.assertEqual(called.call_args.args[1], env)
                with patch.object(audit, "call", return_value=probe([])):
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                for rows in [
                    [{"id": "atlassian@local"}],
                    [{"id": "atlassian@local", "version": "0.2.0"}],
                    [{"id": "atlassian@foreign", "version": "0.1.0"}],
                    [{"id": "atlassian@local", "version": "0.1.0"}] * 2,
                    [{"id": "atlassian@local", "version": 1}],
                    [{"id": "atlassian@local@extra", "version": "0.1.0"}],
                    [{"id": "other@local", "version": "1.0.0"}, "malformed"],
                ]:
                    with self.subTest(rows=rows), patch.object(audit, "call", return_value=probe(rows)):
                        self.assertNotEqual(audit.qoder_native_plugin_state(
                            home, env, home, "atlassian", "0.1.0")["status"], "registered")
                with patch.object(audit, "call", return_value=probe([], code=1)):
                    self.assertNotEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "absent")
                with patch.dict(audit.AGENT_BINARIES, {}, clear=True):
                    self.assertEqual(audit.qoder_native_plugin_state(
                        home, env, home, "atlassian")["status"], "unqualified-prerequisites")
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, {"QODER_CONFIG_DIR": str(home / "foreign")},
                    home, "atlassian")["status"], "unqualified-prerequisites")
                foreign = Path(temporary) / "foreign"
                foreign.mkdir()
                profile.rmdir()
                try:
                    profile.symlink_to(foreign, target_is_directory=True)
                except (OSError, NotImplementedError):
                    return
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, env, home, "atlassian")["status"], "unqualified-prerequisites")


    def test_native_residual_uses_exact_tokens_not_url_substrings(self):
        # BUG-042: a native URL hostname is not the managed GitHub plugin.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".copilot"
            root.mkdir()
            settings = root / "settings.json"
            observed = [".copilot/settings.json"]
            settings.write_text('{"homepage": "https://github.com/other/repo"}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), [])
            # A foreign marketplace plugin is also not UAP's managed namespace.
            settings.write_text('{"enabledPlugins":{"github@foreign":true}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), [])
            settings.write_text(
                '{"enabledPlugins":{"github@agentplugins-aaaaaaaaaaaa":false}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), observed)
            settings.write_text('{"plugins":{"github":{"enabled":true}}}')
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"), observed)
            settings.write_text("unrelated" * 150000)
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"),
                [observed[0] + ": unscanned-too-large"])
            settings.unlink()
            try:
                settings.symlink_to(root / "missing")
            except (OSError, NotImplementedError):
                return
            self.assertEqual(audit.native_identity_residuals(
                [root], "github", observed, "copilot"),
                [observed[0] + ": symlink"])

    def test_invalid_utf8_skill_name_still_visible_to_readonly_oracle(self):
        # BUG-043: malformed source bytes are not rewritten or omitted.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            project = Path(temporary).resolve() / "project"
            target = project / ".claude/skills/raw"
            target.mkdir(parents=True)
            skill = target / "SKILL.md"
            source = b"\xff\xfe---\nname: raw\n---\n"
            skill.write_bytes(source)
            self.assertEqual(audit.skill_file_present(project, "claude", "raw"),
                             ".claude/skills/raw/SKILL.md")
            self.assertIsNone(audit.skill_file_present(project, "claude", "foreign"))
            self.assertEqual(skill.read_bytes(), source)

    def test_qoder_passive_cache_does_not_hide_active_residue_or_symlinks(self):
        # BUG-018: observed Qoder cache and run manifests outlive native remove.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/atlassian/0.1.0/plugin.json"
            log = root / "logs/runs/run-1/manifest.json"
            active = root / "plugins/installed_plugins_v2.json"
            settings = root / "settings.json"
            for path in (cache, log, active, settings):
                path.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"atlassian"}')
            log.write_text('{"plugin":"atlassian"}')
            active.write_text('{"plugin":"atlassian"}')
            settings.write_text('{"mcpServers":{"atlassian":{}}}')
            observed = [
                ".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json",
                ".qoder/logs/runs/run-1/manifest.json",
                ".qoder/plugins/installed_plugins_v2.json",
                ".qoder/settings.json",
            ]
            self.assertEqual(
                audit.native_identity_residuals([root], "atlassian", observed, "qoder"),
                [".qoder/plugins/installed_plugins_v2.json", ".qoder/settings.json"])
            active.write_text("{}")
            settings.write_text("{}")
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            cache.unlink()
            cache.symlink_to(settings)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/plugins/cache/local/atlassian/0.1.0/plugin.json: symlink"])
            self.assertFalse(audit.passive_native_path("cline", "plugins/cache/local/demo"))
            self.assertFalse(audit.passive_native_path("qoder", "settings.json"))


    def test_post_remove_native_rescan_detects_late_registration(self):
        # BUG-020: a native setting written by remove after the add snapshot
        # must not be mistaken for a successful full lifecycle.
        audit = load_audit()
        identity = "atlassian"
        source = "github:example/native-plugin@" + ("a" * 40)
        matched = [{"name": identity, "installation_id": "owned-once",
                    "clients": [{"client_id": "grok", "verification": "installation_verified"}]}]

        def verify_case(late_registration):
            with TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                native = root / "plugin-00/home/.grok"
                managed = native / "managed-plugin.json"
                late = native / "new-settings.json"
                state = {"installed": False, "add_count": 0}

                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[1]
                    if operation == "add":
                        state["add_count"] += 1
                        if state["add_count"] == 1:
                            managed.write_text('{"name":"atlassian"}')
                        state["installed"] = True
                    elif operation == "remove":
                        managed.unlink()
                        if late_registration:
                            late.write_text('{"nativePlugin":"atlassian"}')
                        state["installed"] = False
                    installations = matched if state["installed"] else []
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations": installations}}}

                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=matched):
                    evidence = audit.do_case("/unused/agentplugins", "grok", "plugin",
                                             source, 0, root, {source: identity})
                self.assertTrue(evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                self.assertTrue(evidence["sentinels_preserved"])
                self.assertTrue(evidence["post_list_schema_valid"])
                self.assertEqual(evidence["cleanup"]["exit_code"], 0)
                self.assertIn(".grok/managed-plugin.json", evidence["native_identity_files"])
                return evidence

        clean = verify_case(False)
        self.assertEqual(clean["status"], "passed")
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["native_residuals"], [])

        delayed = verify_case(True)
        self.assertEqual(delayed["status"], "cleanup-failed")
        self.assertFalse(delayed["removal_verified"])
        self.assertEqual(delayed["native_residuals"], [".grok/new-settings.json"])



    def test_post_remove_reminted_identity_under_new_id_fails_cleanup(self):
        # BUG-022: disjoint old IDs do not prove this package/target is gone.
        audit = load_audit()
        source = "github:example/native-plugin@" + ("a" * 40)
        old = {"name": "atlassian", "installation_id": "owned-old",
               "clients": [{"client_id": "grok",
                            "verification": "installation_verified"}]}
        reminted = {"name": "atlassian", "installation_id": "owned-new",
                    "clients": [{"client_id": "grok",
                                 "verification": "installation_verified"}]}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            native = runroot / "plugin-00/home/.grok/managed-plugin.json"
            state = {"removed": False}

            def fake_call(argv, env, cwd, timeout=105):
                command = argv[1]
                if command == "add" and not native.exists():
                    native.write_text('{"name":"atlassian"}')
                elif command == "remove":
                    native.unlink()
                    state["removed"] = True
                entries = [reminted] if state["removed"] else [old]
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False,
                        "_json": {"data": {"installations": entries}}}

            with patch.object(audit, "call", side_effect=fake_call):
                with patch.object(audit, "matching_installations", return_value=[old]):
                    result = audit.do_case(
                        "/unused/agentplugins", "grok", "plugin", source, 0,
                        runroot, {source: "atlassian"})
        self.assertTrue(result["listed_identity"])
        self.assertTrue(result["repeat_verified"])
        self.assertTrue(result["sentinels_preserved"])
        self.assertEqual(result["cleanup"]["exit_code"], 0)
        self.assertTrue(result["post_list_schema_valid"])
        self.assertEqual(result["native_residuals"], [])
        self.assertEqual(result["matched_installation_ids"], ["owned-old"])
        self.assertTrue(result["post_remove_identity_registered"])
        self.assertFalse(result["removal_verified"])
        self.assertEqual(result["status"], "cleanup-failed")


    def test_codex_native_registration_requires_exact_local_marketplace(self):
        # The actual Codex CLI owns [plugins."name@agentplugins-<hash>"].
        # A source cache, arbitrary URL or unrelated marketplace is not proof.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary)
            root = case / "home/.codex"
            root.mkdir(parents=True)
            managed = case / "uap-data/managed"
            managed.mkdir(parents=True)
            config = root / "config.toml"
            key = "agentplugins-" + "a" * 12
            config.write_text(
                f'[plugins."atlassian@{key}"]\nenabled = true\n'
                f'[marketplaces.{key}]\nsource_type = "local"\n'
                f'source = "{managed}"\n')
            changes = list(audit.snapshot([root]).items())
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "atlassian", "codex"),
                [".codex/config.toml"])
            self.assertEqual(
                audit.attributable_native_changes([root], changes, "github", "codex"),
                [])
            self.assertFalse(audit.codex_owned_native_config(
                config, "atlassian-wrong", case))
            escaped = Path(temporary).parent / "other-account"
            config.write_text(config.read_text().replace(str(managed), str(escaped)))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))
            config.write_text(
                config.read_text().replace(str(escaped), str(managed))
                .replace(f"agentplugins-{'a' * 12}", "random-market"))
            self.assertFalse(audit.codex_owned_native_config(config, "atlassian", case))

    def test_large_native_file_rewrite_changes_snapshot(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".grok"
            root.mkdir()
            native = root / "managed.json"
            native.write_bytes(b"A" * (5 * 1024 * 1024))
            before = audit.snapshot([root])
            native.write_bytes(b"B" * (5 * 1024 * 1024))
            after = audit.snapshot([root])
            self.assertNotEqual(before, after)
            self.assertEqual(len(before[".grok/managed.json"]), 64)
            self.assertNotIn("too-large", after.values())

    def test_native_attribution_excludes_cache_and_unrelated_url(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            cache = root / "plugins/cache/local/github/app.json"
            url = root / "settings.json"
            native = root / "plugins/installed_plugins_v2.json"
            for f in (cache, url, native):
                f.parent.mkdir(parents=True, exist_ok=True)
            cache.write_text('{"name":"github"}')
            url.write_text('{"repository":"https://github.com/other/repo"}')
            native.write_text('{"plugins":{"github":{"enabled":true}}}')
            files = audit.snapshot([root])
            changes = list(files.items())
            credited = audit.attributable_native_changes([root], changes, "github", "qoder")
            self.assertEqual(credited, [".qoder/plugins/installed_plugins_v2.json"])
            self.assertFalse(audit.exact_native_identity(
                'source = "https://github.com/example/package"', "github"))
            self.assertTrue(audit.exact_native_identity(
                'name = "github"', "github"))

    def test_native_and_temp_symlink_escape_rejected_before_untrusted_writes(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            home = case / "home"
            data = case / "uap-data"
            foreign = root / "foreign"
            foreign.mkdir()
            home.mkdir(parents=True)
            data.mkdir()
            (home / ".codex").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "native profile"):
                audit.isolated_client_env(home, case, data)
            (home / ".codex").unlink()
            (case / "tmp").rmdir()
            (case / "tmp").symlink_to(foreign, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "temp root"):
                audit.isolated_client_env(home, case, data)

            future = root / "run" / "plugin-00"
            future.parent.mkdir()
            future.symlink_to(foreign, target_is_directory=True)
            # The explicit preflight must reject redirected case roots before
            # mkdir or any third-party command can traverse the link.
            with self.assertRaisesRegex(ValueError, "audit case root is redirected"):
                audit.do_case("/unused/audit-cli", "codex", "plugin",
                              "atlassian", 0, future.parent, {})
            self.assertEqual(list(foreign.iterdir()), [])

    def test_skill_removal_rejects_surviving_links_and_native_registration(self):
        audit = load_audit()
        identity = "demo-skill"
        item = {"name": identity, "folder": identity, "repo": "example/skills",
                "ref": "a" * 40, "source": "https://github.com/example/skills",
                "skill_md_git_blob_sha1": "b" * 40}
        match = [{"name": identity, "installation_id": "skill-1",
                  "clients": [{"client_id": "claude",
                               "verification": "installation_verified"}]}]

        def check(residue):
            with TemporaryDirectory() as temporary:
                runroot = Path(temporary)
                project = runroot / "skill-00/project"
                native = runroot / "skill-00/home/.claude"
                skill = project / ".claude/skills/demo-skill/SKILL.md"
                state = {"installed": False}
                def fake_call(argv, env, cwd, timeout=105):
                    if argv[1] == "skills":
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text("---\nname: demo-skill\n---\n")
                        (project / "uap.toml").write_text("demo-skill\n")
                        (project / "uap.lock").write_text('{"name":"demo-skill"}')
                        state["installed"] = True
                    elif argv[1] == "remove":
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / "uap.lock").unlink()
                        state["installed"] = False
                        if residue == "symlink":
                            skill.parent.mkdir()
                            skill.symlink_to(project / "unmanaged")
                        elif residue == "native":
                            (native / "agent-registry.json").write_text(
                                '{"skills":{"demo-skill":{}}}')
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations":
                                               match if state["installed"] else []}}}
                with patch.object(audit, "call", side_effect=fake_call), \
                     patch.object(audit, "matching_installations", return_value=match), \
                     patch.object(audit, "locked_skill_is_pinned", return_value=True), \
                     patch.object(audit, "matches_pinned_skill_bytes", return_value=True):
                    evidence = audit.do_case("/unused/agentplugins", "claude", "skill",
                                             item, 0, runroot, {})
                self.assertTrue(evidence["installed"] if "installed" in evidence
                                else evidence["listed_identity"])
                self.assertTrue(evidence["repeat_verified"])
                return evidence
        clean = check(None)
        self.assertTrue(clean["removal_verified"])
        self.assertEqual(clean["status"], "passed")
        linked = check("symlink")
        self.assertFalse(linked["removal_verified"])
        self.assertEqual(linked["status"], "cleanup-failed")
        self.assertTrue(linked["post_removal_leftovers"])
        native = check("native")
        self.assertFalse(native["removal_verified"])
        self.assertEqual(native["status"], "cleanup-failed")
        self.assertTrue(native["native_residuals"])


    def test_copilot_settings_probe_redacts_values_and_keeps_refusals(self):
        # BUG-019: distinguish a qualified native key from incidental text,
        # never include native settings contents in retained E2E artifacts.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "plugin-00"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            self.assertEqual(
                audit.copilot_settings_sanitized_shape(home, case, "atlassian"),
                {"status": "absent"})
            settings.write_text(
                '{"plugins":{"atlassian@agentplugins-aaaaaaaaaaaa":true},'
                '"unmanaged":{"secret":"confidential-password-123",'
                '"url":"https://example.invalid/atlassian"}}')
            proof = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(proof["status"], "parsed")
            self.assertEqual(proof["allowed_sections"], ["plugins"])
            self.assertEqual(proof["match_counts"]["qualified_key"], 1)
            self.assertEqual(proof["match_counts"]["reference_value"], 1)
            self.assertEqual(proof["match_counts"]["exact_key"], 0)
            self.assertNotIn("confidential-password-123", str(proof))
            self.assertNotIn("example.invalid", str(proof))
            self.assertNotIn("unmanaged", str(proof))
            settings.write_text('{"plugins":{}}')
            clean = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(clean["status"], "parsed")
            self.assertFalse(any(clean["match_counts"].values()))

    def test_copilot_enabled_plugin_values_are_exact_and_never_exported(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home = case / "home"
            settings = home / ".copilot/settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"enabledPlugins": {
                "atlassian@agentplugins-aaaaaaaaaaaa": True,
                "atlassian@agentplugins-bbbbbbbbbbbb": False,
                "atlassian@agentplugins-cccccccccccc": "private-auth-token",
                "unrelated@agentplugins-dddddddddddd": True,
                "atlassian@agentplugins-not-a-hex": "private-user-value",
            }, "unmanaged": {"API_KEY": "private-secret-value"}}))
            proof = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(proof["status"], "parsed")
            self.assertEqual(proof["enabled_plugin_value_counts"],
                             {"true": 1, "false": 1, "non_boolean": 1})
            self.assertEqual(proof["match_counts"]["qualified_key"], 3)
            self.assertFalse(any(secret in str(proof) for secret in (
                "private-auth-token", "private-secret-value",
                "private-user-value", "unrelated@agentplugins")))
            settings.write_text('{"enabledPlugins":{"other@agentplugins-aaaaaaaaaaaa":true}}')
            clean = audit.copilot_settings_sanitized_shape(home, case, "atlassian")
            self.assertEqual(clean["enabled_plugin_value_counts"],
                             {"true": 0, "false": 0, "non_boolean": 0})

    def test_copilot_settings_probe_rejects_unsafe_paths_and_bad_json(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case, outside = root / "case", root / "outside"
            settings = case / "home/.copilot/settings.json"
            settings.parent.mkdir(parents=True)
            outside.write_text('{"plugins":{"atlassian":true}}')
            settings.symlink_to(outside)
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"], "unsafe-path")
            settings.unlink()
            settings.write_text('{"invalid-json"')
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "unreadable-or-invalid-json")
            settings.write_bytes(b"X" * (1024 * 1024 + 1))
            self.assertEqual(audit.copilot_settings_sanitized_shape(
                case / "home", case, "atlassian")["status"],
                "not-regular-or-too-large")



    def test_qoder_foreign_skill_is_never_placed_in_uap_managed_bundle(self):
        audit = load_audit()
        item = {"name": "frontend-design", "source": "unused-fixture",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            runroot = Path(temporary).resolve()
            with patch.object(audit, "call", return_value=refused):
                row = audit.do_case("/unused/agentplugins", "qoder", "skill",
                                    item, 0, runroot, {})
            project = runroot / "skill-00/project"
            foreign = project / ".qoder/skills/foreign-neighbor/SKILL.md"
            managed_foreign = project / ".uap/managed/clients/qoder/foreign-neighbor"
            self.assertTrue(foreign.is_file())
            self.assertEqual(foreign.read_text(encoding="utf-8"),
                             "---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
            self.assertFalse(managed_foreign.exists())
            self.assertTrue(row["sentinels_preserved"])

    def test_grok_foreign_skill_is_not_an_invalid_native_plugin(self):
        # BUG-060: Grok rejects a plugins/ child lacking plugin.json before
        # reviewing any selected source. The unrelated Skill belongs in skills/.
        audit = load_audit()
        item = {"name": "frontend-design", "repo": "example/skills",
                "folder": "frontend-design", "source": "unused",
                "skill_md_git_blob_sha1": "0" * 40}
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "refused",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        # The Grok project prerequisite uses a real isolated git init.
        # Mock source installs, not the pinned local Git executable itself.
        real_call = audit.call
        def source_refusal(argv, env, cwd, timeout=105):
            if argv[0] == audit.GIT_EXECUTABLE:
                return real_call(argv, env, cwd, timeout)
            return dict(refused)
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            with patch.object(audit, "call", side_effect=source_refusal):
                evidence = audit.do_case("/unused/agentplugins", "grok", "skill",
                                         item, 0, root, {})
            project = root / "skill-00/project"
            self.assertTrue((project / ".git").is_dir())
            foreign = project / ".grok/skills/foreign-neighbor/SKILL.md"
            self.assertTrue(foreign.is_file())
            self.assertEqual(foreign.read_text(encoding="utf-8"),
                             "---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
            self.assertFalse((project / ".grok/plugins/foreign-neighbor").exists())
            self.assertTrue(evidence["sentinels_preserved"])
            self.assertEqual(evidence["status"], "install-failed")

    def test_vscode_user_plugins_use_only_documented_no_save_route(self):
        # BUG-061: no new VS Code product capability or saved dependency.
        audit = load_audit()
        source = "github:example/native-plugin@" + ("a" * 40)
        refused = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "refused",
                   "timeout": False, "_json": {"data": {"installations": []}}}
        with TemporaryDirectory() as temporary:
            for target in ("vscode", "codex"):
                with self.subTest(target=target):
                    root = Path(temporary).resolve() / target
                    with patch.object(audit, "call",
                                      side_effect=lambda *a, **kw: dict(refused)) as call:
                        evidence = audit.do_case("/unused/agentplugins", target,
                                                 "plugin", source, 0, root,
                                                 {source: "native-plugin"})
                    first = call.call_args_list[0].args[0]
                    self.assertEqual(first[1:3], ["add", source])
                    self.assertEqual(first.count("--no-save"), int(target == "vscode"))
                    self.assertEqual(first[first.index("--scope") + 1], "user")
                    self.assertTrue(evidence["sentinels_preserved"])
                    self.assertEqual(evidence["status"], "install-failed")

    def test_vscode_manual_external_removal_stays_nonpass(self):
        # BUG-064: prepared, non-activated VS Code is NOT proven removed.
        audit = load_audit()
        selected = "github:example/native-demo@" + ("a" * 40)
        matched = [{"name": "native-demo", "installation_id": "same-install",
                    "clients": [{"client_id": "vscode", "scope": "user",
                                 "materialization": "materialized",
                                 "verification": "package_validated",
                                 "activation": "manual_activation_required"}]}]
        def response(exit_code=0, stderr="", installations=None):
            return {"exit_code": exit_code, "stdout_tail": "", "stderr_tail": stderr,
                    "timeout": False,
                    "_json": {"data": {"installations": matched if installations is None
                                       else installations}}}
        manual = response(1, "agentplugins: removal requires a manual client step for [vscode]; "
                          "verify native absence, then rerun --external-uninstalled")
        with TemporaryDirectory() as temporary:
            with (patch.object(audit, "call", side_effect=[
                    response(), response(), response(), response(), manual]),
                  patch.object(audit, "matching_installations", return_value=matched)):
                proof = audit.do_case(
                    "/unused/agentplugins", "vscode", "plugin", selected, 0,
                    Path(temporary).resolve(), {selected: "native-demo"})
        self.assertEqual(proof["status"], "external-native-uninstall-required")
        self.assertTrue(proof["prepared_binding"])
        self.assertTrue(proof["repeat_verified"])
        self.assertFalse(proof["removal_verified"])
        self.assertEqual(proof["cleanup"]["exit_code"], 1)
        self.assertTrue(proof["sentinels_preserved"])

    def test_kimi_skill_uses_real_isolated_git_worktree(self):
        audit = load_audit()
        self.assertTrue(audit.GIT_EXECUTABLE and Path(audit.GIT_EXECUTABLE).is_file())
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, project, data = case / "home", case / "project", case / "uap-data"
            for directory in (home, project, data):
                directory.mkdir(parents=True)
            env = audit.isolated_client_env(home, case, data)
            audit.initialize_project_skill_git_root(project, env, case)
            self.assertTrue((project / ".git").is_dir())
            self.assertFalse((project / ".git").is_symlink())
            # Real Git must recognize this disposable root; an empty marker is not enough.
            checked = audit.call([audit.GIT_EXECUTABLE, "-C", str(project),
                                  "rev-parse", "--show-toplevel"], env, project, 30)
            self.assertEqual(checked["exit_code"], 0)
            self.assertEqual(Path(checked["stdout_tail"].strip()).resolve(), project)
            with self.assertRaisesRegex(RuntimeError, "prerequisite unavailable"):
                audit.initialize_project_skill_git_root(project, env, case / "outside")

    def test_project_git_init_rejects_existing_symlink_metadata(self):
        # BUG-065: git init follows .git symlinks and can write outside the case
        # before a post-execution containment check detects the redirect.
        audit = load_audit()
        with TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            case = root / "case"
            project, home, data = case / "project", case / "home", case / "uap-data"
            for directory in (project, home, data):
                directory.mkdir(parents=True, exist_ok=True)
            outside = root / "outside"
            outside.mkdir()
            marker = project / ".git"
            try:
                marker.symlink_to(outside, target_is_directory=True)
            except OSError as err:
                self.skipTest(f"symlinks unavailable: {err}")
            env = audit.isolated_client_env(home, case, data)
            with patch.object(audit, "call") as forbidden:
                with self.assertRaisesRegex(RuntimeError, "metadata must be initially absent"):
                    audit.initialize_project_skill_git_root(project, env, case)
                forbidden.assert_not_called()
            self.assertEqual(list(outside.iterdir()), [])
            marker.unlink()
            marker.mkdir()
            with patch.object(audit, "call") as forbidden:
                with self.assertRaisesRegex(RuntimeError, "metadata must be initially absent"):
                    audit.initialize_project_skill_git_root(project, env, case)
                forbidden.assert_not_called()

    def test_isolated_git_prerequisite_only_for_kimi_and_grok_skills(self):
        audit = load_audit()
        failed = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                  "timeout": False, "_json": {"data": {"installations": []}}}
        fixture = {"name": "frontend-design", "source": "unused",
                   "repo": "example/skills", "folder": "frontend-design",
                   "skill_md_git_blob_sha1": "0" * 40}
        with (TemporaryDirectory() as temporary,
              patch.object(audit, "call", return_value=failed),
              patch.object(audit, "initialize_project_skill_git_root") as initialize):
            root = Path(temporary).resolve()
            for target, kind, item in (
                    ("kimi", "plugin", "atlassian"),
                    ("grok", "plugin", "atlassian"),
                    ("qoder", "skill", fixture)):
                audit.do_case("/unused/agentplugins", target, kind, item, 0,
                              root / f"{target}-{kind}", {})
            self.assertEqual(initialize.call_count, 0)
            for target in ("kimi", "grok"):
                audit.do_case("/unused/agentplugins", target, "skill", fixture,
                              0, root / f"{target}-skill", {})
            self.assertEqual(initialize.call_count, 2)
            self.assertEqual(
                {call.args[0] for call in initialize.call_args_list},
                {root / "kimi-skill/skill-00/project",
                 root / "grok-skill/skill-00/project"})

    def test_grok_project_skill_gets_real_git_root_before_install(self):
        # BUG-063: upstream grok inspect projectRoot is Git-discovered, not cwd.
        audit = load_audit()
        self.assertTrue(audit.GIT_EXECUTABLE and Path(audit.GIT_EXECUTABLE).is_file())
        item = {"name": "frontend-design", "source": "unused",
                "repo": "example/skills", "folder": "frontend-design",
                "skill_md_git_blob_sha1": "0" * 40}
        failed = {"exit_code": 1, "stdout_tail": "", "stderr_tail": "test refusal",
                  "timeout": False, "_json": {"data": {"installations": []}}}
        original_call = audit.call
        def fake_call(argv, env, cwd, timeout=105):
            if argv[0] == audit.GIT_EXECUTABLE:
                return original_call(argv, env, cwd, timeout)
            return dict(failed)
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            with patch.object(audit, "call", side_effect=fake_call):
                result = audit.do_case("/unused/agentplugins", "grok", "skill",
                                       item, 0, root, {})
            project = root / "skill-00/project"
            self.assertEqual(result["status"], "install-failed")  # mocked add is not PASS
            self.assertTrue(result["sentinels_preserved"])
            self.assertTrue((project / ".git").is_dir())
            self.assertFalse((project / ".git").is_symlink())
            checked = subprocess.run([audit.GIT_EXECUTABLE, "-C", str(project),
                                      "rev-parse", "--show-toplevel"],
                                     capture_output=True, text=True, check=False)
            self.assertEqual(checked.returncode, 0)
            self.assertEqual(Path(checked.stdout.strip()).resolve(), project)
            self.assertTrue((project / ".grok/skills/foreign-neighbor/SKILL.md").is_file())
            self.assertFalse((project / ".grok/plugins/foreign-neighbor").exists())

    def test_qoder_latest_log_link_only_qualifies_closed_run_diagnostics(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve() / ".qoder"
            run = root / "logs" / "runs" / "session-1"
            run.mkdir(parents=True)
            (run / "qodercli.log").write_text("diagnostic only")
            link = root / "logs/latest"
            observed = [".qoder/logs/latest"]
            link.symlink_to("runs/session-1", target_is_directory=True)
            self.assertTrue(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "codex"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            outside = root.parent / "outside"
            outside.mkdir()
            link.symlink_to("../../outside", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", observed, "qoder"),
                [".qoder/logs/latest: symlink"])
            link.unlink()
            link.symlink_to("runs/missing", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            (root / "logs/runs/chain").symlink_to(run, target_is_directory=True)
            link.symlink_to("runs/chain", target_is_directory=True)
            self.assertFalse(audit.qoder_passive_latest_log_link(root, link))
            link.unlink()
            other = root / "logs/other"
            other.symlink_to("runs/session-1", target_is_directory=True)
            self.assertEqual(audit.native_identity_residuals(
                [root], "atlassian", [".qoder/logs/other"], "qoder"),
                [".qoder/logs/other: symlink"])

    def test_post_source_redirected_ancestor_is_rejected_before_reading_native_state(self):
        # BUG-044: a plugin can replace a parent path AFTER the initial
        # containment check; the descendant native root itself is not a symlink.
        import shutil
        audit = load_audit()
        for target, poisoned in (("codex", "home"), ("opencode", "native-ancestor"),
                                 ("codex", "project")):
            with self.subTest(target=target, poisoned=poisoned), TemporaryDirectory() as temp:
                base = Path(temp).resolve()
                runroot = base / "runs"
                foreign = base / "foreign"
                foreign.mkdir()
                calls = 0

                def malicious_install(argv, env, cwd, timeout=105):
                    nonlocal calls
                    calls += 1
                    self.assertEqual(calls, 1)  # no subsequent read-only list
                    case = runroot / "plugin-00"
                    path = (case / "home/.config" if poisoned == "native-ancestor"
                            else case / poisoned)
                    self.assertTrue(path.is_dir())
                    shutil.rmtree(path)
                    path.symlink_to(foreign, target_is_directory=True)
                    return {"exit_code": 0, "_json": None, "stdout_tail": "",
                            "stderr_tail": "", "timeout": False}

                with patch.object(audit, "call", side_effect=malicious_install), \
                     self.assertRaisesRegex(ValueError, "audit case"):
                    audit.do_case("/unused/agentplugins", target, "plugin",
                                  "atlassian", 0, runroot, {})
                self.assertEqual(calls, 1)
                self.assertEqual(list(foreign.iterdir()), [])


    def test_case_root_symlink_never_redirects_prewrite_fixtures(self):
        # BUG-041: an old case could redirect the *case* trust anchor itself.
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            foreign = outer / "foreign"
            foreign.mkdir()
            for poisoned in ("case-root", "run-root", "post-mkdir-swap"):
                with self.subTest(poisoned=poisoned):
                    root = outer / poisoned
                    case = root / "plugin-00"
                    if poisoned == "run-root":
                        root.symlink_to(foreign, target_is_directory=True)
                    else:
                        root.mkdir()
                        if poisoned == "case-root":
                            case.symlink_to(foreign, target_is_directory=True)
                    original_mkdir = Path.mkdir
                    modified = False

                    def post_mkdir(path, *args, **kwargs):
                        nonlocal modified
                        result = original_mkdir(path, *args, **kwargs)
                        if poisoned == "post-mkdir-swap" and path == case and not modified:
                            modified = True
                            case.rmdir()
                            case.symlink_to(foreign, target_is_directory=True)
                        return result

                    with (patch.object(Path, "mkdir", post_mkdir),
                          patch.object(audit, "call") as call,
                          self.assertRaisesRegex(ValueError, "audit (case|run) root")):
                        audit.do_case("/unused/agentplugins", "codex", "plugin",
                                      "atlassian", 0, root, {})
                    call.assert_not_called()
                    self.assertEqual(list(foreign.iterdir()), [])
                    if poisoned != "run-root":
                        # Leave the root available for the next disposable case.
                        case.unlink()
                        root.rmdir()
                    else:
                        root.unlink()

    def test_case_creation_never_writes_through_planted_child_symlinks(self):
        audit = load_audit()
        original_mkdir = Path.mkdir
        with TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            foreign = outer / "foreign"
            foreign.mkdir()
            for poisoned in ("home", "local-parent"):
                with self.subTest(poisoned=poisoned):
                    runroot = outer / poisoned
                    case = runroot / "plugin-00"

                    injected = set()

                    def injected_mkdir(path, *args, **kwargs):
                        result = original_mkdir(path, *args, **kwargs)
                        # Path.mkdir(parents=True) may retry the same path.
                        # Plant a symlink exactly once, before the next guard.
                        if not injected and poisoned == "home" and path == case:
                            injected.add(poisoned)
                            (case / "home").symlink_to(foreign, target_is_directory=True)
                        elif not injected and poisoned == "local-parent" and path == case / "home":
                            injected.add(poisoned)
                            (case / "home/.local").symlink_to(
                                foreign, target_is_directory=True)
                        return result

                    with (patch.object(Path, "mkdir", injected_mkdir),
                          self.assertRaisesRegex(ValueError, "nested audit directory")):
                        audit.do_case("/unused/agentplugins", "codex", "plugin",
                                      "atlassian", 0, runroot, {})
                    self.assertEqual(injected, {poisoned})
                    self.assertEqual(list(foreign.iterdir()), [])

    def test_failed_first_list_cannot_attest_plugin_lifecycle(self):
        # BUG-029: matching JSON from a failed list is not an attestation.
        audit = load_audit()
        rows = [{"installation_id": "owned-id", "name": "atlassian",
                 "clients": [{"client_id": "claude"}]}]
        operations = []

        def fake_call(argv, env, cwd, timeout=105):
            operation = argv[1]
            operations.append(operation)
            first_list_failed = operation == "list" and operations.count("list") == 1
            return {"exit_code": 1 if first_list_failed else 0, "stdout_tail": "",
                    "stderr_tail": "", "timeout": False,
                    "_json": {"data": {"installations": rows}}}

        with TemporaryDirectory() as tmp, \
             patch.object(audit, "call", side_effect=fake_call), \
             patch.object(audit, "matching_installations", return_value=rows):
            result = audit.do_case("/unused/agentplugins", "claude", "plugin",
                                   "atlassian", 0, Path(tmp), {})
        self.assertEqual(result["install"]["exit_code"], 0)
        self.assertEqual(result["list"]["exit_code"], 1)
        self.assertTrue(result["listed_identity"])
        self.assertIsNone(result["repeat_add"])
        self.assertIsNone(result["cleanup"])
        self.assertEqual(operations, ["add", "list"])
        self.assertEqual(result["status"], "incomplete-native-proof")


    def test_grok_prepared_skill_is_source_bound_and_never_native_pass(self):
        # A real prepared Grok Skill is not a native installation attestation.
        audit = load_audit()
        item = {"name": "frontend-design", "repo": "sample/skills",
                "folder": "frontend-design", "ref": "a" * 40}
        name = "skill-" + "b" * 56
        bundle = "skill-" + "b" * 45 + "-" + "c" * 12
        uid = "c99742fc-59bf-4225-b862-0c21e5fcc992"
        revision = {"resolved_revision": item["ref"],
                    "tree_digest": "sha256:" + "d" * 64,
                    "manifest_digest": "sha256:" + "e" * 64}
        entry = {"installation_id": uid, "name": name,
                 "source": "sample/skills/skills/frontend-design",
                 "clients": [{"client_id": "grok", "scope": "project",
                              "materialization": "materialized", "verification": "package_validated",
                              "activation": "prepared", "policy": "allowed",
                              "package_revision": revision}]}
        response = {"data": {"installations": [entry]}}
        with TemporaryDirectory() as tmp:
            project = Path(tmp) / "project"
            relative = f".grok/plugins/{bundle}/skills/{item['name']}/SKILL.md"
            skill = project / relative
            skill.parent.mkdir(parents=True)
            skill.write_text("---\nname: frontend-design\n---\n")
            (project / "uap.lock").write_text(json.dumps({"packages": [{
                "alias": item["name"], "tree_digest": revision["tree_digest"],
                "manifest_digest": revision["manifest_digest"]}]}))
            first = {"status": "succeeded", "items": [{
                "alias": item["name"], "targetId": "grok", "status": "succeeded",
                "candidateId": "sha256:" + "f" * 64,
                "destinationPath": str(project / ".grok/plugins" / bundle)}]}
            output = {"stdout_tail": json.dumps(first)}
            matched = audit.grok_prepared_skill_installations(response, item, relative, project, output)
            self.assertEqual([node["installation_id"] for node in matched], [uid])
            repeat = json.loads(json.dumps(first))
            repeat["items"][0]["status"] = "unchanged"
            repeat_output = {"stdout_tail": json.dumps(repeat)}
            self.assertEqual(len(audit.grok_prepared_skill_installations(
                response, item, relative, project, repeat_output, repeated=True)), 1)
            self.assertEqual(audit.grok_prepared_skill_installations(
                response, item, relative, project, output, repeated=True), [])
            for field in ("source", "name", "installation_id"):
                forged = json.loads(json.dumps(response))
                forged["data"]["installations"][0][field] = "forged"
                self.assertEqual(audit.grok_prepared_skill_installations(
                    forged, item, relative, project, output), [])
            for field in ("activation", "verification", "scope", "policy"):
                forged = json.loads(json.dumps(response))
                forged["data"]["installations"][0]["clients"][0][field] = "forged"
                self.assertEqual(audit.grok_prepared_skill_installations(
                    forged, item, relative, project, output), [])
            forged = json.loads(json.dumps(response))
            forged["data"]["installations"][0]["clients"][0]["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
            self.assertEqual(audit.grok_prepared_skill_installations(
                forged, item, relative, project, output), [])
            self.assertFalse(audit.grok_prepared_source_survives(
                {"data": {"installations": []}}, item, name, uid))
            self.assertTrue(audit.grok_prepared_source_survives(
                {"data": {"installations": [{**entry, "name": "reminted", "installation_id": "new"}]}},
                item, name, uid))
            original_bytes = skill.read_bytes()
            foreign = project / "outside.md"
            foreign.write_bytes(original_bytes)
            skill.unlink()
            skill.symlink_to(foreign)
            self.assertEqual(audit.grok_prepared_skill_installations(
                response, item, relative, project, output), [])
            skill.unlink()
            skill.write_bytes(original_bytes)
            self.assertEqual(len(audit.grok_prepared_skill_installations(
                response, item, relative, project, output)), 1)


    def test_codex_marketplace_containment_follows_physical_directory_identity(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            case = root / "case"
            source = case / "home/.codex/plugins/marketplaces/owned"
            foreign = root / "case-replacement/owned"
            source.mkdir(parents=True)
            foreign.mkdir(parents=True)
            self.assertTrue(audit.codex_marketplace_source_in_case(source, case))
            self.assertFalse(audit.codex_marketplace_source_in_case(foreign, case))
            self.assertFalse(audit.codex_marketplace_source_in_case(
                case / "home/.codex/plugins/missing", case))
            self.assertFalse(audit.codex_marketplace_source_in_case(Path("relative"), case))
            if os.name == "nt":
                # Same on-disk directory; Go may use the Windows extended prefix.
                extended = Path("\\\\?\\" + str(source))
                self.assertTrue(audit.codex_marketplace_source_in_case(extended, case))


    def test_genuine_project_git_init_uses_empty_hooks_inside_case(self):
        # Windows has no POSIX /dev/null; never use ambient hooks or config.
        audit = load_audit()
        if not audit.GIT_EXECUTABLE:
            self.skipTest("Git binary unavailable")
        with TemporaryDirectory() as temporary:
            case = Path(temporary).resolve() / "case"
            home, data, project = (case / "home", case / "uap-data", case / "project")
            for folder in (home, data, project):
                folder.mkdir(parents=True)
            env = audit.isolated_client_env(home, case, data)
            audit.initialize_project_skill_git_root(project, env, case)
            hooks = case / "empty-git-hooks"
            self.assertTrue(hooks.is_dir())
            self.assertFalse(hooks.is_symlink())
            self.assertTrue(hooks.resolve().is_relative_to(case.resolve()))
            self.assertTrue((project / ".git").is_dir())



class CrossPlatformPublishedProofRegressions(unittest.TestCase):
    """Keep raw published client proofs honest on ARM and Windows runners."""

    def test_windows_qoder_cache_and_log_paths_are_passive_but_settings_are_not(self):
        audit = load_audit()
        for relative in ("plugins/cache/local/atlassian/0.1.0/plugin.json",
                         "logs/runs/session-1/manifest.json"):
            self.assertTrue(audit.passive_native_path("qoder", relative), relative)
        # Only Windows treats backslashes as path separators. A POSIX
        # literal-backslash filename is not a passive managed cache.
        for relative in (r"plugins\cache\local\atlassian\0.1.0\plugin.json",
                         r"logs\runs\session-1\manifest.json"):
            self.assertEqual(audit.passive_native_path("qoder", relative),
                             os.name == "nt", relative)
        self.assertEqual(
            audit.passive_native_path("codex", r"plugins\cache\example.json"),
            os.name == "nt")
        for relative in ("settings.json", r"plugins\installed_plugins_v2.json",
                         "plugins/installed_plugins_v2.json"):
            self.assertFalse(audit.passive_native_path("qoder", relative), relative)

    def test_posix_literal_backslashes_cannot_hide_active_qoder_state(self):
        if os.name == "nt":
            self.skipTest("Windows cannot create literal backslash filenames")
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary) / ".qoder"
            root.mkdir()
            disguised = root / r"plugins\cache\managed.json"
            disguised.write_text('{"plugins":{"atlassian":{"enabled":true}}}',
                                 encoding="utf-8")
            observed = f"{root.name}/{disguised.name}"
            self.assertFalse(audit.passive_native_path("qoder", disguised.name))
            self.assertEqual(
                audit.native_identity_residuals(
                    [root], "atlassian", [observed], "qoder"),
                [observed])

    def test_qoder_profile_requires_same_canonical_disposable_root(self):
        audit = load_audit()
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            home = root / "home"
            profile = home / ".qoder"
            profile.mkdir(parents=True)
            trusted_binary = str(root / "trusted-qodercli")
            env = {"QODER_CONFIG_DIR": str(profile.resolve(strict=True))}
            rows = [{"id": "atlassian@local", "version": "0.1.0"}]
            with (patch.dict(audit.AGENT_BINARIES, {"qodercli": trusted_binary}, clear=True),
                  patch.object(audit, "call", return_value={"exit_code": 0, "_json": rows}) as probe):
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, env, root, "atlassian", "0.1.0"), {"status": "registered"})
                self.assertEqual(probe.call_args.args[0][2], str(profile))
                self.assertEqual(audit.qoder_native_plugin_state(
                    home, {"QODER_CONFIG_DIR": str(root / "other")},
                    root, "atlassian", "0.1.0")["status"], "unqualified-prerequisites")

    def test_prepared_only_opencode_config_is_not_native_pass(self):
        # Real published 0.1.102 can write opencode.json while all components
        # are reported as prepared, never native. Disk attribution is not activation.
        audit = load_audit()
        matched = [{"name": "atlassian", "installation_id": "frozen",
                    "clients": [{"client_id": "opencode", "verification": "installation_verified"}]}]
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            config = root / "plugin-00/home/.config/opencode/opencode.json"
            state = {"installed": False}

            def fake_call(argv, _env, _cwd, timeout=105):
                operation = argv[1]
                if operation == "add":
                    if not state["installed"]:
                        config.parent.mkdir(parents=True, exist_ok=True)
                        config.write_text('{"mcp":{"atlassian":{"type":"local"}}}')
                    state["installed"] = True
                elif operation == "remove":
                    config.unlink()
                    state["installed"] = False
                return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                        "timeout": False,
                        "_json": ({"data": {"actions": [
                            {"component_support": [{"name": "atlassian", "support": "prepared"}]}
                        ]}} if operation == "add" else
                                  {"data": {"installations": matched if state["installed"] else []}})}

            with (patch.object(audit, "call", side_effect=fake_call),
                  patch.object(audit, "matching_installations", return_value=matched)):
                row = audit.do_case("/unused/agentplugins", "opencode", "plugin",
                                    "atlassian", 0, root, {})
            self.assertTrue(row["prepared_components_only"])
            self.assertTrue(row["native_identity_files"])
            self.assertTrue(row["repeat_verified"])
            self.assertTrue(row["removal_verified"])
            self.assertEqual(row["status"], "prepared-lifecycle-passed")
            self.assertNotEqual(row["status"], "passed")


    def test_open_code_prepared_lifecycle_is_accepted_only_as_non_native_proof(self):
        audit = load_audit()
        row = {
            "status": "prepared-lifecycle-passed", "kind": "plugin",
            "target": "opencode", "source": "atlassian",
            "prepared_components_only": True, "native_component_candidates": [],
            "native_identity_files": ["opencode/opencode.json"],
            "listed_identity": True, "repeat_verified": True,
            "removal_verified": True, "native_residuals": [],
            "post_remove_identity_registered": False,
            "sentinels_preserved": True, "failed_install_residue": False,
            "install": {"exit_code": 0}, "cleanup": {"exit_code": 0},
        }
        self.assertTrue(audit.expected_negative_coverage(row))
        self.assertNotEqual(row["status"], "passed")
        for key, bad in (
                ("status", "passed"),
                ("target", "qoder"),
                ("prepared_components_only", False),
                ("native_component_candidates", ["atlassian"]),
                ("native_identity_files", ["outside/active.json"]),
                ("post_remove_identity_registered", True),
                ("listed_identity", False),
                ("repeat_verified", False),
                ("removal_verified", False),
                ("native_residuals", ["opencode/settings.json"]),
                ("sentinels_preserved", False),
                ("failed_install_residue", True),
                ("install", {"exit_code": 1}),
                ("cleanup", {"exit_code": 1})):
            with self.subTest(key=key):
                self.assertFalse(audit.expected_negative_coverage({**row, key: bad}))
        self.assertTrue(audit.expected_negative_coverage(
            {**row, "source": "docker-hub", "native_identity_files": []}))
        self.assertTrue(audit.expected_negative_coverage({
            **row, "source": "cloudflare",
            "native_identity_files": [
                "opencode/opencode.json",
                "opencode/skills/cloudflare/SKILL.md",
                "opencode/skills/cloudflare/references/api.md"]}))
        self.assertFalse(audit.expected_negative_coverage({
            **row, "native_identity_files": ["opencode/../other.json"]}))

    def test_kimi_windows_skill_downloads_exact_pinned_native_binary(self):
        # Kimi's actual 2.3.2 Windows .exe is accepted by the released adapter;
        # npm .cmd wrappers are not, so the audit must provision verified bytes.
        import runpy
        scripts = Path(__file__).resolve().parent
        matrix = runpy.run_path(str(scripts / "run-native-client-matrix.py"))
        self.assertEqual(matrix["PINS"]["windows-amd64"]["kimi"], (
            "github", "MoonshotAI/kimi-code", "@moonshot-ai/kimi-code@2.3.2",
            "kimi-code-win32-x64.zip",
            "sha256:398fff295d3ddaffe4fbefd9586e6cdc3eacd67bdb4437a43184615a1934ee74",
            "kimi.exe"))
        workflow = (scripts.parent / ".github/workflows/agentplugins-published-windows-audit.yml").read_text()
        self.assertIn('module.provision(module.PINS["windows-amd64"]["kimi"], target, "kimi")',
                      workflow)
        self.assertIn('binary != target / "kimi.exe"', workflow)
        self.assertIn('Verified genuine Kimi 2.3.2 Windows executable, sha256 pinned', workflow)
        self.assertIn('"$nativeBin" | Out-File -FilePath "$env:GITHUB_PATH"', workflow)
        self.assertNotIn("$nativePackage = '@moonshot-ai/kimi-code@2.3.2'", workflow)


    def test_codex_windows_cache_is_not_native_proof_but_real_uninstall_is(self):
        audit = load_audit()
        source = "github:example/agent-plugin@" + ("a" * 40)
        matched = [{"name": "atlassian", "installation_id": "one",
                    "clients": [{"client_id": "codex",
                                 "verification": "installation_verified"}]}]
        for native_removed in (False, True):
            with self.subTest(native_removed=native_removed), TemporaryDirectory() as temp:
                runroot = Path(temp).resolve()
                cache = (runroot / "plugin-00/home/.codex/plugins/cache/agentplugins-123"
                         / "atlassian/0.1.0/plugin.json")
                installed = {"value": False}

                def simulate(argv, _env, _cwd, timeout=105):
                    command = argv[1]
                    if command == "add":
                        if not installed["value"]:
                            cache.parent.mkdir(parents=True, exist_ok=True)
                            cache.write_text('{"name": "atlassian"}')
                        installed["value"] = True
                    elif command == "remove":
                        installed["value"] = False
                    return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                            "timeout": False,
                            "_json": {"data": {"installations":
                                              matched if installed["value"] else []}}}

                with (patch.object(audit, "call", side_effect=simulate),
                      patch.object(audit, "matching_installations", return_value=matched),
                      patch.object(audit, "uninstall_native_codex_plugin",
                                   return_value={"completed": native_removed})):
                    row = audit.do_case(
                        "/unused/agentplugins", "codex", "plugin", source, 0,
                        runroot, {source: "atlassian"})
                self.assertEqual(row["native_identity_files"], [])
                self.assertTrue(row["repeat_verified"])
                self.assertTrue(row["removal_verified"])
                self.assertEqual(row["status"], "passed" if native_removed
                                 else "package-installed-native-unqualified")


    def test_scoped_file_capture_reads_native_json_without_pipe_wait(self):
        audit = load_audit()
        with TemporaryDirectory() as scratch:
            result_json = {"schema_version": 1, "result": "success"}
            def completed(*_args, **kwargs):
                self.assertNotIn("capture_output", kwargs)
                self.assertEqual(kwargs["timeout"], 7)
                self.assertTrue(kwargs["stdout"].seekable())
                self.assertTrue(kwargs["stderr"].seekable())
                kwargs["stdout"].write(json.dumps(result_json).encode("utf-8"))
                kwargs["stderr"].write(b"native child stderr")
                return subprocess.CompletedProcess(["agentplugins"], 0)
            with patch.object(audit.subprocess, "run", side_effect=completed):
                actual = audit.call(["agentplugins", "add", "firebase"],
                                    {"TEMP": scratch}, scratch, timeout=7,
                                    capture_files=True)
            self.assertEqual(actual["exit_code"], 0)
            self.assertFalse(actual["timeout"])
            self.assertEqual(actual["_json"], result_json)
            self.assertEqual(actual["stderr_tail"], "native child stderr")
            with patch.object(audit.subprocess, "run", side_effect=subprocess.TimeoutExpired(
                    ["agentplugins", "add", "firebase"], 7)):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    timeout=7, capture_files=True)
            self.assertIsNone(actual["exit_code"])
            self.assertTrue(actual["timeout"])
            def oversized(*_args, **kwargs):
                kwargs["stdout"].write(b"x" * ((1 << 20) + 1))
                return subprocess.CompletedProcess(["agentplugins"], 0)
            with patch.object(audit.subprocess, "run", side_effect=oversized):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    capture_files=True)
            self.assertIsNone(actual["exit_code"])
            self.assertIsNone(actual["_json"])
            self.assertEqual(actual["stderr_tail"], "bounded native output capture exceeded")
            for payload in (b"invalid json", b'{"result":"failed"}'):
                def invalid(*_args, **kwargs):
                    kwargs["stdout"].write(payload)
                    return subprocess.CompletedProcess(["agentplugins"], 0)
                with patch.object(audit.subprocess, "run", side_effect=invalid):
                    actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                        capture_files=True)
                self.assertIsNone(actual["exit_code"])
                self.assertIsNone(actual["_json"])
                self.assertEqual(actual["stderr_tail"], "native add success JSON unavailable")
            def rejected(*_args, **kwargs):
                kwargs["stdout"].write(b'{"result":"failed"}')
                kwargs["stderr"].write(b"provider refused immutable source")
                return subprocess.CompletedProcess(["agentplugins"], 1)
            with patch.object(audit.subprocess, "run", side_effect=rejected):
                actual = audit.call(["agentplugins"], {"TEMP": scratch}, scratch,
                                    capture_files=True)
            self.assertEqual(actual["exit_code"], 1)
            self.assertEqual(actual["stderr_tail"], "provider refused immutable source")
        with patch.object(audit.subprocess, "run",
                          return_value=subprocess.CompletedProcess(
                              ["agentplugins"], 0, stdout='{"result":"success"}',
                              stderr="")) as run:
            actual = audit.call(["agentplugins"], {}, ".", timeout=7)
            self.assertEqual(actual["_json"], {"result": "success"})
            self.assertTrue(run.call_args.kwargs["capture_output"])

    def test_published_windows_firebase_timeout_is_bounded_and_scoped(self):
        audit = load_audit()
        with patch.object(audit.sys, "platform", "win32"):
            for target in ("codex", "claude", "opencode", "gemini",
                           "copilot", "cline", "kimi", "qoder", "pi"):
                self.assertEqual(audit.install_case_timeout(target, "plugin", "firebase"), 180)
                self.assertEqual(audit.install_case_timeout(target, "plugin", "docker-hub"), 105)
                self.assertEqual(audit.install_case_timeout(target, "skill", "firebase"), 105)
        with patch.object(audit.sys, "platform", "darwin"):
            self.assertEqual(audit.install_case_timeout("codex", "plugin", "firebase"), 105)
        with patch.object(audit.sys, "platform", "linux"):
            self.assertEqual(audit.install_case_timeout("claude", "plugin", "firebase"), 105)


if __name__ == "__main__":
    unittest.main()
