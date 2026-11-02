export const registryFaqItems = [
  {
    question: 'Which AI agents are supported?',
    answer:
      'Codex, ChatGPT, Cursor, GitHub Copilot CLI, VS Code, Kiro, Claude Code, Gemini CLI, OpenCode, Cline, Windsurf, Grok Build, Kimi Code, Qoder: each has a client-specific installation or preparation path. Package components, scope and activation requirements determine compatibility.',
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
      'Yes. Install a complete plugin.json package from a local folder, Git over HTTPS or SSH, supported provider links, or a ready archive. The CLI records exact source pins for eligible saved dependencies.',
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
