// VitePress client navigation scrolls to headings without opening enclosing
// disclosures. Keep preserved targets reachable, including encoded IDs.
export function revealHistoricalFragment(document, hash) {
  let id;
  try { id = decodeURIComponent(hash.replace(/^#/, "")); } catch { return false; }
  if (!id) return false;
  const target = document.getElementById(id);
  if (!target?.closest(".vp-doc")) return false;
  let opened = false;
  for (let parent = target.parentElement; parent; parent = parent.parentElement) {
    if (parent.tagName === "DETAILS" && !parent.open) { parent.open = true; opened = true; }
  }
  if (opened) {
    target.scrollIntoView();
    if (!target.hasAttribute("tabindex")) target.setAttribute("tabindex", "-1");
    target.focus({ preventScroll: true });
  }
  return opened;
}

export function installHistoricalFragments(window) {
  const reveal = () => revealHistoricalFragment(window.document, window.location.hash);
  const observer = new window.MutationObserver(reveal);
  observer.observe(window.document.body, { childList: true, subtree: true });
  window.addEventListener("hashchange", reveal);
  window.addEventListener("popstate", reveal);
  reveal();
  return () => {
    observer.disconnect();
    window.removeEventListener("hashchange", reveal);
    window.removeEventListener("popstate", reveal);
  };
}
