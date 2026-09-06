---
title: "API"
description: "Référence API générée pour plugin-kit-ai."
canonicalId: "page:api:index"
section: "api"
locale: "fr"
generated: false
translationRequired: true
aside: false
outline: false
---

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/) — Préparation, pas une version publiée. La migration des projets vers v2 est indisponible.
<details><summary>Documentation historique plugin-kit-ai v1 · 1.2.4 ; les anciennes instructions ne constituent pas le parcours Build actuel.</summary>

<div class="docs-hero docs-hero--compact">
  <p class="docs-kicker">RÉFÉRENCE GÉNÉRÉE</p>
  <h1>Surfaces API</h1>
  <p class="docs-lead">
    Cette section rassemble les API publiques de plugin-kit-ai : CLI, Go SDK, helpers runtime, événements de plateforme et capacités.
  </p>
</div>

<div class="docs-grid">
  <a class="docs-card" href="./cli/">
    <h2>CLI</h2>
    <p>Commandes exportées depuis l'arbre Cobra vivant.</p>
  </a>
  <a class="docs-card" href="./go-sdk/">
    <h2>Go SDK</h2>
    <p>Paquets Go publics pour les plugins runtime prêts pour la production.</p>
  </a>
  <a class="docs-card" href="./runtime-node/">
    <h2>Runtime Node</h2>
    <p>Helpers runtime typés pour les consommateurs JS et TS.</p>
  </a>
  <a class="docs-card" href="./runtime-python/">
    <h2>Runtime Python</h2>
    <p>Helpers runtime Python publics uniquement, sans wrappers d'installation.</p>
  </a>
  <a class="docs-card" href="./platform-events/">
    <h2>Événements de plateforme</h2>
    <p>Surfaces événementielles regroupées par plateforme cible.</p>
  </a>
  <a class="docs-card" href="./capabilities/">
    <h2>Capacités</h2>
    <p>Capacités regroupées sur plusieurs plates-formes et événements.</p>
  </a>
</div>

## Ouvrez la bonne surface

- Ouvrez `CLI` lorsque vous avez besoin de commandes, d'indicateurs ou du flux de travail de création.
- Ouvrez `Go SDK` lorsque vous créez un plugin d'exécution prêt pour la production dans Go.
- Ouvrez `Runtime Node` ou `Runtime Python` lorsque vous avez besoin de l'API partagée des helpers pour un runtime local au dépôt.
- Ouvrez `Platform Events` lorsque vous choisissez des événements spécifiques à une cible.
- Ouvrez `Capabilities` lorsque vous souhaitez voir quelles actions et points d'extension existent sur les plates-formes.

## Ce que couvre cette section API

- l'arbre de commandes Cobra en direct
- packages publics Go
- assistants d'exécution partagés pour Node et Python
- événements spécifiques à la plateforme
- métadonnées multiplateformes au niveau des capacités

</details>
