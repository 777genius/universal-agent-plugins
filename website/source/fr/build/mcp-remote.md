---
title: "Créer un paquet MCP distant"
description: "Décrivez un service MCP existant en distinguant configuration, connectivité et authentification."
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Créer un paquet MCP distant

**Création depuis le code source :** ces commandes nécessitent la [compilation où la création est activée](./#build-from-source). Elles ne figurent pas dans l’installateur actuel.

Choisissez MCP distant si le serveur existe déjà et si le paquet doit décrire la connexion de l’agent. La création enregistre l’URL sans la contacter. Avant de transmettre le paquet, vous devez connaître le transport et les exigences d’authentification du service.

## Indiquez explicitement le point de terminaison

`https://mcp.example.com/mcp` est un exemple de documentation. Remplacez-le par le véritable point de terminaison MCP de votre service. La commande accepte l’URL comme configuration ; elle ne vérifie ni que l’adresse propose MCP ni que votre compte peut l’utiliser.

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

La destination doit être absente, sous un dossier parent existant. Le modèle exige une URL HTTPS absolue, ou HTTP sur l’interface de bouclage. Il n’accepte pas d’identifiants, de fragments ni de paramètres d’URL non résolus en entrée. N’insérez pas de mot de passe dans l’URL.

## Comprenez les fichiers

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

Le manifeste racine définit l’identité du paquet. `mcp.json` décrit le serveur sous `mcpServers` ; ce modèle utilise `type: streamable-http` et l’URL indiquée. Il ne génère pas de serveur et n’ajoute pas de Skill. Choisissez [un paquet hybride](./hybrid) si vous avez aussi besoin de consignes sur l’emploi de ses outils.

Examinez le README et décrivez le rôle du service, les prérequis de compte et les instructions d’authentification. Gardez les secrets hors du paquet. Une URL dans un fichier conforme ne prouve pas que le service fonctionne.

## Vérifiez la configuration et les cibles

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

`inspect` affiche les composants et les exigences non résolues. `compat` évalue la prise en charge statique des adaptateurs des clients choisis. Ces commandes n’exigent pas leur installation. `doctor` ne contacte pas le service. Pour une connexion MCP explicite depuis le code source, consultez [les vérifications d’exécution](./checks#evaluate-mcp-runtime-explicitly) ; un serveur distant nécessite `--allow-network`.

## Indiquez le travail restant

Les preuves statiques doivent identifier le transport et les clients visés. L’installateur destinataire évalue encore sa propre politique et fournit les instructions d’activation. Une connexion au compte, une approbation d’accès ou une nouvelle session peuvent être nécessaires. La découverte et les appels d’outils à l’exécution demandent des preuves distinctes.

Si une vérification statique échoue, corrigez d’abord la configuration enregistrée. Si le service devient inaccessible ensuite, examinez le service ou la connexion du client ; un nouveau test statique réussi ne démontre pas la connectivité.

Poursuivez avec [l’interprétation des rapports](./checks) et [la transmission au planificateur](./handoff).
