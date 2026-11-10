import claudeBadge from '~/assets/images/agent-badges/claude.webp';
import codexBadge from '~/assets/images/agent-badges/codex.svg';
import cursorBadge from '~/assets/images/agent-badges/cursor.svg';
import geminiBadge from '~/assets/images/agent-badges/gemini.svg';
import openCodeBadge from '~/assets/images/agent-badges/opencode.webp';

const agentBadgeMap = {
  Claude: claudeBadge,
  Codex: codexBadge,
  Cursor: cursorBadge,
  Gemini: geminiBadge,
  OpenCode: openCodeBadge,
} as const;

export const resolveAgentBadge = (label: string): string | undefined => {
  return agentBadgeMap[label as keyof typeof agentBadgeMap];
};
