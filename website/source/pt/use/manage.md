---
title: "Manter plugins instalados"
description: "Diferencie diagnóstico do estado instalado, atualizações, reparo e verificações de criação."
canonicalId: "page:use:manage"
section: "use"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Manter plugins instalados {#maintain-installed-plugins}

Use o instalador para inspecionar e alterar o estado das instalações gerenciadas.
Ao investigar um problema, registre no relatório a origem do pacote, os clientes
selecionados, o escopo e a identidade da instalação. Os exemplos abaixo usam o
escopo de usuário, exceto quando selecionam um projeto explicitamente.

## Inspecione a situação primeiro {#inspect-the-situation-first}

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` informa as instalações, `doctor` diagnostica o estado gerenciado e
`outdated` encontra possíveis atualizações sem aplicá-las. Nenhum desses comandos
comprova que um serviço remoto está funcionando ou que um cliente concluiu a
autenticação.

| Sintoma | Próximo passo |
| --- | --- |
| O pacote está ausente | Volte a [encontrar e instalar](./install). |
| Arquivos gerenciados estão ausentes ou divergentes | Revise um plano de reparo para esse pacote e escopo. |
| Uma origem acompanhada tem uma versão mais recente | Revise um plano de atualização. |
| O cliente precisa de confiança, ativação ou acesso à conta | Siga as instruções do cliente indicadas para o destino. |
| O pacote local falha na validação | Entregue ao autor o relatório do pacote e as [orientações de verificação](../build/checks). |

## API local de gerenciamento (CLI 0.1.95) {#local-management-api}

A [CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95)
adiciona `agentplugins manage serve` para aplicativos que incorporam o instalador
e `agentplugins manage discover` para descobrir clientes e perfis locais sem
alterá-los. A descoberta não executa clientes, não consulta suas versões,
não busca fontes nem altera arquivos.

`manage serve` é um processo controlado pelo aplicativo integrador, com um
contexto capturado na inicialização e uma API privada autenticada no loopback
IPv4. O aplicativo fornece JSON pelo stdin e mantém privadas a URL inicial
e a credencial bearer. Confira a ajuda dos comandos antes da integração:

```bash
agentplugins version
agentplugins manage serve --help
agentplugins manage discover --help
```

Consulte o [contrato do protocolo de gerenciamento](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md)
para conhecer as regras de inicialização e ciclo de vida.
O [TypeScript Management SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md)
tem versionamento independente como pacote `universal-agent-plugins-management-sdk`.
Consulte o README do SDK para saber quais versões do motor `agentplugins` são
compatíveis e como instalar o pacote. Escolha uma versão publicada no npm ou, para
trabalhar com alterações ainda não lançadas, compile e teste um pacote a partir
de um commit específico do código-fonte. Fixe as versões tanto do SDK quanto do motor.

## Atualize de forma deliberada {#update-deliberately}

```bash
agentplugins update context7 --target codex,cursor --dry-run
```

Revise a origem, a versão e os locais selecionados. Omita `--dry-run` para aplicar:

```bash
agentplugins update context7 --target codex,cursor
```

`update --all` abrange todos os candidatos a atualização gerenciados. Use-o quando
pretender atualizar esse conjunto inteiro. Uma origem com commit exato permanece
imutável; uma atualização não a faz acompanhar uma branch. Faixas de versão
salvas e referências móveis só avançam pela operação de atualização explícita
apropriada.

## Repare o estado registrado {#repair-recorded-state}

```bash
agentplugins repair context7 --target codex,cursor --dry-run
agentplugins repair context7 --target codex,cursor
```

O primeiro comando mostra a prévia; o segundo aplica. O reparo reutiliza a origem
registrada para restaurar o conteúdo gerenciado. Ele não seleciona uma nova revisão
remota nem sobrescreve silenciosamente entradas nativas alteradas externamente.
Revise conflitos e instruções de recuperação pendente antes de continuar.

## Remova a instalação selecionada {#remove-the-selected-installation}

```bash
agentplugins remove context7 --target codex,cursor --dry-run
agentplugins remove context7 --target codex,cursor
```

Revise o primeiro plano antes de executar o segundo comando. A remoção usa os
registros de propriedade mantidos pela CLI. Ela não revoga uma conta de serviço,
remove ambientes de execução não relacionados nem apaga o diretório-fonte de um
autor. A remoção de um projeto preserva os dados dos plugins e o armazenamento
compartilhado de conteúdo verificado por padrão.

## Use um ambiente de dependências salvo {#use-a-saved-dependency-environment}

`uap.toml` registra os pacotes e destinos desejados; `uap.lock` registra suas
origens resolvidas exatas e seus resumos criptográficos. Inspecione o ambiente
selecionado com:

```bash
agentplugins deps status
agentplugins deps tree
```

Para um ambiente novo, use `agentplugins deps init`, edite o manifesto de forma
deliberada e execute `agentplugins deps lock` para resolver as dependências.
Depois, use `agentplugins install --locked --scope user` para aplicar esse lock.
Revise as confirmações e use `--dry-run` para conferir uma prévia.
O [guia de dependências](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md)
explica a sintaxe do manifesto, dependências obrigatórias dos pacotes, faixas de
versão e regras de atualização.

Operações remotas `add` elegíveis salvam por padrão a intenção portátil de
dependências na CLI 0.1.93. Caminhos locais explícitos, origens de descoberta,
preparação exclusiva para ChatGPT e `--no-save` usam o fluxo imperativo.
Não presuma que uma instalação imperativa antiga consta do acordo de dependências:
confira `deps status` e os arquivos reais antes de transferi-los.

## Restaure em outra máquina {#restore-on-another-machine}

Transfira um ambiente de usuário salvo por uma pasta de intercâmbio. Na máquina
de origem, execute a partir dessa pasta:

```bash
agentplugins deps export
```

Copie a pasta para o destino, instale os clientes de destino e execute lá:

```bash
agentplugins deps import
agentplugins install --locked --scope user
```

Ou combine importação e restauração:

```bash
agentplugins deps import --install
```

O argumento opcional de diretório seleciona outra pasta de intercâmbio.
A exportação lê o ambiente de usuário e a importação o grava; `AGENTPLUGINS_HOME`
pode selecionar um ambiente específico. A pasta transferida não substitui, por
si só, o perfil de um cliente. O `--scope user` explícito mantém a restauração
no ambiente do usuário.

Somente `uap.toml` e `uap.lock`, inalterados e validados estritamente, são
transferidos. A cópia efetiva mostra uma prévia e exige confirmação ou `--yes`;
`--dry-run` apenas mostra a prévia. A validação da importação funciona sem rede,
mas a instalação precisa das origens registradas ou de pacotes verificados em
cache. Uma restauração sem cache busca as versões fixadas exatas e verifica seus
resumos criptográficos; ela não escolhe versões mais novas.

Perfis dos clientes, configurações, credenciais, recibos e `PLUGIN_DATA` não
fazem parte da transferência. Configure a autenticação na nova máquina e faça
backup dos dados separadamente. Se a instalação falhar depois que a importação
salvou o par, corrija a causa informada e repita `install --locked --scope user`.
Execute primeiro `deps recover --scope user` se houver recuperação pendente.
Não recrie as versões fixadas transferidas com `deps lock`, pois um seletor móvel
pode resolver para outra versão.

<a id="restore-project-dependencies"></a>

## Restaure e mantenha um projeto {#restore-and-maintain-a-project}

A instalação automática de projetos está disponível na
[CLI 0.1.94 e posteriores](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94).
Inclua `uap.toml` e `uap.lock` do projeto no controle de versão. Em outra cópia
existente do repositório, instale os clientes necessários e execute a partir do
diretório exato do projeto:

```bash
agentplugins install
```

Um lock existente é validado estritamente. `--locked` também exige que ele exista,
o que é útil em CI; sem lock, a primeira instalação pode criá-lo. Ao executar a
partir de outro diretório, selecione o projeto existente explicitamente:

```bash
agentplugins install --locked --scope project --project-root /path/to/project
```

Selecione o mesmo escopo e a mesma raiz para manutenção:

```bash
agentplugins list --scope project --project-root /path/to/project
agentplugins deps status --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Use a identidade da instalação informada por `list` se o nome for ambíguo.
Comandos de manutenção usam o escopo de usuário por padrão, então selecione o
escopo de projeto explicitamente. Um `install` sem argumento de origem segue a
regra de seleção da 0.1.94 descrita abaixo. O `--scope project` explícito sem
`--project-root` usa somente o diretório de execução.

Mantenha localmente os recibos de `.uap/`, os diários de recuperação e os dados
preservados. Restaure componentes nativos a partir do acordo portátil de
dependências e siga as instruções de confiança ou ativação do cliente. As origens
e o acesso a origens privadas precisam continuar disponíveis. Consulte
[instalação em projetos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)
e [limites por cliente](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md).

## Seleção padrão de instalação na CLI 0.1.94 {#default-install-selection-in-cli-0194}

Confira `agentplugins version` primeiro. Esse comportamento exige a versão
0.1.94; se o canal do gerenciador de pacotes estiver desatualizado, use o instalador
nativo ou o Homebrew. Os comandos explícitos de escopo de projeto acima continuam
compatíveis com a 0.1.93.

**Lançado na [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94):**
`agentplugins install` sem argumento de origem nem escopo explícito seleciona o
escopo de projeto quando existe um arquivo regular `uap.toml` no diretório atual
exato. Ele não busca em pastas superiores. Um diretório ou link simbólico com
esse nome é recusado durante a seleção automática; escolha o escopo explicitamente
para resolver a ambiguidade. Sem esse manifesto local, a instalação seleciona o
ambiente do usuário. `--scope user` ou `--scope project` explícitos sempre
prevalecem. Os demais comandos continuam usando o escopo de usuário por padrão.

Um `uap.lock` existente é validado estritamente antes das alterações, mesmo sem
`--locked`. Uma divergência interrompe a operação; use `update` para avançar as
versões fixadas, em vez de esperar que a instalação as atualize. Somente a ausência
do lock permite resolver dependências e criar o lock na primeira instalação.
`--locked` também exige um lock existente. Para automação previsível e restauração
de projetos, os exemplos acima com escopo explícito e `--locked` continuam adequados.

## Relate um problema com clareza {#report-a-problem-clearly}

Registre a versão da CLI, a operação, o escopo, a origem do pacote, os clientes
de destino e a instrução de ativação pendente. Informe se a falha ocorreu na
obtenção da origem, no planejamento, na aplicação dos arquivos, na recuperação,
na descoberta pelo cliente, na autenticação ou no uso em execução. Remova
credenciais antes de compartilhar o diagnóstico. O `doctor ./my-plugin` de
criação inspeciona um pacote-fonte; o `doctor` do instalador diagnostica o estado
das instalações gerenciadas.
