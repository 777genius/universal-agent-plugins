---
title: "Criar um pacote híbrido"
description: "Combine uma Skill e uma configuração MCP sob uma única identidade de pacote."
canonicalId: "page:build:hybrid"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Criar um pacote híbrido {#build-a-hybrid-package}

**Criação a partir do código-fonte:** os comandos desta página exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não inclui `agentplugins author`.

Um híbrido é útil quando as instruções explicam como e quando usar ferramentas.
Os dois componentes ficam sob um único manifesto na raiz. Um cliente pode
oferecer suporte diferente a cada um; mantenha os resultados de compatibilidade
por componente visíveis na entrega.

## Escolha a parte MCP explicitamente {#choose-the-mcp-side-explicitly}

A opção de híbrido é `--mcp-template`. Escolha `mcp-remote` ou `mcp-stdio` e forneça
os parâmetros obrigatórios desse modelo. Um híbrido não infere o transporte da
URL, dos arquivos locais nem dos ambientes de execução instalados.

Exemplo de híbrido remoto:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Orientar pesquisas usando o serviço MCP da equipe' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

A URL é um marcador de exemplo, não um serviço garantido. Substitua-a pelo seu
endpoint real. Leia os [requisitos de MCP remoto](./mcp-remote).

## Ou escolha um processo local {#or-choose-a-local-process}

Para outro destino inexistente, a variante stdio é:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Orientar pesquisas usando ferramentas Node locais' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

Não adicione `--url` à variante stdio. Revise os
[pré-requisitos de stdio e limites de execução](./mcp-stdio).

## Conecte as instruções às ferramentas {#connect-instructions-to-tools}

Um híbrido remoto tem esta estrutura principal:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

README e `.gitignore` também são criados. Um híbrido stdio acrescenta o código
Node e os arquivos de pacote descritos no caminho stdio. Revise `SKILL.md` para
que indique quando usar as ferramentas, quais entradas são necessárias e como
informar falta de acesso. Não instrua o agente a tratar o texto da Skill como
uma concessão de autorização.

## Verifique os dois componentes {#check-both-components}

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

Para a variante local, selecione `./local-research`. Preserve os resultados por
componente: um cliente que aceita a Skill não necessariamente aceita o transporte
MCP nem ativa o serviço. Um pacote combinado não elimina esse limite.

Continue com [mais Skills](./skills) ou com a [entrega](./handoff).
