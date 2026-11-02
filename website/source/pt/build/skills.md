---
title: "Adicionar mais Skills"
description: "Adicione outra Skill em uma subpasta direta de skills/ a um pacote padrão existente."
canonicalId: "page:build:skills"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Adicionar mais Skills {#add-extra-skills}

**Criação a partir do código-fonte:** os comandos desta página exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não inclui `agentplugins author`.

Use a operação `skills init` para adicionar uma Skill a um pacote padrão
existente. Ela funciona em pacotes Skill, MCP e híbridos. Selecione a raiz do
pacote que contém `plugin.json`, não o diretório `skills/` nem um arquivo
`SKILL.md` individual.

## Identifique a nova Skill e o pacote {#name-the-new-skill-and-package}

Suponha que `./review-helper` seja o pacote do [caminho de Skill independente](./skill)
e que `skills/release-notes/` ainda não exista.

Com a compilação do código-fonte com criação habilitada:

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use ao redigir notas de versão a partir de alterações revisadas'
```

O nome posicional da Skill é obrigatório, assim como uma descrição explícita
de 1–1024 caracteres. Os nomes não são normalizados. Use um nome portátil exato
em letras minúsculas, com hífens, e escolha um destino novo em vez de esperar
que um existente seja sobrescrito.

## Revise o componente adicional {#review-the-additional-component}

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

Edite `skills/release-notes/SKILL.md` para explicar quando usar a Skill, seu
método e a saída esperada. Mantenha o nome e a descrição precisos. Por exemplo,
instrua a separar alterações visíveis aos usuários das mudanças de manutenção
e a citar a origem revisada para cada afirmação sobre a versão.

Scripts, referências e recursos podem complementar as instruções, mas sua
presença não prova execução nem concede permissão para executá-los. O campo
`allowed-tools` não é uma evidência independente de autorização. Revise os
materiais de apoio com o mesmo cuidado dedicado às instruções.

## Valide o pacote e suas Skills {#validate-the-package-and-immediate-skills}

```bash
agentplugins author skills validate ./review-helper
```

O comando informa a prontidão do pacote inteiro e os resultados individuais das
Skills que ficam nas subpastas diretas de `skills/`. Por isso, um erro em outra parte do pacote
pode afetar o resultado geral, mesmo quando a nova Skill não apresenta problemas.

Omitir o caminho do pacote usa somente o diretório atual, sem busca em ancestrais.
Raízes explícitas tornam scripts e diagnósticos mais claros. Executar dentro
de `skills/release-notes/` não substitui a seleção da raiz do pacote.

## Trate uma falha sem sobrescrever o trabalho {#handle-a-failure-without-overwriting-work}

Leia `committed` e as orientações que acompanham o erro se a criação falhar.
Uma Skill pode ter sido gravada antes que as verificações do pacote falhem ou
fiquem incompletas. Inspecione os arquivos reais e o relatório antes de tentar
novamente; um segundo init em um destino existente não o sobrescreve.

Corrija a entrada ou o problema de pacote informado e repita a validação.
Se o nome estiver errado, revise explicitamente a alteração desejada no código-fonte,
em vez de adicionar uma opção de força. Instalar e manter Skills externas
pertence ao caminho separado do instalador; `author skills` edita e valida
componentes no seu pacote-fonte.

Continue com as [verificações estáticas do pacote](./checks) e a [entrega](./handoff).
