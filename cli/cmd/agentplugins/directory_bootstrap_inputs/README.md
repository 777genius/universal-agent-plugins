# Release-bound Directory bootstrap inputs

Stable release validation requires three exact files in this directory:

- `snapshot.json`: signed Directory sequence 43 from publication run
  `40229838422`, digest
  `sha256:7ffb6078ac08e7ed00575dbf9a871b0dba53c755591d8983dd9399bfdc4edaf6`;
- `envelope.json`: its byte-exact detached Ed25519 envelope from immutable
  protected ledger snapshot sequence 43 (published 2026-10-25);
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
