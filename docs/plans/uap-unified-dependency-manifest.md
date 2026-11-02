# Единый манифест зависимостей UAP

Статус: архитектурное решение принято владельцем 2026-10-18; план уточнён после проверки через plan-improve и Q и передан в реализацию. Приёмка реализации определяется разделом 13; этот документ сам по себе не подтверждает прохождение проверок или публичный выпуск.

Цель: использовать один корневой `uap.toml` и одну секцию `[dependencies]` для требований плагина, личного окружения и будущего проектного окружения. При установке плагина UAP автоматически устанавливает его обязательные зависимости рекурсивно. Корень операции определяет targets и точные pins всего графа.

Это продолжение [плана dependency management MVP](./uap-dependency-management-mvp.md), а не повторная реализация installer, store или solver. В части источника пакетных требований и схемы TOML решения ниже имеют приоритет над старым планом и предыдущими примерами из обсуждения.

## 1. Принятые решения

1. Единственный источник новых пакетных требований UAP: `uap.toml` в корне конкретного пакета, рядом с `plugin.json`.
2. `[dependencies]` всегда означает прямые обязательные требования владельца манифеста. Расположение файла не меняет смысл секции.
3. Нет секций `package.dependencies`, `environment.dependencies`, разных типов манифеста или обязательной генерации TOML в JSON.
4. `plugin.json` единолично определяет стандартную идентичность и метаданные плагина. Имя, версия, `$schema`, компоненты и runtime configuration не переносятся в TOML и не дублируются там.
5. `[targets]` выбирает агентов текущего корневого окружения. Это не декларация совместимости плагина. При чтении вложенного пакета его targets не влияют на targets потребителя.
6. Один lock выбранного окружения фиксирует весь разрешённый граф. Вложенные `uap.lock` не участвуют в выборе pins потребителя.
7. Сохраняются один canonical source / одна revision в окружении, существующий SemVer resolver, target union, immutable store и typed installer lifecycle.
8. Не вводятся JSON fallback, YAML fallback, автоматическая конвертация старых пакетов, второй resolver или второй поддерживаемый источник деклараций.
9. Проектная установка остаётся следующим отдельным этапом. Новый пакетный TOML не даёт права писать в профили проекта или менять текущий глобальный root.
10. Dev requirements имеют отдельную будущую семантику и не маскируются под обычные dependencies. В текущем delivery `[dev-dependencies]` не включается: неизвестное поле должно вызывать понятную ошибку.

Внешний Agent Plugins и продукт Universal Agent Plugins различаются: `uap.toml` является контрактом менеджера UAP, а не новым стандартным компонентом Agent Plugins 1.0. Другие установщики не получают автоматическую поддержку dependency graph от одного наличия TOML или JSON extension.

## 2. Основа реализации и существующие планы

Проверенная основа: `c2693a4316f5b5fca3e556bc46fd9b3d9637afb6`. В ней присутствуют merge транзитивного графа PR #447 и исправление холодного воспроизведения PR #448. Наличие кода в этой основе не является утверждением о публичном выпуске. Перед реализацией обновить карту файлов относительно актуального main; не переносить чужие незакоммиченные изменения.

Transfer/import/export развивается отдельным workstream: соответствующие файлы отсутствуют в указанной основе и были отдельно замечены в более позднем commit `89ed476ef93e06198e726a1646f175a7281c664e` рабочего checkout. Это не утверждение о его merge или release. Если transfer вошёл в выбранную implementation base, адаптировать и проверить его; иначе этот план не поручает реализовывать transfer engine и проверяет перенос обычным копированием manifest/lock в существующий cold-restore сценарий.

| Участок на проверенной основе | Что уже существует | Необходимое изменение |
| --- | --- | --- |
| `cli/internal/agentpluginscli/deps_manifest.go` | TOML reader для root, Git/release/version selectors | Общий decoder и отдельная валидация root/package |
| `install/integrationctl/agentplugins/environment/model.go` | Requirements, source keys, manifest intent digest | Logical keys с canonical ID; пакетные requirements без target assignments |
| `install/integrationctl/agentplugins/conformance/plugin.go` | Чтение dependencies из JSON extension | Убрать исполнение этих данных из standard decoder; сохранить lossless JSON |
| `install/integrationctl/agentplugins/environment/resolve_graph.go`, `resolve_expand.go`, `graph.go` | Граф, constraints, minimum selection, target union и проверка lock edges | Принимать типизированные требования из TOML и убрать registry-only проверки рёбер |
| `cli/internal/agentpluginscli/deps_candidates.go`, `deps_resolver.go`, `deps_replay.go` | Acquisition, immutable replay и проверка declared edges | Читать package view из того же sealed snapshot во всех маршрутах |
| `cli/internal/agentpluginscli/manifestedit` | TOML editor с сохранением комментариев и quoted keys | Проверить новые logical keys без отдельного редактора |
| `cli/internal/agentpluginscli/saved_lifecycle.go`, `deps_tree.go` | Remove через `Coordinator{}` и связь root keys с aliases графа | Подключить verified package facts; сопоставлять declarations и узлы по source key |
| `cli/internal/authoring/project/project.go`, `commands.go`, SDK `adapters/packageview` | Authoring читает захваченные bytes через lease | Проверять TOML из того же capture, без повторного чтения source root |
| `adapters/packagedigest`, `adapters/environmentstore` в SDK | Tree digest и проверяемые snapshots | Сохранить включение `uap.toml` в исходные bytes и digest |
| `cli/internal/agentpluginscli/deps_transfer*.go`, если workstream включён в implementation base | Перенос root manifest/lock в отдельном изменении | Использовать тот же root decoder; сохранить strict locked preflight без реализации нового transfer engine |

В старом MVP-плане транзитивность и программная правка TOML ещё названы отложенными. Эти строки описывают прежний этап и не являются основанием удалить текущую реализацию. Соседние планы `uap-default-save-completion.md` и `uap-all-agent-dependencies.md` определяют уже существующие saved-command и target boundaries; этот delivery их не расширяет.

Полезные legacy-возможности сохраняются по `docs/AUTHORING_CAPABILITY_PRESERVATION.md`. VS Code остаётся заморожен; его adapters, dedicated fixtures и tests не изменять. Публикация CLI, npm или сайта не входит в подготовку или исполнение этого плана без соответствующего delivery scope.

## 3. Границы текущего delivery

### Входит

- Общая грамматика `uap.toml`, root/package views и диагностика с путём и строкой.
- Канонический shortcut вида `"acme/context" = "^2.0"`, включая root targets, которые ссылаются на такой ключ.
- Пакетные requirements в TOML и их транзитивное разрешение через существующие source kinds.
- Повторное чтение declarations при replay, cold restore и проверке lock.
- Переключение normal saved `add/update/remove/install/deps` на один источник пакетных требований.
- Явный отказ на устаревший JSON dependency field вместо незаметной потери рёбер.
- Корректное сохранение TOML в существующих путях упаковки/копирования и проверка готового артефакта.
- Актуальные примеры, author validation и минимальные регрессионные проверки поведения.

### Не входит

Проектная активация, новые client adapters, editable/local sources, private Git authentication, новые Git hosts, dev groups, compatibility matrix в TOML, install hooks, автоматический garbage collection, новый publisher, восстановление замороженных authoring phases 9-11 или отдельный build engine. Существующие paths и поддержка этих возможностей не удаляются.

Установка зависимостей не обещает полностью зафиксировать Node/Python, секреты, модели и внешние сервисы. Языковые manifests продолжают описывать языковые зависимости; UAP dependencies описывают плагины UAP.

## 4. Схема манифеста

Сохраняется `schema_version = 1`: это формирование первого публичного контракта этой функции. Новый обязательный version bump не нужен лишь из-за добавления package view. Уже опубликованные release assets и теги не переписываются.

Корневые поля текущего delivery: `schema_version`, `dependencies`, `targets`. Неизвестные поля, duplicate TOML keys, неподдерживаемая версия, неверный тип значения или selector возвращают ошибку до эффектов. Отсутствующая `[dependencies]` равнозначна пустой. Если файл существует, `schema_version` обязателен.

### 4.1 Личное окружение

Файл берётся из существующего `AGENTPLUGINS_HOME`; новый resolver домашнего каталога не создаётся.

```toml
schema_version = 1

[dependencies]
"acme/review" = "^1.0"
"acme/docs" = "2.1.0"

[targets]
codex = ["acme/review", "acme/docs"]
claude = ["acme/review"]
```

Обе записи являются direct roots. Targets `review` наследуются его транзитивными зависимостями. `docs` получает только Codex, если другой потребитель не требует его для другого target.

### 4.2 Обычный проект после добавления project scope

```toml
schema_version = 1

[dependencies]
"acme/api-review" = "^3.0"

[targets]
codex = ["acme/api-review"]
```

Этот пример определяет будущую совместимость формата, а не включение проектной установки сейчас. Добавление рядом `plugin.json` не переписывает файл и не меняет трактовку root operation. Только явная установка этого каталога как пакета экспортирует его обязательные requirements.

### 4.3 Публикуемый плагин `acme/review`

```toml
schema_version = 1

[dependencies]
"acme/context" = "^2.0"
```

Имя и версия `review` находятся в `plugin.json`. Отсутствие targets здесь нормально: это package view. Файл ищется только в точном package root, с учётом source `path`, а не обязательно в корне Git-репозитория.

Если у `context` аналогично объявлен `acme/storage`, установка `review` добавляет `review`, `context` и `storage`. Повторное перечисление транзитивных requirements в родительском TOML не требуется.

### 4.4 Плагин с настройками рабочего окружения автора

```toml
schema_version = 1

[dependencies]
"acme/context" = "^2.0"

[targets]
codex = ["acme/context"]
```

Когда этот файл впоследствии выбран как root окружения автора, targets действуют. Когда пакет устанавливает другой пользователь, вся секция targets остаётся без authority: `context` получает targets потребителя `review`, даже если автор проверял его только с Codex.

Тестовые помощники автора не должны попадать в `[dependencies]`, если потребителю они не нужны. До отдельной реализации dev requirements их можно держать в личном окружении автора. Будущая `[dev-dependencies]` будет читаться только у выбранного root при явном dev-режиме и не передаваться транзитивно; её формат lock и CLI activation надо реализовать отдельным связанным изменением, а не молча игнорировать поле сейчас.

### 4.5 Существующие длинные формы источников

Они сохраняются для root и package declarations; новый decoder не ограничивается registry strings.

```toml
schema_version = 1

[dependencies.review]
git = "https://github.com/acme/agent-tools.git"
path = "plugins/review"
commit = "0123456789012345678901234567890123456789"

[dependencies.docs]
registry = "uap"
distribution = "acme/docs"
release = 42
```

Сохраняются текущие правила: `commit` либо `ref`, иначе стабильный default marker; только поддержанный GitHub source; `path` задаёт пакет внутри repository; registry table задаёт точный directory release. Не добавлять inline `version` к source table без отдельного решения: SemVer shortcut уже покрывает этот сценарий.

Для публикуемых package declarations registry identity должна быть канонической. Shortcut `context7 = "^1.2"` сохраняет существующее удобство поиска только у пользовательского root; в пакете нужен `"owner/name"` либо table с каноническим `distribution`. Пакет не должен менять смысл после появления другого short-name кандидата в Directory.

## 5. Logical keys и identity

Текущий `Requirement.Alias` одновременно служит ключом root declaration и именем узла графа, а `validateGraph` ищет direct roots по alias. Для canonical shortcut нужно разделить эти роли в policy и сопоставлении; простая замена regex не решает задачу.

| Значение | Назначение и authority |
| --- | --- |
| Declaration key | Ключ в TOML и `[targets]`, пользовательский selector и часть нормализованного root intent |
| Canonical source key | Объединение требований и сопоставление root declaration с единственным locked node |
| Node alias | Стабильная ссылка в `LockedPackage.Alias`, `required_by`, receipts и существующем installer coordinator |
| Native identity | `plugin.json.name`, installation ID, binding ID; не выводится из нового TOML key |

- Declaration key допускает существующий безопасный alias либо canonical Directory ID `owner/name`. Canonical shortcut сохраняется целиком в TOML и targets; не превращать его автоматически в новый owner alias.
- При наличии previous pin и matching owner receipt source сохраняет node alias, installation ID и binding IDs. Добавление транзитивного source в direct roots, удаление direct declaration при оставшемся потребителе и изменение только targets не являются переименованием или reinstall.
- Для нового узла короткий root alias сохраняет существующее поведение; canonical-key root и transitive source получают текущий default alias из immutable package metadata. Конфликт разных source identities на один node alias остаётся явным отказом до Apply. Автоматический rename или overwrite не вводится.
- Root validation, `validateGraph`, target propagation, saved add/update/remove и tree сопоставляют declaration с pin через canonical source key. Изменение display/declaration key не требует миграции receipt identity. Неподдержанная явная операция переименования owner alias по-прежнему отклоняется.
- Root semantic digest вычисляется по declaration keys, source keys, selectors и target assignments и не зависит от того, какой alias выбрал предыдущий lock. Persisted mapping не нужен: root -> pin восстанавливается через source key, уже имеющийся с обеих сторон.
- `deps tree` JSON `roots` продолжает ссылаться на aliases существующих `packages`, а не становится массивом неразрешимых TOML keys. Если declaration key отличается, вывести дополнительное соответствие key/source/alias; human output показывает пользовательский key вместе с причиной установки. CLI selector по canonical key разрешается через то же соответствие; неоднозначность не выбирает случайный receipt.
- Canonical IDs нормализуются к lower case при сравнении source identity. Два declaration keys одного документа, нормализующиеся к одному source key, отклоняются как дубликаты и в root, и в package view; constraints разных потребителей объединяются на уровне графа.
- Source identity по-прежнему repository + package path либо Directory distribution ID; revision и declaration alias не входят в stable source key. Package-local alias не навязывает окружению installation alias.
- Slash в declaration key не является разрешением использовать его как путь. Продуктовые директории и receipt IDs продолжают строиться существующими безопасными механизмами; не расширять path/name validators глобальной заменой regex.
- TOML editor обязан round-trip сохранять quoted keys с `/` и `.`, comments и line endings. Ограничение существующего editor на неподдерживаемые layouts сохраняется как явная ошибка до записи.

### 5.1 Короткие имена и offline-сопоставление

Короткий root shortcut ещё не является canonical source key. Сначала применять существующий Directory lookup, сохраняя исходный declaration key и targets, затем повторно проверять уникальность sources. Например, `docs = "^2.0"` и `"acme/docs" = "^2.0"` в одном root должны дать ошибку, если lookup раскрывает `docs` в `acme/docs`; не объединять такие declarations и их targets автоматически. Package view не делает lookup и сразу требует canonical registry identity.

В `decodeDependencyLockBytes` и `canonicalizeRequirementsFromLock` сейчас используется совпадение `pkg.Alias == declaration key`. После разделения identity этого недостаточно. Для offline/locked чтения использовать уже записанные registry coordinates lock: сохранить однозначное существующее сопоставление старых коротких keys, а при отличающемся node alias допускать только единственный подходящий canonical source по сохранённым product/distribution facts. Затем обязательно выполнять обычную полную проверку lock, selectors, graph и manifest digest; совпадение basename само по себе не делает lock допустимым. При отсутствии или неоднозначности соответствия вернуть понятный mismatch; `status/tree` не обращаются в Directory и не исправляют intent. Пользователь может явно записать canonical source в manifest и выполнить обычную проверку/lock operation.

Это расширение существующего сопоставления, без новой таблицы имён в lock, эвристического выбора первого кандидата или сетевого lookup в read-only commands. Для mutating commands lookup разрешён существующим workflow; resolved source identity дальше используется одинаково во всех потребителях.

## 6. Один parser и две контекстные проверки

В SDK добавить небольшой TOML adapter, например `adapters/dependencymanifest`, используя уже закреплённый `pelletier/go-toml/v2`. Это размещение является точкой расширения, а не требованием создать новый module.

Decoder принимает bytes и имя источника, возвращает типизированный документ с source requirements и расположениями ошибок. Файловые операции, Directory lookup, root selection и установка в decoder не входят. Типы requirement/selector принадлежат существующей policy boundary, не библиотеке TOML.

Достаточно одного decoder и двух обычных функций контекстной проверки. Не создавать registry форматов, иерархию manifest classes, новые wrappers вокруг каждого существующего DTO или второй source provider: разные views защищают права root/package, а не требуют отдельных engines.

**Root view** проверяет source declarations и target references, допустимость targets через текущую CLI/SDK authority и наличие назначения у каждого direct root. Непустой root без targets не превращается автоматически в установку для всех обнаруженных агентов.

**Package view** проверяет source declarations без требования target assignments. Секция targets проверяется как структурно корректное отображение строк в массивы строк, затем не участвует в package requirements. Нельзя проверять её через доступность агентов на машине потребителя, использовать как allowlist совместимости или создавать bindings для её значений.

Пример границы, имена API иллюстративные:

```text
Decode(bytes, source) -> Document
Document.AsRoot(targetAuthority) -> environment.Requirements
Document.AsPackage() -> PackageRequirements
```

Не вызывать `environment.Requirements.Validate()` для сырого package document: на текущей основе эта функция требует root targets. Общая валидация dependency expressions должна переиспользоваться между views; DOM/TOML types не попадают в graph solver.

Нет package TOML: нет обязательных UAP requirements, если не обнаружена устаревшая JSON declaration. Файл только с `schema_version = 1` корректен и также означает отсутствие requirements. Файл нулевой длины некорректен: обязательная версия отсутствует. Существующий, но нечитаемый/неверный TOML не равнозначен отсутствующему. Package/author validation не обращается в Directory: наличие canonical declarations проверяется локально, доступность и совместимость выбранных releases проверяются при managed resolution.

## 7. Загрузка пакета и граница стандарта

1. Приобрести и проверить источник существующим acquisition path.
2. Получить immutable package snapshot с source identity, tree digest и executable inventory.
3. Загрузить `plugin.json` стандартным loader. TOML не превращает native-only/YAML каталог в стандартный пакет и не меняет schema selection.
4. На уровне UAP package-management прочитать ровно `snapshot.Root/uap.toml` через существующую contained-file policy и получить package view.
5. Передать metadata стандартного пакета и типизированные requirements resolver. Ни один target adapter не парсит TOML самостоятельно.
6. Разрешить полный граф, выполнить существующий общий preflight и только затем переходить к Apply.

Standard conformance decoder сохраняет raw extensions и raw bytes, но не делает UAP dependencies частью portable `PluginManifest`. Source requirements возвращаются отдельным manager-level результатом. Не менять `ManifestDigest`: это по-прежнему digest сырых bytes `plugin.json`; TOML защищает tree digest всего пакета.

Для installer/store reader это bytes sealed snapshot. Для authoring использовать существующий `project.Service` и тот же `packageview.Lease`: получить `uap.toml` из captured inventory/content, как сегодня читаются commands. Не добавлять `os.ReadFile(exactRoot/uap.toml)` после Capture или после закрытия lease. Отсутствующий файл, unreadable/blocked entry, неверный тип и источник, изменившийся при capture, различаются; unavailability не выдаётся за отсутствие dependencies. Захват подчиняется текущим filesystem containment и file/tree byte limits, без нового безлимитного reader. Проверка legacy JSON key использует уже decoded raw extensions, а не второй независимый JSON parser.

Agent Plugins §8 определяет namespace для client extensions, а официальное руководство относит installation/distribution вне portable contract. Существование дополнительного root-файла само по себе не запрещено; применение root sidecar как manager contract не следует выдавать за стандартную dependency semantics или за автоматически доказанную полную conformance всего дополнительного поведения. При реализации описать границу в текущем ADR новым принятым решением/дополнением, сохранив историю, а не переписывая старый ADR задним числом. Если обнаружится конкретное нормативное противоречие, исправить этот пункт до cutover; не делать namespace-generation pipeline молча.

## 8. Граф и source selectors

Заменить `ResolvedPackage.Dependencies map[string]string` типизированным набором requirements. Это необходимо, чтобы одинаковый TOML действительно поддерживал existing Git/release sources и package declarations не имели отдельного урезанного parser.

**Verified facts обязательны.** Для normal managed resolution/replay непустого root `Coordinator` требует resolver с package-description capability существующего `GraphResolver`. Resolver-only SDK вызов получает typed `dependency_facts_unavailable` до выдачи lock; отсутствие capability нельзя интерпретировать как пустой пакет. Удалить normal fallback из `graphSolver.describe`, который сейчас восстанавливает declarations из `previous.Lock`, и ветку `chooseExact`, считающую свежий пакет leaf без description. Low-level direct source resolver может остаться отдельным primitive, но не обещает полный managed graph.

Текущий saved remove вызывает `Coordinator{}.Resolve`; его необходимо перевести на existing `dependencyResolver` для retained graph. Это связано с cutover, а не необязательный refactor. Проверяются declarations оставшихся узлов с сохранением compatible pins, без refresh moving refs. Пустой новый root может вернуть пустой lock без facts provider: он не утверждает наличие какого-либо leaf или retained edge. Отдельный публичный prune engine не вводится.

- Expansion использует canonical source keys, а не package-local aliases, для объединения узлов.
- Каждое ребро несёт consumer identity и исходный typed selector. Убрать предположение, что у любого child requirement обязательно есть `Registry`, не только в expansion, но и в `environment/graph.go`, replay и locked-selector validation. В проверенной основе `graph.go` принимает required_by лишь для registry version/range; простая замена parser без этой правки недостаточна.
- Registry version/range пересекаются существующим solver. Если хотя бы один demand задаёт exact release, сначала фиксируется этот release и его immutable version сверяется со всеми version/range demands. Разные exact releases одного source конфликтуют даже при одинаковой version string.
- Для Git одного canonical source в этой версии требуются одинаковые selector kind/value. Разные refs или сочетание ref/commit считаются конфликтом, а не обещанием, что они случайно указывают на одинаковый commit. Не создавать дополнительный resolver для доказательства эквивалентности refs.
- Existing compatible pins имеют приоритет; свежий registry lock выбирает минимальную совместимую stable version и затем release sequence согласно текущему контракту. Target-only edits не продвигают revisions.
- Target child = union targets всех потребителей плюс собственные direct assignments, если этот source одновременно указан root.
- Сохраняются все immediate `required_by` edges, включая diamonds и циклы. Текущий fixed-point обход циклов должен завершаться без повторного приобретения одних pins.
- `update X` меняет выбранный root и необходимую часть его closure, сохраняя независимые pins. Exact selectors меняются через явное изменение intent.

Ошибка constraints перечисляет canonical source, потребителей и selectors. Она не допускает установку только совместимой части нового графа. Native Apply после успешного preflight всё ещё может завершиться частично; текущий recovery/lifecycle contract остаётся в силе.

### 8.1 Targets при приобретении точных источников

Во все source-resolution requests передавать вычисленные solver targets узла явно, включая exact registry release и Git source. В текущем `dependencyResolver.resolveRegistry` targets выводятся из root `requirements.Targets` по alias; у транзитивного exact-release узла там записи нет. Простое присваивание `pkg.Targets` после acquisition не исправляет уже выполненную Directory eligibility-проверку.

Минимально расширить существующий resolver request/параметры данными effective targets и использовать тот же Directory/acquisition path. Не добавлять synthetic assignments в root manifest и не искать targets по package-local alias. При позднем расширении union повторно проверить eligibility для нового полного набора через существующий `Eligible`; package facts могут переиспользоваться, но прежний verdict для другого набора targets не становится новым разрешением.

Например, если A требует B только для Codex, а D требует тот же B для Claude, B и его closure должны быть проверены для обоих targets. Если выбранный exact release B не подходит Claude, вернуть конфликт до первого managed Apply, не пропускать B для Claude и не устанавливать только A/D. Существующие отдельные правила для mixed/unsupported components сохраняются; новый target allowlist из TOML пакета не создаётся.

## 9. Lock, replay и целостность

Сохраняются `lock_version = 1` и state schema 5: source keys, typed selectors, exact source coordinates, version, manifest/tree digests и required_by уже представлены существующим lock. Новая persisted dependency source discriminator не нужна для одного поддерживаемого normal source. Не менять version без реального изменения persisted contract.

При replay для каждого pin читать package requirements из тех же проверенных snapshot bytes, что соответствуют lock. Проверка должна обнаруживать отсутствующее, лишнее или изменённое ребро, неверный source key и selector. Она не выводит declarations только из lock: иначе удалённое ребро останется незамеченным.

- `uap.toml` включается в tree digest, store copy, source acquisition и существующие готовые package artifacts.
- Изменение TOML при неизменном `plugin.json` меняет tree digest и отклоняется при replay старого pin.
- Вложенный `uap.lock`, если он присутствует в source tree, может влиять на bytes/digest, но не получает authority над consumer graph.
- Cold restore приобретает каждый exact pin один раз, не перепроверяет moving refs и не выбирает новую release. Не откатить исправление PR #448.
- Root semantic digest включает нормализованные прямые requirements и targets; порядок, комментарии и whitespace не создают новый intent. Изменение package requirements защищает pin tree digest; отдельное top-level TOML hash поле в lock не добавлять без необходимости.
- Для прежних коротких root keys сохранить существующие digest vectors и нормализованное представление intent. Разделение declaration key и node alias не должно само по себе инвалидировать неизменённый manifest/lock или продвигать pins.
- `--locked` при mismatch возвращает ошибку без relock, обновления источников и частичной правки профилей. Совместимый pin без TOML и без старых JSON dependencies остаётся допустимым leaf.
- Сохраняются scanner, revocation, eligibility, host/client support, ownership и recovery guards. Valid lock и store hit не означают разрешённую установку.

### 9.1 Запись файлов, гонки и повтор операции

Сохраняется существующий порядок normal saved mutation: прочитать исходную пару -> построить и проверить candidate graph -> показать effects и получить штатное confirmation -> взять existing environment lock -> повторно проверить входные bytes/state -> опубликовать manifest, затем lock -> свежий Prepare/Apply для каждого binding. `deps lock` изменяет только lock и перед записью также перепроверяет прочитанный manifest. Внешний editor не удерживает process lock, поэтому проверка исходных bytes остаётся обязательной.

| Ситуация | Ожидаемое поведение и следующий шаг |
| --- | --- |
| Ошибка TOML, конфликт selectors, неизвестный target, ошибка получения/проверки source | Не публиковать candidate manifest/lock и не менять profiles/state. Обычный запуск может оставить проверенный immutable cache; это не частичная установка |
| Отмена confirmation обычного запуска | Не публиковать manifest/lock и не менять profiles/ownership/state. Проверенный immutable cache, появившийся при normal preflight, может остаться; не строить общий rollback кеша |
| Dry-run | Не записывать manifest/lock, persistent store/state/profiles/scanner cache. Existing dry-run acquisition направляется в собственный временный root и очищается; его результат не является confirmation для будущего Apply |
| Входной manifest/lock или ownership изменились после preview | Вернуть existing changed-plan/stale-input ошибку до следующего эффекта; не применять старое подтверждение к изменившемуся scope |
| Manifest опубликован, запись lock не удалась | Не запускать Apply. Сохранить честный результат «intent сохранён, lock устарел»; предложить явную проверку/reconciliation через existing `deps lock`/`deps sync`, без автоматического продвижения refs |
| При targeted update manifest не менялся, а новый lock записать не удалось | Старая пара остаётся прежним valid agreement; новый pin не объявляется сохранённым. Не запускать Apply и предложить повторить тот же explicit update: обычный `deps lock` может намеренно сохранить старый compatible pin |
| Сбой после нескольких committed bindings | Сохранить опубликованный desired lock и receipts завершённых шагов, остановить новые эффекты, вывести completed/remaining и existing next actions. Повтор не переустанавливает завершённые bindings |
| Есть pending transaction | Сначала existing typed `recover`, затем отдельный sync. Не создавать второй journal, собственную rollback-транзакцию всего окружения или скрытый recovery |
| Прерван перенос готовой пары файлов | Если transfer workstream включён, сохранить его existing retry contract: повторить исходный import/export из целой пары, а не пересоздавать её через moving-ref relock |

Это не crash-atomic transaction пары файлов или всего набора агентов. Не обещать автоматический общий rollback при native failure и не добавлять новый ledger ради такой иллюзии. Установка и recovery продолжают опираться на текущие installer transactions.

## 10. Прежние JSON declarations и пути установки

Функция ещё не получила первый публичный dependency release; длительный dual-read не нужен. Но отсутствие compatibility burden не разрешает молча отбросить requirements старого source snapshot.

- Manager-level reader обнаруживает сам факт наличия `extensions.io.github.777genius.agentplugins.dependencies` и возвращает typed ошибку `dependency_declaration_moved`, указывающую на перенос в `uap.toml`.
- Отказ одинаков при отсутствии TOML, наличии TOML и даже при совпадающих declarations. Нет precedence между JSON и TOML и нет автоматической конвертации во время install.
- Вне UAP dependency management raw JSON остаётся lossless extension data; standard core schema не расширяется.
- Владелец исходного пакета переносит declaration и создаёт новую source revision/publication. Зафиксированные old commits, store objects, выпуски и подписи не переписываются.
- Lock, указывающий на old JSON-dependency snapshot, получает понятный отказ до Apply. Нельзя советовать слепой relock с потерей exact pins: сначала исправить соответствующий source requirement осознанно, затем явно построить новый lock. Миграционный writer и historical JSON engine не строятся.

Все публичные saved installation paths читают один manager-level набор facts: normal `add`, `update`, `install`, `deps lock/sync`, restore/import installation и repair preflight. Внесение нового TOML в пакет не должно менять выбранный standard/native layout.

Новые правила admission не блокируют cleanup: для удаления узла, который больше не нужен ни одному root, не требуется заново признать поддерживаемым его старый JSON manifest или скачивать его bytes. Удаление выполняется по existing typed ownership/receipts. Retained graph проверяется через facts provider; если старый неподдержанный пакет остаётся retained, operation отказывает до публикации нового intent и Apply, а не угадывает его requirements. `recover` завершает уже записанную transaction по существующей authority и не получает новый metadata gate, мешающий убрать pending state. Это не JSON fallback: старые declarations не используются для формирования нового retained graph.

Маршруты, которые сегодня не поддерживают managed graph (`--no-save`, отдельные local/discovery/manual paths), при наличии обязательных package requirements отказывают с указанием поддержанного managed workflow. Не устанавливать только родительский пакет с молча потерянными dependencies. Для leaf packages существующее поведение сохраняется. Single-package SDK installer остаётся низкоуровневым lifecycle primitive; рекурсивный контракт принадлежит environment coordinator.

## 11. CLI, authoring и документация

- Все root commands используют общий decoder и existing saved-session/manifest editor, а не копируют schema validation.
- Human preview показывает выбранный scope, абсолютный путь root manifest и lock, direct roots, transitively required packages, versions и targets. `deps tree` и JSON сохраняют точные source/consumer facts.
- `status` и `tree` остаются offline/read-only; не выполнять acquisition ради печати root path.
- `author validate`/существующий package validation проверяет optional UAP TOML и retired JSON declaration как отдельный UAP diagnostic после standard manifest validation. Не менять стандартный `plugin.json` schema и не открывать замороженные authoring phases.
- Ручное создание optional package TOML поддерживается сразу. Новые `author deps` команды, обязательный scaffold-файл для каждого leaf и conversion build step не нужны для этого delivery.
- В актуальном `docs/UAP_DEPENDENCIES.md`, public Use/Build примерах и затронутых SDK примерах заменить JSON declaration пример на package TOML. Объяснить root-only targets/lock и auto-install closure.
- Сохранять release labels: source availability не равна установленному CLI/npm release. Не запускать релиз или менять production preview flags ради документации.

## 12. Последовательность реализации

### Шаг 1. Общая грамматика и ключи

Внедрить decoder и контекстные views в SDK, выделить общие правила requirements, поддержать canonical declaration keys и их сопоставление со stable node aliases через source key. Перевести root reader на общий decoder без изменения source selection, target policy и редактора. Согласованно адаптировать lock validation, tree и saved-command selectors; проверить round-trip root manifests/locks, прежние digest vectors и сохранение owner identities при direct/transitive переходах.

Это допустимый подготовительный checkpoint без переключения package graph source: новая публичная package TOML поддержка ещё не объявляется. Нельзя отдельно доставить новый root parser, оставив lock/CLI consumers с предположением declaration key = node alias.

### Шаг 2. Полный пакетный graph cutover

Связанно изменить manager-level package facts, typed graph requirements, candidates, replay и source-selector edge checks. Сделать facts capability обязательной для непустого managed graph, убрать lock-derived fallback и переключить saved remove с `Coordinator{}` на тот же verified facts provider. Перевести остальные managed installation paths и включить retired JSON refusal с сохранением cleanup/recovery boundaries. За один checkpoint не должно оставаться ситуации, когда fresh resolve читает TOML, а replay проверяет JSON, либо один install path теряет closure.

Адаптировать существующие graph/restore fixtures к TOML. Сохранить independent integrity assertions; negative legacy-source fixtures остаются явными проверками отказа. Не удалять полезные tests из-за смены места декларации.

### Шаг 3. Законченный пользовательский сценарий

Обновить author validation через captured `packageview` content, editor/canonical-key coverage, output и текущие docs. Проверить existing copy/archive/transfer paths; если путь упаковки исключает TOML, исправить только текущий supported path, не реализуя отложенный publisher.

В disposable roots пройти создание lock, установку closure, повтор без изменений, targeted update, retained transitive removal, перенос пары manifest/lock и cold locked restore. Если transfer workstream отсутствует в implementation base, перенести проверенную пару как обычные файлы для существующего restore fixture. Один независимый review точного итогового diff и штатный required CI; повторять только затронутые проверки после исправлений.

### Будущий этап. Project scope и dev requirements

Это отдельный implementation scope и не gate для первых трёх шагов. Сохранить следующие границы:

- Явно выбранный project root, собственный manifest/lock и environment identity; не объединять все найденные вложенные TOML и глобальный intent автоматически.
- Общий verified store, но отдельная ownership/removal authority. Реальная namespace/profile isolation и приоритет global/project проверяются по возможностям клиента, а не предполагаются из наличия файлов.
- Current working directory не переключает существующие глобальные mutating commands на новый root незаметно. Пока project scope не реализован, он отклоняется до записей.
- Dev requirements читаются только у выбранного root при явном режиме; normal dependencies выбранного dev plugin рекурсивны, его собственные dev requirements не передаются дальше.
- Перед включением dev-режима определить его intent/lock representation и поведение при последующей обычной синхронизации. Не переключать один lock между разными наборами неявно.
- Совместимость с агентами отделена от назначения targets. Новый compatibility field добавляется только под конкретную поддержанную потребность; автоматическая универсальность всех client capabilities не обещается.

## 13. Проверки и приёмка

Использовать существующие contract/integration suites. Новые проверки нужны там, где прежние JSON/root-only fixtures не ловят конкретную регрессию.

| Проверяемый риск | Наблюдаемое успешное поведение |
| --- | --- |
| Package TOML без targets | A -> B -> C устанавливается для targets корня |
| Canonical quoted key | `"acme/review"` работает в read/edit/lock/update/remove; root key сопоставляется с locked node по source key |
| Short key раскрывается в уже объявленный source | Root с коротким и canonical именем одного distribution отказывает после lookup до публикации candidate файлов; declaration keys не переписываются и targets не объединяются молча |
| Offline lookup зависит от прежнего alias | При отличающемся node alias короткий root key однозначно сопоставляется по locked source facts и проходит полный lock validation; неоднозначный/подменённый lock отклоняется без сетевого обращения |
| Direct/transitive переход меняет owner identity | Продвижение child в direct root и обратное удаление declaration сохраняют compatible pin, node alias, installation/binding IDs и receipts; targets/edges изменяются согласно оставшимся потребителям |
| Новая нормализация меняет старые pins | Неизменённые короткие keys сохраняют прежний manifest digest и no-op install; JSON tree roots ссылаются на существующие package aliases |
| Root и package перепутаны | Root без assignments отказывает; тот же корректный package declaration принимается без targets |
| Отсутствие файла перепутано с ошибкой | Нет TOML и TOML только с `schema_version = 1` дают leaf; zero-byte, неверный тип/версия и blocked/unreadable TOML дают ошибку. Структурно корректные author targets с неизвестными consumer target IDs или несуществующими local declarations не получают authority при package view |
| Вложенный target/lock получает authority | Author targets и чужие nested pins не меняют consumer graph |
| Сканирование всех подпапок | Посторонний nested TOML не читается; учитывается точный package path |
| Разные grammar для root/package | Registry string, exact release и supported Git source принимаются одинаковым decoder |
| Смешанный graph остаётся registry-only | `A(registry) -> B(Git ref/default) -> C(registry exact release)` плюс второй SemVer-потребитель C успешно resolve/install/replay; сохраняются typed selectors, target union и cold restore без moving-ref lookup; подмена Git edge отклоняется до Apply |
| Exact child получен без targets потребителей | Directory/acquisition получает полный effective union для nested exact release; поздний неподдержанный Claude target приводит к отказу до manifest/lock publication и Apply, при поддержанном union граф устанавливается полностью |
| Shared source constraints | Diamond сохраняет все edges/union; conflicting release/Git selectors отказывают до Apply |
| TOML изменён при прежнем plugin.json | Old tree pin отвергается; удаление edge из lock также обнаруживается |
| Authoring перечитывает изменяемый source | TOML берётся из captured lease; change/blocked/unreadable outcome не принимается как leaf, а standard JSON facts не парсятся повторно |
| SDK без facts capability выдаёт неполный graph | Resolver-only вызов с непустым root возвращает `dependency_facts_unavailable`, не lock с ложным leaf; пустой root не требует описывать удалённые packages |
| Remove использует lock как источник declarations | Удаление одного root в mixed graph сохраняет общий Git child, его pin/owner и оставшиеся edges/targets; удаление последнего потребителя убирает child |
| Старый JSON dependency source | Typed отказ при JSON-only и JSON+TOML; нет fallback и silent root-only install |
| Legacy refusal блокирует cleanup | Удалённый из desired graph JSON-only пакет можно убрать по own receipt без acquisition; retained legacy package отказывает до intent/Apply; existing pending recovery сохраняет свой прежний completion contract |
| Неподдержанный imperative path | Package с requirements не устанавливается частично через `--no-save` |
| Cold restore | Exact pins приобретены по одному разу, без moving-ref lookup и выбора новых releases |
| Saved commands и transfer | Existing comments/owners/pins сохранены; strict preflight и confirmation semantics не ослаблены |
| Реальный artifact теряет TOML | Install из результата текущей упаковки воспроизводит тот же граф |
| Ошибка/отмена/dry-run | Прежние manifest/lock/profile/state invariants сохранены; partial не объявляется success |

Для новой границы effective targets расширить существующий mixed-source/lifecycle сценарий, а проверки сбоев записи, отмены, input race и recovery переиспользовать из текущих saved-session/transfer suites. Не дублировать каждый случай во всех слоях и не создавать отдельный orchestration harness.

Для коротких имён достаточно расширить существующие root-reader/lock/status проверки: один post-lookup duplicate и один offline alias-mismatch с отказом на неоднозначность. Проверку captured TOML добавить к существующему authoring capture-тесту; запускать отдельный resolver или acquisition ради author validation не требуется.

Команды для актуальной раскладки модулей; имя нового adapter package согласовать с фактическим размещением:

```sh
# install/integrationctl/agentplugins
go test ./environment ./conformance ./adapters/loader ./adapters/packagedigest ./adapters/environmentstore ./installer
go test ./adapters/dependencymanifest

# cli
go test ./internal/agentpluginscli ./internal/agentpluginscli/manifestedit
go test ./internal/authoring/commands ./internal/authoring/report
```

В существующие CLI E2E suites включить минимум `TestInstallDependencyGraphLifecycle` и `TestDependencyFreshRestoreFromPortableFiles`. Если transfer workstream вошёл в implementation base, также выполнить `TestDependencyTransferGraphRoundTrip`. Положительный mixed-source сценарий выше добавить в существующий lifecycle/restore suite, без нового harness. Реальный сценарий должен работать через публичный CLI/SDK с disposable profiles и fixture sources, не только через mocked solver. Выбор оставшихся обязательных checks определяется текущими repository workflows и изменёнными пакетами.

Готово, когда пакет, публикующий требования только в корневом `uap.toml`, автоматически получает полное проверенное closure во всех поддержанных managed flows; strict replay воспроизводит его из root manifest/lock; nested targets/locks не влияют на пользователя; JSON declarations не являются вторым источником; документы показывают один `[dependencies]` без неподдержанных обещаний project/dev/release.

## 14. Откат и ограничения

Изменения оформлять dependency-safe checkpoint с сохранением существующих ownership/recovery mechanisms. Откат к binary, который не знает TOML package requirements, небезопасен для новых graphs: он может не уметь проверить их declarations. Не обходить refusal и не восстанавливать работу автоматическим relock на moving sources. Сохранить manifest/lock, immutable store, receipts и действующий binary до завершения pending recovery; новый lock или source revision создаётся только явной операцией.

Практические риски и выбранные ограничения:

| Риск | Как ограничиваем его в этом delivery |
| --- | --- |
| Похожий alias у разных источников | Сопоставляем по source key; конфликт node aliases явно отклоняем. Отдельную систему автоматических имён не строим |
| Короткое имя скрывает дубликат или требует сеть для `status` | Повторная уникальность после normal Directory lookup; offline mapping только по однозначным locked facts с полной проверкой lock. При неоднозначности явный отказ, без новой persisted name registry |
| Повторный fetch и network lookup после parser cutover | Одна shared acquisition/description path, existing verified store, reuse exact pins; cold-restore assertions PR #448 сохраняются |
| Собственные targets/lock автора меняют bytes опубликованного пакета | Они не управляют consumer graph, но входят в tree digest как обычные package files. Не вводить второй «семантический package hash», исключение TOML из digest или генератор другого артефакта |
| Декларации понимаются как порядок запуска сервисов | В v1 dependency edge задаёт совместную установку и constraints. Он не является install hook или readiness/startup dependency; существующий детерминированный per-binding lifecycle order сохраняется, новый scheduler или topological runtime не вводится |
| Исполнитель расширяет задачу ради идеальной универсальности | Project scope, dev groups, новые adapters, compatibility schema, publishers и новые recovery engines остаются вне этого delivery |

Самые существенные риски cutover — неполный closure в обходном install path, изменение owner identity при direct/transitive переходе и рассогласование fresh resolve с replay. Поэтому шаги 1 и 2 проверяются как целые инварианты: нельзя объявлять поддержку package TOML только по успешному decoder-тесту. При новой находке расширять проверку конкретного затронутого пути, а не добавлять новый общий слой или повторять весь набор проверок без причины.

При диагностике выводить конкретный package/declaration key, source, selector и этап отказа, используя существующие error/output DTOs. Новые typed ошибки нужны для различимого нового поведения; отдельная taxonomy всех возможных filesystem/network ошибок не требуется.

Plan work не создаёт release, install, commit или merge. При реализации соблюдать обычные независимое code review, required CI и ограничения конкретного delivery scope; не добавлять повторные раунды общего review без новой находки.

## 15. Основания решений

- [Composer Root Package](https://getcomposer.org/doc/04-schema.md#root-package): один формат может описывать библиотеку и корень; часть настроек действует только у корня.
- [Cargo dependency kinds](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html#development-dependencies): инструменты разработки не переходят потребителям как обязательные requirements.
- [Agent Plugins specification](https://agent-plugins.org/specification#8-client-extensions) и [package boundaries](https://agent-plugins.org/plugin-authors/build-an-agent-plugin#package-boundaries): portable core, client extensions и manager behavior не следует смешивать в одном обещании совместимости.

Эти примеры обосновывают семантические границы; они не заменяют проверку UAP graph, locks, native client capabilities и publication bytes.
