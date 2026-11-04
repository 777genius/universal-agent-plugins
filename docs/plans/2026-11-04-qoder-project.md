# Qoder Project: отдельный узкий implementation plan

2026-11-04. Статус: план принят после двух review rounds; код и native E2E ещё не выполнены.
Новая инструкция владельца: этот leaf имеет отдельный draft PR; основной #707
приоритетно завершает shared scope support и шесть клиентов. Этот leaf не блокирует их merge.

## Scope и независимость

База ветки — актуальный `main` (исследованный HEAD `87b769a8f546b0bb1c25c4c34422f3f756995697`),
не серия коммитов #707. Ветка: `feat/qoder-project-components-20261104`.
Задача — native `.qoder/skills` и owned Project MCP stdio/Streamable HTTP;
сохранить существующие Global plugin и Project prepared-import routes.
Новые hooks/Apps/commands, shared cache ownership и автоматическое trust approval не входят.
До кода — два review раунда этого leaf; после кода — independent review и exact-head checks.

## Файлы и единственный владелец

| Leaf-владелец | Изменяемая область |
| --- | --- |
| Qoder worker | `install/integrationctl/agentplugins/clients/qoder/{scope,project,project_lifecycle}.go`; небольшие `project_components.go` / `project_policy.go` рядом, если нужны |
| Qoder worker | Существующие Qoder cases в `installer/project_qoder_test.go`, `cli/cmd/agentplugins/project_qoder_e2e_test.go`, collector/cycle под `scripts/project_native_*qoder*` |
| Main integrator, по atomic request | `domain/clients_catalog.go`, `domain/project_policy.go`, `clients/shared/*`, `adapters/nativeconfig/*`, общий native CI/transport fixture |

Не копировать shared kernel. Не менять central capabilities/traits, Directory,
CLI graph/composition, site или файлы Pi/Devin в этой ветке. Leaf doc — только этот файл.
Перед пересекающимся edit перечитать moving #698/#704 diff/checks; #704 исследован
на `46c38445`, это не фиксированный upstream dependency и не разрешение дублировать его fix.

## Q0: native prerequisites до production edits

Зафиксировать genuine Qoder executable/version/hash и original user profile.
Проверить native Project Skill discovery и оба MCP transport на disposable roots;
зафиксировать source paths и реальную возможность controlled `tools/call`.
Официальные [Skills](https://docs.qoder.com/cli/Skills) задают `.qoder/skills/<name>`;
одноимённый user Skill имеет приоритет. [MCP reference](https://docs.qoder.com/cli/mcp-reference)
задаёт User → project settings → root `.mcp.json` → local → CLI precedence.
Если observer, provider/auth или бинарная provenance недоступны — конкретная ячейка BLOCKED,
не PASS по наличию файлов. Не создавать новый provider/model framework ради этого leaf.

## Реализация

1. Native Skill destination — `<project>/.qoder/skills/<skill>`. Managed package,
   state, data и receipts остаются в существующей Project `.uap` namespace.
2. Предпочтительный Project MCP destination — root `.mcp.json` с `mcpServers`:
   он native и имеет фиксированный приоритет над `.qoder/settings.json`.
   Использовать существующий подходящий codec после exact field round-trip проверки;
   если требуется узкий Qoder codec delta — atomic request integrator, не local fork.
3. Перед записью проверить selected-name foreign ownership и higher-priority local/CLI
   source. Не копировать user entries, не изменять local overrides, trust/allowed/excluded
   или approval policy. Конфликтный selected source — понятный отказ/blocked discovery;
   чужие не конфликтующие entries сохраняются. Native policy не обязана выбирать Project.
4. Записывать отдельные exact Skill-directory/MCP-member receipts через
   `shared.ProjectComponents`, `ProjectComponentMutation`, existing nativeconfig kernel.
   Переиспользовать stdio package/data/cwd resolution; remote headers не превращать в secrets store.
5. Existing prepared-import bindings распознавать по recorded physical profile/native
   object identity. Fresh-process repair/remove старой установки идёт прежним путём;
   новый native route не усыновляет `.agentplugins-qoder-native` либо native global cache.
   Если существующей identity недостаточно, остановить и переоценить этот узкий участок.

Atomic request main owner: согласовать scoped lifecycle/native-effect traits Qoder
для новых component receipts, сохранив legacy binding dispatch. Enablement/grants
в каталоге — только после native qualification; это не prerequisite Local/Git spike.
Такой небольшой central change может быть принят отдельно от всей #707 серии;
интегратор применяет его согласованно с leaf, без временной ложной capability claim.

Central request применяется integrator в ТОТ ЖЕ leaf PR, что и adapter code;
в один проверенный merge result входят реализация и нужные traits. Leaf worker не
редактирует shared/central files параллельно, но они могут входить в его PR через
integrator. Ранний отдельный capability-only commit на main запрещён.

## Lean verification: только новые риски

Переиспользовать `TestProjectQoderPublicPreparedLifecycleNeverCallsNative`,
`TestProjectQoderSavedLifecycleSeparatesUserAndProjects` и
`TestProjectQoderDirectGroupRepairAndDefaultRemovalKeepExactAuthority` для legacy replay.
Расширить существующий facade/cycle, не писать второй Qoder lifecycle harness.

| Новая assertion | Где добавить |
| --- | --- |
| Новый mixed package: Skill + один stdio + один HTTP server; exact owned paths, repeat/update/repair/remove A, user/B/foreign preserved | Один параметризованный facade lifecycle case |
| MCP-only и Skills-only не требуют другого компонента; local/foreign/edited-owned refusal до эффекта; user Skill shadows Project без overwrite | Компактная таблица на том же fixture |
| Genuine host: native Skill source и stdio+HTTP initialize/list/call в Global и Project; restart/removal source исчезает, B/user не изменены | Расширение существующих Qoder collector/cycle и общей probe package |

Не повторять 30×30, solver/crash/security permutations, отдельные SaaS аккаунты или
всю OS×package матрицу. Использовать существующий pinned native CI; проверить paths
на заявленных платформах. Shared stdio/HTTP server fixture считается один раз у integrator.
Не считать контролируемый transport call доказательством Notion/HubSpot OAuth.

## Последовательность и бюджет

Q0 и local test preparation независимы от #707/Directory v2. Затем adapter delta,
atomic central integration, focused regression и native cycle; общий catalog closure
replay подключает main owner после появления соответствующего published grant.
Каждый законченный commit сразу push в отдельный draft PR; публикация release не разрешена.

| Работа | Новые/существенно изменённые LOC |
| --- | ---: |
| Production adapter/profile/receipt delta | 650–1 400 |
| Facade/table assertions, включая старый receipt replay | 220–450 |
| Native cycle/source/call assertions поверх existing collector | 180–350 |
| **Новые tests/fixtures** | **400–800** |

Это не измеренный diff; confidence 6/10. Сокращение с 1 000–2 200 tests достигается
reuse и одним mixed case, не удалением meaningful E2E. Existing test reruns, common
transport helper, central metadata, generated evidence и docs не считать повторно.
DoD leaf: новые native components реально обнаружены/вызваны, lifecycle и старые
receipts сохранены; недоказанные ячейки и remaining trust actions названы явно.
Применяются [owner scope](../PROJECT_INSTALL_OWNER_SCOPE.md) и capability preservation;
workers/reviewers — GPT-6.1-sol xhigh. ChatGPT/VS Code не затрагивать.

## Проверка плана

Два независимых направления (Registry/authority и integration/CI) прочитали весь
leaf в каждом из двух раундов. Автор отдельно сделал self-check; он не считается
независимым reviewer. Root согласовал результат с main 6+3 планом.

- Round 1 SHA-256: `8ff7724bf0a6797da5f6a7b774e93ea87366e54bb9a3612ca2af178e86263bed`.
- Round 2 SHA-256: `dc96efaef9ad5fad0534823c6ba2a4b3f56c5376ee9dd5ab9f225782a3d65c42`.
- Round 2: Registry PASS 9/10; integration PASS 9,2/10; открытых findings нет.
- Same-leaf central patch и текущий xhigh footer уточнены до второго round.

После review изменены только этот журнал и статус документа. Review принимает
план, не объявляет реализацию, native E2E, merge либо release завершёнными.
