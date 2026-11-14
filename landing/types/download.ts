/** Only presentation text belongs in a translated download overlay. */
export interface DownloadOverlay {
  shell: {
    download: {
      heading: { title: string; note: string };
      channels: Record<string, { title: string; description: string; note: string }>;
      steps: Record<string, { title: string; note: string }>;
    };
  };
}
