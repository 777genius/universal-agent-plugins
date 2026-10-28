---
title: "Documentation Agent Plugins"
description: "Guides publics pour installer et créer des paquets Agent Plugins."
canonicalId: "page:home"
section: "home"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">DOCUMENTATION PUBLIQUE</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">Installez des Skills et des outils MCP dans vos agents, ou créez votre propre paquet portable.</p>
</div>

## Choisissez votre parcours

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>Utiliser des plugins</h2>
    <p>Examinez un paquet, prévisualisez ses changements et installez-le pour votre compte ou pour un projet.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>Créer des plugins</h2>
    <p>Préparez un paquet standard avec plugin.json, des Skills et une configuration MCP. Découvrez les outils de création disponibles dans le code source.</p>
  </a>
</div>

## Installez votre premier plugin

Avec Node.js 22 ou une version ultérieure :

```bash
npx universal-agent-plugins add context7
```

Le CLI vous demande quels agents compatibles utiliser. Examinez le plan et suivez les étapes d’activation ou de connexion indiquées. Pour utiliser un exécutable natif, consultez le [guide d’installation native](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) pour macOS, Linux ou Windows. Le gestionnaire natif ne nécessite pas Node.js.

## Ce qui est disponible

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93) est la version actuelle de l’installateur. Elle comprend la sélection explicite de la portée projet et la restauration à partir d’un fichier de verrouillage. Ajoutez `--scope project` à chaque commande visant un projet ; la version publiée utilise par défaut l’environnement utilisateur. Consultez [l’installation](./use/install) et [la maintenance](./use/manage).

L’installateur actuel n’inclut pas `agentplugins author`. Le [guide de création](./build/) explique comment écrire un paquet à la main et utiliser les commandes de création implémentées dans le code source actuel. Les preuves des anciennes versions des outils de création conservent leur portée d’origine ; elles ne décrivent pas l’installateur téléchargé aujourd’hui.

## Interprétez le résultat

La validation du paquet, l’installation, la découverte par le client, l’exécution des outils MCP et l’authentification du service sont des vérifications distinctes. Une installation de fichiers réussie peut encore demander la confiance du client, un rechargement ou une nouvelle session. Les tests natifs couvrent des versions et plateformes précises ; ils ne certifient pas tous les plugins ni tous les agents.

- [Démarrage rapide](./guide/quickstart)
- [Structure du paquet](./build/layout)
- [Vérifications et preuves](./build/checks)
- [Compatibilité des clients et portée des tests](./reference/client-compatibility)
- [Spécification Agent Plugins 1.0](https://agent-plugins.org/specification)


<!-- locale-historical-source:start
locale-historical-frontmatter:start
---
title: "Documentation de plugin-kit-ai"
description: "Documentation publique pour plugin-kit-ai."
canonicalId: "page:home"
section: "home"
locale: "fr"
generated: false
translationRequired: true
---
locale-historical-frontmatter:end
<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">DOCUMENTATION PUBLIQUE</p>
  <h1>plugin-kit-ai</h1>
  <p class="docs-lead">
    Travaillez dans un seul dépôt, commencez par Go par défaut, puis ajoutez plus tard des packages,
    des hooks Claude, Gemini ou une configuration d'intégration gérée par le dépôt sans scinder le projet.
  </p>
</div>

## Démarrage par défaut

- `Codex runtime Go` est le démarrage par défaut lorsque vous souhaitez l'histoire d'exécution et de version la plus solide.

## Ce qu'il faut savoir immédiatement

- un dépôt reste la source de vérité à mesure que vous ajoutez plus de voies
- choisissez le chemin de départ qui correspond à ce dont vous avez besoin aujourd'hui
- développer plus tard à partir du même référentiel lorsque le produit a besoin de plus de sorties
- utilisez `generate` et `validate --strict` comme flux de travail de préparation partagé

## Chemins Node et Python pris en charge

- `codex-runtime --runtime node --typescript` est le principal chemin non-Go pris en charge.
- `codex-runtime --runtime python` est le premier chemin Python pris en charge.
- les deux sont des chemins d'exécution interprétés localement, donc la machine cible a toujours besoin de Node.js `20+` ou Python `3.10+`.
- ce sont des options précoces claires pour les équipes travaillant déjà dans ces piles, mais ce ne sont pas le départ par défaut.

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>Démarrage rapide</h2>
    <p>Utilisez d'abord le chemin par défaut le plus fort, puis développez-le uniquement lorsque le produit a besoin de plus de sorties.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>Voir la forme du produit</h2>
    <p>Découvrez comment un référentiel se transforme en configuration d'exécution, de package, d'extension et d'intégration appartenant au référentiel.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>Choisissez une cible</h2>
    <p>Faites correspondre la cible à la manière dont vous souhaitez expédier le plugin au lieu de traiter chaque sortie comme la même chose.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>Vérifiez le contrat exact</h2>
    <p>Utilisez les pages de référence lorsque vous avez besoin des limites précises du support et des conditions de compatibilité.</p>
  </a>
</div>

## Si vous en avez besoin plus tard

- Ajoutez `Claude default lane` lorsque les crochets Claude sont l'exigence du produit.
- Ajoutez `Codex package` ou `Gemini packaging` lorsque le produit est une sortie de package ou d'extension.
- Ajoutez `OpenCode` ou `Cursor` lorsque le dépôt doit posséder la configuration d'intégration.
- Utilisez `validate --strict` comme porte de préparation avant le transfert ou le CI.

## Chemins d'extension courants

- Commencez avec un référentiel d'exécution Codex, puis ajoutez le package Codex ou Gemini lorsque l'emballage fait partie du produit.
- Commencez par Claude lorsque les hooks Claude sont le produit, puis gardez le dépôt ouvert pour des voies de livraison plus larges plus tard.
- Démarrez sur Node ou Python localement, puis ajoutez un transfert de bundle lorsque la livraison en aval est importante.
- Ajoutez OpenCode ou Cursor lorsque le dépôt doit gérer la configuration d'intégration, pas seulement le comportement de l'exécutable.

## Lire dans cet ordre

<div class="docs-grid">
  <a class="docs-card" href="./guide/quickstart">
    <h2>1. Démarrage rapide</h2>
    <p>Commencez par un chemin recommandé avant de penser à l'expansion.</p>
  </a>
  <a class="docs-card" href="./guide/what-you-can-build">
    <h2>2. Ce que vous pouvez construire</h2>
    <p>Voir la forme du produit à travers les voies d'exécution, de package, d'extension et d'intégration.</p>
  </a>
  <a class="docs-card" href="./guide/choose-a-target">
    <h2>3. Choisissez une cible</h2>
    <p>Choisissez la cible qui correspond à la manière dont vous souhaitez réellement expédier le plugin.</p>
  </a>
  <a class="docs-card" href="./reference/support-boundary">
    <h2>4. Limite de support</h2>
    <p>Utilisez le cluster de référence lorsque vous avez besoin d'un langage de compatibilité exact et de détails de support.</p>
  </a>
</div>

Si vous êtes nouveau, vous pouvez vous arrêter après les pages de départ. Tout le reste est une référence plus profonde ou des détails de mise en œuvre.

## Base de référence actuelle du dépôt

- La référence publique actuelle dans cet ensemble de documents est [`v1.1.2`](/fr/releases/v1-1-2).
- Cette ligne de patch a rétabli la compatibilité d'installation des aliases first-party entre les layouts auteur legacy et actuels, puis a corrigé les installations Gemini multi-target complètes depuis les sources GitHub repo-path.
- Commencez par là lorsque vous souhaitez connaître la ligne de base recommandée actuelle.

## Ce que ce site vous aide à faire

- démarrer un dépôt de plugin au lieu de diviser la source de vérité par écosystème
- choisissez un chemin de départ recommandé sans apprendre tous les détails de la cible à l'avance
- étendre le même dépôt plus tard dans plus de chemins d'expédition
- conserver une histoire de révision et de validation à mesure que le dépôt se développe
- trouvez le contrat exact uniquement lorsque vous en avez besoin
locale-historical-source:end -->
