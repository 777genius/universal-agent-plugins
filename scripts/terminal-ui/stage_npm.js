// Synthetic local-only package: exercise the unchanged launcher and digest seam.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const [source, binary, root] = process.argv.slice(2);
const { detectPlatform, expectedAssetName } = require(path.join(source, 'lib/platform'));
const { stageEvidence } = require(path.join(source, 'scripts/stage-release'));
const platform = detectPlatform();
const version = '0.0.1-terminal-fixture';
fs.mkdirSync(root);
for (const dir of ['bin', 'lib']) fs.cpSync(path.join(source, dir), path.join(root, dir), {recursive:true});
const evidence = stageEvidence(root, path.join(source, 'test/fixtures/historical-evidence'));
const bytes = fs.readFileSync(binary);
const file = expectedAssetName(version, platform);
fs.writeFileSync(path.join(root, 'package.json'), JSON.stringify({name:'universal-agent-plugins', version}));
fs.writeFileSync(path.join(root, 'assets.json'), JSON.stringify({schema_version:2, version,
  npm_package:'universal-agent-plugins', repository:'777genius/plugin-kit-ai', tag:`agentplugins-v${version}`,
  producer:{repository:'777genius/plugin-kit-ai', tag:`agentplugins-v${version}`, commit:'a'.repeat(40),
    release_manifest:{schema_version:2, sha256:'b'.repeat(64), version}},
  client_evidence:evidence, assets:{[platform.key]:{file, size:bytes.length, sha256:crypto.createHash('sha256').update(bytes).digest('hex')}}}));
fs.writeFileSync(path.join(root, file), bytes, {mode:0o700});
process.stdout.write(JSON.stringify({asset:path.join(root,file), launcher:path.join(root,'bin/agentplugins.js')}));
