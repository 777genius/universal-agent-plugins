---
title: "Créer un paquet avec un Skill"
description: "Préparez un paquet standard contenant des instructions utiles pour une tâche répétable."
canonicalId: "page:build:skill"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Créer un paquet avec un Skill

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). Elles ne figurent pas dans l’installateur actuel.

Choisissez ce parcours si l’agent a besoin d’une méthode répétable, d’une liste de vérification ou de consignes de rédaction, sans connexion à des outils MCP. Le paquet contient un Skill et un manifeste racine. Ces étapes de création ne nécessitent ni serveur, ni installation d’un environnement d’exécution, ni profil client.

## Choisissez une destination explicite

Travaillez dans un dossier parent jetable où `./review-helper` n’existe pas. Choisissez une identité portable en minuscules. Le chemin désigne la destination ; `--name` désigne le nom du paquet dans son manifeste.

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

`init` exige une destination absente et n’écrase pas de dossier existant. Si un résultat enregistré est suivi d’un échec, examinez les fichiers avant de réessayer. Ce parcours ne comporte pas d’option d’écrasement.

## Examinez les fichiers créés

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

Le Skill généré est un point de départ. Remplacez son corps par une méthode concrète et utile, en gardant le nom du Skill cohérent avec son dossier. Par exemple, le corps de `skills/review-docs/SKILL.md` pourrait être :

```markdown
# Examiner les changements de documentation

Utilisez ce Skill pour examiner une modification avant sa transmission.

1. Identifiez le lecteur et la tâche que la page l’aide à terminer.
2. Vérifiez que chaque commande indique ses entrées et décrit ses effets.
3. Distinguez le comportement vérifié des prérequis et des points non testés.
4. Signalez les consignes ambiguës et proposez une phrase de remplacement.

Renvoyez une courte liste de problèmes avec leurs emplacements. Si vous n’en
trouvez aucun, indiquez les pages examinées et les vérifications hors périmètre.
```

Conservez les métadonnées initiales générées au-dessus du corps. `name` et `description` identifient le Skill et aident l’agent à décider quand l’utiliser. Ses instructions n’autorisent pas l’exécution de scripts ni l’accès à des ressources privées.

## Vérifiez la racine exacte du paquet

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

Ces vérifications sont statiques : elles ne demandent pas à un agent de suivre les instructions et n’évaluent pas sa réponse. Examinez séparément la conformité et la préparation. Un Skill bien formé nécessite encore une revue humaine de son utilité, de son exactitude et de sa portée.

Poursuivez avec [des Skills supplémentaires](./skills), [les vérifications des cibles](./checks) ou la [transmission à l’installateur](./handoff). Pour réunir outils et instructions dans un nouveau paquet, suivez le [parcours hybride](./hybrid).
