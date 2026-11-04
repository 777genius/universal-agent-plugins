# Global / Project: сначала шесть клиентов, три адаптера отдельно

Дата решения владельца: 2026-11-04. Редакция 6+3 принята после двух раундов независимой проверки. Реализация ещё не выполнена.

Этот документ задаёт последующую реализацию. Наличие плана или draft PR не означает,
что новый CLI, подписанный Directory v2 либо новая совместимость уже выпущены.
Последняя инструкция разрешает реализацию и ранний merge основной части после
применимых проверок. Все PR при разработке — draft; каждый законченный commit сразу
push. Qoder, Pi и Devin ведутся в отдельных PR и не блокируют основную шестёрку.
Публикация релиза остаётся отдельным этапом с действующими release gates.

## 1. Результат и границы scope

Приоритет: один совместимый пакет, включая его обязательные зависимости, можно
установить в пользовательское окружение (`--scope user`, Global) либо в выбранный
проект (`--scope project`, Project). В обоих режимах должны доставляться Skills и
MCP; запись конфигурации без подтверждённого обнаружения клиентом не закрывает цель.

| Статус программы | Клиенты | Новая работа |
| --- | --- | --- |
| Main #707, первый merge | Claude Code, Codex, OpenCode, Cursor, Gemini CLI, GitHub Copilot CLI | Shared scope/Directory/dependencies + существующие пути шести клиентов; Skills/MCP и Marketing Kit |
| Отдельные parallel draft PR | Qoder, Pi, Windsurf/Devin | По одному адаптеру в PR; свои native gates, lifecycle и доказательства; main их не ждёт |
| Deferred | Cline, Kimi Code, Kiro, Grok Build | Только явный TODO в разделе 12; не расширять совместимость в этом этапе |
| Excluded | ChatGPT, VS Code | Не добавлять поддержку, отдельные адаптеры/конфиги/фикстуры либо новые compatibility claims |

Это актуальная приоритизация именно этой программы. Она заменяет прежний широкий
список целей и зависимость общего merge от девяти клиентов. Существующие исходники,
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
   допускает только `ScopeUser` для v1, но также `ScopeProject` для schema 2.
   Обе ветки обязаны сохранять exact source/current authorization checks.
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
протокол выбирается до загрузки, без retry через другой protocol. Таблица ниже
применяется к final v2 candidate N ПОСЛЕ explicit CLI release-preparation cutover.
До него production constructor/bootstrap и fresh user/legacy restore routing остаются
v1; main merge не добавляет обязательный v2 network/bootstrap dependency (§8).
Новые v2-only операции до cutover не рекламируются как выпущенные.

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

## 5. Основной PR и независимые адаптеры

### Основной #707: один согласованный набор для шести клиентов

| ID | Работа и основные места | Зависит от | Проверяемый выход |
| --- | --- | --- | --- |
| M0 | Уточнить exact heads/rules; закрепить v2 examples, six-client matrix и существующие oracles | Два review изменённого плана | Один контракт, известные baseline/native ограничения, нет зависимости от трёх leaves |
| M1 | Registry schemas, publisher, readiness и scoped policy/evidence | M0 | V1 preserved; signed v2/closure и paired CAS проверены в отдельном связанном Registry PR |
| M2 | UAP strict readers/cache/bootstrap, normalized domain eligibility | M0 | Оба формата нормализуются в один путь; прежний v1 decoder остаётся строгим |
| M3 | CLI composition, per-child authorization, project cold lock restore, receipt routing/dry-run | M2, M1 fixtures | Пакет и required closure устанавливаются в оба scopes через настоящий CLI; отказ до mutation |
| M4 | Mirror/landing codecs/types, scoped capabilities и selection | M0 fixtures; M1–M3 integration | CLI/site согласованы; production до cutover сохраняет v1/released commands (§8) |
| M5 | Шесть существующих adapters + Marketing Kit: небольшие assertions в существующих suites | M3/M4 | Changed-path install/update/repair/remove/restore E2E; честный native evidence inventory |
| M6 | Документация, independent code review, final-head CI и merge основного PR | M1–M5 code gates | Код merged после его gates; отсутствующие native/release claims остаются открытыми отдельно |

Ни один из M0–M6 не зависит от окончания Qoder/Pi/Devin. Новую core-функцию проверяем
в одной вертикали; не создаём шесть PR для одинакового изменения каталога/CLI.
Marketing Kit требует явного расширения root grant на Copilot: наличие разрешений
Copilot у трёх children не разрешает root автоматически.

### Три отдельных draft PR

| Leaf | Ветка от актуального main | Единственная новая возможность | План/PR |
| --- | --- | --- | --- |
| Qoder | `feat/qoder-project-components-20261104` | Native Project Skills + MCP, preserve prepared-import | [Draft #708](https://github.com/777genius/universal-agent-plugins/pull/708) |
| Pi | `feat/pi-global-components-20261104` | Generic Global Skills + MCP, preserve extension route | [Draft #709](https://github.com/777genius/universal-agent-plugins/pull/709) |
| Windsurf/Devin | `feat/devin-global-components-20261104` | Qualified Devin Global Skills + MCP, preserve Cascade | [Draft #710](https://github.com/777genius/universal-agent-plugins/pull/710) |

У каждого leaf собственный короткий Markdown-план, два plan review, code review и
свои native gates. Его branch не наследует всю незавершённую серию #707. После merge
общей границы leaf один раз обновляет базу. Central metadata/shared helpers вносит
единственный integrator в ТОТ ЖЕ leaf PR как минимальный atomic patch: adapter и
нужные traits входят в один проверенный merge result. Не выпускать capability-only
commit на main до реализации; владение shared files не означает отдельный ранний merge. Это не причина ждать другие leaves.

Local/Git native spike не требует Directory v2. Подписанный grant требует своей
доказанной реализации и фактически опубликованной версии CLI; детали в §8.

## 6. Параллельная работа и владение файлами

Доступны четыре слота: integrator и три исполнителя. Приоритет слотов — критический
путь основной шестёрки. Отдельные PR позволяют независимо готовить, проверять и
мержить leaves; обещание семи одновременно работающих исполнителей не делается.

| Владелец | Исключительная область записи | Передача результата |
| --- | --- | --- |
| Integrator | План/owner scope, `clients/shared/*`, нужные `adapters/nativeconfig/*`, native evidence reconciliation, итоговый merge | Contract examples, shared fixes, final SHA, bounded central requests |
| Worker A | Registry schema/publisher/readiness/policy/evidence | Signed fixtures и совместимая публикация; после M1 слот получает один leaf |
| Worker B | UAP readers/cache/domain/CLI/composition, central capabilities | Reader/grants/receipt integration; после M3 слот получает другой leaf |
| Worker C | Mirror/landing/scoped selection и существующий browser suite | Совместимый site consumer; после M4 слот получает третий leaf |

M1, M2/M3 и M4 стартуют параллельно после M0. Integrator параллельно проводит короткие
host/profile prerequisites и проверяет минимальные leaf requests. По мере освобождения
слотов Qoder, Pi и Devin ведутся одновременно, каждый в своей ветке/рабочем каталоге.
Если общий путь ещё требует помощи, он получает приоритет над очередным leaf slice.
Review занимает существующий слот; владелец файла передаётся явно, без двойных edits.

Не менять параллельно `clients_catalog.go`, `project_policy.go`, composition или shared
kernel. При пересечении #698/#704 — прочитать новый exact HEAD/diff/checks, принять
подходящий фикс один раз. Не блокировать всю работу из-за непересекающегося внешнего PR.
Workers/reviewers — GPT-6.1-sol xhigh по owner selection; локальные targeted checks
выполняются до commit, законченный commit сразу push.

## 7. Минимальные достаточные проверки

### Сначала использовать существующий тест

Перед новым test file назвать конкретный отсутствующий assertion и ближайший helper.
Добавлять scope/protocol/profile row либо небольшой helper в существующую suite;
не писать отдельный solver/crypto/transaction/lifecycle harness на каждого клиента.

| Existing suite | Что переиспользовать | Что действительно добавить |
| --- | --- | --- |
| `adapters/directoryv1/directoryv1_test.go`, `cache_concurrency_test.go` | Signed fixtures, fetch/signature/expiry/rollback, реальные cache races | Strict v2 dimensions и cross-protocol/cache/floor isolation |
| `domain/directory_locked_restore_test.go` | Existing policy mutation table | Qualified v2 project positive и v1 project rejection |
| `cli/internal/agentpluginscli/deps_fresh_restore_e2e_test.go` | Real Git graph, cold store, exact pins, no-effects helper | Scope/protocol parameter, missing/revoked child, changed closure |
| `directory_rollout_semantics_test.go`, `deps_snapshot*_test.go` | Receipt/source identity, late refusal/cleanup | Explicit v1/v2 routing, added target, offline ownership-only remove |
| Registry `tests/test_directory_publication*.py` | Signing/source fixtures и реальный Git/CAS | Two-view atomic commit, immutable readback и bootstrap projection |
| Registry `tests/test_catalog_publication_readiness.py` | Qualification identity и uncovered expansion rejection | Six main target grants/closure; separate leaf expansion later |
| Existing per-client facade/composed lifecycle and native collectors | User/A/B/foreign, source attribution, restart/remove | Только новые scope/closure/profile assertions; existing reruns не считать новым кодом |
| Existing Landing/browser suite | Themes/mobile/Copy/selection | Scope+closure matrix и сохранение текущего released path до cutover |

### Общие рискованные границы — один набор

- Замороженный released CLI 0.1.95 сохраняет v1 user behavior; strict v1 не принимает v2.
- Два composition assertions: pre-cutover source-built production CLI сохраняет fresh
  v1 user install/restore без v2 dependency; final candidate N использует exact
  authenticated v2 bootstrap, выбранный v2 failure терминален.
- V2 scope/schema/signature substitution, независимые floors/cache/high-water marks;
  existing expiry/revocation/equal-sequence/concurrent-cache tests переиспользуются.
- Один missing/revoked required child или wrong source/tree/lock запрещает mutation,
  даже если root из Git. Changed child при том же root и `node` меняет closure digest.
- Старый valid lock не обязан равняться новой canonical closure/floor. Отсутствующий
  matching runtime proof не превращается в новый install ban.
- Fresh Project restore с пустыми receipts/cache, exact pins без moving refresh;
  v1 receipt lifecycle, v2 added target, terminal selected-protocol failure и dry-run.
- Offline owned remove при expired/revoked/unavailable feed; scope A не меняет user/B/foreign.
- Paired publication через один ledger/CAS, incomplete stage/conflict/retry/readback;
  authenticated bootstrap без фиктивного нового native/closure proof.

### Main merge: проверяется реально изменённый путь

Использовать настоящий собранный CLI и существующие local/Git/Directory fixtures для
install → repeat → update/repair → cold restore → remove и no-mutation refusals.
Проверить six-client selection и root+required children Marketing Kit в обоих scopes;
компонентные source/config assertions различаются по клиенту, общий graph не копируется.
Один mixed fixture (Skill + stdio + HTTP) и небольшая таблица Skills-only/MCP-only
достаточны там, где existing builders уже выражают эти варианты.

Native evidence inventory фиксирует, какой настоящий profile/version/platform уже
проверен и какой outcome ещё BLOCKED. Два известных research case не являются green
oracles: `TestAgentpluginsClaudeNativeTrustedProjectMCPCall` квалифицирует foreign
config, а `TestAgentpluginsCursorNativeProjectRuntimeAttempt` намеренно завершает
capture с `t.Fatal`. Не переименовывать их в подтверждение UAP-managed MCP и не
назначать их новыми обязательными green gates без реальной недостающей реализации.

Merge безопасного общего кода проходит после changed-path E2E, independent review и
существующих applicable gates. Он не требует окончания трёх leaves и сам по себе
не добавляет неподтверждённые native grants или публичные обещания.

### Native qualification и новые compatibility grants

Для каждого нового заявленного client/profile/scope требуется genuine pinned binary,
exact source HEAD, platform и closure. Нативная проверка обязательна **для каждого
заявленного и поддержанного компонента**: Skill source — только если пакет объявляет
Skills; stdio и Streamable HTTP initialize/list/call — только для соответствующих
объявленных MCP-транспортов. Для applicable компонентов обязательны restart,
exact removal и user/A/B preservation. Skills-only не требует MCP, MCP-only не
требует Skill. Недоказанная applicable ячейка остаётся BLOCKED; Prepared/files/list
не заменяют actual call. Leaf имеет те же правила, но собственную матрицу и срок.

Новая матрица native qualification содержит шесть main клиентов; новые leaf features
квалифицируются отдельно. Existing applicable all-client/Pi Project/другие baseline
CI checks сохраняются без вырезания клиентов ради очереди. Не умножать
общие policy negatives или весь SaaS каталог на все agents/OS. Сохранять native priority:
Global не обязан перекрывать Project/local policy. Не добавлять OAuth broker, provider
framework или новые model harness ради неизвестного upstream поведения.

Marketing Kit root — Skills, children — Notion HTTP, HubSpot CRM HTTP, Playwright
Bridge stdio. Проверяется вся closure, без Marketing-specific bypass. Реальный SaaS
OAuth — отдельный opt-in smoke с разрешённым disposable account; без него нет OAuth
PASS, но отдельно доказанная доставка конфигурации сохраняет собственный статус.
Suspended/revoked packages не включаются ради заявления «все пакеты».

### Сайт

Новая логика scope/closure проверяется authenticated fixtures в существующем browser
suite, включая Copy, обе темы и mobile. Production до release cutover продолжает
проверенный v1 и команды актуального released CLI (§8); fixture feed не становится
production authority. После разрешённого cutover нужны actual full-feed/released
binary read-back, Pages и browser production smoke.

## 8. PR, очередь GitHub Actions и публикация

### Отдельные PR и merge порядок

Основной [UAP #707](https://github.com/777genius/universal-agent-plugins/pull/707)
содержит shared code, шесть простых клиентов, mirror/site и этот план. Registry —
другой repo, поэтому имеет один связанный draft implementation PR. Qoder/Pi/Devin —
три независимых draft PR из §5, без переноса общей серии или копий shared фиксов.

Сначала merge готовой основной части после её exact-head gates и post-merge проверки.
Затем каждый готовый leaf проходит собственный merge; один leaf не ждёт два других.
Документировать фактический native/release статус, а не приравнивать merged code к
опубликованной новой совместимости. Все source/baseline старых клиентов сохраняются.

### Работа с очередью без изменения gates

1. Draft при разработке. До commit — локальные targeted checks в изолированной ветке;
   каждый законченный commit немедленно push. Не создавать микро-коммиты на каждую
   assertion, не копить уже готовые commits и не использовать skip-ci.
2. Coherent main batches: contract/reader/domain → CLI authorization/restore →
   mirror/site/selection и docs. Independent Registry/leaf code проверяется параллельно.
3. Draft не выключает всё: Core fast и Landing могут работать; native workflow может
   сработать от изменения своего test path. Не переименовывать файлы для обхода filters.
4. Final docs/review disposition записать до freeze последнего code HEAD. Filters
   смотрят на cumulative PR diff: поздний docs-only push тоже перезапустит применимые
   workflow. Runtime receipts хранить в existing CI artifacts, не дописывать их новым
   code commit после успешной квалификации без необходимости.
5. Одна ready/full-CI волна сначала для main; final native matrices leaves запускать
   по мере освобождения очереди. Разработка leaves при этом остаётся параллельной.
6. Не дублировать активный PR run ручным dispatch: у manual runs отдельная concurrency
   group. Retry только реально failed/cancelled job с исправленной причиной; queued
   не является failure. Не отменять чужие workflows и не менять глобальные runner vars.
7. Существующий documented `ci:github-hosted` использовать адресно лишь при доказанной
   подходящей очереди; не подменять genuine physical-host admission или platform proof.

### Merge-safe CLI и сайт до релиза

Main добавляет v2 decoder, normalized policy и test fixtures. До отдельного CLI
release-preparation cutover production constructor/embedded bootstrap и normal fresh
user install/cold legacy restore используют прежний явный v1 path. Source-built CLI
после main merge не должен зависеть от ещё отсутствующего v2 bootstrap. V2 integration
проверяется через существующую explicit authenticated fixture boundary.

После actual initial signed v2 bootstrap publication отдельный CLI release-preparation
commit переключает production composition/default new-operation routing на v2 и
закрепляет actual embedded bytes ДО freeze candidate N. Это явное изменение release
composition, без runtime flags, availability probes либо try-v2-then-v1 fallback.

До более позднего site cutover production mirror/site явно сохраняет authenticated
v1 и текущие released-CLI commands.
Не пробовать отсутствующий v2 с тихим fallback, не вводить readiness feature flag и
не показывать Project/Pi/Devin claim по test fixture. Новые source-built integration
cases получают explicit signed fixture/bootstrap через существующую test boundary.
Переход production feed/schema и real selectors — отдельный release cutover commit
после actual published N + full qualified v2 + read-back. Это изменение версии
протокола в релизе, а не новый параллельный механизм настройки совместимости.

### Первый v2 bootstrap: две публикации Registry и один основной CLI release N

1. По существующему release authorization/runbook выбрать N. До final release freeze
   Registry публикует signed v2 bootstrap с эквивалентными legacy user facts и обновляет
   v1. Нет новых project/Pi/Qoder/Devin grants или вымышленных closure/runtime/OAuth claims.
   Floor grant — `max(old floor, N)`; source/status/delivery/auth/evidence сохраняются.
2. Все existing released-CLI v1 parity/readiness gates сохранены. Добавить bounded
   strict v1→v2 semantic projection, actual signatures и pinned source-built decoding;
   не писать, что старый released CLI прочитал v2, и не переименовывать старый proof в N.
3. После доступности actual signed v2 bootstrap выполнить CLI composition cutover
   выше и включить authenticated bytes в final candidate N; затем freeze. Frozen main tag,
   existing draft qualification, шесть native platform proofs, receipt verification,
   protected promotion и npm используют те же bytes без rebuild и без ослабления gates.
4. Actual published N квалифицирует новые grants основной шестёрки и полную Marketing
   closure, включая Copilot root. Полная paired v2/v1 публикация включает только
   действительно квалифицированные новые grants. При BLOCKED cell не выпускать
   неподтверждённую claim и не объявлять шестёрку полностью квалифицированной.
5. После full-feed/released-binary read-back выполнить production cutover сайта и smoke.
   Общий rollout не ждёт Qoder/Pi/Devin, их baseline policies остаются прежними.

Поздний leaf получает собственный grant floor по первой реально опубликованной CLI
версии, содержащей его adapter; автоматически ставить N нельзя. Каждый leaf добавляет
квалифицированные grants через existing paired publisher независимо от других leaves.
Ему не нужен третий wire format, второй resolver или повторное открытие общего PR.
Новая инструкция разрешает implementation/merge; публикация release требует своего
owner authorization по AGENTS, который не заменяется этим планом.

## 9. Пересмотренная оценка строк кода

Считаются gross added/meaningfully modified handwritten строки один раз, не сумма
additions+deletions. Existing test bodies и повторные прогоны не являются новым LOC.
Generated feeds/evidence/locks/binaries/vendor, планы, public docs и переводы исключены.
Документация изменённого поведения остаётся deliverable; journey pages обновляются
во всех девяти maintained locales. Число строк — оценка, не цель расходования бюджета.

| Основной клиент | Production LOC | NEW test/assertion LOC |
| --- | ---: | ---: |
| Claude Code | 0–120 | 50–140 |
| Codex | 0–100 | 30–90 |
| OpenCode | 0–180 | 50–140 |
| Cursor | 0–100 | 50–140 |
| Gemini CLI | 0–80 | 20–60 |
| GitHub Copilot CLI | 0–200 | 50–180 |
| **Шесть клиентов, сверх shared** | **0–780** | **250–750** |

Это thin rows/smokes в existing suites. Нулевой production минимум означает существующий
route; верхняя граница — локальный hardening, а не перепись адаптера. Unknown native
runtime gap не считается заранее решённым этим небольшим allowance (§7/10).

| Общая работа | Production/schema/workflow LOC | NEW tests/harness LOC |
| --- | ---: | ---: |
| Scoped schemas/semantic normalization | 350–650 | 200–350 |
| Prepare/sign/CAS/bootstrap co-publication | 550–1 000 | 400–650 |
| Closure/evidence/readiness | 450–850 | 400–700 |
| Go readers/cache/bootstrap/floors | 450–850 | 350–600 |
| CLI routing/children/lock/repair/dry-run | 450–800 | 350–600 |
| Reusable Marketing/catalog integration | 150–350 | 100–200 |
| Mirror/landing/scoped selection | 600–1 100 | 300–550 |
| **Shared, один раз** | **3 000–5 600** | **2 100–3 650** |

| PR / объём | Production LOC | NEW tests/fixtures LOC | Вместе |
| --- | ---: | ---: | ---: |
| **Main #707 + связанный Registry, шесть клиентов** | **3 000–6 380** | **2 350–4 400** | **5 350–10 780** |
| Qoder Project, отдельный PR | 650–1 400 | 400–800 | 1 050–2 200 |
| Pi Global, отдельный PR | 500–1 000 | 250–550 | 750–1 550 |
| Devin Global, отдельный PR | 650–1 300 | 400–850 | 1 050–2 150 |
| **Вся программа 6+3** | **4 800–10 080** | **3 400–6 600** | **8 200–16 680** |

Дополнительно 150–450 handwritten policy-data LOC. Marketing marginal 0–100 production
и 200–450 scenario/test LOC уже распределены внутри shared, не добавляются сверху.
Leaf totals не включают повторно общий HTTP probe, central metadata и Directory.
ChatGPT/VS Code и четыре deferred клиента не включены.

Предыдущие 14 150–29 380 total (9 350–19 300 tests) заменены этой оценкой. Снижение
обосновано existing signed fixtures, cold-restore/CAS helpers, параметризацией одного
mixed case и compact mutation tables (§7), а не удалением meaningful E2E. Shared
production диапазон сохраняется до измеренного diff; количество новых тестов может
вырасти только из конкретного выявленного риска, не ради coverage/LOC цели.

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

## 11. Раздельные критерии готовности

### Основной код и ранний merge

- [ ] Два review изменённого 6+3 плана закрыты; code review исправлен до отсутствия применимых findings.
- [ ] Shared v2/closure/CLI integration реализована для шести existing clients; Marketing root/children и Copilot root учтены.
- [ ] Changed-path CLI install/update/repair/remove/cold-restore E2E в обоих scopes проходит.
- [ ] V1/old-lock/old-receipt и offline removal сохранены; wrong/revoked/missing child отказывает до mutation.
- [ ] Main production site/commands сохраняют released behavior до отдельного cutover.
- [ ] Existing baseline всех остальных клиентов сохранён; leaf capabilities не включены преждевременно.
- [ ] Applicable exact final HEAD CI и review успешны; ready/merge и post-merge checks завершены.
- [ ] Новые Qoder/Pi/Devin leaf features/matrices/completion не входят в required main
      aggregate и не задерживают merge; existing applicable baseline checks сохранены.

### Новая публичная совместимость и release

- [ ] Шесть main client profiles имеют genuine native Skills и stdio/HTTP initialize/list/call в заявленных scopes/platforms.
- [ ] Marketing Kit полностью авторизован/квалифицирован в обоих scopes, включая root+required children.
- [ ] Actual released N, authenticated bootstrap, immutable promotion/npm и paired v1/v2 read-back подтверждены.
- [ ] Published signed grants и site selection содержат только реально доказанные claims; production smoke завершён.

### Каждый leaf отдельно

- [ ] Его два plan reviews и independent code review закрыты; scoped implementation, old-receipt/native gates выполнены.
- [ ] Свой exact-head CI, merge и post-merge checks завершены независимо от других leaves.
- [ ] Новые grants/public claims получают actual first supported released version и собственное native evidence.

Merged main code не закрывает автоматически public/native/release пункты. Заблокированная
native ячейка остаётся явно открытой; отдельные листья не превращаются в скрытую зависимость
основного PR. Полное завершение всей программы требует соответствующих outcomes всех её частей.

## 12. Отложенные TODO

| TODO | Что сделать позже | Что является завершением |
| --- | --- | --- |
| Cline | Проверить process-wide `CLINE_MCP_SETTINGS_PATH`, selected launch, same-name user Skill shadow и ordinary IDE discovery | Реальный выбранный процесс читает проектные Skills/MCP; user/foreign state сохранён; ограничения описаны |
| Kimi Code | Перепроверить native versions, nearest-Git-root правило, user/project precedence и различия SSE | Exact supported profile + scope lifecycle/evidence; отдельные grants после квалификации |
| Kiro | Завершить authenticated native Project lane, переиспользовать [#686](https://github.com/777genius/universal-agent-plugins/pull/686) после review | Native source/discovery/MCP и user/A/B подтверждены; auth refusal не принят за PASS |
| Grok Build | Проверить bundled Project MCP и три Skills-only guards: scope, projection, preflight lifecycle | Genuine native user/A/B MCP source/collision/lifecycle доказаны; guard removal лишь после этого |

Deferred TODO не является скрытой зависимостью текущего этапа. ChatGPT и VS Code
исключены; возвращаются в отдельный scope только по новой приоритизации владельца.

## 13. Самопроверка и критика

### История принятого общего контракта до разбиения 6+3

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
Эти первые review относятся к прежнему монолитному разбиению, сохранённому в commit
`73bf2d04904c52b5adb9782983c7481fa0242a56`. Они не подменяют проверку новой редакции 6+3.

### Два раунда новой редакции 6+3

| Этап | Статус | Проверяемые решения |
| --- | --- | --- |
| Self-check | Выполнен для draft | 6+3+deferred/excluded; новые суммы; leaf independence; CI queue и merge/release границы |
| Split round 1 | Завершён; все findings закрыты во втором round | Один High CLI staging finding, уточнения baseline checks/xhigh и same-leaf atomic integration |
| Split round 2 | PASS, без открытых findings | Main: Registry 9/10, adapters 9,2/10, integration 9,3/10. Leaf plans независимо приняты Registry и integration |

Split round 1 читал main SHA-256
`053b0e7408247a837c1c2581756dc909767bea33480cfdc93f3125a83d9323ef`, owner
`a1cb26ede0d91e29b6bd37c07a18a5b3f4d21a83dc15a2d6bd36133e7b6bb15a` и три leaf drafts.
Registry: NEEDS_CHANGES 8/10; integration: NEEDS_CHANGES 8,7/10; adapters/main: PASS 9/10.
Integration critic независимо проверил три leaf plans; adapter author проводил для
собственных leaf docs только self-check, что не считается independent approval.

| Finding | Минимальное исправление для round 2 |
| --- | --- |
| S1-R1 / S1-I1 High: ранний CLI merge может требовать ещё отсутствующий v2 bootstrap | Production v1 до явного CLI cutover после signed bootstrap; cutover до freeze N; два composition assertions |
| S1-I2 Low: main aggregate можно прочитать как удаление baseline клиентов | Исключены только новые leaf features/matrices/completion; existing baseline явно сохранён |
| S1-A1 P3: текущие model footers high/xhigh расходились с owner xhigh | Все текущие footers согласованы на xhigh; исторические решения сохранены |
| Root self-check: физическое место central patch | Integrator вносит patch в тот же leaf PR/merge result, без раннего capability-only main commit |

Split round 2 прочитал main 674 строки и owner 130 строк, а также каждый leaf целиком.
Все замечания таблицы выше CLOSED. Точные SHA-256 reviewed snapshots:

| Документ | SHA-256 |
| --- | --- |
| Main | `ab88ef93ce954c6ee86d6784ecabbdf75b22ac7a4238165e39f8e9c5f84a1e6f` |
| Owner | `8ae66122c66726a786dc04a4321e594b564a38afad6dec289363ea54ff0e2f87` |
| Qoder | `dc96efaef9ad5fad0534823c6ba2a4b3f56c5376ee9dd5ab9f225782a3d65c42` |
| Pi | `91dd1ffd9f83597429a3dcb783065163ed5461595668c073f5ad69d24d61543a` |
| Devin | `e027216a31775489fd59b4f28eb3aff64156659d2d50b5d5c62b40bce77284cc` |

Независимые leaf verdicts во втором round: Registry — 9/10 для каждого;
integration — Qoder 9,2/10, Pi 9,3/10, Devin 9,1/10. Авторские self-checks отдельно
и не считаются независимым approval. После review добавлены только журналы, статусы
и фактические ссылки на draft PR #708/#709/#710; implementation contract не изменён.

Новая версия не меняет принятый §4 authority contract. Проверяются изменённые
приоритеты, PR boundaries, budgets, queue strategy и каждый leaf plan; before-code
review gate сохраняется. Будущая реализация/нативные outcomes здесь не объявлены PASS.

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

Workers/reviewers используют GPT-6.1-sol xhigh согласно текущему owner scope.
Перед будущими edits перечитать AGENTS на новом HEAD. Никаких feature flags либо
ослабления existing safety/architecture gates ради зелёного результата.
