---
title: "Maintain installed plugins"
description: "Traduction indisponible ; consultez la page actuelle en anglais"
canonicalId: "page:use:manage"
section: "use"
locale: "fr"
generated: false
translationRequired: true
---

# Maintain installed plugins

[Traduction indisponible ; consultez la page actuelle en anglais](/en/use/manage). La création Agent Plugins est disponible via Agent Plugins dans `universal-agent-plugins`.

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/)

## Maintenir un projet dans le code actuel

**Code source uniquement ; fonctionnalité non publiée.** Choisissez la même
portée et le même projet pour chaque commande de maintenance :

[Les commandes de maintenance figurent dans la section anglaise](/en/use/manage#maintain-a-project-in-the-current-source).

Versionnez les fichiers portables `uap.toml` et `uap.lock` pour partager les
dépendances et leurs révisions. Les traces de propriété, la récupération et les
données de `.uap/` restent locales ; sauvegardez les données séparément.
La suppression conserve les données et le cache partagé. Repair restaure les
composants manquants et les paquets gérés endommagés ; les fichiers natifs modifiés
nécessitent une vérification. La récupération concerne le projet initialement choisi.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites actuelles par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
