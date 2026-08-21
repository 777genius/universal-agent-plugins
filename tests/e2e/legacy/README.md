# Legacy launch observations

Files in this directory preserve historical observations at their original
scope and revision. They are not inputs to the schema-3 stable launch gate and
must not be linked as current release evidence.

`launch-evidence-host-2026-08-20.json` predates immutable GitHub release,
signed-Directory, challenge, native-platform, and signed-observer binding. Its
many `not_tested` rows remain unchanged; moving it here prevents the canonical
client-evidence validator from misclassifying it as a current client result.
