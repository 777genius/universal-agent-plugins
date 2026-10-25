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

```sh
agentplugins add owner/repo --target cursor \
  --scope project --project-root /path/to/project
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
```

Le dossier doit exister. La portée par défaut reste `user` ; `project` sans
`--project-root` choisit le dossier courant sans rechercher ses parents.
Le navigateur et le terminal utilisent la même portée et proposent uniquement
les cibles admises. Après la préparation des fichiers, le client peut demander
un redémarrage, la confiance de l’espace de travail ou une configuration de session.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites actuelles par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
