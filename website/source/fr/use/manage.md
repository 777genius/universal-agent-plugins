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

**Disponible dans CLI 0.1.94 et versions ultérieures.** L’installation sans argument
identifie le projet par son fichier de dépendances dans le répertoire courant,
sans chercher dans les répertoires parents. Le fichier de verrouillage existant
est validé strictement avant toute modification. Le choix explicite de la portée
reste prioritaire ; les autres commandes de maintenance demandent de choisir
explicitement la portée et le projet.

[Les commandes de maintenance figurent dans la section anglaise](/en/use/manage#maintain-a-project-in-the-current-source).

Versionnez les fichiers portables `uap.toml` et `uap.lock` pour partager les
dépendances et leurs révisions. Les traces de propriété, la récupération et les
données de `.uap/` restent locales ; sauvegardez les données séparément.
La suppression conserve les données et le cache partagé. Repair restaure les
composants manquants et les paquets gérés endommagés ; les fichiers natifs modifiés
nécessitent une vérification. La récupération concerne le projet initialement choisi.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites actuelles par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
