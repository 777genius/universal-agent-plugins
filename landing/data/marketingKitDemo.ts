import type { RegistryPlugin } from '../types/registry';

/**
 * This is a declarative, verified package-version example, not a claim that
 * 'add marketing-kit@0.1.0' is supported as a positional CLI selector.
 * The three target IDs match UAP's real manifest contract.
 */
export const marketingKitName = 'marketing-kit';
export const marketingKitVersion = '0.1.0';
export const marketingKitDependency = '777genius/marketing-kit';
export const marketingKitProjectManifest = `schema_version = 1

[dependencies]
"777genius/marketing-kit" = "0.1.0"

[targets]
claude = ["777genius/marketing-kit"]
codex = ["777genius/marketing-kit"]
gemini = ["777genius/marketing-kit"]`;

/** The published marketing-kit 0.1.0 Directory policy is user-only. */
export const marketingKitInstallCommand = 'npx -y universal-agent-plugins install --scope user';
/** Export the saved user environment's portable manifest and lockfile. */
export const marketingKitExportCommand = 'npx -y universal-agent-plugins deps export';
/** Import a portable user environment and install its recorded exact pins. */
export const marketingKitRestoreCommand = 'npx -y universal-agent-plugins deps import --install';

/** Names denote declared compatible targets, never detection of local installs. */
export const marketingKitTargetIDs = ['claude', 'codex', 'gemini'] as const;

// Only a signed/reviewed canonical Directory listing may use this exact
// manifest-first Marketing Kit example; unrelated listings can share a name.
export function isReviewedMarketingKit(plugin: RegistryPlugin): boolean {
  return plugin.name === marketingKitName &&
    plugin.install_source === marketingKitName &&
    plugin.trust_state === 'reviewed' &&
    plugin.installable;
}
