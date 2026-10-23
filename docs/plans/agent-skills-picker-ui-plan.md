# Agent Skills Picker: план браузерного интерфейса

Статус: начальная поставка реализована и локально проверена; доставка ожидает обязательных проверок PR. Дата: 2026-10-23.
Этот документ не подтверждает готовность кода: приёмка требует проверок ниже и checkpoint I installer-плана.
Уточнено по `/plan-improve` и Q: готовые engines отвечают за interaction mechanics,
frontend model — только за правила picker; ниже зафиксирован один набор зависимостей.
Связанный [installer-план](./agent-skills-installer-plan.md) задаёт канонический API и общую оценку.
Это отдельная frontend-дорожка начальной поставки installer; backend, discovery, безопасность,
установка и перенос Skills принадлежат installer-плану. После согласования API дорожки параллельны.
Исторический baseline исследования — `cbf20205ee56c8fa732ae594e015cbc9359ea217`.
Реализация начата в отдельном worktree от main `3ea24e3014ddf6679ca86729c56f1069112b0d3a`;
чужой dirty checkout `6dc79a8` не используется для кода.

## Цель и границы

Пользователь открывает временную страницу из Go CLI, находит вложенные Skills репозитория,
выбирает несколько из разных папок, читает preview, выбирает цель и подтверждает план установки.
Качество дерева, поиска, корзины, состояний и доступности — часть первой поставки.
Один CLI обслуживает loopback API и встроенные статические assets; Node нужен
только разработке/сборке. Нет удалённого frontend-сервиса, SSR или пользовательского npm runtime.
Новый `ui/skills-picker/` независим от Vue/Nuxt landing; его миграция и редизайн вне задачи.
Не писать frontend-парсер `uap.toml`, Git resolver или второй installer.
Сохранить implementation/тесты по [capability preservation](../AUTHORING_CAPABILITY_PRESERVATION.md);
UI не расширяет release qualification authoring и не меняет availability labels PR190.

## Обязательный стек и ответственность состояния

- React + TypeScript: компоненты, события и производные представления.
- TanStack Query: единственный cache server state — session, catalog, preview,
  targets, plan, operation. Query keys включают session и immutable snapshot identity.
- TanStack Router: проверенные URL-параметры `q`, `path`, `view=tree|results`,
  `skill`; Back/Forward восстанавливает browse/search/preview только в живой CLI-сессии.
- Zustand: только ephemeral IDs выбранных кандидатов (store feature selection) и
  раскрытых папок (store feature catalog), привязанные к catalog identity.
  Target/form/dialog state остаётся локально в feature.
  Не хранить копии DTO, поисковой строки, результатов, operation status или Query cache.
- Дерево, индексы, счётчики и поисковая выдача вычисляются из catalog + URL + IDs.
  Нет синхронизации производных массивов через effects.

## Готовые библиотеки: выбор и границы

Top 3 дерева; оценки относятся к точному UX picker, сложность выше = сложнее.
LOC — приблизительный production integration diff дерева/checkbox/virtualizer, без общего UI.

| Вариант | Пригодность и ограничение | Оценка / integration LOC |
| --- | --- | --- |
| **Headless Tree core/react 1.8.0 + TanStack Virtual 3.14.22 — выбран** | Controlled checked/expanded state, отдельные checks/focus, flat visible rows, configurable hotkeys; нужен небольшой adapter eligible descendants. MIT; React peer `*`, поэтому реальную React 19 совместимость подтверждает browser-flow. | 🎯 9/10 · 🛡️ 8/10 · 🧠 4/10; 300–500 LOC |
| **React Aria Components 1.22.1 Tree** | Apache-2.0, React 19 входит в peer range; сильные keyboard/action/controlled selection. Но checkbox selection не задаёт нужную policy tri-state всех eligible descendants; она остаётся нашей. Для flat-row virtualizer лучше документирован Headless adapter. Это не утверждение невозможности RAC virtualization. | 🎯 8/10 · 🛡️ 9/10 · 🧠 6/10; 450–700 LOC |
| **react-arborist 4.0.1** | MIT, React ≥18; встроенные virtualization/keyboard. Обязательные `react-dnd`/HTML5 backend peers, Redux/react-window и преимущественно imperative expansion увеличивают лишнюю связность для immutable picker; отдельная check-policy всё равно нужна. | 🎯 7/10 · 🛡️ 8/10 · 🧠 7/10; 500–800 LOC |

Основания: [Headless checks](https://headless-tree.lukasbach.com/features/checkboxes/),
[controlled state](https://headless-tree.lukasbach.com/guides/state/),
[flattened virtualization](https://headless-tree.lukasbach.com/recipe/virtualization/),
[React Aria Tree](https://react-aria.adobe.com/Tree), [Arborist API/peers](https://github.com/jameskerr/react-arborist).

Остальной выбранный набор: **`@base-ui/react` 1.9.0** (MIT, React 17/18/19) — Dialog,
Checkbox вне tree, Select/RadioGroup, Tabs, Tooltip, Progress; imports по компонентам,
один небольшой `shared/ui` со стилями. [Base UI](https://base-ui.com/react/overview/quick-start)
даёт unstyled primitives и focus management без своего overlay/checkbox engine.
React Aria primitives не добавляем поверх Base UI; [Radix](https://www.radix-ui.com/primitives/docs/overview/introduction)
также подходит, но второй набор не нужен. [shadcn/ui](https://ui.shadcn.com/docs) передаёт
код компонентов нам на сопровождение; здесь прямые Base UI + CSS Modules/tokens короче и прозрачнее.
**`lucide-react` 1.61.0** (ISC, React 19) — только named icon imports, без dynamic icon loader;
[standalone SVG icons](https://lucide.dev/guide/react) дополняют доступные подписи.
**`react-markdown` 10.1.0 + `remark-gfm` 4.0.1** (MIT, renderer React ≥18) — preview.
Других tree/UI engines, drag-and-drop, fuzzy search, theme framework, form framework
и одновременно Zod/Valibot не добавляем; API schema/codegen принадлежит общему backend-контракту.

Каталог ограничен сервером по scan policy installer-плана и загружается целиком один раз
после завершения scan; локальный поиск и раскрытие папок не вызывают API.
При превышении серверных лимитов `complete=false` с причиной;
пользователь может сузить scope. Валидированный кандидат доступен для выбора,
но eligibility всего artifact проверяется при review/preflight. Неполный scope не доказывает
отсутствие, уникальность или полноту глобального выбора.
Каталог immutable: `staleTime: Infinity`, без refetch на focus/reconnect;
обычная свежесть cache не доказывает актуальность целей установки.
После mutation инвалидировать targets/plan/installed state, а не пересканировать SHA.
Session/operation polling около 1 s включён только при scan/install в работе;
terminal state останавливает polling. SSE, WebSocket, worker и streaming index не нужны.

## Макет и визуальный контракт

Desktop от 1100 px: header, рабочая область в три колонки, нижняя action bar.
Слева 260–300 px — дерево папок; центр гибкий — кандидаты/результаты;
справа 320–400 px — preview, затем компактная корзина и target selector.
Доступные targets идут первыми; недоступные с причинами — в свёрнутом по умолчанию
доступном disclosure «Недоступные цели · N». Раскрытие клавиатурой показывает disabled controls
и причины, не выбирает цель. Каталог из десятка неподдержанных клиентов не должен
выталкивать review и полезное содержимое далеко за первый экран.
Header показывает repo, короткий SHA с доступным полным значением, scan completeness,
число Skills и поиск. Action bar показывает «Выбрано N» и «Проверить установку».
На 760–1099 px preview — drawer; ниже 760 px дерево/результаты — вкладки,
корзина — отдельная панель с постоянной кнопкой счётчика.
Нет горизонтального scroll всей страницы; длинный путь переносится в preview/корзине.

Визуальная основа: нейтральный фон, чёткие панели, один акцент выбора/действий,
warning/error tokens и системный шрифт.
Плотность строк 40–44 px, зона нажатия controls минимум 24 px, видимый focus ring;
отступы и состояния задаются небольшим набором CSS tokens. Иконка дополняет подпись.
Путь — обязательный второй ряд кандидата; одинаковые имена различимы по полному пути.
Preview показывает title, description, путь, источник, ограничения и SKILL.md: text/safe Markdown без raw HTML.
[react-markdown](https://github.com/remarkjs/react-markdown) работает с `skipHtml`, только `remark-gfm`,
без `rehype-raw` и plugins, исполняющих содержимое. Image renderer выводит alt/placeholder без `src`;
link renderer разрешает только абсолютные `https:`/`http:` с `noopener noreferrer` и `referrerPolicy=no-referrer`.
Relative, hash, `file:`, `data:`, `javascript:` и command links показываются текстом, без запросов к loopback;
raw source доступен как текст из `rawContent` по общему API. Markdown body в `content` уже отделён
backend от YAML frontmatter: frontend не разбирает исходный документ повторно.
При `format=text` использовать текстовый renderer; отсутствие rawContent не даёт обещания
показать полный исходник. Проверить preview на настоящем документе с frontmatter,
а не только на fixture, содержащем один Markdown body. GFM task checkboxes read-only; tables/code scroll внутри preview.

Первые визуальные fixtures: глубокое дерево, одинаковые имена, длинные Unicode-пути,
пустой/ошибочный description, 1/100/1 000/2 000 кандидатов, warning и disabled candidate.
Review на fixtures — до install mutation; приёмка включает bundled assets в тестовой loopback-сессии.

## Дерево, поиск и выбор

Дерево контейнеров выводится из repo-relative candidate paths, не всех файлов.
Сохраняет nested Skills и `.agents`/`.github`/`.curated`, без отбрасывания папок;
`path` фильтрует поддерево. Раскрытие не выбирает.
Действие папки «Выбрать все N в папке» включает eligible descendants независимо от
search/viewport; отдельное «Выбрать результаты N» действует на весь filtered result.
Mixed checkbox отражает всех потомков; неполный scope подписан «N найденных», без
обещания полноты. Headless Tree sync loader получает производный immutable index;
каждый Skill — leaf с candidate ID, контейнеры имеют отдельные namespace IDs. Если путь
содержит свой SKILL.md и вложенные Skills, его собственный leaf остаётся рядом с дочерними папками.
`checkedItems` controlled через selection callbacks, `expandedItems` — catalog Zustand;
focus остаётся transient state библиотеки, preview — Router, второго checked store нет.
Не передавать весь Zustand store в library state; setters поддерживают functional update.

Выбор глобален для полного текущего catalog, даже когда виден один subtree или результат поиска.
Adapter передаёт действие `setChecked(ids, checked)` через публичный selection callback:
добавить eligible IDs к текущему множеству либо удалить только указанные IDs.
Нельзя заменять корзину списком checked IDs видимой проекции. Index eligible descendants
строится по полному catalog; folder mixed state учитывает выбранных потомков вне viewport/search.
Catalog adapter не импортирует store соседней feature: `app` связывает callbacks и selected IDs.

Небольшой catalog adapter использует публичное расширение checkbox feature: eligible descendants
индексируются один раз на snapshot; checked/mixed и bulk действия учитывают только их.
Конкретная extension seam: после `checkboxesFeature` добавить feature override
`toggleCheckedState`/`getCheckedState`; они делегируют чистой eligible policy и selection callback.
Library enum/TreeInstance остаются в adapter, а policy не импортирует Headless Tree.
Это необходимо: встроенная propagation не фильтрует backend-disabled leaves. Disabled остаются
видимыми и focusable для preview, но не checkable; ноль eligible отключает checkbox папки.
Внутри tree использовать native checkbox props/ref библиотеки, вне tree — Base UI Checkbox;
не вкладывать checkbox в button, не навешивать два onChange и не менять выбор кликом строки.
Виртуализатор получает flat visible rows; stable item key — node ID, scrollToItem соединён
со scrollToIndex, refs tree/measurement объединены. Keyboard focus за viewport прокручивает строку.
Single-child chains не сжимать в MVP: меньше adapter-кода и однозначные keyboard/ARIA уровни.

Непустой поиск по умолчанию охватывает весь каталог и показывает плоскую выдачу.
Активное ограничение папкой включается явно chip «Внутри …»; оно удаляется отдельно.
Слова после trim/case-fold ищутся через AND в name/description/полном пути;
сначала exact name, затем prefix name, затем остальные совпадения; внутри групп
стабильная сортировка по пути. Fuzzy search и внешняя search-библиотека не нужны.
«Показать в дереве» сбрасывает `q`, задаёт `path`, раскрывает предков и сохраняет корзину.
Пустой запрос возвращает browse; «Нет Skills» и «Нет совпадений» различаются.
Изменение текста поиска заменяет текущую history entry; явная навигация добавляет entry.

Checkbox/Space меняет выбор; название/Enter открывает preview; focus/preview/выбор визуально различаются.
Выбор хранится по opaque candidate ID, не по имени.
Смена поиска, папки и вида сохраняет выбор из остальных папок; корзина показывает
полный состав, путь каждого элемента, remove и явное «Очистить выбор».
Disabled candidate виден с причиной и недоступным checkbox; он не попадает в bulk select.
Новая catalog identity требует повторного выбора без remap IDs по именам; перезапуск CLI — новая сессия.

## Подключение общего API v1

Канонические endpoints, DTO, статусы, ошибки и lifecycle описаны в §8
[installer-плана](./agent-skills-installer-plan.md); здесь повторно их не определять.
Контракт фиксируется исполнителями до независимых PR; это не дополнительный owner approval.
Единственный editable source — backend-owned `contracts/skills-picker/openapi.yaml`;
versioned fixtures лежат рядом с Go bridge. Схема порождает TS types через `openapi-typescript`;
`openapi-fetch` — тонкий transport в `shared/api`.
Query hooks остаются в features, без дополнительного `openapi-react-query` слоя.
Generated types не являются runtime validation: backend strict decode и identity/action guards,
frontend fail-closed при неизвестном action; общие fixtures проверяют wire contract без копии Zod schema.

Ключ preview включает session/catalog/candidate; Query hook использует AbortSignal.
Не показывать previous candidate как placeholder: поздний ответ старого catalog не меняет открытый preview.
`409 PLAN_STALE|CATALOG_MISMATCH` сбрасывает review, `410 SESSION_EXPIRED` завершает сессию.
Frontend не решает overwrite по совпадению имени и не конструирует filesystem paths целей.
Bootstrap до Router считывает capability из initial URL fragment, очищает его и сохраняет
только в origin/tab-scoped sessionStorage для refresh; localStorage и Router/query не используются.
`openapi-fetch` добавляет same-origin Authorization header; Host/Origin проверки принадлежат Go.
`410` очищает token и останавливает polling. Ассеты и API same-origin.

## Подтверждение, прогресс и восстановление

«Проверить установку» доступно при непустом валидированном выборе и допустимой цели.
Review показывает destination paths, итоговые действия, conflicts/warnings; confirm требует `confirmable=true`.
Смена выбора/цели сбрасывает план. Истёкший/устаревший план требует нового review.
Ошибка resolve/preflight одного выбранного Skill блокирует весь apply по §2 installer-плана;
корзина сохраняется, пользователь видит причину и может убрать проблемный элемент перед новым review.
UI не сообщает об установке оставшихся Skills и не меняет состав batch самостоятельно.
Ответ подготовки review принимается только для тех же catalog/selection/targets, с которыми
отправлен запрос; поздний ответ не восстанавливает сброшенный план. Это проверка текущей
identity входных данных, без копии server state в Zustand.
Во время mutation выбор и цель заблокированы; двойной клик не создаёт второй запрос.
Для уже принятого planId backend возвращает прежний operation до expiry/stale проверок;
для первого apply такие проверки обязательны. После аварии CLI действуют его receipts/recovery.
Два разных planId не обходят защиту: атомарный session gate допускает одну active operation;
другой получает `409 OPERATION_IN_PROGRESS`. Старые непринятые планы инвалидируются.
После потери ответа сначала читать session/operation и сопоставлять operation.planId с ожидаемым планом;
никаких автоматических повторов POST с новым planId и optimistic success; Query mutation retry выключен.
Если ожидаемая operation пока не найдена, показать неопределённый исход и явное действие
«Повторить подтверждение» с тем же planId по правилам §8 installer-плана. До восстановления
результата либо подтверждённого отказа старого плана не создавать новый plan для того же намерения.
Сбой polling показывает «Состояние неизвестно, переподключиться»; известный result сохраняется, включая partial.
Закрытие вкладки или abort fetch не объявляются отменой принятой операции; её lifecycle
принадлежит CLI session по §1.2 installer-плана. UI может перестать наблюдать и затем восстановить result,
но не показывает «Отменено» только из-за AbortError или unmount компонента.
При refresh live session восстанавливает последнюю принятую operation, включая уже завершённую,
корзина может быть сброшена. Новый явный review после terminal outcome получает свежие facts;
старый результат не объявляется результатом нового плана.

Relocation в начальной поставке принадлежит обычному update: если такой plan открыт в picker,
UI показывает sourceMoved, старый/новый путь, warnings и результат в review/progress.
Отдельный экран relocation и отдельная browser-команда обновления в MVP не требуются.
Не делать drag-and-drop файлов, browser filesystem access или frontend-миграцию.

## Feature slices и зависимости

```text
ui/skills-picker/
  src/app/                 # providers, Router, page composition, bootstrap
  src/features/catalog/    # ui/, api/, model/: tree, results, preview, expanded IDs
  src/features/selection/  # ui/, model/: basket и ephemeral selected IDs
  src/features/install/    # ui/, api/, model/: target, plan, confirm, progress
  src/shared/api/          # transport, errors, imports backend DTO/fixtures
  src/shared/ui/           # tokens, Button, Checkbox, Dialog, status primitives
  tests/browser/           # рискованные пользовательские сценарии на fixtures
  package.json, lockfile, build config, TypeScript config
```

Каждая feature имеет `index.ts` с небольшим публичным API. `app` соединяет features
через props/callbacks и публичные exports; features не импортируют внутренности соседей.
Selection принимает IDs и presentation inputs, не импортирует Query hook catalog.
Канонические backend fixtures: `cli/internal/agentpluginscli/skillspicker/testdata/api-v1/`.
Frontend test tooling читает эти файлы напрямую; generated DTO imports принадлежат `shared/api`.
Headless/Virtual imports локальны catalog adapter; Base UI/Lucide — shared UI; Markdown — catalog preview.
Чистые model функции не принимают TreeInstance, QueryClient или Zustand store.
Чистый model — дерево/поиск/выбор; api — Query/transport; ui — React. По Q достаточно этих границ:
без DI container, repository на DTO, глобального services layer и копий models.
HTTP client не реализует install-policy; shared UI — небольшие primitives.

## Состояния, доступность и проверки

Scan loading показывает фазу и отмену только если разрешена; пустой catalog — объяснение/diagnostics;
scan error — безопасную причину и допустимый retry. Preview error сохраняет корзину.
Partial scan явно помечен; поиск ограничен `scope`, с breadcrumb и явным переходом
к «Посмотреть весь репозиторий»/«Сузить сканирование» через явный POST catalog на том же SHA.
Это смена server catalog, а не локальный folder chip: при непустой корзине заранее показать её сброс.
С момента отправки явного rescan review и confirm недоступны; после принятия запроса сбросить
selection и preview route, проверить folder path относительно новой области. Даже при ошибке scan
не возвращать прежний confirmable plan. Ответы старых preview/review не меняют новый экран.
Обычные поиск, раскрытие и folder chip не делают запросов и не сбрасывают выбор.
Headless Tree отвечает за ARIA, focus/navigation и hotkeys; своего roving-focus engine нет.
Оставить Up/Down, Left/Right, Home/End и library type-ahead; Space через public hotkey config
меняет check, Enter открывает preview/папку. Отключить stock select-all/range/rename/DnD shortcuts:
они не должны менять basket в обход eligible policy. Type-ahead state библиотеки не равен URL-поиску.
Проверить focus и check отдельно, scroll к невидимой строке и folder mixed announcements.
Base UI Dialog возвращает focus через finalFocus; при unmounted virtualized trigger — на search.
Escape закрывает до mutation. Live region объявляет итог/ошибки/фазы без polling ticks.
Обязательны контраст WCAG AA, 200% zoom, reduced motion и клавиатура.

Минимальные meaningful tests: selection сохраняется между папками/поиском, bulk меняет только
нужные IDs и не протекает между snapshots; stale plan/смена target и поздний review блокируют confirm; API contract
отвергает неправильную identity и неизвестные действия. Не тестировать getters.
Browser flow: использовать доступные имена targets из API, а не зашитый список клиентов;
отдельный fixture с 2 доступными и 10 недоступными проверяет компактность и keyboard disclosure.
Выбрать одинаково названные Skills из разных папок, проверить пути
в корзине/preview, сменить цель, review → confirm → progress → terminal result;
отдельно stale/partial scan с поздним preview и потеря сети после confirm без второго эффекта,
включая быстро завершившуюся operation; nested own Skill
и folder bulk с disabled leaf проверяют именно наш adapter, а не повторяют library unit tests.
Preview fixture с raw HTML, command/file/relative links и image URL не выполняет запросов/кода.
На synthetic fixture 1 000 кандидатов проверить плавный поиск/раскрытие, bounded DOM
и отсутствие API вызовов на каждый ввод; 2 000 — верхняя проверка server bounds.
Один browser-проход keyboard/focus и responsive screenshots на fixtures ловит UX-риск;
не дублировать весь сценарий unit-тестами на каждом слое. Только disposable test repos.
Приёмка: typecheck, build, focused contract/browser checks и Go smoke bundled assets
без Node на пользовательском runtime; inspect diff независимым осмысленным проходом.

## Параллельная доставка и оценка

1. **Общий контракт:** DTO, bounds, identity, actions/relocation, session lifecycle и fixtures;
   точка K0 installer-плана. Готового wire contract достаточно для разработки UI на fixtures;
   ожидать готовности Git scanner или реальной установки для этого не нужно.
2. **UI lane A:** shell + catalog/tree/search/preview + basket на fixtures;
   ownership `src/app`, `features/catalog`, `features/selection`, `shared/ui` и общего
   HTTP transport в `shared/api`, без Go wiring. При необходимости два связных
   PR примерно до 2 000 changed LOC: browsing, затем selection/preview/a11y.
3. **UI lane B:** install review/progress и relocation presentation на согласованном
   контракте; ownership `features/install` и относящихся к ней проверок, без правки Go policy
   или shared schema. Hooks/models и экраны на fixtures можно готовить параллельно A;
   соединение с shell использует публичные props/callbacks, `app` wiring остаётся у A.
4. **Integration lane:** backend владеет embed/CLI/session; frontend владеет API adapter
   и проверкой bundled UI. Это checkpoint I installer-плана: критерий завершения — сценарий
   в собранном Go binary с реальным loopback bridge, а не только Vite/fixture preview.
   Fixture adapter не попадает в production bundle. Повторно писать этот E2E на каждом слое не нужно.

Оценка frontend: **2 600–4 200 production LOC**, включая CSS, responsive layout,
a11y/Headless adapter, виртуализацию, API/model и app wiring; **3 900–6 200 total changed LOC** с
fixtures, meaningful tests, build config и краткой документацией. Lockfile/generated
assets учитывать отдельно, не маскировать ими review budget. Это диапазон планирования,
не измеренный diff; Go API/embed/installer/relocation engine и editable OpenAPI/codegen config
в него не входят. Generated DTO учитываются отдельно.
Снижение прежней оценки на 400–600 production LOC связано с готовыми keyboard/ARIA/overlay
mechanics; search, layout, eligible policy, plan/recovery и browser checks остаются нашими.
Это ожидаемое сокращение, не измеренная экономия; уверенность средняя, а не гарантия.

UX-ориентир на контрольной desktop-машине: фильтрация/раскрытие по 1 000 кандидатам
должны визуально отвечать в пределах около 100 ms; измерить фактическое значение до приёмки.
Зафиксировать размер viewport, число кандидатов и измерение локального поиска/раскрытия;
при превышении ориентира сначала проверить повторные full-index построения и объём DOM,
не добавлять worker или новый search engine без измеренной причины.
Не делать flaky timing test для Git network и не подменять responsive UX одним наличием virtualizer.

## Основания решений

Проверено по первичным docs 2026-10-23: [React derived state](https://react.dev/learn/you-might-not-need-an-effect),
[Zustand state updates](https://zustand.docs.pmnd.rs/learn/guides/updating-state),
[TanStack Router search params](https://tanstack.com/router/latest/docs/guide/search-params),
[TanStack Query defaults](https://tanstack.com/query/latest/docs/framework/react/guides/important-defaults),
[TanStack Virtual](https://tanstack.com/virtual/latest/docs/framework/react/react-virtual),
[WAI-ARIA tree pattern](https://www.w3.org/WAI/ARIA/apg/patterns/treeview/).
Registry research 2026-10-23: React/DOM 19.3.1, Zustand 5.0.16, Router 1.170.46,
Query 5.104.5, Virtual 3.14.22, Vite 8.3.5, TypeScript **5.9.3**. TS 7.0.2 исключён из pinned baseline:
`openapi-typescript` 7.13.0 требует TypeScript `^5.x`; не обходить peer checks через force.
Глобальный `tsc7` допустим как дополнительный preflight, не замена pinned CI compiler.
Это исследованные версии, не installation:
при реализации повторно проверить stable, engines/peer dependencies и закрепить lockfile.
Добавленные/сравниваемые версии, license и peer ranges проверены по первичным npm manifests:
[Headless](https://registry.npmjs.org/@headless-tree/react/1.8.0),
[RAC](https://registry.npmjs.org/react-aria-components/1.22.1),
[Arborist](https://registry.npmjs.org/react-arborist/4.0.1),
[Base UI](https://registry.npmjs.org/@base-ui/react/1.9.0),
[Markdown](https://registry.npmjs.org/react-markdown/10.1.0),
[GFM](https://registry.npmjs.org/remark-gfm/4.0.1), [Lucide](https://registry.npmjs.org/lucide-react/1.61.0).
Headless 1.8.0 опубликован 2026-10-15, RAC 1.22.1 — 2026-10-09, Arborist 4.0.1 — 2026-10-11;
peer compatibility означает разрешение package manager, а не уже выполненную browser qualification.
