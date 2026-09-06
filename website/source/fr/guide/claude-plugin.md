---
title: "Créer un plugin Claude"
description: "Un guide ciblé pour le chemin stable du plugin Claude dans plugin-kit-ai."
canonicalId: "page:guide:claude-plugin"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Créer un plugin Claude</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Créer un plugin Claude

Choisissez ce chemin lorsque vous ciblez explicitement les hooks Claude au lieu du chemin d'exécution par défaut Codex.

## Point de départ recommandé

```bash
plugin-kit-ai init my-claude-plugin --platform claude
cd my-claude-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform claude --strict
```

## Ce que signifie ce chemin

- le projet vise l'exécution du hook Claude
- le sous-ensemble stable est plus restreint que l'ensemble complet des fonctionnalités d'exécution Claude
- `validate --strict` reste le principal contrôle de préparation

## Utilisez les crochets étendus avec précaution

```bash
plugin-kit-ai init my-claude-plugin --platform claude --claude-extended-hooks
```

Choisissez des crochets étendus uniquement lorsque vous souhaitez intentionnellement un ensemble pris en charge plus large et que vous acceptez une stabilité plus lâche que le sous-ensemble stable.

## Quand ce chemin convient

- un plugin qui doit s'intégrer aux hooks d'exécution Claude
- les équipes qui souhaitent un dépôt et un flux de travail au lieu d'éditer manuellement les artefacts Claude natifs
- les utilisateurs qui ont besoin d'une structure plus solide que les scripts locaux ad hoc

## Prochaines étapes

- Lisez [Modèle cible](/fr/concepts/target-model) pour voir en quoi Claude diffère des cibles d'empaquetage ou de configuration d'espace de travail.
- Vérifiez [Événements de plateforme](/fr/api/platform-events/claude) pour une référence au niveau de l'événement.

</details>
