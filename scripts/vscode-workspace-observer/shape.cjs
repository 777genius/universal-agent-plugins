'use strict';
const MAX_ROWS = 128;
function text(value) {
  if (typeof value !== 'string' || Buffer.byteLength(value, 'utf8') > 8192 || value.includes('\0') || !value.isWellFormed()) throw new Error('metadata string invalid or oversized');
  return value;
}
function flag(value) {
  if (typeof value !== 'boolean') throw new Error('metadata boolean required');
  return value;
}
function texts(value) {
  if (!Array.isArray(value) || value.length > MAX_ROWS) throw new Error('metadata string array invalid or oversized');
  return value.map(text);
}
function dictionary(value, nullable = false) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length > MAX_ROWS) throw new Error('metadata dictionary required');
  const result = Object.create(null);
  for (const [key, entry] of Object.entries(value)) {
    result[text(key)] = nullable && (entry === null || (typeof entry === 'number' && Number.isFinite(entry))) ? entry : text(entry);
  }
  return result;
}
function uri(value) {
  if (!value || typeof value !== 'object' || typeof value.toString !== 'function') throw new Error('metadata URI required');
  return {value: text(value.toString()), scheme: text(value.scheme), path: text(value.path), authority: text(value.authority)};
}
function rows(value, project) {
  if (!Array.isArray(value) || value.length > MAX_ROWS) throw new Error('native metadata array invalid or oversized');
  return value.map(project);
}
function mcp(value) {
  return rows(value, row => {
    if (!row || typeof row !== 'object') throw new Error('MCP metadata row required');
    const result = {label: text(row.label)};
    if (row.version !== undefined) result.version = text(row.version);
    if (row.uri !== undefined) {
      if (['command','args','env','cwd'].some(key => row[key] !== undefined)) throw new Error('mixed MCP transport fields');
      result.kind = 'http';
      result.uri = uri(row.uri);
      result.headers = dictionary(row.headers === undefined ? {} : row.headers);
    } else {
      if (row.headers !== undefined) throw new Error('mixed MCP transport fields');
      result.kind = 'stdio';
      result.command = text(row.command);
      result.args = texts(row.args === undefined ? [] : row.args);
      result.env = dictionary(row.env === undefined ? {} : row.env, true);
      if (row.cwd !== undefined) result.cwd = uri(row.cwd);
    }
    return result;
  });
}
function skills(value) {
  return rows(value, row => {
    if (!row || typeof row !== 'object' || !['local','user','extension','plugin','builtin'].includes(row.source)) throw new Error('unknown skill metadata source');
    const result = {uri: uri(row.uri), name: text(row.name), source: row.source, disableModelInvocation: flag(row.disableModelInvocation)};
    for (const key of ['description','extensionId']) if (row[key] !== undefined) result[key] = text(row[key]);
    if (row.sessionTypes !== undefined) result.sessionTypes = texts(row.sessionTypes);
    if (row.userInvocable !== undefined) result.userInvocable = flag(row.userInvocable);
    if (row.pluginUri !== undefined) result.pluginUri = uri(row.pluginUri);
    return result;
  });
}
module.exports = {text, flag, texts, uri, mcp, skills};

