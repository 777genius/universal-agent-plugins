import type { ClientTarget } from '../types/registry';

export type AgentSetup = 'Automatic' | 'Guided' | 'Prepared' | 'Manual';
export interface ClientSupport {
  id: ClientTarget['id'];
  name: string;
  user: string;
  project: string;
  setup: AgentSetup;
  nextStep: string;
}

/** Translation mapping for the display layer. Keep English README source rows unchanged. */
export const clientSupportCapabilityKeys = {
  'Plugins, Skills, MCP': 'pluginsSkillsMcp',
  'Skills, MCP': 'skillsMcp',
  'Plugins, Skills, MCP, hooks': 'pluginsSkillsMcpHooks',
  'Extensions, Skills, MCP': 'extensionsSkillsMcp',
  'Copilot-backed package': 'copilotPackage',
  'Desktop local plugins; Web connected apps': 'desktopLocalPluginsWebConnectedApps',
  'Personal app setup': 'personalAppSetup',
  'Skills only': 'skillsOnly',
  'Skills, MCP (prepared)': 'skillsMcpPrepared',
  'Not supported': 'notSupported',
} as const;

// SYNC: Update landing/data/clientSupport.ts AND README.md "Supported clients".
// The landing renders this in landing/components/registry/RegistryClientSupport.vue
// before FAQ in landing/pages/index.vue. Icon mapping: landing/data/clients.ts,
// assets: landing/public/client-icons/. Enforcement: landing/tests/client-support-readme.test.ts.
export const clientSupportRows: ClientSupport[] = [
  {
    id: 'claude',
    name: 'Claude Code',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Trust the workspace and reload Claude Code if prompted.',
  },
  {
    id: 'codex',
    name: 'Codex CLI',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Open a new session; approve project MCP access if prompted.',
  },
  {
    id: 'opencode',
    name: 'OpenCode',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Start a new session and review effective project settings.',
  },
  {
    id: 'cursor',
    name: 'Cursor',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Reload Cursor; enable plugins or MCP servers if requested.',
  },
  {
    id: 'copilot',
    name: 'GitHub Copilot CLI',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Restart Copilot CLI if requested.',
  },
  {
    id: 'gemini',
    name: 'Gemini CLI',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Reload Gemini CLI and approve tool access if prompted.',
  },
  {
    id: 'qoder',
    name: 'Qoder',
    user: 'Plugins, Skills, MCP, hooks',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Restart Qoder and review MCP permissions.',
  },
  {
    id: 'pi',
    name: 'Pi',
    user: 'Extensions, Skills, MCP',
    project: 'Extensions, Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Trust the project and reload Pi resources.',
  },
  {
    id: 'windsurf',
    name: 'Windsurf / Devin',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Open or restart Devin in the selected project; review access.',
  },
  {
    id: 'kimi',
    name: 'Kimi Code',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Run /reload or /new and review MCP permissions.',
  },
  {
    id: 'cline',
    name: 'Cline',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Select the project MCP settings path and reload Cline.',
  },
  {
    id: 'kiro',
    name: 'Kiro',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Sign in, approve access, and restart Kiro if prompted.',
  },
  {
    id: 'grok',
    name: 'Grok Build',
    user: 'Skills, MCP',
    project: 'Skills, MCP (prepared)',
    setup: 'Guided',
    nextStep: 'Open the selected project in Grok; review and trust the plugin, then verify Skills and MCP connectivity.',
  },
  {
    id: 'vscode',
    name: 'VS Code',
    user: 'Copilot-backed package',
    project: 'Skills, MCP (prepared)',
    setup: 'Prepared',
    nextStep: 'Open and trust the folder; review and enable MCP servers.',
  },
  {
    id: 'chatgpt',
    name: 'ChatGPT',
    user: 'Desktop local plugins; Web connected apps',
    project: 'Not supported',
    setup: 'Guided',
    nextStep: 'Desktop: local install via Codex CLI; Web: connect or install in ChatGPT.',
  },
];
