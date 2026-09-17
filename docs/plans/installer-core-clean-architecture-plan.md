# План рефакторинга ядра инсталла `agentplugins`: Clean Architecture с адаптером на клиента

| Поле | Значение |
|---|---|
| Репозиторий | `github.com/777genius/universal-agent-plugins` |
| Рабочая ветка | `refactor/installer-core-clean-architecture` (от `origin/main`, HEAD `b443eb733`) |
| Worktree | `/Users/belief/dev/projects/_worktrees/uap-installer-core-refactor` |
| Статус документа | Финальная версия плана после адверсариальной критики (read-only анализ кода; код не менялся) |
| Дата | 2026-09-17 |

Сокращения: `ap/…` = `install/integrationctl/agentplugins/…`; `cli/…` = `cli/plugin-kit-ai/internal/agentpluginscli/…`; `cmd` = `cli/plugin-kit-ai/cmd/agentplugins`. Ядро инсталла = три пакета: `ap/*`, `cli/*`, `cmd`.

Цель: довести ядро инсталла минимум до паритета с конкурентом (аудит: наше ядро 5.8/10, конкурент 7.4/10 по SOLID / DRY / Clean Architecture / Модульность / Качество кода), не хуже по каждому критерию, с контрактом, который позже можно развить до полноценной модульности без разрушительного передела.

Утверждённые пользователем решения (не обсуждаются): единая базовая ветка `refactor/installer-core-clean-architecture`; части стэкаются последовательными PR в неё; автомердж частей в базовую ветку разрешён после ревью/фиксов; **финальный PR в `main` мерджится только по прямой команде пользователя (жёсткое ограничение, не подлежит автономному пересмотру)**; обязательный CI-чек (~10-14 мин) не должен тормозить мелкие PR; заложить контракт/интерфейс для будущей модульности.

---

## 0. Изменения по критике

Черновая версия этого плана прошла адверсариальную проверку с перепроверкой фактов в коде (`wc`/`grep`/`gh`). Вердикт: направление (Strategy/Registry на клиента, DIP для usecase, lint-гейт) здравое, фундаментального изъяна нет; 6 правок обязательны. Все 6 применены ниже, плюс применимые желательные. Числа, помеченные «VERIFIED», перепроверены в коде на HEAD `b443eb733`.

### 0.1. Шесть обязательных правок (применены)

| # | Проблема критики | Что изменено в плане | Почему |
|---|---|---|---|
| **A-1** | `ports.CommandRunner` невозможно перенести в `ap/ports` без нарушения правила «ports → только domain»: VERIFIED `activator.go:18-23` объявляет `CommandRunner` поверх `legacyports.Command`/`CommandResult` из `install/integrationctl/ports`; рантайм-реализация — `processadapter.OS{}`. Part 1 сделал бы Part 0a красным. | **Не дублируем типы в domain.** В depguard-правило `ports-only-domain` явно добавлен allow на `install/integrationctl/ports` с комментарием-обоснованием прямо в `.golangci.yml` (§6.3); исключение описано в `docs/ARCHITECTURE.md` как осознанное (§6.6). | `Command`/`CommandResult` — простые data-типы без поведения и без I/O, а не адаптер. Дублирование в domain создало бы два источника правды и конвертацию на каждом вызове. |
| **F-1 + F-2** | Ratchet-таблица Part 0b («activator 33, planner 31, …») невоспроизводима механически: подсчёт selector-выражений даёт activator 38 / planner 39 / stager 21 / native_identity 16 / detector 22 / usecase 40 / cli 89; подсчёт `case`+`==/!=` даёт третий набор. Числа были посчитаны вручную. Archtest с ними красный в первый же день. Плюс в domain не 4 скрытых ветвления, а 5. | **Числа убраны из плана.** В §7 формально определена метрика archtest (AST selector-выражения `domain.Client<PascalName>` в non-test файлах вне allow-list, §7.2), и зафиксировано: baseline-таблица **генерируется первым прогоном самого инструмента** в Part 0b, а не берётся из ручных оценок. Везде в частях критерий сформулирован как «бюджет пакета X → 0», без абсолютных стартовых значений. `domain/directory_context7_preparation.go:20` добавлен в объём Part 4 (§8.4) и в §4. | Ручная метрика не переживает первый же прогон. Механически определённая метрика + автогенерируемый baseline — единственный воспроизводимый вариант. |
| **O-1** | DoD недостижим: требует «LEGACY SIZE BASELINE пуст, gocyclo≤20 во всём ядре», но Part 0-arch оставляет `adapters/*` без изменений. VERIFIED вне скоупа частей: `transaction/kernel.go` 874, `domain/directory.go` 834, `packageview/source_windows.go` 682, `directoryv1/validate.go` 660 (cyclo 168), `discoveryv1/models.go` 644, `statemigration/migrate.go` 596, `statev2/store.go` (cyclo 98), `conformance/yaml_budget.go` (cyclo 86), CLI `source.go` 1063, `add_multi.go` 937, `lifecycle.go` 883, `read.go` 659, `search.go` 575. Плюс `usecase/remove_group.go RemoveGroup` 240/cyclo 68 — внутри usecase, но не был в Part 9. | **DoD сужен явно до ядра в смысле аудита** (§11): `domain`, `ports`, `usecase`, `planner`, `providers`, `clientdetect`, `clients/*` внутри трёх пакетов `ap`/`agentpluginscli`/`cmd`. Явный список того, что остаётся в baseline как отдельная будущая задача. `RemoveGroup` добавлен в объём Part 9b (§8.9). | DoD, который нельзя выполнить перечисленными частями, — не DoD. Сужение делает его проверяемым; исключения названы поимённо, а не умолчанием. |
| **C-1 + O-3** | Fast gate не компилирует код вне ядра, который от ядра зависит: VERIFIED 16+ non-test файлов (`authoring/{commands,mcpruntime,nativeimport,project,readiness,report,scaffold,skills}`, `terminalprompts/{huh,plain}.go`, `cmd/agentplugins-conformance-adapter`, `cmd/agentplugins-registry-mirror`) + тесты `repotests/*`. Parts 1,2,4,10 меняют именно эти API. Плюс `providers.ManagedMarketplaceName` экспортируема и используется вне providers (`cli/read_reconciliation.go` non-test, `usecase/service_test.go`, `repotests/agentplugins_codex_native_e2e_test.go`). | В `core-fast.yml` добавлен job `vet-all`: `go vet ./...` по **всем 5 модулям** `go.work` (§10). Стоимость 1-2 мин, ловит компиляцию `authoring`/`repotests`/`cmd`. В Part 2 (§8.2) и Part 11 (§8.11) явно подтверждено: `providers.ManagedMarketplaceName` остаётся экспортируемым алиасом до Part 11. | Дешевле заплатить 1-2 мин на PR, чем обнаружить поломку post-merge через 10-27 мин. |
| **C-2** | Долгоживущая ветка против скорости `main`: VERIFIED 412 коммитов в `origin/main` за 30 дней, 97 трогают ядро; за 60 дней `activator.go` менялся 23 раза, `add.go` 26, `service.go` 22, `planner.go` 15, `clients.go` 16, `stager.go` 14, `group.go` 13; 163 remote-ветки. Плана синхронизации нет. Part 5 режет файлы на 4-5 → rename detection не работает → modify/delete-конфликты. | **Trunk-based отвергнут пользователем осознанно** (мердж в `main` — только по прямой команде). Вместо этого в §5 прописана явная стратегия смягчения: (a) `git merge origin/main` в базовую ветку сразу после мерджа каждой части; (b) части, трогающие один горячий файл, выполняются подряд без пауз; (c) конфликты разрешаются вручную с сохранением И апстрим-изменений, И поведение-сохраняющих трансформаций. Зафиксировано как **принятый остаточный риск** (§13, риск №11), не как открытый вопрос. | Ограничение на мердж в `main` — решение пользователя, не техническое. Раз его снять нельзя, риск смягчается процедурой и принимается явно. |
| **F-6** | Взаимоисключающие требования: Part 0-pre §8 «структуры остаются zero-value-friendly, не вводим обязательных конструкторов» vs Part 1 «отсутствие `Paths` → fail-fast». Fail-fast ломает ~55-62 конструкции (VERIFIED `usecase.Service{}` 23, `Planner{}` 32 строки), включая вне ядра (`authoring/commands`, `repotests`) — на рантайме. Сохранить zero-value можно только дефолтом `pathpolicy.Policy{}`, что вернёт импорт `pathpolicy` в usecase и нарушит `usecase-through-ports`. | **Fail-fast побеждает.** Требование zero-value-friendly снято (§2.1, факт 8). ~55-62 места конструирования обновляются через test-helper'ы (механическая правка полей). Объём Part 1 увеличен с ~600 до ~900 строк (§8.1). | `RequireExactPath` делает `Lstat`-проверку symlink-предков — это инвариант безопасности. Тихая подстановка дефолта при nil ослабляет его незаметно. Удобство тестов дешевле. |

### 0.2. Применённые желательные правки

- **A-2/A-3** — формулировка «`clients` — leaf-пакет, готовый к выносу» понижена до честной: «близко к выносимому, известное исключение — `nativeconfig`». VERIFIED: пакет лежит по пути `ap/providers/nativeconfig` (не `adapters/`), и его non-stdlib импорты — ровно два: `github.com/tailscale/hujson` (`kernel.go:11`, `document.go:11`, `project.go:7`) и `install/integrationctl/adapters/atomicfile`. Nil-`Registry` в generic-диспетчерах больше **не** резолвится молча в `all.Default()` — nil → ошибка; реестр инжектируется в composition root и в тестах явно (§3.2, §3.3).
- **A-4** — compile-time assertions `var _ clients.Lifecycle = (*Adapter)(nil)` обязательны в каждом `clients/<id>` с Part 2/3, а не «когда-нибудь»; parity-тест «трейт ⇒ интерфейс реализован» заводится в `contracttest` сразу (§8.2, §8.3).
- **A-5** — зафиксировано: `planner` сохраняет тонкий публичный фасад (`Capabilities`, `Compatibility`, `ClientCompatibility`, `KiroPrepareAction`, `ChatGPTAppBindingAction`, `DetectedPhysicalClient`, `ApplyInstallIntent`) для внешних потребителей навсегда; запрет Part 10 относится к прямой композиции ядра, не к использованию фасада (§3.5, §8.4, §8.10).
- **O-2/O-4** — объём Part 5 поднят с ~5000 до **≈8800 строк** (VERIFIED 4533 prod + 4267 test клиент-специфичных строк в `providers`); разбиение Part 9 на 9a/9b сделано **обязательным**, а не опциональным; ожидаемый рост LOC от резки `applyGroup` — **+15-30 %**, а не «без роста».
- **S-1** — итоговый прогноз понижен с 8.2 до честных **7.0-7.5** в среднем; в §12 явно перечислены ограничения archtest/depguard/contracttest как механизмов обнаружения.
- **F-7** — локальный preflight чинится: в `make test-core` прокидывается `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null`. VERIFIED прецедент в репозитории уже есть: `.github/workflows/authoring-native.yml:82-83` использует ровно эту технику (`GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath`), плюс `authoring-native.yml:46-48` и `agentplugins-native-clients.yml:48-50` для `core.autocrlf`.
- **F-9** — **проверено по исходникам revive, не по памяти.** `rule/file_length_limit.go` сравнивает ключи через `isRuleOption`, который равен `config.NormalizeOption(arg) == config.NormalizeOption(name)`, а `NormalizeOption` = `strings.ToLower(strings.ReplaceAll(name, "-", ""))`. Следствия: (1) принимаемые ключи — **`max`**, `skipComments`, `skipBlankLines`; (2) дефисные написания (`skip-comments`, `skip-blank-lines`) эквивалентны camelCase, дефисы стираются; (3) **`maxLines` / `max-lines` — НЕВЕРНО**: нормализуется в `maxlines` ≠ `max`, ключ молча игнорируется и правило не срабатывает. В §6.3 используется `max`. Underscore, в отличие от дефиса, не стирается — `max_lines` тоже не сработает.
- **R-1** — добавлены негативные contracttest для реализаций `PathPolicy` (symlink-предок → ошибка, path escape → ошибка) и depguard-правило, запрещающее вторую реализацию `ports.PathPolicy` вне `adapters/pathpolicy` (§8.1).
- **R-3** — OCP переформулирован честно: «добавить клиента = 3 точки правки (`domain/clients.go`, `clients/<id>`, `clients/all`) + прохождение `contracttest`», без иллюзии «ничего не трогать» (§3.4, §11).
- **R-4** — перед созданием `docs/adr/0007-*` в Part 11 обязательна проверка `ls docs/adr/` на актуальном `main`: VERIFIED в репозитории уже есть **два** файла с номером 0006 (`0006-authoring-inventory.md`, `0006-standard-first-authoring.md`), то есть гонка номеров реальна (§8.11).

### 0.3. Исправленные фактические ошибки черновика

- **F-3** — `clientdetect.Detector{} ≈ 37` REFUTED: в тестах ровно **1** литерал `Detector{`, остальное — хелпер `testDetector(...)` (25 вызовов) и `NewOS(` (2). Part 3 дешевле, чем закладывалось.
- **F-4** — VERIFIED на HEAD: `usecase.Service{}` — **23** (17 квалифицированных вхождений в 7 файлах + 6 in-package в `usecase`), из них 2 вне ядра (`cli/internal/authoring/commands`); `Planner{}` — **32 строки** (21 in-package в `planner`, 3 non-test в CLI, 1 `cmd/main.go`, 1 `repotests/agentplugins_native_fixture_test.go`, остальное — тесты `usecase`/`providers`). Черновая оценка 25 и 20 занижена; оценка критика 39 и 23 получена другим способом подсчёта. Планируем по верхней границе: **≈55-62 места**, точное число фиксируется прогоном на месте.
- **F-5** — тест-дубли посчитаны неверно: в тестах **4** типа с методом `Detect()` (не 0) и **9** типов с `Activate()` (не 2). На стоимость влияет слабо, но цифры исправлены (§2.1, факт 8).
- **F-8** — CLI-тестов **277** `func Test` в `agentpluginscli` (+29 в `cmd/agentplugins`), не 275.
- **O-5** — archtest пишется на stdlib `go/parser` (режим `ImportsOnly` для границ + `go/ast` для бюджета), а **не** на `golang.org/x/tools/go/packages`: VERIFIED `x/tools` отсутствует во всех `go.mod`, и в репозитории уже есть прецедент на stdlib — `agentplugins/conformance/architecture_test.go:TestConformanceHasNoEffectDependencies`. Новая зависимость не добавляется.
- **O-6** — `repotests/agentplugins_native_fixture_test.go` держит `planner.Planner{}` (root-модуль) — учтено в объёме Part 1 и в `vet-all`.
- **C-3/C-4** — вариант A CI honestly помечен: до post-merge пропускает `generated-check`, conformance-kit adapter, `npm test`, vet ×5 (частично закрыт новым `vet-all`). Финальный PR в `main` получает больше гейтов, чем перечислялось: VERIFIED дополнительно `agentplugins-native-clients.yml` (`pull_request` на `main`, `paths=agentplugins/**`, матрица codex/claude/opencode × macos-15 + windows-2025, 5-13 мин), `dependency-review.yml`, `coverage.yml`, `codeql.yml`. Платформенных build-тегов в ядре 39 файлов (§10).
- **R-2** — внешних потребителей SDK нет (`sdk` и `install/plugininstall` не импортируют `agentplugins`), но внутренних — 16+ файлов вне ядра; совместимость реальна, просто не публичная.
- **1.4** — `go.work` содержит **5** модулей (`.`, `./cli/plugin-kit-ai`, `./install/integrationctl`, `./install/plugininstall`, `./sdk`), не 3. Lint-матрица остаётся из 3 модулей (там весь код ядра), `go vet` — по 5.
- **1.5** — длительность Required по `gh run list` (15 run): 9, 11, 13, 9, 14, 12, 19, 27, 15, 16, 17 мин — хвост до **27** мин на push в `main`, а не «9-15».
- **1.6** — gofmt-дрейф не в 3, а в **7** файлах (§2.1, факт 5) — прогон B и formatters идут по модулю целиком.
- **1.9** — `go test -count=1 ./agentplugins/...` = **62.6 с** wall (usecase 55.2 с, providers 19.3 с); CLI = 56 с.
- **1.10** — `add.go:426-449 normalizeTarget`: `default` возвращает `domain.ClientID(strings.ToLower(strings.TrimSpace(value)))`, то есть неизвестное имя **проходит насквозь**, а не отвергается; есть алиасы `github-copilot`, `vs-code`, `claude-code`, `gemini-cli`, `open-code`, `devin`. `domain.ParseClientID` обязан сохранить и алиасы, и lenient pass-through (§4, §8.10).

---

## 1. Резюме

1. Ядро сегодня: слой `domain` чистый (stdlib-only), но `usecase` зависит от конкретных адаптеров (`pathpolicy`, `planner`) и скрытых optional-контрактов; `providers.Activator` (1247 строк), `planner.Plan`, `providers.Stager`, `providers.NativeIdentityObserver`, `clientdetect.Detector` ветвятся по `ClientID`; шесть текстуально идентичных предикатов и три копии проекций; линтера нет.
2. Целевое состояние: `domain → ports → usecase`, реализации портов — тонкие generic-диспетчеры, клиент-специфика — в пакетах `ap/clients/<id>` через контракт `ap/clients` (segregated capability-интерфейсы + `Registry` + `As[T]` + `Env`), декларативные трейты клиента в `domain.ClientTraits`, границы закреплены `depguard` и arch-тестом, размер/сложность — lint-гейтом с shrink-only baseline.
3. Порядок: Part 0a (lint-гейт) → 0b (guardrails: golden-тесты, arch-тест, `core-fast` CI) → 1 (порты/DIP) → 2 (контракт + shared + дедупликация) → 3 detect → 4 plan → 5 перенос файлов → 6 staging → 7a/7b/7c lifecycle → 8 identity → 9a трейты → 9b резка монолитов usecase → 10 CLI/composition root → 11 финализация + ADR → финальный PR в `main`.
4. CI: `core-fast` (4-6 мин: `lint` + `test-core` + `cross-build` + `vet-all` по 5 модулям `go.work`) как merge-gate для PR в базовую ветку; полный `Required` — на `push` в `refactor/**` после мерджа; локальный `make lint test-core` перед PR.
5. Прогноз: SOLID 7.5-8, DRY 7.5-8, Clean Architecture 7-7.5, Модульность 7-7.5, Качество кода 7 (среднее **≈ 7.0-7.5** против 7.4 у конкурента) — паритет, а не превосходство по каждому критерию. Обоснование понижения — §12.

---

## 2. Part 0-pre. Подтверждённые факты (прочитано в коде worktree)

### 2.1. Таблица фактов разведки

| # | Факт разведки | Статус | Уточнение |
|---|---|---|---|
| 1 | Domain импортирует только stdlib | Подтверждено | Единственное упоминание `pathpolicy` в `ap/domain/identity.go:71` — комментарий. В domain **5** скрытых ветвлений по клиенту: `install_intent.go:21` (Kiro/ChatGPT для `prepare`), `clients.go:92` (SSE unsupported для OpenCode/Codex), `clients.go:96` (App support для ChatGPT), `directory.go:617` (Copilot+VSCode как один backend), `directory_context7_preparation.go:20` (`len(request.Targets) != 1 \|\| request.Targets[0] != ClientChatGPT`). |
| 2 | usecase → pathpolicy | Подтверждено, шире | 5 вызовов, все `pathpolicy.RequireExactPath`: `service.go:1330`, `repair.go:110`, `remove.go:136`, `remove_group.go:121`, `managed_selection.go:51`. Полный набор non-test импортов usecase: `domain`, `transaction`, `pathpolicy`, `ports`, `planner`, **`pathcontract`** (последний должен попасть в целевой allow-list). Usecase держит 5 неявных optional-интерфейсов через type assertion: `dataPathPreflighter` (`component_readiness.go:16`), `managedStdioPreflighter` (`component_readiness.go:20`), `pluginDataAwareStager` (`service.go:68`), `automaticActivationClassifier` (`service.go:1261`), `activationPreflighter` (`service.go:1265`). Порт `CommandRunner` (+ `duplexCommandRunner`, `duplexCapabilityRunner`, `treeCommandRunner`) объявлен в `providers`, а не в `ports`. |
| 3 | `activator.go` — god object | Подтверждено | Реально 1247 строк. Ветвления по `ClientID` в 5 методах (`AutomaticallyActivates`, `PreflightActivation`, `Deactivate`, `Activate`, `activationObservable`) + парсеры вывода трёх CLI (`codexPluginStatus`, `claudePluginStatus`, `copilotPluginStatus`/`copilotLivePluginStatus`) + приватные типы статусов на клиента. |
| 4 | Ветвления по ClientID разбросаны | Подтверждено, **числа не фиксируем вручную** | Затронутые файлы (порядок по убыванию плотности): `providers/activator.go`, `planner/planner.go`, `providers/stager.go`, `providers/native_identity.go`, `adapters/clientdetect/detector.go`, `usecase/{service,repair,intent,group,component_readiness}.go`, `planner/compatibility.go`, `providers/managed_selection.go`, `planner/intent.go`, `domain/{clients,install_intent,directory,directory_context7_preparation}.go`, CLI `add.go` (включая 11-case switch `normalizeTarget` в `add.go:426-449`, дублирующий `domain.ClientDefinitions()`), `prepare.go`, `interactive_targets.go`, `source.go`, `read_reconciliation.go`, `update_multi.go`, `preflight.go`, `read.go`, `add_multi.go`, `lifecycle.go`. **Абсолютные значения бюджета берутся из первого прогона archtest (Part 0b, §7.2), а не из ручного подсчёта:** три разных способа счёта (selector-выражения / `case`-метки / `==`+`!=`) дают три разных набора чисел, и ручные оценки черновика не воспроизводятся ни одним из них. |
| 5 | Нет golangci-lint | Подтверждено | `gofmt -l` показывает дрейф в **7** файлах в трёх модулях: `ap/adapters/packageview/source_unsupported.go`, `ap/adapters/packageview/source_windows_test.go`, `cmd/release_workflow_test.go`, `ap/adapters/evidence/registry.go`, `cli/cmd/plugin-kit-ai/publication_doctor_run.go`, `cli/internal/authoring/scaffold/apply_test.go`, `cli/internal/platformexec/gemini_generate_render_outputs.go`. `go vet ./agentplugins/...` чист. Локально установлен `golangci-lint v1.64.8` (формат конфига v2 несовместим; v1.64.8 не знает правило `file-length-limit` — проверено эмпирически, правило не срабатывает ни в одном написании аргументов). |
| 6 | Go-версии | Подтверждено + уточнение | root `go 1.22`, `install/integrationctl` `go 1.25.0`, `cli/plugin-kit-ai` `go 1.25.8`, toolchain везде `go1.25.13`. `go.work` (`go 1.25.8`) содержит **5** модулей: `.`, `./cli/plugin-kit-ai`, `./install/integrationctl`, `./install/plugininstall`, `./sdk`; root `go.mod` без `require`. `sdk` и `install/plugininstall` не импортируют `agentplugins`. Lint-матрица — 3 модуля (там весь код ядра), `go vet` в `core-fast` — по всем 5. |
| 7 | CI Required 10-14 мин | Подтверждено + следствие | По `gh run list --workflow=ci.yml` (15 run): 9, 11, 13, 9, 14, 12, 19, 27, 15, 16, 17 мин — типично 9-17, **хвост до 27 мин** на push в `main`. `ci.yml` триггерится на `pull_request` только для `branches: [main, master]` → PR в `refactor/**` не запускают ни Required, ни Coverage. Единственный job — `test`. Branch protection на `main` отсутствует (404), rulesets пусты, `allow_auto_merge: false`, squash/merge/rebase разрешены. Локальный `go test -count=1 ./agentplugins/...` = **62.6 с** wall (usecase 55.2 с, providers 19.3 с, остальные < 5 с); CLI-пакет = 56 с — ядро дешёвое; дорого всё остальное в Required (root `./...` с `repotests`, `npm test`, сборка conformance-kit, `generated-check`, vet ×5 модулей). |
| 8 | Тестовое покрытие и стоимость смены сигнатур | Подтверждено, цифры исправлены | providers 8030 prod / 7185 test строк, usecase 4428 / 6736, planner 825 / 967, clientdetect 681 / 761, transaction 874 / 646, domain 2164 / 1080 (ядро ≈ 91 % test/prod). Все тесты внутренние (`package providers` и т.д.), testify не используется. Тест-дубли портов: `ports.DeliveryPlanner` — 1 фейк (`usecase/codex_sse_lifecycle_test.go`); типов с методом `Detect()` — **4**; типов с `Activate()` — **9**; `ports.PackageStager` — 0. Конструкций конкретных структур: `providers.Activator{}` — 105; `clientdetect.Detector{}` — **1 литерал** (+ хелпер `testDetector(...)` 25 вызовов, `NewOS(` 2); `usecase.Service{}` — **23**; `Planner{}` — **32 строки**. **Требование «структуры остаются zero-value-friendly» СНЯТО** (см. F-6 в §0.1): для `Paths` вводится fail-fast, ≈55-62 места обновляются через test-helper'ы. Для остальных полей zero-value остаётся (в частности 105 `Activator{}` не ломаются — `Registry` инжектируется через тот же helper). |

### 2.2. Дополнительные находки

- DRY: пять текстуально идентичных предикатов «среди поддерживаемых компонентов только skills+MCP» — `planner.hasOnlyKiroNativeComponents`, `planner.hasOnlyPortableNativeComponents`, `planner.geminiNativePlanComponents`, `providers.kiroNativeComponents`, `providers.openCodeNativeComponents` (+ `geminiNativeComponents` в `gemini_native.go`). Три копии построителя манифеста через локальный `copyString` (`projectOpenAI`, `projectCursor`, `projectedOpenAIManifest`), три копии цикла проекции MCP (`projectOpenAIMCP`, `projectCursorMCP`, `projectKiroMCP`). В gemini/opencode/cline — лестницы вариантов `applyX / applyXWithKernel / applyXWithRename / applyXWithKernelAndRename / applyXWithKernelRenameAndCapacity` (тест-швы через варианты функций вместо инъекции зависимостей).
- CLI сам конструирует `clientplanner.Planner{ManagedRoot, Detected}` в трёх местах (`interactive_targets.go:284`, `lifecycle.go:611`, `read.go:379`), минуя composition root; `Planner.Detected` — состояние, которое по смыслу параметр запроса. `cli/read_reconciliation.go:238` использует `providers.ManagedMarketplaceName`.
- **Расхождение `PlanRequest.Detected`, которое нужно сохранить явно:** `cmd/main.go:136` создаёт planner для usecase с **пустой** картой `Detected`, а CLI строит свой planner с **реальной** картой. От этого зависит поведение `hasNativeCopilotBackend` через `planner.Detected[ClientCopilot]`. При переходе на `PlanRequest` расхождение фиксируется в тестах, а не «выравнивается» походя (иначе это скрытая смена поведения).
- Composition root один: `cmd/main.go:120-170` (собирает `providers.Stager/Activator/NativeIdentityObserver/PluginDataManager`, `planner.Planner`, `usecase.Service`, `clientdetect.NewOS`, `agentpluginscli.App`). `App.Lifecycle` — конкретная структура `usecase.Service`.
- `providers/nativeconfig` — generic CAS-ядро для нативных конфигов (1507 строк), от `providers` не зависит; переносится свободно. **Его non-stdlib импорты — ровно два:** `github.com/tailscale/hujson` (`kernel.go:11`, `document.go:11`, `project.go:7`) и `install/integrationctl/adapters/atomicfile`. Это и есть причина, по которой контракт `clients` не является полностью «leaf» (см. §3.2).
- ADR-0005 (`docs/adr/0005-standard-first-agent-plugins-installer.md:64`) допускает «agentplugins can later move to its own repository … without forking the engine» — границу модульности проектируем совместимой с этим.
- Локально падают тесты из-за пользовательского git-хука на автора коммита («Commit blocked: author must be iliya»): `ap/adapters/sourceacquisition` (7 тестов), `cli/internal/agentpluginscli` (7 тестов, `source_directory_test.go:1166-1450`), плюс 1 тест `adapters/packagedigest` (case-insensitive APFS hazard). То есть «локальный preflight» на этой машине красный без обхода хука — лечится `GIT_CONFIG_*` в `make test-core` (§6.2).
- `go list ./...` из корня в режиме `go.work` не охватывает пакеты других модулей — линт нужно гонять отдельно по модулям, `go vet` — по каждому модулю `go.work`.
- Экспортируемая поверхность ядра, используемая снаружи пакетов: `providers.Activator` (31 упоминание), `providers.NativeIdentityObserver` (18), `providers.Stager` (9), `providers.PluginDataManager` (5), `providers.ManagedMarketplaceName` (5, в т.ч. non-test `cli/read_reconciliation.go` и root-модуль `repotests/agentplugins_codex_native_e2e_test.go`); `planner.Planner` (9), `planner.Capabilities` (5), `planner.Compatibility`/`ClientCompatibility` (6), `planner.ApplyInstallIntent` (3, в т.ч. 2 в `cli/interactive_targets.go`), `planner.KiroPrepareAction`, `planner.DetectedPhysicalClient`, `planner.ChatGPTAppBindingAction` (по 1); `clientdetect.Detector`/`NewOS`. Часть этого используется и вне ядра: `cli/internal/authoring/{readiness,report}`, `authoring/commands/readiness_test.go`.
- **Потребители ядра вне ядра (важно для CI-гейта, см. C-1):** 16+ non-test файлов — `cli/internal/authoring/{commands/commands.go, commands/dev_session.go, mcpruntime/runtime.go, nativeimport/native.go, project/project.go, project/read_root_windows.go, readiness/doctor.go, readiness/readiness.go, report/public.go, report/report.go, scaffold/plan.go, skills/skills.go}`, `cli/internal/terminalprompts/{huh.go, plain.go}`, `cli/cmd/agentplugins-conformance-adapter/main.go`, `cmd/agentplugins-registry-mirror/main.go` (root-модуль); плюс тесты `repotests/agentplugins_codex_native_e2e_test.go`, `repotests/agentplugins_native_fixture_test.go`, `authoring/readiness/readiness_test.go`, `authoring/commands/readiness_test.go`.

### 2.3. Метрики размера ядра (174 prod-файла, 1370 функций)

| Метрика | p50 | p75 | p90 | p95 | max |
|---|---|---|---|---|---|
| Строк в файле (физ.) | 142 | 305 | 644 | 842 | 1418 (`usecase/service.go`) |
| Строк в функции | 13 | — | 56 | 84 | 620 |
| Statements в функции | 8 | — | 40 | 61 | 451 |
| Цикломатическая сложность | 4 | — | 19 | 28 | 234 |

Файлов > 500 строк: 25. Топ функций по сложности (VERIFIED ±2): `usecase/group.go applyGroup` 620 строк / 451 statements / cyclo 234; `usecase/service.go apply` 371 / cyclo 152; `adapters/directoryv1/validate.go validateSnapshot` 261 / 168; `adapters/statev2/store.go Validate` 189 / 101; `usecase/repair.go Repair` 318 / 93; `conformance/yaml_budget.go preflightYAML` 165 / 87; `planner.Plan` 199 / 84; `gemini_native.go applyGeminiNativeMutationWithKernelRenameAndCapacity` 233 / 81; `activator.go Activate` 279 / 78; `usecase/remove_group.go RemoveGroup` 240 / 68; `activator.go Deactivate` 192 / 58.

Важно (см. O-1): из 25 файлов > 500 строк **в скоуп рефакторинга попадают не все**. Вне скоупа остаются `transaction/kernel.go` 874, `domain/directory.go` 834, `adapters/packageview/source_windows.go` 682, `adapters/directoryv1/validate.go` 660, `adapters/discoveryv1/models.go` 644, `adapters/statemigration/migrate.go` 596, `adapters/statev2/store.go`, `conformance/yaml_budget.go`, CLI `source.go` 1063, `add_multi.go` 937 (`runAddManyLoaded` 186 строк), `lifecycle.go` 883, `read.go` 659, `search.go` 575 — они остаются в LEGACY SIZE BASELINE как отдельная будущая задача (§11).

---

## 3. Целевая архитектура

### 3.1. Слои и направление зависимостей (все стрелки внутрь)

```
cmd/agentplugins (composition root: registry клиентов, runner, store, service, app)
   ▼
cli/agentpluginscli (presentation) — зависит от usecase + ports + domain (+ clients для листинга реестра,
                                     + тонкий публичный фасад planner: Capabilities/Compatibility/константы)
   ▼
usecase (Service: Add/Update/Remove/Repair/Group…) — ТОЛЬКО ports + domain + transaction(kernel API) + pathcontract
   ▼
ports (интерфейсы: DeliveryPlanner(PlanRequest), PackageStager, ClientActivator, ClientDetector,
       PathPolicy*, CommandRunner*, optional-capabilities*)                      * — новые
   ▼
domain (stdlib-only: типы, декларативный реестр ClientDefinition + ClientTraits*, инварианты)

Реализации портов (зависят только от ports/domain и друг от друга строго вниз):
  providers/            тонкие generic-диспетчеры: Activator, Stager (pipeline), NativeIdentityObserver,
                        PluginDataManager. Без `switch ClientID` — только registry lookup.
  planner/              generic-pipeline планирования + тонкий публичный фасад; клиент-специфика
                        через clients.PlanRefiner/TargetLayout.
  adapters/clientdetect generic Detector; поверхности клиентов через clients.HostDetector.
  clients/              КОНТРАКТ адаптера клиента + Registry. Близок к выносимому, но НЕ полностью leaf:
                        импортирует adapters/nativeconfig, который тянет github.com/tailscale/hujson
                        и adapters/atomicfile. Это известное и принятое исключение (§3.2).
  clients/shared        общие helpers адаптеров (предикаты компонентов, marketplace-имена, strict JSON,
                        stdio data contract, проекция MCP/манифестов, generic run-CLI, registry-finding helpers,
                        rename-exclusive).
  clients/<id>/         ОДИН пакет на клиента: claude, codex, chatgpt, copilot (регистрирует copilot и vscode),
                        cursor, kiro, gemini, opencode, cline, windsurf. Реализуют только нужные интерфейсы,
                        с обязательными compile-time assertions `var _ clients.X = (*Adapter)(nil)`.
  clients/all           сборка Registry по умолчанию (единственное место со списком всех клиентов).
                        Используется ТОЛЬКО composition root и тестами — generic-пакеты его не импортируют.
  clients/contracttest  переиспользуемый тест-харнесс соответствия контракту (in-tree и будущие out-of-tree адаптеры).
  adapters/nativeconfig перенос из providers/nativeconfig, без изменений.
  adapters/{statev2, loader, catalog, …} без изменений (вне скоупа DoD, см. §11).
```

### 3.2. Правила импорта (закрепляются `depguard` и arch-тестом)

| Пакет | Разрешено | Запрещено |
|---|---|---|
| `domain` | stdlib | всё остальное |
| `ports` | `domain`, **`install/integrationctl/ports`** (legacy `Command`/`CommandResult`) | всё остальное |
| `usecase` | `domain`, `ports`, `transaction`, `pathcontract` | `providers`, `planner`, `ap/adapters/*`, `install/integrationctl/adapters/pathpolicy`, `clients/*` |
| `clients` (контракт) | `domain`, `ports`, `adapters/nativeconfig` | `providers`, `planner`, `usecase`, `clientdetect`, `clients/<id>`, `clients/all` |
| `clients/shared` | как `clients` + `pathpolicy`, `pathcontract`, `managedstdio`, `atomicfile`, `filetree` | `clients/<id>`, `clients/all`, `providers`, `planner`, `usecase` |
| `clients/<id>` | `clients`, `clients/shared`, `domain`, `ports`, `adapters/nativeconfig`, `pathpolicy`, `managedstdio`, `pathcontract` | `providers`, `planner`, `usecase`, `clients/<другой id>`, `clients/all` |
| `providers`, `planner`, `clientdetect` | `clients`, `clients/shared`, `domain`, `ports` | `clients/<id>` напрямую, **`clients/all`** |
| `cli/agentpluginscli` | `usecase`, `ports`, `domain`, `clients`, тонкий фасад `planner` (только `Capabilities`/`Compatibility`/`ClientCompatibility`/константы) | `providers`, `pathpolicy`, конструирование `planner.Planner{}` |
| `cmd/agentplugins` | всё (composition root) | — |

**Два осознанных исключения, названных явно (а не умолчанием):**

1. **`ports` → `install/integrationctl/ports`** (правка A-1). `Command`/`CommandResult` — простые data-структуры без поведения и I/O; их реализация (`processadapter.OS{}`) остаётся адаптером и в `ports` не попадает. Альтернатива — продублировать типы в `domain` и конвертировать на каждом вызове — создаёт два источника правды ради формальной чистоты. Исключение фиксируется комментарием в `.golangci.yml` и абзацем в `docs/ARCHITECTURE.md`.
2. **`clients` → `adapters/nativeconfig` → `hujson` + `atomicfile`** (правка A-2). Контракт клиента не является полностью leaf-пакетом: `nativeconfig.Kernel` фигурирует в `clients.Env`, а сам `nativeconfig` тянет внешнюю зависимость `github.com/tailscale/hujson`. Формулировка «leaf-пакет, готовый к выносу» из черновика **заменена на честную**: «близко к выносимому; известное исключение — `nativeconfig`». Если вынос `clients`+`domain`+`ports` в отдельный модуль когда-то станет задачей, `nativeconfig` придётся либо унести вместе с ними, либо спрятать за узкий порт `ports.NativeConfigKernel`. Вводить такой порт **сейчас** мы не будем (лишняя абстракция без текущего потребителя), но ограничение записано, чтобы не выяснять его в момент выноса.

**Nil-Registry больше не резолвится молча** (правка A-3). В черновике `Registry == nil → clients/all.Default()` подавалось как идиома `http.DefaultServeMux`. На деле это скрытый service locator: любой импорт `providers` компилировал бы **все** адаптеры всех клиентов, что обнуляет тезис «сборка с подмножеством клиентов возможна» — то есть ровно тот антипаттерн, который план сам критикует в разделе DIP. Правило: `Registry` инжектируется явно в composition root и в тестовых хелперах; `nil` → ошибка конструирования/вызова, не тихий дефолт. `clients/all` импортируют только `cmd` и тесты; для generic-пакетов он в deny-списке depguard.

Следствие для Part 2 (VERIFIED): 11 unexported-хелперов из generic-файлов `providers` используются клиент-специфичными файлами и **должны** уехать в `clients/shared` именно в Part 2 — иначе в Part 5 возникнет цикл `providers ↔ clients/<id>`.

### 3.3. Контракт адаптера клиента (пакет `ap/clients`, иллюстративные сигнатуры)

```go
// Базовая идентичность — обязательна. Definition/Traits берутся из domain.ClientDefinitionFor(ID()):
// один источник правды остаётся в domain (нужен Directory-валидации и CLI без registry).
type Adapter interface{ ID() domain.ClientID }

// Detection — использует adapters/clientdetect.Detector.
type HostDetector interface{ DetectSurfaces(Host) Detection }
//   Host: HomeDir, GOOS, Env, SystemApplicationsDir, WindowsProgramFiles, LinuxApplicationDirs,
//         LookPath/Lstat/ReadDir + методы BinarySurface/DirectorySurface/AppSurface/WindowsAppSurface/
//         LinuxDesktopSurface/ExtensionSurface/XDGConfigRoot/EditorChannelConfigRoot/VSCodeConfigRoot.
//   Detection: ConfigRoot, ExecutablePath string; Surfaces []domain.ClientSurface; SelectionSurfaceIDs []string.

// Planning — использует planner.Planner.
type TargetLayout interface {
    TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (anchor, root string, err error)
}
type PlanRefiner interface{ RefinePlan(ctx context.Context, in PlanInput, plan *domain.DeliveryPlan) error }
//   PlanInput: Envelope, Client, Detected map[domain.ClientID]domain.DetectedClient, Intent domain.InstallIntent;
//   метод BackendSibling() (domain.DetectedClient, bool) — по BackendFamily.
type CompatibilityLimiter interface {
    ClientLimitations(envelope domain.PackageEnvelope) []string
    ComponentLimitations(envelope domain.PackageEnvelope, item domain.ComponentDecision) (reject []string, note []string)
}

// Staging — использует providers.Stager.
type StagingLayout interface {
    StagingBase(plan domain.DeliveryPlan) string
    ValidateTargetLayout(plan domain.DeliveryPlan) error
}
type Projector interface{ Project(ctx context.Context, in ProjectionInput) ([]domain.NativeObjectOwnership, error) }
//   ProjectionInput: StagingPath, Envelope, Plan, Hints, PluginDataPath, Launcher StdioLauncherDeliverer.

// Lifecycle — использует providers.Activator.
type Lifecycle interface {
    Activate(ctx context.Context, env Env, req domain.ActivationRequest) (domain.ActivationOutcome, error)
    Deactivate(ctx context.Context, env Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error)
}
type ActivationPreflighter interface{ PreflightActivation(env Env, req domain.ActivationRequest) error }
type AutomaticActivator    interface{ AutomaticallyActivates(env Env, req domain.ActivationRequest) bool }
type ReadOnlyVerifier      interface{ VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool }

// Identity — используют providers.NativeIdentityObserver и Stager.ManagedMCPNames.
type RegistryInspector interface {
    InspectNativeRegistry(ctx context.Context, env Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (RegistryFinding, error)
    UsesNativeRegistryExecutable() bool
}
type PreparedRegistryInspector interface{ InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (RegistryFinding, error) }
type SelectionReader           interface{ ManagedMCPSelection() SelectionLayout } // {File ".mcp.json"|"mcp.json"; Nested bool}

// Инфраструктура передаётся адаптеру (DIP), а не принадлежит ему.
type Env struct {
    Runner       ports.CommandRunner
    NativeConfig nativeconfig.Kernel
    Paths        ports.PathPolicy
    Launcher     StdioLauncherDeliverer
    Now          func() time.Time
}

// Registry (Strategy + Registry).
func NewRegistry(adapters ...Adapter) (*Registry, error)   // дубликат ID или ID вне domain — ошибка
func (r *Registry) Lookup(domain.ClientID) (Adapter, bool)
func (r *Registry) All() []Adapter                          // порядок = domain.ClientDefinitions()
func As[T any](r *Registry, id domain.ClientID) (T, bool)   // capability lookup: адаптер, реализующий T

type RegistryFinding uint8 // Clear, Expected, Collision, Indeterminate (перенос providers.registryFinding)
```

**Компенсация слабости `As[T]` (правка A-4).** `As[T]` институционализирует «скрытый optional-интерфейс через type assertion» — опечатка в имени метода приведёт к тому, что адаптер молча «не реализует» capability, и ошибка всплывёт в рантайме вместо компиляции. Поэтому обязательны с самого начала (Part 2/3, а не «когда-нибудь»):

1. В каждом `clients/<id>` — блок compile-time assertions на **все** заявленные capability: `var _ clients.Lifecycle = (*Adapter)(nil)` и т.д.
2. В `contracttest` — parity-тест «трейт ⇒ интерфейс»: например `Traits.LifecycleKind == LifecycleNativeConfig` ⇒ адаптер реализует `Lifecycle`; `Traits.InstallIntents` содержит `prepare` ⇒ реализует `ActivationPreflighter`. Тест бежит по `clients/all` и падает при рассинхроне.

Пример диспетчера после рефакторинга (`providers.Activator.Activate`): generic-инварианты остаются в диспетчере (совпадение `ClientID` плана/доставки, `Paths.RequireContainedChild(OwnedBase, ActivePath)`, реальная директория без symlink), затем `lifecycle, ok := clients.As[clients.Lifecycle](a.Registry, req.Client.ClientID)`; при `!ok` — та же ошибка `unsupported activation client %q`, что сегодня в `default:`. Ноль `switch` по клиенту. `Activator{Runner, NativeConfig, Registry}`: `Registry == nil` → ошибка `activator registry is required` (не тихий дефолт); 105 конструкций `Activator{}` в тестах получают реестр через общий test-helper.

### 3.4. Как это закрывает нарушения

- **SRP**: `Activator`/`Planner`/`Stager`/`NativeIdentityObserver`/`Detector` — только generic-инварианты и диспетчеризация; `clients/<id>` — только свой клиент; парсеры вывода CLI живут рядом со своим клиентом.
- **OCP (честная формулировка, правка R-3)**: добавить клиента = **3 точки правки + прохождение contracttest**: (1) строка таблицы `domain.ClientDefinitions` (+ трейты), (2) пакет `clients/<id>`, (3) строка в `clients/all`. Ни один generic-файл не меняется — это верно буквально и измеримо (archtest «бюджет ветвлений по ClientID в generic-пакетах = 0» + тестовый `exampleclient`), но иллюзии «ничего не трогать» не создаём: источник правды о клиенте после рефакторинга живёт в трёх местах, и это цена того, что `domain` остаётся stdlib-only и не знает о registry.
- **LSP**: `contracttest` гарантирует одинаковое поведение любого адаптера на инвариантах (`VerifyOnly` не мутирует; mismatched ID отклоняются; детерминированный порядок surfaces; `Runner == nil` → Indeterminate, не паника).
- **ISP**: адаптер реализует только нужные интерфейсы (Cursor — без мутирующего lifecycle и registry-инспекции; ChatGPT — без executable); диспетчер спрашивает `As[T]`, а не требует «толстый» интерфейс; compile-time assertions закрывают риск опечатки.
- **DIP**: usecase зависит от `ports.PathPolicy`, `ports.DeliveryPlanner(PlanRequest)`, явных optional-портов; адаптеры получают `Env` снаружи; composition root — единственное место, знающее конкретику; `Registry` инжектируется, а не резолвится глобально.
- **DRY**: `shared.OnlyNativeComponents`, `shared.HasSupportedMCP`, `shared.ProjectMCPServers(dialect)`, `shared.ManifestFromEnvelope`, `shared.RunClientCommand`, `shared.DecodeStrictJSON*`; лестницы `*WithKernel/WithRename/WithOps` заменяются одной функцией с `Env`/`Ops`.
- **Clean Architecture**: направление зависимостей формально закреплено `depguard` (full-file) + arch-тестом; domain stdlib-only; порты полные и явные; composition root один; CLI не конструирует ядро. Два названных исключения (§3.2) — известная остаточная нечистота, а не незамеченная.
- **Модульность**: `clients` + `domain` + `ports` — стабильная поверхность; адаптер зависит от узкого набора пакетов; registry инжектируется (сборка с подмножеством клиентов действительно возможна, т.к. `clients/all` не импортируется generic-пакетами); `contracttest` — проверка внешних адаптеров. Следующий шаг вне плана: вынести `clients`+`domain`+`ports` в отдельный Go-модуль с semver (совместимо с ADR-0005), предварительно решив вопрос `nativeconfig`→`hujson`.

### 3.5. Инварианты совместимости на весь рефакторинг

- JSON `DeliveryPlan`, `ActivationOutcome`, `DeactivationOutcome`, `ClientCapabilities`, `ClientCompatibility`, `DetectedClient` неизменен (golden).
- Тексты `UserActions`/`Warnings`/`LocalActions`/`Diagnostics` и их порядок неизменны (golden).
- Формат `state-v2.json`, `NativeObjectOwnership.Kind/ObjectID` (например `package:<client>:<id>`, `kiroSkillObjectKind`) неизменны.
- Поведение CLI (`--format json` и текст) неизменно (golden).
- Новые флаги трейтов клиента — в отдельной несериализуемой `domain.ClientTraits` на `ClientDefinition`, не в `ClientCapabilities` (та эмитится в публичный JSON).
- **`planner` сохраняет тонкий публичный API-фасад навсегда** (правка A-5): `Capabilities`, `Compatibility`, `ClientCompatibility`, `KiroPrepareAction`, `ChatGPTAppBindingAction`, `DetectedPhysicalClient`, `ApplyInstallIntent`. Эти имена используются non-test кодом CLI (`interactive_targets.go` — 2 вызова `ApplyInstallIntent`) и **вне ядра** (`cli/internal/authoring/{readiness,report}`, `authoring/commands/readiness_test.go`). Запрет Part 10 «CLI не импортирует planner» относится к **прямой композиции** (`clientplanner.Planner{}`), а не к использованию стабильного фасада: после Part 10 внутренности фасада делегируют в registry, сигнатуры не меняются.
- Экспортируемые типы/функции `providers.*`, `planner.*`, `clientdetect.*`, поля `usecase.Service` сохраняются (добавляются новые поля) до Part 11; переходные алиасы удаляются только в Part 11, когда все in-repo вызовы переведены. `providers.ManagedMarketplaceName` — экспортируемый алиас до Part 11 включительно (правка O-3).
- `go build`, `go vet`, `go test` зелёные на каждом мердже в базовую ветку; `go vet ./...` — по всем 5 модулям `go.work`.

---

## 4. Стратегия избавления от ClientID-ветвлений

Паттерн: Strategy + Registry (`clients.Registry` → `clients/<id>`) для операционной логики; декларативные трейты (`domain.ClientTraits`) для решений в usecase/domain/CLI, где нельзя тянуть registry. Правило: usecase/domain/CLI ветвятся по capability/trait, никогда по ID.

| Источник (сейчас) | Куда переезжает |
|---|---|
| `clientdetect/detector.go:118-144` switch → `detectCodex…detectWindsurf` | `clients/<id>/detect.go` реализует `HostDetector`; `Detector.detect` итерирует `registry.All()`. Generic surface-хелперы становятся методами `clients.Host`. Выбор канала Windsurf — внутри `clients/windsurf`. |
| `planner.targetRoot` (Claude → `<configRoot>/skills`, Cursor → `<configRoot>/plugins/local`, иначе `<managed>/clients/<id>`) | `TargetLayout` у claude/cursor; default `shared.ManagedTargetRoot`. |
| `planner.Plan`: `trusted_claude_cli_required`; ChatGPT mapping/app-binding/`LocalPreparationAuthorized`; промоушены Ready для Copilot/VSCode/Kiro/OpenCode/Gemini/Windsurf/Cline; switch user actions; `windsurf_skills_prepared_only`; `setNativeRegistry` (VSCode → Copilot); `hasNativeCopilotBackend` | `PlanRefiner` каждого адаптера; общий «Ready, если есть configRoot и only-native компоненты» — `shared.PromoteNativeReady`; сосед по backend — `PlanInput.BackendSibling()`. |
| `planner.componentDecisions` (ChatGPT app-mapping; OpenCode/Codex SSE reason) | ChatGPT-mapping → пост-корректировка в `PlanRefiner` chatgpt; SSE reason → generic: reason ставится любому клиенту с `MCPTransports["sse"]==unsupported` при объявленном `sse` (для остальных клиентов поведение не меняется). |
| `planner/compatibility.go` (ChatGPT/Windsurf limitations) | `CompatibilityLimiter` у chatgpt/windsurf; `Compatibility()` спрашивает registry (сигнатура фасада не меняется). |
| `planner/intent.go ApplyInstallIntent` (ChatGPT/Kiro prepare) | ветка `prepare` в `PlanRefiner` chatgpt/kiro; допустимость intent — `Traits.InstallIntents`; экспортируемая `ApplyInstallIntent` остаётся фасадом. |
| `planner.DetectedPhysicalClient` (Copilot↔VSCode) | generic `domain.BackendSiblings(id)`; имя функции сохраняется как фасад. |
| `stager.stage`: проекции по клиенту, `buildXNativeObjects`, `deliverManagedStdio` (Claude/Windsurf), `projectCodexMarketplace` (Codex/ChatGPT), `projectCopilotMarketplace` (Copilot/VSCode) | `Projector.Project` каждого адаптера (возвращает native objects); launcher — через `ProjectionInput.Launcher`. |
| `stager.Discard`/`stage` staging base для Claude; `validatePlanPaths` Claude `skills` | `StagingLayout` у claude; default `shared.DefaultStagingLayout`. |
| `stager.writeSanitizedApp` (ChatGPT сохраняет `.app.json`) | generic sanitize удаляет `.app.json`; `Projector` chatgpt пишет его сам из `envelope.App.Raw` (режим 0644/0600 сохраняется). |
| `activator.Activate/Deactivate/PreflightActivation/AutomaticallyActivates/activationObservable` + `activateCopilot/verifyCopilot/activateCodex/verifyCodex/verifyClaude/verifyKiroMCP/deactivateCopilot/removeCodexMarketplace/removeCodexPlugin`; `codexStatus/claudeStatus/copilotStatus` | `Lifecycle`/`ActivationPreflighter`/`AutomaticActivator` адаптеров; парсеры → `clients/codex/list.go`, `clients/claude/list.go`, `clients/copilot/list.go`. Общие `failedActivation`, `requireExternalUninstall`, `attestedUnknownVerification`, `runClientResult`, `commandOutputContains`, `errRecognizedNegativeEvidence` → `clients/shared`. |
| `native_identity.inspectNativeRegistry` switch; `nativeAttempted` (Codex/Claude/Copilot/VSCode); `inspectClaudeSkillsRegistry` | `RegistryInspector` у всех; `nativeAttempted` = адаптер реализует `RegistryInspector` и `UsesNativeRegistryExecutable()`; Claude — `PreparedRegistryInspector`; default prepared — `shared.InspectUnqualifiedPluginRoot`. |
| `providers/managed_selection.go` (`.mcp.json` для Claude/Codex/ChatGPT; плоский документ Claude) | `SelectionReader` у claude/codex/chatgpt; default `{mcp.json, nested}`. |
| `usecase.clientVerifierAvailable` | `providers.Activator` реализует optional-порт `ports.ActivationVerifierClassifier`, диспетчеризуя в `ReadOnlyVerifier` адаптера; usecase спрашивает порт (как уже для `AutomaticallyActivates`). |
| `usecase.nativeLifecycleClient` (Gemini/OpenCode/Cline/Windsurf) | `Traits.LifecycleKind == LifecycleNativeConfig`. |
| `usecase.openAIOAuthApplies` (Codex) | `Traits.HonorsOpenAIMCPAuthHints`. |
| `usecase/component_readiness.go` (Windsurf/Claude managed stdio) | `Traits.UsesManagedStdioLauncher`. |
| `usecase/group.go` (Codex recovery eligibility) | `Traits.SupportsPreparedRecovery`. |
| `usecase/intent.go` правила ChatGPT personal-mapping | `Traits.RequiresPersonalMappingForPrepare` + generic-проверки. |
| `usecase.sameNativeBackend(x, ClientCopilot)`, `group.go`, `domain/directory.go:617` | `domain.SharesBackend(id)` / `domain.BackendSiblings(id)` (по `BackendFamily`). |
| `domain.InstallIntent.Validate` (`install_intent.go:21`), `clients.go:92/96` | декларативные поля таблицы `ClientDefinitions` (`Traits.InstallIntents`, явные аргументы SSE/App), без `if id ==`. |
| **`domain/directory_context7_preparation.go:20`** — `len(request.Targets) != 1 \|\| request.Targets[0] != ClientChatGPT` (пропущено в черновике, правка F-2) | Трейт `Traits.RequiresSingleTargetContext7Preparation` (или обобщение через `Traits.RequiresPersonalMappingForPrepare`, если семантика совпадает — решается в Part 4 при чтении вызывающего кода). Объём работ Part 4. |
| CLI `add.go:426-449 normalizeTarget` name → ID | `domain.ParseClientID(string)` по реестру. **Обязательно сохранить обе особенности** (VERIFIED): алиасы `github-copilot`, `vs-code`, `claude-code`, `gemini-cli`, `open-code`, `devin`; и **lenient pass-through** — неизвестное имя возвращается как `domain.ClientID(strings.ToLower(strings.TrimSpace(value)))`, а не отвергается ошибкой. Закрепляется табличным тестом до рефакторинга. |
| CLI Copilot/VSCode (`add.go:457-473`, `read_reconciliation.go:98-124`, `update_multi.go:285-304`) | `domain.BackendSiblings`. |
| CLI Kiro/ChatGPT prepare (`prepare.go`, `interactive_targets.go`, `preflight.go`, `source.go`) | `Traits.InstallIntents` / `Traits.RequiresPersonalMappingForPrepare`. |

Где ID клиентов разрешены после завершения: строки таблицы `domain/clients.go`, пакет `clients/<id>` (только свой ID), список в `clients/all`, тесты.

---

## 5. Порядок частей, зависимости и синхронизация с `main`

```
0a lint-gate ──┐
               ├──► 1 ports/DIP ──► 2 contract+shared ──► 3 detect ──► 4 plan ──► 5 move files ──► 6 staging
0b guardrails ─┘                                                                                      │
                                                                                                      ▼
   11 finalize ◄── 10 CLI/root ◄── 9b split ◄── 9a traits ◄── 8 identity ◄── 7c native ◄── 7b kiro ◄── 7a cli-registry
```

0a и 0b можно вести параллельно (0b зависит от 0a только через переиспользуемый `lint.yml`). Все остальные — строго последовательно. Каждая часть — один PR в базовую ветку.

### 5.1. Общие правила для всех частей

- (a) `go build`, `go vet`, `go test` зелёные на каждом мердже; `go vet ./...` — по всем 5 модулям `go.work`.
- (b) публичные JSON/тексты/state не меняются — golden-тесты Part 0b.
- (c) перемещаемые тесты переезжают дословно, новых `t.Skip` нет, счётчик `func Test*` в ядре до/после равен (или растёт).
- (d) PR-описание содержит таблицы «бюджет ветвлений до/после» (числа из archtest, не из головы) и «LEGACY SIZE BASELINE до/после».
- (e) файл, которого часть касается, но не режет, остаётся в baseline с пометкой «режется в Part N».
- (f) мердж — `gh pr merge --squash` после зелёного `core-fast` и ревью.
- (g) PR явно заявляет: legacy `install/integrationctl/{adapters,usecase,domain}` вне `agentplugins` не затронуты (требование AGENTS.md репозитория о сохранении legacy `plugin.yaml` возможностей).

### 5.2. Синхронизация с `origin/main` (правка C-2 — принятый остаточный риск)

Контекст (VERIFIED): `origin/main` движется быстро — 412 коммитов за 30 дней, 97 из них трогают ядро; за 60 дней `activator.go` менялся 23 раза, `add.go` 26, `service.go` 22, `planner.go` 15, `clients.go` 16, `stager.go` 14, `group.go` 13; в репозитории 163 remote-ветки. Part 5 режет большие файлы на 4-5 частей, и git rename detection на split **не работает** — апстрим-правки в разрезанный файл превращаются в `modify/delete`-конфликты. Это самый большой практический риск плана.

Trunk-based (мердж каждой части сразу в `main`) устранил бы риск, но **отвергнут пользователем осознанно**: мердж в `main` разрешён только по прямой команде. Это ограничение не пересматривается автономно. Поэтому риск смягчается процедурой и принимается:

1. **Сразу после мерджа каждой части** в базовую ветку — `git fetch origin && git merge origin/main` (или rebase, если история части ещё не публична) в базовую ветку. Дивергенция никогда не превышает длительность одной части.
2. **Части, трогающие один горячий файл, выполняются подряд без пауз.** Горячие файлы: `activator.go` (Parts 5, 7a-7c), `add.go` (Part 10), `service.go` (Parts 1, 9a, 9b), `planner.go` (Parts 1, 4), `clients.go` (Part 9a), `stager.go` (Parts 2, 6), `group.go` (Parts 9a, 9b). Между Part 5 и Part 7c пауз не делаем — это окно максимальной уязвимости к конфликтам.
3. **Конфликты слияния разрешаются вручную**, с сохранением И апстрим-изменений, И поведение-сохраняющих трансформаций. Чужая работа никогда не отбрасывается «потому что мешает мерджу»; при сомнении — спросить пользователя. После каждого нетривиального merge — прогон golden-тестов Part 0b: они и есть детектор того, что при разрешении конфликта потерялось поведение.
4. Перед стартом Part 5 — снимок `git log --oneline origin/main -- <список файлов к переносу>`, чтобы при конфликте было видно, какие апстрим-коммиты нужно перенести в новые файлы вручную.

Остаточный риск после смягчения: **средний**. Если в апстрим за время Part 5-7 придёт крупная правка в `activator.go`/`stager.go`, ручной перенос в 4-5 новых файлов займёт часы и может внести регрессию, которую поймают только golden-тесты. Это принято сознательно как цена ограничения на мердж в `main`.

---

## 6. Part 0a — Lint gate: golangci-lint с лимитами размера и depguard-границами

Цель: механический контроль качества и размера для нового/изменяемого кода без обязанности чинить легаси одним PR; архитектурные границы закреплены линтером, а не договорённостью.

### 6.1. Обоснование лимитов

| Метрика | Текущее распределение | Лимит | Почему |
|---|---|---|---|
| Строк в файле | p50=142, p90=644, p95=842, max=1418 | 500 строк кода (revive `file-length-limit` с `skipComments`/`skipBlankLines`; ≈ 560-600 физических) | 25 core-файлов выше — god-файлы; целевые `clients/<id>/{detect,plan,project,lifecycle,identity}.go` ≤ 300 каждый. Тест-файлы из лимита исключены (табличные тесты; `cli_test.go` 3287 строк — отдельная тема вне рефакторинга). |
| Длина функции | p50=13/8, p90=56/40, p95=84/61 (строк/statements) | 60 строк / 40 statements (`ignore-comments`) — дефолты golangci | Дефолт лежит на p90 текущего идиоматичного кода: «новый код не хуже 90 % существующего». 123 функции выше — все в baseline. |
| gocyclo | p90=19, p95=28 | 20 | Выше 20 — 123 функции (9 %), все известные монолиты. |
| gocognit | — | 25 | Между рекомендацией golangci (10-20) и дефолтом 30; ловит вложенность, невидимую gocyclo. |
| dupl | — | 120 токенов | Дефолт 150 не поймает 15-строчные дубли предикатов; проверить первым прогоном, что 6 дублей `*NativeComponents` флагуются, иначе 100. |

**Аргументы `file-length-limit` — проверено по исходникам revive, не по памяти** (правка F-9). `rule/file_length_limit.go` сравнивает ключи через `isRuleOption`, реализованный как `config.NormalizeOption(arg) == config.NormalizeOption(name)`, где `NormalizeOption` = `strings.ToLower(strings.ReplaceAll(name, "-", ""))`. Отсюда:

- принимаемые ключи: **`max`**, `skipComments`, `skipBlankLines`;
- дефисные написания эквивалентны camelCase (`skip-comments` == `skipComments`), потому что дефисы стираются;
- **`maxLines` и `max-lines` — неверно**: нормализуются в `maxlines`, не совпадают с `max`, ключ молча игнорируется и лимит не применяется. Underscore не стирается, поэтому `max_lines` тоже не работает.

Проверить эмпирически на первом прогоне всё равно нужно (негативный тест в §6.7, п.3): локальный `golangci-lint v1.64.8` встроенного revive нужной версии не содержит — правило не срабатывает ни в одном написании, то есть проверять придётся уже на v2.13.2.

### 6.2. Файлы

- `.golangci.yml` в корне (один конфиг на 3 модуля с кодом ядра; различий между модулями нет).
- `Makefile`:
  - `lint` — прогон B (full-file) + прогон A (`--new-from-merge-base=$(BASE)`, `BASE ?= origin/main`);
  - `lint-fix` — formatters;
  - `test-core` — `go test -count=1` по ядру. **Обязательно с обходом пользовательского git-хука** (правка F-7): `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null`, иначе на машине разработчика падают 15 тестов (`sourceacquisition` 7, `agentpluginscli/source_directory_test.go` 7, `packagedigest` 1) по причине, не связанной с кодом. Прецедент в репозитории есть: `.github/workflows/authoring-native.yml:82-83` использует ровно эту технику (плюс `GIT_CONFIG_NOSYSTEM=1`), `authoring-native.yml:46-48` и `agentplugins-native-clients.yml:48-50` — для `core.autocrlf`.
- `.github/workflows/lint.yml` (`workflow_call`; вызывается из `ci.yml` и `core-fast.yml`).
- `.github/workflows/ci.yml`: job `lint` параллельно `test` (через `lint.yml`).
- `scripts/check-lint-baseline.sh` — shrink-only проверка baseline.
- `CONTRIBUTING.md` — секция «Lint gate and size limits».
- `docs/ARCHITECTURE.md` — секция «Agent Plugins core: layering, import rules, size limits», включая явное описание двух принятых исключений из §3.2.
- `.github/pull_request_template.md` — чекбокс `make lint`.
- Фикс gofmt-дрейфа в **7** файлах; дешёвые автофиксы первого прогона (goimports, misspell, unconvert, ineffassign, copyloopvar, intrange).

### 6.3. Конфиг (v2; ключи сверены с `.golangci.reference.yml` тега v2.13.2)

```yaml
version: "2"

run:
  timeout: 6m
  tests: true
  relative-path-mode: gitroot   # пути в exclusions/depguard — от git-корня; линт гоняется из 3 модулей отдельно

linters:
  default: none
  enable:
    # корректность
    - errcheck
    - govet
    - staticcheck
    - unused
    - ineffassign
    - errorlint
    - nilerr
    - bodyclose
    - noctx
    - copyloopvar
    - intrange
    - wastedassign
    - unconvert
    - unparam
    - prealloc
    - gosec
    - nolintlint
    # стиль
    - gocritic
    - misspell
    - revive
    # размер / сложность / дубли (size gate)
    - funlen
    - gocyclo
    - gocognit
    - dupl
    # архитектура
    - depguard
  settings:
    revive:
      rules:
        # ВНИМАНИЕ: ключ максимума называется `max`, НЕ `maxLines`.
        # revive нормализует имена аргументов как ToLower(ReplaceAll(name,"-","")),
        # поэтому `maxLines`/`max-lines` молча игнорируются и лимит не применяется.
        - name: file-length-limit
          arguments: [{ max: 500, skipComments: true, skipBlankLines: true }]
        # Только правила с нулевым/дешёвым легаси-шумом (подтверждается первым прогоном),
        # чтобы revive можно было гонять в full-file режиме вместе с size-гейтом.
        - name: var-naming
        - name: receiver-naming
        - name: error-return
        - name: error-strings
        - name: errorf
        - name: unreachable-code
        - name: superfluous-else
        - name: indent-error-flow
        - name: context-as-argument
        - name: redundant-import-alias
        - name: unused-parameter
        # `exported` (godoc на каждый экспорт) сознательно не включаем — проект так не документируется, шум.
    funlen:
      lines: 60
      statements: 40
      ignore-comments: true
    gocyclo:
      min-complexity: 20
    gocognit:
      min-complexity: 25
    dupl:
      threshold: 120
    gocritic:
      enabled-tags: [diagnostic, style]
      # disabled-checks — по результатам первого прогона (кандидаты: ifElseChain, singleCaseSwitch, hugeParam)
    gosec:
      excludes: [G304, G306]   # чтение/запись файлов по путям — суть продукта; контейнмент обеспечивает pathpolicy; уточнить по прогону
    misspell:
      locale: US
    errorlint:
      errorf: true
    depguard:
      rules:
        domain-stdlib-only:                       # выполняется уже сейчас — включаем в Part 0a
          files: ["**/install/integrationctl/agentplugins/domain/**", "!$test"]
          allow: ["$gostd"]
        ports-only-domain:                        # выполняется уже сейчас — включаем в Part 0a
          files: ["**/install/integrationctl/agentplugins/ports/**"]
          allow:
            - "$gostd"
            - "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
            # ОСОЗНАННОЕ ИСКЛЮЧЕНИЕ (решение A-1). install/integrationctl/ports содержит Command и
            # CommandResult — простые data-типы без поведения и без I/O, на которых уже построен
            # providers.CommandRunner (activator.go:18-23) и treeCommandRunner (native_identity.go:39).
            # Их рантайм-реализация (adapters/process.OS) остаётся адаптером и сюда не попадает.
            # Альтернатива — дублировать типы в domain и конвертировать на каждом вызове — дала бы
            # два источника правды ради формальной чистоты. Описано в docs/ARCHITECTURE.md.
            - "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
        usecase-through-ports:                    # deny растёт по мере устранения нарушений
          files: ["**/install/integrationctl/agentplugins/usecase/**", "!$test"]
          deny:
            - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters
              desc: usecase depends on ports, not adapters
            - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers
              desc: usecase depends on ports, not providers
            - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients
              desc: usecase never sees client adapters
            # Part 1 добавит:
            # - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/adapters   (pathpolicy)
            # - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner
        # Part 1 добавит single-pathpolicy-implementation (решение R-1):
        #   files: ["**/agentplugins/**", "!**/install/integrationctl/adapters/pathpolicy/**", "!$test"]
        #   deny: любые собственные реализации ports.PathPolicy вне adapters/pathpolicy
        #   (закрепляется также archtest-ом: тип с полным набором методов PathPolicy вне разрешённого пакета = ошибка)
        # Part 2 добавит clients-no-upward:
        #   files: ["**/agentplugins/clients/**", "!$test"]; deny: providers, planner, usecase, adapters/clientdetect
        #   files: ["**/agentplugins/clients/shared/**"]; deny дополнительно: clients/<id>, clients/all
        # Part 8 добавит generic-no-concrete:
        #   files: ["**/agentplugins/providers/**", "**/agentplugins/planner/**", "**/agentplugins/adapters/clientdetect/**", "!$test"]
        #   deny: clients/claude, clients/codex, …, И clients/all (allow: clients, clients/shared)
        # Part 10 добавит cli-no-core-internals:
        #   files: ["**/cli/plugin-kit-ai/internal/agentpluginscli/**", "!$test"]; deny: providers, pathpolicy
        #   (planner НЕ в deny: CLI продолжает пользоваться тонким публичным фасадом, см. §3.5)
  exclusions:
    generated: lax
    paths:
      - "docs/generated"
      - ".*_gen\\.go$"
      - "website/"
    rules:
      - path: _test\.go
        linters: [funlen, gocyclo, gocognit, dupl, gosec]
      - path: _test\.go
        linters: [revive]
        text: "file length is"
      # BEGIN LEGACY SIZE BASELINE (shrink-only; scripts/check-lint-baseline.sh)
      # Генерируется первым full-file прогоном size-гейта; ожидаемо ~30-45 файлов на 3 модуля.
      # Записи файлов ИЗ СКОУПА удаляются той частью рефакторинга, которая режет файл.
      # Записи файлов ВНЕ СКОУПА (см. §11) остаются навсегда как отдельная будущая задача.
      - path: ^install/integrationctl/agentplugins/providers/activator\.go$
        linters: [revive, funlen, gocyclo, gocognit, dupl]
        text: "file length is|is too long|too many statements|cyclomatic complexity|cognitive complexity|lines are duplicate"
      # В СКОУПЕ (выходят из baseline по ходу плана): service.go, group.go, repair.go, remove_group.go,
      #   stager.go, native_identity.go, planner.go, detector.go, opencode_native.go, gemini_native.go,
      #   kiro_acp.go, cline_native.go, windsurf_native.go, kiro_native.go, cli/add.go
      # ВНЕ СКОУПА (остаются): transaction/kernel.go, domain/directory.go, directoryv1/validate.go,
      #   statev2/store.go, statemigration/migrate.go, discoveryv1/models.go, packageview/source_windows.go,
      #   conformance/yaml_budget.go, cli/source.go, cli/add_multi.go, cli/lifecycle.go, cli/read.go, cli/search.go
      # (точный список — из прогона)
      # END LEGACY SIZE BASELINE

formatters:
  enable: [gofmt, goimports]   # gofumpt сознательно нет — переформатирует много старых файлов, шум в диффах
  settings:
    goimports:
      local-prefixes: [github.com/777genius/plugin-kit-ai]

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

### 6.4. Механизм «новое строго, легаси — по мере касания» (два прогона)

Один режим на всё не работает: `new-from-*` фильтрует по изменённым строкам, а `funlen`/`gocyclo`/`file-length-limit` репортят на строке объявления функции/файла — добавление 40 строк в середину старой 200-строчной функции такой фильтр не поймает. Поэтому:

- **Прогон A** — корректность и стиль, только изменённые строки: все линтеры, кроме size/arch. CI: `only-new-issues: true` (на PR — патч через GitHub API; на push — `new-from-rev=<before>`). Локально: `golangci-lint run --new-from-merge-base=$(BASE)`; в checkout нужен `fetch-depth: 0`.
- **Прогон B** — size/arch-гейт, полные файлы, всегда: `golangci-lint run --enable-only=revive,funlen,gocyclo,gocognit,dupl,depguard`. Легаси-нарушения размера покрыты baseline-исключениями (сгенерированы этим же прогоном в Part 0a); всё остальное строго: новый файл > 500 строк кода, новая функция > 60/40, рост сложности в файле вне baseline, любой запрещённый импорт — красный. `depguard` в full-file режиме — граница держится всегда, а не только на изменённых строках.
- **Baseline shrink-only**: `scripts/check-lint-baseline.sh <base-ref>` извлекает блок `BEGIN/END LEGACY SIZE BASELINE` из `.golangci.yml` в HEAD и из `git show <base-ref>:.golangci.yml`, `comm -13` → любые добавленные `- path:` = ошибка. Файл из скоупа выходит из baseline в той части, которая его режет (критерий приёмки части). В Part 11 блок содержит только файлы вне скоупа (§11).

### 6.5. CI job (`lint.yml` с `workflow_call`)

```yaml
name: Lint
on:
  workflow_call:
jobs:
  lint:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        module: [".", "cli/plugin-kit-ai", "install/integrationctl"]
    steps:
      - uses: actions/checkout@<sha>   # v7.0.1, пин по SHA как остальные actions в репо
        with: { persist-credentials: false, fetch-depth: 0 }
      - uses: actions/setup-go@<sha>    # v7.0.0
        with: { go-version: "1.25.13", cache: false }
      - name: Lint changed lines (correctness, style)
        uses: golangci/golangci-lint-action@<sha>   # v9.3.0
        with: { version: v2.13.2, working-directory: ${{ matrix.module }}, only-new-issues: true }
      - name: Size and architecture gate (full files)
        uses: golangci/golangci-lint-action@<sha>   # v9.3.0
        with:
          version: v2.13.2
          working-directory: ${{ matrix.module }}
          only-new-issues: false
          args: --enable-only=revive,funlen,gocyclo,gocognit,dupl,depguard
      - name: Baseline is shrink-only
        if: matrix.module == '.'
        run: bash scripts/check-lint-baseline.sh "origin/${{ github.base_ref || 'main' }}"
```

Матрица — 3 модуля, потому что весь код ядра там; `install/plugininstall` и `sdk` из `go.work` линтом не покрываются (не меняются планом), но покрываются `go vet` в `core-fast` (§10).

`ci.yml`: `jobs.lint: uses: ./.github/workflows/lint.yml` параллельно с `test`. Почему отдельный job, а не шаг в `test`: `test` — критический путь 9-27 мин; lint в трёх параллельных матричных job ожидаемо 3-5 мин и не удлиняет путь; при этом он в том же workflow «Required», т.е. входит в required-гейт.

Версии: golangci-lint v2.13.2 (релиз 2026-08-27) и golangci-lint-action v9.3.0 (2026-06-29) — последние стабильные по GitHub Releases на дату плана (VERIFIED). Локально нужен апгрейд с v1.64.8 (`brew upgrade golangci-lint`): v1 не читает `version: "2"` и не содержит правило `file-length-limit`.

### 6.6. Документация

- `CONTRIBUTING.md` → «Lint gate and size limits»: команды (`make lint`, `make lint BASE=origin/<base>`, `make test-core`), два режима, таблица лимитов, правило baseline (shrink-only; как вывести файл), политика `//nolint` (только с причиной, `nolintlint` проверяет), ссылка на `docs/ARCHITECTURE.md`.
- `docs/ARCHITECTURE.md` → «Agent Plugins core»: таблица слоёв (`domain → ports → usecase → adapters/providers/planner/clients → cli → cmd`), правила импорта 1:1 с depguard, **явный абзац про два принятых исключения** (`ports → install/integrationctl/ports`; `clients → nativeconfig → hujson`) с обоснованием, лимиты размера, «как добавить клиента: 3 точки правки + contracttest» (наполняется с Part 2-3).
- PR-шаблон: чекбокс `make lint`.

### 6.7. Объём, приёмка, оценки

Объём: ~350 строк конфигов/скриптов/доков + автофиксы первого прогона (ожидаю < 300 строк правок).

Критерии приёмки:
1. `lint` job зелёный на PR, wall ≤ 5 мин; `test` не удлинился.
2. Прогон B без baseline даёт список файлов; с baseline — 0 findings; baseline-скрипт падает на тестовой ветке с добавленной записью.
3. Негативные проверки в тестовой ветке (не мерджатся): новый файл на 501 строку кода → красный (**и заодно подтверждает, что ключ `max` сработал**: если правило молча не применилось, тест будет зелёным — это и есть детектор ошибки F-9); функция на 61 строку → красный; `import ".../agentplugins/providers"` в `usecase` → красный; добавление строки в `activator.go` без разбиения → зелёный (baseline работает).
4. `domain-stdlib-only` и `ports-only-domain` включены и зелёные.
5. `make test-core` зелёный локально с обходом git-хука.
6. Документация и PR-шаблон обновлены; `make lint` воспроизводит CI локально.

Надёжность 8/10, уверенность 8/10. Оговорки: состав `gocritic disabled-checks` и `gosec excludes` фиксируется по первому прогону (риск: сотни легаси-находок gosec/gocritic — они не блокируют, но требуют осознанных excludes, иначе шум на каждом касании старого кода); порог `dupl` проверяется на известных дублях; поведение `depguard` с `relative-path-mode: gitroot` для glob `files:` проверяется негативными тестами п.3.

---

## 7. Part 0b — Guardrails рефакторинга (без изменения поведения)

Цель: измеримые ворота поведения и архитектуры до первого рефакторинга.

### 7.1. Файлы

- `.github/workflows/core-fast.yml` (см. §10): `lint` (через `lint.yml`), `test-core`, `cross-build`, `vet-all`.
- `.github/workflows/ci.yml`: добавить `push: branches: [main, master, 'refactor/**']`.
- Новый пакет `ap/internal/archtest` (см. §7.2).
- Golden-тесты:
  - `planner/golden_test.go`: `Plan` для 11 клиентов × 4 репрезентативных envelope (skills-only; stdio MCP; remote MCP + app; invalid components) → JSON плана + `UserActions`/`Warnings`/`LocalActions` в `testdata/golden/*.json`. **Отдельный кейс фиксирует расхождение `Detected`**: planner с пустой картой (как в `cmd/main.go:136`) и с реальной картой (как в CLI) дают разные результаты для `hasNativeCopilotBackend` — это текущее поведение, и оно не должно «выровняться» при переходе на `PlanRequest`;
  - `clientdetect/golden_test.go`: ID/порядок surfaces, `ConfigRoot`/`ExecutablePath` на darwin/linux/windows с фейковыми probes;
  - `providers/activator_golden_test.go`: `Activate`/`Deactivate` для каждого клиента с fake runner — outcome + список argv; отдельно `VerifyOnly=true` и `Confirmed=false`;
  - `cli` golden `--format json` для `add --dry-run` и `compat` на фикстурах;
  - `cli/add_normalize_target_test.go`: табличный тест `normalizeTarget` со всеми 11 case, 6 алиасами и **lenient pass-through** для неизвестного имени — контракт, который обязана сохранить `domain.ParseClientID` в Part 10.

### 7.2. Archtest: формальная метрика вместо ручных чисел (правка F-1)

Инструмент пишется на **stdlib** `go/parser` + `go/ast`, без `golang.org/x/tools/go/packages`: VERIFIED `x/tools` отсутствует во всех `go.mod`, а прецедент stdlib-архтеста в репозитории уже есть — `agentplugins/conformance/architecture_test.go:TestConformanceHasNoEffectDependencies` (режим `parser.ImportsOnly`).

**(a) Границы импортов** — дублируют depguard как исполняемый тест (работают и без линтера, например в обычном `go test ./...`): `parser.ParseDir(..., parser.ImportsOnly)` по каждому пакету ядра, сверка со списком allow/deny из §3.2.

**(b) Бюджет ветвлений по ClientID** — метрика определена формально:

> **Метрика:** количество AST-узлов `*ast.SelectorExpr`, у которых `X` — идентификатор, разрешающийся в импорт пакета `domain`, а `Sel.Name` соответствует регулярному выражению `^Client[A-Z][A-Za-z0-9]*$`, подсчитанное по non-test файлам (`!strings.HasSuffix(name, "_test.go")`) пакетов, не входящих в allow-list. Allow-list: `domain/clients.go`, `clients/<id>/**`, `clients/all/**`.

Свойства метрики: воспроизводима (один проход AST, без эвристик), не зависит от формы кода (`switch`, `if`, `map`-литерал считаются одинаково), не зависит от форматирования.

**Baseline генерируется первым прогоном самого инструмента** (`go run ./internal/archtest -update`) и коммитится как `internal/archtest/testdata/client_id_budget.json`. Абсолютные числа в этом документе **намеренно не приводятся**: три разных способа подсчёта дают три разных набора значений, и ручные оценки черновика не воспроизводятся ни одним из них. В критериях приёмки каждой части фигурирует только направление («бюджет пакета X → 0») и запрет на рост (ratchet).

**Признанные ограничения метрики** (важно для §12): она ловит только selector-выражения `domain.Client<X>`. Обходится строковым литералом ID (`domain.ClientID("cursor")`) или сравнением по `BackendFamily`. Это детектор регрессии, а не доказательство отсутствия клиент-специфики. Компенсация — ревью и `contracttest`, а не иллюзия полноты.

Объём: ~600 строк тестов + 2 workflow.
Совместимость: нулевое изменение поведения.
Приёмка: `core-fast` < 6 мин; archtest фиксирует baseline и падает при росте; golden-файлы закоммичены и детерминированы (повторный прогон без diff); негативная проверка — намеренный `case domain.ClientCursor` в generic-файле тестовой ветки → archtest красный.
Надёжность 9/10, уверенность 9/10.

---

## 8. Части рефакторинга

### 8.1. Part 1 — Ports & DIP: `PathPolicy`, `CommandRunner`, `PlanRequest`, явные optional-порты

Цель: usecase зависит только от портов; planner получает всё через запрос; скрытые контракты становятся явными.

Изменения:
- `ap/ports/paths.go`: `type PathPolicy interface { ValidateLeafID(string) error; RequireContainedChild(base, candidate string) error; RequireExactPath(expected, candidate string) error }`. Реализация — `pathpolicy.Policy{}` в существующем `install/integrationctl/adapters/pathpolicy` (методы-обёртки над функциями; логика не меняется).
- **Безопасность порта (правка R-1).** Инверсия `pathpolicy` в интерфейс создаёт новую возможность: подставить no-op/permissive реализацию (тестовый фейк или «удобный» дефолт) и незаметно отключить проверки `RequireExactPath` → `RequireContainedChild` → `os.Lstat` по symlink-предкам (`pathpolicy.go:84,111,130-145`). Поэтому:
  - depguard-правило `single-pathpolicy-implementation` + archtest: тип с полным набором методов `PathPolicy` вне `adapters/pathpolicy` — ошибка (в тестах разрешено только через явный helper, который возвращает настоящую `pathpolicy.Policy{}`);
  - `contracttest.RunPathPolicy` с негативными кейсами: symlink-предок → ошибка; `..`-escape за пределы base → ошибка; точное несовпадение пути → ошибка; относительный путь → ошибка. Харнесс обязателен для любой будущей реализации.
- `usecase.Service` получает поле `Paths ports.PathPolicy`; 5 вызовов → `service.Paths.RequireExactPath`. Проверка «agentplugins service dependencies are incomplete» в `apply()` и входные проверки `Remove/Repair/RemoveGroup/ApplyGroup` расширяются на `Paths` — **fail-fast, без тихого fallback** (решение F-6; это инвариант безопасности, а не удобство).
- `planner.Planner` получает `Paths ports.PathPolicy` (используется в `Plan`, `ResolveTarget`, `targetRoot` вместо `pathpolicy.*`); nil → ошибка `planner path policy is required`.
- **Обновление мест конструирования.** VERIFIED на HEAD: `usecase.Service{}` — 23 (7 файлов + 6 in-package), `Planner{}` — 32 строки (21 in-package `planner`, 3 non-test CLI, 1 `cmd/main.go`, 1 `repotests/agentplugins_native_fixture_test.go`, остальное — тесты `usecase`/`providers`). Планируем по верхней границе **≈55-62 места**. Все обновляются через два test-helper'а (`usecasetest.NewService(opts…)`, `plannertest.NewPlanner(opts…)`), чтобы следующие части не повторяли эту правку. Затрагиваются файлы **вне ядра**: `cli/internal/authoring/commands/{installer_test.go, packed_installer_test.go}`, `repotests/agentplugins_native_fixture_test.go` — их компиляцию ловит новый job `vet-all` (§10).
- `ports.CommandRunner` (+ optional `TreeCommandRunner`, `DuplexCommandRunner`, `DuplexCapabilityRunner`) переезжают из `providers`/`kiro_acp.go`/`native_identity.go` в `ap/ports/runner.go`. Типы `Command`/`CommandResult` остаются в legacy `install/integrationctl/ports` — это **осознанное исключение A-1**, закреплённое allow-правилом `ports-only-domain` (§6.3) и абзацем в `docs/ARCHITECTURE.md`. В `providers` — алиасы `type CommandRunner = ports.CommandRunner` до Part 11.
- `ports.DeliveryPlanner.Plan(ctx, domain.PlanRequest)`; `PlanRequest{Envelope, Client, Scope, PhysicalArtifactID, InstallIntent, Detected map[ClientID]DetectedClient}`. `Planner.Plan` внутри применяет intent (вызов `ApplyInstallIntent` переезжает из `usecase/intent.go` внутрь планировщика; функция остаётся экспортируемой в `planner/intent.go` как фасад, §3.5). `Planner.Detected` сохраняется как fallback при `request.Detected == nil` (для трёх конструкций в CLI до Part 10). Обновить 1 фейк. usecase перестаёт импортировать `planner`. **Расхождение `Detected` между `cmd/main.go` и CLI фиксируется golden-тестом Part 0b, а не выравнивается.**
- Optional-интерфейсы usecase → экспортируемые в `ap/ports/optional.go`: `ActivationPreflighter`, `AutomaticActivationClassifier`, `ActivationVerifierClassifier` (новый: `VerifierAvailable(client, plan, backendExecutable) bool`; реализуется в `providers.Activator` переносом тела `usecase.clientVerifierAvailable` — временно со switch, уходит в адаптеры в Part 7), `ManagedStdioPreflighter`, `PluginDataAwareStager`, `DataPathPreflighter`. Механизм type assertion остаётся (идиома Go, как `io.WriterTo`), контракт задокументирован в одном месте.
- `depguard usecase-through-ports`: добавить deny `install/integrationctl/adapters` (pathpolicy) и `agentplugins/planner`; добавить allow `pathcontract` (VERIFIED — уже импортируется usecase, в черновике был пропущен).

Объём: **~900 строк** (было ~600; увеличено из-за решения F-6: ≈55-62 места конструирования + два test-helper'а + негативные тесты PathPolicy).
Совместимость: экспортируемая поверхность `providers` не меняется; `planner.ApplyInstallIntent` остаётся экспортируемой навсегда; `Planner.Detected` сохранён до Part 10.
Приёмка: depguard-правило для usecase зелёное в полном составе; бюджет archtest для `usecase` уменьшился; все существующие тесты usecase/planner/cli проходят без изменения семантики; новые тесты «Service без Paths → ошибка incomplete dependencies», «Planner без Paths → ошибка», `contracttest.RunPathPolicy` негативные кейсы зелёные; `vet-all` зелёный (компиляция `authoring`/`repotests`).
Проверка: `core-fast`; golden Part 0b без diff.
Надёжность 9/10, уверенность 8/10 (уверенность снижена: объём правки конструкций больше, чем оценивал черновик, и часть их — вне ядра).

### 8.2. Part 2 — Контракт `clients`, `clients/shared`, перенос `nativeconfig`, дедупликация

Цель: точка расширения и общие helpers до миграции клиентов; DRY-долг закрыт до переноса кода.

Изменения:
- Новый `ap/clients`: контракт из §3.3 (`Adapter`, `HostDetector`, `Host`, `Detection`, `TargetLayout`, `PlanRefiner`, `PlanInput`, `CompatibilityLimiter`, `StagingLayout`, `Projector`, `ProjectionInput`, `Lifecycle`, `ActivationPreflighter`, `AutomaticActivator`, `ReadOnlyVerifier`, `RegistryInspector`, `PreparedRegistryInspector`, `SelectionReader`, `SelectionLayout`, `Env`, `RegistryFinding`), `Registry` (`NewRegistry`, `Lookup`, `All`, `As[T]`; **nil-Registry → ошибка, не дефолт**), пакет `clients/contracttest` (скелет: `RunAdapter` — ID валиден и есть в domain; **parity-тест «трейт ⇒ интерфейс» заводится сразу**, правка A-4; наполняется в Part 3-8).
- `ap/providers/nativeconfig` → `ap/adapters/nativeconfig` (git mv, 1507 строк; обновить импорты в gemini/opencode/cline/windsurf/activator). Пакет тянет `github.com/tailscale/hujson` и `adapters/atomicfile` — это и есть известное исключение §3.2, которое фиксируется в `docs/ARCHITECTURE.md` в этой же части.
- Новый `ap/clients/shared` (экспортируемые, с переносом тестов): `OnlyNativeComponents` (один вместо шести), `HasSupportedMCP`, `ComponentKindPresent`, `SupportedMCPNames`, `ManagedMarketplaceName`, `DecodeUniqueJSONValue`, `DecodeStrictJSONObject`, `ReadJSONManifestName`, `FoldJSONKey`, `ApplyStdioDataContract`, `ResolveStdioPaths` (из `stdio_paths.go`), `CloneObject`, `WriteJSON`, `ManifestFromEnvelope` (единый билдер вместо трёх `copyString`; варианты через опции `WithAuthorObject`/`WithAuthorNameEmail`), `ProjectMCPServers(root, envelope, names, dialect, pluginRoot, dataPath)` с диалектами `openai|cursor|kiro` (единый цикл), `RunClientCommand` (из `runClientResult`), `CommandOutputContains`, `FailedActivation`, `RequireExternalUninstall`, `AttestedUnknownVerification`, `ErrRecognizedNegativeEvidence`, `InspectUnqualifiedPluginRoot`, `NativeManifestIdentity`, `SameCleanPath`, `ManagedPackageDigest`, `BoundedNativeDiagnostic`, `RenameExclusive*` (из `rename_exclusive_*.go`, с build-тегами), `ManagedTargetRoot`, `DefaultStagingLayout`, `PromoteNativeReady`.
- **Обязательный перенос 11 unexported-хелперов** (VERIFIED, правка A-3): хелперы из generic-файлов `providers`, используемые клиент-специфичными файлами, уезжают в `clients/shared` **именно здесь**. Если этого не сделать, Part 5 упрётся в цикл `providers ↔ clients/<id>`.
- Старые приватные функции в providers/planner удаляются, вызовы переписываются на `shared`; 6 дублей предикатов схлопнуты.
- `depguard`: правило `clients-no-upward` (`clients/**` deny `providers`, `planner`, `usecase`, `adapters/clientdetect`; `clients/shared` дополнительно deny `clients/<id>`, `clients/all` — добавляется заранее).

Объём: ~1500 строк переноса + ~300 правок.
**Совместимость (правка O-3, явно подтверждено):** `providers.ManagedMarketplaceName` **остаётся экспортируемым** как тонкая обёртка над `shared.ManagedMarketplaceName` **до Part 11 включительно**. VERIFIED потребители вне `providers`: `cli/internal/agentpluginscli/read_reconciliation.go:238` (non-test), `usecase/service_test.go`, `repotests/agentplugins_codex_native_e2e_test.go` (root-модуль). Удаление обёртки раньше Part 11 сломает root-модуль, который `core-fast` не тестирует (ловит только `vet-all`).
Приёмка: `dupl` не находит дублей предикатов/манифестов/MCP-циклов; golden Part 0b без diff; depguard зелёный; тесты `nativeconfig` зелёные по новому пути; `contracttest` parity-тест «трейт ⇒ интерфейс» бежит (пока на пустом наборе адаптеров); `vet-all` зелёный.
Проверка: `go test ./ap/clients/... ./ap/providers/... ./ap/planner/... ./ap/adapters/nativeconfig/...`.
Надёжность 8/10, уверенность 8/10 (риск: при унификации `ProjectMCPServers` перепутать диалекты — ловят существующие `stager_test.go`/`codex_transport_test.go` и golden staging-дерева из Part 6).

### 8.3. Part 3 — Detection через registry

Цель: `clientdetect.Detector` без switch.

Изменения: `clients/<id>/detect.go` для 11 клиентов (перенос `detectX` дословно; хелперы через `clients.Host`); `clients/all` создаётся здесь (`func Default() *clients.Registry` регистрирует все 11 адаптеров; на этом шаге они реализуют только `Adapter` + `HostDetector`, далее те же типы получают остальные интерфейсы); `Detector` получает поле `Registry *clients.Registry` (**nil → ошибка**, не дефолт); `detect` итерирует `registry.All()`, `detectClient` = `As[HostDetector]` + generic-сборка `DetectedClient` из `clients.Detection` (бывшее `detectedClientWithSelectionSurfaces`); version probe остаётся generic. В каждом `clients/<id>` появляется блок compile-time assertions (правка A-4).

Объём: ~900 строк (в основном перенос).
Совместимость: `clientdetect.NewOS`, все поля `Detector` сохраняются. **Стоимость обновления тестов ниже, чем в черновике** (правка F-3): VERIFIED в тестах ровно 1 литерал `Detector{`, 25 вызовов хелпера `testDetector(...)` и 2 `NewOS(` — достаточно добавить реестр в один хелпер.
Приёмка: golden detection совпадает; бюджет archtest `detector` → 0; `detector.go` выходит из baseline; parity-тест в `clients/all`: каждый `domain.ClientDefinitions()` ID имеет ровно один адаптер и наоборот; `contracttest.RunHostDetector` (уникальные ID surfaces, детерминированный порядок, отсутствие I/O кроме probes — fake `Host` считает вызовы); compile-time assertions присутствуют во всех 11 пакетах.
Надёжность 9/10, уверенность 9/10.

### 8.4. Part 4 — Planning через registry (`TargetLayout`, `PlanRefiner`, `CompatibilityLimiter`)

Цель: `planner.go`/`compatibility.go`/`intent.go` без клиент-специфики.

Изменения: `clients/<id>/plan.go`; generic-pipeline `Plan` = базовый план из capabilities → `client_not_detected`/`scope_not_supported` → `TargetLayout` (default managed) → `componentDecisions` (generic; SSE-reason generic) → catalog compatibility (generic) → диагностики (generic) → `no_valid_components`/`no_supported_components` → `refiner.RefinePlan` (промоушены, user actions, warnings, ChatGPT mapping) → intent (`Traits.InstallIntents`; до Part 9a — через `InstallIntent.Validate`; prepare-ветка в `RefinePlan`). `DetectedPhysicalClient` → `domain.BackendSiblings`. `Compatibility()` → `As[CompatibilityLimiter]`. `Planner` получает `Registry` (nil → ошибка). Порядок вызова `RefinePlan` относительно generic-шагов фиксируется так, чтобы итоговые `UserActions` сохраняли текущий порядок строк (golden проверяет).

**Дополнительно (правка F-2):** `domain/directory_context7_preparation.go:20` — ветвление `len(request.Targets) != 1 || request.Targets[0] != ClientChatGPT` — устраняется здесь: либо трейтом `Traits.RequiresSingleTargetContext7Preparation`, либо обобщением через существующий `RequiresPersonalMappingForPrepare`, если при чтении вызывающего кода окажется, что семантика совпадает. Решение принимается в начале Part 4 и фиксируется в PR-описании.

Объём: ~700 строк.
Совместимость: **публичный фасад `planner` сохраняется навсегда** (§3.5): `Capabilities`, `Compatibility`, `ClientCompatibility`, `ApplyInstallIntent` (тонкая обёртка над registry), константы `KiroPrepareAction`, `ChatGPTAppBindingAction` (реэкспорт из `clients/kiro`/`clients/chatgpt`), `DetectedPhysicalClient`. VERIFIED потребители вне ядра: `cli/internal/authoring/{readiness,report}`, `authoring/commands/readiness_test.go`.
Приёмка: golden planner совпадает; бюджеты archtest `planner`, `compatibility`, `planner/intent` → 0, `domain` уменьшился на `directory_context7_preparation.go`; `planner.go` выходит из baseline; `contracttest.RunPlanRefiner` (refiner не меняет `ClientID/Scope/PhysicalArtifactID/TargetRoot/ActivePath`, не ставит `PlanReady` поверх `PlanUnsupported`, идемпотентен); `vet-all` зелёный.
Надёжность 8/10, уверенность 8/10.

### 8.5. Part 5 — Механический перенос клиент-специфичных файлов providers → `clients/<id>`

Цель: один раз переместить большие файлы, чтобы Part 6-8 были логическими, а не «переездными».

Перенос (git mv + смена package + экспорт необходимого): `claude.go`, `claude_probe.go` → `clients/claude`; `kiro_native.go`, `kiro_acp.go`, `kiro_pipe_*.go` → `clients/kiro`; `gemini_native.go` → `clients/gemini`; `opencode_native.go` → `clients/opencode`; `cline_native.go` → `clients/cline`; `windsurf_native.go` → `clients/windsurf`; `codex_marketplace_cleanup.go`, `codexPluginStatus`/`codexRegistryFinding`, `projectOpenAI*` → `clients/codex`; `projectChatGPT` → `clients/chatgpt`; `projectCursor*` → `clients/cursor`; copilot-парсеры, `copilotMarketplaceVersion`, `projectCopilotMarketplace` → `clients/copilot`. Соответствующие `*_test.go` переезжают дословно. Лестницы `*WithKernel/WithRename/WithOps/WithKernelRenameAndCapacity` схлопываются в одну функцию с `Env` и опциональным `Ops{Rename, RemoveAll}` (тест-швы сохраняются).

При переезде файлы сразу режутся по будущим интерфейсам контракта на `project.go` / `lifecycle.go` / `identity.go` / `native.go` (≤ 400 строк каждый), чтобы не заводить их в baseline под новым путём. Если для конкретного клиента резка раздувает PR, допускается перенос как есть с новой записью в baseline + allow-list переименований для `check-lint-baseline.sh` в этом PR (единственное разрешённое исключение shrink-only). Начать с Kiro (самый крупный: 539 + 669 + pipe) как пробы.

Переходное состояние: `providers` временно импортирует `clients/<id>` напрямую (тела `case` вызывают `kiro.ActivateNative(...)` и т.д.); depguard-правило `generic-no-concrete` включается только в Part 8.

Объём: **≈8800 строк перемещения** (VERIFIED: 4533 prod + 4267 test клиент-специфичных строк в `providers`; черновая оценка «~5000» занижена почти вдвое), ~300 строк правок; ревью с `git diff -M --stat`.
Совместимость: экспортируемая поверхность providers не меняется.
**Синхронизация:** это самая уязвимая к конфликтам часть (§5.2) — перед стартом снять `git log --oneline origin/main -- <переносимые файлы>`, после мерджа немедленно смерджить `origin/main`, к Part 6-7 переходить без паузы.
Приёмка: golden Part 0b без diff; build-теги (`_darwin/_linux/_windows/_unix`, `linux_amd64/arm64`) сохранены — `cross-build` зелёный на трёх GOOS; счётчик тест-функций равен; записи baseline для перенесённых файлов удалены (или явно переименованы); `vet-all` зелёный.
Надёжность 7/10, уверенность 7/10 (самая большая по объёму часть — почти вдвое больше, чем оценивалось; риск — сломать build-теги и внутренние тесты при смене package, плюс максимальное окно конфликтов с `main`; митигируется cross-build, тем, что тесты остаются внутренними, и процедурой §5.2).

### 8.6. Part 6 — Staging через registry (`Projector`, `StagingLayout`)

Цель: `stager.go` без switch.

Generic-pipeline `stage`: validate plan paths (`Paths` + `StagingLayout.ValidateTargetLayout`) → reserved stdio env → mkdir target root → staging path (`StagingLayout.StagingBase` + sha256(operationID)) → copy snapshot → sanitize (generic: hooks, invalid/unsupported skills, `mcp.json`, `.app.json` удаляется всегда, extensions) → `Projector.Project` (адаптер; возвращает native objects) → digest → objects = `managed_package_directory` + adapter objects → `StagedDelivery`. `Discard` использует `StagingLayout.StagingBase`.

Изменения: `clients/<id>/project.go` реализует `Projector` (перенос тел `case` из `stage`; `buildXNativeObjects` уже в пакетах); ChatGPT `Projector` пишет `.app.json` сам; `Stager` получает `Registry` (nil → ошибка) и `Paths ports.PathPolicy` (nil → fail-fast), `LauncherSource` пробрасывается в `ProjectionInput.Launcher`; `ManagedMCPNames` → `SelectionReader` (файл `managed_selection.go` относится к Stager).

Объём: ~600 строк.
Совместимость: поля `providers.Stager` сохраняются; `StageWithPluginData`, `PreflightManagedStdio` сохраняются.
Приёмка: бюджеты archtest `stager`, `managed_selection` → 0; `stager.go` выходит из baseline; `stager_test.go` зелёный; новый golden «дерево staging-директории + digest файлов для каждого клиента на фиксированном envelope»; `contracttest.RunProjector` (повторный `Project` на чистом staging даёт те же объекты; запись только внутри `StagingPath` — проверяется walk; адаптер не создаёт объект `managed_package_directory`).
Надёжность 8/10, уверенность 8/10.

### 8.7. Part 7 — Lifecycle через registry (три PR)

- **7a** — CLI-registry клиенты: cursor, chatgpt, codex, copilot (+vscode), claude.
- **7b** — kiro (ACP duplex, prepare-intent, `PreflightActivation` с `DuplexCapabilityRunner`).
- **7c** — native-config клиенты: gemini, opencode, cline, windsurf (однородны: `nativeconfig.Kernel` из `Env`; `committedNativeCleanup` → `shared`).

В каждом PR `providers.Activator` диспетчеризует в адаптер, если `As[Lifecycle]` найден, иначе — в legacy switch (Strangler); `PreflightActivation`/`AutomaticallyActivates`/`VerifierAvailable` — аналогично через `As[ActivationPreflighter]`/`As[AutomaticActivator]`/`As[ReadOnlyVerifier]`. После 7c switch и все приватные `activateX/verifyX/deactivateX/runCopilot*/runClaude*` удаляются; `Activator` ≈ 150-200 строк: generic-инварианты `Activate` (mismatch ID/path, `RequireContainedChild`, real dir), `PreflightActivation` = `InstallIntent.Validate` + адаптер, `Deactivate` = `ctx.Err` + адаптер. Общие ветки `InstallIntentPrepare` и `ActivationComplete && !AutomaticallyActivates` остаются generic в диспетчере (по форме не клиент-специфичны; prepare-логика ChatGPT/Kiro — в адаптерах).

Объём: 400-600 строк на PR.
Совместимость: поля `providers.Activator{Runner, NativeConfig}` сохраняются; добавляется обязательный `Registry` (105 конструкций `Activator{}` в тестах получают его через общий helper, введённый в Part 1).
**Синхронизация:** 7a-7c выполняются подряд без пауз (§5.2) — `activator.go` менялся в `main` 23 раза за 60 дней.
Приёмка: golden activator (outcome + argv fake-runner) совпадает; бюджет archtest `activator` → 0 после 7c; `activator.go` выходит из baseline; `activator_test.go` разнесён по `clients/<id>/lifecycle_test.go` без потери кейсов (счётчик); `contracttest.RunLifecycle`: `VerifyOnly=true` не порождает мутирующих argv (fake runner классифицирует `list` / `marketplace add` / `plugin install` …), mismatched IDs → ошибка, `Deactivate` с `Confirmed=false` не вызывает runner, `Runner==nil` → manual outcome/ошибка без паники; usecase golden (`service_test.go` сценарии) без diff.
Надёжность 7/10, уверенность 8/10 (самая логически насыщенная часть: порядок проверок в `Activate` важен для текстов outcome; golden по argv и outcome — обязательное условие).

### 8.8. Part 8 — Identity через registry (`RegistryInspector`, `PreparedRegistryInspector`)

Изменения: `native_identity.go` оставляет generic-оркестрацию (`observeIdentity`: timeout, объединение prepared/native findings, digest verify через `Stager.Verify`, сборка `NativeIdentityObservation`); `inspectNativeRegistry` → `As[RegistryInspector]`; `nativeAttempted` = `inspector.UsesNativeRegistryExecutable() && NativeRegistryExecutable != ""`; prepared → `As[PreparedRegistryInspector]`, иначе `shared.InspectUnqualifiedPluginRoot`; `inspectCodexCLI/inspectCodexFiles/inspectCodexCache` → `clients/codex/identity.go`; `inspectCopilotCLI/copilotRegistryFindingAt` → `clients/copilot/identity.go`; `inspectKiroRegistry` → `clients/kiro`; `inspectGeminiRegistry`, `inspectWindsurfRegistry` — уже в своих пакетах. Включается depguard `generic-no-concrete` (providers/planner/clientdetect → deny `clients/<id>` **и `clients/all`**; allow `clients`, `clients/shared`): после этого прямых импортов не остаётся, и тезис «сборка с подмножеством клиентов возможна» становится проверяемым, а не декларативным.

Объём: ~700 строк.
Приёмка: бюджет archtest `native_identity` → 0; `native_identity.go` выходит из baseline; depguard `generic-no-concrete` зелёный; `native_identity_*_test.go` зелёные (часть переезжает в `clients/<id>/identity_test.go`); `contracttest.RunRegistryInspector` (`Runner==nil` → Indeterminate без паники; отмена `ctx` → `ctx.Err()`; `managed==nil` + найдено → Collision; `managed!=nil` + найдено по ожидаемому пути → Expected).
Надёжность 8/10, уверенность 8/10.

### 8.9. Part 9 — usecase и domain: трейты вместо ID; резка монолитов (два PR, разбиение обязательно)

Разбиение на 9a/9b **обязательно** (правка O-4), а не «допускается»: смешивать смену семантики (трейты) и крупную структурную резку в одном PR — гарантированный способ получить нечитаемое ревью на самой опасной функции проекта.

**Part 9a — трейты (без резки).**
- `domain.ClientTraits{InstallIntents []InstallIntent; LifecycleKind (cli_registry|native_config|manual|prepared); UsesManagedStdioLauncher, HonorsOpenAIMCPAuthHints, SupportsPreparedRecovery, RequiresPersonalMappingForPrepare bool}` на `ClientDefinition` (не сериализуется); таблица `clientDefinitions` полностью декларативная (SSE/App — явные аргументы, без `if id ==`); `InstallIntent.Validate` читает трейты; `domain.BackendSiblings/SharesBackend`; `directoryEligibilityTargets` через siblings.
- usecase: `nativeLifecycleClient`, `openAIOAuthApplies`, `component_readiness.go`, `group.go` (Codex recovery), `intent.go` ChatGPT, `sameNativeBackend(..., ClientCopilot)` → трейты/siblings.
- `contracttest` parity-тест, введённый в Part 2, впервые наполняется реальными проверками: `LifecycleNativeConfig` ⇒ адаптер реализует `Lifecycle` и не заявляет `UsesNativeRegistryExecutable`; `prepare ∈ InstallIntents` ⇒ реализует `ActivationPreflighter`.
- Объём: ~400 строк. Приёмка: бюджеты archtest `usecase`, `domain` → 0 (строки таблицы в allow-list); JSON `ClientCapabilities` без изменений (golden `compat` CLI).

**Part 9b — резка монолитов usecase.** Строго extract-method без изменения порядка вызовов:
- `apply()` (371 строки / cyclo 152) → `validateApplyInput`, `resolveInstallation`, `planAndPreflight`, `resolveBinding`, `dryRunPath`, `noChangeOrResume`, `stageAndCommit`;
- `applyGroup()` (620 / 451 statements / cyclo 234) → по фазам (plan targets → collide on physical backend → observe identity → stage all → kernel group apply → activate each → persist);
- `Repair` (318 / 93) → `verifyRepairPreconditions`, `repairNative`, `repairPackage`, `persistRepair`;
- **`RemoveGroup` (240 строк / cyclo 68)** — добавлено по правке O-1: функция находится в `usecase/remove_group.go`, то есть внутри скоупа DoD, но в черновике не упоминалась. Режется на `validateRemoveGroupInput`, `resolveGroupBindings`, `removeGroupNative`, `persistRemoveGroup`.

Объём: **~600 строк логики с ожидаемым ростом LOC +15-30 %** (правка O-4). Честная оценка: свести `applyGroup` (620 строк / 451 statements / cyclo 234) под лимиты 60/40/20 потребует ≥12-16 extract-method с прокидыванием общих локальных переменных через структуру-контекст фазы; «без роста LOC» — нереалистично. Рост принимается: читаемость и тестируемость важнее абсолютного LOC, а lint-гейт меряет размер функции и файла, а не суммарный объём пакета.
Приёмка: `service.go`, `group.go`, `repair.go`, `remove_group.go` выходят из baseline; все usecase-тесты (6.7k строк) без изменения семантики; golden групповых сценариев без diff.
Надёжность 7/10, уверенность 7/10 (риск — `applyGroup`; митигируется тем, что резка идёт под `group_test.go` 1383 строки + golden групповых сценариев + отдельный PR).

### 8.10. Part 10 — CLI и composition root

Изменения: `domain.ParseClientID(string) (ClientID, bool)` вместо 11-case switch в `add.go:426-449`. **Семантика сохраняется полностью** (VERIFIED, правка 1.10): алиасы `github-copilot`, `vs-code`, `claude-code`, `gemini-cli`, `open-code`, `devin` **и** lenient pass-through — неизвестное имя возвращается как `domain.ClientID(strings.ToLower(strings.TrimSpace(value)))`, а не отвергается; это закреплено табличным тестом ещё в Part 0b. `App` получает `Planner ports.DeliveryPlanner`, `Targets ports.DeliveryTargetResolver`, `Clients *clients.Registry`; три прямых конструирования `clientplanner.Planner{}` (`interactive_targets.go:284`, `lifecycle.go:611`, `read.go:379`) заменяются на `app.Planner.Plan(ctx, PlanRequest{Detected: …})` / `app.Targets.ResolveTarget`; Copilot/VSCode ветки → `domain.BackendSiblings`; Kiro/ChatGPT prepare-ветки → `Traits.InstallIntents`/`RequiresPersonalMappingForPrepare`; `providers.ManagedMarketplaceName` в `read_reconciliation.go` → `shared.ManagedMarketplaceName`; `cmd/main.go` строит `all.Default()` один раз и передаёт в `Activator/Stager/NativeIdentityObserver/Detector/Planner/App`; depguard `cli-no-core-internals`. Опционально (если не раздувает PR): `App.Lifecycle` за интерфейсом `Lifecycle` для тестируемости CLI.

**Важно (правка A-5):** запрет «CLI не импортирует planner» относится к прямой композиции `clientplanner.Planner{}`. Тонкий публичный фасад (`Capabilities`, `Compatibility`, `ClientCompatibility`, `KiroPrepareAction`, `ChatGPTAppBindingAction`, `ApplyInstallIntent`, `DetectedPhysicalClient`) CLI и `cli/internal/authoring/*` продолжают использовать — он не в deny-списке depguard.

Объём: ~500 строк.
Приёмка: бюджет archtest CLI → ≤ 3 (остаток — только UX-тексты про ChatGPT, если не выражаются трейтом; цель 0); все **277** CLI-тестов в `agentpluginscli` и 29 в `cmd/agentplugins` зелёные; golden CLI JSON без diff; `add.go` выходит из baseline. `source.go`, `add_multi.go`, `lifecycle.go`, `read.go`, `search.go` — **вне скоупа DoD** (§11), остаются в baseline.
Надёжность 8/10, уверенность 8/10.

### 8.11. Part 11 — Финализация

- Удалить переходные алиасы: `providers.CommandRunner`, обёртка `providers.ManagedMarketplaceName` (**только здесь — не раньше**, правка O-3; перед удалением проверить, что переведены все потребители, включая root-модуль `repotests/agentplugins_codex_native_e2e_test.go`), `Planner.Detected` fallback.
- `planner.ApplyInstallIntent` и остальной публичный фасад `planner` **НЕ удаляются** (§3.5) — у них есть потребители вне ядра.
- Включить «бюджет = 0» как жёсткое правило archtest (allow-list только `domain/clients.go`, `clients/<id>`, `clients/all`).
- LEGACY SIZE BASELINE содержит только файлы вне скоупа (§11); затянуть `gocyclo` до 15 для core-пакетов, если полный прогон чист; снять `only-new-issues` для core-пакетов (full-file для всех линтеров), если чисто.
- ADR «Client adapter contract & registry». **Номер проверить прямо перед созданием** (правка R-4): VERIFIED в `docs/adr/` уже есть два файла 0006 (`0006-authoring-inventory.md`, `0006-standard-first-authoring.md`), то есть коллизии номеров в этом репозитории реальны; выполнить `git fetch origin && ls docs/adr/` на актуальном `main` и взять первый свободный номер. Содержание: контекст, решение, инварианты, «как добавить клиента: 3 точки правки + contracttest», non-goals (отдельные Go-модули пока нет), **явное перечисление двух принятых исключений §3.2**.
- `docs/ARCHITECTURE.md` — финальная таблица слоёв/импортов; README `clients/contracttest`; тестовый адаптер `clients/internal/exampleclient` (только в тестах), доказывающий, что добавление клиента не требует правок generic-пакетов.

Приёмка: полный `Required` + `Coverage` зелёные на финальном PR в `main`; Codecov не ниже базы; `generated-check` без изменений; **финальный PR в `main` — только по прямой команде пользователя**.

---

## 9. DIP: исправление usecase → pathpolicy и usecase → planner

Проблема: `usecase` (слой политики) импортирует `install/integrationctl/adapters/pathpolicy` (адаптер с I/O — `os.Lstat` по компонентам пути) и вызывает `RequireExactPath` в 5 местах; плюс импортирует конкретный `planner` ради `ApplyInstallIntent`.

Варианты для pathpolicy:

1. **Рекомендуется:** порт `ports.PathPolicy` + реализация `pathpolicy.Policy{}` в существующем адаптерном пакете + явная инъекция `Service.Paths`/`Planner.Paths`; отсутствие → fail-fast «dependencies are incomplete». Надёжность 9/10, уверенность 9/10. Плюс: одна реализация, инвариант безопасности не ослабляется, тесты получают одно поле через helper. Минус: ≈55-62 конструкции в тестах и вне ядра обновить (одноразово) + нужен запрет второй реализации (§8.1, правка R-1), иначе инверсия сама становится дырой в безопасности.
2. Перенести `RequireExactPath` в domain как «чистую» функцию. Не подходит: функция делает `Lstat`-проверку symlink-предков — I/O-политика; перенос либо тянет I/O в domain, либо тихо ослабляет проверку. Надёжность 4/10, уверенность 9/10 (что это плохая идея).
3. Service locator (`init()`-регистрация адаптера). Скрытая зависимость, антипаттерн. Надёжность 5/10, уверенность 8/10.
4. Сохранить zero-value-friendly через дефолт `pathpolicy.Policy{}` при nil. **Отвергнуто явно** (F-6): вернуло бы импорт `pathpolicy` в usecase и нарушило `usecase-through-ports`; плюс тихий дефолт маскирует ошибку композиции. Надёжность 4/10, уверенность 9/10.

Для planner: intent передаётся в `ports.DeliveryPlanner.Plan(PlanRequest)`; usecase не знает о пакете planner. Альтернатива — отдельный порт `ports.InstallIntentApplier` — отвергнута: лишняя зависимость Service и второй вызов там, где должна быть одна операция (планирование с intent).

Скрытые optional-контракты usecase (5 type assertions) выносятся в `ports/optional.go` как экспортируемые интерфейсы — механизм остаётся, контракт становится документированным и единым. Признаём остаточную слабость: type assertion не даёт compile-time гарантии; компенсация — compile-time assertions в реализациях и parity-тесты (§3.3).

---

## 10. CI-механика для последовательных PR в базовую ветку

Исходные факты (VERIFIED): `ci.yml` («Required», единственный job `test`, 9-17 мин типично, хвост до 27 мин) триггерится на `pull_request` только для `main/master` → PR в `refactor/installer-core-clean-architecture` сегодня не запускают ни Required, ни Coverage (защиты нет вообще). Branch protection на `main` не настроен, `allow_auto_merge: false`. Ядро дешёвое: `go test ./ap/...` = 62.6 с локально, CLI-пакет 56 с; дорогая часть Required — root `./...` (`repotests`), `npm test`, сборка conformance-kit, `generated-check`, vet ×5 модулей. Локальный полный прогон на этой машине ненадёжен без обхода git-хука (§6.2).

### Вариант A (рекомендуется): «быстрые ворота на PR + полный Required после мерджа + локальный preflight»

`core-fast.yml`: `on: pull_request: branches: [main, 'refactor/**']` + `paths` (ядро, `go.mod/go.work/go.sum`, `.golangci.yml`, workflows). **Четыре** параллельных job:

| Job | Что делает | Ожидаемое время |
|---|---|---|
| `lint` | через `lint.yml`, матрица 3 модуля | 3-5 мин |
| `test-core` | `go test -count=1 -cover` для `ap/...`, `cli/…/agentpluginscli/...`, `cli/…/cmd/agentplugins/...`; проценты в job summary | 2-4 мин |
| `cross-build` | `GOOS=windows\|darwin\|linux go build ./...` в `install/integrationctl` и `cli/plugin-kit-ai` | 1-2 мин |
| **`vet-all`** (новый, правка C-1) | `go vet ./...` в каждом из **5** модулей `go.work`: `.`, `cli/plugin-kit-ai`, `install/integrationctl`, `install/plugininstall`, `sdk` | 1-2 мин |

Wall 4-6 мин. Это merge-gate для каждой части.

**Зачем `vet-all`.** `test-core` не компилирует код вне ядра, который от ядра зависит, а Parts 1, 2, 4, 10 меняют именно те API, которые он использует: VERIFIED 16+ non-test файлов (`cli/internal/authoring/{commands,mcpruntime,nativeimport,project,readiness,report,scaffold,skills}`, `cli/internal/terminalprompts/{huh,plain}.go`, `cli/cmd/agentplugins-conformance-adapter/main.go`, `cmd/agentplugins-registry-mirror/main.go` в root-модуле) плюс тесты `repotests/*` и `authoring/*_test.go`. Без этого job поломка компиляции видна только post-merge через 10-27 мин. `go vet` компилирует пакеты вместе с тестами, то есть ловит и `repotests`.

Остальное:
- `ci.yml`: добавить `push: branches: [main, master, 'refactor/**']` → полный Required идёт после мерджа каждой части в базовую ветку, асинхронно. Правило: следующая часть не мерджится, пока Required на базовой ветке красный (fix-forward PR).
- Локально перед открытием PR: `make lint test-core` (1-3 мин, с обходом git-хука, §6.2). Полный локальный Required не требуем: дублирует CI.
- Мердж части: `gh pr merge --squash` после зелёного `core-fast` и ревью; авто-мердж репозитория не включаем (настройка видна всем, требует отдельного согласия).
- Сразу после мерджа — `git merge origin/main` в базовую ветку (§5.2).
- Финальный PR в `main`: **только по прямой команде пользователя**.

**Что вариант A всё равно пропускает до post-merge** (честный список, правка C-3): `generated-check`, сборка conformance-kit adapter, `npm test`, полный root `./...` (`vet-all` ловит компиляцию, но не прогон `repotests`). Риск: поломка обнаруживается через ~15 мин после мерджа; fix-forward дёшев.

**Что получит финальный PR в `main` сверх перечисленного** (правка C-4, VERIFIED): `agentplugins-native-clients.yml` (`pull_request` на `main`, `paths=agentplugins/**`, матрица codex/claude/opencode × macos-15 + windows-2025, 5-13 мин), `dependency-review.yml`, `coverage.yml`, `codeql.yml`. Платформенных build-тегов в ядре **39 файлов**; сейчас `GOOS=windows/linux go build` зелёный (VERIFIED), и `cross-build` в `core-fast` держит это на каждой части — но **реальный прогон тестов на macOS/Windows будет только на финальном PR**. Это принятый риск: платформенные native-client тесты дороги и требуют раннеров, которые не окупаются на каждой части.

Почему не `pull_request` на `refactor/**` для Required: либо ждём 10-27 мин на каждую часть (то, чего избегаем), либо мерджим при «pending» — тогда это шум на странице PR и двойной расход минут (PR + push). Post-merge push даёт тот же сигнал на один PR позже при нулевом ожидании; базовая ветка не пользовательская. Надёжность 8/10, уверенность 8/10.

### Вариант B: `pull_request: branches: ['refactor/**']` в `ci.yml`, мерджить по зелёному `core-fast`, не дожидаясь Required

Плюс: сигнал приходит до мерджа, если ревьюер ждёт; минус: pending-статусы на PR, соблазн «подождать», двойные минуты (или отказ от `push`-триггера). Надёжность 7/10, уверенность 7/10.

### Вариант C: разбить Required на параллельные job (test per module ×5, vet, generated-check, conformance)

Wall 5-7 мин и для `main`. Полезно, но меняет общий CI-контракт репозитория (имя status-check «Required / test»), затрагивает всех и выходит за рамки рефакторинга ядра; только по отдельному согласию. Надёжность 7/10, уверенность 6/10 (не замерял root `./...` и conformance-kit по отдельности).

### Вариант D: мерджить части только по локальным прогонам без изменения триггеров

Не рекомендую: локальный полный прогон ненадёжен на этой машине и не воспроизводим для ревьюера. Надёжность 3/10, уверенность 9/10.

Дополнительно: Coverage бегает только на `main` — финальный PR его получит; для промежуточных частей контроль покрытия — счётчик тест-функций + `go test -cover` в `test-core` с выводом процентов по core-пакетам в job summary (без порога, информативно).

---

## 11. Definition of Done всего рефакторинга

**Скоуп DoD (сужен явно, правка O-1).** DoD относится к «ядру инсталла» в том смысле, в каком его оценивал аудит: пакеты `ap/domain`, `ap/ports`, `ap/usecase`, `ap/planner`, `ap/providers`, `ap/adapters/clientdetect`, `ap/clients/*`, `cli/plugin-kit-ai/internal/agentpluginscli` (файлы прямой композиции инсталла), `cli/plugin-kit-ai/cmd/agentplugins`.

**Явно ВНЕ скоупа** (остаются в LEGACY SIZE BASELINE как отдельная будущая задача, не блокируют DoD):
`ap/adapters/{transaction, statev2, statemigration, directoryv1, discoveryv1, packageview, packagedigest, loader, catalog, evidence, sourceacquisition, …}`, `ap/conformance/*`, `ap/transaction/kernel.go`, `ap/domain/directory.go` (файл целиком — режется только в части ветвлений по ClientID, размер остаётся), CLI-файлы вне прямой композиции инсталла: `source.go`, `add_multi.go`, `lifecycle.go`, `read.go`, `search.go`.

Критерии:

- Ноль ветвлений по `ClientID` вне allow-list **в скоуповых пакетах** (archtest; с учётом признанных ограничений метрики, §7.2).
- depguard-границы для `domain`/`ports`/`usecase`/`clients`/`providers`/`planner`/`clientdetect`/`cli` зелёные в full-file режиме; два принятых исключения (§3.2) выражены в конфиге явными allow-правилами с комментарием-обоснованием, а не умолчанием.
- Каждый из 11 клиентов проходит `contracttest`; тестовый `exampleclient` добавляется без правок generic-пакетов; в каждом `clients/<id>` присутствуют compile-time assertions.
- LEGACY SIZE BASELINE **не содержит файлов из скоупа**; `gocyclo ≤ 20` (цель 15) и `funlen 60/40` соблюдены **в скоуповых пакетах**.
- Golden-тесты (plan/detect/activate/staging/CLI JSON) идентичны базе `b443eb733` — с поправкой на легитимные изменения поведения, пришедшие из `origin/main` при синхронизации (§5.2); каждое такое обновление golden делается отдельным коммитом с обоснованием.
- Счётчик `func Test*` в ядре ≥ базового; Codecov не ниже.
- Документация: ADR (номер проверен на актуальном `main`) + `docs/ARCHITECTURE.md` + `CONTRIBUTING.md` обновлены; «добавить клиента» описано честно как 3 точки правки + contracttest.

---

## 12. Самооценка после выполнения плана (прогноз) против аудита

Прогноз понижен относительно черновика (8.2 → 7.0-7.5) по итогам критики (правка S-1). Основание: черновая оценка считала механизмы обнаружения (archtest/depguard/contracttest) более сильными, чем они есть, и не учитывала, что часть ядра остаётся вне скоупа.

| Критерий | Сейчас (аудит 5.8 средн.) | Прогноз | Обоснование и ограничения |
|---|---|---|---|
| SOLID | ~5 | **7.5-8** | SRP: диспетчеры без клиент-логики, адаптер = один клиент. OCP: новый клиент без правок generic-кода — `exampleclient` + archtest. LSP: `contracttest`. ISP: capability-интерфейсы + compile-time assertions. DIP: зависимости usecase/planner — порты, инфра в `Env`, composition root один. **Ограничение:** `As[T]` остаётся runtime-механизмом; assertions и parity-тесты снижают риск, но не устраняют его на уровне типов. |
| DRY | ~5 | **7.5-8** | Устранены 6 дублей предикатов, 3 билдера манифестов, 3 цикла MCP-проекции, лестницы `*WithKernel/WithRename`; `dupl` в гейте. **Остаток:** gemini/cline/opencode структурно похожи — возможная следующая абстракция «native-config lifecycle» над `nativeconfig` (вне плана). |
| Clean Architecture | ~6 | **7-7.5** | Направление зависимостей закреплено `depguard` full-file + archtest; domain stdlib-only; порты полные и явные; один composition root; CLI не конструирует ядро. **Потолок 7.5 из-за:** (A-1) `ports` зависит от legacy `install/integrationctl/ports`; (A-2) контракт `clients` тянет `nativeconfig` → `hujson`; (A-4) optional-capability через type assertion. Все три — названные и обоснованные исключения, но они реальны. |
| Модульность | ~7 → ~5 (аудит) | **7-7.5** | `clients/<id>` с узкой импортной поверхностью, инжектируемый Registry, `clients/all` не импортируется generic-пакетами (сборка с подмножеством клиентов действительно возможна — после отказа от nil-дефолта, A-3), `contracttest` для внешних адаптеров, ADR. **Ограничение:** без отдельных Go-модулей — сознательно; вынос потребует решить вопрос `nativeconfig`→`hujson`. |
| Качество кода | ~6 | **7** | Lint-gate с лимитами размера/сложности и baseline shrink-only; gofmt-чистота (7 файлов); cyclo `Activate`/`Deactivate` с 78/58 до < 15; `apply`/`applyGroup`/`Repair`/`RemoveGroup` порезаны; тесты сохранены + golden + харнесс; ADR/ARCHITECTURE/CONTRIBUTING. **Потолок 7 из-за O-1:** `adapters/*`, `conformance/*` и 5 крупных CLI-файлов остаются в baseline; «качество ядра» улучшено, «качество репозитория» — частично. |
| **Среднее** | **5.8** | **≈ 7.0-7.5** | Паритет с конкурентом (7.4), а не превосходство по каждому критерию. |

**Ограничения механизмов обнаружения — явно** (то, что нельзя выдавать за гарантии):

- `depguard` гарантирует только **направление импортов**. Он не видит, что логика клиента переехала в generic-пакет в виде `map[string]func()`.
- `archtest` «бюджет = 0» ловит только selector `domain.Client<X>`. Обходится строковым литералом ID или сравнением по `BackendFamily` (§7.2).
- `contracttest` гарантирует ровно то, что в нём написано, — не «соответствие контракту» вообще.
- `golden`-тесты фиксируют поведение на выбранных фикстурах; клиент-специфичный путь, не покрытый фикстурой, может измениться незаметно.

Уверенность, что итог будет ≥ 7.0 по каждому критерию при дисциплинированном выполнении: **7/10**. Надёжность самого плана (компилируемость и зелёные тесты на каждом шаге): **8/10**.

---

## 13. Главные риски

1. **Дрейф пользовательских текстов/порядка** при переносе planner/activator в адаптеры — закрывается golden-тестами Part 0b; без них рефакторинг начинать нельзя.
2. **Объём переезда внутренних тестов** (`package providers`) — самое дорогое по времени; тесты переезжают с кодом, экспортов ради тестов не добавляем, счётчик `Test*` контролируется. Реальный объём Part 5 — ≈8800 строк, почти вдвое больше первой оценки.
3. **Build-теги и платформенные файлы** при `git mv` — 39 файлов с тегами в ядре; `cross-build` в `core-fast` на каждой части; реальный прогон на macOS/Windows — только на финальном PR.
4. **`applyGroup`** (620 строк / 451 statements / cyclo 234) — самый опасный кусок Part 9b; резать extract-method под `group_test.go` + golden; ожидаемый рост LOC +15-30 %.
5. **Два источника правды о клиенте** (таблица domain vs адаптер) — адаптер отдаёт только `ID()`, определение читается из domain; parity-тест в `clients/all`. Формально после рефакторинга точек правки три (§3.4) — это записано честно, а не спрятано.
6. **Публичный JSON `ClientCapabilities`** — трейты в отдельной несериализуемой структуре; golden CLI-вывода.
7. **Временный импорт `providers → clients/<id>`** в Part 5-7 — переходное; depguard включается в Part 8; помечать в PR.
8. **Первый прогон `gosec`/`gocritic`** может дать сотни легаси-находок — не блокируют (прогон A только по новым строкам), но требуют осознанных `excludes`, иначе шум при каждом касании старого кода; заложить время в Part 0a.
9. **Post-merge Required** в варианте A обнаруживает поломку с задержкой — правило «следующая часть не мерджится при красной базовой ветке»; `vet-all` закрывает самый частый случай (компиляция вне ядра).
10. **AGENTS.md репозитория** содержит правила программы authoring (не удалять legacy `plugin.yaml` возможности) — рефакторинг не трогает legacy `install/integrationctl/{adapters,usecase,domain}` вне `agentplugins`; заявлять это явно в описаниях PR.
11. **Дивергенция с `origin/main` — ПРИНЯТЫЙ остаточный риск** (правка C-2, §5.2). 412 коммитов за 30 дней, 97 в ядре; Part 5 режет файлы на 4-5, из-за чего апстрим-правки станут `modify/delete`-конфликтами. Trunk-based отвергнут пользователем осознанно (мердж в `main` — только по прямой команде). Смягчение: немедленный `git merge origin/main` после каждой части, части по одному горячему файлу подряд без пауз, ручное разрешение конфликтов с сохранением обеих сторон, golden-тесты как детектор потери поведения. Остаточный уровень — **средний**; крупная апстрим-правка в `activator.go`/`stager.go` во время Part 5-7 будет стоить часов ручного переноса.
12. **Инверсия `pathpolicy` в интерфейс** сама по себе создаёт возможность подставить permissive-реализацию и отключить symlink/containment-проверки. Закрывается запретом второй реализации (depguard + archtest) и негативными `contracttest` (§8.1). Без этих мер правка DIP ухудшила бы безопасность, а не улучшила архитектуру.

---

## 14. Что не проверял / где не уверен

- Код конкурента AgentBridge (оценки взяты из аудита пользователя).
- Длительность CLI-тестов и root `./...` в CI по отдельности (замерено только локально: `ap/...` = 62.6 с, CLI-пакет = 56 с).
- Точный состав `gocritic disabled-checks`/`gosec excludes` и порог `dupl` — по первому прогону в Part 0a.
- Поведение `golangci-lint-action only-new-issues` для `push`-событий (`new-from-rev=before`) — по документации action; проверить на первом push в базовую ветку.
- Поведение `depguard` при `relative-path-mode: gitroot` для glob-паттернов `files:` — проверить на негативных тестах Part 0a (§6.7, п.3).
- Аргументы `revive file-length-limit` прочитаны в исходниках `mgechev/revive` (`rule/file_length_limit.go`, `rule/utils.go`, `internal/config/config.go`) на ветке `master`, а не на теге, вшитом в golangci-lint v2.13.2 — теоретически они могли отличаться в момент вендоринга. Негативный тест §6.7 п.3 закрывает это эмпирически на первом прогоне.
- Точное число мест конструирования `Service{}`/`Planner{}`: подсчёты разными способами дают 23/32 (grep по строкам на HEAD) против 23/39 (подсчёт критика). Планируем по верхней границе ≈62; точное число выяснится при первой компиляции после введения fail-fast.
- Совпадает ли семантика `domain/directory_context7_preparation.go:20` с `RequiresPersonalMappingForPrepare` — вызывающий код целиком не читал; решается в начале Part 4.
- Стоимость ручного переноса апстрим-правок в разрезанные файлы (риск 11) — оценка «часы» экспертная, не измеренная.
