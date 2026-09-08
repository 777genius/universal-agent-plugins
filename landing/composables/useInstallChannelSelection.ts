import type { ComputedRef } from 'vue';
import { useInstallPreferencesStore } from '~/stores/installPreferences';
import type { InstallChannel } from '~/types/content';
import {
  normalizeInstallPlatform,
  recommendedInstallChannelId,
  type InstallPlatform,
} from '~/utils/installPlatform';

export function useInstallChannelSelection(channels: ComputedRef<InstallChannel[]>) {
  const preferences = useInstallPreferencesStore();
  const selectedInstallChannelId = computed(() => preferences.channelId);
  const detectedInstallPlatform = ref<InstallPlatform | null>(null);
  const detectionComplete = ref(false);

  const recommendedChannelId = computed(() => {
    if (detectedInstallPlatform.value) {
      const detectedRecommendation = recommendedInstallChannelId(detectedInstallPlatform.value);
      if (
        detectedRecommendation &&
        channels.value.some((channel) => channel.id === detectedRecommendation)
      ) {
        return detectedRecommendation;
      }

      if (detectedInstallPlatform.value === 'mobile') {
        return null;
      }
    }

    return (
      channels.value.find((channel) => channel.recommended)?.id ?? channels.value[0]?.id ?? null
    );
  });

  // Pinia actions read shared selection state. Track only this consumer's inputs,
  // otherwise overlapping locale route instances can retrigger each other forever.
  watch(
    [() => channels.value.map((channel) => channel.id), recommendedChannelId, detectionComplete],
    ([available, recommendation, ready]) => {
      preferences.reconcileChannels(available);
      // A newly created route has not detected its platform yet. Keep the hydrated
      // selection until detection settles instead of briefly restoring SSR defaults.
      if (!ready && preferences.channelId) return;
      const next = recommendation ?? available.find((id) => id === 'npm') ?? available[0];
      if (next) preferences.selectChannel(next, available, false);
    },
    { immediate: true },
  );

  onMounted(async () => {
    try {
      const { default: Bowser } = await import('bowser');
      detectedInstallPlatform.value = normalizeInstallPlatform(
        Bowser.getParser(window.navigator.userAgent).getOSName(true),
      );
    } catch {
      detectedInstallPlatform.value = null;
    } finally {
      detectionComplete.value = true;
    }
  });

  function selectInstallChannel(channelId: string) {
    if (!channels.value.some((channel) => channel.id === channelId)) {
      return;
    }
    preferences.selectChannel(
      channelId,
      channels.value.map((channel) => channel.id),
    );
  }

  return {
    detectedInstallPlatform,
    recommendedChannelId,
    selectedInstallChannelId,
    selectInstallChannel,
  };
}
