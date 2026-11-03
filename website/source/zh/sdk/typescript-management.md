---
title: "TypeScript 管理 SDK"
description: "在可信 Node 应用中使用 Go 管理引擎。"
canonicalId: "page:sdk:typescript-management"
section: "sdk"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# TypeScript 管理 SDK

此 SDK 提供可信 **Node/Electron 宿主** 使用的 **Management API v1** 类型化客户端。插件解析、原生配置、状态、事务及恢复仍由 **Go 引擎**负责。

## 版本与安装

源码包 `universal-agent-plugins-management-sdk` 版本 **0.1.1**，要求 Node.js **22.18+**。CLI **0.1.95+** 支持 `manage serve` 和 `manage discover`。先检查 npm 是否实际发布对应版本：

```bash
npm view universal-agent-plugins-management-sdk@0.1.1 version
npm install --save-exact universal-agent-plugins-management-sdk@0.1.1
```

SDK **不下载 Go 引擎**；应另行固定并验证可执行文件与 SHA-256。根入口 `createClient` 用于已拥有可信私有 endpoint 的环境；`/node` 入口提供 `startUap` 与 `discoverTargets`。

## 启动自有管理进程

```ts
import { startUap } from 'universal-agent-plugins-management-sdk/node';

const host = await startUap({
  executable: '/absolute/path/to/pinned/agentplugins',
  config: {
    stateRoot: '/absolute/private/uap-state',
    dependenciesRoot: '/absolute/private/uap-dependencies',
    scope: 'user',
    targets: [{
      id: 'claude-profile',
      client: 'claude',
      configRoot: '/absolute/private/claude-profile',
      executable: '/absolute/path/to/claude',
    }],
  },
});

try {
  const environment = await host.client.environment();
  console.log(environment.entries.map((entry) => entry.alias));
  // Inspect environment.pendingOperations before admitting new work.
} finally {
  await host.close();
}
```

示例中的绝对路径由宿主控制。通常应省略 `stateRoot` 与 `dependenciesRoot`，与 CLI 共享标准状态及锁；只有明确隔离时才配置独立路径。**切勿向浏览器、renderer、存储或日志泄露 bearer token、私有 Profile 信息和 bootstrap stdout。** `close()` 只关闭它所拥有的进程。

## 准备、确认与观察

```ts
const plan = await host.client.prepare({
  action: 'install',
  source: 'example/plugin',
  alias: 'example',
  targetIds: ['claude-profile'],
});
if (!plan.confirmable) {
  throw new Error(plan.conflicts.map((item) => item.message).join('\n'));
}
// Display exact planned actions, targets, warnings and conflicts first.
// Persist plan.planId AND requestId in durable host state before apply.
const requestId = crypto.randomUUID();
const accepted = await host.client.apply(plan.planId, requestId);
const operation = await host.client.wait(accepted.operationId, {
  timeoutMs: 120_000,
  onProgress: (snapshot) => console.log(snapshot.phase),
});
console.log(operation.state, operation.result?.nextActions);
```

`prepare` 返回待审查计划，`apply` 在用户确认后接受计划，`wait` 观察最终结果。调用 apply **之前**保存 `planId` 和 `requestId`；丢失响应后必须复用**同一组 ID**。`wait` 的超时或 AbortSignal **不会取消已接受的操作**。`cancel` 只发出取消请求，不保证回滚。应展示 `partial` / `failed` / `interrupted` 及 `environment.pendingOperations`。

`discoverTargets` 只检查原生客户端和私有位置；`discoverSkills`、`skills.preview`、`skills.discard` 管理 Skills 搜索会话；`skills.local.prepare` 仅编辑宿主明确授权的根目录。某些原生删除需要用户先在客户端手动卸载，再针对当前操作重新生成带 `externalUninstalled: true` 的计划。

**兼容性提示：**SDK 0.1.1 提供 `checkUpdates`，但 **CLI 0.1.95 及以下缺少对应 endpoint，返回 HTTP 404**。需要已验证的更新引擎或锁定源码构建。该操作只是只读检查，**不会更新插件**。运行 `npm run build`、`npm run typecheck`、`npm test`、`npm run test:consumer` 并另外执行原生 E2E。

[完整 SDK 文档](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md)。
