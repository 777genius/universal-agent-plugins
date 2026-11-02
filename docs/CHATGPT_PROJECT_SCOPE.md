# ChatGPT and project scope

UAP does not qualify ChatGPT for native project installation. Selecting
`--scope project` with `--target chatgpt` returns:

```text
chatgpt does not support project scope
```

The refusal applies to saved additions, imperative `--no-save` additions,
positional `install`, updates, repairs, removal, mixed target requests and
personal preparation options. Completion or external-uninstall attestations
cannot establish a supported project lifecycle. Omitting `--project-root`
uses the invocation directory and preserves this same refusal.

Do not add `chatgpt` to a project `uap.toml` target table. Dependency lock,
update, sync and environment install reject that declaration before source
acquisition or project mutation. Lock/update perform an early read-only target
check before creating an environment lock, then retain the normal authoritative
manifest reread and validation inside the locked operation.

Unsupported project requests do not create a local preparation artifact,
`uap.lock`, mutable `.uap` state or native configuration. An existing portable
manifest remains unchanged. UAP does not substitute a user profile, connect an
app, grant trust, or claim that a remote ChatGPT installation was observed.

## Existing user preparation

The existing user-scoped personal/manual preparation workflow remains
available under its existing validation requirements. For example, a validated
personal Context7 registration can be prepared explicitly in user scope:

```sh
agentplugins add context7 --target chatgpt --scope user --chatgpt-app-id <your-registration-id>
```

Use the registration ID obtained through the existing ChatGPT setup flow.
This produces local preparation and manual next steps; it does not certify
remote app connection, account/workspace installation, authentication or native
project discovery. ChatGPT remains excluded from saved dependency targets.

The accepted scope decision and evidence are recorded in
[the reviewed client plan](plans/project-install-e2e/client-plans.md#13-chatgpt--explicit-unsupported-project-installation).
