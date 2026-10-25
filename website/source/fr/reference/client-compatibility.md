---
title: "Compatibilité des clients UAP"
description: "Les preuves détaillées de compatibilité sont tenues à jour en anglais."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "english-fallback"
---

# Compatibilité des clients UAP

Les preuves détaillées, les versions testées et leurs limites sont tenues à jour sur la page canonique en anglais.

[Consulter les preuves en anglais](/en/reference/client-compatibility)

## Portée projet dans le code actuel

**Code source uniquement ; fonctionnalité non publiée.** Les observations
historiques des versions et clients ci-dessus gardent leur portée initiale.
L’installation par projet a un contrat propre à chaque client : propriété des
fichiers, découverte native et fonctionnement MCP/modèle sont vérifiés séparément.
ChatGPT refuse explicitement cette portée ; les autres cibles ont leurs propres
limites de composants, de préparation et de client compatible.

La préparation de projets pour VS Code utilise `.github/skills` et
`.vscode/mcp.json`. L’[observation de VS Code 1.143.1 épinglé](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034)
a vérifié les métadonnées dans des dossiers A/B distincts, après un nouveau
lancement de A et après suppression, sans accorder la confiance à l’espace de
travail. Cette preuve concerne les métadonnées : l’adaptateur indique Prepared
et vérifie les fichiers installés, sans attester le démarrage de MCP,
les appels d’outils ou l’exécution du modèle.

[Guide d’installation par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Compatibilité et limites actuelles par projet](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
