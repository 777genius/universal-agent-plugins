---
title: "Início rápido"
description: "Instale plugins disponíveis ou prepare um pacote portátil Agent Plugins 1.0."
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

<a id="quickstart"></a>

# Usar plugins / Criar plugins {#use-plugins-build-plugins}

Instale um pacote nos agentes que você usa ou crie um pacote portátil Agent
Plugins 1.0 com instruções e configuração de ferramentas.

## Usar plugins {#use-plugins}

<a id="optional-first-proof"></a>

Com Node.js 22 ou mais recente:

```bash
npx universal-agent-plugins add context7
```

A CLI pergunta quais agentes compatíveis você quer usar e informa as etapas de
ativação pendentes. Para inspecionar e conferir o plano primeiro:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

O [guia de instalação nativa](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)
também fornece executáveis para macOS, Linux e Windows. O gerenciador nativo
não precisa de Node.js, embora plugins individuais possam precisar de seus
próprios ambientes de execução. Confira `npx universal-agent-plugins version`
ao usar npm: um canal de gerenciador de pacotes ainda pode fornecer uma CLI
mais antiga. A seleção automática abaixo exige 0.1.94; o instalador nativo
fornece a versão verificada. Os comandos explícitos de projeto deste guia também
funcionam com a 0.1.93.

A [CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
mantém o escopo de projeto explícito e a seleção automática de `install`,
introduzida na 0.1.94. Ela também adiciona uma
[API local de gerenciamento](../use/manage#local-management-api) e
[extensões nativas do Pi](../reference/client-compatibility#pi-native-extensions).
Para conferir o plano de adição de um pacote a um projeto:

Para uma entrada revisada e compatível do catálogo, copie a origem Git fixada em um SHA completo e o cliente adequado. Os aliases do Directory v1, como `context7`, continuam limitados ao escopo do usuário. Substitua `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` e `PACKAGE_PATH`; se o pacote estiver na raiz do repositório, omita `//PACKAGE_PATH`.

Modelo: `github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH`. O comando abaixo usa um commit histórico real do GitLab, não uma confirmação de revisão atual no catálogo. Para uma nova instalação, copie a origem verificada atual do catálogo.

```bash
npx universal-agent-plugins add github:777genius/universal-agent-plugins@2ddbb99dd190c1792b79904f9875e6322bccd243//plugins/gitlab --target codex --scope project \
  --project-root /path/to/project --dry-run
```

Escolha um diretório de projeto existente. Os comandos de adição e manutenção
usam o escopo de usuário por padrão, então selecione o escopo de projeto
explicitamente. Na 0.1.94, `install` sem argumento de origem só seleciona o
projeto se a pasta atual exata contiver um arquivo regular `uap.toml`; o escopo
explícito prevalece e não há busca em pastas superiores. Revise o plano antes
de omitir `--dry-run`. Descoberta nativa, confiança, autenticação e chamadas MCP
bem-sucedidas continuam separadas da gravação dos arquivos do pacote.

[Caminho completo de instalação](../use/install) ·
[Compatibilidade dos clientes e limites dos testes](../reference/client-compatibility)

### Restaure um conjunto de dependências salvo {#restore-a-saved-dependency-set}

Confira o que está registrado no ambiente de usuário selecionado:

```bash
npx universal-agent-plugins deps status
```

Para um par `uap.toml`/`uap.lock` salvo, exporte a partir de uma pasta de intercâmbio:

```bash
npx universal-agent-plugins deps export
```

Copie a pasta para outra máquina, instale seus clientes de destino e execute lá:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

Somente o acordo de dependências validado e o lock são transferidos. Perfis,
configurações, credenciais, recibos e dados dos plugins ficam separados.
A instalação ainda precisa das origens exatas ou de pacotes verificados em cache.
Para um projeto, inclua o par no controle de versão e restaure com
`install --locked --scope project` explícito. Consulte [manutenção e restauração](../use/manage)
para detalhes de recuperação e transferência.

## Criar plugins {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

Um pacote padrão começa com `plugin.json` na raiz e pode incluir
`skills/<name>/SKILL.md`, `mcp.json` ou ambos. Você pode escrever esses arquivos
manualmente e validar o pacote com o instalador lançado:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

Esses exemplos pressupõem que `./my-plugin` já contém um pacote completo.
O [guia de criação](../build/) apresenta um pequeno pacote Skill manual e as
ferramentas de geração de estruturas iniciais compiladas do código-fonte.

**Disponibilidade das ferramentas de criação:** a versão atual do instalador
npm e nativo não inclui `agentplugins author`. O código-fonte atual implementa
geração de estruturas iniciais, verificações estáticas, testes explícitos de
execução MCP, ciclos de desenvolvimento, bootstrap de dependências e manutenção
de JSON. Use a [configuração de compilação do código-fonte](../build/#build-from-source)
para esses comandos. As evidências de versões anteriores das ferramentas de
criação permanecem vinculadas às respectivas versões; elas não prometem esses
recursos no instalador atualmente disponível para download.

A validação estática não comprova execução, OAuth nem ativação. O suporte a
transportes MCP também varia por cliente; por exemplo, Codex não aceita MCP SSE
declarado. Mantenha os requisitos do pacote e as verificações pendentes visíveis
no README.

[Especificação Agent Plugins 1.0](https://agent-plugins.org/specification)
