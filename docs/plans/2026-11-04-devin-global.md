# Devin Global Skills/MCP: отдельный qualified-profile leaf

2026-11-04. Статус: план принят после двух review rounds; native Global route ещё не реализован/квалифицирован.
Отдельный draft PR, основной #707 со shared scope и шестью клиентами не ждёт этот leaf.
Legacy Cascade/Windsurf IDE и текущий Devin CLI — разные native profiles, не синонимы.

## База и исключительная область

Ветка `feat/devin-global-components-20261104` от актуального `main` (исследованный
`87b769a8f546b0bb1c25c4c34422f3f756995697`), не от commit series #707.
Scope — genuine выбранный Devin Global Skills + stdio/Streamable HTTP MCP;
сохранить текущий Project Devin и старый Cascade user lifecycle. Новые hooks,
native plugin/Cloud registration, generic version unlock и fake HOME не входят.
Перед production edits — два review раунда leaf; перед merge — independent code review.

| Владелец | Файлы/контракт |
| --- | --- |
| Devin worker | `install/integrationctl/agentplugins/clients/windsurf/{detect,scope,plan,project_native,project_lifecycle,project_validation,version_profile}.go`; небольшие global layout/lifecycle helpers рядом |
| Devin worker | `installer/project_devin_*_test.go`, existing Devin CLI/provider cases и `scripts/project_native_windsurf_*` cases только по новой assertion |
| Main integrator, atomic request | Central capabilities/profile admission, `clients/shared/*`, `adapters/nativeconfig/*`, CLI selected-host propagation и общий native probe/CI seam |

Не менять legacy `native.go`/`lifecycle.go`/`native_server.go` по умолчанию: они владеют
Cascade codec/receipts. Если узкая корректировка нужна, сначала доказать конкретный
cross-route defect. Shared code запрашивать, не копировать. Перечитать moving #698/#704
(ранее #704 `46c38445`), не дублировать env/profile/collision/recovery infrastructure.

## D0: первый blocking spike

1. Genuine binary/version/hash и platform: уже наблюдавшийся профиль `devin/devin.exe`
   `3000.11.205`; [retained evidence](project-install-e2e/windsurf-devin-implementation-evidence.md)
   доказывает bounded version/help и формат, не Global discovery/runtime.
2. Определить, как existing public CLI/Management selection связывает один executable,
   observed version и original config root при coexistence `windsurf`/`devin`.
   Сегодня detection выбирает Windsurf первым, `ConfigRoot` — legacy channel. Нельзя
   молча reorder, подменить root, перенести receipts или invent host presence из каталога.
3. Если existing selection/state contract не выражает отдельный точный профиль,
   передать integrator узкий atomic request и пересчитать только этот seam; это
   prerequisite готовности leaf, но не причина блокировать шесть клиентов #707.
4. До существенных edits проверить Global Skill source и controlled stdio/HTTP call
   на этом genuine host. Недоступный observer/provider/auth/provenance — BLOCKED,
   не причина добавлять provider framework или выдавать файлы за runtime PASS.

## Пути и реализация после D0

Native Global root из original user/XDG authority — обычно `~/.config/devin`,
Windows `%APPDATA%/devin`. Skill — `<root>/skills/<name>`, MCP — `<root>/mcp_config.json`.
Project остаётся `.devin/skills` и `.devin/mcp_config.json`. Native references:
[Skills](https://docs.devin.ai/cli/extensibility/skills/overview),
[MCP](https://docs.devin.ai/cli/extensibility/mcp/configuration),
[precedence](https://docs.devin.ai/cli/reference/configuration/global-vs-local).
Current docs сами по себе не разблокируют иные версии/IDE profiles.

1. Параметризовать existing `devinComponents`/`devinDesiredServers` на validated
   Global/Project layout и exact ownership kinds. Использовать `CodecDevin`,
   `ProjectComponentMutation`, `CompleteNativeConfigActivation` и managed stdio launcher.
   Direct component config не требует `devin plugins install`/cloud auth lifecycle.
2. Canonical original HOME/XDG/APPDATA и selected executable/version/profile должны
   совпадать в planner, native effect, receipt и fresh-process replay. Global mutable
   data принадлежат user environment; Project A/B data не разделяются с ним.
3. Проверять legacy main-config `mcpServers`, source aliases/imports и приоритетные
   project/local settings; UAP не запускает native migration, не меняет trust/team
   policy и не заставляет Global побеждать разрешённый Project override.
4. Старый Cascade user receipt остаётся на старом codec/root, даже если теперь
   установлен Devin. Fresh-process repair использует записанный route и прежнюю
   source policy; exact owned remove остаётся offline. Новый version gate не
   блокирует удаление уже owned записи и не делает её сетевой.
5. Generic activation attestation не доказывает загрузку. До genuine observation
   точные установленные components остаются Prepared с конкретным remaining action.

Atomic main-owner request: bounded selection/profile propagation и metadata label
для qualified Devin route, необходимые lifecycle/native-effect traits. Нельзя менять
весь `windsurf` user SkillSupport на безусловный native: legacy prepared-only route
сохраняется. Signed/normalized/site prerequisite должен назвать Devin profile.
Central patch интегрируется с leaf отдельно от большой #707 серии; Directory grant
и публичный rollout остаются у main owner после квалификации.

Central request применяется integrator в ТОТ ЖЕ leaf PR, что и adapter code;
в один проверенный merge result входят реализация и нужные traits. Leaf worker не
редактирует shared/central files параллельно, но они могут входить в его PR через
integrator. Ранний отдельный capability-only commit на main запрещён.

## Lean проверки

Переиспользовать `TestProjectDevinPublicSDKStdioDataRepairRestartAndRemove`,
`TestProjectDevinComposedSourceLifecycle`, current version/admission/receipt/policy
cases и существующие Windsurf native collector/cycle. Не писать второй 30×30 harness.

| Новая assertion | Самая маленькая проверка |
| --- | --- |
| Exact selected Devin profile/root; missing/unknown version, legacy executable, coexistence и wrong XDG/APPDATA не дают новый native route | Таблица поверх existing admission fixtures |
| Global owned Skill/MCP update/repair/remove, old Cascade receipt replay; A/B/foreign и legacy root byte-preserved | Один scope-parameterized facade lifecycle case |
| Genuine Global Skill виден из A/B; stdio+HTTP initialize/list/call в Global/Project; restart и exact-source removal | Один mixed native cycle на existing collector, общий inert probe |

Один mixed package: один Skill + stdio/HTTP servers с отдельными nonces/data roots.
Проверить положительный source и один same-name override/refusal control, не полный
перебор policy. Profiles/paths тестировать на заявленных платформах; native claims
привязать к действительно запущенным OS/version cells. Новый общий HTTP helper
считается один раз у integrator. Нет OAuth/SaaS matrix или нового scripted provider.
Если отсутствует уже поддержанный способ actual client-driven call, D0 остаётся BLOCKED;
сокращённый test budget не разрешает объявлять tools/list либо чужой RPC вызов PASS.

## Бюджет и выход

| Работа | Handwritten changed/new LOC |
| --- | ---: |
| Production qualified Global profile/components/lifecycle | 650–1 300 |
| Profile/root/legacy replay facade assertions | 220–450 |
| Genuine native source/transport/call assertions поверх existing cycle | 180–400 |
| **Новые tests/fixtures** | **400–850** |

Confidence 6/10. Это replacement прежних 1 000–2 200 tests за счёт parameterization,
не сокращение native acceptance. Central/shared production остаётся общим бюджетом;
не считать его, existing tests/reruns, docs или evidence logs дважды.
DoD leaf: qualified Devin Global и сохранённый Project/legacy lifecycle, реальный
source+stdio/HTTP calls в обоих scopes; отрицательные profile/ownership checks проходят.
Законченный commit сразу push в отдельный draft PR; никакого release authorization.
Следовать [owner scope](../PROJECT_INSTALL_OWNER_SCOPE.md) и preservation; workers/reviewers
GPT-6.1-sol xhigh. ChatGPT/VS Code и Cline/Kimi/Kiro/Grok этот leaf не меняет.

## Проверка плана

Два независимых направления (Registry/authority и integration/CI) прочитали весь
leaf в каждом из двух раундов. Автор отдельно сделал self-check; он не считается
независимым reviewer. Root согласовал результат с main 6+3 планом.

- Round 1 SHA-256: `1ec1fabaf49cd8013aea6220866a1af7c92c564b56810e77e3c0879bfcb3e36b`.
- Round 2 SHA-256: `e027216a31775489fd59b4f28eb3aff64156659d2d50b5d5c62b40bce77284cc`.
- Round 2: Registry PASS 9/10; integration PASS 9,1/10; открытых findings нет.
- Same-leaf central patch и текущий xhigh footer уточнены до второго round.

После review изменены только этот журнал и статус документа. Review принимает
план, не объявляет реализацию, native E2E, merge либо release завершёнными.

## Owner scope revision — 2026-11-05 (supersedes runtime-call merge requirement)

The owner explicitly accepts genuine **installation and discovery** as the
mandatory E2E for this leaf. Authenticated model sessions and client-driven
MCP `initialize` / `tools/list` / `tools/call` are optional **runtime**
qualification, not an install-only merge gate. No authorization bypass is used.

The required hosted source-built UAP + hash-verified Devin cycle is:
install the mixed Global Skill + stdio + HTTP package in an isolated original
profile; have **real `devin skills list --json` and `devin mcp list`**
confirm all three UAP-written names independently in A and B; remove an owned
Skill and repair it; confirm native discovery again; remove the installation
and confirm all three names disappear. Original Global/Project/Cascade and
foreign configuration must survive byte-identically. Exact owned receipt,
profile and source identities must be validated by existing paths.

This was directly observed at HEAD `2108d60d22da527ffcbc25a231036d55699fa2ff`,
GitHub Actions run `42067857378`, artifact `13435429930`: eight A/B
native inventory checkpoints passed, actual UAP add/repair/remove all exited
0 and the original components remained. That run was marked red solely because
the previous workflow incorrectly made model MCP calls required. The revised
workflow gates only the above genuine installation/discovery cells and
retains `runtime_mcp_calls_qualified=false` and
`qualified_product_e2e=false` when vendor auth/model execution is absent.
Source/configured does not imply connected/tool-executed.

After independent code reviews, applicable exact-final-HEAD CI and the
native source E2E pass, this narrowly qualified code may be squash-merged.
New signed/public runtime-capability claims and product release remain
separate. The earlier D0/runtime-call requirement in this file is historical
and is not a veto on this owner's narrower installation-only acceptance.
