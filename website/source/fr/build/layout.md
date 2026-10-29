---
title: "Comprendre la racine du paquet"
description: "Gardez l’identité et les composants portables dans une racine standard explicite."
canonicalId: "page:build:layout"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Comprendre la racine du paquet

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). L’installateur actuel n’inclut pas `agentplugins author`.

Le dossier sélectionné est la racine du paquet. Les outils de création lisent son `plugin.json` et les composants standard. Ils ne cherchent pas de projet dans les dossiers parents et ne se rabattent pas sur le manifeste caché d’un client natif.

## Structures minimale et combinée

Paquet avec un Skill :

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Paquet hybride distant :

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

Un paquet stdio contient aussi le code du serveur et les manifestes de dépendances. Ces fichiers sont nécessaires à une exécution ultérieure, mais leur présence ne fait pas exécuter de code par le lecteur de création.

## Donnez un rôle clair à chaque fichier

| Fichier ou dossier | Rôle |
| --- | --- |
| `plugin.json` | Identité et métadonnées standard du paquet. |
| `mcp.json` | Configuration des serveurs MCP lorsque le paquet fournit des outils. |
| `skills/<name>/SKILL.md` | Identité, description et instructions d’un Skill immédiat. |
| `README.md` | Usage prévu, prérequis, validation et limites restantes. |
| `package.json`, `package-lock.json`, `src/server.mjs` | Entrées de la structure Node stdio pour le travail d’exécution distinct. |
| `LICENSE` | Licence choisie explicitement, si indiquée pendant `init`. |

Le manifeste généré commence à la version `0.1.0`. C’est une métadonnée de votre nouveau paquet, pas la version de l’exécutable de création. Distinguez-les lorsque vous communiquez des versions aux utilisateurs.

## Identité et métadonnées facultatives

Indiquez explicitement `--name` et `--description` dans des instructions `init` reproductibles. Le contrat public peut déduire un nom d’une destination simple et fournit une description par défaut déterministe, mais un chemin comme `./review-helper` doit avoir un nom explicite. Ces exemples donnent les deux pour éviter une identité accidentelle.

Dans un modèle contenant un Skill, `--skill-name` reprend le nom du paquet s’il est omis. Choisissez-le explicitement si le Skill a besoin d’une autre identité. Les noms de Skills ne sont jamais normalisés silencieusement.

`--author-name` fournit un auteur facultatif explicite ; aucun auteur n’est déduit de Git ou de l’environnement. La génération de licence accepte les choix explicites MIT ou ISC et exige `--copyright-holder` ainsi qu’un `--copyright-year` sur quatre chiffres. Aucune licence n’est choisie par défaut.

## Sélectionnez toujours la même racine

```bash
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper
agentplugins author test ./research-helper
```

Les trois commandes sélectionnent la même racine. `./research-helper/skills` serait le mauvais dossier. Pour les commandes de lecture, le chemin facultatif utilise uniquement le dossier courant, sans découvrir un parent.

La conformité au standard et la sécurité du système de fichiers sont distinctes. Une identité conforme au schéma peut être inadaptée à un chemin sur la machine, et un manifeste lisible peut laisser des preuves de composants incomplètes. Examinez le rapport entier.

Poursuivez avec [les vérifications statiques](./checks), puis [la transmission](./handoff).
