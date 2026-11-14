# Pi native extensions

UAP materializes one admitted package directly in
`<Pi agent directory>/extensions/<physicalArtifactID>`. The existing package
directory transaction owns installation, update, repair, rollback and removal.
No Pi registry command or second native copy transaction is involved.

The standard manifest opts in through one bounded namespace:

```json
{"extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"com.universal-agent-plugins.pi"}}}
```

The source is a contained real directory with exactly one `index.ts` or
`index.js`. UAP rejects source links, traversal, reserved root entrypoints and
Pi `package.json` discovery overrides. Unknown namespaces stay unsupported.
Namespace compatibility is a pure metadata check; staging separately validates
the sealed package's files. UAP never executes extension code during installation.

The generated root factory compares its physical filename to the frozen active
path before importing source code. Its descriptor contains schema/bootstrap
versions, active/package/data roots and the canonical source digest. The source
factory receives `(pi, descriptor)`. The expected path is never re-resolved
through a changed symlink. Staging lives outside the watched extensions root;
retained copies and explicitly loaded backups cannot activate source code.

The physical agent directory defaults to `~/.pi/agent` and honors
`PI_CODING_AGENT_DIR`. Running Pi must reload resources or restart after changes.
Filesystem verification proves installed discovery bytes, not loaded runtime
activation. Edited packages are preserved and refused by repair; absent owned
packages can be restored. Foreign files and masking root entrypoints are never
adopted or overwritten.

The runtime-only `sdk/pi` carrier exposes `App.Pi().OnObservation` and its context
variant through the fixed `pi-event` process invocation. It accepts one strict
64 KiB JSON frame for `agent_settled` or `ui_prompt_start`, with observer/session
identity and captured loaded revision. Its output is neutral and cannot request
continuation, execution or approval. Consumers independently authenticate the
installed binding and decide notification policy; decoding alone grants none.

The disposable driver under `clients/pi/testdata/driver` calls the public
installer facade for native integration tests. It is not shipped as a command.
