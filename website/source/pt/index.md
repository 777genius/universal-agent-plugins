---
title: "Documentação do Agent Plugins"
description: "Documentação pública do Agent Plugins."
canonicalId: "page:home"
section: "home"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">DOCUMENTAÇÃO PÚBLICA</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">Instale Skills e ferramentas MCP nos agentes que você usa ou crie seu próprio pacote portátil.</p>
</div>

## Escolha seu caminho {#choose-your-journey}

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>Usar plugins</h2>
    <p>Inspecione um pacote, confira as alterações previstas e instale-o para sua conta ou para um projeto.</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>Criar plugins</h2>
    <p>Escreva um pacote padrão com plugin.json, Skills e configuração MCP. Explore as ferramentas de criação disponíveis em compilações do código-fonte.</p>
  </a>
</div>

## Instale seu primeiro plugin {#install-your-first-plugin}

Com Node.js 22 ou mais recente:

```bash
npx universal-agent-plugins add context7
```

A CLI pergunta quais agentes compatíveis você quer usar. Confira o plano e
conclua as etapas de ativação ou de acesso à conta indicadas no resultado.
Prefere um executável nativo? Siga o [guia de instalação nativa](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
para macOS, Linux ou Windows; o gerenciador nativo em si não exige Node.js.

## O que está disponível {#what-is-available}

A [CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
é a versão atual do instalador nativo, publicada em 29 de outubro de 2026.
Ela adiciona uma API local de gerenciamento e extensões nativas do Pi no escopo
de usuário. Consulte [gerenciamento local](./use/manage#local-management-api) e
[compatibilidade com o Pi](./reference/client-compatibility#pi-native-extensions).

O instalador oferece escopo de projeto explícito e restauração com versões
fixadas. A maioria dos comandos usa o ambiente do usuário por padrão. Um
`install` sem argumento de origem seleciona o projeto atual quando essa pasta
exata contém um arquivo regular `uap.toml`; o escopo explícito prevalece.
Selecione o escopo de projeto nos comandos de manutenção. Confira
`agentplugins version`: a seleção automática exige a versão 0.1.94. Canais de
gerenciadores de pacotes podem demorar para receber a atualização; o instalador
nativo fornece a versão atual verificada. As opções explícitas de projeto estão
disponíveis desde a 0.1.93. Consulte [instalação](./use/install) e
[manutenção](./use/manage).

O instalador atual não inclui `agentplugins author`. O [guia de criação](./build/)
explica como escrever pacotes manualmente e apresenta os comandos de criação
implementados no código-fonte atual. As evidências de versões anteriores das
ferramentas de criação mantêm sua versão original e não descrevem o instalador
atualmente disponível para download.

## Entenda o resultado {#understand-the-result}

Validação do pacote, instalação, descoberta pelo cliente, execução de ferramentas
MCP e autenticação no serviço são verificações distintas. Uma instalação de
arquivos bem-sucedida ainda pode exigir confiança no cliente, recarga ou uma
nova sessão. Os testes nativos abrangem versões específicas de clientes e
plataformas; eles não certificam todos os plugins nem todos os agentes.

- [Início rápido](./guide/quickstart)
- [Estrutura do pacote](./build/layout)
- [Verificações e evidências](./build/checks)
- [Compatibilidade dos clientes e limites dos testes](./reference/client-compatibility)
- [Especificação Agent Plugins 1.0](https://agent-plugins.org/specification)
