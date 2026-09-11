---
title: "Créer un plugin d'exécution Node/TypeScript"
description: "Le principal chemin non-Go pris en charge pour les plugins d'exécution locaux."
canonicalId: "page:guide:node-typescript-runtime"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Créer un plugin d&#x27;exécution Node/TypeScript</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Créer un plugin d'exécution Node/TypeScript

Il s'agit du principal chemin non-Go pris en charge lorsque votre équipe souhaite TypeScript mais a toujours besoin d'un plugin d'exécution local pris en charge.

## Débit recommandé

```bash
plugin-kit-ai init my-plugin --platform codex-runtime --runtime node --typescript
plugin-kit-ai doctor ./my-plugin
plugin-kit-ai bootstrap ./my-plugin
plugin-kit-ai generate ./my-plugin
plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Ce qu'il faut retenir

- il s'agit d'un chemin d'exécution local stable, pas du chemin Go sans dépendance d'exécution
- la machine d'exécution a encore besoin de Node.js `20+`
- `doctor` et `bootstrap` importent plus ici que dans le chemin par défaut Go

## Quand c'est le bon choix

- votre équipe travaille déjà en TypeScript
- le plugin reste local au dépôt de par sa conception
- vous voulez le chemin principal non-Go pris en charge sans tomber dans une trappe de secours bêta

## Quand Go est encore meilleur

Préférez plutôt Go lorsque :

- vous voulez le contrat de production le plus solide
- vous voulez que les utilisateurs en aval évitent d'installer Node
- vous voulez le moins de frictions d'amorçage dans CI et sur d'autres machines

Voir [Choisir le runtime](/fr/concepts/choosing-runtime) et [Node Runtime API](/fr/api/runtime-node/) pour le niveau de détail suivant.
</details>
