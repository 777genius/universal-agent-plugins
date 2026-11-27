"""Production CLI local collection proof; native runtime discovery is not qualified."""

import os
from pathlib import Path
import stat

from package_manager_binary import FOREIGN_SETTINGS, FOREIGN_SKILL, read_json, require


NAMES = ("collection-review", "collection-testing")


def skill_bytes(name):
    return ("---\nname: " + name + "\ndescription: Read a harmless local fixture\n---\n"
            "Disposable collection documentation.\n").encode("utf-8")


def tree_bytes(root):
    """Snapshot entry kinds, link destinations and file bytes without following links."""
    def entry(path):
        mode = path.lstat().st_mode
        if stat.S_ISLNK(mode):
            return ("symlink", os.readlink(path))
        if stat.S_ISDIR(mode):
            return ("directory",)
        if stat.S_ISREG(mode):
            return ("file", path.read_bytes())
        return ("other", stat.S_IFMT(mode))

    try:
        result = {".": entry(root)}
    except FileNotFoundError:
        return {}
    if result["."][0] != "directory":
        return result

    def failed(error):
        raise error

    for directory, directories, files in os.walk(root, followlinks=False, onerror=failed):
        for name in directories + files:
            path = Path(directory) / name
            result[str(path.relative_to(root))] = entry(path)
    return result


def run_collections(run, binary, scratch, env):
    facts = {"status": "running", "evidence_class": "production_binary_skill_collection",
             "target": "gemini", "native_runtime_qualified": False, "commands": []}
    run.report["binary_skill_collections"] = facts
    run.save()
    try:
        _collections(run, binary, scratch, env, facts)
    except (OSError, ValueError, KeyError, IndexError, TypeError, AttributeError, RuntimeError) as error:
        facts.update(status="fail", error=str(error))
        run.save()
        raise RuntimeError("production collection proof failed: " + str(error)) from error
    facts["status"] = "pass"
    run.save()
    return True


def _collections(run, binary, scratch, env, facts):
    workspace = Path(scratch) / "binary skill collection workspace Ω"
    workspace.mkdir()
    source = workspace / "local skills with spaces"
    for name in NAMES:
        document = source / name / "SKILL.md"
        document.parent.mkdir(parents=True)
        document.write_bytes(skill_bytes(name))
    original_source = tree_bytes(source)
    home = Path(env["AGENTPLUGINS_HOME"])
    native = Path(env["GEMINI_CLI_HOME"]) / ".gemini"
    native.mkdir()
    settings = native / "settings.json"
    settings.write_bytes(FOREIGN_SETTINGS)
    foreign = native / "skills" / "foreign" / "SKILL.md"
    foreign.parent.mkdir(parents=True)
    foreign.write_bytes(FOREIGN_SKILL)
    empty_path = workspace / "empty executable path"
    empty_path.mkdir()
    child_env = dict(env, PATH=str(empty_path))

    def command(name, *args):
        facts["commands"].append(name)
        require(run.command(name, [binary, *args, "--scope", "user", "--format", "json"],
                            "production_binary_skill_collection", cwd=workspace, env=child_env),
                name + " failed")
        result = read_json(run.directory / (name + ".stdout"))
        require(isinstance(result, dict) and result.get("schema_version") == 1
                and result.get("result") == "success", name + " returned invalid or failed JSON")
        return result

    before = tree_bytes(home), tree_bytes(native), tree_bytes(source)
    listing = command("binary-collection-list", "skills", "list", source).get("data", {})
    require(listing.get("complete") is True and listing.get("revision") == ""
            and sorted(row.get("name") for row in listing.get("candidates", [])) == sorted(NAMES),
            "collection listing lost candidates or invented a Git revision")
    require((tree_bytes(home), tree_bytes(native), tree_bytes(source)) == before,
            "read-only collection listing changed entries or durable bytes")
    first = command("binary-collection-add", "skills", "add", source, "--all", "--target", "gemini", "--yes")
    require(first.get("result") == "success", "collection all-selection did not succeed")
    for name in NAMES:
        require((native / "skills" / name / "SKILL.md").read_bytes() == skill_bytes(name),
                "collection install differs from fixture bytes")
    durable = {name: (home / name).read_bytes() for name in ("state-v2.json", "uap.toml", "uap.lock")}
    command("binary-collection-repeat", "skills", "add", source, "--all", "--target", "gemini", "--yes")
    require(all((home / name).read_bytes() == value for name, value in durable.items()),
            "unchanged collection repeat changed state, requirements or lock")
    sibling = "collection-later"
    (source / sibling).mkdir()
    (source / sibling / "SKILL.md").write_bytes(skill_bytes(sibling))
    command("binary-collection-update", "update", "--all", "--yes")
    require(not (native / "skills" / sibling).exists()
            and sibling not in (home / "uap.toml").read_text(encoding="utf-8"),
            "update automatically enrolled an unselected sibling")
    for name in NAMES:
        command("binary-collection-remove-" + name, "remove", name, "--target", "gemini")
        require(not (native / "skills" / name).exists(), "removal retained owned Skill")
    require(settings.read_bytes() == FOREIGN_SETTINGS and foreign.read_bytes() == FOREIGN_SKILL,
            "collection lifecycle changed foreign native bytes")
    current_source = tree_bytes(source)
    require(all(current_source.get(relative) == value
                for relative, value in original_source.items()),
            "collection lifecycle changed source entries or bytes")
    facts["assertions"] = ["read_only_local_listing", "two_local_skills_all_selection",
                           "unchanged_repeat_no_drift", "update_does_not_enroll_sibling",
                           "owned_removal_preserves_foreign_bytes"]
