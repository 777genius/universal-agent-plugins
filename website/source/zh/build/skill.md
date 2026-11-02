---
title: "构建独立技能"
description: "创建包含一项实用技能的标准插件包。"
canonicalId: "page:build:skill"
section: "build"
locale: "zh"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# 构建独立技能

**源码创作：** 以下命令要求[启用创作功能的源码构建](./#build-from-source)。当前安装器版本不包含 `agentplugins author`。

当智能体需要可重复的方法、检查表或写作说明，且不需要 MCP 工具连接时，选择技能。
包包含一项技能和根清单；这些创作步骤不需要服务器、运行时安装或客户端配置目录。

## 明确目标目录

以下示例假设当前工作目录是可丢弃的父目录，且 `./review-helper` 尚不存在。
包名使用可移植的小写标识。位置参数指定目录，`--name` 指定清单中的包名。

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

init 要求目标不存在，不覆盖现有目录。若报告已经提交但后续失败，应先检查结果，再决定如何继续；此流程没有覆盖开关。

## 检查生成的文件

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

生成的技能是起点。将正文改成对用户有帮助的具体方法，并保持技能名与目录名一致。例如：

```markdown
# 审查文档变更

在交付前审查文档变更时使用此技能。

1. 明确读者，以及该页面帮助读者完成的任务。
2. 检查每条命令是否说明输入和效果。
3. 区分已验证行为、前提条件和未经测试的断言。
4. 为不清楚的说明提供替换句子。

简要报告问题与文件位置。没有发现问题时，说明检查过的页面和本次未覆盖的检查。
```

保留正文上方生成的 frontmatter。`name` 和 `description` 标识技能，并帮助智能体判断何时使用。
技能文本本身不授予运行脚本或访问私有资源的权限。

## 检查同一个包根目录

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

这些都是静态检查，不会让智能体实际遵循说明，也不会评价其回答质量。
分别查看符合性和就绪情况。格式正确的技能仍需要人工检查用途、准确性和范围。

继续[添加技能](./skills)、[检查目标](./checks)或[交付给安装器](./handoff)。需要工具与说明时，可为新包选择[混合包](./hybrid)。
