const DEFAULT_CLI_INVOCATION = 'agentplugins';

export function applyCliInvocation(command: string, invocation?: string | null): string {
  const normalizedInvocation = invocation?.trim() || DEFAULT_CLI_INVOCATION;
  return command.replace(/^(?:plugin-kit-ai|universal-agent-plugins)\b/gm, normalizedInvocation);
}

export function getCliInvocation(invocation?: string | null): string {
  return invocation?.trim() || DEFAULT_CLI_INVOCATION;
}

// Only rewrite this product's own npx invocation, never other packages or flags.
const NPX_UAP_INVOCATION = /^npx(?: (?:-y|--yes))? universal-agent-plugins(?:@[A-Za-z0-9._-]+)?(?=\\s|$)/gm;

export function hasNpxUapInvocation(command: string): boolean {
  return /^npx(?: (?:-y|--yes))? universal-agent-plugins(?:@[A-Za-z0-9._-]+)?(?=\\s|$)/m.test(command);
}

export function withNativeCliInvocation(command: string, nativeCli: boolean): string {
  return nativeCli ? command.replace(NPX_UAP_INVOCATION, 'agentplugins') : command;
}
