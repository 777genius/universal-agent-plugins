import type { Plugin } from "@opencode-ai/plugin";
import { createObserver } from "./index.js";

// Compile against the exact OpenCode plugin client and native event types.
export const plugin: Plugin = async ({ client }) => {
  const observer = createObserver({ client, emit: () => {} });
  return { event: async ({ event }) => observer.observe(event) };
};
