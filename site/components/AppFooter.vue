<script setup lang="ts">
const { repositoryUrl } = useSite()
</script>

<template>
  <footer class="site-footer">
    <div class="container site-footer__inner">
      <div>
        <strong>Universal Agent Plugins</strong>
        <p>A community directory and installer for the Agent Plugins 1.0 standard.</p>
      </div>
      <nav aria-label="Footer navigation">
        <a href="https://agent-plugins.org/specification" target="_blank" rel="noreferrer">Agent Plugins 1.0</a>
        <a :href="`${repositoryUrl}/blob/main/docs/COMPATIBILITY.md`" target="_blank" rel="noreferrer">Compatibility</a>
        <a :href="`${repositoryUrl}/blob/main/SECURITY.md`" target="_blank" rel="noreferrer">Security</a>
        <a :href="repositoryUrl" target="_blank" rel="noreferrer">Source</a>
      </nav>
    </div>
    <div class="container site-footer__legal">Apache-2.0 project. Third-party product marks belong to their owners. No analytics or tracking scripts.</div>
  </footer>
</template>
