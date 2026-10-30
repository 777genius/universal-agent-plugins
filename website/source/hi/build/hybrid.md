---
title: "हाइब्रिड पैकेज बनाएँ"
description: "एक पैकेज पहचान में एक Skill और एक MCP कॉन्फ़िगरेशन जोड़ें।"
canonicalId: "page:build:hybrid"
section: "build"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# हाइब्रिड पैकेज बनाएँ {#build-a-hybrid-package}

**सोर्स बिल्ड में पैकेज निर्माण:** इस पेज की कमांड के लिए [पैकेज निर्माण की सुविधा वाले सोर्स बिल्ड](./#build-from-source) चाहिए। वर्तमान इंस्टॉलर रिलीज़ में `agentplugins author` शामिल नहीं है।

जब निर्देश टूल के इस्तेमाल का समय और तरीका समझाते हों, तब हाइब्रिड उपयोगी है। दोनों घटक एक रूट मेनिफ़ेस्ट के अंतर्गत रहते हैं। क्लाइंट उनका अलग-अलग समर्थन कर सकता है, इसलिए हस्तांतरण में हर घटक के संगतता निष्कर्ष स्पष्ट रखें।

## MCP वाला भाग स्पष्ट चुनें {#choose-the-mcp-side-explicitly}

हाइब्रिड फ़्लैग `--mcp-template` है। `mcp-remote` या `mcp-stdio` चुनकर उस टेम्प्लेट के आवश्यक इनपुट दें। हाइब्रिड URL, लोकल फ़ाइल या इंस्टॉल किए रनटाइम से ट्रांसपोर्ट का अनुमान नहीं लगाता।

दूरस्थ हाइब्रिड का उदाहरण:

```bash
agentplugins author init ./research-helper \
  --template hybrid \
  --name research-helper \
  --description 'Guide research using the team MCP service' \
  --skill-name research-guide \
  --mcp-template mcp-remote \
  --url https://mcp.example.com/mcp
```

URL केवल उदाहरण का प्लेसहोल्डर है; सेवा उपलब्ध होने का वादा नहीं। उसे अपने वास्तविक एंडपॉइंट से बदलें। [दूरस्थ MCP की आवश्यकताएँ](./mcp-remote) पढ़ें।

## या लोकल प्रक्रिया चुनें {#or-choose-a-local-process}

दूसरे, अभी अनुपस्थित गंतव्य के लिए stdio रूप यह है:

```bash
agentplugins author init ./local-research \
  --template hybrid \
  --name local-research \
  --description 'Guide research using local Node tools' \
  --skill-name research-guide \
  --mcp-template mcp-stdio \
  --runtime node
```

Stdio रूप में `--url` न जोड़ें। [Stdio पूर्वापेक्षाएँ और रनटाइम सीमाएँ](./mcp-stdio) देखें।

## निर्देशों को टूल से जोड़ें {#connect-instructions-to-tools}

दूरस्थ हाइब्रिड की मूल संरचना:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

README और `.gitignore` भी बनते हैं। Stdio हाइब्रिड में stdio गाइड की Node स्रोत और पैकेज फ़ाइलें जुड़ती हैं। `SKILL.md` की समीक्षा करके बताएँ कि टूल कब इस्तेमाल हों, किस इनपुट की ज़रूरत है और पहुँच न होने पर क्या रिपोर्ट करें। एजेंट को Skill पाठ को अनुमति मानने का निर्देश न दें।

## दोनों घटक जाँचें {#check-both-components}

```bash
agentplugins author skills validate ./research-helper
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper --target codex,claude
agentplugins author test ./research-helper
agentplugins author compat ./research-helper --target codex,claude
```

लोकल रूप के लिए इसके बजाय `./local-research` चुनें। घटक-स्तर के निष्कर्ष बनाए रखें: Skill समर्थित होने से MCP ट्रांसपोर्ट का समर्थन या सेवा सक्रियण अपने-आप नहीं हो जाता। संयुक्त पैकेज यह सीमा नहीं मिटाता।

आगे [अतिरिक्त Skills](./skills) या [पैकेज सौंपना](./handoff) देखें।
