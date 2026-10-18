# Release-bound Directory bootstrap inputs

Stable release validation requires three exact files in this directory:

- `snapshot.json`: signed Directory sequence 39 from publication run
  `36769713154`, digest
  `sha256:4d3d7755d89123aa238b28e6ffb92d01ec20b70586dbc292293d6b4d5524f532`;
- `envelope.json`: its byte-exact detached Ed25519 envelope from immutable
  sequence tag commit `298a9af013354cae1b9a2091f4be225debb41b5a`;
- `trusted-keys.json`: the reviewed schema-1 trust document, unchanged from
  the previous bootstrap, with digest
  `sha256:cbcc644df25cfe12b7f7cda236c95594a12f31714569e957d62723d334a1c198`.

Do not edit or reserialize these files. From `cli`, reproduce the
compiled Go bootstrap only from these publication-ledger artifacts:

```sh
go run ./cmd/agentplugins/bootstrapgen \
  -snapshot cmd/agentplugins/directory_bootstrap_inputs/snapshot.json \
  -envelope cmd/agentplugins/directory_bootstrap_inputs/envelope.json \
  -trust cmd/agentplugins/directory_bootstrap_inputs/trusted-keys.json \
  -expected-key-id uap-directory-2026-01 \
  -expected-public-key HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI= \
  -output cmd/agentplugins/directory_bootstrap_generated.go
```

The stable workflow reruns the same verification, checks byte-for-byte source
reproducibility, and rejects a snapshot that is not valid at release time.
