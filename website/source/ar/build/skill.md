---
title: "بناء Skill مستقلة"
description: "أنشئ حزمة معيارية تحتوي على Skill تعليمية واحدة مفيدة."
canonicalId: "page:build:skill"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# بناء Skill مستقلة {#build-a-standalone-skill}

**التأليف من المصدر:** تتطلب أوامر هذه الصفحة [بناءً من المصدر يدعم التأليف](./#build-from-source). لا يتضمن إصدار المثبّت الحالي `agentplugins author`.

اختر هذا المسار عندما يحتاج الوكيل إلى منهج قابل للتكرار أو قائمة تحقق أو تعليمات كتابة، ولا يحتاج إلى اتصال بأداة MCP. تحتوي الحزمة على Skill واحدة وبيان جذري. لا يلزم خادم أو تثبيت بيئة تشغيل أو ملف تعريف عميل لخطوات التأليف.

## اختر وجهة صريحة {#choose-an-explicit-destination}

تفترض الأمثلة أن مجلد العمل مجلد أب مؤقت، وأن `./review-helper` غير موجود. اختر هوية حزمة قابلة للنقل بحروف صغيرة. يسمّي المسار الوجهة؛ أما `--name` فيسمّي الحزمة داخل بيانها.

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

يتطلب init وجهة غير موجودة ولا يستبدل مجلدًا موجودًا. إذا أبلغ الإنشاء عن نتيجة committed أعقبها فشل، فافحص النتيجة قبل إعادة المحاولة. لا يوجد علم للكتابة فوق الملفات في هذا المسار.

## راجع الملفات المنشأة {#review-the-created-files}

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

تُعد Skill المولّدة نقطة بداية للتعليمات. استبدل متنها بمنهج محدد يفيد مستخدميك. حافظ على توافق اسم Skill ومجلدها. مثلًا، يمكن أن يكون متن `skills/review-docs/SKILL.md` كما يلي:

```markdown
# مراجعة تغييرات الوثائق

استخدم هذه Skill عند مراجعة تغيير في الوثائق قبل تسليمه.

1. حدّد القارئ المقصود والمهمة التي تساعده الصفحة على إتمامها.
2. تحقّق من أن كل أمر يحدد مدخلاته ويصف تأثيره.
3. ميّز بين السلوك المتحقق منه والمتطلبات المسبقة والادعاءات غير المختبرة.
4. أبلغ عن التعليمات غير الواضحة واقترح جملة بديلة.

قدّم قائمة موجزة بالنتائج مع مواقع الملفات. إذا لم تجد مشكلة، فاذكر الصفحات
التي راجعتها والفحوص التي تبقى خارج نطاق المراجعة.
```

احتفظ بالبيانات الوصفية المولّدة أعلى المتن. يعرّف `name` و`description` فيها Skill ويساعدان الوكيل على تحديد وقت استخدامها. لا تمنح تعليمات Skill إذنًا بتشغيل برامج نصية أو الوصول إلى موارد خاصة.

## افحص جذر الحزمة الدقيق {#check-the-exact-package-root}

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

هذه فحوص ساكنة. ولا تطلب من وكيل اتباع التعليمات أو تقيّم جودة إجابته.

راجع المطابقة والجاهزية في التقرير منفصلتين. تظل Skill سليمة البنية بحاجة إلى مراجعة بشرية لفائدتها ودقتها ونطاقها المناسب.

تابع إلى [إضافة Skills](./skills) أو [فحوص الأهداف](./checks) أو [التسليم المحلي إلى المثبّت](./handoff). ولإضافة أدوات إلى جانب التعليمات، اختر [مسار الحزمة الهجينة](./hybrid) لحزمة جديدة.
