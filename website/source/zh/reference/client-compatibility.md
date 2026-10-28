---
title: "UAP 客户端兼容性证据"
description: "查看当前发布边界、固定版本的原生观察和已知限制。"
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# UAP 客户端兼容性证据

## 当前版本与范围

[CLI 0.1.93](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93)于 2026 年 10 月 26 日发布，支持显式项目安装和锁定恢复。
安装器的发布证据不能证明每个原生客户端的运行行为，也不能替代标准创作工具的可执行文件发布验收。
当前 npm/原生安装器不包含 `agentplugins author`；源码工具见[构建指南](../build/)。

| 检查层次 | 含义 |
| --- | --- |
| 包符合性 | 标准文件满足加载器和 schema 约定。 |
| 组件交付 | 适配器准备或安装其拥有的组件。 |
| 原生发现 | 特定客户端观察到预期组件。 |
| MCP 运行 | 记录中的进程或连接完成初始化及所测试调用。 |
| 认证和模型使用 | 只有明确执行过相应流程的证据才能证明。 |

项目交付通常保持 **Prepared**，因为仍可能需要信任、重新加载、明确配置或新会话。
ChatGPT 明确不支持原生项目安装。VS Code 项目范围为单个本地文件夹准备文件；固定版本元数据观察不能证明 MCP 启动或模型执行。
请参阅[逐客户端项目支持](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)和
[项目证据索引](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/PROJECT_INSTALLATION.md)。

下列历史观察对应精确的安装器、客户端、平台和测试范围。结果不会自动延伸到 0.1.93、其他客户端版本、其他安装范围或未经测试的包。
UAP 是独立社区工具；这些是维护者进行的有限观察，不是厂商认证或通用的生产就绪保证。

## 历史证据索引

详细英文记录保留完整提交、哈希和限定条件。下表提供中文导读，链接指向原始运行和证据身份；
查看报告时应同时核对版本、平台和范围。

| 历史记录 | 覆盖与结果 | 来源 |
| --- | --- | --- |
| 0.1.55 草稿，2026 年 9 月快照 | 两个独立验收范围，共 12 条原生客户端任务和 24 个顶层必需测试通过，无跳过；当时是未发布草稿，不是当前发布状态。 | [九任务运行](https://github.com/777genius/universal-agent-plugins/actions/runs/34309083566)、[Linux amd64](https://github.com/777genius/universal-agent-plugins/actions/runs/34307075390)、[固定生产者](https://github.com/777genius/universal-agent-plugins/actions/runs/34305637370)、[精确元数据](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-draft-0.1.55.json) |
| 已发布 0.1.53 | Linux arm64、macOS arm64、Windows amd64 上 Codex、Claude、OpenCode 的九任务与 18 个必需测试通过，无跳过。 | [运行 34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934)、[精确身份](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-released-0.1.53.json)、[公开证据归档](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934) |
| 0.1.53 的 Linux amd64 补充 | Ubuntu 24.04 托管环境，三个任务和六个必需测试通过；补充原始结果，不改写原始九任务证据。 | [运行 34195481283](https://github.com/777genius/universal-agent-plugins/actions/runs/34195481283)、[精确身份](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-linux-amd64-0.1.53.json) |
| 2026-09-07 源码构建快照 | Linux arm64，以及独立的 macOS/Windows 源码检查点；不是当时公开 0.1.51 二进制的运行结果。 | [身份记录](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-2026-09-07.json)、[macOS/Windows 运行](https://github.com/777genius/universal-agent-plugins/actions/runs/34158664586) |

0.1.55 的历史运行使用固定生产者的原始 npm 包、禁用安装脚本的离线 npm 安装、固定原生二进制冷启动，
以及没有证明源码的热启动。离线归档一致性复核不是新的原生运行、实时草稿检查或新的证明验证。
原始日志和归档在披露审核前保持私有；公共元数据不能当作完整可重放日志。

## 固定客户端的原生观察

0.1.53 及上述历史草稿使用以下固定版本。生命周期包含安装、更新、同版本刷新、修复和移除。

| 客户端 | 记录中的通过范围 | 关键限制 |
| --- | --- | --- |
| Codex 0.153.4 | stdio/HTTP、生命周期和脚本化技能上下文。 | Git 获取接缝不等于新的公共 Git MCP 运行证明；SSE 不支持。 |
| Claude Code 2.1.263 | Linux/macOS 的 stdio/HTTP/技能生命周期；Windows 的 HTTP 和已安装技能生命周期。 | Windows 返回 `managed_stdio_platform_unsupported`；未建立 stdio 的 cwd/argv/env/data 行为。 |
| OpenCode 1.18.29 | 生命周期、独立的扩展 stdio/技能和工具名称冲突观察。 | 观察到的工具名碰撞仍未解决；`opencode_tool_namespace_not_evaluated` 是警告，不是唯一性证明。 |

OpenCode 的 JSON/JSONC 生命周期与全局 JSON stdio/技能正文测试是不同套件。
不能从 JSONC 生命周期推断 JSONC stdio 或正文运行覆盖。配置逻辑键不同不保证可调用工具名不同。
历史碰撞中 `api/server` 和 `api server` 同时存在时暴露一个 `api_server_inspect_runtime` 名称。

技能正文进入脚本化模型请求，证明的是观察到的上下文传递，不是模型理解或回答质量。
各客户端的 HTTP 头、重定向、失败矩阵和并发文件保护测试范围也不同；不要把一个客户端的观察复制为另一个客户端的结果。

## 原生运行不包括哪些证明

这些维护者运行使用真实固定版本客户端、临时配置和脚本化回环模型服务，并允许网络访问。
临时 HOME 与显式子进程环境不提供操作系统或进程隔离。
测试不证明真实模型质量、OAuth/登录、桌面应用、实际线上服务、独立采用或厂商认可。
六平台二进制构建或启动器检查也不能扩大原生客户端矩阵。

历史源码检查点与公开版本必须保持区分。源树、测试接缝或归档哈希不能替代不同提交的重新运行。
旧失败记录仍是失败；后续通过可更新平台摘要，不能将旧记录改写为通过。

## 已发布项目范围与版本化观察

CLI 0.1.93 支持显式项目范围；上面的历史用户范围结果不能自动证明项目行为。
项目组件交付、所有权、原生发现与 MCP/模型运行分别报告。
ChatGPT 项目范围会被拒绝，其他客户端各有组件、准备步骤和版本要求。

[VS Code 1.143.1 固定观察](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034)在未授予工作区信任的情况下，
验证独立 A/B 文件夹、A 的新启动和移除后的元数据。适配器返回 Prepared，并验证安装文件；这不能证明 MCP 启动、工具调用或模型执行。

[完整英文历史测量与哈希](/en/reference/client-compatibility) ·
[复现和证据发布流程](https://github.com/777genius/universal-agent-plugins/blob/main/docs/released-native-client-proof.md) ·
[当前项目支持与限制](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
