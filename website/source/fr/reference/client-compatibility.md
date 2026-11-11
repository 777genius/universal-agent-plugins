---
title: "Compatibilité des clients UAP"
description: "Portée actuelle et guide des preuves historiques par version, plateforme et client."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Compatibilité des clients UAP

## Version actuelle et portée

[CLI](https://github.com/777genius/universal-agent-plugins/releases/latest) ajoute une API de gestion locale dont l’hôte gère le cycle de vie, la découverte en lecture seule des clients et profils, ainsi que les extensions Pi, Skills et MCP stdio/Streamable HTTP en portée utilisateur et projet admissible. Consultez [la gestion locale](../use/manage#local-management-api). Les versions des gestionnaires de paquets peuvent être antérieures ; vérifiez `agentplugins version`.

Depuis CLI 0.1.94, `install` sans argument de source sélectionne le projet si le dossier courant exact contient un fichier `uap.toml` ordinaire. Une portée explicite reste prioritaire et les dossiers parents ne sont pas parcourus. Les verrouillages existants sont strictement validés, même sans `--locked`. La portée projet explicite est disponible depuis 0.1.93. Les preuves de publication de l’installateur ne qualifient ni l’exécution de tous les clients natifs ni l’exécutable standard de création. L’installateur actuel npm ou natif n’inclut pas `agentplugins author` ; consultez [Créer des plugins](../build/) pour les outils source.

Interprétez la compatibilité comme des vérifications distinctes :

| Vérification | Signification |
| --- | --- |
| Conformité du paquet | Ses fichiers standard satisfont le contrat du lecteur et du schéma. |
| Livraison des composants | L’adaptateur a préparé ou installé les composants qu’il gère. |
| Découverte native | Un client précis a observé le composant attendu. |
| Exécution MCP | Un processus ou une connexion consignée s’est initialisé et a effectué les appels testés. |
| Authentification et modèle | Établis uniquement si les preuves les exercent explicitement. |

La livraison de projet reste souvent **Prepared** tant que confiance, rechargement, configuration explicite ou nouvelle session sont nécessaires. ChatGPT refuse l’installation de projet. Consultez [la prise en charge par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) et [l’index des preuves de projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/PROJECT_INSTALLATION.md).

## Hermes et Antigravity : vérification du code source {#hermes-antigravity-source}

Au point de contrôle du 2026-11-10, le CLI publié `0.1.102` ne contient ni Hermes ni Antigravity. Les résultats suivants qualifient le code source, pas un artefact publié. La vérification du binaire de publication immuable et l’admissibilité dans le Directory signé restent deux étapes distinctes en attente. Les capacités du client n’autorisent pas un paquet du Directory pour cette cible.

**Hermes 0.23.0 :** Skills utilisateur et configuration MCP stdio/Streamable HTTP appartenant à UAP ; dans les projets, Skills uniquement. Project MCP est refusé sans redirection vers la configuration utilisateur. Linux x64, macOS arm64 et Windows x64 natif ont réussi le premier ajout et sa répétition, la mise à jour, la réparation, la suppression, la découverte dans un projet sans confiance accordée et l’isolation A/B : [42813277862](https://github.com/777genius/universal-agent-plugins/actions/runs/42813277862), UAP `a50c8a245f1c273efb3d171f4c54ed540475336a`. Quatre échanges HTTP en boucle locale `initialize`/`tools/list` par système prouvent la connexion et la découverte des outils, mais pas `tools/call`, l’utilisation d’un modèle, l’authentification ou l’exécution dans un projet de confiance. SSE et les hooks ne sont pas pris en charge. Les commandes portables sont projetées en Skills ; celles qui exigent une invocation exclusivement explicite sont retenues si le client ne peut pas imposer cette contrainte.

**Antigravity :** les paquets de plugins natifs appartenant à UAP, avec Skills et MCP stdio/Streamable HTTP dans les portées utilisateur et projet, restent **Prepared**. Le véritable `agy` 1.5.3 a réussi la validation stricte et les opérations CLI import/list/uninstall isolées sur Linux, macOS et Windows : [42804680737](https://github.com/777genius/universal-agent-plugins/actions/runs/42804680737). Ces observations ne prouvent ni la découverte ni l’activation dans l’IDE réel, ni le démarrage de MCP, l’exécution d’outils ou l’authentification ; la vérification de l’IDE reste distincte. UAP n’accorde pas sa confiance au projet.

## Extensions Pi, Skills et MCP {#pi-native-extensions}

Le CLI publié `0.1.102` accepte `--target pi` pour les paquets Agent Plugins 1.0 compatibles : [extensions Pi natives](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PI_NATIVE_EXTENSION_CONTRACT.md), Skills et MCP stdio/Streamable HTTP. Les portées utilisateur et projet admissible sont prises en charge ; MCP SSE et les composants App génériques ne le sont pas. Pour un projet, les extensions sont placées dans `.pi/extensions`, les Skills dans `.pi/skills` et les paramètres MCP appartenant à UAP dans `.pi/mcp.json`. Pi peut encore demander l’approbation du projet, un rechargement ou une nouvelle session.

**La capacité du CLI ne constitue pas une autorisation dans le catalogue signé.** Au 9 novembre 2026, aucune des 37 politiques actives du Directory ne déclare la cible `pi`. Ainsi `add context7 --target pi` avec le nom court est correctement refusé. Une source exacte `github:owner/repo@FULL_SHA//path` relève d’un chemin d’installation explicite et distinct, toujours validé par le CLI ; elle ne remplace pas la signature du catalogue.

Les [preuves de projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) et le [vrai Pi `tools/call` (#656)](https://github.com/777genius/universal-agent-plugins/pull/656) couvrent des scénarios précis de Skills/MCP et d’appels avec un modèle scripté local sur Linux, macOS et Windows. Les premières [observations d’extensions](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PI_NATIVE_EXTENSION_EVIDENCE.md) ne concernaient que Linux ; cette limite historique n’est pas une limite actuelle de l’adaptateur. La livraison de fichiers ne prouve ni OAuth, ni modèle distant, ni activation.

## Comment lire les preuves historiques

Les enregistrements suivants conservent leurs versions, plateformes et limites d’origine. Ils ne s’appliquent pas automatiquement à la version actuelle, à un client ultérieur, à une autre portée ou à un paquet non testé. UAP est un outil communautaire indépendant ; ces observations des mainteneurs ne sont pas une certification du fournisseur.

| Enregistrement | Portée et résultat observé | Source vérifiable |
| --- | --- | --- |
| Brouillon historique 0.1.55, septembre 2026 | Deux portées indépendantes : 12 combinaisons client et plateforme, 24 tests principaux obligatoires sans omission. À ce point précis, il s’agissait d’un brouillon non publié. | [Métadonnées exactes](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-draft-0.1.55.json), exécutions [34309083566](https://github.com/777genius/universal-agent-plugins/actions/runs/34309083566) et [34307075390](https://github.com/777genius/universal-agent-plugins/actions/runs/34307075390). |
| Installateur publié 0.1.53 | Binaires publics : neuf combinaisons sur Linux arm64, macOS arm64 et Windows amd64 ; 18 tests obligatoires sans omission. Manifeste et attestations vérifiés avant exécution. | [Publication](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.53), [exécution 34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934), [identités et empreintes](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-released-0.1.53.json). |
| Complément Linux amd64 de 0.1.53 | Mêmes binaires publiés, Ubuntu 24.04 ; trois combinaisons et six tests obligatoires sans omission. Complète les neuf tâches précédentes sans les réécrire. | [Exécution 34195481283](https://github.com/777genius/universal-agent-plugins/actions/runs/34195481283), [identités et empreintes](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-linux-amd64-0.1.53.json). |
| Code source du 7 septembre 2026 | Linux arm64 possède sa propre identité. macOS arm64 et Windows amd64 utilisaient `ef20b93` ; six combinaisons sans omission, 348 empreintes vérifiées. Il ne s’agit ni de binaires publiés ni d’une nouvelle exécution du commit de fusion. | [Identités historiques](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-2026-09-07.json), [exécution 34158664586](https://github.com/777genius/universal-agent-plugins/actions/runs/34158664586). |

Les clients épinglés sont Codex 0.153.4, Claude Code 2.1.263 et OpenCode 1.18.29. Les tests de cycle de vie couvrent installation, changement de version, actualisation de la même version, réparation et suppression. Les tests MCP stdio/HTTP et Skills ont leur propre portée ; n’étendez pas une réussite à un autre transport ou une autre configuration.

Le brouillon 0.1.55 utilisait l’archive npm originale, une installation npm hors ligne avec scripts désactivés, un démarrage à froid avec le binaire figé puis un démarrage sans les sources des preuves. Les deux portées se rattachent au [même producteur](https://github.com/777genius/universal-agent-plugins/actions/runs/34305637370). Revérifier des empreintes hors ligne n’est ni une nouvelle exécution native ni une vérification actuelle de l’état du brouillon. Les journaux et archives complets restent privés en attendant leur revue avant divulgation ; seules les métadonnées sont publiques.

Les preuves 0.1.53 sont publiées dans [une archive distincte](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934), avec [ZIP](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip) et [somme de contrôle](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip.sha256). L’[archive Linux amd64](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-linux-amd64-34195481283) est également indépendante. Le [guide de reproduction](https://github.com/777genius/universal-agent-plugins/blob/main/docs/released-native-client-proof.md) décrit la vérification. Un test du lanceur sur six plateformes n’étend pas à lui seul les preuves natives à d’autres architectures.

## Limites qui restent explicites

- **Claude sur Windows :** les étapes HTTP et Skills installés observées réussissent. Stdio géré indique `managed_stdio_platform_unsupported` ; son exécution et le comportement cwd, argv, environnement et continuité des données ne sont pas établis.
- **OpenCode :** la collision de noms d’outils reste non résolue et produit `opencode_tool_namespace_not_evaluated`. Préserver des clés de configuration distinctes ne garantit pas des noms appelables distincts. Les tests de cycle de vie JSON/JSONC ne prouvent ni stdio ni le corps des Skills en JSONC ; le test étendu concerne JSON global.
- **Codex :** MCP SSE déclaré est exclu. Ses tests de défaillances, d’en-têtes et de redirections ont leur propre portée ; ils ne sont pas attribués automatiquement à Claude ou OpenCode. L’acquisition Git observée ne transforme pas les tests en preuve d’exécution MCP publique via Git.
- **Source et publication :** le Linux historique utilisait un installateur compilé depuis le code source. La version 0.1.51 vérifiée alors ne contenait pas PR #184. N’appliquez pas ces résultats candidats à ce binaire ou à un autre SHA.
- **Environnement :** de vrais clients ont utilisé des fournisseurs de modèle programmés en bouclage local avec accès réseau. Profils jetables et environnements de processus explicites ne constituent pas un isolement du système d’exploitation. Ils ne prouvent ni raisonnement d’un modèle réel, ni OAuth, connexion, applications de bureau, services publics, adoption indépendante ou certification officielle. La protection observée des fichiers étrangers ne garantit pas la sûreté face à des écritures concurrentes arbitraires.

L’[ancien enregistrement macOS](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md) d’UAP 0.1.22 ne couvre que découverte et cycle de vie avec ses propres clients et empreintes. Il ne qualifie pas l’exécution actuelle. La [référence historique détaillée en anglais](/en/reference/client-compatibility) conserve toutes les empreintes, identités des bancs de test, matrices et distinctions ; cette page en fournit le guide de lecture en français.

## Preuves de la portée projet

La livraison de projet explicite a été publiée dans CLI 0.1.93. Propriété des fichiers, découverte native et fonctionnement MCP ou modèle sont rapportés séparément pour chaque client. Les réussites en portée utilisateur ne qualifient pas la portée projet.

L’[observation épinglée de VS Code 1.143.1](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034) a vérifié les métadonnées dans des dossiers A/B séparés, un nouveau lancement de A et la suppression, sans accorder la confiance à l’espace de travail. L’adaptateur prépare `.github/skills` et `.vscode/mcp.json`, garde le résultat Prepared et vérifie les fichiers installés. Il ne revendique ni démarrage MCP, ni appels d’outils, ni exécution de modèle.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
