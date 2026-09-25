# Installer automation principle

Universal Agent Plugins should make installing one plugin across several agents feel like one task, not a sequence of client-specific setup guides. The default path is: detect compatible clients, let the user choose, validate the complete plan, install through supported client interfaces, verify what was installed, and explain the result.

## Product rule

- Automate every step for which the client exposes a documented, ownership-safe interface. A user should not have to copy paths, edit JSON, or repeat commands that the installer can perform and verify.
- Ask the user only for a real external boundary: consent to the plan, client/account authentication, a client restart or reload that cannot be invoked safely, a required client-side approval, or resolution of an ownership conflict.
- Prefer a precise partial result over a misleading success claim. Package validation, configuration delivery, client discovery, callable runtime tools, and authentication are different evidence levels. `AUTO` means the planned installation action is automatic; it does not claim that an already-running client has reloaded or that every tool call was tested.
- Keep failure recovery as convenient as installation: preserve foreign data, make retries idempotent, and give one specific next action when automation cannot continue.

## OpenCode namespace rule

OpenCode can normalize different MCP server/tool names to the same callable tool ID. The installer should check the effective names it can observe before claiming namespace safety, prevent a known collision before changing configuration, and automatically choose a safe delivery route when that can be done without changing the plugin's meaning or another owner's configuration. Unknown project-local servers, dynamic tool catalogs, and client-version changes must remain explicit limits rather than silently becoming `AUTO` proof.

## Acceptance target

For each supported client and package shape, maintain a disposable end-to-end test that covers initial install, repeat install, update, repair, removal, and the strongest available client-side verification. Track any remaining manual step by its external reason and revisit it when the client gains a safe automation API. Never use real user projects to qualify an installer flow.
