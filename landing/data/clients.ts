import type { ClientTarget } from '../types/registry';

export interface ClientLandingPage extends ClientTarget {
  slug: string;
  intro: string;
  delivery: string;
  activation: string;
  vendorDocsUrl?: string;
}

export const clientLandingPages: ClientLandingPage[] = [
  {
    id: 'codex',
    slug: 'codex',
    name: 'Codex',
    icon: 'openai.svg',
    note: 'Skills and supported MCP transports',
    status: 'Supported',
    intro:
      'Install portable Agent Plugins 1.0 for Codex with one CLI command. Skills and supported MCP servers are prepared in the format Codex understands.',
    delivery:
      'The CLI prepares the OpenAI-compatible package and keeps its managed state reversible.',
    activation: 'Follow any activation hint printed by the CLI, then start a new Codex session.',
    vendorDocsUrl: 'https://developers.openai.com/codex/plugins',
  },
  {
    id: 'chatgpt',
    slug: 'chatgpt',
    name: 'ChatGPT',
    icon: 'openai.svg',
    note: 'Verified ChatGPT connections',
    status: 'Setup in app',
    intro:
      'Prepare compatible Agent Plugins 1.0 for ChatGPT while keeping app activation and account consent inside ChatGPT.',
    delivery: 'The CLI validates and prepares the compatible package or registered app binding.',
    activation: 'Complete the final app activation or sign-in step in ChatGPT when prompted.',
  },
  {
    id: 'cursor',
    slug: 'cursor',
    name: 'Cursor',
    icon: 'cursor.svg',
    note: 'Native Agent Plugin package',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 into Cursor projects without translating each plugin by hand. The CLI plans the native package before changing files.',
    delivery:
      'The CLI writes the supported Cursor plugin, skill, and MCP projections for the selected project.',
    activation: 'Reload Cursor when requested, then verify that the plugin is discovered.',
    vendorDocsUrl: 'https://docs.cursor.com/context/mcp',
  },
  {
    id: 'copilot',
    slug: 'github-copilot-cli',
    name: 'GitHub Copilot CLI',
    icon: 'github-copilot.svg',
    note: 'Managed native plugin',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 through GitHub Copilot CLI using its native plugin workflow and the same lifecycle command used for other agents.',
    delivery:
      'The CLI uses the supported Copilot plugin layout and verifies the resulting install state.',
    activation: 'Restart or reload the client only when the native Copilot workflow asks for it.',
  },
  {
    id: 'vscode',
    slug: 'vscode',
    name: 'VS Code',
    icon: 'vscode.svg',
    note: 'Copilot plugin integration',
    status: 'Supported',
    intro:
      'Prepare Agent Plugins 1.0 for VS Code through the supported GitHub Copilot integration, with exact follow-up instructions when manual setup remains.',
    delivery:
      'The CLI reuses the compatible Copilot plugin package when the native client path is available.',
    activation:
      'If automatic activation is unavailable, follow the exact setting or import path printed by the CLI.',
  },
  {
    id: 'kiro',
    slug: 'kiro',
    name: 'Kiro',
    icon: 'kiro.svg',
    note: 'Native folder package',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 for Kiro from the same portable package used by other AI agents.',
    delivery:
      "The CLI prepares Kiro's native folder package with the compatible plugin components.",
    activation:
      'Follow the printed import or activation hint when Kiro requires a final client-side step.',
  },
  {
    id: 'claude',
    slug: 'claude-code',
    name: 'Claude Code',
    icon: 'claude.svg',
    note: 'Managed MCP configuration',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 for Claude Code with managed skills and MCP configuration instead of maintaining a separate setup by hand.',
    delivery:
      'The CLI creates the supported Claude Code projections and tracks them for update, repair, and removal.',
    activation:
      'Reload Claude Code when requested and complete any service sign-in in the client or provider.',
    vendorDocsUrl: 'https://code.claude.com/docs/en/discover-plugins',
  },
  {
    id: 'gemini',
    slug: 'gemini-cli',
    name: 'Gemini CLI',
    icon: 'gemini.svg',
    note: 'Managed MCP configuration',
    status: 'Supported',
    intro:
      'Install compatible Agent Plugins 1.0 for Gemini CLI while preserving the portable plugin package as the source of truth.',
    delivery:
      'The CLI writes the supported Gemini CLI configuration and compatible package components.',
    activation: 'Enable or reload the integration when Gemini CLI prints a follow-up step.',
    vendorDocsUrl: 'https://geminicli.com/docs/extensions/reference/',
  },
  {
    id: 'opencode',
    slug: 'opencode',
    name: 'OpenCode',
    icon: 'opencode.svg',
    note: 'Managed MCP configuration',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 into OpenCode projects with managed MCP configuration and reversible lifecycle state.',
    delivery:
      'The CLI prepares the supported project configuration and plugin components for OpenCode.',
    activation:
      'Open a new OpenCode session after installation so the project configuration is reloaded.',
    vendorDocsUrl: 'https://opencode.ai/docs/plugins/',
  },
  {
    id: 'cline',
    slug: 'cline',
    name: 'Cline',
    icon: 'cline.svg',
    note: 'Managed MCP configuration',
    status: 'Supported',
    intro:
      'Install compatible Agent Plugins 1.0 for Cline without rebuilding the MCP configuration for every plugin.',
    delivery: 'The CLI prepares and tracks the supported Cline MCP configuration.',
    activation:
      'Reload Cline when requested and complete provider authentication outside the plugin package.',
  },
  {
    id: 'windsurf',
    slug: 'windsurf',
    name: 'Windsurf',
    icon: 'windsurf.svg',
    note: 'Prepared package; manual activation required',
    status: 'Prepared by CLI',
    intro:
      'Prepare Agent Plugins 1.0 for Windsurf with one CLI command, then finish the client-specific activation step explicitly.',
    delivery:
      'The CLI validates and prepares the compatible Windsurf package without claiming automatic activation.',
    activation: 'Follow the exact manual activation path printed after preparation.',
  },
];

export const clients: ClientTarget[] = clientLandingPages.map(
  ({ id, name, icon, note, status }) => ({ id, name, icon, note, status }),
);

export const clientLandingBySlug = new Map(
  clientLandingPages.map((client) => [client.slug, client]),
);

export const clientLandingById = new Map(clientLandingPages.map((client) => [client.id, client]));
