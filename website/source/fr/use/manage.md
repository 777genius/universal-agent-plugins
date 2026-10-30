---
title: "Maintenir les plugins installés"
description: "Inspectez, mettez à jour, réparez et restaurez les installations utilisateur ou projet."
canonicalId: "page:use:manage"
section: "use"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Maintenir les plugins installés

Utilisez l’installateur pour inspecter et modifier l’état qu’il gère. Pour examiner un problème, conservez avec le rapport la source du paquet, les clients sélectionnés, la portée et l’identité d’installation. Ces exemples utilisent la portée utilisateur, sauf sélection explicite d’un projet.

## Examinez d’abord la situation

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` indique les installations, `doctor` diagnostique l’état géré et `outdated` cherche les mises à jour possibles sans les appliquer. Ces commandes ne prouvent ni la disponibilité d’un service distant ni la réussite de l’authentification d’un client.

| Symptôme | Étape suivante |
| --- | --- |
| Le paquet est absent | Revenez à [la recherche et l’installation](./install). |
| Des fichiers gérés manquent ou ont changé | Examinez un plan de réparation pour ce paquet et cette portée. |
| Une source suivie propose une version plus récente | Examinez un plan de mise à jour. |
| Le client demande confiance, activation ou connexion | Suivez les instructions indiquées pour cette cible. |
| Un paquet local échoue à la validation | Transmettez à l’auteur le rapport et le [guide des vérifications](../build/checks). |

## API de gestion locale (CLI 0.1.95) {#local-management-api}

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) ajoute `agentplugins manage serve` pour les applications qui intègrent l’installateur et `agentplugins manage discover` pour la découverte en lecture seule des clients et profils locaux. Cette découverte n’exécute pas les clients, ne sonde pas leurs versions, ne récupère pas de sources et ne modifie aucun fichier.

`manage serve` est un processus dont l’application hôte gère le cycle de vie. Il capture son contexte au démarrage et expose une API privée authentifiée sur la boucle locale IPv4. L’hôte fournit du JSON sur stdin et garde privés l’URL d’amorçage et le jeton d’autorisation bearer. Consultez l’aide des commandes avant l’intégration :

```bash
agentplugins version
agentplugins manage serve --help
agentplugins manage discover --help
```

Le [contrat du protocole de gestion](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md) décrit les règles de démarrage et de cycle de vie. Les versions du [SDK de gestion TypeScript](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md) sont gérées séparément sous le nom de paquet `universal-agent-plugins-management-sdk`. Consultez son README pour connaître les versions compatibles du moteur `agentplugins` et les instructions d’installation du paquet. Choisissez une version publiée sur npm ou, pour travailler sur des changements pas encore publiés, compilez et testez un paquet à partir d’un commit précis du code source. Fixez les versions du SDK et du moteur.

## Mettez à jour délibérément

```bash
agentplugins update context7 --target codex,cursor --dry-run
```

Examinez la source, la version et les destinations choisies. Omettez `--dry-run` pour appliquer :

```bash
agentplugins update context7 --target codex,cursor
```

`update --all` couvre tous les candidats de mise à jour gérés. Utilisez-le lorsque vous souhaitez traiter cet ensemble complet. Une source fixée à un commit exact reste immuable : une mise à jour ne lui fait pas suivre une branche. Les plages de dépendances enregistrées et les références évolutives avancent via l’opération explicite de mise à jour appropriée.

## Réparez l’état enregistré

```bash
agentplugins repair context7 --target codex,cursor --dry-run
agentplugins repair context7 --target codex,cursor
```

La première commande prévisualise ; la seconde applique. La réparation reprend la source enregistrée pour restaurer le contenu géré. Elle ne choisit pas une nouvelle révision distante et n’écrase pas silencieusement des entrées natives modifiées à l’extérieur. Examinez les conflits et les instructions de récupération en attente avant de poursuivre.

## Supprimez l’installation sélectionnée

```bash
agentplugins remove context7 --target codex,cursor --dry-run
agentplugins remove context7 --target codex,cursor
```

Examinez le premier plan avant d’exécuter la seconde commande. La suppression utilise la propriété enregistrée par le CLI. Elle ne révoque pas un compte de service, ne supprime pas un environnement d’exécution sans rapport et n’efface pas les sources de l’auteur. Par défaut, la suppression d’une installation de projet conserve les données du plugin et le magasin partagé de contenu vérifié.

## Utilisez un environnement de dépendances enregistré

`uap.toml` décrit les paquets et les cibles souhaités ; `uap.lock` enregistre les sources résolues exactes et leurs empreintes. Inspectez l’environnement sélectionné :

```bash
agentplugins deps status
agentplugins deps tree
```

Pour un nouvel environnement, lancez `agentplugins deps init`, modifiez le manifeste délibérément, résolvez-le avec `agentplugins deps lock`, puis appliquez ce verrouillage avec `agentplugins install --locked --scope user`. Examinez les confirmations et utilisez `--dry-run` pour prévisualiser. Le [guide des dépendances](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md) explique le manifeste, les dépendances obligatoires, les plages de versions et les règles de mise à jour.

Depuis CLI 0.1.93, les installations distantes éligibles enregistrent les dépendances par défaut. `--no-save`, les sources locales, les sélecteurs littéraux `discovery:` et la préparation de ChatGPT sont exclus de cet enregistrement portable. Si une sélection mêle ChatGPT à d’autres cibles, séparez-la ou utilisez `--no-save`. Ne supposez pas qu’une ancienne installation impérative figure dans l’accord : vérifiez `deps status` et les fichiers réels avant de les transférer.

## Restaurez sur une autre machine

Transférez un environnement utilisateur enregistré avec un dossier d’échange. Depuis ce dossier sur la machine source :

```bash
agentplugins deps export
```

Copiez le dossier sur la destination, installez les clients cibles, puis exécutez :

```bash
agentplugins deps import
agentplugins install --locked --scope user
```

Vous pouvez aussi combiner importation et restauration :

```bash
agentplugins deps import --install
```

L’argument de dossier facultatif choisit un autre dossier d’échange. L’exportation lit l’environnement utilisateur et l’importation l’écrit ; `AGENTPLUGINS_HOME` peut sélectionner un environnement précis. Le dossier transféré ne remplace pas un profil client. `--scope user` explicite maintient la restauration dans l’environnement utilisateur.

Seuls les octets inchangés de `uap.toml` et `uap.lock`, strictement validés, sont transférés. Chaque copie effective affiche un aperçu et demande une confirmation ou `--yes` ; `--dry-run` prévisualise. La validation de l’importation fonctionne hors ligne, mais l’installation nécessite les sources enregistrées ou des paquets vérifiés en cache. Une restauration sans cache télécharge les révisions exactes et vérifie leurs empreintes ; elle ne sélectionne pas de nouvelles versions.

Les profils clients, réglages, identifiants, reçus et `PLUGIN_DATA` ne sont pas transférés. Configurez l’authentification sur la nouvelle machine et sauvegardez les données séparément. Si l’installation échoue après l’enregistrement des deux fichiers, corrigez la cause et répétez `install --locked --scope user`. Lancez d’abord `deps recover --scope user` si une récupération est en attente. Ne recréez pas les révisions transférées avec `deps lock` : un sélecteur mobile pourrait se résoudre autrement.

<a id="restore-project-dependencies"></a>

## Restaurez et maintenez un projet

L’installation automatique de projet est disponible à partir de [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94). Versionnez les fichiers `uap.toml` et `uap.lock` du projet. Dans une autre copie existante du dépôt, installez les clients requis, puis lancez depuis le répertoire exact du projet :

```bash
agentplugins install
```

Un verrouillage existant est strictement validé. `--locked` exige aussi son existence, ce qui est utile en CI ; sans verrouillage, la première installation peut le créer. Depuis un autre répertoire, sélectionnez explicitement le projet existant :

```bash
agentplugins install --locked --scope project --project-root /path/to/project
```

Sélectionnez la même portée et la même racine pour la maintenance :

```bash
agentplugins list --scope project --project-root /path/to/project
agentplugins deps status --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Si un nom est ambigu, utilisez l’identité d’installation renvoyée par `list`. `add` et les commandes de maintenance utilisent par défaut la portée utilisateur ; sélectionnez le projet explicitement. Depuis 0.1.94, `install` sans argument de source peut le sélectionner automatiquement, comme expliqué ci-dessous. `--scope project` explicite, sans `--project-root`, utilise uniquement le dossier d’invocation.

Gardez les reçus, journaux de récupération et données de `.uap/` en local. Restaurez les composants natifs depuis l’accord portable et suivez les instructions du client pour la confiance ou l’activation. Les sources et l’accès aux sources privées doivent rester disponibles. Consultez [l’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) et les [limites par client](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md).

## Installation automatique de projet dans CLI 0.1.94

**Disponible depuis CLI 0.1.94 :** vérifiez d’abord la version de l’exécutable :

```bash
agentplugins version
```

Ce comportement nécessite 0.1.94 ou une version ultérieure. Les canaux peuvent être mis à jour à des moments différents ; si le vôtre fournit encore une version antérieure, utilisez [l’installateur natif](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) ou Homebrew. Les exemples précédents avec portée explicite restent compatibles avec 0.1.93.

`agentplugins install` sans argument de source sélectionne la portée projet si un fichier `uap.toml` ordinaire existe dans le dossier courant exact. Il ne recherche pas les parents. Lors de la sélection automatique, un dossier ou un lien symbolique nommé `uap.toml` est refusé ; précisez la portée pour lever l’ambiguïté. Une portée explicite reste prioritaire ; sans ce manifeste local, l’environnement utilisateur est sélectionné.

Un `uap.lock` existant est strictement validé avant tout changement, même sans `--locked` : ses révisions épinglées ne sont pas résolues à nouveau. Seule l’absence du verrouillage permet la résolution initiale des dépendances et sa création. `--locked` exige un verrouillage valide existant. Utilisez `update` pour avancer les révisions épinglées ; une installation ordinaire ne sert pas à les modifier. Les exemples précédents avec portée explicite et `--locked` restent une restauration reproductible du projet.

## Signalez un problème clairement

Notez la version du CLI, l’opération, la portée, la source du paquet, les clients et l’activation restante. Précisez si l’échec concerne l’acquisition de la source, la planification, l’application des fichiers, la récupération, la découverte client, l’authentification ou l’exécution. Retirez les identifiants avant de partager les diagnostics. `author doctor ./my-plugin` examine un paquet source ; `agentplugins doctor` diagnostique l’état géré par l’installateur.
