---
title: "Intégration CI"
description: "Transformez le flux créé par le public en une porte CI stable pour les projets plugin-kit-ai."
canonicalId: "page:guide:ci-integration"
section: "guide"
locale: "fr"
generated: false
translationRequired: true
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

# Intégration CI

L’histoire de l’IC la plus sûre n’est pas compliquée. C'est juste strict en ce qui concerne le marché public.

<MermaidDiagram
  :chart="`
flowchart LR
  Doctor[doctor] --> Bootstrap[bootstrap si nécessaire]
  Bootstrap -> Générer[générer]
  Générer --> Valider[validate --strict]
  Valider --> Fumée[chèques de fumée ou de bundle]
`"
/>

## La porte CI minimale

Pour la plupart des projets rédigés, voici la base de référence :

```bash
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform <target> --strict
```

Si votre voie comporte des tests de fumée stables ou des contrôles de faisceaux, ajoutez-les après la porte de validation au lieu de la remplacer.

## Pourquoi ça marche

- `doctor` détecte rapidement les prérequis d'exécution manquants
- `generate` prouve que les sorties générées peuvent être reproduites à partir de l'état de création
- `validate --strict` prouve que le repo est cohérent en interne pour la cible choisie
- pour un dépôt multi-cibles, la même logique doit s'appliquer à chaque cible dans la portée du support

## Notes spécifiques à l'exécution

### Go

Go est le chemin CI le plus propre car la machine d'exécution n'a pas besoin de Python ou Node juste pour satisfaire la voie d'exécution.

Pour les dépôts Go basés sur un launcher, construisez d'abord le launcher vérifié :

```bash
go build -o bin/my-plugin ./cmd/my-plugin
plugin-kit-ai doctor .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Node/TypeScript

Ajoutez explicitement bootstrap :

```bash
plugin-kit-ai doctor .
plugin-kit-ai bootstrap .
plugin-kit-ai generate .
plugin-kit-ai validate . --platform codex-runtime --strict
```

### Python

Utilisez le même modèle que Node et rendez la version Python explicite dans CI.

## Erreurs courantes d'IC

- exécuter `validate --strict` sans `generate`
- traiter les artefacts générés comme des fichiers gérés manuellement
- oubli des prérequis d'exécution pour les voies Node ou Python
- compatibilité prometteuse pour une cible située en dehors de la limite de support stable

## Règle recommandée

Si CI ne peut pas reproduire les sorties créées et transmettre `validate --strict`, le dépôt n'est pas prêt pour un transfert stable. Pour un dépôt multi-cibles, cela signifie une exécution verte explicite pour chaque cible à l'intérieur de la portée de support.

Associez cette page à [Préparation à la production](/fr/guide/production-readiness), [Limite de support](/fr/reference/support-boundary) et [Dépannage](/fr/reference/troubleshooting).

</details>
