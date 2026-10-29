---
title: "构建 stdio MCP 包"
description: "准备本地 Node MCP 服务器，分别检查依赖与运行行为。"
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 构建 stdio MCP 包

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

包提供通过标准输入输出通信的本地进程时，选择 stdio。骨架支持 Node.js 22 或更高版本，使用官方 MCP SDK。
创建骨架不会安装依赖或启动服务器。

## 创建骨架

在现有父目录下选择尚不存在的目标：

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

此可移植模板支持 `node` 运行时；Python 或 Go 运行时参数不属于该命令当前约定。

## 检查文件

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` 声明服务器 `local-tools`，命令为 `node`，参数为 `${PLUGIN_ROOT}/src/server.mjs`。
令牌相对于所选包解析，不是 shell 当前目录。源码从一个小型 `hello` 工具开始。
按需求修改，并将应用日志写到 stderr，为 MCP 协议保留 stdout。

检查依赖清单和锁文件。init 只写文件，不运行 npm，也不能证明主机可导入 SDK。

## 收集静态证据

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

这些是静态检查。doctor 检查有界的项目与可执行文件元数据，不能证明依赖已安装、握手成功或工具可用。
请分别理解[各层证据](./checks)。

## 为识别到的模板安装依赖

**当前平台限制：** bootstrap 的实际应用仅支持 Linux；其他平台可以先用 `--dry-run` 检查计划。
非 Linux 主机会拒绝应用，并报告 `bootstrap_platform_unsupported`。

当前源码可为具有匹配锁文件的生成 Node 布局预览并安装依赖：

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

先检查预览再应用。bootstrap 使用锁定的 npm 安装并禁用生命周期脚本。
它要求已识别的生成布局、匹配的包文件和不存在的 `node_modules`，不能替代通用包管理器。
修改过的依赖文件、竞争锁文件或已有输出可能导致拒绝。应解决报告中的布局问题；没有覆盖开关。获取依赖可能使用网络。

## 显式运行一次 MCP 检查

**当前 stdio 执行前提：** 仅支持 Linux，需要位于系统路径的 bubblewrap（`bwrap`），并通过进程约束（需要委派的 cgroup v2）和所需命名空间预检。
解析后的 Node 可执行文件必须位于 `/usr`、`/bin`、`/lib` 或 `/lib64` 下的受支持系统运行时目录；
例如用户目录内的 Node 版本管理器路径不会被接受。不满足这些条件时会拒绝执行。
远程 Streamable HTTP 检查使用独立的传输路径，不受此 stdio bubblewrap 要求限制。

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

此命令执行所选本地进程，检查 MCP 初始化和工具发现。调用 `hello` 还需明确的 JSON 参数文件；
参见[运行示例](./checks#evaluate-mcp-runtime-explicitly)。持续迭代可运行：

```bash
agentplugins author dev ./local-tools --server local-tools
```

保留运行结果对应的引擎、包和平台身份。直接创作检查不能证明同一服务器在所有原生客户端中都可用，
也不证明客户端已授予信任或完成认证。

## 准备交付

README 应说明工具、Node.js 前提、依赖文件和运行证据，标明所选服务器以及未测试的工具或环境。
然后[交付给安装器](./handoff)。需要操作说明时可[添加技能](./skills)。
