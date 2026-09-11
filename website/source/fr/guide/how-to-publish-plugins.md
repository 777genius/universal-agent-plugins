---
title: "Comment publier des plugins"
description: "Un guide pratique pour publier des projets plugin-kit-ai sur Codex, Claude et Gemini sans confondre l'application locale avec la planification de la publication."
canonicalId: "page:guide:how-to-publish-plugins"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<p class="locale-historical-identity">Comment publier des plugins</p>

[Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.](/en/legacy/v1/)

<details><summary>Instantané historique de la branche plugin-kit-ai v1. 1.2.4 est la référence des commandes, pas la version exacte de cet instantané.</summary>

# Comment publier des plugins

Utilisez ce guide lorsque votre dépôt est déjà créé dans `plugin-kit-ai` et que vous souhaitez connaître la prochaine étape la plus claire pour la publication Codex, Claude ou Gemini.

Commencez par un dépôt où `plugin-kit-ai generate .` et `plugin-kit-ai validate . --strict` passent déjà, afin que les commandes de publication lisent des artefacts gérés à jour plutôt que des fichiers obsolètes.

## Ce que couvre ce guide

- quelles plates-formes prennent en charge les applications locales réelles aujourd'hui
- quelle plate-forme utilise plutôt la planification et la préparation
- quelle commande exécuter en premier
- quel résultat attendre une fois la commande terminée

## Comparaison rapide

| Plateforme | Modèle de publication | Réel appliquer dans `plugin-kit-ai` | Commande principale | Ce que vous obtenez |
|---|---|---:|---|---|
| Codex | racine du marché local | oui | `publish --channel codex-marketplace` | `.agents/plugins/marketplace.json` plus `plugins/<name>/...` |
| Claude | racine du marché local | oui | `publish --channel claude-marketplace` | `.claude-plugin/marketplace.json` plus `plugins/<name>/...` |
| Gemini | préparation au dépôt/sortie | non | `publish --channel gemini-gallery --dry-run` | un plan de publication limité et des diagnostics de préparation |

## La règle courte

- utilisez `publish` lorsque vous souhaitez un workflow de publication
- utilisez `publication` lorsque vous souhaitez d'abord une inspection ou une vue médicale
- Codex et Claude prennent en charge les applications locales réelles aujourd'hui
- Gemini utilise la publication de planification et de préparation dans la v1, et non l'application locale

La forme du dépôt reste la même :

- `plugin.yaml` est le manifeste principal du plugin
- `targets/...` contient des entrées créées spécifiques à la cible
- `publish/...` détient une intention de publication
- `publication` est la surface d'inspection et de docteur
- `publish` est la surface du workflow de publication

## Publier sur Codex

Pour Codex, la publication signifie matérialiser une racine de marché locale.

Exécutez ceci en premier :

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace --dry-run
```

Appliquez-le lorsque le plan semble correct :

```bash
plugin-kit-ai publish ./my-plugin --channel codex-marketplace --dest ./local-codex-marketplace
```

Résultat attendu :

- `.agents/plugins/marketplace.json`
- `plugins/<name>/...`

Une racine locale comme celle-ci peut déjà servir de source de plugin Codex.

## Publier sur Claude

Pour Claude, publier signifie aussi matérialiser une racine de marché local.

Exécutez ceci en premier :

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace --dry-run
```

Appliquez-le lorsque le plan semble correct :

```bash
plugin-kit-ai publish ./my-plugin --channel claude-marketplace --dest ./local-claude-marketplace
```

Résultat attendu :

- `.claude-plugin/marketplace.json`
- `plugins/<name>/...`

## Publier sur Gemini

Pour Gemini, la publication ne signifie **pas** la création d'une racine de marché locale.

Dans la v1, `plugin-kit-ai` fait trois choses limitées :

- valide l'intention de publication
- vérifie l'état de préparation du référentiel
- construit un plan de publication

Commencez par être prêt :

```bash
plugin-kit-ai publication doctor ./my-plugin --target gemini
```

Inspectez ensuite le plan de publication :

```bash
plugin-kit-ai publish ./my-plugin --channel gemini-gallery --dry-run
```

Prérequis attendus :

- un référentiel public GitHub
- une télécommande `origin` valide pointant vers GitHub
- le sujet GitHub `gemini-cli-extension`
- `gemini-extension.json` à la bonne racine

Gemini utilise la publication de planification et de préparation dans la v1, et non l'application locale.

## Plan sur tous les canaux créés

Utilisez-le lorsqu'un dépôt crée plusieurs canaux de publication :

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --dest ./local-marketplaces --format json
```

Règles importantes :

- il utilise uniquement les canaux `publish/...` créés
- il ne déduit pas les chaînes de `targets`
- c'est une planification uniquement dans la v1
- `--dest` est requis uniquement lorsque les canaux créés incluent Codex ou Claude flux de marché local.
- L'orchestration uniquement Gemini ne nécessite pas `--dest`

Si les auteurs du dépôt ne sont que `gemini-gallery`, cela fonctionne également :

```bash
plugin-kit-ai publish ./my-plugin --all --dry-run --format json
```

## Quelle commande dois-je exécuter ?

- Je veux une racine de marché Codex locale : `plugin-kit-ai publish --channel codex-marketplace --dest <marketplace-root>`
- Je veux une racine de marché Claude locale : `plugin-kit-ai publish --channel claude-marketplace --dest <marketplace-root>`
- Je veux que Gemini soit prêt à être publié : `plugin-kit-ai publication doctor --target gemini`
- Je souhaite un plan de publication Gemini : `plugin-kit-ai publish --channel gemini-gallery --dry-run`
- Je souhaite un plan de publication combiné : `plugin-kit-ai publish --all --dry-run` et j'ajoute `--dest <marketplace-root>` lorsque Codex ou Claude chaînes créées sont incluses.

## Lectures complémentaires

- [CLI Section de publication README](https://github.com/777genius/plugin-kit-ai/tree/main/cli/plugin-kit-ai)
- [`plugin-kit-ai publish`](/fr/api/cli/plugin-kit-ai-publish)
- [`plugin-kit-ai publication`](/fr/api/cli/plugin-kit-ai-publication)
- [`plugin-kit-ai publication doctor`](/fr/api/cli/plugin-kit-ai-publication-doctor)

</details>
