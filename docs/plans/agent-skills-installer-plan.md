# Agent Skills: установка, сканирование и обновление через UAP

Статус: начальная реализация доставлена в main через PR #478 (`e3d84d05b241426e06a3dafe4f386ecb1f448e30`); обязательные проверки и 12 browser E2E прошли. Публичный CLI-релиз не опубликован. Проверка реальных коллекций определила дополнительные критерии качества и повторной приёмки, перечисленные ниже. Решения владельца от 2026-10-23: сначала совместимость; использовать существующий установщик и `uap.toml`/`uap.lock`; включить простой поиск перенесённого Skill по имени; браузерный picker делать параллельно на React + Zustand + TanStack Router/Query, с feature-owned slices. Unit-тесты — только на существенные новые правила.

Уточнение по Q и `/plan-improve`, 2026-10-23: выбран готовый UI stack, один генерируемый Go/TS wire contract и существующие backend библиотеки. Состояние каталога, mutable intent, sealed source и нормализованный artifact имеют разную authority; ниже эти границы названы явно. Этот документ задаёт требования, а не подтверждает состояние реализации: готовность определяется проверками §9–10, а не наличием файлов или предыдущими статусными формулировками.

Связанный независимый frontend-план: [agent-skills-picker-ui-plan.md](./agent-skills-picker-ui-plan.md). Его реализация может идти параллельно backend после согласования малого API ниже. Это одна поставка с двумя lanes, а не два установщика.

## Уточнение качества после реальных коллекций, 2026-10-23

Владелец запросил несколько проходов улучшения без расширения архитектуры; приоритет — desktop и CLI. Отдельная мобильная доработка не требуется.

Обязательные исправления: preview inventory не загружает resource blobs ради их размеров (неизвестный размер остаётся неизвестным); desktop сохраняет удобный доступ к корзине и целям при открытом preview; review возвращает фокус на инициирующую кнопку; невалидные кандидаты имеют читаемые названия и объяснения; terminal flow даёт понятные действия при ошибках и удобный выбор существующими prompt primitives. Повторная проверка включает реальный loopback bridge, disposable installation, browser refresh, keyboard и измерение раскрытия дерева на 1 000 кандидатах. Строгую совместимость стандарта не ослаблять ради невалидных upstream Skills.

Первый аудит реальных immutable коллекций: OpenAI 44/44 доступны, Anthropic 18/20, Vercel 5/9; все каталоги полные. Это наблюдение на конкретных SHA, не обещание текущего состава коллекций. Два отказа Anthropic: длинное description и несовпадение имени template; четыре Vercel — несовпадение имени с папкой. Установка PDF из Anthropic в disposable profile и восстановление результата после refresh прошли. Повторное сканирование загруженных источников измерено отдельно от холодной Git-загрузки: 25–111 ms. Эти результаты не закрывают найденные UX/resource-inventory недостатки.

## 1. Проверенная основа и границы

- Baseline реализации: `3ea24e3014ddf6679ca86729c56f1069112b0d3a`, отдельный worktree; исторические source links ниже сохранены как основания решений.
- Проверенный при исследовании remote main: `cbf20205ee56c8fa732ae594e015cbc9359ea217` от 2026-10-22. Локальный checkout `6dc79a8` старее и содержит чужие изменения; не переносить его старую архитектуру в реализацию и не сбрасывать изменения.
- В main уже существуют `environment`, Git ref resolution, граф зависимостей, lock/replay, store, сохранение TOML и общий installer lifecycle. Реализацию начинать от актуального main с повторной узкой сверкой этих точек, без повторной разработки dependency manager.
- Применять Q соразмерно: чистые контракты discovery/normalization на границе acquisition; policy отделена от Git/HTTP/React. Не создавать generic provider platform, event bus или отдельный resolver.
- Соблюдать [capability preservation](../AUTHORING_CAPABILITY_PRESERVATION.md) и owner decisions [основного authoring-плана](../STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md). Legacy `plugin.yaml` и внешний Skills lifecycle сохранить изолированно; скрытых fallback нет.
- Наличие кода на main не является утверждением о опубликованной доступности. Этот документ не разрешает merge/release, реализацию отложенных authoring phases или изменение production wording.

Входит: публичный GitHub repository/folder/`SKILL.md` URL, существующие Git selectors, вложенный каталог Skills, выбор одного/нескольких Skills, установка в существующие поддержанные targets, saved add/update/remove/repair/locked restore и простой перенос внутри того же repository. CLI и browser используют одни application services.

Не входит: новые клиенты, private Git auth, другие Git hosts, hosted registry Skills, автообновление в фоне, semantic evaluation качества навыков, выполнение scripts при установке, fuzzy-переименования или перенос между repositories. Локальные disposable fixtures используются для проверок; editable/local dependencies в TOML этим планом не вводятся.

## 1.1 Готовые библиотеки и точные границы reuse

| Задача | Выбор | Что остаётся нашим |
| --- | --- | --- |
| HTTP и bundled assets | Go `net/http.ServeMux`, `encoding/json`, `embed` | Session admission, безопасный bootstrap, mapping application results и lifecycle shutdown |
| Git tree, selectors, snapshots | Существующий изолированный Git Runner/acquirer | Обнаружение SKILL.md, completeness и точное relocation; не второй Git client |
| Сканирование concurrency | Уже используемый `golang.org/x/sync/errgroup` с `SetLimit(2)` | Атомарное резервирование byte budget и детерминированная сборка результатов |
| Skill YAML, TOML и editing | Существующие bounded conformance primitives, `yaml.v3`, `go-toml/v2` 2.3.0, `tomledit` 0.0.29 | Новые поля source kind и их смысл; без нового frontmatter/TOML parser |
| CLI/TTY | Существующие Cobra и Charm/huh prompt adapters | Одни selections и application services с browser flow |
| Go/TS API-контракт | Один OpenAPI 3.0.3 file; `oapi-codegen` **2.8.0** только для Go models; `openapi-typescript` **7.13.0** для TS types | Domain policies, handler services, strict input/identity checks |
| Browser HTTP client | `openapi-fetch` **0.17.0**, в `shared/api` frontend | Feature-owned Query hooks, session header и нормализованные ошибки |

`net/http` и текущий Git Runner уже закрывают нужные механизмы: chi/Gin/Huma и go-git не добавляются ради семи основных API operations и одного rescan. `openapi-fetch` выбран как небольшой typed transport; он не добавляет второй Query cache и не валидирует JSON в runtime. OpenAPI tools — build/dev dependencies; HTTP framework, Go client SDK и runtime OpenAPI interpreter не нужны.

Это не поручение обновить все существующие Go dependencies: версии reuse проверены на main; каждую реально добавляемую dependency закрепить в нужном module, без попутной миграции YAML или installer. `oapi-codegen` — Apache-2.0, TypeScript tools — MIT. Go models держать на транспортной границе, не импортировать generated API package в scanner/domain/environment.

Изолированный исследовательский baseline frontend и сравнение библиотек указаны в отдельном плане. Существенная совместимость: `openapi-typescript` 7.13.0 объявляет peer TypeScript `^5.x`, поэтому общий frontend/compiler baseline — **5.9.3**, а не ранее исследованный TS 7. Глобальный `tsc7` может быть быстрым дополнительным preflight; CI использует закреплённый поддержанный compiler, без `--force` и обхода peer checks.

## 1.2 Общий вход в application logic

Из текущего saved flow выделить минимальные типизированные операции подготовки и применения batch requirements; CLI adapter и HTTP handlers вызывают их напрямую. HTTP не вызывает Cobra `Execute`, CLI command handlers, subprocess или разбор текстового CLI output. Existing resolver/preflight/lifecycle остаются владельцами installer policy; транспорт преобразует input/result и передаёт context с нужным временем жизни.

Preview и непринятая подготовка review могут использовать request context. Принятые фоновые scan/install принадлежат CLI session и получают её context **до** ответа HTTP 202: завершение handler, отмена fetch или закрытие вкладки их не отменяют. Scan сохраняет свой deadline; SIGINT/выход CLI обрабатываются существующим cleanup/recovery lifecycle. Не запускать application flow как detached background process и не привязывать его к context уже завершённого HTTP-запроса.

Если подходящая операция уже экспортирована на актуальном main, использовать её. Не вводить новый service framework или интерфейс для каждого helper: граница нужна между транспортом и существующим use case, а не между всеми функциями.

## 2. Основной пользовательский контракт

Предлагаемый новый CLI в основном бинарнике:

```sh
agentplugins skills add owner/repository
agentplugins skills add https://github.com/owner/repository/tree/main/skills/review
agentplugins skills add https://github.com/owner/repository/blob/main/skills/review/SKILL.md
agentplugins skills add owner/repository --skill review --skill pdf --target codex,claude
agentplugins skills add owner/repository --path skills/review --target codex
```

- Явные `--skill`, `--path` или `--target` по умолчанию выбирают terminal flow; один `--path` задаёт один Skill, повторяемый `--skill` — несколько точных имён. Для JSON/non-TTY нужны явные targets и подтверждение `--yes`; без них никаких effects. `--ui` несовместим с `--skill`, `--target` и `--dry-run`: отказ до acquisition с подсказкой terminal-команды, без молчаливого игнорирования флагов. Folder/file URL может задать область browser через `--ui`; переносить CLI selections в browser query parameters в первой поставке не требуется.
- Обычный repository input открывает browser picker в интерактивном desktop-сеансе. `--ui` выбирает browser явно, `--no-ui` оставляет терминальный поиск/выбор. Если браузер недоступен, вывести URL и предложить тот же TTY flow без нового acquisition. JSON/non-TTY не открывает браузер; нужны явные selections/targets, иначе структурированная ошибка с доступными кандидатами.
- URL файла означает весь каталог Skill. URL папки с непосредственным `SKILL.md` выбирает этот кандидат; URL категории задаёт начальную область просмотра. Само указание URL не обходит обычный review/confirmation.
- Для точного file/folder path сначала проверяется только выбранный Skill: не сканировать весь repository ради одного известного кандидата. «Посмотреть весь репозиторий» запускает catalog scan на том же commit. Scope completeness не равна repository completeness.
- `--skill` — точное имя; неоднозначность требует путь. `--path` — точная repository-relative папка одного Skill. Multi-select в picker сохраняет каждый выбранный Skill отдельным dependency alias.
- Общие update/remove/repair/environment-команды переиспользуются. Новые Skills в обновившемся repository сами не устанавливаются. Переносы не меняют список выбранных targets.
- Alias проходит существующий environment validator: допустимое имя Skill используется по умолчанию, иначе предлагается стабильный ASCII alias; конфликты путей получают различимые suffixes. Сохраняемый alias показывается до apply и не переписывает исходное имя Skill.
- GitHub shorthand/web/raw URL нормализуются к canonical repository, selector и относительному пути. Разделение ref/path с `/` в ref должно проверяться по фактическим refs, а при неоднозначности требовать явный `--ref`; не угадывать по первому slash. SHA URL закрепляет commit. Git ref/default/update semantics остаются существующими.
- Группа `skills` выбирает raw Skill source явно. Отсутствие `plugin.json` в обычном package flow никогда не включает новый loader автоматически.

Multi-select формирует все новые requirements/aliases и TOML edits в памяти, затем один candidate environment graph/lock и общий preflight выбранных targets через текущий saved flow. Не вызывать CLI add циклом и не запускать отдельный resolver на каждый checkbox. Acquisition одного repository/SHA и metadata переиспользуются. После preflight действуют существующие manifest/store/receipt и partial-outcome guarantees; общей атомарности чужих native activation не обещать.

Если любой выбранный источник не разрешается или не проходит общий preflight, весь batch останавливается до durable изменений: не исключать проблемный Skill автоматически и не сохранять смесь новых и прежних pins как успешное обновление. Пользователь получает причину и может явно изменить выбор перед новым review. Это существующая граница resolver/preflight, а не новая транзакция поверх installer; I/O-сбои после начала durable стадии обрабатываются его receipts/recovery.

## 3. Manifest и lock: intent отдельно от текущего размещения

Предлагаемое расширение существующей `[dependencies]`, без отдельного `[skills]`:

```toml
schema_version = 1

[dependencies.review]
kind = "skill"
git = "https://github.com/acme/agent-skills.git"
skill = "review"
path = "skills/review"
ref = "refs/heads/main"

[targets]
codex = ["review"]
claude = ["review"]
```

`kind` и `skill` реализованы как явное расширение Git dependencies. `kind = "skill"` требует Git source, точное ожидаемое имя `skill` и выбранный `path` (пустой root допустим для корневого Skill). Alias остаётся пользовательским именем зависимости. Без `kind` действуют прежние правила plugin package.

Для Skill `path` фиксирует исходно выбранный путь и является стабильным якорем intent; текущий найденный путь хранится отдельно в lock. Это явное правило нового source kind: значение `path` существующих plugin dependencies не меняется. После relocation TOML автоматически не переписывается.

Source key Skill включает kind, canonical repository, точное имя и requested path; ref/commit, alias и текущий resolved path в стабильный key не входят. Старые plugin source keys и digest vectors сохраняются. Разные requested anchors, разрешившиеся в один `(repository, commit, resolved path)`, обнаруживаются до установки как повторный выбор одного Skill.

Этот же стабильный Skill SourceKey задаёт synthetic declared name и installer SourceBindingID. На проверенном main даже hint `uap-dependency-git` в `domain.ComputeSourceBindingID` включает `PackageSubpath`: подставлять туда новый resolved path для вычисления Skill binding нельзя. Добавить узкую ветку нового Skill kind/hint со стабильным binding input; key вычисляет environment policy и передаёт adapter, domain не импортирует environment или lock DTO. Commit/resolved path остаются координатами acquisition/provenance. Прежний plugin binding hash не менять. Relocation сохраняет InstallationID, SourceBindingID и native PhysicalArtifactID, если target/установка те же; content digests могут меняться при обновлении bytes.

В locked Git source для Skill добавить discriminator, expected name, `resolved_skill_path`, raw-source digest и `normalizer = "agent-skill-package/v1"`. Существующие package tree/manifest digests описывают полученный стандартный artifact; не подменять их hash исходной папки. Requested selector/path остаются доступными для сравнения с TOML. Изменение targets сохраняет совместимые pins.

Сохраняются версии manifest/lock `1` как добавочное расширение с обязательным явным discriminator. Основание на закреплённом main: [`dependencymanifest.Decode`](https://github.com/777genius/universal-agent-plugins/blob/cbf20205ee56c8fa732ae594e015cbc9359ea217/install/integrationctl/agentplugins/adapters/dependencymanifest/decode.go#L42) отклоняет неизвестные TOML fields, а [`decodeDependencyLockBytes`](https://github.com/777genius/universal-agent-plugins/blob/cbf20205ee56c8fa732ae594e015cbc9359ea217/cli/internal/agentpluginscli/deps_resolver.go#L41) использует `DisallowUnknownFields`, включая compatible-pin path. Поэтому каждый Skill pin обязан сериализовать новые поля явно: не прятать kind/normalizer внутри старого строкового поля или только во внешнем cache. Эти readers откажут до replay вместо трактовки Skill как обычного Git package. Новый reader отклоняет неизвестный kind/normalizer; lock validation проверяет source kind и его key. Это проверка кода указанной ревизии, не утверждение о всех исторических binaries.

`metadata.version` можно показывать как строку автора. Она не становится SemVer constraint, release sequence или authority установленной версии. Фактический pin — immutable commit плюс проверяемые digests.

## 4. Сначала совместимость

Закреплённый profile Agent Skills уже указывает на текущий upstream `69ef37e9424c0a7ea9dd2293b559e43ec8176379`. Обновлять pin без новой фактической ревизии не нужно.

Согласовать installer/authoring по одному нормативному контракту: имя и совпадение каталога, Unicode-политика согласно подробному тексту спецификации, длины, string-to-string metadata, opaque неизвестные поля и сохранение исходных bytes. Общие bounded parsing primitives разместить в существующем conformance boundary, без зависимости installer от authoring CLI. Отдельно отличать invalid input от safety budget/unavailable evidence.

`allowed-tools` сохраняется как данные и не даёт installer разрешений на выполнение. Рекомендации по размеру body не превращаются в новые нормативные запреты. Не добавлять автоматическое исправление невалидного YAML из ненормативного client guide. Сохранение scripts/references/assets проверять на реальном внутреннем artifact, а не только на DTO парсера.

Этот checkpoint должен пройти до объявления поддержки raw Skills. Frontend может разрабатываться одновременно против согласованных fixtures.

## 5. Сканер вложенных repositories

Расширить существующий `adapters/sourceacquisition`: shallow fetch с фильтрацией blobs и просмотр Git tree на точном SHA. Не вызывать `AcquireGitHub(repo, sha, "")` для discovery: этот путь делает checkout/snapshot всего repository. Использовать существующий изолированный Git Runner и временный workspace; новый downloader/credential reader не нужен.

Алгоритм:

1. Один раз разрешить selector в commit и получить Git tree через bounded `ls-tree -r -z`. Для непустого scope сначала разрешить точное subtree на этом commit и перечислять его: entry/metadata/depth budgets применяются внутри scope, а не к корню до фильтрации. Полный cached tree пригоден только при доказанной completeness; усечённый root listing не заменяет scoped scan. Рассматривать только tracked entries, вернуть repository-relative paths.
2. Найти все пути с basename ровно `SKILL.md`, в том числе глубоко вложенные `.agents`, `.github`, `.curated`, `.system`, категории и nested Skills. Наличие внешнего Skill не останавливает поиск внутри его дерева. Не исключать `examples`/`tests`/`vendor` по догадке.
3. Проверить размер blob через `cat-file -s <oid>` и зарезервировать aggregate bytes **до** запуска чтения, затем читать документ по object ID с bounded output. Использовать `errgroup` limit 2 вместо собственного worker-pool engine; invalid metadata не отменяет здоровые siblings. Текущий Runner не принимает stdin, поэтому первоначально допустимы отдельные `cat-file blob <oid>` вызовы. Batch вводить только если измерения покажут реальную проблему.
4. Общим decoder получить имя/описание/диагностику. Invalid/unreadable/too-large различаются; неподходящие кандидаты видны с причиной и недоступны для установки.
5. Собрать плоский immutable catalog кандидатов и родительских folder paths. Дерево UI строится из этих путей: произвольные файлы repository в каталог не отправляются.

Предлагаемые scan-policy v1 defaults: 100 000 tree entries, 16 MiB tree metadata, depth 64, 2 000 кандидатов, 1 MiB на документ, 16 MiB aggregate документов, deadline 120 секунд и context cancellation. Это прикладные пределы discovery, а не обещание ограничить весь Git transport/object disk usage. Сохранить текущие acquisition limits и cleanup; до реализации проверить реальные knobs Runner.

Symlink/gitlink `SKILL.md` не читать как обычный blob; показать диагностику, наружные filesystem paths не обходить. Порядок catalog детерминирован по repository-relative пути. Отдельный persistent search index не нужен.

Разбирать NUL-delimited tree records, не строки вывода/цитированные display paths. Непредставимый или unsafe path не нормализовать в другой допустимый путь; сохранить безопасную диагностику и отметить неполноту соответствующего discovery. Размер listing должен быть явно поддержан bounded Runner, а не предполагаться поверх его прежнего лимита 1 MiB.

Catalog имеет `complete` и причины неполноты. Достигнутый предел или недоступные metadata нельзя показывать как «в репозитории больше ничего нет». Можно установить явно выбранного уже проверенного кандидата из частичного каталога; для расширения пользователь сужает папку/область. Автоматическое relocation по уникальному имени требует полного результата по repository, а не только текущему folder filter.

Явное «Сузить сканирование» создаёт новый catalog в той же session на **том же SHA** через `POST /api/v1/catalog`, без повторного ref lookup. Разрешён только проверенный repository-relative `scopePath`. Это отдельное действие от локального folder/search фильтра: перед новым scan UI предупреждает о сбросе текущего выбора и plan; после него меняются catalogId/digest и Query key. Во время install rescan запрещён. Полный raw Git tree можно переиспользовать из bounded session cache; старый catalog не остаётся второй mutable authority.

Сканирование и materialization разделены: SKILL.md metadata читаются для списка; preview использует bounded SKILL.md из session cache/blob и resource inventory из Git tree, без скачивания всех assets. Полный выбранный subtree приобретается для plan/preflight/install. Branch не разрешается повторно между выбором и установкой. Если после чтения полного subtree изменились проверяемые facts, действие отказывает до client effects.

## 6. Простое relocation сразу

Использовать идею Vercel, но сравнивать нормативные имена точно: без lowercasing, замены `_`, fuzzy matching и анализа похожести текста.

На явном update moving selector разрешается один раз:

1. Проверить последний `resolved_skill_path` и совпадение expected name; затем исходный requested path, если он другой.
2. Если прежнее размещение не подходит, выполнить полный supported scan repository на том же SHA.
3. Один валидный кандидат с точным expected name — новое размещение. Сохранить новый resolved path в lock при сохранении source key, alias и managed ownership.
4. Ноль, несколько совпадений или неполный scan — ошибка разрешения по правилу batch из §2: установленный pin остаётся прежним, новый candidate graph/lock не применяется. Это не успешный `unchanged` update. Другой Skill в прежней папке не подставлять.

Никакого отдельного постоянного global Skill ID и Git rename-history анализа. Изменение имени/репозитория — явное изменение dependency. Закреплённый commit не продвигается. Locked replay вообще не выполняет ref lookup, scan или relocation.

## 7. Нормализация и reuse установщика

Явный adapter raw Skill → standard skills-only package:

- Получает sealed subtree выбранного Skill; проверяет имя и все действующие containment/size правила.
- Создаёт детерминированный root `plugin.json` с ASCII physical-safe именем, полученным из стабильного Skill SourceKey по §3, а не из acquisition SourceIdentity с SHA/resolved path; исходное имя Skill сохраняет в `skills/<name>/`. Alias, timestamps, машинные пути и ambient state не входят в bytes.
- Копирует всё допустимое дерево Skill с исходным `SKILL.md`, executable bits и ресурсами. Markdown/метаданные не переписываются, ссылки не «лечатся». Ресурсы вне selected subtree не скачиваются автоматически.
- Фиксирует raw digest отдельно, затем существующие artifact tree/manifest digests. Одинаковые raw bytes + стабильный Skill SourceKey + normalizer version дают одинаковый artifact на всех платформах.
- Передаёт результат обычным loader/security/environment/store/planner/stager/lifecycle. Вложенный `uap.toml` в ресурсе не превращается в root package dependency manifest.

В `deps_staging` нужен явный Skill branch **до** текущего `AcquireGitHubVerified(..., lock.TreeDigest)`: recorded commit/resolved path → raw digest → locked normalizer → artifact digests. Сравнивать raw дерево с artifact digest запрещено. Snapshot/cache keys различают raw source, kind/name/requested+resolved path и normalizer.

Repair должен восстанавливать принятую **конкретным target** revision, даже если общий lock уже продвинулся, а другой target сохранил прежний partial outcome. Поэтому минимальные Skill source facts (kind/name, стабильный requested anchor, точные commit/resolved path, raw digest и normalizer) сохраняются в существующей provenance принятой per-client revision вместе с artifact digests. Это не второй lock/resolver: данные записываются из проверенного результата acquisition, а repair читает их без ref lookup/relocation. Не выводить прежний raw source из нового общего lock, synthetic manifest name или наличия cache. Отсутствующие/неизвестные facts дают безопасный отказ с инструкцией восстановления; уже установленные plugin revisions сохраняют прежнее поведение.

Store hit сохраняет действующие security/eligibility/ownership проверки. Synthetic manifest не делает прямой Git source подписанной Directory publication. Общий installer не обходит чужие файлы, drift, pending recovery или revoked source policy. `sync --locked` при missing store восстанавливает прежние bytes либо отказывает; неизвестный normalizer не заменяется «последним».

## 8. Общий небольшой HTTP-контракт для picker

Единственный editable wire contract — `contracts/skills-picker/openapi.yaml` (OpenAPI 3.0.3). Backend lane владеет им и общими examples/fixtures. Из него генерировать Go DTO в `cli/internal/agentpluginscli/skillspicker/apitypes.gen.go` и TS declarations в `ui/skills-picker/src/shared/api/generated/schema.d.ts`. Generated files коммитятся, вручную не редактируются; один узкий generation target в существующем build process и regeneration-diff check ловят drift. Небольшие fixtures проверяют фактическую сериализацию handlers, а не копируют определения types. Не заводить второй scanner/parser/manifest editor в TypeScript.

Генераторы не заменяют validation: `MaxBytesReader` для request body, strict `encoding/json` decode с запретом неизвестных полей и лишнего trailing document, проверка enums, candidate/target IDs, lengths и limit selections. Responses из Go имеют обязательные массивы как `[]`, не `null`; отсутствующие optional поля опускаются; `sourceRevision` опущен только до resolution, даты — UTC RFC3339 strings. `apiVersion = 1`, ошибки `{code,message,retryable}` без provider logs/секретов. В OpenAPI отметить required/optional/enum и размеры, не использовать `any`/свободные extension maps для внутренних actions. Транспортные types не становятся policy-моделями.

| Запрос | Контракт |
| --- | --- |
| `GET /api/v1/session` | `{apiVersion,sessionId,state,source:{repository,sourceRevision?,scope},scan:{discovered,validated,bytes},catalogId?,operationId?,expiresAt}`; operationId сохраняет последнюю принятую операцию, включая terminal; polling только при активной scan/install операции |
| `GET /api/v1/catalog` | finalized catalogId/digest, repository, immutable SHA, scope, complete, counts, candidates, diagnostics |
| `POST /api/v1/catalog` | `{scopePath}` → HTTP 202; новый bounded scan в той же session/SHA; предыдущие selection/непринятые plans инвалидируются |
| `GET /api/v1/candidates/:id/preview` | catalogId/candidateId/path, bounded content/format/truncated, optional rawContent, относительный resource inventory и diagnostics на том же SHA |
| `GET /api/v1/targets` | допустимые target IDs, labels, scopes, display paths и причины недоступности; актуальность повторно проверяет plan/preflight |
| `POST /api/v1/plans` | catalogId/catalogDigest + selectedIds + targetIds → planId, expiresAt, actions, conflicts, warnings, confirmable; без persistent mutation |
| `POST /api/v1/plans/:id/apply` | явное нажатие Install, один operationId; повтор того же plan возвращает тот же operation, не запускает второй install |
| `GET /api/v1/operations/:id` | planId, state/phase/completed/total, items по Skill/target, diagnostics, result с aliases/paths и partial/repair instructions; polling только до terminal outcome |

Preview `content` содержит Markdown body без frontmatter; `rawContent`, если передан, — исходный SKILL.md для текстового просмотра. Разделение выполняет общий backend decoder, не YAML-парсер frontend. Для text-only/error fallback явно использовать `format=text`; обрезка помечена `truncated` и не выдаётся за полное содержимое. Оба представления укладываются в предел документа/ответа; rawContent не означает разрешения исполнять HTML или загружать ресурсы.

Candidate: `{id, name, description, path, status, diagnosticCodes}`; ID привязан сервером к session/SHA/path, не конструируется frontend из произвольного пути. Catalog digest связывает проверенные metadata и completeness. Folder/search — проекция данных, не новая source authority.

Candidate status: `available|invalid|unavailable`; только available допускает выбор. Это результат metadata discovery, а не проверка всего artifact/security или разрешение installation: их выполняет обычный plan/preflight. Source/catalog scope — repository-relative scan root (пустая строка = весь repository), а не target/profile scope. Targets сохраняют поддержанный environment scope; проектную активацию API не вводит. Все эти enums/optional поля входят в единственный OpenAPI contract.

Operation states: `queued|running|succeeded|failed|partial`; последние три terminal, partial сохраняет фактические per-item outcomes. Item содержит candidate/target ID, status и безопасную причину. Diagnostics имеют code/severity/message и optional path/candidateId. Ошибки `409 PLAN_STALE|CATALOG_MISMATCH` требуют нового review, `409 OPERATION_IN_PROGRESS` указывает на принятую operation, `410 SESSION_EXPIRED` завершает session. Не выводить completeness из counts и success из HTTP 202.

`planId` — единственный session-bound ключ подтверждения и идемпотентности; отдельные confirmationToken/planDigest/idempotencyKey не нужны. Сервер держит expected manifest/state facts внутри плана. Записи принятых plans/operations и results живут до завершения session; session.operationId указывает на последнюю принятую operation до следующего apply, даже если она закончилась раньше восстановления HTTP-связи. Operation содержит planId: клиент сопоставляет результат с ожидаемым планом. Actions: `install|update|unchanged`, опциональный `sourceMoved: {from,to}` и стабильные reason codes. Conflicts блокируют confirmable; неизвестный action не подтверждается старым UI. Relocation — часть обычного update, отдельного HTTP endpoint нет. Канонические contract fixtures положить рядом с новым Go bridge в `cli/internal/agentpluginscli/skillspicker/testdata/api-v1/`; frontend tests читают их из этого места, не редактируют копии.

Отсутствие ожидаемой operation в ответе session после сетевого сбоя не доказывает, что исходный apply уже не будет принят. Клиент может предложить явный повтор **того же** apply с тем же planId: сервер либо вернёт имеющуюся operation, либо примет его один раз после обычных проверок. Если непринятый план уже истёк, требуется новый review; автоматического повторного подтверждения нет. Session expiry и старый operation другого плана не считаются успешным результатом ожидаемого apply.

Session states: `resolving|scanning|ready|installing|done|failed`. Обычный путь: resolving → scanning → ready → installing → done/failed; succeeded даёт done, failed/partial — failed с сохранением operation result. Явный rescan из готового каталога снова проходит scanning. После terminal install outcome успешная явная подготовка нового review возвращает ready со свежими facts; pending recovery обрабатывает существующий installer, без автоматического retry. При повторном apply уже принятого planId сервер сначала возвращает прежний operation; expiry/stale checks выполняются перед первым принятием. Атомарный session gate допускает только одну active operation: другой planId получает `409 OPERATION_IN_PROGRESS`; остальные непринятые планы инвалидируются. Между разными sessions остаются существующий environment mutation lock и повторный preflight, без второго глобального lock manager. Это in-session идемпотентность; после аварии процесса используются persisted receipts/recovery, без автоматического нового apply.

Bridge обслуживается только на `127.0.0.1`/random port и на время CLI-сеанса. Передать случайный 256-bit capability во fragment начального browser URL; bootstrap прочитает его **до** Router, очистит fragment через history replacement и хранит только в tab-scoped `sessionStorage` этого origin для refresh. `openapi-fetch` добавляет `Authorization: Bearer` к same-origin API; не использовать cookies, Router query, localStorage, логи, TOML/lock или provider URLs для capability. Проверять точный Host, для mutations — Origin; CORS закрыт. Это один локальный capability, не authentication framework.

Idle lifetime — 15 минут с последнего авторизованного API request; running scan/install удерживает session. При refresh bootstrap использует sessionStorage, `GET session` восстанавливает последнюю принятую operation, включая уже завершённую; `410 SESSION_EXPIRED` очищает локальный token и останавливает polling. Вкладка не переоткрывает CLI автоматически. Static assets локальные, CSP без внешних scripts/connect, без CDN. Mutation принимает проверенные IDs, а не shell command/произвольный filesystem path. Preview не исполняет HTML, scripts или удалённые ресурсы.

Подготовка plan может получать исходники и строить artifact только во временном workspace; она не публикует их в final store/scanner cache и не меняет manifest/lock, environment state, receipts или профили. Подготовленные bytes принадлежат непринятому плану и удаляются при expiry/session cleanup. При apply владение переходит операции: повторный preflight проверяет свежие facts, затем existing durable flow проверяет/promote artifact и выполняет изменения. Если bytes не удержаны, восстановить их по тому же sealed source; не fetch новый HEAD. Это cleanup существующих временных ресурсов, без отдельной lease platform.

Перед apply повторно проверить manifest/state expectations обычным installer preflight. Устаревший plan возвращает actionable stale status и новый review; не применять selection к другому SHA. При неопределённом HTTP-исходе фронт читает тот же operationId; mutation retry не порождает новый запуск. Закрытие вкладки до apply не меняет окружение; после apply CLI доводит уже принятую операцию и сохраняет receipts по правилам времени жизни §1.2. Нет обещания общей атомарности всех native activations: partial outcome отражает существующий lifecycle.

## 8.1 Встраивание UI и сборка

Go `embed` не может читать `../../ui`: frontend `dist` копируется build step в `cli/internal/agentpluginscli/skillspicker/webdist/`, и только этот локальный package path встраивается в бинарник. Source checkout содержит маленький development placeholder, чтобы обычные Go tests компилировались без Node; такой binary явно предлагает `--no-ui`, а не изображает готовый picker. Release build обязан заменить placeholder настоящими Vite assets и пройти smoke на bundled UI. Generated assets отдельно от authored LOC, без CDN/runtime npm.

Один рукописный TypeScript build helper собирает frontend, переносит assets с manifest/digests и исключает fixture adapter из production. Не добавлять второй frontend/server framework. Выбранный CI Node должен удовлетворять Vite engines (исследованный Vite 8.3.5: Node `^20.19.0 || >=22.12.0`); runtime готового Go binary Node не требует. ServeMux отдаёт только bundled files и SPA routes; asset request не становится filesystem path из произвольного URL.

## 9. Реализация параллельно и review boundaries

| Checkpoint / owner | Начинается после | Наблюдаемый результат завершения |
| --- | --- | --- |
| **K0 — совместимость и общий контракт**, backend | Узкой сверки current main | Installer и authoring дают согласованный нормативный verdict на общих Skill fixtures; прежние plugin vectors сохранены. Source/lock и wire contract определены, Go/TS generation воспроизводима. Реальная установка raw Skills открывается только после compatibility checks. |
| **A — один Skill через existing installer**, backend A | Source contracts K0 | Минимальный CLI путь `skills add owner/repo --path … --no-ui` с явными selector/targets устанавливает Skill, сохраняет TOML/lock и выполняет cold locked restore. Этот вертикальный результат не зависит от browser, полного catalog scan или интерактивного выбора. CLI adapter для него входит в A. |
| **B — discovery, relocation и bridge**, backend B | Source contracts K0; интеграция relocation требует A | Repository/folder/file URLs и nested scan дают определённый catalog/completeness; batch selection использует A. Moving-ref relocation сохраняет identity, HTTP bridge соблюдает §8. B не дублирует resolver или минимальный CLI adapter A. |
| **F — browser picker**, frontend | Wire contract/examples K0, параллельно A/B | Tree/search/basket/review/progress работают на общих fixtures с keyboard/responsive checks. Это готовность frontend к соединению с bridge; fixture adapter не доказывает реальную установку. Внутреннее ownership описано в frontend-плане. |
| **I — объединённая поставка**, backend + frontend | Готовых A/B/F | Собранный Go binary отдаёт production UI; выбор двух Skills из разных папок проходит review → apply → terminal result и cold restore в disposable environment. Fixture adapter исключён, runtime Node не нужен, help/docs соответствуют фактическому поведению. |

Это критерии завершения имеющихся этапов, а не дополнительные review gates. Использовать проверки §10 и frontend-плана: не создавать новый набор тестов на каждую строку таблицы. После определения wire contract frontend может начинать fixtures независимо от длительности compatibility/backend работ; публичная поддержка raw Skills остаётся закрыта до их прохождения. Код frontend не зависит от старой Vue landing архитектуры. Release qualification проходит по существующему процессу, без ложных availability claims.

Каждый checkpoint должен оставаться coherent/reversible. Ориентир — bounded PR около 2 000 changed LOC, без искусственного разрезания source/lock/replay инварианта. Shared contracts редактирует один owner; остальные lanes не дублируют их. Не создавать новые пользовательские чаты для subtasks.

## 10. Минимальные содержательные проверки

Новые unit tests только там, где защищают контракт: parser/identity различает plugin и Skill; unsafe path/неверный kind не достигает mutation; relocation выбирает единственное точное имя и не принимает partial scan за уникальность. Существующие сильные tests переиспользовать, не дублировать проверки общих getters/wrappers.

Ближайшие сильные integration/contract границы:

- Disposable repository с вложенными категориями, одинаковыми именами, `.agents` и 1 000 кандидатами: discovery полный, bounded и отменяемый; truncation явно отличается от пустоты, сужение scope сокращает перечисление до применения budgets. Действия с непроверенным candidateId не устанавливают ничего.
- Один install → TOML/lock → cold restore при более новом upstream commit: создать новый store/state/profile и перенести **только** TOML/lock; сохраняются исходные pin, executable bits и ресурсы. Отдельно в том же lifecycle-сценарии внести owned drift, убедиться в отказе обычного sync, выполнить repair и saved remove; чужие файлы не удаляются. Изменение одного ресурса меняет проверяемый artifact; изменение соседнего Skill не переустанавливает выбранный без причины.
- Partial update двух targets: один принял новую revision, другой сохранил старую; после удаления только disposable cache repair каждого восстанавливает его принятую revision по provenance, без adoption нового lock.
- Moving-ref update после переноса папки: expected Skill обновляется, alias/ownership, InstallationID/SourceBindingID, synthetic declared name и native PhysicalArtifactID сохраняются; `sync --locked` после этого восстанавливает новый resolved path без discovery. Смена content digest при изменении bytes допустима.
- Browser happy path и неопределённый apply response проверяются через тот же operationId; чужие/локально изменённые файлы остаются защищены existing lifecycle.
- Transport contract: два разных planId не создают параллельные effects; непринятый review и отказ одного выбранного источника на resolve/preflight не меняют persistent installer state; завершение HTTP 202 handler не отменяет принятые scan/install; refresh сохраняет авторизацию и находит даже быстро завершившуюся operation, явный повтор потерянного apply с тем же planId не создаёт второй effect; rescan не продвигает SHA; generated arrays/enums совпадают с реальным JSON. Это один небольшой contract набор, не unit test каждого endpoint/getter.

Тесты запускаются только на новых disposable fixtures/test roots. Ни реальные профили пользователя, ни live агентские runtime не нужны для этого этапа. Время cold Git network не выдавать за измеренную скорость сканера; отдельно измерить cached discovery и UI response.

## 11. Оценка объёма

Диапазоны добавочного changed LOC поверх проверенного main; total включает production, meaningful tests, документацию и конфигурацию, но не generated assets/lockfiles/vendor. Это оценка, не измеренный будущий diff.

| Участок | Production | Total |
| --- | ---: | ---: |
| Совместимость + source kind/TOML/lock/normalizer/replay | 900–1 500 | 2 000–3 200 |
| URL, scanner, CLI selection, relocation | 700–1 100 | 1 400–2 300 |
| Временный HTTP bridge + embedding/build wiring | 400–700 | 650–1 150 |
| Editable OpenAPI + генерация/config/contract checks | 150–300 | 250–450 |
| React picker, стили, accessibility, проверки — отдельный план | 2 600–4 200 | 3 900–6 200 |
| **Вся поставка** | **4 750–7 800** | **8 200–13 300** |

Строки таблицы разделены по ownership и суммируются; backend API/bridge/OpenAPI не посчитаны повторно во frontend. Изменения existing tests считаются один раз. Первая строка включает compatibility checkpoint. Generated Go/TS DTO, vendor assets и package lockfiles считаются отдельно. Из прежней UI оценки убрано около 400–600 authored строк собственных interaction mechanics; schema и реальные boundary fixes компенсируют часть экономии. Обещать двукратное уменьшение всего installer было бы необоснованно. Уверенность в диапазоне средняя; codegen ценен согласованностью, а не искусственным снижением LOC.

Рекомендуемая архитектура: 🎯 9/10 уверенность, 🛡️ 9/10 надёжность при указанных проверках, 🧠 7/10 сложность. Главные затраты — scanner/lock/artifact согласованность и качественный UI, а не CRUD scaffolding.

## 12. Источники и точки reuse

- [Agent Skills specification](https://agentskills.io/specification), [upstream pin](https://github.com/agentskills/agentskills/tree/69ef37e9424c0a7ea9dd2293b559e43ec8176379).
- [Agent Plugins §7.1](https://agent-plugins.org/specification#71-skills): embedding Skills в стандартный package; ordinary loader остаётся standard-first.
- [Vercel relocation](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/skill-relocation.ts), [folder locks](https://github.com/vercel-labs/skills/blob/a80e4c9d12d607631249d4d7d059f835d4b2f13a/src/skill-lock.ts). Используется идея поведения; автоматического копирования нормализации имён нет.
- Main: `install/integrationctl/agentplugins/environment/model.go`, `adapters/dependencymanifest`, `adapters/sourceacquisition/acquirer.go`, `cli/internal/agentpluginscli/{saved_add,deps_resolver,deps_replay,deps_staging}.go` на указанном baseline. Конкретная identity seam: [`domain/identity.go`](https://github.com/777genius/universal-agent-plugins/blob/cbf20205ee56c8fa732ae594e015cbc9359ea217/install/integrationctl/agentplugins/domain/identity.go#L27).
- Граница отказа всего batch до effects: [`environment/resolve_graph.go`](https://github.com/777genius/universal-agent-plugins/blob/cbf20205ee56c8fa732ae594e015cbc9359ea217/install/integrationctl/agentplugins/environment/resolve_graph.go#L81), [`deps_apply.go`](https://github.com/777genius/universal-agent-plugins/blob/cbf20205ee56c8fa732ae594e015cbc9359ea217/cli/internal/agentpluginscli/deps_apply.go#L106).
- [Существующий dependency plan](./uap-dependency-management-mvp.md), [единый manifest](./uap-unified-dependency-manifest.md). Их уже реализованные возможности не объявлять будущей инфраструктурой.
- [Go ServeMux](https://pkg.go.dev/net/http#ServeMux), [embed](https://pkg.go.dev/embed), [errgroup SetLimit](https://pkg.go.dev/golang.org/x/sync/errgroup#Group.SetLimit).
- [oapi-codegen 2.8.0, model generation](https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0#generating-api-models), [openapi-typescript](https://openapi-ts.dev/introduction), [openapi-fetch](https://openapi-ts.dev/openapi-fetch/). Версии/npm manifests и license проверены 2026-10-23, installation не выполнялась.
