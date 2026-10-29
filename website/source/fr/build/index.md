---
title: "Créer des plugins"
description: "Préparez un Skill, un paquet MCP ou un plugin hybride en vérifiant leurs limites."
canonicalId: "page:build:index"
section: "build"
locale: "fr"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Créer des plugins

Un paquet Agent Plugins 1.0 est un dossier avec `plugin.json` à sa racine et des Skills, une configuration MCP ou les deux. Commencez par le plus petit composant répondant au besoin de vos utilisateurs. Un Skill fournit des instructions ; MCP fournit une connexion à des outils.

## Disponibilité

L’installateur natif actuel, [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), peut valider et installer des paquets standard. **Il n’inclut pas `agentplugins author`.** Les commandes de création de ce parcours décrivent des **compilations du code source actuel** où la création est activée. Les preuves de publication des outils de création 0.1.65 restent propres à cette version et ne décrivent pas l’installateur actuel.

Vous pouvez écrire les fichiers à la main et utiliser l’installateur publié, ou compiler les outils ci-dessous pour créer une structure initiale, effectuer des vérifications statiques et lancer explicitement un cycle MCP. Une vérification statique réussie ne prouve ni l’activation par un client natif ni le bon fonctionnement d’une connexion à un service.

## Écrivez un petit paquet à la main

Créez cette structure :

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Définissez l’identité du paquet dans `plugin.json` :

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Review documentation changes for clarity and evidence"
}
```

Commencez `skills/review-docs/SKILL.md` ainsi :

```markdown
---
name: review-docs
description: À utiliser pour examiner la clarté et les preuves d’une documentation.
---

# Examiner la documentation

Identifiez le lecteur et la tâche. Vérifiez les entrées et les effets de chaque
commande. Distinguez le comportement vérifié des prérequis et des affirmations
non testées. Signalez les problèmes avec leur emplacement et une correction.
```

Installez le gestionnaire publié, puis validez et prévisualisez le paquet :

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Examinez le plan et consultez [Utiliser des plugins](../use/install) pour l’appliquer. La [structure du paquet](./layout) explique le rôle de chaque fichier.

## Préparez les outils de création depuis le code source {#build-from-source}

Cette préparation pour développeurs compile le contrat public de création du code source actuel. Elle nécessite Git et la version de Go déclarée par le dépôt, actuellement Go 1.26.10. Notez le commit extrait lorsque vous partagez des résultats.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Utilisez un autre dossier de travail pour les paquets des exemples. Ce binaire est une compilation de développement ; sa présence ne qualifie pas une publication publique des outils de création. Vérifiez le chemin de l’exécutable si le gestionnaire indique encore que la commande `author` est inconnue.

## Choisissez une structure initiale

| Besoin | Parcours | Entrées de modèle obligatoires |
| --- | --- | --- |
| Instructions pour un agent | [Un Skill](./skill) | `--template skill` |
| Connexion à un service existant | [MCP distant](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Processus MCP local en Node | [MCP stdio](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Instructions et outils | [Plugin hybride](./hybrid) | `--template hybrid --mcp-template <choice>` et les entrées de ce modèle MCP |

Un paquet avec un seul Skill contient toujours `plugin.json` à sa racine. Installer des Skills autonomes depuis un autre dépôt relève d’une [opération de l’installateur](../use/install#install-agent-skills) distincte.

## Suivez le cycle de création

1. Créez un paquet dans une destination absente, puis modifiez les fichiers générés.
2. Lancez [validate, inspect et le test statique](./checks) sur la racine exacte.
3. Vérifiez les clients visés avec `compat` et examinez les preuves de `doctor`.
4. Pour MCP, évaluez l’exécution séparément et explicitement avec la compilation du code source.
5. Partagez le paquet et ses preuves au moyen de la [transmission à l’installateur](./handoff).

Le code source actuel propose aussi `bootstrap` fondé sur un fichier de verrouillage, `dev` en continu, `normalize` pour JSON et `import native` explicite pour MCP de Claude. Consultez [les vérifications](./checks) et [MCP stdio](./mcp-stdio). La génération de projections pour les clients, l’exportation de paquets et leur publication restent hors de ces commandes de création. Les hooks propres aux clients restent distincts des composants portables Skill et MCP.
