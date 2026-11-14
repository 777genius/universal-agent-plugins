"""Closed pure contract for the dedicated, unreviewed Cline recovery draft."""
import hashlib
import json
import re

NATIVE = "cline-project-evidence.json"
FRONTEND = "cline-project-frontend.json"
FRONT_LABELS = frozenset(("frontend-commit", "frontend-tree", "frontend-clean", "frontend-go-version", "frontend-build", "frontend-gate"))
CLASSES = {
    "git": ("git-user", "git-A", "git-B"),
    "public_uap": ("add-user", "add-A", "add-B", "repeat-add-A", "unchanged-update-A", "intact-repair-A", "update-A", "missing-repair-A", "remove-A", "add-same-user", "add-same-A", "add-same-B"),
    "native": ("version", "initial-A-skills", "initial-B-skills", "default-A", "default-B", "override-A", "override-B", "cross-A-from-B", "restart-default-A", "stable-A-skills", "stable-A-mcp", "updated-A-skills", "updated-A-mcp", "repaired-A-skills", "repaired-A-mcp", "removed-A-skills", "removed-A-mcp", "same-user-skills", "same-A-skills", "same-B-skills", "same-user-mcp", "same-A-mcp", "same-B-mcp"),
    "cleanup": ("cleanup",),
}
NATIVE_LABELS = {label: kind for kind, labels in CLASSES.items() for label in labels}
EXTRAS = frozenset(("prepared-user-config.json", "prepared-A-config.json", "prepared-B-config.json", "updated-A-config.json", "repaired-A-config.json", "removed-A-config.json", "prepared-A-inspection.json", "prepared-B-inspection.json", "repaired-A-inspection.json", "foreign-snapshots.json", "same-name-winners.json", "same-user-config.json", "same-A-config.json", "same-B-config.json"))
NATIVE_FALSE = ("native_project_activation_qualified", "native_mcp_initialization_qualified", "native_runtime_qualified", "native_model_use_qualified", "native_ide_qualified", "native_trust_qualified", "same_name_active_isolation_qualified", "native_skill_execution_qualified", "native_authentication_qualified", "native_session_policy_qualified")
FRONT_FALSE = NATIVE_FALSE
STAGES = frozenset(("source_install", "unique_discovery", "stable_lifecycle", "update", "missing_repair", "remove", "same_name_precedence", "foreign_preservation"))


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate Cline evidence JSON key")
        result[key] = value
    return result


def invalid_constant(_value):
    raise ValueError("nonfinite Cline evidence JSON number")


def document(body):
    try:
        value = json.loads(body.decode("utf-8"), object_pairs_hook=unique_object, parse_constant=invalid_constant)
    except (UnicodeError, ValueError, RecursionError) as error:
        raise ValueError("ordinary bounded Cline evidence JSON required") from error
    if not isinstance(value, dict):
        raise ValueError("Cline evidence object required")
    return value


def summary(body, false_flags):
    value = document(body)
    if type(value.get("schema_version")) is not int or value["schema_version"] != 1 or value.get("client") != "cline" or value.get("version") != "3.0.73":
        raise ValueError("closed Cline evidence identity differs")
    if type(value.get("collection_complete")) is not bool:
        raise ValueError("explicit Cline collection result required")
    for key in false_flags:
        if value.get(key) is not False:
            raise ValueError("unobserved Cline qualification must remain false")
    if value["collection_complete"] and (value.get("cleanup_complete") is not True or value.get("read_only_unique_marker_discovery_qualified") is not True):
        raise ValueError("completed Cline evidence lacks measured discovery/cleanup")
    return value


def streams(value, bodies, labels, kind=None):
    commands = value.get("commands")
    if not isinstance(commands, list) or len(commands) > len(labels):
        raise ValueError("closed Cline command count refused")
    seen, expected = set(), set()
    for command in commands:
        if not isinstance(command, dict):
            raise ValueError("Cline command object required")
        label = command.get("label")
        if not isinstance(label, str) or label not in labels or label in seen:
            raise ValueError("unknown/duplicate Cline command label")
        seen.add(label)
        wanted_class = kind if kind is not None else labels[label]
        if command.get("class") != wanted_class:
            raise ValueError("Cline command class differs")
        if kind is None and command.get("in_process") is not (wanted_class == "public_uap"):
            raise ValueError("Cline public Cobra/process boundary differs")
        if value["collection_complete"] and (type(command.get("exit")) is not int or command["exit"] != 0 or command.get("error") not in (None, "") or command.get("overflow", False) is not False):
            raise ValueError("completed Cline collection contains failed command")
        for stream in ("stdout", "stderr"):
            name = label + "." + stream + ".log"
            body = bodies.get(name)
            if command.get(stream + "_file") != name or not isinstance(body, bytes) or len(body) > 128 << 10:
                raise ValueError("closed bounded Cline stream refused")
            if type(command.get(stream + "_bytes")) is not int or len(body) != command[stream + "_bytes"] or hashlib.sha256(body).hexdigest() != command.get(stream + "_sha256"):
                raise ValueError("Cline stream length/hash differs")
            expected.add(name)
    if value["collection_complete"] and seen != set(labels):
        raise ValueError("completed Cline collection omitted fixed commands")
    return expected


def validate(bodies):
    if FRONTEND not in bodies:
        raise ValueError("Cline frontend receipt missing")
    frontend = summary(bodies[FRONTEND], FRONT_FALSE)
    expected = {FRONTEND} | streams(frontend, bodies, FRONT_LABELS, "frontend")
    if NATIVE in bodies:
        native = summary(bodies[NATIVE], NATIVE_FALSE)
        captured = streams(native, bodies, NATIVE_LABELS)
        extras = set(bodies) & EXTRAS
        for name in extras:
            document(bodies[name])
        hashes = native.get("artifact_sha256")
        if not isinstance(hashes, dict) or set(hashes) != captured | extras:
            raise ValueError("Cline native artifact receipt membership differs")
        for name in captured | extras:
            if hashlib.sha256(bodies[name]).hexdigest() != hashes[name]:
                raise ValueError("Cline native artifact SHA256 differs")
        for key in ("source_commit", "source_tree"):
            if not isinstance(native.get(key), str) or not re.fullmatch(r"[0-9a-f]{40}", native[key]) or native[key] != frontend.get(key):
                raise ValueError("Cline source commit/tree differs")
        if frontend["collection_complete"] and not native["collection_complete"]:
            raise ValueError("frontend promoted incomplete native evidence")
        if native["collection_complete"]:
            stages = native.get("stages")
            if not isinstance(stages, dict) or set(stages) != STAGES or native.get("same_name_observation_complete") is not True:
                raise ValueError("completed Cline stage membership differs")
            for name, stage in stages.items():
                wanted = "blocked" if name == "same_name_precedence" else "passed"
                if not isinstance(stage, dict) or stage.get("status") != wanted:
                    raise ValueError("Cline measured stage result differs")
            if extras != set(EXTRAS):
                raise ValueError("completed Cline source evidence omitted")
        expected |= {NATIVE} | captured | extras
    elif frontend["collection_complete"]:
        raise ValueError("completed Cline frontend lacks actual native receipt")
    if set(bodies) != expected:
        raise ValueError("unrecorded Cline artifact member refused")
