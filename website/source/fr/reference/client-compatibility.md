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

[CLI natif 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), publié le 28 octobre 2026, comprend la sélection automatique de projet pour `install` sans argument et la validation stricte d’un verrouillage existant. L’installation de projet explicite était déjà disponible dans 0.1.93. La référence native et Homebrew est 0.1.94 ; les canaux peuvent être mis à jour à des moments différents. Vérifiez la version de l’exécutable avant de suivre des instructions propres à 0.1.94. Les preuves de sa publication comme installateur ne qualifient ni l’exécution de tous les clients natifs ni l’exécutable standard de création. L’installateur actuel npm ou natif n’inclut pas `agentplugins author` ; consultez [Créer des plugins](../build/) pour les outils source.

Interprétez la compatibilité comme des vérifications distinctes :

| Vérification | Signification |
| --- | --- |
| Conformité du paquet | Ses fichiers standard satisfont le contrat du lecteur et du schéma. |
| Livraison des composants | L’adaptateur a préparé ou installé les composants qu’il gère. |
| Découverte native | Un client précis a observé le composant attendu. |
| Exécution MCP | Un processus ou une connexion consignée s’est initialisé et a effectué les appels testés. |
| Authentification et modèle | Établis uniquement si les preuves les exercent explicitement. |

La livraison de projet reste souvent **Prepared** tant que confiance, rechargement, configuration explicite ou nouvelle session sont nécessaires. ChatGPT refuse l’installation de projet. VS Code prépare les fichiers d’un seul dossier local ; observer ses métadonnées ne prouve ni le démarrage MCP ni l’exécution du modèle. Consultez [la prise en charge par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) et [l’index des preuves de projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/PROJECT_INSTALLATION.md).

## Comment lire les preuves historiques

Les enregistrements suivants conservent leurs versions, plateformes et limites d’origine. Ils ne s’appliquent pas automatiquement à 0.1.94, à un client ultérieur, à une autre portée ou à un paquet non testé. UAP est un outil communautaire indépendant ; ces observations des mainteneurs ne sont pas une certification du fournisseur.

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
