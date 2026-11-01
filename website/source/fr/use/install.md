---
title: "Rechercher et installer un plugin"
description: "Examinez la provenance d’un paquet, prévisualisez ses changements et terminez son activation."
canonicalId: "page:use:install"
section: "use"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Rechercher et installer un plugin

Choisissez un paquet dont vous comprenez la provenance et l’objectif. Les commandes suivantes utilisent l’installateur publié ; consultez [l’installation du CLI](./) si `agentplugins` n’est pas encore disponible. Examinez le plan avec `--dry-run` avant d’appliquer des changements.

## Trouvez le paquet

```bash
agentplugins search docs
agentplugins info context7
```

Un nom court comme `context7` provient du registre signé Universal Agent Plugins Registry. La recherche et les informations du paquet vous aident à choisir ; elles ne l’exécutent pas et ne prouvent pas que son service est disponible. Avant l’installation, vérifiez les outils et Skills proposés, les prérequis d’exécution et les accès aux comptes nécessaires.

## Prévisualisez une sélection explicite

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

Examinez la source, les cibles sélectionnées, les destinations, les constats de sécurité et les étapes d’activation restantes. Cette commande affiche un plan sans appliquer l’installation. Elle ne teste pas un serveur MCP.

## Installez après avoir examiné le plan

```bash
agentplugins add context7 --target codex,cursor
```

Pour une sélection interactive, omettez `--target`. Le CLI peut sélectionner automatiquement un seul agent détecté ou proposer un sélecteur si plusieurs sont disponibles. Utilisez des cibles explicites pour reproduire précisément une sélection et pour l’automatisation.

L’installateur vérifie les clients choisis et protège les fichiers qui ne lui appartiennent pas. Si une opération signale un échec d’annulation ou une récupération en attente, conservez le rapport et suivez ses instructions de récupération avant de réessayer.

## Terminez l’activation

Suivez les étapes propres à chaque client : rechargement, nouvelle session, confiance accordée au projet, connexion ou activation explicite peuvent rester nécessaires. Des fichiers préparés et un enregistrement natif réussi ne prouvent ni la réussite d’OAuth ni celle des appels d’outils. Utilisez le [diagnostic de l’état géré](./manage) si l’installation manque ou si ses fichiers ont changé ; vérifiez l’authentification du client lorsque le paquet est présent mais indisponible.

## Utilisez un paquet local

Sélectionnez le dossier contenant `plugin.json` à sa racine :

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

La validation et la planification de l’installateur appliquent aussi la politique de sécurité. Elles peuvent télécharger des métadonnées de sécurité ou une version épinglée de l’analyseur, même pour un paquet local. `--dry-run` évite les changements gérés d’installation, mais ne promet pas une opération hors ligne.

Après examen du plan, omettez `--dry-run` pour installer. Un dossier local reste une installation locale ; il ne devient pas une dépendance portable qu’une autre machine pourra télécharger. Demandez à l’auteur les [preuves accompagnant le paquet](../build/handoff).

## Sélectionnez une source GitHub exacte

La syntaxe GitHub immuable exige le SHA complet d’un commit, sur 40 caractères. Remplacez cet exemple par un dépôt réel, une révision examinée et le chemin du paquet :

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

La forme `owner/repository@...` refuse les branches, les tags et les SHA abrégés. Sans chemin, l’installateur choisit un paquet valide à la racine ou l’unique candidat imbriqué valide. Plusieurs candidats imposent un chemin explicite ; au-delà de 16 paquets possibles, ce chemin est requis avant leur téléchargement pour évaluation. La réparation reprend la source enregistrée. Passer une installation immuable à une autre source constitue une opération de sélection distincte.

Depuis CLI 0.1.93, Git HTTPS direct, SSH, les URL de navigation des fournisseurs pris en charge et les archives complètes HTTPS ou locales sont acceptés. Remplacez ces exemples par des sources de paquets examinées :

```text
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target codex --dry-run
agentplugins add 'ssh://git@git.example.com/team/tools.git#main//plugins/demo' --target codex --dry-run
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target codex --dry-run
```

Git utilise les identifiants natifs et la confiance SSH existants. Les archives HTTPS n’utilisent pas d’identifiants et doivent contenir un paquet standard complet. Le verrouillage fixe les révisions Git exactes ou les octets originaux de l’archive, ainsi que les empreintes du paquet. Une source évolutive peut avancer lors d’une mise à jour explicite ; une restauration verrouillée conserve la révision fixée. Ces sources de paquets complets diffèrent de l’acquisition de Skills autonomes. Consultez [les sources directes de paquets complets](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md) pour la syntaxe des fournisseurs, la sélection de paquets, les sommes de contrôle et les limites d’extraction.

## Installez des Agent Skills {#install-agent-skills}

Utilisez le groupe `skills` pour des Skills autonomes provenant d’un dépôt GitHub public, d’un dossier ou d’une URL `SKILL.md`, ou d’un éditeur HTTPS proposant un index de Skills complet dans `.well-known` :

```bash
agentplugins skills add owner/repository --ui
```

Remplacez `owner/repository` par une source réelle. `--ui` ouvre le sélecteur intégré dans le navigateur et nécessite un terminal interactif. Utilisez `--no-ui` pour choisir dans le terminal. Pour une sélection explicite :

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- Répétez `--skill` pour des noms exacts. Si un nom est ambigu, utilisez `--path` avec le chemin exact du dossier du Skill dans le dépôt.
- `--ref` sélectionne une référence Git exacte lorsque les limites d’une URL sont ambiguës.
- `--dry-run` prévisualise l’installation choisie.
- L’usage JSON ou non interactif exige une sélection explicite des Skills et des cibles, ainsi que `--yes`, sauf pour une prévisualisation avec `--dry-run`.
- `--ui` ne se combine pas avec `--skill`, `--target` ou `--dry-run`.
- Les éditeurs HTTPS nécessitent un nom `--skill` exact et un index énumérant chaque fichier du Skill.

Les constats de sécurité automatiques peuvent bloquer l’installation. `--security-details` les affiche dans la sortie destinée aux personnes ; examinez-les avant de décider si un `--accept-security-risk` explicite est justifié. Les Skills installés figurent dans `agentplugins list` et utilisent les commandes de maintenance habituelles.

## Installez pour un projet

La portée projet explicite est disponible à partir de [CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93). Choisissez un dossier existant et indiquez la portée à chaque commande :

Pour une entrée vérifiée et compatible du catalogue, copiez sa source Git épinglée à un SHA complet et le client adapté. Les alias du Directory v1, comme `context7`, restent réservés à la portée utilisateur. Remplacez `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` et `PACKAGE_PATH` ; si le paquet est à la racine du dépôt, omettez `//PACKAGE_PATH`.

```bash
agentplugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

La première commande prévisualise ; omettez `--dry-run` pour appliquer le plan. La seconde ouvre un sélecteur avec cette portée et peut appliquer votre sélection. Sans `--project-root`, la portée projet explicite utilise le dossier d’invocation. Elle ne recherche pas dans les dossiers parents. Les commandes `add` et `skills add` utilisent par défaut la portée utilisateur. La sélection automatique de projet par `install` sans argument de source, publiée dans 0.1.94, est expliquée dans [la maintenance](./manage).

La cible doit prendre en charge les composants de projet demandés. L’installation de projet pour ChatGPT n’est pas prise en charge et est refusée explicitement. VS Code prépare `.github/skills` et les membres `servers` appartenant à l’installation dans `.vscode/mcp.json`, pour un seul dossier local. Son résultat reste Prepared : examinez la confiance de l’espace de travail et des serveurs, puis les réglages du client. Cette observation de métadonnées ne prouve ni le démarrage de MCP, ni les appels d’outils, ni l’exécution du modèle.

Consultez le [guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) et les [limites par client](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) pour les chemins et l’activation. Poursuivez avec la [maintenance et la restauration](./manage).
