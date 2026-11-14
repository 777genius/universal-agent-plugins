/**
 * Official Node and TypeScript runtime helpers for plugin-kit-ai executable plugins.
 */

/**
 * JSON-shaped payload used by the runtime helpers when a stricter schema is not known.
 */
export type JSONMap = Record<string, unknown>;
/**
 * Handler signature for Claude hooks that return an object response or no value.
 */
export type ClaudeHandler = (event: JSONMap) => JSONMap | void;
/**
 * Handler signature for Codex events that return an exit code or no value.
 */
export type CodexHandler = (event: JSONMap) => number | void;

/**
 * Stable Claude hook names supported by the public runtime lane.
 */
export declare const CLAUDE_STABLE_HOOKS: readonly string[];
/**
 * Extended Claude hook names exposed by the beta runtime lane.
 */
export declare const CLAUDE_EXTENDED_HOOKS: readonly string[];

/**
 * Returns the empty JSON object expected by Claude when a hook allows the action.
 */
export declare function allow(): Record<string, never>;
/**
 * Returns exit code `0` for Codex handlers that want normal continuation.
 */
export declare function continue_(): number;

/**
 * Minimal Claude hook app that dispatches supported hook names to registered handlers.
 */
export declare class ClaudeApp {
  /**
   * Creates a Claude runtime app.
   *
   * @param options.allowedHooks Hook names that this binary accepts on argv.
   * @param options.usage Usage string printed when the invocation is invalid.
   */
  constructor(options: { allowedHooks: string[] | readonly string[]; usage: string });
  /**
   * Registers a handler for an arbitrary Claude hook name.
   */
  on(hookName: string, handler: ClaudeHandler): this;
  /**
   * Registers a handler for the `Stop` hook.
   */
  onStop(handler: ClaudeHandler): this;
  /**
   * Registers a handler for the `PreToolUse` hook.
   */
  onPreToolUse(handler: ClaudeHandler): this;
  /**
   * Registers a handler for the `UserPromptSubmit` hook.
   */
  onUserPromptSubmit(handler: ClaudeHandler): this;
  /**
   * Dispatches the current process invocation and returns the exit code.
   */
  run(): number;
}

/**
 * Minimal Codex app that dispatches the `notify` event to a registered handler.
 */
export declare class CodexApp {
  /**
   * Creates a Codex runtime app with no registered handlers.
   */
  constructor();
  /**
   * Registers a handler for the Codex `notify` event.
   */
  onNotify(handler: CodexHandler): this;
  /**
   * Dispatches the current process invocation and returns the exit code.
   */
  run(): number;
}
