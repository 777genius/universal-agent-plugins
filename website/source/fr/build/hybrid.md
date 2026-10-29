---
title: "Créer un paquet hybride"
description: "Réunissez des instructions Skill et une configuration MCP sous une identité de paquet."
canonicalId: "page:build:hybrid"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Créer un paquet hybride

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). Elles ne figurent pas dans l’installateur actuel.

Un paquet hybride est utile lorsque les instructions expliquent comment et quand utiliser des outils. Les deux composants partagent un manifeste racine. Chaque client peut les prendre en charge différemment ; conservez les résultats de compatibilité par composant lors de la transmission.

## Choisissez explicitement la partie MCP

L’option est `--mcp-template`. Choisissez `mcp-remote` ou `mcp-stdio`, puis fournissez les entrées obligatoires du modèle. Un paquet hybride ne déduit pas le transport de l’URL, des fichiers locaux ou des environnements d’exécution installés.

Exemple hybride distant :

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

L’URL est un exemple, sans garantie de service. Remplacez-la par votre véritable point de terminaison et consultez [les exigences de MCP distant](./mcp-remote).

## Ou choisissez un processus local

Pour une autre destination également absente, utilisez la variante stdio :

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

N’ajoutez pas `--url` à cette variante. Examinez [les prérequis et limites d’exécution de stdio](./mcp-stdio).

## Reliez les instructions aux outils

La structure essentielle d’un hybride distant est :

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

README et `.gitignore` sont aussi créés. L’hybride stdio ajoute le code Node et les fichiers de dépendances décrits dans son parcours. Examinez `SKILL.md` pour préciser quand employer les outils, quelles entrées fournir et comment signaler un accès manquant. Le texte d’un Skill ne constitue pas une autorisation.

## Vérifiez les deux composants

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

Pour la variante locale, sélectionnez `./local-research`. Conservez les résultats par composant : la prise en charge du Skill ne signifie pas que le client prend en charge le transport MCP ou active le service. La réunion des composants ne supprime pas cette limite.

Poursuivez avec [des Skills supplémentaires](./skills) ou [la transmission](./handoff).
