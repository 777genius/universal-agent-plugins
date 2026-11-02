'use strict';
// Read proposed metadata only; never start/gateway MCP, invoke prompts or grant trust.
const fs = require('node:fs');
const path = require('node:path');
const shape = require('./shape.cjs');
const LIMIT = 512 * 1024;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
function store(output, result, fileSystem = fs) {
  if (!output || !path.isAbsolute(output) || path.basename(output) !== 'observation.json') throw new Error('fixed result path required');
  const body = Buffer.from(JSON.stringify(result) + '\n', 'utf8');
  if (body.length > LIMIT) throw new Error('observation JSON exceeds bound');
  const parent = path.dirname(output);
  const pending = path.join(parent, '.observation.pending');
  const ready = path.join(parent, 'observation.ready');
  const fd = fileSystem.openSync(pending, fs.constants.O_WRONLY | fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_NOFOLLOW, 0o600);
  try { fileSystem.writeFileSync(fd, body); fileSystem.fsyncSync(fd); } finally { fileSystem.closeSync(fd); }
  fileSystem.linkSync(pending, output);
  fileSystem.unlinkSync(pending);
  const gate = fileSystem.openSync(ready, fs.constants.O_WRONLY | fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_NOFOLLOW, 0o600);
  fileSystem.closeSync(gate);
}
exports.storeResult = store;
async function observe() {
  const vscode = require('vscode');
  const folders = vscode.workspace.workspaceFolders;
  if (!Array.isArray(folders) || folders.length !== 1) throw new Error('exactly one selected folder required');
  const folder = folders[0];
  const selected = shape.uri(folder.uri);
  if (selected.scheme !== 'file' || selected.authority !== '' || folder.uri.fsPath !== process.env.UAP_VSCODE_WORKSPACE_FOLDER || vscode.workspace.isTrusted !== false) throw new Error('selected folder or untrusted context differs');
  const config = vscode.workspace.getConfiguration(undefined, folder.uri);
  const policy = {skills_enabled: config.get('chat.useAgentSkills'), mcp_access: config.get('chat.mcp.access'), mcp_autostart: config.get('chat.mcp.autostart')};
  shape.flag(policy.skills_enabled); shape.text(policy.mcp_access);
  if (policy.mcp_autostart !== 'never') throw new Error('restrictive MCP autostart setting differs');
  const extension = vscode.extensions.getExtension('uap-research.workspace-observer');
  if (!extension || extension.id !== 'uap-research.workspace-observer') throw new Error('observer identity missing');
  const activated = await extension.activate();
  if (!activated || !Number.isInteger(activated.extensionMode)) throw new Error('actual activation context mode missing');
  const proposals = shape.texts(extension.packageJSON.enabledApiProposals);
  if (JSON.stringify(proposals) !== JSON.stringify(['mcpServerDefinitions','chatPromptFiles'])) throw new Error('observer proposal set differs');
  if (typeof vscode.chat?.getSkills !== 'function') throw new Error('proposed skill API missing or denied');
  const deadline = performance.now() + 15000;
  let inventory;
  do {
    const token = new vscode.CancellationTokenSource();
    let timer;
    try {
      const remaining = Math.max(1, deadline - performance.now());
      const rawSkills = await Promise.race([vscode.chat.getSkills(token.token), new Promise((_, reject) => {timer = setTimeout(() => {token.cancel(); reject(new Error('skill metadata API deadline'));}, remaining);})]);
      inventory = {mcp: shape.mcp(vscode.lm.mcpServerDefinitions), skills: shape.skills(rawSkills)};
      if (Buffer.byteLength(JSON.stringify(inventory), 'utf8') > LIMIT - 16384) throw new Error('metadata inventory exceeds result bound');
    } finally { clearTimeout(timer); token.cancel(); token.dispose(); }
    const remaining = deadline - performance.now();
    if (remaining > 0) await sleep(Math.min(500, remaining));
  } while (performance.now() < deadline);
  if (vscode.workspace.isTrusted !== false || vscode.workspace.workspaceFolders?.length !== 1 || vscode.workspace.workspaceFolders[0].uri.fsPath !== folder.uri.fsPath || config.get('chat.mcp.autostart') !== 'never') throw new Error('workspace context or restrictive policy changed');
  return {schema_version:1, collected:true, version:shape.text(vscode.version), app_root:shape.text(vscode.env.appRoot), app_host:shape.text(vscode.env.appHost),
    observer:{id:extension.id, mode:activated.extensionMode, proposals}, folder:selected, trusted:false, policy, ...inventory,
    native_project_qualified:false, native_mcp_initialization_qualified:false, native_runtime_qualified:false};
}
exports.run = async function () {
  try {store(process.env.UAP_VSCODE_WORKSPACE_RESULT, await observe());}
  catch (error) {store(process.env.UAP_VSCODE_WORKSPACE_RESULT, {schema_version:1, collected:false, stage_error:String(error?.message || error).slice(0,2048),
    native_project_qualified:false, native_mcp_initialization_qualified:false, native_runtime_qualified:false});}
  await new Promise(() => {});
};

