---
title: "Go Installer SDK"
description: "通过明确的预览、确认和恢复流程嵌入安装器。"
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Installer SDK

Go Installer facade 让 Go 宿主应用直接控制**安装、更新、修复、重新生成投影、删除、状态检查及恢复**。它与 Hooks SDK 是不同的模块。

## 模块与发布边界

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

所属模块是 `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins`。应锁定**已审查的源码提交和依赖**；不要假定此模块与 Hooks SDK 共用 `v1.3.0`。

## 创建和发现客户端

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }

engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }

available := engine.Discover()
_ = available // inspect scopes, capabilities and detected executables
```

`StateRoot` 必须为绝对路径，`Registry` 必须显式列出允许的适配器。`New` 只验证配置，**不会创建目录、运行辅助程序或启动 Agent**。`Discover` 报告已注册适配器的能力、scope 与可执行文件是否存在，不能证明原生激活成功。实际部署时还需要正确设置 helper、runner、目标配置文件和内容信任策略。

## 预览、批准、执行与恢复

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "host-generated-unique-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()

plan := prepared.Plan()
// Show plan, targets, components, warnings and digests to the user.
// Only after explicit approval, with immutable prepared inputs:
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
if err != nil { return err }
_ = result

view, err := engine.Inspect(ctx)
if err != nil { return err }
if view.Recovery.Required {
    _, err = engine.Recover(ctx, view)
    if err != nil { return err }
}
```

上面是需要 `ctx`、`engine` 及导入语句的**片段**，可参考[完整外部示例](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go)。调用 `Decision{Confirmed: true}` **之前**必须向用户展示不可变 `Plan`。`Apply` 会重新验证权限和摘要；`Inspect` 只读；`Recover` 必须取得同一次检查返回的**类型化状态**。

`ClientConfigRoot` 不会从 `HOME` / `CODEX_HOME` 推断。处理本地包时必须采用 `Assess` 或与摘要绑定的 `Request.Assessment`；`TrustedLocalPackages` 仅适用于宿主明确预先信任的本地文件。`Request.Targets` 支持有限的多客户端流程；OpenCode 分组和原生删除仍有限制。旧状态迁移须先人工审查 `PrepareStateMigration`，再执行 `ApplyStateMigration` 并保留备份。

[完整安装器契约](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md)。
