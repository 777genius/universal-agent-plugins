# Part 12 — Вынос контрактного слоя (`domain` + `ports` + `clients`) в отдельный Go-модуль

> Раздел для вставки в `docs/plans/installer-core-clean-architecture-plan.md` после Part 11.
> Черновик написан на срезе кода после Part 4 (`82de37452`); настоящая редакция — после адверсариальной
> критики, все числа переверены на `d5250a3d9` (ветка `refactor/installer-core-clean-architecture`).

> **Статус: планируется ПОСЛЕ Part 11.** Part 5-9 добавят реальный код в `clients/<id>`, что может изменить
> набор зависимостей контрактных пакетов. **Условие старта:** перед реализацией — переверка §12.1 на
> актуальном HEAD базовой ветки (`go list -deps` по трём пакетам **плюс** их тестовые импорты, команды ниже).
> Если список разошёлся — сначала обновить план, не реализовывать вслепую. Дополнительно: guard test из
> §12.1.G добавляется **сейчас**, до старта Part 5, чтобы расхождение ловилось на вносящем PR, а не через
> пять частей.

---

## 12.-1. Изменения по критике

Черновик прошёл адверсариальную критику. Ниже — что именно исправлено и почему. Строки «Находка 4» и
«Три ребра» появились уже при верификации самой критики: это новые факты, которых не было ни в черновике, ни
в критике.

| # | Проблема в черновике | Что изменено | Почему |
|---|---|---|---|
| 1 | Объём внутренней квалификации типов в реализации `nativeconfig` после DTO-split занижен ~в 2.5 раза | §12.2.A1: реальный churn — **63 строки в non-test файлах реализации + 130 в её тестах = 193**; принято лекарство — блок одноимённых алиасов в пакете реализации (~22 строки), после которого внутренний churn = **0** | Занижение объёма 12a было главной причиной снижения оценки плана; алиас-блок делает правку дешевле черновиковой оценки, а не дороже |
| 2 | Пропущено поле `providers.Activator.NativeConfig *nativeconfig.Kernel` | §12.2.A1 «Второе место, где тип становится интерфейсом»: поле + `nativeConfigKernel()` + **5 мест конструирования** в тестах (`providers/opencode_native_test.go:313,347,366,381`, `usecase/manual_remote_lifecycle_test.go:241`) | `*nativeconfig.Kernel` после смены типа — указатель на интерфейс; объявление скомпилируется, а `&committedKernel` конкретного типа — нет. Пропуск сломал бы сборку тестов в 12a |
| 3 | Фактическая ошибка: «из правила для `clients` удаляется разрешение на `adapters/nativeconfig`» | §12.5 и критерий приёмки 7 переписаны: такого разрешения в `.golangci.yml` **нет**; `clients-no-upward` и `clients-no-concrete-clients` — deny-списки. A-2 существует только прозой в `clients/clients.go:13-16` и `docs/ARCHITECTURE.md:106-114` | Нельзя удалить то, чего нет; вместо этого 12a **добавляет** deny-правило, которого сегодня не хватает |
| 4 | Критерий «импорт `contracttest` из `clients/all/registry_test.go` доказывает пригодность для out-of-tree» | Заменён на машинную проверку `GOWORK=off` (§12.8, критерий 10) | Импорт внутри одного коммита доказывает только работу `go.work` + `replace`, а не отвязываемость модуля |
| 5 | CI-поверхности после выноса молча перестают видеть контракт | §12.5 «Пять CI-поверхностей» — конкретная правка для каждой: `Makefile` (`test-required`, `test-core`, `LINT_MODULES`, `vet`), `coverage.yml`, `core-fast.yml` (paths-фильтр), `agentplugins-release.yml`, `codeql.yml`/`govulncheck.yml` | Эмпирически подтверждено: `go list ./...` в корне видит **только 6 пакетов root-модуля**, ни одного из workspace-модулей. Без явной строки 1171 LOC тестов контракта выпадают из Required молча |
| 6 | Числовые неточности | 296 → **302** ссылки `nativeconfig.*`; baseline-записи на строках **674 и 677** (не 443/446); «16 файлов компилируются без правок» → **15** (шестнадцатый — сам `ports/runner.go`, который правится); добавлены **14 ссылок `ports.Command*` в 4 файлах `cli/`** | Собственная разбивка черновика суммировалась в 302; ссылки в `cli/` — причина, по которой `cli/go.mod` получает `require`/`replace` |
| 7 | `go.mod` нового модуля копировал `go 1.25.0` у соседей | §12.3 «Версия Go»: объявлять **минимальную** версию, под которой контракт реально компилируется; процедура эмпирического бисекта + риск смены семантики loop-переменной на границе 1.21/1.22 | Завышенный `go`-директив отсекает потребителей — прямо против цели части |
| 8 | Правило «мерджится в тот же день» было только у 12b | §12.3: правило распространено на **12a** | 12a трогает `providers/activator.go` — самый горячий файл проекта (23 правки в `main` за 60 дней) и все четыре `*_native.go` |
| 9 | Требование к Part 7c дублировалось в Part 12 | §12.1 Находка 3: текст удалён, дана ссылка на **§8.7 основного плана**, куда требование уже перенесено | Один источник правды; требование живёт там, где его будут исполнять |
| 10 | Не было защиты §12.1 от дрейфа в Part 5-9 | Новый раздел **§12.1.G** — guard test, добавляемый СЕЙЧАС, с полной спецификацией для реализатора | Новый импорт в контрактном слое ловится на вносящем PR, а не через 5-7 частей |
| 11 | Прогноз модульности 8-8.5 не учитывал `clients/shared` | §12.4: **7.5-8** с явной оговоркой про `clients/shared` (остаётся в монолитном модуле и тянет `hujson`, 11 пакетов `x/text`, `jsonschema/v6`, `yaml.v3`, `conformance`, `pathcontract`, legacy `domain`/`ports`); среднее **≈7.35-7.65** против конкурента 7.4 | Обещание «внешнему автору не нужен движок» верно для узкого контрактного слоя, но НЕ для практического сценария написания реального адаптера. Это должно быть видно, а не спрятано |
| 12 | Самооценка плана 8/8 | §12.10: **надёжность 7/10, уверенность 7/10** | Занижение объёма 12a и два молчаливых CI-провала — обоснование снижения |
| 13 | Порядок 12a/12b/12c не был подтверждён явно | §12.3: явная гарантия — циклов нет, 12a самоценна и мерджится независимо от 12b | Если 12b отменят, 12a остаётся чистым выигрышем: A-1/A-2 закрыты, Clean Architecture → ~8 |
| 14 | **Сверх критики (найдено при верификации).** Черновик утверждал «`domain` уже идеально чист» и «внутримонорепных зависимостей ровно две» | Находка 4 и §12.2.D: `agentplugins/domain/identity_portable_test.go` (`package domain_test`) импортирует `install/integrationctl/adapters/pathpolicy` → **третье ребро**, и оно тянет `golang.org/x/text/{transform,unicode/norm}` | Тестовые импорты пакетов модуля **попадают в его `require`-блок**. Без правки DoD «ни одного `require`» недостижим, и возникает цикл модулей — ровно тот блокер, ради которого закрывается A-1 |

---

## 12.0. Цель

Поднять «Модульность» с прогнозных 7-7.5 (§12 основного плана) до уровня конкурента (8) единственным
точечным шагом: превратить контракт расширения (`domain` + `ports` + `clients` верхнего уровня) в
**самостоятельный Go-модуль с нулём внешних зависимостей**, который out-of-tree автор клиентского адаптера
может подключить, не втягивая движок инсталла, `hujson`, `yaml.v3`, `jsonschema`, `regexp2`, `go-toml`,
`x/mod`, `x/sys`, `x/text`.

Побочный, но самостоятельно ценный результат: **оба принятых исключения §3.2 (A-1 и A-2) закрываются по
существу**, а не остаются «названной нечистотой». Это поднимает и потолок «Clean Architecture» (сейчас
7-7.5 именно из-за A-1/A-2/A-4; после Part 12 остаётся только A-4 — `As[T]`).

**Честная граница цели (правка по критике №11).** Вынос делает stdlib-only только **контрактный слой**.
`clients/shared` — пакет, которым пользуется каждый реальный клиентский адаптер (`OnlyNativeComponents`,
`HasSupportedMCP`, `ProjectMCPServers`, `ManifestFromEnvelope`, `RunClientCommand`) — **остаётся в монолитном
модуле**. Поэтому тезис «автору адаптера не нужен движок» строго верен для того, кто реализует голые
интерфейсы `clients.Lifecycle`/`Detector`/`PlanRefiner`, и неверен для того, кто пишет практичный адаптер по
образцу существующих. См. §12.4, где это развёрнуто и учтено в оценке.

**Non-goals Part 12:** отдельный GitHub-репозиторий, свой CI/CD и релизный цикл; вынос `clients/<id>`,
`clients/shared`, `clients/all`, `adapters/*`; публикация semver-тега (см. §12.6). Отдельный репозиторий —
логичный следующий шаг ПОСЛЕ этой части, но он заметно другого масштаба и в объём Part 12 не входит.

---

## 12.1. Факты разведки (VERIFIED на `d5250a3d9`)

Команды переверки перед стартом реализации — **обе обязательны**, вторая появилась из-за Находки 4:

```sh
cd install/integrationctl
# (1) non-test замыкание
go list -deps ./agentplugins/domain ./agentplugins/ports ./agentplugins/clients | grep -E '\.'
# (2) тестовые импорты - они тоже попадают в require-блок будущего модуля
go list -f '{{.ImportPath}}: {{join .TestImports " "}} | X: {{join .XTestImports " "}}' \
  ./agentplugins/domain ./agentplugins/ports ./agentplugins/clients
```

**Примечание про LOC-снимок (найдено при ревью guard-теста, до старта 12a/12b).** `d5250a3d9` уже включает
рост трёх файлов из ещё не смёрженного Part 4: `domain/clients.go` (313→351),
`domain/directory_context7_preparation.go` (41→48), `clients/planning.go` (59→110). На HEAD ветки, где
писался guard-тест из §12.1.G (Part 3, без Part 4), non-test LOC ядра контракта сейчас **2985**, а не 3081
(`domain` 2178, `clients` 587; `ports` и все test-LOC не отличаются). Это не ошибка снимка — заголовок честно
указывает коммит верификации, — но перед стартом 12b счётчики файлов и LOC нужно пересчитать заново той же
командой. Зависимостный снимок §12.1(c) (рёбра, замыкание, внешние модули), на котором стоит guard-тест, от
этого дрейфа не пострадал: он был независимо перепроверен на HEAD Part 3 и совпал с таблицей ниже один в
один.

### Состав контрактного слоя

| Пакет | non-test файлов | non-test LOC | test LOC |
|---|---|---|---|
| `agentplugins/domain` | 14 | 2223 | 1080 |
| `agentplugins/ports` | 6 | 220 | 0 |
| `agentplugins/clients` | 8 | 638 | 91 |
| **Ядро контракта** | **28** | **3081** | **1171** |
| `ports/contracttest` (опционально, §12.3) | 1 | 144 | 0 |
| `clients/contracttest` (опционально) | 4 | 439 | 189 |

Test-файлов в ядре контракта — **11**: 10 `_test.go` в `domain` (1080 LOC) + 1 в `clients`
(`registry_test.go`, 91 LOC); `ports` тестов не имеет.

`domain`: `planning.go` 14, `selection.go` 24, `acquisition.go` 29, `errors.go` 38,
`directory_context7_preparation.go` 48, `install_intent.go` 50, `chatgpt_mapping.go` 68, `identity.go` 79,
`catalog.go` 105, `security.go` 154, `types.go` 196, `state.go` 233, `clients.go` 351, `directory.go` 834.

`ports`: `lock.go` 11, `legacy.go` 17, `paths.go` 18, `runner.go` 37, `optional.go` 50, `interfaces.go` 87.

`clients`: `lifecycle.go` 36, `staging.go` 36, `clients.go` 50, `identity.go` 54, `detection.go` 60,
`registry.go` 82, `planning.go` 110, `host.go` 210.

### Полный список зависимостей вне этих трёх пакетов

**(a) stdlib — не проблема.**
`context`, `crypto/rand`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `errors`, `fmt`, `io`, `io/fs`,
`os`, `path/filepath`, `regexp`, `sort`, `strconv`, `strings`, `time`.

**(b) Внешние модули (non-test) — транзитивно, без прямых импортов у контракта:**
на linux/darwin ровно `github.com/tailscale/hujson` (через `adapters/nativeconfig`); на Windows к нему
добавляется `golang.org/x/sys` (см. `(b'')`). Версия `hujson` в `install/integrationctl/go.mod`:
`v0.0.0-20260302212456-ecc657c15afd`.

**(b') Внешние модули, приходящие через ТЕСТЫ:** `golang.org/x/text/transform`, `golang.org/x/text/unicode/norm`
(через `adapters/pathpolicy`, Находка 4).

**(b'') Внешний модуль, видимый только на Windows:** `golang.org/x/sys` — `adapters/nativeconfig/lock_windows.go`
и `adapters/nativeconfig/open_nofollow_windows.go` импортируют `golang.org/x/sys/windows`.
`adapters/atomicfile` на всех трёх GOOS остаётся stdlib-only (`syncdir_windows.go` внешних импортов не имеет).
`GOOS=windows go list -deps ./agentplugins/clients` этот модуль показывает; `go list` на linux/darwin — нет.
Host-platform CI поэтому его не увидит, и guard test из §12.1.G обязан пересчитывать замыкание для каждого
GOOS отдельно (см. §12.1.G, инвариант 3), иначе он слеп к дрейфу за Windows-only файлом ровно так же, как
черновик изначально был слеп к Находке 4 через тестовый импорт.

**(c) Внутримонорепные зависимости — ровно ТРИ (черновик знал две):**

| # | Ребро | Место | Что тянет транзитивно | Статус в плане |
|---|---|---|---|---|
| 1 | `agentplugins/ports` → `install/integrationctl/ports` | `ports/runner.go:8` (`legacyports.Command`/`CommandResult` в 4 сигнатурах: `CommandRunner`, `TreeCommandRunner`, `DuplexCommandRunner`, `DuplexCapabilityRunner`) | `install/integrationctl/domain` | исключение A-1, §12.2.B |
| 2 | `agentplugins/clients` → `agentplugins/adapters/nativeconfig` | `clients/clients.go:22` (поле `Env.NativeConfig nativeconfig.Kernel`) | `github.com/tailscale/hujson`, `install/integrationctl/adapters/atomicfile` | исключение A-2, §12.2.A |
| 3 | **`agentplugins/domain` (test) → `install/integrationctl/adapters/pathpolicy`** | `domain/identity_portable_test.go:6`, `package domain_test`, 71 строка, 4 вызова `pathpolicy.ValidateLeafID` (строки 28, 48, 51, 54) | `golang.org/x/text/transform`, `golang.org/x/text/unicode/norm`, `install/integrationctl/{domain,ports}` | **новое**, §12.2.D |

**Уточнение про «`domain` идеально чист».** Утверждение черновика верно **только для non-test кода**:
`go list -deps ./agentplugins/domain` действительно не даёт ни одного non-stdlib пакета. Но `go.mod` модуля
обязан покрывать и тестовые импорты его пакетов, поэтому ребро 3 — настоящий блокер DoD, а не косметика.

`adapters/atomicfile` сам по себе stdlib-only (`fmt`, `os`, `path/filepath`, `strings`), используется в
`nativeconfig/io.go:39` одной функцией `atomicfile.Write`.

Файлы `adapters/nativeconfig`, которым нужен `hujson`: `kernel.go`, `document.go`, `project.go`.
Файлы с `golang.org/x/sys/windows`: `lock_windows.go`, `open_nofollow_windows.go`.
Файлы без внешних зависимостей: `types.go`, `lock.go`, `lock_unix.go`,
`open_nofollow_unix.go` (и `io.go` — только `atomicfile`).

**Тестовые импорты `contracttest`-пакетов (VERIFIED, подтверждает §12.3):** `ports/contracttest` импортирует
только `agentplugins/ports` + `os`/`path/filepath`/`strings`/`testing`; `clients/contracttest` — только
`agentplugins/clients` + `agentplugins/domain` + `context`/`fmt`/`io/fs`/`os`/`reflect`/`testing`. Оба
переезжают без развязок.

### Четыре находки, которых не было в постановке задачи

**Находка 1 (меняет постановку A-2).** «Спроектировать узкий интерфейс, который `nativeconfig.Kernel`
реализует имплиситно» — **сам по себе не решает проблему**. Публичная поверхность `Kernel` — три метода:

```go
Apply(Request) (Receipt, error)
ApplyBatch(requests []Request) ([]Receipt, error)
Inspect(paths Paths, codec Codec, name string, owned *Receipt) (present, exactlyOwned bool, err error)
```

Все параметры и возвраты — типы **того же пакета** `nativeconfig`. Интерфейс, объявленный в `ports` или
`clients`, обязан их назвать, то есть импорт `adapters/nativeconfig` никуда не денется. Разорвать ребро
можно только **разделив пакет на DTO-часть и реализацию** (§12.2.A).

**Находка 2 (меняет серьёзность A-1).** На уровне пакетов A-1 — стилистическое исключение. На уровне
**модулей** это жёсткий блокер: если новый модуль `require`-ит
`github.com/777genius/plugin-kit-ai/install/integrationctl`, получаем

- (i) цикл модулей `integrationctl → core → integrationctl`;
- (ii) в go.mod контракта приезжает весь `require`-блок `integrationctl`: `gopkg.in/yaml.v3`,
  `github.com/tailscale/hujson`, `github.com/dlclark/regexp2`, `github.com/santhosh-tekuri/jsonschema/v6`,
  `github.com/pelletier/go-toml/v2`, `golang.org/x/mod`, `golang.org/x/sys`, `golang.org/x/text`;
- (iii) модуль перестаёт быть `go get`-абельным без `replace`.

A-1 придётся закрыть, а не пронести. Это не «желательно», а предусловие части.
**То же самое, дословно, относится к ребру 3 (Находка 4).**

**Находка 3 (окно дешёвой правки уже открыто).** `clients.Env.NativeConfig` **сегодня не используется ни в
одной строке кода**. Реальные потребители появятся в Part 7c (gemini/opencode/cline/windsurf получают ядро
через `Env`). То есть смена типа поля `Env.NativeConfig` со структуры на интерфейс сейчас стоит ноль.

Требование к Part 7c, страхующее это бесплатно, **уже внесено в §8.7 основного плана** — здесь оно намеренно
не дублируется, чтобы не создавать второй источник правды. При реализации 7c сверяться с §8.7.

Замечание о стоимости отсрочки: сам объём правок при разделении `nativeconfig` от момента запуска почти не
зависит (~300 ссылок `nativeconfig.*` переедут из `providers` в `clients/<id>` в Part 5/7c, но их количество
не изменится). А вот окно конфликтов с `origin/main` (§5.2) в Part 5-7 максимальное, поэтому выполнять
развязку после Part 11 — правильно, а не вынужденно.

**Находка 4 (новая; закрывает дыру в DoD).** `agentplugins/domain/identity_portable_test.go` —
`package domain_test`, 71 строка — импортирует `install/integrationctl/adapters/pathpolicy` и четыре раза
вызывает `pathpolicy.ValidateLeafID`, проверяя, что генерируемые `domain` leaf-ID удовлетворяют политике
путей. Тест содержательно верный и удалять его нельзя. Но:

- тестовые импорты пакетов модуля **входят в его `require`-блок** — значит `agentplugins-core/go.mod`
  получил бы `require github.com/777genius/plugin-kit-ai/install/integrationctl` **и**
  `golang.org/x/text`;
- это ровно цикл модулей из Находки 2, только через тестовый граф, где его никто не искал: `go list -deps`
  (без флагов) его **не показывает**.

Решение — §12.2.D. Стоимость: перемещение 71 строки без единой правки логики.

---

### 12.1.G. Guard test, добавляемый СЕЙЧАС (до старта Part 12)

> **Этот подраздел — спецификация для отдельной задачи, выполняемой немедленно, до Part 5.**
> Реализуется не в рамках Part 12, а как самостоятельный небольшой PR в текущей стопке.

#### Зачем

§12.1(c) — снимок на `d5250a3d9`. Part 5-9 наполняют `clients/<id>` кодом и активно правят `clients`,
`ports`, `domain`. Если в любой из этих трёх пакетов (включая их тесты) просочится новый импорт — например
`conformance`, `pathcontract`, `jsonschema/v6` или `yaml.v3`, которые уже тянет соседний `clients/shared`, —
§12.1(c) устареет молча. Сегодня это заметят только при старте Part 12, то есть через 5-7 частей, когда
правка будет стоить на порядок дороже и придётся переписывать план.

Находка 4 — доказательство, что риск не гипотетический: ребро `domain(test) → pathpolicy` существует
**уже сейчас**, прожило незамеченным весь черновик и всю критику, и его не видно ни в `go list -deps`, ни в
depguard-правилах (`domain-stdlib-only` объявлено с `"!$test"` и тесты намеренно не покрывает).

#### Что именно фиксировать

Тест утверждает **три инварианта** на контрактном слое `C = {agentplugins/domain, agentplugins/ports,
agentplugins/clients}` (только эти три пакета, без подпакетов `contracttest`):

1. **Исходящие рёбра.** Множество импортов из файлов `C` (включая `_test.go` и внешние тестовые пакеты
   `*_test`) в пакеты этого репозитория, не входящие в `C`, равно **ровно**:

   | из | в | почему разрешено |
   |---|---|---|
   | `agentplugins/ports` | `install/integrationctl/ports` | A-1, закрывается в Part 12a |
   | `agentplugins/clients` | `agentplugins/adapters/nativeconfig` | A-2, закрывается в Part 12a |
   | `agentplugins/domain` (только `_test.go`) | `install/integrationctl/adapters/pathpolicy` | Находка 4, закрывается в Part 12a |

2. **Транзитивное замыкание по репозиторию.** Замыкание импортов `C` внутри модуля равно ровно:
   `agentplugins/domain`, `agentplugins/ports`, `agentplugins/clients`,
   `agentplugins/adapters/nativeconfig`, `install/integrationctl/ports`, `install/integrationctl/domain`,
   `install/integrationctl/adapters/atomicfile`, `install/integrationctl/adapters/pathpolicy`.
   Это ловит дрейф, который прячется за уже разрешённым ребром (например, если `adapters/nativeconfig`
   однажды начнёт импортировать `conformance`).

3. **Внешние модули.** Множество внешних (не-stdlib, не-репозиторных) модулей в этом замыкании, по всем
   платформам, равно ровно `{github.com/tailscale/hujson, golang.org/x/sys, golang.org/x/text}`.
   `golang.org/x/sys` попадает в множество только через Windows-only файлы `adapters/nativeconfig`
   (`lock_windows.go`, `open_nofollow_windows.go`). `GOOS=windows go list -deps` его показывает;
   linux/darwin CI — нет, поэтому тест обязан пересчитывать замыкание для каждого GOOS из
   `{linux, darwin, windows}`, а не полагаться на платформу раннера.

Каждый инвариант — с точным сообщением об ошибке в стиле «появилось новое ребро `X → Y`; если это осознанно,
обнови §12.1(c) плана Part 12 и baseline этого теста, иначе — убери импорт».

#### Как реализовать (без новых внешних зависимостей)

`golang.org/x/tools` в модулях репозитория **нет**, добавлять его ради guard-теста нельзя. Вызывать
`go list -deps -json` через `os/exec` из теста — работает, но делает тест зависимым от наличия toolchain в
PATH и от режима workspace, а в CI это лишняя поверхность отказа.

**Рекомендуемый способ: расширить существующий инструмент `internal/archtest`.** Он уже:

- лежит в `install/integrationctl/agentplugins/internal/archtest/` как `package main` с тестами рядом
  (`boundaries_test.go`, `budget_test.go`, `pathpolicy_test.go`);
- парсит исходники через stdlib `go/parser` + `go/ast` + `go/token`;
- умеет находить корень репозитория (`repoRoot()` идёт вверх до каталога с `go.work`);
- уже знает нужные константы: `modulePath`, `domainImportPath`, `legacyPortsPath`.

Реализация — новый файл `contract_edges_test.go` в том же пакете (по факту реализации, а не как черновик
изначально описывал ниже — эквивалентно дешевле):

- собрать импорты только трёх пакетов `domain`/`ports`/`clients` (не рекурсивно — файлы, лежащие прямо в
  каталоге, `parser.ParseFile` с `parser.ImportsOnly` через `os.ReadDir`, без `filepath.WalkDir`/
  `parser.ParseDir` по всему `install/integrationctl`/`cli` — обход всего дерева не нужен: для
  инвариантов 2-3 достаточно BFS от уже найденных исходящих рёбер по производственным (non-test) импортам
  каждого следующего пакета до фиксированной точки, что на порядок дешевле полного обхода при тех же
  гарантиях);
- **обязательно включать `_test.go`** для самих `domain`/`ports`/`clients` — иначе Находка 4 не ловится. Это
  ключевое отличие от depguard-правила `domain-stdlib-only`, которое объявлено с `"!$test"`. Тестовые файлы
  пакетов, найденных на шаге BFS (не входящих в контрактную тройку), при этом не разбираются — они не часть
  будущего модуля;
- отличать stdlib от внешнего модуля по стандартной эвристике `goimports`: если первый сегмент пути импорта
  содержит точку — это внешний модуль, иначе stdlib. Репозиторные пакеты определяются по префиксу
  `modulePath`;
- нормализовать импорт до пути модуля + пути пакета; для инварианта 3 схлопывать
  `golang.org/x/text/...` → `golang.org/x/text` по первым трём сегментам для `golang.org/x/*`, иначе по
  первым трём сегментам домена (`github.com/owner/repo`);
- baseline держать **константами в самом тесте**, а не в `testdata/*.json`: множеств три, они крошечные, и
  правка константы в diff'е PR читается лучше, чем правка JSON.

`go/build.Context.MatchFile` для фильтрации файлов по GOOS/GOARCH — **не запасной вариант, а обязательная
часть реализации**, вопреки первоначальному черновику этого раздела: в самих `domain`/`ports`/`clients`
build-тегов действительно нет (§12.2.C), но инварианты 2-3 обходят весь достижимый снаружи них замыкание, а
там теги есть — `adapters/nativeconfig` (`lock_windows.go`, `open_nofollow_windows.go`) прячет
`golang.org/x/sys` за файлом, который компилируется только на Windows; `adapters/atomicfile`
(`syncdir_windows.go`) тоже платформенный, но внешних модулей не тянет. Без `MatchFile`
инвариант 3 либо не увидел бы `golang.org/x/sys` вовсе на не-Windows раннере, либо ошибочно требовал бы его
на всех платформах. Реализация поэтому прогоняет весь снимок (`computeContractSnapshot`) для каждого GOOS из
`{linux, darwin, windows}` (GOARCH зафиксирован на `amd64`, `CgoEnabled: false` — архитектурных и
cgo-зависимых файлов в контракте нет) и объединяет результаты, а не полагается на GOOS раннера, на котором
запущен `go test`.

#### Приёмка guard-теста

- Тест зелёный на текущем HEAD и фиксирует ровно три ребра из таблицы выше.
- Искусственная проверка (делается локально, в коммит не попадает):
  - добавить в `clients/clients.go` импорт `agentplugins/pathcontract` (реально существующий пакет) — тест
    падает сразу по всем трём инвариантам, включая каскад через `conformance`,
    `github.com/santhosh-tekuri/jsonschema` и `gopkg.in/yaml.v3`, которые `pathcontract` тянет транзитивно;
  - добавить в `ports` тестовый файл с импортом `gopkg.in/yaml.v3` — тест падает на инварианте 3;
  - добавить `_ "gopkg.in/yaml.v3"` в `adapters/nativeconfig/lock_windows.go` (файл с `//go:build windows`) —
    тест падает на инварианте 3, только если гоняет замыкание под `GOOS=windows`; это целевая проверка
    GOOS-развёртки, без неё тест на macOS/Linux-раннере остался бы зелёным.
- Тест выполняется быстрее 1 с даже при пересчёте замыкания для трёх GOOS (ImportsOnly-разбор), то есть не
  утяжеляет `core-fast`.
- Комментарий в шапке файла ссылается на §12.1(c) и §12.1.G этого документа.

Надёжность 8/10, уверенность 8/10 (минус — эвристика «точка в первом сегменте» формально не покрывает
экзотические пути импорта, но в этом репозитории таких нет).

---

## 12.2. Решения по каждой внутримонорепной зависимости

### A. `clients.Env.NativeConfig` → `adapters/nativeconfig` (исключение A-2)

#### Вариант A1 (Рекомендуется) — разделить `nativeconfig` на контракт-DTO и реализацию

**В новый модуль уезжает пакет `ports/nativeconfig`** (только данные, только stdlib, ~130 LOC — перенос из
`types.go`):

- `Codec` + типизированные константы `CodecMCPServers`, `CodecGemini`, `CodecOpenCode`, `CodecWindsurf`,
  `CodecCline` (`types.go:13-23`);
- `Action` + константы `ActionAdd`, `ActionUpdate`, `ActionRemove` (`types.go:26-32`);
- сентинелы `ErrAmbiguousConfig`, `ErrCollision`, `ErrNotOwned`, `ErrMalformed`, `ErrConcurrentChange`
  (`types.go:34-40`, объявлены через `errors.New`);
- `CommittedCleanupError` + `IsCommittedCleanup`;
- `Server`, `Paths`, `Placeholders`, `Receipt`, `Request`;
- сам порт:

```go
// agentplugins-core/ports/nativeconfig/kernel.go
type Kernel interface {
    Apply(Request) (Receipt, error)
    ApplyBatch(requests []Request) ([]Receipt, error)
    Inspect(paths Paths, codec Codec, name string, owned *Receipt) (present, exactlyOwned bool, err error)
}
```

**В основном дереве остаётся реализация:** конкретная структура `Kernel`, `FileIO`, `conditionalFileIO`,
`lockAcquirer`, `New`/`NewWithFileIO`/`NewWithLockAcquirer`, `validateRequest`, `supportedCodec`,
`codecCollectionKey`, `validateExactPath`, а также `document.go`, `io.go`, `kernel.go`, `lock*.go`,
`open_nofollow_*.go`, `project.go`. `DesiredReceipt` требует `hujson` для канонизации записи — остаётся
реализацией; `clients/<id>` её вызывать по-прежнему можно (§3.2 это разрешает), запрет касается только
контрактного пакета `clients`.

Конкретная структура реализует интерфейс имплиситно — правок в её методах не требуется.

**Где жить интерфейсу — в `ports/nativeconfig`, не в `clients`.** Обоснование по направлению зависимостей:
это порт инфраструктуры (наравне с `ports.CommandRunner` и `ports.PathPolicy`), и его потребитель не только
`clients.Env` — `providers`, `clients/<id>` и тесты принимают «ядро» параметром (VERIFIED: 24 ссылки
`nativeconfig.Kernel` в позиции типа, в основном в сигнатурах `*WithKernel`-функций). Класть
инфраструктурный порт в пакет контракта клиента значило бы, что `providers` импортирует `clients` ради типа,
не относящегося к клиенту.

##### Коллизия имён пакетов и её цена (пересчитано по критике)

Всего в модуле `install/integrationctl` **302** ссылки `nativeconfig.*` (VERIFIED подсчётом; черновик
говорил 296 — арифметическая ошибка в заголовочном числе, разбивка была верна и суммируется в 302):

| Уезжает в контракт (252) | Остаётся в реализации (50) |
|---|---|
| `Server` 65, `Kernel` 24, `Paths` 21, `Request` 19, `Receipt` 15, `Placeholders` 13, `ErrNotOwned` 13, `CodecOpenCode` 12, `CodecGemini` 12, `ActionAdd` 12, `CodecCline` 11, `CodecWindsurf` 8, `ActionRemove` 8, `IsCommittedCleanup` 5, `ErrCollision` 5, `ActionUpdate` 4, `ErrAmbiguousConfig` 2, `Codec` 2, `Action` 1 | `New` 36, `DesiredReceipt` 10, `NewWithFileIO` 2, `NewWithLockAcquirer` 2 |

Практически каждый native-config файл нуждается в обоих пакетах (например `providers/cline_native.go`
использует и `Request`/`Server`/`Codec*`, и `New()`/`DesiredReceipt()`). Поэтому:

**имя `nativeconfig` остаётся за контрактом** (252 ссылки не трогаются), а пакет реализации переименовывается
`adapters/nativeconfig` → `adapters/nativeconfigos` (package `nativeconfigos`, по аналогии с
`clientdetect.NewOS`). Правится 50 префиксов вызовов + 15 строк импорта.

Альтернатива — оставить имя реализации и алиасить импорт в каждом файле (`nativeconfigos "…/adapters/nativeconfig"`)
— даёт тот же объём правок, но оставляет два пакета с одинаковым именем, что ломает grep и путает ревью.

##### Внутренний churn в самой реализации и как он обнуляется (правка по критике №1)

Черновик оценивал эту статью как «~130 перенос + 50 префиксов + 15 импортов» и **не учёл**, что после выноса
DTO код самой реализации перестаёт видеть эти типы без квалификации. VERIFIED подсчётом по
`adapters/nativeconfig/`:

| Где | Строк, ссылающихся на переезжающие идентификаторы |
|---|---|
| non-test файлы реализации (`document.go` 11, `io.go` 2, `kernel.go` 38, `lock.go` 9, `project.go` 16, остаток `types.go`) | **63** |
| тесты реализации (`kernel_test.go` 107, `cline_cwd_test.go` 15, `opencode_exact_keys_test.go` 8, `lock_test.go` 2) | **130** |
| **Итого «наивный» churn** | **193 строки** |

193 строки механической правки `Request` → `ncports.Request` — это в **2.5 раза** больше, чем черновик
закладывал на всю статью, и это чистый шум в диффе самого содержательного PR части.

**Принятое лекарство: блок одноимённых алиасов в пакете реализации.** Один новый файл
`adapters/nativeconfigos/contract_aliases.go`, ~22 строки:

```go
package nativeconfigos

import ncports "github.com/777genius/plugin-kit-ai/agentplugins-core/ports/nativeconfig"

type (
	Codec                 = ncports.Codec
	Action                = ncports.Action
	Server                = ncports.Server
	Paths                 = ncports.Paths
	Placeholders          = ncports.Placeholders
	Receipt               = ncports.Receipt
	Request               = ncports.Request
	CommittedCleanupError = ncports.CommittedCleanupError
)

const (
	CodecMCPServers = ncports.CodecMCPServers
	CodecGemini     = ncports.CodecGemini
	CodecOpenCode   = ncports.CodecOpenCode
	CodecWindsurf   = ncports.CodecWindsurf
	CodecCline      = ncports.CodecCline
	ActionAdd       = ncports.ActionAdd
	ActionUpdate    = ncports.ActionUpdate
	ActionRemove    = ncports.ActionRemove
)

var (
	ErrAmbiguousConfig  = ncports.ErrAmbiguousConfig
	ErrCollision        = ncports.ErrCollision
	ErrNotOwned         = ncports.ErrNotOwned
	ErrMalformed        = ncports.ErrMalformed
	ErrConcurrentChange = ncports.ErrConcurrentChange
	IsCommittedCleanup  = ncports.IsCommittedCleanup
)
```

После этого внутренний churn = **0 строк**: код реализации и её тесты продолжают писать `Request`,
`ErrNotOwned`, `CodecGemini` без изменений.

Корректность механики (проверяемая компилятором):

- `type X = pkg.X` — **алиас**, не новый тип: идентичность типа сохраняется, никаких конверсий;
- `Codec`/`Action` — типизированные константы (`CodecMCPServers Codec = "mcpServers"`), поэтому
  `const CodecMCPServers = ncports.CodecMCPServers` сохраняет и значение, и тип;
- сентинелы — `var`, а не `const` (объявлены через `errors.New`), поэтому алиасятся как `var`: это **тот же
  указатель**, `errors.Is` через границу пакетов работает;
- `Kernel` в этот блок **не входит**: в пакете реализации `Kernel` — конкретная структура, а
  `nativeconfig.Kernel` — интерфейс контракта. Одноимённость здесь намеренная и безопасная
  (`nativeconfigos.Kernel` реализует `nativeconfig.Kernel`), но требует явного абзаца в godoc обоих пакетов.

Цена лекарства, названная честно: два публичных имени на один тип. Внешний код может написать
`nativeconfigos.Request` вместо `nativeconfig.Request` и получить то же самое. Это осознанный размен
22 строк на 193 и на чистый дифф в горячем PR; если позже алиасы захочется убрать, это отдельная
механическая правка, не блокирующая ничего.

##### Второе место, где тип становится интерфейсом (правка по критике №2)

Черновик учёл только `clients.Env.NativeConfig`. Пропущено поле в `providers` (VERIFIED,
`providers/activator.go:28-36`):

```go
type Activator struct {
	Runner       CommandRunner
	NativeConfig *nativeconfig.Kernel
}

func (activator Activator) nativeConfigKernel() nativeconfig.Kernel {
	if activator.NativeConfig != nil {
		return *activator.NativeConfig
	}
	return nativeconfig.New()
}
```

После A1 `nativeconfig.Kernel` — интерфейс, а `*nativeconfig.Kernel` — **указатель на интерфейс**. Само
объявление поля скомпилируется (Go это разрешает), а вот пять мест конструирования — нет, потому что они
передают `&committedKernel`, где `committedKernel` имеет конкретный тип `nativeconfigos.Kernel`:

| Файл | Строки |
|---|---|
| `providers/opencode_native_test.go` | 313, 347, 366, 381 |
| `usecase/manual_remote_lifecycle_test.go` | 241 |

Правка (входит в 12a, ~12 строк):

- поле `NativeConfig *nativeconfig.Kernel` → `NativeConfig nativeconfig.Kernel` (интерфейс);
- `nativeConfigKernel()` → `if activator.NativeConfig != nil { return activator.NativeConfig }`,
  разыменование убрать, nil-дефолт (`return nativeconfigos.New()`) сохранить;
- в пяти местах конструирования убрать `&`.

Совместимость: `Activator{NativeConfig: ...}` остаётся валидным литералом, `Activator{}` продолжает работать
через nil-дефолт, поведение не меняется. Заметим, что §8.7 основного плана явно оставляет
`providers.Activator.NativeConfig` как есть **до** Part 12 — эта правка и есть исполнение той отсрочки.

##### Пересмотренный объём A1

| Статья | Строк |
|---|---|
| Перенос DTO `types.go` → `ports/nativeconfig` | ~130 |
| Новый `kernel.go` с интерфейсом `Kernel` | ~10 |
| Алиас-блок `contract_aliases.go` в реализации | ~22 |
| Внутренний churn реализации и её тестов **после алиасов** | **0** (вместо 193) |
| Префиксы `nativeconfig.New`/`DesiredReceipt`/… → `nativeconfigos.` | 50 |
| Строки импорта | 15 |
| `clients.Env.NativeConfig` → интерфейс | 1 |
| `providers.Activator.NativeConfig` + `nativeConfigKernel()` + 5 конструкций | ~12 |
| **Итого A1** | **~240 строк** |

Надёжность 8/10, уверенность 8/10.

#### Вариант A2 — унести `adapters/nativeconfig` целиком в новый модуль

Вместе с `atomicfile` (stdlib-only, одна функция). Плюс: ноль правок call-site, минимальный технический риск.
Минус: модуль контракта перестаёт быть контрактом — в нём лежит CAS-реализация с внешней зависимостью
`hujson`, блокировками, платформенными файлами (`lock_unix.go`/`lock_windows.go`/`open_nofollow_*.go`);
out-of-tree адаптер тянет `hujson`. Это ровно то размывание границы, ради устранения которого всё затевается.

Надёжность (работоспособность) 9/10, ценность для цели 4/10, уверенность 9/10.

#### Вариант A3 — оставить как есть

Новый модуль `require`-ит `hujson` и `atomicfile` через `adapters/nativeconfig` как под-модуль. Требует
сделать под-модулем ещё и `atomicfile` (или дублировать 40 строк). Модуль контракта получает внешнюю
зависимость и версионные обязательства по ней.

Надёжность 6/10, уверенность 8/10. Не рекомендуется.

---

### B. `agentplugins/ports` → `install/integrationctl/ports` (исключение A-1)

#### Вариант B1 (Рекомендуется) — перенос + type alias

`Command` и `CommandResult` **переезжают** в `ports` нового модуля, а в legacy `install/integrationctl/ports`
остаются type alias:

```go
// install/integrationctl/ports/interfaces_runtime.go
import coreports "github.com/777genius/plugin-kit-ai/agentplugins-core/ports"

type Command = coreports.Command
type CommandResult = coreports.CommandResult
```

Это снимает возражение основного плана против A-1 («два источника правды и конвертация на каждом вызове»)
буквально: alias — это **один** тип, конвертации нет вообще.

**VERIFIED объём (числа уточнены по критике №6):**

- 113 ссылок `ports.Command`/`ports.CommandResult` в 14 файлах legacy-движка
  (`adapters/claude/adapter_apply.go`, `adapters/claude/adapter_inspect_plugin_list.go`,
  `adapters/gemini/adapter_apply_lifecycle.go`, `adapters/process/osrunner.go`,
  `adapters/source/resolver_clone.go`, `adapters/source/resolver_shared.go` + тесты) — **без правок**;
- внутри `agentplugins` ссылка встречается в 16 файлах, из которых **15 компилируются без единой правки**
  (`clients/shared/command.go`, `clients/clients.go`, `providers/{activator,kiro_acp,native_identity,
  claude_probe}.go` + 5 тестов `providers`, `adapters/sourceacquisition/acquirer.go`, 3 теста `usecase`);
  шестнадцатый — сам `ports/runner.go`, который и есть место правки. Черновик писал «16 файлов без правок» —
  ошибка на единицу;
- **дополнительно (в черновике отсутствовало): 14 ссылок в 4 файлах `cli`** —
  `internal/authoring/bootstrap/bootstrap.go:495`, `internal/authoring/mcpruntime/runtime.go:323`,
  `internal/agentpluginscli/idempotent_update_test.go` (3), `internal/agentpluginscli/cli_test.go` (9).
  Они тоже компилируются без правок (alias), **но именно из-за них** `cli/go.mod` обязан
  получить `require`/`replace` на новый модуль: он транзитивно называет типы, объявленные там.

`adapters/process.OS{}` продолжает удовлетворять и legacy `ports.ProcessRunner`, и `coreports.CommandRunner`.

Циклов нет: legacy `ports` → core `ports` → core `domain` → stdlib.

Требование AGENTS.md о сохранении legacy `plugin.yaml`-возможностей соблюдено: поведение не меняется ни на
байт, меняется только место объявления двух struct. Заявить это явно в описании PR (требование §5.1(g)).

Надёжность 9/10, уверенность 9/10.

#### Вариант B2 — независимые типы + мост в composition root

Объявить `Command`/`CommandResult` в core `ports` независимо и написать `processbridge`, конвертирующий
legacy `ProcessRunner` в `coreports.CommandRunner`. Плюс: legacy-пакет вообще не трогаем. Минусы: два
одинаковых типа реально существуют; мост нужен и для `TreeCommandRunner`, и для `DuplexCommandRunner`
(последний прокидывает `func(io.Writer, io.Reader) error`); `shared.RunClientCommand` меняет возвращаемый тип
→ правки во всех 15 файлах `agentplugins` и 4 файлах `cli/`.

Надёжность 7/10, уверенность 8/10.

#### Вариант B3 — оставить зависимость

Сделать `install/integrationctl` требованием нового модуля. Даёт цикл модулей и втягивает 8 внешних
зависимостей (Находка 2).

Надёжность 2/10, уверенность 9/10 (что это плохо).

---

### D. `agentplugins/domain` (test) → `adapters/pathpolicy` (Находка 4, новое)

Ребро существует только в `domain/identity_portable_test.go` (`package domain_test`, 71 строка). Тест
утверждает, что leaf-ID, которые генерирует `domain`, проходят `pathpolicy.ValidateLeafID` — содержательный
инвариант на стыке домена и политики путей, терять его нельзя.

#### Вариант D1 (Рекомендуется) — перенести тест на сторону потребителя

Файл целиком переезжает в `install/integrationctl` — в пакет, которому разрешено знать оба: например
`install/integrationctl/agentplugins/internal/domainpolicy/identity_portable_test.go` (новый каталог,
только тесты) или рядом с существующими тестами `adapters/pathpolicy`. Импорт `domain` становится импортом
из нового модуля, импорт `pathpolicy` остаётся локальным.

Плюсы: ассерт сохраняется дословно, логика не меняется ни на строку, ребро исчезает из контрактного модуля
вместе с `golang.org/x/text`. Направление зависимостей правильное: проверка «реализация порта принимает то,
что производит домен» — это проверка **реализации**, и жить ей у реализации.
Минус: тест физически уезжает из каталога `domain`, где его удобнее искать — лечится ссылкой в godoc
`domain/identity.go`.

Объём: перемещение 71 строки + правка пакета/импортов, ~5 изменённых строк.
Надёжность 9/10, уверенность 9/10.

#### Вариант D2 — скопировать правило валидации в тест

Заинлайнить проверку `ValidateLeafID` в сам тест, чтобы не импортировать адаптер. Минус решающий: правило
дублируется, и тест перестаёт проверять именно то, ради чего написан (согласованность с **реальной**
политикой). Дрейф гарантирован.

Надёжность 4/10, уверенность 9/10 (что это плохо).

#### Вариант D3 — удалить тест

Потеря реального инварианта ради чистоты `go.mod`. Не рассматривается.

Надёжность 2/10.

**Куда попадает:** в PR **12a**, вместе с остальной развязкой. Это дешевле и безопаснее, чем тащить в 12b,
где дифф и так на 294 файла.

---

### C. Прочие находки

- `usecase` контракт клиентов **не импортирует** (VERIFIED: `grep -rn "agentplugins/clients"` по
  `agentplugins/usecase/` пуст; закреплено depguard-правилом `usecase-through-ports`). Правило импорта плана
  соблюдается, дополнительных работ нет.
- Импортёры `agentplugins/clients` на сегодня: `adapters/clientdetect` (3 файла), `planner` (4),
  `providers` (1), `clients/all` (2), `clients/shared` (1), `clients/contracttest` (6),
  11 пакетов `clients/<id>` (по 2), `internal/archtest` (1), `cli/internal/agentpluginscli` (1). Ни одного
  запрещённого правилами §3.2.
- `ports/paths.go`, `ports/lock.go`, `ports/legacy.go`, `ports/optional.go`, `ports/interfaces.go` — только
  stdlib + `domain`. Чисто. Тестов у пакета `ports` нет вовсе (`TestImports` и `XTestImports` пусты).
- `clients/host.go`, `clients/detection.go` используют `os`/`io/fs` — stdlib, для модуля безвредно.
- `clients` в тестах импортирует только `domain` + `testing`. Чисто.
- Build-тегов (`_windows`/`_darwin`/`_unix`/`linux_*`) в переезжающих 28 non-test файлах **нет**. Риск
  `cross-build` для Part 12 низкий (в отличие от Part 5, где 39 платформенных файлов).

---

## 12.3. Механика выноса

### Почему пути импорта неизбежно меняются

Нельзя дать новому модулю путь `…/install/integrationctl/agentplugins` и разместить его в подкаталоге: Go
резолвит импорт по **самому длинному совпадающему префиксу модуля**, поэтому `…/agentplugins/providers`
попал бы в новый модуль и не нашёлся бы. Пересекающиеся префиксы двух модулей в одном дереве не работают.

Значит, `domain`/`ports`/`clients` получают новые пути импорта, и это главный механический объём части.
*(Вывод из правил резолва модулей, эмпирически не проверял — уверенность 9/10; проверяется первой же
компиляцией.)*

### Размещение и имя модуля

#### Вариант M1 (Рекомендуется)

Каталог `agentplugins-core/` в корне репозитория, модуль
`github.com/777genius/plugin-kit-ai/agentplugins-core`.

```
agentplugins-core/
  go.mod                     # module …/agentplugins-core; НИ ОДНОГО require; см. «Версия Go»
  README.md                  # что это, правила импорта, политика версионирования
  domain/                    # 14 файлов, 2223 LOC - содержимое без изменений
  ports/                     # 5 файлов + Command/CommandResult из B1
    nativeconfig/            # новый: DTO + интерфейс Kernel (A1)
    contracttest/            # опционально, см. ниже
  clients/                   # 8 файлов, 638 LOC (пакетов clients/<id> здесь НЕТ)
    contracttest/            # опционально
```

Плюсы: короткие пути (`…/agentplugins-core/domain`); имя явно говорит «контракт ядра»; тег-префикс короткий
(`agentplugins-core/v0.1.0`); при будущем выносе в отдельный репозиторий (ADR-0005 допускает «agentplugins
can later move to its own repository») каталог переезжает целиком без переразметки.
Минус: контракт лежит не рядом с `install/integrationctl`, где остальной код инсталла — нужен явный указатель
в `docs/ARCHITECTURE.md`.

Надёжность 8/10, уверенность 8/10.

#### Вариант M2

Каталог `install/integrationctl/agentplugins/core/`, модуль
`…/install/integrationctl/agentplugins/core`.
Плюс: физически рядом с ядром, `git log` по каталогу `agentplugins` продолжает видеть контракт.
Минусы: пути импорта под 90 символов
(`github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/core/domain`); тег
`install/integrationctl/agentplugins/core/v0.1.0`; будущий split репозитория сложнее.

Надёжность 8/10, уверенность 8/10.

#### Вариант M3

Отдельный путь `github.com/777genius/agentplugins-core` при каталоге внутри репо. Отвергнуть: путь модуля не
соответствует реальному расположению → работает только через `replace`, не `go get`-абелен, то есть
уничтожает основную ценность.

Надёжность 3/10, уверенность 9/10.

### Версия Go в `go.mod` нового модуля (правка по критике №7)

Черновик копировал `go 1.25.0` / `toolchain go1.25.13` у `install/integrationctl`. **Это ошибка по существу
цели части.** Начиная с Go 1.21, если зависимость объявляет `go 1.X` выше, чем main-модуль потребителя,
сборка падает. То есть завышенный `go`-директив на контрактном модуле отсекает ровно тех out-of-tree
потребителей, ради которых вынос затевается — при том, что ни одной возможности Go 1.25 контракт не
использует.

Соседние модули репозитория, для калибровки (VERIFIED): root `go 1.22`, `plugininstall` `go 1.22`,
`sdk` `go 1.22`, `install/integrationctl` `go 1.25.0`, `cli` `go 1.25.8`. Единого стандарта
нет — копировать «как у соседа» бессмысленно.

**Что реально нужно контракту (VERIFIED по исходникам):**

- дженерики — `func As[T any](r *Registry, id domain.ClientID) (T, bool)` (`clients/registry.go:71`) → **1.18**;
- `any` как алиас `interface{}` → 1.18;
- `slices`/`maps`/`cmp`/`min`/`max`/range-over-func/`errors.Join` — **не используются**.

Формальный потолок, таким образом, — `go 1.18`.

**Процедура (обязательная, выполняется в 12b, занимает минуты):**

```sh
cd agentplugins-core
for v in 1.18 1.19 1.20 1.21 1.22 1.23; do
  go mod edit -go=$v && GOWORK=off go build ./... && GOWORK=off go vet ./... \
    && echo "OK $v" || echo "FAIL $v"
done
```

**Рекомендация: `go 1.22`.** Надёжность 8/10, уверенность 7/10. Обоснование — не «минимально возможное», а
«минимальное безопасное»:

- на границе **1.21/1.22** меняется семантика переменной цикла (`for _, x := range …` — своя переменная на
  итерацию с 1.22). В контракте есть циклы, берущие элемент по значению (`clients/registry.go:60`,
  `domain/clients.go:145`, `clients/host.go:102,116`); объявить `go 1.21` означает попросить компилятор
  применить **старую** семантику к этому коду. Сегодня он собирается под 1.25 с новой. Разница может быть
  нулевой, но проверять её на 3081 LOC — не та работа, ради которой стоит выигрывать три минорных версии;
- `1.22` совпадает с тремя из пяти модулей репозитория, то есть не создаёт нового прецедента;
- запас до `install/integrationctl` (1.25.0) — три минорных версии, чего достаточно для практического
  «не отсекаем потребителей».

Альтернатива: `go 1.18` — максимальная совместимость, но обязателен явный прогон тестов под старой
loop-семантикой и отказ от `toolchain`-директивы (она понимается только с 1.21). Надёжность 6/10,
уверенность 7/10.

`toolchain go1.25.13` оставить можно: для потребителя-зависимости `toolchain` игнорируется, а в CI он
закрепляет воспроизводимость. Решение принимается после прогона бисекта, а не до.

### `contracttest` — включать ли в модуль

Рекомендую **да, отдельным третьим PR**: `ports/contracttest` (144 LOC, импортирует только `ports`) и
`clients/contracttest` (628 LOC, импортирует только `clients` + `domain` + `testing`) — это ровно то, что
нужно out-of-tree автору адаптера, чтобы доказать соответствие контракту. Без них тезис «контракт пригоден
для внешних адаптеров» остаётся декларативным. Оба уже stdlib-чистые (VERIFIED по `Imports`, `TestImports` и
`XTestImports`, §12.1), дополнительных развязок не требуют.

**Остаточный риск для guard-теста (§12.1.G).** `ports/contracttest` и `clients/contracttest` намеренно вне
множества `C`, которое стережёт guard test — он видит только `agentplugins/domain`, `agentplugins/ports`,
`agentplugins/clients` верхнего уровня. Если один из `contracttest`-пакетов до PR 12c наберёт зависимость,
не совместимую с будущим модулем, guard test этого не заметит: его нужно либо явно расширить на оба
`contracttest`-пакета к моменту, когда 12c реально становится следующим шагом, либо принять точечную
повторную проверку `go list -f '{{.Imports}}' ./agentplugins/ports/contracttest ./agentplugins/clients/contracttest`
непосредственно перед стартом 12c.

### Объём механических правок (VERIFIED подсчётом, с учётом правок критики)

| Что | Файлов | Строк |
|---|---|---|
| Смена путей импорта `domain`/`ports`/`clients` | **294** (202 в `install/integrationctl`, 91 в `cli`, 1 в `repotests/`) | ~360 строк импортов (285 `domain` + 34 `ports` + 41 `clients`) |
| Переезд файлов контракта (`git mv`) | 28 non-test + 11 test | 4252 LOC перемещения |
| Разделение `nativeconfig` (A1, пересчитано) | ~17 | **~240** (см. таблицу в §12.2.A1) |
| Alias `Command`/`CommandResult` (B1) | 2 | ~10 |
| Переезд `identity_portable_test.go` (D1) | 2 | 71 перемещения + ~5 |
| `go.mod` нового модуля, `go.work`, `require`/`replace` в `install/integrationctl/go.mod` и `cli/go.mod` | 4 | ~15 |
| `.golangci.yml`: depguard-globы, `pkg:`-префиксы, новое deny-правило для `clients` | 1 | ~65 |
| `internal/archtest/archtest.go`: `domainImportPath`, `legacyPortsPath`, `domainDir`, `clientsRoot`, список scan-баз (строки 19, 24-25, 33-35, 104, 164, 167) + baseline guard-теста §12.1.G | 2 | ~25 |
| CI: `Makefile`, `lint.yml`, `core-fast.yml`, `coverage.yml`, `agentplugins-release.yml`, `codeql.yml`, `govulncheck.yml` (§12.5) | 7 | ~35 |
| `docs/ARCHITECTURE.md`, ADR, README модуля | 3 | ~120 |

**Итого ≈330 изменённых файлов, из них 294 — однострочная механическая правка импорта.** Реальной новой
логики — ~150 строк (`ports/nativeconfig` + алиасы + alias `Command`).

Правки импорта выполняются скриптом (`gofmt -r` / `sed` + `goimports`), а не руками; ревью — по
`git diff -M --stat` и по факту компиляции всех шести модулей.

### Разбиение на PR

Часть слишком широка для одного PR, но и дробить сверх меры незачем. Три PR, строго последовательно:

- **12a — развязка контракта (внутри текущего модуля, без `go.mod`).**
  Разделение `nativeconfig` на `ports/nativeconfig` (DTO + порт) и `adapters/nativeconfigos` (реализация) с
  алиас-блоком; `Command`/`CommandResult` → `agentplugins/ports` + alias в legacy; `Env.NativeConfig` и
  `Activator.NativeConfig` становятся интерфейсом (5 конструкций теряют `&`); переезд
  `identity_portable_test.go` (D1); depguard: из `ports-only-domain` удаляется allow на
  `install/integrationctl/ports`, а в правила для `clients` **добавляется** deny на
  `agentplugins/adapters/nativeconfig` (сегодня его там нет — см. §12.5). Объём ~250 строк логики + ~80
  правок ссылок.
- **12b — собственно модуль.**
  `go.mod` (с бисектом версии Go), `go.work`, `git mv` 28+11 файлов, массовая смена импортов (294 файла),
  `require`/`replace` у потребителей, CI/Makefile/archtest/`.golangci.yml`. Объём ~4300 строк перемещения +
  ~400 правок.
- **12c — `contracttest` в модуль + документация.**
  `ports/contracttest`, `clients/contracttest`, README модуля, ADR, `docs/ARCHITECTURE.md`. Объём ~800 строк
  перемещения + ~150 доков.

**Порядок подтверждён критиком (правка №13).** Циклов между PR нет; зависимость строго односторонняя
12a → 12b → 12c. **12a самоценна и мерджится независимо от судьбы 12b:** если 12b отменят или отложат,
12a остаётся чистым выигрышем — A-1 и A-2 закрыты по существу, потолок «Clean Architecture» поднимается с
7-7.5 до ~8, а стоимость отката 12a равна нулю (ничего в неё не упирается).

**Жёсткое правило A (порядок):** 12b не начинается, пока 12a не смёржен и depguard-исключения не сняты
(иначе 12b «заработает» на `go.work` + `replace`, но модуль будет невыносимым за пределы репозитория —
полная цена при недостигнутой цели).

**Жёсткое правило B (синхронизация, правка по критике №8):** **и 12a, и 12b мерджатся в тот же день, в
который открыты, и сразу после мерджа тянут `origin/main`** (§5.2 основного плана). Для 12b основание —
дифф на 294 файла. Для 12a основание не слабее: 12a трогает `providers/activator.go` (**23 правки в `main`
за 60 дней — самый горячий файл проекта**, см. §8.7) и все четыре `*_native.go`
(`gemini_native.go`, `opencode_native.go`, `cline_native.go`, `windsurf_native.go`). Черновик распространял
это правило только на 12b — это была недооценка.

---

## 12.4. Что получается на выходе (и почему это именно «модульность»)

`agentplugins-core/go.mod` после 12a+12b:

```
module github.com/777genius/plugin-kit-ai/agentplugins-core

go 1.22

toolchain go1.25.13
```

**Ни одного `require`. Ни одного `go.sum`.**

Контракт расширения — stdlib-only модуль. Это проверяемое компилятором и `go mod` свойство, а не
договорённость и не lint-правило: правила `domain-stdlib-only` и `ports-only-domain` из `.golangci.yml`
становятся дублирующей страховкой поверх границы, которую держит сама система модулей.

Out-of-tree автор адаптера делает `go get …/agentplugins-core`, реализует `clients.Lifecycle` и прогоняет
`clients/contracttest` — не получая ни движка инсталла, ни `hujson`, ни `yaml.v3`.

### Оговорка, без которой оценка была бы завышенной (правка по критике №11)

Предыдущий абзац верен буквально — и именно поэтому его легко прочитать шире, чем он есть. Что остаётся
**вне** нового модуля: пакет `clients/shared`, которым пользуется **каждый** реальный клиентский адаптер в
репозитории. Его публичные функции — `OnlyNativeComponents`, `HasSupportedMCP`, `ProjectMCPServers`,
`ManifestFromEnvelope`, `RunClientCommand`.

Его транзитивные зависимости (VERIFIED, `go list -deps ./agentplugins/clients/shared`):

`github.com/tailscale/hujson`, `github.com/santhosh-tekuri/jsonschema/v6` (+ `/kind`), `gopkg.in/yaml.v3`,
**11 пакетов** `golang.org/x/text/*`, `agentplugins/conformance`, `agentplugins/pathcontract`,
`agentplugins/adapters/nativeconfig`, `install/integrationctl/adapters/atomicfile`, legacy
`install/integrationctl/{domain,ports}`.

Практический вывод, который надо говорить вслух:

- **верно:** автору, реализующему голые интерфейсы `clients.Lifecycle`/`Detector`/`PlanRefiner`/`Stager`,
  достаточно stdlib-only модуля;
- **неверно:** что то же самое справедливо для автора, пишущего адаптер «как существующие». Ему нужен
  `clients/shared` — а значит весь перечисленный набор.

Вынести `clients/shared` в Part 12 нельзя: он по построению стоит на `conformance`, `pathcontract` и
реализации `nativeconfig`, то есть его вынос — это вынос половины движка, другая часть работы.

Поэтому граница, которую Part 12 делает структурной, — **узкая контрактная**, а не «полный SDK для
внешнего адаптера». Это по-прежнему заметное улучшение (сегодня границы нет вовсе, только линтер), но
обещать по ней паритет с конкурентом «с запасом» нельзя.

### Пересчёт таблицы §12 основного плана после Part 12

| Критерий | Прогноз после Part 11 | Прогноз после Part 12 | Что изменилось |
|---|---|---|---|
| Clean Architecture | 7-7.5 | **8** | A-1 и A-2 закрыты по существу (плюс третье, тестовое ребро — Находка 4); остаётся только A-4 (`As[T]` — runtime-механизм). Оценка согласована с критиком |
| Модульность | 7-7.5 | **7.5-8** | Граница контрактного слоя держится системой модулей, а не линтером; контракт stdlib-only и `go get`-абелен; `contracttest` доступен внешнему потребителю. Не 8-8.5, потому что `clients/shared` остаётся в монолите (см. оговорку выше) |
| Среднее | ≈7.0-7.5 | **≈7.35-7.65** | Против конкурента 7.4 — **паритет с маргинальным краем**, а не уверенный обгон |

Честная оговорка: SOLID, DRY и «Качество кода» Part 12 не двигает — их потолки заданы `As[T]`, остаточным
структурным сходством native-config клиентов и файлами вне скоупа DoD (§11).

Вторая честная оговорка: оценка конкурента (AgentBridge, модульность 8/10, среднее 7.4) взята из аудита
пользователя; код конкурента не смотрел. Разница 7.35-7.65 против 7.4 лежит внутри погрешности любой такой
оценки — формулировка «обгоняем» была бы натяжкой.

---

## 12.5. CI и инфраструктура

### Базовое

- **`go.work`:** `use ./agentplugins-core` → **6 модулей**. Все упоминания «5 модулей `go.work`» в §3.5,
  §5.1(a), §10 и в комментарии `core-fast.yml` обновить на 6.
- **`.golangci.yml`** (фактическое состояние, правка по критике №3):
  - globы `**/install/integrationctl/agentplugins/{domain,ports,clients}/**` →
    `**/agentplugins-core/{domain,ports,clients}/**` в правилах `domain-stdlib-only`, `ports-only-domain`,
    `clients-no-upward`, `clients-no-concrete-clients`;
  - `pkg:`-префиксы deny-правил `clients-no-upward`, `clients-no-concrete-clients`,
    `dispatchers-take-an-injected-registry`, `usecase-through-ports` — на новый путь для `clients`;
  - из `ports-only-domain` **удаляется** allow на
    `github.com/777genius/plugin-kit-ai/install/integrationctl/ports` вместе с комментарием
    «DELIBERATE EXCEPTION» (VERIFIED: allow-список и комментарий действительно существуют);
  - **исправление черновика.** Черновик требовал «удалить из правила для `clients` разрешение на
    `adapters/nativeconfig`». **Такого разрешения нет.** `clients-no-upward` и `clients-no-concrete-clients` —
    **deny-списки**, а не allow-списки; `adapters/nativeconfig` в них просто не упомянут, поэтому и
    разрешён. Исключение A-2 существует сегодня **только прозой**: комментарий в
    `clients/clients.go:13-16` и абзац в `docs/ARCHITECTURE.md:106-114`.
    Следовательно 12a не удаляет правило, а **добавляет** недостающее:

    ```yaml
    # в clients-no-upward, после закрытия A-2 в Part 12a
    - pkg: github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters
      desc: the client contract names ports/nativeconfig.Kernel, never a concrete adapter
    ```

    и вычищает прозу в двух указанных местах, заменяя её на «A-2 закрыто в Part 12».
- **LEGACY SIZE BASELINE — отдельный подводный камень (VERIFIED).** В baseline 173 записи, из них **две**
  относятся к переезжающим файлам:
  `^install/integrationctl/agentplugins/domain/directory\.go$` (**строка 674**) и
  `^install/integrationctl/agentplugins/domain/security\.go$` (**строка 677**).
  Черновик указывал строки 443 и 446 — устаревшие номера. Номера строк в `.golangci.yml` сдвигаются от части
  к части; при реализации брать их `grep`-ом, а не из плана.
  При смене пути `scripts/check-lint-baseline.sh` увидит две **добавленные** записи и упадёт (shrink-only).
  Нужен тот же allow-list переименований, что предусмотрен для Part 5 (§8.5), явно заявленный в описании
  PR 12b.
- **Новый guard:** шаг в `core-fast`, падающий, если `agentplugins-core/go.mod` содержит блок `require` или
  если рядом появился `go.sum`. Это машинная проверка главного инварианта части — сильнее любого
  depguard-правила, потому что ловит и транзитивный дрейф, в том числе через тестовые импорты (Находка 4).

### Пять CI-поверхностей, которые молча перестанут видеть контракт (правка по критике №5)

**Фундаментальный факт, проверенный эмпирически на `d5250a3d9`:**

```
$ go list ./...            # в корне репозитория, workspace активен
→ 6 пакетов, все из root-модуля. Ни одного из install/integrationctl, cli, sdk, plugininstall.

$ go list ./install/integrationctl/...
→ 70 пакетов.
```

То есть бесплатного покрытия нового модуля не будет **нигде**: явный подкаталожный паттерн workspace-модули
видит, а `./...` — нет. Каждая из пяти поверхностей требует отдельной строки, иначе провал будет **тихим**
(зелёный CI при выпавшем из проверки коде).

#### (a) `Makefile` → `test-required` — 1171 LOC тестов контракта выпадают из Required

Сегодня (строки 57-64) цель перечисляет модули руками:

```make
test-required:
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./cli/...
	go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./install/integrationctl/...
	...
```

Правка: добавить строку `go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./agentplugins-core/...`.
Без неё `./install/integrationctl/...` перестаёт видеть `domain`/`ports`/`clients`, и **1171 LOC тестов
контракта уходят из Required-гейта без единого сообщения**.

Там же: `LINT_MODULES` (строка 13) `. cli install/integrationctl` → **4 модуля**;
`vet` (строка 206) получает `cd agentplugins-core && go vet ./...`; `test-core` (строка 47) получает
`./agentplugins-core/...`.

#### (b) `.github/workflows/coverage.yml` — конфликт с DoD «Codecov не ниже»

Строки 36-37 собирают профили отдельно по модулям, а строка `files:` в шаге Codecov перечисляет их поимённо:

```yaml
files: coverage/root.out,coverage/cli.out,coverage/integrationctl.out,coverage/plugininstall.out,coverage/sdk.out
```

После выноса `domain` (2223 non-test / 1080 test LOC) и `clients` уходят из профиля `integrationctl.out` и
никуда не приходят. Это **прямой конфликт с DoD Part 11** («Codecov не ниже базы»): покрытие формально
изменится, причём непредсказуемо в обе стороны, и разбираться придётся на финальном PR.

Правка — две строки:

```yaml
go test -count=1 -timeout=20m -covermode=atomic -coverprofile=coverage/agentplugins-core.out ./agentplugins-core/...
```

и `agentplugins-core.out` добавляется в `files:`. **Обе строки обязательны:** без второй профиль соберётся и
будет молча выброшен (`disable_search: true`).

#### (c) `.github/workflows/core-fast.yml` — paths-фильтр перестаёт ловить PR по контракту

Строки 9-14 ограничивают merge-гейт путями:

```yaml
paths:
  - 'install/integrationctl/agentplugins/**'
  - 'install/integrationctl/adapters/pathpolicy/**'
  - 'cli/internal/agentpluginscli/**'
  ...
```

После выноса PR, трогающий **только** контракт (например, добавляющий метод в `clients.Lifecycle`), не
запустит `core-fast` вовсе — включая guard на отсутствие `require`-блока, то есть именно ту проверку, ради
которой всё делалось. `'**/go.mod'` в списке спасает только правки самого `go.mod`, а не `.go`-файлов.

Правка: добавить `- 'agentplugins-core/**'`.

Там же: `cross-build` (строка ~103) — добавить модуль в цикл сборки по трём GOOS (сейчас цикл
`for module in install/integrationctl cli`); `vet-all` (строка ~125) — через обновлённый
`make vet`.

#### (d) `.github/workflows/agentplugins-release.yml` — релиз перестаёт тестировать контракт

Строка 124 (шаг «Test standard-first engine and CLI»):

```bash
(cd install/integrationctl && go test ./agentplugins/... ./adapters/dirswap ./adapters/source)
```

`./agentplugins/...` после выноса не покрывает контракт. Правка: добавить строку
`(cd agentplugins-core && GOWORK=off go test ./...)`. `GOWORK=off` здесь не косметика — он заодно доказывает
на каждом релизе, что модуль собирается standalone.

#### (e) `.github/workflows/lint.yml`, `codeql.yml`, `govulncheck.yml`

- **`lint.yml`:** матрица модулей (строки 18-30) 3 → 4. Без этого новый модуль не линтуется вообще, и
  depguard-правила, перенесённые на новые пути, не применяются ни к чему.
- **`codeql.yml`:** шаг «Build Go modules for CodeQL» (строки 43-67) перечисляет модули явно, каждый как
  `(cd <dir>; export GOWORK=off; go build ./...)`. Без добавления блока для `agentplugins-core` из
  CodeQL-покрытия выпадают **3081 LOC** — весь контрактный слой, включая `domain/security.go` и
  `domain/directory.go`, то есть код валидации путей и границ. Правка: ещё один такой же блок.
- **`govulncheck.yml`:** матрица модулей (строки 11-25). Добавить запись `agentplugins-core` стоит для
  единообразия, но **ценность близка к нулю и это надо сказать прямо**: у модуля не будет зависимостей, а
  stdlib-уязвимости той же toolchain уже покрыты пятью существующими записями. Это не дыра, в отличие от
  CodeQL.

---

## 12.6. Версионирование

**Рекомендация: semver-тег в Part 12 НЕ ставить.** Надёжность 8/10, уверенность 8/10.

Обоснование: тег — это публичное обязательство о совместимости. Ставить его на контракт, который только что
(Part 5-11) закончил меняться и у которого **нет ни одного внешнего потребителя**, значит принять
обязательство раньше, чем появилась причина. Nested-модуль в `go.work` работает без тегов; потребители
внутри репозитория используют `require v0.0.0` + `replace` — ровно та конвенция, которая уже действует для
`install/integrationctl`, `plugininstall` и `sdk` в `cli/go.mod` (VERIFIED). Ранний тег, кроме
того, потребует решить вопрос синхронизации версии контракта с релизами `plugin-kit-ai` — а это уже работа
другого масштаба.

Что сделать вместо тега: секция «Versioning» в `agentplugins-core/README.md` —

> модуль подключается через `go.work`; semver-теги с префиксом `agentplugins-core/` будут введены с
> появлением первого out-of-tree потребителя; до этого контракт может меняться вместе с ядром в рамках
> одного PR.

Первый тег — `agentplugins-core/v0.1.0`, при этом стандартная семантика pre-1.0 (минорная версия может
ломать) фиксируется в README явно.

Честная оговорка: без тега `go get …/agentplugins-core` резолвится в псевдоверсию с дефолтной ветки. Для
раннего внешнего адоптера этого достаточно, но «стабильным API» это называть нельзя, и в README так и надо
написать.

Альтернативы:

2. Поставить `agentplugins-core/v0.1.0` сразу. Плюс: сигнал «граница настоящая», внешний адоптер получает
   воспроизводимую версию. Минус: обязательство без потребителя; каждое изменение контракта требует решения
   о bump. Надёжность 6/10, уверенность 7/10.
3. `v0.0.x` автоматически на каждый мердж. Шум в тегах репозитория при нулевой пользе (псевдоверсии дают то
   же самое). Надёжность 4/10, уверенность 8/10.

---

## 12.7. Совместимость

- Публичные JSON (`DeliveryPlan`, `ActivationOutcome`, `DeactivationOutcome`, `ClientCapabilities`,
  `ClientCompatibility`, `DetectedClient`), тексты `UserActions`/`Warnings`/`LocalActions`/`Diagnostics` и их
  порядок, формат `state-v2.json`, `NativeObjectOwnership.Kind/ObjectID`, поведение CLI — **не меняются ни на
  байт**. Часть механическая: переезд объявлений и смена путей импорта.
- Экспортируемые имена типов и функций не меняются — меняются только пути их импорта. Для всего кода вне ядра
  (`cli/internal/authoring/{commands,mcpruntime,nativeimport,project,readiness,report,scaffold,skills}`,
  `cli/internal/terminalprompts/{huh,plain}.go`, `cli/cmd/agentplugins-conformance-adapter/main.go`,
  `cmd/agentplugins-registry-mirror/main.go`, `repotests/*`) правка — одна строка импорта на файл.
- **Единственное изменение публичной сигнатуры в ядре:** `providers.Activator.NativeConfig` меняет тип с
  `*nativeconfig.Kernel` на интерфейс `nativeconfig.Kernel` (§12.2.A1). Поле остаётся, имя остаётся,
  `Activator{}` без него работает по-прежнему; ломается только код, берущий адрес конкретного ядра. Внутри
  репозитория таких мест пять, все в тестах. Это надо назвать в описании PR 12a явно.
- Legacy-движок `plugin.yaml` (`install/integrationctl/{adapters,usecase,domain}` вне `agentplugins`)
  затрагивается **ровно двумя строками alias** в `ports/interfaces_runtime.go` (вариант B1). Заявить это явно
  в описании PR 12a — требование §5.1(g) и AGENTS.md репозитория.
- `cli` получает `require`/`replace` на новый модуль — не из-за прямых импортов контракта, а
  из-за 14 ссылок `ports.Command*` в 4 файлах (§12.2.B1), которые после alias указывают на типы нового
  модуля.
- Переходные алиасы `providers.CommandRunner` и `providers.ManagedMarketplaceName` к Part 12 уже удалены
  (Part 11) — конфликта нет.
- Публичный фасад `planner` (`Capabilities`, `Compatibility`, `ClientCompatibility`, `KiroPrepareAction`,
  `ChatGPTAppBindingAction`, `DetectedPhysicalClient`, `ApplyInstallIntent`) остаётся в
  `install/integrationctl` и не затрагивается.

---

## 12.8. Приёмка

1. `agentplugins-core/go.mod` не содержит блока `require`; файла `agentplugins-core/go.sum` не существует;
   guard-шаг в `core-fast` это проверяет и падает при нарушении.
2. `go list -deps ./...` в `agentplugins-core` не выводит ни одного пакета вне stdlib и самого модуля;
   **отдельно** — `go list -f '{{join .TestImports " "}} {{join .XTestImports " "}}' ./...` не выводит ни
   одного пакета вне stdlib и самого модуля (проверка, которой не хватало и которая нашла Находку 4).
3. `go build`, `go vet`, `go test` зелёные во всех **6** модулях `go.work`; `make vet`, `make lint`,
   `make test-core`, `make test-required` зелёные.
4. `cross-build` зелёный на linux/darwin/windows для всех модулей ядра.
5. Счётчик `func Test*` в ядре не уменьшился — контроль того, что тесты `domain`/`ports`/`clients` реально
   попали в `test-core`/`test-required` нового модуля, а не выпали молча.
6. Golden-тесты (plan / detect / activate / staging / CLI JSON) — без diff.
7. depguard: правило `ports-only-domain` больше не содержит исключения на `install/integrationctl/ports`;
   в правило для `clients` **добавлено** deny на `agentplugins/adapters` (его там не было — §12.5); проза
   про осознанные исключения удалена из `clients/clients.go:13-16` и переписана в `docs/ARCHITECTURE.md`
   (строки 106-114) как «A-1/A-2 закрыты в Part 12».
8. `archtest` зелёный после обновления путей; бюджет ветвлений по `ClientID` остаётся 0; guard-тест из
   §12.1.G обновлён (после 12a контрактный слой не имеет исходящих рёбер вовсе — baseline становится пустым,
   и это и есть его финальная форма).
9. Baseline shrink-only: две переименованные записи (`domain/directory.go`, `domain/security.go`) проведены
   через allow-list переименований, новых записей нет.
10. **Отвязываемость модуля доказывается машинно (правка по критике №4), а не импортом внутри репозитория:**

    ```sh
    cd agentplugins-core
    GOWORK=off go build ./...
    GOWORK=off go test ./...
    GOWORK=off go list -deps ./... | grep -E '\.' && echo "FAIL: external dep" || echo "OK: stdlib only"
    ```

    Черновиковый критерий «`clients/all/registry_test.go` импортирует `contracttest` из другого модуля»
    **снят как невалидный**: внутри одного коммита он доказывает только, что работают `go.work` и `replace`,
    а не что модуль пригоден для out-of-tree потребителя. Импорт через межмодульную границу остаётся как
    полезный сам по себе тест, но не как доказательство.
11. `docs/ARCHITECTURE.md` и ADR обновлены: §3.2 «два осознанных исключения» → «исключения A-1/A-2 закрыты в
    Part 12; остаётся A-4 (`As[T]`)»; таблица самооценки §12 пересчитана по §12.4, **включая оговорку про
    `clients/shared`** — без неё таблица вводит в заблуждение.
12. `README.md` нового модуля содержит: назначение, правила импорта, «как написать свой клиентский адаптер»,
    секцию Versioning **и честный абзац о том, что `clients/shared` в модуль не входит** (§12.4).

---

## 12.9. Риски

1. **Ширина изменения — 294 файла с правкой импорта.** Самый широкий blast radius среди всех частей (шире
   Part 5 по числу файлов, хотя уже по строкам). Конфликты с `origin/main` при таком диффе почти
   гарантированы, если PR провисит.
   Смягчение: PR 12b генерируется скриптом за один проход и мерджится в тот же день; сразу после мерджа —
   `git merge origin/main` (§5.2); перед стартом — снимок `git log --oneline origin/main` по затрагиваемым
   каталогам. Уровень: **средний**.
2. **12a конфликтует с `main` по `activator.go`.** Файл менялся в `main` 23 раза за 60 дней; 12a трогает его
   и все четыре `*_native.go`. Смягчение — жёсткое правило B (§12.3): мердж в день открытия + немедленный
   `git merge origin/main`. Уровень: **средний**. *(В черновике этот риск не был выделен: правило «в тот же
   день» относилось только к 12b.)*
3. **A-1/A-2/ребро 3 не закрыты → цикл модулей.** Правило A «12b не начинается, пока 12a не смёржен»
   (§12.3). Уровень: **низкий при соблюдении правила, критический при нарушении**.
4. **Тесты контракта молча выпадают из Required и Coverage.** Пять независимых CI-поверхностей (§12.5), и ни
   одна не падает «сама» — все провалы тихие. Ловится критерием приёмки 5 (счётчик `Test*`) и явными
   правками из §12.5. Уровень: **средний** (черновик оценивал как низкий, учитывая только `test-core`;
   после разбора поверхностей стало ясно, что их пять, а `./...` не покрывает workspace-модули).
5. **Тестовые импорты как скрытый источник `require`.** Находка 4 показала, что `go list -deps` без флагов
   не видит этот класс рёбер. Смягчение: критерий приёмки 2 (вторая команда) + guard-тест §12.1.G, который
   сканирует `_test.go`. Уровень: **низкий после правки, был высокий до её обнаружения**.
6. **Baseline shrink-only ломается на переименовании** (VERIFIED: 2 записи, строки 674 и 677). Ловится CI
   сразу, чинится allow-list'ом. Уровень: **низкий**.
7. **Два пакета с семантикой «nativeconfig» плюс алиас-блок.** После переименования реализации в
   `nativeconfigos` и добавления одноимённых алиасов у части типов существует **два** валидных публичных
   имени (`nativeconfig.Request` и `nativeconfigos.Request`), а имя `Kernel` означает интерфейс в одном
   пакете и структуру в другом. Смягчение: godoc-заголовки обоих пакетов с явной перекрёстной ссылкой и
   указанием, какое имя канонично; абзац в `docs/ARCHITECTURE.md`; алиас-файл называется
   `contract_aliases.go` и содержит комментарий «не использовать снаружи пакета». Уровень: **низкий-средний**
   (черновик оценивал как низкий, но алиас-блок — новая сущность, добавляющая двусмысленность в обмен на
   193 строки).
8. **Неверная версия Go в `go.mod`.** Скопировать `go 1.25.0` — значит обнулить смысл части для внешних
   потребителей, и это пройдёт весь CI зелёным. Ловится только ревью. Смягчение: бисект-процедура §12.3 как
   обязательный шаг чек-листа 12b. Уровень: **средний** (тихий провал цели при зелёном CI).
9. **Дрейф плана относительно реального HEAD.** Раздел написан после Part 4; Part 5-9 наполняют
   `clients/<id>` кодом. Сами `clients/<id>` не выносятся, но если в ходе Part 5-9 в **контрактные** пакеты
   просочится новый импорт (например, в `clients` верхнего уровня добавят зависимость на `pathcontract` или
   `managedstdio`), §12.1(c) устареет.
   Смягчение: **guard-тест §12.1.G, добавляемый сейчас** — расхождение ловится на вносящем PR; плюс
   обязательное предусловие старта (обе команды `go list` из §12.1). Уровень: **низкий после guard-теста**
   (был средний).
10. **`require`/`replace` у потребителей.** Root-модуль сегодня вообще не имеет `require` на
    `install/integrationctl` и полагается на `go.work` (VERIFIED: root `go.mod` состоит из трёх строк).
    Скорее всего, для него ничего добавлять не придётся, но это надо подтвердить компиляцией `repotests`, а
    не предположением. Для `cli` правка нужна точно (§12.7). Уровень: **низкий**.
11. **Релизные workflow.** `agentplugins-release.yml` перечисляет модули явно (строка 124) — правка описана
    в §12.5(d). `agentplugins-npm-publish.yml` и `agentplugins-platform-proof.yml` не смотрел (§12.11).
    Уровень: **низкий**.

---

## 12.10. Надёжность и уверенность

Оценки снижены относительно черновика по итогам критики (правка №12).

- **Надёжность плана Part 12 (код компилируется, тесты зелёные, поведение не меняется): 7/10** *(было 8)*.
  Часть по-прежнему механическая, и все содержательные правки (DTO-split, alias, интерфейс в `Env` и в
  `Activator`, переезд теста) малы и проверяются компилятором. Минус три балла:
  (i) черновик занизил объём 12a в 2.5 раза на статье внутренней квалификации — ошибка того же класса может
  повториться на другой статье;
  (ii) два тихих CI-провала (Required и Coverage) не диагностируются падением, только ревью;
  (iii) план написан на срезе Part 4, а исполняться будет после Part 11.
- **Уверенность, что цель достигнута (модульность выше конкурента, «go.mod без require» реально получится):
  7/10** *(было 8)*.
  Развязка посчитана по факту: рёбер три, каждое локализовано, `domain` чист в non-test коде. Минус три
  балла: Находка 1 (постановка задачи содержала неверную гипотезу про «просто узкий интерфейс»), Находка 4
  (третье ребро нашлось уже после критики — значит класс «скрытых рёбер» разведан не полностью), и оговорка
  §12.4 про `clients/shared`, из-за которой цель «выше конкурента» достигается на краю погрешности, а не с
  запасом.
- **Уверенность в оценке объёма (±25 %): 7/10** *(было 8)*. Числа получены подсчётом по репозиторию, но
  одна статья уже была занижена в 2.5 раза, и это откалибровало доверие к остальным.

---

## 12.11. Что не проверял / где не уверен

- Не проверял эмпирически, что Go отказывается резолвить пересекающиеся префиксы модулей (§12.3) — вывод из
  правил резолва; проверяется первой компиляцией.
- Не выполнял бисект версии Go (§12.3) — рекомендация `go 1.22` выведена из набора используемых языковых
  возможностей и из границы смены loop-семантики, а не из прогона.
- Не проверял, потребуется ли `require`/`replace` в root `go.mod` для `repotests` (сейчас root не требует
  даже `install/integrationctl`).
- Не читал `ports/contracttest/pathpolicy.go` и `clients/contracttest/*` построчно — оценка их пригодности к
  переезду сделана по спискам `Imports`/`TestImports`/`XTestImports` (только `ports`/`clients`/`domain`/stdlib).
- Не проверял, какие записи появятся в LEGACY SIZE BASELINE после Part 5-11 для файлов контракта (сейчас их
  две, строки 674 и 677).
- Не оценивал взаимодействие Part 12 с `agentplugins-npm-publish.yml` и `agentplugins-platform-proof.yml`
  (`agentplugins-release.yml` разобран в §12.5(d)).
- Не проверял, как `scripts/check-generated-sync.sh` и `generated-check` реагируют на появление шестого
  модуля.
- Не проверял, нет ли ещё рёбер класса Находки 4 в подпакетах, которые появятся в Part 5-9 — именно поэтому
  guard-тест §12.1.G добавляется сейчас, а не в Part 12.
- Оценки конкурента AgentBridge по модульности (8/10) и среднего 7.4 взяты из аудита пользователя, код
  конкурента не смотрел.

---

## Приложение. Ключевые файлы для реализатора и критика

Пути репозиторно-относительные; worktree черновика —
`/Users/belief/dev/projects/_worktrees/uap-installer-core-refactor`.

| Файл | Зачем |
|---|---|
| `install/integrationctl/agentplugins/ports/runner.go` | единственное место ребра A-1 (строка 8 импорта, 4 сигнатуры) |
| `install/integrationctl/agentplugins/clients/clients.go` | место ребра A-2 (строка 22 импорта, поле `Env.NativeConfig`); строки 13-16 — проза исключения |
| `install/integrationctl/agentplugins/domain/identity_portable_test.go` | **ребро 3 (Находка 4)**: строка 6 импорта, вызовы на 28, 48, 51, 54 |
| `install/integrationctl/agentplugins/adapters/nativeconfig/types.go` | файл, который режется на DTO (строки 13-40 и далее) и реализацию |
| `install/integrationctl/agentplugins/adapters/nativeconfig/{kernel,document,project}.go` | файлы, которым нужен `hujson` — остаются реализацией |
| `install/integrationctl/agentplugins/adapters/nativeconfig/io.go` | единственное место использования `atomicfile` (строка 39) |
| `install/integrationctl/agentplugins/providers/activator.go` | строки 28-36: поле `NativeConfig *nativeconfig.Kernel` и `nativeConfigKernel()`; самый горячий файл проекта |
| `install/integrationctl/agentplugins/providers/opencode_native_test.go` | строки 313, 347, 366, 381 — `&committedKernel` |
| `install/integrationctl/agentplugins/usecase/manual_remote_lifecycle_test.go` | строка 241 — `&kernel` |
| `install/integrationctl/ports/interfaces_runtime.go` | строки 39-52: `Command`/`CommandResult` под alias |
| `install/integrationctl/agentplugins/internal/archtest/archtest.go` | строки 19, 24-25, 33-35, 104, 164, 167 — захардкоженные пути; сюда же ложится guard-тест §12.1.G |
| `install/integrationctl/agentplugins/clients/shared/` | пакет, который в модуль НЕ уезжает — основание оговорки §12.4 |
| `.golangci.yml` | depguard-правила (строки ~115-190); baseline-записи на строках 674 и 677 |
| `Makefile` | строки 13 (`LINT_MODULES`), 47-51 (`test-core`), 57-64 (`test-required`), 206-211 (`vet`) |
| `.github/workflows/lint.yml` | матрица модулей, строки 18-30 |
| `.github/workflows/core-fast.yml` | paths-фильтр (строки 9-14), `cross-build` (~103), `vet-all` (~125) |
| `.github/workflows/coverage.yml` | сбор профилей (строки 29-38), `files:` в шаге Codecov |
| `.github/workflows/agentplugins-release.yml` | строка 124 — тесты движка при релизе |
| `.github/workflows/codeql.yml` | строки 43-67 — поимённая сборка модулей |
| `.github/workflows/govulncheck.yml` | строки 11-25 — матрица модулей |
| `go.work` | список `use` (5 → 6) |
| `cli/go.mod` | образец конвенции `require v0.0.0` + `replace`; получает новую запись |
| `docs/plans/installer-core-clean-architecture-plan.md` | §8.7 — требование к Part 7c (не дублировать здесь); §5.2 — правило синхронизации |
