---
title: "Vérifier un paquet et ses preuves"
description: "Distinguez les vérifications statiques, l’exécution MCP explicite et l’activation du client."
canonicalId: "page:build:checks"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Vérifier un paquet et ses preuves

**Création depuis le code source :** ces commandes `agentplugins author` nécessitent la [compilation où la création est activée](./#build-from-source). L’installateur actuel ne les inclut pas. Lancez les exemples relatifs depuis le dossier parent du paquet et sélectionnez toujours la même racine exacte.

## Lancez le cycle de création hors ligne

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

La validation vérifie la configuration standard enregistrée et la préparation du paquet. L’inspection affiche les composants et les exigences non résolues. Sans options d’exécution, `test` vérifie statiquement configuration, hygiène, Skills et MCP. Ces commandes ne démarrent pas de serveur et n’exécutent pas les instructions d’un Skill.

## Interprétez les niveaux de preuve

| Niveau | Ce qu’il établit | Ce qui exige une autre vérification |
| --- | --- | --- |
| Conformité | Les données standard satisfont les schémas et le profil intégrés. | Un comportement utile dans un agent précis. |
| Sécurité de l’hôte | L’entrée satisfait les vérifications délimitées des fichiers et de l’hôte. | L’autorisation de l’utilisateur d’installer ou d’exécuter. |
| Préparation | Les preuves disponibles satisfont la politique vérifiée. | L’approbation d’une publication ou l’acceptation par une plateforme. |
| Compatibilité | La prise en charge statique des cibles et composants sélectionnés. | La découverte et l’activation effectives dans un vrai client. |
| Chaîne d’outils | Des métadonnées délimitées du projet, des fichiers natifs et des exécutables. | La résolution des dépendances et le démarrage du serveur. |
| Exécution | Un test MCP explicite rapporte les étapes observées. | D’autres outils, clients, flux OAuth ou environnements non testés. |

Lisez séparément la conformité et la préparation globale. Un manifeste valide peut laisser des preuves de composants ou d’hôte incomplètes. Gardez « non évalué » visible, sans le transformer en réussite.

## Sélectionnez les clients explicitement

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

`compat` exige des identifiants de cibles distincts séparés par des virgules et n’accepte pas `all`. Ces vérifications statiques ne nécessitent pas de clients installés. `doctor` examine les fichiers natifs enregistrés et les métadonnées d’exécutables sans processus ni réseau. `capabilities` indique les schémas, métadonnées clients et commandes implémentés par ce moteur ; il ne certifie pas un paquet et ne qualifie pas une publication publique.

## Évaluez explicitement l’exécution MCP {#evaluate-mcp-runtime-explicitly}

Pour le paquet du [parcours stdio](./mcp-stdio), installez séparément ses dépendances verrouillées reconnues et sélectionnez le nom exact du serveur dans `mcp.json` :

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

Le mode stdio nécessite Linux avec un confinement des processus disponible via cgroup v2 délégué, bubblewrap (`bwrap`) du système et une vérification réussie de ses espaces de noms. Node doit se trouver dans les dossiers d’exécution système (`/usr`, `/bin`, `/lib` ou `/lib64`). Si le confinement ne peut pas être démontré, `test` et `dev` refusent le démarrage. Cette exigence du processus stdio ne s’applique pas au transport Streamable HTTP.

Cette commande source exécute le processus MCP choisi et vérifie l’initialisation et la découverte des outils. Pour appeler un outil, indiquez à la fois `--tool` et `--fixture`. Le fichier de test contient un objet JSON d’arguments, avec un chemin relatif au paquet. Pour `hello`, créez `fixtures/hello.json` contenant `{}` :

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

Pour Streamable HTTP, autorisez explicitement le réseau :

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

Utilisez l’identité réelle du serveur du paquet. Le délai est de dix secondes par défaut ; `--deadline` accepte une durée positive d’une minute au maximum. Examinez séparément initialisation, outils, appel facultatif et nettoyage. Une réussite ici ne constitue pas une exécution via un agent installé et ne prouve ni OAuth, ni d’autres outils, ni un déploiement public.

## Itérez avec le cycle de développement source

```bash
agentplugins author dev ./local-tools --server local-tools
```

`dev` lance des cycles MCP délimités quand le paquet change ; arrêtez-le avec Ctrl+C. Utilisez `--once` pour un cycle unique et pour demander une sortie JSON. Les serveurs distants nécessitent `--allow-network` ; les appels d’outils exigent toujours le couple explicite outil et fichier d’arguments. Cette commande n’installe rien dans un client natif.

## Produisez un rapport vérifiable

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

Utilisez `--format human` pour une sortie lisible et `--no-color` au besoin. `--include-root` révèle la racine choisie : évitez les chemins privés inutiles dans les diagnostics partagés. `--release-policy` ajoute des contrôles délimités de préparation à la publication ; ce n’est pas une approbation de publication. Les sous-commandes Skills proposent moins d’options : consultez leur propre aide.

## Maintenez JSON délibérément

Le code source actuel peut planifier la normalisation déterministe d’un fichier standard sélectionné, puis l’écrire uniquement avec `--write` explicite :

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

Sélectionnez `mcp.json` si c’est le document à normaliser. L’importation native lit un fichier MCP Claude explicite en JSON strict ; elle ne recherche pas les profils. Planifiez une nouvelle destination absolue et absente avant d’appliquer :

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Imported Claude MCP package'
```

L’importation accepte uniquement des entrées stdio conservatrices avec `command` et, facultativement, `args`. Les entrées avec URL HTTP, `env`, `type` ou d’autres champs sont ignorées ; elle ne migre pas l’authentification ni une configuration client arbitraire.

Examinez les serveurs sûrs ou ignorés et les champs non pris en charge. Ajoutez `--write` pour enregistrer le paquet validé dans cette destination absente. Les serveurs dangereux ou contenant des identifiants sont ignorés : vérifiez le résultat au lieu de supposer qu’ils ont tous été importés. Aucune importation d’un autre client natif n’est exposée ici.

## Choisissez la bonne commande

`agentplugins validate ./review-helper` lit un paquet pour l’installation ; `agentplugins doctor` diagnostique l’état géré. Ces opérations diffèrent de la création depuis le code source. Les options de portée et de sécurité de l’installateur ne deviennent pas des options de création. `init` crée de vrais fichiers ; normalisation et importation exigent `--write` pour appliquer leurs plans. Poursuivez avec [la transmission du paquet](./handoff).
