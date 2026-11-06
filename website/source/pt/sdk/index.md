---
title: "SDKs e integrações"
description: "Escolha SDKs de hooks, detecção, instalação e gestão."
canonicalId: "page:sdk:index"
section: "sdk"
locale: "pt"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDKs e integrações

Use SDKs para **integrar Universal Agent Plugins a outro aplicativo** ou tratar eventos nativos dos agentes. Para apenas instalar plugins, consulte [Usar plugins](../use/).

## Escolha um SDK

| Objetivo | Guia | API |
| --- | --- | --- |
| Processar hooks nativos | [Go Hooks](./go-hooks) | `sdk` |
| Descobrir qual agente chamou o hook | [Detecção](./host-detection) | `sdk/hostdetect` |
| Incorporar instalação e recuperação em Go | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Gerenciar plugins no Node/Electron | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## Versões e limites

- Go Hooks: módulo publicado `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`; estabilidade varia **por evento**.
- `sdk/hostdetect` está no código-fonte atual; confira sua presença no tag exato do módulo usado.
- Go Installer é um módulo separado: fixe a revisão e as dependências; não reutilize a versão de hooks.
- Management SDK TypeScript: versão-fonte **0.1.1**, Node.js **22.18+**, API **v1**; confirme a publicação npm.
- CLI **0.1.95+** fornece `manage serve` e `manage discover`. O método `checkUpdates` **não existe no CLI 0.1.95 ou anteriores**.

`hostdetect.Detect` identifica quem **iniciou um processo hook**, enquanto `discoverTargets` encontra **clientes e perfis privados**, e `Engine.Discover` lista adaptadores Go registrados. Nenhum deles instala plugins automaticamente. Validação, instalação de arquivos, ativação nativa e autenticação MCP exigem verificações separadas.

Próximos passos: [Go Hooks](./go-hooks), [detecção](./host-detection), [Go Installer](./go-installer) e [TypeScript Management](./typescript-management).
