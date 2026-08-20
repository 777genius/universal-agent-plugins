<script setup lang="ts">
const route = useRoute()
const { asset, repositoryUrl } = useSite()
const theme = ref<'light' | 'dark'>('dark')
const menuOpen = ref(false)

onMounted(() => {
  theme.value = document.documentElement.dataset.theme === 'light' ? 'light' : 'dark'
})

watch(() => route.fullPath, () => { menuOpen.value = false })

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  document.documentElement.dataset.theme = theme.value
  try { localStorage.setItem('uap-theme', theme.value) } catch { /* storage is optional */ }
}
</script>

<template>
  <header class="site-header">
    <div class="container site-header__inner">
      <NuxtLink to="/" class="brand" aria-label="Universal Agent Plugins home">
        <img :src="asset('logo.svg')" alt="" width="34" height="34" />
        <span>Universal <b>Agent Plugins</b></span>
      </NuxtLink>
      <nav id="site-navigation" class="site-nav" :class="{ 'site-nav--open': menuOpen }" aria-label="Main navigation">
        <NuxtLink to="/plugins">Directory</NuxtLink>
        <NuxtLink to="/#how-it-works">How it works</NuxtLink>
        <a :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`" target="_blank" rel="noreferrer">Add a plugin</a>
        <a :href="repositoryUrl" target="_blank" rel="noreferrer">GitHub</a>
      </nav>
      <div class="site-header__actions">
        <button class="icon-button" type="button" :aria-label="`Use ${theme === 'dark' ? 'light' : 'dark'} theme`" @click="toggleTheme">
          <span aria-hidden="true">{{ theme === 'dark' ? '☀' : '☾' }}</span>
        </button>
        <button class="menu-button" type="button" aria-controls="site-navigation" :aria-expanded="menuOpen" @click="menuOpen = !menuOpen">
          <span class="sr-only">Toggle navigation</span><span aria-hidden="true">{{ menuOpen ? '×' : '☰' }}</span>
        </button>
      </div>
    </div>
  </header>
</template>
