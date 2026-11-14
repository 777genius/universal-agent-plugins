# Local Skills picker API

`openapi.yaml` is the only editable transport schema. Go policy types stay outside
this package; HTTP adapters translate policy results to these generated models.

From the repository root (Go and Node >=22.18 installed):

```sh
node contracts/skills-picker/generate.mts
node contracts/skills-picker/generate.mts --check
```

Both generators are pinned. Check mode generates into an owned temporary directory,
compares committed bytes and leaves the checkout untouched. No server framework,
Go client or runtime OpenAPI interpreter is generated.

Canonical JSON examples live in
`cli/internal/agentpluginscli/skillspicker/testdata/api-v1/`. Frontend fixtures read
these files directly. Do not maintain a second schema or independently edited
copy of these examples in the UI.

The browser receives the capability in `#capability=...`, removes it before router
initialization, then sends an Authorization bearer header to same-origin API only.
