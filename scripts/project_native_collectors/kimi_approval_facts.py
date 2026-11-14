"""Closed failure-only facts; no native value leaves approval quarantine."""
import copy
import math
import re


FIELDS = ("approval_id", "session_id", "agent_id", "tool_call_id", "tool_name", "action",
          "tool_input_display", "created_at", "expires_at", "turn_id")
TYPES = frozenset(("absent", "null", "boolean", "integer", "number", "string", "array", "object", "other"))
EQUALS = ("session", "agent", "call", "tool", "action", "display")
REFUSED = {"topic": "single-approval", "status": "projection-refused"}
KEYS = frozenset((*REFUSED, "row_type", "exact_keys", "fields", "equals", "approval_id_valid",
                  "created_at_valid", "expires_at_valid", "turn", "display_object", "display_keys"))


def _bounded(value):
    pending, nodes, characters = [(value, 0)], 0, 0
    while pending:
        item, depth = pending.pop()
        nodes += 1
        if nodes > 16384 or depth > 32:
            return False
        kind = type(item)
        if kind is str:
            characters += len(item)
        elif kind is dict:
            if nodes + len(pending) + 2 * len(item) > 16384:
                return False
            for key, child in item.items():
                if type(key) is not str:
                    return False
                pending.extend(((key, depth + 1), (child, depth + 1)))
        elif kind is list:
            if nodes + len(pending) + len(item) > 16384:
                return False
            pending.extend((child, depth + 1) for child in item)
        elif kind is float:
            if not math.isfinite(item):
                return False
        elif kind not in (type(None), bool, int):
            return False
        if characters > 1 << 20:
            return False
    return True


def _type(value):
    return {type(None): "null", bool: "boolean", int: "integer", float: "number",
            str: "string", list: "array", dict: "object"}.get(type(value), "other")


def project(value, session_id, call_id, args, tool, instant):
    # Validate builtin types and bounds before equality, regex or iteration
    # beyond the bounded walk. Never invoke a hostile object's repr/eq method.
    if (not _bounded(value) or not _bounded([session_id, call_id, args, tool])
            or any(type(item) is not str for item in (session_id, call_id, tool)) or type(args) is not dict):
        return dict(REFUSED)
    row = value if type(value) is dict else {}
    action = "Approve " + tool
    display = row.get("tool_input_display")
    turn = row.get("turn_id")
    fields = {key: {"present": key in row, "type": _type(row[key]) if key in row else "absent"} for key in FIELDS}

    def valid_stamp(key):
        try:
            instant(row.get(key))
        except (ValueError, TypeError):
            return False
        return True

    result = {"topic": "single-approval", "status": "captured", "row_type": _type(value),
              "exact_keys": type(value) is dict and set(row) == set(FIELDS), "fields": fields,
              "equals": {key: row.get(field) == expected for key, field, expected in (
                  ("session", "session_id", session_id), ("agent", "agent_id", "main"),
                  ("call", "tool_call_id", call_id), ("tool", "tool_name", tool),
                  ("action", "action", action), ("display", "tool_input_display", {
                      "kind": "generic", "summary": action, "detail": args}))},
              "approval_id_valid": type(row.get("approval_id")) is str and bool(re.fullmatch(
                  r"approval_[A-Za-z0-9_-]{1,128}", row["approval_id"])),
              "created_at_valid": valid_stamp("created_at"), "expires_at_valid": valid_stamp("expires_at"),
              "turn": ("negative" if turn < 0 else "positive" if turn > 0 else "zero") if type(turn) is int else "noninteger",
              "display_object": type(display) is dict,
              "display_keys": type(display) is dict and set(display) == {"kind", "summary", "detail"}}
    return validate(result)


def validate(value):
    if not _bounded(value) or type(value) is not dict:
        raise ValueError("Kimi finite approval facts refused")
    if value == REFUSED:
        return dict(REFUSED)
    if set(value) != KEYS or value["topic"] != "single-approval" or value["status"] != "captured":
        raise ValueError("Kimi finite approval facts refused")
    if (type(value["row_type"]) is not str or value["row_type"] not in TYPES or type(value["turn"]) is not str
            or value["turn"] not in ("negative", "zero", "positive", "noninteger")):
        raise ValueError("Kimi finite approval facts refused")
    flags = ("exact_keys", "approval_id_valid", "created_at_valid", "expires_at_valid", "display_object", "display_keys")
    if any(type(value[key]) is not bool for key in flags):
        raise ValueError("Kimi finite approval facts refused")
    fields, equal = value["fields"], value["equals"]
    if type(fields) is not dict or set(fields) != set(FIELDS) or type(equal) is not dict or set(equal) != set(EQUALS):
        raise ValueError("Kimi finite approval facts refused")
    for field in FIELDS:
        fact = fields[field]
        if (type(fact) is not dict or set(fact) != {"present", "type"} or type(fact["present"]) is not bool
                or type(fact["type"]) is not str or fact["type"] not in TYPES
                or fact["present"] != (fact["type"] != "absent")):
            raise ValueError("Kimi finite approval facts refused")
    if any(type(equal[key]) is not bool for key in EQUALS):
        raise ValueError("Kimi finite approval facts refused")
    return copy.deepcopy(value)


class Refusal(ValueError):
    """The original strict refusal, with separately validated optional facts."""

    def __init__(self, facts):
        self.facts = validate(facts)
        super().__init__("Kimi selected approval refused")

    def public(self):
        return validate(self.facts)
