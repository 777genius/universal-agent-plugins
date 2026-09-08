<script setup lang="ts">
import { useLocaleStore } from './stores/locale';
const { locale, t } = useI18n();
const store = useLocaleStore();
const ready = ref(false);
const navigations = ref(0);
const abortNext = useState('fixture:abort', () => false);
const { initLocale } = useLocation();
const remove = useRouter().afterEach((_to, _from, failure) => {
  if (!failure) navigations.value++;
});
onBeforeUnmount(remove);
onMounted(() => { initLocale(); ready.value = true; });
</script>

<template>
  <v-app>
    <main :data-hydrated="ready">
      <LayoutLanguageSwitcher />
      <output data-testid="locale">{{ locale }}</output>
      <output data-testid="preference">{{ store.preferredLocale || 'none' }}</output>
      <output data-testid="navigations">{{ navigations }}</output>
      <p data-testid="message">{{ t('language.label') }}</p>
      <button @click="abortNext = true">Abort next navigation</button>
      <NuxtPage />
    </main>
  </v-app>
</template>
