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
  /* This is a flex item: wide tables must scroll inside their own region,
     not increase the width of the entire application and its background. */
  min-width: 0;
  width: 100%;
  padding-top: 64px;
}
</style>
