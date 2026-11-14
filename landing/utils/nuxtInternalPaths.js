function stripEdgeSlashes(segment) {
  return segment.replace(/^\/+|\/+$/g, '');
}

function joinRelativeURL(...parts) {
  const normalized = parts
    .filter((part) => typeof part === 'string' && part.length > 0)
    .map((part, index) => {
      if (index === 0) {
        const hasLeadingSlash = part.startsWith('/');
        const trimmed = stripEdgeSlashes(part);
        if (!trimmed) {
          return hasLeadingSlash ? '/' : '';
        }
        return `${hasLeadingSlash ? '/' : ''}${trimmed}`;
      }

      return stripEdgeSlashes(part);
    })
    .filter(Boolean);

  return normalized.join('/') || '/';
}

export function baseURL() {
  return process.env.NUXT_APP_BASE_URL || '/';
}

export function buildAssetsDir() {
  return process.env.NUXT_APP_BUILD_ASSETS_DIR || '/_nuxt/';
}

export function publicAssetsURL(...path) {
  const publicBase = process.env.NUXT_APP_CDN_URL || baseURL();
  return path.length ? joinRelativeURL(publicBase, ...path) : publicBase;
}

export function buildAssetsURL(...path) {
  return joinRelativeURL(publicAssetsURL(), buildAssetsDir(), ...path);
}
