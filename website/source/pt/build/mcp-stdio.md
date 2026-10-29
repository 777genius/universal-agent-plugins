---
title: "Criar um pacote MCP stdio"
description: "Prepare um servidor MCP Node local, mantendo a configuração do ambiente e a execução separadas."
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Criar um pacote MCP stdio {#build-a-stdio-mcp-package}

**Criação a partir do código-fonte:** os comandos abaixo exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
Eles não estão incluídos na versão atual do instalador.

Escolha stdio quando seu pacote fornecer um processo local que se comunica pela
entrada e saída padrão. A estrutura inicial aceita Node.js 22 ou mais recente
e usa o SDK MCP oficial. Criá-la não instala dependências nem inicia um servidor.

## Crie a estrutura inicial {#create-the-scaffold}

Escolha um destino inexistente dentro de um diretório superior existente:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Fornecer ferramentas locais por um servidor MCP Node' \
  --runtime node
```

`node` é o ambiente de execução aceito por esse modelo portátil. Uma opção de
ambiente Python ou Go não faz parte do contrato atual desse comando.

## Revise os arquivos {#review-the-files}

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` declara o servidor `local-tools`, com o comando `node` e o argumento
`${PLUGIN_ROOT}/src/server.mjs`. O token do pacote é resolvido dentro do pacote
selecionado; ele não significa o diretório atual do seu shell. O código-fonte
começa com uma pequena ferramenta `hello`. Adapte-a à sua tarefa e envie os logs
da aplicação para stderr, deixando stdout disponível para o protocolo MCP.

Revise o manifesto de dependências e o lockfile. Init os grava, mas não executa
npm nem comprova que o host consegue importar o SDK.

## Reúna evidências estáticas {#gather-static-evidence}

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

Essas verificações são estáticas. Doctor inspeciona metadados limitados do
projeto e dos executáveis; ele não comprova dependências instaladas, um handshake
bem-sucedido ou uma ferramenta funcional. Interprete as [camadas de evidência](./checks)
separadamente.

## Instale as dependências do modelo reconhecido {#bootstrap-the-recognized-template}

Para a estrutura Node gerada com seu lockfile correspondente, o código-fonte
atual pode mostrar a prévia e instalar dependências:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

Aplicar bootstrap exige atualmente Linux e suporte do sistema de arquivos
vinculado a descritores; outras plataformas informam `bootstrap_platform_unsupported`.
A simulação ainda pode inspecionar uma estrutura reconhecida sem instalar
dependências.

Revise a prévia antes de aplicar. Bootstrap usa instalação npm pelo lockfile,
com scripts de ciclo de vida desabilitados. Ele aceita a estrutura gerada
reconhecida, arquivos de pacote correspondentes e ausência de `node_modules`;
não é um substituto geral de gerenciador de pacotes. Arquivos de dependências
modificados, lockfiles concorrentes ou saídas existentes podem causar recusa.
Atenda ao requisito de estrutura informado, em vez de adicionar uma opção de
sobrescrita. A obtenção de dependências pode usar a rede.

## Execute uma verificação MCP explícita {#run-one-explicit-mcp-check}

A execução stdio local exige atualmente Linux, `bwrap` (bubblewrap) do sistema,
contenção de processos funcional (incluindo cgroup v2 delegada), suporte a
namespaces e Node resolvido dentro dos diretórios de execução do sistema
(`/usr`, `/bin`, `/lib` ou `/lib64`). O comando exige aprovação nas verificações
prévias de contenção de processos e bubblewrap antes de iniciar o código do
pacote e recusa hosts onde não consegue estabelecê-las. Uma instalação Node
no diretório de um gerenciador de versões do usuário não atende a esse contrato
de execução. As verificações estáticas continuam disponíveis separadamente.

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

Isso executa o processo local selecionado e verifica a inicialização MCP e a
descoberta de ferramentas. Chamar `hello` exige uma fixture explícita de argumentos
JSON; consulte o [exemplo de verificação de execução](./checks#evaluate-mcp-runtime-explicitly).
Para iterar continuamente:

```bash
agentplugins author dev ./local-tools --server local-tools
```

Guarde a identidade exata do mecanismo, do pacote e da plataforma no resultado
da execução. Uma verificação direta de criação não comprova que o mesmo servidor
funciona por todos os agentes nativos nem que um cliente concedeu confiança ou
concluiu a autenticação.

## Prepare a entrega {#prepare-the-handoff}

Explique as ferramentas, o pré-requisito de Node.js, os arquivos de dependências
e as evidências de execução no README. Informe o servidor selecionado e as
ferramentas ou os ambientes não testados. Depois, compartilhe o pacote pela
[entrega ao instalador](./handoff). Acrescente instruções com [mais Skills](./skills)
quando os usuários precisarem de orientações junto às ferramentas.
