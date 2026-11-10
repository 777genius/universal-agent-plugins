import type { ClientTarget } from '../types/registry';

export interface ClientLandingPage extends ClientTarget {
  slug: string;
  presentationKey?: string;
  intro: string;
  delivery: string;
  activation: string;
  vendorDocsUrl?: string;
}

/** Public presentation only: keep technical compatibility and previously installed profiles intact.
 * Hermes and Antigravity require a qualified CLI release before public install actions are exposed.
 */
export const hiddenPublicClientIds: ReadonlySet<string> = new Set(['vscode', 'hermes', 'antigravity']);

const allClientLandingPages: ClientLandingPage[] = [
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
    note: 'Desktop: automatic via Codex CLI; Web: setup in ChatGPT',
    status: 'Supported',
    intro:
      'ChatGPT supports plugins on Desktop and Web. Desktop local plugins use the Codex CLI marketplace, while Web plugins require a compatible cloud app or marketplace connection.',
    delivery:
      'UAP installs eligible local Desktop packages through the Codex CLI and prepares qualified Web app bindings; it cannot silently install a cloud plugin into a ChatGPT account.',
    activation:
      'Desktop: open a new local thread and verify the plugin. Web: connect or install the app in ChatGPT and grant any required permissions.',
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
      'The CLI prepares supported Cursor plugin, skill and MCP components for the selected scope. The add command defaults to user scope; select project scope explicitly for add.',
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
    note: 'Managed skills and MCP configuration',
    status: 'Supported',
    intro:
      'Install Agent Plugins 1.0 for Kiro from the same portable package used by other AI agents.',
    delivery:
      'The CLI installs its owned Kiro skills and MCP entries while preserving unrelated global configuration.',
    activation:
      'Complete the printed OAuth or restart step. macOS and Windows preparation does not claim automatic runtime verification.',
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
  {
    id: 'grok',
    slug: 'grok-build',
    name: 'Grok Build',
    icon: 'grok.svg',
    note: 'Native skills and MCP plugin; CLI verification',
    status: 'Supported',
    intro:
      'Install portable Agent Plugins 1.0 skills and MCP servers for Grok Build from the same CLI used for other clients.',
    delivery:
      "The CLI writes a native .grok-plugin manifest in Grok's user plugin directory and manages registration with the Grok CLI when available.",
    activation:
      'Reload Grok Build after installation. If the Grok CLI is unavailable, install it and rerun the update command for native verification.',
    vendorDocsUrl: 'https://docs.x.ai/build/features/skills-plugins-marketplaces',
  },
  {
    id: 'kimi',
    slug: 'kimi-code',
    name: 'Kimi Code',
    icon: 'kimi.svg',
    note: 'Managed user plugin with skills and MCP',
    status: 'Supported',
    intro:
      'Install portable Agent Plugins 1.0 skills and MCP servers into Kimi Code through a managed user plugin.',
    delivery:
      'The CLI writes a native .kimi-plugin manifest and registers the managed package while preserving unrelated Kimi plugins.',
    activation:
      'Run /reload or /new in Kimi Code. Runtime MCP connections and provider authentication still need confirmation in Kimi.',
    vendorDocsUrl: 'https://www.kimi.com/code/docs/en/kimi-code-cli/customization/plugins',
  },
  {
    id: 'qoder',
    slug: 'qoder',
    name: 'Qoder',
    icon: 'qoder.svg',
    note: 'Native skills and MCP plugin; CLI verification',
    status: 'Supported',
    intro:
      'Install portable Agent Plugins 1.0 skills and MCP servers for Qoder from the same CLI used for other clients.',
    delivery:
      'The CLI writes a native .qoder-plugin manifest and registers the managed package through the Qoder CLI when available.',
    activation:
      'Reload Qoder plugins or restart Qoder after installation. Runtime MCP connections and provider authentication still need confirmation in Qoder.',
  },
  {
    id: 'hermes',
    slug: 'hermes',
    name: 'Hermes Agent',
    icon: 'terminal.svg',
    note: 'User Skills/MCP and Project Skills; Project MCP is unsupported',
    status: 'Supported',
    intro:
      'Install native Hermes Agent Skills for your user or a selected Git project, plus user-scoped MCP servers, without changing unrelated settings.',
    delivery:
      'UAP manages owned Skills directories and exact User MCP entries in config.yaml. Project installation never modifies User MCP configuration.',
    activation:
      'Restart Hermes to reload the integration. Review project trust before using Project Skills; runtime discovery and authentication still need confirmation.',
    vendorDocsUrl: 'https://github.com/NousResearch/hermes-agent',
  },
  {
    id: 'antigravity',
    slug: 'antigravity',
    name: 'Google Antigravity',
    icon: 'terminal.svg',
    note: 'Native User/Project plugins; activation requires verification',
    status: 'Prepared by CLI',
    intro:
      'Prepare native Google Antigravity plugins with supported Skills and MCP servers in the selected User or Project scope.',
    delivery:
      'UAP prepares a managed plugin folder under the selected user or project configuration, preserves unrelated plugins, and does not enable hooks implicitly.',
    activation:
      'Open or reload Antigravity and verify the plugin is discovered and enabled. Prepared files do not prove a model session, authentication, or live MCP execution.',
    vendorDocsUrl: 'https://antigravity.google/docs/build-plugins',
  },
  {
    id: 'pi',
    slug: 'pi',
    name: 'Pi',
    icon: 'pi.svg',
    note: 'Native Pi extensions, Skills and MCP',
    status: 'Supported',
    intro:
      'Install compatible Pi extensions, Skills, and stdio/Streamable HTTP MCP servers in user or project scope. Catalog packages require a compatible signed release; direct sources are validated by the CLI.',
    delivery:
      'The CLI installs eligible components in the selected Pi profile or project and tracks updates, repair, and removal.',
    activation: 'Reload resources or restart Pi; trust the project and review MCP permissions if prompted.',
    vendorDocsUrl:
      'https://pi.dev/docs/latest',
  },
];

export const clientLandingPages: ClientLandingPage[] = allClientLandingPages.filter(
  (client) => !hiddenPublicClientIds.has(client.id),
);

export const clients: ClientTarget[] = clientLandingPages.map(
  ({ id, name, icon, note, status }) => ({ id, name, icon, note, status }),
);

// Shell consumers can use this contract with clients[] without importing message JSON.
export function clientPresentationKey(
  id: ClientTarget['id'],
): `registryUi.clients.${ClientTarget['id']}` {
  return `registryUi.clients.${id}`;
}

// IDs and URLs stay canonical; captions are resolved by the active locale at the UI boundary.
for (const client of clientLandingPages) client.presentationKey = clientPresentationKey(client.id);

export const clientLandingBySlug = new Map(
  clientLandingPages.map((client) => [client.slug, client]),
);

export const clientLandingById = new Map(clientLandingPages.map((client) => [client.id, client]));
