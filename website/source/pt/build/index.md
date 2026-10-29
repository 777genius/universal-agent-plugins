---
title: "Criar plugins"
description: "Crie um pacote Skill, MCP ou híbrido com as ferramentas de criação do Agent Plugins."
canonicalId: "page:build:index"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Criar plugins {#build-plugins}

Um pacote Agent Plugins 1.0 é um diretório com `plugin.json` na raiz e Skills,
configuração MCP ou ambos. Comece pelo menor componente que resolva a tarefa
dos seus usuários. Uma Skill fornece instruções; o MCP fornece uma conexão
com ferramentas.

## Disponibilidade {#availability}

A versão atual do instalador nativo,
[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94),
pode validar e instalar pacotes padrão. Canais de gerenciadores de pacotes podem
estar desatualizados; confira a versão instalada com `agentplugins version`.
Ela **não** inclui `agentplugins author`. Os comandos de criação deste caminho
descrevem **compilações do código-fonte atual** com criação habilitada.
As evidências históricas da versão de criação 0.1.65 continuam específicas
daquela versão e não descrevem o instalador atual.

Você pode escrever os arquivos manualmente e usar o instalador lançado ou
compilar as ferramentas abaixo para gerar estruturas iniciais, executar
verificações estáticas e realizar um ciclo explícito de execução MCP.
Uma verificação estática aprovada não comprova a ativação no cliente nativo
nem uma conexão funcional com o serviço.

## Monte um pacote pequeno manualmente {#make-a-small-package-by-hand}

Crie esta estrutura:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Coloque a identidade do pacote em `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Revisar alterações de documentação quanto à clareza e às evidências"
}
```

Comece `skills/review-docs/SKILL.md` com:

```markdown
---
name: review-docs
description: Use ao revisar documentação quanto à clareza e às evidências.
---

# Revisar documentação

Identifique o leitor pretendido e a tarefa. Verifique a entrada e o efeito de cada comando.
Separe o comportamento verificado dos pré-requisitos e das afirmações não testadas.
Retorne os achados com a localização nos arquivos e uma correção sugerida.
```

Instale o gerenciador lançado e depois valide o pacote e confira o plano:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Revise o plano e siga [Usar plugins](../use/install) quando quiser aplicá-lo.
Consulte a [estrutura do pacote](./layout) para entender a responsabilidade
de cada arquivo.

## Compile a partir do código-fonte {#build-from-source}

Os comandos abaixo compilam a CLI atual a partir do código-fonte com as
ferramentas de criação habilitadas. Eles exigem Git e a cadeia de ferramentas
Go declarada no repositório (atualmente Go 1.26.10). Registre o
commit selecionado ao compartilhar resultados.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Use um diretório de trabalho separado para os pacotes dos tutoriais. Esse binário
é uma compilação de desenvolvimento. Sua existência não representa uma versão
pública qualificada das ferramentas de criação. Confira o caminho do executável
se o gerenciador instalado ainda informar que o comando `author` é desconhecido.

## Escolha o modelo de pacote {#choose-a-package-to-scaffold}

| Necessidade | Caminho | Parâmetros obrigatórios do modelo |
| --- | --- | --- |
| Instruções que um agente possa seguir | [Skill independente](./skill) | `--template skill` |
| Conexão com um serviço existente | [MCP remoto](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Processo MCP Node local | [MCP stdio](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Instruções junto com ferramentas | [Híbrido](./hybrid) | `--template hybrid --mcp-template <choice>` e os parâmetros desse modelo MCP |

Um pacote Skill independente também tem `plugin.json` na raiz. Instalar Skills
avulsas de outro repositório é uma [operação separada do instalador](../use/install#install-agent-skills).

## Siga o ciclo de criação {#follow-the-authoring-loop}

1. Crie um pacote em um destino que ainda não exista e edite os arquivos gerados.
2. Execute [validate, inspect e test estático](./checks) na raiz exata do pacote.
3. Verifique os clientes desejados com `compat` e inspecione as evidências de `doctor`.
4. Para MCP, avalie a execução separadamente, com execução explícita pela compilação do código-fonte.
5. Compartilhe o pacote e suas evidências pela [entrega ao instalador](./handoff).

O código-fonte atual também oferece `bootstrap` baseado em lockfile, `dev`
contínuo, `normalize` de JSON e `import native` explícito para MCP do Claude.
Consulte [verificações](./checks) e [MCP stdio](./mcp-stdio). Projeções para
clientes, exportação de bundles e publicação de pacotes não fazem parte do
conjunto atual de comandos de criação. Hooks específicos de clientes permanecem
separados dos componentes portáteis Skill e MCP.
