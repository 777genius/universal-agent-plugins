---
title: "Find and install a plugin"
description: "Traduction indisponible ; consultez la page actuelle en anglais"
canonicalId: "page:use:install"
section: "use"
locale: "fr"
generated: false
translationRequired: true
---

# Find and install a plugin

[Traduction indisponible ; consultez la page actuelle en anglais](/en/use/install). La création Agent Plugins est disponible via Agent Plugins dans `universal-agent-plugins`.

[Utiliser des plugins](/fr/use/) · [Créer des plugins](/fr/build/)

## Installation par projet dans le code actuel

**Code source uniquement ; fonctionnalité non publiée.** Une compilation
compatible permet de choisir explicitement un projet pour les paquets et Agent Skills :

[Les commandes pour la compilation depuis les sources figurent dans la section anglaise](/en/use/install#project-installation-in-the-current-source).

Le dossier doit exister. La portée par défaut reste `user` ; `project` sans
`--project-root` choisit le dossier courant sans rechercher ses parents.
Le navigateur et le terminal utilisent la même portée. Seules les cibles admises
sont sélectionnables ; les cibles indisponibles peuvent rester visibles avec le
motif. Après la préparation des fichiers, le client peut demander
un redémarrage, la confiance de l’espace de travail ou une configuration de session.

Pour VS Code, la portée projet prépare `.github/skills` et les entrées
`servers` appartenant à l’installation dans `.vscode/mcp.json`. Le résultat reste
Prepared : ouvrez le dossier local sélectionné, examinez la confiance accordée
à l’espace de travail et aux serveurs, puis vérifiez les compétences et les
réglages MCP. Cet adaptateur ne prend pas en charge les espaces distants ni
les espaces comportant plusieurs dossiers racines.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites actuelles par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
