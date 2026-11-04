---
title: "Go Installer SDK"
description: "योजना की समीक्षा और पुनर्प्राप्ति के साथ इंस्टॉलर जोड़ें।"
canonicalId: "page:sdk:go-installer"
section: "sdk"
locale: "hi"
generated: false
translationRequired: true
localeDisposition: "current-translation"
---

# Go Installer SDK

यह Go facade आपके ऐप में **install, update, repair, refresh projection, remove, inspect और recover** जोड़ता है। यह Go Hooks SDK से अलग है।

## मॉड्यूल और उपलब्धता

```text
github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer
```

यह `github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins` मॉड्यूल का हिस्सा है। जाँची हुई source revision और dependencies pin करें; Hooks SDK की `v1.3.0` रिलीज़ इस installer पर लागू **नहीं** मानी जा सकती।

## Engine और client discovery

```go
registry, err := clients.NewRegistry(codex.New(), claude.New())
if err != nil { return err }

engine, err := uapinstaller.New(uapinstaller.Config{
    StateRoot: "/absolute/private/uap-state",
    Registry: registry,
})
if err != nil { return err }

available := engine.Discover()
_ = available // inspect scopes, capabilities and detected executables
```

`StateRoot` एक पूर्ण (absolute) पथ और `Registry` स्पष्ट रूप से चुने हुए adapters का संग्रह होना चाहिए। `New` केवल configuration सत्यापित करता है; **फ़ोल्डर नहीं बनाता या एजेंट नहीं चलाता**। `Discover` registered adapters की capabilities, scopes और executable मौजूदगी बताता है; native activation की गारंटी नहीं। वास्तविक workflow में helper, runner, client profile और content-trust policy भी चुनें।

## योजना → समीक्षा → पुष्टि → लागू करें

```go
req := uapinstaller.Request{
    Operation:        uapinstaller.OpInstall,
    PackageRoot:      "/absolute/reviewed/package",
    ClientID:         "codex",
    ClientConfigRoot: "/absolute/codex-profile",
    ClientExecutable: "/absolute/path/to/codex",
    InstallationID:   "00000000-0000-4000-8000-000000000099",
    OperationID:      "host-generated-unique-operation-id",
}
prepared, err := engine.Prepare(ctx, req)
if err != nil { return err }
defer prepared.Close()

plan := prepared.Plan()
// Show plan, targets, components, warnings and digests to the user.
// Only after explicit approval, with immutable prepared inputs:
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
if err != nil { return err }
_ = result

view, err := engine.Inspect(ctx)
if err != nil { return err }
if view.Recovery.Required {
    _, err = engine.Recover(ctx, view)
    if err != nil { return err }
}
```

यह **code fragment** है; इसके लिए `ctx`, `engine` और [पूरे नमूने](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/example/main.go) के imports चाहिए। `Decision{Confirmed:true}` से **पहले वास्तविक immutable Plan दिखाएँ**। `Apply` authority और digest फिर जाँचता है। `Inspect` read-only है; `Recover` को वही typed observation देना होता है।

`ClientConfigRoot` को `HOME` या `CODEX_HOME` से अनुमानित नहीं किया जाता। Local bytes को `Assess` या digest-bound `Request.Assessment` से मंज़ूर करें; `TrustedLocalPackages` सिर्फ स्पष्ट रूप से पहले से विश्वसनीय सामग्री के लिए है। `Request.Targets` से सीमित multi-client समूह बनते हैं; OpenCode group और removal की अपनी सीमाएँ हैं। पुरानी state में `PrepareStateMigration` / `ApplyStateMigration` का अलग review और backup आवश्यक है।

[पूरा Installer contract](https://github.com/777genius/universal-agent-plugins/blob/main/install/integrationctl/agentplugins/installer/README.md)।
