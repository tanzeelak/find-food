export type UiEvent =
  | { kind: "text"; text: string }
  | { kind: "reasoning"; text: string }
  | { kind: "tool-start"; label: string }
  | { kind: "tool-end"; label: string; isError: boolean }
  | { kind: "error"; message: string };

// Loose shape: Mastra's fullStream chunk payloads are a strict union, so we
// accept them structurally and narrow fields defensively at runtime.
type StreamChunk = { type: string; payload?: Record<string, unknown> };

export function toolLabel(toolName: string, args?: Record<string, unknown>): string {
  const name = toolName.toLowerCase();
  if (name.includes("research")) {
    const restaurant = typeof args?.restaurantName === "string" ? args.restaurantName : "";
    return restaurant ? `researching ${restaurant}` : "researching a restaurant";
  }
  if (name.includes("search")) {
    const query = typeof args?.query === "string" ? args.query : "";
    return query ? `searching the web: "${query}"` : "searching the web";
  }
  if (name.includes("fetch") || name.includes("crawl") || name.includes("content")) {
    return "reading a page";
  }
  return `using ${toolName}`;
}

export async function* normalizeStream(fullStream: AsyncIterable<unknown>): AsyncGenerator<UiEvent> {
  for await (const raw of fullStream) {
    const chunk = raw as StreamChunk;
    const payload = chunk.payload ?? {};
    switch (chunk.type) {
      case "text-delta": {
        const text = typeof payload.text === "string" ? payload.text : "";
        if (text) {
          yield { kind: "text", text };
        }
        break;
      }
      case "reasoning-delta": {
        const text = typeof payload.text === "string" ? payload.text : "";
        if (text) {
          yield { kind: "reasoning", text };
        }
        break;
      }
      case "tool-call": {
        const toolName = typeof payload.toolName === "string" ? payload.toolName : "tool";
        yield { kind: "tool-start", label: toolLabel(toolName, payload.args as Record<string, unknown>) };
        break;
      }
      case "tool-result": {
        const toolName = typeof payload.toolName === "string" ? payload.toolName : "tool";
        yield {
          kind: "tool-end",
          label: toolLabel(toolName, payload.args as Record<string, unknown>),
          isError: payload.isError === true
        };
        break;
      }
      case "tool-error": {
        const toolName = typeof payload.toolName === "string" ? payload.toolName : "tool";
        yield { kind: "tool-end", label: toolLabel(toolName, payload.args as Record<string, unknown>), isError: true };
        break;
      }
      case "error": {
        const err = payload.error ?? payload.message ?? payload;
        yield { kind: "error", message: typeof err === "string" ? err : JSON.stringify(err) };
        break;
      }
      default:
        break;
    }
  }
}
