---
title: "Transmettre un paquet au planificateur d’installation"
description: "Transmettez les sources et leurs preuves en gardant une décision d’installation explicite."
canonicalId: "page:build:handoff"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Transmettre un paquet au planificateur d’installation

Une transmission utile réunit un dossier de paquet standard et une explication claire des vérifications réalisées. L’installateur destinataire prépare son propre plan pour les agents et la portée choisis. Les outils de création compilés depuis le code source peuvent fournir des preuves, mais ne sont pas nécessaires sur la machine destinataire.

## Préparez le paquet et les preuves

Sélectionnez le dossier contenant `plugin.json` à sa racine, plutôt qu’un dépôt parent ambigu. Incluez :

- L’identité, la version et la révision exacte ou la copie examinée du paquet.
- Sa racine relative et le rôle de ses Skills et serveurs MCP.
- Les résultats de validation et d’inspection, avec la version ou révision de l’outil.
- Les identifiants clients visés et la compatibilité par composant.
- Les prérequis d’exécution, d’authentification et les points non évalués.
- Tout test d’exécution explicite, avec serveur, outil et plateforme.

La version du paquet et celle du CLI décrivent des choses différentes. Gardez cette distinction. Évitez les identifiants et les chemins absolus inutiles dans les diagnostics partagés.

## Terminez les vérifications de création disponibles

Ces commandes facultatives nécessitent la [compilation où la création est activée](./#build-from-source), distincte de l’installateur actuel :

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

Cette séquence est statique : elle ne lance pas le code du paquet et n’examine pas le profil d’un véritable utilisateur. Consignez séparément les [tests d’exécution MCP](./checks) réalisés. Une réussite directe ne prouve pas non plus la découverte native par le client ou OAuth.

## Demandez un plan à l’installateur publié

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

L’installateur peut télécharger des métadonnées de sécurité ou son analyseur épinglé même pour une source locale. Sa validation et sa prévisualisation diffèrent des vérifications de création hors ligne.

La première commande vérifie la lecture du paquet par l’installateur. La seconde sélectionne Codex et prévisualise l’installation sans appliquer de changements gérés. Pour un autre client, choisissez une cible prise en charge selon les résultats de compatibilité. Pour un projet, ajoutez explicitement `--scope project --project-root /path/to/project`.

Examinez identité, source, destinations, constats de sécurité et activation restante. Résolvez les écarts avec l’intention de l’auteur avant d’appliquer. Un plan n’est pas un reçu d’installation effectuée.

## Partagez une source reproductible

Pour une distribution distante, utilisez une source acceptée par le CLI destinataire : un paquet de l’annuaire signé, une source GitHub exacte ou une source Git ou archive directe prise en charge. Consultez [les sources d’installation](../use/install) pour leur syntaxe et leur disponibilité. Préférez une révision immuable ou une archive versionnée durable pour une restauration exacte. Un dossier local exige un transfert délibéré distinct.

Le CLI de création source n’expose pas de commande de publication de paquet, d’exportation de bundle ou de soumission à l’annuaire. Héberger les sources est distinct de leur validation. Les déclarations dans `uap.toml` relèvent du contrat du gestionnaire UAP ; `plugin.json` racine définit toujours l’identité du paquet portable.

## Expliquez ce qui reste à faire

| Preuve transmise | Travail restant |
| --- | --- |
| Conformité au standard | Instructions utiles et comportement correct des outils. |
| Vérifications statiques de création | Approbation de publication et qualification des canaux publics. |
| Compatibilité statique du client | Découverte et activation natives effectives. |
| Test MCP direct, si réalisé | Autres outils, clients, authentification et environnements non testés. |
| Prévisualisation de l’installateur | Appliquer les changements examinés et terminer la configuration du client. |

Le destinataire peut suivre [Utiliser des plugins](../use/install) pour installer. Conservez avec le paquet les consignes de confiance et d’activation propres aux clients.
