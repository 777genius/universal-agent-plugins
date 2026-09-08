export const registryFaqItems = [
  {
    question: 'Which AI agents are supported?',
    answer:
      'Codex, ChatGPT, Cursor, GitHub Copilot CLI, VS Code, Kiro, Claude Code, Gemini CLI, OpenCode, Cline, and Windsurf are supported or prepared through an explicit client-specific path.',
  },
  {
    question: 'Does one command install into every agent?',
    answer:
      'By default the CLI detects installed compatible agents and asks you to confirm. You can also select one or many agents explicitly.',
  },
  {
    question: 'What happens when an agent needs sign-in?',
    answer:
      'The CLI prepares the package and shows the exact activation or sign-in step. It never receives your OAuth password.',
  },
  {
    question: 'Can I install a plugin outside this directory?',
    answer:
      'Yes. Use a pinned GitHub source that contains a valid Agent Plugins 1.0 plugin.json package.',
  },
  {
    question: 'Are all discovered plugins reviewed?',
    answer:
      'No. Reviewed entries and automatically discovered community packages are labeled separately so you can choose the trust level you need.',
  },
  {
    question: 'Can I undo an installation?',
    answer:
      'Yes. The same CLI records managed state and supports inspect, update, repair, source switching, and removal.',
  },
] as const;
