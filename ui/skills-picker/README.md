# Local Skills picker

React frontend served by the Go CLI's loopback bridge. The built binary needs no Node runtime.
Run `npm ci`, `npm run dev` for development (API proxy: `127.0.0.1:4174`), or
`npm run bundle:go` to build and copy hashed production assets to the Go embed package.
Use Node 22.18+ for the TypeScript bundle helper; pinned TypeScript 5.9 checks every handwritten source.

`npm run test:model` protects global selection and eligible folder checks.
`npm test -- tests/browser/picker.spec.ts` runs risk-focused browser flows against canonical backend
contract fixtures. The adapter exists exclusively under `tests/`; production has only real API fetches.
The joined `live-bridge.spec.ts` suite is maintained by the integration lane.

The editable API schema is `contracts/skills-picker/openapi.yaml`. Regenerate transport declarations
using the repository's contract-generation command; do not edit `schema.d.ts` manually.
Feature public exports are the composition boundary. Catalog owns Headless Tree and Virtual;
selection owns the ID basket; install owns targets/review/progress. Source/install policy remains in Go.
