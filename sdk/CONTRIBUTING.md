# Contributing to plugin-kit-ai SDK

## Build and test

From the repository root (with `go.work`), `go test ./...` runs the **`repotests`** package (including **`TestSDKModule`**, which executes `go test ./...` inside this module):

```bash
go test ./...
```

Or directly in this module:

```bash
cd sdk && go test ./... && go vet ./...
```

## Pull requests

- Keep changes focused; match existing style and package boundaries (`ports` / `domain` / `usecase` / adapters / `claude` facade types).
- Add or update tests for behavior changes; prefer golden-style fixtures under `testdata/` when touching wire JSON.
- For larger design shifts, open a short discussion issue before a big refactor.

## Security

Report sensitive issues via the repository’s security advisory channel or maintainer contact listed in the root `README` / `SECURITY.md` when present. Do not file public issues for undisclosed vulnerabilities.
