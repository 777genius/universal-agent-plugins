<template>
  <section
    id="why"
    class="registry-benefits container"
    aria-labelledby="why-title"
    data-scroll-reveal
  >
    <div class="section-heading section-heading--center">
      <p class="eyebrow">Why it works</p>
      <h2 id="why-title">One standard<br />Native delivery</h2>
      <p>The package stays portable; the installer handles each client's format and lifecycle.</p>
    </div>
    <div class="registry-benefits__grid">
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">01</span>
          <svg viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="m16 3 11 6v14l-11 6-11-6V9Z M5 9l11 6 11-6 M16 15v14 M10.5 6l11 6v6" />
          </svg>
        </div>
        <h3>Standard first</h3>
        <p>
          Packages are read from Agent Plugins 1.0 <code>plugin.json</code>, without a second
          install manifest.
        </p>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">02</span>
          <svg viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="12" width="8" height="8" rx="2" />
            <rect x="22" y="3" width="7" height="7" rx="2" />
            <rect x="22" y="22" width="7" height="7" rx="2" />
            <path d="M11 16h5V6.5h6 M16 16v9.5h6 M16 16h12" />
          </svg>
        </div>
        <h3>Client-aware</h3>
        <p>
          Codex, Cursor, Claude Code, Gemini CLI, OpenCode, and other clients receive the format
          they support.
        </p>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">03</span>
          <svg viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M26 12a11 11 0 0 0-19-4L4 11 M4 5v6h6 M6 20a11 11 0 0 0 19 4l3-3 M28 27v-6h-6 M11 16l3 3 7-7" />
          </svg>
        </div>
        <h3>Full lifecycle</h3>
        <p>Add, inspect, update, repair, switch source, or remove a plugin through the same CLI.</p>
      </article>
      <article class="registry-benefits__card">
        <div class="registry-benefits__signature" aria-hidden="true">
          <span class="registry-benefits__number">04</span>
          <svg viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M2 16s5-9 14-9 14 9 14 9-5 9-14 9S2 16 2 16Z" />
            <circle cx="16" cy="16" r="4" />
            <path d="M16 2v2 M16 28v2" />
          </svg>
        </div>
        <h3>Source visible</h3>
        <p>
          Reviewed packages and community discovery remain clearly labeled, with their source
          available before install.
        </p>
      </article>
    </div>
  </section>
</template>

<style src="~/assets/styles/registry-benefits.scss" lang="scss" />
