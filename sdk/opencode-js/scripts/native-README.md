# Real native SDK qualification

These TypeScript fixtures load the **packed** SDK in actual OpenCode server
plugins. All model traffic uses a scripted loopback HTTP provider. Credentials
are test literals. Each invocation creates a fresh TEST Git project, HOME/XDG
profile, private server port, evidence log and owned process group. The first
argument must be an absolute path containing `TEST-`, outside a real project.
Node 24+ (TypeScript stripping), npm and Git are required. The command currently
targets Linux and the exact current V1 1.19.3 / V2 2.0.32 native executables.

From `sdk/opencode-js`, with dependencies installed using the repository lock:

```sh
NATIVE_ROOT=/absolute/isolated/TEST-uap-sdk-native
mkdir -p "$NATIVE_ROOT/tmp" "$NATIVE_ROOT/cache" "$NATIVE_ROOT/package" "$NATIVE_ROOT/tools"
export TMPDIR="$NATIVE_ROOT/tmp" npm_config_cache="$NATIVE_ROOT/cache"
npm ci --registry=https://registry.npmjs.org
./node_modules/.bin/tsc -p scripts/native-tsconfig.json
npm pack --ignore-scripts --pack-destination "$NATIVE_ROOT/package" --json > "$NATIVE_ROOT/pack.json"
ARCHIVE="$NATIVE_ROOT/package/universal-agent-plugins-opencode-events-0.3.0.tgz"
npm install --prefix "$NATIVE_ROOT/tools" --registry=https://registry.npmjs.org --legacy-peer-deps --ignore-scripts --no-audit --no-fund "$ARCHIVE" opencode-ai@1.19.3 @opencode/cli@2.0.32 @opencode/plugin@2.0.32 @opencode/client@2.0.32
node "$NATIVE_ROOT/tools/node_modules/opencode-ai/postinstall.mjs"
node "$NATIVE_ROOT/tools/node_modules/@opencode/cli/postinstall.mjs"
node scripts/native-qualify.ts "$NATIVE_ROOT" v1 "$ARCHIVE"
node scripts/native-qualify.ts "$NATIVE_ROOT" v2 "$ARCHIVE"
```

Each command prints its new `run-vN-TIMESTAMP` directory. `result.json` records
success only after scenario assertions. `evidence.jsonl` retains measured CLI and
server versions, V2 server PID, plugin execution location, installed SDK runtime
file hashes, tarball hash, original native envelopes, emitted facts, final native
terminal proofs, checkpoint consumption and actual host process closure.

Both strict subpaths cover completion, a genuinely unanswered question and
permission, question reply and question rejection/cancellation during held async
consumer preparation, permission rejection during preparation, permanent model
HTTP 400, disposal during pending preparation, and a successful new plugin after
native host reload. Fenced requests must produce zero attention facts. V2 owns an
actual public `ctx.event.subscribe` reader and `ctx.rpc.register` marker registration;
its proof ports read public session/context/permission APIs and native envelopes.
Only known irrelevant ephemeral session text deltas and usage updates are omitted
from durable sequence projection. No native event, run or request identity is made
up by the fixture. V1 has no public plugin cleanup hook: fixture control explicitly
calls the public SDK `dispose()` before the real instance disposal/reload. V2 uses
actual native location reload and the plugin's cleanup callback.

These tests qualify this Linux/current-host SDK contract. They do not qualify
older host floors, AN platform binaries, a full external consumer integration,
child/fork/workspace variants or every transport failure. Existing unit and
packed tests cover separate contracts. Preserve failed provisional evidence;
only explicit passing run directories belong in a release receipt.
