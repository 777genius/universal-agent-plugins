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

  watchEffect(() => {
    const available = channels.value.map((channel) => channel.id);
    preferences.reconcileChannels(available);
    const next =
      recommendedChannelId.value ??
      channels.value.find((channel) => channel.id === 'npm')?.id ??
      available[0];
    if (next) preferences.selectChannel(next, available, false);
  });

  onMounted(async () => {
    try {
      const { default: Bowser } = await import('bowser');
      detectedInstallPlatform.value = normalizeInstallPlatform(
        Bowser.getParser(window.navigator.userAgent).getOSName(true),
      );
    } catch {
      detectedInstallPlatform.value = null;
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
