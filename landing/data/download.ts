import type { InstallChannel, QuickstartStep } from '../types/content';
import type { DownloadOverlay } from '../types/download';

// Technical bytes are shared across locales; overlays cannot override them.
export const downloadTechnical = {
  installChannels: [
    {
      id: 'brew',
      href: 'https://github.com/777genius/homebrew-agentplugins',
      command: 'brew install 777genius/agentplugins/agentplugins',
      invocation: 'agentplugins',
      recommended: true,
    },
    {
      id: 'npm',
      href: 'https://www.npmjs.com/package/universal-agent-plugins',
      command: 'npx universal-agent-plugins version',
      invocation: 'npx universal-agent-plugins',
    },
    {
      id: 'script',
      href: 'https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md',
      command:
        'curl -fsSL https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.sh | sh',
      invocation: '$HOME/.local/bin/agentplugins',
    },
    {
      id: 'powershell',
      href: 'https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md#windows-powershell',
      command:
        'irm https://raw.githubusercontent.com/777genius/universal-agent-plugins/main/install.ps1 | iex',
      invocation: '& "$HOME\\.local\\bin\\agentplugins.exe"',
    },
  ],
  quickstartSteps: [
    {
      id: 'install-cli',
      command: 'npx universal-agent-plugins version',
    },
    {
      id: 'try-plugin',
      command: 'universal-agent-plugins add context7 --target codex,cursor',
    },
    {
      id: 'build-plugin',
      command:
        'universal-agent-plugins update context7 --target codex,cursor\nuniversal-agent-plugins repair context7 --target codex,cursor',
    },
    {
      id: 'validate',
      command: 'universal-agent-plugins remove context7 --target codex,cursor',
    },
  ],
};

export function assembleDownloadContent(overlay: DownloadOverlay) {
  const copy = overlay.shell.download;
  return {
    download: { title: copy.heading.title, note: copy.heading.note },
    installChannels: downloadTechnical.installChannels.map((channel) => {
      const text = copy.channels[channel.id]!;
      return { ...channel, title: text.title, description: text.description, note: text.note };
    }) satisfies InstallChannel[],
    quickstartSteps: downloadTechnical.quickstartSteps.map((step) => {
      const text = copy.steps[step.id]!;
      return { ...step, title: text.title, note: text.note };
    }) satisfies QuickstartStep[],
  };
}
