export type InstallPlatform = 'macos' | 'windows' | 'linux' | 'mobile' | 'other';

export function normalizeInstallPlatform(osName?: string | null): InstallPlatform {
  const normalized = osName?.trim().toLowerCase() ?? '';

  if (normalized.includes('mac') || normalized.includes('os x')) {
    return 'macos';
  }
  if (normalized.includes('windows')) {
    return 'windows';
  }
  if (
    normalized.includes('android') ||
    normalized.includes('ios') ||
    normalized.includes('iphone') ||
    normalized.includes('ipad') ||
    normalized.includes('ipod')
  ) {
    return 'mobile';
  }
  if (normalized.includes('linux')) {
    return 'linux';
  }

  return 'other';
}

export function recommendedInstallChannelId(platform: InstallPlatform): string | null {
  switch (platform) {
    case 'macos':
      return 'brew';
    case 'windows':
      return 'powershell';
    case 'linux':
      return 'script';
    case 'mobile':
      return null;
    default:
      return 'npm';
  }
}
