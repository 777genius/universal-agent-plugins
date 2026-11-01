---
title: "त्वरित शुरुआत"
description: "उपलब्ध प्लगइन इंस्टॉल करें या पोर्टेबल Agent Plugins 1.0 पैकेज तैयार करें।"
canonicalId: "page:guide:quickstart"
section: "guide"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

<a id="quickstart"></a>

# प्लगइन इस्तेमाल करें / प्लगइन बनाएँ {#use-plugins--build-plugins}

अपने एजेंटों के लिए पैकेज इंस्टॉल करें, या निर्देशों और टूल कॉन्फ़िगरेशन वाला पोर्टेबल Agent Plugins 1.0 पैकेज बनाएँ।

## प्लगइन इस्तेमाल करें {#use-plugins}

<a id="optional-first-proof"></a>

Node.js 22 या उसके बाद के संस्करण के साथ:

```bash
npx universal-agent-plugins add context7
```

CLI पूछता है कि किन संगत एजेंटों का इस्तेमाल करना है और बाकी सक्रियण चरण बताता है। पहले जानकारी और पूर्वावलोकन देखने के लिए:

```bash
npx universal-agent-plugins info context7
npx universal-agent-plugins add context7 --target codex,cursor --dry-run
```

[नेटिव इंस्टॉलेशन गाइड](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) में macOS, Linux और Windows के एक्ज़ीक्यूटेबल भी मिलते हैं। नेटिव मैनेजर को Node.js नहीं चाहिए, हालाँकि अलग प्लगइन को अपने रनटाइम चाहिए हो सकते हैं। npm इस्तेमाल करते समय `npx universal-agent-plugins version` जाँचें: पैकेज मैनेजर चैनल अब भी पुराना CLI दे सकता है। नीचे के स्वचालित चयन के लिए 0.1.94 आवश्यक है; ज़रूरत हो तो नेटिव इंस्टॉलर इस्तेमाल करें। इस गाइड की स्पष्ट प्रोजेक्ट कमांड 0.1.93 पर भी काम करती हैं।

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) में स्पष्ट प्रोजेक्ट स्कोप और 0.1.94 में आया स्वचालित `install` चयन उपलब्ध रहता है। इसमें [लोकल मैनेजमेंट API](../use/manage#local-management-api) और [Pi के नेटिव एक्सटेंशन](../reference/client-compatibility#pi-native-extensions) भी जोड़े गए हैं। प्रोजेक्ट में पैकेज जोड़ने का पूर्वावलोकन देखें:

संगत और समीक्षा की गई कैटलॉग प्रविष्टि के लिए पूरा commit SHA वाला Git स्रोत और उपयुक्त एजेंट चुनें। Directory v1 के `context7` जैसे उपनाम केवल user scope में मान्य हैं। `OWNER/REPOSITORY`, `FULL_COMMIT_SHA` और `PACKAGE_PATH` बदलें; पैकेज रिपॉज़िटरी की जड़ में हो तो `//PACKAGE_PATH` हटाएँ।

```bash
npx universal-agent-plugins add 'github:OWNER/REPOSITORY@FULL_COMMIT_SHA//PACKAGE_PATH' --target codex --scope project \
  --project-root /path/to/project --dry-run
```

मौजूदा प्रोजेक्ट डायरेक्टरी चुनें। Add और रखरखाव कमांड डिफ़ॉल्ट रूप से उपयोगकर्ता स्कोप चुनती हैं, इसलिए प्रोजेक्ट स्कोप स्पष्ट दें। 0.1.94 में स्रोत आर्ग्युमेंट के बिना `install` प्रोजेक्ट तभी चुनता है जब ठीक वर्तमान फ़ोल्डर में नियमित `uap.toml` फ़ाइल हो। स्पष्ट स्कोप प्राथमिक है और पैरेंट नहीं खोजे जाते। `--dry-run` हटाने से पहले योजना देखें। नेटिव पहचान, ट्रस्ट, प्रमाणीकरण और सफल MCP कॉल पैकेज की फ़ाइलें लिखने से अलग जाँचें हैं।

[पूरी इंस्टॉलेशन प्रक्रिया](../use/install) ·
[क्लाइंट संगतता और परीक्षण सीमाएँ](../reference/client-compatibility)

### सहेजा हुआ निर्भरता सेट बहाल करें {#restore-a-saved-dependency-set}

चुने हुए उपयोगकर्ता परिवेश में दर्ज जानकारी देखें:

```bash
npx universal-agent-plugins deps status
```

सहेजी हुई `uap.toml`/`uap.lock` जोड़ी के लिए आदान-प्रदान फ़ोल्डर से एक्सपोर्ट करें:

```bash
npx universal-agent-plugins deps export
```

फ़ोल्डर दूसरी मशीन पर कॉपी करें, उसके टार्गेट क्लाइंट इंस्टॉल करें और वहाँ चलाएँ:

```bash
npx universal-agent-plugins deps import
npx universal-agent-plugins install --locked --scope user
```

केवल वैलिडेट किया गया अनुबंध और लॉक स्थानांतरित होते हैं। प्रोफ़ाइल, सेटिंग, क्रेडेंशियल, रसीदें और प्लगइन डेटा अलग रहते हैं। इंस्टॉलेशन को अब भी सटीक स्रोत या सत्यापित कैश किए पैकेज चाहिए। प्रोजेक्ट के लिए फ़ाइलों की जोड़ी कमिट करें और स्पष्ट `install --locked --scope project` से बहाल करें। रिकवरी और स्थानांतरण के विवरण के लिए [रखरखाव और बहाली](../use/manage) देखें।

## प्लगइन बनाएँ {#build-plugins}

<a id="recommended-default"></a>
<a id="if-you-only-read-one-thing"></a>

मानक पैकेज की शुरुआत रूट `plugin.json` से होती है। इसमें `skills/<name>/SKILL.md`, `mcp.json` या दोनों हो सकते हैं। ये फ़ाइलें हाथ से लिखकर प्रकाशित इंस्टॉलर से पैकेज वैलिडेट कर सकते हैं:

```bash
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add ./my-plugin --target codex --dry-run
```

इन उदाहरणों में माना गया है कि `./my-plugin` में पहले से पूरा पैकेज मौजूद है। [बिल्ड गाइड](../build/) एक छोटा Skill पैकेज हाथ से बनाने और सोर्स बिल्ड के ढाँचा तैयार करने वाले टूल समझाती है।

**पैकेज निर्माण की उपलब्धता:** वर्तमान npm और नेटिव इंस्टॉलर रिलीज़ में `agentplugins author` शामिल नहीं है। वर्तमान सोर्स में ढाँचा निर्माण, स्थिर जाँचें, स्पष्ट MCP रनटाइम परीक्षण, विकास चक्र, निर्भरता bootstrap और JSON रखरखाव लागू हैं। उन कमांड के लिए [सोर्स बिल्ड सेटअप](../build/#build-from-source) इस्तेमाल करें। पुरानी पैकेज निर्माण रिलीज़ के प्रमाण अपने संस्करण तक सीमित हैं; वे वर्तमान इंस्टॉलर डाउनलोड की सुविधाओं का वादा नहीं हैं।

स्थिर वैलिडेशन रनटाइम, OAuth या सक्रियण सिद्ध नहीं करता। MCP ट्रांसपोर्ट समर्थन भी क्लाइंट के अनुसार बदलता है; उदाहरण के लिए Codex घोषित MCP SSE का समर्थन नहीं करता। पैकेज की आवश्यकताएँ और बाकी जाँचें README में स्पष्ट रखें।

[Agent Plugins 1.0 विनिर्देश](https://agent-plugins.org/specification)
