import { directoryIsExpired } from '~/utils/registry'

export function useDirectoryStatus(maintainClock = false) {
  const registry = useRegistry()
  const now = useState('directory-clock', () => Date.now())
  let timer: ReturnType<typeof setTimeout> | undefined

  if (maintainClock) {
    const refreshClock = () => {
      now.value = Date.now()
      if (directoryIsExpired(registry, now.value)) return
      const untilExpiry = registry.expires_at ? Date.parse(registry.expires_at) - now.value + 1 : 60_000
      timer = setTimeout(refreshClock, Math.min(60_000, Math.max(1, untilExpiry)))
    }
    onMounted(refreshClock)
    onBeforeUnmount(() => {
      if (timer) clearTimeout(timer)
    })
  }

  return {
    expired: computed(() => directoryIsExpired(registry, now.value)),
    published: computed(() => registry.data_source === 'published_snapshot'),
    current: computed(() => registry.data_source === 'published_snapshot' && !directoryIsExpired(registry, now.value)),
  }
}
