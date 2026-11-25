"""Separate pinned Devin User installation/discovery proof without a Cascade profile.

The existing Project cohort keeps its own entrypoint and acceptance. This cohort
uses real native catalogs, not a model call or an authentication qualification.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import sys
import tempfile

import devin_project_auto_install_e2e as pinned

NAME = "uap-e2e-user-v57"
DESCRIPTION = "Inert User installation proof"
STDIO = "uap_e2e_user_57_stdio"
HTTP = "uap_e2e_user_57_http"
MAX_ENTRIES = 256
MAX_FILE = 1 << 20
MAX_TOTAL = 4 << 20
LAUNCHER = Path("io.github.777genius.agentplugins/managed-stdio-v1/agentplugins")
DATA_SENTINEL = b"owned User data must survive repair\n"


def require(value, message):
    if not value:
        raise RuntimeError(message)


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_nlink, info.st_uid,
            info.st_gid, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def read_owned(path, anchor):
    """Read a bounded owned regular file without following or blocking on a replacement."""
    path, anchor = Path(path), Path(anchor)
    require(path.is_absolute() and path.is_relative_to(anchor)
            and path.resolve(strict=True) == path, "User file escaped or aliased")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_uid == os.getuid()
            and before.st_nlink == 1 and before.st_size <= MAX_FILE, "User file is not bounded, regular and owned")
    with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK), "rb") as stream:
        require(identity(os.fstat(stream.fileno())) == identity(before), "User file replaced before read")
        body = stream.read(MAX_FILE + 1)
        require(identity(os.fstat(stream.fileno())) == identity(before), "User file changed while reading")
    require(len(body) == before.st_size and identity(path.lstat()) == identity(before), "User file identity drift")
    return body


def read_json(path, anchor):
    return json.loads(read_owned(path, anchor).decode("utf-8"))


def file_digest(path):
    require(path.is_absolute() and path.resolve(strict=True) == path, "aliased User executable path")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_mode & 0o111 and before.st_uid == os.getuid()
            and before.st_nlink == 1 and 0 < before.st_size <= 256 << 20,
            "unqualified User executable")
    digest, count = hashlib.sha256(), 0
    with os.fdopen(os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK), "rb") as stream:
        require(identity(os.fstat(stream.fileno())) == identity(before), "User executable replaced")
        for block in iter(lambda: stream.read(1 << 20), b""):
            count += len(block)
            require(count <= before.st_size, "User executable grew")
            digest.update(block)
        require(identity(os.fstat(stream.fileno())) == identity(before), "User executable changed")
    require(count == before.st_size and identity(path.lstat()) == identity(before), "User executable drift")
    return digest.hexdigest()


def executable_fact(path):
    before = identity(path.lstat())
    digest = file_digest(path)
    require(identity(path.lstat()) == before, "User executable changed during observation")
    return {"identity": before, "sha256": digest}


def snapshot(paths, anchor, trusted_launcher=None):
    """Bounded no-follow identities and contents of the explicitly owned inputs."""
    anchor = Path(anchor)
    require(anchor.is_absolute() and anchor.resolve(strict=True) == anchor, "aliased User case")
    result, total = {}, 0

    def visit(path):
        nonlocal total
        require(path.is_absolute() and path.is_relative_to(anchor)
                and path.resolve(strict=True) == path, "User snapshot path escaped or aliased")
        info = path.lstat()
        require(info.st_uid == os.getuid() and not stat.S_ISLNK(info.st_mode)
                and (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)), "unowned User snapshot object")
        require(str(path) not in result and len(result) < MAX_ENTRIES, "User snapshot entry bound or duplicate")
        row = [info.st_dev, info.st_ino, info.st_mode]
        result[str(path)] = row
        if stat.S_ISDIR(info.st_mode):
            row.append(info.st_mtime_ns)
            with os.scandir(path) as entries:
                for entry in entries:
                    visit(Path(entry.path))
            require(identity(path.lstat()) == identity(info), "User snapshot directory changed")
        else:
            require(info.st_nlink == 1, "aliased User snapshot file")
            if trusted_launcher is not None and path == trusted_launcher[0]:
                digest = file_digest(path)
                require(digest == trusted_launcher[1], "generated launcher differs from exact source CLI")
            else:
                require(info.st_size <= MAX_FILE, "oversized ordinary User snapshot file")
                body = read_owned(path, anchor)
                total += len(body)
                require(len(body) == info.st_size and total <= MAX_TOTAL, "User snapshot byte bound")
                digest = pinned.digest(body)
            require(identity(path.lstat()) == identity(info), "User snapshot file identity changed")
            row.extend([info.st_nlink, info.st_size, info.st_mtime_ns, digest])
    for path in paths:
        visit(Path(path))
    return result


def protected_context_paths(native, projects):
    return [native / "config.json", native / "skills" / "foreign-0",
            *(project / ".devin" for project in projects)]


def require_project_state_absent(projects):
    require(all(not os.path.lexists(project / ".uap") for project in projects),
            "User operation created Project installation state")


def seed_user_data(data):
    sentinel = data / "fixture-user-data.txt"
    with os.fdopen(os.open(sentinel, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600), "wb") as stream:
        stream.write(DATA_SENTINEL)


def assert_data_preserved(data, anchor, before):
    require(snapshot([data], anchor) == before
            and read_owned(data / "fixture-user-data.txt", anchor) == DATA_SENTINEL,
            "User operation changed original data directory or contents")


def assert_repair_data(data, anchor, before, receipt_before, receipt_after):
    assert_data_preserved(data, anchor, before)
    require(receipt_before == receipt_after, "User repair changed original owned data receipt")


def no_cascade(home):
    require(not os.path.lexists(Path(home) / ".codeium"), "User cohort created or inherited Cascade authority")


def skill_present(body, expected_path):
    rows = json.loads(body)
    require(isinstance(rows, list) and len(rows) <= 128
            and all(isinstance(row, dict) for row in rows), "User Skill catalog shape differs")
    matches = [row for row in rows if row.get("name") == NAME or row.get("display_name") == NAME]
    if not matches:
        return False
    expected = {"name": NAME, "description": DESCRIPTION, "provider": "Devin",
                "base_dir": str(expected_path), "display_name": NAME,
                "triggers": ["user", "model"], "warnings": [], "errors": []}
    require(matches == [expected],
            "User Skill catalog has wrong source, duplicates or invalid metadata")
    return True


def add_identity(data, repeated=False):
    require(isinstance(data, dict), "User add response shape differs")
    if repeated:
        require("targets" not in data and "operation_id" not in data and data.get("dry_run") is False,
                "User unchanged add used a batch response")
        output = data
    else:
        targets = data.get("targets")
        require(isinstance(targets, list) and len(targets) == 1
                and isinstance(targets[0], dict) and targets[0].get("target") == "windsurf"
                and targets[0].get("status") == "external_completed", "User add target differs")
        output = targets[0].get("output")
    require(isinstance(output, dict) and isinstance(output.get("result"), dict), "User add result missing")
    outcome = output["result"]
    if repeated:
        require(outcome.get("no_change") is True and outcome.get("mutated") is False
                and outcome.get("requires_confirmation") is False, "User repeat claims mutation")
    iid = outcome.get("installation_id")
    require(isinstance(iid, str) and re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", iid),
            "User installation ID differs")
    return iid


def selected_binding(document, iid, native, managed_root, required=True):
    require(isinstance(document, dict) and type(document.get("schema_version")) is int
            and document["schema_version"] == 5 and isinstance(document.get("installations"), list),
            "User state schema differs")
    items = document["installations"]
    require(all(isinstance(item, dict) and isinstance(item.get("clients"), dict) for item in items),
            "User installation state shape differs")
    require(all(isinstance(key, str) and key and isinstance(binding, dict)
                and binding.get("client_binding_id") == key
                for item in items for key, binding in item["clients"].items()),
            "User saved client binding shape differs")
    identifiers = [item.get("installation_id") for item in items]
    require(all(isinstance(value, str) and value for value in identifiers)
            and len(set(identifiers)) == len(identifiers), "User installation state IDs differ")
    require(set(identifiers) <= {iid}, "User state has an unrelated installation")
    rows = [item for item in items if item["installation_id"] == iid]
    if not required:
        require(all(not item["clients"] for item in items),
                "removed User installation retains a client binding")
        return None
    require(len(rows) == 1 and len(rows[0]["clients"]) == 1, "User binding missing or duplicated")
    key, binding = next(iter(rows[0]["clients"].items()))
    require(isinstance(binding, dict) and key == binding.get("client_binding_id")
            and binding.get("client_id") == "windsurf" and binding.get("scope") == "user"
            and binding.get("native_profile_root") == str(native), "User binding changed selected native authority")
    locator = binding.get("target_locator")
    require(isinstance(locator, str) and locator, "User managed locator absent")
    path = Path(locator)
    require(path.is_absolute() and path.parent == managed_root
            and path.name == binding.get("physical_artifact_id")
            and path.resolve(strict=True) == path and path.is_dir(), "User managed package escaped selected state")
    objects = binding.get("native_objects")
    require(isinstance(objects, list) and len(objects) == 4
            and all(isinstance(row, dict) for row in objects), "User native ownership shape differs")
    expected_objects = {
        ("managed_package_directory", NAME, str(path), "package:windsurf:" + path.name),
        ("devin_global_skill_directory", NAME, str(native / "skills" / NAME),
         "devin_global_skill_directory:" + NAME),
        ("devin_global_mcp_server", STDIO, str(native / "mcp_config.json"), "devin_global_mcp_server:" + STDIO),
        ("devin_global_mcp_server", HTTP, str(native / "mcp_config.json"), "devin_global_mcp_server:" + HTTP),
    }
    require({(row.get("kind"), row.get("logical_name"), row.get("path"), row.get("object_id"))
             for row in objects} == expected_objects
            and all(row.get("protection_class") == "managed" and isinstance(row.get("managed_digest"), str)
                    and row["managed_digest"] for row in objects), "User native ownership path or identity differs")
    data_root = managed_root.parents[2] / "plugin-data" / path.name
    receipts = rows[0].get("data_receipts")
    receipt_id = binding.get("data_receipt_id")
    require(isinstance(receipts, dict) and isinstance(receipt_id, str)
            and set(receipts) == {receipt_id}, "User data receipt missing or duplicated")
    receipt = receipts[receipt_id]
    require(isinstance(receipt, dict) and receipt.get("data_receipt_id") == receipt_id
            and receipt.get("physical_backend_id") == path.name and receipt.get("scope") == "user"
            and receipt.get("state") == "owned" and receipt.get("locator") == str(data_root)
            and isinstance(receipt.get("ownership_digest"), str)
            and re.fullmatch(r"sha256:[0-9a-f]{64}", receipt["ownership_digest"]),
            "User data receipt changed original owned authority")
    require(data_root.resolve(strict=True) == data_root and data_root.is_dir()
            and data_root.stat().st_uid == os.getuid(), "User data directory differs")
    return binding


def verify_projection(managed, package, native, data, service, port, cli_hash, anchor, foreign_servers):
    """Verify authored resources and the exact production-generated stdio projection."""
    observed = snapshot([managed], anchor, trusted_launcher=(managed / LAUNCHER, cli_hash))
    files = {str(Path(name).relative_to(managed)): row[-1]
             for name, row in observed.items() if stat.S_ISREG(row[2])}
    expected_files = {"plugin.json", "mcp.json", "skills/" + NAME + "/SKILL.md",
                      str(LAUNCHER), str(LAUNCHER.parent / "metadata.json")}
    require(set(files) == expected_files, "User managed generated-file inventory differs")
    for relative in ("plugin.json", "skills/" + NAME + "/SKILL.md"):
        require(files[relative] == pinned.digest(read_owned(package / relative, anchor)), "User authored payload differs")
    metadata = read_json(managed / LAUNCHER.parent / "metadata.json", anchor)
    require(metadata == {"protocol": 1, "sha256": cli_hash, "cliVersion": "0.1.0-development"},
            "User generated launcher metadata differs")
    argv = ["--internal-stdio-v1", str(managed), str(data), str(managed), "plugin", "--",
            "python3", str(service)]
    stdio = {"command": str(managed / LAUNCHER), "args": argv,
             "env": {"PLUGIN_ROOT": str(managed), "PLUGIN_DATA": str(data)}}
    url = "http://127.0.0.1:" + str(port) + "/mcp"
    expected = {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                "mcpServers": {STDIO: dict(stdio, type="stdio"),
                               HTTP: {"type": "streamable-http", "url": url}}}
    require(read_json(managed / "mcp.json", anchor) == expected,
            "User managed MCP roots, wrapper argv or environment differs")
    actual = read_json(native / "mcp_config.json", anchor).get("mcpServers", {})
    expected_native = {**foreign_servers, STDIO: stdio, HTTP: {"url": url, "transport": "http"}}
    require(actual == expected_native,
            "User native MCP projection differs from selected owned resources")
    return files


def qualified(report):
    return (report.get("user_installation_discovery_e2e") is True
            and report.get("cleanup_complete") is True)


def run(root, report):
    checkout = Path(os.environ["GITHUB_WORKSPACE"]).resolve(strict=True)
    expected = os.environ["EXPECTED_COMMIT"]
    require(os.environ.get("GITHUB_ACTIONS") == "true" and sys.platform == "linux",
            "User native cohort requires its disposable Linux workflow")
    require(re.fullmatch(r"[0-9a-f]{40}", expected), "User source commit invalid")
    require(pinned.command(["git", "rev-parse", "HEAD"], checkout, os.environ).strip() == expected
            and not pinned.command(["git", "status", "--porcelain"], checkout, os.environ).strip(),
            "User source is not exact and clean")
    report["source_tree"] = pinned.command(["git", "rev-parse", "HEAD^{tree}"], checkout, os.environ).strip()
    binary, integrity = pinned.install_genuine_devin(root)
    report.update(npm_tarball_integrity=integrity, verified_binary_sha256=pinned.BINARY_SHA)
    home, config, state = root / "home", root / "config", root / "user-state"
    native = config / "devin"
    projects = [root / "a", root / "b"]
    for path in (home, native, state, root / "scratch", root / "data", root / "cache", root / "xdg-state", *projects):
        path.mkdir(mode=0o700, parents=True, exist_ok=True)
    env = {"HOME": str(home), "USERPROFILE": str(home), "XDG_CONFIG_HOME": str(config),
           "XDG_DATA_HOME": str(root / "data"), "XDG_CACHE_HOME": str(root / "cache"),
           "XDG_STATE_HOME": str(root / "xdg-state"), "AGENTPLUGINS_HOME": str(state),
           "TMPDIR": str(root / "scratch"), "TMP": str(root / "scratch"), "TEMP": str(root / "scratch"),
           "PATH": str(binary.parent) + os.pathsep + os.defpath, "LANG": "C.UTF-8",
           "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
           "GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.hooksPath", "GIT_CONFIG_VALUE_0": "/dev/null",
           "CI": "true", "DO_NOT_TRACK": "1"}
    # Same pinned baseline as raw_project_skills in the unchanged Project helper.
    (native / "config.json").write_text('{"version":1,"auto_update":false}\n')
    foreign_mcp = {"mcpServers": {"foreign-user": {"url": "https://foreign.invalid/mcp", "disabled": True}}}
    (native / "mcp_config.json").write_text(json.dumps(foreign_mcp) + "\n")
    protected = protected_context_paths(native, projects)
    for index, path in enumerate([native, *(project / ".devin" for project in projects)]):
        foreign = path / "skills" / ("foreign-" + str(index))
        foreign.mkdir(parents=True)
        (foreign / "SKILL.md").write_text("---\nname: foreign-" + str(index) + "\ndescription: Inert foreign Skill\n---\n")
        if index:
            (path / "mcp_config.json").write_text('{"mcpServers":{}}\n')
    original = snapshot(protected, root)
    cli = root / "agentplugins"
    pinned.command(["go", "build", "-trimpath", "-o", str(cli), "./cli/cmd/agentplugins"],
                   checkout, os.environ, timeout=200)
    executable_facts = {path: executable_fact(path) for path in (cli, binary)}
    report["uap_binary_sha256"] = executable_facts[cli]["sha256"]
    no_cascade(home)
    anchors = [root, home, config, native, state, binary.parent, binary.parent.parent, *projects]
    anchors_before = {path: (path.stat().st_dev, path.stat().st_ino, path.stat().st_mode) for path in anchors}
    source = None

    def guard():
        no_cascade(home)
        require_project_state_absent(projects)
        require(all(path.resolve(strict=True) == path and not path.is_symlink()
                    and (path.stat().st_dev, path.stat().st_ino, path.stat().st_mode) == saved
                    for path, saved in anchors_before.items()), "User authority directory replaced")
        require(all(executable_fact(path) == saved for path, saved in executable_facts.items()),
                "User executable identity changed")
        if source is not None:
            require(snapshot([package, service], root) == source, "User inert source changed")
        require(snapshot(protected, root) == original, "User cohort changed foreign or Project authority")
        require(read_json(native / "mcp_config.json", root).get("mcpServers", {}).get("foreign-user")
                == foreign_mcp["mcpServers"]["foreign-user"], "User cohort changed foreign Global MCP")

    def command(stage, argv, cwd=projects[0], timeout=90):
        guard()
        receipt = {"stage": stage, "argv": argv, "cwd": str(cwd)}
        report["commands"].append(receipt)
        result = pinned.command(argv, cwd, env, timeout=timeout, receipt=receipt)
        guard()
        return result

    def public(stage, *args):
        output = json.loads(command(stage, [str(cli), stage, *args, "--target", "windsurf",
                                            "--scope", "user", "--format", "json"]))
        require(output.get("command") == stage and output.get("result") == "success",
                "public User command did not succeed")
        data = output["data"]
        targets = data.get("targets", []) if isinstance(data, dict) else []
        report["public"].append({"stage": stage, "result": output["result"],
            "readiness": output.get("readiness"), "status": data.get("status") if isinstance(data, dict) else None,
            "targets": [{"target": row.get("target"), "status": row.get("status"),
                "readiness": row.get("output", {}).get("readiness"),
                "activation": row.get("output", {}).get("result", {}).get("activation")}
                for row in targets if isinstance(row, dict)]})
        return data

    skill = native / "skills" / NAME
    state_file = state / "state-v2.json"
    managed_root = state / "managed" / "clients" / "windsurf"

    def binding(iid, required=True):
        snapshot([state_file], root)
        return selected_binding(read_json(state_file, root), iid, native, managed_root, required)

    def inventory(stage, present):
        observations = []
        for index, cwd in enumerate(projects):
            skills = command(stage + "-skills-" + str(index), [str(binary), "skills", "list", "--json"], cwd, 45)
            servers = command(stage + "-mcp-" + str(index), [str(binary), "mcp", "list", "--json"], cwd, 45)
            found = [skill_present(skills, skill), pinned.available_mcp(servers, STDIO, native / "mcp_config.json", "stdio"),
                     pinned.available_mcp(servers, HTTP, native / "mcp_config.json", "http")]
            require(found == [present] * 3, "native User catalogs differ across A/B or selected XDG root")
            require(len(json.loads(servers)) <= 128, "User MCP catalog entry bound")
            observations.append({"project": index, "owned_skill_stdio_http": found,
                                 "skills": json.loads(skills), "mcp": json.loads(servers)})
        report["inventories"].append({"stage": stage, "observations": observations})

    version = command("version", [str(binary), "--version"])
    require(re.fullmatch(r"devin 3000\.12\.57(?: \([0-9a-f]+\))?\s*", version), "User native version differs")
    with pinned.external_http_fixture(root) as port:
        service = root / "inert-mcp.py"
        service.write_text(
            'import json,sys\nfor line in sys.stdin:\n'
            ' request=json.loads(line)\n method=request.get("method")\n'
            ' result={"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"uap-user-inert","version":"1"}} if method=="initialize" else {"tools":[]} if method=="tools/list" else {}\n'
            ' if "id" in request: print(json.dumps({"jsonrpc":"2.0","id":request["id"],"result":result}),flush=True)\n')
        package = root / "package"
        payload = package / "skills" / NAME / "SKILL.md"
        payload.parent.mkdir(parents=True)
        payload.write_text("---\nname: " + NAME + "\ndescription: " + DESCRIPTION + "\n---\nNo model action required.\n")
        (package / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                                      "name": NAME, "version": "1.0.0"}))
        (package / "mcp.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {STDIO: {"type": "stdio", "command": "python3", "args": [str(service)]},
                           HTTP: {"type": "streamable-http", "url": "http://127.0.0.1:" + str(port) + "/mcp"}}}))
        source = snapshot([package, service], root)
        inventory("before-add", False)
        iid = add_identity(public("add", str(package), "--no-save"))
        selected = binding(iid)
        report["selected_binding"] = selected
        managed = Path(selected["target_locator"])
        data = state / "plugin-data" / selected["physical_artifact_id"]
        report["projected_files_sha256"] = verify_projection(
            managed, package, native, data, service, port, executable_facts[cli]["sha256"], root,
            foreign_mcp["mcpServers"])
        require(read_owned(skill / "SKILL.md", root) == read_owned(payload, root), "User native pinned Skill bytes differ")
        seed_user_data(data)
        held = snapshot([managed, skill, data, state_file, native / "mcp_config.json"], root,
                        trusted_launcher=(managed / LAUNCHER, executable_facts[cli]["sha256"]))
        inventory("after-add", True)
        repeated = add_identity(public("add", str(package), "--no-save"), repeated=True)
        require(repeated == iid and binding(iid) == selected
                and snapshot([managed, skill, data, state_file, native / "mcp_config.json"], root,
                             trusted_launcher=(managed / LAUNCHER, executable_facts[cli]["sha256"])) == held,
                "User repeat changed identity, state, ownership or native bytes")
        report["repeat_data_preserved"] = True
        verify_projection(managed, package, native, data, service, port, executable_facts[cli]["sha256"], root,
                          foreign_mcp["mcpServers"])
        inventory("after-repeat", True)
        data_before = snapshot([data], root)
        state_before = read_json(state_file, root)
        receipt_before = next(row["data_receipts"][selected["data_receipt_id"]]
                              for row in state_before["installations"] if row["installation_id"] == iid)
        current_skill = snapshot([skill], root)
        require(current_skill == {key: value for key, value in held.items()
                                  if Path(key).is_relative_to(skill)}, "User repair target identity changed")
        shutil.rmtree(skill)
        public("repair", iid)
        repaired = binding(iid)
        require(repaired["client_binding_id"] == selected["client_binding_id"]
                and repaired["target_locator"] == str(managed)
                and repaired["data_receipt_id"] == selected["data_receipt_id"]
                and read_owned(skill / "SKILL.md", root) == read_owned(payload, root), "User repair changed authority or bytes")
        verify_projection(managed, package, native, data, service, port, executable_facts[cli]["sha256"], root,
                          foreign_mcp["mcpServers"])
        state_after = read_json(state_file, root)
        receipt_after = next(row["data_receipts"][repaired["data_receipt_id"]]
                             for row in state_after["installations"] if row["installation_id"] == iid)
        assert_repair_data(data, root, data_before, receipt_before, receipt_after)
        report["repair_data_preserved"] = True
        inventory("after-repair", True)
        public("remove", iid)
        inventory("after-remove", False)
        binding(iid, required=False)
        require(not os.path.lexists(managed) and not os.path.lexists(skill)
                and read_json(native / "mcp_config.json", root) == foreign_mcp,
                "User removal retained owned payload or changed foreign native config")
        assert_data_preserved(data, root, data_before)
        report["remove_data_retained"] = True
        require(snapshot([package, service], root) == source, "User cohort changed inert source")
    guard()
    require(all(executable_fact(path) == saved for path, saved in executable_facts.items()), "User executable identity drifted")
    require(pinned.command(["git", "rev-parse", "HEAD"], checkout, os.environ).strip() == expected
            and not pinned.command(["git", "status", "--porcelain"], checkout, os.environ).strip(),
            "User source changed during native qualification")
    report["cascade_absent"] = not os.path.lexists(home / ".codeium")
    report["user_installation_discovery_e2e"] = True


if __name__ == "__main__":
    report = {"commit": os.environ.get("EXPECTED_COMMIT"), "version": pinned.VERSION,
              "commands": [], "public": [], "inventories": [], "user_installation_discovery_e2e": False,
              "model_runtime_qualified": False, "vendor_authentication_qualified": False,
              "native_readiness_qualified": False, "cleanup_complete": False}
    try:
        with tempfile.TemporaryDirectory(prefix="devin-user-v57-", dir=os.environ["RUNNER_TEMP"]) as temporary:
            run(Path(temporary).resolve(strict=True), report)
        report["cleanup_complete"] = True
    except Exception as error:
        report["user_installation_discovery_e2e"] = False
        report["failure"] = pinned.command_diagnostic(str(error).encode(), {})
    output = Path(os.environ["RUNNER_TEMP"]) / "devin-user-v57-e2e.json"
    output.write_text(json.dumps(report, sort_keys=True, indent=2) + "\n")
    print(json.dumps({"commit": report["commit"], "user_installation_discovery_e2e": qualified(report),
                      "failure": report.get("failure")}))
    sys.exit(0 if qualified(report) else 1)
