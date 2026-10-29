---
title: "Ajouter des Skills"
description: "Complétez un paquet standard existant avec un autre Skill immédiat."
canonicalId: "page:build:skills"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Ajouter des Skills

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). Elles ne figurent pas dans l’installateur actuel.

`skills init` ajoute un Skill à un paquet standard existant : Skill, MCP ou hybride. Sélectionnez la racine contenant `plugin.json`, et non le dossier `skills/` ou un fichier `SKILL.md` isolé.

## Nommez le nouveau Skill et le paquet

Supposons que `./review-helper` soit le paquet du [parcours avec un Skill](./skill) et que `skills/release-notes/` n’existe pas :

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

Le nom positionnel et une description explicite de 1 à 1024 caractères sont obligatoires. Les noms ne sont pas normalisés. Utilisez un nom portable exact, en minuscules avec des tirets, et une destination nouvelle ; ne comptez pas sur l’écrasement d’une destination existante.

## Examinez le composant ajouté

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

Modifiez `skills/release-notes/SKILL.md` pour expliquer son déclenchement, sa méthode et la sortie attendue. Gardez le nom et la description exacts. Par exemple, demandez de distinguer les changements visibles par les utilisateurs de la maintenance et de citer la source examinée de chaque affirmation sur une version.

Scripts, références et ressources peuvent soutenir les instructions, mais leur présence ne prouve ni leur exécution ni l’autorisation de les lancer. `allowed-tools` n’est pas une autorisation indépendante. Examinez les éléments annexes avec le même soin que les instructions.

## Validez le paquet et les Skills immédiats

```bash
agentplugins author skills validate ./review-helper
```

La commande indique la préparation globale du paquet et examine les Skills immédiats séparément. Une erreur ailleurs dans le paquet peut donc affecter le résultat global même si le nouveau Skill ne présente aucun problème.

Sans chemin, seule la racine du dossier courant est utilisée, sans recherche dans les parents. Des racines explicites clarifient scripts et diagnostics. Exécuter depuis `skills/release-notes/` ne remplace pas la sélection de la racine du paquet.

## Corrigez un échec sans écraser votre travail

En cas d’échec de création, lisez `committed` et les instructions de l’erreur. Le Skill peut avoir été enregistré avant l’échec ou l’inachèvement des vérifications du paquet. Examinez les fichiers et le rapport avant de réessayer ; un second `init` n’écrasera pas la destination existante.

Corrigez l’entrée ou le problème du paquet, puis répétez la validation. Si le nom est incorrect, examinez explicitement la modification prévue plutôt que d’ajouter une option de forçage. Ce parcours de création ne comprend pas l’installation, la mise à jour ou la suppression de Skills externes, ni un cycle `generate`.

Poursuivez avec [les vérifications statiques](./checks) et [la transmission](./handoff).
