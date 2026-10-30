---
title: "إضافة Skills أخرى"
description: "أضف Skill مباشرة أخرى إلى حزمة معيارية موجودة."
canonicalId: "page:build:skills"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# إضافة Skills أخرى {#add-extra-skills}

**التأليف من المصدر:** تتطلب أوامر هذه الصفحة [بناءً من المصدر يدعم التأليف](./#build-from-source). لا يتضمن إصدار المثبّت الحالي `agentplugins author`.

استخدم `skills init` لإضافة Skill واحدة إلى حزمة معيارية موجودة. يعمل ذلك مع حزم Skill أو MCP أو الحزم الهجينة. اختر جذر الحزمة الذي يحتوي على `plugin.json`، وليس مجلد `skills/` أو ملف `SKILL.md` منفردًا.

## سمِّ Skill الجديدة وحدّد الحزمة {#name-the-new-skill-and-package}

لنفترض أن `./review-helper` هي الحزمة من [مسار Skill المستقلة](./skill)، وأن `skills/release-notes/` غير موجود.

باستخدام بناء المصدر الداعم للتأليف:

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

يلزم اسم Skill كوسيطة موضعية، ووصف صريح من 1 إلى 1024 محرفًا. لا تُطبّع الأسماء تلقائيًا. استخدم اسمًا دقيقًا قابلًا للنقل بحروف صغيرة وشرطات، واختر وجهة جديدة بدل توقع الكتابة فوق وجهة موجودة.

## راجع المكوّن الإضافي {#review-the-additional-component}

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

عدّل `skills/release-notes/SKILL.md` لشرح متى يُستخدم، ومنهجه، والمخرجات المتوقعة. حافظ على دقة الاسم والوصف. مثلًا، اطلب فصل التغييرات التي يراها المستخدم عن تغييرات الصيانة، والاستشهاد بالمصدر الذي تمت مراجعته لكل ادعاء عن الإصدار.

يمكن للبرامج النصية والمراجع والأصول دعم التعليمات، لكن وجودها ليس دليلًا على تنفيذها أو إذنًا بتشغيلها. ولا يُعد حقل `allowed-tools` دليل تفويض مستقلًا. راجع المواد الداعمة بالعناية نفسها التي تراجع بها نص التعليمات.

## تحقّق من الحزمة وSkills المباشرة {#validate-the-package-and-immediate-skills}

```bash
agentplugins author skills validate ./review-helper
```

يعرض الأمر جاهزية الحزمة ككل ونتيجة كل Skill موضوعة مباشرةً تحت `skills/` على حدة. لذلك قد يؤثر خطأ في موضع آخر من الحزمة على النتيجة العامة، حتى لو لم تتضمن نتائج Skill الجديدة أي مشكلة.

عند حذف مسار الحزمة، يُستخدم المجلد الحالي فقط دون البحث في المجلدات الأعلى. يجعل تحديد الجذور صراحةً البرامج النصية وتشخيص المشكلات أوضح. ولا يُغني التنفيذ من داخل `skills/release-notes/` عن اختيار جذر الحزمة.

## عالج الفشل دون استبدال العمل {#handle-a-failure-without-overwriting-work}

اقرأ `committed` وإرشادات الخطأ المصاحبة إذا فشل الإنشاء. قد تكون Skill قد كُتبت قبل أن تفشل فحوص الحزمة الناتجة أو تبقى غير مكتملة. افحص الملفات الفعلية والتقرير قبل المحاولة مجددًا؛ فلن يستبدل init ثانٍ وجهة موجودة.

صحح مشكلة المدخلات أو الحزمة المبلّغ عنها وأعد التحقق. إذا كان الاسم خاطئًا، فراجع تغيير المصدر المقصود صراحةً بدل إضافة علم إجبار. ينتمي تثبيت Skills الخارجية وصيانتها إلى مسار المثبّت المنفصل؛ أما `author skills` فيعدّل مكونات حزمة المصدر ويتحقق منها.

تابع إلى [الفحوص الساكنة للحزمة](./checks) و[التسليم](./handoff).
