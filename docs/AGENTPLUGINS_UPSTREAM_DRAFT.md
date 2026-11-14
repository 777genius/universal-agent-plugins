# Local upstream proposal drafts

Status: not submitted, not reviewed, not listed, not endorsed. Prepared on
2026-09-06. Publication and maintainer contact require explicit owner approval.
No release or native runtime proof for the compatibility candidate is claimed.

## Informational tooling proposal

Universal Agent Plugins (UAP) is an independent cross-client installer and
lifecycle manager for Agent Plugins packages. Selected clients provide the
runtime. Local and immutable Git installation work independently of the optional
Directory. Published Agent Plugins text and schemas remain the portable contract.

Would an informational installation-tooling entry be useful, or does the current
compatible-client category require a runtime? We propose one product-family
entry, with per-target limitations in a linked version-bound matrix. We would
not combine unrelated adapter capabilities into a universal supports claim.
An installation-tooling section is a proposal, not an existing acceptance path.

Source: [UAP](https://github.com/777genius/universal-agent-plugins).
[Release preparation and evidence limits](AGENTPLUGINS_RELEASE_PREPARATION.md)
and [existing lifecycle record](AGENTPLUGINS_CLIENT_E2E.md) distinguish source,
projection, discovery, activation, tool execution and OAuth evidence.

Before submission, attach the actual compatibility release/version, setup guide,
artifact provenance, completed matrix, reproducible demo and logo provenance.
These attachments are pending, not implied by this draft. Align the eventual
entry with [the pinned submission guidance](https://github.com/agentplugins/agent-plugins-site/blob/c4a3a8dc683838f14d178392aaeaabeba4533377/CONTRIBUTING.md).
Informational inclusion would not mean endorsement or certification. Any reference
implementation discussion would need a separate bounded scope and governance
decision, reproducible release ownership and demonstrated implementer usefulness.

## Published 1.0 extension clarification

For the same otherwise valid package containing a healthy skill, compare:

```json
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"example","extensions":null}
```

```json
{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"example","extensions":{"com.example.future":"opaque"}}
```

Both fail the original author schema. For the first, our installer reports
`plugin_extensions_ignored` and loads the healthy skill under the published 1.0
container exception. For the second, it currently rejects the manifest. One
reading applies the object-member requirement before namespace interpretation;
the other ignores an unimplemented namespace before checking its contents.
Please clarify the published 1.0 loader disposition and reporting obligation for
the second input independently of author-schema validity and draft 1.1 changes.

The [local profile decision](../install/integrationctl/agentplugins/conformance/profiles/README.md)
records the pinned corpus fixture, disputed status and unchanged policy. No
canonical schema change or assertion of official certification is proposed.

Candidate lifecycle limitation: same-source, same-revision update normally returns
`NoChange` and does not refresh renderer/helper bytes. The local unreleased
candidate is implementing a narrow explicit-update exception to withdraw proven
previously delivered MCP components now statically unsupported, such as historical
OpenCode SSE. Add/repair must not reactivate those components and instead explain
controlled update. This is not a shipped claim or a completed verification claim. New-revision update requires the old
artifact to pass integrity checks. Damaged historical artifacts require verified
exact repair or an explicitly verified recovery path; this proposal does not
promise that upgrading the CLI and running update repairs them. See the
[historical projection limits](AGENTPLUGINS_RELEASE_PREPARATION.md#updating-historical-projections).
