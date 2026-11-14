<!-- plugin-kit-ai:begin managed-guidance -->
# Plugin Boundary

This plugin uses the canonical `plugin/` authoring layout.

- Edit only files under `plugin/`.
- Start with [`plugin/README.md`](./plugin/README.md) for the full plugin guide.
- Read [`GENERATED.md`](./GENERATED.md) before touching plugin-root files. It is the required inventory of managed generated outputs.
- Treat plugin-root `README.md`, manifests, and config files as generated outputs.
- `plugin-kit-ai generate` may overwrite every path listed in `GENERATED.md`.

After changing `plugin/`, run:

```bash
plugin-kit-ai normalize .
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai inspect . --format json
```

Then run `plugin-kit-ai validate . --platform <enabled-target> --strict` for each enabled target listed in the inspection output.
<!-- plugin-kit-ai:end managed-guidance -->
