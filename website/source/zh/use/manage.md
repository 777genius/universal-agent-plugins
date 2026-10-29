---
title: "维护已安装插件"
description: "区分状态诊断、更新、修复、移除和锁定恢复。"
canonicalId: "page:use:manage"
section: "use"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 维护已安装插件

安装器检查并修改它管理的状态。排查问题时，请记录包来源、所选客户端、范围和安装标识。
下方更新、修复和移除示例默认使用用户范围；项目维护与当前目录自动安装在后文分别说明。

## 先检查状态

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` 报告安装，`doctor` 诊断受管理状态，`outdated` 查找更新而不应用。
这些命令不能证明远程服务健康或客户端已认证。

| 现象 | 下一步 |
| --- | --- |
| 包不存在 | 返回[查找和安装](./install)。 |
| 受管理文件缺失或漂移 | 检查该包和范围的修复计划。 |
| 跟踪的来源有新版本 | 检查更新计划。 |
| 客户端需要信任、激活或登录 | 按该目标报告的步骤处理。 |
| 本地包未通过验证 | 将报告和[检查指南](../build/checks)交给作者。 |

## 明确更新或修复

```bash
agentplugins update context7 --target codex,cursor --dry-run
agentplugins update context7 --target codex,cursor
```

先检查来源、版本和路径，再运行第二条应用变更。`update --all` 面向全部受管理更新候选项，只在确实要更新整个集合时使用。
固定提交不会因 update 开始跟踪分支；保存的依赖范围和移动引用通过相应的显式更新操作前进。

```bash
agentplugins repair context7 --target codex,cursor --dry-run
agentplugins repair context7 --target codex,cursor
```

repair 重放记录的源以恢复受管理内容，不选择新提交，也不会悄悄覆盖外部修改的原生条目。
继续前先检查冲突和待恢复说明。

## 移除指定安装

```bash
agentplugins remove context7 --target codex,cursor --dry-run
agentplugins remove context7 --target codex,cursor
```

先检查第一条计划再应用。移除依据记录的所有权进行，不会撤销服务账户、卸载无关运行时或删除作者的源目录。
项目移除默认保留插件数据和共享的已验证内容缓存。部分客户端要求先完成外部卸载；只有完成报告的操作后才使用 `--external-uninstalled`。

## 使用已保存的依赖环境

`uap.toml` 记录所需包和目标；`uap.lock` 记录精确解析的来源与摘要。检查环境：

```bash
agentplugins deps status
agentplugins deps tree
```

新环境先执行 `agentplugins deps init`，有意编辑清单，再用 `agentplugins deps lock` 解析，并以 `agentplugins install --locked --scope user` 应用。
检查确认提示；可用 `--dry-run` 预览。[依赖指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md)说明清单、传递依赖、版本范围和更新规则。

不要假设旧的单次安装已经进入可移植清单。转移前检查 `deps status` 和实际文件。
CLI 0.1.93 默认将符合条件的远程 add 保存为依赖；本地来源、discovery 选择器、ChatGPT 准备和 `--no-save` 使用单次安装流程。
不能仅凭安装收据推断已生成可移植清单。

## 在另一台机器恢复 {#restore-on-another-machine}

在源机器上进入用于转移的文件夹并运行：

```bash
agentplugins deps export
```

将文件夹复制到目标机器，安装目标客户端，再在该文件夹中运行：

```bash
agentplugins deps import
agentplugins install --locked --scope user
```

也可合并导入和恢复：

```bash
agentplugins deps import --install
```

可选的目录参数用于选择其他交换文件夹。export 读取用户环境，import 写入用户环境；
`AGENTPLUGINS_HOME` 可指定环境。交换文件夹不是替代客户端配置目录。

只转移经过严格验证且字节不变的 `uap.toml` 和 `uap.lock`。发生实际复制时会先预览并要求确认或 `--yes`；
`--dry-run` 仅预览。导入验证可离线完成，安装仍需要确切来源或已验证缓存。
冷恢复获取固定版本并检查摘要，不重新选择较新版本。

客户端配置、设置、凭证、收据和 `PLUGIN_DATA` 不在转移内容内。请单独配置认证和备份数据。
导入已保存文件但安装失败时，处理原因后重试 `install --locked --scope user`；
若报告待恢复状态，先运行 `deps recover --scope user`。不要用 `deps lock` 重建转移的固定版本，因为移动选择器可能解析到不同内容。

<a id="restore-project-dependencies"></a>

## 恢复并维护项目

自动项目安装从 [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94)起可用。
将项目的 `uap.toml` 和 `uap.lock` 提交到版本控制，在另一处已存在的检出目录安装客户端后，从项目的确切目录运行：

```bash
agentplugins install
```

已有锁文件会严格验证。`--locked` 还要求锁已存在，适合 CI；没有锁时，首次安装可创建锁。
若从其他目录调用，请显式选择已存在的项目：

```bash
agentplugins install --locked --scope project --project-root /path/to/project
```

维护时选择相同的范围和根目录：

```bash
agentplugins list --scope project --project-root /path/to/project
agentplugins deps status --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

名称有歧义时使用 list 中的安装标识。add 和维护命令默认使用用户范围。CLI 0.1.94 的无参数 `install` 会识别当前目录的项目清单；
显式范围优先。明确项目范围但省略 `--project-root` 时只使用调用目录。将 `.uap/` 中的收据、恢复记录和保留数据留在本机，
从可移植清单重建原生组件，再完成客户端信任与激活。来源和私有来源访问权限仍须可用。

## 当前目录的安装与严格锁定

**CLI 0.1.94 已发布：** 无参数 `agentplugins install` 在当前目录存在普通 `uap.toml` 文件时自动选择项目范围，
不搜索父目录。符号链接或同名目录会被拒绝；显式 `--scope` 优先。没有本地清单时选择用户环境。
有源参数的 install 与 add 相同，仍默认用户范围。

先以 `agentplugins version` 确认版本为 0.1.94 或更高。若安装渠道较旧，使用[原生或 Homebrew 安装](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md)，
或继续用上面的显式范围命令。

```bash
agentplugins version
cd /path/to/project
agentplugins install
```

已有 `uap.lock` 会在安装或修复前严格验证，即使没有 `--locked`，或指定 `--locked=false` 也一样。
不匹配时不会修改环境；用 `update` 推进固定版本，或在有意编辑依赖声明后明确重新生成锁。
仅当锁缺失时，首次安装才可解析依赖并创建锁；`--locked` 还要求锁已存在。
[项目指南](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)说明完整范围和恢复约定。
