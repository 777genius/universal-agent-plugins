<script setup lang="ts">
type Setup = 'Automatic' | 'Guided' | 'Prepared' | 'Manual';

interface ClientSupport {
  name: string;
  user: string;
  project: string;
  setup: Setup;
  nextStep: string;
}

// Current main-source capabilities. An installed component can still require
// client trust, sign-in, reload, or manual activation before use.
const clients: ClientSupport[] = [
  {
    name: 'Claude Code',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Trust the workspace and reload Claude Code if prompted.',
  },
  {
    name: 'Codex',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Open a new session; approve project MCP access if prompted.',
  },
  {
    name: 'OpenCode',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Start a new session and review effective project settings.',
  },
  {
    name: 'Cursor',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Reload Cursor; enable plugins or MCP servers if requested.',
  },
  {
    name: 'GitHub Copilot CLI',
    user: 'Plugins, Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Restart Copilot CLI if requested.',
  },
  {
    name: 'Gemini CLI',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Reload Gemini CLI and approve tool access if prompted.',
  },
  {
    name: 'Qoder',
    user: 'Plugins, Skills, MCP, hooks',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Restart Qoder and review MCP permissions.',
  },
  {
    name: 'Pi',
    user: 'Extensions, Skills, MCP',
    project: 'Extensions, Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Trust the project and reload Pi resources.',
  },
  {
    name: 'Windsurf / Devin',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Open or restart Devin in the selected project; review access.',
  },
  {
    name: 'Kimi Code',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Automatic',
    nextStep: 'Run /reload or /new and review MCP permissions.',
  },
  {
    name: 'Cline',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Select the project MCP settings path and reload Cline.',
  },
  {
    name: 'Kiro',
    user: 'Skills, MCP',
    project: 'Skills, MCP',
    setup: 'Guided',
    nextStep: 'Sign in, approve access, and restart Kiro if prompted.',
  },
  {
    name: 'Grok Build',
    user: 'Skills, MCP',
    project: 'Skills only',
    setup: 'Guided',
    nextStep: 'Reload Grok; review project Skills and any CLI setup hint.',
  },
  {
    name: 'VS Code',
    user: 'Copilot-backed package',
    project: 'Skills, MCP (prepared)',
    setup: 'Prepared',
    nextStep: 'Open and trust the folder; review and enable MCP servers.',
  },
  {
    name: 'ChatGPT',
    user: 'Personal app setup',
    project: 'Not supported',
    setup: 'Manual',
    nextStep: 'Enable Developer Mode, connect the app, and select it in a new chat.',
  },
];
</script>

<template>
  <section
    id="agent-support"
    class="registry-client-support container"
    aria-labelledby="agent-support-title"
    lang="en"
    dir="ltr"
    data-scroll-reveal
  >
    <header class="section-heading section-heading--center">
      <p class="eyebrow">Compatibility</p>
      <h2 id="agent-support-title">Supported agents and installation scopes</h2>
      <p>
        Install for your user account or a specific project. UAP handles supported files and
        configuration; the agent may still ask for trust, sign-in, or a restart.
      </p>
    </header>

    <div
      class="registry-client-support__scroll"
      role="region"
      aria-label="Agent installation support table"
      tabindex="0"
    >
      <table>
        <caption class="sr-only">
          User and project installation support, setup automation, and actions after installing.
        </caption>
        <thead>
          <tr>
            <th scope="col">Agent</th>
            <th scope="col">User scope</th>
            <th scope="col">Project scope</th>
            <th scope="col">Setup</th>
            <th scope="col">What you do after installing</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="client in clients" :key="client.name">
            <th scope="row">{{ client.name }}</th>
            <td>{{ client.user }}</td>
            <td :class="{ 'registry-client-support__unavailable': client.project === 'Not supported' }">
              {{ client.project }}
            </td>
            <td>
              <span
                class="registry-client-support__setup"
                :class="`registry-client-support__setup--${client.setup.toLowerCase()}`"
              >
                {{ client.setup }}
              </span>
            </td>
            <td>{{ client.nextStep }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="registry-client-support__note">
      “Automatic” means UAP delivers the supported components, not that it signs you in or grants
      trust. Capabilities shown are from current source; the published CLI may be older. Availability
      depends on the plugin and client version.
      <a
        href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md"
        target="_blank"
        rel="noopener noreferrer"
      >Check compatibility details</a>.
    </p>
  </section>
</template>

<style scoped lang="scss">
.registry-client-support {
  padding-block: 64px 72px;

  .section-heading {
    max-width: 760px;
  }

  h2 {
    font-size: clamp(1.8rem, 3.4vw, 2.8rem);
  }

  &__scroll {
    max-width: 100%;
    overflow-x: auto;
    border: 1px solid var(--line-strong);
    border-radius: 16px;
    background: var(--surface);
    box-shadow: 0 14px 38px color-mix(in srgb, var(--text) 4%, transparent);
    overscroll-behavior-inline: contain;
  }

  table {
    width: 100%;
    min-width: 900px;
    border-collapse: collapse;
    text-align: left;
    line-height: 1.45;
  }

  th,
  td {
    padding: 13px 15px;
    border-bottom: 1px solid var(--line);
    vertical-align: middle;
    font-size: 0.81rem;
  }

  thead {
    background: var(--surface-raised);
  }

  thead th {
    color: var(--muted);
    font-size: 0.73rem;
    font-weight: 700;
    white-space: nowrap;
  }

  tbody th {
    color: var(--text);
    font-weight: 700;
    white-space: nowrap;
  }

  tbody td {
    color: var(--muted);
  }

  tbody td:nth-child(2),
  tbody td:nth-child(3) {
    color: var(--text);
  }

  tbody tr:last-child th,
  tbody tr:last-child td {
    border-bottom: 0;
  }

  tbody tr:hover {
    background: color-mix(in srgb, var(--primary-bright) 5%, transparent);
  }

  &__unavailable {
    color: var(--muted) !important;
  }

  &__setup {
    display: inline-block;
    padding: 4px 9px;
    border: 1px solid var(--line-strong);
    border-radius: 7px;
    color: var(--text);
    background: var(--surface-raised);
    font-size: 0.73rem;
    font-weight: 650;
    white-space: nowrap;
  }

  &__setup--automatic {
    color: var(--green);
    border-color: color-mix(in srgb, var(--green) 35%, var(--line));
    background: color-mix(in srgb, var(--green) 8%, transparent);
  }

  &__note {
    max-width: 970px;
    margin: 14px auto 0;
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.65;
    text-align: center;
  }

  &__note a {
    color: var(--primary-bright);
    text-underline-offset: 3px;
  }
}

@media (max-width: 720px) {
  .registry-client-support {
    padding-block: 48px 56px;

    table {
      min-width: 850px;
    }
  }
}
</style>
