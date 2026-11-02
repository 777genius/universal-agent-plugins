# `plugin-kit-ai-runtime` npm package

> **Supported legacy runtime helper.** The existing helper API remains supported
> for the earlier `plugin-kit-ai` runtime workflow. It is separate from the
> retired YAML CLI and from the current public installer,
> `universal-agent-plugins` (`agentplugins`). Start new portable packages with
> the [current documentation](https://universal-agent-plugins.com/docs/en/).
> The examples below describe this helper API; they do not add legacy authoring
> commands to the current installer release.

Official Node/TypeScript helper package for launcher-based `plugin-kit-ai` plugins.

Use it when you want the supported handler-oriented API in a shared dependency instead of copying a local helper file into every repo.

Install:

```bash
npm i plugin-kit-ai-runtime
```

Example:

```ts
import { CodexApp, continue_ } from "plugin-kit-ai-runtime";

const app = new CodexApp().onNotify((event) => {
  void event;
  return continue_();
});

process.exit(app.run());
```

Notes:

- Go is still the recommended path when you want the most self-contained delivery model.
- Node authoring remains a stable supported lane, but the machine running the plugin still needs Node.js `20+`.
- The helper API mirrors the generated `plugin/plugin-runtime.ts` and `plugin/plugin-runtime.mjs` scaffold surface.
