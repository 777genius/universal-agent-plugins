---
title: "Entregar um pacote ao planejador do instalador"
description: "Transmita o pacote-fonte e as evidências estáticas por um limite explícito de instalação."
canonicalId: "page:build:handoff"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Entregar um pacote ao planejador do instalador {#hand-a-package-to-the-installer-planner}

Uma entrega útil consiste no diretório de um pacote padrão acompanhado de uma
descrição clara do que foi verificado. O instalador de quem recebe cria seu
próprio plano para os agentes e o escopo selecionados. Ferramentas de criação
compiladas do código-fonte podem fornecer evidências, mas não são necessárias
na máquina de destino.

## Prepare o pacote e as evidências {#prepare-the-package-and-evidence}

Selecione o diretório com `plugin.json` na raiz, em vez de um repositório superior
ambíguo. Inclua:

- Identidade do pacote, versão e revisão exata da origem ou cópia revisada.
- Raiz relativa do pacote e finalidade das Skills e dos servidores MCP.
- Resultados de validação e inspeção, com a versão da ferramenta ou revisão do código-fonte.
- IDs dos clientes desejados e resultados de compatibilidade por componente.
- Pré-requisitos de execução, necessidades de autenticação e aspectos ainda não avaliados.
- Todo teste explícito de execução, incluindo o servidor, a ferramenta e a plataforma selecionados.

A versão do pacote e a versão da CLI descrevem coisas diferentes. Registre-as
separadamente. Evite credenciais e caminhos absolutos desnecessários em
relatórios compartilhados.

## Conclua as verificações de criação quando disponíveis {#finish-source-authoring-checks-when-available}

Estes comandos opcionais exigem a [compilação do código-fonte com criação habilitada](./#build-from-source),
separada da versão atual do instalador:

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

Essa sequência é estática: não inicia código do pacote nem inspeciona o perfil
de cliente de um usuário real. Registre separadamente as [verificações de execução MCP](./checks)
explícitas, quando realizadas. Uma verificação direta de execução bem-sucedida
ainda não comprova descoberta pelo cliente nativo nem OAuth.

## Peça um plano ao instalador lançado {#ask-the-released-installer-to-plan}

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

O instalador pode buscar metadados de segurança ou seu scanner de versão fixada
mesmo para uma origem local. Sua validação e simulação não equivalem às
verificações de criação sem rede.

O primeiro comando fornece evidências de leitura do pacote pelo instalador.
O segundo seleciona Codex e mostra a instalação prevista sem aplicar alterações
gerenciadas. Para outro cliente, escolha um destino compatível conforme os
resultados de compatibilidade. Para um plano de projeto, adicione explicitamente
`--scope project --project-root /path/to/project`.

Confira a identidade do pacote, a origem, os locais de destino, os resultados de
segurança e os requisitos de ativação pendentes. Resolva divergências em relação
à entrega pretendida pelo autor antes de aplicar. Um plano não é um recibo de
estado instalado.

## Compartilhe uma origem reproduzível {#share-a-reproducible-source}

Para distribuição remota, use uma origem aceita pela CLI de quem recebe: um
pacote assinado do Directory, uma origem exata do GitHub ou uma origem direta
Git ou de arquivo compactado compatível. Consulte [origens de instalação](../use/install)
para sintaxe e disponibilidade. Use uma revisão imutável ou um arquivo versionado
durável quando precisar de restauração exata. Um diretório local exige uma
transferência separada e deliberada.

A CLI de criação do código-fonte não expõe comandos de publicação de pacotes,
exportação de bundles ou envio ao Directory. Hospedar a origem é uma etapa
separada de validá-la. Declarações de dependência em `uap.toml` são um contrato
do gerenciador UAP; o `plugin.json` da raiz continua sendo a autoridade sobre a
identidade do pacote portátil.

## Explique o que ainda falta {#explain-what-remains}

| Evidência fornecida | Trabalho restante |
| --- | --- |
| Conformidade com o padrão | Instruções úteis e comportamento correto das ferramentas. |
| Verificações estáticas de criação | Aprovação da versão e qualificação nos canais públicos. |
| Compatibilidade estática dos clientes | Descoberta e ativação nativas efetivas. |
| Verificação direta de execução MCP, se realizada | Outras ferramentas, clientes, autenticação e ambientes não testados. |
| Simulação do instalador | Aplicar as alterações revisadas e concluir a configuração do cliente. |

Quem recebe pode continuar em [Usar plugins](../use/install) quando quiser
instalar. Mantenha as instruções de confiança e ativação de cada cliente junto
ao pacote.
