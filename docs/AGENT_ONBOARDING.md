# Adding an agent across the CLI, website and Directory

The existing Go client definition and registered adapters are the authority for
client identity and technical capabilities. The website and Registry consume
generated contracts. Publishing a new adapter does not authorize a particular
catalog package: the released CLI, package policy and dependency closure must
all support the requested installation.

Read [ARCHITECTURE.md](ARCHITECTURE.md), [CONTRIBUTING.md](../CONTRIBUTING.md)
and [AGENTS.md](../AGENTS.md) before changing a client. Check overlapping work
and preserve the recorded VS Code freeze. The synchronization rollout and its
production acceptance criteria are tracked in the
[implementation plan](plans/2026-11-14-agent-support-synchronization.md).

## 1. Define and qualify the native adapter

Add the canonical lower-kebab-case ID and definition in
`install/integrationctl/agentplugins/domain/clients_catalog.go`. Declare only
the supported components, transports, activation mode, saved-dependency
qualification and installation scopes. Project capabilities must describe
the actual Project adapter; they must not inherit unsupported User behavior.

Implement the native behavior under `clients/<id>` through the existing client
contracts. Register it in `clients/all` at the existing composition roots and
extend the existing contract and native lifecycle tests. Generic planners,
providers and use cases must not acquire client-ID branches. Test containment,
foreign-file ownership, receipts, rollback, update, repair and removal for the
affected filesystem and operating-system behavior.

The management API carries client IDs as strings so adding an adapter does not
require a second SDK enum. If passive discovery needs a new profile locator,
review the exact allowlisted environment names in
`sdks/typescript/src/discovery.ts` and the owned-process discovery test. Do not
pass credentials, proxy settings or arbitrary native launch arguments. Hermes
uses `HERMES_HOME` and `HERMES_DATA_DIR_SUFFIX`; those names are separate from
credential variables with the same prefix.

## 2. Generate the consumer contract and complete presentation

From the repository root:

```sh
go run ./cli/cmd/genclientcapabilities -output landing/data/clientCapabilities.generated.json
make client-contract-check
```

The same generator writes both files in `landing/data/`:

| File | Consumer contract |
| --- | --- |
| `clientCapabilities.generated.json` | Adapter-derived User and Project capabilities and saved-dependency qualification |
| `clientIds.generated.ts` | Canonical runtime ID tuple and the frontend `ClientID` type |

Do not edit these files by hand. The Directory decoder imports the generated
known-client set; it has no separate list to update. `-check` verifies both
artifacts without rewriting them, and `make generated-check` includes that
drift check. Changes to the definitions, client contracts or generator trigger
the Landing workflow and its contract check.

Website presentation remains explicit and reviewable. Add one entry to each of
`landing/data/clients.ts` and `landing/data/clientSupport.ts`, its icon under
`landing/public/client-icons/`, the matching README support-table row, and the
client presentation and support messages in all published locales. Keep the
website's intentional order. `hiddenPublicClientIds` is a presentation policy,
not technical removal of an adapter.

`make client-contract-check` detects stale generation, missing or duplicate
presentation/support rows, absent public icons, missing translations and
README drift. Its new-client fixture exists only in tests. Adding a client to
the canonical contract requires completing its presentation before a build
can pass; it never inserts an example production adapter.

Do not expand unrelated curated lists or historical wire contracts. In
particular, product-specific install lanes and old discovery-schema client
sets are not duplicate copies of the complete adapter registry.

## 3. Publish and pin the released capabilities in Registry

Qualify and release the CLI through the existing
[release runbook](agentplugins-release.md). Registry must consume the exact
released contract, not an uncommitted working tree or mutable upstream main.

In `777genius/universal-agent-plugins-registry`, the contract snapshot is
`registry/client-capabilities.json`. Its adjacent
`client-capabilities.lock.json` records the release, immutable UAP commit,
source path and SHA-256. To select a locally available released tag:

```sh
python3 scripts/sync_client_contract.py --from-uap ../universal-agent-plugins --release agentplugins-v<VERSION>
python3 scripts/sync_client_contract.py --check
```

Replace `<VERSION>` with the actual qualified release. Review the snapshot,
lock and generated schema changes together. `--write` regenerates schemas
from the existing pin; `--check` validates them offline without updating the
pin. The Registry validation workflow runs that check before publication.
The first synchronization pin uses the unchanged 0.1.108 capability JSON.

The pinned IDs define admission to modern publication tooling. They do not
grant package eligibility. Preserve the frozen historical signer and wire
client subset for both target and evidence records, including revoked or
suspended policies. Modern readers may retain a well-formed future identity
after authenticating the complete signed document, but cannot select it or
let it authorize a known adapter.

## 4. Qualify packages and complete the public journey

For every proposed catalog target, qualify the exact immutable package source,
required components, transports, dependency closure and native behavior.
Publish only with the applicable reviewed evidence, issuer and protected
promotion process. The independent modern feed needs its own approved trust,
origin and ledger; a fixture signature is not a production release.

Schema-1 target grants remain `scopes: ["user"]`. Explicit Project placement
continues through the existing source and native Project policy checks.
Generated Project capabilities cannot fabricate a signed Project grant.
Standalone Skills use their existing independent installation path. Direct
source alternatives require the existing exact-source and scope checks.

Preserve the distinctions the user needs to understand:

| Fact | Required evidence |
| --- | --- |
| Adapter supports a component | Generated released capability and native contract |
| A package is available for an agent | Valid signed package policy, exact source and complete required dependencies |
| Files are prepared | Successful CLI installation and owned native state |
| Agent has activated the integration | Actual native discovery, activation and runtime evidence as required |

Hermes Project remains Skills-only: a package requiring Project MCP must
refuse. Antigravity remains Prepared until the user completes native
activation. Pi extension support applies to its supported namespace. Never
silently remove required components, change the user's scope, or convert a
plugin into a Skills-only installation to make a target appear available.

## Completion checks

Run the repository's applicable gates from CONTRIBUTING, including `make lint`,
`make test-core`, the generated-contract check and affected native lifecycle
tests. Use the pinned TypeScript 7.0.2 or newer toolchain for the frontend.
Verify hero, cards, detail pages and standalone Skills at desktop and mobile
sizes, in the affected locales, including unavailable and Prepared states.

After the reviewed heads are merged and publication completes, verify the
released binary, signed snapshot and dependency policies, protected promotion
receipt, website source/tree provenance and the exact command generated by the
deployed UI for the chosen agent and scope. Exercise install, info, update,
repair and remove on disposable profiles across the applicable operating
systems. A pending, skipped, synthetic or evidence-only workflow is not a
successful published installation. Record any remaining production
prerequisite explicitly instead of declaring the rollout complete.
