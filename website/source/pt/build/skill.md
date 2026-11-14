---
title: "Criar uma Skill independente"
description: "Crie um pacote padrão com uma Skill de instruções útil."
canonicalId: "page:build:skill"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Criar uma Skill independente {#build-a-standalone-skill}

**Criação a partir do código-fonte:** os comandos desta página exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não inclui `agentplugins author`.

Use este caminho quando o agente precisar de um método reproduzível, uma lista
de verificação ou instruções de escrita e não precisar de conexão com uma
ferramenta MCP. O pacote contém uma Skill e um manifesto na raiz. As etapas de
criação não precisam de servidor, instalação de ambiente de execução nem perfil
de cliente.

## Escolha um destino explícito {#choose-an-explicit-destination}

Os exemplos pressupõem que seu diretório de trabalho seja uma pasta superior
descartável e que `./review-helper` não exista. Escolha uma identidade portátil
para o pacote, em letras minúsculas. O caminho nomeia o destino; `--name` nomeia
o pacote dentro do manifesto.

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Revisar alterações de documentação quanto à clareza e às evidências' \
  --skill-name review-docs
```

Init exige um destino inexistente e não sobrescreve diretórios existentes.
Se a criação informar um resultado gravado seguido de falha posterior, inspecione
o resultado antes de tentar novamente. Não há opção de sobrescrita neste fluxo.

## Revise os arquivos criados {#review-the-created-files}

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

A Skill gerada é uma instrução inicial. Substitua seu corpo por um método concreto
e útil aos seus usuários. Mantenha o nome da Skill alinhado ao diretório.
Por exemplo, o corpo de `skills/review-docs/SKILL.md` poderia ser:

```markdown
# Revisar alterações de documentação

Use esta Skill ao revisar uma alteração de documentação antes da entrega.

1. Identifique o leitor pretendido e a tarefa que a página o ajuda a concluir.
2. Confira se cada comando identifica sua entrada e descreve seu efeito.
3. Separe o comportamento verificado dos pré-requisitos e das afirmações não testadas.
4. Relate instruções pouco claras com uma frase sugerida para substituí-las.

Retorne uma lista curta de achados com a localização nos arquivos. Se não houver
problemas, informe quais páginas foram revisadas e quais verificações ficaram fora da revisão.
```

Mantenha o bloco de metadados gerado acima desse corpo. Seus campos `name` e
`description` identificam a Skill e ajudam o agente a decidir quando usá-la.
As instruções de uma Skill não concedem permissão para executar scripts nem
acessar recursos privados.

## Verifique a raiz exata do pacote {#check-the-exact-package-root}

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

Essas verificações são estáticas. Elas não pedem a um agente que siga as instruções
nem avaliam a qualidade de sua resposta.

Revise conformidade e prontidão separadamente no relatório. Uma Skill bem
formada ainda precisa de revisão humana quanto à utilidade, precisão e adequação
do escopo.

Continue com [mais Skills](./skills), [verificações de destinos](./checks) ou
[entrega ao instalador local](./handoff). Para acrescentar ferramentas às
instruções, escolha o [caminho híbrido](./hybrid) para um pacote novo.
