"""Finite failed-poll facts; no pending approval value leaves quarantine."""


FIELDS = frozenset(("topic", "status", "reason", "polls"))
REASONS = frozenset(("multiple-pending", "no-pending-after-polls"))


def validate(value):
    if (type(value) is not dict or any(type(key) is not str for key in value)
            or set(value) != FIELDS
            or any(type(value[key]) is not str for key in ("topic", "status", "reason"))
            or value["topic"] != "single-approval-poll" or value["status"] != "refused"
            or value["reason"] not in REASONS or type(value["polls"]) is not int
            or not 1 <= value["polls"] <= 20
            or value["reason"] == "no-pending-after-polls" and value["polls"] != 20):
        raise ValueError("Kimi finite approval poll facts refused")
    return {key: value[key] for key in ("topic", "status", "reason", "polls")}


class Refusal(ValueError):
    """The original polling refusal with only a fixed reason and held ordinal."""

    def __init__(self, reason, polls):
        self.facts = validate({"topic": "single-approval-poll", "status": "refused",
                               "reason": reason, "polls": polls})
        message = ("Kimi has foreign or multiple pending approvals" if reason == "multiple-pending"
                   else "Kimi exact native manual approval was not observed")
        super().__init__(message)

    def public(self):
        return validate(self.facts)
