export type DownloadOs = "macos" | "windows" | "linux"
export type DownloadArch = "arm64" | "x64" | "universal"

export type ReleaseAsset = {
  name: string
  browser_download_url: string
}

export type GitHubRelease = {
  tag_name: string
  published_at: string
  assets: ReleaseAsset[]
}

export type Variant = {
  url: string | null
  platformKey: string | null
  version: string | null
}

export type DownloadsApiResponse = {
  ok: boolean
  source: "github-releases"
  fetchedAt: string
  version: string | null
  pubDate: string | null
  variants: {
    macos: { arm64: Variant; x64: Variant; universal: Variant }
    windows: { x64: Variant }
    linux: { appimage: Variant; deb: Variant }
  }
}

const emptyVariant: Variant = { url: null, platformKey: null, version: null }

function findAsset(assets: ReleaseAsset[], pattern: RegExp): ReleaseAsset | null {
  return assets.find((asset) => pattern.test(asset.name)) || null
}

function toVariant(
  asset: ReleaseAsset | null,
  version: string | null
): Variant {
  if (!asset) {
    return { ...emptyVariant }
  }

  return {
    url: asset.browser_download_url,
    platformKey: asset.name,
    version
  }
}

export function parseGitHubRelease(release: GitHubRelease): DownloadsApiResponse {
  const version = release.tag_name?.replace(/^(?:(?:agentplugins|plugin-kit-ai)-)?v/, "") || null
  const assets = release.assets || []

  return {
    ok: true,
    source: "github-releases",
    fetchedAt: new Date().toISOString(),
    version,
    pubDate: release.published_at || null,
    variants: {
      macos: {
        arm64: toVariant(findAsset(assets, /darwin_arm64\.tar\.gz$/i), version),
        x64: toVariant(findAsset(assets, /darwin_amd64\.tar\.gz$/i), version),
        universal: { ...emptyVariant }
      },
      windows: {
        x64: toVariant(findAsset(assets, /windows_amd64\.zip$/i), version)
      },
      linux: {
        appimage: toVariant(findAsset(assets, /\.AppImage$/i), version),
        deb: toVariant(findAsset(assets, /\.deb$/i), version)
      }
    }
  }
}

export function emptyDownloadsResponse(): DownloadsApiResponse {
  return {
    ok: false,
    source: "github-releases",
    fetchedAt: new Date().toISOString(),
    version: null,
    pubDate: null,
    variants: {
      macos: {
        arm64: { ...emptyVariant },
        x64: { ...emptyVariant },
        universal: { ...emptyVariant }
      },
      windows: {
        x64: { ...emptyVariant }
      },
      linux: {
        appimage: { ...emptyVariant },
        deb: { ...emptyVariant }
      }
    }
  }
}
