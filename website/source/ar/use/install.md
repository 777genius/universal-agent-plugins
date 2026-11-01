---
title: "البحث عن إضافة وتثبيتها"
description: "افحص حزمة موجودة، وعاين تغييرات الأهداف، وافهم متطلبات التفعيل."
canonicalId: "page:use:install"
section: "use"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# البحث عن إضافة وتثبيتها {#find-and-install-a-plugin}

اختر حزمة تفهم مصدرها والغرض منها. تستخدم الأوامر التالية المثبّت المنشور؛ راجع [تثبيت CLI](./) إذا لم يكن `agentplugins` متاحًا بعد. استخدم `--dry-run` لمراجعة الخطة قبل تطبيق أي تغييرات.

## اكتشف الحزمة {#discover-the-package}

```bash
agentplugins search docs
agentplugins info context7
```

يأتي الاسم المختصر مثل `context7` من Universal Agent Plugins Registry الموقّع. تساعدك نتائج البحث ومعلومات الحزمة على اختيار مرشح؛ ولا تنفّذ الحزمة أو تثبت توفر خدمتها. راجع الأدوات المعروضة وSkills ومتطلبات التشغيل وصلاحيات الحساب قبل التثبيت.

## عاين اختيارًا صريحًا واحدًا {#preview-one-explicit-selection}

```bash
agentplugins add context7 --target codex,cursor --dry-run
```

راجع المصدر والأهداف المختارة والوجهات والنتائج الأمنية وما تبقى من أعمال التفعيل. يطبع هذا الأمر خطة من دون تطبيق التثبيت. ولا يختبر خادم MCP.

## ثبّت بعد مراجعة الخطة {#install-after-reviewing-the-plan}

```bash
agentplugins add context7 --target codex,cursor
```

للاختيار التفاعلي، احذف `--target`. يمكن لواجهة CLI اختيار وكيل واحد مكتشَف تلقائيًا، أو عرض منتقي عند توفر عدة وكلاء. استخدم أهدافًا صريحة لاختيار قابل للتكرار وفي عمليات الأتمتة.

يجري المثبّت فحوصًا مسبقة للعملاء المختارين ويحمي الملفات التي لا يملكها. إذا أبلغت عملية عن فشل التراجع أو عن استرداد معلّق، فاحتفظ بالتقرير واتبع إرشادات الاسترداد قبل إعادة المحاولة.

## أكمل التفعيل {#finish-activation}

أكمل الخطوات الخاصة بالعميل في النتيجة: قد يلزم إعادة تحميل، أو جلسة جديدة، أو ثقة بمساحة العمل، أو تسجيل دخول، أو تفعيل صريح. لا تثبت الملفات المجهزة والتسجيل الأصلي الناجح نجاح OAuth أو استدعاء الأدوات. استخدم [تشخيص الحالة المُدارة](./manage) إذا كانت ملفات التثبيت مفقودة أو تغيرت؛ وافحص المصادقة داخل العميل إذا كانت الحزمة موجودة لكنها غير متاحة.

## استخدم حزمة محلية {#use-a-local-package}

اختر المجلد الذي يحتوي على `plugin.json` في جذره:

```bash
agentplugins validate ./my-plugin
agentplugins add ./my-plugin --target codex --dry-run
```

تظل سياسة الأمان مطبّقة في تحقق المثبّت وتخطيطه. وقد يجلب بيانات أمان وصفية أو ماسحًا مثبت الإصدار حتى للحزمة المحلية. يتجنب `--dry-run` تغييرات التثبيت المُدار، ولا يَعِد بأن العملية تعمل بلا اتصال.

بعد مراجعة الخطة، احذف `--dry-run` للتثبيت. يبقى تثبيت المجلد المحلي محليًا؛ ولا يصبح اعتمادية قابلة للنقل تستطيع آلة أخرى جلبها. اطلب من المؤلف [أدلة تسليم الحزمة](../build/handoff).

## اختر مصدر GitHub دقيقًا {#select-an-exact-github-source}

تتطلب صيغة GitHub غير القابلة للتغيير SHA كاملًا للالتزام من 40 محرفًا. استبدل هذا المثال بمستودع حقيقي ومراجعة جرى تدقيقها ومسار حزمة:

```text
agentplugins add owner/repository@0123456789abcdef0123456789abcdef01234567//path/to/plugin --target codex --dry-run
```

ترفض صيغة `owner/repository@...` الفروع والوسوم وقيم SHA المختصرة. دون مسار، يختار المثبّت حزمة صالحة في الجذر أو المرشح المتداخل الصالح الوحيد. يتطلب وجود عدة مرشحين مسارًا صريحًا؛ وعند وجود أكثر من 16 حزمة محتملة، يلزم المسار قبل جلب المرشحين. يعيد الإصلاح استخدام المصدر المسجل. ونقل تثبيت غير قابل للتغيير إلى مصدر آخر عملية مستقلة لاختيار المصدر.

يقبل CLI 0.1.93 أيضًا Git مباشرًا عبر HTTPS وSSH وروابط المتصفح لمزوّدين مدعومين وأرشيفات كاملة عبر HTTPS أو محلية. استبدل هذه القيم التوضيحية بمصادر حزم راجعتها، مثلًا:

```text
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target codex --dry-run
agentplugins add 'ssh://git@git.example.com/team/tools.git#main//plugins/demo' --target codex --dry-run
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target codex --dry-run
```

يستخدم Git بيانات الاعتماد الأصلية وثقة مضيف SSH الموجودتين. لا تستخدم أرشيفات HTTPS بيانات اعتماد، ويجب أن تحتوي على حزمة معيارية كاملة. يثبّت القفل مراجعات Git الدقيقة أو البايتات الأصلية للأرشيف، إلى جانب بصمات الحزم. يمكن تقديم مصدر متحرك عبر تحديث صريح؛ أما الاستعادة المقفلة فتحافظ على المرجع المثبت. تختلف مصادر الحزم الكاملة هذه عن جلب Skills منفردة. اقرأ [مصادر الحزم الكاملة المباشرة](https://github.com/777genius/universal-agent-plugins/blob/main/docs/DIRECT_PACKAGE_SOURCES.md) لصيغ المزوّدين واختيار الحزمة والمجاميع الاختبارية وحدود الاستخراج.

## ثبّت Agent Skills {#install-agent-skills}

استخدم مجموعة أوامر `skills` لجلب Skills منفردة من مستودع GitHub عام، أو مجلد، أو رابط `SKILL.md`، أو ناشر HTTPS يوفر فهرس Skills كاملًا ضمن `.well-known`:

```bash
agentplugins skills add owner/repository --ui
```

استبدل `owner/repository` بمصدر حقيقي. يفتح `--ui` منتقي المتصفح المضمّن ويتطلب طرفية تفاعلية. استخدم `--no-ui` للاختيار في الطرفية. وللاختيار الصريح:

```bash
agentplugins skills add owner/repository --skill review --skill pdf \
  --target codex,claude --no-ui --yes
```

- كرر `--skill` للأسماء الدقيقة. استخدم `--path` لمجلد Skill دقيق نسبةً إلى المستودع إذا كان الاسم ملتبسًا.
- يحدد `--ref` مرجع Git دقيقًا عندما يكون حد الرابط ملتبسًا.
- يعاين `--dry-run` التثبيت المختار.
- يتطلب استخدام JSON والوضع غير التفاعلي تحديد Skills والأهداف صراحةً وإضافة `--yes`، باستثناء المعاينة عبر `--dry-run`.
- لا يمكن الجمع بين `--ui` و`--skill` أو `--target` أو `--dry-run`.
- يتطلب ناشر HTTPS اسم `--skill` دقيقًا وفهرسًا يسرد كل ملف ضمن Skill تلك.

قد تمنع النتائج الأمنية الآلية التثبيت. يعرض `--security-details` هذه النتائج في المخرجات المقروءة؛ راجعها قبل تقرير مدى ملاءمة القبول الصريح عبر `--accept-security-risk`. تظهر Skills المثبّتة في `agentplugins list` وتستخدم أوامر الصيانة المعتادة.

## ثبّت لمشروع واحد {#install-for-one-project}

يتوفر نطاق المشروع الصريح في [CLI 0.1.93 وأحدث](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93). اختر مجلدًا موجودًا وحدّد النطاق في كل أمر:

لإدخال تمت مراجعته ومتوافق في الفهرس، انسخ مصدر Git المثبّت بمعرّف commit كامل والعميل المتوافق. تظل أسماء Directory v1 المختصرة مثل `context7` مخصّصة لنطاق المستخدم فقط. استبدل `OWNER/REPOSITORY` و`FULL_COMMIT_SHA` و`PACKAGE_PATH`؛ وإذا كانت الحزمة في جذر المستودع فاحذف `//PACKAGE_PATH`.

```bash
agentplugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
agentplugins skills add owner/repository --ui --scope project \
  --project-root /path/to/project
```

يعاين الأمر الأول الخطة؛ احذف `--dry-run` عندما تصبح مستعدًا لتطبيقها. يفتح الثاني منتقيًا ضمن النطاق المحدد ويمكن أن يطبّق اختيار المستخدم. من دون `--project-root`، يستخدم نطاق المشروع الصريح مجلد تنفيذ الأمر، ولا يبحث في المجلدات الأب. تعمل أوامر الإضافة والصيانة افتراضيًا ضمن نطاق المستخدم في غير ذلك. في CLI 0.1.94، يختار `install` من دون وسيطة مصدر نطاق المشروع إذا وُجد ملف عادي باسم `uap.toml` في المجلد الحالي نفسه؛ وتكون الأولوية للنطاق الصريح. راجع [قواعد الاستعادة](./manage) لمعرفة سلوك القفل الصارم.

يجب أن يدعم الهدف مكونات المشروع المطلوبة. تثبيت مشاريع ChatGPT غير مدعوم صراحةً. يجهّز VS Code ملفات `.github/skills` وعناصر `servers` التي يملكها داخل `.vscode/mcp.json` لمجلد محلي واحد. وتبقى النتيجة **Prepared**: راجع ثقة مساحة العمل والخادم وافحص إعدادات العميل. لا تثبت ملاحظة البيانات الوصفية هذه بدء MCP أو استدعاء الأدوات أو استخدام النموذج.

اقرأ [دليل تثبيت المشروع](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) و[حدود المشاريع لكل عميل](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md) للمسارات وحدود التفعيل. ثم انتقل إلى [الصيانة والاستعادة](./manage).
