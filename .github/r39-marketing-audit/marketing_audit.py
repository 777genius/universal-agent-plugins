"""Offline Marketing-six retained-byte audit; no product imports or execution."""
import ast
import base64
import hashlib
import io
import json
from pathlib import PurePosixPath as Path
import posixpath
import re
import stat
import unicodedata
import zipfile

HEAD = "fccd72199a1eedc12e3828cf2fdc2f10a2f418f7"
CHECKOUT = "8a1f7d8c87f467e98ca8bfb2ba57fc2a5a16a221"
TREE = "41d82c5eb527f34d16ec2e458194496012bc1c22"
RUN = 46139632998
CLIENTS = ("codex", "cursor", "claude", "gemini", "opencode", "copilot")
ALIASES = frozenset(("marketing-kit", "notion", "hubspot-crm", "playwright", "playwright-bridge"))
NATIVE_TEST = "test_real_native_four_plugin_initialize_retains_only_bound_passive_usage"
REMOVALS = (("remove B", "project"), ("remove A", "project"),
            ("User C remove", "user"), ("User A remove", "user"))
NATIVE_PINS = {"codex": "0.153.4", "claude": "2.1.263", "gemini": "0.66.0",
               "copilot": "1.0.105", "opencode": "1.18.34"}
CURSOR_PINS = {
    "agent_archive_sha256": "85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294",
    "editor_archive_sha256": "8b9198f7dde79f7e27cc83dabf3c66a93ca5303b6919719100c82f29d9498a65",
    "editor_version": "3.22.12",
}
SOURCE_FILES = {
    ".github/workflows/marketing-kit-project-release-e2e.yml": "7d1c957e25d528fddd36734db2b95ee55ccfc994",
    "scripts/claude_cache_retirement_audit.py": "ecb56574b3e892be42cb6c79c3df346268c522c6",
    "scripts/codex_runtime_audit.py": "759b35886a434a7f17e2c36d8fd93d6a12bbe45d",
    "scripts/live-install-audit.py": "ceab34554b6851be09fae9dbc5345fdd753bd0f8",
    "scripts/marketing-kit-project-release-e2e.py": "af2ac520fbd9adee43b411864df73b0ea3b936de",
    "scripts/marketing_kit_claude_discovery.py": "7f1e14c453d7e9f8bcabcfed7bb4343343bf5743",
    "scripts/marketing_kit_claude_retirement.py": "c96774600899745327892d6e24e9a47c68c0ee2a",
    "scripts/marketing_kit_codex_profile.py": "7214510d013d7077fa0cc8adf949ec82778c9574",
    "scripts/native_client_project_artifacts.py": "bfcb69f1393b9b5671062da6e40dce54b3ea7951",
    "scripts/native_client_research.py": "c28e5f88dc26fa58efd45492b41ad899e6335605",
    "scripts/test_live_install_audit_env.py": "1d4c36f2b7e6b64233a037634a644f52013ccf7c",
    "scripts/test_marketing_kit_claude_discovery.py": "1c1d1fa5cde86130ff24dfaf102bcfb8ad8eab58",
    "scripts/test_marketing_kit_claude_retirement.py": "0ad598ed18ba88f3e0589b191b380461c866c7b2",
    "scripts/test_marketing_kit_codex_profile.py": "37159810eb32778cee838d1ec540012c3b2da04a",
    "scripts/test_marketing_kit_codex_sqlite.py": "0f01d6fab0a0fdda7205d418797bafc6a605e947",
    "scripts/test_marketing_kit_copilot_refusal.py": "6f7516aa1e8ec435b572d93190a61ffc57743d55",
    "scripts/test_marketing_kit_peer_initialization.py": "7faacd179627922f0bc48d4a4ea317bafac9d356",
    "scripts/test_marketing_kit_policy_boundary.py": "c17da88d634165cc7de087ae275933ac997f8002"
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






def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)


def hash_object(value):
    return sha(canonical(value).encode())


def literal(text, name):
    matches = [node for node in ast.parse(text).body
               if isinstance(node, ast.Assign) and len(node.targets) == 1
               and isinstance(node.targets[0], ast.Name) and node.targets[0].id == name]
    require(len(matches) == 1, "missing/ambiguous literal authority")
    return ast.literal_eval(matches[0].value)


def authority(manifest):
    sources = manifest["sources"]
    require(isinstance(sources, dict) and set(sources) == set(SOURCE_FILES), "source inventory")
    for path, pin in SOURCE_FILES.items():
        git_blob(sources[path], pin)
    closure = literal(sources["scripts/live-install-audit.py"], "MARKETING_KIT_SIGNED_CLOSURE")
    require(isinstance(closure, dict) and len(closure) == 4, "signed closure authority")
    # Only AST data is inspected: no imported unittest module, loader or product code.
    counts = {}
    for path in SOURCE_FILES:
        if "/test_" not in path:
            continue
        names = []
        for node in ast.parse(sources[path]).body:
            if isinstance(node, ast.ClassDef):
                names.extend(child.name for child in node.body
                             if isinstance(child, ast.FunctionDef)
                             and child.name.startswith("test_"))
        if path.endswith("test_live_install_audit_env.py"):
            names = [name for name in names if "marketing_kit_" in name]
        require(len(names) == len(set(names)), "ambiguous selected unittest names")
        counts[path] = len(names)
    expected = {
        "scripts/test_marketing_kit_claude_retirement.py": 78,
        "scripts/test_marketing_kit_claude_discovery.py": 11,
        "scripts/test_live_install_audit_env.py": 174,
        "scripts/test_marketing_kit_codex_sqlite.py": 5,
        "scripts/test_marketing_kit_codex_profile.py": 12,
        "scripts/test_marketing_kit_policy_boundary.py": 4,
        "scripts/test_marketing_kit_copilot_refusal.py": 21,
        "scripts/test_marketing_kit_peer_initialization.py": 2,
    }
    require(counts == expected, "canonical preflight selection changed")
    return closure, counts


def preflight(files, client):
    retirement = files["claude-retirement-regressions.log"].decode("utf-8")
    regression = files["regressions.log"].decode("utf-8")

    def groups(text):
        require(not re.search(r"^(?:FAILED\b|FAIL:|ERROR:|Traceback \(most recent call last\):)",
                              text, re.M), "preflight failed")
        summaries = re.findall(
            r"^Ran (\d+) tests? in [0-9.]+s\r?\n\r?\nOK(?: \(skipped=(\d+)\))?\r?$",
            text, re.M)
        return [(int(count), int(skipped or 0)) for count, skipped in summaries]

    first = groups(retirement)
    require(len(first) == 1 and first[0][0] == 78, "retirement preflight incomplete")
    native_lines = [line for line in retirement.splitlines()
                    if line.startswith(NATIVE_TEST + " (")]
    require(len(native_lines) == 1, "genuine Claude fixture result missing")
    if client == "claude":
        require(first[0][1] == 0 and native_lines[0].endswith(" ... ok"),
                "Claude lane did not execute its genuine private native fixture")
        native = "passed"
    else:
        require(first[0][1] in (0, 1), "unexpected retirement skips")
        if first[0][1]:
            require(native_lines[0].endswith(" ... skipped 'pinned native Claude unavailable'")
                    and sum(" ... skipped " in line for line in retirement.splitlines()) == 1,
                    "unqualified retirement skip")
            native = "expected_non_Claude_lane_skip"
        else:
            require(native_lines[0].endswith(" ... ok"), "native fixture did not pass")
            native = "passed_as_preflight_only"
    expected = [174, 5] + ([89] if client == "claude" else []) + [12, 4, 21, 2]
    observed = groups(regression)
    require(observed == [(count, 0) for count in expected], "regression preflight incomplete")
    return {"tests_attempted": 78 + sum(expected), "tests_skipped": first[0][1],
            "genuine_Claude_fixture": native,
            "retirement_log_sha256": sha(files["claude-retirement-regressions.log"]),
            "regression_log_sha256": sha(files["regressions.log"])}


def policy(report, closure):
    value = report.get("positive_policy")
    require(isinstance(value, dict) and value.get("source_mode") == "production_default"
            and type(value.get("snapshot_sequence")) is int and value["snapshot_sequence"] > 0
            and isinstance(value.get("snapshot_digest"), str)
            and re.fullmatch("sha256:[0-9a-f]{64}", value["snapshot_digest"])
            and canonical(value.get("closure")) == canonical(closure), "positive signed policy")
    final = report.get("final_policy_observation")
    require(isinstance(final, dict) and final.get("status") == "passed"
            and final.get("source_profile") == "E"
            and final.get("profile_usage") == "never_installed"
            and canonical(final.get("policy")) == canonical(value), "final never-installed policy")
    return value


def steps(report, client):
    expected = [
        "add", "repeat add", "update", "repair intact",
        "corrupt Project signed lock refused before install",
        "cold locked restore", "remove B", "remove A",
        "User add", "User repeat add", "User update", "User repair intact",
        "corrupt User signed lock refused before install", "User cold locked restore C",
        "User remove C", "User remove",
    ]
    if client == "codex":
        expected.insert(expected.index("User remove C"), "User C native HubSpot uninstall")
        expected.insert(expected.index("User remove"), "User native HubSpot uninstall")
    if client == "copilot":
        expected.insert(4, "cold Copilot Project folder consent refused")
        compatibility = report["positive_policy"]["root_target_compatibility"]
        contract = ("saved_management_capability" if compatibility.get("vscode") ==
                    {"scopes": ["user"], "delivery": "prepared"} else "signed_grant_refusal")
        expected.append("Current-production VS Code User boundary: " + contract)
    values = report.get("steps")
    require(isinstance(values, list) and all(isinstance(row, dict) for row in values)
            and [row.get("stage") for row in values] == expected, "lifecycle stage order")
    negative = {"corrupt Project signed lock refused before install",
                "corrupt User signed lock refused before install"}
    for row in values:
        refusal = row["stage"].startswith(("cold Copilot", "Current-production"))
        code = 1 if row["stage"] in negative or refusal else 0
        require(type(row.get("exit_code")) is int and row["exit_code"] == code
                and row.get("result") == ("expected_refusal" if refusal else "passed"),
                "lifecycle stage result")
    return len(values)


def owned(name):
    return name.split("@", 1)[0].split(":", 1)[0].casefold() in ALIASES


def registrations(body):
    rows = document(body)
    require(isinstance(rows, list), "Claude registrations not a list")
    seen = set()
    for row in rows:
        require(isinstance(row, dict) and isinstance(row.get("id"), str) and "@" in row["id"]
                and row.get("scope") in ("user", "project", "local")
                and type(row.get("enabled")) is bool and isinstance(row.get("installPath"), str)
                and Path(row["installPath"]).is_absolute(), "Claude registration shape")
        key = (row["id"], row["scope"], row["installPath"])
        require(key not in seen, "duplicate native registration")
        seen.add(key)
    return rows


def commands(body):
    value = document(body)
    require(isinstance(value, dict) and value.get("type") == "control_response",
            "Claude initialize response")
    response = value.get("response", {})
    require(response.get("request_id") == "native-discovery"
            and response.get("subtype") == "success", "Claude initialize request join")
    result = response.get("response", {})
    require(result.get("account", {}).get("tokenSource") == "none"
            and isinstance(result.get("commands"), list), "Claude credential-free commands")
    found = {}
    for command in result["commands"]:
        require(isinstance(command, dict) and isinstance(command.get("name"), str)
                and command["name"] and command["name"] not in found,
                "duplicate/malformed Claude command")
        found[command["name"]] = command
    return found


def claude(report, closure):
    flags = report.get("native_removal_qualification", {})
    require(flags.get("status") ==
            "registration-and-skill-absence-qualified; effective-MCP-absence-unqualified"
            and flags.get("plugin_registration_absence_qualified") is True
            and flags.get("skill_discovery_absence_qualified") is True
            and all(flags.get(key) is False for key in (
                "full_native_removal_qualified", "physical_absence_qualified",
                "writer_closure_qualified", "mcp_discovery_absence_qualified")),
            "Claude qualification boundary")
    captures, expected = report.get("claude_native_observations"), []
    for index, (stage, _) in enumerate(REMOVALS):
        for when in ("before", "after"):
            expected.extend(stage + "-" + when + "-" + kind
                            for kind in ("version", "registration", "skills"))
        if index == 0:
            expected.append(stage + "-mcp-observational")
    require(isinstance(captures, list) and [row.get("probe") for row in captures] == expected,
            "Claude original capture coverage/order")
    records, decoded, hashes = {}, {}, []
    fields = ("probe", "exit", "error", "stdout_bytes", "stderr_bytes",
              "stdout_sha256", "stderr_sha256")
    for row in captures:
        name = row["probe"]
        require(row.get("native_absence_qualified") is False, "capture flag overclaim")
        streams = {}
        for key in ("stdout", "stderr"):
            encoded = row.get(key + "_base64")
            require(isinstance(encoded, str) and len(encoded) <= 1400000,
                    "Claude Base64 stream bound")
            raw = base64.b64decode(encoded, validate=True)
            require(len(raw) <= 1 << 20 and type(row.get(key + "_bytes")) is int
                    and row[key + "_bytes"] == len(raw)
                    and row.get(key + "_sha256") == sha(raw), "Claude original stream join")
            streams[key] = raw
            hashes.append(sha(raw))
        observational = name.endswith("-mcp-observational")
        if not observational:
            require(type(row.get("exit")) is int and row["exit"] == 0
                    and row.get("error") is None and streams["stderr"] == b"",
                    "Claude genuine process failure")
        if name.endswith("-version"):
            require(streams["stdout"].strip() == b"2.1.263 (Claude Code)",
                    "wrong original native Claude version")
        records[name] = {key: row[key] for key in fields}
        decoded[name] = streams["stdout"]
    proofs = report.get("claude_fresh_removal_discovery")
    require(isinstance(proofs, list) and [(row.get("stage"), row.get("scope")) for row in proofs]
            == list(REMOVALS), "four fresh Claude removal checkpoints")
    binary = set()
    for proof, (stage, scope) in zip(proofs, REMOVALS):
        require(proof.get("status") == "verified-registration-and-skill-absence"
                and all(proof.get(key) is True for key in (
                    "plugin_registration_absence_qualified", "skill_discovery_absence_qualified",
                    "foreign_native_inventory_preserved"))
                and all(proof.get(key) is False for key in (
                    "mcp_discovery_absence_qualified", "physical_absence_qualified",
                    "writer_closure_qualified", "full_native_removal_qualified")),
                "Claude removal proof bounds")
        binary.add(hex_digest(proof.get("native_executable_sha256")))
        names = [stage + "-" + when + "-" + kind for when in ("before", "after")
                 for kind in ("registration", "skills")]
        require(proof.get("observations") == [records[name] for name in names],
                "Claude short-record/original capture join")
        before, after = registrations(decoded[names[0]]), registrations(decoded[names[2]])
        selected = [row for row in before if owned(row["id"])]
        require(len(selected) == 4 and {row["id"].split("@", 1)[0] for row in selected}
                == {"marketing-kit", "notion", "hubspot-crm", "playwright"}
                and all(row["scope"] == scope for row in selected),
                "Claude pre-remove positive registration control")
        first, last = commands(decoded[names[1]]), commands(decoded[names[3]])
        require("marketing-kit:campaign-planning" in first
                and first["marketing-kit:campaign-planning"].get("builtin") is not True,
                "Claude pre-remove positive Skill control")
        require(not any(owned(row["id"]) for row in after)
                and not any(owned(name) for name in last)
                and canonical(after) == canonical([row for row in before if not owned(row["id"])])
                and canonical(last) == canonical({name: row for name, row in first.items()
                                                 if not owned(name)}),
                "Claude original removal/foreign inventory mismatch")
    require(len(binary) == 1 and len(hashes) == 50, "Claude executable/capture count")
    cleanups = report.get("native_cleanup_proofs")
    require(isinstance(cleanups, list) and [row.get("stage") for row in cleanups]
            == ["remove B", "remove A", "User A remove"], "Claude retirement proof coverage")
    for index, proof in enumerate(cleanups):
        require(proof.get("status") == (
            "verified-signed-closure-cache-retirement-at-checkpoint" if index < 2 else
            "verified-unchanged-signed-project-cache-retirement-at-user-removal")
            and proof.get("scope") == "project" and proof.get("retired_bytes_present") is True
            and all(proof.get(key) is False for key in (
                "writer_closure_claimed", "physical_absence_qualified",
                "writer_closure_qualified", "full_native_removal_qualified",
                "managed_digest_independently_recomputed_by_harness"))
            and proof.get("managed_digest_validation_authority") == "genuine-published-cli-remove"
            and proof.get("custody_validation_authority") == "held-descriptor-image-and-namespace-equality"
            and isinstance(proof.get("raw_native_residuals"), list)
            and type(proof.get("matched_residual_count")) is int
            and proof["matched_residual_count"] == len(proof["raw_native_residuals"]) > 0,
            "Claude retained-cache qualification")
        usage = proof.get("passive_plugin_usage", {})
        require(usage.get("native_version") == "2.1.263"
                and type(usage.get("receipt_bound_key_count")) is int
                and usage["receipt_bound_key_count"] in (0, 4)
                and usage.get("foreign_document_equality") == "strict-decoded-json"
                and usage.get("preserved_foreign_document") is True
                and usage.get("post_removal_bytes_and_identity_held") is True,
                "Claude passive usage custody assertion")
        if index == 2:
            transition = usage.get("user_initialize_transition", {})
            require(transition.get("receipt_bound_user_key_count") == 4
                    and transition.get("original_project_usage_contract_preserved") is True
                    and transition.get("exact_four_user_remove_receipts_qualified") is True
                    and transition.get("full_native_removal_qualified") is False
                    and transition.get("writer_closure_qualified") is False
                    and set(transition.get("user_owners", {})) ==
                    {"marketing-kit", "notion", "hubspot-crm", "playwright"},
                    "Claude four original User removal receipts")
            for owner in transition["user_owners"].values():
                hex_digest(owner.get("installation_id_sha256"))
                hex_digest(owner.get("binding_id_sha256"))
        if "user_initialize_backups" in usage:
            backup = usage["user_initialize_backups"]
            require(backup.get("full_bytes_equal_to_authorized_image") is True
                    and backup.get("post_action_namespace_and_original_descriptor_held") is True
                    and backup.get("writer_closure_qualified") is False
                    and backup.get("physical_absence_qualified") is False,
                    "Claude optional backup custody bounds")
        if index < 2:
            members = proof.get("members")
            require(isinstance(members, list) and len(members) == 4
                    and {member.get("distribution") for member in members} == set(closure),
                    "Claude retired signed closure")
            for member in members:
                require(canonical(member.get("signed_release")) ==
                        canonical(closure[member["distribution"]])
                        and type(member.get("file_count")) is int and member["file_count"] > 0
                        and type(member.get("bytes")) is int and member["bytes"] > 0,
                        "Claude retirement source/count assertion")
                hex_digest(member.get("snapshot_equality_sha256"))
    return {"original_native_captures": 25, "original_native_streams": 50,
            "original_native_stream_hashes_sha256": hash_object(hashes),
            "four_fresh_removal_checkpoints": True, "native_executable_sha256": next(iter(binary)),
            "private_0600_custody": "source-pinned executed assertions; descriptor images not uploaded",
            "MCP_observation": "one original observational capture; absence unqualified"}


def copilot(report, closure):
    require(report.get("cold_native_restore_qualified") is False
            and report.get("copilot_negative_cold_profile") == "D", "Copilot cold boundary")
    refusal = next(row for row in report["steps"]
                   if row["stage"] == "cold Copilot Project folder consent refused")
    require(refusal.get("reason") == "missing_remembered_folder_consent"
            and type(refusal.get("bindings")) is int and refusal["bindings"] == 4
            and refusal.get("cold_native_restore_qualified") is False, "cold closure refusal")
    trust = report.get("copilot_project_b_trust_fixture", {})
    require(trust.get("status") == "authored-native-prerequisite"
            and trust.get("source_profile") == "B"
            and trust.get("restore_scope") == "cold-UAP-state-with-preauthorized-native-folder"
            and trust.get("uap_state_empty_before") is True
            and all(trust.get(key) is False for key in (
                "virgin_native_trust_qualified", "ui_consent_observed", "product_trust_grant_claimed")),
            "Copilot positive B prerequisite boundary")
    folders = trust.get("trusted_folders")
    require(isinstance(folders, list) and len(folders) == 1
            and isinstance(folders[0], str) and folders[0].startswith("/")
            and posixpath.normpath(folders[0]) == folders[0]
            and folders[0].endswith("/B/project")
            and trust.get("config_path") == folders[0][:-len("/project")] + "/home/.copilot/config.json"
            and trust.get("config_identity_at_creation", {}).get("mode") == 0o600,
            "Copilot authored trust coordinate")
    root = folders[0][:-len("/B/project")]
    require(posixpath.basename(root).startswith("uap-real-marketing-kit-")
            and len(posixpath.basename(root)) > len("uap-real-marketing-kit-"),
            "Copilot source-authorized case family")
    # Recompute the exact authored JSON bytes; no on-disk config is retained.
    body = (json.dumps({"trustedFolders": folders}, sort_keys=True) + "\n").encode()
    require(trust.get("config_sha256_at_creation") == sha(body), "Copilot authored trust hash")
    expected = {"scope": "user", "logical_owner": "copilot", "packages": sorted(closure),
                "affected_surfaces": ["copilot", "vscode"], "separate_vscode_binding": False,
                "native_editor_runtime": "not_qualified"}
    require(report.get("logical_owner_evidence") == expected, "Copilot logical owner")
    compatibility = report["positive_policy"]["root_target_compatibility"]
    grants = {client: {"scopes": ["user"], "delivery": "managed"} for client in CLIENTS}
    require(isinstance(compatibility, dict) and all(compatibility.get(k) == v for k, v in grants.items())
            and all(grant == {"scopes": ["user"], "delivery": "managed"}
                    for client, grant in compatibility.items() if client not in grants and client != "vscode"),
            "Copilot positive policy grant shape")
    expected_contract = ("signed_grant_refusal" if "vscode" not in compatibility else
                         "saved_management_capability")
    if "vscode" in compatibility:
        require(compatibility["vscode"] == {"scopes": ["user"], "delivery": "prepared"},
                "VS Code prepared grant")
    boundary = report.get("current_vscode_user_boundary", {})
    require(boundary.get("status") == "refused_as_expected"
            and boundary.get("contract") == expected_contract
            and boundary.get("reason") == ("missing_vscode_grant" if expected_contract ==
                                          "signed_grant_refusal" else "unsupported_saved_target")
            and canonical(boundary.get("policy")) == canonical(report["positive_policy"]),
            "current policy VS Code refusal")
    return {"cold_D_expected_refusal_bindings": 4, "cold_native_restore_qualified": False,
            "B_restore": "explicitly preauthorized native folder, not virgin trust",
            "cold_runtime_link_custody": "source-pinned regression and lifecycle assertions only",
            "original_cold_refusal_JSON_uploaded": False}


def codex(report):
    maps = ("codex_runtime_profiles", "codex_sqlite_profiles")
    for family in maps:
        values = report.get(family)
        require(isinstance(values, dict) and set(values) == {"A", "B", "C"},
                "Codex A/B/C profile coverage")
        for name, proof in values.items():
            require(proof.get("native_version") == "0.153.4"
                    and proof.get("runtime_root") == "home/.local/state/codex-sqlite"
                    and proof.get("runtime_semantics_unchanged") is True
                    and proof.get("final_checkpoint") == "passed"
                    and proof.get("failed_phase") is None
                    and isinstance(proof.get("runtime_baseline"), dict)
                    and isinstance(proof.get("observations"), list)
                    and proof["observations"][0].get("phase") == "before-install"
                    and proof["observations"][-1].get("phase") == "final-lifecycle"
                    and any(row.get("phase") == "final-qualification" for row in proof["observations"]),
                    "Codex final runtime assertion")
            for observed in proof["observations"]:
                require(isinstance(observed.get("phase"), str)
                        and isinstance(observed.get("runtime_files_fully_read"), dict),
                        "Codex retained checkpoint shape")
                for path, fact in observed["runtime_files_fully_read"].items():
                    require(isinstance(path, str) and "/" not in path and "\\" not in path
                            and isinstance(fact, dict) and type(fact.get("bytes")) is int
                            and 0 <= fact["bytes"] <= 32 << 20, "Codex retained file count")
                    hex_digest(fact.get("sha256"))
    return {"profile_families": list(maps), "profiles_each": ["A", "B", "C"],
            "native_runtime_version_assertion": "0.153.4", "original_RPC_uploaded": False}


def audit_marketing(blob, source, run, job, manifest):
    """Exact R39 PR scope; no generic future-main/source fallback."""
    require(source == {"head": HEAD, "checkout": CHECKOUT, "tree": TREE}, "wrong source")
    require(type(run) is int and run == RUN and type(job) is int and job > 0
            and manifest["run_id"] == run and manifest["job_id"] == job,
            "run/job binding")
    client = manifest["client"]
    require(client in CLIENTS, "unknown client")
    closure, _ = authority(manifest)
    files, directories, expanded = zip_files(blob, manifest)
    required = {"report.json", "claude-retirement-regressions.log", "regressions.log"}
    require(required <= set(files), "missing retained report/preflight")
    report = document(files["report.json"])
    require(report.get("cli_version") == "0.1.116" and report.get("target") == client
            and report.get("source") == "real signed marketing-kit"
            and report.get("status") == "passed" and "failure" not in report
            and report.get("two_scope_lifecycle_qualified") is True
            and all(report.get("scope_results", {}).get(scope, {}).get("expectation") == "positive_lifecycle"
                    and report["scope_results"][scope].get("status") == "passed"
                    for scope in ("project", "user")), "Marketing lifecycle incomplete")
    expected_scope = ("user_and_project_installation_lifecycle_with_retained_Claude_bytes"
                      if client == "claude" else "user_and_project")
    require(report.get("qualification_scope") == expected_scope, "qualification scope")
    selected_policy = policy(report, closure)
    stage_count = steps(report, client)
    tests = preflight(files, client)
    proof = (claude(report, closure) if client == "claude" else
             copilot(report, closure) if client == "copilot" else
             codex(report) if client == "codex" else {})
    require(report.get("historical_policy_results", {}).get("signed45") ==
            {"status": "separate_point_in_time_audit", "required_for_current_qualification": False},
            "historical policy must remain separate")
    if client != "copilot":
        require(report.get("current_vscode_user_boundary", {}).get("status") == "not_applicable",
                "unrelated boundary promoted")
    boundaries = [
        "Archive CRC/size/SHA, all retained file hashes, original preflight logs and report audited.",
        "Report has no source-commit field: authenticated job/checkout/source binding is wrapper duty.",
        "Published UAP0.1.116 acquisition/version and signed policy authentication are source-executed "
        "assertions; release binaries, catalog signature bytes and complete CLI outputs are not uploaded.",
        "Only Claude embeds original native streams. Other stages/checkpoint hashes are not complete "
        "CLI/RPC/FS reconstruction. Source/private0600/foreign/state/peer custody remains assertions.",
        "Claude initialization uses strict empty MCP; one effective MCP listing is observational only.",
        "Copilot coldDrefusal is compact result evidence; full refusal JSON and permitted runtime-link "
        "FS custody are source-pinned assertions, B positive restore has preauthorized native trust.",
        "Workflow provisioning pins do not independently prove native binary bytes for every client. "
        "Cursor Agent+Editor hashes are checked by workflow; GUI/IDE execution remains unqualified.",
        "No vendor-authentication, real-model, tools/call, physical Claude absence or writer-closure "
        "qualification is added by this audit.",
    ]
    return {"audit_kind": "Marketing_six_retained_evidence", "status": "passed",
            "source": source, "run_id": run, "job_id": job, "client": client,
            "published_UAP": "0.1.116", "native_provisioning_pin": NATIVE_PINS.get(client),
            "cursor_provisioning_pins": CURSOR_PINS if client == "cursor" else None,
            "archive_sha256": sha(blob), "archive_bytes": len(blob), "retained_files": len(files),
            "directory_entries": directories, "expanded_bytes": expanded,
            "whole_member_hash_inventory_sha256": hash_object(
                {name: sha(body) for name, body in sorted(files.items())}),
            "report_sha256": sha(files["report.json"]), "lifecycle_stages": stage_count,
            "signed_closure_sha256": hash_object(closure),
            "policy_snapshot_sequence": selected_policy["snapshot_sequence"],
            "policy_snapshot_digest": selected_policy["snapshot_digest"],
            "source_blob_pins": SOURCE_FILES, "preflight": tests, **proof, "boundaries": boundaries}
