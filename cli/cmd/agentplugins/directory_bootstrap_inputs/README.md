# Release-bound Directory bootstrap inputs

Stable release validation requires three exact files in this directory:

- `snapshot.json`: signed Directory sequence 38 from publication run
  `35333960862`, digest
  `sha256:fb1817f377adc3008bf1d20493599f688f96d08511bd33fcb5c0409867547c28`;
- `envelope.json`: its byte-exact detached Ed25519 envelope from immutable
  sequence tag commit `fc7a74f4cb675826f5168a1385b5a3d80c358a7f`;
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
