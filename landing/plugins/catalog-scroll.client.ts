/** Catalog state belongs to the current viewport, not an earlier section anchor. */
export default defineNuxtPlugin((nuxtApp) => {
  const router = useRouter();
  // Nuxt installs its final scroll handler during app:created. Wrap it only
  // after mounting so normal page, anchor and history navigation keep that handler.
  nuxtApp.hook('app:mounted', () => {
    const original = router.options.scrollBehavior;
    router.options.scrollBehavior = (to, from, savedPosition) => {
      if (savedPosition) return savedPosition;
      if (
        to.path.replace(/\/$/, '') === from.path.replace(/\/$/, '') &&
        JSON.stringify(to.query) !== JSON.stringify(from.query) &&
        (!to.hash || to.hash === from.hash) &&
        document.querySelector('.unified-catalog')
      )
        return false;
      return original?.(to, from, savedPosition);
    };
  });

  // Static pages restore their initial query/hash after async layout hydration.
  // This is navigation, so run after that restoration rather than at idle time.
  nuxtApp.hooks.hookOnce('app:suspense:resolve', async () => {
    await nextTick();
    const route = router.currentRoute.value;
    // Older catalog links retained this unrelated homepage anchor. Repair only
    // initial search links; later intentional navigation to the section still works.
    if (
      route.hash === '#plugin-dependencies' &&
      (route.query.tab === 'skills' ||
        (typeof route.query.q === 'string' && Boolean(route.query.q.trim()))) &&
      document.querySelector('#plugins .unified-catalog')
    )
      await router.replace({ path: route.path, query: route.query, hash: '#plugins' });
  });
});
