# Release-bound Directory bootstrap inputs

Stable release validation requires three exact files in this directory:

- `snapshot.json`: signed Directory sequence 2 from publication run
  `32596313615`, digest
  `sha256:fe6422853423f447d797a54c5c2af0b0eda6f89c23815f8945f5b6f48d50a460`;
- `envelope.json`: its byte-exact detached Ed25519 envelope from immutable
  sequence tag commit `8b6bf802419d7566a6313be748e9d8a4dd23bb26`;
- `trusted-keys.json`: the reviewed schema-1 trust document from exact source
  commit `20f8f0b85a38e7291d6e9c133c548a5316e314c8`.

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
