# Global / Project: Skills и MCP для девяти агентов

Дата решения владельца: 2026-11-04. Статус: draft плана, до двух раундов независимого review.

Этот документ задаёт последующую реализацию. Наличие плана или draft PR не означает,
что новый CLI, подписанный Directory v2 либо новая совместимость уже выпущены.
Текущий запрос разрешает подготовить план, исправить его после двух раундов review,
сохранить его в основном draft PR и сразу публиковать каждый коммит этой ветки.

## 1. Результат и границы scope

Приоритет: один совместимый пакет, включая его обязательные зависимости, можно
установить в пользовательское окружение (`--scope user`, Global) либо в выбранный
проект (`--scope project`, Project). В обоих режимах должны доставляться Skills и
MCP; запись конфигурации без подтверждённого обнаружения клиентом не закрывает цель.

| Статус программы | Клиенты | Новая работа |
| --- | --- | --- |
| Active | Claude Code, Codex, OpenCode, Cursor, Gemini CLI, GitHub Copilot CLI, Qoder, Pi, Windsurf/Devin | Skills + MCP в обоих scopes; зависимости и lifecycle; проверенные разрешения каталога |
| Deferred | Cline, Kimi Code, Kiro, Grok Build | Только явный TODO в разделе 12; не расширять совместимость в этом этапе |
| Excluded | ChatGPT, VS Code | Не добавлять поддержку, отдельные адаптеры/конфиги/фикстуры либо новые compatibility claims |

Это актуальная приоритизация именно этой программы. Она заменяет прежний широкий
список целей для нового scoped Directory rollout. Существующие исходники,
установки, разрешения user scope и полезные тесты отложенных/исключённых клиентов
сохраняются; новая runtime denylist из этой таблицы не создаётся.

### Входит

- Совместимые Agent Plugins 1.0 пакеты со Skills, MCP stdio и Streamable HTTP.
- Skills-only, MCP-only, смешанные пакеты и обязательные dependency closures.
- Local и immutable Git источники, подписанный Directory; прямые источники
  остаются независимыми от наличия родительского пакета в Directory.
- Install, повторная установка, update, repair, remove, dry-run и восстановление
  по существующим `uap.toml` / `uap.lock`, в том числе на чистом компьютере.
- Изоляция user, Project A, Project B; сохранение чужих файлов и настроек.
- Scoped metadata, CLI authorization, зеркало каталога и выбор агентов на сайте.
- Документация изменённого пользовательского поведения и конкретных activation steps.

### Не входит

- Новые переносимые semantics для hooks, Apps, LSP, agents или vendor extensions.
- Универсальная поддержка SSE; существующие поддерживаемые маршруты сохраняются.
- Новые solver, lock format, transaction engine, plugin standard или signing service.
- Собственный MCP bridge, OAuth broker, облачный plugin lifecycle или background daemon.
- Feature flags, широкая архитектурная перестройка, замена стека, новые security providers.
- Ручная сертификация каждого найденного community GitHub-плагина и каждого SaaS-аккаунта.
- Удаление legacy implementation, массовая миграция пользовательских конфигов,
  изменение default scope или неявное повышение trust/permissions.

Неподдерживаемый обязательный компонент приводит к понятному отказу до mutation.
Допустимое отсутствие optional-компонента отражается явно. Нельзя ради успешной
установки молча выбрасывать обязательный MCP либо выдавать prepared-only Skill за active.

## 2. Проверенная исходная точка

- UAP: [`87b769a8f546b0bb1c25c4c34422f3f756995697`](https://github.com/777genius/universal-agent-plugins/commit/87b769a8f546b0bb1c25c4c34422f3f756995697).
- Registry: [`6212d52ece6bcfefebb35c1420dbe694b087e9f3`](https://github.com/777genius/universal-agent-plugins-registry/commit/6212d52ece6bcfefebb35c1420dbe694b087e9f3).
- Подписанный Directory v1 snapshot 43: 31 product, 39 distributions, 57 release policies,
  609 target rows; все rows имеют только user scope. 36 distributions active, 3 suspended.
- Последние active releases содержат 35 MCP-конфигураций: 29 Streamable HTTP и 6 stdio;
  ещё один active distribution — Skills-корень Marketing Kit. Это инвентаризация
  закреплённых исходников, не доказательство выполнения всех SaaS-инструментов.
- Registry [issue #398](https://github.com/777genius/universal-agent-plugins-registry/issues/398)
  остаётся открытым. Новый Registry HEAD изменил SkillSpector, но не scope contract.
- UI PR [#702](https://github.com/777genius/universal-agent-plugins/pull/702) уже merged:
  Project/Global пример и Copy готовы; это не публикация проектных разрешений.

### Конкретные пробелы

1. V1 schema, publisher и Go validator требуют `scopes == ["user"]`; Go decoder
   отвергает unknown fields. Добавление project/Pi в прежний endpoint сломает старых читателей.
2. `DirectoryResolveLockedRestore` в `domain/directory_context7_preparation.go`
   явно требует `ScopeUser`; новая установка из project lock должна получить отдельный
   квалифицированный путь, сохранив exact source/current authorization checks.
3. CLI записывает hardcoded `directory-v1`; composition/dry-run привязаны к конкретному
   v1 client. Это нужно исправить через существующую границу, без второго resolver.
4. `directoryEligibilityTargets` использует unscoped backend siblings; проверить
   переход на существующее scope-aware правило для независимых project profiles.
5. Pi Global пока extension-only; проектные Skills/MCP уже реализованы.
6. Qoder Project — Skills-only preparation/import view; native import имеет общий кэш.
7. Windsurf Global оставляет Skills в prepared package. Для цели нужен отдельно
   квалифицированный Devin Global component route; legacy Cascade не становится им автоматически.
8. Генератор client capabilities сериализует user definition; сайту нужна явная scope dimension.

### Связанные изменения: переиспользовать после проверки

| Работа | Связь с планом | Правило интеграции |
| --- | --- | --- |
| [UAP #698](https://github.com/777genius/universal-agent-plugins/pull/698) | Live audit, provenance, bootstrap, foreign-neighbor/removal helpers | Переиспользовать применимые helpers; не создавать второй 30×30 harness |
| [UAP #704](https://github.com/777genius/universal-agent-plugins/pull/704) | Claude profile isolation, Codex env, shared Skill collision, crash recovery | Не дублировать исправления и shared-file edits; брать только проверенный exact HEAD |
| [UAP #705](https://github.com/777genius/universal-agent-plugins/pull/705), [#706](https://github.com/777genius/universal-agent-plugins/pull/706) | Архитектурные правила и existing size guardrails | Читать актуальные правила; не строить новый framework линтеров |

Эти PR при исследовании были открыты и двигались. Их обсуждения/исторические артефакты
не считаются final-head PASS. Перед пересекающимся slice обновить HEAD, прочитать diff
и релевантные checks. Непересекающаяся работа не обязана ждать весь сторонний PR.

## 3. Инварианты пользовательского поведения

1. Scope выбирается явно в примерах и сохраняется для всей mandatory closure.
   Существующая семантика аргументного/безаргументного install не меняется этим проектом.
2. Global использует исходный user profile; Project использует выбранный canonical root.
   Нельзя подменять HOME или выдавать каталог проекта за новый пользовательский профиль.
3. Один и тот же пакет может иметь независимые состояния user/A/B. Shared immutable
   cache допустим; mutable data, receipts и право удаления принадлежат своему окружению.
4. Удаление или repair A не изменяет user/B/foreign neighbors. Изменённые пользователем
   owned artifacts проходят существующую conflict policy; force-overwrite не добавляется.
5. Все обязательные дети отдельно проходят source, client и scope authorization.
   Git-origin корня не отменяет Directory authorization его Registry dependencies.
6. Нативный приоритет конфигураций сохраняется. Если project/local policy затеняет
   Global, диагностируется фактически выбранный источник; override не переписывается.
7. Prepared, native discovery, MCP initialize/list/call и OAuth остаются разными claims.
   Trust, sign-in или restart описываются конкретным remaining action.
8. Shared changes сохраняют baseline deferred/excluded клиентов без новой qualification.

## 4. Минимальный контракт Directory v2

### 4.1. Wire и domain

Новый подписанный endpoint: `/registry/schemas/2/`. V1 endpoint, schema, decoder и
signature domain сохраняют прежнее поведение. Версия Directory не меняет Agent Plugins 1.0.

В policy immutable release одна target grant на `(client, scope)`:

| Поле grant | Назначение |
| --- | --- |
| `client` | Существующий ClientID; Pi добавляется только в v2 |
| `scope` | Одно значение user либо project |
| `minimum_installer_version` | Минимальная версия для этого grant; учитывает исходный release floor |
| `delivery`, `authentication` | Поведение и оставшиеся шаги именно этого grant |
| `current_evidence` | Evidence IDs этого client/scope/qualified native profile |
| `app_binding`, если уже существует | Только сохранение эквивалентного legacy user-контракта |

Уникальность: `(distribution_id, release_sequence, client, scope)`. Не объединять два
scope в один массив с общими evidence/floor. Package repository, full revision, path,
release sequence, tree/manifest digests и component identity остаются существующими.

Два строгих wire decoder нормализуют проверенные данные в один существующий domain
eligibility path. V1 target становится user grant с прежними условиями. V2 target
имеет явный scope. Никакого `DirectoryResolverV2` или нового dependency graph engine.

### 4.2. Evidence и closure

- Client evidence связывает scope, exact package/release/tree, installer/adapter/native
  versions, platform и реально проверенный outcome. Schema evidence остаётся отдельным.
- Qualification полного дерева связывается с exact dependency identity. Повторно
  используем существующий resolver/identity calculation; браузер дерево не разрешает.
- User evidence не квалифицирует Project. A/B/user fixture имеет независимые canonical
  roots; реальные абсолютные пути пользователя не публикуются в каталоге.
- Сайт получает подписанный результат совместимости closure. CLI всё равно повторно
  авторизует каждый выбранный child при install/update/restore.
- Наличие ровно одного неподдержанного required child запрещает новую compatibility claim.

### 4.3. Кэш, receipts, lock и публикация

- Wire/signature identity, bootstrap, cache key и high-water marks разделены по protocol.
  V1 sequence 43 и v2 sequence 1 не сравниваются как одна последовательность.
- Receipt `SnapshotSchema`, sequence и digest заполняются фактически проверенным источником.
  Immutable package pins и schema `uap.lock` не меняются ради нового policy protocol.
- Новая v2 project операция не откатывается к v1/user при любой ошибке. Для обычных
  новых v2 выборок не добавлять автоматический downgrade; legacy v1 чтение имеет
  явный compatibility purpose и не выдаёт новые project/Pi grants.
- Существующие signature, key revocation, expiry, digest, redirect/size, rollback,
  equal-sequence conflict и concurrent cache checks сохраняются.
- Publisher формирует v1 user view и v2 view из одного reviewed source model,
  используя существующие signer, immutable ledger и CAS promotion. Release identities
  не перевыпускаются только ради добавления scope.
- Согласованно публиковать suspension/revocation. Проверить partial publication,
  повтор promotion, independent expiry/refresh. V1 продолжает обновляться, а не
  навсегда замораживается на истекающем snapshot.
- Dry-run использует существующую staged clone/commit границу и не изменяет рабочие
  manifests, locks, receipts, native config и authoritative cache.

## 5. Реализация по рабочим пакетам

| ID | Работа и основные места | Зависит от | Критерий завершения |
| --- | --- | --- | --- |
| P0 | Перечитать AGENTS/owner scope, обновить overlap PR heads; зафиксировать v2 positive/negative examples и host prerequisites | Этот план и review | Wire/normalization/authorization договорены; нет незаявленного клиента или новой engine abstraction |
| P1 | Registry `schemas/`, `scripts/directory_publication.py`, preparer/CAS/readiness, version-specific profiles | P0 | V1 preserved; v2 scope/evidence validated; exact sources/old policies сохранены |
| P2 | UAP `adapters/directoryv1/` и тонкий v2 decoder; shared verification/cache через текущие границы | P0 | Оба формата проверяются независимо и нормализуются в один domain path |
| P3 | `domain/directory_*`, `agentpluginscli/source.go`, `deps_sync.go`, `compose.go` | P2; P1 fixtures | Actual protocol identity, per-child scope, fresh project locked restore, scope-aware siblings, mutation-free dry-run |
| P4 | Qoder Project: native component ownership вместо обязательного native import | P0; существующий kernel | Skills и оба MCP transport обнаруживаются; native shared import cache не присваивается UAP |
| P5 | Pi Global: Skills/MCP через переиспользуемое component ownership | P0; существующий kernel | User Skills/MCP работают; native extension route и Project сохранены |
| P6 | Devin Global profile, Skills/MCP; legacy Windsurf preservation | P0 host/profile gate | Доказаны global native discovery и lifecycle; scope/profile/root receipt согласованы |
| P7 | `cmd/agentplugins-registry-mirror`, landing codecs/types, capabilities generator и selection UI | P0 fixtures; P1–P3 contract | Одинаковые client/scope/closure решения CLI и сайта; реальный catalog gate не заменён demo bypass |
| P8 | Marketing Kit closure и остальные active catalog cases; controlled native probes | P1–P7 | Применимые assertion outcomes на exact head, qualified binary/profile и preserved user/A/B |
| P9 | Документация, independent implementation reviews, release rehearsal и coordinated rollout | P8 | Все заявленные targets подтверждены; публикация только по отдельному release authorization |

### P4: Qoder

Использовать существующие shared native component/config операции и текущие проектные
Skills paths после exact native проверки. MCP писать в существующий допустимый project
config (`.qoder/settings.json` либо `.mcp.json`) по одному детерминированному правилу.
Обнаруживать конфликтующие `.qoder/settings.local.json`, flags и foreign ownership.
Проверять effective source, сохранение пользовательских записей и точное удаление.
Старый manual import path сохраняется; импортированный самим Qoder общий cache не
становится owned только потому, что исходный bundle подготовил UAP.

### P5: Pi

Переиспользовать `project_components.go`, shared component mutation и lifecycle для
user roots `.pi/agent/skills` и `.pi/agent/mcp.json` с учётом реального native profile.
Не менять семантику extension route, не строить MCP bridge. Обновить user capability
лишь вместе с реализацией и native evidence. Project trust остаётся решением хоста.

### P6: Windsurf / Devin — обязательный отдельный gate

Сейчас Global route — legacy Cascade MCP + prepared-only Skills. Поэтому прежний
запас `0–180` production LOC не покрывает поставленную цель. Нужен Devin Global route.

Официальные native пути: Global Skills `~/.config/devin/skills`, MCP
`~/.config/devin/mcp_config.json`; Windows использует `%APPDATA%/devin`. Project —
`.devin/skills` и `.devin/mcp_config.json`. XDG/APPDATA authority берётся из исходного
пользовательского окружения. Эти пути не объявляются путями старого Windsurf IDE.

До записи кода подтвердить выбор одного executable/version/profile при coexistence
`windsurf` и `devin`. Сегодня detection предпочитает windsurf, а ConfigRoot относится
к legacy channel. Нельзя молча менять этот приоритет, переиспользовать Cascade root
как Devin root или переносить receipts между профилями. Использовать существующий
selection contract; если он не выражает точный профиль, переоценить только этот участок.

Переиспользовать `CodecDevin`, `devinComponents`, `devinDesiredServers`, shared ownership
и native activation checks. Не добавлять облачный `devin plugins install` lifecycle.
Точная наблюдавшаяся версия — `3000.11.205`; документация новой версии сама по себе
не расширяет admission. Проверить main-config migration, local/project overrides,
legacy imports и отсутствие mutation legacy Cascade. UAP сам миграцию не запускает.

Global Skill и MCP должны обнаруживаться реальным Devin из двух чистых проектов.
Проверить Skills source, stdio/HTTP initialize/list/call, global update/remove и
сохранение A/B/foreign/legacy state. Если native gate не проходит, девятый target
остаётся BLOCKED; нельзя заменить его prepared-only успехом или молча исключить из DoD.

## 6. Параллельная работа без конфликтов

Доступная модель выполнения: один интегратор и три workers. Это четыре параллельных
направления после короткого согласования контракта; девять адаптеров не требуют
девяти независимых переписанных установщиков.

| Владелец | Исключительная область записи | Что передаёт соседям |
| --- | --- | --- |
| Integrator | Этот план, final integration, mirror/landing/UX, итоговый evidence index | Contract examples, решения по спорным findings, exact integration SHA |
| Worker A | Registry schemas/publisher/readiness/policy/evidence data | Signed fixtures и versioned publication contract |
| Worker B | UAP readers/cache/domain/CLI/composition, central capabilities | Verified Directory facade, grant selection и actual protocol identity |
| Worker C | Qoder/Pi/Windsurf adapter files и их native ownership cases | Capability deltas Worker B; qualification results Integrator |

Сначала P0. Затем A ведёт P1, B — P2/P3, C — P4/P5/P6, Integrator — P7 на
зафиксированных fixtures. Внутри C независимые adapter slices идут по готовности;
освободившийся worker может взять следующий slice после явной передачи владения.
Для review временно освобождать worker slot; не считать пятого исполнителя доступным.

Не редактировать параллельно `clients_catalog.go`, `project_policy.go`, composition
или `clients/shared`. Shared change оформляется как конкретный запрос владельцу,
без generic framework и без расходящихся копий helper. Integration/merge/release
операции выполняет один владелец последовательно.

## 7. E2E: проверять утверждения, переиспользовать инфраструктуру

### Общие gates — один набор

- Замороженный released CLI 0.1.95 читает обновлённый v1 и сохраняет user lifecycle.
- V2 positive и negative wire/signature/schema/floor/expiry/revocation/cache cases.
- Scope mismatch, отсутствующий root/один required child, wrong source/tree/lock,
  неподдерживаемый компонент/transport, stale evidence: отказ до mutation.
- Fresh project restore из lock с пустым receipt/cache; exact retained source reuse;
  запрет moving-ref refresh или project-to-user downgrade.
- Существующие dependency graph/conflict/transaction/crash tests повторно используются.
  Добавляются только отсутствующие scope/closure assertions, без девяти копий solver harness.

### Каждый active client и оба scopes

Использовать genuine pinned binary и независимые user/A/B roots. К каждой проверке
привязать source HEAD, native binary/version/profile, OS/architecture и exact closure.

1. Install Skills-only, MCP-only и mixed fixture; проверить native effective source.
2. Install Marketing Kit с тремя обязательными зависимостями, repeat и cold restore.
3. Update/repair, restart и remove: сохранить другие scopes и foreign neighbors/data.
4. Проверить одинаковые имена и native precedence. Не требовать, чтобы Global побеждал
   более приоритетный Project/local override; проверять предусмотренный host результат.
5. Подтвердить stdio и controlled Streamable HTTP: initialize/list/call, где именно
   эти claims публикуются. Stub executable и одно наличие файлов не дают native PASS.
6. Для новых Qoder Project, Pi Global, Devin Global — дополнительные ownership,
   profile selection, source-path, edited-owned-content и removal cases.

Платформы: переиспользовать существующие Linux/macOS/Windows lanes. Новый native claim
привязан к реально проверенной платформе; platform-dependent пути новых adapters
проверить на всех заявленных платформах. Не умножать весь SaaS-каталог на каждую OS.

### Marketing Kit и остальной каталог

Root `777genius/marketing-kit@0.1.0` — Skills; required children: Notion HTTP,
HubSpot CRM HTTP, Playwright Bridge stdio. Qualification должна охватывать полную
closure, а не только root. Не писать Marketing-specific policy bypass.

Для остальных active compatible distributions использовать общий case generator
по exact signed source. Сохранять suspended/revoked state; не оживлять такие пакеты
ради фразы «все». Required unsupported cases остаются отказами с причиной.

Контролируемые MCP fixtures доказывают клиент/transport/scope. Реальный SaaS OAuth
проверяется отдельным opt-in smoke при наличии disposable account и разрешений.
Отсутствие такого аккаунта — NOT_RUN/BLOCKED для OAuth claim; оно не подделывается
fixture success и не обнуляет отдельно доказанную доставку/обнаружение конфигурации.

### Сайт и выпуск

- Совпадают agent/scope/closure eligibility CLI и сайта на положительных и отрицательных fixtures.
- Выбор Global/Project, команды, Copy, подписи, обе темы и mobile проверяются существующим Landing E2E.
- Новые публичные команды согласованы с опубликованным CLI; demo #702 остаётся иллюстрацией.
- Зелёные applicable CI и independent review относятся к exact final integration HEAD.
- После разрешённой публикации: protected publisher, immutable read-back обоих feed,
  released CLI smoke, Pages и browser production smoke. Без них release status не completed.

## 8. PR, коммиты и порядок публикации

Основной draft PR находится в UAP. Сначала в нём план, owner-scope ссылка и результаты
двух review rounds. Последующие UAP изменения могут продолжать эту же ветку.
Каждый законченный commit сразу push; не накапливать локальную неотправленную серию.

Registry находится в другом репозитории: его implementation требует отдельного linked
PR. Это связанный deliverable основной программы, а не попытка включить два repo в
один физический PR. Пустой Registry PR только ради ссылки сейчас не создаётся.

Порядок будущего rollout: implementation/review в обоих repo; release rehearsal на
зафиксированных v2 fixtures; совместимая публикация reader CLI; qualified v2 grants и
регулярно обновляемый v1; переключение реальных site selectors; production read-back.
Если выпуск CLI и первый v2 snapshot требуют согласованной bootstrap последовательности,
P0 фиксирует её до implementation; публичный install не должен попадать в промежуточный
неработающий путь. Плановый draft сам по себе не разрешает публичный release.

Не удалять опубликованные immutable artifacts при rollback. Сохранить последний
безопасный reader/v1 path; отозвать ошибочный grant новой policy publication по
существующим правилам. Project failure не лечится установкой в user scope.

## 9. Оценка строк кода

Это engineering estimate, не измеренный diff. Считаются добавленные и существенно
изменённые handwritten строки один раз, не additions+deletions. Сгенерированные feeds,
evidence dumps, locks, binaries, vendored code и переводы не включены. Уже существующие
тесты повторно запускаются; они не считаются заново написанными.

| Активный адаптер | Реализация | Тесты/fixtures | Содержание |
| --- | ---: | ---: | --- |
| Claude Code | 0–120 | 350–900 | Остаточные native MCP/profile/precedence проверки; reuse #704 |
| Codex | 0–100 | 250–650 | Scoped dependency regressions; reuse env/ownership fixes |
| OpenCode | 0–180 | 400–1 000 | Существующие V1/V2 profiles; не расширять несовместимые provider routes |
| Cursor | 0–100 | 250–650 | Effective discovery/call/activation boundaries |
| Gemini CLI | 0–80 | 150–400 | Существующий qualified route; targeted regression |
| Copilot CLI | 0–200 | 250–700 | Config source/precedence и scoped closure; не открывать shared-family commands |
| Qoder | 650–1 400 | 1 000–2 200 | Native Project Skills/MCP |
| Pi | 500–1 000 | 900–1 800 | Native Global Skills/MCP |
| Windsurf/Devin | 650–1 300 | 1 000–2 200 | Новый Devin Global profile/Skills/MCP; legacy preservation |
| **Адаптеры, расширенный allowance** | **1 800–4 480** | **4 550–10 500** | |

Нулевые нижние границы означают существующий путь. Верхние включают локальное hardening
и оставшуюся native qualification, а не обязательную перепись каждого адаптера.

| Общая работа | Реализация/schema/workflow | Тесты/harness |
| --- | ---: | ---: |
| Scoped v2 schema и semantic validators | 350–650 | 450–800 |
| Prepare/sign/CAS и поддерживаемая co-publication v1 | 550–1 000 | 700–1 300 |
| Scoped evidence/readiness | 450–850 | 900–1 700 |
| Go readers/cache/bootstrap/floors | 450–850 | 800–1 400 |
| CLI identity/children/lock/repair/dry-run | 450–800 | 800–1 400 |
| Reusable catalog closure qualification | 150–350 | 350–700 |
| Mirror/landing/scope capabilities/selection | 600–1 100 | 800–1 500 |
| **Общая часть** | **3 000–5 600** | **4 800–8 800** |

| Бюджет | Реализация | Тесты | Сумма |
| --- | ---: | ---: | ---: |
| Shared + три реальные feature gaps (Qoder/Pi/Devin) | 4 800–9 300 | 7 700–15 000 | **12 500–24 300** |
| С остаточной qualification/hardening всей активной девятки | 4 800–10 080 | 9 350–19 300 | **14 150–29 380** |

Вторая строка включает первую. Shared учитывается один раз. Marketing Kit marginal
0–100 production +200–450 scenario/test LOC уже внутри общей суммы. Дополнительно
150–450 handwritten policy-data LOC; они не являются новым executable subsystem.
ChatGPT, VS Code и четыре deferred adapters в эти суммы не входят.

Ранний механический пересчёт без уточнения Windsurf давал 12 850–26 960 LOC.
Он заменён этой оценкой: подготовленные Global Skills не закрывают новое требование.
Devin spike/profile имеет confidence 6/10; при несовместимом selection/state contract
переоценить этот slice, не скрывать увеличение внутри общего refactor.

## 10. Риски и критерии остановки

Оценки по 10-балльной шкале субъективные; 10 означает высокую сложность/уверенность
соответственно, а не измеренную вероятность безотказности.

| Область | Сложность | Уверенность оценки | Как ограничить риск |
| --- | ---: | ---: | --- |
| V1/v2 policy/publisher/cache совместимость | 7/10 | 8/10 | Strict decoders, explicit protocol identity, legacy replay, revocation/partial-publication cases |
| Closure/lock restore/ownership | 7/10 | 8/10 | Existing solver/kernel, один reusable negative harness, user/A/B |
| Qoder Project | 7/10 | 6/10 | Direct owned components, effective precedence, не присваивать native cache |
| Pi Global | 6/10 | 7/10 | Reuse project components, preserve extension route |
| Devin Global | 7/10 | 6/10 | Genuine host/profile gate, native paths, original environment, preserve Cascade |
| Mirror/landing | 4/10 | 8/10 | Signed closure view, shared fixtures, существующий browser suite |

Если native API/выбор профиля не подтверждены, target остаётся BLOCKED с точной причиной.
Если unknown supported component требует нового runtime, вынести решение в явное
уточнение scope; не расширять portable standard. Ошибки реального host/OAuth не
маскировать patch-ами фикстур. Непрошедший active target не исчезает из итогового отчёта.

## 11. Definition of Done будущей реализации

- [ ] Все девять active клиентов имеют подтверждённые Skills и MCP user/project пути
      в явно заявленных native profiles/platforms; prepared-only storage не считается discovery.
- [ ] Совместимая mandatory closure, включая Marketing Kit, устанавливается в оба scopes.
- [ ] Cold lock restore, update, repair, remove и user/A/B/foreign preservation проверены.
- [ ] V1 старых клиентов продолжает обслуживаться; v2 не ломает их parser/expiry/authorization.
- [ ] Неподдержанные/отозванные/неразрешённые случаи отказывают до mutation с полезной причиной.
- [ ] Deferred/excluded baseline сохранён; новых claims для них нет.
- [ ] Два раунда plan review и независимое implementation review закрыли применимые findings.
- [ ] Applicable exact-head CI, native evidence и browser checks прошли; skip не выдан за PASS.
- [ ] Markdown/public docs и actual site selection соответствуют released CLI/feed.
- [ ] При отдельном release authorization выполнены protected publication и production read-back.

Последние publication пункты относятся к будущему выпуску. Текущий plan-only draft
готов, когда документ исправлен после двух review rounds, сохранён в GitHub и ссылки
проверены; это не выполнение checkbox-ов реализации.

## 12. Отложенные TODO

| TODO | Что сделать позже | Что является завершением |
| --- | --- | --- |
| Cline | Проверить process-wide `CLINE_MCP_SETTINGS_PATH`, selected launch, same-name user Skill shadow и ordinary IDE discovery | Реальный выбранный процесс читает проектные Skills/MCP; user/foreign state сохранён; ограничения описаны |
| Kimi Code | Перепроверить native versions, nearest-Git-root правило, user/project precedence и различия SSE | Exact supported profile + scope lifecycle/evidence; отдельные grants после квалификации |
| Kiro | Завершить authenticated native Project lane, переиспользовать [#686](https://github.com/777genius/universal-agent-plugins/pull/686) после review | Native source/discovery/MCP и user/A/B подтверждены; auth refusal не принят за PASS |
| Grok Build | Проверить bundled Project MCP и три Skills-only guards: scope, projection, preflight lifecycle | Genuine native user/A/B MCP source/collision/lifecycle доказаны; guard removal лишь после этого |

Deferred TODO не является скрытой зависимостью текущего этапа. ChatGPT и VS Code
исключены; возвращаются в отдельный scope только по новой приоритизации владельца.

## 13. Самопроверка и два раунда критики

Каждый formal round читает конкретную версию этого документа. Findings содержат
severity, место, сценарий ошибки и минимальное исправление. Integrator проверяет
основание каждого finding; необоснованное расширение scope отклоняется с причиной.
После исправлений коммит сразу push. Второй round проверяет обновлённый документ,
закрытие первого и новые регрессии. Если остаётся blocker, исправление проверяется
повторно; номер round сам по себе не означает принятие.

| Этап | Статус | Результат |
| --- | --- | --- |
| Self-check v1 | Выполнен до formal review | Scope9, deferred4/excluded2, двойной repo, reusable kernels, новый Devin Global gap и арифметика учтены |
| Independent round 1 | Pending | Architecture/registry, adapters/native acceptance, integration/estimate |
| Independent round 2 | Pending | Повторная проверка исправленной версии теми же независимыми направлениями |

## 14. Источники и обязательные правила

- [PROJECT_INSTALL_OWNER_SCOPE](../PROJECT_INSTALL_OWNER_SCOPE.md),
  [capability preservation](../AUTHORING_CAPABILITY_PRESERVATION.md),
  [project support matrix](../PROJECT_CLIENT_SUPPORT.md).
- [Registry guardrails](https://github.com/777genius/universal-agent-plugins-registry/blob/6212d52ece6bcfefebb35c1420dbe694b087e9f3/docs/ECOSYSTEM_GUARDRAILS.md).
- [V1 schema](https://github.com/777genius/universal-agent-plugins-registry/blob/6212d52ece6bcfefebb35c1420dbe694b087e9f3/schemas/directory-distribution.schema.json),
  [publisher](https://github.com/777genius/universal-agent-plugins-registry/blob/6212d52ece6bcfefebb35c1420dbe694b087e9f3/scripts/directory_publication.py),
  [snapshot43](https://github.com/777genius/universal-agent-plugins-registry/blob/directory-publication-ledger/registry/schemas/1/snapshots/00000000000000000043.json).
- [Locked restore guard](../../install/integrationctl/agentplugins/domain/directory_context7_preparation.go),
  [project policy](../../install/integrationctl/agentplugins/domain/project_policy.go),
  [capabilities generator](../../cli/cmd/genclientcapabilities/main.go).
- [Marketing Kit exact dependencies](https://github.com/777genius/universal-agent-plugins-registry/blob/c5bcedd4ecf12585a8a50401852e57a44891ebec/plugins/marketing-kit/uap.toml).
- [Qoder project MCP](https://docs.qoder.com/cli/mcp-reference),
  [Pi native MCP](https://pi.dev/changelog/releases/0.99.0).
- [Devin Skills](https://docs.devin.ai/cli/extensibility/skills/overview),
  [MCP configuration](https://docs.devin.ai/cli/extensibility/mcp/configuration),
  [precedence](https://docs.devin.ai/cli/reference/configuration/global-vs-local),
  [existing exact-version evidence](project-install-e2e/windsurf-devin-implementation-evidence.md).

Workers/reviewers используют GPT-6.1-sol high/xhigh согласно текущему owner scope.
Перед будущими edits перечитать AGENTS на новом HEAD. Никаких feature flags либо
ослабления existing safety/architecture gates ради зелёного результата.
