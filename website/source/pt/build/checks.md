---
title: "Verificar um pacote e suas evidências"
description: "Diferencie verificações estáticas, execução MCP explícita pela compilação do código-fonte e evidências de ativação."
canonicalId: "page:build:checks"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Verificar um pacote e suas evidências {#check-a-package-and-its-evidence}

**Criação a partir do código-fonte:** estes comandos `agentplugins author`
exigem a [compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não os inclui. Execute os exemplos com caminhos
relativos a partir da pasta superior do pacote e use sempre a mesma raiz exata.

## Execute o ciclo de criação sem rede {#run-the-offline-authoring-loop}

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

A validação verifica a configuração padrão capturada e a prontidão de criação.
A inspeção mostra os componentes e os requisitos não resolvidos. `test` sem
opções de execução verifica estaticamente configuração, higiene do pacote,
Skills e MCP. Esses comandos não iniciam servidores nem executam as instruções
de uma Skill.

## Interprete as camadas de evidência {#read-the-evidence-layers}

| Camada | O que comprova | O que exige outra verificação |
| --- | --- | --- |
| Conformidade | Os dados padrão capturados atendem aos esquemas e ao perfil incorporados. | Comportamento útil em um agente específico. |
| Segurança do host | A entrada atende às verificações limitadas do sistema de arquivos e do host. | Autorização do usuário para instalar ou executar. |
| Prontidão | As evidências disponíveis atendem à política verificada. | Aprovação de uma versão pública ou aceitação pela plataforma. |
| Compatibilidade | Suporte estático aos destinos e componentes selecionados. | Descoberta e ativação efetivas em um cliente real. |
| Ferramentas | Metadados limitados do projeto, de arquivos nativos e de executáveis. | Resolução de dependências e inicialização do servidor. |
| Execução | Uma verificação MCP executada explicitamente informa as etapas observadas. | Outras ferramentas, clientes, fluxos OAuth ou ambientes não testados. |

Leia conformidade e prontidão geral separadamente. Um manifesto válido ainda
pode ter evidências incompletas de componentes ou do host. Mantenha “não avaliado”
visível em vez de convertê-lo em aprovação.

## Selecione clientes explicitamente {#select-clients-explicitly}

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

`compat` exige IDs de destino distintos, separados por vírgulas; ele não aceita
`all`. Essas verificações estáticas não exigem clientes instalados. Doctor
inspeciona arquivos nativos capturados e metadados de executáveis sem processos
ou rede. Capabilities informa os esquemas, os metadados de clientes e os comandos
implementados neste mecanismo; ele não certifica um pacote nem estabelece a
qualificação de uma versão pública.

## Avalie a execução MCP explicitamente {#evaluate-mcp-runtime-explicitly}

A execução stdio local exige atualmente Linux, bubblewrap do sistema e Node
nos diretórios de execução do sistema. As verificações prévias de contenção de
processos (incluindo cgroup v2 delegada) e dos namespaces do bubblewrap precisam
passar antes do lançamento. Consulte os [pré-requisitos de stdio](./mcp-stdio).
A execução Streamable HTTP não usa essa sandbox de processos locais e exige
sua própria autorização explícita de rede.

Para um pacote criado pelo [tutorial stdio](./mcp-stdio), instale separadamente
as dependências reconhecidas e fixadas e selecione o nome exato do servidor
em `mcp.json`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

Esse comando do código-fonte executa o processo MCP selecionado e verifica a
inicialização e a descoberta de ferramentas. Para chamar uma ferramenta, forneça
`--tool` e `--fixture`. A fixture é um objeto JSON relativo ao pacote que contém
os argumentos dessa ferramenta. Para `hello` do modelo, crie
`fixtures/hello.json` contendo `{}`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

Para Streamable HTTP, permita explicitamente o acesso à rede:

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

Use a identidade real do servidor do seu pacote. O prazo padrão é de dez segundos
e pode ser definido por `--deadline` como uma duração positiva de até um minuto.
Inspecione separadamente as evidências de inicialização, ferramentas, chamada
opcional de ferramenta e limpeza. Uma aprovação aqui não é execução por um
agente instalado e não comprova OAuth, outras ferramentas nem uma implantação
pública.

## Itere com o ciclo de desenvolvimento do código-fonte {#iterate-with-the-source-development-loop}

```bash
agentplugins author dev ./local-tools --server local-tools
```

`dev` executa ciclos MCP limitados à medida que o pacote muda; interrompa-o com
Ctrl+C. Use `--once` para um único ciclo e ao solicitar saída JSON. Servidores
remotos exigem `--allow-network`; chamadas de ferramentas continuam exigindo o
par explícito de ferramenta e fixture. O comando não instala em um cliente nativo.

## Capture um relatório para revisão {#capture-a-reviewable-report}

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

Use `--format human` para saída legível por pessoas e `--no-color` quando
necessário. `--include-root` revela a raiz selecionada; evite caminhos privados
desnecessários em diagnósticos compartilhados. `--release-policy` acrescenta
verificações limitadas de higiene de lançamento ao pacote e não aprova a
publicação. Os subcomandos de Skills têm menos opções; confira a ajuda deles.

## Faça a manutenção de JSON de forma deliberada {#maintain-json-deliberately}

O código-fonte atual pode planejar a normalização determinística de um arquivo
padrão selecionado e gravá-lo apenas com `--write` explícito:

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

Selecione `mcp.json` se esse for o documento a normalizar. A importação nativa lê
um único arquivo MCP do Claude explicitamente selecionado, em JSON estrito;
ela não busca perfis. O importador atual só aceita entradas stdio seguras com
`command` e `args` opcional; entradas HTTP/URL, `env` e campos extras do servidor
são ignorados, mesmo quando o cliente de origem os aceita. Planeje um destino
absoluto novo e inexistente antes de aplicar:

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Pacote MCP do Claude importado'
```

Revise os servidores seguros e ignorados e os campos incompatíveis. Adicione
`--write` para gravar o pacote validado nesse destino inexistente. Servidores
inseguros ou com credenciais são ignorados; inspecione o resultado em vez de
presumir que todos os servidores originais foram transferidos. Nenhuma outra
importação de cliente nativo é exposta aqui.

## Respeite os limites de cada comando {#choose-the-right-command-boundary}

O `agentplugins validate ./review-helper` do instalador lê um pacote para
instalação; `agentplugins doctor` diagnostica o estado gerenciado. Eles são
separados das operações de criação compiladas do código-fonte. Opções de escopo
e segurança do instalador não se tornam opções de criação. Init cria arquivos
reais; normalização e importação exigem `--write` para aplicar seus planos.
Continue com a [entrega do pacote](./handoff).
