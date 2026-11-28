"""Read-only PM artifact audit core. No network, extraction, or project execution."""
import ast
import collections
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import textwrap
import zipfile


def audit_pm(blob, source, run, job, manifest):
    """Return bounded facts or raise a short invariant label.

    Caller authenticates metadata, verifies pinned source checkout/tree, and
    limits transport bytes. No raw logs, ZIP content, or URLs are returned.
    expected_nodes is optional and specific to a host/artifact manifest.
    """
    def need(ok, label):
        if not ok:
            raise ValueError(label)

    def digest(body):
        return hashlib.sha256(body).hexdigest()

    need(isinstance(blob, bytes), "zip_type")
    need(len(blob) == manifest["size"] and digest(blob) == manifest["sha256"],
         "zip_bytes")
    need(run["id"] == manifest["run_id"] and run["head_sha"] == manifest["head"],
         "workflow_source")
    need(job["id"] == manifest["job_id"] and job["run_id"] == manifest["run_id"],
         "job_identity")
    need(job["status"] == "completed" and job["conclusion"] == "success",
         "job_result")
    source = Path(source)
    helper = (source / "scripts/package-manager-e2e.py").read_bytes()
    constants = {}
    for node in ast.parse(helper).body:
        if isinstance(node, ast.Assign) and isinstance(node.targets[0], ast.Name):
            try:
                constants[node.targets[0].id] = ast.literal_eval(node.value)
            except (ValueError, TypeError):
                pass
    catalog_body = (source / "cli/internal/agentpluginscli/testdata/package_manager_e2e_cases.json").read_bytes()
    install_body = (source / "cli/cmd/agentplugins/testdata/package_manager_install_cases.json").read_bytes()
    catalog, installations = json.loads(catalog_body), json.loads(install_body)
    groups = constants["CATALOG_GROUPS"]
    root, install_root = constants["ROOT_TEST"], constants["INSTALL_ROOT_TEST"]
    composed = constants["COMPOSED_TESTS"]
    need(set(catalog) == set(groups), "catalog_groups")
    for cases in list(catalog.values()) + [installations]:
        need(isinstance(cases, list) and bool(cases)
             and all(isinstance(case, str) and re.fullmatch(r"[a-z][a-z0-9-]*", case)
                     for case in cases)
             and len(cases) == len(set(cases)), "catalog_case_ids")
    expected = {
        "catalog": [root] + [root + "/" + group for group in groups]
                   + [root + "/" + group + "/" + case
                      for group in groups for case in catalog[group]],
        "installations": [install_root] + [install_root + "/" + case
                                          for case in installations],
        "composition": [name + "/zip" for name in composed] + composed,
        "collections": constants["COLLECTION_TESTS"] + constants["GIT_COLLECTION_TESTS"],
    }
    cli, main = constants["CLI_PACKAGE"], constants["MAIN_PACKAGE"]
    packages = {"catalog": cli, "collections": cli,
                "installations": main, "composition": main}
    phases = ["build", "binary-help", "binary-init", "binary-missing-lock",
              "binary-malformed-package", "binary-empty-lock",
              "binary-empty-sync", "binary-empty-status"]
    phases += ["binary-install-" + suffix
               for suffix in ("first", "repeat", "info", "repair", "remove")]
    commands = ["binary-collection-" + suffix for suffix in (
        "list", "add", "repeat", "update", "remove-collection-review",
        "remove-collection-testing")]
    phases += commands + list(expected)
    with zipfile.ZipFile(io.BytesIO(blob)) as archive:
        infos = archive.infolist()
        names = archive.namelist()
        need(len(names) <= 100 and len(names) == len(set(names)), "zip_members")
        need(sum(info.file_size for info in infos) <= 32 * 1024 * 1024,
             "zip_uncompressed_limit")
        for info in infos:
            name = info.filename
            parts = PurePosixPath(name).parts
            need(bool(name) and not name.startswith(("/", "\\"))
                 and "\\" not in name and "\x00" not in name
                 and ":" not in name and all(p not in (".", "..") for p in parts)
                 and len(parts) == 1 and not info.is_dir(), "zip_name_safety")
            need(not (info.flag_bits & 1), "zip_encrypted")
        need(archive.testzip() is None, "zip_crc")
        report = json.loads(archive.read("report.json"))
        for key, value in [
            ("schema_version", 2), ("suite", "full"), ("status", "pass"),
            ("full_qualification", True), ("source_unchanged_during_run", True),
            ("excluded_tests", []),
        ]:
            need(report.get(key) == value, "report_" + key)
        identity = report["source"]
        need(identity["commit"] == manifest["checkout"]
             and identity["tree"] == manifest["tree"]
             and identity["dirty"] is False
             and identity["working_changes_sha256"] == digest(b""),
             "report_source")
        need(report["platform"] == manifest["expected_platform"], "report_platform")
        required = sum(expected.values(), [])
        need(len(required) == len(set(required)) == 140
             and report["required_tests"] == required, "canonical_140")
        need(report["catalog_sha256"] == digest(catalog_body)
             and report["installation_catalog_sha256"] == digest(install_body),
             "catalog_hashes")
        need([phase["name"] for phase in report["phases"]] == phases
             and len(phases) == 23, "canonical_23_phases")
        all_events = []
        stream_count = 0

        def events(name):
            result = []
            for line in archive.read(name).decode("utf-8").splitlines():
                if line.strip():
                    event = json.loads(line)
                    need(isinstance(event, dict)
                         and isinstance(event.get("Action"), str), "go_event_shape")
                    result.append(event)
            return result

        def validate(es, required_names, package=None):
            selected = [event for event in es
                        if package is None or event.get("Package") == package]
            need(not any(event.get("Action") in ("fail", "skip") for event in es),
                 "go_fail_skip")
            runs = collections.Counter(
                (event.get("Package"), event["Test"]) for event in es
                if event.get("Test") and event.get("Action") == "run")
            passes = collections.Counter(
                (event.get("Package"), event["Test"]) for event in es
                if event.get("Test") and event.get("Action") == "pass")
            need(runs == passes, "go_per_package_test_counts")
            for name in required_names:
                matched = [event for event in selected if event.get("Test") == name]
                need({"run", "pass"} <= {event.get("Action") for event in matched},
                     "go_required_run_pass")
                passing_packages = {event["Package"] for event in matched
                                    if event["Action"] == "pass"}
                need(bool(passing_packages) and all(
                    any(event.get("Test") is None and event.get("Action") == "pass"
                        and event.get("Package") == pkg for event in selected)
                    for pkg in passing_packages), "go_package_pass")
            for pkg, unused_test in runs:
                need(any(event.get("Test") is None and event.get("Action") == "pass"
                         and event.get("Package") == pkg for event in es),
                     "go_all_packages_pass")
            return selected

        for phase in report["phases"]:
            name = phase["name"]
            need(phase["status"] == "pass", "phase_status")
            expected_exit = 1 if name in (
                "binary-missing-lock", "binary-malformed-package") else 0
            need(phase["exit_code"] == expected_exit, "phase_exit")
            for stream in ("stdout", "stderr"):
                need(digest(archive.read(name + "." + stream))
                     == phase[stream + "_sha256"], "stream_hash")
                stream_count += 1
            if name in expected:
                need(phase["required_tests"] == expected[name],
                     "canonical_phase_required")
                argv = phase["argv"]
                need(argv[:2] == ["go", "test"] and argv.count("-count=1") == 1
                     and sum(arg == "-count" or arg.startswith("-count=")
                             for arg in argv) == 1
                     and argv.count("-json") == 1, "go_phase_argv")
                es = events(name + ".stdout")
                selected = validate(es, expected[name], packages[name])
                all_events += es
                for action, field in [
                    ("run", "started_tests"), ("pass", "passed_tests")]:
                    actual = sorted({event["Test"] for event in selected
                                     if event.get("Test")
                                     and event.get("Action") == action})
                    need(phase[field] == actual, "reported_go_inventory")
        need(stream_count == 46, "stream_count")
        counts = collections.Counter(event["Action"] for event in all_events
                                     if event.get("Test"))
        need(counts["run"] == counts["pass"], "raw_go_counts")
        if manifest.get("expected_nodes") is not None:
            need(counts["run"] == manifest["expected_nodes"],
                 "artifact_expected_nodes")
        workflow = (source / ".github/workflows/package-manager-e2e.yml").read_text()
        candidates = re.findall(
            r"(?m)^          tests = (\{.*?^          \})", workflow, re.S)
        literal_sets = []
        for candidate in candidates:
            try:
                candidate_value = ast.literal_eval(candidate)
            except (ValueError, SyntaxError):
                continue
            if isinstance(candidate_value, set) and len(candidate_value) == 17:
                literal_sets.append(candidate_value)
        need(len(literal_sets) == 1, "canonical_agreement_17_ambiguity")
        agreement = literal_sets[0]
        es = events("agreement-boundary.stdout")
        validate(es, agreement)
        actual_roots = {event["Test"] for event in es
                        if event.get("Test") and "/" not in event["Test"]}
        need(actual_roots == agreement, "canonical_agreement_17")
        collection = report["binary_skill_collections"]
        need(collection["status"] == "pass" and collection["target"] == "gemini"
             and collection["native_runtime_qualified"] is False
             and collection["commands"] == commands, "collection_contract")
        need(collection["assertions"] == [
            "read_only_local_listing", "two_local_skills_all_selection",
            "unchanged_repeat_no_drift", "update_does_not_enroll_sibling",
            "owned_removal_preserves_foreign_bytes"], "collection_assertions")
        for name in commands:
            receipt = json.loads(archive.read(name + ".stdout"))
            need(receipt.get("schema_version") == 1
                 and receipt.get("result") == "success", "collection_command")
        installation = report["binary_installation"]
        need(installation["status"] == "pass" and installation["target"] == "gemini"
             and installation["native_runtime_qualified"] is False,
             "plugin_receipt")
        need(installation["scanner"] == {"id": "lintai", "version": "0.1.3"}
             and installation["scanner_evidence_source"] == "local_scan"
             and installation["scanned_files"] == 2, "plugin_scan_receipt")
        conditional = "Verify Windows held readers and bounded metadata admission"
        source_condition = "matrix.os == 'windows-2025'"
        window_blocks = re.findall(
            r"(?ms)^      - name: " + re.escape(conditional)
            + r"\n(.*?)(?=^      - name: |\Z)", workflow)
        need(len(window_blocks) == 1
             and window_blocks[0].splitlines()[0] == "        if: " + source_condition
             and workflow.count("    name: package-manager (${{ matrix.os }})") == 1,
             "windows_reader_source_condition")
        windows = job["name"] == "package-manager (windows-2025)"
        need(job["name"] == manifest["job_name"], "job_name")
        if windows:
            need(report["platform"]["os"] == "Windows", "windows_reader_platform")
            reader_tuples = re.findall(
                r"(?ms)^          tests = (\(.*?^          \))", window_blocks[0])
            need(len(reader_tuples) == 1, "windows_reader_canonical_tuple")
            reader_names = ast.literal_eval(reader_tuples[0])
            need(isinstance(reader_names, tuple) and len(reader_names) == 6
                 and len(set(reader_names)) == 6
                 and all(isinstance(name, str) for name in reader_names),
                 "windows_reader_canonical_six")
            reader_body = archive.read("windows-readers.stdout")
            reader_events = events("windows-readers.stdout")
            validate(reader_events, reader_names)
            reader_roots = {event["Test"] for event in reader_events
                            if event.get("Test") and "/" not in event["Test"]}
            need(reader_roots == set(reader_names), "windows_reader_exact_six")
        windows_reader_receipt = {
            "status": "qualified" if windows else "not_applicable",
            "source_condition": source_condition,
            "job_name": job["name"],
            "required_roots": 6 if windows else 0,
            "raw_run_pass_verified": windows,
            "qualified_roots": sorted(reader_names) if windows else [],
            "stream_sha256": digest(reader_body) if windows else None,
            "stream_bytes": len(reader_body) if windows else 0,
            "raw_run": sum(event.get("Action") == "run" and bool(event.get("Test"))
                           for event in reader_events) if windows else 0,
            "raw_pass": sum(event.get("Action") == "pass" and bool(event.get("Test"))
                            for event in reader_events) if windows else 0,
        }
        gates = [
            "Check evidence rejection cases", "Check CI policy rejection cases",
            "Verify native failure boundaries, agreement files and ownership refusals",
            "Execute the full portable catalog",
            "Verify Win32 Unicode aliases across ZIP, TAR, source and snapshot",
            conditional, "Verify native Git policy with real Git on each OS",
            "Verify project lock isolation and typed foreign Skill refusal through public CLI",
            "Verify actual process crash and durable transaction recovery",
            "Verify immutable Git EOL bytes across platforms",
            "Preserve original agreement boundary streams", "Retain execution evidence",
        ]
        for name in gates:
            matching = [step for step in job["steps"] if step["name"] == name]
            need(len(matching) == 1, "step_inventory")
            step = matching[0]
            conclusion = "skipped" if name == conditional and not windows else "success"
            need(step["status"] == "completed"
                 and step["conclusion"] == conclusion, "actual_gate_result")
        for step in job["steps"]:
            conclusion = ("skipped" if step["name"] == conditional and not windows
                          else "success")
            need(step["status"] == "completed"
                 and step["conclusion"] == conclusion, "actual_step_result")
    return {
        "status": "pass", "artifact_id": manifest["artifact_id"],
        "run_id": run["id"], "job_id": job["id"], "head": manifest["head"],
        "checkout": identity["commit"], "tree": identity["tree"],
        "platform": report["platform"], "zip_size": len(blob),
        "zip_sha256": digest(blob), "zip_crc": "pass", "phases": 23,
        "stream_hashes": 46, "canonical_required": 140,
        "raw_run": counts["run"], "raw_pass": counts["pass"],
        "agreement_roots": 17, "collection_commands": 6,
        "windows_held_readers": windows_reader_receipt,
        "native_runtime_qualified": False,
        "boundaries": [
            "Filesystem preservation follows executed pinned assertions; full original filesystem snapshots are not retained.",
            "Plugin scanner receipt does not independently qualify collection scanner receipt.",
            "No authentication or model-call qualification.",
        ],
    }
