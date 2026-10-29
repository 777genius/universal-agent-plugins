---
title: "بناء الإضافات"
description: "أنشئ Skill أو حزمة MCP أو حزمة هجينة بأدوات تأليف Agent Plugins."
canonicalId: "page:build:index"
section: "build"
locale: "ar"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# بناء الإضافات {#build-plugins}

حزمة Agent Plugins 1.0 هي مجلد يحتوي على `plugin.json` في جذره، إلى جانب Skills أو إعدادات MCP أو كليهما. ابدأ بأصغر مكوّن يحل مهمة المستخدمين. توفر Skill التعليمات، ويوفر MCP الاتصال بالأدوات.

## التوفر {#availability}

يستطيع إصدار المثبّت الأصلي الحالي، [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94)، التحقق من الحزم المعيارية وتثبيتها. قد تتأخر قنوات مديري الحزم؛ افحص `agentplugins version` لمعرفة الإصدار المثبّت. لا يتضمن إصدار المثبّت الحالي `agentplugins author`. تصف أوامر التأليف في هذا المسار **البناء من المصدر الحالي مع تمكين التأليف**. تبقى أدلة إصدار التأليف التاريخي 0.1.65 خاصة بذلك الإصدار، ولا تصف المثبّت الحالي.

يمكنك كتابة الملفات يدويًا واستخدام المثبّت المنشور، أو بناء أدوات المصدر أدناه لتوليد الهياكل والفحوص الساكنة ودورة تشغيل MCP صريحة. لا يثبت نجاح الفحص الساكن تفعيل العميل الأصلي أو سلامة الاتصال بالخدمة.

## أنشئ حزمة صغيرة يدويًا {#make-a-small-package-by-hand}

أنشئ هذه البنية:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

ضع هوية الحزمة في `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "مراجعة تغييرات الوثائق للتحقق من وضوحها وأدلتها"
}
```

ابدأ `skills/review-docs/SKILL.md` بما يلي:

```markdown
---
name: review-docs
description: تُستخدم عند مراجعة الوثائق للتحقق من وضوحها وأدلتها.
---

# مراجعة الوثائق

حدّد القارئ المقصود والمهمة. افحص مدخلات كل أمر وتأثيره.
ميّز بين السلوك المتحقق منه والمتطلبات المسبقة والادعاءات غير المختبرة.
قدّم النتائج مع مواقع الملفات وتصحيح مقترح.
```

ثبّت المدير المنشور، ثم تحقّق من الحزمة وعاينها:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

راجع الخطة واتبع [استخدام الإضافات](../use/install) عندما تصبح جاهزًا لتطبيقها. راجع [بنية الحزمة](./layout) لمعرفة مسؤولية كل ملف.

## ابنِ من المصدر {#build-from-source}

تبني الأوامر التالية واجهة CLI الحالية من المصدر مع تمكين أدوات التأليف. ويتطلب Git وسلسلة أدوات Go المعلنة في المستودع، وهي حاليًا Go 1.26.10. سجّل الالتزام الذي سحبت نسخته عند مشاركة النتائج.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

استخدم مجلد عمل منفصلًا لحزم الدروس. هذا الملف التنفيذي المبني من المصدر هو بناء تطويري؛ ولا يعني وجوده أن إصدار تأليف عام قد اجتاز التأهيل. افحص مسار الملف التنفيذي إذا كان المدير المثبّت لا يزال يبلغ عن أن `author` أمر غير معروف.

## اختر هيكل الحزمة المراد توليده {#choose-a-package-to-scaffold}

| الحاجة | المسار | مدخلات القالب المطلوبة |
| --- | --- | --- |
| تعليمات يستطيع الوكيل اتباعها | [Skill مستقلة](./skill) | `--template skill` |
| الاتصال بخدمة موجودة | [MCP بعيد](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| توفير عملية MCP محلية عبر Node | [MCP عبر stdio](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| تعليمات مع أدوات | [حزمة هجينة](./hybrid) | `--template hybrid --mcp-template <choice>` ومدخلات قالب MCP المختار |

تحتوي حزمة Skill المستقلة أيضًا على `plugin.json` في الجذر. أما تثبيت Skills منفردة من مستودع آخر فهو [عملية مستقلة للمثبّت](../use/install#install-agent-skills).

## اتبع دورة التأليف {#follow-the-authoring-loop}

1. أنشئ الحزمة في وجهة غير موجودة، ثم عدّل الملفات المولّدة.
2. شغّل [validate وinspect والاختبار الساكن](./checks) على جذر الحزمة الدقيق.
3. افحص العملاء المقصودين باستخدام `compat` وراجع أدلة `doctor`.
4. بالنسبة إلى MCP، قيّم التشغيل على نحو منفصل عبر تنفيذ صريح من بناء المصدر.
5. شارك الحزمة وأدلتها عبر [التسليم إلى المثبّت](./handoff).

يوفر المصدر الحالي أيضًا `bootstrap` المعتمد على ملف القفل، و`dev` المستمر، و`normalize` لـ JSON، و`import native` الصريح لـ Claude MCP. راجع [الفحوص](./checks) و[MCP عبر stdio](./mcp-stdio). تقع إسقاطات إعدادات العملاء وتصدير الحزم المجمعة ونشر الحزم خارج واجهة أوامر التأليف الحالية. وتبقى الخطافات الخاصة بكل عميل منفصلة عن مكونات Skill وMCP القابلة للنقل.
