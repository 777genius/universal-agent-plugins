---
title: "Encontrar e instalar um plugin"
description: "Inspecione um pacote existente, confira as alterações por destino e entenda a ativação."
canonicalId: "page:use:install"
section: "use"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Encontrar e instalar um plugin {#find-and-install-a-plugin}

Escolha um pacote cuja origem e finalidade você compreenda. Os comandos abaixo
usam o instalador lançado; consulte [instalação da CLI](./) se `agentplugins`
ainda não estiver disponível. Use `--dry-run` para conferir um plano antes de
aplicar alterações.

## Descubra o pacote {#discover-the-package}

```bash
agentplugins search docs
agentplugins info context7
```

Um nome curto como `context7` vem do Universal Agent Plugins Registry assinado.
A busca e as informações do pacote ajudam a escolher um candidato; elas não o
executam nem comprovam que seu serviço está disponível. Antes de instalar,
revise as ferramentas expostas, as Skills, os requisitos de execução e o acesso
a contas.

## Confira uma seleção explícita {#preview-one-explicit-selection}

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

Revise a origem, os destinos selecionados, os locais de instalação, os resultados
de segurança e o trabalho de ativação pendente. Esse comando exibe um plano sem
aplicar a instalação. Ele não testa um servidor MCP.

## Instale após revisar o plano {#install-after-reviewing-the-plan}

```bash
agentplugins add context7 --target codex,cursor
```

Para seleção interativa, omita `--target`. A CLI pode selecionar automaticamente
um único agente detectado ou oferecer um seletor quando houver vários. Use
destinos explícitos para uma seleção reproduzível e em automações.

O instalador faz verificações prévias nos clientes selecionados e protege
arquivos que não lhe pertencem. Se uma operação informar falha na reversão ou
recuperação pendente, guarde o relatório e siga as orientações de recuperação
antes de tentar novamente.

## Conclua a ativação {#finish-activation}

Siga as etapas específicas do cliente indicadas no resultado: ainda pode ser
necessário recarregar, iniciar uma nova sessão, confiar no espaço de trabalho,
entrar na conta ou ativar explicitamente. Arquivos preparados e registro nativo
bem-sucedido não comprovam OAuth nem chamadas de ferramentas bem-sucedidas.
Use o [diagnóstico do estado gerenciado](./manage) se a instalação estiver
ausente ou divergente; verifique a autenticação no cliente quando o pacote
estiver presente, mas indisponível.

## Use um pacote local {#use-a-local-package}

Selecione o diretório que contém `plugin.json` na raiz:

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

A validação e o planejamento do instalador continuam aplicando a política de
segurança. Eles podem buscar metadados de segurança ou um scanner de versão
fixada mesmo para um pacote local; `--dry-run` evita alterações na instalação
gerenciada, mas não promete uma operação sem rede.

Após revisar o plano, omita `--dry-run` para instalar. Um diretório local continua
sendo uma instalação local; ele não é uma dependência portátil que outra máquina
possa buscar. Peça ao autor as [evidências para entrega do pacote](../build/handoff).

## Selecione uma origem exata do GitHub {#select-an-exact-github-source}

A sintaxe estabelecida para origens imutáveis do GitHub exige um SHA de commit
completo de 40 caracteres. Substitua este exemplo por um repositório real, uma
revisão analisada e o caminho do pacote:

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

A forma `owner/repository@...` rejeita branches, tags e SHAs abreviados. Sem um
caminho, o instalador seleciona um pacote válido na raiz ou o único candidato
válido aninhado. Vários candidatos exigem caminho explícito; mais de 16 pacotes
possíveis exigem esse caminho antes da busca dos candidatos. O reparo reutiliza
a origem registrada. Mover uma instalação imutável para outra origem é uma
operação separada de seleção de origem.

A CLI 0.1.93 também aceita Git HTTPS direto, SSH, URLs de navegação de provedores
compatíveis e arquivos compactados completos por HTTPS ou locais. Por exemplo,
substitua estes marcadores por origens de pacotes revisadas:

```text
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target codex --dry-run
agentplugins add 'ssh://git@git.example.com/team/tools.git#main//plugins/demo' --target codex --dry-run
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target codex --dry-run
```

O Git usa as credenciais nativas existentes e a confiança no host SSH. Arquivos
compactados HTTPS não usam credenciais e devem conter um pacote padrão completo.
O arquivo de lock fixa revisões exatas do Git ou os bytes originais do arquivo
compactado, além dos resumos criptográficos do pacote. Uma origem móvel pode
avançar por meio de atualização explícita; a restauração pelo lock mantém a
versão fixada. Essas origens de pacotes completos diferem da aquisição de Skills
avulsas. Consulte [origens diretas de pacotes completos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md)
para a sintaxe dos provedores, seleção de pacotes, somas de verificação e limites
de extração.

## Instale Agent Skills {#install-agent-skills}

Use o grupo de comandos `skills` para Skills avulsas de um repositório público
do GitHub, de uma URL de pasta ou de `SKILL.md`, ou de um publicador HTTPS com
um índice completo de Skills em `.well-known`:

```bash
agentplugins skills add owner/repository --ui
```

Substitua `owner/repository` por uma origem real. `--ui` abre o seletor de navegador
incluído e exige um terminal interativo. Use `--no-ui` para selecionar no terminal.
Para uma seleção explícita:

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- Repita `--skill` para nomes exatos. Use `--path` para uma pasta exata de Skill relativa ao repositório quando o nome for ambíguo.
- `--ref` seleciona uma referência Git exata quando os limites das partes da URL são ambíguos.
- `--dry-run` mostra o plano da instalação selecionada.
- O uso em JSON e não interativo exige seleção explícita de Skills, destinos e `--yes`, exceto na prévia com `--dry-run`.
- `--ui` não pode ser combinado com `--skill`, `--target` ou `--dry-run`.
- Publicadores HTTPS exigem um nome exato em `--skill` e um índice que liste todos os arquivos dessa Skill.

Resultados automatizados de segurança podem bloquear a instalação.
`--security-details` mostra os detalhes na saída para leitura humana; revise-os
antes de decidir se um `--accept-security-risk` explícito é adequado. Skills
instaladas aparecem em `agentplugins list` e usam os comandos normais de manutenção.

## Instale para um projeto {#install-for-one-project}

O escopo de projeto explícito está disponível na
[CLI 0.1.93 e posteriores](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93).
Escolha um diretório existente e selecione o escopo em cada comando:

```bash
agentplugins add context7 --target cursor --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

O primeiro comando mostra uma prévia; omita `--dry-run` quando quiser aplicá-la.
O segundo abre um seletor no escopo escolhido e pode aplicar a seleção do usuário.
Sem `--project-root`, o escopo de projeto explícito usa o diretório de execução.
Ele não busca em pastas superiores. Nos demais casos, comandos de adição e
manutenção usam o escopo de usuário por padrão. Na CLI 0.1.94, `install` sem
argumento de origem seleciona o escopo de projeto quando existe um arquivo regular
`uap.toml` na pasta atual exata; o escopo explícito prevalece. Consulte as
[regras de restauração](./manage) para o comportamento estrito do lock.

O destino precisa oferecer suporte aos componentes de projeto solicitados.
A instalação em projetos do ChatGPT é explicitamente incompatível. O VS Code
prepara `.github/skills` e os membros `servers` sob sua responsabilidade em
`.vscode/mcp.json` para uma única pasta local. O resultado continua Prepared:
revise a confiança no espaço de trabalho e no servidor e inspecione as
configurações do cliente. Essa observação de metadados não comprova inicialização
MCP, chamadas de ferramentas ou uso do modelo.

Leia o [guia de instalação em projetos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)
e os [limites de projeto por cliente](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
para conhecer caminhos e limites de ativação. Continue com [manutenção e restauração](./manage).
