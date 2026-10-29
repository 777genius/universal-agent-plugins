---
title: "प्लगइन बनाएँ"
description: "Agent Plugins पैकेज निर्माण से Skill, MCP पैकेज या हाइब्रिड बनाएँ।"
canonicalId: "page:build:index"
section: "build"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# प्लगइन बनाएँ {#build-plugins}

Agent Plugins 1.0 पैकेज एक डायरेक्टरी है जिसके रूट में `plugin.json` और Skills, MCP कॉन्फ़िगरेशन या दोनों होते हैं। उपयोगकर्ता का काम हल करने वाले सबसे छोटे घटक से शुरू करें। Skill निर्देश देता है; MCP टूल से कनेक्शन देता है।

## उपलब्धता {#availability}

वर्तमान नेटिव इंस्टॉलर रिलीज़ [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94) मानक पैकेज वैलिडेट और इंस्टॉल कर सकती है। पैकेज मैनेजर चैनल पीछे रह सकते हैं; इंस्टॉल किया संस्करण जानने के लिए `agentplugins version` जाँचें। इसमें `agentplugins author` **शामिल नहीं है**। इस बिल्ड गाइड की पैकेज निर्माण कमांड **वर्तमान पैकेज निर्माण की सुविधा वाले सोर्स बिल्ड** के लिए हैं। ऐतिहासिक 0.1.65 पैकेज निर्माण रिलीज़ के प्रमाण उसी संस्करण से जुड़े हैं और आज के इंस्टॉलर का विवरण नहीं हैं।

फ़ाइलें हाथ से लिखकर प्रकाशित इंस्टॉलर इस्तेमाल कर सकते हैं, या नीचे दिए सोर्स टूल बनाकर ढाँचा निर्माण, स्थिर जाँचें और स्पष्ट MCP रनटाइम चक्र चला सकते हैं। सफल स्थिर जाँच नेटिव क्लाइंट सक्रियण या काम करता सेवा कनेक्शन सिद्ध नहीं करती।

## छोटा पैकेज हाथ से बनाएँ {#make-a-small-package-by-hand}

यह संरचना बनाएँ:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

पैकेज की पहचान `plugin.json` में रखें:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "दस्तावेज़ों के बदलावों की स्पष्टता और प्रमाणों की समीक्षा करें"
}
```

`skills/review-docs/SKILL.md` की शुरुआत इस तरह करें:

```markdown
---
name: review-docs
description: दस्तावेज़ों की स्पष्टता और प्रमाणों की समीक्षा करते समय इस्तेमाल करें।
---

# दस्तावेज़ों की समीक्षा करें

इच्छित पाठक और काम पहचानें। हर कमांड के इनपुट और प्रभाव जाँचें।
सत्यापित व्यवहार को पूर्वापेक्षाओं और न जाँचे गए दावों से अलग रखें।
निष्कर्षों के साथ फ़ाइल का स्थान और सुझाया सुधार दें।
```

प्रकाशित मैनेजर इंस्टॉल करें, फिर पैकेज वैलिडेट करें और पूर्वावलोकन देखें:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

योजना की समीक्षा करें और उसे लागू करने के लिए तैयार होने पर [प्लगइन इस्तेमाल करें](../use/install) देखें। हर फ़ाइल की ज़िम्मेदारी के लिए [पैकेज संरचना](./layout) पढ़ें।

## सोर्स से बिल्ड करें {#build-from-source}

नीचे की कमांड वर्तमान CLI को सोर्स से बनाकर पैकेज निर्माण की सुविधा सक्षम करती हैं। इसके लिए Git और रिपॉज़िटरी में घोषित Go टूलचेन चाहिए (अभी Go 1.26.10)। परिणाम साझा करते समय चेकआउट किया हुआ कमिट दर्ज करें।

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

ट्यूटोरियल पैकेजों के लिए अलग कार्य डायरेक्टरी इस्तेमाल करें। यह सोर्स बाइनरी विकास बिल्ड है। इसका मौजूद होना योग्य सार्वजनिक पैकेज निर्माण रिलीज़ का प्रमाण नहीं है। इंस्टॉल किया मैनेजर अब भी `author` को अज्ञात कमांड बताए तो एक्ज़ीक्यूटेबल का पाथ जाँचें।

## बनाने के लिए पैकेज चुनें {#choose-a-package-to-scaffold}

| ज़रूरत | मार्गदर्शिका | टेम्प्लेट के आवश्यक इनपुट |
| --- | --- | --- |
| एजेंट के पालन करने योग्य निर्देश | [स्वतंत्र Skill](./skill) | `--template skill` |
| मौजूदा सेवा से जुड़ना | [दूरस्थ MCP](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| लोकल Node MCP प्रक्रिया देना | [Stdio MCP](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| टूल के साथ निर्देश | [हाइब्रिड](./hybrid) | `--template hybrid --mcp-template <choice>` और उस MCP टेम्प्लेट के इनपुट |

स्वतंत्र Skill पैकेज में भी रूट `plugin.json` होता है। दूसरी रिपॉज़िटरी से अलग Skills इंस्टॉल करना अलग [इंस्टॉलर कार्रवाई](../use/install#install-agent-skills) है।

## पैकेज निर्माण चक्र अपनाएँ {#follow-the-authoring-loop}

1. ऐसे गंतव्य में पैकेज बनाएँ जो अभी मौजूद न हो, फिर बनाई गई फ़ाइलें संपादित करें।
2. ठीक पैकेज रूट पर [validate, inspect और स्थिर test](./checks) चलाएँ।
3. `compat` से इच्छित क्लाइंट जाँचें और `doctor` के प्रमाण देखें।
4. MCP के लिए स्पष्ट सोर्स बिल्ड निष्पादन से रनटाइम अलग जाँचें।
5. [इंस्टॉलर को सौंपने की प्रक्रिया](./handoff) से पैकेज और उसके प्रमाण साझा करें।

वर्तमान सोर्स में लॉकफ़ाइल आधारित `bootstrap`, निरंतर `dev`, JSON `normalize` और स्पष्ट Claude MCP `import native` भी उपलब्ध हैं। [जाँचें](./checks) और [stdio MCP](./mcp-stdio) देखें। क्लाइंट प्रोजेक्शन, बंडल एक्सपोर्ट और पैकेज प्रकाशन वर्तमान पैकेज निर्माण कमांड के दायरे से बाहर हैं। क्लाइंट-विशिष्ट हुक पोर्टेबल Skill और MCP घटकों से अलग रहते हैं।
