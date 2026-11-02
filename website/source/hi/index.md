---
title: "Agent Plugins दस्तावेज़"
description: "Agent Plugins के सार्वजनिक दस्तावेज़।"
canonicalId: "page:home"
section: "home"
locale: "hi"
localeDisposition: "current-translation"
generated: false
translationRequired: true
---

<div class="docs-hero docs-hero--feature">
  <p class="docs-kicker">सार्वजनिक दस्तावेज़</p>
  <h1>Agent Plugins</h1>
  <p class="docs-lead">अपने एजेंटों के लिए Skills और MCP टूल इंस्टॉल करें, या अपना पोर्टेबल पैकेज बनाएँ।</p>
</div>

## अपनी शुरुआत चुनें {#choose-your-journey}

<div class="docs-grid">
  <a class="docs-card" href="./use/">
    <h2>प्लगइन इस्तेमाल करें</h2>
    <p>पैकेज की जाँच करें, बदलावों का पूर्वावलोकन देखें और उसे अपने खाते या किसी एक प्रोजेक्ट के लिए इंस्टॉल करें।</p>
  </a>
  <a class="docs-card" href="./build/">
    <h2>प्लगइन बनाएँ</h2>
    <p>plugin.json, Skills और MCP कॉन्फ़िगरेशन के साथ मानक पैकेज लिखें। सोर्स से बने पैकेज निर्माण टूल जानें।</p>
  </a>
</div>

## अपना पहला प्लगइन इंस्टॉल करें {#install-your-first-plugin}

Node.js 22 या उसके बाद के संस्करण के साथ:

```bash
npx universal-agent-plugins add context7
```

CLI पूछता है कि किन संगत एजेंटों का इस्तेमाल करना है। योजना की समीक्षा करें और बताए गए सक्रियण या साइन-इन चरण पूरे करें। नेटिव एक्ज़ीक्यूटेबल चाहिए? macOS, Linux या Windows के लिए [नेटिव इंस्टॉलेशन गाइड](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) देखें। नेटिव मैनेजर को स्वयं Node.js की ज़रूरत नहीं होती।

## अभी क्या उपलब्ध है {#what-is-available}

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) वर्तमान नेटिव इंस्टॉलर रिलीज़ है, जो 29 अक्टूबर 2026 को प्रकाशित हुई। इसमें लोकल मैनेजमेंट API और उपयोगकर्ता स्कोप में Pi के नेटिव एक्सटेंशन जोड़े गए हैं। [लोकल मैनेजमेंट](./use/manage#local-management-api) और [Pi की संगतता](./reference/client-compatibility#pi-native-extensions) देखें।

इसमें स्पष्ट प्रोजेक्ट स्कोप और लॉक के अनुसार बहाली उपलब्ध रहती है। अधिकांश कमांड डिफ़ॉल्ट रूप से उपयोगकर्ता परिवेश पर काम करती हैं। 0.1.94 से, स्रोत आर्ग्युमेंट के बिना `install` वर्तमान प्रोजेक्ट तभी चुनता है जब ठीक उसी फ़ोल्डर में नियमित `uap.toml` फ़ाइल हो; स्पष्ट स्कोप प्राथमिक होता है। प्रोजेक्ट के लिए add और रखरखाव कमांड में `--scope project` दें। [इंस्टॉलेशन](./use/install) और [रखरखाव](./use/manage) देखें।

अलग वितरण चैनलों के संस्करण अलग हो सकते हैं। `agentplugins version` जाँचें; सत्यापित 0.1.95 के लिए [नेटिव इंस्टॉलर](https://github.com/777genius/universal-agent-plugins/blob/main/docs/NATIVE_INSTALL.md) इस्तेमाल करें। स्वचालित प्रोजेक्ट चयन के लिए 0.1.94 या बाद का संस्करण चाहिए; स्पष्ट प्रोजेक्ट स्कोप 0.1.93 से उपलब्ध है।

वर्तमान इंस्टॉलर में `agentplugins author` शामिल नहीं है। [बिल्ड गाइड](./build/) में हाथ से पैकेज बनाना और वर्तमान सोर्स में लागू पैकेज निर्माण कमांड समझाई गई हैं। पुरानी पैकेज निर्माण रिलीज़ के प्रमाण अपने मूल संस्करण से जुड़े रहते हैं; वे वर्तमान इंस्टॉलर डाउनलोड का विवरण नहीं हैं।

## परिणाम का अर्थ समझें {#understand-the-result}

पैकेज वैलिडेशन, इंस्टॉलेशन, क्लाइंट द्वारा पहचान, MCP टूल का निष्पादन और सेवा का प्रमाणीकरण अलग-अलग जाँचें हैं। फ़ाइलें सफलतापूर्वक इंस्टॉल होने के बाद भी क्लाइंट का ट्रस्ट, रीलोड या नया सत्र आवश्यक हो सकता है। नेटिव परीक्षण विशिष्ट क्लाइंट संस्करणों और प्लेटफ़ॉर्म को कवर करते हैं; वे हर प्लगइन या हर एजेंट को प्रमाणित नहीं करते।

- [त्वरित शुरुआत](./guide/quickstart)
- [पैकेज की संरचना](./build/layout)
- [जाँचें और प्रमाण](./build/checks)
- [क्लाइंट संगतता और परीक्षण की सीमाएँ](./reference/client-compatibility)
- [Agent Plugins 1.0 विनिर्देश](https://agent-plugins.org/specification)
