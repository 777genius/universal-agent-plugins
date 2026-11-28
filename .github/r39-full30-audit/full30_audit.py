"""Offline full-cohort retained-evidence audit; no product imports or execution.

Pure Windsurf report-reader checks below are adapted from pinned source summary /
native_reference. Filesystem custody/oracle code is never imported or evaluated.
"""
import ast
import base64
import hashlib
import io
import json
import math
from pathlib import PurePosixPath as Path
import posixpath
import re
import stat
import unicodedata
from uuid import UUID
import zipfile

HEAD = "fccd72199a1eedc12e3828cf2fdc2f10a2f418f7"
TREE = "41d82c5eb527f34d16ec2e458194496012bc1c22"
FIXTURE_SHA = "e22cb3b5b23e74a9e03b55e24a684ab93038f89cf18e28fd5d9755175754c0d9"
SOURCE_BLOBS = {
    "scripts/live-install-audit.py": "ceab34554b6851be09fae9dbc5345fdd753bd0f8",
    "scripts/windsurf_installation_audit.py": "a2aa4976159c1664d916d4471475c6c6430858e7",
    "scripts/windsurf_global_skill_audit.py": "5c6ebe69d5ced9f6f44843171265f07c1988efbb",
    "scripts/codex_runtime_audit.py": "759b35886a434a7f17e2c36d8fd93d6a12bbe45d",
    "scripts/codex_plugin_retention_audit.py": "d4d7a75645bbbda74999b159e4f2d49d51e04004",
    "tests/live-install/fixtures.json": "6c8ed47be814d45629b0dee2b98d58e283486d32",
    ".github/workflows/live-install-audit.yml": "f1cec48991179933910e026195372ab8c5a0d5e8",
    ".github/workflows/live-install-remaining-clients.yml": "8e63129f0d59c139abd57c7bedfe0025f9f75632",
}
def require(ok, reason="unresolved retained evidence"):
    if not ok:
        raise ValueError(reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def hex_digest(value):
    require(isinstance(value, str) and re.fullmatch("[0-9a-f]{64}", value),
            "invalid SHA-256")
    return value


def object_pairs(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON key")
        result[key] = value
    return result


def document(body):
    require(isinstance(body, bytes) and len(body) <= 64 << 20, "JSON size/type")
    return json.loads(body.decode("utf-8"), object_pairs_hook=object_pairs,
                      parse_constant=lambda _: require(False, "nonfinite JSON"))


def git_blob(text, expected):
    require(isinstance(text, str), "missing canonical source text")
    body = text.encode("utf-8")
    observed = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
    require(observed == expected, "canonical source blob mismatch")


def zip_files(blob, manifest):
    require(isinstance(blob, bytes) and len(blob) <= 256 << 20, "archive size")
    require(len(blob) == manifest["archive_size"], "archive metadata size")
    require(sha(blob) == hex_digest(manifest["archive_sha256"]), "archive digest")
    files, seen, directories, total = {}, set(), set(), 0
    with zipfile.ZipFile(io.BytesIO(blob)) as archive:
        require(len(archive.infolist()) <= 20000, "too many ZIP members")
        for item in archive.infolist():
            name = item.filename
            require(isinstance(name, str) and 0 < len(name) <= 2048
                    and not any(unicodedata.category(c) == "Cc" for c in name)
                    and item.orig_filename == name
                    and "\\" not in name and ":" not in name
                    and not name.startswith("/"),
                    "unsafe ZIP member")
            directory = name.endswith("/")
            parts = name[:-1].split("/") if directory else name.split("/")
            require(all(part not in ("", ".", "..") for part in parts), "ZIP traversal")
            canonical = "/".join(parts)
            require(canonical not in seen, "duplicate ZIP name")
            seen.add(canonical)
            require(not item.flag_bits & 1, "encrypted ZIP member")
            require(item.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED),
                    "unsupported ZIP compression")
            mode = stat.S_IFMT(item.external_attr >> 16)
            require(mode in (0, stat.S_IFDIR if directory else stat.S_IFREG),
                    "nonregular ZIP member")
            require(item.file_size <= 64 << 20, "ZIP member too large")
            total += item.file_size
            require(total <= 512 << 20, "ZIP expanded-size limit")
            body = archive.read(item)  # zipfile verifies the original CRC.
            require(len(body) == item.file_size, "ZIP size mismatch")
            if directory:
                require(not body, "nonempty directory entry")
                directories.add(canonical)
            else:
                files[canonical] = body
    for name in seen:
        for index in range(1, len(name.split("/"))):
            require("/".join(name.split("/")[:index]) not in files,
                    "ZIP file/directory collision")
    return files, len(directories), total




def selected(fixtures, index):
    require(type(index) is int and 0 <= index < 30)
    sources = fixtures["plugin_aliases"]
    require(type(sources) is list and len(sources) == 30)
    source = sources[index]
    require(type(source) is str)
    return source, fixtures["plugin_declared_names"].get(source, source)


def encoded(value, limit):
    body = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
    require(len(body) <= limit, "canonical JSON bound")
    return body


def digest(value, limit=4 << 20):
    return sha(encoded(value, limit))


def literals(text, names):
    values = {}
    for node in ast.parse(text).body:
        if isinstance(node, ast.Assign) and len(node.targets) == 1:
            name = node.targets[0]
            if isinstance(name, ast.Name) and name.id in names:
                require(name.id not in values, "duplicate literal authority")
                values[name.id] = ast.literal_eval(node.value)
    require(set(values) == set(names), "missing literal authority")
    return values


def authority(manifest):
    texts = manifest["sources"]
    require(isinstance(texts, dict) and set(texts) == set(SOURCE_BLOBS),
            "source inventory")
    for path, expected in SOURCE_BLOBS.items():
        git_blob(texts[path], expected)
    fixture_body = texts["tests/live-install/fixtures.json"].encode()
    require(sha(fixture_body) == FIXTURE_SHA, "fixture SHA")
    fixtures = document(fixture_body)
    require(fixtures.get("schema_version") == 1
            and len(fixtures["plugin_aliases"]) == len(fixtures["skills"]) == 30
            and len(set(fixtures["plugin_aliases"])) == 30, "fixture cohort")
    names = ("ACTIVE_SLOTS", "COMPONENT_REFUSALS", "PREPARED_GIT_PINS", "PREPARED",
             "FIREBASE_RUNTIME_SOURCE", "FIREBASE_RUNTIME_MANIFEST",
             "FIREBASE_RUNTIME_CONFIG", "CLI_VERSION", "CLIENT_VERSION")
    constants = literals(texts["scripts/windsurf_installation_audit.py"], names)
    constants["CASES"] = literals(texts["scripts/windsurf_global_skill_audit.py"],
                                 ("CASES",))["CASES"]
    constants["ACTIVE_SKILLS"] = dict(constants["PREPARED"], activation="active",
                                     authentication="not_required")
    require(set(constants["ACTIVE_SLOTS"]) == {22, 25, 26, 29}
            and set(constants["COMPONENT_REFUSALS"]) == {23, 24, 27, 28}
            and constants["CLI_VERSION"] == "agentplugins 0.1.115"
            and constants["CLIENT_VERSION"] == "devin 3000.12.57 (96542e930752)",
            "closed Windsurf authority")
    return fixtures, constants


def command(record):
    require(isinstance(record, dict) and type(record.get("exit_code")) is int
            and record["exit_code"] == 0 and record.get("timeout") is False,
            "public/native command did not complete")
    for key in ("stdout_tail", "stderr_tail"):
        require(isinstance(record.get(key), str) and len(record[key]) <= 1800,
                "command tail shape")


def canonical_absolute(value):
    require(isinstance(value, str) and value.startswith("/")
            and posixpath.normpath(value) == value and not value.startswith("//")
            and not any(unicodedata.category(c) == "Cc" for c in value),
            "noncanonical Linux coordinate")
    return value


def source_selection(report, fixtures, target, kind, run, source):
    require(report.get("target") == target and report.get("kind") == kind
            and report.get("platform") == "linux" and report.get("architecture") == "x86_64"
            and report.get("fixture_sha256") == FIXTURE_SHA
            and report.get("github_sha") == source["checkout"]
            and report.get("pr_head_sha") == HEAD
            and report.get("github_run_id") == str(run), "report identity")
    require(report.get("source_selection_complete") is True
            and report.get("full_coverage") is True
            and type(report.get("fixture_count")) is int and report["fixture_count"] == 30
            and type(report.get("cases_attempted")) is int and report["cases_attempted"] == 30
            and isinstance(report.get("results"), list) and len(report["results"]) == 30,
            "not a complete thirty-case cohort")
    require(report.get("cli_version") ==
            {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0},
            "wrong audit CLI")
    canonical_absolute(report.get("cli"))
    canonical_absolute(report.get("client_binary"))
    cases = fixtures["plugin_aliases"] if kind == "plugin" else fixtures["skills"]
    for index, (row, item) in enumerate(zip(report["results"], cases)):
        selected = item if kind == "plugin" else item["source"]
        identity = fixtures["plugin_declared_names"].get(item, item) if kind == "plugin" else item["name"]
        require(isinstance(row, dict) and type(row.get("index")) is int
                and row["index"] == index and row.get("target") == target
                and row.get("kind") == kind and row.get("canonical_client_id") == target
                and row.get("source") == selected and row.get("identity") == identity,
                "canonical case order/source mismatch")
        require(row.get("sentinels_preserved") is True
                and row.get("failed_install_registered") is False
                and row.get("failed_install_residue") is False
                and row.get("post_remove_identity_registered") is False
                and row.get("native_residuals") == [], "residue/foreign guard")
    return cases


def native_inventory(body):
    rows = document(body)
    require(isinstance(rows, list) and len(rows) <= 256, "native inventory row count")
    result, folded = {}, set()
    keys = set("name description provider base_dir display_name triggers warnings errors".split())
    for row in rows:
        require(isinstance(row, dict) and set(row) == keys, "native inventory fields")
        require(all(isinstance(row[k], str) for k in
                    ("name", "description", "provider", "base_dir", "display_name")),
                "native inventory scalars")
        require(row["name"] and row["display_name"] and row["name"].casefold() not in folded,
                "native duplicate identity")
        folded.add(row["name"].casefold())
        require(row["provider"] in ("Builtin", "Devin", "Windsurf", "Claude", "Copilot")
                and (row["base_dir"] == "" and row["provider"] == "Builtin"
                     or row["base_dir"] == canonical_absolute(row["base_dir"])),
                "native provider/path")
        triggers = row["triggers"]
        require(isinstance(triggers, list) and 1 <= len(triggers) <= 2
                and all(t in ("model", "user") for t in triggers)
                and len(set(triggers)) == len(triggers)
                and row["warnings"] == row["errors"] == [], "native triggers/diagnostics")
        result[row["name"]] = dict(row, triggers=sorted(triggers))
    return result


def stream(capture, record, key):
    value = capture[key]
    require(isinstance(value, dict) and set(value) == {"bytes", "sha256", "base64"}
            and isinstance(value["base64"], str) and len(value["base64"]) <= 1400000,
            "retained native stream")
    raw = base64.b64decode(value["base64"], validate=True)
    require(len(raw) <= 1 << 20 and type(value["bytes"]) is int
            and value["bytes"] == len(raw) and value["sha256"] == sha(raw)
            and type(record[key + "_bytes"]) is int
            and record[key + "_bytes"] == len(raw)
            and record[key + "_sha256"] == sha(raw)
            and record[key + "_file"] == record["probe"] + "." + key + ".log",
            "original native stream hash/size join")
    return raw


def windsurf_native(row, report, constants):
    proof, index = row["windsurf_global_native_proof"], row["index"]
    pin = constants["CASES"][row["source"]]
    root = canonical_absolute(proof["case_root"])
    require(posixpath.basename(root) == "plugin-" + format(index, "02d"), "case root")
    profile = root + "/home/.config/devin"
    require(proof["profile_authored_before_first_child"]["global_root"] == profile
            and proof["profile_authored_before_first_child"]["native_cwd"] == "/"
            and proof["binary"]["path"] == report["client_binary"]
            and proof["binary"]["sha256"] ==
            "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d",
            "native profile/binary")
    require(type(proof.get("successful_guards")) is int and proof["successful_guards"] > 0,
            "native custody guards absent")
    owner = proof["owner"]
    selected = owner["native_rows"]
    require(isinstance(selected, dict) and set(selected) == set(pin["skills"]),
            "closed independent Skill set")
    for name, row_value in selected.items():
        require(row_value.get("name") == row_value.get("display_name") == name
                and row_value.get("provider") == "Devin"
                and row_value.get("base_dir") == profile + "/skills/" + name
                and row_value.get("triggers") == ["model", "user"]
                and row_value.get("warnings") == row_value.get("errors") == [],
                "owned native Skill identity")
    observations = proof["observations"]
    baseline, stream_hashes = None, []
    keys = set("probe argv cwd error exit elapsed_seconds stdout_file stderr_file "
               "stdout_bytes stderr_bytes stdout_sha256 stderr_sha256".split())
    for phase, capture in zip(("before", "add", "repeat", "remove"), observations):
        record = capture["record"]
        require(set(record) == keys and record["probe"] == "windsurf-global-" + phase
                and record["argv"] == [report["client_binary"], "skills", "list", "--json"]
                and record["cwd"] == "/" and record["error"] is None
                and type(record["exit"]) is int and record["exit"] == 0
                and type(record["elapsed_seconds"]) in (int, float)
                and math.isfinite(record["elapsed_seconds"])
                and 0 <= record["elapsed_seconds"] <= 20, "native probe invocation")
        stdout, stderr = stream(capture, record, "stdout"), stream(capture, record, "stderr")
        require(stderr == b"", "native stderr")
        actual = native_inventory(stdout)
        if phase == "before":
            baseline = actual
            def entry(name, description, provider, base, triggers=("model", "user")):
                return dict(name=name, description=description, provider=provider,
                            base_dir=base, display_name=name, triggers=list(triggers),
                            warnings=[], errors=[])
            original = {
                "devin-cli": entry("devin-cli", "Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)",
                                   "Builtin", posixpath.dirname(posixpath.dirname(report["client_binary"])) + "/share/devin", ("model",)),
                "declarative-repo-setup": entry("declarative-repo-setup", "Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo", "Builtin", ""),
                "upload-secrets": entry("upload-secrets", "Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation", "Builtin", ""),
                "foreign-windsurf-global-audit": entry("foreign-windsurf-global-audit", "Inert foreign Global Skill", "Devin", profile + "/skills/foreign-windsurf-global-audit"),
            }
            require(baseline == original, "source-authorized builtin/foreign native baseline")
        expected = dict(baseline)
        if phase in ("add", "repeat"):
            expected.update(selected)
        require(actual == expected, "complete original native inventory differs")
        stream_hashes.extend((sha(stdout), sha(stderr)))
    # Active lanes preserve full public stdout; join to six canonical-JSON hashes.
    labels = ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")
    observations = row["windsurf_installation_audit"]["observations"]
    for label, observed in zip(labels, observations):
        operation = row[label]
        command(operation)
        require(isinstance(operation.get("stdout_full"), str)
                and isinstance(operation.get("stderr_full"), str), "missing active public stream")
        raw = operation["stdout_full"].encode()
        require(len(raw) <= 1 << 20
                and digest(document(raw), 1 << 20) == observed["json_sha256"],
                "public full-output/canonical-observation join")
    for phase, label in (("add", "install"), ("repeat", "repeat_add"), ("remove", "cleanup")):
        transition = next(t for t in proof["public_transitions"] if t["phase"] == phase)
        raw = row[label]["stdout_full"].encode()
        require(transition["stdout_bytes"] == len(raw)
                and transition["stdout_sha256"] == sha(raw), "public transition stream join")
    return stream_hashes


def windsurf_report(report, fixtures, constants):
    require(report.get("acceptance") == "windsurf-global-installation",
            "wrong Windsurf acceptance")
    checked = windsurf_summary(report, fixtures, constants)
    for key, expected in {
        "installation_passed": 25, "installation_prepared_lifecycle_passed": 21,
        "installation_active_skill_lifecycle_passed": 4,
        "installation_verified_negative_coverage": 5,
        "installation_acceptance_passed": True, "retained_native_acceptance_passed": True,
        "native_runtime_qualified": False, "authentication_qualified": False,
        "model_qualified": False, "mcp_tools_qualified": False,
    }.items():
        require(type(checked.get(key)) is type(expected) and checked[key] == expected
                and type(report.get(key)) is type(expected) and report[key] == expected,
                "Windsurf independent summary mismatch: " + key)
    require(type(report.get("passed")) is int and report["passed"] == 4, "native PASS count")
    streams = []
    for row in report["results"]:
        index = row["index"]
        negative = index in {13, 23, 24, 27, 28}
        observations = row["windsurf_installation_audit"]["observations"]
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add = [report["cli"], "add", row["source"], *scope]
        listed = [report["cli"], "list", *scope]
        selector = (row["windsurf_global_native_proof"]["owner"]["uuid"]
                    if index in constants["ACTIVE_SLOTS"] else row["identity"])
        expected = [add, listed] if negative else [
            add, listed, add, listed, [report["cli"], "remove", selector, *scope], listed]
        require([entry["argv"] for entry in observations] == expected,
                "Windsurf original public observation order")
        if index in constants["ACTIVE_SLOTS"]:
            streams.extend(windsurf_native(row, report, constants))
    require(len(streams) == 32, "four cases/four phases/two original streams")
    # Firebase index six: six recorded saved-authority observations, not uploaded frames.
    firebase = report["results"][6]
    require(firebase["source"] == "firebase" and firebase["identity"] == "firebase"
            and len(firebase["windsurf_installation_audit"]["observations"]) == 6,
            "Firebase complete installation observations")
    independent = fixtures["signed_release_pins"]["firebase"]
    runtime = constants["FIREBASE_RUNTIME_SOURCE"]
    require(all(independent[k] == v for k, v in {
        "repository": runtime["repository"], "path": runtime["package_subpath"],
        "revision": runtime["resolved_revision"], "tree_digest": runtime["tree_digest"],
        "manifest_digest": constants["FIREBASE_RUNTIME_MANIFEST"]}.items()),
        "Firebase closed immutable source tuple")
    return {"installation_positive": 25, "prepared_positive": 21, "active_skill_positive": 4,
            "verified_negatives": 5, "native_pass": 4, "original_native_streams": 32,
            "original_native_stream_hashes_sha256": digest(streams),
            "firebase_saved_observations": 6,
            "firebase_capture_scope": "source-pinned in-memory assertions; frames not uploaded",
            "firebase_byte_pools": {"ordinary": 256 << 20, "qualified_runtime": 256 << 20}}


def file_facts(files, *, temporary=False):
    require(isinstance(files, dict) and len(files) <= 20000, "runtime file facts")
    if temporary:
        wanted = sha(b"ref: refs/heads/main\n")
        require(len(files) <= 64 and all(
            isinstance(name, str) and re.fullmatch(r"\.codex/\.tmp/git-[A-Za-z0-9]{6}/HEAD", name)
            and value == wanted for name, value in files.items()),
            "temporary Git original HEAD hash/namespace")
        return
    for name, fact in files.items():
        require(isinstance(name, str) and name and "\\" not in name
                and not name.startswith("/") and all(p not in ("", ".", "..") for p in name.split("/"))
                and isinstance(fact, dict) and type(fact.get("bytes")) is int
                and 0 <= fact["bytes"] <= 32 << 20
                and isinstance(fact.get("sha256"), str)
                and re.fullmatch("[0-9a-f]{64}", fact["sha256"]), "runtime file hash/size")
    require(sum(fact["bytes"] for fact in files.values()) <= 64 << 20,
            "runtime aggregate byte assertion")


def sqlite_baseline(value):
    require(isinstance(value, dict) and set(value) <= {
        "state_5.sqlite", "logs_2.sqlite", "goals_1.sqlite", "memories_1.sqlite",
        "queue_1.sqlite", "thread_history_1.sqlite"}, "SQLite baseline databases")
    for database in value.values():
        require(isinstance(database, dict) and set(database) == {"schema", "tables", "metadata"}
                and isinstance(database["schema"], list) and database["schema"]
                and isinstance(database["tables"], dict)
                and set(database["metadata"]) == {"user_version", "application_id"},
                "SQLite semantic baseline shape")
        for table in database["tables"].values():
            require(type(table.get("rows")) is int and 0 <= table["rows"] <= 20000
                    and re.fullmatch("[0-9a-f]{64}", table.get("sha256", "")),
                    "SQLite table semantic digest")


def signed_pin(row, pin):
    wanted = {"source": pin["repository"] + ("/" + pin["path"] if pin["path"] else ""),
              "recorded_distribution": pin["distribution"], "recorded_revision": pin["revision"],
              "recorded_release_sequence": pin["sequence"], "resolved_revision": pin["revision"],
              "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
              "manifest_digest": pin["manifest_digest"]}
    require(row.get("signed_release_observation") == [wanted], "signed source pin observation")


def codex_report(report, fixtures, kind):
    # expected_negative_coverage's pinned branches cover other targets, never Codex.
    require(report.get("acceptance", "native") == "native"
            and report.get("client_version") ==
            {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0}
            and type(report.get("passed")) is int and report["passed"] == 30
            and type(report.get("verified_negative_coverage")) is int
            and report["verified_negative_coverage"] == 0, "Codex full native acceptance")
    phases = ("before-install", "installed", "repeat-installed", "removed")
    marketplace_nonzero = 0
    for row in report["results"]:
        require(row.get("status") == "passed"
                and all(row.get(key) is True for key in (
                    "listed_identity", "repeat_verified", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid"))
                and (row.get("repeat_native_unchanged") is True
                     or row.get("repeat_native_runtime_only") is True),
                "Codex lifecycle/repeat assertions")
        for label in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            command(row.get(label))
        ids = row.get("matched_installation_ids")
        require(isinstance(ids, list) and len(ids) == 1 and isinstance(ids[0], str)
                and ids == row.get("repeat_installation_ids"), "stable installation identity")
        if kind == "plugin":
            pin = fixtures["signed_release_pins"].get(row["source"])
            pin = fixtures.get("signed_release_pins_by_target", {}).get("codex", {}).get(row["source"], pin)
            if pin is not None:
                signed_pin(row, pin)
            uninstall = row.get("native_uninstall", {})
            require(uninstall.get("completed") is True
                    and uninstall.get("native_plugin_absent") is True
                    and uninstall.get("native_marketplace_absent") is True, "native uninstall")
            command(uninstall.get("plugin_remove"))
            market = uninstall.get("marketplace_remove")
            require(isinstance(market, dict) and type(market.get("timeout")) is bool
                    and (market.get("exit_code") is None
                         or type(market.get("exit_code")) is int)
                    and all(isinstance(market.get(key), str) and len(market[key]) <= 1800
                            for key in ("stdout_tail", "stderr_tail")),
                    "native marketplace removal record missing")
            # Canonical completed joins actual plugin success and both absent
            # config entries; an already-absent marketplace may return nonzero.
            marketplace_nonzero += market.get("exit_code") != 0
            proof = row.get("codex_plugin_sqlite_proof", {})
            require(proof.get("native_version") == "0.153.4"
                    and proof.get("runtime_root") == "home/.local/state/codex-sqlite"
                    and proof.get("runtime_semantics_unchanged") is True,
                    "plugin SQLite source-pinned assertion")
            sqlite_baseline(proof.get("runtime_baseline"))
            observations = proof.get("observations", [])
            require([o.get("phase") for o in observations] ==
                    ["before-install"] + ["after-public-command"] * 4 +
                    ["after-native-uninstall"] + ["after-public-command"] * 2,
                    "plugin SQLite checkpoint sequence")
            for observed in observations:
                file_facts(observed.get("runtime_files_fully_read"))
        else:
            require(all(row.get(key) is True for key in (
                "codex_public_add_ready", "codex_public_repeat_ready", "lock_exact_source_pinned",
                "skill_bytes_match"))
                and row.get("post_removal_leftovers") == []
                and row.get("public_skill_ready") == {"after-add": True, "repeat-add": True},
                "raw Skill readiness/pin/cleanup assertions")
            proof = row.get("codex_runtime_proof", {})
            require(proof.get("native_version") == "0.153.4"
                    and proof.get("runtime_root") == "home/.local/state/codex-sqlite",
                    "raw Skill runtime identity")
            sqlite_baseline(proof.get("runtime_baseline"))
            observations = proof.get("observations", [])
            require([o.get("phase") for o in observations] == list(phases),
                    "raw Skill four native checkpoint sequence")
            for observed, present in zip(observations, (False, True, True, False)):
                require(observed.get("selected_skill_present") is present
                        and observed.get("foreign_skill_present") is True
                        and type(observed.get("native_exit_code")) is int
                        and observed["native_exit_code"] == 0
                        and observed.get("runtime_integrity_verified") is True
                        and observed.get("runtime_semantics_unchanged") is True,
                        "raw Skill native assertion incomplete")
                file_facts(observed.get("runtime_files_fully_read"))
                file_facts(observed.get("temporary_git_files_fully_read"), temporary=True)
    return {"native_pass": 30, "verified_negatives": 0,
            "native_client_version": "0.153.4", "audit_cli_version": "0.1.115",
            "native_rpc_streams_uploaded": False,
            "marketplace_remove_nonzero_or_unknown": marketplace_nonzero,
            "proof_scope": "retained lifecycle/checkpoint/hash assertions, not raw RPC reconstruction"}



def vendor_devin(files):
    matches = [name for name in files if name == "vendor-proof/devin-npm.json"]
    require(len(matches) == 1, "missing retained Devin acquisition proof")
    proof = document(files[matches[0]])
    reviewed = "sha512-SZhb9rrYghHvOh0PRpZT1GpNPMMo+7P9VH6t6ZNMrBmyVX3x3XflPvCv8yomx/i7vnvf64bdFR++cRRg/c8mLQ=="
    require(proof.get("schema_version") == 1 and proof.get("version") == "3000.12.57"
            and proof.get("tarball_url") ==
            "https://registry.npmjs.org/devin/-/devin-3000.12.57-linux-x64.tgz"
            and proof.get("reviewed_integrity") == proof.get("observed_integrity") == reviewed
            and proof.get("acquisition_verified") is True
            and proof.get("binary_sha256") ==
            "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d",
            "Devin acquisition pin mismatch")
    return sha(files[matches[0]])

def audit_full30(blob, source, run, job, manifest):
    """Manifest supplies authenticated archive facts and canonical source texts.

    source={head,checkout,tree}; checkout is the actual PR synthetic checkout.
    Network metadata/run/job/checkout authenticity belongs to a separate wrapper.
    """
    require(isinstance(source, dict) and set(source) == {"head", "checkout", "tree"}
            and source["head"] == HEAD and source["tree"] == TREE
            and isinstance(source["checkout"], str)
            and re.fullmatch("[0-9a-f]{40}", source["checkout"]), "source identity")
    require(type(run) is int and type(job) is int and run > 0 and job > 0
            and run == manifest["run_id"] and job == manifest["job_id"], "run/job binding")
    target, kind = manifest["target"], manifest["kind"]
    require((target, kind) in (("windsurf", "plugin"), ("codex", "plugin"), ("codex", "skill")),
            "unsupported full cohort")
    require(run == (46139632929 if target == "windsurf" else 46139633123),
            "wrong reviewed current run")
    fixtures, constants = authority(manifest)
    files, directories, expanded = zip_files(blob, manifest)
    basename = target + "-" + kind + ".json"
    matches = [name for name in files if name.split("/")[-1] == basename]
    require(len(matches) == 1, "missing/ambiguous cohort report")
    member = matches[0]
    allowed = {"windsurf-plugin.json", "live-audit-report/windsurf-plugin.json"} if target == "windsurf" else {basename}
    require(member in allowed, "unexpected report layout")
    report = document(files[member])
    source_selection(report, fixtures, target, kind, run, source)
    checked = (windsurf_report(report, fixtures, constants) if target == "windsurf"
               else codex_report(report, fixtures, kind))
    if target == "windsurf":
        checked["devin_acquisition_proof_sha256"] = vendor_devin(files)
    inventory = {name: sha(body) for name, body in sorted(files.items())}
    boundaries = [
        "Original archive bytes, CRCs, member hashes, exact30 source order and retained assertions audited.",
        "Run/job/artifact authenticity and clean actual checkout/tree are wrapper responsibilities.",
        "No full native/saved/source filesystem frames are uploaded; no independent frame reconstruction.",
        "Firebase closed runtime tuple, six saved observations, independent256MiB pools and projection "
        "oracle are source-pinned executed assertions; underlying captured frames/oracle inputs absent.",
        "Codex successful app-server transcripts and complete public outputs are not retained; "
        "checkpoint booleans/hashes/tails cannot recreate original RPC or SQLite bytes.",
        "Windsurf active cases retain original nativeBase64/publicstdout; prepared21 public outputs "
        "retain hashes/tails only. Foreign/state/Skill-byte custody remains source-pinned assertions.",
        "No authentication, real-model, MCP tools/call, atomic snapshot or writer-closure claim granted.",
    ]
    return {"audit_kind": "full_cohort_retained_evidence", "status": "passed",
            "source": source, "run_id": run, "job_id": job, "target": target, "kind": kind,
            "archive_sha256": sha(blob), "archive_bytes": len(blob),
            "retained_files": len(files), "directory_entries": directories,
            "expanded_bytes": expanded, "report_member": member,
            "report_sha256": sha(files[member]), "fixture_sha256": FIXTURE_SHA,
            "whole_member_hash_inventory_sha256": digest(inventory, 4 << 20),
            "source_blob_pins": SOURCE_BLOBS, **checked, "boundaries": boundaries}

def independent_source_pin(source, fixtures, constants):
    GLOBAL_SKILL_CASES, PREPARED_GIT_PINS = constants['CASES'], constants['PREPARED_GIT_PINS']
    pin = (GLOBAL_SKILL_CASES.get(source, PREPARED_GIT_PINS.get(source))
           if source.startswith("github:") else fixtures["signed_release_pins"].get(source))
    require(type(pin) is dict)
    return pin


def native_reference(row, source, identity, index, constants):
    ACTIVE_SLOTS, GLOBAL_SKILL_CASES = constants['ACTIVE_SLOTS'], constants['CASES']
    require(index in ACTIVE_SLOTS and identity == ACTIVE_SLOTS[index]
            and source in GLOBAL_SKILL_CASES)
    pin = GLOBAL_SKILL_CASES[source]
    proof = row.get("windsurf_global_native_proof")
    require(type(proof) is dict and proof.get("scenario") == "four-immutable-global-pure-skills"
            and proof.get("source") == source and proof.get("native_discovery_verified") is True
            and type(proof.get("cache_exception_granted")) is bool
            and proof.get("writer_closure_claimed") is False
            and proof.get("reasons") == []
            and encoded(proof.get("independent_source_pin"), 1 << 20) == encoded(pin, 1 << 20))
    owner, stored = proof.get("owner"), proof.get("immutable_source_store")
    require(type(owner) is dict and owner.get("name") == identity
            and owner.get("native_lifecycle_qualified") is True
            and type(owner.get("uuid")) is str and str(UUID(owner["uuid"])) == owner["uuid"]
            and owner.get("revision") == {"version": pin["version"], "resolved_revision": source.rsplit("@", 1)[1],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}
            and row.get("matched_installation_ids") == row.get("repeat_installation_ids") == [owner["uuid"]]
            and type(stored) is dict and stored.get("tree_digest") == pin["tree_digest"]
            and stored.get("complete_skills") == list(pin["skills"])
            and type(stored.get("held_frame")) is dict and stored["held_frame"] == stored.get("last_verified_frame"))
    if "source_skills" in pin:
        require(stored.get("source_skills") == list(pin["source_skills"])
                and stored.get("rejected_skills") == pin["rejected_skills"])
    observations, transitions = proof.get("observations"), proof.get("public_transitions")
    require(type(observations) is list and all(type(v) is dict for v in observations)
            and [v.get("phase") for v in observations]
            == ["before", "add", "repeat", "remove"]
            and all(v.get("verified") is True and type(v.get("custody")) is dict
                    and (v["custody"].get("unchanged") is True
                        and v["custody"].get("before") == v["custody"].get("after")
                        and v["custody"].get("native_cache_only") is False
                         or v["custody"].get("unchanged") is False
                        and v["custody"].get("before") != v["custody"].get("after")
                        and v["custody"].get("native_cache_only") is True
                        and type(v["custody"].get("cache_changed_entries")) is int
                        and 1 <= v["custody"]["cache_changed_entries"] <= 40)
                    for v in observations)
            and type(transitions) is list and all(type(v) is dict for v in transitions)
            and [v.get("phase") for v in transitions]
            == ["add", "repeat", "remove"] and all(v.get("verified") is True for v in transitions))
    cache_observations = [v for v in observations if v["custody"].get("native_cache_only") is True]
    cache_events = proof.get("native_cache_evidence")
    expected_events = [{"phase": observed["phase"], "before": observed["custody"]["before"],
                        "after": observed["custody"]["after"],
                        "changed_entries": observed["custody"]["cache_changed_entries"]}
                       for observed in cache_observations]
    for transition in transitions:
        if "native_cache_custody" in transition:
            event = transition["native_cache_custody"]
            require(type(event) is dict and event.get("phase") == "public-" + transition["phase"]
                    and transition["phase"] in ("add", "repeat"))
            expected_events.append(event)
    require(type(cache_events) is list and len(cache_events) == len(expected_events)
            and proof["cache_exception_granted"] is bool(cache_events)
            and all(type(event) is dict and set(event) == {
                "phase", "before", "after", "changed_entries"}
                and type(event["changed_entries"]) is int and 1 <= event["changed_entries"] <= 40
                and event["before"] != event["after"] for event in cache_events)
            and sorted(cache_events, key=lambda event: event["phase"])
                == sorted(expected_events, key=lambda event: event["phase"]))
    return {"source": source, "installation_id": owner["uuid"], "sha256": digest(proof)}


def windsurf_summary(report, fixtures, constants):
    """Adapted pinned pure report reader; no filesystem custody code."""
    fixture_sha256 = FIXTURE_SHA
    ACTIVE_SLOTS = constants["ACTIVE_SLOTS"]
    COMPONENT_REFUSALS = constants["COMPONENT_REFUSALS"]
    PREPARED, ACTIVE_SKILLS = constants["PREPARED"], constants["ACTIVE_SKILLS"]
    CLI_VERSION, CLIENT_VERSION = constants["CLI_VERSION"], constants["CLIENT_VERSION"]
    result = {"installation_passed": 0, "installation_prepared_lifecycle_passed": 0,
              "qualification_scope": "owned Devin Global installation lifecycle and exact refusals",
              "installation_active_skill_lifecycle_passed": 0,
              "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False,
              "retained_native_acceptance_passed": False,
              "native_runtime_qualified": False, "authentication_qualified": False,
              "model_qualified": False, "mcp_tools_qualified": False}
    try:
        require((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
                == ("windsurf", "plugin", "linux", "x86_64")
                and report.get("cli_version") == {"status": "reported", "value": CLI_VERSION, "exit_code": 0}
                and report.get("client_version") == {"status": "reported", "value": CLIENT_VERSION, "exit_code": 0}
                and type(report.get("client_binary")) is str
                and Path(report["client_binary"]).is_absolute() and Path(report["client_binary"]).name == "devin"
                and type(report.get("cli")) is str and Path(report["cli"]).is_absolute()
                and re.fullmatch(r"[0-9a-f]{64}", fixture_sha256)
                and report.get("fixture_sha256") == fixture_sha256
                and report.get("source_selection_complete") is True and report.get("full_coverage") is True
                and type(report.get("fixture_count")) is int and report["fixture_count"] == 30
                and type(report.get("cases_attempted")) is int and report["cases_attempted"] == 30
                and type(report.get("passed")) is int and 0 <= report["passed"] <= len(ACTIVE_SLOTS)
                and type(report["cli_version"].get("exit_code")) is int
                and type(report["client_version"].get("exit_code")) is int
                and type(report.get("results")) is list
                and len(report["results"]) == 30)
        preserved_native = 0
        for index, row in enumerate(report["results"]):
            require(type(row) is dict)
            source, identity = selected(fixtures, index)
            proof = row.get("windsurf_installation_audit")
            negative = index == 13 or index in COMPONENT_REFUSALS
            require(type(row.get("index")) is int and row["index"] == index
                    and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                    == ("windsurf", "plugin", source, identity)
                    and type(proof) is dict and type(proof.get("fixture_index")) is int
                    and proof["fixture_index"] == index
                    and (proof.get("source"), proof.get("identity"), proof.get("canonical_client_id"), proof.get("scope"))
                    == (source, identity, "windsurf", "user")
                    and proof.get("native_runtime_qualified") is False
                    and proof.get("writer_closure_claimed") is False
                    and proof.get("native_roots") == [".windsurf", ".devin", ".config/devin"]
                    and proof.get("project_unchanged") is True
                    and proof.get("original_native_preserved") is True
                    and proof.get("status") == ("verified-installation-negative" if negative else "verified-installation-lifecycle")
                    and proof.get("public_state") == (None if negative else ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED)
                    and type(proof.get("observations")) is list
                    and len(proof["observations"]) == (2 if negative else 6))
            require(row.get("sentinels_preserved") is True
                    and row.get("native_residuals") == []
                    and all(row.get(key) is False for key in (
                        "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")))
            initial = proof.get("initial_native_image")
            terminal = proof.get("terminal_native_image")
            roles = proof.get("retained_infrastructure")
            mode = proof.get("terminal_mode")
            require(type(roles) is list and roles in (
                [], ["mcp_document", "writer_lock"], ["skills_parent"],
                ["mcp_document", "skills_parent", "writer_lock"]))
            frames = [entry.get("native_image") for entry in proof["observations"] if type(entry) is dict]
            require(len(frames) == (2 if negative else 6))
            for frame in [initial, terminal, *frames]:
                require(type(frame) is dict and set(frame) == {"sha256", "entries"}
                        and type(frame["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", frame["sha256"])
                        and type(frame["entries"]) is int and frame["entries"] > 0)
            require(type(proof.get("initial_saved_authority_sha256")) is str
                    and re.fullmatch(r"[0-9a-f]{64}", proof["initial_saved_authority_sha256"]))
            require(all(type(entry.get("saved_authority_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["saved_authority_sha256"])
                        for entry in proof["observations"]))
            require(all(type(entry.get("json_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["json_sha256"])
                        for entry in proof["observations"]))
            if roles:
                require(mode == "retained-neutral-infrastructure" and not negative
                        and terminal != initial
                        and terminal["entries"] == initial["entries"] + 2 * len(roles)
                        and (index not in ACTIVE_SLOTS or roles == ["skills_parent"]))
            else:
                require(mode == "restored-original-image" and terminal == initial)
            if row.get("status") == "passed":
                require(proof.get("preserved_native_proof") == native_reference(row, source, identity, index, constants)
                        and row["windsurf_global_native_proof"]["binary"]["path"] == report["client_binary"])
                preserved_native += 1
            else:
                require("preserved_native_proof" not in proof)
            if negative:
                require(frames == [initial, initial]
                        and proof.get("native_post_remove_state") == "pristine"
                        and row.get("status") == ("documented-or-source-unsupported" if index == 13 else "install-failed"))
            else:
                retained = proof.get("native_post_remove_state") == "empty-writer-coordination"
                require(proof.get("native_post_remove_state") in ("pristine", "empty-writer-coordination")
                        and retained is bool(roles)
                        and frames[0] == frames[1] == frames[2] == frames[3]
                        and frames[4] == frames[5] == terminal
                        and row.get("status") in ("prepared-lifecycle-passed", "passed")
                        and row.get("prepared_components_only") is True
                        and all(row.get(key) is True for key in (
                            "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                            "removal_verified", "post_list_schema_valid"))
                        and row.get("post_removal_leftovers") == [])
                projection = proof.get("source_projection")
                independent = independent_source_pin(source, fixtures, constants)
                require(type(projection) is dict and set(projection) == {"tree_digest", "manifest_digest", "delivery_digest"}
                        and all(projection.get(key) == independent[key] for key in ("tree_digest", "manifest_digest"))
                        and type(projection.get("delivery_digest")) is str
                        and re.fullmatch(r"sha256:[0-9a-f]{64}", projection["delivery_digest"]))
            result["installation_verified_negative_coverage" if negative else "installation_passed"] += 1
            if not negative:
                result["installation_active_skill_lifecycle_passed" if index in ACTIVE_SLOTS
                       else "installation_prepared_lifecycle_passed"] += 1
        require(report["passed"] == preserved_native)
        require((result["installation_passed"], result["installation_verified_negative_coverage"],
                 result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"])
                == (25, 5, 21, 4))
        result["installation_acceptance_passed"] = True
        result["retained_native_acceptance_passed"] = preserved_native == len(ACTIVE_SLOTS)
    except (ValueError, TypeError, KeyError, RecursionError):
        pass
    return result
