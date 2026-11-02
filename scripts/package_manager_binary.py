"""Nonempty unchanged-binary installation with the real production scanner.

The disposable Gemini configuration is detected normally. No Gemini executable,
model, account, security override, or fabricated assessment is provided.
"""

import hashlib
import json
import os
from pathlib import Path
import shutil


PLUGIN = "binary-install-fixture"
SKILL = "binary-install-docs"
SKILL_BYTES = (
    "---\nname: binary-install-docs\ndescription: Read a harmless package marker\n---\n"
    "This fixture documents a disposable installer check.\n"
    "Marker: UAP-UNCHANGED-BINARY-NONEMPTY-INSTALL.\n"
).encode("utf-8")
FOREIGN_SKILL = b"foreign documentation preserved by the binary lifecycle\n"
FOREIGN_SETTINGS = b'{"mcpServers":{},"binaryForeign":"preserve"}\n'


def require(condition, message):
    # Python -O must never disable a qualification assertion.
    if not condition:
        raise RuntimeError(message)


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def run_installations(run, binary, scratch, env):
    """Run five real CLI commands, retaining command logs through Run.command."""
    facts = {"status": "running", "evidence_class": "production_binary_installation",
             "source_kind": "local_directory", "scope": "user", "target": "gemini",
             "native_runtime_qualified": False, "commands": []}
    run.report["binary_installation"] = facts
    run.save()
    try:
        _installations(run, binary, scratch, env, facts)
    except (OSError, ValueError, KeyError, IndexError, TypeError, AttributeError, RuntimeError) as error:
        facts.update(status="fail", error=str(error))
        run.save()
        raise RuntimeError("unchanged-binary installation proof failed: " + str(error)) from error
    facts["status"] = "pass"
    run.save()
    return True


def _installations(run, binary, scratch, env, facts):
    workspace = Path(scratch) / "binary installation workspace Ω"
    workspace.mkdir()
    source = workspace / "local package with spaces"
    relative_skill = Path("skills") / SKILL / "SKILL.md"
    (source / relative_skill).parent.mkdir(parents=True)
    (source / relative_skill).write_bytes(SKILL_BYTES)
    manifest_bytes = (json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                 "name": PLUGIN, "version": "1.0.0"}) + "\n").encode("utf-8")
    (source / "plugin.json").write_bytes(manifest_bytes)
    home = Path(env["AGENTPLUGINS_HOME"])
    state_path = home / "state-v2.json"
    require(not state_path.exists() and not (home / "security").exists(),
            "binary installation profile must have cold state and security caches")
    native = Path(env["GEMINI_CLI_HOME"]) / ".gemini"
    native.mkdir()
    settings = native / "settings.json"
    settings.write_bytes(FOREIGN_SETTINGS)
    foreign = native / "skills" / "foreign" / "SKILL.md"
    foreign.parent.mkdir(parents=True)
    foreign.write_bytes(FOREIGN_SKILL)
    # Hide host-installed native executables while preserving normal directory
    # detection. The scanner is resolved to its downloaded absolute path.
    empty_path = workspace / "empty executable path"
    empty_path.mkdir()
    child_env = dict(env, PATH=str(empty_path))

    def command(name, expected_command, *args):
        facts["commands"].append(name)
        if not run.command(name, [binary, *args, "--scope", "user", "--format", "json"],
                           "production_binary_installation", cwd=workspace, env=child_env):
            raise RuntimeError(name + " did not complete successfully")
        document = read_json(run.directory / (name + ".stdout"))
        require(document.get("schema_version") == 1 and document.get("result") == "success"
                and document.get("command") == expected_command and isinstance(document.get("data"), dict),
                name + " returned an unexpected public JSON envelope")
        return document["data"]

    def foreign_unchanged():
        require(settings.read_bytes() == FOREIGN_SETTINGS and foreign.read_bytes() == FOREIGN_SKILL,
                "binary lifecycle changed foreign Gemini configuration or skill bytes")
        require(not (home / "uap.toml").exists() and not (home / "uap.lock").exists(),
                "unsaved binary installation published an agreement")
        require((source / relative_skill).read_bytes() == SKILL_BYTES
                and (source / "plugin.json").read_bytes() == manifest_bytes,
                "binary lifecycle changed local source bytes")

    def owned():
        state = read_json(state_path)
        installations = state.get("installations", [])
        require(len(installations) == 1 and installations[0].get("declared_name") == PLUGIN,
                "binary installation did not persist exactly the expected package")
        installation = installations[0]
        clients = installation.get("clients", {})
        require(len(clients) == 1, "binary installation has unexpected client bindings")
        binding = next(iter(clients.values()))
        require(installation.get("source", {}).get("tree_digest") == first.get("tree_digest")
                and installation.get("package", {}).get("manifest_digest") == first.get("manifest_digest")
                and binding.get("package_revision", {}).get("tree_digest") == first.get("tree_digest")
                and binding.get("package_revision", {}).get("manifest_digest") == first.get("manifest_digest"),
                "persisted package/binding identity disagrees with the scanned installation")
        require(binding.get("client_id") == "gemini" and binding.get("scope") == "user"
                and binding.get("verification") == "installation_verified"
                and binding.get("activation") == "active"
                and binding.get("authentication") == "not_checked"
                and Path(binding.get("native_profile_root", "")).resolve() == native.resolve(),
                "binary installation lost its selected profile or truthful readiness")
        managed = Path(binding["target_locator"])
        require(managed.parent.resolve() == (home / "managed" / "clients" / "gemini").resolve()
                and managed.name.startswith(PLUGIN + "-"), "binary receipt points outside its managed namespace")
        require((managed / relative_skill).read_bytes() == SKILL_BYTES
                and (native / relative_skill).read_bytes() == SKILL_BYTES,
                "binary installed payload differs from independent fixture bytes")
        foreign_unchanged()
        return installation, binding, managed

    first = command("binary-install-first", "add", "add", source, "--no-save", "--target", "gemini")
    security = first.get("security", {})
    require(first.get("status") == "completed" and first.get("succeeded") == 1
            and first.get("failed") == 0 and first.get("environment", {}).get("saved") is False,
            "first binary installation did not complete an unsaved nonempty install")
    require(security.get("scanner") == {"id": "lintai", "version": "0.1.3"}
            and security.get("evidence_source") == "local_scan"
            and security.get("policy", {}).get("id") == "agent-plugin-install"
            and security.get("policy", {}).get("version") == 2
            and security.get("outcome") == "no_blocking_findings"
            and security.get("counts") == {"blocking": 0, "warnings": 0, "total": 0}
            and security.get("scanned_files", 0) >= 2,
            "first binary install did not use the real pinned production scanner")
    installation, binding, managed = owned()
    iid, source_id = installation["installation_id"], installation["source"]["source_binding_id"]
    require(security.get("subject") == {"tree_digest": first.get("tree_digest"),
                                         "manifest_digest": first.get("manifest_digest")},
            "scanner assessment does not describe the installed package")
    scanners = list((home / "security" / "lintai").rglob("lintai.exe" if os.name == "nt" else "lintai"))
    require(len(scanners) == 1 and scanners[0].is_file() and not scanners[0].is_symlink(),
            "production scanner executable is absent or ambiguous")
    facts.update(installation_id=iid, scanner=security["scanner"], policy=security["policy"],
                 scanner_binary_sha256=hashlib.sha256(scanners[0].read_bytes()).hexdigest(),
                 scanner_evidence_source=security["evidence_source"], scanned_files=security["scanned_files"],
                 scan_report_digest=security["report_digest"], tree_digest=first["tree_digest"],
                 manifest_digest=first["manifest_digest"])
    before = state_path.read_bytes()
    repeated = command("binary-install-repeat", "add", "add", source, "--no-save", "--target", "gemini")
    require(repeated.get("result", {}).get("mutated") is False
            and repeated.get("result", {}).get("installation_id") == iid
            and repeated.get("security", {}).get("evidence_source") == "cache"
            and repeated.get("security", {}).get("report_digest") == security["report_digest"]
            and state_path.read_bytes() == before, "unchanged binary repeat mutated state or lost its exact assessment")
    owned()
    info = command("binary-install-info", "info", "info", iid, "--target", "gemini")
    rows = info.get("clients", [])
    require(info.get("installation_id") == iid and len(rows) == 1
            and rows[0].get("verification") == "installation_verified"
            and rows[0].get("native_discovery_reconciled") is False
            and state_path.read_bytes() == before, "binary inspection changed state or claimed native discovery")
    shutil.rmtree(native / "skills" / SKILL)
    repaired = command("binary-install-repair", "repair", "repair", iid, "--target", "gemini")
    require(repaired.get("succeeded") == 1 and repaired.get("failed") == 0,
            "binary missing-native-skill repair did not complete")
    repaired_installation, repaired_binding, repaired_managed = owned()
    require(repaired_installation["installation_id"] == iid
            and repaired_installation["source"]["source_binding_id"] == source_id
            and repaired_managed == managed and repaired_binding["client_binding_id"] == binding["client_binding_id"],
            "binary repair changed package or binding ownership")
    removed = command("binary-install-remove", "remove", "remove", iid, "--target", "gemini")
    require(removed.get("succeeded") == 1 and removed.get("failed") == 0
            and removed.get("plugin_data_preserved") is True
            and not managed.exists() and not (native / "skills" / SKILL).exists(),
            "binary removal retained an owned artifact or misreported retained data")
    require(all(not row.get("clients") for row in read_json(state_path).get("installations", [])),
            "binary removal retained an active client binding")
    foreign_unchanged()
    facts["assertions"] = ["exact_managed_and_native_skill_bytes", "real_cold_production_scan",
                           "unchanged_repeat", "read_only_inspection", "missing_native_skill_repair",
                           "owned_removal", "foreign_and_source_bytes_preserved", "no_saved_agreement"]
