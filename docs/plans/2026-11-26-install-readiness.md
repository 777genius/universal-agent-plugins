# UAP: проверка ресурсов Skills и готовности MCP

Статус: **ОТЛОЖЕНО по решению владельца; сохранённый план v1**.
Реализация и E2E не начаты.

## Решение о приоритете — 26 ноября 2026

Владелец отложил эту реализацию: сейчас больше непосредственной пользы дают
надёжная установка плагинов, MCP и Skills, понятный консольный UX, объяснимые
ошибки и восстановление после отказов. Следующий анализ должен учитывать уже
слитые и открытые installer PR, чтобы не дублировать их изменения.

Этот документ сохраняет исследование, оценку и возможный будущий scope.
PR #1114 остаётся отдельным Draft с планом. Не начинать реализацию и не
переводить PR в Ready как поставку функции до явного решения вернуться к ней.
При возобновлении заново проверить актуальный main, открытые PR, спецификации
и оценку; изложенный ниже план не является текущим приоритетом.

Дата: 26 ноября 2026, Europe/Kiev.
Исследованный UAP main: `78af3127f8d3b01c042864253952583bad2e7746`.
Изученный ToolHive: `08e27b79d1ba9575a13ff9f833ad87ecf43cd834`.

## 1. Цель и формат поставки

Один Draft PR для двух связанных пользовательских результатов:

1. До установки увидеть, что Skill ссылается на отсутствующий или недоступный
   ресурс внутри пакета, с указанием файла и строки.
2. Явной командой проверить выбранный установленный MCP-сервер и получить
   точную причину неготовности либо ограниченное подтверждение работы протокола.

При возобновлении все изменения, исправления ревью, тесты и итоговое слияние
остаются в этом PR; коммиты публикуются сразу. Пока работа отложена, сохраняется
статус Draft. Опубликованные коммиты содержат только план и его уточнения.

Критерий ценности: пользователь может отличить корректно доставленные файлы,
обнаруженную проблему пакета и свежий результат MCP-проверки. Наличие файлов,
старое наблюдение и успешный обмен протоколом не подменяют друг друга.

### Когда вызываются проверки и как учитывается целый плагин

Единица установки и диагностики — плагин и его привязка к выбранному клиенту.
Skill и MCP server — компоненты этого плагина. Новые проверки расширяют
существующий package/delivery lifecycle, отдельный MCP installer не появляется.

| Этап | Проверка | Вызов |
| --- | --- | --- |
| Пакет получен, до изменения установки | Существующие manifest/digest/security checks; новые предупреждения о ресурсах каждого Skill в standard snapshot | Автоматически при фактической загрузке snapshot, до install review |
| План доставки выбранному клиенту | Поддержка компонентов, scope, activation/authentication и необходимые действия | Существующий preflight перед подтверждением install/update/repair |
| Применение установки | Доставка и регистрация выбранных компонентов, принадлежность файлов и существующая verification | В действующем lifecycle; runtime readiness не выводится из успешной записи |
| Установленный плагин | Живой MCP protocol check одного выбранного server этой installation/client binding | Только явный `probe`, после установки и необходимых setup действий |

Новая статическая проверка охватывает фактически загружаемые managed immutable
standard snapshots в поддержанных CLI путях и нормализованные standalone
Skills. Результат относится к пакету и передаётся в существующие client plans;
новый глобальный cache или повторный analyzer на каждом этапе не нужен.
Обновление проверяет новый snapshot; восстановление — загруженный записанный
snapshot. Не обещать новую проверку для SDK или repair/no-op пути, который не
проходит эту точку загрузки. Конкретное покрытие подтверждается E2E.

Плагин без MCP не получает ошибку отсутствующего сервера. При нескольких MCP
проверка одного не означает успешную проверку остальных. Сам `probe` выбирает
компонент по installation/client/server, а не устанавливает отдельный MCP.
App bindings, extensions и остальные уже поддержанные элементы продолжают
проходить действующие проверки формата и клиентских адаптеров. Выполнение
всего плагина или инструкций Skill моделью эта проверка не подтверждает.
Результаты package validation, delivery, Skill references и MCP probe остаются
различимыми: единого нового флага «весь плагин работает» не добавлять.

## 2. Что изменилось после глубокого исследования

Первоначальная идея общего мастера обязательных настроек оказалась слишком
широкой для предполагаемого небольшого переноса `ToolHive/pkg/runner/env.go`.

- Agent Plugins 1.0.0 не задаёт универсальных `required/default/secret` inputs.
  MCP `env` — словарь строк; из имени `API_KEY`, пустого значения или `${FOO}`
  нельзя выводить обязательность, секретность и правила подстановки.
- UAP уже показывает Authentication, Verification, warnings и дальнейшие
  действия в существующем install review. Второй preflight service не нужен.
- `${PLUGIN_ROOT}` и `${PLUGIN_DATA}` уже обрабатываются действующим shared
  stdio-контрактом; неизвестные выражения намеренно остаются литеральными.
- [PR #832](https://github.com/777genius/universal-agent-plugins/pull/832)
  фиксирует security NO-GO для Codex env-name forwarding. Этот план его не
  разблокирует, не повторяет исследование и не добавляет обходные способы.
- Plain `doctor` намеренно не запускает MCP. У `info --target` уже есть
  отдельное явное наблюдение native identity; его не следует дублировать.
- Текущая MCP revision `2026-07-28` использует per-request metadata и
  `server/discover`; обязательный `initialize` относится к прежней revision
  `2025-11-25`. Старый ToolHive helper нельзя переносить как универсальный
  клиент актуального протокола.

Поэтому общий wizard не входит в PR. Существующие setup/authentication
состояния используются в допуске и объяснении результата. Отсутствующее
описание требований остаётся unknown; оно не становится missing или ready.
Само отсутствие metadata не запрещает probe точного уже выраженного recipe:
неизвестными остаются требования provider, а наблюдение протокола оценивается
отдельно. Новые native input schemas/readers в этот PR не добавляются.

## 3. Оценка объёма

Оценка добавленных и существенно изменённых физических строк; без vendor,
сгенерированных файлов, lock-файлов зависимостей и неизменённых fixtures.
Точный diff уточняется после соединения существующих selection/runner seams.

| Работа | Production LOC | Tests/E2E LOC |
| --- | ---: | ---: |
| Skills: bounded reference analyzer и интеграция | 450–650 | 350–550 |
| Выбор установленного MCP, ownership и подготовка запроса | 300–450 | 300–450 |
| Общий ограниченный обмен и Streamable HTTP | 450–650 | 450–650 |
| Stdio и положительное подтверждение завершения процессов | 400–650 | 550–800 |
| CLI, отчёт, facade/composition | 250–350 | 200–300 |
| Совместимость MCP 2026-07-28 / 2025-11-25 | 250–400 | 400–600 |
| Повторное использование существующего preflight | 0 | 0 |
| **Всего** | **2 100–3 150** | **2 250–3 350** |

Документация, этот подробный план и attribution: ещё **500–750 строк**.
Общий диапазон: **4 850–7 250 строк**, рабочий ориентир — около **6 000**.
Это существенно
больше простого переноса проверки ссылок: сюда включена законченная проверка
двух транспортов, двух поколений MCP и реальная cross-platform cleanup proof.
Базовая protocol строка считает один путь обмена; compatibility — вторую era,
классификацию переходов и discovery timeout. Они не считаются дважды.
Existing preflight не получает wrapper/tests ради объёма: новые admission
integration cases уже включены в строки MCP selection и CLI.

- Полезность выбранного scope: **9/10**, инженерная оценка.
- Сложность: **7/10**; главный риск — полномочия запуска и lifetime процесса.
- Уверенность в оценке: **7/10**, до реализации selection seam.
- Целевая надёжность после всех gates: **8/10**; это цель, не текущий результат.

Ключевые пользовательские результаты проверяются через production CLI,
настоящий дочерний процесс и настоящий loopback HTTP. Табличные tests нужны
для ветвистой обработки путей, протокола и редких отказов cleanup. Уже
проверенные механизмы не покрываются повторно ради количества тестов.

## 4. Границы

### Входит

- Standard Agent Plugins и нормализованные standalone Skills в действующих
  CLI acquisition/authoring путях.
- Markdown links, images и reference definitions, связанные Markdown-файлы.
- Предупреждения на существующем этапе просмотра установки, без нового gate
  нормативной валидности или отдельного разрешения security risk.
- Новая явная команда `probe` для одной installation, одной client binding
  и одного MCP server.
- Stdio и Streamable HTTP; JSON и SSE response bodies последнего.
- Только MCP revisions `2026-07-28` и `2025-11-25` с ограниченным fallback.
- Точное завершение ресурсов и контролируемый отчёт без секретных данных.
- E2E на итоговом коммите и независимые раунды review с исправлениями.

### Не входит

- Новые клиенты, LM Studio, изменения замороженного VS Code.
- Собственные runtime, daemon, vault/keychain, OAuth flow или setup wizard.
- Передача ambient credentials, новый env resolver, trust/approval writers.
- Legacy HTTP+SSE transport, остальные версии MCP, WebSocket.
- UAP-вызовы `tools/call`, model turns и непосредственно инициируемые UAP
  provider operations; реальные токены и платные подписки в проверках.
- Выполнение или shell-анализ скриптов Skill, проверка внешних ссылок сетью.
- Второй resolver, state engine, signer/feed, глобальная переделка адаптеров.
- Публикация новой версии CLI/npm/Homebrew. Merge и release — разные этапы.

## 5. Skills: контракт и точки интеграции

### Источник истины

Анализатор получает уже захваченные записи и байты. Не читает произвольное
исходное дерево повторно, не выполняет код, не делает сетевые запросы.

`packageview.Input.Inventory` задаёт Path/Kind/State/Target/Captured,
`Lease.Content(path)` — захваченные bytes. Стандартная conformance остаётся
отдельной от новых предупреждений.

Standalone acquisition уже создаёт synthetic `plugin.json` и sealed snapshot
в `adapters/skillsource/normalize.go`. Исходное `Source.Skill` / `Source.WebSkill`
сохраняется. Новый raw-Skill capture engine не требуется.

Containment root:

- Bundled Skill: корень plugin, поэтому ссылки на соседние общие ресурсы
  допустимы, если ресурс действительно находится внутри пакета.
- Standalone Skill: конкретное `skills/<name>` нормализованного snapshot.
  Synthetic wrapper не расширяет разрешённую область ресурсов.
- Рекурсивное чтение linked Markdown ограничено исходным Skill subtree.

### Минимальный алгоритм

1. Разобрать Markdown готовым AST parser. Одна закреплённая dependency
   Goldmark в недоменном пакете; не импортировать ToolHive module и не писать
   собственный Markdown parser.
2. Найти relative links/images/reference definitions и исходные строки,
   учитывая frontmatter и CRLF. Пропустить URL, anchors, absolute/home paths,
   variables/templates/globs по явно документированным правилам.
3. После percent-decoding повторно проверить абсолютность и containment.
   Повторно применить одинаковую классификацию: decoded absolute/drive/home
   paths пропускаются как внешние ссылки, никогда не передаются в lookup;
   decoded relative escape получает controlled warning.
   Проверять уже захваченные компоненты пути; не схлопывать
   `missing/../existing` до проверки промежуточного компонента.
4. Symlink, неполный capture или неизвестное промежуточное состояние дают
   unavailable. Новый symlink resolver не добавляется.
5. Missing утверждается только при достаточном complete inventory.
6. Обходить связанные Markdown с dedup и лимитами. Стартовые лимиты:
   50 Markdown/Skill, 1 MiB/document, 100 findings/Skill; общий package budget:
   8 MiB анализируемого Markdown и 200 ordinary findings плюс конечные
   once-per-reason incomplete markers. Linked documents входят в общий бюджет;
   после его исчерпания следующие Skills не начинают независимый обход.
7. Лимит или неполное чтение отражается как incomplete/unavailable. Это не
   результат «проблем нет» и не новая причина отказа в валидной установке.

### Интеграция без нового authority

| Точка | Изменение |
| --- | --- |
| `agentplugins/skillrefs/` | Чистый bounded analyzer и private typed findings |
| `adapters/packageview/skill_references.go` | Проекция готового capture в analyzer input |
| `agentpluginscli/source_snapshot.go` | Вызов после standard load и проверки immutable digest, до возврата loaded package |
| `authoring/project/` | Использование уже открытого lease; отдельное private поле diagnostics |
| `authoring/report/` | Отдельные warning findings и `Check{ID: skill_references}` в существующем `Checks` |

CLI snapshot path не располагает готовым lease: открыть существующий
`packageview.Reader` на sealed normalized standard snapshot с явно переданным
trusted scratch parent, вызвать `Capture`, закрыть lease через defer. Не
вызывать authoring service и не создавать generic capture API. Эти примерно
65–95 строк уже учтены в Skills integration. Unavailable capture даёт одно
контролируемое warning; authoritative integrity failures не поглощаются.

`domain.Diagnostic.Path` остаётся файловым путём. Номер строки передаётся в
контролируемом сообщении; author `Finding.Location` использует существующий
формат `relative/file.md:17`. Raw link destinations и OS error strings не
попадают в отчёт. Public DTO и state/lock schemas не меняются. Reference
diagnostics не меняют `Coverage`, `Conformance`, `HostSafety`, `Readiness`,
`Successful()` или exit status. Incomplete даёт `not_evaluated` только новому
reference check.

Ошибка факультативного дополнительного анализа не становится installation
refusal; отмена остаётся отменой. Ошибка существующей проверки целостности
snapshot по-прежнему запрещает продолжение. SecurityAssessment, его подписи,
cache identity и разрешения на blocking findings сохраняют прежний смысл.

Не заявлять автоматическое покрытие любых SDK `PackageLoader.Load` callers.
Installed inspection переиспользует анализ только там, где уже восстанавливает
managed immutable standard package. Native client trees заново не сканируются.

## 6. MCP: пользовательский и архитектурный контракт

Предлагаемый новый интерфейс, пока отсутствующий в CLI:

```sh
agentplugins probe <installation-id> --target <client> --server <server>
agentplugins probe <installation-id> --target <client> --server <server> --format json
agentplugins probe <installation-id> --scope project --project-root /path/to/project --target <client> --server <server>
```

Обычные `doctor`, `info`, `--dry-run` и их старые JSON/exit contracts не
получают скрытых запусков, запросов на ввод или сетевых probes.
Help явно сообщает: probe запускает выбранный сервер либо контактирует с его
endpoint; сервер может иметь собственные эффекты. Явная команда выражает
намерение, отдельный consent framework/flag не добавляется.

### Выбор и допуск

- Выбирать ровно одну существующую managed installation/binding/server.
  Использовать owned content, receipts, immutable artifact identity и
  существующие adapter selection readers. Не принимать произвольный URL,
  каталог HEAD или одноимённую чужую native запись вместо выбранного сервера.
  Несколько подходящих bindings дают ambiguous_selection до эффекта;
  scope/project context задаётся существующими параметрами.
- Для согласованного чтения есть конкретный CLI seam:
  `read_snapshot.go:acquireDoctorLock` использует optional `AcquireReadOnly`
  capability существующего mutation lock. `installer.Inspect` сам этого не
  гарантирует. При проведении через facade зависимость передаётся явно;
  отсутствие capability не маскируется обычным незаблокированным read.
- Recipe захватывается как detached immutable data, с content/digest и
  identity checks. До запуска нужно подтвердить безопасный handoff выбранного
  bundled executable/CWD существующими snapshot/path/kernel механизмами:
  повторная проверка пути сама по себе не закрывает check-to-exec race.
  Если конкретный recipe нельзя безопасно передать существующими средствами,
  вернуть unsupported/not_probed до эффекта. Второй snapshot/locking engine
  не строить; selection feasibility проверяется первым implementation шагом.
  Готовый переносимый descriptor-based executable handoff в текущем duplex
  runner не найден. Проверяется cooperating UAP lifecycle race в действующей
  модели угроз; защита от произвольного hostile filesystem writer не заявлена.
- Не держать глобальную mutation lock в течение долгого protocol exchange.
  Post-observation identity check обнаруживает concurrent lifecycle change
  и даёт inconclusive; это проверка свежести, не замена launch authority.
- Переиспользовать `domain.MCPServer`, loader/conformance и shared stdio path
  semantics. Не создавать второй интерпретатор MCP-конфига.
- Известные policy/ownership/trust refusals и неподдерживаемые способы
  получения credentials останавливают probe до эффекта. Неизвестные требования
  нельзя выдумывать по имени поля. Recognized unresolved native inputs дают
  configuration_required; стандартные неизвестные токены остаются literal.
- Отсутствие declared input metadata само по себе не блокирует exact recipe.
  Пустой literal `env` или стандартный `${FOO}` не становятся required input
  по догадке. Не добавлять новые readers native inputs в этом PR.
- Package-authored literal env/HTTP headers передаются по существующим
  projection правилам, без вывода значений. Значения из native config,
  secret store и ambient env по ссылкам не импортируются. Если существующий
  adapter распознаёт reference, требующий такого разрешения, вернуть
  configuration_required/not_probed до эффекта, не удалять поле молча.
- Fresh read-only admission выбранного client adapter должен давать достаточную
  authority для данной lane. Старый PolicyAllowed receipt не является свежим
  разрешением. Direct probe не воспроизводит native sandbox/MCP approvals;
  shared stdio path semantics не предоставляют trust/secret полномочий.
  Недостаточная authority означает unsupported/not_probed, без writers.
- Probe не вызывает activation helpers. В частности,
  `finishClaudeMCPActivation` сначала выдаёт MCP approval и поэтому не является
  допустимым read-only helper для этой команды.

### Слои

Private подготовка выбранного запроса проходит через installer facade и
существующие ownership/selection ports. Сетевой/процессный обмен находится в
`adapters/mcpprobe`; CLI только разбирает параметры и отображает результат.

Предварительные файлы: `domain/mcp_probe.go` (инертные данные),
`ports/mcp_probe.go` (одна реальная граница I/O), `usecase/mcp_probe.go`,
`installer/mcp_probe.go`, небольшие файлы `adapters/mcpprobe/`,
`agentpluginscli/probe_commands.go` и `probe_output.go`. Composition остаётся
в нынешних executable roots. Не создавать сервис регистрации probes, новый
state store или client-ID ветвления в generic code.

### Два поколения MCP

Поддерживаемый набор фиксирован: modern `2026-07-28`, legacy `2025-11-25`.

| Транспорт | Modern | Ограниченный переход к legacy |
| --- | --- | --- |
| Stdio | `server/discover`, затем `tools/list`; per-request `_meta` | Валидная legacy RPC error или discovery silence в отдельном подбюджете; тот же supervised process, затем `initialize`/`notifications/initialized` |
| Streamable HTTP | POST discovery/list, matching metadata и headers | Только initial HTTP 400 с bounded non-modern body; тот же exact endpoint |

Recognized modern errors не вызывают слепой legacy fallback. Unsupported
version после распознавания modern допускает только modern `2026-07-28`;
если совместимой реализованной modern версии нет, результат unsupported,
даже если advertised list содержит legacy `2025-11-25`. Только отдельное
разрешённое распознавание legacy ведёт к initialize `2025-11-25`; другую
negotiated revision отклонять. Recognized modern codes: `-32020`, `-32021`,
`-32022`; они не разрешают расширять capabilities или менять credentials.
401/403, policy refusal, transport error, malformed/noisy stream, общий timeout,
cancel и unconfirmed cleanup не являются разрешением сменить протокол.

Modern требует `params._meta` на каждом запросе и matching `MCP-Protocol-Version` /
`Mcp-Method` HTTP headers. Modern не создаёт protocol session, не выполняет
`initialize`, GET или DELETE. Legacy session ID и bounded DELETE относятся
только к legacy path. `input_required` остаётся configuration_required;
никаких inputResponses, OAuth или расширения capabilities по требованию сервера.
Modern self-reported serverInfo необязателен и не публикуется.

Для stdio discovery timeout не отменяет всего runner: один persistent bounded
reader продолжает работать на том же процессе. Unique request IDs позволяют
ограниченно обработать поздний ответ на завершённый discovery; остальные
несоответствующие сообщения не игнорируются произвольно.

### Stdio

- Запускать exact installed recipe непосредственно через argv и существующий
  kernel-contained duplex runner. PATH разрешается до CWD по нынешним правилам;
  relative executable results не становятся разрешёнными.
- Использовать явную разрешённую среду и описанный выше fresh admission. Не брать
  ambient credentials и не использовать `managedstdio` process replacement
  как probe runner. Общий `${ENV}` resolver отсутствует.
- UAP не добавляет установку runtime или retry installer. Авторская команда
  вроде `npx` сама может загружать/выполнять код; probe не объявляется sandbox
  для враждебного пакета и не обещает отсутствие любых эффектов чужого процесса.
- Stdout — строгий MCP stream; banner/log вместо JSON является ошибкой.
  Stderr дренируется с лимитом и не публикуется. Никаких raw error chains.
- Success, protocol error, timeout и cancel требуют положительной cleanup
  evidence для leader и descendants. Нельзя выдавать успех после потери
  контроля над дочерними процессами.
- `RunDuplexWithPlannedShutdown` сейчас не устанавливает ту же closure proof,
  что `RunDuplexToCompletion`. Добавить небольшой proof-enabled helper на
  существующем supervisor, сохранив `errors.Is/As`. Legacy-exempt
  `osrunner.go` должен уменьшиться либо не вырасти; новые suppression запрещены.

### HTTP

- Разрешены exact selected public HTTPS и literal-loopback HTTP/HTTPS.
  IP parse/unmap выполняется через `netip`; hostname `localhost`, zone IDs и
  неоднозначные адреса в этом scope не поддерживаются. Private LAN/alternate
  endpoints в этот scope не входят.
- Public DNS answers проверяются целиком, dial привязан к проверенным IP,
  TLS сохраняет исходную authority. Не ослаблять существующую acquisition
  network policy ради probe; разные назначения не смешиваются.
- Без redirects, ambient proxy/cookies/credentials и endpoint events,
  меняющих назначение. Не читать secret store и не запускать sign-in.
- Уже выраженные допустимые literal headers сохраняются. URL userinfo,
  CR/LF, произвольный `Host`, подмена protocol/transport authority headers
  отклоняются до запроса. Обязательные transport headers строит адаптер.
- Обрабатывать JSON и request-scoped SSE ответ до correlated final message,
  не ждать EOF бесконечного stream.

### Ограничения и отчёт

Один общий budget protocol exchange; начальный ориентир 15 секунд. Cancel/
deadline прекращает protocol work и новые effects, затем существующий supervisor
выполняет bounded shutdown/reap с отдельным cleanup grace, сохраняющимся после
cancel caller. Wall time может включать этот конечный grace. Нельзя вернуть
PASS до завершения cleanup или ждать бесконечно. Bounded protocol limits:
1 MiB/response, 2 MiB cumulative inventory, 20 pages, 2 000 tools, конечное
число notifications и line/event bytes. Конкретные constants проверяются E2E.

Неверный ID/версия, duplicate JSON keys, repeated cursors, duplicate tool
identities, неизвестные result types и превышенные лимиты имеют явные outcomes.
Отсутствие tools capability отличается от корректного пустого списка.
Для этой команды отсутствие capability означает discovery pass,
tools_list=not_advertised, inventory_complete=false,
outcome=unsupported_for_tools_probe и ненулевой exit. Корректный пустой tools
list считается complete. Truncated inventory не превращается в completed.

Один короткий phase-based результат в существующей JSON envelope:

- `evidence_scope: protocol_only`, `observer: uap_direct_mcp_client`;
- installation/binding/server IDs, transport, protocol era/version, время;
- configuration, connection, protocol_discovery, tools_list, cleanup;
- tool count, inventory completeness, конечный outcome и controlled code.

Cleanup status остаётся отдельным: корректный tools list не скрывает отказ
завершения. Endpoint, query, headers, session ID, serverInfo, tool names/
descriptions/schemas, stdout/stderr и чужие сообщения не выводятся.

Probe не меняет записанные Activation/Authentication/Verification. Protocol
PASS не доказывает загрузку компонента агентом, provider auth или tools/call.
Exit 0 означает успешный поддерживаемый probe с complete requested observation
и cleanup; incomplete/refused/failed/timeout не получают успешный exit.

## 7. E2E и review gates

Тестовые данные синтетические. Private disposable profiles, локальные fixtures,
никаких настоящих tokens/подписок и model turns.

| Проверяемый результат | Обязательное доказательство |
| --- | --- |
| Skill references | 2–3 production CLI fixtures: file/line warning, bundled/standalone roots, сохранение admission; analyzer/capture contracts: все AST формы, CRLF, encoded paths, unavailable, циклы/caps |
| Сохранение прежнего admission | Warnings не ломают валидный install; существующий blocking security/ownership refusal всё ещё блокирует; `Path` остаётся путём |
| Exact MCP selection | Чужой/невыбранный одноимённый server не запускается и не получает HTTP; drift/rebind/known policy refusal останавливают эффект |
| Положительная production lane | Как минимум один полезный installed recipe для каждого транспорта проходит production CLI selection → admission → probe → cleanup; одни unsupported ответы и прямые adapter fixtures не закрывают scope |
| Два MCP поколения | Modern/legacy/dual-era fixtures обоих транспортов; metadata на каждой странице; discovery silence → legacy на том же процессе, поздний retired-ID ответ, total timeout после sub-timeout |
| Protocol bounds | Wrong IDs, malformed/duplicate JSON, stdout banner, input_required, no tools/empty tools, pagination/repeated cursor, byte/page/tool limits |
| Настоящий HTTP | Loopback listener, JSON и удерживаемый SSE; 400 empty/nonmodern отдельно; recognized-modern 400 и 401/403 дают zero initialize; redirects, modern no DELETE, legacy cleanup |
| Настоящие процессы | Production binary + fixture child/grandchild/pipe holder; timeout/cancel/early exit/stderr flood; PID evidence, все descendants завершены |
| Cleanup failure | Injected containment/close/reap error не теряется за protocol success; unavailable containment не имеет небезопасного fallback |
| Секреты и authority | Receive-side child/HTTP fixture не получает запрещённые ambient canaries. Разрешённые authored literal env/headers доходят точно и не утекают при echo в metadata/serverInfo/ошибках/output/artifacts |
| Сохранение состояния | Installer-owned state/receipts/config/trust/foreign files до и после; controlled fixture effects отдельно; concurrent change даёт inconclusive и не позволяет mutable executable подменить launch |
| Lifecycle regression | Установка, повтор, update, repair, remove и locked restore для затронутых fixture paths через existing harness |

Обязательны `make lint`, `make test-core` и существующие affected package-manager,
rollback, native/cross-platform gates согласно CONTRIBUTING. Для stdio/closure
нужны реальные Linux, Windows и macOS, а не mock-подтверждение завершения.
Фикстурный MCP E2E и native-agent E2E в отчёте обозначаются отдельно.

Переиспользовать текущие workflow/harness, добавлять только необходимые cases.
Не менять frozen VS Code, лимиты lint или gates ради зелёного результата.
Queued/cancelled/skipped не равны PASS; обычное ожидание runner не блокировка.

Ревью реализации: как минимум два независимых прохода — сначала architecture /
authority / secret / process lifetime, затем negative E2E / compatibility /
user-facing claims. Каждое критичное замечание исправляется в этом же PR.
После фиксов повторяются затронутые сценарии, полный обязательный gate — на
итоговом HEAD. Self-review не подменяет требуемое независимое GitHub approval.

## 8. Последовательность в одном PR при возобновлении

1. Зафиксировать план, LOC и два раунда критики. Открыть Draft PR.
2. Подтвердить конкретный installed selection seam и versioned MCP fixtures.
   При отсутствии authority вернуть unsupported/not_probed; не строить обход.
   До основной реализации показать положительную production lane для обоих
   транспортов. Если это невозможно в данных границах, скорректировать оценку
   и план явно; нельзя молча назвать ограниченный отказ поддержкой stdio.
3. Реализовать Skills analyzer с минимальной интеграцией и fixture E2E.
4. Реализовать общий bounded protocol, оба транспорта и closure proof.
5. Подключить CLI/result, убедиться, что existing review и doctor не дублируются
   и не меняют side effects. Реализация остаётся в границах этого документа.
6. Пройти review/fixes, affected E2E и required CI на финальном HEAD.
7. Перевести PR в Ready после фактических результатов и выполнить штатное
   слияние с соблюдением branch rules. Проверить реальный merged main.

Изучены пересекающиеся PR #832, #1023, #1071, #1016, #1087 и merged #1104.
Перед реализацией повторно проверить main и их текущие diff. Не забирать
чужую ветку и не возвращать старые client-surface/trust решения.

## 9. Журнал ревью плана

### Раунд 1 — scope и повторное использование

- Удалён придуманный универсальный required-input wizard: стандарт не даёт
  такого контракта, existing review уже покрывает setup evidence.
- Убраны shell/code-span heuristics и новый symlink resolver из Skills scope.
- Сохранены Path/JSON semantics, separate normative/security authorities.
- Подтверждён normalized standalone seam; не нужен второй capture engine.
- HTTP и stdio включены в один PR; выделенный HTTP-only PR не создаётся.

### Раунд 2 — протокол и границы доказательств

- По актуальной первичной MCP specification исправлено предположение об
  обязательном initialize; добавлена конкретная modern/legacy матрица.
- Добавлены discovery sub-deadline, late response и error-specific fallback.
- Зафиксированы positive closure proof, repeated input identity check,
  literal-loopback policy, отдельные cleanup/incomplete outcomes.
- Удалены обещания native-agent/provider readiness по одному protocol probe.
- LOC увеличен с учётом реальных transport/lifetime/versioning работ.

### Независимое line-by-line review v1

Три отдельные проверки: Skills/capture/report; MCP protocol/process lifetime;
setup/secret/trust authority. Исправлены замечания о Coverage/Readiness,
дополнительном bounded capture, package cap, полях env/headers, ложной
check-to-exec гарантии, caller-cancel/cleanup, modern fallback, receive-side
secrecy oracle и scope неизменяемого setup. Все три автора ревью повторно
проверили исправления: оставшихся замечаний в рассмотренных областях нет.
Это review плана, не approval production code.

Финальное line-by-line ревью документа: **завершено с исправлениями**.
Отдельное GitHub review реализации и фактические тесты ещё не выполнены.

## 10. Источники и условия переноса

- [UAP architecture](../ARCHITECTURE.md), [contributing](../../CONTRIBUTING.md),
  [owner rules](../../AGENTS.md).
- [ToolHive refcheck.go](https://github.com/stacklok/toolhive/blob/08e27b79d1ba9575a13ff9f833ad87ecf43cd834/pkg/skills/refcheck.go)
  и [tests](https://github.com/stacklok/toolhive/blob/08e27b79d1ba9575a13ff9f833ad87ecf43cd834/pkg/skills/refcheck_test.go):
  адаптировать только необходимые алгоритмы/кейсы, без ambient filesystem I/O.
- [ToolHive env.go](https://github.com/stacklok/toolhive/blob/08e27b79d1ba9575a13ff9f833ad87ecf43cd834/pkg/runner/env.go):
  источник идеи; runtime secret handling в UAP не переносится.
- [ToolHive MCP client](https://github.com/stacklok/toolhive/blob/08e27b79d1ba9575a13ff9f833ad87ecf43cd834/pkg/mcp/client/client.go):
  transport/lifetime lessons; не готовый dual-era/stdin client.
- [Apache-2.0](https://github.com/stacklok/toolhive/blob/08e27b79d1ba9575a13ff9f833ad87ecf43cd834/LICENSE):
  сохранить нужные copyright/license/NOTICE и отмечать адаптации; проверить
  лицензии и supply-chain новой parser dependency.
- [MCP versioning 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)
  и [transports](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports),
  [stdio binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio),
  [Streamable HTTP binding](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http),
  [legacy lifecycle 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle).
  Реализация сверяется с конкретными versioned binding schemas, не с latest.

## 11. Acceptance checklist

- [x] Исследован актуальный исходный код и пересекающиеся работы.
- [x] Оценка разделяет production, E2E/tests и документацию.
- [x] Два предметных раунда критики учтены в scope.
- [x] Line-by-line review готового плана завершён с повторной проверкой фиксов.
- [ ] Skills diagnostics реализованы и проверены.
- [ ] MCP dual-era stdio/HTTP probe реализован и проверен.
- [ ] Source/selection/secret/trust/cleanup отрицательные сценарии пройдены.
- [ ] Обязательные lint/core/affected native/cross-platform E2E прошли.
- [ ] Независимые review замечания исправлены на финальном HEAD.
- [ ] PR штатно слит; фактический merged main проверен.
