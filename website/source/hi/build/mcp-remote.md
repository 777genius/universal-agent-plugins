---
title: "दूरस्थ MCP पैकेज बनाएँ"
description: "मौजूदा MCP एंडपॉइंट का विवरण दें और कॉन्फ़िगरेशन को कनेक्टिविटी से अलग रखें।"
canonicalId: "page:build:mcp-remote"
section: "build"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# दूरस्थ MCP पैकेज बनाएँ {#build-a-remote-mcp-package}

**सोर्स बिल्ड में पैकेज निर्माण:** इस पेज की कमांड के लिए [पैकेज निर्माण की सुविधा वाले सोर्स बिल्ड](./#build-from-source) चाहिए। वर्तमान इंस्टॉलर रिलीज़ में `agentplugins author` शामिल नहीं है।

दूरस्थ MCP तब चुनें जब सर्वर पहले से मौजूद हो और पैकेज को एजेंट के उससे जुड़ने का तरीका बताना हो। पैकेज निर्माण URL दर्ज करता है, उससे संपर्क नहीं करता। पैकेज सौंपने से पहले सेवा के ट्रांसपोर्ट और प्रमाणीकरण की आवश्यकताएँ समझना आपकी ज़िम्मेदारी है।

## एंडपॉइंट स्पष्ट दें {#supply-the-endpoint-explicitly}

उदाहरणों में `https://mcp.example.com/mcp` दस्तावेज़ का प्लेसहोल्डर है। अपना पैकेज तैयार करते समय उसे सेवा के वास्तविक MCP एंडपॉइंट से बदलें। पैकेज निर्माण कमांड URL को कॉन्फ़िगरेशन मानती है; वह यह सत्यापित नहीं करती कि पता MCP देता है या आपका खाता उसे इस्तेमाल कर सकता है।

पैकेज निर्माण की सुविधा वाले सोर्स बिल्ड के साथ:

```bash
agentplugins author init ./docs-service \
  --template mcp-remote \
  --name docs-service \
  --description 'Connect to the team documentation MCP service' \
  --url https://mcp.example.com/mcp
```

मौजूदा पैरेंट के भीतर अनुपस्थित गंतव्य चुनें। टेम्प्लेट को पूर्ण HTTPS URL या loopback पर HTTP चाहिए। क्रेडेंशियल, फ़्रैगमेंट और अनसुलझे URL टोकन टेम्प्लेट इनपुट के रूप में स्वीकार नहीं हैं। URL में पासवर्ड न रखें।

## फ़ाइलें समझें {#understand-the-files}

```text
docs-service/
  plugin.json
  mcp.json
  README.md
  .gitignore
```

रूट मेनिफ़ेस्ट पैकेज की पहचान रखता है। `mcp.json` में `mcpServers` के अंतर्गत सर्वर का विवरण होता है। यह टेम्प्लेट `type: streamable-http` और दिया गया URL इस्तेमाल करता है। यह सर्वर नहीं बनाता और Skill नहीं जोड़ता। टूल कब कॉल करने हैं, यह समझाने वाले निर्देश भी चाहिए हों तो [हाइब्रिड](./hybrid) इस्तेमाल करें।

README की समीक्षा करें और एंडपॉइंट का उद्देश्य, खाते की पूर्वापेक्षाएँ तथा प्रमाणीकरण निर्देश मिलने की जगह लिखें। सीक्रेट पैकेज से बाहर रखें। मानक-अनुरूप फ़ाइल में URL होना सक्रिय सेवा का प्रमाण नहीं है।

## कॉन्फ़िगरेशन और टार्गेट समर्थन जाँचें {#check-configuration-and-target-support}

```bash
agentplugins author validate ./docs-service
agentplugins author inspect ./docs-service --target codex,claude
agentplugins author test ./docs-service
agentplugins author compat ./docs-service --target codex,claude
agentplugins author doctor ./docs-service
```

`inspect` घटक और अनसुलझी आवश्यकताएँ दिखा सकता है। `compat` चुने क्लाइंट के स्थिर एडेप्टर समर्थन का मूल्यांकन करता है। किसी भी कमांड के लिए वे क्लाइंट इंस्टॉल होना आवश्यक नहीं है। Doctor एंडपॉइंट से संपर्क नहीं करता।

## बाकी काम स्पष्ट रखें {#keep-the-remaining-work-visible}

लेखक के स्थिर प्रमाण में ट्रांसपोर्ट और इच्छित क्लाइंट बताए जाने चाहिए। प्राप्त करने वाले इंस्टॉलर को अब भी इंस्टॉलेशन नीति का मूल्यांकन और सक्रियण निर्देश दिखाने होते हैं। उपयोगकर्ता को साइन-इन, पहुँच अनुमोदन या नया क्लाइंट सत्र शुरू करना पड़ सकता है। रनटाइम टूल खोज और कॉल के लिए अलग प्रमाण चाहिए।

स्थिर जाँचें असफल हों तो पहले पढ़ा गया कॉन्फ़िगरेशन सुधारें। बाद में एंडपॉइंट तक पहुँच न हो तो सेवा या क्लाइंट कनेक्शन जाँचें; एक और सफल स्थिर परीक्षण को कनेक्टिविटी जाँच न मानें।

आगे [रिपोर्ट का अर्थ](./checks) और [योजनाकार को सौंपना](./handoff) देखें।
