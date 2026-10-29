---
title: "Evidências de compatibilidade dos clientes UAP"
description: "Versões exatas dos clientes, identidade do código-fonte, camadas de execução testadas e limitações das versões."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: false
---

# Evidências de compatibilidade dos clientes UAP {#uap-client-compatibility-evidence}

## Versão atual e escopo {#current-release-and-scope}

A [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94), versão nativa publicada em 28 de outubro de 2026,
adiciona seleção automática de projeto para `install` sem argumento de origem
quando a pasta atual exata contém um arquivo regular `uap.toml`. O escopo
explícito prevalece; pastas superiores não são pesquisadas. Locks existentes
são verificados estritamente mesmo sem `--locked`. A entrega explícita em
projetos foi introduzida na 0.1.93. Essas evidências da versão do instalador
não qualificam a execução em todos os agentes nativos nem o executável padrão
de criação. O instalador npm/nativo atual não inclui `agentplugins author`;
consulte o [guia de criação](../build/) para as ferramentas do código-fonte.

Interprete a compatibilidade como várias perguntas separadas:

| Verificação | Significado |
| --- | --- |
| Conformidade do pacote | Os arquivos padrão do pacote atendem ao contrato do carregador e dos esquemas. |
| Entrega de componentes | O adaptador preparou ou instalou os componentes que gerencia. |
| Descoberta nativa | Um cliente específico observou o componente pretendido. |
| Execução MCP | Um processo ou conexão registrado inicializou e realizou as chamadas testadas. |
| Autenticação e uso do modelo | Só são comprovados quando as evidências os exercitam explicitamente. |

A entrega em projetos costuma permanecer **Prepared** enquanto faltam confiança,
recarga, configuração explícita ou etapas de sessão. A instalação em projetos
do ChatGPT é explicitamente incompatível. O escopo de projeto do VS Code prepara
arquivos para uma única pasta local; a observação de metadados de sua versão
fixada não é evidência de inicialização MCP nem de execução do modelo. Consulte
[suporte a projetos por cliente](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) e o [índice de evidências de projetos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/PROJECT_INSTALLATION.md).

Os registros abaixo preservam versões históricas exatas, versões dos clientes,
identidades das plataformas e limitações observadas. Eles não se aplicam
automaticamente à 0.1.94, a uma versão posterior de cliente, a outro escopo ou
a um pacote não testado. UAP é uma ferramenta independente da comunidade; estas
são observações delimitadas dos mantenedores, não certificação de fornecedores
nem uma afirmação de prontidão universal para produção.

## Rascunho histórico do instalador 0.1.55 {#historical-draft-installer-0155}

**No ponto de verificação registrado de setembro de 2026, a 0.1.55 era um rascunho
não publicado.** Esses resultados qualificam aquelas cópias do rascunho; eles
não representam o estado atual da versão nem uma instrução de atualização.
As evidências da 0.1.53 lançada, abaixo, continuam válidas e separadas.

Em dois escopos qualificados independentemente, **12 cenários de clientes nativos
passaram nos 24 testes obrigatórios de nível superior, sem omissões**. Cada um
usou o tarball npm original do produtor, instalação npm sem rede e com scripts
desabilitados, bootstrap a frio a partir do binário nativo congelado e uma
inicialização subsequente sem o código-fonte da prova. Os clientes genuínos nas
versões fixadas foram executados em perfis novos e descartáveis.

| Cliente testado | Linux arm64 | Linux amd64 | macOS arm64 | Windows amd64 |
| --- | --- | --- | --- | --- |
| Codex 0.153.4 | Ciclo de vida, stdio/HTTP e contexto de Skill aprovados | Mesmo escopo aprovado | Mesmo escopo aprovado | Mesmo escopo aprovado |
| Claude Code 2.1.263 | Ciclo de vida, stdio/HTTP e Skills instaladas aprovados | Mesmo escopo aprovado | Mesmo escopo aprovado | Ciclo de vida HTTP/Skill instalada aprovado; stdio gerenciado incompatível |
| OpenCode 1.18.29 | Suítes de ciclo de vida, stdio/Skill ampliadas e observação de colisões aprovadas | Mesmo escopo aprovado | Mesmo escopo aprovado | Mesmo escopo aprovado |

O ciclo de vida abrange instalação, atualização, renovação da mesma versão,
reparo e remoção. Claude no Windows informa `managed_stdio_platform_unsupported`;
sua execução stdio e o comportamento de cwd/argv/env/dados não foram comprovados.
A colisão observada no OpenCode continua sem solução, com o aviso
`opencode_tool_namespace_not_evaluated`. O ciclo de vida JSONC não comprova a
execução stdio/do corpo da Skill em JSONC. O ponto de substituição da aquisição
Git nos testes Codex não representa uma nova comprovação de execução nativa
usando Git público.

A [execução histórica de nove cenários](https://github.com/777genius/universal-agent-plugins/actions/runs/34309083566) passou em nove cenários e
18 testes obrigatórios com o harness `0cfa2b7`; sua última reverificação do
rascunho ocorreu em **2026-09-09T04:10:39.529018+00:00**. A [execução Linux amd64](https://github.com/777genius/universal-agent-plugins/actions/runs/34307075390)
passou em três cenários e seis testes obrigatórios com o harness `fde7298`;
sua última reverificação do rascunho ocorreu em
**2026-09-09T03:36:42.614887+00:00**. Ambas se vinculam à mesma
[execução congelada do produtor](https://github.com/777genius/universal-agent-plugins/actions/runs/34305637370). Os [metadados compactos das evidências](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-draft-0.1.55.json)
registram as identidades exatas do produtor, pacote, harness, qualificação e
arquivos compactados.

Os dois escopos preservados passaram na verificação sem rede. Uma cópia nova
do verificador em `0cfa2b7` também reverificou o escopo Linux amd64 preservado
`fde7298`; isso não foi uma nova execução nativa nem uma reverificação ao vivo
do rascunho e não acrescenta tempo de qualificação. A consistência dos hashes
dos arquivos compactados não é uma nova verificação de atestações nem evidência
do estado atual do rascunho. Logs brutos e arquivos compactados continuam
privados, aguardando revisão para divulgação; somente os metadados são públicos.

Estas são observações dos mantenedores com clientes genuínos, provedores de
modelo simulados por scripts em loopback e acesso à rede. Perfis descartáveis
e ambientes explícitos de processos filhos não oferecem isolamento de sistema
operacional ou de processos. Delas não decorrem afirmações sobre modelos reais,
OAuth/acesso à conta, aplicações desktop, serviços ativos, certificação de
fornecedores, condição oficial ou adoção independente. Validade do pacote,
instalação, ativação, execução e OAuth continuam sendo camadas separadas de
evidência. Essa prova do instalador não qualifica a versão do executável padrão
de criação (D5).

## Instalador lançado 0.1.53 {#released-installer-0153}

A [versão pública 0.1.53](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.53) usa o commit de instalador
`28cf05af0a1e4fea642825dd34b78f9c99094ab5`. A [execução nativa 34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934)
passou nos nove jobs e nos 18 testes obrigatórios sem omissões. Ela baixou os
binários públicos, verificou o manifesto e as atestações antes da execução e
usou auxiliares separados de teste/sondagem compilados do código-fonte no commit
do harness `9c33cfac81fc00e61205591321c89c5675fedb60`.

| Cliente testado | Linux arm64 | macOS arm64 | Windows amd64 |
| --- | --- | --- | --- |
| Codex 0.153.4 | Ciclo de vida stdio/HTTP e de contexto de Skill aprovado | Mesmo escopo aprovado | Mesmo escopo aprovado |
| Claude Code 2.1.263 | Ciclo de vida stdio/HTTP/Skill aprovado | Mesmo escopo aprovado | Ciclo de vida HTTP/Skill instalada aprovado; stdio gerenciado incompatível |
| OpenCode 1.18.29 | Suítes de ciclo de vida, observação de colisões e stdio/Skill ampliadas aprovadas | Mesmo escopo aprovado | Mesmo escopo aprovado |

O ciclo de vida inclui instalação, atualização, renovação da mesma versão,
reparo e remoção. O ciclo de vida JSON/JSONC e as fixtures de colisão HTTP do
OpenCode são separados de sua suíte global JSON de stdio/corpo de Skill;
não se infere cobertura de stdio/corpo em JSONC. Os nomes de ferramentas do
OpenCode que colidem continuam sendo uma limitação, com o aviso
`opencode_tool_namespace_not_evaluated`. O ponto de substituição da aquisição
Git na suíte obrigatória Codex não representa nova comprovação de execução
nativa usando Git público.

Estas são observações dos mantenedores em perfis novos hospedados pelo GitHub,
com provedores de loopback simulados por scripts e acesso à rede. Elas não
comprovam qualidade de modelos reais, OAuth/acesso à conta, aplicações desktop,
serviços ativos, adoção independente ou certificação de fornecedores. Validação
do pacote, instalação, ativação/descoberta e execução continuam sendo camadas
distintas de evidência. A [execução separada do inicializador em seis plataformas](https://github.com/777genius/universal-agent-plugins/actions/runs/34164699278)
não estende essa tabela de clientes nativos a outras arquiteturas.

As [identidades das evidências da versão lançada](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-released-0.1.53.json) registram os nove
cenários e os hashes exatos. Um arquivo compactado verificado contém 529 arquivos
originais e seu resumo, SHA-256:
`0d315f3a24bb6c41d184f0632d76a7228e18a6c1c41be2774e23c8066dfd3af6`.
O [arquivo público de evidências](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip) e seu [arquivo de soma de verificação](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip.sha256)
foram baixados novamente e tiveram os hashes conferidos após a publicação na
[versão separada de evidências](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934). O [procedimento de reprodução e publicação](https://github.com/777genius/universal-agent-plugins/blob/main/docs/released-native-client-proof.md)
mantém esses recursos separados da versão imutável do instalador.

## Prova complementar de Linux amd64 para 0.1.53 {#supplemental-linux-amd64-proof-for-0153}

A [execução 34195481283](https://github.com/777genius/universal-agent-plugins/actions/runs/34195481283) passou nos três jobs Linux amd64 e nos seis
testes obrigatórios sem omissões, em runners Ubuntu 24.04 hospedados pelo GitHub.
Ela baixou a mesma versão pública UAP 0.1.53 identificada acima, com SHA-256 do
binário nativo
`d9d1a99a6d0d5e497bad793df017efc8e59a1e53c4012e6375ade6a3d12c3df9`.
Commit do harness: `376d06b3b71c3826a3c3dbbbf8ed5def43eb5c72`.

| Cliente testado | Observação Linux amd64 |
| --- | --- |
| Codex 0.153.4 | Ciclo de vida nativo, stdio/HTTP e asserções de contexto de Skill aprovados |
| Claude Code 2.1.263 | Ciclo de vida nativo e asserções de execução stdio/HTTP/Skill aprovados |
| OpenCode 1.18.29 | Suítes de ciclo de vida, stdio/Skill ampliadas e observação de colisões aprovadas |

Isso complementa os nove jobs originais; não reescreve suas evidências.
Aplicam-se os mesmos limites de provedores loopback, modelos reais/OAuth e
colisões do OpenCode. O ciclo de vida JSONC ainda não comprova execução
stdio/corpo em JSONC. A limitação anterior de stdio gerenciado do Claude no
Windows permanece.

As [identidades das evidências e os hashes de arquivos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-linux-amd64-0.1.53.json) identificam
a versão e o harness exatos. O [arquivo separado de evidências](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-linux-amd64-34195481283)
contém 175 arquivos originais e o resumo do verificador; SHA-256 do arquivo:
`d4baaf822ea1cc9c29d46d4393e2ae7771d34bd68b80864aaf0ceed34ac1df8d`.
Verifique-o usando `scripts/verify-released-native-evidence.py --scope linux-amd64`
com as identidades registradas da versão e do harness.

## Observações históricas de compilações do código-fonte {#historical-source-built-observations}

Retrato de **2026-09-07**. Estas são observações delimitadas de CLIs nativas para
o instalador `agentplugins`. Elas não estabelecem suporte a todas as versões
de um cliente, aplicação desktop, sistema operacional, pacote ou fluxo de
autenticação. UAP é uma ferramenta independente da comunidade; esta matriz não
é uma certificação de fornecedores.

### Limite entre código-fonte e versão histórica {#historical-source-and-release-boundary}

O ponto de verificação macOS/Windows abaixo é `ef20b93`; as evidências Linux
mantêm sua identidade histórica separada. Nenhum resultado é transferido
implicitamente para outro SHA.

As execuções Linux abaixo usaram um **instalador compilado do código-fonte**,
SHA-256 do binário
`3e5d4b8ee25a16d274b0f3bbdaf35f6963416650fed972995e65817f274cf8d8`,
da árvore de produto `dcd9090458b036515c9aed77ee5d3e66f0dcb4f5`, commit base
`1c831fa8b2016fda68c73fc60fe6f691581d0b13` e SHA-256 do patch binary/full-index
`d768da15903e0db5138dab72d2dbf8c94fbb5ca2e3b46f06492cbd3ac2bcaa42`.

A entrega foi integrada no [PR #184](https://github.com/777genius/universal-agent-plugins/pull/184) como
[`7d17c77599306d662e0bec0e0fd77d35a4f8941c`](https://github.com/777genius/universal-agent-plugins/commit/7d17c77599306d662e0bec0e0fd77d35a4f8941c). Essas execuções históricas
identificam a árvore de produto testada antes da integração, não uma repetição
de execução naquele commit de merge. A versão mais recente verificada para o
retrato do código-fonte de 2026-09-07 é **0.1.51 (2026-09-06)**, que **não** contém
o PR #184. Não aplique esses resultados de execução do candidato àquele binário
publicado.

As [identidades compactas das evidências](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-2026-09-07.json) registram hashes de imagem,
cliente, instalador, teste, sondagem, chamada e log de `codex-03`, `claude-03`,
`opencode-03`, `git-01` e `opencode-extended-11`, auditado independentemente.
As transcrições brutas não acompanham este resumo; os hashes identificam
artefatos preservados, não um arquivo público de transcrições reproduzíveis.

### Matriz histórica de CLIs nativas Linux arm64 {#historical-linux-arm64-native-cli-matrix}

Cada execução usou um novo projeto de teste e raízes HOME/cliente/XDG isoladas
em um contêiner. A execução usou servidores MCP loopback e provedores simulados
por scripts, sem credenciais de usuário nem modelo real. Resumo criptográfico
da imagem do contêiner:
`sha256:df6ab45e95c4425fcdccfc9a143bec29e2bf08293b63482c7843f398e06baabb`.
O registro compacto não especifica a versão da distribuição.

| Camada de evidência | Codex 0.153.4 | Claude Code 2.1.263 | OpenCode 1.18.29 |
| --- | --- | --- | --- |
| Validade do pacote padrão | Verificação separada de carregador/conformidade | Verificação separada de carregador/conformidade | Verificação separada de carregador/conformidade |
| Caminho de instalação | Pacote local, marketplace gerenciado/plugin nativo | Pacote local, `@skills-dir` pessoal | JSON e JSONC globais, fixtures separadas |
| Instalar, atualizar versão, renovar a mesma versão, reparar, remover | Aprovado | Aprovado | Aprovado |
| Ativação/descoberta | Descoberta nativa de plugin pelo app-server e de Skill | Descoberta nativa de Skill pela CLI | Configuração/descoberta nativa; catálogo HTTP em execução |
| Execução nativa de ferramentas MCP | Stdio padrão/explícito e HTTP | Dois servidores stdio e HTTP | Stdio padrão/explícito e HTTP, suítes separadas |
| Continuidade de cwd, argv, env e dados stdio | Aprovada em atualização/renovação/reparo | Aprovada em atualização/renovação/reparo | Aprovada em atualização/renovação/reparo na suíte JSON ampliada |
| Corpo da Skill no contexto de saída | Aprovado, Skill selecionada explicitamente com provedor Responses simulado por scripts | Aprovado, ferramenta Skill nativa com provedor Messages simulado por scripts | Aprovado, ferramenta Skill nativa com provedor simulado por scripts |
| Comportamento de modelo real | Não avaliado | Não avaliado | Não avaliado |
| OAuth/acesso à conta | Não avaliado | Não avaliado | Não avaliado |

Instalação, ativação e execução são observações separadas. Nenhuma substitui a
validação pelo contrato Agent Plugins publicado. O ciclo histórico do adaptador
Linux não repetiu o corpus fixado de conformidade e não faz nova afirmação de
conformidade. Solicitações dos provedores simulados por scripts comprovam o
comportamento observado dos clientes e as chamadas de ferramentas; não comprovam
raciocínio de modelo real nem OAuth bem-sucedido.

A execução ampliada do OpenCode usa a imagem
`sha256:cf4b5019fbb6d887a96ff066402594b367b7df75c90a86813d9ab9ef5fa491cb`
com ripgrep 14.1.1 sem rede. Ela reutiliza a identidade exata do instalador
acima; o SHA-256 do código-fonte do teste é
`55314bbed0587390965914a2d685a3b16d0ea926950625cc4dd91ab95ea2b0d4`.
A revisão independente verificou os 21 hashes de transcrições referenciados e
81 hashes de manifestos de código-fonte. O marcador único BODY da Skill chega
ao contexto de saída da ferramenta e não consta dos metadados/definições de
ferramentas iniciais; isso não comprova compreensão pelo modelo. A execução
não comprova novo código do instalador nem outro sistema operacional.

### Limitações históricas e provas mais restritas {#historical-limitations-and-narrower-proofs}

- **Colisão de nomes de ferramentas no OpenCode:** as chaves lógicas `api/server` e `api server` funcionam isoladamente, mas juntas expõem um único nome invocável `api_server_inspect_runtime`, embora ambos os catálogos MCP estejam visíveis. Preservar chaves de configuração não significa unicidade dos nomes invocáveis. O ponto de código-fonte `c154185` adiciona o aviso `opencode_tool_namespace_not_evaluated` à saída de entrega da CLI. Ele alerta sobre combinações incompatíveis com colisões em execução; não renomeia ferramentas, comprova unicidade nem invalida um pacote padrão.
- As fixtures HTTP do OpenCode comprovam preservação de configuração alheia em JSON/JSONC e recusa de colisões. A execução global JSON separada `opencode-extended-11` comprova cwd stdio, argv/env, expansão única de tokens e continuidade de dados. Ferramentas e metadados de Skills ficam ausentes após a remoção, sem atividade de sondagem e com uma solicitação real ao provedor principal observada. Não se infere cobertura de stdio/corpo em JSONC.
- Codex excluiu SSE, Skills inválidas e ambientes de execução ausentes; isolou falhas de inicialização de executáveis; manteve chamadas saudáveis funcionando; e recusou entradas totalmente incompatíveis sem alterações. Claude não exercitou toda essa matriz de falhas. OpenCode comprovou exclusão de SSE com componentes irmãos saudáveis, não a matriz completa do Codex.
- Codex observou prioridade de protocolo/cabeçalhos HTTP e um redirecionamento que o cliente não seguiu para outra origem. Claude e OpenCode comprovaram chamadas HTTP sem testar independentemente redirecionamentos ou prioridade de cabeçalhos.
- Codex e Claude comprovaram proteção estática de conteúdo alheio e divergências dentro de suas fixtures. Testes de regressão separados cobrem condições de corrida na reversão de publicação; isso não garante proteção contra quaisquer gravações simultâneas no sistema de arquivos.
- `git-01` comprovou aquisição real de Git público imutável, instalação, ativação, descoberta nativa de Skill no Codex e remoção. Sua fixture apenas de Skill foi [AP-6.2-MISSING-LOCATION-OK no commit fixado do corpus](https://github.com/Booyaka101/agent-plugins-conformance-kit/tree/4d163c6281a9b4929f482f387efe340b4dc175a1/fixtures/core/AP-6.2-MISSING-LOCATION-OK/plugin). O acesso à rede terminou antes da descoberta nativa. Execução MCP de Git e troca/atualização imutável não foram avaliadas. Execuções nativas Git de Claude/OpenCode não foram avaliadas. Instalações locais e de Git imutável não exigem o Directory.

### Ponto histórico de código-fonte macOS arm64 e Windows amd64 {#historical-macos-arm64-and-windows-amd64-source-checkpoint}

A [execução GitHub Actions 34158664586](https://github.com/777genius/universal-agent-plugins/actions/runs/34158664586) passou nos seis cenários
obrigatórios de clientes nativos sem omissões no commit de código-fonte
[`ef20b9314b69d4094f4922946b8ad39024371202`](https://github.com/777genius/universal-agent-plugins/commit/ef20b9314b69d4094f4922946b8ad39024371202), árvore
`4ba999db51304b056ec559c5429ed9aab9aace60`. Todos os **348 hashes indexados de
artefatos** foram verificados. Essa é uma evidência nativa daquele ponto exato
compilado do código-fonte, não do instalador Linux histórico, de um binário
publicado nem de aprovação em todas as verificações de qualidade/lançamento
do repositório.

| Cliente testado | macOS arm64 | Windows amd64 |
| --- | --- | --- |
| Codex 0.153.4 | Ciclo de vida nativo, execução stdio/HTTP e contexto de Skill simulado por scripts aprovados | Ciclo de vida nativo, execução stdio/HTTP e contexto de Skill simulado por scripts aprovados, incluindo remoção, verificações de remoção repetida e preservação de conteúdo alheio |
| Claude Code 2.1.263 | Descoberta nativa e ciclo de vida de execução stdio/HTTP simulado por scripts aprovados | Ciclo de vida HTTP e de Skill instalada aprovado; stdio gerenciado explicitamente incompatível |
| OpenCode 1.18.29 | As três suítes aprovadas: ciclo de vida JSON/JSONC, stdio/corpo de Skill ampliados, colisão de ferramentas HTTP | As três suítes aprovadas: ciclo de vida JSON/JSONC, stdio/corpo de Skill ampliados, colisão de ferramentas HTTP |

Para Codex e OpenCode, o ciclo de vida aprovado inclui instalação, atualização
de versão, renovação da mesma versão, reparo e remoção. A suíte JSON global
ampliada do OpenCode comprova separadamente continuidade de cwd/argv/env/dados
stdio e corpo da Skill na saída; isso não infere cobertura stdio/corpo em JSONC
nem resolve a colisão observada de nomes de ferramentas. Codex também exercita
aquisição imutável em sua suíte nativa. A validade do pacote padrão continua
separada dessas observações dos clientes.

**Claude no Windows tem um contrato de execução mais restrito.** As etapas
nativas HTTP e de Skill instalada passaram por A/B/C, reparo e remoção, com
retenção de dados do instalador verificada. Stdio gerenciado informa o diagnóstico
exato de prontidão `managed_stdio_platform_unsupported`. A execução stdio foi
observada como incompatível; seu comportamento de cwd/argv/env/dados permanece
não avaliado. Essa limitação explícita da plataforma não invalida o pacote
portátil nem bloqueia os componentes HTTP/Skill saudáveis testados.

Foram usadas máquinas descartáveis hospedadas pelo GitHub, com perfis novos
e projetos de teste. As versões dos sistemas operacionais não foram capturadas
nos registros compactos dos runners. **A rede não foi bloqueada**; os endpoints
de modelo eram serviços loopback simulados por scripts. Os resumos criptográficos
exatos dos clientes/compilações e das evidências dos runners constam do registro
compacto. Essas execuções não permitem afirmações sobre modelos reais, acesso
à conta, OAuth ou aplicações desktop.

As fixtures nativas, o wrapper do runner e o workflow nativo são idênticos byte
a byte ao ponto `a6462a7`, revisado independentemente. Sua revisão semântica é
reutilizada; os novos artefatos das execuções tiveram seus hashes verificados
independentemente. Isso não substitui a revisão de outras alterações do produto
entre os commits.

Pontos anteriores, incluindo cenários Windows com falhas, permanecem nas
identidades das evidências como histórico. Essa execução substitui seu resumo
por plataforma; as falhas não foram reclassificadas como aprovações.

## Outras plataformas e clientes {#other-platforms-and-clients}

| Superfície | Limite das evidências |
| --- | --- |
| Ponto de código-fonte macOS arm64 | Todas as suítes nativas obrigatórias passaram em `ef20b93`, incluindo a execução ampliada OpenCode |
| Ponto de código-fonte Windows amd64 | Todas as suítes nativas obrigatórias passaram em `ef20b93`; stdio gerenciado do Claude continua explicitamente incompatível |
| Linux amd64 | Prova complementar da 0.1.53 lançada, acima; não faz parte das execuções históricas |
| Aplicações desktop | Não avaliadas por essas execuções de CLI |
| Histórico macOS 15.6.1 arm64, UAP 0.1.22 | Apenas descoberta/ciclo de vida; Claude 2.1.205, Gemini 0.36.0, OpenCode 1.18.4; projeção de configuração Cline/Windsurf, versões dos clientes não registradas |

O [registro histórico de macOS](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md) tem seus próprios resumos
criptográficos de instalador, pacote e transcrições. Ele não comprova chamadas
de ferramentas de navegador, modelo, OAuth nem execução para o candidato atual.
Novos resultados precisam identificar seu próprio SHA de código-fonte e as
versões de cliente/SO antes de substituir qualquer célula pendente.

## Evidências do escopo de projeto {#project-scope-evidence}

A entrega explícita em projetos foi lançada na CLI 0.1.93. As versões históricas
e observações nativas acima mantêm seu escopo original. A instalação em projetos
tem um contrato separado por cliente: propriedade instalada, descoberta nativa
e operação MCP/modelo são informadas separadamente. O escopo de projeto do
ChatGPT é recusado explicitamente; outros destinos têm restrições próprias de
componentes, preparação ou harness.

A preparação de projetos VS Code usa `.github/skills` e `.vscode/mcp.json`.
A [observação da versão fixada VS Code 1.143.1](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034) verificou metadados em
pastas A/B separadas, uma nova abertura de A e a remoção, sem conceder confiança
ao espaço de trabalho. Essa é uma evidência de metadados; o adaptador informa
Prepared e verificação dos arquivos instalados, sem afirmar inicialização MCP,
chamadas de ferramentas ou execução do modelo.

[Guia de instalação em projetos](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Suporte atual a projetos e limitações](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
