export type RestoreDemoMode = 'project' | 'global';

/** CLI examples, not an alternative to signed distribution/scope eligibility. */
export const restoreDemoCommands = {
  // Released CLI 0.1.101 authenticates the signed v1 source and independently
  // checks project placement for every required child before installation.
  project: {
    install: 'npx -y universal-agent-plugins install marketing-kit --scope project',
    restore: 'npx -y universal-agent-plugins install --scope project',
    export: '',
  },
  global: {
    install: 'npx -y universal-agent-plugins install marketing-kit',
    restore: 'npx -y universal-agent-plugins deps import --install',
    export: 'npx -y universal-agent-plugins deps export',
  },
} as const satisfies Record<RestoreDemoMode, { install: string; restore: string; export: string }>;
