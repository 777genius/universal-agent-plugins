# UAP dependencies: план MVP

Статус: план v7 после семи раундов ревью/исправлений, 2026-10-15; реализация продолжается Codex по временному указанию владельца после сбоя MiMo runtime; итоговые проверки и доставка ещё не подтверждены.
Решение владельца: сейчас глобальная установка текущего пользователя; локальную установку предусмотреть архитектурно, реализовать следующим этапом.
Основа: `main@cc42681db647b0db1983fa36714519001043d083`. Локальный checkout `6dc79a8` старее, содержит чужие изменения и ещё использует `cli/plugin-kit-ai`; новые точки расширения сверены с main, где CLI находится в `cli` и опубликован `agentplugins/installer`.

## Результат и границы

Пользователь описывает плагины и целевых агентов в `uap.toml`, получает точные источники в `uap.lock` и воспроизводит тот же набор на другой машине через существующий Go SDK UAP.

**В MVP:** один пользовательский набор, стандартные пакеты `plugin.json` + MCP/skills, прямые GitHub-зависимости по commit/ref и точные публикации существующего подписанного directory, общий проверяемый store, синхронизация/обновление/удаление собственных привязок, обнаружение внешних изменений. Использовать уже опубликованные installer-адаптеры: сейчас стандартный состав Claude/Codex. Поддержку других SDK-адаптеров проверять по их объявленным возможностям, не расширять её этим планом. Для pinning автору пакета не нужна новая version metadata; произвольный layout репозитория этим не становится поддерживаемым форматом.

**Вне MVP:** проектная активация, именованные окружения, транзитивные зависимости и SemVer solver, OCI/произвольные Git-хосты/private Git authentication, локальные editable sources, install hooks, bundles для offline-переноса, автоматическая очистка store, автоматический импорт имеющихся установок, compatibility import новых native-only/YAML пакетов и программная правка существующего TOML. Полный pin runtime, моделей и внешних сервисов не обещается. Полезные legacy YAML возможности сохраняются вне standard graph; новых YAML fallback нет.

Перенос manifest/lock воспроизводит заявленные плагины при доступных источниках или заполненном store. Секреты, локальные пути профилей и PLUGIN_DATA не переносятся. Другие вручную установленные плагины агента не становятся частью этого набора.

## Файлы и пользовательский сценарий

Глобальные файлы: `<AGENTPLUGINS_HOME>/uap.toml` и `uap.lock`; использовать текущий resolver каталога UAP, без нового XDG resolver. Store и локальный state остаются под тем же пользовательским root. В будущем проект получит такую же пару файлов в своём root и собственные receipts; содержимое store общее.

Пример предложения схемы; URL и release иллюстративные:

```toml
schema_version = 1

[dependencies.review]
git = "https://github.com/acme/agent-tools.git"
path = "plugins/review"
ref = "main"

[dependencies.docs]
registry = "uap"
distribution = "acme/docs"
release = 42

[targets]
codex = ["review", "docs"]
claude = ["review"]
```

Git source принимает `commit` (полный lowercase 40 SHA) **или** `ref`, никогда оба. `path` означает относительную папку пакета внутри репозитория, по умолчанию root; local filesystem dependency им не задаётся. Короткий ref разрешён только при единственном совпадении ветки/тега; `refs/heads/...` и `refs/tags/...` снимают неоднозначность. Annotated tag разрешается до commit object. Теги не интерпретируются как SemVer. Если selector не задан, manifest intent содержит стабильный `default` marker; первый lock сохраняет фактически разрешённую default branch отдельно. Неизменный selector при lock/sync сохраняет commit без ref lookup; только `update <alias>` разрешает ref (для default marker - текущую default branch) заново. Смена selector инвалидирует pin только этого alias. TOML автоматически не переписывается.

Resolver валидирует ref средствами Git, передаёт аргументы без shell interpolation, сравнивает точные ref names и использует текущие context/timeout/output limits Runner. Git revision expressions и выбор по glob не поддерживаются. Stable source key не включает revision; lock повторного fetch требует совпадения и SHA, и tree digest.

Registry source содержит canonical distribution ID и точный release sequence из directory, не произвольную upstream version. Короткие registry names разрешаются CLI до canonical ID и при неоднозначности отклоняются. В текущем `DirectoryResolveRequest` exact desired selector отсутствует: добавить узкий выбор sequence с обычной eligibility нового install, без перехода на другую release. `RecordedDirectoryRelease` заполнять только из действительного installed state; lock не даёт исключений для suspended/superseded release.

`uap.lock` - генерируемый JSON: `lock_version`, семантический digest manifest, canonical source каждого alias (`git/path/commit`, requested ref при наличии), canonical directory publication identity, `agentplugins-tree-sha256-v1` tree digest и выбранные targets. Ни абсолютных путей, ни timestamps, ни секретов. Digest manifest = SHA-256 нормализованного intent schema v1: canonical source key/path, вид и значение selector (`commit`, `ref`, `default`, `release`), sorted aliases/target IDs и assignments. Resolved commit/tree/default-branch name не входят в intent digest, кроме commit/release, явно заданных как selector. Порядок TOML, комментарии и пробелы digest не меняют. Совместимость pin проверяется по source+selector alias: одно изменение targets не продвигает revision. При registry source сохранять точные GitHub coordinates и directory identity/provenance, уже доступные в домене UAP.

Добавить тонкую группу команд `agentplugins deps`:

| Команда | Контракт |
| --- | --- |
| `init` | Создать небольшой manifest только если файла нет; существующий файл не переписывать |
| `lock` | Разрешить новые/изменённые зависимости, сохранить совместимые pins остальных; записать lock без установки |
| `sync` | Проверить manifest/lock, при необходимости выполнить lock; затем привести только собственные bindings к lock |
| `sync --locked` | Отсутствующий/устаревший lock - ошибка; никаких ref lookup или записи lock, immutable fetch по lock разрешён |
| `update <alias>` | Явно перепроверить только ref этого alias, записать новый lock; установка последующим sync |
| `status` | Без сети и мутаций сравнить lock, receipts и фактические package/projection digest; показать drift/partial |
| `recover` | Явно fresh Inspect -> scope/actions -> confirmation -> SDK Recover с typed observation; после успешного recovery отдельный sync |

Точные `commit` и registry `release` не обновляются этой командой: новый pin меняется в TOML, затем `lock` и `sync`. Удаление: убрать alias или target assignment из TOML, выполнить lock/sync. Неизвестные поля/версии схем, неоднозначные sources, отсутствующие aliases/targets и unsafe paths возвращают ошибку с местом в файле до эффектов. Для deps manifest - единственная target authority: inherited `--target` явно отклоняется; `--scope project` отклоняется до acquisition/записей. CLI читает manifest целиком существующим `go-toml`; не создаёт самописный TOML parser/editor. Редактирование вручную/агентом сохраняет комментарии; будущий `add --save` потребует отдельно квалифицированной библиотеки редактора.

Offline `status` получает из расширенного typed read-only SDK inspection live binding/owner и per-client revision; projection проверяется свежим существующим `Stager.Verify`, store - package digest. Manifest/lock agreement, receipt/live binding, store и projection - отдельные факты: различать `not_synced`, `partial`, `store_missing`, `store_mismatch`, `projection_missing`, `projection_mismatch`, `foreign` и pending recovery. Возможны несколько фактов одновременно. Отсутствие store не означает drift целого projection; receipt без проверки bytes не доказывает current. Status не делает lookup и не обещает актуальность moving ref.

`--dry-run` разрешает network/read-only проверки и временные snapshots; candidate lock/действия остаются в памяти. Не писать manifest/lock, final store, state/receipts, профили и persistent scanner cache; temp/cache scanner направить в собственный временный root. Не брать mutation locks и не выполнять Apply/Recover. Preview не создаёт authorization для последующего запуска: реальный запуск заново читает state, checks и consent. Отмена до первого durable effect не меняет установку.

Существующие imperative `add/update/remove` сохраняются. Автоматического adoption нет: найденная установка без environment owner считается конфликтом, а не разрешением захватить её. Для environment-owned bindings расширить mutation request typed environment authority (environment ID + ожидаемый receipt/desired action); Prepare копирует её. Guard принадлежит общей service/transaction границе под существующим mutation lock и покрывает как SDK, так и старые CLI lifecycle пути. Update/remove без owner authority отклоняются; repair допустим только для recorded pin и после явного решения пользователя о сохранении/восстановлении правок. Существующий CLI repair передаёт ожидаемую owner authority выбранного live binding только для этой exact-pin операции; SDK callers также передают typed authority. Это не разрешает nil-owner bypass, ref lookup/update, adoption или repair при foreign/indeterminate pending recovery. Для агентов использовать уже принятые `--format json`, `--dry-run` и confirmation policy CLI; вывод содержит выбранный scope и причины ошибки. JSON preview не является replayable Apply authority.

## Архитектура по Q

Один небольшой публичный пакет `agentplugins/environment` в существующем Go SDK module содержит модели requirements/lock, правила сравнения и coordinator. CLI занимается файлами, профилями и выводом; domain/usecase не импортируют TOML, Cobra или Git subprocess. Конкретные adapters собираются в текущем composition root. Новый daemon, сервис, БД и универсальная система providers не нужны.

Стек MVP: Go SDK, существующий `pelletier/go-toml/v2`, `encoding/json`, Git Runner и SDK storage/transaction primitives. Не добавлять Rust/Python runtime или новый downloader. PubGrub сейчас не нужен; при реальных транзитивных constraints первым проверить `posit-dev/go-pubgrub` за resolution boundary, не менять installer/store ради solver.

| Ответственность | Повторно использовать / минимально расширить |
| --- | --- |
| Source resolution | Signed directory resolver; branch/tag -> full SHA через существующий изолированный Git Runner. Первый direct pin: `AcquireGitHub` и вычисленный digest; повторный fetch: `AcquireGitHubVerified`; directory expected digest берётся из проверенной publication |
| Integrity | `packagedigest` и его canonical algorithm; не смешивать source digest с ArtifactDigest/projection digest |
| Package storage | Store snapshots по algorithm+tree digest, source coordinates отдельно; сохранять логический executable inventory вместе с bytes и проверять digest с ним |
| Install/update/remove | `installer.Prepare/Apply/Inspect/Recover`; сохранить typed handles, per-binding digests и отдельные activation/authentication/verification outcomes |
| Ownership/state | Environment receipts в существующем `statev2`/transaction lifecycle, не параллельный install ledger |
| Files/processes | Существующие atomicfile, dirswap journal, processlock, pathpolicy и contained runner |

Высокоуровневые контракты: `Resolve(requirement, previousPin, updateSelection) -> LockedPackage`; `PrepareSync(requirements, lock, observedBindings) -> typed prepared plan`; `ApplySync(plan, decision) -> per-binding outcomes`. Внешние вызовы проходят через небольшие consumer-owned ports только там, где защищают policy/тестовую границу. Не дублировать уже имеющиеся SDK DTO без различия смысла.

Расширить публичный Request одним узким входом приобретённого snapshot: проверенные `domain.SourceIdentity`, digest algorithm и executable paths; для directory также текущие `OriginMode`, `DirectoryOrigin` и compatibility/security facts существующего AddInput/envelope. Prepare копирует и валидирует их, повторно sealing/verify bytes; usecase/transaction сохраняют provenance вместе с installation. `PackageRoot` - bytes, не upstream identity. Для этого входа не применять local `reuseMatchingSourceIdentity`, переписывающий новую revision старой; stable binding identity сохраняется отдельно, а commit/tree и per-client revision берутся из нового проверенного snapshot. Environment receipts не дублируют source identity. В MVP выполнять single-target Prepare/Apply: передавать `ExecutableFiles`, включая non-nil пустой список; групповой Request сейчас этот параметр не поддерживает. Для существующего sibling передавать фактические `KnownTargets`; его digest не заменяется новой общей версией.

Переиспользовать текущий CLI assessment для точного digest, сохранив исходный scanner verdict. При допустимом явном risk consent CLI формирует отдельную digest-bound host authorization через существующий `installer.Assessment`, как current facade; это не изменение scanner verdict. Scanner failure/unavailable флагом не обходится; cached signed evidence/local scan следуют текущей policy. Lock/integrity/filesystem confirmation не являются content verdict, store hit не даёт автоматического allow. Не включать `TrustedLocalPackages` для скачанных пакетов; разрешение не записывать в lock/store и не переносить из dry-run.

Environment identity и installation/binding identity различны. MVP использует `user-default`; receipt связывает environment+alias+target с существующими installation/binding IDs, package digest и profile authority. Физическая installation не выбирается по одному alias. Future project scope получает другой environment ID, но scope-independent source/store и тот же installer lifecycle; сейчас `project` отклоняется явно. Общий store экономит bytes, не доказывает namespace isolation или приоритет локальной версии.

Canonical source key = GitHub repository+package path либо directory distribution ID; commit/release меняет revision, не этот key. Существующий binding выбирается только своим receipt. Новый target того же alias использует installation ID из его живого receipt; KnownTargets проверяет всех живых siblings. Смена canonical source и alias rename в MVP возвращают `source_change`: сначала убрать прежнюю запись и sync, затем добавить новую. Дублирующие aliases одного source и пересекающиеся plugin/MCP/skill namespace claims отклоняются до эффектов; source equality не разрешает захват unmanaged installation.

Store хранит неизменяемые оригиналы; agent projections и PLUGIN_DATA отдельны. Начать с обычных копий, без writable hardlinks в store. Перед повторным использованием проверять digest. Неполный/повреждённый entry не применять; восстановление только тех же pinned bytes. Remove не удаляет store и PLUGIN_DATA, не затрагивает иного владельца. Автоматический GC отложен.

Публикация entry: job-owned staging на файловой системе store -> проверка bytes+executable inventory -> атомарный перенос полного entry в final digest path. Короткий store processlock существующей библиотеки сериализует publication/repair; уже существующий destination проверяется и переиспользуется. Readers не читают staging; package перед Prepare повторно sealing/verify, поэтому внешняя правка не подменяет подтверждённые bytes. Store lock освобождается до SDK Apply.

## Порядок и ошибки

1. Прочитать и валидировать manifest/lock; наблюдать state без мутаций. При pending installer recovery показать существующий next action, не запускать recovery скрыто.
2. Для lock/update разрешить только изменённые требования, получить/проверить bytes и загрузить каждую новую ревизию существующим standard loader до замены lock. Native-only/YAML/malformed package - format error, без compatibility fallback. Атомарно записать deterministic lock после проверки, что manifest не изменился. При source/network/format failure старый lock сохраняется. Подписанная directory identity и текущая eligibility/security policy перепроверяются без выбора другого release; lock не выдаёт себя за signature или recorded-install proof.
3. Для sync выполнить общий read-only preflight изменений: namespaces, ownership, profile facts и drift до первого Apply. Включить удаления лишь для прежних environment receipts, отсутствующих в новом desired set.
4. После решения по bindings в стабильном порядке: свежий single-target Prepare с текущими receipts/KnownTargets -> сверка с одобренным desired action -> Apply. Не переиспользовать handles, ставшие stale после собственного предыдущего commit. Изменившийся смысл действия требует reprepare/нового решения, а не автоматического расширения подтверждения. Receipts фиксируются с managed commit; перед эффектами проверяются manifest/lock identity и SDK prepared authority.
5. Набор не является общей ACID-транзакцией нескольких агентов. При частичном результате остановить новые эффекты и показать committed/remaining/next actions. Receipt+digest подтверждают materialization, не activation/authentication/verification: terminal success определяется существующим SDK outcome. Retry продолжает pending lifecycle; remove authority сохраняется до terminal removal, включая recovery. Не пересоздавать завершённые bindings и не пропускать pending по совпадению digest. Environment association и removal authority сохраняются до terminal SDK removal; при terminal state commit association удаляется атомарно вместе с завершением lifecycle. Partial retry удаляет только оставшиеся targets. Удалённая association не служит adoption authority и не блокирует последующее новое добавление alias/source. Использовать существующие journal/receipt phases; отдельные desired/pending/committed/removed phases или tombstones в MVP не вводить.

Использовать отдельный environment processlock для mutating lock/update/sync/recover, реализованный существующей библиотекой; mutation.lock продолжает принадлежать installer. Не брать mutation.lock снаружи Apply: это вызвало бы повторный захват. Ownership/digest guard и environment receipts принадлежат текущей transaction, не позднему CLI callback; owner authority попадает в recovery receipt/journal до первого managed effect. Recover использует полный fresh typed Inspection и не принимает authority из JSON. До recovery preparation/effects и повторно под mutation lock перед Kernel/native callbacks проверить owner каждого pending journal, state receipt и native intent. Foreign/indeterminate элемент отклоняет весь recovery как `recovery_required` с существующим next action: не фильтровать Inspection и не восстанавливать owned subset. Новый scoped recovery engine не нужен.

Ownership/receipts вводят state schema 5. Расширить существующие strict reader и `PlanCurrentV2`/`MigrateCurrentV2`: explicit dry-run -> digest-checked apply под mutation lock, backup и atomic save для 4 -> 5; сохранить прежние 2/3 migration/read-only пути и legacy 0/1 importer. До explicit migration старый state не допускает мутаций; все старые записи остаются unmanaged, owners не выводятся из installation/scope. Старые binary/writer обязаны fail-closed для schema 5.

Migration preflight не вызывает обычный `Kernel.Recover` через Store, нормализующий старую схему в 5. В MVP pending journal/receipt/native intent у schema 2/3/4 означает refusal до backup и любых state/journal effects; показать восстановление соответствующим прежним SDK/binary, затем новый dry-run/apply migration. Не направлять пользователя в цикл «сначала migrate, затем recover» тем же несовместимым writer. После read-only проверки отсутствия pending под mutation lock повторно сверить digest, сохранить исходные bytes в backup и атомарно записать schema 5. Отдельный schema-preserving recovery engine/adapter ради этого MVP не вводить.

При внешней правке управляемого package, projection или принадлежащей UAP config entry: `drift`, без автоматической перезаписи. Проверять added/missing files, не mtime. Изменения чужих config keys не считаются drift UAP объекта. Пользователь может сохранить правки и явно восстановить pinned package существующим repair; изменённые bytes не получают исходный upstream commit. Process locks не блокируют произвольный редактор, поэтому SDK revalidation сохраняется.

## Как выполняем без затяжной подготовки

- Подготовка завершена: четыре дополнительных ревью выполнены, реализация уже идёт. Новые общие ревью плана не требуются. Открывать решение заново только при конкретном противоречии в коде, провале проверки или изменении требований владельца; исправлять затронутый пункт, не перезапускать подготовку целиком.
- Использовать существующий SDK и библиотеки. Не добавлять новую абстракцию, зависимость или механизм «на будущее», если текущий сценарий обходится существующим. Неясные обратимые детали решает исполнитель в рамках этого плана.
- После реализации: сборка и указанные focused tests, один реальный сценарий CLI/SDK в disposable roots, одно независимое ревью точного diff и штатный required CI. Закрытые проверки повторять только после затрагивающих их изменений; обязательный CI на новом head сохраняется. Новый раунд ревью нужен для конкретных исправлений/изменений, а не для очередного подтверждения того же плана.
- Инфраструктурный сбой разбирать отдельно от продуктовой задачи: сохранять job ID и готовые результаты, чинить конкретный сбой и продолжать с места остановки. Истечение ожидания само по себе не означает остановку или повторный запуск worker. Не обещать срок и не считать отсутствие результата доказательством успеха.

## Реализация: три связанных шага

1. **Requirements и lock:** публичные типы/валидация, TOML adapter, Git ref resolver, directory exact pins, deterministic JSON, init/lock/update/status. Все refs и digests заимствуются из существующих source/directory контрактов; никаких resolver algorithms.
2. **Store и sync:** verified package reuse, минимальные environment receipts внутри state lifecycle, preflight/drift/ownership, public SDK PrepareSync/ApplySync и тонкий CLI. Никакого второго installer и новых native support claims.
3. **Законченный сценарий:** короткая документация, перенос пары файлов на чистый пользовательский root, repeat/no-change, targeted update, deletion и безопасный partial retry. Мержить связанными checkpoint, а не выделять половину ownership-инварианта ради размера PR.

## Достаточные проверки и приёмка

Один table-driven contract набор для source selectors/lock agreement: движущийся branch, annotated tag, ambiguity, точный release, target change, комментарии, broken source/format. Один filesystem integration набор для store tampering, installed drift, удаления только owned binding, stale plan/partial retry через явный recover, dry-run без durable writes и executable inventory. Один SDK/CLI сценарий в disposable профилях: lock -> sync -> repeat -> targeted update -> remove -> restore на втором root; scanner unavailable и owner/target guards проверить на этой же сильной границе. Добавить в эти же наборы mixed owned+foreign recovery (ни один journal/receipt не меняется), owned-only recovery и schema 4 -> 5 migration/round-trip без автоматического ownership; un-migrated и старые writer остаются fail-closed. В этих же наборах проверить selector/default digest, отсутствие lookup для unchanged ref и target-only change, независимые store/projection failures и removal retry/re-add. Проверить pending state_committed/rolled_back fixtures schema 2/3/4: migration refusal не меняет bytes/schema/journals, после соответствующего recovery повторный migration не зацикливается и backup содержит исходные bytes этого apply. В реальный CLI сценарий включить exact-pin managed drift -> подтверждённый существующий repair -> current, без ref lookup/update/adoption; foreign pending блокирует repair. Не дублировать один сценарий unit-тестами каждого слоя.

Команды после реализации, из соответствующих модулей актуального main:

```sh
# install/integrationctl/agentplugins
go test ./environment ./adapters/sourceacquisition ./adapters/packagedigest ./adapters/statev2 ./installer
# cli
go test ./internal/agentpluginscli ./cmd/agentplugins
```

Новые проверки критичных переходов включаются в эти пакеты, затем штатный required CI на exact head. Реальные пользовательские профили не использовать. Реализация выполняется; успешная сборка и реальный продуктовый E2E пока не подтверждены.

Готово, когда перенос manifest/lock даёт те же source digests/target assignments; repeat не делает ref lookup или повторную загрузку intact store; update одного ref не меняет остальные pins; locked mismatch и drift останавливаются до overwrite; partial outcome не выдаётся за success; удаление оставляет unmanaged installations, чужие config entries и PLUGIN_DATA; старые команды и standard authoring не приобретают YAML fallback.

Откат: revert тонкого CLI entrypoint и coordinator, сохранив state/store/PLUGIN_DATA и существующие recovery receipts. Не запускать старый writer на state schema, которую он не понимает; сохранить текущий SDK binary до завершения recovery. Автоматической очистки данных при rollback нет.

## Опыт проектов, который применяем

- [Cargo](https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html): intent отдельно от locked commit; [uv](https://docs.astral.sh/uv/concepts/projects/sync/): сохранить locked выбор и обновлять отдельную зависимость явно. Не копировать режим, игнорирующий расхождение manifest/lock.
- [npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci/): строгий mismatch error. Не копировать удаление всего каталога зависимостей в общие профили агентов.
- [pnpm store](https://pnpm.io/cli/store): shared bytes и проверка изменённого содержимого. UAP не использует writable links к единственному оригиналу.
- [APM drift detection](https://microsoft.github.io/apm/enterprise/drift-detection/): проверять drift **до** sync; install-before-audit скрывает прежние изменения. Source pin, deployed baseline и runtime data разделены.
- [Agentpack environments](https://github.com/OlegHQ/agentpack/blob/953a213491dfcce0520512b7d2d531e14eb321f3/docs/src/concepts/environments.md): portable definition отдельно от machine bindings. Не переносить широкое обещание изоляции на неподтверждённые adapters.
- [Qvr lock](https://github.com/try-agora/qvr/blob/71783910729031afe6ad1d640728b1534ee4c198/internal/model/lockfile.go) и [Skillshare pins](https://github.com/runkids/skillshare/blob/109466840801de1656a6324e666d5eeb8237b071/internal/install/lock.go): полные source coordinates и commit, разделение portable lock и локального состояния. Не копировать Qvr frozen, игнорирующий manifest; не объединять overwrite/drift и security consent в общий force.

## Раунды ревью

| Раунд | Найдено и исправлено |
| --- | --- |
| 1: scope/Q/reuse, v0 -> v1 | First-pin acquisition без expected digest; существующий assessment вместо trust cache; executable inventory/single-target SDK; сохранение upstream identity в transaction |
| 2: identity/state/сбои, v1 -> v2 | Receipt-based matching/source-change refusal; SDK owner authority; materialization отдельно от lifecycle success; atomic store publication; свежий Prepare после каждого commit |
| 3: UX/приёмка/простота, v2 -> v3 | Dry-run без durable writes; явный Recover через SDK; scanner verdict отдельно от host consent; standard loader до записи lock; manifest targets без CLI subset override |
| 4: дополнительный 1/4, v3 -> v4 | Acquired snapshot/provenance в SDK; исключение local identity reuse для новой revision; exact desired Directory selector без Recorded-state исключений |
| 5: дополнительный 2/4, v4 -> v5, MiMo Pro | All-or-nothing ownership полного recovery scope; обязательная schema 5 и расширение существующего migration пути |
| 6: дополнительный 3/4, v5 -> v6, MiMo Pro | Явный selector intent/pin reuse; независимые offline status facts и fresh projection verification; terminal removal association без новой state machine |
| 7: дополнительный 4/4, v6 -> v7, MiMo Pro | Read-only migration preflight/refusal для old-schema pending без раннего Save; проверяемый exact-pin repair с owner authority |

Первые три раунда выполнены последовательно локальным независимым read-only reviewer: на том этапе MiMo route отклонил source SHA (`mimo_project_bridge_source_mismatch`). Дополнительный первый раунд выполнен локальным GPT-6.1 в разрешённый владельцем период. Дополнительный второй завершён MiMo Pro / thinking / default на точном main SHA; основной агент независимо проверил замечания и исправил неточность отчёта: существующий in-place migrator уже поддерживает 2/3, а не только legacy 0/1. Снимки и review evidence сохранены вне репозитория. Четыре дополнительных последовательных раунда завершены, замечания независимо проверены и внесены; неуспешные/timeout attempts не засчитаны. План готов к реализации; реализация продолжается Codex по временному указанию владельца после сбоя MiMo runtime; итоговые проверки и доставка ещё не подтверждены.
