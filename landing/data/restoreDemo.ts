export type RestoreDemoMode = 'project' | 'global';

/** CLI examples, not an alternative to signed distribution/scope eligibility. */
export const restoreDemoCommands = {
  project: {
    install: 'npx -y universal-agent-plugins install marketing-kit --scope project',
    restore: 'npx -y universal-agent-plugins install',
    export: '',
  },
  global: {
    install: 'npx -y universal-agent-plugins install marketing-kit',
    restore: 'npx -y universal-agent-plugins deps import --install',
    export: 'npx -y universal-agent-plugins deps export',
  },
} as const satisfies Record<RestoreDemoMode, { install: string; restore: string; export: string }>;
