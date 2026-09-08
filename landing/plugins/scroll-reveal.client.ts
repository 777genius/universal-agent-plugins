const REVEAL_SELECTOR = '[data-scroll-reveal]';

export default defineNuxtPlugin((nuxtApp) => {
  let observer: IntersectionObserver | undefined;
  let activationFrame: number | undefined;

  const revealSections = () => {
    observer?.disconnect();
    if (activationFrame !== undefined) cancelAnimationFrame(activationFrame);
    document.documentElement.classList.remove('scroll-reveal-active');

    const revealable = [...document.querySelectorAll<HTMLElement>(REVEAL_SELECTOR)];

    for (const section of revealable) section.classList.add('scroll-reveal');

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (reduceMotion || !('IntersectionObserver' in window)) {
      for (const section of revealable) section.classList.add('scroll-reveal--visible');
      document.documentElement.classList.add('scroll-reveal-ready');
      return;
    }

    document.documentElement.classList.add('scroll-reveal-ready');
    activationFrame = requestAnimationFrame(() => {
      document.documentElement.classList.add('scroll-reveal-active');
      observer = new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (!entry.isIntersecting) continue;
            const section = entry.target as HTMLElement;
            section.classList.add('scroll-reveal--visible');
            observer?.unobserve(section);
          }
        },
        { rootMargin: '0px 0px -8% 0px', threshold: 0.06 },
      );
      for (const section of revealable) {
        if (section.isConnected) observer.observe(section);
      }
    });
  };

  nuxtApp.hook('page:finish', () => requestAnimationFrame(revealSections));
});
