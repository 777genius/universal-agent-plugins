# Global / Project: Skills и MCP для девяти агентов

Дата решения владельца: 2026-11-04. Статус: план принят после двух раундов независимой проверки; реализация ещё не выполнена.

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
| `canonical_closure`, если рассчитана | Подписанный итог текущей fresh resolution; контракт ниже |
| `app_binding`, если уже существует | Только сохранение эквивалентного legacy user-контракта |

Уникальность: `(distribution_id, release_sequence, client, scope)`. Не объединять два
scope в один массив с общими evidence/floor. Package repository, full revision, path,
release sequence, tree/manifest digests и component identity остаются существующими.

Два строгих wire decoder нормализуют проверенные данные в один существующий domain
eligibility path. V1 target становится user grant с прежними условиями. V2 target
имеет явный scope. Никакого `DirectoryResolverV2` или нового dependency graph engine.
При нормализации сохраняются также package-wide/schema blocking evidence и release
status; перенос evidence на уровень grant не должен отфильтровать общий запрет.

Qualified native profile сохраняется в evidence, normalized eligibility и публичной
подписи. CLI проверяет реально выбранные executable/version/root через существующий
adapter admission. Devin-only evidence не разрешает legacy Windsurf или неизвестную
версию; сайт называет этот путь Devin и показывает prerequisite. Новый реестр профилей
для этого не вводится.

### 4.2. Evidence и closure

- Client evidence связывает scope, exact package/release/tree, installer/adapter/native
  versions, platform и реально проверенный outcome. Schema evidence остаётся отдельным.
- Для полного дерева вводится отдельный `closure_digest`: SHA-256 от детерминированно
  сериализованных client/scope, root identity, canonical exact pins и required edges
  уже проверенного resolved Lock/graph. Источники, revisions, paths, release sequences,
  tree/manifest digests входят в pins; HOME, секреты и mutable state не входят.
  Existing `deterministicDependencyIdentity` описывает runtime executable вроде `node`,
  а не граф. Его смысл сохраняется; он не используется как closure fingerprint.
- Повторно используем existing resolver и validated lock, добавляя только bounded
  canonicalization. Новый lock schema или browser resolver не нужен.
- User evidence не квалифицирует Project. A/B/user fixture имеет независимые canonical
  roots; реальные абсолютные пути пользователя не публикуются в каталоге.
- Optional `canonical_closure` на grant содержит `eligible`, `closure_digest` и
  `minimum_installer_version`; при `eligible=true` digest и effective floor обязательны.
  Scope/client/root release и snapshot authority задаются окружающим signed contract.
  Floor равен максимуму применимых floors выбранных узлов. Существующие delivery/auth
  display requirements агрегируются по required closure, а не только по root.
- Это итог policy eligibility текущей fresh resolution, а не runtime evidence.
  Evidence получает отдельный `closure_digest`; его authenticated artifact содержит
  или закрепляет существующий validated lock. Допустимы несколько current proofs с
  разными exact digests, пока их собственные pins/grants остаются разрешены.
- На каждой публикации publisher заново проверяет canonical closure и применимость
  текущих referenced proofs к grants, floors, component constraints, statuses/revocations;
  весь исторический evidence ledger заново не обрабатывается.
  Отзыв required child инвалидирует затронутый summary/proof. Простое появление более
  новой canonical версии не инвалидирует старую всё ещё разрешённую closure.
- Сайт читает только authenticated canonical summary. CLI повторно авторизует все
  фактически выбранные pins; старый valid lock не обязан совпадать с fresh digest и
  не наследует floor более новой canonical closure. Runtime proof применим лишь к
  собственному exact digest; отсутствие такого proof не становится новым запретом
  установки при успешной существующей per-node authorization.
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
- Одна coordinated publication подготавливает оба immutable snapshot/envelope и оба
  указателя latest; единица продвижения — один ledger commit и один existing CAS ref
  update. Не выдавать частично подготовленные views за coordinated success. Проверить
  повтор promotion, independent expiry/refresh и CDN stale read. V1 продолжает
  обновляться, а не навсегда замораживается на истекающем snapshot.
- Dry-run использует существующую staged clone/commit границу и не изменяет рабочие
  manifests, locks, receipts, native config и authoritative cache.

Для операций, которым существующая политика требует Directory read/authorization,
протокол выбирается до загрузки, без retry через другой protocol:

| Операция | Протокол и authority |
| --- | --- |
| Existing validated v1 user receipt: требующий Directory read lifecycle | Explicit v1 compatibility purpose, прежняя receipt/floor authority |
| Existing v2 receipt | V2 |
| Новый install или cold lock restore без receipt | V2, в том числе старые exact user pins; отсутствие receipt не даёт installed-state privileges |
| Project, Pi, новый Devin profile либо новый scope/client grant | V2 |
| Явное добавление v2-only target к v1 установке | Exact same pin заново авторизуется в v2; resulting operation пишет фактическую v2 provenance, не переписывая unrelated receipts |

Неуказанная protocol metadata в историческом состоянии читается только по уже
валидированному legacy interpretation. Ошибка выбранного protocol терминальна;
это не разрешение попробовать другой decoder или изменить scope.
Не добавлять сетевую зависимость в receipt/ownership-only remove или иные ранее
локальные операции. Удаление owned установки остаётся возможным при недоступном,
истёкшем или отозвавшем пакет каталоге; install authorization не является правом удаления.

## 5. Реализация по рабочим пакетам

| ID | Работа и основные места | Зависит от | Критерий завершения |
| --- | --- | --- | --- |
| P0 | Перечитать AGENTS/owner scope, обновить overlap PR heads; зафиксировать v2 examples; сразу проверить Devin profile selection и native prerequisites всех active lanes | Этот план и review | Wire/normalization/authorization договорены; ранние host blockers известны до большой adapter работы |
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
| Integrator | Этот план, final integration, mirror/landing/UX, итоговый evidence index; `clients/shared/*` и требуемые `adapters/nativeconfig/*` изменения | Contract examples, минимальные shared fixes, решения по findings, exact integration SHA |
| Worker A | Registry schemas/publisher/readiness/policy/evidence data | Signed fixtures и versioned publication contract |
| Worker B | UAP readers/cache/domain/CLI/composition, central capabilities | Verified Directory facade, grant selection и actual protocol identity |
| Worker C | Qoder/Pi/Windsurf adapter files и их native ownership cases | Capability deltas Worker B; qualification results Integrator |

Сначала P0; Worker C проверяет Devin host/profile spike до последовательной P4/P5/P6 работы.
Затем A ведёт P1, B — P2/P3, C — P4/P5/P6, Integrator — P7 на
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
  неподдерживаемый компонент/transport, stale authorization evidence: отказ до mutation.
  Отсутствие matching runtime proof само по себе не запрещает установку (§4.2).
- Fresh project restore из lock с пустым receipt/cache; exact retained source reuse;
  запрет moving-ref refresh или project-to-user downgrade.
- Тот же root и тот же `node`, но изменённый required child pin; revoked/missing child;
  stale canonical summary; unchanged closure; старый valid lock при новой canonical
  closure/floor. Proof и eligibility должны изменяться только по своим входам.
- Existing v1 receipt lifecycle, fresh legacy user lock, v2-only added target,
  unknown legacy protocol metadata и выбранный feed load failure: explicit routing.
- Offline remove собственных v1/v2 установок при expired/revoked/unavailable feed:
  existing ownership policy действует без новой обязательной загрузки Directory.
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
5. Минимальный runtime gate программы: genuine admitted client/profile выполняет
   controlled stdio и Streamable HTTP initialize/list/call в обоих scopes. Existing
   oracles переиспользуются. Неподтверждённая ячейка остаётся BLOCKED и не закрывает
   target; это не девять новых harness и не SaaS OAuth проверка каждого пакета.
   Stub executable, только tools/list или наличие файлов не дают runtime PASS.
6. Для новых Qoder Project, Pi Global, Devin Global — дополнительные ownership,
   profile selection, source-path, edited-owned-content и removal cases.
7. Fresh-process repair/remove старых Qoder prepared-import, Pi extension-only и
   Cascade user receipts: dispatch по existing recorded physical profile/native
   object kind. Не мигрировать старую установку и не усыновлять Qoder cache неявно.
   Если persisted identity не различает routes, переоценить этот узкий gap до кода.
8. Devin-only evidence + selected legacy Windsurf/unknown version не даёт
   unconditional compatible/installed claim; проверить и CLI, и site labeling.

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

Основной draft PR: [UAP #707](https://github.com/777genius/universal-agent-plugins/pull/707).
Сначала в нём план, owner-scope ссылка и результаты
двух review rounds. Последующие UAP изменения могут продолжать эту же ветку.
Каждый законченный commit сразу push; не накапливать локальную неотправленную серию.

Registry находится в другом репозитории: его implementation требует отдельного linked
PR. Это связанный deliverable основной программы, а не попытка включить два repo в
один физический PR. Пустой Registry PR только ради ссылки сейчас не создаётся.

### Первый v2 bootstrap: две публикации Registry и один CLI release N

1. Подготовить reviewed implementation и выбрать будущую версию N по существующему
   release runbook, когда на выпуск есть authorization. До final UAP release freeze
   Registry публикует начальный signed v2 bootstrap с эквивалентными legacy user facts
   и обновлённым v1. Нет project/Pi/new Devin grants и новых closure/runtime/OAuth claims.
   Floor каждого bootstrap grant — `max(old floor, N)`; source/status/delivery/auth и
   исходные evidence identities сохраняются, old proof не переименовывается в proof N.
2. Bootstrap gate сохраняет все existing released-CLI v1 parity/readiness checks.
   Дополнительно проверяет strict v1-to-v2 semantic projection, actual signatures и
   pure v2 decoding на pinned source-built reader. Нельзя писать, что старый released
   CLI прочитал v2. Это ограниченная format-transition проверка, а не feature flag
   или отмена readiness. Bootstrap legacy user facts используют эквивалентную legacy
   eligibility semantics; новый native/closure proof из них не фабрикуется.
3. Включить authenticated v2 bootstrap в финальный UAP candidate N; заморозить tag
   на reviewed main commit. Выполнить существующие draft qualification, шесть native
   platform proofs и frozen receipt verification. При authorized promotion GitHub и
   npm публикуют те же проверенные bytes без rebuild. Действуют все environment gates.
4. Actual published N квалифицирует новые scoped grants active девятки. Registry
   публикует полный qualified v2, поддерживая v1. До появления новых grants CLI
   честно отказывает именно в ещё не разрешённых новых сценариях; baseline user
   authorization сохраняется, а сайт не обещает готовность этих новых сценариев.
5. Только после released-binary/full-feed read-back переключить реальные site
   selectors на v2 summaries и проверить production. Ранний demo #702 остаётся примером.

Первый v2 bootstrap уже существует к моменту freeze/publication N, поэтому нет
цикла «reader требует bootstrap, publisher требует выпущенный новый reader».
Нельзя пропускать финальную проверку actual embedded bytes или публиковать новый
compatibility claim на основании source-built bootstrap decoder.
Плановый draft сам по себе не разрешает публичный release.

Не удалять опубликованные immutable artifacts при rollback. Сохранить последний
безопасный reader/v1 path; отозвать ошибочный grant новой policy publication по
существующим правилам. Project failure не лечится установкой в user scope.

## 9. Оценка строк кода

Это engineering estimate, не измеренный diff. Считаются добавленные и существенно
изменённые handwritten строки один раз, не additions+deletions. Сгенерированные feeds,
evidence dumps, locks, binaries, vendored code, планы и пользовательская документация
(включая переводы) не включены. Документация остаётся deliverable P9; изменения journey
pages выпускаются во всех девяти поддерживаемых locales по AGENTS. Уже существующие
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
- [ ] Controlled native stdio и HTTP initialize/list/call подтверждены в обоих scopes
      для каждого заявленного active profile; ни одна обязательная ячейка не скрыта skip-ом.
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
| Self-check v1/v2 | Выполнен | Scope и суммы; exact source contract; no-cycle release; relative links и отсутствие implementation claims |
| Independent round 1 | Три review завершены; все findings закрыты во втором round | Два High contract findings и четыре acceptance/ownership/profile уточнения; перечислены ниже |
| Independent round 2 | PASS, три из трёх | Registry/зависимости: 9/10; adapters/native: 9/10; integration/оценка: 9,3/10. Новых обязательных исправлений нет |

Первый round читал все 468 строк commit `aa2b38a97c628fce6bdc8b42254c264c5347457b`
(SHA-256 файла `15473e3064ca9d666e77d033c9b4ba1d83ac390184615ba42a8003fe64cc1d91`).

| Finding | Исправление v2 | Результат второго round |
| --- | --- | --- |
| R1-R1 High: closure carrier/identity не определены | Canonical summary, отдельный closure digest, policy revalidation и old-lock semantics (§4.2) | CLOSED: fresh/old lock и child revocation согласованы; browser solver не нужен |
| R1-R2 High: bootstrap cycle оставлен на потом | Конкретные bootstrap gates, routing table и 2 Registry publications +1 CLI release (§4.3/8) | CLOSED: нет false release proof, downgrade либо missing-bootstrap window |
| R1-A1: Devin-only claim может расшириться до legacy Windsurf | Native profile prerequisite и отрицательный fixture (§4.1/7) | CLOSED: CLI/site qualification одинаково ограничены |
| R1-A2: новый route может перехватить old receipt lifecycle | Fresh-process replay трёх прежних receipt families (§7) | CLOSED: recorded ownership остаётся authority |
| R1-I1: MCP runtime floor был условным | Явный controlled native initialize/list/call gate и DoD (§7/11) | CLOSED: discovery не закрывает runtime |
| R1-I2: shared files без конкретного владельца | Integrator владеет shared/nativeconfig seam (§6) | CLOSED: нет двойных edits; #704 учитывается |

Второй round прочитал все 577 строк исправленного плана и 99 строк owner scope на commit
`83991acca42b8e9c9eaaf8a2a45b1d7759b59cf2`. SHA-256 плана:
`58b912cc51c0ded227beb8c30ba755244aba13f209d90e69ad9d8c7b20b7ef4f`;
owner scope: `5520fe6de93cb03fff72022ddbba77ba21e0658da56d01bd85d3092bb10b8e57`.
Оба hash проверены независимо. Оценки относятся к готовности плана, не к native PASS.

Дополнительно закрыты замечания о сохранении schema-wide blocking evidence,
единой CAS publication и явном исключении пользовательских docs из LOC. Финальная
редакция уточняет два уже принятых правила: повторно проверяются current referenced
proofs, а не вся история; stale authorization evidence отличается от отсутствующего
runtime proof. Других изменений implementation contract после второго round нет.

Самопроверка подтвердила арифметику всех таблиц, восемь относительных ссылок и точное
совпадение опубликованных plan/owner файлов с локальной версией, прочитанной критиками.
Future implementation DoD выше остаётся открытым; эти review не заменяют будущие
проверки кода, настоящих клиентов, релиза и production.

## 14. Источники и обязательные правила

- [PROJECT_INSTALL_OWNER_SCOPE](../PROJECT_INSTALL_OWNER_SCOPE.md),
  [capability preservation](../AUTHORING_CAPABILITY_PRESERVATION.md),
  [project support matrix](../PROJECT_CLIENT_SUPPORT.md),
  [release runbook](../agentplugins-release.md).
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
