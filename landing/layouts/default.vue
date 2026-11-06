<template>
  <v-app class="app-layout">
    <AppHeader />
    <main class="app-layout__main">
      <slot />
    </main>
    <AppFooter />
  </v-app>
</template>

<style scoped>
.app-layout__main {
  flex: 1;
  min-width: 0;
  padding-top: 64px;
  // RTL content, wide support tables and decorative orbs must not expand the
  // document. Inner table regions retain their own horizontal scrolling.
  overflow-x: clip;
}
</style>
