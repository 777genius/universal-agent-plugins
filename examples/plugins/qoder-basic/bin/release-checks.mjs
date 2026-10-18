#!/usr/bin/env node
// Minimal dependency-free MCP stdio server for the qoder-basic example.
// Speaks newline-delimited JSON-RPC 2.0 on stdin/stdout and exposes the
// release-checks tool so the packaged example is runnable end to end.
import { createInterface } from "node:readline";

const SERVER_INFO = { name: "release-checks", version: "0.1.0" };
const SUPPORTED_PROTOCOL_VERSIONS = ["2024-11-05", "2025-03-26", "2025-06-18"];
const DEFAULT_PROTOCOL_VERSION = "2024-11-05";
const CHECKLIST = [
  "generated outputs are in sync (plugin-kit-ai generate --check)",
  "skills load in the target client",
  "MCP servers start and list their tools",
  "install and removal pass on a clean machine",
];

function send(message) {
  process.stdout.write(JSON.stringify(message) + "\n");
}

function result(id, value) {
  send({ jsonrpc: "2.0", id, result: value });
}

function error(id, code, message) {
  send({ jsonrpc: "2.0", id, error: { code, message } });
}

const tools = [
  {
    name: "release-checks",
    description: "Return the release readiness checklist for this plugin.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
];

function callTool(name, args) {
  if (name !== "release-checks") {
    return null;
  }
  if (args !== undefined && !(args === null || (typeof args === "object" && !Array.isArray(args) && Object.keys(args).length === 0))) {
    return { invalid: "tool accepts no arguments" };
  }
  return { content: [{ type: "text", text: CHECKLIST.map((line) => `- ${line}`).join("\n") }] };
}

function handle(message) {
  const { id, method, params } = message;
  if (method === undefined || id === undefined) {
    return; // notification (for example `notifications/initialized`)
  }
  switch (method) {
    case "initialize":
      result(id, {
        protocolVersion: SUPPORTED_PROTOCOL_VERSIONS.includes(params?.protocolVersion)
          ? params.protocolVersion
          : DEFAULT_PROTOCOL_VERSION,
        capabilities: { tools: {} },
        serverInfo: SERVER_INFO,
      });
      return;
    case "ping":
      result(id, {});
      return;
    case "tools/list":
      result(id, { tools });
      return;
    case "tools/call": {
      const output = callTool(params?.name, params?.arguments);
      if (output === null) {
        error(id, -32602, `unknown tool: ${String(params?.name)}`);
        return;
      }
      if (output.invalid) {
        error(id, -32602, `invalid arguments for ${String(params?.name)}: ${output.invalid}`);
        return;
      }
      result(id, output);
      return;
    }
    default:
      error(id, -32601, `method not found: ${String(method)}`);
  }
}

createInterface({ input: process.stdin }).on("line", (line) => {
  if (line.trim() === "") {
    return;
  }
  try {
    handle(JSON.parse(line));
  } catch {
    error(null, -32700, "parse error");
  }
});
