---
title: "Criar um pacote MCP remoto"
description: "Descreva um endpoint MCP existente sem confundir configuração com conectividade."
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "pt"
lang: "pt-BR"
generated: false
localeDisposition: "current-translation"
translationRequired: true
---

# Criar um pacote MCP remoto {#build-a-remote-mcp-package}

**Criação a partir do código-fonte:** os comandos desta página exigem a
[compilação do código-fonte com criação habilitada](./#build-from-source).
A versão atual do instalador não inclui `agentplugins author`.

Escolha MCP remoto quando o servidor já existir e o pacote precisar descrever
como um agente se conecta a ele. A criação registra a URL sem acessá-la. Você
continua responsável por conhecer os requisitos de transporte e autenticação
do serviço antes de entregar o pacote aos usuários.

## Forneça o endpoint explicitamente {#supply-the-endpoint-explicitly}

Os exemplos usam `https://mcp.example.com/mcp` como marcador de documentação.
Substitua-o pelo endpoint MCP real do seu serviço ao preparar seu pacote.
O comando de criação aceita a URL como configuração; ele não verifica se o
endereço oferece MCP nem se sua conta pode usá-lo.

Com a compilação do código-fonte com criação habilitada:

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Conectar ao serviço MCP de documentação da equipe' \
  --url https://mcp.example.com/mcp
```

Use um destino inexistente dentro de uma pasta superior existente. O modelo
exige uma URL HTTPS absoluta ou HTTP em loopback. Credenciais, fragmentos e
tokens de URL não resolvidos não são aceitos como entradas do modelo.
Não coloque senha na URL.

## Entenda os arquivos {#understand-the-files}

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

O manifesto da raiz define a identidade do pacote. `mcp.json` descreve o servidor
em `mcpServers`; este modelo usa `type: streamable-http` e a URL fornecida.
Ele não gera um servidor nem adiciona uma Skill. Use [híbrido](./hybrid) se o
pacote também precisar de instruções sobre quando chamar suas ferramentas.

Revise o README e documente a finalidade do endpoint, os pré-requisitos de conta
e onde os usuários encontram orientações de autenticação. Mantenha segredos
fora do pacote. Uma URL em um arquivo conforme não é evidência de um serviço ativo.

## Verifique a configuração e o suporte dos destinos {#check-configuration-and-target-support}

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

`inspect` pode mostrar componentes e requisitos não resolvidos. `compat` avalia
o suporte estático dos adaptadores dos clientes selecionados; nenhum dos dois
comandos exige esses clientes instalados. Doctor não acessa o endpoint.

## Deixe o trabalho restante visível {#keep-the-remaining-work-visible}

As evidências estáticas do autor devem identificar o transporte e os clientes
pretendidos. O instalador de quem recebe ainda precisa avaliar a política de
instalação e mostrar instruções de ativação. O usuário pode precisar entrar na
conta, aprovar acesso ou iniciar uma nova sessão do cliente. A descoberta e as
chamadas de ferramentas em execução precisam de evidências separadas.

Se as verificações estáticas falharem, corrija primeiro a configuração capturada.
Se o endpoint ficar inacessível depois, investigue o serviço ou a conexão do
cliente, em vez de interpretar outro teste estático aprovado como teste de
conectividade.

Continue com [interpretação dos relatórios](./checks) e [entrega ao planejador](./handoff).
