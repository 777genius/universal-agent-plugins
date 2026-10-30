---
title: "Entender a raiz do pacote"
description: "Mantenha a identidade portátil e os componentes em uma raiz explícita de pacote padrão."
canonicalId: "page:build:layout"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Entender a raiz do pacote {#understand-the-package-root}

**Criação a partir do código-fonte:** os comandos desta página exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não inclui `agentplugins author`.

O diretório selecionado é a raiz do pacote. As ferramentas de criação leem o
`plugin.json` da raiz e os componentes padrão. Elas não procuram um projeto em
diretórios superiores nem recorrem ao manifesto oculto de um cliente nativo.

## Estruturas mínima e combinada {#minimal-and-combined-layouts}

Um pacote contendo apenas uma Skill:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Um pacote híbrido remoto:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

Um pacote stdio também contém o código-fonte de execução e os manifestos de
dependências. Esses arquivos são necessários para executar depois, mas sua
presença não faz o leitor das ferramentas de criação executar nada.

## Defina uma responsabilidade clara para cada arquivo {#give-each-file-a-clear-responsibility}

| Arquivo ou diretório | Responsabilidade |
| --- | --- |
| `plugin.json` | Identidade e metadados padrão do pacote. |
| `mcp.json` | Configuração de servidores MCP quando o pacote fornece ferramentas. |
| `skills/<name>/SKILL.md` | Identidade, descrição e instruções de uma Skill em uma subpasta direta de `skills/`. |
| `README.md` | Uso pretendido, pré-requisitos, validação e limites restantes. |
| `package.json`, `package-lock.json`, `src/server.mjs` | Arquivos da estrutura inicial Node stdio para trabalho separado de execução. |
| `LICENSE` | Licença explicitamente selecionada, quando fornecida durante init. |

A estrutura inicial define a versão do manifesto como `0.1.0`. Esse valor é um
metadado do novo pacote, não a versão do executável de criação. Preserve essa
distinção ao informar versões aos usuários.

## Identidade e metadados opcionais {#identity-and-optional-metadata}

Use `--name` e `--description` explícitos em instruções de init reproduzíveis.
O contrato público pode derivar um nome de um destino indicado apenas pelo nome
e fornece uma descrição padrão determinística, mas caminhos como
`./review-helper` devem trazer um nome explícito. Estes tutoriais fornecem ambos
para evitar uma identidade acidental.

Em modelos que contêm uma Skill, `--skill-name` assume o nome do pacote quando
omitido. Escolha-o explicitamente se a Skill precisar de outra identidade.
Nomes de Skills nunca são normalizados silenciosamente.

O `--author-name` opcional fornece um autor explícito. Nenhum autor é inferido
do Git ou do ambiente. A geração de licença aceita escolhas explícitas MIT ou
ISC e exige tanto `--copyright-holder` quanto um `--copyright-year` de quatro
dígitos. Não há seleção de licença por padrão.

## Selecione a raiz de forma consistente {#select-the-root-consistently}

```bash
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper
agentplugins author test ./research-helper
```

Os três comandos selecionam a mesma raiz do pacote. Passar
`./research-helper/skills` selecionaria o diretório errado. O caminho opcional
nos comandos de leitura usa apenas o diretório atual por padrão, não um diretório
superior descoberto automaticamente.

Conformidade com o padrão e segurança do sistema de arquivos são questões
separadas. Uma identidade válida segundo o esquema ainda pode ser inadequada
para um caminho no host, e um manifesto legível pode deixar incompletas as
evidências dos componentes necessários. Use o relatório completo.

Continue com [verificações estáticas](./checks) e depois com a [entrega](./handoff).
