# План: безопасный `add`, когда пакет уже существует

Статус: реализация согласованного scope в PR #1087; финальная E2E-квалификация и ревью выполняются после контрольного коммита.
Дата: 2026-11-21. База исследования: `main` `5f63d48bf4461600e6bb916a6e4997f9830545e1`.
Ветка: `docs/install-existing-state-plan-20261121`; [Draft PR #1087](https://github.com/777genius/universal-agent-plugins/pull/1087).

Этот документ заменяет набор вариантов из [v4 на fa5afb631](https://github.com/777genius/universal-agent-plugins/blob/fa5afb631452841e901096c19754dcd24abdd0ef/docs/plans/DRAFT-2026-11-21-install-existing-state-design.md).
Первый этап этой ветки изолировал план и исследования от [#1086](https://github.com/777genius/universal-agent-plugins/pull/1086). Следующим сообщением пользователь разрешил реализацию, дополнительные круги ревью и E2E в том же worktree. Новый путь документа избегает add/add-конфликта при последующем merge #1086; его Kiro/picker-изменения сюда не переносятся. Общий `ForeignBindingError`/`CheckSourceOwnership` сохраняет предложенные там имена и расширяет факты владения.

## 1. Решение и границы

Цель следующей реализации: пользователь saved `add` своевременно понимает, почему существующая установка мешает операции, что уже установлено и какое следующее действие действительно допустимо. Отказ сохраняет чужое состояние; частичный результат не выдаётся за полный успех.

| Включено в следующую реализацию | Отложено в отдельные задачи |
| --- | --- |
| Типизированная причина существующего владения, одна общая проверка и точная CLI/JSON-диагностика | Adoption, смена владельца, mixed ownership |
| Ранняя advisory-проверка корня, когда существующий resolver уже установил его identity | Автоматические remove + reinstall, purge-data, switch/rebind |
| Полная проверка графа до подтверждения; существующая финальная проверка под locks | Новая dependency-resolution или state/recovery подсистема |
| Понятное действие без потери scope/root/targets и без невозможного no-save-совета | Автоматическое переключение saved → imperative, новые флаги |
| Уточнение существующего partial output и безопасного повтора | Изменение порядка публикации manifest/lock, групповой rollback |
| Использование существующих activity/picker/result-компонентов | Новый wizard, переписывание адаптеров или расширение поддержки клиентов |

Текущая поставка #1086 остаётся отдельной: picker, progress и Kiro prepare-only. #1017 отдельно меняет представление результатов. Pi уже развивается в main; прежнее утверждение «только project scope» сюда не переносится. VS Code остаётся frozen.

План не обещает завершить saved-установку при конфликте владения. Для этого может понадобиться отдельная миграция; phase 1 должна честно объяснить это, сохранив данные.

## 2. Факты и исправленные гипотезы

1. Владелец installation распространяется на все её bindings: новый target не обходит чужое/unmanaged владение. `--no-save` не отменяет receipt.
2. Пакеты с обязательными UAP-зависимостями не допускаются в imperative flow. Совет «поставить корень с --no-save» для конфликта в дочерней зависимости неверен.
3. У пустой installation есть исключение в authority guard, но этого недостаточно для миграции. В выполненном legacy `github:repo@SHA` сценарии imperative → remove-all → saved add одинаковые `SourceKey`, commit и `TreeDigest` дают разные `SourceBindingID`: пути используют hints `direct-github-full-sha` и `uap-dependency-git`. Sticky-source guard отказывает до проверки смены digest. Это не доказательство поведения Directory или всех Git transport. Три варианта pin/digest и сохранность данных проверены в [spike evidence](evidence/2026-11-21-install-existing-state/README.md).
4. `SourceKey` отвечает за конфликт источника в environment; `ProductID` помогает Directory выбрать удержанный distribution. Они не взаимозаменяемы с `SourceBindingID`, installation ID или display name.
5. `uap.toml` публикуется перед `uap.lock`, затем применяются bindings. Это установленная desired-state модель. Ошибка одного binding не останавливает независимые действия; cancel/recovery/plan change и другие небезопасные состояния останавливают их.
6. No-op определяется immutable pin/digest и состоянием, а не только SemVer. Подготовленный пакет, manual activation и auth-pending не равны runtime verification и не должны запускать бесконечный reinstall.
7. Unrelated unmanaged source не блокирует sync по правилу foreign SourceKey: это проверено реальным CLI sync в fixture с разными native identities. Совпадение native identity всё ещё может блокировать отдельный target. Pending recovery действует на всю соответствующую область состояния.

Код-основание: [ownership](../../install/integrationctl/agentplugins/transaction/environment.go), [source identity](../../install/integrationctl/agentplugins/domain/identity.go), [coordinator](../../install/integrationctl/agentplugins/environment/coordinator.go), [imperative dependency guard](../../cli/internal/agentpluginscli/deps_package_facts.go), [publication](../../install/integrationctl/agentplugins/environmentapp/application.go), [intent](../../install/integrationctl/agentplugins/environmentapp/intent.go).

## 3. Матрица решений

Классифицируем отдельно identity, owner и наблюдаемое состояние. Это таблица для существующих типов, а не новый state engine. Несовпадающее/нечитаемое свидетельство нельзя превращать в «пакета нет».

| Случай | Решение saved add / диагностика |
| --- | --- |
| Installation отсутствует | Обычный текущий install workflow |
| Наш receipt, тот же pin, выбранные targets сходятся | Unchanged; отдельно сохранить manual/prepared/auth facts |
| Наш receipt, новый target, тот же pin | Добавить выбранную привязку; существующие targets сохранить |
| Нужна другая revision/digest | Существующая update/intent-change семантика; add не обновляет молча |
| Unmanaged binding того же source на любом target | Отказ для пакета целиком, включая bindings вне выбора пользователя |
| Receipt принадлежит другому environment | Отдельная причина владения; не предлагать imperative remove/no-save как обход |
| Корень зависит от unmanaged пакета | Назвать корень → зависимость и всех конфликтующих клиентов; не терять граф через no-save |
| Пустая retained installation | Проверять точную source identity и pin; не обещать migration через remove/add |
| Alias занят, short name переехал, ProductID неоднозначен | Существующая диагностика resolver; не угадывать по manifest name |
| Native identity занята чужим объектом | Target-specific отказ; чужие bytes сохраняются |
| Native registry недоступен или indeterminate | «Проверить не удалось»; не «свободно» и не выдуманный foreign conflict |
| Drift/missing/unsettled operation | Текущие repair/recovery правила; не лечить внутри этой UX-фазы |
| Другой unmanaged source | Не блокирует этот add только фактом существования |
| User и project | Отдельные существующие namespaces; native collisions всё равно проверяются адаптером |

Per-client пометки допустимы как информация о фактах. Они не превращают package-wide ownership refusal в разрешение на другие targets. Снятие галочки в `add` не означает uninstall.

## 4. Последовательность и UX

### 4.1 Ввод и ранняя диагностика

1. Существующий bootstrap выбирает effective scope/project root; обычные parser/intent/recovery ошибки имеют приоритет. Не сбрасывать состояние при ошибке чтения.
2. Существующий source resolver определяет canonical identity и удержанный pin. Short name может требовать Directory; multi-package source может требовать выбора. Не обещать проверку до любого network/download и не строить второй resolver ради раннего экрана.
3. Для новой известной canonical identity без explicit alias вызвать общий read-only ownership predicate до target picker. Explicit alias и уже сохранённый source/pin сначала проходят обычные решения resolver, чтобы ownership advice не скрывал alias/selector ошибки. Использовать текущую installer inspection/observation boundary, включая `InspectedBinding.Owner`; не собирать копию SourceKey/receipt-логики в UI. Полный `Inspect` не запускает native executable, но может хешировать деревья; не рекламировать его как бесплатный StateStore.Load. Финальную revalidation сохранить.
4. Если root identity ещё не определена, продолжить необходимый source/target selection, затем выполнить ту же проверку до подтверждения. Конфликты полного dependency closure проверяются после разрешения графа, также до подтверждения.
5. Ошибка одинакова по смыслу для picker, explicit `--target`, non-TTY и JSON. «До picker» — только для случая, где source identity уже известна. Не пытаться угадать выбранные targets заранее.

Пока идёт работа, использовать существующий `startCommandActivity`; activity не попадает в JSON stdout. Отмена закрывает preview и временные приобретённые файлы; manifest/lock/bindings не изменяются до принятой операции. Полный native preflight выполняется на выбранных клиентах, как сейчас; D-2 с запуском native registry до выбора исключён.

### 4.2 Отказ и следующий шаг

Human output отвечает на четыре вопроса: какой пакет/зависимость конфликтует, кто уже держит binding, почему текущий режим недопустим, что реально можно сделать дальше.

Для unreceipted установки без обязательных dependencies допустим совет о самостоятельном повторе `add --no-save`, только если существующие source/scope/owner проверки разрешают этот путь. Это отдельная явная команда пользователя. При другом receipt, dependencies, несовместимом alias или неопределённости такой совет отсутствует.

Выбранная минимальная реализация показывает безопасный `info` для конкретной installation и scope либо точную текстовую инструкцию, если аргумент нельзя переносимо представить. Она не строит no-save/remove-команды по неполным preview facts.

- Не предлагать auto-adopt, remove+add или `--purge-data` как миграцию.
- `--yes` подтверждает выбранную saved-операцию, не меняет её режим.
- `--alias` не переносится в imperative retry, а потеря alias не происходит автоматически.
- Remove-подсказка означает только отсоединение привязок; она не обещает дальнейший saved add.
- Если клиент требует manual uninstall, объяснить шаг; `--external-uninstalled` является attestation пользователя и не выставляется автоматически.
- Если безопасного готового пути нет, сказать это прямо. Цикл из двух взаимоисключающих советов запрещён.

Selector зависит от команды: remove/info принимают installation UUID; `add --no-save` и повтор add требуют поддерживаемый SOURCE с точным подходящим pin. До выбора targets показывать предложение выбрать клиентов, не подставлять конфликтующие bindings как намерение пользователя.

Сохранять explicit scope, точный project root и известные выбранные targets. Использовать `publicCLITargetName`/`PublicTargetFlagName` и один `--target a,b`: persisted `chatgpt` означает Web, а публичный `--target chatgpt` означает Desktop; для Web требуется `chatgptweb`. Повторяющиеся `--target` теряют предыдущее значение.

Для runnable retry переиспользовать safe-argument gate из `add_multi_retry.go`. Если source/root/аргумент нельзя безопасно представить для поддерживаемых shells, не печатать готовую команду; вывести точные значения и следующий шаг текстом. Существующий `quoteResumeArgument` использует POSIX quoting и не доказывает совместимость с Windows CMD. Универсальный shell renderer не входит в scope; второй quote helper не нужен.

### 4.3 Машинный контракт

Точное изменение контракта следующего PR: один стабильный код `error.code=FOREIGN_BINDING` для типизированного environment ownership refusal. Additive `error.conflicts[]` содержит `reason` (`unmanaged_binding` или `foreign_environment`), `alias`, `source_key`, `installation_id`, `client_ids` (canonical IDs); `root_alias` для подтверждённого dependency-конфликта и `owner_environment_id` — только когда известны. Несколько bindings одной installation группируются; conflicts и clients отсортированы детерминированно. Не выдумывать root/owner при неполных фактах. `retryable=false`: без изменения состояния повтор не поможет.

Использовать существующий error/result envelope. Для early ownership refusal в public `add`/`install <source>` и `deps sync` JSON печатается ровно один раз, с nonzero exit; stdout не содержит spinner/human текста. Update/management/skills consumers получают нейтральную typed причину через существующие DTO/formatter boundaries; их схемы, HTTP-коды и протоколы этим планом не расширяются.

Не расширять `ShouldWriteAgentFailure` на все ошибки. Точечно подключить новый typed refusal, сохранив старые unrelated native refusals с пустым stdout и source-selection envelopes. Late-race от kernel сохраняет существующий plan-changed/authority error и exit status: не переклассифицировать его по stale preview. Если apply/result уже напечатан, сохранить этот документ и не создавать второй error envelope. Не сериализовать raw native stderr, credentials или произвольный transport error. Fields/envelope закрепляются CLI contract тестом до внедрения.

## 5. Применение, сбои и повтор

| Момент | Состояние и следующий шаг |
| --- | --- |
| Отказ/отмена до публикации | Нет новых intent/binding effects; исправить выбранную причину |
| План изменился после review | Отказ существующего recheck под locks; новый preview и подтверждение |
| Manifest записан, lock не записан | Сохранённый intent остаётся; явно сообщить рассогласование и существующий `deps lock`/sync путь |
| Atomic write изменил bytes, затем fsync вернул ошибку; либо финальное чтение не удалось | Не заявлять «ничего не изменилось» или «всё сохранено»; сообщить известные effects/неопределённость и необходимость inspection |
| Intent опубликован, один binding не применён | Сохранить desired state; отдельно вывести applied/unchanged/failed/not_attempted |
| Recovery, cancel, plan/state corruption | Прекратить unsafe дальнейшие мутации; recovery не подменять обычным retry |
| Повтор после binding-local сбоя | Прежний locked pin; успешные bindings остаются unchanged |

`environmentapp.Result.Published` вычисляется как `before.Check(files) != nil`: файлы изменились **либо их прежнее состояние не удалось подтвердить**, например при read error. Сам boolean не доказывает durable write даже одного файла. Сообщение «intent сохранён» опирается на подтверждённые результаты публикации; при неизвестности — «intent мог измениться; проверьте environment». `environment.saved` в старом JSON не переопределять как новый durable-success флаг. На этом шаге уточняем существующий human/error output и связываем его с фактическим apply result; отдельную модель persistence/status и схему state не вводим.

Narrow retry сохраняет исходную add-операцию и явно выбранные failed public targets, source/pin и root. `deps sync --locked` сверяет весь saved environment; `deps` не поддерживает `--target`. Не выдавать общий sync за ограниченный повтор и не генерировать несуществующий `deps sync --target`. Если нужен общий sync — сообщить расширение области и дать сначала dry-run.

Final authority всегда остаётся в текущем kernel/mutation lock. Early observation не является разрешением и не удерживает lock во время пользовательского ввода. Порядок locks, receipts, rollback и recovery не меняются.

## 6. SOLID / Clean Architecture / DRY: конкретные границы

| Владелец | Разрешённое изменение |
| --- | --- |
| `environment` | Typed ownership refusal и один общий pure predicate, используемый coordinator и preview; нейтральная причина без CLI-команд |
| Existing installer inspection/observation | Передать уже известный owner/identity; только минимальное additive поле в существующем observation при доказанной необходимости |
| `agentpluginscli` | Порядок preview, human/JSON formatting, проверенные next actions, activity; без повторной политики владения |
| Existing client contracts/adapters | Потребляются как есть; все native/projection проверки остаются на нынешних границах |
| `environmentapp` / transaction / usecase | Существующее применение и revalidation; без переписывания алгоритмов |

Ожидаемые production-файлы: `environment/{coordinator,observe,model,foreign_binding}.go` (последний уже предложен #1086), `agentpluginscli/{saved_add,saved_add_acquisition,saved_add_targets,saved_add_foreign,agent_error,deps_apply,deps_render}.go`; включать только реально необходимые. Допустим небольшой новый private helper рядом с владельцем. Это не разрешение менять все перечисленные файлы.

Новые интерфейсы, state stores, dependency engines, flag modes и per-client ветвления в generic code не нужны. CLI не импортирует providers/pathpolicy; composition root остаётся владельцем concrete registry. Типизированный отказ должен сохраняться через `errors.As`/`errors.Is`.

## 7. Порядок реализации и оценки

Сначала сверить fresh main и открытые #1086/#1017. Если общий typed error уже merged, расширить его; если нет — согласовать один минимальный общий prerequisite либо дождаться его интеграции. Не cherry-pick весь Kiro/picker PR и не создавать вторую ошибку с тем же смыслом. Проверка #1017 нужна перед редактированием result/acquisition файлов, не для изменения его scope.

Оценки ниже относятся к будущей реализации, не к объёму этого docs PR. Сложность: 1 = просто, 10 = сложно; надёжность и уверенность: 10 = выше. LOC — ожидаемые изменённые строки Go без generated/vendor, не жёсткая квота.

| Шаг | Изменение и критерий завершения | Production LOC | Test LOC | Сложность / надёжность / уверенность |
| --- | --- | ---: | ---: | --- |
| 1 | Общая typed причина, различение unreceipted/foreign-owner, применимые next actions и точечный JSON contract | 100–180 | 100–180 | 4 / 9 / 8 |
| 2 | Advisory root check на существующем preview; одинаковый отказ explicit/interactive; graph guard сохранён | 80–160 | 100–180 | 5 / 8 / 7 |
| 3 | Уточнение partial/publish/retry вывода без смены семантики | 40–80 | 60–100 | 3 / 9 / 8 |
| 4 | Интеграция, ключевые E2E, обзор UX и документации | 0–30 | 40–90 | 4 / 8 / 8 |
| Итого | Один bounded implementation PR после общего prerequisite | 220–450 | 300–550 | 5 / 8 / 7 |

Если шаг требует другого lock order, persistence schema, source-migration policy, запуска native клиента перед picker или нового порта без реальной зависимости — он вышел за этот scope и выделяется отдельно. Не увеличивать ограничения lint и не прятать изменения в refactor общего установщика.

## 8. Приёмка будущей реализации

1. Unmanaged source на невыбранном target блокирует saved add этого пакета; manifest/lock/state/native bindings не изменены. Unrelated unmanaged source не мешает.
2. Root → unmanaged dependency: показана цепочка, нет no-save-обхода, граф не обрезан.
3. Другой environment receipt: отдельная причина; imperative remove/no-save не предложены как рабочая миграция.
4. Пустая retained installation: тот же commit/digest отдельно от смены revision/digest; текущие sticky/digest refusals сохраняются, успешная migration не обещана. PLUGIN_DATA сохраняются. Directory distribution — отдельный regression case будущей реализации, не покрытый выполненным legacy Git spike.
5. Один и несколько targets; explicit target, non-TTY, `--yes`, JSON, `--alias`, cancel: нет скрытой смены intent и лишних effects.
6. Native foreign и indeterminate различаются; preview не запускает native executable; возникший после preview conflict повторно отклоняется без перезаписи чужих bytes.
7. Частичный apply → повтор: successful target unchanged; failed target сходится; recovery/cancel прекращают дальнейшие небезопасные мутации; not_attempted не success.
8. Manifest-only/rename-then-sync-error/read-error: truthful publication output, nonzero exit при ошибке операции, существующий reconciliation путь; неизвестность не выдаётся за доказанный write.
9. Runnable next actions проходят реальный parser и проверку command-specific argv: UUID против SOURCE, public ChatGPT targets, comma-separated list, scope и root вне cwd. Для root с пробелами/кавычками — безопасный textual fallback с точным значением, без непереносимой команды. Неизвестные targets и неподдерживаемые сочетания не предлагаются.
10. Manual/prepared/auth pending не превращаются в бесконечный reinstall. Уже automatic Kiro не становится prepare-only из-за недоступной проверки; это regression assertion, без изменения Kiro policy/adapter.
11. Human/NO_COLOR при 40 колонках: причина доступна текстом; нет обещания uninstall по снятию галочки. JSON — один документ без activity.
12. Изменения между preview/confirmation/apply отклоняются существующими locks/recheck. Нет новых допущений по чужому SDK writer.

Переиспользовать уже существующие tests/helpers; не дублировать всю client matrix. Для Go-реализации обязательны обычные `make lint`/`make test-core`, релевантный package-manager E2E и затронутые cross-OS/native gates по CONTRIBUTING. Плановые fixture spikes ниже не заменяют эти проверки. Frozen VS Code tests/adapters не расширяются.

## 9. ToolHive: что взять и где граница

Сравнение зафиксировано на [ToolHive 2d33d6b](https://github.com/stacklok/toolhive/tree/2d33d6bea91715c3421ddac2e94a3fae015f174f).

Полезны: отдельные состояния AlreadyCurrent/Missing/Drifted/NeverManaged/RemovedFromLock; явные adopt/prune, read-only check, exact-pin restore, проверка файлов и adapter Health. [Sync](https://github.com/stacklok/toolhive/blob/2d33d6bea91715c3421ddac2e94a3fae015f174f/pkg/plugins/pluginsvc/sync.go), [install dispatch](https://github.com/stacklok/toolhive/blob/2d33d6bea91715c3421ddac2e94a3fae015f174f/pkg/plugins/pluginsvc/install_extraction.go).

Это аргумент за ясные состояния и явный переход владения, а не за копирование API/хранилища. ToolHive dependencies пока только сохраняет, не материализует; Codex marketplace требует отдельного клиентского install. Адаптеры вызывают принудительную замену target directory. Поэтому его механизмы не доказывают безопасность UAP graph/adoption/native ownership. [Границы](https://github.com/stacklok/toolhive/blob/2d33d6bea91715c3421ddac2e94a3fae015f174f/docs/arch/14-plugins-system.md), [Codex](https://github.com/stacklok/toolhive/blob/2d33d6bea91715c3421ddac2e94a3fae015f174f/pkg/plugins/adapters/codex.go).

Будущая adoption-задача должна отдельно доказать точную identity, совпадение всех клиентских pin/bytes, отсутствие drift/pending, сохранение data/approvals и включение всех adopted bindings в manifest. Remove+add не является её готовой реализацией. В этом PR операция не проектируется до уровня новых API и не реализуется.

## 10. Исследования и круги критики

- Круг 1: независимые проверки ownership/state, UX/automation и architecture/scope на fresh main. Приняты исправления по no-save, source identity, source-vs-target timing, public targets и bounded retry.
- Круг 2: повторное независимое ревью выбранной спецификации после executable spikes. Исправлены пределы Git-доказательства, семантика Published, UUID против SOURCE, неизвестные targets и переносимость retry-команд.
- Круг 3: финальный UX/architecture review закрыл замечания; проверены границы diff и воспроизводимость. Replay: 43 top-level tests, 95 PASS с учётом вложенных subtests, 0 FAIL, 0 SKIP. Уточнён assertion сохранения unrelated installation и guard полноты research cases.

Доказательства и ограничения: [evidence/2026-11-21-install-existing-state/README.md](evidence/2026-11-21-install-existing-state/README.md). Включены точный main, версия Go, команды, исходники disposable spikes, RUN/PASS/FAIL/SKIP и ожидаемые отказы. Исследование текущего поведения не означает, что будущая функциональность уже реализована или прошла native E2E.

## 11. Дополнительные круги реализации

1. Перед кодом три независимых разбора уточнили наблюдение чужого owner, приоритет ошибок и реальные E2E-границы. Принят private CLI wrapper только для раннего ownership refusal: общий typed error после apply не печатает второй JSON.
2. Ревью реализации устранило speculative Directory lookup по ProductID, маскирование malformed intent/recovery и alias/selector ошибок, неоднозначный root alias, unsafe retry после recovery и неоднозначные public target names. Directory identity берётся только из существующей resolution/acquisition; нового feed-запроса ради advisory нет.
3. Независимое ревью E2E усилило actual `run()` boundary и terminal assertion: проверяется точный typed отказ до дополнительных HTTP/security calls, а не произвольная prompt/EOF ошибка. Проверки user/project, чужого receipt, dependency chain и unrelated unmanaged source включены в существующий portable catalog.

Изменение retry gate является частью partial-result scope: `RECOVERY_REQUIRED` у одного клиента подавляет повтор для всех клиентов выбранного state namespace. Исходные outcomes и инструкции recovery сохраняются. Подтверждение полноты реализации требует результатов на окончательном production source; прежние 95 PASS относятся только к исследовательскому baseline.
