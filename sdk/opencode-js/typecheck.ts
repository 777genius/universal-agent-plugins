import type { Plugin } from "@opencode-ai/plugin";
import { createObserver } from "./index.js";

// Compile against the exact OpenCode plugin client and native event types.
export const plugin: Plugin = async ({ client }) => {
  const observer = createObserver({ client, emit: () => {} });
  return { event: async ({ event }) => observer.observe(event) };
};

// V2 consumers need no V1 messages method or @opencode/plugin runtime import.
import { createV2Observer, type V2Client } from './index.js';
const v2Client: V2Client = {
  get: async ({ sessionID }) => ({ id: sessionID, location: { directory: '/TEST-project' } }),
  context: async () => [],
};
const v2Observer = createV2Observer({ client: v2Client, location: { directory: '/TEST-project' }, emit: () => {} });
v2Observer.observe({ type: 'session.created', data: { sessionID: 'session' } });
v2Observer.dispose();
