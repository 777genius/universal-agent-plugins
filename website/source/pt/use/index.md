---
title: "Usar plugins"
description: "Escolha entre instalar e criar plugins e siga o caminho adequado."
canonicalId: "page:use:index"
section: "use"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Usar plugins {#use-plugins}

O Universal Agent Plugins instala pacotes padrão nos agentes de IA que você
seleciona. Um pacote pode conter Skills, configuração de servidores MCP ou ambos.
Você pode instalar para sua conta ou manter as dependências de um projeto separadas.

## Obtenha a CLI {#get-the-cli}

Com Node.js 22 ou mais recente, execute o wrapper npm diretamente:

```bash
npx universal-agent-plugins search docs
```

Para uso frequente, instale-o globalmente:

```bash
npm install -g universal-agent-plugins
agentplugins version
```

Outra opção é o Homebrew:

```bash
brew install 777genius/agentplugins/agentplugins
```

O [guia de instalação nativa](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
traz instruções para macOS, Linux e Windows. A instalação nativa não exige
Node.js para o gerenciador, embora plugins individuais possam precisar de seu
próprio ambiente de execução. Os exemplos deste caminho usam `agentplugins`;
`npx universal-agent-plugins` executa os mesmos comandos do instalador.

## Siga o caminho de instalação {#follow-the-installation-journey}

1. [Encontre e inspecione um pacote](./install). Confira sua origem e seus pré-requisitos.
2. Confira o plano para os destinos e o escopo desejados com `--dry-run`.
3. Aplique a instalação revisada e siga as instruções de ativação.
4. [Mantenha o estado instalado](./manage) com listagem, atualização, reparo e remoção.
5. Quando necessário, restaure um conjunto compartilhado de dependências a partir de `uap.toml` e `uap.lock`.

A [CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
é o instalador nativo atual. Ela adiciona uma API local de gerenciamento e
extensões nativas do Pi no escopo de usuário. Consulte
[gerenciamento local](./manage#local-management-api) e
[compatibilidade com o Pi](../reference/client-compatibility#pi-native-extensions).
Canais de gerenciadores de pacotes podem demorar para receber a atualização;
confira `agentplugins version`. O instalador nativo fornece a versão atual
verificada.

O escopo de projeto explícito funciona desde a 0.1.93. A maioria dos comandos usa
o escopo de usuário por padrão. Desde a 0.1.94, `install` sem argumento de origem
nem escopo explícito só seleciona um projeto quando existe um arquivo regular
`uap.toml` na pasta atual exata; não há busca em pastas superiores. Selecione
`--scope project` nos comandos de manutenção de projetos. O suporte a projetos
e a ativação variam conforme o destino.

## Interprete cada nível do resultado {#read-the-result-at-the-right-level}

A instalação ainda pode deixar pendentes uma recarga do cliente, uma solicitação
de confiança, uma ativação manual ou uma etapa de OAuth. Um resultado Prepared
significa que ainda há trabalho a fazer no cliente. Leia o resultado de cada
destino, sem tratar a operação inteira como prova de sucesso na execução.
Consulte a [compatibilidade dos clientes](../reference/client-compatibility)
para conhecer as evidências das observações nativas em versões específicas.

## Se você está escrevendo um pacote {#if-you-are-writing-a-package}

Comece por [Criar plugins](../build/). O material entregue é um diretório com
`plugin.json` na raiz e seus componentes. Quem instala não precisa executar
ferramentas de criação compiladas do código-fonte.

| Comando | Finalidade |
| --- | --- |
| `agentplugins validate ./my-plugin` | Ler e validar um pacote local para instalação. |
| `agentplugins doctor` | Diagnosticar o estado gerenciado pelo instalador. |
| `agentplugins author validate ./my-plugin` | Verificar conformidade e prontidão em uma compilação do código-fonte com criação habilitada. |
| `agentplugins author doctor ./my-plugin` | Inspecionar evidências do projeto e das ferramentas nessa compilação. |

A versão atual do instalador npm e nativo não inclui `agentplugins author`.
