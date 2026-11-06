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
  /* Prevent RTL content, wide tables and decorative orbs from widening the
     document; inner tables retain independent horizontal scrolling. */
  overflow-x: clip;
}
</style>
