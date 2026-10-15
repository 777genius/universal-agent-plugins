# План v9: сохранение изменений при обычных `add`/`update`/`remove`/`repair` в окружении текущего пользователя

**Статус.** Это окончательный план после единственного независимого круга ревью. Реализацию можно начинать.
- **Основа:** main `781686d4…`, дерево `eb31948…` (то же, что у `126a5ac`).
- **Что заменяет:** в плане v7 (`docs/plans/uap-dependency-management-mvp.md:13,61,67`) отложенную правку TOML и отдельный `add --save`. Остальные инварианты v7 в силе.
- **Схемы не меняются:** manifest `schema_version = 1`, lock `lock_version = 1`, state schema 5.
- **Запрещено:** новые журналы, базы данных, движки восстановления, solver, downloader или store, adoption, автоматическая миграция, удаление legacy-кода, релиз и публикация в npm.

## 0. Что изменено после ревью

- **Attestation.** Явная attestation превращает `unchanged` в `update` ровно для одного выбранного target (D12).
- **Разрыв между записью двух файлов.** Порядок публикации: **manifest, затем lock**. В v8 было наоборот.
  - Точечная проверка показала тупик при обратном порядке. Если lock записан, а manifest нет, `deps sync` должен заново разрешить *старый* registry-release. Для него выполняется проверка `DirectoryInstall` (`resolveRegistry` → `acquireDirectory`). Superseded-release её не проходит: это подтверждает тест `deps_manifest_test.go:84-86`. В итоге reconcile отказывает, а обычная команда отказывает по D8.
  - При выбранном порядке состояние разрыва всегда одно и то же: intent впереди, производный lock устарел. Существующий `deps lock` (или `deps sync`) приводит lock к intent. Для холодного root нет состояния «lock без manifest».
  - Повтор той же команды без reconcile **не** обещается. Точная подсказка и тест с внедрённым сбоем описаны в D9.
- **Удаление только по receipt.** Конкретное правило построения плана — в D7.
- **Короткий registry-селектор.** Сопоставление только по одному однозначному кандидату, иначе отказ (D5).
- **Local, `discovery:` и ChatGPT-only.** Остаются на прежнем imperative-маршруте с честным `environment.saved=false` и причиной. Массовой миграции вызовов нет.
- **SDK сокращён.** Остаются Observe и правило сходимости lifecycle, минимальное планирование по выбранным алиасам, типизированные поля завершения, guard на уровне installation и небольшой исполняемый пример. Правка intent и `DeriveAlias` — приватные функции CLI.
- **Три PR.** Допускается одна инертная подготовительная часть PR2, если этого требует размер.
- **tomledit v0.0.29 уже прошёл узкую квалификацию** (`toml-editor-qualification.json`, exit 0: CRLF, кавычки и точки в ключах, комментарии, новое назначение target, удаление секции, семантика через go-toml v2.3.0). Отдельного решения владельца не нужно.
- **Уточнены формулировки.**
  - Формат пакета определяется после получения snapshot. Поэтому обещание звучит как «до эффектов на lock, store, state и профили», а не «до acquisition».
  - `processlock` создаёт родительский каталог (`adapters/processlock/lock.go:43-49`). Реальный порядок появления каталогов — в §4.4.

**Проверенные факты из v8, которые остаются в силе:**
- `ResolveGitHubRef` уже учитывается в счётчике (`deps_e2e_test.go:555-558`).
- Нативный загрузчик возвращает `LoaderKindAgentPlugins` с FormatID `openai-agent-plugin/current` (`loader/native.go:48`).
- Транзакция уже запрещает создать owned binding в unmanaged installation (`transaction/environment.go:144-165`), но не наоборот (`:19-25`).
- Security consent хранится внутри `loadedPackage` (`security.go:24-27`).
- Сайт генерирует `--scope project`, который CLI отклоняет (`pluginInstall.ts:119-121`, `root.go:61-63`).

## 1. Цель и границы

**Сценарий:**
1. Команда с сайта → обычный `add` сохраняет зависимость в `<AGENTPLUGINS_HOME>/uap.toml` и `uap.lock` (по умолчанию `os.UserConfigDir()/agentplugins`, `main.go:232-245`). Установка идёт через существующий coordinator и SDK, с owner receipt.
2. Обычные `update`/`remove`/`repair`.
3. Повтор без изменений.
4. Два файла копируются на другую машину и восстанавливаются через `agentplugins deps sync --locked`.

«Глобально» значит «для текущего пользователя во всех проектах».

**Вне объёма:** project scope, транзитивные зависимости и SemVer, private и произвольные Git-хосты, editable и offline bundles, GC store, adoption, интерактивные вопросы об attestation в сохраняющем `add`, публичный набор инструментов для правки intent.

## 2. Решения

**D1. Маршрут `add`.**

| Источник / targets | Маршрут |
| --- | --- |
| Подходящий удалённый источник (точный GitHub `[github:]owner/repo@SHA[//path]` или Directory selector), все targets ∈ D2 | **Сохранение** (по умолчанию) |
| Подходящий удалённый источник, хотя бы один target ∉ D2 или смешанный набор, кроме случая ниже | **Ранний отказ** до acquisition: печатается точная команда `… --no-save`. Тихого переключения на imperative нет |
| Подходящий удалённый источник, targets ровно `{chatgpt}` (подготовка ChatGPT, в том числе `--chatgpt-app-id`) | Существующий imperative-путь, `saved=false`, `reason=chatgpt_preparation_not_saved` |
| Явный local path (`explicitLocalPath`, `source.go:1052`) | Существующий imperative-путь, `saved=false`, `reason=local_source_not_portable` |
| `discovery:<slug>` | Существующий imperative-путь, `saved=false`, `reason=discovery_source_not_saved` |
| `--no-save` с любым источником | Существующий imperative-путь, `saved=false`, `reason=no_save_requested` |

- Тесты и скрипты с local, discovery или ChatGPT-only вызовами не меняются.
- Executor меняет только те вызовы, у которых поведение действительно изменилось: подходящий удалённый источник с Claude/Codex в режиме по умолчанию или с неподдерживаемыми targets. Если тест проверяет imperative-движок, ему добавляется `--no-save`. Проверки released-артефактов не трогаются.
- **Guard владельца на любом маршруте.** Создание binding с nil authority в installation, у которой есть environment receipt, отклоняется в `transaction.AuthorizeEnvironmentMutation`. Это единственный авторитет для всех путей: `--no-save`, local-пакет, разрешившийся в owned installation, ChatGPT-only.
  - Executor проверяет, что guard стоит на путях single и group install.
  - CLI дополнительно рано отказывает, если source key известен до acquisition.
- `--alias` вне маршрута сохранения отклоняется. Флаги `--auth-complete` и `--activation-complete` на imperative-пути работают как раньше.
- **Human-терминал без `--target` на маршруте сохранения:** выбор только среди обнаруженных клиентов из D2 через `promptTargetChoices`, без загрузки пакета. Неподдерживаемые клиенты показаны как skipped с подсказкой про `--no-save`. Если подходящих нет — отказ.
- **JSON:** в существующий конверт `add` добавляется объект `environment`: `saved`, `reason`, `alias`, `manifest_path`, `lock_path`, `intent_change`.
- Security disclosure и `--accept-security-risk` идентичны imperative-пути. Сбой сканера не обходится.

**D2. Поддерживаемые targets** определяются только через `app.Installer.SupportsClient` (`installer/engine.go:48`). Сейчас это Claude и Codex (`compose.go:120`). Эта же проверка используется:
- в `validateDependencyTargets` (`deps_commands.go:271-281`) — неподдерживаемый target в manifest отклоняется до acquisition со списком поддерживаемых;
- в `supported_targets` вывода `deps status`.

**D3. Как источник превращается в требование.**
- **Git:**
  - Запись в manifest: `git = "https://github.com/<repo в нижнем регистре>.git"`, `path` (пропускается для корня), `commit`.
  - Источник без пути означает корень. Если в корне нет `plugin.json`, вызывается `DiscoverGitHubPackages`, и пользователь получает ошибку «choose //path» со списком кандидатов. Кандидаты не загружаются.
- **Directory:**
  - Release выбирается так: `acquireDirectory`, `DirectoryInstall`, `DesiredReleaseSequence=0`, выбранные targets — то есть новейший release, разрешённый для новой установки. Загрузка пакета одна.
  - Запись в manifest: `registry = "uap"`, канонический `distribution`, `release = N`.
  - `LockedPackage` строит общий конструктор из `resolveRegistry` (`deps_resolver.go:143-173`).
  - Если канонический ID совпал с уже сохранённым алиасом, применяется ветка «тот же источник» (pin не меняется). Загруженные байты остаются только в content-addressed store.

**D4. Алиас** (приватная функция CLI).
- Производится из `manifest.Name`: нижний регистр, `.`→`-`, первый символ не буква → префикс `plugin-`, обрезка до 64 символов, проверка по `aliasPattern` (`model.go:29`).
- Переопределение — флаг `--alias`.
- Алиас уже занят другим источником → отказ с подсказкой `--alias`. Автоматических суффиксов нет.
- Источник уже записан под другим алиасом → используется существующий. Явный другой `--alias` → отказ `source_change`.

**D5. Тот же источник: однозначное сопоставление до сети.**
- **Git:** точный `SourceKey` (repo + path). Без пути — только алиас с корневым path.
- **Registry:**
  - Кандидаты — только registry-записи lock, прошедшего строгую проверку D8. Порядок записей в lock не влияет на выбор.
  - Точное совпадение с каноническим DistributionID имеет приоритет.
  - Иначе совпадение по ProductID или по basename(DistributionID) должно указывать ровно на один источник и алиас. Если совпадений несколько — отказ с перечислением алиасов и подсказкой: `--alias <alias>` или канонический `owner/name`.
  - Если передан `--alias`, он должен совпадать с найденным источником.
  - Совпадений нет → Directory resolution (D3).
- **Матрица результатов:**

| Сохранённый intent | Новый `add` | Результат |
| --- | --- | --- |
| commit X | commit X, новые targets | Добавляются только назначения targets, pin тот же |
| commit X | commit Y | Явная смена pin только у этого алиаса; в плане видно `X→Y` |
| ref/default, в lock commit X | commit X | Добавляются только назначения targets |
| ref/default | commit ≠ X | Отказ с подсказкой `update`. Intent не конвертируется |
| registry N | тот же источник | Pin N сохраняется; подсказка `update` |
| всё назначено и сошлось | — | `unchanged` до Directory и acquisition: ни записей, ни сети |

- **Новый target у registry-алиаса** проходит проверку D13. Если у алиаса есть live owned installation, `Recorded` берётся из её state. Скопированный lock — это только вход для identity, а не доказательство установленного release.

**D6. `update <имя>` для owned-зависимости.** `--target` либо не указывается, либо равен всем targets алиаса: pin задан на уровне алиаса.
- **ref/default:** `Coordinator.Resolve(…, updateAlias)` даёт ровно один lookup, затем scoped sync этого алиаса.
- **commit:** pin не двигается, bindings приводятся к нему. Сообщение: «новый pin — `add <repo>@<sha>//path`».
- **registry:**
  - Выбор: `ResolveDirectory` с `Selector=DistributionID` и `DirectoryUpdate`. `Recorded` берётся из state live owned installation (`recordedDirectoryRelease`, `source.go:75`). Если installation нет — `DirectoryInstall` с sequence 0.
  - Тот же release или `ErrDirectoryNoSafeUpdate` → `unchanged`.
  - Изменился ProductID или DistributionID → отказ.
  - Иначе в TOML пишется `release = M` и меняется только этот алиас.
- **`update --all`:** unmanaged installations обновляются как сейчас. Owned пропускаются с next action `agentplugins update <alias>`.

**D7. `remove <имя> [--target]` и удаление только по receipt.**
1. **Разрешение алиаса** — по receipts `user-default`. Принимается алиас, точный ID installation или declared name owned installation. Результат должен быть единственным.
2. **Targets:** запрошенные targets должны входить в (оставшиеся receipts алиаса ∪ targets алиаса в intent). Иначе отказ.
3. **Правка intent:**
   - Если алиас есть в intent, выбранные targets убираются из `[targets]`; пустой массив удаляется вместе с ключом.
   - Если у алиаса не осталось targets, удаляется `[dependencies.<alias>]` (`model.go:254-258`).
   - Если алиаса в intent нет, это идемпотентная пустая правка: файлы не пишутся.
4. **Планирование:** `PrepareSyncFor` с явным непустым набором `{alias}`. Пустой intent при этом не превращается в выбор всего окружения.
   - Действия удаления строятся из owned-наблюдений (второй цикл `coordinator.go:127-143`).
   - Authority, ID installation и binding берутся из live receipts.
   - Ни пакета из lock, ни Directory lookup не нужно: путь remove в `dependencyInstaller.Prepare` выходит до материализации (`deps_sync.go:201-203`).
5. **Неизвестное имя.** Нет receipts и нет записи в intent → существующий unmanaged-путь без изменений. Он сообщает «not installed» или о retained installation (например для `--purge-data`). Так неизвестное имя отличается от уже завершённого удаления.
6. **Флаги.** `--external-uninstalled` относится ровно к действиям удаления этой команды; их список печатается до подтверждения. `--purge-data` при owned bindings — отказ. После полного удаления next action: `remove <installation-id> --purge-data`. Store и PLUGIN_DATA сохраняются.

**D8. Планирование по выбранным алиасам и предусловие согласия файлов.**
- Обычные операции меняют только выбранный алиас. `deps sync` по-прежнему сверяет всё окружение.
- **Предусловие:** после канонизации по lock в пределах алиаса (`canonicalizeRequirementsFromLock`) должен проходить `lock.Validate`. Также допустимы: manifest отсутствует, или manifest пуст при отсутствующем lock.
- **Иначе отказ** с точной подсказкой: «uap.toml и uap.lock расходятся (в том числе после прерванной записи); выполните `agentplugins deps lock` (без изменений в агентах) или `agentplugins deps sync`, затем повторите команду».
- Конфликты identity и source проверяются глобально. Drift и пробелы lifecycle других алиасов не блокируют операцию.
- Защитная проверка в коде: locked package каждого невыбранного алиаса побайтно равен прежнему. При нарушении — внутренняя ошибка без записи.

**D9. Порядок публикации и восстановление после разрыва.**
- Порядок: кандидат в памяти → подтверждение → под `environment.lock` **manifest, затем lock**, каждый через `atomicfile`. Права существующего файла сохраняются; новые файлы получают 0600.
- Каждый файл пишется только если его байты изменились. `ref`-update и `deps sync` пишут только lock, поэтому разрыва у них нет.
- **Сбой записи manifest:** ничего не изменилось, повторить команду.
- **Сбой записи lock:** manifest уже новый, lock прежний или отсутствует (холодный root), ничего не установлено. Вывод: «uap.toml сохранён, uap.lock не записан: …; выполните `agentplugins deps lock` (без изменений в агентах), затем повторите команду, или `agentplugins deps sync` (сначала покажет план)».
  - `deps lock` заново разрешает только новые или изменённые алиасы. Их pins точные (commit или release), остальные pins совместимы и не меняются.
  - Для холодного root `deps lock` работает при отсутствующем lock. Этот путь ограничен и детерминирован.
- **Сбой после записи файлов, до или во время Apply:** файлы — желаемое состояние. Повтор команды или `deps sync` доводит до него, частичные эффекты покрывают существующие journal и receipts.
- Псевдо-ACID нет.

**D10. Редактор TOML.**
- `creachadair/tomledit` закреплён на v0.0.29 (`838c41eb…`, MIT). Импортирует его только пакет `cli/internal/agentpluginscli/manifestedit`.
- **Контракт:**
  - после правки строгий reader на go-toml v2.3.0 обязан выдать ровно ожидаемые `Requirements`; иначе отказ без записи;
  - комментарии вне изменяемого значения, порядок ключей и таблиц, CRLF/LF сохраняются; пробелы могут нормализоваться;
  - inline-таблица или dotted-форма зависимости, которую адаптер не может безопасно изменить, даёт `ErrUnsupportedLayout` («отредактируйте вручную»).
- Регулярных выражений и собственного parser нет.
- Детали API библиотеки и тесты адаптера — на усмотрение executor. Существенный blocker — только реальная невозможность выполнить контракт этой библиотекой.

**D11. Устойчивое состояние.** Единая функция `environment.LifecycleFacts(binding)`:
- `LifecycleConverged` → без действия.
- Иначе, если `materialized && active && installation_verified` → retry нет, только факт:
  - `auth_unverified` (информационный, для `not_checked`);
  - `auth_pending` или `auth_failed` — с next action на attestation.
- Иначе → retry (`update`, факт `partial`). Восстановление activation и verification сохраняется.
- `domain.LifecycleConverged` и `verifiedRegistrationUnchanged` (`usecase/lifecycle.go:131-140`) не меняются.

**D12. Явная attestation.**
- `add <тот же источник> --target T --auth-complete|--activation-complete` выбирает ровно один алиас и один target.
- `Coordinator.PrepareSyncFor` превращает `unchanged` или `update` этого binding в `ActionUpdate` с `reason=explicit_lifecycle_attestation`. Флаги кладутся в поля `PreparedBinding`.
- **Условия**, иначе отказ до эффектов:
  - binding owned;
  - binding materialized;
  - digest и revision равны pin. Если pin меняется, сначала обновление, потом attestation.
- **Цепочка передачи:** `dependencyInstaller` кладёт флаги в `installer.Request.ActivationComplete`/`AuthComplete` → Prepare копирует их в preview `AddInput` (`installer/prepare.go:246-252`) → Apply копирует в `AddInput` (`installer/apply.go:190-197`).
- Проверка usecase «только для materialized» (`usecase/apply.go:248-250`) остаётся авторитетом.
- Без флагов `unchanged` никогда не превращается в `update`.

**D13. Переносимость registry-pin.**
- Git-pin воспроизводится, пока commit доступен на GitHub.
- Registry-pin воспроизводится, только пока подписанный Directory разрешает **новую установку** этого release: `verifyLockedDirectory` с `recorded=nil` (`deps_sync.go:222-269`).
- Superseded release требует `update <alias>`. Revoked не воспроизводится.
- Новый target у live owned installation проходит `DirectoryNewTarget` с `Recorded` из её state, как imperative `retainDirectoryRelease`. Без installation — `DirectoryInstall` с точным sequence. `DirectoryReproduce` не используется.

**D14. Сайт.**
- Pages деплоится из main, а опубликованный CLI не знает `--no-save`. Поэтому новый генератор команд стоит за константой `SAVED_DEPENDENCIES_CLI_RELEASED = false`. При `false` вывод побайтно прежний.
- Исправление `--scope project` — без gate. Для OpenCode/Cursor используется user scope, если его поддерживает опубликованный CLI (проверить по тегу последней публикации); иначе lane скрывается.

## 3. Поведение команд

| Команда | Поведение |
| --- | --- |
| `add <src> [--target] [--alias] [--auth-complete] [--activation-complete] [--no-save] [--dry-run]` | D1–D5, D8, D9, D12. Политика подтверждения как у imperative `add` (`mutationConfirmed`, `lifecycle.go:755`) |
| `update <имя\|id\|alias>` | Owned → D6. Unmanaged → без изменений |
| `remove <…>` | Owned или receipt-only → D7. Unmanaged → без изменений |
| `repair <…>` | Существующий exact-pin repair (`deps_sync.go:273-316`); добавлено разрешение алиаса |
| `deps init` | Исправление 1 |
| `deps lock`, `deps update <alias>` | Только lock; исправления 2 и 4; способ reconcile после разрыва из D9 |
| `deps sync [--locked]` | Общая сессия: lock пишется после подтверждения; рендеринг по исправлению 5. Восстановление на втором root: `deps sync --locked` |
| `deps status` | Шапка: scope, пути, `supported_targets`. Если manifest нет — печатает пути и завершается с кодом 0. Факты D11, ID installation, snake_case JSON, next actions |
| `deps recover` | Без изменений |

**Разрешение selector для `update`/`remove`/`repair`:**
1. Алиас найден (в intent или в receipts) и installation отсутствует либо принадлежит ему → owned-маршрут.
2. Найдена owned installation (по имени или ID) → owned-маршрут с алиасом из receipt.
3. Алиас совпадает с именем unmanaged installation, либо receipt принадлежит другому алиасу → отказ «неоднозначно; укажите ID installation».
4. Иначе — существующий unmanaged-путь.

## 4. Поток данных и границы

**4.1. SDK (`install/integrationctl/agentplugins`), минимум.**
- **`environment.Observe(view installer.Inspection) ([]BindingObservation, error)`**
  - Переносит вывод наблюдений из `dependencyActions` (`deps_apply.go:28-56`) и применяет D11.
  - Дополнительно отдаёт **unmanaged**-наблюдения (`Managed=false`, `SourceKey`, `InstallationID`, `ClientID`).
  - `validateDesiredBinding` отклоняет пакет, если у его `SourceKey` есть любой unmanaged binding (уровень installation). Это заменяет потаргетный `rejectForeignDependencies` (`deps_apply.go:120-136`).
- **`LifecycleFacts(installer.InspectedBinding)`** — общая функция для status и sync.
- **Планирование по выбранным алиасам:**
  ```go
  // Illustrative; names may differ.
  type Selection struct {
    Aliases []string     // non-empty; may name aliases absent from intent (receipt-only removal)
    Attest  *Attestation // optional, exactly one selected alias/target
  }
  type Attestation struct {
    Alias, ClientID                  string
    ActivationComplete, AuthComplete bool
  }
  func (c Coordinator) PrepareSyncFor(r Requirements, l Lock, observed []BindingObservation, s Selection) (PreparedSync, error)
  ```
  - `PrepareSync` остаётся планом для всего окружения.
  - Оба цикла фильтруются по `Aliases`. Проверки identity остаются глобальными.
  - `PreparedBinding` получает поля `ActivationComplete` и `AuthComplete`.
- **`installer.Request.ActivationComplete`/`AuthComplete`.** Только single-target install/update/repair; для remove и групп Prepare их отклоняет.
- **`transaction`:** guard из D1.
- **`environment/example_test.go`** — небольшой исполняемый пример на реальных контрактах:
  - `installer.New` (Claude/Codex) на одноразовом root → `Inspect` → `Observe`;
  - `Coordinator.Resolve` с **host Resolver**;
  - `PrepareSyncFor` → печать действий;
  - тип **host BindingMutator**: показывает, какие поля `installer.Request` хост заполняет из `PreparedBinding` и что package root и `Assessment` хост получает сам, затем `Engine.Prepare/Apply`. Apply в примере не выполняется.
  - Host adapters явно помечены. Готового «ready-to-use» адаптера нет; сериализацию мутаций окружения обеспечивает хост.

**4.2. CLI.**
- `manifestedit/` — D10 и контрактные тесты.
- `deps_manifest.go` — `readDependencyManifestBytes` и один шаблон для `init` и холодного `add`.
- Приватные функции правки intent (D4–D7) — рядом с потребителем.
- **`environment_session.go`** — общая сессия для `deps sync` и обычных операций:
  1. **Snapshot** (без `environment.lock` и без записей в intent): `RequireMutationReady` через type assertion, как в `usecase/service.go:237`; `Inspect` (`Recovery.Required` → отказ с `deps recover`); байты manifest и lock; канонизация; D8.
  2. **Plan:**
     - правка intent;
     - кандидат через `Coordinator.Resolve` с *seeded resolver*: уже разрешённый `LockedPackage` нового алиаса, без второй загрузки;
     - `Observe`, `PrepareSyncFor`;
     - preflight Prepare (`preflightDependencyActions`), security consent запоминается в памяти по digest; в lock и store он не попадает.
  3. **Render и подтверждение** без блокировок. Dry-run или отмена — выход без записей.
  4. **Commit под `environment.lock`:**
     - повторно `RequireMutationReady`;
     - байты обоих файлов совпадают со snapshot, иначе `ErrPlanChanged`;
     - повторный Inspect, действия `DeepEqual` с подготовленными (`deps_apply.go:171`);
     - `ensureDependencyStore` только для выбранных алиасов;
     - запись по D9 через seam публикатора (подменяется в тестах для сбоя);
     - `ApplySync` с `beforeApply`, который сверяет **опубликованные** байты.
  5. **Render результатов.**
- `deps_render.go` — snake_case DTO плана и результатов и одна функция next actions для status и sync.

**4.3. Блокировки без рекурсии.**
- `environment.lock` (`compose.go:178`) берут только шаг 4 сессии и обработчики `deps lock`/`deps update`/`deps recover`.
- `store.lock` — только внутри `Store.Put`.
- `mutation.lock` — только внутри SDK Apply/Recover.
- Подтверждение и сетевое разрешение выполняются без блокировок.
- Два конкурентных UAP-процесса: второй на шаге 4 видит изменённые байты → `ErrPlanChanged`, без эффектов.
- Внешний редактор: окно TOCTOU между сверкой и rename задокументировано; `beforeApply` останавливает Apply после чужой правки.

**4.4. Реальный порядок появления каталогов на холодном root** (маршрут сохранения).

| Шаг | Что происходит с файловой системой |
| --- | --- |
| 1. Проверка флагов, синтаксиса и targets | Ничего не создаётся. Существующие тесты `cmd/agentplugins/main_test.go:107-114` и `release_binary_test.go:49-65` должны остаться зелёными |
| 2. Readiness, Inspect, чтение manifest | Только чтение. Executor проверяет, что `Inspect` на отсутствующем root ничего не создаёт; иначе проверки шага 1 идут раньше |
| 3. Acquisition | Существующий ленивый temp-каталог под `AGENTPLUGINS_HOME`, как у imperative `add` |
| 4. Проверенные байты | `store/` — content-addressed кеш, не intent |
| 5. Подтверждение | — |
| 6. `environment.lock` | `MkdirAll(root, 0700)` и файл блокировки (`processlock`) |
| 7. Запись | manifest, затем lock |
| 8. SDK Apply | state, профили, PLUGIN_DATA |

- Отмена на шаге 5: могут остаться root и store; `uap.toml`, `uap.lock`, state и профили не меняются.
- Dry-run пишет только во временные root (`dependencyPreviewApp`). Dry-run evaluator получает временный кеш сканера (`compose.go:201-206`).

## 5. Отказы и владение (сводка)

| Ситуация | Поведение |
| --- | --- |
| Пакет не в стандартном формате | Обнаруживается после acquisition во временный snapshot. Отказ до записи в store, lock, state и профили |
| Schema 4 | Отказ до сети и записей. Текст: `agentplugins migrate-state --dry-run` (или `npx universal-agent-plugins migrate-state --dry-run`), затем запуск без `--dry-run`. Явная миграция сохраняется |
| Unmanaged installation того же источника | Отказ с подсказкой «удалите `<id> --target …`, затем повторите `add`». Adoption нет |
| `--no-save` или local-пакет попадает в owned installation | Отказ (guard в транзакции, ранний отказ в CLI) |
| Drift выбранного алиаса | Отказ с подсказкой `repair <installation-id> --target T`. Drift других алиасов не блокирует |
| Разрыв между записью manifest и lock | D9 |
| Частичный Apply | Результаты по каждому binding. `deps recover` предлагается только при `Recovery.Required`; иначе `Result.NextActions` и `ManualActions` |

**Откат:** revert PR. Файлы остаются валидными для прежних команд `deps`. Автоматического удаления данных нет.

## 6. Исправления из аудита

1. **Холодный `deps init`** (`deps_commands.go:258-269`, `deps_manifest.go:158-178`).
   - Без dry-run: `MkdirAll(root, 0700)`, затем `O_EXCL`. С dry-run каталоги не создаются.
   - Вывод: human — `Created <path>` и next step; JSON — `{created, manifest_path}`.
   - Если файл уже есть — ошибка с полным путём.
   - Тест на уровне `cmd`.
2. **Только стандартный формат.**
   - Общий помощник: корневой `plugin.json` должен быть обычным файлом → `app.PackageLoader` вызывается напрямую → `FormatID == FormatIDAgentPluginsV1` → скан.
   - Где применяется: `resolveGit`; `acquireDirectory` через флаг запроса `StandardOnly` (imperative-путь продолжает использовать `loadAcquiredSnapshot`, нативный importer сохраняется); ветка загрузки в `materializeLockedPackage` **до** `store.Put` (`deps_sync.go:46-58`).
3. **D11.** Status и sync используют одну функцию.
4. **Канонизация коротких registry-имён.** В `resolveDependencyLock` (`deps_commands.go:283-298`): прочитать предыдущий lock → канонизация в пределах алиаса → проверка Directory только для оставшихся новых коротких имён. Порядок одинаков для `lock`, `update`, `sync` без `--locked` и обычных операций. Проверка eligibility для новых алиасов не ослабляется.
5. **Рендеринг sync.**
   - До вопроса: изменения и удаления, число unchanged, список attestation `--external-uninstalled`.
   - После: результаты по каждому binding в human и JSON.
   - Безусловного «recover» нет.
   - `ExternalUninstalled` передаётся только в запросах remove (`deps_sync.go:195-197`); если удалений нет — отказ.
   - Status переходит на snake_case (`deps_status.go:22`).
6. **Тесты:** §8.

**Дополнительно:** guard на уровне installation, запоминание consent, D2, текст миграции.

## 7. PR

Ветки — conventional, от актуального main. Коммиты — от штатной identity репозитория. Обычный PR, без force и admin. Для каждого mergeable PR — независимое ревью кода и все обязательные проверки CI на точном head.

1. **`fix(deps): close audited dependency-environment gaps`** (~1500–1900 LOC).
   - Исправления §6.1–6.5 и дополнительные пункты.
   - `Observe`/`LifecycleFacts` с unmanaged-наблюдениями; guard в транзакции.
   - Регрессионные тесты из §8, относящиеся к этим исправлениям.
   - `docs/UAP_DEPENDENCIES.md`: статус «unreleased», пути, D2, D13, миграция.
2. **`feat(deps): save ordinary add/update/remove/repair in the user environment`** (~2200–2800 LOC).
   - `manifestedit`, сессия, `PrepareSyncFor` с `Attestation`, поля Request и PreparedBinding, приватные функции правки intent и алиаса.
   - Маршрут D1 и `--no-save`; маршрутизация update/remove/repair; удаление только по receipt; пропуск owned в `update --all`; пример в SDK.
   - Точечные изменения только тех тестов, у которых меняется поведение.
   - Главный сценарий, сценарий registry и тест со сбоем записи.
   - Документация обычного потока; статус v9 в файле плана.
   - Сохраняющий `add` по умолчанию не мержится без owned lifecycle.
   - Если PR больше ~2500 строк без механических правок тестов, допускается одна инертная подготовительная часть **2a**: `manifestedit` с контрактными тестами и сессия для `deps sync`. Без нового пользовательского режима и публичного framework.
3. **`feat(landing): saved-dependency commands behind CLI-release gate`** (~400–700 LOC).
   - Генератор (D14), исправление scope, локали по существующему правилу fallback, честные формулировки «unreleased».
   - Существующие метки доступности и исторические ссылки сохраняются там, где им положено быть.
   - Gate остаётся `false`; README и README в npm не меняются.

## 8. Проверка

**Главный сценарий** — in-process, на сильнейшей детерминированной границе. Реальный cobra root, реальный Git (локальный repo через `URLForRepo`), standard loader, store, SDK installer и транзакция, проекции Codex/Claude. Процессы клиентов и сканер симулируются: это fixture evidence, а не live-квалификация релиза. Используется существующий harness в `deps_e2e_test.go` со счётчиком вызовов runner.

| Шаг | Ожидание |
| --- | --- |
| 1. Холодный root: `add github:acme/demo@SHA1 --target codex --format json` | Созданы root, manifest и lock; owner receipt; lookups = 0; загрузок = 1 |
| 2. Повтор той же команды | `unchanged`; побайтно равны `uap.toml`, `uap.lock`, `state-v2.json`; не изменились store, runner, загрузки и lookups |
| 3. Комментарий в TOML, затем `add … --target claude` | Комментарий на месте; codex не тронут |
| 4. `add … --target codex --auth-complete` | В state только для codex: `authenticated` и attested. Следующий повтор без флага — `unchanged` |
| 5. Ручная правка на `ref = "main"` → `deps sync --yes` (lookups > 0) → новый commit → `update demo` | Ровно +1 lookup; обновлены оба target |
| 6. `remove demo --target claude` | Таблица зависимости сохранена |
| 7. `remove demo` | Таблица удалена. Удаление target, требующего ручного шага, неполное → повтор `remove demo --external-uninstalled` проходит **по receipt**: без lookups и загрузок, до terminal. Store и PLUGIN_DATA сохранены |
| 8. Повторный `add` | Установка проходит |
| 9. Копия двух файлов на второй root → `deps sync --locked --yes` | Те же digests; lookups = 0; повтор `unchanged` |

**Сценарий registry** (`newRolloutDirectoryFixture` и `localBackedSourceAcquirer` плюс маленький routing acquirer внутри теста):
- `add rollout-demo --target codex` → записан канонический `owner/rollout`, release 1; повтор без сети.
- Вторая сохранённая distribution с тем же ProductID или basename → `add <short>` отказывает и перечисляет алиасы.
- Release 1 стал superseded → `deps lock`, update другого алиаса и `sync` без `--locked` проходят.
- `update docs` → release 2; другие pins побайтно прежние.
- Перенос на второй root → `--locked` восстанавливает.
- Свежий root с superseded pin → отказ по задокументированной политике.

**Сбой записи (холодный root):** seam публикатора отказывает на lock.
- Ожидается: manifest записан, lock отсутствует, state не тронут.
- Повтор `add` → отказ по D8 с точной подсказкой.
- `deps lock` → файлы согласованы → повтор `add` устанавливает.

**Точечные проверки:**
- холодный `deps init`;
- не стандартный формат: корень Git, release в registry, rematerialize из store по подготовленному lock — нет lock и нет записи в store;
- таблица D11; провал activation → retry; `auth_pending` без флага → `unchanged`;
- human-ответ `n` → действия напечатаны, «No changes made»; сбой без pending → нет текста «recover»;
- guard в обе стороны, включая local-пакет, попавший в owned installation, и `--no-save` с owned-источником;
- ранний отказ для неподдерживаемого или смешанного набора targets при нетронутом `AGENTPLUGINS_HOME` (на уровне `cmd`);
- local, discovery и ChatGPT-only → `saved=false` с причиной;
- контрактные тесты `manifestedit`;
- `PrepareSyncFor` (выбранный набор, удаление по receipt, attestation);
- поля Request;
- оба состояния gate на сайте;
- счётчики: положительный первый lookup и раздельный учёт Selector/Ref.

**Команды:**
```sh
# install/integrationctl/agentplugins
go test ./environment ./installer ./transaction ./usecase ./adapters/statev2 ./adapters/environmentstore
# cli
go test ./internal/agentpluginscli/... ./cmd/agentplugins
# landing
npm run test:registry && npm run lint
```
Затронутые npm-тесты и terminal-ui тесты запускаются для изменённых вызовов. Полная проверка — в CI на точном head, включая Windows, govulncheck, dependency-review и landing. После исправлений по ревью перезапускаются только затронутые тесты. Реальные пользовательские профили и live-клиенты не используются.

## 9. Приёмка

- Главный сценарий, сценарий registry и тест со сбоем записи проходят в CI на Linux, macOS и Windows.
- Повтор: ни записей, ни процессов, ни сети. Явная attestation меняет только выбранный target.
- Targeted update даёт +1 lookup; чужие pins побайтно прежние.
- Удаление снимает только выбранные назначения. Повтор по receipt доводит удаление до конца после исчезновения таблицы.
- Перенос двух файлов восстанавливает те же digests.
- Подходящие удалённые источники сохраняются по умолчанию. Неподдерживаемые или смешанные targets отказывают рано с подсказкой `--no-save`. Local, discovery и ChatGPT-only честно не сохраняются. Ни один маршрут не обходит owner authority.
- Adoption нет, YAML и нативный fallback в графе deps нет. Комментарии TOML переживают правки.
- `deps status` показывает `auth_unverified` как информационный факт, а также пути и supported targets.
- В production сайт выдаёт прежние команды, кроме исправления scope. Документы помечены «unreleased». Релиза и публикации в npm нет.

## 10. Риски

- **WIP-статус tomledit.** Изолирован закреплённой версией, единственным пакетом-импортёром и обязательной перепроверкой go-toml. Квалификация v0.0.29 уже пройдена. Вопрос открывается только при реальной невозможности выполнить контракт.
- **Пользователи только с Cursor, Gemini или OpenCode** на маршруте сохранения получают ранний отказ с подсказкой `--no-save`. Так задумано, пока эти адаптеры не включены в composition.
- **Окно TOCTOU с внешним редактором** задокументировано. Process lock не связывает сторонние редакторы.
- Новый общий круг ревью плана не требуется. Рутинные обратимые решения (имена, раскладка файлов, тексты) остаются за executor.

## Дополнение оркестратора: pin update и новый target за один sync

# Parent correction from additional Opus implementation review

The separately requested Opus review completed on base781686d4, report opus-implementation-review/report.md. Most findings already covered by v8. One extra confirmed required regression: transferring a newer pin plus an extra target onto a machine with an old pin/current target must work in one sync.

Actual source proof: environment/coordinator.go validateDesiredBinding assigns existing InstallationID to a new binding (225-227), sorts lexical target order (144-150), while deps_sync.go preflights every action before Apply (142); installer/prepare.go392-395 and package.go287-289 refuse OpInstall with an old recorded installation digest. Thus both old-state preflight and install-before-update ordering can reject a valid desired graph.

Implementation requirement, minimal boundary: within each alias, apply verified existing binding pin-updates before new target installs. Preflight a new-target install affected by a preceding pin update against the explicitly projected desired pin while still verifying actual source/namespace/ownership and all security/capability checks; never mutate state merely for preflight, never weaken real SDK Apply digest or receipt validation. The SDK already reparses/prepares each action after prior commit; preserve that. Choose the smallest clear simulation/request preparation seam, not a second planner or transaction engine. Regression: pinA/codex -> pinB/codex+claude using one deps sync --locked; prove same final digests, receipts, and no unmanaged adoption. Include relevant SDK/CLI tests once, not a matrix.

Other extra findings (update--all skipowned, same-bytes parsing, postconfirm lock, repair result output) are already part of plan correction; verify implementation includes them. This is a specific code finding, not a new general plan review round.
