import { MCPClient } from "@mastra/mcp";
import { requireEnv } from "../env.js";

const exaUrl = new URL("https://mcp.exa.ai/mcp");
exaUrl.searchParams.set("exaApiKey", requireEnv("EXA_API_KEY"));

export const mcpExa = new MCPClient({
  id: "find-food-exa",
  servers: {
    exa: {
      url: exaUrl
    }
  }
});

export async function exaTools() {
  return mcpExa.listTools();
}

export async function exaSearchTools() {
  const tools = await mcpExa.listTools();
  const entries = Object.entries(tools).filter(([name]) => name.toLowerCase().includes("search"));
  return Object.fromEntries(entries) as typeof tools;
}
