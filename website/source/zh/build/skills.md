---
title: "添加更多技能"
description: "为现有标准包添加一项直接子技能。"
canonicalId: "page:build:skills"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 添加更多技能

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

`skills init` 为现有标准包添加一项技能，适用于技能包、MCP 包和混合包。
应选择含 `plugin.json` 的包根目录，而不是 `skills/` 或单个 `SKILL.md`。

## 指定技能和包

假设 `./review-helper` 来自[独立技能教程](./skill)，且 `skills/release-notes/` 尚不存在：

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

位置参数技能名必填，描述也必须明确提供，长度为 1–1024 字符。
名称不会被规范化；请使用准确的小写可移植名称和连字符。选择新目录，不要期待覆盖。

## 检查新增组件

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

编辑 `skills/release-notes/SKILL.md`，说明触发条件、步骤和预期输出。
例如要求区分用户可见变化和维护变化，并为每项发布断言引用审核过的来源。
脚本、参考材料和资源可辅助说明，但其存在不证明脚本已执行或操作已获授权。

## 再次验证整个包

```bash
agentplugins author skills validate ./review-helper
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

技能验证同时报告整个包的就绪情况和直接子技能的结果。因此，新技能本身没有问题时，包内其他错误仍可能影响整体结果。
省略包路径只使用当前目录，不向上查找。从技能子目录运行不能代替选择正确包根目录。

## 失败时保留已有工作

创建失败时查看 `committed` 和错误说明。技能可能已经写入，而后续包检查失败或未完成。
重试前先检查实际文件和报告；再次 init 不会覆盖现有目标。
修正报告的输入或包问题，再重复验证。名称错误应作为明确的源修改处理，没有 force 开关。
此创作流程不包括外部技能的安装、更新、移除或 generate 生命周期。
继续[包检查](./checks)和[交付](./handoff)。
