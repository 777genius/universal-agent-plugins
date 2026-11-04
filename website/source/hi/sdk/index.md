---
title: "SDK और एकीकरण"
description: "Hooks, पहचान, इंस्टॉलेशन और प्रबंधन के लिए सही SDK चुनें।"
canonicalId: "page:sdk:index"
section: "sdk"
locale: "hi"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# SDK और एकीकरण

जब आपको **Universal Agent Plugins को अपने ऐप में जोड़ना हो** या एजेंट के native hooks संभालने हों, तब SDK उपयोगी हैं। सामान्य इंस्टॉलेशन के लिए [प्लगइन उपयोग](../use/) देखें।

## सही SDK चुनें

| काम | गाइड | API |
| --- | --- | --- |
| Native hook event संभालना | [Go Hooks](./go-hooks) | `github.com/777genius/plugin-kit-ai/sdk` |
| Hook चलाने वाले एजेंट का पता लगाना | [Host detection](./host-detection) | `sdk/hostdetect` |
| Go ऐप में इंस्टॉल, अपडेट, रिपेयर और रिकवरी | [Go Installer](./go-installer) | `install/integrationctl/agentplugins/installer` |
| Node/Electron से प्लगइन प्रबंधित करना | [TypeScript Management](./typescript-management) | `universal-agent-plugins-management-sdk` |

## संस्करण और सीमाएँ

- प्रकाशित Go Hooks मॉड्यूल: `github.com/777genius/plugin-kit-ai/sdk@v1.3.0`। स्थिरता **हर event के लिए अलग** होती है।
- `sdk/hostdetect` वर्तमान स्रोत में मौजूद है; जिस रिलीज़ टैग का उपयोग करेंगे उसमें पैकेज होना सुनिश्चित करें।
- Go Installer एक **अलग मॉड्यूल** है; उसकी जाँची हुई source revision और dependencies pin करें, Hooks SDK का version न मानें।
- TypeScript Management SDK का स्रोत संस्करण **0.1.1**, Node **22.18+**, Management API **v1** है। npm पर उपलब्धता जाँचें।
- CLI **0.1.95+** में `manage serve` और `manage discover` हैं, लेकिन `checkUpdates` **CLI 0.1.95 और पुराने संस्करणों में नहीं है**।

`hostdetect.Detect` **मौजूदा hook चलाने वाले एजेंट** को पहचानता है। `discoverTargets` **इंस्टॉल किए हुए client और निजी profiles** ढूँढता है। `Engine.Discover` केवल दर्ज किए गए Go adapters दिखाता है। इनमें से कोई भी अपने आप इंस्टॉल नहीं करता। Schema validation, फ़ाइल इंस्टॉलेशन, native activation और MCP authentication अलग जाँच हैं।

आगे पढ़ें: [Go Hooks](./go-hooks), [डिटेक्शन](./host-detection), [Go Installer](./go-installer), [TypeScript SDK](./typescript-management)।
