---
title: "Stdio MCP पैकेज बनाएँ"
description: "लोकल Node MCP सर्वर तैयार करें और रनटाइम सेटअप व निष्पादन अलग रखें।"
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

# Stdio MCP पैकेज बनाएँ {#build-a-stdio-mcp-package}

**सोर्स बिल्ड में पैकेज निर्माण:** नीचे की कमांड के लिए [पैकेज निर्माण की सुविधा वाले सोर्स बिल्ड](./#build-from-source) चाहिए। वे वर्तमान इंस्टॉलर रिलीज़ में शामिल नहीं हैं।

Stdio तब चुनें जब पैकेज मानक इनपुट और आउटपुट से संवाद करने वाली लोकल प्रक्रिया देता हो। ढाँचा Node.js 22 या बाद के संस्करणों को समर्थन देता है और आधिकारिक MCP SDK इस्तेमाल करता है। उसे बनाने से निर्भरताएँ इंस्टॉल नहीं होतीं और सर्वर शुरू नहीं होता।

## ढाँचा बनाएँ {#create-the-scaffold}

मौजूदा पैरेंट डायरेक्टरी के भीतर अनुपस्थित गंतव्य चुनें:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

इस पोर्टेबल टेम्प्लेट के लिए `node` समर्थित रनटाइम इनपुट है। Python या Go रनटाइम फ़्लैग इस कमांड के वर्तमान अनुबंध का हिस्सा नहीं है।

## फ़ाइलें देखें {#review-the-files}

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` सर्वर `local-tools` घोषित करता है, जिसकी कमांड `node` और आर्ग्युमेंट `${PLUGIN_ROOT}/src/server.mjs` है। पैकेज टोकन चुने हुए पैकेज के भीतर हल होता है; वह शेल की वर्तमान डायरेक्टरी नहीं है। स्रोत में शुरुआत के लिए छोटा `hello` टूल है। उसे अपने काम के अनुसार बदलें और एप्लिकेशन लॉग stderr में भेजें ताकि stdout MCP प्रोटोकॉल के लिए उपलब्ध रहे।

निर्भरता मेनिफ़ेस्ट और लॉकफ़ाइल देखें। Init उन्हें लिखता है, लेकिन npm नहीं चलाता और न सिद्ध करता है कि होस्ट SDK इम्पोर्ट कर सकता है।

## स्थिर प्रमाण जुटाएँ {#gather-static-evidence}

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

ये स्थिर जाँचें हैं। Doctor सीमित प्रोजेक्ट और एक्ज़ीक्यूटेबल मेटाडेटा देखता है; वह इंस्टॉल निर्भरताएँ, सफल हैंडशेक या काम करता टूल सिद्ध नहीं करता। [प्रमाण की परतें](./checks) अलग समझें।

## पहचाने गए टेम्प्लेट का bootstrap करें {#bootstrap-the-recognized-template}

मेल खाती लॉकफ़ाइल के साथ बने Node ढाँचे के लिए वर्तमान सोर्स निर्भरताओं का पूर्वावलोकन और इंस्टॉलेशन कर सकता है:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

Bootstrap लागू करने के लिए अभी Linux और उसका handle-bound फ़ाइल सिस्टम समर्थन चाहिए। दूसरे प्लेटफ़ॉर्म `bootstrap_platform_unsupported` बताते हैं। Dry run बिना निर्भरताएँ इंस्टॉल किए पहचाना गया ढाँचा देख सकता है।

लागू करने से पहले पूर्वावलोकन देखें। Bootstrap lifecycle स्क्रिप्ट बंद रखकर लॉक के अनुसार npm इंस्टॉलेशन करता है। वह पहचाने गए बनाए ढाँचे, मेल खाती पैकेज फ़ाइलों और अनुपस्थित `node_modules` को स्वीकार करता है; वह सामान्य पैकेज मैनेजर का विकल्प नहीं है। बदली निर्भरता फ़ाइलें, दूसरे प्रतिस्पर्धी लॉक या मौजूदा आउटपुट अस्वीकृति का कारण हो सकते हैं। ओवरराइट फ़्लैग जोड़ने के बजाय बताई संरचना आवश्यकता ठीक करें। निर्भरता प्राप्त करने में नेटवर्क इस्तेमाल हो सकता है।

## एक स्पष्ट MCP जाँच चलाएँ {#run-one-explicit-mcp-check}

लोकल stdio निष्पादन के लिए अभी Linux, सिस्टम `bwrap` (bubblewrap), काम करता प्रक्रिया नियंत्रण (delegated cgroup v2 सहित), नेमस्पेस समर्थन और सिस्टम रनटाइम के भीतर मिला Node (`/usr`, `/bin`, `/lib` या `/lib64`) चाहिए। पैकेज कोड शुरू करने से पहले प्रक्रिया नियंत्रण और bubblewrap की शुरुआती जाँचें सफल होनी चाहिए। जहाँ यह स्थापित न हो सके, कमांड निष्पादन अस्वीकार करती है। उपयोगकर्ता के version-manager डायरेक्टरी में स्थापित Node इस रनटाइम अनुबंध को पूरा नहीं करता। स्थिर जाँचें अलग उपलब्ध रहती हैं।

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

यह चुनी लोकल प्रक्रिया चलाता है और MCP प्रारंभिकरण तथा टूल खोज जाँचता है। `hello` कॉल करने के लिए स्पष्ट JSON आर्ग्युमेंट फ़िक्स्चर चाहिए; [रनटाइम जाँच का उदाहरण](./checks#evaluate-mcp-runtime-explicitly) देखें। लगातार सुधार के लिए:

```bash
agentplugins author dev ./local-tools --server local-tools
```

रनटाइम परिणाम के साथ सटीक इंजन, पैकेज और प्लेटफ़ॉर्म पहचान रखें। प्रत्यक्ष पैकेज निर्माण जाँच यह सिद्ध नहीं करती कि वही सर्वर हर नेटिव एजेंट के माध्यम से काम करेगा या क्लाइंट ने ट्रस्ट दिया और प्रमाणीकरण पूरा किया है।

## सौंपने की तैयारी करें {#prepare-the-handoff}

README में टूल, Node.js पूर्वापेक्षा, निर्भरता फ़ाइलें और रनटाइम प्रमाण समझाएँ। चुना सर्वर तथा न जाँचे टूल या परिवेश बताएँ। फिर [इंस्टॉलर को सौंपने की प्रक्रिया](./handoff) से पैकेज साझा करें। टूल के साथ मार्गदर्शन चाहिए तो [अतिरिक्त Skills](./skills) से निर्देश जोड़ें।
