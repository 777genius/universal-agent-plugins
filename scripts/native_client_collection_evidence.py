"""Offline retained-byte auditor for the pinned current-source native collection lanes.

No product imports, execution, network access, extraction, or source writes.
The caller must authenticate run/job/artifact metadata and source-file custody.

Preserved from the PR #1130 auxiliary auditor at commit
05c8051785ea3f5f8af0a29a8881997f0ccbd5a1 (native_audit.py, blob
95e43720d4a8a2344a2242cc62adb8ae69b72c89). This is a historical, exact-source
qualification tool, not a claim about a newer source or released binary.
"""
import ast
import hashlib
import io
import json
import ntpath
import posixpath
import re
import stat
import unicodedata
import zipfile

COMMIT = "fccd72199a1eedc12e3828cf2fdc2f10a2f418f7"
TREE = "41d82c5eb527f34d16ec2e458194496012bc1c22"
RUNNER = "6036bbdfd219e15a9819995fa1f57ff0608530ab"
WORKFLOW = "37112c3561d51f298c10368dd4530e0db0b7dca3"
SKILLS = ("benchmark-note", "benchmark-secondary")
STAGES = ("local_collection_all_install", "local_collection_all_repeat",
          "local_collection_all_remove")
CLAUDE_ARGV = ["-p", "--input-format", "stream-json", "--output-format",
              "stream-json", "--verbose", "--permission-mode", "dontAsk",
              "--strict-mcp-config", "--mcp-config", '{"mcpServers":{}}']
CLAUDE_REQUEST = {"type": "control_request", "request_id": "native-discovery",
                  "request": {"subtype": "initialize"}}


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def digest(value):
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


def authorities(manifest):
    text = manifest["runner_source"]
    git_blob(text, RUNNER)
    git_blob(manifest["workflow_source"], WORKFLOW)
    needed = {"PINS", "CODEX_SOURCE_PINS", "REQUIRED_TESTS", "PROJECT_TESTS",
              "BROWSER_PROJECT_TEST"}
    values = {}
    for node in ast.parse(text).body:
        if isinstance(node, ast.Assign) and len(node.targets) == 1:
            target = node.targets[0]
            if isinstance(target, ast.Name) and target.id in needed:
                require(target.id not in values, "duplicate source authority")
                values[target.id] = ast.literal_eval(node.value)
    require(set(values) == needed, "incomplete literal source authority")
    return values


def zip_files(blob, manifest):
    require(isinstance(blob, bytes) and len(blob) <= 256 << 20, "archive size")
    require(len(blob) == manifest["archive_size"], "archive metadata size")
    require(sha(blob) == digest(manifest["archive_sha256"]), "archive digest")
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
    require("runner-evidence.json" in files and "native-tests.log" in files,
            "missing runner evidence")
    runner = document(files["runner-evidence.json"])
    actual = {name: sha(body) for name, body in files.items()
              if name != "runner-evidence.json"}
    require(runner.get("artifact_sha256") == actual, "whole retained inventory mismatch")
    return files, runner, len(directories), total


def asset(observed, pin):
    kind, repository, version, archive, integrity, _ = pin
    url = ("https://github.com/" + repository + "/releases/tag/" + version
           if kind == "github" else
           "https://registry.npmjs.org/" + repository + "/-/" + archive)
    require(isinstance(observed, dict), "missing tool asset")
    for key, expected in (("source", url), ("version", version),
                          ("archive", archive), ("archive_integrity", integrity)):
        require(observed.get(key) == expected, "tool acquisition pin mismatch: " + key)
    digest(observed.get("binary_sha256"))


def runner_checks(runner, files, source, client, target, authority):
    require(source == {"commit": COMMIT, "tree": TREE}, "unsupported reviewed source")
    require(runner.get("schema_version") == 1 and runner.get("status") == "passed"
            and type(runner.get("exit_code")) is int and runner["exit_code"] == 0,
            "runner did not pass")
    require(runner.get("commit") == COMMIT and runner.get("tree") == TREE
            and runner.get("client") == client and runner.get("target") == target
            and runner.get("project_components") is True, "runner identity mismatch")
    require("installer_release" not in runner and
            not runner.get("project_artifact_admission_errors"), "wrong source lane")
    pins = authority["PINS"][target]
    selected = authority["CODEX_SOURCE_PINS"][target] if client == "codex" else pins[client]
    asset(runner.get("client_asset"), selected)
    asset(runner.get("scanner_asset"), pins["lintai"])
    asset(runner.get("ripgrep_asset"), pins["rg"])
    if client == "codex":
        asset(runner.get("dual_peer_asset"), pins["claude"])
    digest(runner.get("installer_sha256"))
    suffix = ".exe" if target.startswith("windows-") else ""
    require(isinstance(runner.get("harness_build_sha256"), dict)
            and set(runner["harness_build_sha256"]) ==
            {"native-probe" + suffix, "repotests" + suffix}, "harness identities missing")
    for value in runner["harness_build_sha256"].values():
        digest(value)
    required = authority["REQUIRED_TESTS"][client] | authority["PROJECT_TESTS"][client]
    if client == "codex" and target == "linux-amd64":
        required = required | {authority["BROWSER_PROJECT_TEST"]}
        require(isinstance(runner.get("browser_skills"), dict), "browser lane omitted")
    require(runner.get("required_tests") == sorted(required), "required selectors differ")
    log = files["native-tests.log"].decode("utf-8")
    require(not re.search(r"^\s*--- (?:FAIL|SKIP):", log, re.M)
            and not re.search(r"^(?:FAIL(?:\s|$)|panic:|fatal error:)", log, re.M),
            "native transcript failure or skip")
    passed = re.findall(r"^--- PASS: (\w+) \(", log, re.M)
    require(len(passed) == len(set(passed)) and set(passed) == required,
            "native transcript required tests incomplete/ambiguous")
    require(runner.get("passed_tests") == sorted(required)
            and runner.get("skipped_tests") == [], "runner test summary mismatch")
    command = runner.get("test_command")
    pattern = "^(?:" + "|".join(re.escape(name) for name in sorted(required)) + ")$"
    require(isinstance(command, list) and len(command) == 4
            and command[1:] == ["-test.v", "-test.timeout=15m", "-test.run=" + pattern],
            "native test command mismatch")
    return selected


def fixture_report(files, runner, client, target, selected):
    basename = ("claude-project-plugin-evidence.json" if client == "claude"
                else "codex-project-components-evidence.json")
    family = "uap-claude-native-evidence-" if client == "claude" else "uap-native-evidence-"
    candidates = []
    for name in files:
        if name.split("/")[-1] != basename:
            continue
        parts = name.split("/")
        require(len(parts) == 2 and parts[0].startswith(family)
                and len(parts[0]) > len(family), "wrong fixture family")
        report = document(files[name])
        require(isinstance(report, dict) and isinstance(report.get("stages"), dict),
                "malformed same-basename native report")
        # Trusted suites share this basename. Select by all collection keys,
        # never by status: a failed duplicate must also make selection ambiguous.
        if set(STAGES).issubset(report["stages"]):
            candidates.append((name, report))
    require(len(candidates) == 1, "missing or ambiguous collection report")
    name, report = candidates[0]
    parts = name.split("/")
    os_name, arch = target.split("-")
    require(report.get("client") == client and report.get("os") == os_name
            and report.get("arch") == arch, "fixture platform mismatch")
    identity = {"installer_base_commit": COMMIT, "installer_tree": TREE,
                "installer_source_state": "committed", "installer_patch_sha256": []}
    require(report.get("source_identity") == identity, "fixture source mismatch")
    version = ("codex-cli " + selected[2].removeprefix("rust-v") if client == "codex"
               else selected[2] + " (Claude Code)")
    require(report.get("client_version_measured") == version
            and report.get("client_sha256") == runner["client_asset"]["binary_sha256"]
            and report.get("installer_sha256") == runner["installer_sha256"],
            "fixture binary identity mismatch")
    scanner = report.get("scanner", {})
    require(scanner.get("version") == "0.1.3"
            and scanner.get("binary_sha256") == runner["scanner_asset"]["binary_sha256"]
            and scanner.get("archive_sha256") ==
            runner["scanner_asset"]["archive_integrity"].removeprefix("sha256:")
            and scanner.get("platform") == target, "fixture scanner mismatch")
    if client == "claude":
        require(report.get("project_contract") == "project_marketplace_installation_v1",
                "wrong Claude project contract")
    for stage in STAGES:
        require(report.get("stages", {}).get(stage, {}).get("status") == "passed",
                "collection stage not passed: " + stage)
    hashes = report.get("artifact_sha256")
    require(isinstance(hashes, dict) and hashes, "fixture inventory missing")
    prefix = parts[0] + "/"
    actual = {path[len(prefix):]: sha(body) for path, body in files.items()
              if path.startswith(prefix) and "/" not in path[len(prefix):]
              and path != name}
    require(hashes == actual, "fixture retained inventory mismatch")
    return parts[0], report


def coordinates(value, target):
    require(isinstance(value, str) and 0 < len(value) <= 8192
            and not any(ord(c) < 32 or ord(c) == 127 for c in value),
            "bad native coordinate")
    path = ntpath if target.startswith("windows-") else posixpath
    require(path.isabs(value), "nonabsolute native coordinate")
    if path is ntpath:
        require(bool(ntpath.splitdrive(value)[0]), "drive-relative native coordinate")
    return path


def same_path(a, b, target):
    path = coordinates(a, target)
    coordinates(b, target)
    left, right = path.normpath(a), path.normpath(b)
    if path is ntpath:
        left, right = left.replace("\\", "/"), right.replace("\\", "/")
        # Go EqualFold uses simple Unicode folding, not multi-character
        # expansions such as sharp-s -> ss. Preserve one character per input.
        def simple_fold(text):
            result = []
            for char in text:
                folded = char.casefold()
                lower = char.lower()
                result.append(folded if len(folded) == 1 else
                              lower if len(lower) == 1 else char)
            return "".join(result)
        return simple_fold(left) == simple_fold(right)
    # Go filepath.Clean folds repeated leading Unix separators as well.
    left, right = "/" + left.lstrip("/"), "/" + right.lstrip("/")
    return left == right


def installer_members(files, directory, client):
    prefix, required = directory + "/", set()

    def read(label):
        names = [label + "-installer.json", label + "-installer-stderr.log"]
        for name in names:
            require(prefix + name in files, "missing collection installer file: " + name)
            required.add(prefix + name)
        result = document(files[prefix + names[0]])
        require(isinstance(result, dict), "installer result object missing")
        return result

    listing = read("local-collection-list")
    require(listing.get("result") == "success", "listing failed")
    data = listing.get("data", {})
    require(data.get("complete") is True and data.get("scope") == "project",
            "listing incomplete/wrong scope")
    candidates = data.get("candidates")
    require(isinstance(candidates, list) and len(candidates) == 2, "listing count")
    names = []
    for candidate in candidates:
        require(isinstance(candidate, dict) and candidate.get("status") == "available"
                and candidate.get("path") == candidate.get("name"), "listing candidate")
        names.append(candidate.get("name"))
    require(sorted(names) == list(SKILLS), "listing selection mismatch")
    for stage in STAGES[:2]:
        result = read(stage)
        require(result.get("status") == "succeeded" and result.get("readiness") == "ready"
                and isinstance(result.get("items"), list) and len(result["items"]) == 2,
                "collection install/repeat not ready")
    for skill in SKILLS:
        label = ("local_collection_all_remove-" if client == "claude"
                 else "local-collection-remove-") + skill
        require(read(label).get("result") == "success", "collection removal failed")
    return data.get("source"), required


def claude_names(collection, target):
    path = coordinates(collection, target)
    canonical = path.normpath(collection)
    if path is ntpath:
        canonical = canonical.replace("\\", "/")
        original = collection.replace("\\", "/")
    else:
        original = collection
    # The listing source is produced by filepath.Abs, not a native response.
    # Keep its original case; never fold ownership hashes for Windows paths.
    require(canonical == original, "listing source is not canonically serialized")
    names = set()
    for skill in SKILLS:
        source = canonical + "/" + skill
        text = json.dumps(["local-skill-v1", source, skill],
                          ensure_ascii=False, separators=(",", ":"))
        # Match encoding/json Marshal HTML and line-separator escaping.
        for before, after in (("&", "\\u0026"), ("<", "\\u003c"),
                              (">", "\\u003e"), ("\u2028", "\\u2028"),
                              ("\u2029", "\\u2029")):
            text = text.replace(before, after)
        body = text.encode("utf-8")
        key = "local-skill-v1:" + sha(body)
        names.add("skill-" + sha(key.encode("utf-8"))[:56] + ":" + skill)
    return names


def claude_collection(files, directory, collection, target, required):
    expected = claude_names(collection, target)
    prefixes = {name.split(":", 1)[0] for name in expected}
    roots, observed = {}, {}
    for stage in STAGES:
        for side in ("A", "B"):
            label = stage + "-initialize-" + side
            members = [directory + "/" + label + suffix
                       for suffix in (".json", ".stdout.log", ".stderr.log")]
            require(all(name in files for name in members), "missing Claude initialization")
            required.update(members)
            metadata = document(files[members[0]])
            require(metadata.get("argv") == CLAUDE_ARGV
                    and metadata.get("request") == CLAUDE_REQUEST
                    and type(metadata.get("exit")) is int and metadata["exit"] == 0
                    and metadata.get("error") == ""
                    and all(metadata.get(key) is False for key in
                            ("deadline", "stdout_limit", "stderr_limit", "model_request")),
                    "Claude invocation failed or changed")
            cwd = metadata.get("cwd")
            path = coordinates(cwd, target)
            require(path.basename(cwd) == "project-" + side, "Claude A/B coordinate")
            root = path.dirname(cwd)
            require(path.basename(root) == directory, "Claude fixture/root join")
            require(same_path(collection, path.join(root, "local-collection"), target),
                    "Claude source/fixture join")
            if side in roots:
                require(cwd == roots[side], "Claude project coordinate changed")
            roots[side] = cwd
            response = document(files[members[1]])
            require(response.get("type") == "control_response", "Claude response type")
            envelope = response.get("response", {})
            require(envelope.get("request_id") == "native-discovery"
                    and envelope.get("subtype") == "success", "Claude response join")
            payload = envelope.get("response", {})
            require(payload.get("account", {}).get("tokenSource") == "none",
                    "Claude credential boundary")
            commands = payload.get("commands")
            require(isinstance(commands, list), "Claude commands missing")
            found = []
            for command in commands:
                require(isinstance(command, dict) and isinstance(command.get("name"), str),
                        "malformed Claude command")
                name = command["name"]
                if name.split(":", 1)[0] in prefixes:
                    require(name in expected and command.get("builtin") is not True,
                            "unexpected selected Claude command")
                    found.append(name)
            wanted = expected if side == "A" and stage != STAGES[2] else set()
            require(len(found) == len(set(found)) and set(found) == wanted,
                    "Claude selected Skill inventory mismatch")
            observed[stage + ":" + side] = sorted(found)
    require(roots["A"] != roots["B"], "Claude A/B alias")
    require(len(required) == 28, "internal Claude collection subset mismatch")
    return {"protocol_sessions": 6, "selected_names": sorted(expected),
            "observation_sha256": sha(json.dumps(observed, sort_keys=True).encode())}


def rpc_inventory(body):
    rows = body.splitlines()
    require(0 < len(rows) <= 10000, "RPC row count")
    sends, receives, pending = {}, {}, None
    for line in rows:
        require(0 < len(line) <= 4 << 20, "RPC line size")
        row = document(line)
        require(isinstance(row, dict) and set(row) == {"direction", "message"}
                and isinstance(row["message"], dict), "RPC row schema")
        message = row["message"]
        if row["direction"] == "send":
            identifier = message.get("id")
            require(type(identifier) is int and identifier == len(sends) + 1
                    and identifier in (1, 2) and pending is None,
                    "RPC request order/id")
            sends[identifier], pending = message, identifier
        else:
            require(row["direction"] == "receive" and pending is not None,
                    "RPC receive direction/order")
            identifier = message.get("id")
            # The canonical logger retains notifications while awaiting a reply.
            if identifier is not None:
                require(type(identifier) is int and identifier == pending,
                        "RPC response id mismatch")
            if identifier == pending:
                require("error" not in message and "result" in message,
                        "RPC response failed")
                receives[identifier], pending = message, None
    require(set(sends) == {1, 2} and set(receives) == {1, 2}
            and pending is None, "RPC response missing")
    require(sends[1] == {"id": 1, "method": "initialize",
                        "params": {"clientInfo": {"name": "uap_raw_project_check",
                                                  "version": "1"},
                                   "capabilities": {"experimentalApi": True}}},
            "RPC initialize changed")
    require(set(sends[2]) == {"id", "method", "params"}
            and sends[2]["method"] == "skills/list", "RPC inventory method")
    params = sends[2]["params"]
    require(isinstance(params, dict) and set(params) == {"cwds", "forceReload"}
            and params["forceReload"] is True and isinstance(params["cwds"], list)
            and len(params["cwds"]) == 1, "RPC inventory arguments")
    result = receives[2]["result"]
    require(isinstance(result, dict) and isinstance(result.get("data"), list)
            and len(result["data"]) == 1, "RPC inventory response")
    return params["cwds"][0], result["data"][0]


def codex_collection(files, directory, collection, target, required):
    observations, roots = {}, {}
    for stage, label in ((STAGES[0], STAGES[0]), (STAGES[1], STAGES[1]),
                         (STAGES[2], "local-collection-removed")):
        for side in ("A", "B"):
            selected = label + ("-project-B" if side == "B" else "")
            names = [directory + "/" + selected + suffix
                     for suffix in ("-rpc.jsonl", "-stderr.log")]
            require(all(name in files for name in names), "missing Codex collection RPC")
            required.update(names)
            cwd, row = rpc_inventory(files[names[0]])
            path = coordinates(cwd, target)
            require(path.basename(cwd) == ("project" if side == "A" else "project-B"),
                    "Codex A/B coordinate")
            root = path.dirname(cwd)
            require(path.basename(root) == directory
                    and same_path(collection, path.join(root, "local-collection"), target),
                    "Codex fixture/source join")
            if side in roots:
                require(cwd == roots[side], "Codex project coordinate changed")
            roots[side] = cwd
            require(isinstance(row, dict) and same_path(row.get("cwd"), cwd, target)
                    and row.get("errors") == [] and isinstance(row.get("skills"), list),
                    "Codex discovery context/error")
            found = {}
            present = side == "A" and stage != STAGES[2]
            for skill in row["skills"]:
                require(isinstance(skill, dict), "malformed Codex Skill")
                name = skill.get("name")
                if name not in SKILLS:
                    continue
                # Pinned Codex 0.162.0 serializes a local Skill's absent plugin
                # association as explicit null. Go's string decoder accepts
                # that as its zero value; the raw JSON auditor must check null.
                require(present and name not in found and skill.get("enabled") is True
                        and "pluginId" in skill and skill["pluginId"] is None
                        and skill.get("scope") == "repo"
                        and same_path(skill.get("path"),
                                      path.join(cwd, ".agents", "skills", name, "SKILL.md"),
                                      target), "Codex selected Skill mismatch")
                found[name] = skill["path"]
            require(set(found) == (set(SKILLS) if present else set()),
                    "Codex selected Skill inventory incomplete")
            observations[stage + ":" + side] = found
    require(roots["A"] != roots["B"], "Codex A/B alias")
    installed, repeated = observations[STAGES[0] + ":A"], observations[STAGES[1] + ":A"]
    require(set(installed) == set(repeated) and all(
        same_path(installed[name], repeated[name], target) for name in installed),
        "Codex repeat paths changed")
    require(len(required) == 22, "internal Codex collection subset mismatch")
    return {"protocol_sessions": 6,
            "observation_sha256": sha(json.dumps(observations, sort_keys=True).encode())}


def audit_native(blob, source, run, job, manifest):
    """Return a compact receipt or raise; metadata/download trust belongs to caller.

    source: exact {"commit": ..., "tree": ...}.
    manifest: client, target, archive_size, archive_sha256, runner_source,
              workflow_source, run_id, job_id.
    run/job are already authenticated integer IDs, not GitHub response bodies.
    """
    require(type(run) is int and type(job) is int and run > 0 and job > 0
            and manifest["run_id"] == run and manifest["job_id"] == job,
            "caller run/job binding")
    client, target = manifest["client"], manifest["target"]
    require(client in ("claude", "codex") and
            target in ("linux-amd64", "darwin-arm64", "windows-amd64"),
            "unsupported native lane")
    authority = authorities(manifest)
    files, runner, directories, expanded = zip_files(blob, manifest)
    selected = runner_checks(runner, files, source, client, target, authority)
    directory, report = fixture_report(files, runner, client, target, selected)
    collection, required = installer_members(files, directory, client)
    coordinates(collection, target)
    observations = (claude_collection if client == "claude" else codex_collection)(
        files, directory, collection, target, required)
    boundaries = [
        "Archive bytes/CRC/hashes and retained collection protocol observations audited.",
        "Run/job/metadata authenticity, source checkout and download custody are caller duties.",
        "Tool archive/binary bytes are not uploaded here; pins and binary hash joins audited.",
        "Installed/source Skill bytes, full state, read-only listing snapshots, stable owned "
        "bindings, cleanup and foreign-byte preservation are source-pinned Go assertions.",
        "Installer argv/cwd/exit are not separately captured; original result/stderr retained.",
        "No vendor authentication or real-model qualification is granted by this audit.",
    ]
    if client == "codex":
        boundaries.append("B is skills/list query isolation; B app-server process cwd is not proven.")
    else:
        boundaries.append("Claude request is normalized metadata; stdout/stderr are original captures.")
    return {"audit": "native_collection_retained_bytes", "status": "passed",
            "source": source, "run_id": run, "job_id": job,
            "client": client, "target": target, "archive_sha256": sha(blob),
            "archive_bytes": len(blob), "retained_files": len(files),
            "directory_entries": directories, "expanded_bytes": expanded,
            "fixture_directory": directory, "collection_required_files": len(required),
            "collection_inventory_sha256": sha(json.dumps(
                {name: sha(files[name]) for name in sorted(required)},
                sort_keys=True, separators=(",", ":")).encode()),
            "required_tests": runner["required_tests"], **observations,
            "boundaries": boundaries}
