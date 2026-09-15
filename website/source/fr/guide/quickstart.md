---
title: "Démarrage rapide"
description: "Installez des plugins disponibles ou préparez un paquet portable Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

<a id="demarrage-rapide"></a>

# Utiliser des plugins / Créer des plugins

Installez des plugins disponibles ou préparez un paquet portable Agent Plugins 1.0.

## Utiliser des plugins {#use-plugins}

Disponible maintenant : installez des plugins avec Universal Agent Plugins. Avec Node.js 22+, exécutez :

```bash
npx universal-agent-plugins add context7
```

Pour une installation native sur macOS, Linux ou Windows, consultez le guide. Le CLI demande quels agents compatibles utiliser ; chaque plugin peut nécessiter son propre environnement d’exécution.

[Guide d’installation](https://github.com/777genius/universal-agent-plugins#quick-start)

La création Agent Plugins est disponible dans `universal-agent-plugins@0.1.65` ; version GitHub : `agentplugins-v0.1.65`.

La compatibilité dépend du paquet. La validation du schéma ne prouve ni l’exécution, ni OAuth, ni l’activation. Codex ne prend pas en charge MCP SSE déclaré ; stdio et Streamable HTTP conservent leur prise en charge par les adaptateurs existants.

[Compatibilité des clients (en anglais)](https://github.com/777genius/universal-agent-plugins#supported-clients)

## Créer des plugins {#build-plugins}

Créez un paquet portable Agent Plugins 1.0 autour de plugin.json, avec skills/ et mcp.json facultatifs. La prise en charge dépend du paquet et de ses composants.

**La création Agent Plugins est disponible**

La création statique est disponible via `agentplugins author` dans `universal-agent-plugins@0.1.65` ; version GitHub : `agentplugins-v0.1.65`. npm, Homebrew, archives natives et E2E public sont vérifiés. Cette version n’exécute pas le code du paquet ni les serveurs de développement, n’installe pas les dépendances, n’exporte pas les paquets et ne les publie pas.

[Spécification Agent Plugins 1.0](https://agent-plugins.org/specification)
