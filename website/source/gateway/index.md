---
title: "Language Gateway"
description: "Choose the public docs locale."
layout: "page"
editLink: false
head:
  - - meta
    - name: robots
      content: noindex,follow
  - - script
    - id: locale-gateway-redirect
    - |
      (() => {
        const query = new URLSearchParams(window.location.search);
        const gateway = (query.get("gateway") || "").toLowerCase();
        if (gateway === "1" || gateway === "true" || gateway === "manual") {
          return;
        }

        const storageKey = "plugin-kit-ai-docs-locale";
        const candidates = [];
        const supportedLocales = ["en", "ru", "es", "fr", "zh"];
        try {
          const saved = window.localStorage.getItem(storageKey);
          if (supportedLocales.includes(String(saved).toLowerCase())) {
            candidates.push(saved);
          }
        } catch {
          // localStorage is optional enhancement only.
        }

        const browserLanguages = Array.isArray(window.navigator.languages) && window.navigator.languages.length > 0
          ? window.navigator.languages
          : [window.navigator.language || ""];
        candidates.push(...browserLanguages);

        const locale = candidates
          .map((candidate) => String(candidate).toLowerCase().split(/[-_]/)[0])
          .find((candidate) => supportedLocales.includes(candidate)) || "en";
        try {
          window.localStorage.setItem(storageKey, locale);
        } catch {
          // localStorage is optional enhancement only.
        }

        const pathname = window.location.pathname.endsWith("/") ? window.location.pathname : `${window.location.pathname}/`;
        const target = `${pathname}${locale}/`;
        if (window.location.pathname !== target) {
          window.location.replace(`${target}${window.location.hash}`);
        }
      })();
---

<LanguageGateway />
