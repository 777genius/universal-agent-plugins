import { tool } from "@opencode-ai/plugin"

export const CustomToolPlugin = async (ctx) => {
  void ctx
  return {
    tool: {
      opencodeBasicEcho: tool({
        description: "Echo a short value back to confirm the custom tool wiring.",
        args: {
          value: tool.schema.string().describe("Value to echo back"),
        },
        async execute(args) {
          return {
            ok: true,
            value: args.value,
          }
        },
      }),
    },
  }
}
